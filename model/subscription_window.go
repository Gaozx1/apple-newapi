package model

import (
	"math"
	"strings"

	"github.com/QuantumNous/new-api/common"
)

// Sliding-window subscription limits
//
// A plan may cap the cost consumed inside a rolling 5 hour and a rolling 7 day
// window (OpenAI-style "5h limit / 7d limit"). Both limits are denominated in
// quota units and are tracked per user subscription, so every funding source of
// that subscription — shared pool, per-model quota buckets and per-model token
// buckets — counts against them.
//
// The usage is kept as a small ring of time buckets (one per hour for the 5 hour
// window, one per day for the 7 day window). Rolling the ring discards the
// buckets that aged out, and the sum of the remaining buckets approximates the
// window usage from above: the current, partially elapsed bucket counts in full.
// That is deliberately conservative — a request is only rejected once the last
// N whole buckets already exhaust the limit — and it keeps the hot path at O(1)
// with no per-request history table.
const (
	// SubscriptionWindow5hSeconds is the length of the short sliding window.
	SubscriptionWindow5hSeconds int64 = 5 * 3600
	// SubscriptionWindow7dSeconds is the length of the long sliding window.
	SubscriptionWindow7dSeconds int64 = 7 * 24 * 3600

	subscriptionWindow5hBuckets = 5
	subscriptionWindow7dBuckets = 7
)

// SubscriptionWindowUsage reports one sliding window for display.
type SubscriptionWindowUsage struct {
	// Limit is the configured quota cap; 0 means the window is disabled.
	Limit int64 `json:"limit"`
	// Used is the quota consumed inside the window (upper bound, see above).
	Used int64 `json:"used"`
	// Remaining is Limit - Used, never negative.
	Remaining int64 `json:"remaining"`
	// ResetAt is the unix time when the oldest bucket leaves the window.
	ResetAt int64 `json:"reset_at"`
}

// subscriptionWindowState is the persisted bucket ring of one window.
type subscriptionWindowState struct {
	// Anchor is the unix time the current bucket started at.
	Anchor int64 `json:"anchor"`
	// Buckets is ordered oldest to newest; the newest bucket is the live one.
	Buckets []int64 `json:"buckets"`
}

func parseSubscriptionWindowState(raw string, buckets int) subscriptionWindowState {
	state := subscriptionWindowState{Buckets: make([]int64, buckets)}
	if strings.TrimSpace(raw) == "" {
		return state
	}
	var stored subscriptionWindowState
	if err := common.UnmarshalJsonStr(raw, &stored); err != nil {
		return state
	}
	if len(stored.Buckets) == buckets {
		copy(state.Buckets, stored.Buckets)
	}
	state.Anchor = stored.Anchor
	return state
}

func (s subscriptionWindowState) marshal() string {
	data, err := common.Marshal(s)
	if err != nil {
		return ""
	}
	return string(data)
}

// used is a saturating sum: an over-large configured limit must never wrap into
// a negative usage that would let an oversized request through.
func (s subscriptionWindowState) used() int64 {
	var total int64
	for _, bucket := range s.Buckets {
		if bucket <= 0 {
			continue
		}
		if total > math.MaxInt64-bucket {
			return math.MaxInt64
		}
		total += bucket
	}
	return total
}

// roll advances the ring to the bucket containing now, dropping the buckets that
// aged out of the window. A window that was never used starts at now.
func (s subscriptionWindowState) roll(now, windowSeconds int64, buckets int) subscriptionWindowState {
	if windowSeconds <= 0 || buckets <= 0 {
		return s
	}
	period := windowSeconds / int64(buckets)
	if period <= 0 {
		return s
	}
	if s.Anchor <= 0 {
		s.Anchor = now
		return s
	}
	elapsed := (now - s.Anchor) / period
	if elapsed <= 0 {
		return s
	}
	if elapsed >= int64(buckets) {
		s.Buckets = make([]int64, buckets)
		s.Anchor += elapsed * period
		return s
	}
	s.Buckets = append(s.Buckets[elapsed:], make([]int64, elapsed)...)
	s.Anchor += elapsed * period
	return s
}

func (s *subscriptionWindowState) charge(amount int64) {
	if amount <= 0 || len(s.Buckets) == 0 {
		return
	}
	s.Buckets[len(s.Buckets)-1] += amount
}

