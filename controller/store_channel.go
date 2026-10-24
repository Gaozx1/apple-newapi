package controller

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"

	"github.com/gin-gonic/gin"
)

// storeChannelView is the wire shape of a channel offer. The merchant key is
// never serialized: only the gateway uses it when it creates the live channel.
type storeChannelView struct {
	Id           int    `json:"id"`
	UserId       int    `json:"user_id"`
	Username     string `json:"username"`
	Name         string `json:"name"`
	Description  string `json:"description"`
	ChannelType  int    `json:"channel_type"`
	BaseURL      string `json:"base_url"`
	GroupName    string `json:"group_name"`
	Models       string `json:"models"`
	Price        int    `json:"price"`
	ChannelId    int    `json:"channel_id"`
	GroupId      int    `json:"group_id"`
	Status       int    `json:"status"`
	SoldCount    int    `json:"sold_count"`
	ErrorCount   int    `json:"error_count"`
	RequestCount int    `json:"request_count"`
	IsOwner      bool   `json:"is_owner"`
	CreatedAt    int64  `json:"created_at"`
}

type storeChannelReportView struct {
	Id         int    `json:"id"`
	OfferId    int    `json:"offer_id"`
	ChannelId  int    `json:"channel_id"`
	ReporterId int    `json:"reporter_id"`
	Reason     string `json:"reason"`
	Status     int    `json:"status"`
	ReviewNote string `json:"review_note"`
	CreatedAt  int64  `json:"created_at"`
	ReviewedAt int64  `json:"reviewed_at"`
}

type storeChannelRequest struct {
	Name        string  `json:"name"`
	Description string  `json:"description"`
	ChannelType int     `json:"channel_type"`
	BaseURL     string  `json:"base_url"`
	Key         string  `json:"key"`
	Models      string  `json:"models"`
	GroupName   string  `json:"group_name"`
	GroupId     int     `json:"group_id"`
	Ratio       float64 `json:"ratio"`
	Status      *int    `json:"status"`
}

type storeChannelReportRequest struct {
	OfferId   int    `json:"offer_id"`
	ChannelId int    `json:"channel_id"`
	Reason    string `json:"reason"`
}

type storeChannelReviewRequest struct {
	Accept bool   `json:"accept"`
	Note   string `json:"note"`
}

func toStoreChannelView(offer *model.StoreChannel, viewerId int) storeChannelView {
	username, _ := model.GetUsernameById(offer.UserId, false)
	return storeChannelView{
		Id:           offer.Id,
		UserId:       offer.UserId,
		Username:     username,
		Name:         offer.Name,
		Description:  offer.Description,
		ChannelType:  offer.ChannelType,
		BaseURL:      offer.BaseURL,
		Models:       offer.Models,
		Price:        offer.Price,
		GroupName:    offer.GroupName,
		GroupId:      offer.GroupId,
		ChannelId:    offer.ChannelId,
		Status:       offer.Status,
		SoldCount:    offer.SoldCount,
		ErrorCount:   offer.ErrorCount,
		RequestCount: offer.RequestCount,
		IsOwner:      offer.UserId == viewerId,
		CreatedAt:    offer.CreatedAt,
	}
}

func toStoreChannelViews(offers []*model.StoreChannel, viewerId int) []storeChannelView {
	views := make([]storeChannelView, 0, len(offers))
	for _, offer := range offers {
		views = append(views, toStoreChannelView(offer, viewerId))
	}
	return views
}

func toStoreChannelReportView(report *model.StoreChannelReport) storeChannelReportView {
	return storeChannelReportView{
		Id:         report.Id,
		OfferId:    report.OfferId,
		ChannelId:  report.ChannelId,
		ReporterId: report.ReporterId,
		Reason:     report.Reason,
		Status:     report.Status,
		ReviewNote: report.ReviewNote,
		CreatedAt:  report.CreatedAt,
		ReviewedAt: report.ReviewedAt,
	}
}

