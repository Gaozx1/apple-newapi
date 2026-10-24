package controller

import (
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"

	"github.com/gin-gonic/gin"
)

// ============================================================================
// Ads API
//
// GET  /api/ads/config      user: feature flag, reward amount, daily cap left
// GET  /api/ads/next        user: one random ad for a placement
// POST /api/ads/:id/view    user: record a completed view (and grant the reward)
// /api/ads/admin/*          admin: CRUD, stats and reward configuration
// ============================================================================

// AdRewardConfig is the admin-editable reward configuration.
func GetAdConfig(c *gin.Context) {
	userId := c.GetInt("id")
	viewsToday := int64(0)
	if common.AdRewardMaxDailyViews > 0 {
		viewsToday, _ = model.CountAdViewsToday(userId, model.AdPlacementSponsor)
	}
	remaining := 0
	if int64(common.AdRewardMaxDailyViews) > viewsToday {
		remaining = common.AdRewardMaxDailyViews - int(viewsToday)
	}
	common.ApiSuccess(c, gin.H{
		"enabled":         common.AdEnabled,
		"reward_quota":    common.AdRewardQuota,
		"reward_amount":   storeAmountFromQuota(common.AdRewardQuota),
		"max_daily_views": common.AdRewardMaxDailyViews,
		"views_today":     viewsToday,
		"remaining_today": remaining,
	})
}

// GetNextAd picks one enabled ad for a placement. Sponsor impressions respect
// the per-user daily cap; the reward page is unlimited by design.
func GetNextAd(c *gin.Context) {
	if !common.AdEnabled {
		common.ApiErrorMsg(c, "广告功能未开启")
		return
	}
	placement := strings.TrimSpace(c.Query("placement"))
	if placement == "" {
		placement = model.AdPlacementReward
	}
	if placement != model.AdPlacementReward && placement != model.AdPlacementSponsor {
		common.ApiErrorMsg(c, "广告位无效")
		return
	}
	userId := c.GetInt("id")
	if placement == model.AdPlacementSponsor && common.AdRewardMaxDailyViews > 0 {
		viewsToday, err := model.CountAdViewsToday(userId, placement)
		if err != nil {
			common.ApiError(c, err)
			return
		}
		if viewsToday >= int64(common.AdRewardMaxDailyViews) {
			common.ApiSuccess(c, gin.H{"ad": nil, "reason": "daily_limit"})
			return
		}
	}
	ads, err := model.GetEnabledAds(placement)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if len(ads) == 0 {
		common.ApiSuccess(c, gin.H{"ad": nil, "reason": "no_ads"})
		return
	}
	total := 0
	for _, ad := range ads {
		total += max(ad.Weight, 1)
	}
	pick := common.GetRandomInt(total)
	chosen := ads[0]
	for _, ad := range ads {
		pick -= max(ad.Weight, 1)
		if pick < 0 {
			chosen = ad
			break
		}
	}
	common.ApiSuccess(c, gin.H{"ad": chosen})
}

// RecordAdView records one completed view and grants the reward for the ads page.
func RecordAdView(c *gin.Context) {
	if !common.AdEnabled {
		common.ApiErrorMsg(c, "广告功能未开启")
		return
	}
	adId, err := strconv.Atoi(c.Param("id"))
	if err != nil || adId <= 0 {
		common.ApiErrorMsg(c, "广告不存在")
		return
	}
	placement := strings.TrimSpace(c.Query("placement"))
	if placement != model.AdPlacementReward && placement != model.AdPlacementSponsor {
		placement = model.AdPlacementReward
	}
	if _, err := model.GetAdById(adId); err != nil {
		common.ApiErrorMsg(c, "广告不存在")
		return
	}
	reward, viewsToday, err := model.RewardAdView(c.GetInt("id"), adId, placement)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, gin.H{
		"reward_quota":  reward,
		"reward_amount": storeAmountFromQuota(reward),
		"views_today":   viewsToday,
	})
}

