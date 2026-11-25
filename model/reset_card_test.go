package model

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func seedResetCardPlan(t *testing.T, plan *SubscriptionPlan) {
	t.Helper()
	plan.PlanType = SubscriptionPlanTypeResetCard
	if plan.ResetCardCount <= 0 {
		plan.ResetCardCount = 1
	}
	require.NoError(t, DB.Create(plan).Error)
}

func seedResetCardSubscription(t *testing.T, sub *UserSubscription) {
	t.Helper()
	require.NoError(t, DB.Create(sub).Error)
}

func countResetCardsForTest(t *testing.T, userId int) int64 {
	t.Helper()
	var count int64
	require.NoError(t, DB.Model(&ResetCard{}).Where("user_id = ?", userId).Count(&count).Error)
	return count
}

func TestGrantResetCardsStoresExpiryAndOwner(t *testing.T) {
	truncateTables(t)

	user := User{Username: "reset-card-grant", Password: "unused", Role: 1, Status: 1, Group: "default", AffCode: "reset-card-grant"}
	require.NoError(t, DB.Create(&user).Error)

	expiresAt := time.Now().Add(48 * time.Hour).Unix()
	keys, err := GrantResetCards(user.Id, 3, expiresAt, "  vip gift  ", 0)

	require.NoError(t, err)
	require.Len(t, keys, 3)
	assert.EqualValues(t, 3, countResetCardsForTest(t, user.Id))

	var cards []ResetCard
	require.NoError(t, DB.Where("user_id = ?", user.Id).Order("id asc").Find(&cards).Error)
	require.Len(t, cards, 3)
	for i, card := range cards {
		assert.Equal(t, keys[i], card.CardKey)
		assert.Equal(t, ResetCardStatusUnused, card.Status)
		assert.Equal(t, ResetCardSourceAdmin, card.Source)
		assert.Equal(t, expiresAt, card.ExpiresAt)
		assert.Equal(t, "vip gift", card.Note)
	}
}

func TestGrantResetCardsRejectsExpiredExpiry(t *testing.T) {
	truncateTables(t)

	user := User{Username: "reset-card-expired", Password: "unused", Role: 1, Status: 1, Group: "default", AffCode: "reset-card-expired"}
	require.NoError(t, DB.Create(&user).Error)

	_, err := GrantResetCards(user.Id, 1, time.Now().Add(-time.Hour).Unix(), "", 0)

	require.Error(t, err)
	assert.EqualValues(t, 0, countResetCardsForTest(t, user.Id))
}

func TestIssueResetCardsFromPlanUsesPlanDurationAsExpiry(t *testing.T) {
	truncateTables(t)

	user := User{Username: "reset-card-plan", Password: "unused", Role: 1, Status: 1, Group: "default", AffCode: "reset-card-plan"}
	require.NoError(t, DB.Create(&user).Error)

	plan := &SubscriptionPlan{
		Id:             9701,
		Title:          "30 day reset card",
		PriceAmount:    5,
		DurationUnit:   SubscriptionDurationDay,
		DurationValue:  30,
		ResetCardCount: 2,
	}
	seedResetCardPlan(t, plan)

	before := GetDBTimestamp()
	keys, err := IssueResetCardsFromPlanTx(DB, user.Id, plan, ResetCardSourceOrder, "trade-1")
	after := GetDBTimestamp()

	require.NoError(t, err)
	require.Len(t, keys, 2)
	var cards []ResetCard
	require.NoError(t, DB.Where("user_id = ? AND plan_id = ?", user.Id, plan.Id).Find(&cards).Error)
	require.Len(t, cards, 2)
	for _, card := range cards {
		assert.Equal(t, ResetCardStatusUnused, card.Status)
		assert.Equal(t, ResetCardSourceOrder, card.Source)
		assert.Equal(t, "trade-1", card.TradeNo)
		assert.GreaterOrEqual(t, card.ExpiresAt, before+30*24*3600)
		assert.LessOrEqual(t, card.ExpiresAt, after+30*24*3600)
	}
}

