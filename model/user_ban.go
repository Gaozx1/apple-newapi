package model

import (
	"errors"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// UserBan is the public metadata for the current ban. The user status remains
// authoritative. Store only the reason intended for public display.
type UserBan struct {
	UserID   int    `gorm:"primaryKey;autoIncrement:false"`
	Source   string `gorm:"type:varchar(16);not null"`
	Reason   string `gorm:"type:varchar(200);not null"`
	BannedAt int64  `gorm:"not null"`
}

var ErrUserBanForbidden = errors.New("cannot change the ban status of this account")

func ValidUserBanReason(reason string) bool {
	reason = strings.TrimSpace(reason)
	if strings.ContainsRune(reason, 0) || utf8.RuneCountInString(reason) > 200 {
		return false
	}
	return strings.ContainsFunc(reason, func(r rune) bool {
		return !unicode.IsSpace(r) && !unicode.IsControl(r) && !unicode.Is(unicode.Cf, r)
	})
}

func SetManualUserBan(userID, operatorRole int, disable bool, reason string) (User, error) {
	var ban *UserBan
	if disable {
		if !ValidUserBanReason(reason) {
			return User{}, errors.New("invalid public ban reason")
		}
		ban = &UserBan{Source: "manual", Reason: strings.TrimSpace(reason)}
	}

	var user User
	var changed bool
	if err := DB.Transaction(func(tx *gorm.DB) error {
		if err := lockForUpdate(tx).First(&user, userID).Error; err != nil {
			return err
		}
		if operatorRole < common.RoleAdminUser ||
			(operatorRole != common.RoleRootUser && operatorRole <= user.Role) ||
			(disable && user.Role == common.RoleRootUser) {
			return ErrUserBanForbidden
		}
		var err error
		changed, err = setUserBanWithTx(tx, &user, ban)
		return err
	}); err != nil {
		return User{}, err
	}
	if changed {
		return user, publishUserBanChange(userID)
	}
	return user, nil
}

// setUserBanWithTx changes the state and its public metadata together. The
// caller must hold the user row lock. Passing nil lifts the ban.
func setUserBanWithTx(tx *gorm.DB, user *User, ban *UserBan) (bool, error) {
	status := common.UserStatusEnabled
	if ban != nil {
		status = common.UserStatusDisabled
	}
	if user.Status == status {
		return false, nil
	}
	if ban == nil {
		if err := tx.Delete(&UserBan{UserID: user.Id}).Error; err != nil {
			return false, err
		}
	} else {
		ban.UserID = user.Id
		ban.BannedAt = common.GetTimestamp()
		if err := tx.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "user_id"}},
			DoUpdates: clause.AssignmentColumns([]string{"source", "reason", "banned_at"}),
		}).Create(ban).Error; err != nil {
			return false, err
		}
	}
	authVersion, err := IncrementUserAuthVersionWithTx(tx, user.Id)
	if err != nil {
		return false, err
	}
	if err := tx.Model(user).Update("status", status).Error; err != nil {
		return false, err
	}
	user.AuthVersion = authVersion
	return true, nil
}

func publishUserBanChange(userID int) error {
	cacheErr := PublishUserAuthCache(userID)
	_, sessionErr := RevokeAllUserSessions(userID, "user_security_changed")
	if err := InvalidateUserTokensCache(userID); err != nil {
		common.SysLog(common.LogText("failed to invalidate tokens cache for user %d: %s", userID, err.Error()))
	}
	return errors.Join(cacheErr, sessionErr)
}

type PublicUserBan struct {
	Account  string `json:"account"`
	Source   string `json:"source"`
	Reason   string `json:"reason,omitempty"`
	BannedAt int64  `json:"banned_at,omitempty"`
}

func ListPublicUserBans(offset, limit int) ([]PublicUserBan, bool, error) {
	// Historical bans with no reliable current-ban provenance are still shown.
	// An old jailbreak verdict could have been followed by an unban and a new ban.
	bans := make([]PublicUserBan, 0, limit+1)
	err := DB.Model(&User{}).
		Select("users.username AS account, COALESCE(user_bans.source, 'legacy') AS source, COALESCE(user_bans.reason, '') AS reason, COALESCE(user_bans.banned_at, 0) AS banned_at").
		Joins("LEFT JOIN user_bans ON user_bans.user_id = users.id").
		Where("users.status = ?", common.UserStatusDisabled).
		Order("COALESCE(user_bans.banned_at, 0) DESC").Order("users.id DESC").
		Offset(offset).Limit(limit + 1).Scan(&bans).Error
	if err != nil {
		return nil, false, err
	}
	hasMore := len(bans) > limit
	if hasMore {
		bans = bans[:limit]
	}
	return bans, hasMore, nil
}
