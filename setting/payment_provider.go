package setting

import (
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/operation_setting"
)

// ============================================================================
// Payment provider profiles
//
// A payment *channel* is what the user picks (Alipay, WeChat, …). A payment
// *provider* is who processes it (Epay, Stripe, Creem, Waffo, …). One channel
// can be served by several providers: PayProviders lists every configured
// provider profile, and a PayMethod entry names the provider it should use via
// the optional "provider" key. Methods without that key keep using Epay, so
// existing configurations behave exactly as before.
// ============================================================================

// PayProvider is one configured provider profile. Fields not relevant to a
// provider stay empty; only the ones its adapter reads are used.
type PayProvider struct {
	// Id is the routing key referenced from PayMethods[].provider.
	Id string `json:"id"`
	// Type selects the adapter: epay / stripe / creem / waffo / waffo_pancake.
	Type string `json:"type"`
	// Name is shown on the method button when the channel itself has no label.
	Name string `json:"name"`
	// Epay fields.
	PayAddress string `json:"pay_address"`
	EpayId     string `json:"epay_id"`
	EpayKey    string `json:"epay_key"`
	// Stripe fields.
	StripeApiSecret     string `json:"stripe_api_secret"`
	StripeWebhookSecret string `json:"stripe_webhook_secret"`
	StripePriceId       string `json:"stripe_price_id"`
	// Creem fields.
	CreemApiKey        string `json:"creem_api_key"`
	CreemWebhookSecret string `json:"creem_webhook_secret"`
	// Waffo fields.
	WaffoApiKey string `json:"waffo_api_key"`
	// Extra fee ratio in percent charged on top of the payment amount.
	FeePercent float64 `json:"fee_percent"`
	Enabled    bool    `json:"enabled"`
}

// PayProviders is the administrator-configured provider profile list.
var PayProviders = []PayProvider{}

// PayProviders2JsonString serializes the provider profiles for the options table.
func PayProviders2JsonString() string {
	if len(PayProviders) == 0 {
		return "[]"
	}
	data, err := common.Marshal(PayProviders)
	if err != nil {
		return "[]"
	}
	return string(data)
}

// UpdatePayProvidersByJsonString replaces the provider profiles.
func UpdatePayProvidersByJsonString(value string) error {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		PayProviders = []PayProvider{}
		return nil
	}
	providers := make([]PayProvider, 0, 4)
	if err := common.UnmarshalJsonStr(trimmed, &providers); err != nil {
		return err
	}
	PayProviders = providers
	return nil
}

// GetEnabledPayProviders returns the enabled provider profiles.
func GetEnabledPayProviders() []PayProvider {
	providers := make([]PayProvider, 0, len(PayProviders))
	for _, provider := range PayProviders {
		if provider.Enabled {
			providers = append(providers, provider)
		}
	}
	return providers
}

// FindPayProvider resolves a provider profile by id.
func FindPayProvider(id string) *PayProvider {
	trimmed := strings.TrimSpace(id)
	if trimmed == "" {
		return nil
	}
	for index := range PayProviders {
		if PayProviders[index].Id == trimmed && PayProviders[index].Enabled {
			return &PayProviders[index]
		}
	}
	return nil
}

// ResolvePayProviderForMethod returns the provider profile backing one payment
// method entry. A method without a "provider" key falls back to the legacy
// single-provider configuration (nil means "use the built-in settings").
func ResolvePayProviderForMethod(method map[string]string) *PayProvider {
	if method == nil {
		return nil
	}
	return FindPayProvider(method["provider"])
}

// EpayConfigOf returns the Epay endpoint and credentials for a payment method.
// A method bound to a provider profile uses that profile; otherwise the legacy
// single-provider settings apply.
func EpayConfigOf(method map[string]string) (payAddress, partnerId, key string) {
	if provider := ResolvePayProviderForMethod(method); provider != nil && provider.Type == "epay" {
		return provider.PayAddress, provider.EpayId, provider.EpayKey
	}
	return operation_setting.PayAddress, operation_setting.EpayId, operation_setting.EpayKey
}

// StripeConfigOf returns the Stripe credentials for a payment method.
func StripeConfigOf(method map[string]string) (apiSecret, webhookSecret, priceId string, promotionCodes bool) {
	if provider := ResolvePayProviderForMethod(method); provider != nil && provider.Type == "stripe" {
		return provider.StripeApiSecret, provider.StripeWebhookSecret, provider.StripePriceId, StripePromotionCodesEnabled
	}
	return StripeApiSecret, StripeWebhookSecret, StripePriceId, StripePromotionCodesEnabled
}

// MethodFeePercent returns the surcharge percentage configured on the provider
// backing a method (0 when the provider has none).
func MethodFeePercent(method map[string]string) float64 {
	provider := ResolvePayProviderForMethod(method)
	if provider == nil || provider.FeePercent <= 0 {
		return 0
	}
	if provider.FeePercent > 100 {
		return 100
	}
	return provider.FeePercent
}

// FindPayMethod resolves one configured method by its type key.
func FindPayMethod(methodType string) map[string]string {
	trimmed := strings.TrimSpace(methodType)
	for _, method := range operation_setting.PayMethods {
		if method["type"] == trimmed {
			return method
		}
	}
	return nil
}

// PayProviderTypeOf reports the adapter type of the provider backing a method,
// falling back to the platform defaults when legacy settings are in use.
func PayProviderTypeOf(method map[string]string) string {
	if provider := ResolvePayProviderForMethod(method); provider != nil {
		return provider.Type
	}
	if method == nil {
		return ""
	}
	return method["type"]
}
