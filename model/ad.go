package model

import (
	"errors"
	"strings"

	"github.com/QuantumNous/new-api/common"

	"gorm.io/gorm"
)

// ============================================================================
// Advertisements (广告位)
//
// Users watch ads on a dedicated page. Two placements exist:
//   - sponsor: shown randomly across the site (pricing page, model square),
//     capped per user by AdRewardMaxDailyViews.
//   - reward: the /ads page, unlimited viewing, pays AdRewardQuota per view.
// Every completed view is recorded in ad_views so the per-user cap and the
// reward ledger stay auditable.
// ============================================================================

const (
	AdPlacementSponsor = "sponsor"
	AdPlacementReward  = "reward"

	AdStatusDisabled = 0
	AdStatusEnabled  = 1
)

// Ad is one administrator-managed advertisement. HTML/JS is rendered inside a
// sandboxed iframe, so provider snippets (Alipay, AdSense, …) work unchanged.
type Ad struct {
	Id          int    `json:"id"`
	Title       string `json:"title" gorm:"type:varchar(128);not null"`
	ImageURL    string `json:"image_url" gorm:"type:varchar(512)"`
	LinkURL     string `json:"link_url" gorm:"type:varchar(512)"`
	HTML        string `json:"html" gorm:"type:text"`
	Placement   string `json:"placement" gorm:"type:varchar(16);default:'reward'"`
	Weight      int    `json:"weight" gorm:"default:1"`
	Status      int    `json:"status" gorm:"default:1"`
	CreatedTime int64  `json:"created_time" gorm:"bigint"`
	UpdatedTime int64  `json:"updated_time" gorm:"bigint"`
}

func (Ad) TableName() string {
	return "ads"
}

// AdView records one completed view for reward settlement and rate limiting.
type AdView struct {
	Id          int64  `json:"id" gorm:"primaryKey;autoIncrement"`
	AdId        int    `json:"ad_id" gorm:"index"`
	UserId      int    `json:"user_id" gorm:"index"`
	Placement   string `json:"placement" gorm:"type:varchar(16)"`
	RewardQuota int    `json:"reward_quota" gorm:"default:0"`
	CreatedTime int64  `json:"created_time" gorm:"bigint;index"`
}

func (AdView) TableName() string {
	return "ad_views"
}

// GetEnabledAds returns the enabled ads for a placement, ready for random draw.
func GetEnabledAds(placement string) ([]Ad, error) {
	ads := make([]Ad, 0, 8)
	err := DB.Where("status = ? AND placement = ?", AdStatusEnabled, placement).
		Order("id asc").Find(&ads).Error
	return ads, err
}

// CountAdViewsToday counts views of one placement by a user since local midnight.
func CountAdViewsToday(userId int, placement string) (int64, error) {
	start := common.GetTimestamp() - int64(common.GetTimestamp()%86400)
	var count int64
	err := DB.Model(&AdView{}).
		Where("user_id = ? AND placement = ? AND created_time >= ?", userId, placement, start).
		Count(&count).Error
	return count, err
}

