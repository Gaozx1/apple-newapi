package model

import (
	"errors"
	"fmt"
	"math"
	"strings"
	"sync"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/setting/ratio_setting"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// Store channel offer status.
const (
	StoreChannelStatusOffShelf  = 0
	StoreChannelStatusOnSale    = 1
	StoreChannelStatusTakenDown = 2
)

// Store channel report status.
const (
	StoreReportStatusPending  = 0
	StoreReportStatusRejected = 1
	StoreReportStatusAccepted = 2
)

const (
	// StoreChannelFeePermille is the platform commission on a channel sale, in
	// per-mille. 10 per-mille = 1%.
	StoreChannelFeePermille = 10
	// StoreChannelPurchaseGroup is the group every purchased user channel joins,
	// so buyers can route through it by turning the group on for a token.
	StoreChannelPurchaseGroup = "用户渠道"
	// StoreChannelHealthWindow is how many of the most recent requests decide the
	// health of a user channel.
	StoreChannelHealthWindow = 100
	// StoreChannelDisablePermille is the error rate that disables a user
	// channel: 800 per-mille = 80% of the last 100 requests failing.
	StoreChannelDisablePermille = 800
	// StoreChannelMaxModels bounds the model list one offer declares.
	StoreChannelMaxModels = 50
	// StoreChannelMinHealthSamples avoids disabling a channel on a tiny sample.
	StoreChannelMinHealthSamples = 20
	// StoreChannelMinRatio / StoreChannelMaxRatio bound the billing multiplier a
	// merchant may attach to an offer.
	StoreChannelMinRatio = 0.01
	StoreChannelMaxRatio = 20.0
)

var (
	ErrStoreChannelNotFound     = errors.New("渠道不存在")
	ErrStoreChannelOffShelf     = errors.New("渠道已下架")
	ErrStoreChannelNotOwned     = errors.New("只能管理自己上传的渠道")
	ErrStoreChannelNoModels     = errors.New("请至少填写一个模型")
	ErrStoreChannelBadInput     = errors.New("请填写渠道名称、接口地址和密钥")
	ErrStoreChannelTypeInvalid  = errors.New("渠道类型不受支持")
	ErrStoreChannelRatioInvalid = errors.New("倍率无效（允许 0.01 ~ 20）")
	ErrStoreReportNotFound      = errors.New("举报不存在")
	ErrStoreReportHandled       = errors.New("举报已处理")
)

// StoreChannel is an upstream a user offers to the store. Buying it creates a
// real channel in the 用户渠道 group so the gateway routes through it with no
// special code path.
type StoreChannel struct {
	Id          int    `json:"id"`
	UserId      int    `json:"user_id" gorm:"index"`
	Name        string `json:"name" gorm:"type:varchar(80);not null"`
	Description string `json:"description" gorm:"type:text"`
	ChannelType int    `json:"channel_type" gorm:"default:0"`
	BaseURL     string `json:"base_url" gorm:"type:varchar(255);default:''"`
	// Key is the merchant's upstream credential. It never leaves the server.
	Key    string `json:"-" gorm:"type:text"`
	Models string `json:"models" gorm:"type:text"`
	// Ratio is the price other users pay when this channel is listed on the
	// market; the owner's own traffic is always billed at 0.
	Ratio float64 `json:"ratio" gorm:"type:decimal(10,4);not null;default:1"`
	Price int     `json:"price" gorm:"default:0"`
	// GroupId is the owner's group this channel belongs to; the group carries the
	// billing ratio and the market listing.
	GroupId int `json:"group_id" gorm:"index"`
	// ChannelId is the single live channel this offer owns. Buyers route through
	// the same row; nothing is duplicated per buyer.
	ChannelId int `json:"channel_id" gorm:"index"`
	// GroupName is the owner's private group (u<userId>) that holds the channel
	// while it is not listed.
	GroupName string `json:"group_name" gorm:"type:varchar(64);default:''"`
	Status    int    `json:"status" gorm:"default:0;index"`
	// SoldCount counts how many users claimed this channel while it was listed.
	SoldCount int `json:"sold_count" gorm:"default:0"`
	// ErrorCount and RequestCount are the rolling health sample of the last
	// StoreChannelHealthWindow requests, refreshed by the health task.
	ErrorCount   int   `json:"error_count" gorm:"default:0"`
	RequestCount int   `json:"request_count" gorm:"default:0"`
	CreatedAt    int64 `json:"created_at" gorm:"bigint;index"`
	UpdatedAt    int64 `json:"updated_at" gorm:"bigint"`
}

func (StoreChannel) TableName() string {
	return "store_channels"
}

// StoreChannelOrder records one purchase, including the live channel it created.
type StoreChannelOrder struct {
	Id           int    `json:"id"`
	OrderNo      string `json:"order_no" gorm:"type:varchar(64);uniqueIndex"`
	OfferId      int    `json:"offer_id" gorm:"index"`
	ChannelId    int    `json:"channel_id" gorm:"index"`
	BuyerUserId  int    `json:"buyer_user_id" gorm:"index"`
	SellerUserId int    `json:"seller_user_id" gorm:"index"`
	Price        int    `json:"price"`
	// Ratio snapshots the multiplier the offer carried at purchase time.
	Ratio        float64 `json:"ratio" gorm:"type:decimal(10,4);not null;default:1"`
	Fee          int     `json:"fee"`
	SellerIncome int     `json:"seller_income"`
	CreatedAt    int64   `json:"created_at" gorm:"bigint;index"`
}

func (StoreChannelOrder) TableName() string {
	return "store_channel_orders"
}

// StoreChannelReport is one buyer complaint awaiting administrator review.
type StoreChannelReport struct {
	Id      int `json:"id"`
	OfferId int `json:"offer_id" gorm:"index"`
	// GroupId is set when the report targets a listed group instead of a channel.
	GroupId    int    `json:"group_id" gorm:"index"`
	ChannelId  int    `json:"channel_id" gorm:"index"`
	ReporterId int    `json:"reporter_id" gorm:"index"`
	Reason     string `json:"reason" gorm:"type:text"`
	Status     int    `json:"status" gorm:"default:0;index"`
	ReviewerId int    `json:"reviewer_id" gorm:"default:0"`
	ReviewNote string `json:"review_note" gorm:"type:text"`
	CreatedAt  int64  `json:"created_at" gorm:"bigint;index"`
	ReviewedAt int64  `json:"reviewed_at" gorm:"bigint;default:0"`
}

func (StoreChannelReport) TableName() string {
	return "store_channel_reports"
}

// storeChannelPriorityForRatio maps a billing multiplier onto the channel
// priority: the cheaper the multiplier, the higher the priority, so gateway
// routing tries the cheapest channel of a model first. The range stays inside
// 0..200, which leaves room for an administrator to place a manual channel above
// or below the whole store tier.
func storeChannelPriorityForRatio(ratio float64) int64 {
	if ratio <= 0 {
		ratio = StoreChannelMaxRatio
	}
	priority := int64(math.Round((StoreChannelMaxRatio - ratio) * 10))
	return min(max(priority, 0), 200)
}

// ResolveStoreChannelGroupRatios fills each offer's Ratio from the group it
// belongs to, so lists never show a stale price after the owner edited the group.
func ResolveStoreChannelGroupRatios(offers []*StoreChannel) {
	groupIds := make([]int, 0, len(offers))
	seen := make(map[int]struct{}, len(offers))
	for _, offer := range offers {
		if offer.GroupId <= 0 {
			continue
		}
		if _, ok := seen[offer.GroupId]; ok {
			continue
		}
		seen[offer.GroupId] = struct{}{}
		groupIds = append(groupIds, offer.GroupId)
	}
	if len(groupIds) == 0 {
		return
	}
	groups := make([]*UserChannelGroup, 0, len(groupIds))
	if err := DB.Select("id", "ratio").Where("id IN ?", groupIds).Find(&groups).Error; err != nil {
		return
	}
	ratios := make(map[int]float64, len(groups))
	for _, group := range groups {
		ratios[group.Id] = group.Ratio
	}
	for _, offer := range offers {
		if ratio, ok := ratios[offer.GroupId]; ok {
			offer.Ratio = ratio
		}
	}
}

// StoreChannelFeeForPrice charges the 1% commission, rounded up so a cheaper
// sale still pays the platform, and never above the price itself.
func StoreChannelFeeForPrice(price int) int {
	if price <= 0 {
		return 0
	}
	fee := (price*StoreChannelFeePermille + 999) / 1000
	if fee > price {
		fee = price
	}
	return fee
}

// NormalizeStoreChannelOffer validates merchant input and returns a clean offer.
// ratio is the billing multiplier: 1 = list price, 0.8 = 20% cheaper.
func NormalizeStoreChannelOffer(name, description string, channelType int, baseURL, key, models string) (*StoreChannel, error) {
	name = strings.TrimSpace(name)
	description = strings.TrimSpace(description)
	baseURL = strings.TrimSpace(baseURL)
	key = strings.TrimSpace(key)
	if name == "" || baseURL == "" || key == "" {
		return nil, ErrStoreChannelBadInput
	}
	if len([]rune(name)) > StoreMaxNameRunes {
		return nil, fmt.Errorf("渠道名称不能超过 %d 个字", StoreMaxNameRunes)
	}
	if len([]rune(description)) > StoreMaxDescriptionRunes {
		return nil, fmt.Errorf("渠道描述不能超过 %d 个字", StoreMaxDescriptionRunes)
	}
	if channelType < 0 || channelType >= len(constant.ChannelBaseURLs) {
		return nil, ErrStoreChannelTypeInvalid
	}
	cleaned := make([]string, 0, StoreChannelMaxModels)
	seen := make(map[string]struct{}, StoreChannelMaxModels)
	for _, raw := range strings.FieldsFunc(models, func(r rune) bool { return r == ',' || r == '\n' || r == ' ' }) {
		modelName := strings.TrimSpace(raw)
		if modelName == "" {
			continue
		}
		if _, exists := seen[modelName]; exists {
			continue
		}
		seen[modelName] = struct{}{}
		cleaned = append(cleaned, modelName)
		if len(cleaned) >= StoreChannelMaxModels {
			break
		}
	}
	if len(cleaned) == 0 {
		return nil, ErrStoreChannelNoModels
	}
	return &StoreChannel{
		Name:        name,
		Description: description,
		ChannelType: channelType,
		BaseURL:     strings.TrimRight(baseURL, "/"),
		Key:         key,
		Models:      strings.Join(cleaned, ","),
		Ratio:       1,
		// A new channel starts private to its owner; listing is an explicit step.
		Status: StoreChannelStatusOffShelf,
	}, nil
}

// UserChannelGroupPrefix marks a group as user-created wherever group names are
// listed, so nobody confuses it with the built-in groups.
const UserChannelGroupPrefix = "[用户]"

// UserChannelGroupPublicName is the name a user group is registered and routed
// under. The raw name stays as the owner typed it; only the public form carries
// the marker.
func UserChannelGroupPublicName(name string) string {
	if strings.HasPrefix(name, UserChannelGroupPrefix) {
		return name
	}
	return UserChannelGroupPrefix + name
}

// UserChannelGroupName is the private group a user's own channels live in. Only
// that user's tokens can select it, so an unlisted channel stays invisible to
// everyone else.
func UserChannelGroupName(userId int) string {
	return fmt.Sprintf("u%d", userId)
}

// StoreGroupClaim records that one user claimed a listed group. Only claimed
// users (and the owner) may route through the group's channels.
type StoreGroupClaim struct {
	Id        int   `json:"id"`
	GroupId   int   `json:"group_id" gorm:"uniqueIndex:idx_store_claim,priority:1"`
	UserId    int   `json:"user_id" gorm:"uniqueIndex:idx_store_claim,priority:2;index"`
	CreatedAt int64 `json:"created_at" gorm:"bigint"`
}

func (StoreGroupClaim) TableName() string {
	return "store_group_claims"
}

var ErrStoreGroupNotListed = errors.New("分组未上架")
var ErrStoreGroupSelfClaim = errors.New("自己的分组无需领取")

// ClaimStoreGroup records a claim on a listed group; claiming twice is a no-op.
func ClaimStoreGroup(userId int, groupId int) error {
	group := &UserChannelGroup{}
	if err := DB.Where("id = ?", groupId).First(group).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrStoreChannelNotFound
		}
		return err
	}
	if !group.Listed {
		return ErrStoreGroupNotListed
	}
	if group.UserId == userId {
		return ErrStoreGroupSelfClaim
	}
	claim := &StoreGroupClaim{GroupId: groupId, UserId: userId, CreatedAt: common.GetTimestamp()}
	return DB.Clauses(clause.OnConflict{DoNothing: true}).Create(claim).Error
}

