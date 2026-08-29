package model

import (
	"errors"
	"strings"

	"github.com/QuantumNous/new-api/common"

	"gorm.io/gorm"
)

// ============================================================================
// OAuth 2.0 call packages (次数包)
//
// An OAuthCallPlan is an admin-defined bundle of prepaid userinfo calls that
// users buy with their wallet balance. Each purchased plan instance becomes an
// OAuthCallGrant; grant calls are consumed only for requests beyond the free
// daily tier (see oauthTierPrice in controller/oauth2.go), before the wallet
// is charged.
// ============================================================================

// OAuthCallPlan is an admin-configurable purchasable call bundle.
type OAuthCallPlan struct {
	Id     int    `json:"id" gorm:"primaryKey;autoIncrement"`
	Name   string `json:"name" gorm:"type:varchar(128);not null"`
	// CallCount is the number of prepaid userinfo calls in the bundle.
	CallCount int `json:"call_count" gorm:"type:int;not null"`
	// PriceAmount is the display price in USD, charged from the wallet.
	PriceAmount float64 `json:"price_amount" gorm:"type:decimal(10,6);not null;default:0"`
	// ValidityDays bounds a purchased grant's lifetime (0 = never expires).
	ValidityDays int   `json:"validity_days" gorm:"type:int;default:0"`
	Enabled      bool  `json:"enabled" gorm:"default:true"`
	SortOrder    int   `json:"sort_order" gorm:"type:int;default:0"`
	CreatedAt    int64 `json:"created_at" gorm:"bigint"`
	UpdatedAt    int64 `json:"updated_at" gorm:"bigint"`
}

func (OAuthCallPlan) TableName() string {
	return "oauth_call_plans"
}

func (p *OAuthCallPlan) BeforeCreate(tx *gorm.DB) error {
	now := common.GetTimestamp()
	p.CreatedAt = now
	p.UpdatedAt = now
	return nil
}

func (p *OAuthCallPlan) BeforeUpdate(tx *gorm.DB) error {
	p.UpdatedAt = common.GetTimestamp()
	return nil
}

// ValidateOAuthCallPlan checks a plan's fields before save.
func ValidateOAuthCallPlan(plan *OAuthCallPlan) error {
	if strings.TrimSpace(plan.Name) == "" {
		return errors.New("套餐名称不能为空")
	}
	if plan.CallCount <= 0 || plan.CallCount > 100000000 {
		return errors.New("调用次数必须在 (0, 1亿] 之间")
	}
	if plan.PriceAmount < 0 || plan.PriceAmount > 9999 {
		return errors.New("价格必须在 [0, 9999] 之间")
	}
	if plan.ValidityDays < 0 || plan.ValidityDays > 36500 {
		return errors.New("有效期必须在 [0, 36500] 天之间")
	}
	return nil
}

// GetAllOAuthCallPlans returns every plan row for the admin editor.
func GetAllOAuthCallPlans() ([]OAuthCallPlan, error) {
	var plans []OAuthCallPlan
	err := DB.Order("sort_order desc, id asc").Find(&plans).Error
	return plans, err
}

// GetEnabledOAuthCallPlans returns the purchasable plans.
func GetEnabledOAuthCallPlans() ([]OAuthCallPlan, error) {
	var plans []OAuthCallPlan
	err := DB.Where("enabled = ?", true).
		Order("sort_order desc, id asc").
		Find(&plans).Error
	return plans, err
}

// AdminCreateOAuthCallPlan adds a new call plan (admin action).
func AdminCreateOAuthCallPlan(plan *OAuthCallPlan) error {
	if err := ValidateOAuthCallPlan(plan); err != nil {
		return err
	}
	return DB.Create(plan).Error
}

// AdminUpdateOAuthCallPlan updates an existing call plan (admin action).
// Existing grants keep their purchased call counts.
func AdminUpdateOAuthCallPlan(plan *OAuthCallPlan) error {
	if plan.Id <= 0 {
		return errors.New("invalid plan id")
	}
	if err := ValidateOAuthCallPlan(plan); err != nil {
		return err
	}
	res := DB.Model(&OAuthCallPlan{}).Where("id = ?", plan.Id).Updates(map[string]interface{}{
		"name":          plan.Name,
		"call_count":    plan.CallCount,
		"price_amount":  plan.PriceAmount,
		"validity_days": plan.ValidityDays,
		"enabled":       plan.Enabled,
		"sort_order":    plan.SortOrder,
		"updated_at":    common.GetTimestamp(),
	})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return errors.New("套餐不存在")
	}
	return nil
}

