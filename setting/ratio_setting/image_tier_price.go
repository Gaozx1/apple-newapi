package ratio_setting

import (
	"encoding/json"
	"strconv"
	"strings"
	"sync"

	"github.com/QuantumNous/new-api/common"
)

// Image tier billing: admins configure per-model, per-resolution flat USD
// prices for image generations/edits. When enabled for a model, image calls
// bill a flat tier price per call derived from the ACTUAL generated image's
// pixel dimensions (1K/2K/4K by longest edge), overriding both the model's
// flat per-call price and upstream token billing.
//
// Config JSON shape (managed by the admin pricing UI, not hand-written):
//
//	{
//	  "gpt-image-1": {"enabled": true, "price_1k": 0.04, "price_2k": 0.08, "price_4k": 0.15},
//	  "dall-e-3":    {"enabled": true, "price_1k": 0.02, "price_2k": 0.03, "price_4k": 0.06}
//	}

type ImageModelTierPrice struct {
	Enabled bool    `json:"enabled"`
	Price1K float64 `json:"price_1k"`
	Price2K float64 `json:"price_2k"`
	Price4K float64 `json:"price_4k"`
}

var (
	imageTierMu    sync.RWMutex
	imageTierTable = make(map[string]ImageModelTierPrice)
)

// UpdateImageTierPriceByJSONString replaces the whole per-model tier table
// from admin JSON. Models without valid prices are dropped so a bad paste can
// never break billing.
func UpdateImageTierPriceByJSONString(jsonStr string) error {
	table := make(map[string]ImageModelTierPrice)
	trimmed := strings.TrimSpace(jsonStr)
	if trimmed != "" {
		var raw map[string]struct {
			Enabled bool    `json:"enabled"`
			Price1K float64 `json:"price_1k"`
			Price2K float64 `json:"price_2k"`
			Price4K float64 `json:"price_4k"`
		}
		if err := json.Unmarshal([]byte(trimmed), &raw); err != nil {
			return err
		}
		for model, prices := range raw {
			model = strings.TrimSpace(model)
			if model == "" || !prices.Enabled {
				continue
			}
			if prices.Price1K <= 0 && prices.Price2K <= 0 && prices.Price4K <= 0 {
				continue
			}
			table[model] = ImageModelTierPrice{
				Enabled: true,
				Price1K: prices.Price1K,
				Price2K: prices.Price2K,
				Price4K: prices.Price4K,
			}
		}
	}
	imageTierMu.Lock()
	imageTierTable = table
	imageTierMu.Unlock()
	return nil
}

// ImageTierPriceJSONString serializes the current table for the admin editor.
func ImageTierPriceJSONString() string {
	imageTierMu.RLock()
	defer imageTierMu.RUnlock()
	data, err := json.Marshal(imageTierTable)
	if err != nil {
		return "{}"
	}
	return string(data)
}

// GetImageModelTierPrice returns the tier config for a model. ok is false when
// the model has no enabled tier pricing.
func GetImageModelTierPrice(model string) (ImageModelTierPrice, bool) {
	imageTierMu.RLock()
	defer imageTierMu.RUnlock()
	cfg, ok := imageTierTable[strings.TrimSpace(model)]
	if !ok || !cfg.Enabled {
		return ImageModelTierPrice{}, false
	}
	return cfg, true
}

// ImageTier classifies pixel dimensions into a billing tier by longest edge:
// 1K: <= 1280px, 2K: <= 2560px, 4K: > 2560px.
type ImageTier string

const (
	ImageTier1K ImageTier = "1K"
	ImageTier2K ImageTier = "2K"
	ImageTier4K ImageTier = "4K"
)

// Valid reports whether the tier carries a real classification.
func (t ImageTier) Valid() bool {
	return t == ImageTier1K || t == ImageTier2K || t == ImageTier4K
}

// ClassifyImageTier maps a width/height pair to a billing tier. ok is false
// when either dimension is missing or non-positive.
func ClassifyImageTier(width, height int) (ImageTier, bool) {
	if width <= 0 || height <= 0 {
		return "", false
	}
	longest := width
	if height > longest {
		longest = height
	}
	switch {
	case longest <= 1280:
		return ImageTier1K, true
	case longest <= 2560:
		return ImageTier2K, true
	default:
		return ImageTier4K, true
	}
}

// TierPrice returns the configured USD price for a tier. ok is false when the
// tier has no positive price configured.
func (p ImageModelTierPrice) TierPrice(tier ImageTier) (float64, bool) {
	var price float64
	switch tier {
	case ImageTier1K:
		price = p.Price1K
	case ImageTier2K:
		price = p.Price2K
	case ImageTier4K:
		price = p.Price4K
	}
	return price, price > 0
}

// parseImageDimensions extracts the first "width x height" pair found in a
// size string. Accepts "1024x1024", "1792x1024", "1024*1024" and similar
// shapes; returns 0,0 when nothing parses.
func parseImageDimensions(size string) (int, int) {
	size = strings.ToLower(strings.TrimSpace(size))
	if size == "" {
		return 0, 0
	}
	normalized := strings.NewReplacer("*", "x", "×", "x", "X", "x").Replace(size)
	parts := strings.SplitN(normalized, "x", 2)
	if len(parts) != 2 {
		return 0, 0
	}
	w, err1 := strconv.Atoi(strings.TrimSpace(parts[0]))
	h, err2 := strconv.Atoi(strings.TrimSpace(parts[1]))
	if err1 != nil || err2 != nil || w <= 0 || h <= 0 {
		return 0, 0
	}
	return w, h
}

// ResolveImageTierPriceForModel classifies the given size and returns the
// per-call price from the model's tier config. tier is empty and ok false when
// the size cannot be classified or the matched tier has no price — the caller
// then falls back to the flat model price.
func ResolveImageTierPriceForModel(model, size string) (price float64, tier ImageTier, ok bool) {
	cfg, cfgOk := GetImageModelTierPrice(model)
	if !cfgOk {
		return 0, "", false
	}
	w, h := parseImageDimensions(size)
	tier, classified := ClassifyImageTier(w, h)
	if !classified {
		return 0, "", false
	}
	price, priced := cfg.TierPrice(tier)
	if !priced {
		return 0, "", false
	}
	return price, tier, true
}

var _ = common.GetTimestamp // import parity with sibling setting files