// UnclaimStoreGroup drops a claim.
func UnclaimStoreGroup(userId int, groupId int) error {
	return DB.Where("group_id = ? AND user_id = ?", groupId, userId).Delete(&StoreGroupClaim{}).Error
}

// ListStoreGroupClaims lists the groups one user claimed.
func ListStoreGroupClaims(userId int) ([]*StoreGroupClaim, error) {
	claims := make([]*StoreGroupClaim, 0, 8)
	if err := DB.Where("user_id = ?", userId).Find(&claims).Error; err != nil {
		return nil, err
	}
	return claims, nil
}

// ListMarketStoreGroups lists every group currently on the market.
func ListMarketStoreGroups() ([]*UserChannelGroup, error) {
	groups := make([]*UserChannelGroup, 0, 16)
	if err := DB.Where("listed = ?", true).Order("id desc").Find(&groups).Error; err != nil {
		return nil, err
	}
	return groups, nil
}

// StoreChannelAccessAllowed reports whether the user may run requests through the
// channel. Only store channels in user groups are gated: the owner and users who
// claimed the group pass; everything else is untouched. A lookup failure fails
// open so billing never breaks the relay.
func StoreChannelAccessAllowed(channelId int, userId int) (allowed bool) {
	if channelId <= 0 || userId <= 0 {
		return true
	}
	defer func() {
		if recovered := recover(); recovered != nil {
			common.SysLog(fmt.Sprintf("store channel access check failed for channel %d: %v", channelId, recovered))
			allowed = true
		}
	}()
	offer := &StoreChannel{}
	if err := DB.Select("id", "user_id", "group_id").Where("channel_id = ?", channelId).First(offer).Error; err != nil {
		return true
	}
	if offer.UserId == userId {
		return true
	}
	if offer.GroupId <= 0 {
		return true
	}
	count := int64(0)
	if err := DB.Model(&StoreGroupClaim{}).Where("group_id = ? AND user_id = ?", offer.GroupId, userId).Count(&count).Error; err != nil {
		return true
	}
	return count > 0
}

