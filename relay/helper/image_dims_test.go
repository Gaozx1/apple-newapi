package helper

import (
	"encoding/base64"
	"testing"

	"github.com/QuantumNous/new-api/setting/ratio_setting"

	"github.com/stretchr/testify/assert"
)

func b64(t *testing.T, data []byte) string {
	t.Helper()
	return base64.StdEncoding.EncodeToString(data)
}

func pngWithSize(w, h uint32) []byte {
	data := []byte("\x89PNG\r\n\x1a\n")
	// IHDR length + "IHDR"
	data = append(data, 0x00, 0x00, 0x00, 0x0D, 'I', 'H', 'D', 'R')
	data = append(data, byte(w>>24), byte(w>>16), byte(w>>8), byte(w))
	data = append(data, byte(h>>24), byte(h>>16), byte(h>>8), byte(h))
	return data
}

func TestDimensionsFromBase64ImagePNG(t *testing.T) {
	b64 := b64(t, pngWithSize(1024, 1792))
	w, h, ok := DimensionsFromBase64Image(b64)
	assert.True(t, ok)
	assert.Equal(t, 1024, w)
	assert.Equal(t, 1792, h)
}

func TestDimensionsFromBase64ImageDataURL(t *testing.T) {
	raw := pngWithSize(2048, 2048)
	b64 := "data:image/png;base64," + b64(t, raw)
	w, h, ok := DimensionsFromBase64Image(b64)
	assert.True(t, ok)
	assert.Equal(t, 2048, w)
	assert.Equal(t, 2048, h)
}

func TestDimensionsFromBase64ImageNotImage(t *testing.T) {
	_, _, ok := DimensionsFromBase64Image(b64(t, []byte("just some text payload")))
	assert.False(t, ok)

	_, _, ok = DimensionsFromBase64Image("")
	assert.False(t, ok)
}

func TestClassifyImageTierBoundaries(t *testing.T) {
	// 1K: longest edge <= 1280
	tier, ok := ratio_setting.ClassifyImageTier(1024, 1024)
	assert.True(t, ok)
	assert.Equal(t, ratio_setting.ImageTier1K, tier)

	tier, ok = ratio_setting.ClassifyImageTier(1280, 720)
	assert.True(t, ok)
	assert.Equal(t, ratio_setting.ImageTier1K, tier)

	// 2K: 1281..2560
	tier, ok = ratio_setting.ClassifyImageTier(2048, 2048)
	assert.True(t, ok)
	assert.Equal(t, ratio_setting.ImageTier2K, tier)

	tier, ok = ratio_setting.ClassifyImageTier(2560, 1440)
	assert.True(t, ok)
	assert.Equal(t, ratio_setting.ImageTier2K, tier)

	// 4K: > 2560
	tier, ok = ratio_setting.ClassifyImageTier(4096, 4096)
	assert.True(t, ok)
	assert.Equal(t, ratio_setting.ImageTier4K, tier)

	// Invalid dims.
	_, ok = ratio_setting.ClassifyImageTier(0, 0)
	assert.False(t, ok)
}

func TestResolveImageTierPriceConfigAndFallback(t *testing.T) {
	ratio_setting.UpdateImageTierBilling(false, 0.04, 0.08, 0.15, 0)
	// Disabled → no override.
	_, _, ok := ratio_setting.ResolveImageTierPrice("1024x1024")
	assert.False(t, ok)

	ratio_setting.UpdateImageTierBilling(true, 0.04, 0.08, 0.15, 0)
	defer ratio_setting.UpdateImageTierBilling(false, 0, 0, 0, 0)

	price, tier, ok := ratio_setting.ResolveImageTierPrice("1024x1024")
	assert.True(t, ok)
	assert.Equal(t, ratio_setting.ImageTier1K, tier)
	assert.Equal(t, 0.04, price)

	price, tier, ok = ratio_setting.ResolveImageTierPrice("4096x4096")
	assert.True(t, ok)
	assert.Equal(t, ratio_setting.ImageTier4K, tier)
	assert.Equal(t, 0.15, price)

	// Unclassifiable size with base price configured → base price, empty tier.
	ratio_setting.UpdateImageTierBilling(true, 0.04, 0.08, 0.15, 0.02)
	price, tier, ok = ratio_setting.ResolveImageTierPrice("auto")
	assert.True(t, ok)
	assert.Empty(t, tier)
	assert.Equal(t, 0.02, price)

	// Unclassifiable without base price → no override.
	ratio_setting.UpdateImageTierBilling(true, 0.04, 0.08, 0.15, 0)
	_, _, ok = ratio_setting.ResolveImageTierPrice("auto")
	assert.False(t, ok)
}
