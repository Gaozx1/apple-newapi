package operation_setting

import (
	"errors"
	"maps"
	"net/url"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"sync"

	"github.com/QuantumNous/new-api/common"
)

// PaymentChannel is an Epay-compatible processor. Wallet addresses are managed
// by the processor; the gateway needs only merchant credentials.
type PaymentChannel struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	Address       string `json:"address"`
	MerchantID    string `json:"merchant_id"`
	Key           string `json:"key,omitempty"`
	KeyConfigured bool   `json:"key_configured,omitempty"`
	Enabled       bool   `json:"enabled"`
}

var paymentChannels = struct {
	sync.RWMutex
	items []PaymentChannel
}{}

var paymentChannelID = regexp.MustCompile(`^[a-zA-Z0-9_-]{1,32}$`)

func GetPaymentChannels() []PaymentChannel {
	paymentChannels.RLock()
	defer paymentChannels.RUnlock()
	return slices.Clone(paymentChannels.items)
}

func UpdatePaymentChannels(value string) error {
	var items []PaymentChannel
	if err := common.UnmarshalJsonStr(value, &items); err != nil {
		return err
	}
	paymentChannels.Lock()
	paymentChannels.items = items
	paymentChannels.Unlock()
	return nil
}

func MaskPaymentChannels(value string) string {
	var items []PaymentChannel
	if common.UnmarshalJsonStr(value, &items) != nil {
		return "[]"
	}
	for i := range items {
		items[i].KeyConfigured = items[i].Key != ""
		items[i].Key = ""
	}
	if items == nil {
		items = []PaymentChannel{}
	}
	data, _ := common.Marshal(items)
	return string(data)
}

// NormalizePaymentChannels preserves masked credentials by immutable channel
// ID. Removing a bound channel is rejected rather than silently rerouting it.
func NormalizePaymentChannels(value string) (string, error) {
	var items []PaymentChannel
	if err := common.UnmarshalJsonStr(value, &items); err != nil || items == nil || len(items) > 100 {
		return "", common.NewMessage("Invalid payment channels")
	}
	previous := GetPaymentChannels()
	seen := make(map[string]bool, len(items))
	for i := range items {
		item := &items[i]
		item.ID = strings.TrimSpace(item.ID)
		item.Name = strings.TrimSpace(item.Name)
		item.Address = strings.TrimRight(strings.TrimSpace(item.Address), "/")
		item.MerchantID = strings.TrimSpace(item.MerchantID)
		if !paymentChannelID.MatchString(item.ID) || seen[item.ID] || item.Name == "" || len(item.Name) > 120 {
			return "", common.NewMessage("Invalid or duplicate payment channel ID or name")
		}
		seen[item.ID] = true
		if item.Key == "" {
			for _, old := range previous {
				if old.ID == item.ID {
					item.Key = old.Key
					break
				}
			}
		}
		item.KeyConfigured = false
		if item.Address != "" {
			u, err := url.Parse(item.Address)
			if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Hostname() == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
				return "", common.NewMessage("Invalid payment gateway address")
			}
		}
		if len(item.Key) > 512 || len(item.MerchantID) > 100 || (item.Enabled && (item.Address == "" || item.MerchantID == "" || item.Key == "")) {
			return "", common.NewMessage("Enabled payment channels require an address, merchant ID and key")
		}
	}
	for _, method := range PayMethods {
		if id := method["channel_id"]; id != "" && !seen[id] {
			return "", common.NewMessage("Unbind payment methods before deleting their channel")
		}
	}
	data, err := common.Marshal(items)
	return string(data), err
}

func IsNativePaymentMethod(method string) bool {
	return slices.Contains([]string{"stripe", "creem", "waffo", "waffo_pancake", "balance"}, method)
}

