package ratio_setting

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestImageSizePriceTableRoundTrip(t *testing.T) {
	require.NoError(t, UpdateImageSizePriceByJSONString(
		`{"gpt-image-1": {"1024x1024": 0.04, "1792x1024": 0.08}, "bad-model": {"tiny": -1}}`,
	))
	defer func() { require.NoError(t, UpdateImageSizePriceByJSONString(`{}`)) }()

	// Exact hit.
	price, ok := GetImageSizePrice("gpt-image-1", "1024x1024")
	assert.True(t, ok)
	assert.Equal(t, 0.04, price)

	// Second size on the same model.
	price, ok = GetImageSizePrice("gpt-image-1", "1792x1024")
	assert.True(t, ok)
	assert.Equal(t, 0.08, price)

	// Unknown size on a configured model → miss.
	_, ok = GetImageSizePrice("gpt-image-1", "256x256")
	assert.False(t, ok)

	// Unknown model → miss.
	_, ok = GetImageSizePrice("dall-e-3", "1024x1024")
	assert.False(t, ok)

	// Non-positive prices are dropped at parse time.
	_, ok = GetImageSizePrice("bad-model", "tiny")
	assert.False(t, ok)

	// JSON round trip keeps valid entries.
	var stored map[string]map[string]float64
	require.NoError(t, json.Unmarshal([]byte(ImageSizePriceJSONString()), &stored))
	assert.Len(t, stored["gpt-image-1"], 2)
}

func TestImageSizePriceEmptyTableMeansNoOverride(t *testing.T) {
	require.NoError(t, UpdateImageSizePriceByJSONString(``))
	_, ok := GetImageSizePrice("gpt-image-1", "1024x1024")
	assert.False(t, ok)
}
