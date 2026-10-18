package controller

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/middleware"
	"github.com/QuantumNous/new-api/model"
	pluginruntime "github.com/QuantumNous/new-api/pkg/jsplugin"
	"github.com/QuantumNous/new-api/relay"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

type nativeRouteBilling struct {
	events      []string
	preConsumed int
	userID      int
	settled     bool
}

func (b *nativeRouteBilling) Settle(int, int) error {
	b.events = append(b.events, "settle")
	b.settled = true
	return nil
}

func (b *nativeRouteBilling) Refund(*gin.Context) {
	b.events = append(b.events, "refund")
	if !b.settled && b.preConsumed > 0 {
		_ = model.IncreaseUserQuota(b.userID, b.preConsumed, true)
		b.preConsumed = 0
	}
}

func (b *nativeRouteBilling) NeedsRefund() bool {
	return !b.settled && b.preConsumed > 0
}

func (b *nativeRouteBilling) GetPreConsumedQuota() int {
	return b.preConsumed
}

func (b *nativeRouteBilling) Reserve(quota int) error {
	b.events = append(b.events, "reserve")
	if err := model.DecreaseUserQuota(b.userID, quota, true); err != nil {
		return err
	}
	b.preConsumed = quota
	return nil
}

func TestKlingNativeRouteSubmitPollSettleAndQuery(t *testing.T) {
	gin.SetMode(gin.TestMode)
	service.InitHttpClient()

	previousDB := model.DB
	previousLogDB := model.LOG_DB
	previousMemoryCache := common.MemoryCacheEnabled
	previousBatchUpdate := common.BatchUpdateEnabled
	previousLogConsume := common.LogConsumeEnabled
	previousRedisEnabled := common.RedisEnabled
	database, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, database.AutoMigrate(&model.User{}, &model.Channel{}, &model.Task{}, &model.Log{}))
	model.DB = database
	model.LOG_DB = database
	common.MemoryCacheEnabled = false
	common.BatchUpdateEnabled = false
	common.LogConsumeEnabled = false
	common.RedisEnabled = false
	previousModelRatios := ratio_setting.ModelRatio2JSONString()
	require.NoError(t, ratio_setting.UpdateModelRatioByJSONString(`{"kling-v1":1}`))
	t.Cleanup(func() {
		model.DB = previousDB
		model.LOG_DB = previousLogDB
		common.MemoryCacheEnabled = previousMemoryCache
		common.BatchUpdateEnabled = previousBatchUpdate
		common.LogConsumeEnabled = previousLogConsume
		common.RedisEnabled = previousRedisEnabled
		require.NoError(t, ratio_setting.UpdateModelRatioByJSONString(previousModelRatios))
	})
	require.NoError(t, database.Create(&model.User{
		Id:       7,
		Username: "native-route-user",
		Group:    "default",
		Quota:    1_000_000,
	}).Error)

	var submitCalls atomic.Int32
	var queryCalls atomic.Int32
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/kling/v1/videos/text2video":
			submitCalls.Add(1)
			body, readErr := io.ReadAll(r.Body)
			if !assert.NoError(t, readErr) {
				http.Error(w, "read request", http.StatusInternalServerError)
				return
			}
			assert.Contains(t, string(body), `"model_name":"kling-v1"`)
			_, _ = io.WriteString(w, `{"code":0,"message":"","data":{"task_id":"kling-private-1","task_status":"submitted"}}`)
		case r.Method == http.MethodGet && r.URL.Path == "/kling/v1/videos/text2video/kling-private-1":
			queryCalls.Add(1)
			_, _ = io.WriteString(w, `{"code":0,"message":"","data":{"task_id":"kling-private-1","task_status":"succeed","task_status_msg":"","task_result":{"videos":[{"id":"video-private","url":"https://cdn.example/video.mp4","duration":"5"}]},"final_unit_deduction":"1"}}`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer upstream.Close()

	channel := model.Channel{
		Type:    constant.ChannelTypeKling,
		Name:    "kling-native-e2e",
		Key:     "sk-test",
		BaseURL: &upstream.URL,
		Status:  common.ChannelStatusEnabled,
		Models:  "kling-v1",
		Group:   "default",
	}
	require.NoError(t, database.Create(&channel).Error)

	generation := pluginruntime.DefaultRegistry.Generation()
	require.NotNil(t, generation)
	submitBinding, found := generation.LookupDeclaredRoute(http.MethodPost, "/kling/v1/videos/text2video")
	require.True(t, found)
	require.Equal(t, "kling", submitBinding.Plugin.Meta.Key)

	submitRecorder := httptest.NewRecorder()
	submitContext, _ := gin.CreateTestContext(submitRecorder)
	submitContext.Request = httptest.NewRequest(
		http.MethodPost,
		"/kling/v1/videos/text2video",
		bytes.NewBufferString(`{"model_name":"kling-v1","prompt":"a lighthouse"}`),
	)
	submitContext.Request.Header.Set("Content-Type", "application/json")
	submitContext.Set(pluginruntime.ContextKeyPinnedRoute, pluginruntime.PinnedRoute{
		Generation: generation,
		Plugin:     submitBinding.Plugin,
		Route:      submitBinding.Route,
	})
	common.SetContextKey(submitContext, constant.ContextKeyUserId, 7)
	common.SetContextKey(submitContext, constant.ContextKeyUserGroup, "default")
	common.SetContextKey(submitContext, constant.ContextKeyUsingGroup, "default")
	common.SetContextKey(submitContext, constant.ContextKeyTokenGroup, "default")
	common.SetContextKey(submitContext, constant.ContextKeyUserQuota, 1_000_000)

	middleware.PrepareTaskPluginRoute()(submitContext)
	require.False(t, submitContext.IsAborted(), submitRecorder.Body.String())
	require.Equal(t, "kling-v1", submitContext.GetString("resolved_task_model"))
	require.Equal(t, "text_to_video", submitContext.GetString("task_action"))
	require.Nil(t, middleware.SetupContextForSelectedChannel(submitContext, &channel, "kling-v1"))

	billing := &nativeRouteBilling{userID: 7}
	relayInfo := &relaycommon.RelayInfo{
		UserId:          7,
		UserGroup:       "default",
		UsingGroup:      "default",
		UserQuota:       1_000_000,
		TokenGroup:      "default",
		OriginModelName: "kling-v1",
		Billing:         billing,
		TaskRelayInfo: &relaycommon.TaskRelayInfo{
			Action:        submitContext.GetString("task_action"),
			PublicTaskID:  "task_kling_public",
			LockedChannel: &channel,
		},
	}

	outcome, taskErr := executeTaskSubmissionWith(submitContext, relayInfo, relay.RelayTaskSubmit)
	require.Nil(t, taskErr)
	require.NotNil(t, outcome)
	require.Equal(t, []string{"reserve", "settle"}, billing.events)
	require.False(t, submitContext.Writer.Written())

	presentTaskSubmission(submitContext, outcome)
	require.Equal(t, http.StatusOK, submitRecorder.Code)
	assert.Contains(t, submitRecorder.Body.String(), `"task_id":"task_kling_public"`)
	assert.NotContains(t, submitRecorder.Body.String(), "kling-private-1")
	assert.Equal(t, int32(1), submitCalls.Load())

	var persisted model.Task
	require.NoError(t, database.Where("task_id = ?", "task_kling_public").First(&persisted).Error)
	assert.Equal(t, constant.TaskPlatform("kling"), persisted.Platform)
	assert.Equal(t, "kling-private-1", persisted.PrivateData.UpstreamTaskID)
	assert.Equal(t, model.TaskStatus(model.TaskStatusNotStart), persisted.Status)

	previousAdaptorFactory := service.GetTaskAdaptorFunc
	service.GetTaskAdaptorFunc = func(platform constant.TaskPlatform) service.TaskPollingAdaptor {
		return relay.GetTaskAdaptor(platform)
	}
	t.Cleanup(func() { service.GetTaskAdaptorFunc = previousAdaptorFactory })
	service.DispatchPlatformUpdate(
		context.Background(),
		persisted.Platform,
		map[int][]string{channel.Id: {"kling-private-1"}},
		map[string]*model.Task{"kling-private-1": &persisted},
	)

	require.NoError(t, database.Where("task_id = ?", "task_kling_public").First(&persisted).Error)
	assert.Equal(t, model.TaskStatus(model.TaskStatusSuccess), persisted.Status)
	assert.Equal(t, "100%", persisted.Progress)
	assert.Equal(t, 1, persisted.Quota)
	assert.Equal(t, int32(1), queryCalls.Load())
	var settledUser model.User
	require.NoError(t, database.First(&settledUser, 7).Error)
	assert.Equal(t, 999_999, settledUser.Quota)

	queryBinding, found := generation.LookupDeclaredRoute(http.MethodGet, "/kling/v1/videos/text2video/:task_id")
	require.True(t, found)
	queryRecorder := httptest.NewRecorder()
	queryContext, _ := gin.CreateTestContext(queryRecorder)
	queryContext.Request = httptest.NewRequest(http.MethodGet, "/kling/v1/videos/text2video/task_kling_public", nil)
	queryContext.Params = gin.Params{{Key: "task_id", Value: "task_kling_public"}}
	queryContext.Set(pluginruntime.ContextKeyPinnedRoute, pluginruntime.PinnedRoute{
		Generation: generation,
		Plugin:     queryBinding.Plugin,
		Route:      queryBinding.Route,
	})
	common.SetContextKey(queryContext, constant.ContextKeyUserId, 7)

	middleware.PrepareTaskPluginRoute()(queryContext)

	require.True(t, queryContext.IsAborted())
	require.Equal(t, http.StatusOK, queryRecorder.Code)
	assert.Contains(t, queryRecorder.Body.String(), `"task_id":"task_kling_public"`)
	assert.Contains(t, queryRecorder.Body.String(), `"task_status":"succeed"`)
	assert.NotContains(t, queryRecorder.Body.String(), "kling-private-1")
	assert.NotContains(t, queryRecorder.Body.String(), upstream.URL)
}

