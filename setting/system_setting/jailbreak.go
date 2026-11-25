package system_setting

import (
	"errors"
	"net"
	"net/url"
	"slices"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/config"
)

type JailbreakSettings struct {
	Enabled         bool     `json:"enabled"`
	BaseURL         string   `json:"base_url"`
	APIKey          string   `json:"api_key"`
	BlacklistGroups []string `json:"blacklist_groups"`
	// WarningLimit is how many confirmed attacks only trigger an email warning
	// before the account is banned. 0 bans on the first confirmed attack.
	WarningLimit int `json:"warning_limit"`
}

var defaultJailbreakSettings = JailbreakSettings{BlacklistGroups: []string{}, WarningLimit: 3}

func init() {
	config.GlobalConfig.Register("jailbreak_setting", &defaultJailbreakSettings)
}

// GetJailbreakSettings snapshots settings under the same lock used by option
// updates, so asynchronous checks never share a mutable configuration.
func GetJailbreakSettings() JailbreakSettings {
	common.OptionMapRWMutex.RLock()
	defer common.OptionMapRWMutex.RUnlock()
	settings := defaultJailbreakSettings
	settings.BaseURL = strings.TrimSpace(settings.BaseURL)
	settings.APIKey = strings.TrimSpace(settings.APIKey)
	settings.BlacklistGroups = slices.Clone(settings.BlacklistGroups)
	if settings.WarningLimit < 0 {
		settings.WarningLimit = 0
	}
	return settings
}

func (s JailbreakSettings) ShouldCheck(userGroup string) bool {
	return s.Enabled && s.BaseURL != "" && s.APIKey != "" && !slices.Contains(s.BlacklistGroups, userGroup)
}

func (s JailbreakSettings) Validate() error {
	if s.BaseURL != "" {
		parsed, err := url.Parse(s.BaseURL)
		if err != nil || parsed.Hostname() == "" || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
			return errors.New("Enter a valid review service Base URL")
		}
		loopback := parsed.Hostname() == "localhost"
		if ip := net.ParseIP(parsed.Hostname()); ip != nil {
			loopback = ip.IsLoopback()
		}
		if parsed.Scheme != "https" && !(parsed.Scheme == "http" && loopback) {
			return errors.New("The review service must use HTTPS, except for localhost")
		}
	}
	if strings.ContainsAny(s.APIKey, "\r\n") {
		return errors.New("The review API key must not contain line breaks")
	}
	for _, group := range s.BlacklistGroups {
		if strings.TrimSpace(group) == "" || group != strings.TrimSpace(group) {
			return errors.New("Review blacklist groups must be non-empty group names")
		}
	}
	if s.Enabled && (s.BaseURL == "" || strings.TrimSpace(s.APIKey) == "") {
		return errors.New("Configure the review service Base URL and API key before enabling review")
	}
	return nil
}

func ValidateJailbreakOption(key, value string) error {
	settings := GetJailbreakSettings()
	switch key {
	case "jailbreak_setting.enabled":
		enabled, err := strconv.ParseBool(value)
		if err != nil {
			return errors.New("Invalid parameters")
		}
		settings.Enabled = enabled
	case "jailbreak_setting.base_url":
		settings.BaseURL = strings.TrimSpace(value)
	case "jailbreak_setting.api_key":
		settings.APIKey = value
	case "jailbreak_setting.blacklist_groups":
		if err := common.UnmarshalJsonStr(value, &settings.BlacklistGroups); err != nil || settings.BlacklistGroups == nil {
			return errors.New("Review blacklist groups must be a JSON array of group names")
		}
	case "jailbreak_setting.warning_limit":
		limit, err := strconv.Atoi(strings.TrimSpace(value))
		if err != nil || limit < 0 || limit > 100 {
			return errors.New("Warning count must be an integer between 0 and 100")
		}
		settings.WarningLimit = limit
	default:
		return errors.New("Invalid parameters")
	}
	return settings.Validate()
}