type storeGroupRequest struct {
	Name   string  `json:"name"`
	Ratio  float64 `json:"ratio"`
	Remark string  `json:"remark"`
}

type storeGroupListingRequest struct {
	Listed bool `json:"listed"`
}

// ListStoreGroups lists the groups the caller owns.
func ListStoreGroups(c *gin.Context) {
	groups, err := model.ListUserChannelGroups(c.GetInt("id"))
	if err != nil {
		common.ApiError(c, err)
		return
	}
	counts, err := model.CountStoreChannelsPerGroup(c.GetInt("id"))
	if err != nil {
		common.ApiError(c, err)
		return
	}
	items := make([]gin.H, 0, len(groups))
	for _, group := range groups {
		items = append(items, gin.H{
			"id":            group.Id,
			"user_id":       group.UserId,
			"name":          group.Name,
			"ratio":         group.Ratio,
			"remark":        group.Remark,
			"listed":        group.Listed,
			"created_at":    group.CreatedAt,
			"channel_count": counts[group.Id],
		})
	}
	common.ApiSuccess(c, gin.H{"items": items, "default_group": model.UserChannelGroupName(c.GetInt("id"))})
}

// CreateStoreGroup creates one group for the caller.
func CreateStoreGroup(c *gin.Context) {
	request := storeGroupRequest{}
	if err := c.ShouldBindJSON(&request); err != nil {
		common.ApiErrorMsg(c, "参数格式错误")
		return
	}
	ratio := request.Ratio
	if ratio == 0 {
		ratio = 1
	}
	group, err := model.CreateUserChannelGroup(c.GetInt("id"), request.Name, ratio, request.Remark)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, group)
}

// UpdateStoreGroup edits the ratio and remark of an owned group.
func UpdateStoreGroup(c *gin.Context) {
	groupId, ok := storeRouteId(c)
	if !ok {
		return
	}
	request := storeGroupRequest{}
	if err := c.ShouldBindJSON(&request); err != nil {
		common.ApiErrorMsg(c, "参数格式错误")
		return
	}
	if err := model.UpdateUserChannelGroup(groupId, c.GetInt("id"), request.Ratio, request.Remark); err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, nil)
}

// SetStoreGroupListing publishes or hides a whole owned group on the market.
func SetStoreGroupListing(c *gin.Context) {
	groupId, ok := storeRouteId(c)
	if !ok {
		return
	}
	request := storeGroupListingRequest{}
	if err := c.ShouldBindJSON(&request); err != nil {
		common.ApiErrorMsg(c, "参数格式错误")
		return
	}
	if err := model.SetUserChannelGroupListing(groupId, c.GetInt("id"), request.Listed); err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, nil)
}

// DeleteStoreGroup removes an owned group and disables its channels.
func DeleteStoreGroup(c *gin.Context) {
	groupId, ok := storeRouteId(c)
	if !ok {
		return
	}
	if err := model.DeleteUserChannelGroup(groupId, c.GetInt("id")); err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, nil)
}

// ListMarketStoreGroups lists every group on the market with its channel count
// and whether the caller already claimed it.
func ListMarketStoreGroups(c *gin.Context) {
	groups, err := model.ListMarketStoreGroups()
	if err != nil {
		common.ApiError(c, err)
		return
	}
	claims, err := model.ListStoreGroupClaims(c.GetInt("id"))
	if err != nil {
		common.ApiError(c, err)
		return
	}
	claimed := make(map[int]bool, len(claims))
	for _, claim := range claims {
		claimed[claim.GroupId] = true
	}
	items := make([]gin.H, 0, len(groups))
	for _, group := range groups {
		username, _ := model.GetUsernameById(group.UserId, false)
		items = append(items, gin.H{
			"id":          group.Id,
			"user_id":     group.UserId,
			"username":    username,
			"name":        group.Name,
			"public_name": model.UserChannelGroupPublicName(group.Name),
			"ratio":       group.Ratio,
			"remark":      group.Remark,
			"listed":      group.Listed,
			"created_at":  group.CreatedAt,
			"is_owner":    group.UserId == c.GetInt("id"),
			"claimed":     claimed[group.Id],
		})
	}
	common.ApiSuccess(c, gin.H{"items": items})
}