// FilterModelSquareUserGroups narrows the model-square group filter to the groups
// a buyer may actually route through: listed groups plus the ones they claimed.
// An unlisted group stays invisible on the market side, even to its owner.
func FilterModelSquareUserGroups(groups map[string]string, userId int) map[string]string {
	return filterUserGroups(groups, userId, false)
}

// FilterOwnedOrClaimedUserGroups keeps a user's own groups and the ones they
// claimed, which is what the token group selector needs.
func FilterOwnedOrClaimedUserGroups(groups map[string]string, userId int) map[string]string {
	return filterUserGroups(groups, userId, true)
}

func filterUserGroups(groups map[string]string, userId int, includeOwned bool) map[string]string {
	if len(groups) == 0 {
		return groups
	}
	all := make([]*UserChannelGroup, 0, 16)
	if err := DB.Find(&all).Error; err != nil || len(all) == 0 {
		return groups
	}
	claims := make(map[int]struct{}, 8)
	claimRows := make([]*StoreGroupClaim, 0, 8)
	if err := DB.Where("user_id = ?", userId).Find(&claimRows).Error; err == nil {
		for _, claim := range claimRows {
			claims[claim.GroupId] = struct{}{}
		}
	}
	filtered := make(map[string]string, len(groups))
	for name, desc := range groups {
		if !strings.HasPrefix(name, UserChannelGroupPrefix) {
			filtered[name] = desc
			continue
		}
		allowed := false
		for _, group := range all {
			if UserChannelGroupPublicName(group.Name) != name {
				continue
			}
			// The model square lists only what the buyer actually claimed, so an
			// unclaimed group never leaks into their group filter even when the
			// owner published it. The token selector additionally keeps owned
			// groups so a merchant can still reach their own channels.
			allowed = includeOwned && group.UserId == userId
			if _, ok := claims[group.Id]; ok {
				allowed = true
			}
			break
		}
		if allowed {
			filtered[name] = desc
		}
	}
	return filtered
}

// UserChannelGroup is a group a user created for their own channels. Only the
// owner sees it, and listing one of its channels on the market is what makes it
// reachable from the market group.
type UserChannelGroup struct {
	Id     int    `json:"id"`
	UserId int    `json:"user_id" gorm:"index"`
	Name   string `json:"name" gorm:"type:varchar(64);not null"`
	// Ratio is what other users pay while this group is listed on the market.
	Ratio  float64 `json:"ratio" gorm:"type:decimal(10,4);not null;default:1"`
	Remark string  `json:"remark" gorm:"type:varchar(255);default:''"`
	// Listed publishes every channel of this group to the market group.
	Listed    bool  `json:"listed" gorm:"default:false"`
	CreatedAt int64 `json:"created_at" gorm:"bigint"`
	UpdatedAt int64 `json:"updated_at" gorm:"bigint"`
}

func (UserChannelGroup) TableName() string {
	return "user_channel_groups"
}

var ErrUserGroupNameInvalid = errors.New("分组名称无效")
var ErrUserGroupExists = errors.New("分组已存在")

// normalizeUserGroupName keeps a group name safe to paste into a token: it must
// not contain the separators the gateway uses for group lists.
func normalizeUserGroupName(name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" || len([]rune(name)) > 32 {
		return "", ErrUserGroupNameInvalid
	}
	for _, forbidden := range []string{",", " ", "\n", "\t"} {
		if strings.Contains(name, forbidden) {
			return "", ErrUserGroupNameInvalid
		}
	}
	return name, nil
}

// CreateUserChannelGroup registers a group for the caller and makes it selectable
// for their tokens.
func CreateUserChannelGroup(userId int, name string, ratio float64, remark string) (*UserChannelGroup, error) {
	name, err := normalizeUserGroupName(name)
	if err != nil {
		return nil, err
	}
	existing := int64(0)
	if err := DB.Model(&UserChannelGroup{}).Where("user_id = ? AND name = ?", userId, name).Count(&existing).Error; err != nil {
		return nil, err
	}
	if existing > 0 {
		return nil, ErrUserGroupExists
	}
	if err := registerChannelGroup(UserChannelGroupPublicName(name), ratio); err != nil {
		return nil, err
	}
	now := common.GetTimestamp()
	group := &UserChannelGroup{
		UserId:    userId,
		Name:      name,
		Ratio:     ratio,
		Remark:    strings.TrimSpace(remark),
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := DB.Create(group).Error; err != nil {
		return nil, err
	}
	return group, nil
}

// ListUserChannelGroups lists the groups the caller owns.
func ListUserChannelGroups(userId int) ([]*UserChannelGroup, error) {
	groups := make([]*UserChannelGroup, 0, 8)
	if err := DB.Where("user_id = ?", userId).Order("id").Find(&groups).Error; err != nil {
		return nil, err
	}
	return groups, nil
}

// UpdateUserChannelGroup edits the ratio and remark of an owned group.
func UpdateUserChannelGroup(groupId int, userId int, ratio float64, remark string) error {
	if ratio < StoreChannelMinRatio || ratio > StoreChannelMaxRatio {
		return ErrStoreChannelRatioInvalid
	}
	return DB.Transaction(func(tx *gorm.DB) error {
		result := tx.Model(&UserChannelGroup{}).Where("id = ? AND user_id = ?", groupId, userId).
			Updates(map[string]any{
				"ratio":      ratio,
				"remark":     strings.TrimSpace(remark),
				"updated_at": common.GetTimestamp(),
			})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return ErrStoreChannelNotOwned
		}
		if err := tx.Model(&StoreChannel{}).Where("group_id = ?", groupId).
			Update("ratio", ratio).Error; err != nil {
			return err
		}
		group := &UserChannelGroup{}
		if err := tx.Where("id = ?", groupId).First(group).Error; err != nil {
			return err
		}
		return registerChannelGroup(UserChannelGroupPublicName(group.Name), ratio)
	})
}