func TestUseResetCardResetsSubscriptionAndConsumesCard(t *testing.T) {
	truncateTables(t)

	now := GetDBTimestamp()
	user := User{Username: "reset-card-use", Password: "unused", Role: 1, Status: 1, Group: "default", AffCode: "reset-card-use"}
	require.NoError(t, DB.Create(&user).Error)

	plan := &SubscriptionPlan{
		Id:               9801,
		Title:            "Pro",
		PriceAmount:      10,
		DurationUnit:     SubscriptionDurationMonth,
		DurationValue:    1,
		TotalAmount:      1000,
		QuotaResetPeriod: SubscriptionResetNever,
	}
	require.NoError(t, DB.Create(plan).Error)
	seedResetCardSubscription(t, &UserSubscription{
		Id: 9802, UserId: user.Id, PlanId: plan.Id, AmountTotal: 1000, AmountUsed: 640,
		StartTime: now - 3600, EndTime: now + 30*24*3600, Status: "active",
	})

	keys, err := GrantResetCards(user.Id, 1, 0, "", 0)
	require.NoError(t, err)
	require.Len(t, keys, 1)

	summary, err := UseResetCard(user.Id, keys[0], 9802)

	require.NoError(t, err)
	require.NotNil(t, summary)
	assert.Zero(t, summary.Subscription.AmountUsed)

	var sub UserSubscription
	require.NoError(t, DB.Where("id = ?", 9802).First(&sub).Error)
	assert.Zero(t, sub.AmountUsed)

	var card ResetCard
	require.NoError(t, DB.Where("card_key = ?", keys[0]).First(&card).Error)
	assert.Equal(t, ResetCardStatusUsed, card.Status)
	assert.EqualValues(t, user.Id, card.UsedByUserId)
	assert.EqualValues(t, 9802, card.UsedSubscriptionId)
	assert.NotZero(t, card.UsedAt)

	// A used card cannot be consumed twice.
	_, err = UseResetCard(user.Id, keys[0], 9802)
	require.Error(t, err)
}

func TestGrantResetCardsBindsTargetSubscription(t *testing.T) {
	truncateTables(t)

	now := GetDBTimestamp()
	owner := User{Username: "reset-card-bind", Password: "unused", Role: 1, Status: 1, Group: "default", AffCode: "reset-card-bind"}
	other := User{Username: "reset-card-bind-other", Password: "unused", Role: 1, Status: 1, Group: "default", AffCode: "reset-card-bind-other"}
	require.NoError(t, DB.Create(&owner).Error)
	require.NoError(t, DB.Create(&other).Error)

	plan := &SubscriptionPlan{
		Id:            9951,
		Title:         "Bound Plan",
		PriceAmount:   1,
		DurationUnit:  SubscriptionDurationMonth,
		DurationValue: 1,
		TotalAmount:   100,
	}
	require.NoError(t, DB.Create(plan).Error)
	seedResetCardSubscription(t, &UserSubscription{
		Id: 9952, UserId: owner.Id, PlanId: plan.Id, AmountTotal: 100, AmountUsed: 80,
		StartTime: now - 3600, EndTime: now + 30*24*3600, Status: "active",
	})
	seedResetCardSubscription(t, &UserSubscription{
		Id: 9953, UserId: owner.Id, PlanId: plan.Id, AmountTotal: 100, AmountUsed: 80,
		StartTime: now - 3600, EndTime: now + 30*24*3600, Status: "active",
	})
	seedResetCardSubscription(t, &UserSubscription{
		Id: 9954, UserId: other.Id, PlanId: plan.Id, AmountTotal: 100, AmountUsed: 80,
		StartTime: now - 3600, EndTime: now + 30*24*3600, Status: "active",
	})

	// Binding to another user's subscription must be rejected outright.
	_, err := GrantResetCards(owner.Id, 1, 0, "", 9954)
	require.Error(t, err)
	assert.Zero(t, countResetCardsForTest(t, owner.Id))

	// Binding to a missing subscription is rejected too.
	_, err = GrantResetCards(owner.Id, 1, 0, "", 999999)
	require.Error(t, err)

	keys, err := GrantResetCards(owner.Id, 1, 0, "", 9952)
	require.NoError(t, err)
	require.Len(t, keys, 1)

	var card ResetCard
	require.NoError(t, DB.Where("card_key = ?", keys[0]).First(&card).Error)
	assert.Equal(t, 9952, card.TargetSubscriptionId)

	// The bound card cannot reset a different subscription of the same owner.
	_, err = UseResetCard(owner.Id, keys[0], 9953)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "9952")
	var afterReset ResetCard
	require.NoError(t, DB.Where("card_key = ?", keys[0]).First(&afterReset).Error)
	assert.Equal(t, ResetCardStatusUnused, afterReset.Status)
	var untouched UserSubscription
	require.NoError(t, DB.Where("id = ?", 9953).First(&untouched).Error)
	assert.EqualValues(t, 80, untouched.AmountUsed)

	// The bound subscription can be reset with its card.
	_, err = UseResetCard(owner.Id, keys[0], 9952)
	require.NoError(t, err)

	// View projection exposes the bound subscription and its plan title.
	views, err := UserResetCards(owner.Id)
	require.NoError(t, err)
	require.Len(t, views, 1)
	assert.Equal(t, 9952, views[0].TargetSubscriptionId)
	assert.Equal(t, "Bound Plan", views[0].TargetSubscriptionPlanTitle)
}

