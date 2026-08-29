package controller

import (
	"net/http"
	"strconv"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
)

// ----------------------------------------------------------------------------
// OAuth 2.0 call packages (次数包): user purchase + admin plan management
// ----------------------------------------------------------------------------

// UserOAuthCallPlans returns the purchasable call plans together with the
// user's remaining prepaid call balance and recent grants.
func UserOAuthCallPlans(c *gin.Context) {
	userId := c.GetInt("id")

	plans, err := model.GetEnabledOAuthCallPlans()
	if err != nil {
		common.ApiError(c, err)
		return
	}
	balance, err := model.GetOAuthCallBalance(userId)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	grants, err := model.ListUserOAuthCallGrants(userId, 20)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, gin.H{
		"plans":   plans,
		"balance": balance,
		"grants":  grants,
	})
}

type purchaseOAuthCallPlanRequest struct {
	PlanId int `json:"plan_id"`
}

// UserPurchaseOAuthCallPlan buys a call plan with the user's wallet balance.
func UserPurchaseOAuthCallPlan(c *gin.Context) {
	userId := c.GetInt("id")
	var req purchaseOAuthCallPlanRequest
	if err := common.DecodeJson(c.Request.Body, &req); err != nil || req.PlanId <= 0 {
		common.ApiErrorMsg(c, "参数错误")
		return
	}
	grant, _, err := model.PurchaseOAuthCallPlan(userId, req.PlanId)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}
	common.ApiSuccess(c, grant)
}

// ----------------------------------------------------------------------------
// Admin: call plan management
// ----------------------------------------------------------------------------

func AdminOAuthCallPlanList(c *gin.Context) {
	plans, err := model.GetAllOAuthCallPlans()
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, plans)
}

type AdminUpsertOAuthCallPlanRequest struct {
	Plan model.OAuthCallPlan `json:"plan"`
}

func AdminOAuthCallPlanCreate(c *gin.Context) {
	var req AdminUpsertOAuthCallPlanRequest
	if err := common.DecodeJson(c.Request.Body, &req); err != nil {
		common.ApiErrorMsg(c, "参数错误")
		return
	}
	req.Plan.Id = 0
	if err := model.AdminCreateOAuthCallPlan(&req.Plan); err != nil {
		common.ApiErrorMsg(c, err.Error())
		return
	}
	common.ApiSuccess(c, req.Plan)
}

func AdminOAuthCallPlanUpdate(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	if id <= 0 {
		common.ApiErrorMsg(c, "无效的ID")
		return
	}
	var req AdminUpsertOAuthCallPlanRequest
	if err := common.DecodeJson(c.Request.Body, &req); err != nil {
		common.ApiErrorMsg(c, "参数错误")
		return
	}
	req.Plan.Id = id
	if err := model.AdminUpdateOAuthCallPlan(&req.Plan); err != nil {
		common.ApiErrorMsg(c, err.Error())
		return
	}
	common.ApiSuccess(c, nil)
}

func AdminOAuthCallPlanDelete(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	if id <= 0 {
		common.ApiErrorMsg(c, "无效的ID")
		return
	}
	if err := model.AdminDeleteOAuthCallPlan(id); err != nil {
		common.ApiErrorMsg(c, err.Error())
		return
	}
	common.ApiSuccess(c, nil)
}
