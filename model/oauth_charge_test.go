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

// ladder mirrors the controller's oauthTierPrice: free below 150, then
// $0.001 / $0.002 / $0.003 ladders converted to quota units.
func testLadder(callCount int) int {
	switch {
	case callCount < 150:
		return 0
	case callCount < 200:
		return int(common.QuotaPerUnit) / 1000
	case callCount < 300:
		return 2 * int(common.QuotaPerUnit) / 1000
	default:
		return 3 * int(common.QuotaPerUnit) / 1000
	}
}

func TestChargeOAuthDailyCallFreeTierCountsOnly(t *testing.T) {
	truncateTables(t)
	seedOAuthCallPlanUser(t, 311, 0)

	for i := 0; i < 5; i++ {
		count, consumedGrant, charged, err := ChargeOAuthDailyCall(311, "2026-08-29", testLadder)
		require.NoError(t, err)
		assert.Equal(t, i, count)
		assert.False(t, consumedGrant)
		assert.Zero(t, charged)
	}

	var usage OAuthDailyUsage
	require.NoError(t, DB.Where("user_id = ?", 311).First(&usage).Error)
	assert.Equal(t, 5, usage.CallCount)
}

func TestChargeOAuthDailyCallInsufficientQuotaRollsBackCount(t *testing.T) {
	truncateTables(t)
	// Wallet balance of 1 quota unit cannot cover a $0.001+ call beyond the
	// free tier once the ladder crosses it — hmm, keep it deterministic: start
	// past the free tier directly and give zero balance.
	seedOAuthCallPlanUser(t, 312, 0)
	require.NoError(t, DB.Create(&OAuthDailyUsage{UserId: 312, Date: "2026-08-29", CallCount: 200}).Error)

	_, _, _, err := ChargeOAuthDailyCall(312, "2026-08-29", testLadder)
	require.ErrorIs(t, err, ErrOAuthInsufficientQuota)

	// The counter must not advance when the charge is refused.
	var usage OAuthDailyUsage
	require.NoError(t, DB.Where("user_id = ?", 312).First(&usage).Error)
	assert.Equal(t, 200, usage.CallCount)
}

func TestChargeOAuthDailyCallNeverDrivesWalletNegative(t *testing.T) {
	truncateTables(t)
	// Balance below the $0.002 tier price must be refused, not deducted.
	seedOAuthCallPlanUser(t, 313, int(common.QuotaPerUnit)/1000) // covers $0.001 only
	require.NoError(t, DB.Create(&OAuthDailyUsage{UserId: 313, Date: "2026-08-29", CallCount: 250}).Error)

	_, _, _, err := ChargeOAuthDailyCall(313, "2026-08-29", testLadder)
	require.ErrorIs(t, err, ErrOAuthInsufficientQuota)

	var user User
	require.NoError(t, DB.First(&user, 313).Error)
	assert.Equal(t, int(common.QuotaPerUnit)/1000, user.Quota)
}

func TestChargeOAuthDailyCallConsumesGrantBeforeWallet(t *testing.T) {
	truncateTables(t)
	seedOAuthCallPlanUser(t, 314, int(common.QuotaPerUnit))
	require.NoError(t, DB.Create(&OAuthCallGrant{
		Id: 9611, UserId: 314, PlanId: 1, PlanName: "A",
		CallsTotal: 1, CallsUsed: 0, ExpiresAt: 0, Status: "active",
	}).Error)
	require.NoError(t, DB.Create(&OAuthDailyUsage{UserId: 314, Date: "2026-08-29", CallCount: 150}).Error)

	count, consumedGrant, charged, err := ChargeOAuthDailyCall(314, "2026-08-29", testLadder)
	require.NoError(t, err)
	assert.Equal(t, 150, count)
	assert.True(t, consumedGrant)
	assert.Zero(t, charged)

	// The wallet is untouched while a grant covers the call.
	var user User
	require.NoError(t, DB.First(&user, 314).Error)
	assert.Equal(t, int(common.QuotaPerUnit), user.Quota)

	var grant OAuthCallGrant
	require.NoError(t, DB.First(&grant, 9611).Error)
	assert.Equal(t, 1, grant.CallsUsed)
	assert.Equal(t, "exhausted", grant.Status)
}