// A native submit route whose request descriptor carries no body must still
// complete. d1jiema is such a plugin: every vendor parameter travels in the
// query string, so the upstream request is built without a body and the relay
// used to close the nil req.Body, panicking inside the plugin route.
func TestD1jiemaBodylessNativeSubmitCompletesImmediately(t *testing.T) {
	gin.SetMode(gin.TestMode)
	service.InitHttpClient()

	previousDB := model.DB
	previousLogDB := model.LOG_DB
	previousMainType, previousLogType := common.MainDatabaseType(), common.LogDatabaseType()
	previousMemoryCache := common.MemoryCacheEnabled
	previousRedisEnabled := common.RedisEnabled
	previousIsMaster := common.IsMasterNode
	previousSQLitePath := common.SQLitePath
	previousPrices := ratio_setting.ModelPrice2JSONString()
	previousGroups := ratio_setting.GroupRatio2JSONString()

	common.SetDatabaseTypes(common.DatabaseTypeSQLite, common.DatabaseTypeSQLite)
	common.IsMasterNode = true
	common.MemoryCacheEnabled = false
	common.BatchUpdateEnabled = false
	common.LogConsumeEnabled = false
	common.RedisEnabled = false
	common.SQLitePath = filepath.Join(t.TempDir(), "d1jiema-route.db")
	t.Setenv("SQL_DSN", "")
	t.Setenv("LOG_SQL_DSN", "")
	require.NoError(t, model.InitDB())
	require.NoError(t, model.InitLogDB())
	database := model.DB
	require.NotNil(t, database)

	t.Cleanup(func() {
		model.DB = previousDB
		model.LOG_DB = previousLogDB
		common.SetDatabaseTypes(previousMainType, previousLogType)
		common.MemoryCacheEnabled = previousMemoryCache
		common.RedisEnabled = previousRedisEnabled
		common.IsMasterNode = previousIsMaster
		common.SQLitePath = previousSQLitePath
		_ = ratio_setting.UpdateModelPriceByJSONString(previousPrices)
		_ = ratio_setting.UpdateGroupRatioByJSONString(previousGroups)
		if sqlDB, err := database.DB(); err == nil {
			_ = sqlDB.Close()
		}
	})

	require.NoError(t, ratio_setting.UpdateModelPriceByJSONString(`{"getPhone":0.05}`))
	require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(`{"default":1}`))

	var upstreamQuery string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upstreamQuery = r.URL.RawQuery
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte("13012345678"))
	}))
	defer upstream.Close()

	require.NoError(t, database.Create(&model.User{
		Id: 1, Username: "sms", Group: "default", Quota: 1_000_000, Status: common.UserStatusEnabled,
		AffCode: "sms1",
	}).Error)
	require.NoError(t, database.Create(&model.User{
		Id: 2, Username: "sms-sub", Group: "default", Quota: 1_000_000, Status: common.UserStatusEnabled,
		AffCode: "sms2",
	}).Error)
	token := &model.Token{
		Id: 1, UserId: 1, Key: "smscode", Name: "sms", Status: common.TokenStatusEnabled,
		ExpiredTime: -1, UnlimitedQuota: true, Group: "default",
	}
	require.NoError(t, database.Create(token).Error)
	require.NoError(t, database.Create(&model.Token{
		Id: 2, UserId: 2, Key: "smscodesub", Name: "sms-sub", Status: common.TokenStatusEnabled,
		ExpiredTime: -1, UnlimitedQuota: true, Group: "default",
	}).Error)
	// The live deployment funds part of this feature from a subscription, so the
	// billing session must survive that branch as well.
	require.NoError(t, database.Create(&model.SubscriptionPlan{
		Id: 1, Title: "sms-test", Enabled: true, DurationUnit: "month", DurationValue: 1,
		TotalAmount: 10_000_000, Currency: "USD",
	}).Error)
	require.NoError(t, database.Create(&model.UserSubscription{
		Id: 1, UserId: 2, PlanId: 1, AmountTotal: 10_000_000, AmountUsed: 0,
		StartTime: time.Now().Unix() - 3600, EndTime: time.Now().Unix() + 30*86400,
		Status: "active", AllowWalletOverflow: true,
	}).Error)
	setting := `{"task_plugin_key":"d1jiema"}`
	channel := model.Channel{
		Id: 1, Type: constant.ChannelTypeTaskPlugin, Name: "d1jiema", Key: "tok-123",
		BaseURL: &upstream.URL, Models: "getPhone", Group: "default",
		Status: common.ChannelStatusEnabled, Setting: &setting,
	}
	require.NoError(t, database.Create(&channel).Error)

	generation := pluginruntime.DefaultRegistry.Generation()
	require.NotNil(t, generation)
	binding, found := generation.LookupDeclaredRoute(http.MethodPost, "/d1jiema/getPhone")
	require.True(t, found)

	for _, tc := range []struct {
		funding string
		user    *model.Token
		want    string
	}{
		{"wallet", token, "(funding=wallet)"},
		{"subscription", &model.Token{Id: 2, UserId: 2, Key: "smscodesub", Name: "sms-sub",
			Status: common.TokenStatusEnabled, ExpiredTime: -1, UnlimitedQuota: true, Group: "default"},
			"(funding=subscription)"},
	} {
		t.Run(tc.funding, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(recorder)
			c.Request = httptest.NewRequest(http.MethodPost, "/d1jiema/getPhone",
				bytes.NewBufferString(`{"model":"getPhone","keyWord":"毛竹"}`))
			c.Request.Header.Set("Content-Type", "application/json")
			c.Set(pluginruntime.ContextKeyPinnedRoute, pluginruntime.PinnedRoute{
				Generation: generation, Plugin: binding.Plugin, Route: binding.Route,
			})
			c.Set(pluginruntime.ContextKeyPinnedPlugin, pluginruntime.PinnedPlugin{
				Generation: generation, Plugin: binding.Plugin,
			})

			middleware.PrepareTaskPluginRoute()(c)
			require.False(t, c.IsAborted(), recorder.Body.String())
			require.Nil(t, middleware.SetupContextForSelectedChannel(c, &channel, "getPhone"))
			common.SetContextKey(c, constant.ContextKeyUserId, tc.user.UserId)
			common.SetContextKey(c, constant.ContextKeyUserGroup, "default")
			common.SetContextKey(c, constant.ContextKeyUsingGroup, "default")
			common.SetContextKey(c, constant.ContextKeyUserQuota, 1_000_000)
			require.NoError(t, middleware.SetupContextForToken(c, tc.user))

			RelayTask(c)

			require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
			assert.Contains(t, recorder.Body.String(), `"code":"ok"`)
			assert.Contains(t, recorder.Body.String(), `"phone":"13012345678"`)
			assert.Equal(t,
				"code=getPhone&token=tok-123&keyWord=%E6%AF%9B%E7%AB%B9&cardType=%E5%85%A8%E9%83%A8",
				upstreamQuery)
		})
	}

	var persisted model.Task
	require.NoError(t, database.Where("platform = ?", "d1jiema").Order("id desc").First(&persisted).Error)
	assert.Equal(t, model.TaskStatus(model.TaskStatusSuccess), persisted.Status)
	assert.Equal(t, "getPhone", persisted.Action)
	assert.NotEmpty(t, persisted.PrivateData.UpstreamTaskID)
}

