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

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// seedModelQuotaSubscription creates a plan with per-model buckets and an
// active subscription instance for the user.
func seedModelQuotaSubscription(t *testing.T, plan *SubscriptionPlan, sub *UserSubscription) {
	t.Helper()
	require.NoError(t, DB.Create(plan).Error)
	require.NoError(t, DB.Create(sub).Error)
}

func TestPreConsumeUserSubscriptionModelBucketsAreNotFungible(t *testing.T) {
	truncateTables(t)

	plan := &SubscriptionPlan{
		Id:          9301,
		Title:       "Model Buckets",
		PriceAmount: 10,
		// Pool size is intentionally large: with model_quotas set the shared
		// pool must not be consulted for funding.
		TotalAmount: 1 << 40,
		ModelQuotas: `{"model-a": 100, "model-b": 200}`,
	}
	now := GetDBTimestamp()
	sub := &UserSubscription{
		Id:          9401,
		UserId:      201,
		PlanId:      plan.Id,
		AmountTotal: plan.TotalAmount,
		StartTime:   now - 3600,
		EndTime:     now + 30*24*3600,
		Status:      "active",
		ModelUsed:   "{}",
	}
	seedModelQuotaSubscription(t, plan, sub)

	// Consume most of model-a's bucket.
	res, err := PreConsumeUserSubscription("req-a1", 201, "model-a", 0, 60, "", 0)
	require.NoError(t, err)
	assert.Equal(t, "model-a", res.ModelName)

	// model-a bucket has 40 left: another 50 must fail even though the shared
	// pool and model-b's bucket have plenty.
	_, err = PreConsumeUserSubscription("req-a2", 201, "model-a", 0, 50, "", 0)
	require.Error(t, err)

	// model-b's bucket is unaffected by model-a consumption.
	resB, err := PreConsumeUserSubscription("req-b1", 201, "model-b", 0, 50, "", 0)
	require.NoError(t, err)
	assert.Equal(t, "model-b", resB.ModelName)

	// A model without a bucket cannot draw from any bucket or the pool.
	_, err = PreConsumeUserSubscription("req-c1", 201, "model-c", 0, 1, "", 0)
	require.Error(t, err)

	var fresh UserSubscription
	require.NoError(t, DB.First(&fresh, sub.Id).Error)
	used := fresh.ParseModelUsed()
	assert.Equal(t, int64(60), used["model-a"])
	assert.Equal(t, int64(50), used["model-b"])
	// The shared pool must stay untouched when model buckets govern funding.
	assert.Zero(t, fresh.AmountUsed)

	// Refund routes back to the originating model bucket.
	require.NoError(t, RefundSubscriptionPreConsume("req-a1"))
	require.NoError(t, DB.First(&fresh, sub.Id).Error)
	used = fresh.ParseModelUsed()
	assert.Equal(t, int64(0), used["model-a"])
	assert.Equal(t, int64(50), used["model-b"])
}

func TestPreConsumeUserSubscriptionModelBucketSettleRespectsBucketCap(t *testing.T) {
	truncateTables(t)

	plan := &SubscriptionPlan{
		Id:          9302,
		Title:       "Bucket Cap",
		PriceAmount: 5,
		ModelQuotas: `{"model-a": 100}`,
	}
	now := GetDBTimestamp()
	sub := &UserSubscription{
		Id:          9402,
		UserId:      202,
		PlanId:      plan.Id,
		AmountTotal: 0,
		StartTime:   now - 3600,
		EndTime:     now + 30*24*3600,
		Status:      "active",
		ModelUsed:   `{"model-a": 90}`,
	}
	seedModelQuotaSubscription(t, plan, sub)

	// Settling up to the bucket cap is allowed.
	require.NoError(t, PostConsumeUserSubscriptionModelDelta(sub.Id, "model-a", 10))
	// Exceeding the bucket cap must fail, not wrap or spill into the pool.
	require.Error(t, PostConsumeUserSubscriptionModelDelta(sub.Id, "model-a", 1))
	// Refunding below zero floors at zero.
	require.NoError(t, PostConsumeUserSubscriptionModelDelta(sub.Id, "model-a", -200))

	var fresh UserSubscription
	require.NoError(t, DB.First(&fresh, sub.Id).Error)
	assert.Equal(t, int64(0), fresh.ParseModelUsed()["model-a"])
	// No bucket exists for model-b.
	require.Error(t, PostConsumeUserSubscriptionModelDelta(sub.Id, "model-b", 1))
}

