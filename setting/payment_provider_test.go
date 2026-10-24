package setting

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/QuantumNous/new-api/setting/operation_setting"
)

func TestPayProviderRoutingResolvesPerMethod(t *testing.T) {
	originalProviders := PayProviders
	originalMethods := operation_setting.PayMethods
	t.Cleanup(func() {
		PayProviders = originalProviders
		operation_setting.PayMethods = originalMethods
	})

	require.NoError(t, UpdatePayProvidersByJsonString(`[
		{"id":"epay-a","type":"epay","name":"A","pay_address":"https://a.example","epay_id":"1001","epay_key":"ka","enabled":true},
		{"id":"epay-b","type":"epay","name":"B","pay_address":"https://b.example","epay_id":"1002","epay_key":"kb","enabled":true,"fee_percent":2.5},
		{"id":"epay-off","type":"epay","pay_address":"https://c.example","epay_id":"1003","epay_key":"kc","enabled":false}
	]`))
	operation_setting.PayMethods = []map[string]string{
		{"name": "支付宝", "type": "alipay", "provider": "epay-a"},
		{"name": "微信", "type": "wxpay", "provider": "epay-b"},
		{"name": "旧通道", "type": "custom1"},
	}

	address, partnerId, key := EpayConfigOf(FindPayMethod("alipay"))
	assert.Equal(t, "https://a.example", address)
	assert.Equal(t, "1001", partnerId)
	assert.Equal(t, "ka", key)

	address, partnerId, _ = EpayConfigOf(FindPayMethod("wxpay"))
	assert.Equal(t, "https://b.example", address)
	assert.Equal(t, "1002", partnerId)
	assert.Equal(t, 2.5, MethodFeePercent(FindPayMethod("wxpay")))

	// A method without a provider keeps using the legacy settings.
	legacyAddress := operation_setting.PayAddress
	address, _, _ = EpayConfigOf(FindPayMethod("custom1"))
	assert.Equal(t, legacyAddress, address)

	// A disabled provider is never resolved, so the method falls back to legacy.
	assert.Nil(t, ResolvePayProviderForMethod(map[string]string{"provider": "epay-off"}))
	assert.Equal(t, 0.0, MethodFeePercent(map[string]string{"provider": "epay-off"}))
}

func TestPayProviderFeeIsCapped(t *testing.T) {
	originalProviders := PayProviders
	t.Cleanup(func() { PayProviders = originalProviders })

	require.NoError(t, UpdatePayProvidersByJsonString(`[
		{"id":"greedy","type":"epay","enabled":true,"fee_percent":250}
	]`))
	assert.Equal(t, 100.0, MethodFeePercent(map[string]string{"provider": "greedy"}))
}
