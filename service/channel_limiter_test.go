package service

import (
	"testing"

	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/relaykit/dto"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newLimitTestChannel(id int, otherSettings, keys string, multiKey bool) *model.Channel {
	channel := &model.Channel{Id: id, Key: keys, OtherSettings: otherSettings}
	if multiKey {
		channel.ChannelInfo.IsMultiKey = true
		channel.ChannelInfo.MultiKeySize = len(channel.GetKeys())
	}
	return channel
}

// A credential admits at most max_concurrency attempts at the same time; the
// slot returns only when the attempt releases it.
func TestAdmitChannelCredentialEnforcesConcurrency(t *testing.T) {
	c, _ := gin.CreateTestContext(nil)
	channel := newLimitTestChannel(991001, `{"concurrency_limit":2}`, "sk-concurrency", false)

	_, _, err := AdmitChannelCredential(c, channel, "sk-concurrency", 0)
	require.NoError(t, err)
	_, _, err = AdmitChannelCredential(c, channel, "sk-concurrency", 0)
	require.NoError(t, err)

	// The third attempt is rejected while the first two hold their slots.
	_, _, err = AdmitChannelCredential(c, channel, "sk-concurrency", 0)
	require.Error(t, err)
	assert.True(t, IsChannelSaturatedError(err))

	// Releasing one slot admits the next attempt.
	ReleaseChannelSlot(c)
	_, _, err = AdmitChannelCredential(c, channel, "sk-concurrency", 0)
	require.NoError(t, err)
	ReleaseChannelSlot(c)
}

// The per-minute limit counts admitted attempts inside the current window and
// does not depend on how long they run.
func TestAdmitChannelCredentialEnforcesPerMinuteLimit(t *testing.T) {
	c, _ := gin.CreateTestContext(nil)
	channel := newLimitTestChannel(991002, `{"rpm_limit":2}`, "sk-rpm", false)

	for range 2 {
		_, _, err := AdmitChannelCredential(c, channel, "sk-rpm", 0)
		require.NoError(t, err)
		ReleaseChannelSlot(c)
	}
	_, _, err := AdmitChannelCredential(c, channel, "sk-rpm", 0)
	require.Error(t, err)
	assert.True(t, IsChannelSaturatedError(err))
}

// The limit belongs to the credential, so one saturated key of a multi-key
// channel must not consume another key's budget.
func TestAdmitChannelCredentialKeepsBudgetsPerKey(t *testing.T) {
	c, _ := gin.CreateTestContext(nil)
	channel := newLimitTestChannel(991003, `{"concurrency_limit":1}`, "sk-key-a\nsk-key-b", true)

	_, _, err := AdmitChannelCredential(c, channel, "sk-key-a", 0)
	require.NoError(t, err)

	// key-a is full, but key-b has its own independent budget.
	_, _, err = AdmitChannelCredential(c, channel, "sk-key-b", 1)
	require.NoError(t, err)
	ReleaseChannelSlot(c)

	// A saturated key-a falls back to another enabled key of the same channel.
	key, index, err := AdmitChannelCredential(c, channel, "sk-key-a", 0)
	require.NoError(t, err)
	assert.Equal(t, "sk-key-b", key)
	assert.Equal(t, 1, index)
	ReleaseChannelSlot(c)
}

// When every enabled key is saturated the attempt is refused locally, so the
// caller can fail over instead of sending a request that upstream would reject.
func TestAdmitChannelCredentialRejectsWhenEveryKeySaturated(t *testing.T) {
	c, _ := gin.CreateTestContext(nil)
	channel := newLimitTestChannel(991004, `{"concurrency_limit":1}`, "sk-only\nsk-disabled", true)
	channel.ChannelInfo.MultiKeyStatusList = map[int]int{1: 2}

	_, _, err := AdmitChannelCredential(c, channel, "sk-only", 0)
	require.NoError(t, err)
	t.Cleanup(func() { ReleaseChannelSlot(c) })

	// The second key is disabled, so there is no fallback left.
	_, _, err = AdmitChannelCredential(c, channel, "sk-only", 0)
	require.Error(t, err)
	assert.True(t, IsChannelSaturatedError(err))
}

