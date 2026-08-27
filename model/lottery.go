package model

import (
	"errors"
	"fmt"
	"math/rand"
	"strconv"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/logger"

	"gorm.io/gorm"
)

// ============================================================================
// Lottery (抽奖)
//
// The lottery lets users win balance top-ups or recharge-rebate coupons.
// Each draw consumes one admin-granted lottery chance; if the user has no
// chance, the draw instead costs a fixed quota amount (LotteryCostQuota).
// Admins grant draw chances via the users management API.
// ============================================================================

// LotteryCostQuota is the quota deducted when a user draws without a free
// admin-granted chance. It equals $5 of balance at the default quota ratio.
var LotteryCostQuota = int(common.QuotaPerUnit * 5)

// LotteryPrizeType categorizes a prize.
type LotteryPrizeType string

const (
	LotteryPrizeBalance LotteryPrizeType = "balance" // direct quota top-up
	LotteryPrizeCoupon  LotteryPrizeType = "coupon"  // recharge-rebate coupon
	LotteryPrizeThanks  LotteryPrizeType = "thanks"  // no prize
)

// LotteryTier describes a single prize tier with its probability weight.
type LotteryTier struct {
	Type       LotteryPrizeType
	Label      string    // human-readable prize label (i18n key on frontend)
	Weight     float64   // probability weight (sums to 100)
	Quota      int       // balance prize amount in quota (0 for non-balance)
	RebateRate float64   // coupon rebate percentage (0 for non-coupon)
}

// LotteryTiers is the fixed prize table. Weights sum to exactly 100.
// 10$ balance: 0.1%, 7$: 3%, 5$: 5.5%, 3$: 6%, 1$: 6.5%,
// 5% coupon: 7%, 3% coupon: 9.4%, 2% coupon: 13%, 1% coupon: 16.5%,
// thanks: 33%.
var LotteryTiers = []LotteryTier{
	{Type: LotteryPrizeBalance, Label: "$10 Balance", Weight: 0.1, Quota: int(common.QuotaPerUnit * 10)},
	{Type: LotteryPrizeBalance, Label: "$7 Balance", Weight: 3, Quota: int(common.QuotaPerUnit * 7)},
	{Type: LotteryPrizeBalance, Label: "$5 Balance", Weight: 5.5, Quota: int(common.QuotaPerUnit * 5)},
	{Type: LotteryPrizeBalance, Label: "$3 Balance", Weight: 6, Quota: int(common.QuotaPerUnit * 3)},
	{Type: LotteryPrizeBalance, Label: "$1 Balance", Weight: 6.5, Quota: int(common.QuotaPerUnit * 1)},
	{Type: LotteryPrizeCoupon, Label: "5% Recharge Coupon", Weight: 7, RebateRate: 5},
	{Type: LotteryPrizeCoupon, Label: "3% Recharge Coupon", Weight: 9.4, RebateRate: 3},
	{Type: LotteryPrizeCoupon, Label: "2% Recharge Coupon", Weight: 13, RebateRate: 2},
	{Type: LotteryPrizeCoupon, Label: "1% Recharge Coupon", Weight: 16.5, RebateRate: 1},
	{Type: LotteryPrizeThanks, Label: "Thanks for participating", Weight: 33},
}

// LotteryCoupon is a recharge-rebate coupon won from the lottery. When the
// user recharges, the highest unused coupon is applied as a bonus rebate.
type LotteryCoupon struct {
	Id         int64   `json:"id" gorm:"primaryKey;autoIncrement"`
	UserId     int     `json:"user_id" gorm:"not null;index"`
	RebateRate float64 `json:"rebate_rate" gorm:"not null"` // percentage, e.g. 5.0 for 5%
	Used       bool    `json:"used" gorm:"not null;index"`
	CreatedAt  int64   `json:"created_at" gorm:"bigint"`
	UsedAt     int64   `json:"used_at" gorm:"bigint;default:0"`
}

func (LotteryCoupon) TableName() string {
	return "lottery_coupons"
}

// LotteryRecord is an audit log of each draw.
type LotteryRecord struct {
	Id         int64          `json:"id" gorm:"primaryKey;autoIncrement"`
	UserId     int            `json:"user_id" gorm:"not null;index"`
	PrizeType  LotteryPrizeType `json:"prize_type" gorm:"type:varchar(16);not null"`
	PrizeLabel string         `json:"prize_label" gorm:"type:varchar(64);not null"`
	PrizeValue int            `json:"prize_value" gorm:"not null"` // quota for balance, 0 otherwise
	RebateRate float64        `json:"rebate_rate" gorm:"not null"` // coupon rate, 0 otherwise
	CostQuota  int            `json:"cost_quota" gorm:"not null"`  // quota spent on this draw
	CreatedAt  int64          `json:"created_at" gorm:"bigint"`
}