// SetUserChannelGroupListing publishes or hides a whole group: every channel in it
// joins or leaves the market group, and nobody gets a duplicated channel.
func SetUserChannelGroupListing(groupId int, userId int, listed bool) error {
	return DB.Transaction(func(tx *gorm.DB) error {
		group := &UserChannelGroup{}
		if err := lockForUpdate(tx).Where("id = ?", groupId).First(group).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrStoreChannelNotFound
			}
			return err
		}
		if group.UserId != userId {
			return ErrStoreChannelNotOwned
		}
		if err := tx.Model(&UserChannelGroup{}).Where("id = ?", groupId).Updates(map[string]any{
			"listed":     listed,
			"updated_at": common.GetTimestamp(),
		}).Error; err != nil {
			return err
		}
		channels := make([]*StoreChannel, 0, 8)
		if err := tx.Where("group_id = ?", groupId).Find(&channels).Error; err != nil {
			return err
		}
		publicName := UserChannelGroupPublicName(group.Name)
		for _, offer := range channels {
			groupNames := publicName
			if listed {
				groupNames = publicName + "," + StoreChannelPurchaseGroup
			}
			if err := tx.Model(&StoreChannel{}).Where("id = ?", offer.Id).Updates(map[string]any{
				"status":     map[bool]int{true: StoreChannelStatusOnSale, false: StoreChannelStatusOffShelf}[listed],
				"group_name": publicName,
				"updated_at": common.GetTimestamp(),
			}).Error; err != nil {
				return err
			}
			if offer.ChannelId == 0 {
				continue
			}
			if err := tx.Model(&Channel{}).Where("id = ?", offer.ChannelId).
				Update("group", groupNames).Error; err != nil {
				return err
			}
			live := &Channel{}
			if err := tx.Where("id = ?", offer.ChannelId).First(live).Error; err != nil {
				continue
			}
			if err := syncStoreChannelAbilities(tx, live); err != nil {
				return err
			}
		}
		return nil
	})
}

// DeleteUserChannelGroup removes an owned group together with its channels.
func DeleteUserChannelGroup(groupId int, userId int) error {
	return DB.Transaction(func(tx *gorm.DB) error {
		group := &UserChannelGroup{}
		if err := lockForUpdate(tx).Where("id = ?", groupId).First(group).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrStoreChannelNotFound
			}
			return err
		}
		if group.UserId != userId {
			return ErrStoreChannelNotOwned
		}
		channels := make([]*StoreChannel, 0, 8)
		if err := tx.Where("group_id = ?", groupId).Find(&channels).Error; err != nil {
			return err
		}
		for _, offer := range channels {
			if offer.ChannelId > 0 {
				if err := tx.Model(&Channel{}).Where("id = ?", offer.ChannelId).
					Update("status", common.ChannelStatusManuallyDisabled).Error; err != nil {
					return err
				}
			}
		}
		if err := tx.Where("group_id = ?", groupId).Delete(&StoreChannel{}).Error; err != nil {
			return err
		}
		return tx.Where("id = ?", groupId).Delete(&UserChannelGroup{}).Error
	})
}

// CountStoreChannelsPerGroup counts how many personal channels sit in each group
// of one user, so the group list can show the size of every group.
func CountStoreChannelsPerGroup(userId int) (map[int]int, error) {
	type row struct {
		GroupId int
		Total   int64
	}
	rows := make([]row, 0, 8)
	if err := DB.Model(&StoreChannel{}).
		Select("group_id, COUNT(*) AS total").
		Where("user_id = ? AND group_id > 0", userId).
		Group("group_id").
		Find(&rows).Error; err != nil {
		return nil, err
	}
	counts := make(map[int]int, len(rows))
	for _, r := range rows {
		counts[r.GroupId] = int(r.Total)
	}
	return counts, nil
}

// ownsUserChannelGroup reports whether the caller owns that group name.
func ownsUserChannelGroup(userId int, name string) bool {
	count := int64(0)
	if err := DB.Model(&UserChannelGroup{}).Where("user_id = ? AND name = ?", userId, name).Count(&count).Error; err != nil {
		return false
	}
	return count > 0
}

// registerChannelGroup makes a group selectable for tokens and keeps its billing
// multiplier in sync. The token selector reads UserUsableGroups while pricing
// reads GroupRatio, so a stale GroupRatio shows the wrong price in the UI.
func registerChannelGroup(name string, ratio float64) error {
	if ratio <= 0 {
		ratio = 1
	}
	groupRatios := ratio_setting.GetGroupRatioCopy()
	if current, exists := groupRatios[name]; !exists || current != ratio {
		groupRatios[name] = ratio
		payload, err := common.Marshal(groupRatios)
		if err != nil {
			return err
		}
		if err := UpdateOption("GroupRatio", string(payload)); err != nil {
			return err
		}
	}
	usableGroups := setting.GetUserUsableGroupsCopy()
	if _, exists := usableGroups[name]; exists {
		return nil
	}
	usableGroups[name] = "个人分组"
	payload, err := common.Marshal(usableGroups)
	if err != nil {
		return err
	}
	return UpdateOption("UserUsableGroups", string(payload))
}

// EnsureUserChannelGroup registers that private group so it can be picked for a
// token. It is idempotent; the owner's own traffic is made free by the per
// request billing override, not by the group ratio.
func EnsureUserChannelGroup(userId int) (string, error) {
	name := UserChannelGroupName(userId)
	if ratio_setting.ContainsGroupRatio(UserChannelGroupPublicName(name)) {
		return name, nil
	}
	if err := registerChannelGroup(UserChannelGroupPublicName(name), 1); err != nil {
		return "", err
	}
	return name, nil
}

