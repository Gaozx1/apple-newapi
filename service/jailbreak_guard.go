package service

import (
	"bytes"
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/i18n"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/setting/system_setting"

	"github.com/bytedance/gopkg/util/gopool"
)

// notifyRootUserJailbreak alerts the root operator about an auto-ban so a false
// positive can be reviewed in the 破甲拦截记录 page.
func notifyRootUserJailbreak(notifyType string, subject string, content string) {
	notifyAdmins(notifyType, func(role int, _ dto.UserSetting) bool {
		return role == common.RoleRootUser
	}, func(_ string) (string, string) {
		return subject, content
	})
}

// The system-wide detector runs asynchronously. Its explicit safe/attack label
// is authoritative; transport failures and invalid replies never ban accounts.
const (
	maxJudgePromptChars      = 32768 // /predict request limit, in Unicode characters.
	maxJudgeEvidenceChars    = 6000
	jailbreakJudgeTimeout    = 45 * time.Second
	jailbreakJudgeReplyLimit = 1 << 20
)

// Use the standard verified TLS transport independently of upstream channels'
// optional insecure TLS settings. Credentials and text must not follow redirects.
var jailbreakHTTPClient = &http.Client{
	Timeout: jailbreakJudgeTimeout,
	Transport: &http.Transport{
		Proxy:                 http.ProxyFromEnvironment,
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          100,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: time.Second,
		TLSClientConfig:       &tls.Config{MinVersion: tls.VersionTLS12},
	},
	CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
		return http.ErrUseLastResponse
	},
}

type JailbreakCheckRequest struct {
	UserId      int
	UserGroup   string
	ChannelId   int
	ChannelName string
	ModelName   string
	RequestId   string
	TokenId     int
	TokenName   string
	Settings    system_setting.JailbreakSettings
	Body        []byte
}

func DispatchJailbreakCheck(request JailbreakCheckRequest) bool {
	request.Settings = system_setting.GetJailbreakSettings()
	if request.UserId <= 0 || !request.Settings.ShouldCheck(request.UserGroup) {
		return false
	}
	content := ExtractUserPrompt(request.Body)
	if strings.TrimSpace(content) == "" {
		return false
	}
	request.Body = nil
	common.RelayCtxGo(context.Background(), func() {
		if err := runJailbreakCheck(request, content); err != nil {
			common.SysError(fmt.Sprintf("anti-jailbreak check failed for user %d: %s", request.UserId, err.Error()))
		}
	})
	return true
}

func runJailbreakCheck(request JailbreakCheckRequest, content string) error {
	verdict, reply, err := callJailbreakJudge(request.Settings, content)
	if err != nil {
		return err
	}
	if verdict.Label != "attack" {
		return nil
	}
	judgeModel := verdict.Model
	if verdict.ModelRevision != "" {
		judgeModel += "@" + verdict.ModelRevision
	}
	reason := "Prompt injection detected"

	record := &model.JailbreakRecord{
		UserId:      request.UserId,
		ChannelId:   request.ChannelId,
		ChannelName: request.ChannelName,
		ModelName:   request.ModelName,
		RequestId:   request.RequestId,
		TokenId:     request.TokenId,
		TokenName:   request.TokenName,
		JudgeModel:  truncateJudgeText(judgeModel, 127),
		Reason:      reason,
		Content:     truncateJudgeText(content, maxJudgeEvidenceChars),
		RawVerdict:  truncateJudgeText(reply, 2000),
	}
	action, username, warningOrdinal, handleErr := model.HandleJailbreakAttempt(request.UserId, request.Settings.WarningLimit)
	if handleErr != nil {
		return fmt.Errorf("handle jailbreak attempt of user %d failed: %w", request.UserId, handleErr)
	}
	record.Username = username
	record.Action = action
	if action == "banned" {
		record.BannedAt = common.GetTimestamp()
	}
	if err := model.CreateJailbreakRecord(record); err != nil {
		return fmt.Errorf("record verdict failed: %w", err)
	}

	switch action {
	case "warned":
		common.SysLog(fmt.Sprintf(
			"anti-jailbreak: warned user %s (id %d) after hit %d/%d on channel %s (id %d), model %s, reason: %s",
			username, request.UserId, warningOrdinal, request.Settings.WarningLimit,
			request.ChannelName, request.ChannelId, request.ModelName, reason,
		))
		sendJailbreakWarningEmail(request.UserId, warningOrdinal, request.Settings.WarningLimit)
	case "banned":
		common.SysLog(fmt.Sprintf(
			"anti-jailbreak: banned user %s (id %d) after a hit on channel %s (id %d), model %s, reason: %s",
			username, request.UserId, request.ChannelName, request.ChannelId, request.ModelName, reason,
		))
		notifyRootUserJailbreak(dto.NotifyTypeJailbreakBan, "账号因破甲被自动封禁", fmt.Sprintf(
			"用户 %s（ID %d）在渠道 %s（ID %d）使用模型 %s 时被判定为破甲尝试。\n理由：%s\n可在「破甲拦截记录」中查看原文。",
			username, request.UserId, request.ChannelName, request.ChannelId, request.ModelName, reason,
		))
	case "already_banned", "skipped_staff":
		common.SysLog(fmt.Sprintf(
			"anti-jailbreak: user %s (id %d) hit the classifier on channel %d, action=%s",
			username, request.UserId, request.ChannelId, action,
		))
	}
	return nil
}