func TestGrantResetCardsToAllUsersSkipsAdminsAndDisabled(t *testing.T) {
	truncateTables(t)

	now := GetDBTimestamp()
	active1 := User{Username: "all-cards-a", Password: "unused", Role: 1, Status: 1, Group: "default", AffCode: "all-cards-a"}
	active2 := User{Username: "all-cards-b", Password: "unused", Role: 1, Status: 1, Group: "default", AffCode: "all-cards-b"}
	disabled := User{Username: "all-cards-disabled", Password: "unused", Role: 1, Status: 2, Group: "default", AffCode: "all-cards-disabled"}
	admin := User{Username: "all-cards-admin", Password: "unused", Role: 10, Status: 1, Group: "default", AffCode: "all-cards-admin"}
	for _, user := range []*User{&active1, &active2, &disabled, &admin} {
		require.NoError(t, DB.Create(user).Error)
	}

	users, cards, err := GrantResetCardsToAllUsers(2, 0, "all-hands", 0)

	require.NoError(t, err)
	assert.Equal(t, 2, users)
	assert.EqualValues(t, 4, cards)
	assert.EqualValues(t, 4, countResetCardsForTest(t, active1.Id)+countResetCardsForTest(t, active2.Id))
	assert.Zero(t, countResetCardsForTest(t, disabled.Id))
	assert.Zero(t, countResetCardsForTest(t, admin.Id))

	var stored []ResetCard
	require.NoError(t, DB.Where("user_id = ?", active1.Id).Find(&stored).Error)
	require.Len(t, stored, 2)
	for _, card := range stored {
		assert.Equal(t, ResetCardSourceAdmin, card.Source)
		assert.Equal(t, "all-hands", card.Note)
		assert.Zero(t, card.TargetSubscriptionId)
		assert.GreaterOrEqual(t, card.CreatedAt, now)
	}
}

func TestGrantResetCardsToAllUsersRejectsExpiredExpiry(t *testing.T) {
	truncateTables(t)

	user := User{Username: "all-cards-expired", Password: "unused", Role: 1, Status: 1, Group: "default", AffCode: "all-cards-expired"}
	require.NoError(t, DB.Create(&user).Error)

	_, _, err := GrantResetCardsToAllUsers(1, time.Now().Add(-time.Hour).Unix(), "", 0)

	require.Error(t, err)
	assert.Zero(t, countResetCardsForTest(t, user.Id))
}

func TestGrantResetCardsToAllUsersBindsTargetSubscription(t *testing.T) {
	truncateTables(t)

	now := GetDBTimestamp()
	owner := User{Username: "all-cards-owner", Password: "unused", Role: 1, Status: 1, Group: "default", AffCode: "all-cards-owner"}
	other := User{Username: "all-cards-other", Password: "unused", Role: 1, Status: 1, Group: "default", AffCode: "all-cards-other"}
	require.NoError(t, DB.Create(&owner).Error)
	require.NoError(t, DB.Create(&other).Error)

	plan := &SubscriptionPlan{
		Id:            9971,
		Title:         "All Users Bound Plan",
		PriceAmount:   1,
		DurationUnit:  SubscriptionDurationMonth,
		DurationValue: 1,
		TotalAmount:   100,
	}
	require.NoError(t, DB.Create(plan).Error)
	seedResetCardSubscription(t, &UserSubscription{
		Id: 9972, UserId: owner.Id, PlanId: plan.Id, AmountTotal: 100, AmountUsed: 60,
		StartTime: now - 3600, EndTime: now + 30*24*3600, Status: "active",
	})

	users, cards, err := GrantResetCardsToAllUsers(1, 0, "bound all", 9972)

	require.NoError(t, err)
	assert.Equal(t, 2, users)
	assert.EqualValues(t, 2, cards)
	var bound []ResetCard
	require.NoError(t, DB.Where("target_subscription_id = ?", 9972).Find(&bound).Error)
	require.Len(t, bound, 2)
	for _, card := range bound {
		assert.Equal(t, ResetCardStatusUnused, card.Status)
	}

	// The other user owns a card but cannot redeem it: it belongs to the
	// owner's subscription.
	var otherCard ResetCard
	require.NoError(t, DB.Where("user_id = ?", other.Id).First(&otherCard).Error)
	_, err = UseResetCard(other.Id, otherCard.CardKey, 9972)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "does not belong to this user")

	// A missing subscription is rejected before any card is created.
	before := countResetCardsForTest(t, owner.Id)
	_, _, err = GrantResetCardsToAllUsers(1, 0, "", 999999)
	require.Error(t, err)
	assert.Equal(t, before, countResetCardsForTest(t, owner.Id))
}

