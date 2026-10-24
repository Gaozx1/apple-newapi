package model

import (
	"errors"
	"fmt"
	"strings"

	"github.com/QuantumNous/new-api/common"

	"gorm.io/gorm"
)

// After-sales ticket status.
const (
	StoreTicketStatusOpen      = 0
	StoreTicketStatusRefunded  = 1
	StoreTicketStatusComplaint = 2
	StoreTicketStatusResolved  = 3
	StoreTicketStatusClosed    = 4
)

// After-sales conversation roles.
const (
	StoreTicketRoleBuyer  = 0
	StoreTicketRoleSeller = 1
	StoreTicketRoleAdmin  = 2
)

var (
	ErrStoreTicketNotFound  = errors.New("售后单不存在")
	ErrStoreTicketForbidden = errors.New("无权操作该售后单")
	ErrStoreTicketClosed    = errors.New("售后单已结束")
	ErrStoreTicketRefunded  = errors.New("该订单已退款")
)

// StoreTicket is an after-sales case opened by a buyer on a purchase. It carries
// a private conversation with the merchant, and can be escalated to an
// administrator as a complaint.
type StoreTicket struct {
	Id int `json:"id"`
	// OrderId is set for a code purchase; GroupId is set for a claimed group.
	OrderId      int    `json:"order_id" gorm:"index"`
	GroupId      int    `json:"group_id" gorm:"index"`
	ProductName  string `json:"product_name" gorm:"type:varchar(80)"`
	BuyerUserId  int    `json:"buyer_user_id" gorm:"index"`
	SellerUserId int    `json:"seller_user_id" gorm:"index"`
	Reason       string `json:"reason" gorm:"type:text"`
	// ComplaintReason is filled when the buyer escalates to an administrator.
	ComplaintReason string `json:"complaint_reason" gorm:"type:text"`
	Status          int    `json:"status" gorm:"default:0;index"`
	// RefundedQuota is what actually went back to the buyer's wallet.
	RefundedQuota int    `json:"refunded_quota"`
	RefundedBy    int    `json:"refunded_by"`
	ResolvedNote  string `json:"resolved_note" gorm:"type:text"`
	CreatedAt     int64  `json:"created_at" gorm:"bigint;index"`
	UpdatedAt     int64  `json:"updated_at" gorm:"bigint"`
}

func (StoreTicket) TableName() string {
	return "store_tickets"
}

// StoreTicketMessage is one line in the buyer/merchant conversation.
type StoreTicketMessage struct {
	Id         int    `json:"id"`
	TicketId   int    `json:"ticket_id" gorm:"index"`
	SenderId   int    `json:"sender_id" gorm:"index"`
	SenderRole int    `json:"sender_role"`
	Body       string `json:"body" gorm:"type:text"`
	CreatedAt  int64  `json:"created_at" gorm:"bigint;index"`
}

func (StoreTicketMessage) TableName() string {
	return "store_ticket_messages"
}

// CreateStoreTicket opens an after-sales case on one of the caller's purchases.
func CreateStoreTicket(buyerUserId int, orderId int, groupId int, reason string) (*StoreTicket, error) {
	reason = strings.TrimSpace(reason)
	if reason == "" {
		return nil, ErrStoreChannelBadInput
	}
	if len([]rune(reason)) > 500 {
		reason = string([]rune(reason)[:500])
	}
	ticket := &StoreTicket{
		OrderId: orderId,
		GroupId: groupId,
		Reason:  reason,
		Status:  StoreTicketStatusOpen,
	}
	switch {
	case orderId > 0:
		order := &StoreOrder{}
		if err := DB.Where("id = ?", orderId).First(order).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil, ErrStoreTicketNotFound
			}
			return nil, err
		}
		if order.BuyerUserId != buyerUserId {
			return nil, ErrStoreTicketForbidden
		}
		ticket.BuyerUserId = order.BuyerUserId
		ticket.SellerUserId = order.SellerUserId
		ticket.ProductName = order.ProductName
	case groupId > 0:
		group := &UserChannelGroup{}
		if err := DB.Where("id = ?", groupId).First(group).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil, ErrStoreTicketNotFound
			}
			return nil, err
		}
		claimed := int64(0)
		if err := DB.Model(&StoreGroupClaim{}).
			Where("group_id = ? AND user_id = ?", groupId, buyerUserId).Count(&claimed).Error; err != nil {
			return nil, err
		}
		if claimed == 0 {
			return nil, ErrStoreTicketForbidden
		}
		ticket.BuyerUserId = buyerUserId
		ticket.SellerUserId = group.UserId
		ticket.ProductName = UserChannelGroupPublicName(group.Name)
	default:
		return nil, ErrStoreChannelBadInput
	}
	now := common.GetTimestamp()
	ticket.CreatedAt = now
	ticket.UpdatedAt = now
	// One open ticket per purchase keeps the conversation in one place.
	existing := &StoreTicket{}
	query := DB.Where("buyer_user_id = ? AND status IN ?", buyerUserId,
		[]int{StoreTicketStatusOpen, StoreTicketStatusComplaint})
	if orderId > 0 {
		query = query.Where("order_id = ?", orderId)
	} else {
		query = query.Where("group_id = ?", groupId)
	}
	if err := query.First(existing).Error; err == nil {
		return existing, nil
	}
	if err := DB.Create(ticket).Error; err != nil {
		return nil, err
	}
	if err := AddStoreTicketMessage(ticket.Id, buyerUserId, StoreTicketRoleBuyer, reason); err != nil {
		return nil, err
	}
	return ticket, nil
}