// refund removes amount from the live bucket. A refund of usage recorded in an
// older bucket therefore only frees the current bucket, and the counter never
// drops below zero.
func (s *subscriptionWindowState) refund(amount int64) {
	if amount <= 0 || len(s.Buckets) == 0 {
		return
	}
	live := &s.Buckets[len(s.Buckets)-1]
	*live -= amount
	if *live < 0 {
		*live = 0
	}
}

// resetAt is when the current bucket (and with it the oldest usage) expires.
func (s subscriptionWindowState) resetAt(windowSeconds int64, buckets int) int64 {
	if windowSeconds <= 0 || buckets <= 0 || s.Anchor <= 0 {
		return 0
	}
	return s.Anchor + windowSeconds/int64(buckets)
}

// chargePlanSubscriptionWindows rolls both windows of the subscription and books
// amount against them. It reports false when a window has no room left, in which
// case the caller must not fund the request from this subscription. Windows that
// the plan disabled stay untouched, so their state remains empty.
func chargePlanSubscriptionWindows(plan *SubscriptionPlan, sub *UserSubscription, now, amount int64) (string, string, bool) {
	if plan == nil || sub == nil || amount <= 0 {
		return sub.window5hState(), sub.window7dState(), true
	}

	fiveHour := sub.window5hState()
	if plan.Window5hAmount > 0 {
		state := parseSubscriptionWindowState(fiveHour, subscriptionWindow5hBuckets).
			roll(now, SubscriptionWindow5hSeconds, subscriptionWindow5hBuckets)
		// Compared without adding the two sides, so an oversized request fails
		// closed instead of wrapping.
		if amount > plan.Window5hAmount || state.used() > plan.Window5hAmount-amount {
			return fiveHour, sub.window7dState(), false
		}
		state.charge(amount)
		fiveHour = state.marshal()
	}

	sevenDay := sub.window7dState()
	if plan.Window7dAmount > 0 {
		state := parseSubscriptionWindowState(sevenDay, subscriptionWindow7dBuckets).
			roll(now, SubscriptionWindow7dSeconds, subscriptionWindow7dBuckets)
		if amount > plan.Window7dAmount || state.used() > plan.Window7dAmount-amount {
			return fiveHour, sevenDay, false
		}
		state.charge(amount)
		sevenDay = state.marshal()
	}

	return fiveHour, sevenDay, true
}

// applySubscriptionWindowDelta books a settlement delta (positive consume, negative
// refund) against one window's stored state. A window that was never charged
// stays empty so disabled windows do not start tracking usage.
func applySubscriptionWindowDelta(raw string, buckets int, windowSeconds, now, delta int64) string {
	if delta == 0 || strings.TrimSpace(raw) == "" {
		return raw
	}
	state := parseSubscriptionWindowState(raw, buckets).roll(now, windowSeconds, buckets)
	if delta > 0 {
		state.charge(delta)
	} else {
		state.refund(-delta)
	}
	return state.marshal()
}

func (s *UserSubscription) window5hState() string {
	if s == nil {
		return ""
	}
	return s.Window5hState
}

func (s *UserSubscription) window7dState() string {
	if s == nil {
		return ""
	}
	return s.Window7dState
}

// buildSubscriptionWindowUsage reports the current usage of one window without
// persisting the roll.
func buildSubscriptionWindowUsage(raw string, limit int64, buckets int, windowSeconds, now int64) *SubscriptionWindowUsage {
	if limit <= 0 {
		return nil
	}
	state := parseSubscriptionWindowState(raw, buckets).roll(now, windowSeconds, buckets)
	used := state.used()
	remaining := limit - used
	if remaining < 0 {
		remaining = 0
	}
	return &SubscriptionWindowUsage{
		Limit:     limit,
		Used:      used,
		Remaining: remaining,
		ResetAt:   state.resetAt(windowSeconds, buckets),
	}
}

// SubscriptionWindowUsages reports the sliding windows a plan configures for a
// subscription. Windows the plan does not limit are nil.
func SubscriptionWindowUsages(plan *SubscriptionPlan, sub *UserSubscription, now int64) (*SubscriptionWindowUsage, *SubscriptionWindowUsage) {
	if plan == nil {
		return nil, nil
	}
	return buildSubscriptionWindowUsage(sub.window5hState(), plan.Window5hAmount, subscriptionWindow5hBuckets, SubscriptionWindow5hSeconds, now),
		buildSubscriptionWindowUsage(sub.window7dState(), plan.Window7dAmount, subscriptionWindow7dBuckets, SubscriptionWindow7dSeconds, now)
}
