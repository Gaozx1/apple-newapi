package ratio_setting

import (
	"encoding/json"
	"strings"
	"sync"

	"github.com/QuantumNous/new-api/common"
)

// ImageSizePriceSetting maps model name → {size: per-call USD price}. When a
// relay image request carries a `size` that hits the table, the lookup price
// replaces the model's flat per-call price for that request (count and group
// ratio still multiply on top). Models absent from the table keep using the
// regular model price.
type ImageSizePriceSetting struct {
	mu    sync.RWMutex
	table map[string]map[string]float64
}

var imageSizePriceSetting = &ImageSizePriceSetting{table: make(map[string]map[string]float64)}

// UpdateImageSizePriceByJSONString replaces the whole table from admin JSON.
// Invalid entries (unknown shapes, non-positive prices) are dropped so a bad
// paste can never zero out billing.
func UpdateImageSizePriceByJSONString(jsonStr string) error {
	table := make(map[string]map[string]float64)
	trimmed := strings.TrimSpace(jsonStr)
	if trimmed != "" {
		var raw map[string]map[string]float64
		if err := json.Unmarshal([]byte(trimmed), &raw); err != nil {
			return err
		}
		for model, sizes := range raw {
			model = strings.TrimSpace(model)
			if model == "" {
				continue
			}
			sizePrices := make(map[string]float64)
			for size, price := range sizes {
				size = strings.TrimSpace(size)
				if size == "" || price <= 0 {
					continue
				}
				sizePrices[size] = price
			}
			if len(sizePrices) > 0 {
				table[model] = sizePrices
			}
		}
	}
	imageSizePriceSetting.mu.Lock()
	imageSizePriceSetting.table = table
	imageSizePriceSetting.mu.Unlock()
	return nil
}

// ImageSizePriceJSONString returns the current table for the admin editor.
func ImageSizePriceJSONString() string {
	imageSizePriceSetting.mu.RLock()
	defer imageSizePriceSetting.mu.RUnlock()
	data, err := json.Marshal(imageSizePriceSetting.table)
	if err != nil {
		return "{}"
	}
	return string(data)
}

// GetImageSizePrice looks up the per-call price for model+size. ok is false
// when the model or the size is not configured.
func GetImageSizePrice(model, size string) (float64, bool) {
	imageSizePriceSetting.mu.RLock()
	defer imageSizePriceSetting.mu.RUnlock()
	sizes, ok := imageSizePriceSetting.table[model]
	if !ok {
		return 0, false
	}
	price, ok := sizes[strings.TrimSpace(size)]
	if !ok || price <= 0 {
		return 0, false
	}
	return price, true
}

// GetImageSizePriceFormatted is GetImageSizePrice with the same model-name
// normalization the flat model price uses, so suffix-matched models resolve.
func GetImageSizePriceFormatted(model, size string) (float64, bool) {
	return GetImageSizePrice(FormatMatchingModelName(model), size)
}

var _ = common.GetTimestamp // keep import parity with sibling files