// channelGroups is the channel's routable group list: the owner's group always,
// the market group only while the offer is listed.
func (offer *StoreChannel) channelGroups() string {
	if offer.Status == StoreChannelStatusOnSale {
		return offer.GroupName + "," + StoreChannelPurchaseGroup
	}
	return offer.GroupName
}

// syncStoreChannelAbilities rewrites the ability rows so the channel is reachable
// from exactly the groups in channel.Group.
func syncStoreChannelAbilities(tx *gorm.DB, channel *Channel) error {
	if err := tx.Where("channel_id = ?", channel.Id).Delete(&Ability{}).Error; err != nil {
		return err
	}
	return channel.AddAbilities(tx)
}

// CreateStoreChannel registers the owner's private group, creates the single real
// channel behind the offer, and stores the offer itself.
func CreateStoreChannel(offer *StoreChannel) error {
	if offer.GroupId > 0 {
		group := &UserChannelGroup{}
		if err := DB.Where("id = ? AND user_id = ?", offer.GroupId, offer.UserId).First(group).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrStoreChannelNotOwned
			}
			return err
		}
		offer.GroupName = UserChannelGroupPublicName(group.Name)
		offer.Ratio = group.Ratio
		if group.Listed {
			offer.Status = StoreChannelStatusOnSale
		}
		// The ratio stays on the group; the offer column is only a legacy mirror.
	} else {
		groupName := strings.TrimSpace(offer.GroupName)
		if groupName == "" {
			groupName = UserChannelGroupName(offer.UserId)
			if _, err := EnsureUserChannelGroup(offer.UserId); err != nil {
				return err
			}
		} else if !ownsUserChannelGroup(offer.UserId, groupName) {
			return ErrUserGroupNameInvalid
		}
		offer.GroupName = UserChannelGroupPublicName(groupName)
	}
	now := common.GetTimestamp()
	offer.CreatedAt = now
	offer.UpdatedAt = now

	return DB.Transaction(func(tx *gorm.DB) error {
		baseURL := offer.BaseURL
		remark := fmt.Sprintf("商店渠道，卖家 #%d，倍率 %.4g", offer.UserId, offer.Ratio)
		priority := storeChannelPriorityForRatio(offer.Ratio)
		channel := &Channel{
			Type:     offer.ChannelType,
			Key:      offer.Key,
			BaseURL:  &baseURL,
			Name:     offer.Name,
			Models:   offer.Models,
			Group:    offer.channelGroups(),
			Status:   common.ChannelStatusEnabled,
			Priority: &priority,
			Remark:   &remark,
		}
		if err := tx.Create(channel).Error; err != nil {
			return err
		}
		if err := syncStoreChannelAbilities(tx, channel); err != nil {
			return err
		}
		offer.ChannelId = channel.Id
		return tx.Create(offer).Error
	})
}

// SetStoreChannelListing lists or unlists an owned offer: the channel moves in or
// out of the market group, and no buyer row is created either way.
func SetStoreChannelListing(offerId int, userId int, listed bool) error {
	return DB.Transaction(func(tx *gorm.DB) error {
		offer := &StoreChannel{}
		if err := lockForUpdate(tx).Where("id = ?", offerId).First(offer).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrStoreChannelNotFound
			}
			return err
		}
		if offer.UserId != userId {
			return ErrStoreChannelNotOwned
		}
		if offer.GroupName == "" {
			offer.GroupName = UserChannelGroupName(offer.UserId)
		}
		status := StoreChannelStatusOffShelf
		if listed {
			status = StoreChannelStatusOnSale
		}
		offer.Status = status
		if err := tx.Model(&StoreChannel{}).Where("id = ?", offerId).Updates(map[string]any{
			"status":     status,
			"group_name": offer.GroupName,
			"updated_at": common.GetTimestamp(),
		}).Error; err != nil {
			return err
		}
		channel := &Channel{}
		if err := tx.Where("id = ?", offer.ChannelId).First(channel).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil
			}
			return err
		}
		channel.Group = offer.channelGroups()
		if err := tx.Model(&Channel{}).Where("id = ?", channel.Id).Update("group", channel.Group).Error; err != nil {
			return err
		}
		return syncStoreChannelAbilities(tx, channel)
	})
}

func GetStoreChannelById(id int) (*StoreChannel, error) {
	offer := &StoreChannel{}
	if err := DB.Where("id = ?", id).First(offer).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrStoreChannelNotFound
		}
		return nil, err
	}
	return offer, nil
}

// ListStoreChannels lists offers on sale for the store front page.
func ListStoreChannels(keyword string, page int, pageSize int) ([]*StoreChannel, int64, error) {
	query := DB.Model(&StoreChannel{}).Where("status = ?", StoreChannelStatusOnSale)
	if keyword = strings.TrimSpace(keyword); keyword != "" {
		like := "%" + keyword + "%"
		query = query.Where("name LIKE ? OR description LIKE ? OR models LIKE ?", like, like, like)
	}
	total := int64(0)
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	offers := make([]*StoreChannel, 0, pageSize)
	if err := query.Order("id desc").Offset((page - 1) * pageSize).Limit(pageSize).Find(&offers).Error; err != nil {
		return nil, 0, err
	}
	ResolveStoreChannelGroupRatios(offers)
	return offers, total, nil
}

// ListUserStoreChannels lists the offers one merchant uploaded.
func ListUserStoreChannels(userId int, page int, pageSize int) ([]*StoreChannel, int64, error) {
	query := DB.Model(&StoreChannel{}).Where("user_id = ?", userId)
	total := int64(0)
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	offers := make([]*StoreChannel, 0, pageSize)
	if err := query.Order("id desc").Offset((page - 1) * pageSize).Limit(pageSize).Find(&offers).Error; err != nil {
		return nil, 0, err
	}
	ResolveStoreChannelGroupRatios(offers)
	return offers, total, nil
}