// RewardAdView credits AdRewardQuota for one completed ad view. The view is
// recorded first; a failure to credit rolls the transaction back so a user is
// never shown a rewarded view without the balance landing.
func RewardAdView(userId, adId int, placement string) (int, int64, error) {
	rewardQuota := 0
	viewsToday := int64(0)
	err := DB.Transaction(func(tx *gorm.DB) error {
		if placement == AdPlacementSponsor {
			// Sponsor impressions never pay out; they only count toward the cap.
			view := &AdView{AdId: adId, UserId: userId, Placement: placement, CreatedTime: common.GetTimestamp()}
			return tx.Create(view).Error
		}
		if rewardQuota <= 0 {
			rewardQuota = common.AdRewardQuota
		}
		if rewardQuota <= 0 {
			return errors.New("ad reward is not configured")
		}
		var user User
		if err := lockForUpdate(tx).First(&user, userId).Error; err != nil {
			return err
		}
		view := &AdView{
			AdId:        adId,
			UserId:      userId,
			Placement:   placement,
			RewardQuota: rewardQuota,
			CreatedTime: common.GetTimestamp(),
		}
		if err := tx.Create(view).Error; err != nil {
			return err
		}
		if err := tx.Model(&User{}).Where("id = ?", userId).
			Update("quota", gorm.Expr("quota + ?", rewardQuota)).Error; err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return 0, 0, err
	}
	viewsToday, _ = CountAdViewsToday(userId, placement)
	return rewardQuota, viewsToday, nil
}

// GetAdById loads one ad.
func GetAdById(id int) (*Ad, error) {
	ad := &Ad{Id: id}
	if err := DB.First(ad, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return ad, nil
}

// ListAds returns every ad for the admin console, newest first.
func ListAds() ([]Ad, error) {
	ads := make([]Ad, 0, 16)
	err := DB.Order("id desc").Find(&ads).Error
	return ads, err
}

// NormalizeAd validates and canonicalizes one ad before persistence.
func NormalizeAd(ad *Ad) error {
	ad.Title = strings.TrimSpace(ad.Title)
	ad.ImageURL = strings.TrimSpace(ad.ImageURL)
	ad.LinkURL = strings.TrimSpace(ad.LinkURL)
	ad.Placement = strings.TrimSpace(ad.Placement)
	if ad.Title == "" {
		return errors.New("title is required")
	}
	if ad.Placement == "" {
		ad.Placement = AdPlacementReward
	}
	if ad.Placement != AdPlacementReward && ad.Placement != AdPlacementSponsor {
		return errors.New("placement must be reward or sponsor")
	}
	if strings.TrimSpace(ad.HTML) == "" && ad.ImageURL == "" {
		return errors.New("provide ad HTML or an image URL")
	}
	if ad.Weight < 1 {
		ad.Weight = 1
	}
	if ad.Status != AdStatusEnabled && ad.Status != AdStatusDisabled {
		ad.Status = AdStatusEnabled
	}
	return nil
}

// CreateAd inserts one ad.
func CreateAd(ad *Ad) error {
	if err := NormalizeAd(ad); err != nil {
		return err
	}
	ad.CreatedTime = common.GetTimestamp()
	ad.UpdatedTime = ad.CreatedTime
	return DB.Create(ad).Error
}

// UpdateAd rewrites the mutable fields of one ad.
func UpdateAd(ad *Ad) error {
	if err := NormalizeAd(ad); err != nil {
		return err
	}
	ad.UpdatedTime = common.GetTimestamp()
	return DB.Model(&Ad{}).Where("id = ?", ad.Id).Updates(map[string]any{
		"title":        ad.Title,
		"image_url":    ad.ImageURL,
		"link_url":     ad.LinkURL,
		"html":         ad.HTML,
		"placement":    ad.Placement,
		"weight":       ad.Weight,
		"status":       ad.Status,
		"updated_time": ad.UpdatedTime,
	}).Error
}

// DeleteAd removes one ad and its view history.
func DeleteAd(id int) error {
	if err := DB.Delete(&Ad{}, id).Error; err != nil {
		return err
	}
	return DB.Where("ad_id = ?", id).Delete(&AdView{}).Error
}

// AdStats summarizes one ad's impressions for the admin console.
type AdStats struct {
	AdId        int   `json:"ad_id"`
	Views       int64 `json:"views"`
	RewardedQuo int64 `json:"rewarded_quota"`
}

// GetAdStats aggregates view counts, newest first by ad id.
func GetAdStats() ([]AdStats, error) {
	stats := make([]AdStats, 0, 16)
	err := DB.Model(&AdView{}).
		Select("ad_id, COUNT(*) as views, COALESCE(SUM(reward_quota),0) as rewarded_quo").
		Group("ad_id").Order("ad_id desc").Scan(&stats).Error
	return stats, err
}