func TestDeleteOAuthClientRevokesTokensAndCodes(t *testing.T) {
	truncateTables(t)

	client := &OAuthClient{ClientId: "cid_del", ClientSecret: "s", Name: "app", Enabled: true, UserId: 1}
	require.NoError(t, DB.Create(client).Error)
	require.NoError(t, DB.Create(&OAuthAccessToken{AccessToken: "tok", ClientId: "cid_del", UserId: 1, ExpiresAt: 9999999999}).Error)
	require.NoError(t, DB.Create(&OAuthAuthorizationCode{Code: "code1", ClientId: "cid_del", UserId: 1, ExpiresAt: 9999999999}).Error)
	// Another client's data must survive.
	require.NoError(t, DB.Create(&OAuthAccessToken{AccessToken: "tok2", ClientId: "cid_keep", UserId: 1, ExpiresAt: 9999999999}).Error)

	require.NoError(t, DeleteOAuthClient(client.Id))

	var tokens, codes int64
	require.NoError(t, DB.Model(&OAuthAccessToken{}).Where("client_id = ?", "cid_del").Count(&tokens).Error)
	require.NoError(t, DB.Model(&OAuthAuthorizationCode{}).Where("client_id = ?", "cid_del").Count(&codes).Error)
	assert.Zero(t, tokens)
	assert.Zero(t, codes)

	var kept int64
	require.NoError(t, DB.Model(&OAuthAccessToken{}).Where("client_id = ?", "cid_keep").Count(&kept).Error)
	assert.Equal(t, int64(1), kept)
}

func TestRevokeAllUserSessionsDeletesOAuthAccessTokens(t *testing.T) {
	truncateTables(t)

	seedOAuthCallPlanUser(t, 315, 0)
	require.NoError(t, DB.Create(&OAuthAccessToken{AccessToken: "bearer", ClientId: "c", UserId: 315, ExpiresAt: 9999999999}).Error)

	_, err := RevokeAllUserSessions(315, "test_revoke")
	require.NoError(t, err)

	var count int64
	require.NoError(t, DB.Model(&OAuthAccessToken{}).Where("user_id = ?", 315).Count(&count).Error)
	assert.Zero(t, count)
}


func TestGetOrCreateOAuthApiTokenIdempotent(t *testing.T) {
	truncateTables(t)

	token, created, err := GetOrCreateOAuthApiToken(321, "oauth2-cid_1", "")
	require.NoError(t, err)
	assert.True(t, created)
	assert.NotEmpty(t, token.Key)
	assert.Equal(t, common.TokenStatusEnabled, token.Status)
	assert.Equal(t, int64(-1), token.ExpiredTime)
	assert.True(t, token.UnlimitedQuota)

	// Second exchange must reuse the same token, not mint a new one.
	again, created, err := GetOrCreateOAuthApiToken(321, "oauth2-cid_1", "")
	require.NoError(t, err)
	assert.False(t, created)
	assert.Equal(t, token.Id, again.Id)
	assert.Equal(t, token.Key, again.Key)
}

func TestGetOrCreateOAuthApiTokenDisabledStaysDisabled(t *testing.T) {
	truncateTables(t)

	token, created, err := GetOrCreateOAuthApiToken(322, "oauth2-cid_2", "")
	require.NoError(t, err)
	assert.True(t, created)

	// The user disables the token (explicit revocation): further exchanges
	// must refuse instead of silently issuing a fresh enabled token.
	require.NoError(t, DB.Model(&Token{}).Where("id = ?", token.Id).
		Update("status", common.TokenStatusDisabled).Error)
	_, _, err = GetOrCreateOAuthApiToken(322, "oauth2-cid_2", "")
	require.ErrorIs(t, err, ErrOAuthApiTokenDisabled)

	// Deleting the token allows the next exchange to mint a fresh one.
	require.NoError(t, DB.Where("id = ?", token.Id).Delete(&Token{}).Error)
	fresh, created, err := GetOrCreateOAuthApiToken(322, "oauth2-cid_2", "")
	require.NoError(t, err)
	assert.True(t, created)
	assert.NotEqual(t, token.Id, fresh.Id)
}

