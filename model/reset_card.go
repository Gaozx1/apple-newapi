package model

import (
	"errors"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
)

// Reset card status values.
const (
	ResetCardStatusUnused   = "unused"
	ResetCardStatusUsed     = "used"
	ResetCardStatusExpired  = "expired"
	ResetCardStatusDisabled = "disabled"
)

// Reset card sources describe how the card reached its owner.
const (
	ResetCardSourceAdmin      = "admin"
	ResetCardSourceOrder      = "order"
	ResetCardSourceBalance    = "balance"
	ResetCardSourceRedemption = "redemption"
	ResetCardSourceLottery    = "lottery"
)

// ResetCard is a single-use card that resets the used quota of one of the
// owner's active subscriptions. Cards are either granted by an administrator
// or issued by purchasing a plan whose PlanType is "reset_card".
type ResetCard struct {
	Id        int    `json:"id"`
	UserId    int    `json:"user_id" gorm:"index;index:idx_reset_card_user_status,priority:1"`
	CardKey   string `json:"card_key" gorm:"type:varchar(64);uniqueIndex"`
	Status    string `json:"status" gorm:"type:varchar(16);index;index:idx_reset_card_user_status,priority:2;index:idx_reset_card_expiry,priority:1"` // unused/used/expired/disabled
	Source    string `json:"source" gorm:"type:varchar(16);default:'admin'"`
	PlanId    int    `json:"plan_id" gorm:"default:0"`
	TradeNo   string `json:"trade_no" gorm:"type:varchar(255);default:''"`
	Note      string `json:"note" gorm:"type:varchar(255);default:''"`
	ExpiresAt int64  `json:"expires_at" gorm:"bigint;default:0;index;index:idx_reset_card_expiry,priority:2"` // 0 = never expires

	// TargetSubscriptionId restricts the card to one of the owner's
	// subscriptions. 0 means the card may reset any active subscription.
	TargetSubscriptionId int `json:"target_subscription_id" gorm:"default:0;index"`

	UsedAt             int64 `json:"used_at" gorm:"bigint;default:0"`
	UsedByUserId       int   `json:"used_by_user_id" gorm:"default:0"`
	UsedSubscriptionId int   `json:"used_subscription_id" gorm:"default:0"`

	CreatedAt int64 `json:"created_at" gorm:"bigint"`
	UpdatedAt int64 `json:"updated_at" gorm:"bigint"`
}

func (c *ResetCard) BeforeCreate(tx *gorm.DB) error {
	now := common.GetTimestamp()
	if c.CreatedAt == 0 {
		c.CreatedAt = now
	}
	c.UpdatedAt = now
	if c.Status == "" {
		c.Status = ResetCardStatusUnused
	}
	return nil
}

func (c *ResetCard) BeforeUpdate(tx *gorm.DB) error {
	c.UpdatedAt = common.GetTimestamp()
	return nil
}

// ResetCardView is the list projection. The card key is only returned to the
// owner and to administrators; it is the credential used to redeem the card.
type ResetCardView struct {
	ResetCard
	PlanTitle string `json:"plan_title,omitempty"`
	Username  string `json:"username,omitempty"`
	// TargetSubscriptionPlanTitle names the plan of the bound subscription so
	// administrators and owners can recognize it without joining manually.
	TargetSubscriptionPlanTitle string `json:"target_subscription_plan_title,omitempty"`
}

func newResetCardKey() string {
	return "rc_" + common.GetUUID()
}

// resetCardExpiryFromPlan returns the expiry timestamp a card issued by plan
// should carry. A reset-card plan reuses the plan duration settings as the
// card's validity period; a missing/zero duration makes the card permanent.
func resetCardExpiryFromPlan(plan *SubscriptionPlan, now time.Time) (int64, error) {
	if plan == nil {
		return 0, errors.New("plan is nil")
	}
	if plan.DurationValue <= 0 && plan.DurationUnit != SubscriptionDurationCustom {
		return 0, nil
	}
	if plan.DurationUnit == SubscriptionDurationCustom && plan.CustomSeconds <= 0 {
		return 0, nil
	}
	return calcPlanEndTime(now, plan)
}

