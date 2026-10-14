package model

import (
	"errors"
	"strings"

	"github.com/QuantumNous/new-api/common"
)

// JailbreakRecord is the audit trail of one anti-jailbreak verdict. The user's
// message is stored so an administrator can review what was sent; the verdict
// reason and the raw classifier reply are kept for false-positive reviews.
type JailbreakRecord struct {
	Id          int    `json:"id"`
	UserId      int    `json:"user_id" gorm:"index"`
	Username    string `json:"username" gorm:"type:varchar(64);index"`
	ChannelId   int    `json:"channel_id" gorm:"index"`
	ChannelName string `json:"channel_name" gorm:"type:varchar(128);default:''"`
	ModelName   string `json:"model_name" gorm:"type:varchar(128);default:''"`
	RequestId   string `json:"request_id" gorm:"type:varchar(64);default:''"`
	TokenId     int    `json:"token_id" gorm:"default:0"`
	TokenName   string `json:"token_name" gorm:"type:varchar(64);default:''"`
	// JudgeModel is the classifier model that produced the verdict.
	JudgeModel string `json:"judge_model" gorm:"type:varchar(128);default:''"`
	Reason     string `json:"reason" gorm:"type:text"`
	// Content is the user prompt that was judged (truncated when very long).
	Content string `json:"content" gorm:"type:text"`
	// RawVerdict keeps the classifier's raw reply; MySQL 5.7 rejects TEXT defaults.
	RawVerdict string `json:"raw_verdict" gorm:"type:text"`
	// Action is "banned" for a first hit, "already_banned" when the account was
	// disabled before, and "skipped_staff" when the target is an administrator.
	Action    string `json:"action" gorm:"type:varchar(16);default:'banned'"`
	BannedAt  int64  `json:"banned_at" gorm:"bigint;default:0"`
	CreatedAt int64  `json:"created_at" gorm:"bigint;index"`
}

// CreateJailbreakRecord stores one verdict. Records are created from the relay
// path, so failures are returned to the caller instead of being swallowed.
func CreateJailbreakRecord(record *JailbreakRecord) error {
	if record == nil {
		return errors.New("record is nil")
	}
	if record.CreatedAt == 0 {
		record.CreatedAt = common.GetTimestamp()
	}
	return DB.Create(record).Error
}

// BanUserForJailbreak disables the account of a confirmed jailbreak attempt and
// returns the account name plus the action taken. Administrators are never
// banned automatically.
func BanUserForJailbreak(userId int) (string, string, error) {
	if userId <= 0 {
		return "", "", errors.New("invalid userId")
	}
	var user User
	if err := DB.Where("id = ?", userId).First(&user).Error; err != nil {
		return "", "", err
	}
	if user.Role >= common.RoleAdminUser {
		return "skipped_staff", user.Username, nil
	}
	if user.Status != common.UserStatusEnabled {
		return "already_banned", user.Username, nil
	}
	user.Status = common.UserStatusDisabled
	if err := user.Update(false); err != nil {
		return "", user.Username, err
	}
	// Update publishes the user hash and revokes browser sessions; relay tokens
	// still need their own cache invalidation.
	if err := InvalidateUserTokensCache(user.Id); err != nil {
		common.SysLog("failed to invalidate tokens cache for jailbreak ban: " + err.Error())
	}
	return "banned", user.Username, nil
}

// GetJailbreakRecords returns the newest records first. An empty userId or
// channelId means "all"; keyword matches the username, channel name or content.
func GetJailbreakRecords(page int, pageSize int, userId int, channelId int, keyword string) ([]JailbreakRecord, int64, error) {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 || pageSize > 100 {
		pageSize = 20
	}
	query := DB.Model(&JailbreakRecord{})
	if userId > 0 {
		query = query.Where("user_id = ?", userId)
	}
	if channelId > 0 {
		query = query.Where("channel_id = ?", channelId)
	}
	if keyword = strings.TrimSpace(keyword); keyword != "" {
		like := "%" + keyword + "%"
		query = query.Where("username LIKE ? OR channel_name LIKE ? OR content LIKE ?", like, like, like)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var records []JailbreakRecord
	if err := query.Order("id desc").Offset((page - 1) * pageSize).Limit(pageSize).Find(&records).Error; err != nil {
		return nil, 0, err
	}
	return records, total, nil
}