// sendJailbreakWarningEmail tells the offending user how many warnings remain
// before the account is disabled. A missing email address or SMTP setup is
// logged but never blocks the guard.
func sendJailbreakWarningEmail(userId int, warningOrdinal int, warningLimit int) {
	gopool.Go(func() {
		user, err := model.GetUserById(userId, false)
		if err != nil || user == nil {
			common.SysLog(common.LogText("failed to load user %d for jailbreak warning email: %v", userId, err))
			return
		}
		setting := user.GetSetting()
		lang := setting.Language
		subject := i18n.Translate(lang, i18n.MsgEmailJailbreakWarningSubject, map[string]any{
			"SystemName": common.SystemName,
		})
		content := i18n.Translate(lang, i18n.MsgEmailJailbreakWarningIntro, map[string]any{
			"Warning": warningOrdinal,
			"Limit":   warningLimit,
		}) + "<br/><br/>" + i18n.Translate(lang, i18n.MsgEmailJailbreakWarningAction, map[string]any{
			"Remaining": warningLimit - warningOrdinal,
			"Limit":     warningLimit,
		})
		emailToUse := setting.NotificationEmail
		if emailToUse == "" {
			emailToUse = user.Email
		}
		if emailToUse == "" {
			common.SysLog(common.LogText("user %d has no email, skip sending jailbreak warning email", userId))
			return
		}
		// Bypass the quota-notification rate limiter: this is a security warning
		// whose ordinal must match the stored record count, not a user-facing
		// balance reminder subject to the ordinary notify budget.
		if err := common.SendEmail(subject, emailToUse, content); err != nil {
			common.SysLog(common.LogText("failed to send jailbreak warning email to user %d: %s", userId, err.Error()))
		}
	})
}

// Pointer fields distinguish a complete detector response from missing or
// malformed data. Never derive a verdict from the rounded attack_score.
type jailbreakPrediction struct {
	Label         string   `json:"label"`
	AttackScore   *float64 `json:"attack_score"`
	Threshold     *float64 `json:"threshold"`
	Truncated     *bool    `json:"truncated"`
	Model         string   `json:"model"`
	ModelRevision string   `json:"model_revision"`
}

