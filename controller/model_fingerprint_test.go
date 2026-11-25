package controller

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	lmfpd "github.com/Ikaleio/lm-detector/sdk/go"
	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/system_setting"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestFingerprintVerdicts(t *testing.T) {
	high, boundary, low := 0.9807, 0.90, 0.89
	tests := []struct {
		model, ref, decision string
		confidence           *float64
		want                 string
	}{
		{"alpha", "", "scored", &high, "match"}, {"alias", "Alpha Model", "scored", &high, "match"},
		{"beta", "", "scored", &high, "mismatch"}, {"unknown", "", "scored", &high, "reference_missing"},
		{"alpha", "", "scored", &boundary, "inconclusive"}, {"alpha", "", "scored", &low, "inconclusive"}, {"alpha", "", "partial", nil, "inconclusive"}, {"beta", "", "not_model", nil, "inconclusive"}}
	for _, tt := range tests {
		a := lmfpd.Analysis{Decision: tt.decision, Prediction: "alpha", Probability: tt.confidence, Results: []lmfpd.AnalysisResult{{Model: "alpha", DisplayName: "Alpha Model"}, {Model: "beta"}}}
		assert.Equal(t, tt.want, fingerprintVerdict(fingerprintTarget{Model: tt.model, Reference: tt.ref}, &a))
	}
	for _, reply := range []string{`{"model":"a","choices":[{"message":{"content":[{"type":"text","text":"17"}]}},{"message":{"content":"99"}}]}`, `{"model":"a","content":[{"type":"thinking","text":"99"},{"type":"text","text":"17"}]}`, `{"model":"a","output":[{"type":"message","content":[{"type":"output_text","text":"17"}]}]}`, `{"modelVersion":"a","candidates":[{"content":{"parts":[{"text":"99","thought":true},{"text":"17"}]}}]}`} {
		text, name, err := fingerprintReply([]byte(reply))
		require.NoError(t, err)
		assert.Equal(t, "17", text)
		assert.Equal(t, "a", name)
	}
	_, _, err := fingerprintReply([]byte(`{"choices":[]}`))
	require.Error(t, err)
}

func TestFingerprintGroupValidation(t *testing.T) {
	previous := common.OptionMap
	common.OptionMap = map[string]string{fingerprintOption: `[{"group":"default","model":"alpha","reference":""}]`}
	t.Cleanup(func() { common.OptionMap = previous })
	for _, body := range []string{`{"group":"missing"}`, `{"group":""}`, `{"group":"  "}`, `{"group":42}`, `{"group":"default"} {}`, `[]`, `{"group":"` + strings.Repeat("x", 1024) + `"}`} {
		t.Run(body[:min(len(body), 40)], func(t *testing.T) {
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request = httptest.NewRequest(http.MethodPost, "/api/model-fingerprint/run", strings.NewReader(body))
			RunModelFingerprint(c)
			var response struct {
				Success bool `json:"success"`
			}
			require.NoError(t, common.Unmarshal(w.Body.Bytes(), &response))
			assert.False(t, response.Success)
			if body == `{"group":"missing"}` {
				assert.Contains(t, w.Body.String(), "No saved fingerprint targets in this group")
			} else {
				assert.Contains(t, w.Body.String(), "Invalid fingerprint group")
			}
			assert.NotContains(t, common.OptionMap, fingerprintSnapshotOption)
		})
	}
}

func TestFingerprintGroupSnapshotPreservesOtherGroups(t *testing.T) {
	oldTime := time.Date(2026, 11, 23, 1, 0, 0, 0, time.UTC)
	newTime := oldTime.Add(time.Hour)
	selected := fingerprintTarget{Group: "selected", Model: "alpha"}
	other := fingerprintTarget{Group: "other", Model: "alpha"}
	removed := fingerprintTarget{Group: "removed", Model: "alpha"}
	changedReference := fingerprintTarget{Group: "other", Model: "beta", Reference: "new"}
	previous := &fingerprintSnapshot{ReferenceBuiltAt: "old-bank", CheckedAt: oldTime, Results: []fingerprintResult{
		{fingerprintTarget: selected, Verdict: "match"},
		{fingerprintTarget: other, Verdict: "mismatch"},
		{fingerprintTarget: removed, Verdict: "match"},
		{fingerprintTarget: fingerprintTarget{Group: "other", Model: "beta", Reference: "old"}, Verdict: "match"},
	}}
	current := fingerprintSnapshot{ReferenceBuiltAt: "new-bank", CheckedAt: newTime, Results: []fingerprintResult{
		{fingerprintTarget: selected, Verdict: "unavailable", CheckedAt: &newTime, ReferenceBuiltAt: "new-bank"},
	}}
	merged := mergeFingerprintSnapshot([]fingerprintTarget{selected, other, changedReference}, previous, current)
	require.Len(t, merged.Results, 2)
	assert.Equal(t, "unavailable", merged.Results[0].Verdict)
	assert.Equal(t, newTime, *merged.Results[0].CheckedAt)
	assert.Equal(t, "mismatch", merged.Results[1].Verdict)
	assert.Equal(t, oldTime, *merged.Results[1].CheckedAt)
	assert.Equal(t, "old-bank", merged.Results[1].ReferenceBuiltAt)
	assert.Nil(t, previous.Results[1].CheckedAt, "merging must not modify the previous snapshot")
}