type storeClaimRequest struct {
	GroupId int `json:"group_id"`
}

// ClaimStoreGroup lets a user claim a listed group so it shows up in their
// model square and their tokens may call its channels.
func ClaimStoreGroup(c *gin.Context) {
	request := storeClaimRequest{}
	if err := c.ShouldBindJSON(&request); err != nil {
		common.ApiErrorMsg(c, "参数格式错误")
		return
	}
	if err := model.ClaimStoreGroup(c.GetInt("id"), request.GroupId); err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, nil)
}

// UnclaimStoreGroup drops a claim.
func UnclaimStoreGroup(c *gin.Context) {
	request := storeClaimRequest{}
	if err := c.ShouldBindJSON(&request); err != nil {
		common.ApiErrorMsg(c, "参数格式错误")
		return
	}
	if err := model.UnclaimStoreGroup(c.GetInt("id"), request.GroupId); err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, nil)
}

// ReportStoreGroup files a complaint about a listed group.
func ReportStoreGroup(c *gin.Context) {
	request := struct {
		GroupId int    `json:"group_id"`
		Reason  string `json:"reason"`
	}{}
	if err := c.ShouldBindJSON(&request); err != nil {
		common.ApiErrorMsg(c, "参数格式错误")
		return
	}
	if err := model.CreateStoreGroupReport(request.GroupId, c.GetInt("id"), request.Reason); err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, nil)
}

// ListMyStoreChannelEarnings is the seller's revenue history: who used their
// channels, how much quota those buyers consumed, and what landed in the wallet
// after the platform fee.
func ListMyStoreChannelEarnings(c *gin.Context) {
	page := common.GetPageQuery(c)
	routes, total, err := model.ListStoreChannelEarningsView(c.GetInt("id"), page.GetPage(), page.GetPageSize())
	if err != nil {
		common.ApiError(c, err)
		return
	}
	summary, err := model.SummarizeStoreChannelEarnings(c.GetInt("id"))
	if err != nil {
		common.ApiError(c, err)
		return
	}
	items := make([]gin.H, 0, len(routes))
	for _, row := range routes {
		items = append(items, gin.H{
			"id":            row.Id,
			"created_at":    row.CreatedAt,
			"buyer_user_id": row.BuyerUserId,
			"buyer_name":    row.BuyerName,
			"group_name":    row.GroupName,
			"channel_name":  row.ChannelName,
			"model_name":    row.ModelName,
			"quota":         row.Quota,
			"quota_amount":  storeAmountFromQuota(row.Quota),
			"fee":           row.Fee,
			"fee_amount":    storeAmountFromQuota(row.Fee),
			"income":        row.Income,
			"income_amount": storeAmountFromQuota(row.Income),
			"ratio":         row.Ratio,
			"request_id":    row.RequestId,
		})
	}
	page.SetTotal(int(total))
	page.SetItems(items)
	common.ApiSuccess(c, gin.H{
		"page": page,
		"summary": gin.H{
			"orders":          summary.Orders,
			"consumed":        summary.Consumed,
			"consumed_amount": storeAmountFromQuota(int(summary.Consumed)),
			"fee":             summary.Fee,
			"fee_amount":      storeAmountFromQuota(int(summary.Fee)),
			"income":          summary.Income,
			"income_amount":   storeAmountFromQuota(int(summary.Income)),
			"unique_buyers":   summary.UniqueBuyers,
		},
	})
}

