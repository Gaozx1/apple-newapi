package controller

import (
	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"

	"github.com/gin-gonic/gin"
)

type storeTicketView struct {
	Id              int     `json:"id"`
	OrderId         int     `json:"order_id"`
	GroupId         int     `json:"group_id"`
	ProductName     string  `json:"product_name"`
	BuyerUserId     int     `json:"buyer_user_id"`
	BuyerName       string  `json:"buyer_name"`
	SellerUserId    int     `json:"seller_user_id"`
	SellerName      string  `json:"seller_name"`
	Reason          string  `json:"reason"`
	ComplaintReason string  `json:"complaint_reason"`
	Status          int     `json:"status"`
	RefundedQuota   int     `json:"refunded_quota"`
	RefundedAmount  float64 `json:"refunded_amount"`
	ResolvedNote    string  `json:"resolved_note"`
	CreatedAt       int64   `json:"created_at"`
	UpdatedAt       int64   `json:"updated_at"`
}

type storeTicketMessageView struct {
	Id         int    `json:"id"`
	SenderId   int    `json:"sender_id"`
	SenderName string `json:"sender_name"`
	SenderRole int    `json:"sender_role"`
	Body       string `json:"body"`
	CreatedAt  int64  `json:"created_at"`
}

type storeTicketRequest struct {
	OrderId int    `json:"order_id"`
	GroupId int    `json:"group_id"`
	Reason  string `json:"reason"`
}

type storeTicketMessageRequest struct {
	Body string `json:"body"`
}

type storeTicketComplaintRequest struct {
	Reason string `json:"reason"`
}

type storeTicketResolveRequest struct {
	Note string `json:"note"`
}

func toStoreTicketView(ticket *model.StoreTicket) storeTicketView {
	buyerName, _ := model.GetUsernameById(ticket.BuyerUserId, false)
	sellerName, _ := model.GetUsernameById(ticket.SellerUserId, false)
	return storeTicketView{
		Id:              ticket.Id,
		OrderId:         ticket.OrderId,
		GroupId:         ticket.GroupId,
		ProductName:     ticket.ProductName,
		BuyerUserId:     ticket.BuyerUserId,
		BuyerName:       buyerName,
		SellerUserId:    ticket.SellerUserId,
		SellerName:      sellerName,
		Reason:          ticket.Reason,
		ComplaintReason: ticket.ComplaintReason,
		Status:          ticket.Status,
		RefundedQuota:   ticket.RefundedQuota,
		RefundedAmount:  storeAmountFromQuota(ticket.RefundedQuota),
		ResolvedNote:    ticket.ResolvedNote,
		CreatedAt:       ticket.CreatedAt,
		UpdatedAt:       ticket.UpdatedAt,
	}
}

func toStoreTicketViews(tickets []*model.StoreTicket) []storeTicketView {
	views := make([]storeTicketView, 0, len(tickets))
	for _, ticket := range tickets {
		views = append(views, toStoreTicketView(ticket))
	}
	return views
}

// CreateStoreTicket opens an after-sales case on one of the caller's purchases.
func CreateStoreTicket(c *gin.Context) {
	request := storeTicketRequest{}
	if err := c.ShouldBindJSON(&request); err != nil {
		common.ApiErrorMsg(c, "参数格式错误")
		return
	}
	ticket, err := model.CreateStoreTicket(c.GetInt("id"), request.OrderId, request.GroupId, request.Reason)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, toStoreTicketView(ticket))
}

// ListMyStoreTickets lists the tickets the caller opened as a buyer.
func ListMyStoreTickets(c *gin.Context) {
	tickets, err := model.ListBuyerStoreTickets(c.GetInt("id"))
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, gin.H{"items": toStoreTicketViews(tickets)})
}

// ListStoreSalesTickets lists the tickets raised against the caller as a seller.
func ListStoreSalesTickets(c *gin.Context) {
	tickets, err := model.ListSellerStoreTickets(c.GetInt("id"))
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, gin.H{"items": toStoreTicketViews(tickets)})
}

