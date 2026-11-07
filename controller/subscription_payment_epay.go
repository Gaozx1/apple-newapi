package controller

import (
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/Calcium-Ion/go-epay/epay"
	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/gin-gonic/gin"
)

type SubscriptionEpayPayRequest struct {
	PlanId        int    `json:"plan_id"`
	PaymentMethod string `json:"payment_method"`
}

func SubscriptionRequestEpay(c *gin.Context) {
	if !requirePaymentCompliance(c) {
		return
	}

	var req SubscriptionEpayPayRequest
	if err := c.ShouldBindJSON(&req); err != nil || req.PlanId <= 0 {
		common.ApiErrorT(c, "Invalid parameters")
		return
	}

	plan, err := model.GetSubscriptionPlanById(req.PlanId)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if !plan.Enabled {
		common.ApiErrorT(c, "Subscription plan is not enabled")
		return
	}
	if plan.PriceAmount < 0.01 {
		common.ApiErrorT(c, "Plan amount is too low")
		return
	}
	method := operation_setting.FindPaymentMethod(req.PaymentMethod)
	if method == nil {
		common.ApiErrorT(c, "Payment method does not exist")
		return
	}

	userId := c.GetInt("id")
	if plan.MaxPurchasePerUser > 0 {
		count, err := model.CountUserSubscriptionsByPlan(userId, plan.Id)
		if err != nil {
			common.ApiError(c, err)
			return
		}
		if count >= int64(plan.MaxPurchasePerUser) {
			common.ApiErrorT(c, "Purchase limit for this plan has been reached")
			return
		}
	}

	callBackAddress := service.GetCallbackAddress()
	returnUrl, err := url.Parse(callBackAddress + "/api/subscription/epay/return")
	if err != nil {
		common.ApiErrorT(c, "Callback URL configuration error")
		return
	}
	notifyUrl, err := url.Parse(callBackAddress + "/api/subscription/epay/notify")
	if err != nil {
		common.ApiErrorT(c, "Callback URL configuration error")
		return
	}

	tradeNo := fmt.Sprintf("%s%d", common.GetRandomString(6), time.Now().Unix())
	tradeNo = fmt.Sprintf("SUBUSR%dNO%s", userId, tradeNo)

	snapshot, snapshotValue, err := operation_setting.NewEpayOrderSnapshot(method)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	client := epayChannelClient(snapshot.Channel)
	if client == nil {
		common.ApiErrorT(c, "Payment information has not been configured by administrator")
		return
	}

	order := &model.SubscriptionOrder{
		UserId:          userId,
		PlanId:          plan.Id,
		Money:           plan.PriceAmount,
		TradeNo:         tradeNo,
		PaymentMethod:   req.PaymentMethod,
		PaymentProvider: model.PaymentProviderEpay,
		PaymentSnapshot: snapshotValue,
		CreateTime:      time.Now().Unix(),
		Status:          common.TopUpStatusPending,
	}
	if err := order.Insert(); err != nil {
		common.ApiErrorT(c, "Failed to create order")
		return
	}
	uri, params, err := client.Purchase(&epay.PurchaseArgs{
		Type:           snapshot.GatewayType,
		ServiceTradeNo: tradeNo,
		Name:           fmt.Sprintf("SUB:%s", plan.Title),
		Money:          strconv.FormatFloat(plan.PriceAmount, 'f', 2, 64),
		Device:         epay.PC,
		NotifyUrl:      notifyUrl,
		ReturnUrl:      returnUrl,
	})
	if err != nil {
		_ = model.ExpireSubscriptionOrder(tradeNo, model.PaymentProviderEpay)
		common.ApiErrorT(c, "Failed to start payment")
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "success", "data": params, "url": uri})
}

func SubscriptionEpayNotify(c *gin.Context) {
	verified, method, err := verifyEpayOrderCallback(c, true)
	if err != nil || verified.TradeStatus != epay.StatusTradeSuccess {
		c.String(http.StatusOK, "fail")
		return
	}
	LockOrder(verified.ServiceTradeNo)
	defer UnlockOrder(verified.ServiceTradeNo)
	if err := model.CompleteSubscriptionOrder(verified.ServiceTradeNo, "", model.PaymentProviderEpay, method); err != nil {
		c.String(http.StatusOK, "fail")
		return
	}
	c.String(http.StatusOK, "success")
}

func SubscriptionEpayReturn(c *gin.Context) {
	verified, method, err := verifyEpayOrderCallback(c, true)
	if err != nil {
		c.Redirect(http.StatusFound, paymentReturnPath("/wallet?pay=fail"))
		return
	}
	if verified.TradeStatus == epay.StatusTradeSuccess {
		LockOrder(verified.ServiceTradeNo)
		defer UnlockOrder(verified.ServiceTradeNo)
		if err := model.CompleteSubscriptionOrder(verified.ServiceTradeNo, "", model.PaymentProviderEpay, method); err != nil {
			c.Redirect(http.StatusFound, paymentReturnPath("/wallet?pay=fail"))
			return
		}
		c.Redirect(http.StatusFound, paymentReturnPath("/wallet?pay=success"))
		return
	}
	c.Redirect(http.StatusFound, paymentReturnPath("/wallet?pay=pending"))
}
