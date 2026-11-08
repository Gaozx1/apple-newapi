package model

import (
	"errors"
	"fmt"
	"strings"

	"github.com/QuantumNous/new-api/common"

	"gorm.io/gorm"
)

// ============================================================================
// Tickets (工单)
//
// Users submit support tickets; admins reply and close them. Each ticket is
// owned by the submitting user. Replies form a chronologically ordered thread
// stored in a separate table so a ticket can hold many messages without
// bloating the parent row.
// ============================================================================

// TicketStatus enumerates the lifecycle states of a support ticket.
type TicketStatus string

const (
	TicketStatusOpen   TicketStatus = "open"
	TicketStatusClosed TicketStatus = "closed"
)

// Ticket is a single support ticket submitted by a user.
//
// AssigneeId is the administrator who claimed the ticket and is responsible
// for handling it; 0 means the ticket is still in the unclaimed pool. A
// claimed ticket may only be replied to and closed by its assignee.
type Ticket struct {
	Id           int64        `json:"id" gorm:"primaryKey;autoIncrement"`
	UserId       int          `json:"user_id" gorm:"not null;index"`
	Username     string       `json:"username" gorm:"type:varchar(64);not null;default:''"`
	Subject      string       `json:"subject" gorm:"type:varchar(255);not null"`
	Content      string       `json:"content" gorm:"type:text;not null"`
	Status       TicketStatus `json:"status" gorm:"type:varchar(16);not null;default:'open';index"`
	AssigneeId   int          `json:"assignee_id" gorm:"not null;default:0;index"`
	AssigneeName string       `json:"assignee_name" gorm:"type:varchar(64);not null;default:''"`
	ClaimedAt    int64        `json:"claimed_at" gorm:"bigint;not null;default:0"`
	CreatedAt    int64        `json:"created_at" gorm:"bigint"`
	UpdatedAt    int64        `json:"updated_at" gorm:"bigint"`
}

func (Ticket) TableName() string {
	return "tickets"
}

// TicketReply is a single message within a ticket thread. Admin replies and
// user replies are both rows; IsAdmin distinguishes who authored the message.
type TicketReply struct {
	Id        int64  `json:"id" gorm:"primaryKey;autoIncrement"`
	TicketId  int64  `json:"ticket_id" gorm:"not null;index"`
	UserId    int    `json:"user_id" gorm:"not null"`
	IsAdmin   bool   `json:"is_admin" gorm:"not null;index"`
	Content   string `json:"content" gorm:"type:text;not null"`
	CreatedAt int64  `json:"created_at" gorm:"bigint"`
}

func (TicketReply) TableName() string {
	return "ticket_replies"
}

// MaxTicketSubjectLen bounds the subject length to keep the column sane.
const MaxTicketSubjectLen = 255

// MaxTicketContentLen bounds a single message body to avoid abuse.
const MaxTicketContentLen = 5000

var (
	ErrTicketNotFound       = errors.New("ticket not found")
	ErrTicketClosed         = errors.New("ticket is closed")
	ErrTicketEmptyField     = errors.New("ticket subject and content cannot be empty")
	ErrTicketNotClaimed     = errors.New("ticket is not claimed")
	ErrTicketClaimedByOther = errors.New("ticket is claimed by another administrator")
)

// Admin ticket list filters for the assignee column.
const (
	TicketAssigneePending = "pending"
	TicketAssigneeMine    = "mine"
)

// TicketListFilter narrows the admin ticket list. Status is optional;
// Assignee is TicketAssigneePending or TicketAssigneeMine, empty lists every
// ticket.
type TicketListFilter struct {
	Status   TicketStatus
	Assignee string
	UserId   int
}