// GetStoreTicket returns one ticket with its conversation. Buyers, the seller
// and administrators may read it.
func GetStoreTicket(c *gin.Context) {
	ticketId, ok := storeRouteId(c)
	if !ok {
		return
	}
	ticket, err := model.GetStoreTicket(ticketId)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	userId := c.GetInt("id")
	isAdmin := c.GetInt("role") >= common.RoleAdminUser
	if !isAdmin && ticket.BuyerUserId != userId && ticket.SellerUserId != userId {
		common.ApiError(c, model.ErrStoreTicketForbidden)
		return
	}
	messages, err := model.ListStoreTicketMessages(ticketId)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	views := make([]storeTicketMessageView, 0, len(messages))
	for _, message := range messages {
		name, _ := model.GetUsernameById(message.SenderId, false)
		views = append(views, storeTicketMessageView{
			Id:         message.Id,
			SenderId:   message.SenderId,
			SenderName: name,
			SenderRole: message.SenderRole,
			Body:       message.Body,
			CreatedAt:  message.CreatedAt,
		})
	}
	common.ApiSuccess(c, gin.H{
		"ticket":   toStoreTicketView(ticket),
		"messages": views,
	})
}

// AddStoreTicketMessage appends one line to the after-sales conversation.
func AddStoreTicketMessage(c *gin.Context) {
	ticketId, ok := storeRouteId(c)
	if !ok {
		return
	}
	ticket, err := model.GetStoreTicket(ticketId)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	userId := c.GetInt("id")
	isAdmin := c.GetInt("role") >= common.RoleAdminUser
	role := model.StoreTicketRoleBuyer
	switch {
	case isAdmin:
		role = model.StoreTicketRoleAdmin
	case ticket.SellerUserId == userId:
		role = model.StoreTicketRoleSeller
	case ticket.BuyerUserId != userId:
		common.ApiError(c, model.ErrStoreTicketForbidden)
		return
	}
	request := storeTicketMessageRequest{}
	if err := c.ShouldBindJSON(&request); err != nil {
		common.ApiErrorMsg(c, "参数格式错误")
		return
	}
	if err := model.AddStoreTicketMessage(ticketId, userId, role, request.Body); err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, nil)
}

// RefundStoreTicket refunds a purchase; the merchant or an administrator may do it.
func RefundStoreTicket(c *gin.Context) {
	ticketId, ok := storeRouteId(c)
	if !ok {
		return
	}
	isAdmin := c.GetInt("role") >= common.RoleAdminUser
	if err := model.RefundStoreTicket(ticketId, c.GetInt("id"), isAdmin); err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, nil)
}

// ComplainStoreTicket escalates a ticket to an administrator.
func ComplainStoreTicket(c *gin.Context) {
	ticketId, ok := storeRouteId(c)
	if !ok {
		return
	}
	request := storeTicketComplaintRequest{}
	if err := c.ShouldBindJSON(&request); err != nil {
		common.ApiErrorMsg(c, "参数格式错误")
		return
	}
	if err := model.ComplainStoreTicket(ticketId, c.GetInt("id"), request.Reason); err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, nil)
}

// AdminListStoreComplaints lists escalated after-sales tickets.
func AdminListStoreComplaints(c *gin.Context) {
	tickets, err := model.ListComplaintStoreTickets()
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, gin.H{"items": toStoreTicketViews(tickets)})
}

// AdminResolveStoreTicket closes a ticket after review.
func AdminResolveStoreTicket(c *gin.Context) {
	ticketId, ok := storeRouteId(c)
	if !ok {
		return
	}
	request := storeTicketResolveRequest{}
	if err := c.ShouldBindJSON(&request); err != nil {
		common.ApiErrorMsg(c, "参数格式错误")
		return
	}
	if err := model.ResolveStoreTicket(ticketId, c.GetInt("id"), request.Note); err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, nil)
}