// IssueResetCardsFromPlanTx issues plan.ResetCardCount reset cards to userId.
// It is used by paid orders, balance purchases, redemptions, lottery prizes and
// admin grants of a reset-card plan.
func IssueResetCardsFromPlanTx(tx *gorm.DB, userId int, plan *SubscriptionPlan, source string, tradeNo string) ([]string, error) {
	if tx == nil {
		return nil, errors.New("tx is nil")
	}
	if plan == nil || plan.Id == 0 {
		return nil, errors.New("invalid plan")
	}
	if userId <= 0 {
		return nil, errors.New("invalid user id")
	}
	count := plan.ResetCardCount
	if count <= 0 {
		count = 1
	}
	now := GetDBTimestamp()
	expiresAt, err := resetCardExpiryFromPlan(plan, time.Unix(now, 0))
	if err != nil {
		return nil, err
	}
	keys := make([]string, 0, count)
	for range count {
		card := &ResetCard{
			UserId:    userId,
			CardKey:   newResetCardKey(),
			Status:    ResetCardStatusUnused,
			Source:    source,
			PlanId:    plan.Id,
			TradeNo:   tradeNo,
			ExpiresAt: expiresAt,
		}
		if err := tx.Create(card).Error; err != nil {
			return keys, err
		}
		keys = append(keys, card.CardKey)
	}
	return keys, nil
}

