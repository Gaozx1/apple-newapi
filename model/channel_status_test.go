package model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupChannelStatusTest(t *testing.T) {
	t.Helper()
	truncateTables(t)
	require.NoError(t, DB.Exec("DELETE FROM abilities").Error)
	require.NoError(t, DB.Exec("DELETE FROM channels").Error)

	memoryCacheEnabled := common.MemoryCacheEnabled
	common.MemoryCacheEnabled = false
	t.Cleanup(func() {
		common.MemoryCacheEnabled = memoryCacheEnabled
	})
}

func TestUpdateChannelStatusPersistsMultiKeyState(t *testing.T) {
	setupChannelStatusTest(t)

	channel := Channel{
		Name:   "multi-key-status",
		Key:    "key-a\nkey-b",
		Status: common.ChannelStatusEnabled,
		ChannelInfo: ChannelInfo{
			IsMultiKey:           true,
			MultiKeySize:         2,
			MultiKeyMode:         constant.MultiKeyModePolling,
			MultiKeyPollingIndex: 1,
		},
	}
	require.NoError(t, DB.Create(&channel).Error)

	changed := UpdateChannelStatus(channel.Id, "key-a", common.ChannelStatusAutoDisabled, "provider rejected key")
	require.True(t, changed)

	var stored Channel
	require.NoError(t, DB.First(&stored, channel.Id).Error)
	assert.Equal(t, common.ChannelStatusEnabled, stored.Status)
	assert.Equal(t, common.ChannelStatusAutoDisabled, stored.ChannelInfo.MultiKeyStatusList[0])
	assert.Equal(t, "provider rejected key", stored.ChannelInfo.MultiKeyDisabledReason[0])
	assert.NotZero(t, stored.ChannelInfo.MultiKeyDisabledTime[0])
	assert.Equal(t, 1, stored.ChannelInfo.MultiKeyPollingIndex)
}

// GetKeyByIndex must return the exact indexed key (including a disabled one)
// without touching the shared polling cursor, which the manual batch key test
// relies on to probe every key without disturbing live traffic rotation.
func TestGetKeyByIndexDoesNotAdvancePollingCursor(t *testing.T) {
	setupChannelStatusTest(t)

	channel := Channel{
		Name:   "multi-key-index",
		Key:    "key-a\nkey-b\nkey-c",
		Status: common.ChannelStatusEnabled,
		ChannelInfo: ChannelInfo{
			IsMultiKey:           true,
			MultiKeySize:         3,
			MultiKeyMode:         constant.MultiKeyModePolling,
			MultiKeyPollingIndex: 0,
			MultiKeyStatusList:   map[int]int{1: common.ChannelStatusAutoDisabled},
		},
	}
	require.NoError(t, DB.Create(&channel).Error)

	// A disabled key is still returned by explicit index: the batch test must be
	// able to re-probe it to decide whether it recovered.
	key, apiErr := channel.GetKeyByIndex(1)
	require.Nil(t, apiErr)
	assert.Equal(t, "key-b", key)

	key, apiErr = channel.GetKeyByIndex(2)
	require.Nil(t, apiErr)
	assert.Equal(t, "key-c", key)
	assert.Equal(t, 0, channel.ChannelInfo.MultiKeyPollingIndex, "index lookup must not advance the polling cursor")

	_, apiErr = channel.GetKeyByIndex(3)
	require.NotNil(t, apiErr, "out-of-range index must be rejected")

	_, apiErr = channel.GetKeyByIndex(-1)
	require.NotNil(t, apiErr, "negative index must be rejected")
}

// The scheduled test's cursor-based selection must keep skipping disabled keys;
// pinning a key is opt-in and must not change that behavior.
func TestGetNextEnabledKeyStillSkipsDisabledKeys(t *testing.T) {
	setupChannelStatusTest(t)

	channel := Channel{
		Name:   "multi-key-skip",
		Key:    "key-a\nkey-b",
		Status: common.ChannelStatusEnabled,
		ChannelInfo: ChannelInfo{
			IsMultiKey:           true,
			MultiKeySize:         2,
			MultiKeyMode:         constant.MultiKeyModePolling,
			MultiKeyPollingIndex: 0,
			MultiKeyStatusList:   map[int]int{0: common.ChannelStatusAutoDisabled},
		},
	}
	require.NoError(t, DB.Create(&channel).Error)

	key, index, apiErr := channel.GetNextEnabledKey()
	require.Nil(t, apiErr)
	assert.Equal(t, "key-b", key, "the only enabled key must be selected")
	assert.Equal(t, 1, index)
}

