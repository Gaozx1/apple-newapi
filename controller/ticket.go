package controller

import (
	"net/http"
	"strconv"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
)

// ============================================================================
// Ticket (工单) controllers
//
// Self routes require UserAuth; admin routes require AdminAuth. Users may only
// read and reply to their own tickets; admins may read, reply to, and close
// any ticket.
// ============================================================================

// createTicketRequest is the body for opening a new ticket.
type createTicketRequest struct {
	Subject string `json:"subject"`
	Content string `json:"content"`
}

// CreateTicket opens a new support ticket for the current user.
func CreateTicket(c *gin.Context) {
	userId := c.GetInt("id")
	if userId == 0 {
		common.ApiErrorMsg(c, "user id is empty")
		return
	}
	var req createTicketRequest
	if err := common.DecodeJson(c.Request.Body, &req); err != nil {
		common.ApiErrorMsg(c, "请求参数无效")
		return
	}

	username := c.GetString("username")
	ticket, err := model.CreateTicket(userId, username, req.Subject, req.Content)
	if err != nil {
		apiTicketError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "工单已提交",
		"data":    ticket,
	})
}

// GetMyTickets returns the current user's tickets with pagination.
func GetMyTickets(c *gin.Context) {
	userId := c.GetInt("id")
	page, pageSize := parseTicketPage(c)
	tickets, total, err := model.GetUserTickets(userId, page, pageSize)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"items": tickets,
			"total": total,
			"page":  page,
			"size":  pageSize,
		},
	})
}

// GetTicket returns a single ticket with its thread. Users may only fetch
// their own tickets; admins may fetch any.
func GetTicket(c *gin.Context) {
	ticketId, ok := parseTicketIdParam(c)
	if !ok {
		return
	}
	ticket, replies, err := model.GetTicket(ticketId)
	if err != nil {
		apiTicketError(c, err)
		return
	}
	userId := c.GetInt("id")
	isAdmin := c.GetInt("role") >= common.RoleAdminUser
	if !isAdmin && ticket.UserId != userId {
		common.ApiErrorMsg(c, "无权访问该工单")
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"ticket":  ticket,
			"replies": replies,
		},
	})
}

// replyTicketRequest is the body for replying to a ticket.
type replyTicketRequest struct {
	Content string `json:"content"`
}

// ReplyTicket appends a reply to a ticket. Users may reply only to their own
// open tickets; admins may reply to any open ticket (and their reply is marked
// as admin-authored).
func ReplyTicket(c *gin.Context) {
	ticketId, ok := parseTicketIdParam(c)
	if !ok {
		return
	}
	var req replyTicketRequest
	if err := common.DecodeJson(c.Request.Body, &req); err != nil {
		common.ApiErrorMsg(c, "请求参数无效")
		return
	}

	userId := c.GetInt("id")
	isAdmin := c.GetInt("role") >= common.RoleAdminUser

	// Non-admins may only reply to their own ticket.
	if !isAdmin {
		ticket, _, err := model.GetTicket(ticketId)
		if err != nil {
			apiTicketError(c, err)
			return
		}
		if ticket.UserId != userId {
			common.ApiErrorMsg(c, "无权访问该工单")
			return
		}
	}

	reply, err := model.AddTicketReply(ticketId, userId, isAdmin, req.Content)
	if err != nil {
		apiTicketError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "回复已提交",
		"data":    reply,
	})
}

// GetAllTickets returns every ticket with optional status filter and
// pagination. Admin only.
func GetAllTickets(c *gin.Context) {
	status := TicketStatus(c.Query("status"))
	page, pageSize := parseTicketPage(c)
	tickets, total, err := model.GetAllTickets(status, page, pageSize)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"items": tickets,
			"total": total,
			"page":  page,
			"size":  pageSize,
		},
	})
}

// CloseTicket closes a ticket. Admin only.
func CloseTicket(c *gin.Context) {
	ticketId, ok := parseTicketIdParam(c)
	if !ok {
		return
	}
	if err := model.CloseTicket(ticketId); err != nil {
		apiTicketError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "工单已关闭",
	})
}

// ----------------------------------------------------------------------------
// Helpers
// ----------------------------------------------------------------------------

// TicketStatus is a thin alias so the controller can reference the model type
// without importing it in signatures where only a filter string is needed.
type TicketStatus = model.TicketStatus

func parseTicketIdParam(c *gin.Context) (int64, bool) {
	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil || id <= 0 {
		common.ApiErrorMsg(c, "无效的工单ID")
		return 0, false
	}
	return id, true
}

func parseTicketPage(c *gin.Context) (int, int) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	return page, pageSize
}

func apiTicketError(c *gin.Context, err error) {
	switch err {
	case model.ErrTicketNotFound:
		common.ApiErrorMsg(c, "工单不存在")
	case model.ErrTicketClosed:
		common.ApiErrorMsg(c, "工单已关闭，无法回复")
	case model.ErrTicketEmptyField:
		common.ApiErrorMsg(c, "标题和内容不能为空")
	default:
		common.ApiError(c, err)
	}
}