// UpdateStoreChannelOffer rewrites the editable fields of an owned offer and keeps
// the live channel behind it in sync.
func UpdateStoreChannelOffer(offer *StoreChannel) error {
	offer.UpdatedAt = common.GetTimestamp()
	return DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&StoreChannel{}).Where("id = ? AND user_id = ?", offer.Id, offer.UserId).
			Updates(map[string]any{
				"name":         offer.Name,
				"description":  offer.Description,
				"channel_type": offer.ChannelType,
				"base_url":     offer.BaseURL,
				"key":          offer.Key,
				"models":       offer.Models,
				"ratio":        offer.Ratio,
				"price":        offer.Price,
				"status":       offer.Status,
				"updated_at":   offer.UpdatedAt,
			}).Error; err != nil {
			return err
		}
		if offer.ChannelId == 0 {
			return nil
		}
		if offer.GroupId == 0 && offer.GroupName != "" {
			group := &UserChannelGroup{}
			rawName := strings.TrimPrefix(offer.GroupName, UserChannelGroupPrefix)
			if err := tx.Where("user_id = ? AND name = ?", offer.UserId, rawName).First(group).Error; err == nil {
				offer.GroupId = group.Id
				offer.Ratio = group.Ratio
				if err := tx.Model(&StoreChannel{}).Where("id = ?", offer.Id).Updates(map[string]any{
					"group_id": group.Id,
					"ratio":    group.Ratio,
				}).Error; err != nil {
					return err
				}
			}
		}
		priority := storeChannelPriorityForRatio(offer.Ratio)
		if err := tx.Model(&Channel{}).Where("id = ?", offer.ChannelId).Updates(map[string]any{
			"type":     offer.ChannelType,
			"key":      offer.Key,
			"base_url": offer.BaseURL,
			"models":   offer.Models,
			"name":     offer.Name,
			"priority": priority,
		}).Error; err != nil {
			return err
		}
		loaded := &Channel{}
		if err := tx.Where("id = ?", offer.ChannelId).First(loaded).Error; err != nil {
			return err
		}
		return syncStoreChannelAbilities(tx, loaded)
	})
}

// DeleteStoreChannelOffer removes an offer and disables the channels it created,
// so a deleted listing cannot keep serving traffic.
func DeleteStoreChannelOffer(offerId int, userId int) error {
	return DB.Transaction(func(tx *gorm.DB) error {
		offer := &StoreChannel{}
		if err := lockForUpdate(tx).Where("id = ?", offerId).First(offer).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrStoreChannelNotFound
			}
			return err
		}
		if offer.UserId != userId {
			return ErrStoreChannelNotOwned
		}
		if offer.ChannelId > 0 {
			if err := tx.Model(&Channel{}).Where("id = ?", offer.ChannelId).
				Update("status", common.ChannelStatusManuallyDisabled).Error; err != nil {
				return err
			}
		}
		return tx.Where("id = ?", offerId).Delete(&StoreChannel{}).Error
	})
}

// ensureStoreChannelAbilities rebuilds the ability rows of one offer when they no
// longer match the channel's current group and model list. Group renames used to
// leave a channel with zero abilities, which silently made it unroutable.
func ensureStoreChannelAbilities(offer *StoreChannel) {
	if offer.ChannelId <= 0 {
		return
	}
	channel := &Channel{}
	if err := DB.First(channel, offer.ChannelId).Error; err != nil {
		return
	}
	want := make(map[string]struct{}, 8)
	for _, name := range strings.Split(channel.Group, ",") {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		for _, modelName := range strings.Split(channel.Models, ",") {
			modelName = strings.TrimSpace(modelName)
			if modelName == "" {
				continue
			}
			want[name+"|"+modelName] = struct{}{}
		}
	}
	existing := make([]*Ability, 0, 8)
	if err := DB.Where("channel_id = ?", channel.Id).Find(&existing).Error; err != nil {
		return
	}
	have := make(map[string]struct{}, len(existing))
	for _, ability := range existing {
		have[ability.Group+"|"+ability.Model] = struct{}{}
	}
	if len(have) == len(want) {
		matched := true
		for key := range want {
			if _, ok := have[key]; !ok {
				matched = false
				break
			}
		}
		if matched {
			return
		}
	}
	if err := syncStoreChannelAbilities(DB, channel); err != nil {
		common.SysLog("failed to repair store channel abilities: " + err.Error())
	}
}

// RefreshStoreChannelHealth recomputes the rolling health sample of every
// active user channel from the log table and disables the ones whose error rate
// crossed the threshold. It returns the disabled offers.
func RefreshStoreChannelHealth() ([]int, error) {
	offers := make([]*StoreChannel, 0, 32)
	if err := DB.Where("status = ?", StoreChannelStatusOnSale).Find(&offers).Error; err != nil {
		return nil, err
	}
	disabled := make([]int, 0, 4)
	for _, offer := range offers {
		if offer.ChannelId <= 0 {
			continue
		}
		ensureStoreChannelAbilities(offer)
		requests, failures, err := storeChannelHealthSample([]int{offer.ChannelId})
		if err != nil {
			return disabled, err
		}
		if err := DB.Model(&StoreChannel{}).Where("id = ?", offer.Id).
			Updates(map[string]any{"request_count": requests, "error_count": failures, "updated_at": common.GetTimestamp()}).Error; err != nil {
			return disabled, err
		}
		if requests < StoreChannelMinHealthSamples {
			continue
		}
		if failures*1000 < requests*StoreChannelDisablePermille {
			continue
		}
		if err := DB.Model(&StoreChannel{}).Where("id = ?", offer.Id).
			Update("status", StoreChannelStatusTakenDown).Error; err != nil {
			return disabled, err
		}
		if err := DB.Model(&Channel{}).Where("id = ?", offer.ChannelId).
			Update("status", common.ChannelStatusAutoDisabled).Error; err != nil {
			return disabled, err
		}
		disabled = append(disabled, offer.Id)
		common.SysLog(fmt.Sprintf("store channel offer %d disabled: %d/%d requests failed", offer.Id, failures, requests))
	}
	return disabled, nil
}

// storeChannelHealthSample inspects the most recent requests of the given
// channels: consume logs are successes, error logs are failures. The sample is
// the rolling window the threshold is expressed in.
func storeChannelHealthSample(channelIds []int) (requests int, failures int, err error) {
	if len(channelIds) == 0 {
		return 0, 0, nil
	}
	rows := make([]Log, 0, StoreChannelHealthWindow)
	if err := LOG_DB.Model(&Log{}).
		Select("id", "type").
		Where("channel_id IN ? AND type IN ?", channelIds, []int{LogTypeConsume, LogTypeError}).
		Order("id desc").
		Limit(StoreChannelHealthWindow).
		Find(&rows).Error; err != nil {
		return 0, 0, err
	}
	for _, row := range rows {
		requests++
		if row.Type == LogTypeError {
			failures++
		}
	}
	return requests, failures, nil
}

// CreateStoreChannelReport files one complaint; duplicates from the same buyer
// on the same offer are ignored.
func CreateStoreChannelReport(offerId int, channelId int, reporterId int, reason string) error {
	return createStoreReport(offerId, 0, channelId, reporterId, reason)
}

// CreateStoreGroupReport files a complaint about a listed group.
func CreateStoreGroupReport(groupId int, reporterId int, reason string) error {
	return createStoreReport(0, groupId, 0, reporterId, reason)
}