func TestUseResetCardRejectsForeignCardAndInactiveSubscription(t *testing.T) {
	truncateTables(t)

	now := GetDBTimestamp()
	owner := User{Username: "reset-card-owner", Password: "unused", Role: 1, Status: 1, Group: "default", AffCode: "reset-card-owner"}
	other := User{Username: "reset-card-other", Password: "unused", Role: 1, Status: 1, Group: "default", AffCode: "reset-card-other"}
	require.NoError(t, DB.Create(&owner).Error)
	require.NoError(t, DB.Create(&other).Error)

	plan := &SubscriptionPlan{
		Id:            9901,
		Title:         "Basic",
		PriceAmount:   1,
		DurationUnit:  SubscriptionDurationMonth,
		DurationValue: 1,
		TotalAmount:   100,
	}
	require.NoError(t, DB.Create(plan).Error)
	seedResetCardSubscription(t, &UserSubscription{
		Id: 9902, UserId: other.Id, PlanId: plan.Id, AmountTotal: 100, AmountUsed: 50,
		StartTime: now - 3600, EndTime: now + 30*24*3600, Status: "active",
	})
	seedResetCardSubscription(t, &UserSubscription{
		Id: 9903, UserId: owner.Id, PlanId: plan.Id, AmountTotal: 100, AmountUsed: 50,
		StartTime: now - 7200, EndTime: now - 60, Status: "active",
	})

	keys, err := GrantResetCards(owner.Id, 1, 0, "", 0)
	require.NoError(t, err)

	// Another user's card must be rejected.
	_, err = UseResetCard(other.Id, keys[0], 9902)
	require.Error(t, err)

	// The owner cannot reset somebody else's subscription.
	_, err = UseResetCard(owner.Id, keys[0], 9902)
	require.Error(t, err)

	// The owner cannot reset their own expired subscription.
	_, err = UseResetCard(owner.Id, keys[0], 9903)
	require.Error(t, err)

	var card ResetCard
	require.NoError(t, DB.Where("card_key = ?", keys[0]).First(&card).Error)
	assert.Equal(t, ResetCardStatusUnused, card.Status)
}

func TestExpireDueResetCardsMarksOnlyPastUnusedCards(t *testing.T) {
	truncateTables(t)

	now := GetDBTimestamp()
	user := User{Username: "reset-card-expiry", Password: "unused", Role: 1, Status: 1, Group: "default", AffCode: "reset-card-expiry"}
	require.NoError(t, DB.Create(&user).Error)

	expiredKeys, err := GrantResetCards(user.Id, 1, now+60, "", 0)
	require.NoError(t, err)
	activeKeys, err := GrantResetCards(user.Id, 1, now+24*3600, "", 0)
	require.NoError(t, err)
	neverKeys, err := GrantResetCards(user.Id, 1, 0, "", 0)
	require.NoError(t, err)

	require.NoError(t, DB.Model(&ResetCard{}).Where("card_key = ?", expiredKeys[0]).
		Update("expires_at", now-10).Error)

	count, err := ExpireDueResetCards(100)

	require.NoError(t, err)
	assert.EqualValues(t, 1, count)
	var expired, active, never ResetCard
	require.NoError(t, DB.Where("card_key = ?", expiredKeys[0]).First(&expired).Error)
	require.NoError(t, DB.Where("card_key = ?", activeKeys[0]).First(&active).Error)
	require.NoError(t, DB.Where("card_key = ?", neverKeys[0]).First(&never).Error)
	assert.Equal(t, ResetCardStatusExpired, expired.Status)
	assert.Equal(t, ResetCardStatusUnused, active.Status)
	assert.Equal(t, ResetCardStatusUnused, never.Status)
}
