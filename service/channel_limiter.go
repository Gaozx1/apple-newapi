package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strconv"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/relaykit/dto"

	"github.com/gin-gonic/gin"
)

var errChannelSaturated = errors.New("channel credential admission limit reached")

// MaxChannelAdmissionReselects bounds how many times one request re-selects a
// channel after losing the admission race for the candidate it picked, so a
// burst that fills every candidate's budget still fails fast with a local 429
// instead of looping over the pool.
const MaxChannelAdmissionReselects = 3

// ChannelSlot represents one relay attempt admitted through a channel
// credential's concurrency and per-minute request limits. Release returns the
// concurrency token; it must be called exactly once when the attempt finishes
// and is safe to call more than once.
type ChannelSlot struct {
	ChannelId int
	release   func()
}

// SetChannelSlot stores the slot held by the current relay attempt. A request
// never holds two slots: the selection wrappers release any previous slot before
// acquiring a new one.
func SetChannelSlot(c *gin.Context, slot *ChannelSlot) {
	common.SetContextKey(c, constant.ContextKeyChannelSlot, slot)
}

// TakeChannelSlot pops the current slot from the context, transferring the
// release responsibility to the caller.
func TakeChannelSlot(c *gin.Context) *ChannelSlot {
	slot, ok := common.GetContextKeyType[*ChannelSlot](c, constant.ContextKeyChannelSlot)
	if ok {
		common.SetContextKey(c, constant.ContextKeyChannelSlot, (*ChannelSlot)(nil))
	}
	return slot
}

// ReleaseChannelSlot releases the slot held by the current attempt, if any.
// Safe to call repeatedly.
func ReleaseChannelSlot(c *gin.Context) {
	if slot := TakeChannelSlot(c); slot != nil {
		slot.Release()
	}
}

func (s *ChannelSlot) Release() {
	if s != nil && s.release != nil {
		s.release()
	}
}

// channelCredentialIdentity derives the accounting key for one credential of a
// channel. The credential is hashed so a reordered key list keeps every key's
// budget and no secret is retained in memory or in a Redis key.
func channelCredentialIdentity(channelId int, credential string) string {
	sum := sha256.Sum256([]byte(credential))
	return strconv.Itoa(channelId) + ":" + hex.EncodeToString(sum[:8])
}

// channelLimits reads the configured admission limits. Both count per
// credential, so a multi-key channel applies the same number to every key
// independently. Values are clamped defensively: the concurrency limit sizes an
// in-memory semaphore, and a stored setting may predate validation.
func channelLimits(channel *model.Channel) (concurrency int, rpm int) {
	settings := channel.GetOtherSettings()
	return min(max(settings.ConcurrencyLimit, 0), dto.MaxChannelAdmissionLimit),
		min(max(settings.RpmLimit, 0), dto.MaxChannelAdmissionLimit)
}

// channelKeyEnabled reports whether one credential of the channel is usable.
// It mirrors GetNextEnabledKey's default: a key with no recorded status counts
// as enabled.
func channelKeyEnabled(channel *model.Channel, index int) bool {
	if !channel.ChannelInfo.IsMultiKey {
		return index == 0
	}
	if status, ok := channel.ChannelInfo.MultiKeyStatusList[index]; ok {
		return status == common.ChannelStatusEnabled
	}
	return true
}

// ChannelPoolHasCapacity reports whether the channel still has a credential with
// admission capacity. Selection uses it to skip a channel whose whole credential
// pool is saturated instead of picking it and failing afterwards.
func ChannelPoolHasCapacity(channel *model.Channel) bool {
	concurrency, rpm := channelLimits(channel)
	if concurrency <= 0 && rpm <= 0 {
		return true
	}
	keys := channel.GetKeys()
	if len(keys) == 0 {
		return true
	}
	for index, key := range keys {
		if !channelKeyEnabled(channel, index) {
			continue
		}
		if hasChannelCapacity(channel.Id, key, concurrency, rpm) {
			return true
		}
	}
	return false
}

// AdmitChannelCredential admits one credential of the channel through its
// concurrency and per-minute limits and stores the resulting slot on the
// request. It starts with the credential the caller already resolved and falls
// back to the channel's other enabled credentials, so one saturated key does not
// make the whole channel unusable. The returned credential and index replace the
// caller's selection.
func AdmitChannelCredential(c *gin.Context, channel *model.Channel, key string, index int) (string, int, error) {
	concurrency, rpm := channelLimits(channel)
	if concurrency <= 0 && rpm <= 0 {
		return key, index, nil
	}
	if release, ok := acquireChannelCredential(channel.Id, key, concurrency, rpm); ok {
		SetChannelSlot(c, &ChannelSlot{ChannelId: channel.Id, release: release})
		return key, index, nil
	}
	if channel.ChannelInfo.IsMultiKey {
		keys := channel.GetKeys()
		for candidate := range keys {
			if candidate == index || !channelKeyEnabled(channel, candidate) {
				continue
			}
			release, ok := acquireChannelCredential(channel.Id, keys[candidate], concurrency, rpm)
			if ok {
				SetChannelSlot(c, &ChannelSlot{ChannelId: channel.Id, release: release})
				return keys[candidate], candidate, nil
			}
		}
	}
	return "", 0, fmt.Errorf("%w: channel #%d has no key below its limit", errChannelSaturated, channel.Id)
}

// IsChannelSaturatedError reports whether the attempt failed because every
// credential of the channel was at its admission limit. Callers treat it as a
// local admission decision: fail over to another channel without touching
// upstream, and never auto-disable the channel for it.
func IsChannelSaturatedError(err error) bool {
	return errors.Is(err, errChannelSaturated)
}

