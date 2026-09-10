package ratio_setting

import (
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"sync"

	"github.com/QuantumNous/new-api/common"
)

// Image tier billing: admins configure per-call USD prices for the 1K/2K/4K
// tiers (and an optional base price for sizes that do not match any tier).
// The tier is derived from the ACTUAL generated image's pixel dimensions when
// the upstream response exposes them, falling back to the requested size, so
// "size: auto" requests are still billed by what was really produced.
//
// Resolution classification:
//   1K: longest edge <= 1280 px          (e.g. 1024x1024)
//   2K: 1280 < longest edge <= 2560 px   (e.g. 2048x2048)
//   4K: longest edge > 2560 px           (e.g. 4096x4096)

type ImageTierPrices struct {
	// Base is the per-call price for requests whose actual size does not
	// classify into 1K/2K/4K (unknown, auto without dimensions, tiny images).
	// When 0, the model's flat per-call price applies for such requests.
	Base1K float64
	Base2K float64
	Base4K float64
	// BasePrice is the fallback per-call price for unclassified sizes. When 0
	// the model's regular ModelPrice is used instead.
	BasePrice float64
	Enabled   bool
}

var (
	imageTierMu      sync.RWMutex
	imageTierEnabled = false
	imageTier1K      = 0.0
	imageTier2K      = 0.0
	imageTier4K      = 0.0
	imageTierBase    = 0.0
)

// UpdateImageTierBilling stores the manual tier prices (USD per call).
func UpdateImageTierBilling(enabled bool, price1K, price2K, price4K, basePrice float64) {
	imageTierMu.Lock()
	defer imageTierMu.Unlock()
	imageTierEnabled = enabled
	imageTier1K = price1K
	imageTier2K = price2K
	imageTier4K = price4K
	imageTierBase = basePrice
}

// GetImageTierBilling returns the current manual tier configuration.
func GetImageTierBilling() ImageTierPrices {
	imageTierMu.RLock()
	defer imageTierMu.RUnlock()
	return ImageTierPrices{
		Enabled:   imageTierEnabled,
		Base1K:    imageTier1K,
		Base2K:    imageTier2K,
		Base4K:    imageTier4K,
		BasePrice: imageTierBase,
	}
}

// ImageTier classifies pixel dimensions into a billing tier.
type ImageTier string

const (
	ImageTier1K ImageTier = "1K"
	ImageTier2K ImageTier = "2K"
	ImageTier4K ImageTier = "4K"
)

// ClassifyImageTier maps a width/height pair to a billing tier by the longest
// edge. ok is false when either dimension is missing or non-positive.
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
func (p ImageTierPrices) TierPrice(tier ImageTier) (float64, bool) {
	var price float64
	switch tier {
	case ImageTier1K:
		price = p.Base1K
	case ImageTier2K:
		price = p.Base2K
	case ImageTier4K:
		price = p.Base4K
	}
	return price, price > 0
}

// parseImageDimensions extracts the first "width x height" pair found in a
// size string. Accepts "1024x1024", "1792x1024", "1024*1024", "auto 2048x2048"
// and similar shapes; returns 0,0 when nothing parses.
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

// ResolveImageTierPrice classifies the given size and returns the configured
// tier price. tier is empty and ok false when tier billing is disabled, the
// size is unclassifiable, or the matched tier has no configured price (the
// caller then falls back to the flat model price).
func ResolveImageTierPrice(size string) (price float64, tier ImageTier, ok bool) {
	cfg := GetImageTierBilling()
	if !cfg.Enabled {
		return 0, "", false
	}
	w, h := parseImageDimensions(size)
	tier, classified := ClassifyImageTier(w, h)
	if !classified {
		// Unclassifiable size: use the optional base price when configured.
		if cfg.BasePrice > 0 {
			return cfg.BasePrice, "", true
		}
		return 0, "", false
	}
	price, priced := cfg.TierPrice(tier)
	if !priced {
		return 0, "", false
	}
	return price, tier, true
}

var _ = common.GetTimestamp // import parity with sibling setting files

// UpdateImageTierBillingByJSONString accepts an admin JSON body shaped as
// {"enabled":true,"price_1k":0.04,"price_2k":0.08,"price_4k":0.15,"base_price":0.02}.
// It exists so the option framework can round-trip the manual form without
// exposing a JSON editor to operators.
func UpdateImageTierBillingByJSONString(jsonStr string) error {
	var body struct {
		Enabled   bool    `json:"enabled"`
		Price1K   float64 `json:"price_1k"`
		Price2K   float64 `json:"price_2k"`
		Price4K   float64 `json:"price_4k"`
		BasePrice float64 `json:"base_price"`
	}
	if err := json.Unmarshal([]byte(strings.TrimSpace(jsonStr)), &body); err != nil {
		return err
	}
	if body.Price1K < 0 || body.Price2K < 0 || body.Price4K < 0 || body.BasePrice < 0 {
		return errors.New("价格不能为负数")
	}
	UpdateImageTierBilling(body.Enabled, body.Price1K, body.Price2K, body.Price4K, body.BasePrice)
	return nil
}

// ImageTierBillingJSONString serializes the current configuration.
func ImageTierBillingJSONString() string {
	cfg := GetImageTierBilling()
	data, err := json.Marshal(map[string]interface{}{
		"enabled":    cfg.Enabled,
		"price_1k":   cfg.Base1K,
		"price_2k":   cfg.Base2K,
		"price_4k":   cfg.Base4K,
		"base_price": cfg.BasePrice,
	})
	if err != nil {
		return "{}"
	}
	return string(data)
}