// AdminDeleteOAuthCallPlan removes a call plan (admin action). Existing grants
// keep working.
func AdminDeleteOAuthCallPlan(planId int) error {
	if planId <= 0 {
		return errors.New("invalid plan id")
	}
	res := DB.Delete(&OAuthCallPlan{}, planId)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return errors.New("套餐不存在")
	}
	return nil
}

// OAuthCallGrant is a purchased call bundle owned by a user.
type OAuthCallGrant struct {
	Id       int64  `json:"id" gorm:"primaryKey;autoIncrement"`
	UserId   int    `json:"user_id" gorm:"not null;index"`
	PlanId   int    `json:"plan_id" gorm:"index"`
	PlanName string `json:"plan_name" gorm:"type:varchar(128)"`
	// CallsTotal is the purchased call count; CallsUsed the consumed count.
	CallsTotal int `json:"calls_total" gorm:"type:int;not null"`
	CallsUsed  int `json:"calls_used" gorm:"type:int;not null;default:0"`
	// ExpiresAt is a unix timestamp; 0 means the grant never expires.
	ExpiresAt int64  `json:"expires_at" gorm:"bigint;default:0"`
	Status    string `json:"status" gorm:"type:varchar(16);not null;default:'active';index"` // active/exhausted
	Source    string `json:"source" gorm:"type:varchar(32);default:'purchase'"`
	CreatedAt int64  `json:"created_at" gorm:"bigint"`
	UpdatedAt int64  `json:"updated_at" gorm:"bigint"`
}

func (OAuthCallGrant) TableName() string {
	return "oauth_call_grants"
}

// GetOAuthCallBalance returns the user's remaining prepaid call count across
// all active, unexpired grants.
func GetOAuthCallBalance(userId int) (int, error) {
	if userId <= 0 {
		return 0, errors.New("invalid userId")
	}
	var grants []OAuthCallGrant
	err := DB.Where("user_id = ? AND status = ? AND calls_used < calls_total", userId, "active").
		Find(&grants).Error
	if err != nil {
		return 0, err
	}
	now := common.GetTimestamp()
	balance := 0
	for _, grant := range grants {
		if grant.ExpiresAt > 0 && grant.ExpiresAt <= now {
			continue
		}
		balance += grant.CallsTotal - grant.CallsUsed
	}
	return balance, nil
}

// ListUserOAuthCallGrants returns the user's grants, newest first.
func ListUserOAuthCallGrants(userId int, limit int) ([]OAuthCallGrant, error) {
	if userId <= 0 {
		return nil, errors.New("invalid userId")
	}
	if limit <= 0 {
		limit = 20
	}
	var grants []OAuthCallGrant
	err := DB.Where("user_id = ?", userId).
		Order("id desc").Limit(limit).Find(&grants).Error
	return grants, err
}