// CreateTicket creates a new open ticket with the author's opening message as
// the first reply in the thread. Runs atomically so a ticket always has its
// opening message.
func CreateTicket(userId int, username, subject, content string) (*Ticket, error) {
	if userId == 0 {
		return nil, errors.New("user id is empty")
	}
	subject = trimAndLimit(subject, MaxTicketSubjectLen)
	content = trimAndLimit(content, MaxTicketContentLen)
	if subject == "" || content == "" {
		return nil, ErrTicketEmptyField
	}

	now := common.GetTimestamp()
	ticket := &Ticket{
		UserId:    userId,
		Username:  username,
		Subject:   subject,
		Content:   content,
		Status:    TicketStatusOpen,
		CreatedAt: now,
		UpdatedAt: now,
	}
	reply := &TicketReply{
		UserId:    userId,
		IsAdmin:   false,
		Content:   content,
		CreatedAt: now,
	}

	err := DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(ticket).Error; err != nil {
			return err
		}
		reply.TicketId = ticket.Id
		return tx.Create(reply).Error
	})
	if err != nil {
		return nil, err
	}
	return ticket, nil
}

// GetTicket returns a ticket by id with its full reply thread ordered
// chronologically. The caller must verify ownership or admin rights.
func GetTicket(ticketId int64) (*Ticket, []TicketReply, error) {
	var ticket Ticket
	if err := DB.First(&ticket, ticketId).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil, ErrTicketNotFound
		}
		return nil, nil, err
	}
	var replies []TicketReply
	if err := DB.Where("ticket_id = ?", ticketId).Order("id ASC").Find(&replies).Error; err != nil {
		return nil, nil, err
	}
	return &ticket, replies, nil
}

// GetUserTickets returns a user's tickets (newest first) with pagination.
func GetUserTickets(userId int, page, pageSize int) ([]Ticket, int64, error) {
	var tickets []Ticket
	var total int64
	if err := DB.Model(&Ticket{}).Where("user_id = ?", userId).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	offset := (page - 1) * pageSize
	err := DB.Where("user_id = ?", userId).Order("id DESC").
		Offset(offset).Limit(pageSize).Find(&tickets).Error
	if err != nil {
		return nil, 0, err
	}
	return tickets, total, nil
}

// GetAllTickets returns tickets (newest first) with optional status and
// assignee filters and pagination. Admin use.
func GetAllTickets(filter TicketListFilter, page, pageSize int) ([]Ticket, int64, error) {
	query := DB.Model(&Ticket{})
	if filter.Status != "" {
		query = query.Where("status = ?", filter.Status)
	}
	switch filter.Assignee {
	case TicketAssigneePending:
		query = query.Where("assignee_id = 0")
	case TicketAssigneeMine:
		query = query.Where("assignee_id = ?", filter.UserId)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	offset := (page - 1) * pageSize
	var tickets []Ticket
	err := query.Order("id DESC").Offset(offset).Limit(pageSize).Find(&tickets).Error
	if err != nil {
		return nil, 0, err
	}
	return tickets, total, nil
}

// AddTicketReply appends a reply to a ticket. The ticket must be open; both
// user and admin replies are permitted, but a closed ticket rejects new
// replies. An admin may reply only after claiming the ticket, and only the
// assignee may reply among admins. Updates the parent's UpdatedAt so lists
// reflect recent activity.
func AddTicketReply(ticketId int64, userId int, isAdmin bool, content string) (*TicketReply, error) {
	content = trimAndLimit(content, MaxTicketContentLen)
	if content == "" {
		return nil, ErrTicketEmptyField
	}
	now := common.GetTimestamp()
	reply := &TicketReply{
		TicketId:  ticketId,
		UserId:    userId,
		IsAdmin:   isAdmin,
		Content:   content,
		CreatedAt: now,
	}
	err := DB.Transaction(func(tx *gorm.DB) error {
		var ticket Ticket
		if err := lockForUpdate(tx).First(&ticket, ticketId).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrTicketNotFound
			}
			return err
		}
		if ticket.Status == TicketStatusClosed {
			return ErrTicketClosed
		}
		if isAdmin && ticket.AssigneeId != userId {
			if ticket.AssigneeId == 0 {
				return ErrTicketNotClaimed
			}
			return ErrTicketClaimedByOther
		}
		if err := tx.Create(reply).Error; err != nil {
			return err
		}
		return tx.Model(&Ticket{}).Where("id = ?", ticketId).
			Update("updated_at", now).Error
	})
	if err != nil {
		return nil, err
	}
	return reply, nil
}

