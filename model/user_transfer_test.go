package model

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/QuantumNous/new-api/common"
)

func TestTransferQuotaToUserFeeModes(t *testing.T) {
	truncateTables(t)

	sender := &User{Id: 301, Username: "sender-t", AffCode: "st1", Quota: 50_000_000}
	receiver := &User{Id: 302, Username: "receiver-t", AffCode: "rt1", Quota: 0}
	require.NoError(t, DB.Create(sender).Error)
	require.NoError(t, DB.Create(receiver).Error)

	quota := int(common.QuotaPerUnit) * 10 // $10

	// deduct 模式：手续费从转账金额内扣，付款方出 quota，收款方得 quota-fee
	fee, err := TransferQuotaToUser(sender.Id, receiver.Username, quota, TransferFeeModeDeduct)
	require.NoError(t, err)
	// 0.5% of 5,000,000 = 25,000
	assert.Equal(t, 25_000, fee)

	var s, r User
	require.NoError(t, DB.First(&s, sender.Id).Error)
	require.NoError(t, DB.First(&r, receiver.Id).Error)
	assert.Equal(t, 50_000_000-quota, s.Quota)
	assert.Equal(t, quota-25_000, r.Quota)

	// extra 模式：收款方得全额，付款方额外付手续费
	fee, err = TransferQuotaToUser(sender.Id, receiver.Username, quota, TransferFeeModeExtra)
	require.NoError(t, err)
	assert.Equal(t, 25_000, fee)

	require.NoError(t, DB.First(&s, sender.Id).Error)
	require.NoError(t, DB.First(&r, receiver.Id).Error)
	assert.Equal(t, 50_000_000-quota-(quota+25_000), s.Quota)
	assert.Equal(t, quota-25_000+quota, r.Quota)
}

func TestTransferQuotaToUserRejectsInvalidTransfers(t *testing.T) {
	truncateTables(t)

	sender := &User{Id: 311, Username: "sender-t2", AffCode: "st2", Quota: int(common.QuotaPerUnit)}
	receiver := &User{Id: 312, Username: "receiver-t2", AffCode: "rt2", Quota: 0}
	require.NoError(t, DB.Create(sender).Error)
	require.NoError(t, DB.Create(receiver).Error)

	quota := int(common.QuotaPerUnit) * 10

	// 余额不足（含手续费）
	_, err := TransferQuotaToUser(sender.Id, receiver.Username, quota, TransferFeeModeExtra)
	require.Error(t, err)

	// 收款人不存在
	_, err = TransferQuotaToUser(sender.Id, "nobody-here", quota, TransferFeeModeDeduct)
	require.Error(t, err)

	// 不能转给自己
	_, err = TransferQuotaToUser(sender.Id, sender.Username, quota, TransferFeeModeDeduct)
	require.Error(t, err)

	// 小于最小转账额度（1 美元）
	_, err = TransferQuotaToUser(sender.Id, receiver.Username, 1, TransferFeeModeDeduct)
	require.Error(t, err)

	// 双方余额不变
	var s, r User
	require.NoError(t, DB.First(&s, sender.Id).Error)
	require.NoError(t, DB.First(&r, receiver.Id).Error)
	assert.Equal(t, int(common.QuotaPerUnit), s.Quota)
	assert.Zero(t, r.Quota)
}