func ValidatePaymentMethods(value string) error {
	var methods []map[string]string
	if err := common.UnmarshalJsonStr(value, &methods); err != nil || methods == nil || len(methods) > 100 {
		return common.NewMessage("Invalid payment methods")
	}
	seen := make(map[string]bool, len(methods))
	channels := GetPaymentChannels()
	for _, method := range methods {
		key := method["type"]
		if strings.TrimSpace(key) == "" || len(key) > 50 || seen[key] || strings.TrimSpace(method["name"]) == "" {
			return common.NewMessage("Payment method keys must be unique and names must be present")
		}
		seen[key] = true
		if value := method["min_topup"]; value != "" {
			minimum, err := strconv.ParseFloat(value, 64)
			if err != nil || minimum < 0 || minimum > float64(common.MaxWalletQuota) || minimum != minimum {
				return common.NewMessage("Invalid minimum top-up amount")
			}
		}
		if len(method["gateway_type"]) > 50 || (IsNativePaymentMethod(key) && (method["channel_id"] != "" || method["gateway_type"] != "")) {
			return common.NewMessage("Native payment methods use their existing gateway settings")
		}
		if id := method["channel_id"]; id != "" && !slices.ContainsFunc(channels, func(channel PaymentChannel) bool { return channel.ID == id }) {
			return common.NewMessage("Payment channel does not exist")
		}
	}
	return nil
}

func FindPaymentMethod(key string) map[string]string {
	for _, method := range PayMethods {
		if method["type"] == key {
			return maps.Clone(method)
		}
	}
	return nil
}

// ResolveEpayChannel never falls back when an explicit binding is unavailable.
func ResolveEpayChannel(method map[string]string) (PaymentChannel, error) {
	if method == nil || IsNativePaymentMethod(method["type"]) {
		return PaymentChannel{}, common.NewMessage("Payment method does not exist")
	}
	if id := method["channel_id"]; id != "" {
		for _, channel := range GetPaymentChannels() {
			if channel.ID == id && channel.Enabled {
				return channel, nil
			}
		}
		return PaymentChannel{}, common.NewMessage("Payment channel is disabled or unavailable")
	}
	channel := PaymentChannel{Address: PayAddress, MerchantID: EpayId, Key: EpayKey, Enabled: true}
	if channel.Address == "" || channel.MerchantID == "" || channel.Key == "" {
		return PaymentChannel{}, common.NewMessage("Payment information has not been configured by administrator")
	}
	return channel, nil
}

// EpayOrderSnapshot binds an order to its credentials and gateway method even
// after its administrator changes, disables or removes the current channel.
// The containing model field must never be serialized into an API response.
type EpayOrderSnapshot struct {
	Channel     PaymentChannel `json:"channel"`
	GatewayType string         `json:"gateway_type"`
}

func NewEpayOrderSnapshot(method map[string]string) (EpayOrderSnapshot, string, error) {
	channel, err := ResolveEpayChannel(method)
	if err != nil {
		return EpayOrderSnapshot{}, "", err
	}
	gatewayType := method["gateway_type"]
	if gatewayType == "" {
		gatewayType = method["type"]
	}
	snapshot := EpayOrderSnapshot{Channel: channel, GatewayType: gatewayType}
	data, err := common.Marshal(snapshot)
	return snapshot, string(data), err
}

func ReadEpayOrderSnapshot(value string, legacyMethod string) (EpayOrderSnapshot, error) {
	if value == "" {
		return EpayOrderSnapshot{Channel: PaymentChannel{Address: PayAddress, MerchantID: EpayId, Key: EpayKey}, GatewayType: legacyMethod}, nil
	}
	var snapshot EpayOrderSnapshot
	if err := common.UnmarshalJsonStr(value, &snapshot); err != nil {
		return snapshot, err
	}
	if snapshot.Channel.Address == "" || snapshot.Channel.MerchantID == "" || snapshot.Channel.Key == "" || snapshot.GatewayType == "" {
		return snapshot, errors.New("invalid payment order snapshot")
	}
	return snapshot, nil
}
