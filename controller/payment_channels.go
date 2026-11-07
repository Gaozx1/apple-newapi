package controller

import (
	"errors"
	"net/http"
	"strconv"
	"sync"

	"github.com/Calcium-Ion/go-epay/epay"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/gin-gonic/gin"
	"github.com/shopspring/decimal"
)

var paymentSettingsUpdateMutex sync.Mutex

func epayChannelClient(channel operation_setting.PaymentChannel) *epay.Client {
	if channel.Address == "" || channel.MerchantID == "" || channel.Key == "" {
		return nil
	}
	client, err := epay.NewClient(&epay.Config{PartnerID: channel.MerchantID, Key: channel.Key}, channel.Address)
	if err != nil {
		return nil
	}
	return client
}

// verifyEpayOrderCallback verifies the exact processor, merchant, method and
// fiat amount from the order. A processor cannot credit another channel's
// orders, and changing current settings cannot change pending-order routing.
func verifyEpayOrderCallback(c *gin.Context, subscription bool) (*epay.VerifyRes, string, error) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 64<<10)
	if err := c.Request.ParseForm(); err != nil {
		return nil, "", errors.New("invalid payment callback form")
	}
	params := make(map[string]string, len(c.Request.Form))
	for key, values := range c.Request.Form {
		if len(values) != 1 {
			return nil, "", errors.New("duplicate payment callback parameter")
		}
		params[key] = values[0]
	}
	tradeNo := params["out_trade_no"]
	if tradeNo == "" || len(tradeNo) > 255 || model.DB == nil {
		return nil, "", errors.New("payment order not found")
	}
	var snapshotValue, paymentMethod string
	var money float64
	if subscription {
		order := model.GetSubscriptionOrderByTradeNo(tradeNo)
		if order == nil || order.PaymentProvider != model.PaymentProviderEpay {
			return nil, "", errors.New("payment order not found")
		}
		snapshotValue, paymentMethod, money = order.PaymentSnapshot, order.PaymentMethod, order.Money
	} else {
		order := model.GetTopUpByTradeNo(tradeNo)
		if order == nil || order.PaymentProvider != model.PaymentProviderEpay {
			return nil, "", errors.New("payment order not found")
		}
		snapshotValue, paymentMethod, money = order.PaymentSnapshot, order.PaymentMethod, order.Money
	}
	snapshot, err := operation_setting.ReadEpayOrderSnapshot(snapshotValue, paymentMethod)
	if err != nil {
		return nil, "", errors.New("invalid payment order configuration")
	}
	client := epayChannelClient(snapshot.Channel)
	if client == nil {
		return nil, "", errors.New("payment processor unavailable")
	}
	verified, err := client.Verify(params)
	if err != nil || !verified.VerifyStatus {
		return nil, "", errors.New("invalid payment signature")
	}
	if params["pid"] != snapshot.Channel.MerchantID || verified.Type != snapshot.GatewayType {
		return nil, "", errors.New("payment merchant or method mismatch")
	}
	actual, err := decimal.NewFromString(verified.Money)
	expected, _ := decimal.NewFromString(strconv.FormatFloat(money, 'f', 2, 64))
	if err != nil || !actual.IsPositive() || !actual.Equal(expected) {
		return nil, "", errors.New("payment amount mismatch")
	}
	return verified, paymentMethod, nil
}
