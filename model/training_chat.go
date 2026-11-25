package model

import (
	"context"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

const (
	TrainingUndecided = "undecided"
	TrainingAccepted  = "accepted"
	TrainingDeclined  = "declined"
	TrainingChatTTL   = 24 * time.Hour
)

// TrainingChat is separate from request, audit, and usage logs.
type TrainingChat struct {
	ID        int64  `json:"-" gorm:"primaryKey"`
	UserID    int    `json:"-" gorm:"not null;index"`
	Prompt    string `json:"-" gorm:"type:text;not null"`
	Reply     string `json:"-" gorm:"type:text;not null"`
	ExpiresAt int64  `json:"-" gorm:"not null;index"`
}

// TrainingConsent contains either an explicit refusal or the revision of an
// opt-in. A fresh revision prevents a reply that straddles withdrawal and
// re-consent from being collected.
type TrainingConsent string

func (consent TrainingConsent) Decision() string {
	switch consent {
	case TrainingDeclined:
		return TrainingDeclined
	default:
		if _, err := uuid.Parse(string(consent)); err == nil {
			return TrainingAccepted
		}
		return TrainingUndecided
	}
}

func GetTrainingConsent(ctx context.Context, userID int) (TrainingConsent, error) {
	var user User
	err := DB.WithContext(ctx).Select("training_consent").First(&user, userID).Error
	return user.TrainingConsent, err
}

func SetTrainingConsent(ctx context.Context, identity AuthSessionIdentity, accepted bool) (TrainingConsent, error) {
	consent := TrainingConsent(TrainingDeclined)
	if accepted {
		consent = TrainingConsent(uuid.NewString())
	}
	err := DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := ValidateAuthSessionWithTx(tx, identity); err != nil {
			return err
		}
		if err := tx.Model(&User{}).Where("id = ?", identity.UserID).UpdateColumn("training_consent", consent).Error; err != nil {
			return err
		}
		if !accepted {
			return tx.Where("user_id = ?", identity.UserID).Delete(&TrainingChat{}).Error
		}
		return nil
	})
	return consent, err
}

func SaveTrainingChat(ctx context.Context, userID int, consent TrainingConsent, prompt, reply string) error {
	if consent.Decision() != TrainingAccepted {
		return nil
	}
	// Chat content must never appear in interpolated SQL or error logs.
	return DB.WithContext(ctx).Session(&gorm.Session{Logger: logger.Default.LogMode(logger.Silent)}).Transaction(func(tx *gorm.DB) error {
		owner := tx.Model(&User{}).Where("id = ? AND status = ? AND training_consent = ?", userID, common.UserStatusEnabled, consent)
		if common.UsingMainDatabase(common.DatabaseTypeSQLite) {
			// Start with a write on SQLite, which has no SELECT FOR UPDATE.
			// Hold its write lock through the check and insert.
			if err := owner.UpdateColumn("training_consent", consent).Error; err != nil {
				return err
			}
		}
		var authorized User
		result := lockForUpdate(owner).Select("id").Find(&authorized)
		if result.Error != nil || result.RowsAffected == 0 {
			return result.Error
		}
		return tx.Create(&TrainingChat{
			UserID: userID, Prompt: prompt, Reply: reply,
			ExpiresAt: time.Now().Add(TrainingChatTTL).Unix(),
		}).Error
	})
}

func DeleteExpiredTrainingChats(now time.Time) error {
	return DB.Where("expires_at <= ?", now.Unix()).Delete(&TrainingChat{}).Error
}
