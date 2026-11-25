package controller

import (
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
)

// ---- User APIs ----

// GetResetCardSelf lists the reset cards owned by the current user.
func GetResetCardSelf(c *gin.Context) {
	userId := c.GetInt("id")
	cards, err := model.UserResetCards(userId)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, cards)
}

type UseResetCardRequest struct {
	CardKey            string `json:"card_key"`
	UserSubscriptionId int    `json:"user_subscription_id"`
}

// UseResetCardSelf consumes one of the user's reset cards and resets the used
// quota of the selected active subscription.
func UseResetCardSelf(c *gin.Context) {
	userId := c.GetInt("id")
	var req UseResetCardRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiErrorT(c, "Invalid parameters")
		return
	}
	req.CardKey = strings.TrimSpace(req.CardKey)
	if req.CardKey == "" || req.UserSubscriptionId <= 0 {
		common.ApiErrorT(c, "Invalid parameters")
		return
	}
	summary, err := model.UseResetCard(userId, req.CardKey, req.UserSubscriptionId)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	model.RecordLog(userId, model.LogTypeSystem, common.NewMessage(
		"Used a reset card to reset subscription quota (subscription ID: {{id}})",
		map[string]any{"id": req.UserSubscriptionId},
	))
	common.ApiSuccess(c, summary)
}

// ---- Admin APIs ----

// AdminListResetCards lists reset cards with optional filters.
func AdminListResetCards(c *gin.Context) {
	pageInfo := common.GetPageQuery(c)
	keyword := c.Query("keyword")
	status := c.Query("status")
	userId, _ := strconv.Atoi(c.Query("user_id"))
	cards, total, err := model.SearchResetCards(keyword, status, userId, pageInfo.GetStartIdx(), pageInfo.GetPageSize())
	if err != nil {
		common.ApiError(c, err)
		return
	}
	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(cards)
	common.ApiSuccess(c, pageInfo)
}

type AdminGrantResetCardsRequest struct {
	UserId               int    `json:"user_id"`
	Count                int    `json:"count"`
	ExpiresAt            int64  `json:"expires_at"`
	Note                 string `json:"note"`
	TargetSubscriptionId int    `json:"target_subscription_id"`
	// AllUsers grants `count` cards to every active common user; UserId must be 0
	// in that case.
	AllUsers bool `json:"all_users"`
}

// AdminGrantResetCards grants reset cards to a user (or, with AllUsers, to every
// active user) with an explicit expiry. TargetSubscriptionId optionally binds
// the cards to one of that user's subscriptions; 0 means the cards can reset any
// active subscription. Binding is not allowed when granting to all users.
func AdminGrantResetCards(c *gin.Context) {
	var req AdminGrantResetCardsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiErrorT(c, "Invalid parameters")
		return
	}
	if req.Count <= 0 || req.Count > 100 || req.TargetSubscriptionId < 0 {
		common.ApiErrorT(c, "Invalid parameters")
		return
	}
	if req.AllUsers {
		if req.UserId != 0 {
			common.ApiErrorT(c, "Invalid parameters")
			return
		}
		users, cards, err := model.GrantResetCardsToAllUsers(req.Count, req.ExpiresAt, req.Note, req.TargetSubscriptionId)
		if err != nil {
			common.ApiError(c, err)
			return
		}
		recordManageAudit(c, "reset_card.grant_all", map[string]any{
			"count":                  req.Count,
			"expires_at":             model.FormatResetCardExpiry(req.ExpiresAt),
			"users":                  users,
			"cards":                  cards,
			"target_subscription_id": req.TargetSubscriptionId,
		})
		common.ApiSuccess(c, gin.H{
			"all_users":     true,
			"granted_users": users,
			"total_cards":   cards,
		})
		return
	}
	if req.UserId <= 0 {
		common.ApiErrorT(c, "Invalid parameters")
		return
	}
	keys, err := model.GrantResetCards(req.UserId, req.Count, req.ExpiresAt, req.Note, req.TargetSubscriptionId)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	recordManageAuditFor(c, req.UserId, "reset_card.grant", map[string]any{
		"target_user_id":         req.UserId,
		"count":                  req.Count,
		"expires_at":             model.FormatResetCardExpiry(req.ExpiresAt),
		"target_subscription_id": req.TargetSubscriptionId,
	})
	common.ApiSuccess(c, gin.H{"card_keys": keys})
}

// AdminDisableResetCard disables an unused reset card.
func AdminDisableResetCard(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	if id <= 0 {
		common.ApiErrorT(c, "Invalid ID")
		return
	}
	if err := model.AdminDisableResetCard(id); err != nil {
		common.ApiError(c, err)
		return
	}
	recordManageAudit(c, "reset_card.disable", map[string]any{"id": id})
	common.ApiSuccess(c, nil)
}

// AdminDeleteResetCard deletes a reset card that has not been used.
func AdminDeleteResetCard(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	if id <= 0 {
		common.ApiErrorT(c, "Invalid ID")
		return
	}
	if err := model.AdminDeleteResetCard(id); err != nil {
		common.ApiError(c, err)
		return
	}
	recordManageAudit(c, "reset_card.delete", map[string]any{"id": id})
	common.ApiSuccess(c, nil)
}

// AdminListUserResetCards lists the reset cards owned by a user.
func AdminListUserResetCards(c *gin.Context) {
	userId, _ := strconv.Atoi(c.Param("id"))
	if userId <= 0 {
		common.ApiErrorT(c, "Invalid user ID")
		return
	}
	cards, err := model.UserResetCards(userId)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, cards)
}