// GrantResetCards issues count admin-granted cards to userId with an explicit
// expiry timestamp (0 = never expires). note is stored on each card.
// targetSubscriptionId > 0 binds every card to one of the user's existing
// subscriptions so it can only reset that subscription; 0 leaves the card
// usable on any active subscription.
func GrantResetCards(userId int, count int, expiresAt int64, note string, targetSubscriptionId int) ([]string, error) {
	if userId <= 0 {
		return nil, errors.New("invalid user id")
	}
	if count <= 0 || count > 100 {
		return nil, errors.New("reset card count must be between 1 and 100")
	}
	if targetSubscriptionId < 0 {
		return nil, errors.New("invalid target subscription id")
	}
	now := common.GetTimestamp()
	if expiresAt != 0 && expiresAt <= now {
		return nil, errors.New("reset card expiry must be in the future")
	}
	note = strings.TrimSpace(note)
	if len(note) > 255 {
		return nil, errors.New("reset card note is too long")
	}
	keys := make([]string, 0, count)
	err := DB.Transaction(func(tx *gorm.DB) error {
		var user User
		if err := tx.Select("id").Where("id = ?", userId).First(&user).Error; err != nil {
			return err
		}
		if targetSubscriptionId > 0 {
			var sub UserSubscription
			if err := tx.Select("id", "user_id").Where("id = ?", targetSubscriptionId).First(&sub).Error; err != nil {
				return common.NewMessage("Subscription does not exist")
			}
			if sub.UserId != userId {
				return common.NewMessage("Subscription does not belong to this user")
			}
		}
		for range count {
			card := &ResetCard{
				UserId:               userId,
				CardKey:              newResetCardKey(),
				Status:               ResetCardStatusUnused,
				Source:               ResetCardSourceAdmin,
				Note:                 note,
				ExpiresAt:            expiresAt,
				TargetSubscriptionId: targetSubscriptionId,
			}
			if err := tx.Create(card).Error; err != nil {
				return err
			}
			keys = append(keys, card.CardKey)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return keys, nil
}

// GrantResetCardsToAllUsers issues count cards to every active common user.
// It backs "grant to all users" when the administrator leaves the user ID
// empty. When targetSubscriptionId > 0 every card is bound to that
// subscription, so only its owner can actually redeem the card; 0 leaves every
// card usable on any active subscription. Returns the number of users that
// received cards and the total number of cards created.
func GrantResetCardsToAllUsers(count int, expiresAt int64, note string, targetSubscriptionId int) (int, int64, error) {
	if count <= 0 || count > 100 {
		return 0, 0, errors.New("reset card count must be between 1 and 100")
	}
	if targetSubscriptionId < 0 {
		return 0, 0, errors.New("invalid target subscription id")
	}
	now := common.GetTimestamp()
	if expiresAt != 0 && expiresAt <= now {
		return 0, 0, errors.New("reset card expiry must be in the future")
	}
	note = strings.TrimSpace(note)
	if len(note) > 255 {
		return 0, 0, errors.New("reset card note is too long")
	}
	if targetSubscriptionId > 0 {
		var sub UserSubscription
		if err := DB.Select("id").Where("id = ?", targetSubscriptionId).First(&sub).Error; err != nil {
			return 0, 0, common.NewMessage("Subscription does not exist")
		}
	}
	var userIds []int
	if err := DB.Model(&User{}).
		Where("role = ? AND status = ?", common.RoleCommonUser, common.UserStatusEnabled).
		Order("id asc").Pluck("id", &userIds).Error; err != nil {
		return 0, 0, err
	}
	if len(userIds) == 0 {
		return 0, 0, common.NewMessage("No active users to grant reset cards to")
	}
	var total int64
	err := DB.Transaction(func(tx *gorm.DB) error {
		const batchSize = 500
		batch := make([]ResetCard, 0, batchSize)
		flush := func() error {
			if len(batch) == 0 {
				return nil
			}
			if err := tx.CreateInBatches(batch, batchSize).Error; err != nil {
				return err
			}
			total += int64(len(batch))
			batch = batch[:0]
			return nil
		}
		for _, userId := range userIds {
			for range count {
				batch = append(batch, ResetCard{
					UserId:               userId,
					CardKey:              newResetCardKey(),
					Status:               ResetCardStatusUnused,
					Source:               ResetCardSourceAdmin,
					Note:                 note,
					ExpiresAt:            expiresAt,
					TargetSubscriptionId: targetSubscriptionId,
					CreatedAt:            now,
					UpdatedAt:            now,
				})
			}
			if len(batch) >= batchSize {
				if err := flush(); err != nil {
					return err
				}
			}
		}
		return flush()
	})
	if err != nil {
		return 0, 0, err
	}
	return len(userIds), total, nil
}

// UseResetCard consumes one card and resets the used quota of the given active
// subscription owned by userId. The scheduled next reset time is preserved.
func UseResetCard(userId int, cardKey string, userSubscriptionId int) (*SubscriptionSummary, error) {
	if userId <= 0 {
		return nil, errors.New("invalid user id")
	}
	cardKey = strings.TrimSpace(cardKey)
	if cardKey == "" {
		return nil, errors.New("reset card key is required")
	}
	if userSubscriptionId <= 0 {
		return nil, errors.New("invalid subscription id")
	}
	now := GetDBTimestamp()
	var summary *SubscriptionSummary
	err := DB.Transaction(func(tx *gorm.DB) error {
		var card ResetCard
		if err := lockForUpdate(tx).Where("card_key = ?", cardKey).First(&card).Error; err != nil {
			return common.NewMessage("Reset card does not exist")
		}
		if card.UserId != userId {
			return common.NewMessage("Reset card does not belong to this user")
		}
		if card.Status != ResetCardStatusUnused {
			return common.NewMessage("Reset card is no longer available")
		}
		if card.ExpiresAt != 0 && card.ExpiresAt <= now {
			return common.NewMessage("Reset card has expired")
		}
		if card.TargetSubscriptionId > 0 && card.TargetSubscriptionId != userSubscriptionId {
			return common.NewMessage("This reset card can only reset subscription #{{id}}", map[string]any{"id": card.TargetSubscriptionId})
		}
		var sub UserSubscription
		if err := lockForUpdate(tx).Where("id = ?", userSubscriptionId).First(&sub).Error; err != nil {
			return common.NewMessage("Subscription does not exist")
		}
		if sub.UserId != userId {
			return common.NewMessage("Subscription does not belong to this user")
		}
		if sub.Status != "active" || (sub.EndTime > 0 && sub.EndTime <= now) {
			return common.NewMessage("Subscription is not active")
		}
		plan, err := getSubscriptionPlanByIdTx(tx, sub.PlanId)
		if err != nil {
			return err
		}
		if err := resetUserSubscriptionTx(tx, &sub, plan, now, false); err != nil {
			return err
		}
		result := tx.Model(&ResetCard{}).
			Where("id = ? AND status = ?", card.Id, ResetCardStatusUnused).
			Updates(map[string]any{
				"status":               ResetCardStatusUsed,
				"used_at":              now,
				"used_by_user_id":      userId,
				"used_subscription_id": sub.Id,
				"updated_at":           now,
			})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return common.NewMessage("Reset card is no longer available")
		}
		subCopy := sub
		summary = &SubscriptionSummary{Subscription: &subCopy}
		if quotas := plan.ParseModelQuotas(); len(quotas) > 0 {
			summary.ModelQuotas = quotas
			summary.ModelUsed = subCopy.ParseModelUsed()
		}
		if tokenQuotas := plan.ParseModelTokenQuotas(); len(tokenQuotas) > 0 {
			summary.ModelTokenQuotas = tokenQuotas
			summary.TokenUsed = subCopy.ParseTokenUsed()
		}
		summary.Window5h, summary.Window7d = SubscriptionWindowUsages(plan, &subCopy, now)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return summary, nil
}

// UserResetCards returns the cards owned by a user, newest first.
func UserResetCards(userId int) ([]ResetCardView, error) {
	if userId <= 0 {
		return nil, errors.New("invalid user id")
	}
	var cards []ResetCard
	if err := DB.Where("user_id = ?", userId).Order("id desc").Find(&cards).Error; err != nil {
		return nil, err
	}
	return buildResetCardViews(cards), nil
}

// SearchResetCards lists reset cards for administrators with optional filters.
func SearchResetCards(keyword string, status string, userId int, startIdx int, num int) ([]ResetCardView, int64, error) {
	query := DB.Model(&ResetCard{})
	keyword = strings.TrimSpace(keyword)
	if keyword != "" {
		query = query.Where("card_key LIKE ? OR note LIKE ?", keyword+"%", keyword+"%")
	}
	if status != "" {
		query = query.Where("status = ?", status)
	}
	if userId > 0 {
		query = query.Where("user_id = ?", userId)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var cards []ResetCard
	if err := query.Order("id desc").Limit(num).Offset(startIdx).Find(&cards).Error; err != nil {
		return nil, 0, err
	}
	return buildResetCardViews(cards), total, nil
}

func buildResetCardViews(cards []ResetCard) []ResetCardView {
	if len(cards) == 0 {
		return []ResetCardView{}
	}
	views := make([]ResetCardView, 0, len(cards))
	planIds := make([]int, 0, len(cards))
	seenPlan := make(map[int]struct{}, len(cards))
	userIds := make([]int, 0, len(cards))
	seenUser := make(map[int]struct{}, len(cards))
	subIds := make([]int, 0, len(cards))
	seenSub := make(map[int]struct{}, len(cards))
	for _, card := range cards {
		if card.PlanId > 0 {
			if _, ok := seenPlan[card.PlanId]; !ok {
				seenPlan[card.PlanId] = struct{}{}
				planIds = append(planIds, card.PlanId)
			}
		}
		if card.UserId > 0 {
			if _, ok := seenUser[card.UserId]; !ok {
				seenUser[card.UserId] = struct{}{}
				userIds = append(userIds, card.UserId)
			}
		}
		if card.TargetSubscriptionId > 0 {
			if _, ok := seenSub[card.TargetSubscriptionId]; !ok {
				seenSub[card.TargetSubscriptionId] = struct{}{}
				subIds = append(subIds, card.TargetSubscriptionId)
			}
		}
	}
	planTitles := make(map[int]string, len(planIds))
	if len(planIds) > 0 {
		var plans []SubscriptionPlan
		if err := DB.Select("id", "title").Where("id IN ?", planIds).Find(&plans).Error; err == nil {
			for _, plan := range plans {
				planTitles[plan.Id] = plan.Title
			}
		}
	}
	usernames := make(map[int]string, len(userIds))
	if len(userIds) > 0 {
		var users []User
		if err := DB.Select("id", "username").Where("id IN ?", userIds).Find(&users).Error; err == nil {
			for _, user := range users {
				usernames[user.Id] = user.Username
			}
		}
	}
	targetSubPlanTitles := make(map[int]string, len(subIds))
	if len(subIds) > 0 {
		var subs []UserSubscription
		if err := DB.Select("id", "plan_id").Where("id IN ?", subIds).Find(&subs).Error; err == nil {
			targetPlanIds := make([]int, 0, len(subs))
			for _, sub := range subs {
				if sub.PlanId > 0 {
					targetPlanIds = append(targetPlanIds, sub.PlanId)
				}
			}
			targetPlanTitles := make(map[int]string, len(targetPlanIds))
			if len(targetPlanIds) > 0 {
				var plans []SubscriptionPlan
				if err := DB.Select("id", "title").Where("id IN ?", targetPlanIds).Find(&plans).Error; err == nil {
					for _, plan := range plans {
						targetPlanTitles[plan.Id] = plan.Title
					}
				}
			}
			for _, sub := range subs {
				targetSubPlanTitles[sub.Id] = targetPlanTitles[sub.PlanId]
			}
		}
	}
	for _, card := range cards {
		view := ResetCardView{ResetCard: card}
		view.PlanTitle = planTitles[card.PlanId]
		view.Username = usernames[card.UserId]
		view.TargetSubscriptionPlanTitle = targetSubPlanTitles[card.TargetSubscriptionId]
		views = append(views, view)
	}
	return views
}

// AdminDisableResetCard disables an unused card.
func AdminDisableResetCard(id int) error {
	if id <= 0 {
		return errors.New("invalid reset card id")
	}
	result := DB.Model(&ResetCard{}).
		Where("id = ? AND status = ?", id, ResetCardStatusUnused).
		Updates(map[string]any{"status": ResetCardStatusDisabled, "updated_at": common.GetTimestamp()})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return common.NewMessage("Only unused reset cards can be disabled")
	}
	return nil
}

// AdminDeleteResetCard deletes a card that has not been used.
func AdminDeleteResetCard(id int) error {
	if id <= 0 {
		return errors.New("invalid reset card id")
	}
	result := DB.Where("id = ? AND status <> ?", id, ResetCardStatusUsed).Delete(&ResetCard{})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return common.NewMessage("Used reset cards cannot be deleted")
	}
	return nil
}

// CountResetCardPlanPurchases counts the completed orders a user placed for a
// reset-card plan. It backs MaxPurchasePerUser for those plans: every purchase
// delivers ResetCardCount cards, so counting cards would misapply the limit.
func CountResetCardPlanPurchases(userId int, planId int) (int64, error) {
	if userId <= 0 || planId <= 0 {
		return 0, errors.New("invalid userId or planId")
	}
	var count int64
	err := DB.Model(&SubscriptionOrder{}).
		Where("user_id = ? AND plan_id = ? AND status = ?", userId, planId, common.TopUpStatusSuccess).
		Count(&count).Error
	return count, err
}

// ExpireDueResetCards marks unused cards past their expiry as expired.
func ExpireDueResetCards(limit int) (int, error) {
	if limit <= 0 {
		limit = 200
	}
	now := GetDBTimestamp()
	var cards []ResetCard
	if err := DB.Where("status = ? AND expires_at > 0 AND expires_at <= ?", ResetCardStatusUnused, now).
		Order("expires_at asc").Limit(limit).Find(&cards).Error; err != nil {
		return 0, err
	}
	if len(cards) == 0 {
		return 0, nil
	}
	ids := make([]int, 0, len(cards))
	for _, card := range cards {
		ids = append(ids, card.Id)
	}
	result := DB.Model(&ResetCard{}).
		Where("id IN ? AND status = ?", ids, ResetCardStatusUnused).
		Updates(map[string]any{"status": ResetCardStatusExpired, "updated_at": now})
	if result.Error != nil {
		return 0, result.Error
	}
	return int(result.RowsAffected), nil
}

// CountUnusedResetCards counts a user's currently usable reset cards.
func CountUnusedResetCards(userId int) (int64, error) {
	if userId <= 0 {
		return 0, errors.New("invalid user id")
	}
	now := common.GetTimestamp()
	var count int64
	err := DB.Model(&ResetCard{}).
		Where("user_id = ? AND status = ? AND (expires_at = 0 OR expires_at > ?)", userId, ResetCardStatusUnused, now).
		Count(&count).Error
	return count, err
}

// FormatResetCardExpiry is a small helper for log messages.
func FormatResetCardExpiry(expiresAt int64) string {
	if expiresAt == 0 {
		return "never"
	}
	return time.Unix(expiresAt, 0).Format(time.DateTime)
}