func callJailbreakJudge(settings system_setting.JailbreakSettings, content string) (jailbreakPrediction, string, error) {
	var verdict jailbreakPrediction
	if err := settings.Validate(); err != nil {
		return verdict, "", err
	}
	if settings.BaseURL == "" || settings.APIKey == "" {
		return verdict, "", errors.New("review service is not configured")
	}
	if len([]rune(content)) > maxJudgePromptChars {
		return verdict, "", errors.New("review text exceeds the detector's character limit")
	}
	// Omit threshold to retain the service's full-precision default. The
	// trained detector does not accept a chat model or a system prompt.
	body, err := common.Marshal(map[string]string{"text": content})
	if err != nil {
		return verdict, "", err
	}
	endpoint := strings.TrimRight(strings.TrimSpace(settings.BaseURL), "/")
	if !strings.HasSuffix(endpoint, "/predict") {
		endpoint += "/predict"
	}
	ctx, cancel := context.WithTimeout(context.Background(), jailbreakJudgeTimeout)
	defer cancel()
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return verdict, "", errors.New("invalid review service URL")
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Authorization", "Bearer "+settings.APIKey)
	response, err := jailbreakHTTPClient.Do(request)
	if err != nil {
		return verdict, "", errors.New("review service request failed")
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return verdict, "", fmt.Errorf("review service returned status %d", response.StatusCode)
	}
	raw, err := io.ReadAll(io.LimitReader(response.Body, jailbreakJudgeReplyLimit+1))
	if err != nil || len(raw) > jailbreakJudgeReplyLimit {
		return verdict, "", errors.New("failed to read review service response")
	}
	if err := common.Unmarshal(raw, &verdict); err != nil {
		return verdict, "", errors.New("invalid review service response")
	}
	if verdict.Label != "safe" && verdict.Label != "attack" {
		return verdict, "", errors.New("review service returned an unknown verdict")
	}
	if verdict.Truncated == nil || *verdict.Truncated || verdict.AttackScore == nil || verdict.Threshold == nil {
		return verdict, "", errors.New("review service returned an incomplete verdict")
	}
	for _, value := range []float64{*verdict.AttackScore, *verdict.Threshold} {
		if math.IsNaN(value) || math.IsInf(value, 0) || value < 0 || value > 1 {
			return verdict, "", errors.New("review service returned an invalid score")
		}
	}
	return verdict, string(raw), nil
}

func truncateJudgeText(text string, limit int) string {
	runes := []rune(text)
	if len(runes) <= limit {
		return text
	}
	return string(runes[:limit]) + "…"
}

// ExtractUserPrompt collects the user-authored text from a relay request body.
// It understands the OpenAI/Claude (`messages`), Responses (`input`) and Gemini
// (`contents`) shapes; system and assistant turns are ignored. Complete user
// text is sent to the detector, which rejects inputs beyond its window limit.
func ExtractUserPrompt(body []byte) string {
	if len(body) == 0 {
		return ""
	}
	var decoded map[string]any
	if err := common.Unmarshal(body, &decoded); err != nil {
		return ""
	}
	var parts []string
	for _, key := range []string{"messages", "input", "contents"} {
		container, ok := decoded[key]
		if !ok {
			continue
		}
		parts = append(parts, collectJudgeUserText(container)...)
	}
	for _, key := range []string{"prompt", "query"} {
		if text, ok := decoded[key].(string); ok && strings.TrimSpace(text) != "" {
			parts = append(parts, text)
		}
	}
	if len(parts) == 0 {
		return ""
	}
	return strings.Join(parts, "\n")
}

func collectJudgeUserText(container any) []string {
	switch value := container.(type) {
	case string:
		if strings.TrimSpace(value) == "" {
			return nil
		}
		return []string{value}
	case []any:
		var collected []string
		for _, item := range value {
			entry, ok := item.(map[string]any)
			if !ok {
				continue
			}
			// Gemini turns omit the role for user content.
			if role, ok := entry["role"].(string); ok {
				lowered := strings.ToLower(strings.TrimSpace(role))
				if lowered != "" && lowered != "user" {
					continue
				}
			}
			collected = append(collected, judgeTextFromEntry(entry)...)
		}
		return collected
	default:
		return nil
	}
}

func judgeTextFromEntry(entry map[string]any) []string {
	var collected []string
	for _, key := range []string{"content", "parts"} {
		switch value := entry[key].(type) {
		case string:
			if strings.TrimSpace(value) != "" {
				collected = append(collected, value)
			}
		case []any:
			for _, item := range value {
				switch typed := item.(type) {
				case string:
					if strings.TrimSpace(typed) != "" {
						collected = append(collected, typed)
					}
				case map[string]any:
					if text, ok := typed["text"].(string); ok && strings.TrimSpace(text) != "" {
						collected = append(collected, text)
					}
				}
			}
		}
	}
	return collected
}
