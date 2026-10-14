package model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The foreign-user declaration is the compliance record that unlocks groups
// serving overseas models. It must be stored once and stay stable afterwards.
func TestConfirmOverseasDeclarationRecordsTheFirstDeclaration(t *testing.T) {
	truncateTables(t)
	DB.Exec("DELETE FROM users")

	require.NoError(t, DB.Create(&User{
		Id:       401,
		Username: "overseas-declaration-user",
		Status:   common.UserStatusEnabled,
	}).Error)

	first, err := ConfirmOverseasDeclaration(401)
	require.NoError(t, err)
	assert.Positive(t, first)

	var user User
	require.NoError(t, DB.Select("overseas_confirmed_at").Where("id = ?", 401).First(&user).Error)
	assert.Equal(t, first, user.OverseasConfirmedAt)

	// A repeated request keeps the original declaration moment.
	second, err := ConfirmOverseasDeclaration(401)
	require.NoError(t, err)
	assert.Equal(t, first, second)
	require.NoError(t, DB.Select("overseas_confirmed_at").Where("id = ?", 401).First(&user).Error)
	assert.Equal(t, first, user.OverseasConfirmedAt)
}

func TestConfirmOverseasDeclarationRejectsUnknownUser(t *testing.T) {
	truncateTables(t)
	DB.Exec("DELETE FROM users")

	_, err := ConfirmOverseasDeclaration(402)
	require.Error(t, err)
}