// ListStoreChannels lists the channel offers on sale.
func ListStoreChannels(c *gin.Context) {
	page := common.GetPageQuery(c)
	offers, total, err := model.ListStoreChannels(c.Query("keyword"), page.GetPage(), page.GetPageSize())
	if err != nil {
		common.ApiError(c, err)
		return
	}
	page.SetTotal(int(total))
	page.SetItems(toStoreChannelViews(offers, c.GetInt("id")))
	common.ApiSuccess(c, page)
}

// ListMyStoreChannels lists the offers the caller uploaded.
func ListMyStoreChannels(c *gin.Context) {
	page := common.GetPageQuery(c)
	offers, total, err := model.ListUserStoreChannels(c.GetInt("id"), page.GetPage(), page.GetPageSize())
	if err != nil {
		common.ApiError(c, err)
		return
	}
	page.SetTotal(int(total))
	page.SetItems(toStoreChannelViews(offers, c.GetInt("id")))
	common.ApiSuccess(c, page)
}

// CreateStoreChannel publishes a channel offer for the caller.
func CreateStoreChannel(c *gin.Context) {
	request := storeChannelRequest{}
	if err := c.ShouldBindJSON(&request); err != nil {
		common.ApiErrorMsg(c, "参数格式错误")
		return
	}
	offer, err := model.NormalizeStoreChannelOffer(
		request.Name, request.Description, request.ChannelType,
		request.BaseURL, request.Key, request.Models,
	)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	offer.UserId = c.GetInt("id")
	offer.GroupName = request.GroupName
	offer.GroupId = request.GroupId
	if request.Status != nil && *request.Status == model.StoreChannelStatusOnSale {
		offer.Status = model.StoreChannelStatusOnSale
	}
	if err := model.CreateStoreChannel(offer); err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, toStoreChannelView(offer, c.GetInt("id")))
}

// UpdateStoreChannel edits one offer the caller owns.
func UpdateStoreChannel(c *gin.Context) {
	offerId, ok := storeRouteId(c)
	if !ok {
		return
	}
	request := storeChannelRequest{}
	if err := c.ShouldBindJSON(&request); err != nil {
		common.ApiErrorMsg(c, "参数格式错误")
		return
	}
	offer, err := model.GetStoreChannelById(offerId)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if offer.UserId != c.GetInt("id") {
		common.ApiError(c, model.ErrStoreChannelNotOwned)
		return
	}
	// The edit form leaves the key blank to mean "keep the stored one".
	if strings.TrimSpace(request.Key) == "" {
		request.Key = offer.Key
	}
	normalized, err := model.NormalizeStoreChannelOffer(
		request.Name, request.Description, request.ChannelType,
		request.BaseURL, request.Key, request.Models,
	)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	offer.Name = normalized.Name
	offer.Description = normalized.Description
	offer.ChannelType = normalized.ChannelType
	offer.BaseURL = normalized.BaseURL
	offer.Key = normalized.Key
	offer.Models = normalized.Models
	offer.Price = normalized.Price
	if request.GroupId > 0 {
		offer.GroupId = request.GroupId
	}
	if request.Status != nil {
		if *request.Status == model.StoreChannelStatusOnSale {
			offer.Status = model.StoreChannelStatusOnSale
		} else {
			offer.Status = model.StoreChannelStatusOffShelf
		}
	}
	if err := model.UpdateStoreChannelOffer(offer); err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, toStoreChannelView(offer, c.GetInt("id")))
}

// DeleteStoreChannel removes an offer and disables its delivered channels.
func DeleteStoreChannel(c *gin.Context) {
	offerId, ok := storeRouteId(c)
	if !ok {
		return
	}
	userId := c.GetInt("id")
	if c.GetInt("role") >= common.RoleAdminUser {
		offer, err := model.GetStoreChannelById(offerId)
		if err != nil {
			common.ApiError(c, err)
			return
		}
		userId = offer.UserId
	}
	if err := model.DeleteStoreChannelOffer(offerId, userId); err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, nil)
}