// A channel without configured limits never rejects and never allocates a slot.
func TestAdmitChannelCredentialUnlimitedIsNoOp(t *testing.T) {
	c, _ := gin.CreateTestContext(nil)
	channel := newLimitTestChannel(991005, ``, "sk-unlimited", false)

	for range 50 {
		_, _, err := AdmitChannelCredential(c, channel, "sk-unlimited", 0)
		require.NoError(t, err)
	}
	assert.Nil(t, TakeChannelSlot(c))
}

// Selection skips a channel whose whole credential pool is saturated.
func TestChannelPoolHasCapacity(t *testing.T) {
	c, _ := gin.CreateTestContext(nil)
	channel := newLimitTestChannel(991006, `{"concurrency_limit":1}`, "sk-pool-a\nsk-pool-b", true)

	assert.True(t, ChannelPoolHasCapacity(channel))

	_, _, err := AdmitChannelCredential(c, channel, "sk-pool-a", 0)
	require.NoError(t, err)
	// One key is still free, so the channel remains selectable.
	assert.True(t, ChannelPoolHasCapacity(channel))

	_, _, err = AdmitChannelCredential(c, channel, "sk-pool-b", 1)
	require.NoError(t, err)
	assert.False(t, ChannelPoolHasCapacity(channel))

	ReleaseChannelSlot(c)
	ReleaseChannelSlot(c)
	assert.True(t, ChannelPoolHasCapacity(channel))
}

// A slot is released exactly once even when several cleanup paths run.
func TestChannelSlotReleaseIsTakeOnce(t *testing.T) {
	c, _ := gin.CreateTestContext(nil)
	assert.Nil(t, TakeChannelSlot(c))

	released := 0
	SetChannelSlot(c, &ChannelSlot{ChannelId: 1, release: func() { released++ }})
	ReleaseChannelSlot(c)
	assert.Equal(t, 1, released)

	ReleaseChannelSlot(c)
	assert.Equal(t, 1, released)
	assert.Nil(t, TakeChannelSlot(c))
}

// A long-lived upstream connection is authenticated with one credential, so a
// saturated key cannot be swapped for another one of the same channel.
func TestAdmitLockedChannelCredentialRefusesInsteadOfFallingBack(t *testing.T) {
	c, _ := gin.CreateTestContext(nil)
	channel := newLimitTestChannel(991010, `{"concurrency_limit":1}`, "sk-locked\nsk-free", true)

	require.NoError(t, AdmitLockedChannelCredential(c, channel, "sk-locked"))
	require.ErrorIs(t, AdmitLockedChannelCredential(c, channel, "sk-locked"), errChannelSaturated)

	// The other key has budget, but the established connection cannot use it.
	assert.True(t, ChannelPoolHasCapacity(channel))
	ReleaseChannelSlot(c)
	require.NoError(t, AdmitLockedChannelCredential(c, channel, "sk-locked"))
	ReleaseChannelSlot(c)
}

// A stored setting may predate validation, so the limits are clamped before
// they size an in-memory semaphore or a counter comparison.
func TestChannelLimitsClampsStoredValues(t *testing.T) {
	concurrency, rpm := channelLimits(newLimitTestChannel(991007, ``, "", false))
	assert.Zero(t, concurrency)
	assert.Zero(t, rpm)

	concurrency, rpm = channelLimits(newLimitTestChannel(
		991008,
		`{"concurrency_limit":-5,"rpm_limit":99999999999}`,
		"sk-clamped",
		false,
	))
	assert.Zero(t, concurrency)
	assert.Equal(t, dto.MaxChannelAdmissionLimit, rpm)

	concurrency, rpm = channelLimits(newLimitTestChannel(
		991009,
		`{"concurrency_limit":3,"rpm_limit":120}`,
		"sk-in-range",
		false,
	))
	assert.Equal(t, 3, concurrency)
	assert.Equal(t, 120, rpm)
}
