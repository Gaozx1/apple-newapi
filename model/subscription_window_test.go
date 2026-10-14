/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
package model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// The sliding window keeps one bucket per period and drops the buckets that aged
// out, so usage expires exactly window-length after it was booked.
func TestSubscriptionWindowBucketsAgeOut(t *testing.T) {
	start := int64(1_700_000_000)
	state := parseSubscriptionWindowState("", subscriptionWindow5hBuckets).
		roll(start, SubscriptionWindow5hSeconds, subscriptionWindow5hBuckets)
	state.charge(100)
	assert.EqualValues(t, 100, state.used())
	assert.EqualValues(t, start+SubscriptionWindow5hSeconds/5, state.resetAt(SubscriptionWindow5hSeconds, subscriptionWindow5hBuckets))

	// One hour later the usage is still inside the window.
	next := state.roll(start+3600, SubscriptionWindow5hSeconds, subscriptionWindow5hBuckets)
	next.charge(50)
	assert.EqualValues(t, 150, next.used())

	// Four more hours drop the first bucket but keep the second.
	cursor := next.roll(start+5*3600, SubscriptionWindow5hSeconds, subscriptionWindow5hBuckets)
	assert.EqualValues(t, 50, cursor.used())

	// Two more hours and even that usage left the window.
	empty := cursor.roll(start+7*3600, SubscriptionWindow5hSeconds, subscriptionWindow5hBuckets)
	assert.EqualValues(t, 0, empty.used())

	// A jump far beyond the window clears everything at once.
	reset := next.roll(start+30*24*3600, SubscriptionWindow5hSeconds, subscriptionWindow5hBuckets)
	assert.EqualValues(t, 0, reset.used())
}

// A windows-only plan (no shared pool) funds requests while the windows have
// room, rejects the request that would exceed them, and frees room again after a
// refund.
func TestPreConsumeUserSubscriptionWindowLimitBlocksAndRecovers(t *testing.T) {
	truncateTables(t)

	plan := &SubscriptionPlan{
		Id:             9311,
		Title:          "5h Window",
		PriceAmount:    5,
		TotalAmount:    -1,
		Window5hAmount: 1000,
	}
	now := GetDBTimestamp()
	sub := &UserSubscription{
		Id:        9411,
		UserId:    211,
		PlanId:    plan.Id,
		StartTime: now - 60,
		EndTime:   now + 30*24*3600,
		Status:    "active",
	}
	require.NoError(t, DB.Create(plan).Error)
	require.NoError(t, DB.Create(sub).Error)

	res, err := PreConsumeUserSubscription("win-1", sub.UserId, "gpt-x", 0, 600, "default", 0)
	require.NoError(t, err)
	assert.EqualValues(t, 600, res.PreConsumed)

	// 600 + 500 exceeds the 5 hour window even though the first draw succeeded.
	_, err = PreConsumeUserSubscription("win-2", sub.UserId, "gpt-x", 0, 500, "default", 0)
	require.Error(t, err)

	summary := loadSubscriptionSummary(t, sub.UserId)
	require.NotNil(t, summary.Window5h)
	assert.EqualValues(t, 1000, summary.Window5h.Limit)
	assert.EqualValues(t, 600, summary.Window5h.Used)
	assert.EqualValues(t, 400, summary.Window5h.Remaining)
	assert.Greater(t, summary.Window5h.ResetAt, now)
	assert.Nil(t, summary.Window7d)

	require.NoError(t, RefundSubscriptionPreConsume("win-1"))
	summary = loadSubscriptionSummary(t, sub.UserId)
	require.NotNil(t, summary.Window5h)
	assert.EqualValues(t, 0, summary.Window5h.Used)

	_, err = PreConsumeUserSubscription("win-3", sub.UserId, "gpt-x", 0, 500, "default", 0)
	require.NoError(t, err)
}

// The window cap applies on top of a shared pool: a plan with plenty of pool
// quota still rejects the request that would cross the window limit.
func TestPreConsumeUserSubscriptionWindowCapsSharedPool(t *testing.T) {
	truncateTables(t)

	plan := &SubscriptionPlan{
		Id:             9312,
		Title:          "Pooled With Window",
		PriceAmount:    20,
		TotalAmount:    1_000_000,
		Window5hAmount: 100,
	}
	now := GetDBTimestamp()
	sub := &UserSubscription{
		Id:          9412,
		UserId:      212,
		PlanId:      plan.Id,
		AmountTotal: plan.TotalAmount,
		StartTime:   now - 60,
		EndTime:     now + 30*24*3600,
		Status:      "active",
	}
	require.NoError(t, DB.Create(plan).Error)
	require.NoError(t, DB.Create(sub).Error)

	_, err := PreConsumeUserSubscription("cap-1", sub.UserId, "gpt-x", 0, 80, "default", 0)
	require.NoError(t, err)
	_, err = PreConsumeUserSubscription("cap-2", sub.UserId, "gpt-x", 0, 30, "default", 0)
	require.Error(t, err)

	// Settling the first request with its real cost moves the window too.
	require.NoError(t, PostConsumeUserSubscriptionDelta(sub.Id, 15))
	summary := loadSubscriptionSummary(t, sub.UserId)
	require.NotNil(t, summary.Window5h)
	assert.EqualValues(t, 95, summary.Window5h.Used)
}