func TestGetOrCreateOAuthApiTokenGroupSwitchUpdatesInPlace(t *testing.T) {
	truncateTables(t)

	token, created, err := GetOrCreateOAuthApiToken(323, "oauth2-cid_3", "")
	require.NoError(t, err)
	assert.True(t, created)
	assert.Empty(t, token.Group)

	// Switching the group on reuse updates the same token in place.
	switched, created, err := GetOrCreateOAuthApiToken(323, "oauth2-cid_3", "vip")
	require.NoError(t, err)
	assert.False(t, created)
	assert.Equal(t, token.Id, switched.Id)
	assert.Equal(t, token.Key, switched.Key)
	assert.Equal(t, "vip", switched.Group)

	var fresh Token
	require.NoError(t, DB.Where("id = ?", token.Id).First(&fresh).Error)
	assert.Equal(t, "vip", fresh.Group)
	assert.Equal(t, common.TokenStatusEnabled, fresh.Status)

	// Empty group leaves the bound group untouched.
	keep, created, err := GetOrCreateOAuthApiToken(323, "oauth2-cid_3", "")
	require.NoError(t, err)
	assert.False(t, created)
	assert.Equal(t, "vip", keep.Group)
}

func TestReviewOAuthClientGatesUsabilityAndRevokesOnReject(t *testing.T) {
	truncateTables(t)

	client := &OAuthClient{ClientId: "cid_review", ClientSecret: "s", Name: "app", Enabled: true, UserId: 1, Status: OAuthClientStatusPending}
	require.NoError(t, DB.Create(client).Error)
	require.NoError(t, DB.Create(&OAuthAccessToken{AccessToken: "rtok", ClientId: "cid_review", UserId: 1, ExpiresAt: 9999999999}).Error)
	require.NoError(t, DB.Create(&OAuthAuthorizationCode{Code: "rcode", ClientId: "cid_review", UserId: 1, ExpiresAt: 9999999999}).Error)

	// Pending + enabled must not be usable.
	var loaded OAuthClient
	require.NoError(t, DB.First(&loaded, client.Id).Error)
	assert.False(t, loaded.IsUsable())

	// Approve → usable, tokens kept.
	require.NoError(t, ReviewOAuthClient(client.Id, OAuthClientStatusApproved))
	require.NoError(t, DB.First(&loaded, client.Id).Error)
	assert.True(t, loaded.IsUsable())
	var toks int64
	require.NoError(t, DB.Model(&OAuthAccessToken{}).Where("client_id = ?", "cid_review").Count(&toks).Error)
	assert.Equal(t, int64(1), toks)

	// Disabled + approved still not usable.
	require.NoError(t, DB.Model(&OAuthClient{}).Where("id = ?", client.Id).Update("enabled", false).Error)
	require.NoError(t, DB.First(&loaded, client.Id).Error)
	assert.False(t, loaded.IsUsable())
	require.NoError(t, DB.Model(&OAuthClient{}).Where("id = ?", client.Id).Update("enabled", true).Error)

	// Reject → not usable and tokens/codes revoked.
	require.NoError(t, ReviewOAuthClient(client.Id, OAuthClientStatusRejected))
	require.NoError(t, DB.First(&loaded, client.Id).Error)
	assert.False(t, loaded.IsUsable())
	assert.Equal(t, OAuthClientStatusRejected, loaded.Status)
	require.NoError(t, DB.Model(&OAuthAccessToken{}).Where("client_id = ?", "cid_review").Count(&toks).Error)
	assert.Zero(t, toks)
	var codes int64
	require.NoError(t, DB.Model(&OAuthAuthorizationCode{}).Where("client_id = ?", "cid_review").Count(&codes).Error)
	assert.Zero(t, codes)

	// Invalid review status refused.
	require.Error(t, ReviewOAuthClient(client.Id, "bogus"))
}
