package model

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResetCardPlanStoresTypeAndCount(t *testing.T) {
	truncateTables(t)

	plan := &SubscriptionPlan{
		Id:             9951,
		Title:          "Reset card pack",
		PriceAmount:    9.9,
		DurationUnit:   SubscriptionDurationDay,
		DurationValue:  30,
		PlanType:       SubscriptionPlanTypeResetCard,
		ResetCardCount: 5,
	}
	require.NoError(t, DB.Create(plan).Error)

	loaded, err := GetSubscriptionPlanById(plan.Id)
	require.NoError(t, err)
	assert.Equal(t, SubscriptionPlanTypeResetCard, loaded.PlanType)
	assert.True(t, loaded.IsResetCardPlan())
	assert.Equal(t, 5, loaded.ResetCardCount)
}

func TestResetCardPlanDefaultsToSubscriptionType(t *testing.T) {
	truncateTables(t)

	plan := &SubscriptionPlan{
		Id:            9952,
		Title:         "Regular plan",
		PriceAmount:   1,
		DurationUnit:  SubscriptionDurationMonth,
		DurationValue: 1,
	}
	require.NoError(t, DB.Create(plan).Error)

	loaded, err := GetSubscriptionPlanById(plan.Id)
	require.NoError(t, err)
	assert.Equal(t, SubscriptionPlanTypeSubscription, loaded.PlanType)
	assert.False(t, loaded.IsResetCardPlan())
}

func TestResetCardPlanExpiryNeverWhenDurationMissing(t *testing.T) {
	truncateTables(t)

	plan := &SubscriptionPlan{
		Id:             9953,
		Title:          "Permanent cards",
		PriceAmount:    1,
		DurationUnit:   SubscriptionDurationCustom,
		DurationValue:  0,
		CustomSeconds:  0,
		PlanType:       SubscriptionPlanTypeResetCard,
		ResetCardCount: 1,
	}
	require.NoError(t, DB.Create(plan).Error)

	expiresAt, err := resetCardExpiryFromPlan(plan, time.Unix(GetDBTimestamp(), 0))
	require.NoError(t, err)
	assert.Zero(t, expiresAt)
}
