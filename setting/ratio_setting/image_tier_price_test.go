package ratio_setting

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUpdateImageTierBillingByJSONStringRoundTrip(t *testing.T) {
	require.NoError(t, UpdateImageTierBillingByJSONString(
		`{"enabled":true,"price_1k":0.04,"price_2k":0.08,"price_4k":0.15,"base_price":0.02}`,
	))
	defer UpdateImageTierBilling(false, 0, 0, 0, 0)

	cfg := GetImageTierBilling()
	assert.True(t, cfg.Enabled)
	assert.Equal(t, 0.04, cfg.Base1K)
	assert.Equal(t, 0.08, cfg.Base2K)
	assert.Equal(t, 0.15, cfg.Base4K)
	assert.Equal(t, 0.02, cfg.BasePrice)

	// JSON serialization round-trips for the option framework.
	serialized := ImageTierBillingJSONString()
	require.NoError(t, UpdateImageTierBillingByJSONString(serialized))
	cfg2 := GetImageTierBilling()
	assert.Equal(t, cfg, cfg2)
}

func TestUpdateImageTierBillingRejectsNegativePrices(t *testing.T) {
	require.Error(t, UpdateImageTierBillingByJSONString(
		`{"enabled":true,"price_1k":-1}`))
}

func TestResolveImageTierPriceTieredOverride(t *testing.T) {
	UpdateImageTierBilling(true, 0.04, 0.08, 0.15, 0)
	defer UpdateImageTierBilling(false, 0, 0, 0, 0)

	// 1024x1024 → 1K
	price, tier, ok := ResolveImageTierPrice("1024x1024")
	require.True(t, ok)
	assert.Equal(t, ImageTier1K, tier)
	assert.Equal(t, 0.04, price)

	// 1792x1024 → longest edge 1792 → 2K
	price, tier, ok = ResolveImageTierPrice("1792x1024")
	require.True(t, ok)
	assert.Equal(t, ImageTier2K, tier)
	assert.Equal(t, 0.08, price)

	// 4096x2160 → 4K
	price, tier, ok = ResolveImageTierPrice("4096x2160")
	require.True(t, ok)
	assert.Equal(t, ImageTier4K, tier)
	assert.Equal(t, 0.15, price)

	// Auto/unknown size with no base price → no override.
	_, _, ok = ResolveImageTierPrice("auto")
	assert.False(t, ok)
}