func createStoreReport(offerId int, groupId int, channelId int, reporterId int, reason string) error {
	reason = strings.TrimSpace(reason)
	if len([]rune(reason)) == 0 {
		return ErrStoreChannelBadInput
	}
	if len([]rune(reason)) > 500 {
		reason = string([]rune(reason)[:500])
	}
	existing := int64(0)
	if err := DB.Model(&StoreChannelReport{}).
		Where("offer_id = ? AND reporter_id = ? AND status = ?", offerId, reporterId, StoreReportStatusPending).
		Count(&existing).Error; err != nil {
		return err
	}
	if existing > 0 {
		return ErrStoreReportHandled
	}
	return DB.Create(&StoreChannelReport{
		OfferId:    offerId,
		GroupId:    groupId,
		ChannelId:  channelId,
		ReporterId: reporterId,
		Reason:     reason,
		Status:     StoreReportStatusPending,
		CreatedAt:  common.GetTimestamp(),
	}).Error
}

// ListStoreChannelReports lists reports for the administrator, newest first.
func ListStoreChannelReports(status int, page int, pageSize int) ([]*StoreChannelReport, int64, error) {
	query := DB.Model(&StoreChannelReport{})
	if status >= 0 {
		query = query.Where("status = ?", status)
	}
	total := int64(0)
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	reports := make([]*StoreChannelReport, 0, pageSize)
	if err := query.Order("id desc").Offset((page - 1) * pageSize).Limit(pageSize).Find(&reports).Error; err != nil {
		return nil, 0, err
	}
	return reports, total, nil
}

// ReviewStoreChannelReport applies the administrator decision. Accepting a
// report takes the offer down and disables every channel it created.
func ReviewStoreChannelReport(reportId int, reviewerId int, accept bool, note string) error {
	return DB.Transaction(func(tx *gorm.DB) error {
		report := &StoreChannelReport{}
		if err := lockForUpdate(tx).Where("id = ?", reportId).First(report).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrStoreReportNotFound
			}
			return err
		}
		if report.Status != StoreReportStatusPending {
			return ErrStoreReportHandled
		}
		now := common.GetTimestamp()
		status := StoreReportStatusRejected
		if accept {
			status = StoreReportStatusAccepted
		}
		if err := tx.Model(&StoreChannelReport{}).Where("id = ?", reportId).Updates(map[string]any{
			"status":      status,
			"reviewer_id": reviewerId,
			"review_note": strings.TrimSpace(note),
			"reviewed_at": now,
		}).Error; err != nil {
			return err
		}
		if !accept {
			return nil
		}
		if err := tx.Model(&StoreChannel{}).Where("id = ?", report.OfferId).
			Update("status", StoreChannelStatusTakenDown).Error; err != nil {
			return err
		}
		offer := &StoreChannel{}
		if err := tx.Where("id = ?", report.OfferId).First(offer).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil
			}
			return err
		}
		if offer.ChannelId > 0 {
			if err := tx.Model(&Channel{}).Where("id = ?", offer.ChannelId).
				Update("status", common.ChannelStatusManuallyDisabled).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

// StoreChannelEarnings is one settled revenue share: the buyer's consumed quota
// on a store channel, the platform fee, and what the merchant received.
type StoreChannelEarnings struct {
	Id           int    `json:"id"`
	ChannelId    int    `json:"channel_id" gorm:"index:idx_store_earning_channel,priority:1"`
	RequestId    string `json:"request_id" gorm:"type:varchar(64);not null;uniqueIndex"`
	OfferId      int    `json:"offer_id" gorm:"index"`
	SellerUserId int    `json:"seller_user_id" gorm:"index"`
	BuyerUserId  int    `json:"buyer_user_id" gorm:"index"`
	Quota        int    `json:"quota"`
	Fee          int    `json:"fee"`
	Income       int    `json:"income"`
	// ModelName is the model the buyer used, for the seller's revenue history.
	ModelName string  `json:"model_name" gorm:"type:varchar(128);default:''"`
	Ratio     float64 `json:"ratio" gorm:"type:decimal(10,4);not null;default:1"`
	CreatedAt int64   `json:"created_at" gorm:"bigint;index"`
}

func (StoreChannelEarnings) TableName() string {
	return "store_channel_earnings"
}

// StoreChannelLookup caches channel -> offer lookups so the relay hot path does
// not hit the database on every request.
var storeChannelLookup sync.Map

type storeChannelLookupEntry struct {
	offerId int
	ratio   float64
	seller  int
	expires int64
}

// StoreChannelByChannelId resolves the offer behind a live channel.
func StoreChannelByChannelId(channelId int) (offerId int, sellerId int, ratio float64, ok bool) {
	if channelId <= 0 {
		return 0, 0, 0, false
	}
	// This runs inside relay billing: an unexpected failure must never take the
	// request down, so a panic degrades to "bill normally" and gets logged.
	defer func() {
		if recovered := recover(); recovered != nil {
			common.SysLog(fmt.Sprintf("store channel lookup failed for channel %d: %v", channelId, recovered))
			offerId, sellerId, ratio, ok = 0, 0, 0, false
		}
	}()
	if cached, hit := storeChannelLookup.Load(channelId); hit {
		entry := cached.(storeChannelLookupEntry)
		if entry.expires > common.GetTimestamp() {
			return entry.offerId, entry.seller, entry.ratio, true
		}
	}
	offer := &StoreChannel{}
	if err := DB.Select("id", "user_id", "ratio", "group_id").Where("channel_id = ?", channelId).First(offer).Error; err != nil {
		return 0, 0, 0, false
	}
	ratio = offer.Ratio
	if offer.GroupId > 0 {
		group := &UserChannelGroup{}
		if err := DB.Select("id", "user_id", "ratio", "name").First(group, offer.GroupId).Error; err == nil {
			ratio = group.Ratio
		}
	}
	storeChannelLookup.Store(channelId, storeChannelLookupEntry{
		offerId: offer.Id,
		seller:  offer.UserId,
		ratio:   ratio,
		expires: common.GetTimestamp() + 60,
	})
	return offer.Id, offer.UserId, ratio, true
}

// CreditStoreChannelRevenue pays the merchant the quota the buyer consumed on
// this channel minus the platform fee. The unique request id makes a retry a
// no-op, so a settlement retry never pays twice.
func CreditStoreChannelRevenue(channelId int, buyerUserId int, quota int, requestId string, modelName string) error {
	if channelId <= 0 || quota <= 0 || strings.TrimSpace(requestId) == "" {
		return nil
	}
	offerId, sellerId, ratio, ok := StoreChannelByChannelId(channelId)
	if !ok || sellerId == buyerUserId {
		return nil
	}
	fee := StoreChannelFeeForPrice(quota)
	income := quota - fee
	if income <= 0 {
		return nil
	}
	return DB.Transaction(func(tx *gorm.DB) error {
		earning := &StoreChannelEarnings{
			ChannelId:    channelId,
			RequestId:    requestId,
			OfferId:      offerId,
			SellerUserId: sellerId,
			BuyerUserId:  buyerUserId,
			Quota:        quota,
			Fee:          fee,
			Income:       income,
			ModelName:    modelName,
			Ratio:        ratio,
			CreatedAt:    common.GetTimestamp(),
		}
		result := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(earning)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return nil
		}
		credit := tx.Model(&User{}).Where("id = ? AND quota <= ?", sellerId, common.MaxWalletQuota-income).
			Update("quota", gorm.Expr("quota + ?", income))
		if credit.Error != nil {
			return credit.Error
		}
		if credit.RowsAffected == 0 {
			return ErrStoreSellerQuotaOverflow
		}
		return nil
	})
}

