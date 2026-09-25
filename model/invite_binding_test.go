package model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupInviteBindingTestState(t *testing.T) {
	t.Helper()
	truncateTables(t)
	require.NoError(t, DB.Exec("DELETE FROM users").Error)
	require.NoError(t, DB.AutoMigrate(&Redemption{}))
	require.NoError(t, DB.Session(&gorm.Session{AllowGlobalUpdate: true}).Unscoped().Delete(&Redemption{}).Error)
	t.Cleanup(func() {
		DB.Session(&gorm.Session{AllowGlobalUpdate: true}).Unscoped().Delete(&Redemption{})
	})

	oldRedisEnabled := common.RedisEnabled
	oldRate := common.InviterRechargeRebateRate
	common.RedisEnabled = false
	common.InviterRechargeRebateRate = 5
	t.Cleanup(func() {
		common.RedisEnabled = oldRedisEnabled
		common.InviterRechargeRebateRate = oldRate
	})
}

func createInviteUser(t *testing.T, id int, username, affCode string) *User {
	t.Helper()
	user := &User{
		Id:          id,
		Username:    username,
		Password:    "unused-password-hash",
		Role:        common.RoleCommonUser,
		Status:      common.UserStatusEnabled,
		Group:       "default",
		AuthVersion: 1,
		AffCode:     affCode,
	}
	require.NoError(t, DB.Create(user).Error)
	return user
}

func insertInviteTopUp(t *testing.T, userId int, provider string, amount int64, money float64, status string) {
	t.Helper()
	topUp := &TopUp{
		UserId:          userId,
		Amount:          amount,
		Money:           money,
		TradeNo:         "invite-" + provider + "-" + status + "-" + common.GetRandomString(6),
		PaymentProvider: provider,
		Status:          status,
	}
	require.NoError(t, DB.Create(topUp).Error)
}

// loadUser re-reads a user through a fresh struct: GORM turns a struct's
// non-zero primary key into an extra WHERE condition, so reusing a variable
// across lookups would filter on the previously loaded id.
func loadUser(t *testing.T, id int) User {
	t.Helper()
	var user User
	require.NoError(t, DB.First(&user, id).Error)
	return user
}

// The historical base must reproduce the credited quota of each recharge path:
// creem stores the quota directly in amount, stripe stores money in money, and
// the remaining providers store money in amount. Only completed recharges and
// used quota redemption codes bear a rebate.
func TestHistoricalInviteRebateBaseMirrorsRechargeConversions(t *testing.T) {
	setupInviteBindingTestState(t)

	user := createInviteUser(t, 101, "invitee-base", "invitee-base-aff")
	unit := int(common.QuotaPerUnit)

	insertInviteTopUp(t, user.Id, PaymentProviderEpay, 10, 10, common.TopUpStatusSuccess)
	insertInviteTopUp(t, user.Id, PaymentProviderStripe, 0, 3, common.TopUpStatusSuccess)
	insertInviteTopUp(t, user.Id, PaymentProviderCreem, int64(4*unit), 0, common.TopUpStatusSuccess)
	// Legacy rows can carry an empty provider with only money filled in.
	insertInviteTopUp(t, user.Id, "", 0, 2, common.TopUpStatusSuccess)
	// Pending orders never credited quota, so they must not enter the base.
	insertInviteTopUp(t, user.Id, PaymentProviderEpay, 99, 99, common.TopUpStatusPending)

	redemptions := []Redemption{
		{UserId: 1, Key: "invite-used", Status: common.RedemptionCodeStatusUsed, Quota: 6 * unit, Type: RedemptionTypeQuota, UsedUserId: user.Id},
		{UserId: 1, Key: "invite-enabled", Status: common.RedemptionCodeStatusEnabled, Quota: 100 * unit, Type: RedemptionTypeQuota, UsedUserId: user.Id},
		{UserId: 1, Key: "invite-lottery", Status: common.RedemptionCodeStatusUsed, Quota: 0, Type: RedemptionTypeLottery, Value: 5, UsedUserId: user.Id},
	}
	require.NoError(t, DB.Create(&redemptions).Error)

	base, err := HistoricalInviteRebateBase(user.Id)
	require.NoError(t, err)
	assert.Equal(t, 19*unit, base.TopUpQuota)
	assert.Equal(t, 6*unit, base.RedemptionQuota)
	assert.Equal(t, 25*unit, base.TotalQuota)

	preview, err := PreviewInviteRebate(user.Id)
	require.NoError(t, err)
	assert.Equal(t, 5.0, preview.RebateRate)
	assert.Equal(t, 25*unit*5/100, preview.RebateQuota)
}

func TestHistoricalInviteRebateBaseRejectsMissingUser(t *testing.T) {
	setupInviteBindingTestState(t)

	_, err := HistoricalInviteRebateBase(0)
	require.Error(t, err)
}