// PurchaseOAuthCallPlan buys a call plan with the user's wallet balance and
// creates an active grant. Atomic: the wallet deduction and grant creation
// commit or roll back together.
func PurchaseOAuthCallPlan(userId int, planId int) (*OAuthCallGrant, int, error) {
	if userId <= 0 || planId <= 0 {
		return nil, 0, errors.New("invalid args")
	}
	var created *OAuthCallGrant
	priceQuota := 0
	err := DB.Transaction(func(tx *gorm.DB) error {
		var plan OAuthCallPlan
		if err := lockForUpdate(tx).
			Where("id = ? AND enabled = ?", planId, true).
			First(&plan).Error; err != nil {
			return errors.New("套餐不存在或未启用")
		}
		var quotaErr error
		priceQuota, quotaErr = calcSubscriptionBalanceQuota(plan.PriceAmount)
		if quotaErr != nil {
			return quotaErr
		}
		var user User
		if err := lockForUpdate(tx).First(&user, userId).Error; err != nil {
			return err
		}
		if user.Status != common.UserStatusEnabled {
			return errors.New("用户不可用")
		}
		if priceQuota > 0 && user.Quota < priceQuota {
			return errors.New("余额不足，请先充值")
		}
		if priceQuota > 0 {
			if err := tx.Model(&User{}).Where("id = ?", userId).
				Update("quota", gorm.Expr("quota - ?", priceQuota)).Error; err != nil {
				return err
			}
		}
		var expiresAt int64
		if plan.ValidityDays > 0 {
			expiresAt = common.GetTimestamp() + int64(plan.ValidityDays)*86400
		}
		grant := &OAuthCallGrant{
			UserId:     userId,
			PlanId:     plan.Id,
			PlanName:   plan.Name,
			CallsTotal: plan.CallCount,
			CallsUsed:  0,
			ExpiresAt:  expiresAt,
			Status:     "active",
			Source:     "purchase",
			CreatedAt:  common.GetTimestamp(),
			UpdatedAt:  common.GetTimestamp(),
		}
		if err := tx.Create(grant).Error; err != nil {
			return err
		}
		created = grant
		return nil
	})
	if err != nil {
		return nil, 0, err
	}
	if created != nil && priceQuota > 0 {
		go func() { _ = cacheDecrUserQuota(userId, int64(priceQuota)) }()
	}
	return created, priceQuota, nil
}

// ConsumeOAuthCallGrant consumes one prepaid call from the user's grants,
// preferring the soonest-expiring active grant. Returns false when the user
// has no usable prepaid calls.
func ConsumeOAuthCallGrant(userId int) (bool, error) {
	if userId <= 0 {
		return false, errors.New("invalid userId")
	}
	consumed := false
	err := DB.Transaction(func(tx *gorm.DB) error {
		var txErr error
		consumed, txErr = consumeOAuthCallGrantTx(tx, userId)
		return txErr
	})
	if err != nil {
		return false, err
	}
	return consumed, nil
}

// consumeOAuthCallGrantTx is the transactional core of ConsumeOAuthCallGrant;
// reuse it inside an existing transaction instead of nesting DB.Transaction
// calls.
func consumeOAuthCallGrantTx(tx *gorm.DB, userId int) (bool, error) {
	now := common.GetTimestamp()
	var grants []OAuthCallGrant
	if err := lockForUpdate(tx).
		Where("user_id = ? AND status = ? AND calls_used < calls_total", userId, "active").
		Find(&grants).Error; err != nil {
		return false, err
	}
	// Prefer the soonest-expiring grant; never-expiring (0) last.
	var chosen *OAuthCallGrant
	for i := range grants {
		grant := &grants[i]
		if grant.ExpiresAt > 0 && grant.ExpiresAt <= now {
			continue
		}
		if chosen == nil {
			chosen = grant
			continue
		}
		chosenNeverExpires := chosen.ExpiresAt == 0
		grantNeverExpires := grant.ExpiresAt == 0
		if chosenNeverExpires && !grantNeverExpires {
			chosen = grant
		} else if !chosenNeverExpires && !grantNeverExpires && grant.ExpiresAt < chosen.ExpiresAt {
			chosen = grant
		}
	}
	if chosen == nil {
		return false, nil
	}
	chosen.CallsUsed++
	if chosen.CallsUsed >= chosen.CallsTotal {
		chosen.Status = "exhausted"
	}
	if err := tx.Model(&OAuthCallGrant{}).Where("id = ?", chosen.Id).Updates(map[string]interface{}{
		"calls_used": chosen.CallsUsed,
		"status":     chosen.Status,
		"updated_at": common.GetTimestamp(),
	}).Error; err != nil {
		return false, err
	}
	return true, nil
}

// SeedOAuthCallPlan inserts a default starter call plan when the table is
// empty, so the purchase flow works out of the box.
func SeedOAuthCallPlan() error {
	var count int64
	if err := DB.Model(&OAuthCallPlan{}).Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return nil
	}
	plan := OAuthCallPlan{
		Name:        "OAuth2 次数包 1000 次",
		CallCount:   1000,
		PriceAmount: 1,
		Enabled:     true,
	}
	return DB.Create(&plan).Error
}