// AddStoreTicketMessage appends one line to the conversation.
func AddStoreTicketMessage(ticketId int, senderId int, role int, body string) error {
	body = strings.TrimSpace(body)
	if body == "" {
		return ErrStoreChannelBadInput
	}
	if len([]rune(body)) > 2000 {
		body = string([]rune(body)[:2000])
	}
	return DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&StoreTicketMessage{
			TicketId:   ticketId,
			SenderId:   senderId,
			SenderRole: role,
			Body:       body,
			CreatedAt:  common.GetTimestamp(),
		}).Error; err != nil {
			return err
		}
		return tx.Model(&StoreTicket{}).Where("id = ?", ticketId).
			Update("updated_at", common.GetTimestamp()).Error
	})
}

// GetStoreTicket loads one ticket.
func GetStoreTicket(ticketId int) (*StoreTicket, error) {
	ticket := &StoreTicket{}
	if err := DB.Where("id = ?", ticketId).First(ticket).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrStoreTicketNotFound
		}
		return nil, err
	}
	return ticket, nil
}

// ListStoreTicketMessages returns the conversation, oldest first.
func ListStoreTicketMessages(ticketId int) ([]*StoreTicketMessage, error) {
	rows := make([]*StoreTicketMessage, 0, 16)
	if err := DB.Where("ticket_id = ?", ticketId).Order("id").Find(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

// ListBuyerStoreTickets lists the tickets one buyer opened.
func ListBuyerStoreTickets(buyerUserId int) ([]*StoreTicket, error) {
	rows := make([]*StoreTicket, 0, 16)
	if err := DB.Where("buyer_user_id = ?", buyerUserId).Order("id desc").Find(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

// ListSellerStoreTickets lists the tickets raised against one seller.
func ListSellerStoreTickets(sellerUserId int) ([]*StoreTicket, error) {
	rows := make([]*StoreTicket, 0, 16)
	if err := DB.Where("seller_user_id = ?", sellerUserId).Order("id desc").Find(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

// ListComplaintStoreTickets is the administrator queue: escalated tickets only.
func ListComplaintStoreTickets() ([]*StoreTicket, error) {
	rows := make([]*StoreTicket, 0, 32)
	if err := DB.Where("status = ?", StoreTicketStatusComplaint).Order("id desc").Find(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

// RefundStoreTicket returns the buyer's money and claws the seller's income back.
// The platform takes the fee loss, so the buyer is made whole.
func RefundStoreTicket(ticketId int, operatorId int, asAdmin bool) error {
	return DB.Transaction(func(tx *gorm.DB) error {
		ticket := &StoreTicket{}
		if err := lockForUpdate(tx).Where("id = ?", ticketId).First(ticket).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrStoreTicketNotFound
			}
			return err
		}
		if !asAdmin && ticket.SellerUserId != operatorId {
			return ErrStoreTicketForbidden
		}
		if ticket.Status == StoreTicketStatusRefunded || ticket.Status == StoreTicketStatusResolved {
			return ErrStoreTicketRefunded
		}
		if ticket.OrderId == 0 {
			// A free group claim has nothing to refund: closing it cancels the claim.
			if err := tx.Where("group_id = ? AND user_id = ?", ticket.GroupId, ticket.BuyerUserId).
				Delete(&StoreGroupClaim{}).Error; err != nil {
				return err
			}
			if err := tx.Model(&StoreTicket{}).Where("id = ?", ticketId).Updates(map[string]any{
				"status":      StoreTicketStatusResolved,
				"refunded_by": operatorId,
				"updated_at":  common.GetTimestamp(),
			}).Error; err != nil {
				return err
			}
			return AddStoreTicketMessage(ticketId, operatorId, roleFor(asAdmin), "已取消领取该分组")
		}

		order := &StoreOrder{}
		if err := lockForUpdate(tx).Where("id = ?", ticket.OrderId).First(order).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrStoreTicketNotFound
			}
			return err
		}
		if order.Status == 2 {
			return ErrStoreTicketRefunded
		}
		refund := order.Price
		if refund <= 0 {
			return ErrStoreChannelBadInput
		}
		// Buyer gets the full price back, including the platform fee.
		creditBuyer := tx.Model(&User{}).Where("id = ? AND quota <= ?", order.BuyerUserId, common.MaxWalletQuota-refund).
			Update("quota", gorm.Expr("quota + ?", refund))
		if creditBuyer.Error != nil {
			return creditBuyer.Error
		}
		if creditBuyer.RowsAffected == 0 {
			return ErrStoreSellerQuotaOverflow
		}
		// The seller only ever received price - fee, so that is what is clawed back.
		if order.SellerIncome > 0 {
			debitSeller := tx.Model(&User{}).Where("id = ? AND quota >= ?", order.SellerUserId, order.SellerIncome).
				Update("quota", gorm.Expr("quota - ?", order.SellerIncome))
			if debitSeller.Error != nil {
				return debitSeller.Error
			}
			if debitSeller.RowsAffected == 0 {
				common.SysLog(fmt.Sprintf("store refund: seller %d cannot cover %d for order %d", order.SellerUserId, order.SellerIncome, order.Id))
			}
		}
		if err := tx.Model(&StoreOrder{}).Where("id = ?", order.Id).
			Update("status", 2).Error; err != nil {
			return err
		}
		if err := tx.Model(&StoreTicket{}).Where("id = ?", ticketId).Updates(map[string]any{
			"status":         StoreTicketStatusRefunded,
			"refunded_quota": refund,
			"refunded_by":    operatorId,
			"updated_at":     common.GetTimestamp(),
		}).Error; err != nil {
			return err
		}
		return AddStoreTicketMessage(ticketId, operatorId, roleFor(asAdmin),
			fmt.Sprintf("已退款 %.6f 美元（含手续费，额度 %d）", float64(refund)/float64(common.QuotaPerUnit), refund))
	})
}

func roleFor(asAdmin bool) int {
	if asAdmin {
		return StoreTicketRoleAdmin
	}
	return StoreTicketRoleSeller
}

// ComplainStoreTicket escalates a ticket to an administrator.
func ComplainStoreTicket(ticketId int, buyerUserId int, reason string) error {
	reason = strings.TrimSpace(reason)
	if reason == "" {
		return ErrStoreChannelBadInput
	}
	ticket, err := GetStoreTicket(ticketId)
	if err != nil {
		return err
	}
	if ticket.BuyerUserId != buyerUserId {
		return ErrStoreTicketForbidden
	}
	if ticket.Status == StoreTicketStatusResolved || ticket.Status == StoreTicketStatusClosed {
		return ErrStoreTicketClosed
	}
	if err := DB.Model(&StoreTicket{}).Where("id = ?", ticketId).Updates(map[string]any{
		"status":           StoreTicketStatusComplaint,
		"complaint_reason": reason,
		"updated_at":       common.GetTimestamp(),
	}).Error; err != nil {
		return err
	}
	return AddStoreTicketMessage(ticketId, buyerUserId, StoreTicketRoleBuyer, "投诉："+reason)
}

// ResolveStoreTicket closes a ticket after an administrator reviewed it.
func ResolveStoreTicket(ticketId int, adminId int, note string) error {
	if err := DB.Model(&StoreTicket{}).Where("id = ?", ticketId).Updates(map[string]any{
		"status":        StoreTicketStatusResolved,
		"resolved_note": strings.TrimSpace(note),
		"updated_at":    common.GetTimestamp(),
	}).Error; err != nil {
		return err
	}
	return AddStoreTicketMessage(ticketId, adminId, StoreTicketRoleAdmin,
		strings.TrimSpace("管理员处理："+note))
}
