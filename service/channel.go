package service

import (
	"fmt"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/i18n"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/QuantumNous/new-api/setting/operation_setting"
)

func formatNotifyType(channelId int, status int) string {
	return fmt.Sprintf("%s_%d_%d", dto.NotifyTypeChannelUpdate, channelId, status)
}

func shouldCloseActiveWebSocketsAfterDisable(channelId int) bool {
	channel, err := model.GetChannelById(channelId, true)
	if err != nil {
		common.SysLog(common.LogText("failed to check channel status before closing active websockets: channel_id=%d, error=%v", channelId, err))
		return true
	}
	return channel.Status != common.ChannelStatusEnabled
}

// disable & notify
func DisableChannel(channelError types.ChannelError, reason string) {
	common.SysLog(common.LogText("channel %q (#%d) failed, disabling it, reason: %s", channelError.ChannelName, channelError.ChannelId, common.LocalLogPreview(reason)))

	// 检查是否启用自动禁用功能
	if !channelError.AutoBan {
		common.SysLog(common.LogText("channel %q (#%d) has automatic disabling turned off, skipping", channelError.ChannelName, channelError.ChannelId))
		return
	}

	success := model.UpdateChannelStatus(channelError.ChannelId, channelError.UsingKey, common.ChannelStatusAutoDisabled, reason)
	if success {
		if shouldCloseActiveWebSocketsAfterDisable(channelError.ChannelId) {
			CloseActiveWebSocketsForChannel(channelError.ChannelId, ChannelDisabledCloseReason)
		}
		params := map[string]any{"Name": channelError.ChannelName, "Id": channelError.ChannelId, "Reason": reason}
		NotifyChannelWatchers(formatNotifyType(channelError.ChannelId, common.ChannelStatusAutoDisabled), func(lang string) (string, string) {
			return i18n.Translate(lang, i18n.MsgChannelNotifyDisabledSubject, params), i18n.Translate(lang, i18n.MsgChannelNotifyDisabledContent, params)
		})
	}
}

func EnableChannel(channelId int, usingKey string, channelName string) {
	success := model.UpdateChannelStatus(channelId, usingKey, common.ChannelStatusEnabled, "")
	if success {
		params := map[string]any{"Name": channelName, "Id": channelId}
		NotifyChannelWatchers(formatNotifyType(channelId, common.ChannelStatusEnabled), func(lang string) (string, string) {
			return i18n.Translate(lang, i18n.MsgChannelNotifyEnabledSubject, params), i18n.Translate(lang, i18n.MsgChannelNotifyEnabledContent, params)
		})
	}
}

func ShouldDisableChannel(err *types.NewAPIError) bool {
	if !common.AutomaticDisableChannelEnabled {
		return false
	}
	return ErrorWarrantsChannelDisable(err)
}

// ErrorWarrantsChannelDisable reports whether an upstream error indicates the
// credential or the upstream account is unusable — authentication failures,
// explicit channel errors, configured status codes, and configured
// quota/permission keywords. It deliberately ignores transient failures
// (timeouts, 5xx, rate limits) so a healthy key is not disabled by them.
//
// It applies the same rules as ShouldDisableChannel but without the global
// auto-disable switch: the manual batch key test is an explicit administrator
// action, so it must not be silently neutered when the global switch is off.
func ErrorWarrantsChannelDisable(err *types.NewAPIError) bool {
	if err == nil {
		return false
	}
	if types.IsChannelError(err) {
		return true
	}
	if types.IsSkipRetryError(err) {
		return false
	}
	if operation_setting.ShouldDisableByStatusCode(err.StatusCode) {
		return true
	}

	lowerMessage := strings.ToLower(err.Error())
	search, _ := AcSearch(lowerMessage, operation_setting.AutomaticDisableKeywords, true)
	return search
}

func ShouldEnableChannel(newAPIError *types.NewAPIError, status int) bool {
	if !common.AutomaticEnableChannelEnabled {
		return false
	}
	if newAPIError != nil {
		return false
	}
	if status != common.ChannelStatusAutoDisabled {
		return false
	}
	return true
}
