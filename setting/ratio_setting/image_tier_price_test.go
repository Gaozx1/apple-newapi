package ratio_setting

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUpdateImageTierPriceByJSONStringRoundTrip(t *testing.T) {
	require.NoError(t, UpdateImageTierPriceByJSONString(
		`{"gpt-image-1":{"enabled":true,"price_1k":0.04,"price_2k":0.08,"price_4k":0.15},"dall-e-3":{"enabled":true,"price_1k":0.02}}`,
	))
	defer UpdateImageTierPriceByJSONString(`{}`)

	cfg, ok := GetImageModelTierPrice("gpt-image-1")
	require.True(t, ok)
	assert.True(t, cfg.Enabled)
	assert.Equal(t, 0.04, cfg.Price1K)
	assert.Equal(t, 0.08, cfg.Price2K)
	assert.Equal(t, 0.15, cfg.Price4K)

	// Serialized table round-trips.
	require.NoError(t, UpdateImageTierPriceByJSONString(ImageTierPriceJSONString()))
	cfg2, ok := GetImageModelTierPrice("gpt-image-1")
	require.True(t, ok)
	assert.Equal(t, cfg, cfg2)

	// Disabled entries are dropped.
	require.NoError(t, UpdateImageTierPriceByJSONString(
		`{"m":{"enabled":false,"price_1k":1}}`))
	_, ok = GetImageModelTierPrice("m")
	assert.False(t, ok)
}

func TestUpdateImageTierPriceRejectsMalformedJSON(t *testing.T) {
	require.Error(t, UpdateImageTierPriceByJSONString(`not-json`))
}

func TestResolveImageTierPriceForModelTieredOverride(t *testing.T) {
	require.NoError(t, UpdateImageTierPriceByJSONString(
		`{"gpt-image-1":{"enabled":true,"price_1k":0.04,"price_2k":0.08,"price_4k":0.15}}`))
	defer UpdateImageTierPriceByJSONString(`{}`)

	// 1024x1024 -> 1K
	price, tier, ok := ResolveImageTierPriceForModel("gpt-image-1", "1024x1024")
	require.True(t, ok)
	assert.Equal(t, ImageTier1K, tier)
	assert.Equal(t, 0.04, price)

	// 1792x1024 -> 2K
	price, tier, ok = ResolveImageTierPriceForModel("gpt-image-1", "1792x1024")
	require.True(t, ok)
	assert.Equal(t, ImageTier2K, tier)
	assert.Equal(t, 0.08, price)

	// 4096x2160 -> 4K
	price, tier, ok = ResolveImageTierPriceForModel("gpt-image-1", "4096x2160")
	require.True(t, ok)
	assert.Equal(t, ImageTier4K, tier)
	assert.Equal(t, 0.15, price)

	// auto/unknown size -> no override at this layer.
	_, _, ok = ResolveImageTierPriceForModel("gpt-image-1", "auto")
	assert.False(t, ok)

	// Other models without config -> no override.
	_, _, ok = ResolveImageTierPriceForModel("other-model", "1024x1024")
	assert.False(t, ok)
}
