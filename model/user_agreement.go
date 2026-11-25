package model

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strings"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const userAgreementOptionKey = "legal.user_agreement"

var ErrUserAgreementChanged = errors.New("user agreement changed")

type UserAgreementStatus struct {
	Revision string `json:"revision"`
	Required bool   `json:"required"`
}

func (option Option) agreementRevision() string {
	content := strings.TrimSpace(option.Value)
	if content == "" {
		return ""
	}
	if option.AgreementRevision != "" {
		return option.AgreementRevision
	}
	// Upgraded instances may have an agreement before revision tracking exists.
	digest := sha256.Sum256([]byte(content))
	return hex.EncodeToString(digest[:])
}

func GetCurrentUserAgreement(ctx context.Context) (string, error) {
	var agreement Option
	err := DB.WithContext(ctx).Where(&Option{Key: userAgreementOptionKey}).Find(&agreement).Error
	return agreement.Value, err
}

func GetUserAgreementStatus(ctx context.Context, userID int) (UserAgreementStatus, error) {
	tx := DB.WithContext(ctx)
	var agreement Option
	if err := tx.Where(&Option{Key: userAgreementOptionKey}).Find(&agreement).Error; err != nil {
		return UserAgreementStatus{}, err
	}
	revision := agreement.agreementRevision()
	if revision == "" {
		return UserAgreementStatus{}, nil
	}
	var user User
	if err := tx.Select("accepted_agreement_revision").First(&user, userID).Error; err != nil {
		return UserAgreementStatus{}, err
	}
	return UserAgreementStatus{Revision: revision, Required: user.AcceptedAgreementRevision != revision}, nil
}

func AcceptUserAgreement(ctx context.Context, identity AuthSessionIdentity, revision string) error {
	return DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := ValidateAuthSessionWithTx(tx, identity); err != nil {
			return err
		}
		var agreement Option
		if err := lockForUpdate(tx).Where(&Option{Key: userAgreementOptionKey}).Find(&agreement).Error; err != nil {
			return err
		}
		if revision == "" || revision != agreement.agreementRevision() {
			return ErrUserAgreementChanged
		}
		return tx.Model(&User{}).Where("id = ?", identity.UserID).UpdateColumn("accepted_agreement_revision", revision).Error
	})
}

func updateUserAgreement(tx *gorm.DB, content string) error {
	seed := Option{Key: userAgreementOptionKey}
	if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&seed).Error; err != nil {
		return err
	}
	var agreement Option
	if err := lockForUpdate(tx).First(&agreement, &Option{Key: userAgreementOptionKey}).Error; err != nil {
		return err
	}
	if strings.TrimSpace(content) != strings.TrimSpace(agreement.Value) {
		agreement.AgreementRevision = uuid.NewString()
	}
	agreement.Value = content
	return tx.Save(&agreement).Error
}