// ListStoreChannelEarnings lists one merchant's settled revenue shares.
func ListStoreChannelEarnings(sellerUserId int, page int, pageSize int) ([]*StoreChannelEarnings, int64, error) {
	query := DB.Model(&StoreChannelEarnings{}).Where("seller_user_id = ?", sellerUserId)
	total := int64(0)
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	rows := make([]*StoreChannelEarnings, 0, pageSize)
	if err := query.Order("id desc").Offset((page - 1) * pageSize).Limit(pageSize).Find(&rows).Error; err != nil {
		return nil, 0, err
	}
	return rows, total, nil
}

// StoreChannelEarningsSummary is the seller's running total.
type StoreChannelEarningsSummary struct {
	Orders       int64 `json:"orders"`
	Consumed     int64 `json:"consumed"`
	Fee          int64 `json:"fee"`
	Income       int64 `json:"income"`
	UniqueBuyers int64 `json:"unique_buyers"`
}

// SummarizeStoreChannelEarnings totals one merchant's revenue.
func SummarizeStoreChannelEarnings(sellerUserId int) (*StoreChannelEarningsSummary, error) {
	summary := &StoreChannelEarningsSummary{}
	type row struct {
		Orders   int64
		Consumed int64
		Fee      int64
		Income   int64
	}
	result := row{}
	if err := DB.Model(&StoreChannelEarnings{}).
		Select("COUNT(*) AS orders, COALESCE(SUM(quota), 0) AS consumed, COALESCE(SUM(fee), 0) AS fee, COALESCE(SUM(income), 0) AS income").
		Where("seller_user_id = ?", sellerUserId).
		Scan(&result).Error; err != nil {
		return nil, err
	}
	buyers := int64(0)
	if err := DB.Model(&StoreChannelEarnings{}).
		Where("seller_user_id = ?", sellerUserId).
		Distinct("buyer_user_id").
		Count(&buyers).Error; err != nil {
		return nil, err
	}
	summary.Orders = result.Orders
	summary.Consumed = result.Consumed
	summary.Fee = result.Fee
	summary.Income = result.Income
	summary.UniqueBuyers = buyers
	return summary, nil
}

// StoreChannelEarningsView is one revenue row with the names already resolved.
type StoreChannelEarningsView struct {
	Id          int     `json:"id"`
	CreatedAt   int64   `json:"created_at"`
	BuyerUserId int     `json:"buyer_user_id"`
	BuyerName   string  `json:"buyer_name"`
	GroupName   string  `json:"group_name"`
	ChannelName string  `json:"channel_name"`
	ModelName   string  `json:"model_name"`
	Quota       int     `json:"quota"`
	Fee         int     `json:"fee"`
	Income      int     `json:"income"`
	Ratio       float64 `json:"ratio"`
	RequestId   string  `json:"request_id"`
}

// ListStoreChannelEarningsView pages through a merchant's revenue with names.
func ListStoreChannelEarningsView(sellerUserId int, page int, pageSize int) ([]StoreChannelEarningsView, int64, error) {
	rows, total, err := ListStoreChannelEarnings(sellerUserId, page, pageSize)
	if err != nil {
		return nil, 0, err
	}
	if len(rows) == 0 {
		return []StoreChannelEarningsView{}, total, nil
	}
	buyerIds := make([]int, 0, len(rows))
	offerIds := make([]int, 0, len(rows))
	channelIds := make([]int, 0, len(rows))
	seenBuyer := make(map[int]struct{}, len(rows))
	seenOffer := make(map[int]struct{}, len(rows))
	seenChannel := make(map[int]struct{}, len(rows))
	for _, row := range rows {
		if _, ok := seenBuyer[row.BuyerUserId]; !ok {
			seenBuyer[row.BuyerUserId] = struct{}{}
			buyerIds = append(buyerIds, row.BuyerUserId)
		}
		if _, ok := seenOffer[row.OfferId]; !ok {
			seenOffer[row.OfferId] = struct{}{}
			offerIds = append(offerIds, row.OfferId)
		}
		if _, ok := seenChannel[row.ChannelId]; !ok {
			seenChannel[row.ChannelId] = struct{}{}
			channelIds = append(channelIds, row.ChannelId)
		}
	}
	names := make(map[int]string, len(buyerIds))
	users := make([]*User, 0, len(buyerIds))
	if err := DB.Select("id", "username", "display_name").Where("id IN ?", buyerIds).Find(&users).Error; err == nil {
		for _, user := range users {
			label := user.DisplayName
			if label == "" {
				label = user.Username
			}
			names[user.Id] = label
		}
	}
	offers := make([]*StoreChannel, 0, len(offerIds))
	groupNames := make(map[int]string, len(offerIds))
	offerNames := make(map[int]string, len(offerIds))
	if err := DB.Select("id", "name", "group_name").Where("id IN ?", offerIds).Find(&offers).Error; err == nil {
		for _, offer := range offers {
			groupNames[offer.Id] = offer.GroupName
			offerNames[offer.Id] = offer.Name
		}
	}
	views := make([]StoreChannelEarningsView, 0, len(rows))
	for _, row := range rows {
		views = append(views, StoreChannelEarningsView{
			Id:          row.Id,
			CreatedAt:   row.CreatedAt,
			BuyerUserId: row.BuyerUserId,
			BuyerName:   names[row.BuyerUserId],
			GroupName:   groupNames[row.OfferId],
			ChannelName: offerNames[row.OfferId],
			ModelName:   row.ModelName,
			Quota:       row.Quota,
			Fee:         row.Fee,
			Income:      row.Income,
			Ratio:       row.Ratio,
			RequestId:   row.RequestId,
		})
	}
	return views, total, nil
}
