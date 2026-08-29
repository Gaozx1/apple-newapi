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
)

func seedOAuthCallPlanUser(t *testing.T, id int, quota int) {
	t.Helper()
	require.NoError(t, DB.Create(&User{Id: id, Username: "oauthuser", Status: common.UserStatusEnabled, Quota: quota}).Error)
}

func TestPurchaseOAuthCallPlanCreatesGrantAndDeductsWallet(t *testing.T) {
	truncateTables(t)

	plan := &OAuthCallPlan{Id: 9501, Name: "P1000", CallCount: 1000, PriceAmount: 1, Enabled: true}
	require.NoError(t, DB.Create(plan).Error)
	seedOAuthCallPlanUser(t, 301, int(common.QuotaPerUnit)*5)

	grant, priceQuota, err := PurchaseOAuthCallPlan(301, plan.Id)
	require.NoError(t, err)
	require.NotNil(t, grant)
	assert.Equal(t, plan.CallCount, grant.CallsTotal)
	assert.Equal(t, 0, grant.CallsUsed)
	assert.Equal(t, "active", grant.Status)
	assert.Zero(t, grant.ExpiresAt)
	assert.Positive(t, priceQuota)

	balance, err := GetOAuthCallBalance(301)
	require.NoError(t, err)
	assert.Equal(t, 1000, balance)

	var user User
	require.NoError(t, DB.First(&user, 301).Error)
	assert.Equal(t, int(common.QuotaPerUnit)*5-priceQuota, user.Quota)
}

func TestPurchaseOAuthCallPlanRejectsInsufficientBalance(t *testing.T) {
	truncateTables(t)

	plan := &OAuthCallPlan{Id: 9502, Name: "P1000", CallCount: 1000, PriceAmount: 1, Enabled: true}
	require.NoError(t, DB.Create(plan).Error)
	seedOAuthCallPlanUser(t, 302, 1)

	_, _, err := PurchaseOAuthCallPlan(302, plan.Id)
	require.Error(t, err)

	var count int64
	require.NoError(t, DB.Model(&OAuthCallGrant{}).Where("user_id = ?", 302).Count(&count).Error)
	assert.Zero(t, count)
}

func TestConsumeOAuthCallGrantPrefersSoonestExpiryAndExhausts(t *testing.T) {
	truncateTables(t)

	seedOAuthCallPlanUser(t, 303, 0)
	// Grant A expires soon and holds 2 calls; grant B never expires.
	require.NoError(t, DB.Create(&OAuthCallGrant{
		Id: 9601, UserId: 303, PlanId: 1, PlanName: "A",
		CallsTotal: 2, CallsUsed: 0, ExpiresAt: GetDBTimestamp() + 60,
		Status: "active",
	}).Error)
	require.NoError(t, DB.Create(&OAuthCallGrant{
		Id: 9602, UserId: 303, PlanId: 1, PlanName: "B",
		CallsTotal: 5, CallsUsed: 0, ExpiresAt: 0,
		Status: "active",
	}).Error)

	// First two consumes draw from the expiring grant A.
	for i := 0; i < 2; i++ {
		consumed, err := ConsumeOAuthCallGrant(303)
		require.NoError(t, err)
		assert.True(t, consumed)
	}
	var grantA OAuthCallGrant
	require.NoError(t, DB.First(&grantA, 9601).Error)
	assert.Equal(t, 2, grantA.CallsUsed)
	assert.Equal(t, "exhausted", grantA.Status)

	// Third consume falls through to the never-expiring grant B.
	consumed, err := ConsumeOAuthCallGrant(303)
	require.NoError(t, err)
	assert.True(t, consumed)
	var grantB OAuthCallGrant
	require.NoError(t, DB.First(&grantB, 9602).Error)
	assert.Equal(t, 1, grantB.CallsUsed)

	balance, err := GetOAuthCallBalance(303)
	require.NoError(t, err)
	assert.Equal(t, 4, balance)
}

func TestConsumeOAuthCallGrantWithoutGrantsReturnsFalse(t *testing.T) {
	truncateTables(t)

	seedOAuthCallPlanUser(t, 304, 0)
	// An expired grant must not be consumable.
	require.NoError(t, DB.Create(&OAuthCallGrant{
		Id: 9603, UserId: 304, PlanId: 1, PlanName: "Old",
		CallsTotal: 5, CallsUsed: 0, ExpiresAt: GetDBTimestamp() - 1,
		Status: "active",
	}).Error)

	consumed, err := ConsumeOAuthCallGrant(304)
	require.NoError(t, err)
	assert.False(t, consumed)

	balance, err := GetOAuthCallBalance(304)
	require.NoError(t, err)
	assert.Zero(t, balance)
}