func TestFingerprintScheduledChecks(t *testing.T) {
	var db *gorm.DB
	versionQuery := "SELECT sqlite_version()"
	if dsn := os.Getenv("TEST_FINGERPRINT_DB_DSN"); dsn != "" {
		previousDB, previousLogDB := model.DB, model.LOG_DB
		previousMaster, previousRedis := common.IsMasterNode, common.RedisEnabled
		previousMainType, previousLogType := common.MainDatabaseType(), common.LogDatabaseType()
		t.Cleanup(func() {
			model.DB, model.LOG_DB = previousDB, previousLogDB
			common.IsMasterNode, common.RedisEnabled = previousMaster, previousRedis
			common.SetDatabaseTypes(previousMainType, previousLogType)
		})
		t.Setenv("SQL_DSN", dsn)
		t.Setenv("LOG_SQL_DSN", "")
		common.IsMasterNode, common.RedisEnabled = false, false
		require.NoError(t, model.InitDB())
		db, model.LOG_DB = model.DB, model.DB
		sqlDB, err := db.DB()
		require.NoError(t, err)
		t.Cleanup(func() { _ = sqlDB.Close() })
		tables := []any{&model.User{}, &model.Token{}, &model.Channel{}, &model.Ability{}, &model.Option{}, &model.SystemTask{}, &model.SystemTaskLock{}, &model.Log{}, &model.StoreChannel{}}
		for _, table := range tables {
			require.False(t, db.Migrator().HasTable(table), "use a disposable empty database for the fingerprint test")
		}
		t.Cleanup(func() { require.NoError(t, db.Migrator().DropTable(tables...)) })
		require.NoError(t, db.AutoMigrate(tables...))
		versionQuery = "SELECT version()"
	} else {
		db = setupModelListControllerTestDB(t)
		require.NoError(t, db.AutoMigrate(&model.Token{}, &model.Option{}, &model.SystemTask{}, &model.SystemTaskLock{}, &model.Log{}, &model.StoreChannel{}))
		// Keep the SQLite in-memory fixture deterministic across concurrent probes.
		sqlDB, err := db.DB()
		require.NoError(t, err)
		sqlDB.SetMaxOpenConns(1)
	}
	var version string
	require.NoError(t, db.Raw(versionQuery).Scan(&version).Error)
	t.Log(common.MainDatabaseType(), version)
	original := common.OptionMap
	common.OptionMap = map[string]string{}
	t.Cleanup(func() { common.OptionMap = original })
	var calls atomic.Int32
	var authorizationValid atomic.Bool
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		key, suffix, ok := strings.Cut(strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer sk-"), "-")
		var token model.Token
		if ok && suffix != "" && db.Where(&model.Token{Key: key}).Take(&token).Error == nil &&
			token.Group == "default" && token.ModelLimitsEnabled && token.ModelLimits == "gpt-4o" &&
			token.ExpiredTime > time.Now().Unix() && !token.UnlimitedQuota {
			authorizationValid.Store(true)
		}
		if calls.Add(1) == 1 {
			w.WriteHeader(http.StatusBadGateway)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"id":"demo","model":"controlled-upstream","choices":[{"index":0,"message":{"role":"assistant","content":"17 24"},"finish_reason":"stop"}],"usage":{"prompt_tokens":22,"completion_tokens":2,"total_tokens":24}}`)
	}))
	defer upstream.Close()
	fixtureURL, err := url.Parse(upstream.URL)
	require.NoError(t, err)
	previousGateway, previousServerAddress := fingerprintGatewayClient, system_setting.ServerAddress
	fingerprintGatewayClient = &http.Client{Transport: fingerprintTestTransport(func(req *http.Request) (*http.Response, error) {
		assert.Equal(t, "http://87.83.206.165", req.URL.Scheme+"://"+req.URL.Host)
		assert.Equal(t, "87.83.206.165", req.Host)
		assert.Equal(t, "/v1/chat/completions", req.URL.Path)
		copy := req.Clone(req.Context())
		copy.URL.Scheme, copy.URL.Host = fixtureURL.Scheme, fixtureURL.Host
		return http.DefaultTransport.RoundTrip(copy)
	})}
	system_setting.ServerAddress = "https://api.mcapple.top"
	t.Cleanup(func() {
		fingerprintGatewayClient, system_setting.ServerAddress = previousGateway, previousServerAddress
	})
	target := fingerprintTarget{Group: "default", Model: "gpt-4o"}
	channel := model.Channel{Type: constant.ChannelTypeOpenAI, Key: "fixture", Name: "private channel", Status: common.ChannelStatusEnabled, Group: target.Group, Models: target.Model, BaseURL: &upstream.URL}
	require.NoError(t, db.Create(&channel).Error)
	require.NoError(t, db.Create(&model.Ability{Group: target.Group, Model: target.Model, ChannelId: channel.Id, Enabled: true}).Error)
	require.NoError(t, db.Create(&model.User{Username: "fingerprint-admin", Role: common.RoleRootUser, Status: common.UserStatusEnabled, Group: "default"}).Error)
	h := modelFingerprintHandler{}
	require.False(t, h.Enabled())
	assert.Equal(t, 10*time.Minute, h.Interval())
	save := func(targets []fingerprintTarget, enabled bool, minutes int) bool {
		t.Helper()
		body, err := common.Marshal(gin.H{"targets": targets, "schedule": fingerprintSchedule{Enabled: enabled, Minutes: minutes}})
		require.NoError(t, err)
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodPut, "/api/model-fingerprint", strings.NewReader(string(body)))
		UpdateModelFingerprintConfig(c)
		var response struct {
			Success bool `json:"success"`
		}
		require.NoError(t, common.Unmarshal(w.Body.Bytes(), &response))
		return response.Success
	}
	require.True(t, save([]fingerprintTarget{target}, true, 10))
	require.True(t, h.Enabled())
	assert.Equal(t, 10*time.Minute, h.Interval())
	require.False(t, save([]fingerprintTarget{target}, true, 0))
	require.False(t, save([]fingerprintTarget{}, true, 10))
	require.False(t, save([]fingerprintTarget{target}, true, 1441))
	require.False(t, save([]fingerprintTarget{target, target}, true, 10))
	require.True(t, save([]fingerprintTarget{target}, true, 15))
	assert.Equal(t, 15*time.Minute, h.Interval())
	var persisted model.Option
	require.NoError(t, db.Where(&model.Option{Key: fingerprintScheduleOption}).Take(&persisted).Error)
	assert.JSONEq(t, `{"enabled":true,"minutes":15}`, persisted.Value)
	// Confirm startup can recover a saved schedule with the in-memory copy cleared.
	delete(common.OptionMap, fingerprintScheduleOption)
	model.InitOptionMap()
	require.True(t, h.Enabled())
	assert.Equal(t, 15*time.Minute, h.Interval())
	runTask := func() *model.SystemTask {
		t.Helper()
		task, err := model.CreateSystemTask(h.Type(), nil, nil)
		require.NoError(t, err)
		claimed, ok, err := model.ClaimSystemTask(task.ID, h.Type(), "fp-test", time.Now().Add(time.Minute).Unix())
		require.NoError(t, err)
		require.True(t, ok)
		h.Run(context.Background(), claimed, "fp-test")
		var end model.SystemTask
		require.NoError(t, db.First(&end, task.ID).Error)
		return &end
	}
	require.True(t, save([]fingerprintTarget{target}, false, 10))
	require.Equal(t, model.SystemTaskStatusSucceeded, runTask().Status)
	assert.Zero(t, calls.Load())
	// Optional integration fixture is the published lmfpd-data package, served exclusively on loopback.
	bank := os.Getenv("TEST_FINGERPRINT_BANK")
	if bank == "" {
		return
	}
	bankServer := httptest.NewServer(http.FileServer(http.Dir(bank)))
	defer bankServer.Close()
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	_, err = lmfpd.Load(context.Background(), lmfpd.LoadOptions{Sources: []string{bankServer.URL + "/"}})
	require.NoError(t, err)
	require.True(t, save([]fingerprintTarget{target}, true, 10))
	end := runTask()
	require.Equal(t, model.SystemTaskStatusSucceeded, end.Status, end.Error)
	require.Equal(t, int32(4), calls.Load(), "only the failed sample is retried")
	assert.True(t, authorizationValid.Load())
	var activeTokens int64
	require.NoError(t, db.Model(&model.Token{}).Count(&activeTokens).Error)
	assert.Zero(t, activeTokens, "the gateway token is revoked when the run finishes")
	visible, err := lastFingerprintRun([]fingerprintTarget{target}, map[string]string{"default": "ok"}, false)
	require.NoError(t, err)
	require.NotNil(t, visible)
	require.Len(t, visible.Results, 1)
	assert.Equal(t, "inconclusive", visible.Results[0].Verdict)
	assert.Empty(t, visible.Results[0].ChannelName)
	assert.Empty(t, visible.Results[0].ReportedModel)
	hidden, err := lastFingerprintRun([]fingerprintTarget{target}, nil, false)
	require.NoError(t, err)
	assert.Nil(t, hidden)
	admin, err := lastFingerprintRun([]fingerprintTarget{target}, nil, true)
	require.NoError(t, err)
	require.NotNil(t, admin)
	assert.Equal(t, "controlled-upstream", admin.Results[0].ReportedModel)

	otherTarget := fingerprintTarget{Group: "other", Model: target.Model}
	otherChannel := model.Channel{Type: constant.ChannelTypeOpenAI, Key: "fixture-other", Name: "other channel", Status: common.ChannelStatusEnabled, Group: otherTarget.Group, Models: otherTarget.Model, BaseURL: &upstream.URL}
	require.NoError(t, db.Create(&otherChannel).Error)
	require.NoError(t, db.Create(&model.Ability{Group: otherTarget.Group, Model: otherTarget.Model, ChannelId: otherChannel.Id, Enabled: true}).Error)
	require.True(t, save([]fingerprintTarget{target, otherTarget}, true, 10))
	oldTime := time.Date(2026, 11, 23, 1, 0, 0, 0, time.UTC)
	admin.CheckedAt = oldTime
	admin.Results = append(admin.Results, fingerprintResult{fingerprintTarget: otherTarget, Verdict: "unavailable"})
	prior, err := common.Marshal(admin)
	require.NoError(t, err)
	require.NoError(t, model.UpdateOptionsBulk(map[string]string{fingerprintSnapshotOption: string(prior)}))
	beforeCalls := calls.Load()
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Set("id", 1)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/model-fingerprint/run", strings.NewReader(`{"group":"default"}`))
	RunModelFingerprint(c)
	var response struct {
		Success bool                `json:"success"`
		Data    fingerprintSnapshot `json:"data"`
	}
	require.NoError(t, common.Unmarshal(w.Body.Bytes(), &response))
	require.True(t, response.Success, w.Body.String())
	groupRun := response.Data
	require.Len(t, groupRun.Results, 2)
	assert.Equal(t, int32(3), calls.Load()-beforeCalls, "only the selected group's model is probed")
	assert.Equal(t, target, groupRun.Results[0].fingerprintTarget)
	assert.Equal(t, otherTarget, groupRun.Results[1].fingerprintTarget)
	assert.Equal(t, oldTime, *groupRun.Results[1].CheckedAt)
	assert.Equal(t, "unavailable", groupRun.Results[1].Verdict)
	var persistedSnapshot model.Option
	require.NoError(t, db.Where(&model.Option{Key: fingerprintSnapshotOption}).Take(&persistedSnapshot).Error)
	var persistedRun fingerprintSnapshot
	require.NoError(t, common.UnmarshalJsonStr(persistedSnapshot.Value, &persistedRun))
	assert.Equal(t, groupRun.Results, persistedRun.Results)
	assert.Equal(t, fingerprintSchedule{Enabled: true, Minutes: 10}, configuredFingerprintSchedule())
	require.NoError(t, db.Model(&model.Token{}).Count(&activeTokens).Error)
	assert.Zero(t, activeTokens)
}

type fingerprintTestTransport func(*http.Request) (*http.Response, error)

func (send fingerprintTestTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	return send(req)
}