// Token-denominated buckets are not quota denominated, so their refund path has
// to restore the sliding window explicitly.
func TestRefundSubscriptionPreConsumeRestoresWindowForTokenBuckets(t *testing.T) {
	truncateTables(t)

	plan := &SubscriptionPlan{
		Id:               9313,
		Title:            "Token Bucket With Window",
		PriceAmount:      5,
		TotalAmount:      -1,
		ModelTokenQuotas: `{"model-a": 1000000}`,
		Window5hAmount:   1000,
	}
	now := GetDBTimestamp()
	sub := &UserSubscription{
		Id:        9413,
		UserId:    213,
		PlanId:    plan.Id,
		StartTime: now - 60,
		EndTime:   now + 30*24*3600,
		Status:    "active",
	}
	require.NoError(t, DB.Create(plan).Error)
	require.NoError(t, DB.Create(sub).Error)

	res, err := PreConsumeUserSubscription("tok-win-1", sub.UserId, "model-a", 0, 400, "default", 50_000)
	require.NoError(t, err)
	assert.Equal(t, "token", res.Unit)
	assert.EqualValues(t, 50_000, res.PreConsumed)

	summary := loadSubscriptionSummary(t, sub.UserId)
	require.NotNil(t, summary.Window5h)
	assert.EqualValues(t, 400, summary.Window5h.Used)

	require.NoError(t, RefundSubscriptionPreConsume("tok-win-1"))
	summary = loadSubscriptionSummary(t, sub.UserId)
	require.NotNil(t, summary.Window5h)
	assert.EqualValues(t, 0, summary.Window5h.Used)
}

// Admin/plan resets clear the window usage along with the other counters.
func TestResetUserSubscriptionClearsSlidingWindows(t *testing.T) {
	truncateTables(t)

	plan := &SubscriptionPlan{
		Id:             9314,
		Title:          "Window Reset",
		PriceAmount:    5,
		TotalAmount:    1000,
		Window7dAmount: 1000,
	}
	now := GetDBTimestamp()
	sub := &UserSubscription{
		Id:          9414,
		UserId:      214,
		PlanId:      plan.Id,
		AmountTotal: plan.TotalAmount,
		StartTime:   now - 60,
		EndTime:     now + 30*24*3600,
		Status:      "active",
	}
	require.NoError(t, DB.Create(plan).Error)
	require.NoError(t, DB.Create(sub).Error)

	_, err := PreConsumeUserSubscription("reset-1", sub.UserId, "gpt-x", 0, 200, "default", 0)
	require.NoError(t, err)

	resetNow := GetDBTimestamp()
	require.NoError(t, DB.Transaction(func(tx *gorm.DB) error {
		var locked UserSubscription
		if err := tx.Where("id = ?", sub.Id).First(&locked).Error; err != nil {
			return err
		}
		return resetUserSubscriptionTx(tx, &locked, plan, resetNow, false)
	}))

	var stored UserSubscription
	require.NoError(t, DB.Where("id = ?", sub.Id).First(&stored).Error)
	assert.Empty(t, stored.Window5hState)
	assert.Empty(t, stored.Window7dState)

	summary := loadSubscriptionSummary(t, sub.UserId)
	require.NotNil(t, summary.Window7d)
	assert.EqualValues(t, 0, summary.Window7d.Used)
}

func loadSubscriptionSummary(t *testing.T, userId int) SubscriptionSummary {
	t.Helper()
	summaries, err := GetAllUserSubscriptions(userId)
	require.NoError(t, err)
	require.Len(t, summaries, 1)
	return summaries[0]
}

// SQLite keeps the plan table in a hand-written DDL list instead of AutoMigrate,
// so a plan column added to the model must reach that list as well. Without it a
// SQLite deployment would silently lose the sliding-window limits.
func TestEnsureSubscriptionPlanTableSQLiteAddsWindowColumns(t *testing.T) {
	if !common.UsingMainDatabase(common.DatabaseTypeSQLite) {
		t.Skip("SQLite-only migration path")
	}
	truncateTables(t)

	require.NoError(t, DB.Create(&SubscriptionPlan{
		Id:            9321,
		Title:         "sqlite-upgrade",
		PriceAmount:   1,
		DurationUnit:  SubscriptionDurationMonth,
		DurationValue: 1,
		TotalAmount:   5000,
	}).Error)

	// A plan table written by the previous release has no window columns.
	for _, column := range []string{"window_5h_amount", "window_7d_amount"} {
		require.NoError(t, DB.Exec("ALTER TABLE ? DROP COLUMN ?",
			clause.Table{Name: "subscription_plans"},
			clause.Column{Name: column},
		).Error)
	}
	require.False(t, DB.Migrator().HasColumn(&SubscriptionPlan{}, "window_5h_amount"))

	require.NoError(t, ensureSubscriptionPlanTableSQLite())
	assert.True(t, DB.Migrator().HasColumn(&SubscriptionPlan{}, "window_5h_amount"))
	assert.True(t, DB.Migrator().HasColumn(&SubscriptionPlan{}, "window_7d_amount"))

	var stored SubscriptionPlan
	require.NoError(t, DB.Where("id = ?", 9321).First(&stored).Error)
	assert.Equal(t, "sqlite-upgrade", stored.Title)
	assert.EqualValues(t, 5000, stored.TotalAmount)
	assert.Zero(t, stored.Window5hAmount)

	// Running the startup migration again must be a no-op.
	require.NoError(t, ensureSubscriptionPlanTableSQLite())
}