// A reading that has not received the message is not a delivery: the plugin
// reports it as a terminal failure, so the host refunds the whole reservation
// and a caller that keeps reading pays only for the read that returns the code.
func TestD1jiemaUndeliveredReadIsRefunded(t *testing.T) {
	gin.SetMode(gin.TestMode)
	service.InitHttpClient()

	previousDB := model.DB
	previousLogDB := model.LOG_DB
	previousMainType, previousLogType := common.MainDatabaseType(), common.LogDatabaseType()
	previousMemoryCache := common.MemoryCacheEnabled
	previousRedisEnabled := common.RedisEnabled
	previousIsMaster := common.IsMasterNode
	previousSQLitePath := common.SQLitePath
	previousPrices := ratio_setting.ModelPrice2JSONString()
	previousGroups := ratio_setting.GroupRatio2JSONString()

	common.SetDatabaseTypes(common.DatabaseTypeSQLite, common.DatabaseTypeSQLite)
	common.IsMasterNode = true
	common.MemoryCacheEnabled = false
	common.BatchUpdateEnabled = false
	common.LogConsumeEnabled = false
	common.RedisEnabled = false
	common.SQLitePath = filepath.Join(t.TempDir(), "d1jiema-undelivered.db")
	t.Setenv("SQL_DSN", "")
	t.Setenv("LOG_SQL_DSN", "")
	require.NoError(t, model.InitDB())
	require.NoError(t, model.InitLogDB())
	database := model.DB
	require.NotNil(t, database)

	t.Cleanup(func() {
		model.DB = previousDB
		model.LOG_DB = previousLogDB
		common.SetDatabaseTypes(previousMainType, previousLogType)
		common.MemoryCacheEnabled = previousMemoryCache
		common.RedisEnabled = previousRedisEnabled
		common.IsMasterNode = previousIsMaster
		common.SQLitePath = previousSQLitePath
		_ = ratio_setting.UpdateModelPriceByJSONString(previousPrices)
		_ = ratio_setting.UpdateGroupRatioByJSONString(previousGroups)
		if sqlDB, err := database.DB(); err == nil {
			_ = sqlDB.Close()
		}
	})

	require.NoError(t, ratio_setting.UpdateModelPriceByJSONString(`{"getMsg":0.25}`))
	require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(`{"default":1}`))

	// The vendor answers this while the SMS has not been delivered.
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte("尚未收到包含关键字“毛竹”的短信，请5秒后再收取。[尚未收到]"))
	}))
	defer upstream.Close()

	require.NoError(t, database.Create(&model.User{
		Id: 1, Username: "sms", Group: "default", Quota: 1_000_000, Status: common.UserStatusEnabled,
		AffCode: "sms1",
	}).Error)
	token := &model.Token{
		Id: 1, UserId: 1, Key: "smscode", Name: "sms", Status: common.TokenStatusEnabled,
		ExpiredTime: -1, UnlimitedQuota: true, Group: "default",
	}
	require.NoError(t, database.Create(token).Error)
	setting := `{"task_plugin_key":"d1jiema"}`
	channel := model.Channel{
		Id: 1, Type: constant.ChannelTypeTaskPlugin, Name: "d1jiema", Key: "tok-123",
		BaseURL: &upstream.URL, Models: "getMsg", Group: "default",
		Status: common.ChannelStatusEnabled, Setting: &setting,
	}
	require.NoError(t, database.Create(&channel).Error)

	generation := pluginruntime.DefaultRegistry.Generation()
	require.NotNil(t, generation)
	binding, found := generation.LookupDeclaredRoute(http.MethodPost, "/d1jiema/getMsg")
	require.True(t, found)

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/d1jiema/getMsg",
		bytes.NewBufferString(`{"model":"getMsg","phone":"13012345678","keyWord":"毛竹"}`))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Set(pluginruntime.ContextKeyPinnedRoute, pluginruntime.PinnedRoute{
		Generation: generation, Plugin: binding.Plugin, Route: binding.Route,
	})
	c.Set(pluginruntime.ContextKeyPinnedPlugin, pluginruntime.PinnedPlugin{
		Generation: generation, Plugin: binding.Plugin,
	})

	middleware.PrepareTaskPluginRoute()(c)
	require.False(t, c.IsAborted(), recorder.Body.String())
	require.Nil(t, middleware.SetupContextForSelectedChannel(c, &channel, "getMsg"))
	common.SetContextKey(c, constant.ContextKeyUserId, 1)
	common.SetContextKey(c, constant.ContextKeyUserGroup, "default")
	common.SetContextKey(c, constant.ContextKeyUsingGroup, "default")
	common.SetContextKey(c, constant.ContextKeyUserQuota, 1_000_000)
	require.NoError(t, middleware.SetupContextForToken(c, token))

	RelayTask(c)

	// The failure is presented through the route's own renderer, so the caller
	// reads it from the envelope instead of the HTTP status.
	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
	assert.Contains(t, recorder.Body.String(), `"code":"error"`)
	assert.Contains(t, recorder.Body.String(), "尚未收到")
	assert.Contains(t, recorder.Body.String(), "本次不计费")

	var refreshed model.User
	require.NoError(t, database.First(&refreshed, 1).Error)
	assert.Equal(t, 1_000_000, refreshed.Quota, "an undelivered read must not be charged")

	var undelivered model.Task
	require.NoError(t, database.Where("platform = ?", "d1jiema").First(&undelivered).Error)
	assert.Equal(t, model.TaskStatus(model.TaskStatusFailure), undelivered.Status)
	assert.Equal(t, 0, undelivered.Quota)
	assert.Contains(t, undelivered.FailReason, "尚未收到")
}
