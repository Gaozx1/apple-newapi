package controller

import (
	"net/http"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
)

// GetLotteryStatus returns the current user's lottery state: whether the
// feature is enabled, how many free chances they hold, how many unused coupons
// they have, and their recent draw history.
func GetLotteryStatus(c *gin.Context) {
	userId := c.GetInt("id")

	chances, couponCount, records, err := model.GetUserLotteryStatus(userId, 20)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"enabled":      common.LotteryEnabled,
			"cost_quota":   model.LotteryCostQuota,
			"chances":      chances,
			"coupon_count": couponCount,
			"records":      records,
		},
	})
}

// DrawLottery handles a single lottery draw for the current user.
func DrawLottery(c *gin.Context) {
	if !common.LotteryEnabled {
		common.ApiErrorMsg(c, "抽奖功能未开启")
		return
	}
	userId := c.GetInt("id")

	result, err := model.DrawLottery(userId)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "抽奖完成",
		"data": gin.H{
			"prize_type":   result.PrizeType,
			"prize_label":  result.PrizeLabel,
			"prize_value":  result.PrizeValue,
			"rebate_rate":  result.RebateRate,
			"cost_quota":   result.CostQuota,
			"free_draw":    result.FreeDraw,
		},
	})
}

// AdminGrantLotteryChances grants lottery draw chances to a user. Admins use
// this to distribute free draws (each free draw avoids the $5 quota cost).
type adminGrantLotteryChancesRequest struct {
	UserId int `json:"user_id"`
	Count  int `json:"count"`
}

func AdminGrantLotteryChances(c *gin.Context) {
	var req adminGrantLotteryChancesRequest
	if err := common.DecodeJson(c.Request.Body, &req); err != nil {
		common.ApiErrorMsg(c, "请求参数无效")
		return
	}
	if req.UserId == 0 {
		common.ApiErrorMsg(c, "用户ID不能为空")
		return
	}
	if req.Count <= 0 {
		common.ApiErrorMsg(c, "抽奖次数必须为正数")
		return
	}

	if err := model.AdminGrantLotteryChances(req.UserId, req.Count); err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "已发放抽奖次数",
	})
}