// Re-enabling one key of a multi-key channel must reach the per-key status map
// even while the channel itself stays enabled. The batch key test relies on this
// to restore a key that recovered, and the old channel-level short-circuit
// silently dropped that update.
func TestUpdateChannelStatusReEnablesKeyWhileChannelStaysEnabled(t *testing.T) {
	setupChannelStatusTest(t)

	channel := Channel{
		Name:   "multi-key-reenable",
		Key:    "key-a\nkey-b",
		Status: common.ChannelStatusEnabled,
		ChannelInfo: ChannelInfo{
			IsMultiKey:             true,
			MultiKeySize:           2,
			MultiKeyMode:           constant.MultiKeyModePolling,
			MultiKeyStatusList:     map[int]int{1: common.ChannelStatusAutoDisabled},
			MultiKeyDisabledReason: map[int]string{1: "invalid api key"},
			MultiKeyDisabledTime:   map[int]int64{1: 1234},
		},
	}
	require.NoError(t, DB.Create(&channel).Error)

	// The channel is still enabled (key-a works), so this must not short-circuit.
	changed := UpdateChannelStatus(channel.Id, "key-b", common.ChannelStatusEnabled, "")
	require.True(t, changed, "re-enabling a key must not be skipped by the channel-status check")

	var stored Channel
	require.NoError(t, DB.First(&stored, channel.Id).Error)
	assert.Equal(t, common.ChannelStatusEnabled, stored.Status)
	_, stillDisabled := stored.ChannelInfo.MultiKeyStatusList[1]
	assert.False(t, stillDisabled, "the recovered key must be cleared from the disabled map")
	assert.Empty(t, stored.ChannelInfo.MultiKeyDisabledReason[1])
	assert.Zero(t, stored.ChannelInfo.MultiKeyDisabledTime[1])
}

// Disabling the last usable key must still auto-disable the whole channel, so the
// batch test's per-key disables keep surfacing an unusable channel.
func TestUpdateChannelStatusDisablesChannelWhenLastKeyGoes(t *testing.T) {
	setupChannelStatusTest(t)

	channel := Channel{
		Name:   "multi-key-last",
		Key:    "key-a\nkey-b",
		Status: common.ChannelStatusEnabled,
		ChannelInfo: ChannelInfo{
			IsMultiKey:         true,
			MultiKeySize:       2,
			MultiKeyMode:       constant.MultiKeyModePolling,
			MultiKeyStatusList: map[int]int{0: common.ChannelStatusAutoDisabled},
		},
	}
	require.NoError(t, DB.Create(&channel).Error)

	changed := UpdateChannelStatus(channel.Id, "key-b", common.ChannelStatusAutoDisabled, "invalid api key")
	require.True(t, changed)

	var stored Channel
	require.NoError(t, DB.First(&stored, channel.Id).Error)
	assert.Equal(t, common.ChannelStatusAutoDisabled, stored.Status,
		"disabling the last usable key must take the channel out of service")
	assert.Equal(t, common.ChannelStatusAutoDisabled, stored.ChannelInfo.MultiKeyStatusList[1])
}

func TestSaveStatusStateFromSingleKeySnapshotPreservesUnownedColumns(t *testing.T) {
	setupChannelStatusTest(t)

	channel := Channel{
		Name:        "single-key-status",
		Key:         "original-key",
		Status:      common.ChannelStatusEnabled,
		Models:      "original-model",
		Group:       "default",
		UsedQuota:   100,
		ChannelInfo: ChannelInfo{},
	}
	require.NoError(t, DB.Create(&channel).Error)

	stale, err := GetChannelById(channel.Id, true)
	require.NoError(t, err)

	concurrentChannelInfo := ChannelInfo{
		IsMultiKey:           true,
		MultiKeySize:         2,
		MultiKeyMode:         constant.MultiKeyModePolling,
		MultiKeyPollingIndex: 1,
	}
	require.NoError(t, DB.Model(&Channel{}).Where("id = ?", channel.Id).Updates(map[string]any{
		"key":          "rotated-key",
		"used_quota":   gorm.Expr("used_quota + ?", 250),
		"models":       "concurrent-model",
		"channel_info": concurrentChannelInfo,
	}).Error)

	stale.Status = common.ChannelStatusManuallyDisabled
	stale.SetOtherInfo(map[string]any{
		"status_reason": "manual operation",
		"status_time":   int64(1234),
	})
	require.NoError(t, stale.saveStatusState())

	var stored Channel
	require.NoError(t, DB.First(&stored, channel.Id).Error)
	assert.Equal(t, common.ChannelStatusManuallyDisabled, stored.Status)
	assert.Equal(t, "rotated-key", stored.Key)
	assert.Equal(t, int64(350), stored.UsedQuota)
	assert.Equal(t, "concurrent-model", stored.Models)
	assert.Equal(t, concurrentChannelInfo, stored.ChannelInfo)

	otherInfo := stored.GetOtherInfo()
	assert.Equal(t, "manual operation", otherInfo["status_reason"])
	assert.Equal(t, float64(1234), otherInfo["status_time"])
}