func (LotteryRecord) TableName() string {
	return "lottery_records"
}

// rollLotteryPrize selects a prize tier using the configured weights.
func rollLotteryPrize() LotteryTier {
	total := 0.0
	for _, tier := range LotteryTiers {
		total += tier.Weight
	}
	r := rand.Float64() * total
	acc := 0.0
	for _, tier := range LotteryTiers {
		acc += tier.Weight
		if r < acc {
			return tier
		}
	}
	// Fallback to the last tier (thanks) if floating point drift occurs.
	return LotteryTiers[len(LotteryTiers)-1]
}

// DrawLotteryResult is returned to the API layer after a draw.
type DrawLotteryResult struct {
	PrizeType  LotteryPrizeType
	PrizeLabel string
	PrizeValue int
	RebateRate float64
	CostQuota  int
	FreeDraw   bool
}

var (
	ErrLotteryNoChance   = errors.New("no lottery chances available")
	ErrLotteryNoQuota    = errors.New("insufficient quota to draw")
	ErrLotteryClosed     = errors.New("lottery is not enabled")
)

// DrawLottery performs a single lottery draw for the user.
// A free admin-granted chance is consumed when available; otherwise the fixed
// quota cost is deducted. The prize is then applied and recorded.
func DrawLottery(userId int) (*DrawLotteryResult, error) {
	if !common.LotteryEnabled {
		return nil, ErrLotteryClosed
	}
	if userId == 0 {
		return nil, errors.New("user id is empty")
	}

	tier := rollLotteryPrize()
	costQuota := 0
	freeDraw := false

	err := DB.Transaction(func(tx *gorm.DB) error {
		var user User
		if err := lockForUpdate(tx).First(&user, userId).Error; err != nil {
			return err
		}

		if user.LotteryChances > 0 {
			// Consume a free admin-granted chance.
			if err := tx.Model(&User{}).Where("id = ?", userId).
				Update("lottery_chances", gorm.Expr("lottery_chances - ?", 1)).Error; err != nil {
				return err
			}
			freeDraw = true
		} else {
			// No free chance: charge the fixed quota cost.
			quotaNeeded := LotteryCostQuota
			if user.Quota < quotaNeeded {
				return ErrLotteryNoQuota
			}
			costQuota = quotaNeeded
			if err := tx.Model(&User{}).Where("id = ?", userId).
				Update("quota", gorm.Expr("quota - ?", quotaNeeded)).Error; err != nil {
				return err
			}
		}

		// Apply the prize.
		switch tier.Type {
		case LotteryPrizeBalance:
			if tier.Quota > 0 {
				if err := tx.Model(&User{}).Where("id = ?", userId).
					Update("quota", gorm.Expr("quota + ?", tier.Quota)).Error; err != nil {
					return err
				}
			}
		case LotteryPrizeCoupon:
			coupon := &LotteryCoupon{
				UserId:     userId,
				RebateRate: tier.RebateRate,
				Used:       false,
				CreatedAt:  common.GetTimestamp(),
			}
			if err := tx.Create(coupon).Error; err != nil {
				return err
			}
		case LotteryPrizeThanks:
			// No prize.
		}

		// Record the draw.
		record := &LotteryRecord{
			UserId:     userId,
			PrizeType:  tier.Type,
			PrizeLabel: tier.Label,
			PrizeValue: tier.Quota,
			RebateRate: tier.RebateRate,
			CostQuota:  costQuota,
			CreatedAt:  common.GetTimestamp(),
		}
		return tx.Create(record).Error
	})
	if err != nil {
		return nil, err
	}

	if !freeDraw && costQuota > 0 {
		go func() { _ = cacheDecrUserQuota(userId, int64(costQuota)) }()
	}
	if tier.Type == LotteryPrizeBalance && tier.Quota > 0 {
		go func() { _ = cacheIncrUserQuota(userId, int64(tier.Quota)) }()
	}

	// Log the outcome.
	switch tier.Type {
	case LotteryPrizeBalance:
		RecordLog(userId, LogTypeSystem, fmt.Sprintf("抽奖中奖：%s，获得额度 %s", tier.Label, logger.LogQuota(tier.Quota)))
	case LotteryPrizeCoupon:
		RecordLog(userId, LogTypeSystem, fmt.Sprintf("抽奖中奖：%s（充值返 %s%%）", tier.Label, formatFloat(tier.RebateRate)))
	case LotteryPrizeThanks:
		RecordLog(userId, LogTypeSystem, "抽奖：谢谢参与")
	}

	return &DrawLotteryResult{
		PrizeType:  tier.Type,
		PrizeLabel: tier.Label,
		PrizeValue: tier.Quota,
		RebateRate: tier.RebateRate,
		CostQuota:  costQuota,
		FreeDraw:   freeDraw,
	}, nil
}