// ClaimTicket assigns an open ticket to the administrator who takes it over.
// Claiming a ticket someone else already holds fails, and claiming one's own
// ticket is a no-op. The conditional update makes concurrent claims safe.
func ClaimTicket(ticketId int64, assigneeId int, assigneeName string) (*Ticket, error) {
	if assigneeId == 0 {
		return nil, errors.New("assignee id is empty")
	}
	now := common.GetTimestamp()
	res := DB.Model(&Ticket{}).
		Where("id = ? AND status = ? AND assignee_id = 0", ticketId, TicketStatusOpen).
		Updates(map[string]any{
			"assignee_id":   assigneeId,
			"assignee_name": assigneeName,
			"claimed_at":    now,
			"updated_at":    now,
		})
	if res.Error != nil {
		return nil, res.Error
	}
	var ticket Ticket
	if err := DB.First(&ticket, ticketId).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrTicketNotFound
		}
		return nil, err
	}
	if ticket.Status == TicketStatusClosed {
		return nil, ErrTicketClosed
	}
	if ticket.AssigneeId != assigneeId {
		return nil, ErrTicketClaimedByOther
	}
	return &ticket, nil
}

// UnclaimTicket returns a claimed open ticket to the pending pool. The
// assignee releases their own claim; a root administrator may release any
// claim so a queue never gets stuck. A ticket that nobody claimed yields
// ErrTicketNotClaimed, and a closed ticket keeps its handler for the record.
func UnclaimTicket(ticketId int64, operatorId int, force bool) error {
	query := DB.Model(&Ticket{}).
		Where("id = ? AND status = ? AND assignee_id <> 0", ticketId, TicketStatusOpen)
	if !force {
		query = query.Where("assignee_id = ?", operatorId)
	}
	res := query.Updates(map[string]any{
		"assignee_id":   0,
		"assignee_name": "",
		"claimed_at":    0,
		"updated_at":    common.GetTimestamp(),
	})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected > 0 {
		return nil
	}
	var ticket Ticket
	if err := DB.Select("id", "status", "assignee_id").First(&ticket, ticketId).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrTicketNotFound
		}
		return err
	}
	if ticket.Status == TicketStatusClosed {
		return ErrTicketClosed
	}
	if ticket.AssigneeId == 0 {
		return ErrTicketNotClaimed
	}
	return ErrTicketClaimedByOther
}

// CloseTicket marks a ticket as closed. Admin only; the caller performs authz.
// An administrator must hold the ticket's claim to close it.
func CloseTicket(ticketId int64, adminId int) error {
	now := common.GetTimestamp()
	res := DB.Model(&Ticket{}).
		Where("id = ? AND status = ? AND assignee_id = ?", ticketId, TicketStatusOpen, adminId).
		Updates(map[string]any{
			"status":     TicketStatusClosed,
			"updated_at": now,
		})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected > 0 {
		return nil
	}
	var ticket Ticket
	if err := DB.Select("id", "status", "assignee_id").First(&ticket, ticketId).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrTicketNotFound
		}
		return err
	}
	if ticket.Status == TicketStatusClosed {
		return ErrTicketClosed
	}
	if ticket.AssigneeId == 0 {
		return ErrTicketNotClaimed
	}
	return ErrTicketClaimedByOther
}

// trimAndLimit trims surrounding whitespace and clamps the string to maxLen.
func trimAndLimit(s string, maxLen int) string {
	s = strings.TrimSpace(s)
	if len(s) > maxLen {
		s = s[:maxLen]
	}
	return s
}

// deleteTicketCascade removes a ticket and its replies. Reserved for future
// admin purge; not wired to a route yet.
func deleteTicketCascade(tx *gorm.DB, ticketId int64) error {
	if err := tx.Where("ticket_id = ?", ticketId).Delete(&TicketReply{}).Error; err != nil {
		return err
	}
	res := tx.Delete(&Ticket{}, ticketId)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ErrTicketNotFound
	}
	return nil
}

// ticketSummary is a small debug helper used in logs.
func ticketSummary(t *Ticket) string {
	return fmt.Sprintf("ticket=%d user=%d status=%s", t.Id, t.UserId, t.Status)
}