// AdmitLockedChannelCredential admits the credential a long-lived upstream
// connection is already authenticated with. Such a session dials upstream once
// per credential, so a saturated key cannot be swapped for another one of the
// same channel; the call is refused instead and the caller reports a local 429.
func AdmitLockedChannelCredential(c *gin.Context, channel *model.Channel, key string) error {
	concurrency, rpm := channelLimits(channel)
	if concurrency <= 0 && rpm <= 0 {
		return nil
	}
	release, ok := acquireChannelCredential(channel.Id, key, concurrency, rpm)
	if !ok {
		return fmt.Errorf("%w: channel #%d has no capacity for its bound credential", errChannelSaturated, channel.Id)
	}
	SetChannelSlot(c, &ChannelSlot{ChannelId: channel.Id, release: release})
	return nil
}

// acquireChannelCredential reserves one in-flight slot and one per-minute slot
// for the credential. The concurrency check runs first (side-effect free); the
// per-minute counter is only consumed by attempts that pass it, so a rejected
// attempt cannot burn the channel's request budget.
func acquireChannelCredential(channelId int, credential string, concurrency, rpm int) (func(), bool) {
	identity := channelCredentialIdentity(channelId, credential)
	release := func() {}
	if concurrency > 0 {
		releaseConcurrency, ok := acquireChannelConcurrency(identity, concurrency)
		if !ok {
			return nil, false
		}
		release = releaseConcurrency
	}
	if rpm > 0 && !allowChannelRpm(identity, rpm) {
		release()
		return nil, false
	}
	return release, true
}

// hasChannelCapacity answers whether the credential would be admitted, without
// consuming any budget.
func hasChannelCapacity(channelId int, credential string, concurrency, rpm int) bool {
	identity := channelCredentialIdentity(channelId, credential)
	if concurrency > 0 && channelConcurrencyInFlight(identity) >= concurrency {
		return false
	}
	if rpm > 0 && channelRpmUsed(identity) >= rpm {
		return false
	}
	return true
}

// channelConcurrency semaphores are per-process: the limit is enforced per node
// when new-api runs behind multiple instances. The release closure captures the
// exact semaphore it acquired from, so a limit change that swaps the entry never
// miscounts in-flight tokens.
var channelConcurrency = struct {
	sync.Mutex
	sems map[string]*channelSemaphoreEntry
}{sems: map[string]*channelSemaphoreEntry{}}

type channelSemaphoreEntry struct {
	limit int
	sem   chan struct{}
}

func acquireChannelConcurrency(identity string, limit int) (func(), bool) {
	channelConcurrency.Lock()
	entry := channelConcurrency.sems[identity]
	if entry == nil || entry.limit != limit {
		entry = &channelSemaphoreEntry{limit: limit, sem: make(chan struct{}, limit)}
		channelConcurrency.sems[identity] = entry
	}
	channelConcurrency.Unlock()

	select {
	case entry.sem <- struct{}{}:
		return func() { <-entry.sem }, true
	default:
		return nil, false
	}
}

func channelConcurrencyInFlight(identity string) int {
	channelConcurrency.Lock()
	defer channelConcurrency.Unlock()
	entry := channelConcurrency.sems[identity]
	if entry == nil {
		return 0
	}
	return len(entry.sem)
}

// allowChannelRpm consumes one slot of the credential's fixed 60-second request
// window. Redis keeps the window consistent across instances; without Redis a
// per-process window is used. Redis failures fail open so a cache hiccup never
// takes protective limits down into an outage.
func allowChannelRpm(identity string, limit int) bool {
	if common.RedisEnabled {
		return allowChannelRpmRedis(identity, limit)
	}
	return allowChannelRpmMemory(identity, limit)
}

func allowChannelRpmRedis(identity string, limit int) bool {
	key := fmt.Sprintf("channel_rpm:%s:%d", identity, time.Now().Unix()/60)
	ctx := context.Background()
	count, err := common.RDB.Incr(ctx, key).Result()
	if err != nil {
		common.SysLog(fmt.Sprintf("channel rpm check failed open: identity=%s, error=%v", identity, err))
		return true
	}
	if count == 1 {
		common.RDB.Expire(ctx, key, 2*time.Minute)
	}
	return count <= int64(limit)
}

type channelRpmWindow struct {
	minute int64
	count  int
}

var channelRpmWindows = struct {
	sync.Mutex
	windows map[string]*channelRpmWindow
}{windows: map[string]*channelRpmWindow{}}

func allowChannelRpmMemory(identity string, limit int) bool {
	minute := time.Now().Unix() / 60
	channelRpmWindows.Lock()
	defer channelRpmWindows.Unlock()
	window := channelRpmWindows.windows[identity]
	if window == nil || window.minute != minute {
		window = &channelRpmWindow{minute: minute}
		channelRpmWindows.windows[identity] = window
	}
	if window.count >= limit {
		return false
	}
	window.count++
	return true
}

// channelRpmUsed reports the requests already counted in the credential's
// current window, for the capacity peek only.
func channelRpmUsed(identity string) int {
	if common.RedisEnabled {
		key := fmt.Sprintf("channel_rpm:%s:%d", identity, time.Now().Unix()/60)
		count, err := common.RDB.Get(context.Background(), key).Int()
		if err != nil {
			return 0
		}
		return count
	}
	minute := time.Now().Unix() / 60
	channelRpmWindows.Lock()
	defer channelRpmWindows.Unlock()
	window := channelRpmWindows.windows[identity]
	if window == nil || window.minute != minute {
		return 0
	}
	return window.count
}