func TestPreConsumeUserSubscriptionUsableGroupsRestrictFunding(t *testing.T) {
	truncateTables(t)

	plan := &SubscriptionPlan{
		Id:           9303,
		Title:        "Group Locked",
		PriceAmount:  5,
		TotalAmount:  1000,
		UsableGroups: "vip, svip,vip",
	}
	now := GetDBTimestamp()
	sub := &UserSubscription{
		Id:          9403,
		UserId:      203,
		PlanId:      plan.Id,
		AmountTotal: 1000,
		StartTime:   now - 3600,
		EndTime:     now + 30*24*3600,
		Status:      "active",
		ModelUsed:   "{}",
	}
	seedModelQuotaSubscription(t, plan, sub)

	// The default group is not in usable_groups: funding must be refused.
	_, err := PreConsumeUserSubscription("req-g1", 203, "gpt-x", 0, 10, "default", 0)
	require.Error(t, err)

	// Allowed groups fund normally; the normalized group list dedupes.
	res, err := PreConsumeUserSubscription("req-g2", 203, "gpt-x", 0, 10, "vip", 0)
	require.NoError(t, err)
	assert.Empty(t, res.ModelName)

	res, err = PreConsumeUserSubscription("req-g3", 203, "gpt-x", 0, 10, "svip", 0)
	require.NoError(t, err)
	assert.Empty(t, res.ModelName)

	// An empty request group keeps the legacy allow-all behavior.
	res, err = PreConsumeUserSubscription("req-g4", 203, "gpt-x", 0, 10, "", 0)
	require.NoError(t, err)
	assert.Empty(t, res.ModelName)
}

func TestPreConsumeUserSubscriptionPoolWithoutModelQuotasUnchanged(t *testing.T) {
	truncateTables(t)

	plan := &SubscriptionPlan{
		Id:          9304,
		Title:       "Plain Pool",
		PriceAmount: 5,
		TotalAmount: 100,
	}
	now := GetDBTimestamp()
	sub := &UserSubscription{
		Id:          9404,
		UserId:      204,
		PlanId:      plan.Id,
		AmountTotal: 100,
		AmountUsed:  90,
		StartTime:   now - 3600,
		EndTime:     now + 30*24*3600,
		Status:      "active",
	}
	seedModelQuotaSubscription(t, plan, sub)

	// Only 10 remains in the shared pool: 11 must fail regardless of group.
	_, err := PreConsumeUserSubscription("req-p1", 204, "any-model", 0, 11, "any-group", 0)
	require.Error(t, err)

	res, err := PreConsumeUserSubscription("req-p2", 204, "any-model", 0, 10, "", 0)
	require.NoError(t, err)
	assert.Empty(t, res.ModelName)

	var fresh UserSubscription
	require.NoError(t, DB.First(&fresh, sub.Id).Error)
	assert.Equal(t, int64(100), fresh.AmountUsed)
	assert.Empty(t, fresh.ParseModelUsed())
}

func TestPreConsumeUserSubscriptionTokenBucketLifecycle(t *testing.T) {
	truncateTables(t)

	plan := &SubscriptionPlan{
		Id:               9305,
		Title:            "Token Buckets",
		PriceAmount:      2,
		TotalAmount:      -1, // no shared pool: token buckets only
		ModelTokenQuotas: `{"deepseek-chat": 1000, "gpt-x": 500}`,
	}
	now := GetDBTimestamp()
	sub := &UserSubscription{
		Id:          9405,
		UserId:      205,
		PlanId:      plan.Id,
		AmountTotal: plan.TotalAmount,
		StartTime:   now - 3600,
		EndTime:     now + 30*24*3600,
		Status:      "active",
		ModelUsed:   "{}",
		TokenUsed:   "{}",
	}
	seedModelQuotaSubscription(t, plan, sub)

	// Pre-consume draws estimated tokens from the model's token bucket.
	res, err := PreConsumeUserSubscription("req-t1", 205, "deepseek-chat", 0, 9999, "", 600)
	require.NoError(t, err)
	assert.Equal(t, "token", res.Unit)
	assert.Equal(t, "deepseek-chat", res.ModelName)
	assert.Equal(t, int64(600), res.PreConsumed)

	// 400 tokens remain: another 500 must fail even though other buckets are full.
	_, err = PreConsumeUserSubscription("req-t2", 205, "deepseek-chat", 0, 9999, "", 500)
	require.Error(t, err)

	// Token buckets are not fungible across models.
	resG, err := PreConsumeUserSubscription("req-t3", 205, "gpt-x", 0, 9999, "", 400)
	require.NoError(t, err)
	assert.Equal(t, "token", resG.Unit)

	// A model without a token bucket cannot draw from the (disabled) pool.
	_, err = PreConsumeUserSubscription("req-t4", 205, "claude-y", 0, 1, "", 1)
	require.Error(t, err)

	var fresh UserSubscription
	require.NoError(t, DB.First(&fresh, sub.Id).Error)
	used := fresh.ParseTokenUsed()
	assert.Equal(t, int64(600), used["deepseek-chat"])
	assert.Equal(t, int64(400), used["gpt-x"])
	// The shared pool and quota buckets stay untouched for token funding.
	assert.Zero(t, fresh.AmountUsed)
	assert.Empty(t, fresh.ParseModelUsed())

	// Settle applies the actual-vs-estimated delta to the token bucket.
	require.NoError(t, PostConsumeUserSubscriptionTokenDelta(sub.Id, "deepseek-chat", -200))
	require.NoError(t, DB.First(&fresh, sub.Id).Error)
	assert.Equal(t, int64(400), fresh.ParseTokenUsed()["deepseek-chat"])

	// Settling past the bucket cap must fail, not spill into the pool.
	require.Error(t, PostConsumeUserSubscriptionTokenDelta(sub.Id, "deepseek-chat", 601))
	// No token bucket exists for this model.
	require.Error(t, PostConsumeUserSubscriptionTokenDelta(sub.Id, "claude-y", 1))

	// Refund routes back to the originating token bucket.
	require.NoError(t, RefundSubscriptionPreConsume("req-t1"))
	require.NoError(t, DB.First(&fresh, sub.Id).Error)
	assert.Equal(t, int64(0), fresh.ParseTokenUsed()["deepseek-chat"])
	assert.Equal(t, int64(400), fresh.ParseTokenUsed()["gpt-x"])
}