type storeChannelListingRequest struct {
	Listed bool `json:"listed"`
}

// SetStoreChannelListing puts an owned channel on the market or takes it off.
// Listing adds the market group to the channel's abilities; nothing is created
// per buyer.
func SetStoreChannelListing(c *gin.Context) {
	offerId, ok := storeRouteId(c)
	if !ok {
		return
	}
	request := storeChannelListingRequest{}
	if err := c.ShouldBindJSON(&request); err != nil {
		common.ApiErrorMsg(c, "参数格式错误")
		return
	}
	if err := model.SetStoreChannelListing(offerId, c.GetInt("id"), request.Listed); err != nil {
		common.ApiError(c, err)
		return
	}
	offer, err := model.GetStoreChannelById(offerId)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, toStoreChannelView(offer, c.GetInt("id")))
}

// ReportStoreChannel files a complaint about one delivered channel.
func ReportStoreChannel(c *gin.Context) {
	request := storeChannelReportRequest{}
	if err := c.ShouldBindJSON(&request); err != nil {
		common.ApiErrorMsg(c, "参数格式错误")
		return
	}
	if err := model.CreateStoreChannelReport(request.OfferId, request.ChannelId, c.GetInt("id"), request.Reason); err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, nil)
}

// ListStoreChannelReports is the administrator review queue.
func ListStoreChannelReports(c *gin.Context) {
	status := -1
	if raw := c.Query("status"); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil {
			status = parsed
		}
	}
	page := common.GetPageQuery(c)
	reports, total, err := model.ListStoreChannelReports(status, page.GetPage(), page.GetPageSize())
	if err != nil {
		common.ApiError(c, err)
		return
	}
	views := make([]storeChannelReportView, 0, len(reports))
	for _, report := range reports {
		views = append(views, toStoreChannelReportView(report))
	}
	page.SetTotal(int(total))
	page.SetItems(views)
	common.ApiSuccess(c, page)
}

type storeFetchModelsRequest struct {
	ChannelType int    `json:"channel_type"`
	BaseURL     string `json:"base_url"`
	Key         string `json:"key"`
}

// FetchStoreChannelModels asks the upstream which models it serves, so a
// merchant does not have to type the list by hand. It is the same lookup the
// channel page in the admin panel performs.
func FetchStoreChannelModels(c *gin.Context) {
	request := storeFetchModelsRequest{}
	if err := c.ShouldBindJSON(&request); err != nil {
		common.ApiErrorMsg(c, "参数格式错误")
		return
	}
	baseURL := strings.TrimSpace(request.BaseURL)
	if baseURL == "" {
		baseURL = constant.GetChannelBaseURL(request.ChannelType)
	}
	key := strings.TrimSpace(request.Key)
	if key == "" {
		common.ApiErrorMsg(c, "请先填写渠道密钥")
		return
	}
	channel := &model.Channel{
		Type:    request.ChannelType,
		Key:     strings.Split(key, "\n")[0],
		BaseURL: &baseURL,
	}
	models, err := fetchChannelUpstreamModelIDs(channel)
	if err != nil {
		common.ApiErrorMsg(c, fmt.Sprintf("获取模型列表失败: %s", err.Error()))
		return
	}
	common.ApiSuccess(c, gin.H{"models": models})
}

// ReviewStoreChannelReport applies the administrator decision on one report.
func ReviewStoreChannelReport(c *gin.Context) {
	reportId, ok := storeRouteId(c)
	if !ok {
		return
	}
	request := storeChannelReviewRequest{}
	if err := c.ShouldBindJSON(&request); err != nil {
		common.ApiErrorMsg(c, "参数格式错误")
		return
	}
	if err := model.ReviewStoreChannelReport(reportId, c.GetInt("id"), request.Accept, request.Note); err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, nil)
}