func TestValidateInviterBindingRejectsSelfAndCycles(t *testing.T) {
	setupInviteBindingTestState(t)

	top := createInviteUser(t, 201, "invite-top", "invite-top-aff")
	middle := createInviteUser(t, 202, "invite-middle", "invite-middle-aff")
	leaf := createInviteUser(t, 203, "invite-leaf", "invite-leaf-aff")

	require.NoError(t, ValidateInviterBinding(leaf.Id, middle.Id))

	// middle is invited by top, then top is invited by leaf: every link on its
	// own is fine because the walk up the chain only rejects an existing path.
	_, err := BindInviter(middle.Id, top.Id, 0)
	require.NoError(t, err)
	_, err = BindInviter(leaf.Id, middle.Id, 0)
	require.NoError(t, err)

	require.ErrorIs(t, ValidateInviterBinding(top.Id, top.Id), ErrInviterSelfBinding)
	// leaf's chain is leaf -> middle -> top, so top cannot be invited by leaf.
	require.ErrorIs(t, ValidateInviterBinding(top.Id, leaf.Id), ErrInviterCycle)
	require.ErrorIs(t, ValidateInviterBinding(middle.Id, leaf.Id), ErrInviterCycle)
	// Inviting top by an unrelated user is still allowed.
	outsider := createInviteUser(t, 204, "invite-outsider", "invite-outsider-aff")
	require.NoError(t, ValidateInviterBinding(top.Id, outsider.Id))
}

func TestBindInviterCreditsRebateAndCountsOnce(t *testing.T) {
	setupInviteBindingTestState(t)

	inviter := createInviteUser(t, 301, "invite-credit-inviter", "invite-credit-inviter-aff")
	invitee := createInviteUser(t, 302, "invite-credit-invitee", "invite-credit-invitee-aff")
	other := createInviteUser(t, 303, "invite-credit-other", "invite-credit-other-aff")

	previous, err := BindInviter(invitee.Id, inviter.Id, 12_345)
	require.NoError(t, err)
	assert.Equal(t, 0, previous)

	assert.Equal(t, inviter.Id, loadUser(t, invitee.Id).InviterId)

	credited := loadUser(t, inviter.Id)
	assert.Equal(t, 12_345, credited.AffQuota)
	assert.Equal(t, 12_345, credited.AffHistoryQuota)
	assert.Equal(t, 1, credited.AffCount)

	// Re-applying the same binding must not inflate the invite count.
	previous, err = BindInviter(invitee.Id, inviter.Id, 0)
	require.NoError(t, err)
	assert.Equal(t, inviter.Id, previous)
	credited = loadUser(t, inviter.Id)
	assert.Equal(t, 1, credited.AffCount)
	assert.Equal(t, 12_345, credited.AffQuota)

	// Moving the invitee to another inviter counts the new inviter only.
	previous, err = BindInviter(invitee.Id, other.Id, 0)
	require.NoError(t, err)
	assert.Equal(t, inviter.Id, previous)
	assert.Equal(t, 1, loadUser(t, other.Id).AffCount)
	assert.Equal(t, 0, loadUser(t, inviter.Id).AffCount)
	assert.Equal(t, other.Id, loadUser(t, invitee.Id).InviterId)
}

func TestBindInviterFailsWhenInviterIsMissing(t *testing.T) {
	setupInviteBindingTestState(t)

	invitee := createInviteUser(t, 401, "invite-missing-invitee", "invite-missing-invitee-aff")

	_, err := BindInviter(invitee.Id, 4999, 0)
	require.ErrorIs(t, err, ErrInviterNotFound)

	var got User
	require.NoError(t, DB.First(&got, invitee.Id).Error)
	assert.Equal(t, 0, got.InviterId)
}

func TestFindInviterByIdentifierAcceptsIdUsernameAndAffCode(t *testing.T) {
	setupInviteBindingTestState(t)

	inviter := createInviteUser(t, 501, "invite-lookup", "invite-lookup-aff")

	byId, err := FindInviterByIdentifier("501")
	require.NoError(t, err)
	assert.Equal(t, inviter.Id, byId.Id)

	byName, err := FindInviterByIdentifier("invite-lookup")
	require.NoError(t, err)
	assert.Equal(t, inviter.Id, byName.Id)

	byCode, err := FindInviterByIdentifier("invite-lookup-aff")
	require.NoError(t, err)
	assert.Equal(t, inviter.Id, byCode.Id)

	_, err = FindInviterByIdentifier("   ")
	require.Error(t, err)

	_, err = FindInviterByIdentifier("no-such-inviter")
	require.ErrorIs(t, err, gorm.ErrRecordNotFound)
}
