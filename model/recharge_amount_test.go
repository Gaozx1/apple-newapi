package model

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// getUserRechargeAmount reads the lifetime paid amount user levels resolve from.
func getUserRechargeAmount(t *testing.T, userId int) float64 {
	t.Helper()
	var user User
	require.NoError(t, DB.Select("recharge_amount").Where("id = ?", userId).First(&user).Error)
	return user.RechargeAmount
}

func TestRechargeEpayAccumulatesPaidAmountOnce(t *testing.T) {
	truncateTables(t)
	DB.Exec("DELETE FROM users")
	DB.Exec("DELETE FROM top_ups")

	require.NoError(t, DB.Create(&User{
		Id:       301,
		Username: "recharge-amount-user",
		Status:   common.UserStatusEnabled,
		Quota:    0,
	}).Error)
	require.NoError(t, (&TopUp{
		UserId:          301,
		Amount:          2,
		Money:           9.99,
		TradeNo:         "recharge-amount-epay",
		PaymentMethod:   "alipay",
		PaymentProvider: PaymentProviderEpay,
		CreateTime:      time.Now().Unix(),
		Status:          common.TopUpStatusPending,
	}).Insert())

	alreadyDone, err := RechargeEpay("recharge-amount-epay", "alipay", "127.0.0.1")
	require.NoError(t, err)
	assert.False(t, alreadyDone)
	assert.InDelta(t, 9.99, getUserRechargeAmount(t, 301), 1e-9)

	// The repeated gateway callback is idempotent and must not count the same
	// payment twice.
	alreadyDone, err = RechargeEpay("recharge-amount-epay", "alipay", "127.0.0.1")
	require.NoError(t, err)
	assert.True(t, alreadyDone)
	assert.InDelta(t, 9.99, getUserRechargeAmount(t, 301), 1e-9)
}

// Completing a subscription order reads the database clock inside its
// transaction, which needs a second connection: the package's in-memory test
// database serves exactly one, so this case runs on a file-backed database.
func TestCompleteSubscriptionOrderAccumulatesPlanPrice(t *testing.T) {
	previousDB, previousLogDB := DB, LOG_DB
	fileDB, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "recharge-amount.db")), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, fileDB.AutoMigrate(
		&User{}, &Log{}, &TopUp{}, &LotteryCoupon{},
		&SubscriptionPlan{}, &SubscriptionOrder{}, &UserSubscription{},
	))
	DB, LOG_DB = fileDB, fileDB
	t.Cleanup(func() {
		DB, LOG_DB = previousDB, previousLogDB
		InvalidateSubscriptionPlanCache(302)
		if sqlDB, dbErr := fileDB.DB(); dbErr == nil {
			require.NoError(t, sqlDB.Close())
		}
	})

	require.NoError(t, DB.Create(&User{
		Id:       302,
		Username: "recharge-amount-subscriber",
		Status:   common.UserStatusEnabled,
		Quota:    0,
	}).Error)
	require.NoError(t, DB.Create(&SubscriptionPlan{
		Id:            302,
		Title:         "Level Plan",
		PriceAmount:   29.9,
		Currency:      "USD",
		DurationUnit:  SubscriptionDurationMonth,
		DurationValue: 1,
		Enabled:       true,
		TotalAmount:   1000,
	}).Error)
	// Warm the plan cache the way checkout does before the order exists.
	_, err = GetSubscriptionPlanById(302)
	require.NoError(t, err)
	require.NoError(t, (&SubscriptionOrder{
		UserId:          302,
		PlanId:          302,
		Money:           29.9,
		TradeNo:         "recharge-amount-sub",
		PaymentMethod:   PaymentProviderEpay,
		PaymentProvider: PaymentProviderEpay,
		Status:          common.TopUpStatusPending,
		CreateTime:      time.Now().Unix(),
	}).Insert())

	require.NoError(t, CompleteSubscriptionOrder("recharge-amount-sub", "", PaymentProviderEpay, ""))
	assert.InDelta(t, 29.9, getUserRechargeAmount(t, 302), 1e-9)

	// Completing the same order again is idempotent.
	require.NoError(t, CompleteSubscriptionOrder("recharge-amount-sub", "", PaymentProviderEpay, ""))
	assert.InDelta(t, 29.9, getUserRechargeAmount(t, 302), 1e-9)
}

func TestRedeemAccumulatesQuotaFaceValue(t *testing.T) {
	previousQuotaPerUnit := common.QuotaPerUnit
	common.QuotaPerUnit = 500000
	t.Cleanup(func() { common.QuotaPerUnit = previousQuotaPerUnit })

	userId, key := setupRedeemFixture(t, 500000)

	_, err := Redeem(key, userId)
	require.NoError(t, err)

	// 500000 quota is one money unit at QuotaPerUnit=500000.
	assert.InDelta(t, 1.0, getUserRechargeAmount(t, userId), 1e-9)
}