// AdminListAds lists every ad with its impression statistics.
func AdminListAds(c *gin.Context) {
	ads, err := model.ListAds()
	if err != nil {
		common.ApiError(c, err)
		return
	}
	stats, err := model.GetAdStats()
	if err != nil {
		common.ApiError(c, err)
		return
	}
	byAd := make(map[int]model.AdStats, len(stats))
	for _, item := range stats {
		byAd[item.AdId] = item
	}
	items := make([]gin.H, 0, len(ads))
	for _, ad := range ads {
		stat := byAd[ad.Id]
		items = append(items, gin.H{
			"id":           ad.Id,
			"title":        ad.Title,
			"image_url":    ad.ImageURL,
			"link_url":     ad.LinkURL,
			"html":         ad.HTML,
			"placement":    ad.Placement,
			"weight":       ad.Weight,
			"status":       ad.Status,
			"created_time": ad.CreatedTime,
			"views":        stat.Views,
			"rewarded":     storeAmountFromQuota(int(stat.RewardedQuo)),
		})
	}
	common.ApiSuccess(c, gin.H{
		"items":           items,
		"enabled":         common.AdEnabled,
		"reward_quota":    common.AdRewardQuota,
		"reward_amount":   storeAmountFromQuota(common.AdRewardQuota),
		"max_daily_views": common.AdRewardMaxDailyViews,
	})
}

type adWriteRequest struct {
	Title     string `json:"title"`
	ImageURL  string `json:"image_url"`
	LinkURL   string `json:"link_url"`
	HTML      string `json:"html"`
	Placement string `json:"placement"`
	Weight    int    `json:"weight"`
	Status    int    `json:"status"`
}

// AdminCreateAd inserts one ad.
func AdminCreateAd(c *gin.Context) {
	var request adWriteRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		common.ApiErrorMsg(c, "参数格式错误")
		return
	}
	ad := &model.Ad{
		Title:     request.Title,
		ImageURL:  request.ImageURL,
		LinkURL:   request.LinkURL,
		HTML:      request.HTML,
		Placement: request.Placement,
		Weight:    request.Weight,
		Status:    request.Status,
	}
	if err := model.CreateAd(ad); err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, gin.H{"id": ad.Id})
}

// AdminUpdateAd rewrites one ad.
func AdminUpdateAd(c *gin.Context) {
	adId, err := strconv.Atoi(c.Param("id"))
	if err != nil || adId <= 0 {
		common.ApiErrorMsg(c, "广告不存在")
		return
	}
	var request adWriteRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		common.ApiErrorMsg(c, "参数格式错误")
		return
	}
	ad := &model.Ad{
		Id:        adId,
		Title:     request.Title,
		ImageURL:  request.ImageURL,
		LinkURL:   request.LinkURL,
		HTML:      request.HTML,
		Placement: request.Placement,
		Weight:    request.Weight,
		Status:    request.Status,
	}
	if err := model.UpdateAd(ad); err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, gin.H{"id": ad.Id})
}

// AdminDeleteAd removes one ad.
func AdminDeleteAd(c *gin.Context) {
	adId, err := strconv.Atoi(c.Param("id"))
	if err != nil || adId <= 0 {
		common.ApiErrorMsg(c, "广告不存在")
		return
	}
	if err := model.DeleteAd(adId); err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, nil)
}

// AdminUpdateAdConfig stores the enable flag, reward amount and daily cap.
func AdminUpdateAdConfig(c *gin.Context) {
	var request struct {
		Enabled       *bool `json:"enabled"`
		RewardQuota   *int  `json:"reward_quota"`
		MaxDailyViews *int  `json:"max_daily_views"`
	}
	if err := c.ShouldBindJSON(&request); err != nil {
		common.ApiErrorMsg(c, "参数格式错误")
		return
	}
	if request.RewardQuota != nil {
		if *request.RewardQuota < 0 || *request.RewardQuota > common.MaxWalletQuota {
			common.ApiErrorMsg(c, "奖励额度超出范围")
			return
		}
		if err := model.UpdateOption("AdRewardQuota", strconv.Itoa(*request.RewardQuota)); err != nil {
			common.ApiError(c, err)
			return
		}
	}
	if request.MaxDailyViews != nil {
		if *request.MaxDailyViews < 0 || *request.MaxDailyViews > 10000 {
			common.ApiErrorMsg(c, "每日上限超出范围")
			return
		}
		if err := model.UpdateOption("AdRewardMaxDailyViews", strconv.Itoa(*request.MaxDailyViews)); err != nil {
			common.ApiError(c, err)
			return
		}
	}
	if request.Enabled != nil {
		value := "false"
		if *request.Enabled {
			value = "true"
		}
		if err := model.UpdateOption("AdEnabled", value); err != nil {
			common.ApiError(c, err)
			return
		}
	}
	common.ApiSuccess(c, gin.H{
		"enabled":         common.AdEnabled,
		"reward_quota":    common.AdRewardQuota,
		"max_daily_views": common.AdRewardMaxDailyViews,
	})
}