func formatFloat(v float64) string {
	return strconv.FormatFloat(v, 'f', -1, 64)
}

// GetUnusedLotteryCoupon returns the highest-rate unused coupon for a user,
// or nil if none exists.
func GetUnusedLotteryCoupon(userId int) (*LotteryCoupon, error) {
	var coupon LotteryCoupon
	err := DB.Where("user_id = ? AND used = ?", userId, false).
		Order("rebate_rate DESC").First(&coupon).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &coupon, nil
}

// CountUnusedLotteryCoupons returns the number of unused coupons for a user.
func CountUnusedLotteryCoupons(userId int) (int64, error) {
	var count int64
	err := DB.Model(&LotteryCoupon{}).Where("user_id = ? AND used = ?", userId, false).Count(&count).Error
	return count, err
}

// MarkLotteryCouponUsed marks a coupon as used.
func MarkLotteryCouponUsed(couponId int64, tx *gorm.DB) error {
	return tx.Model(&LotteryCoupon{}).Where("id = ?", couponId).
		Updates(map[string]interface{}{
			"used":    true,
			"used_at": common.GetTimestamp(),
		}).Error
}

// ApplyLotteryCoupon applies the user's best unused lottery coupon as a bonus
// on a recharge. The bonus is added directly to the user's spendable quota and
// the coupon is marked used. Must be called inside the recharge transaction.
func ApplyLotteryCoupon(tx *gorm.DB, userId int, creditedQuota int) (int, error) {
	if creditedQuota <= 0 {
		return 0, nil
	}
	coupon, err := GetUnusedLotteryCoupon(userId)
	if err != nil {
		return 0, err
	}
	if coupon == nil {
		return 0, nil
	}
	bonusFloat := float64(creditedQuota) * coupon.RebateRate / 100.0
	bonus, clamp := common.QuotaFromFloatChecked(bonusFloat)
	if clamp != nil {
		logger.LogWarn(nil, fmt.Sprintf("lottery coupon bonus clamped user=%d rate=%.2f credited=%d", userId, coupon.RebateRate, creditedQuota))
	}
	if bonus <= 0 {
		return 0, nil
	}
	if err := tx.Model(&User{}).Where("id = ?", userId).
		Update("quota", gorm.Expr("quota + ?", bonus)).Error; err != nil {
		return 0, err
	}
	if err := MarkLotteryCouponUsed(coupon.Id, tx); err != nil {
		return 0, err
	}
	RecordLog(userId, LogTypeSystem, fmt.Sprintf("抽奖充值优惠券返利 %s (充值额度: %s, 优惠比例: %s%%)", logger.LogQuota(bonus), logger.LogQuota(creditedQuota), formatFloat(coupon.RebateRate)))
	return bonus, nil
}

// GetUserLotteryStatus returns the user's lottery chances, unused coupon count,
// and recent draw records.
func GetUserLotteryStatus(userId int, limit int) (chances int, couponCount int64, records []LotteryRecord, err error) {
	var user User
	if err = DB.Select("lottery_chances").First(&user, userId).Error; err != nil {
		return 0, 0, nil, err
	}
	chances = user.LotteryChances

	couponCount, err = CountUnusedLotteryCoupons(userId)
	if err != nil {
		return chances, 0, nil, err
	}

	if limit <= 0 {
		limit = 20
	}
	err = DB.Where("user_id = ?", userId).Order("id DESC").Limit(limit).Find(&records).Error
	return chances, couponCount, records, err
}

// AdminGrantLotteryChances adds lottery chances to a user (admin action).
func AdminGrantLotteryChances(userId int, count int) error {
	if userId == 0 {
		return errors.New("user id is empty")
	}
	if count <= 0 {
		return errors.New("chance count must be positive")
	}
	return DB.Model(&User{}).Where("id = ?", userId).
		Update("lottery_chances", gorm.Expr("lottery_chances + ?", count)).Error
}