func TestPreConsumeUserSubscriptionTokenBucketBeatsQuotaBucket(t *testing.T) {
	truncateTables(t)

	plan := &SubscriptionPlan{
		Id:               9306,
		Title:            "Mixed Buckets",
		PriceAmount:      5,
		TotalAmount:      1 << 40,
		ModelQuotas:      `{"model-a": 100}`,
		ModelTokenQuotas: `{"model-a": 800}`,
	}
	now := GetDBTimestamp()
	sub := &UserSubscription{
		Id:          9406,
		UserId:      206,
		PlanId:      plan.Id,
		AmountTotal: plan.TotalAmount,
		StartTime:   now - 3600,
		EndTime:     now + 30*24*3600,
		Status:      "active",
		ModelUsed:   "{}",
		TokenUsed:   "{}",
	}
	seedModelQuotaSubscription(t, plan, sub)

	// When both bucket kinds cover the model, the token bucket is consumed first.
	res, err := PreConsumeUserSubscription("req-m1", 206, "model-a", 0, 50, "", 300)
	require.NoError(t, err)
	assert.Equal(t, "token", res.Unit)
	assert.Equal(t, int64(300), res.PreConsumed)

	var fresh UserSubscription
	require.NoError(t, DB.First(&fresh, sub.Id).Error)
	assert.Equal(t, int64(300), fresh.ParseTokenUsed()["model-a"])
	assert.Empty(t, fresh.ParseModelUsed())

	// Without a token estimate the draw falls back to the quota bucket.
	res, err = PreConsumeUserSubscription("req-m2", 206, "model-a", 0, 50, "", 0)
	require.NoError(t, err)
	assert.Equal(t, "quota", res.Unit)
	require.NoError(t, DB.First(&fresh, sub.Id).Error)
	assert.Equal(t, int64(50), fresh.ParseModelUsed()["model-a"])
}

func TestPreConsumeUserSubscriptionNoPoolRejectsUnbucketedModel(t *testing.T) {
	truncateTables(t)

	// TotalAmount == -1 with no buckets at all: the plan funds nothing.
	plan := &SubscriptionPlan{
		Id:          9307,
		Title:       "No Pool",
		PriceAmount: 5,
		TotalAmount: -1,
	}
	now := GetDBTimestamp()
	sub := &UserSubscription{
		Id:          9407,
		UserId:      207,
		PlanId:      plan.Id,
		AmountTotal: -1,
		StartTime:   now - 3600,
		EndTime:     now + 30*24*3600,
		Status:      "active",
	}
	seedModelQuotaSubscription(t, plan, sub)

	_, err := PreConsumeUserSubscription("req-n1", 207, "any-model", 0, 1, "", 0)
	require.Error(t, err)

	var fresh UserSubscription
	require.NoError(t, DB.First(&fresh, sub.Id).Error)
	assert.Zero(t, fresh.AmountUsed)
}
