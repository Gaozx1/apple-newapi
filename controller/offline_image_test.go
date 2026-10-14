package controller

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/i18n"
	"github.com/QuantumNous/new-api/model"
	relaydto "github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/ratio_setting"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// 1x1 PNG, small enough to inline but real enough for content sniffing.
const offlineImageTestPngBase64 = "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mP8z8BQDwAEhQGAhKmMIQAAAABJRU5ErkJggg=="

const (
	offlineImageTestModel   = "offline-image-test"
	offlineImageTestGroup   = "default"
	offlineImageTestUserID  = 41
	offlineImageTestOtherID = 42
)

type offlineImageUpstreamCall struct {
	path        string
	contentType string
	body        []byte
}

type offlineImageFixture struct {
	db       *gorm.DB
	t        *testing.T
	upstream *httptest.Server

	mu        sync.Mutex
	recorded  []offlineImageUpstreamCall
	responder http.HandlerFunc
}

func (f *offlineImageFixture) calls() []offlineImageUpstreamCall {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]offlineImageUpstreamCall{}, f.recorded...)
}

// replyWith replaces the upstream answer for the rest of the test.
func (f *offlineImageFixture) replyWith(handler http.HandlerFunc) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.responder = handler
}

func (f *offlineImageFixture) handleUpstream(w http.ResponseWriter, r *http.Request) {
	body, readErr := io.ReadAll(r.Body)
	require.NoError(f.t, readErr)
	f.mu.Lock()
	f.recorded = append(f.recorded, offlineImageUpstreamCall{
		path:        r.URL.Path,
		contentType: r.Header.Get("Content-Type"),
		body:        body,
	})
	responder := f.responder
	f.mu.Unlock()

	if responder != nil {
		responder(w, r)
		return
	}
	// The default answers every image call with two inline PNGs.
	w.Header().Set("Content-Type", "application/json")
	_, _ = io.WriteString(w, fmt.Sprintf(
		`{"created":1,"data":[{"b64_json":"%s"},{"b64_json":"%s"}]}`,
		offlineImageTestPngBase64, offlineImageTestPngBase64,
	))
}

// newOfflineImageFixture wires a real migrated database (the same init path the
// server uses, so dialect column names and AutoMigrate are covered), one OpenAI
// channel backed by a fake upstream, and a temporary image directory.
func newOfflineImageFixture(t *testing.T) *offlineImageFixture {
	t.Helper()
	gin.SetMode(gin.TestMode)
	service.InitHttpClient()
	// The relay reports channel rejections through the i18n catalog, which the
	// server initializes at startup.
	require.NoError(t, i18n.Init())

	previousDB := model.DB
	previousLogDB := model.LOG_DB
	previousMainType, previousLogType := common.MainDatabaseType(), common.LogDatabaseType()
	previousMemoryCache := common.MemoryCacheEnabled
	previousBatchUpdate := common.BatchUpdateEnabled
	previousLogConsume := common.LogConsumeEnabled
	previousRedisEnabled := common.RedisEnabled
	previousRetryTimes := common.RetryTimes
	previousMasterNode := common.IsMasterNode
	previousSQLitePath := common.SQLitePath

	common.SetDatabaseTypes(common.DatabaseTypeSQLite, common.DatabaseTypeSQLite)
	// Migrations run on the master node only, which is also what creates the
	// offline_image_tasks table this feature adds.
	common.IsMasterNode = true
	common.MemoryCacheEnabled = false
	common.BatchUpdateEnabled = false
	common.LogConsumeEnabled = false
	common.RedisEnabled = false
	common.RetryTimes = 0
	common.SQLitePath = filepath.Join(t.TempDir(), "offline-image.db")
	t.Setenv("SQL_DSN", "")
	t.Setenv("LOG_SQL_DSN", "")
	require.NoError(t, model.InitDB())
	require.NoError(t, model.InitLogDB())
	database := model.DB
	require.NotNil(t, database)

	t.Setenv("OFFLINE_IMAGE_DIR", t.TempDir())
	previousModelRatios := ratio_setting.ModelRatio2JSONString()
	require.NoError(t, ratio_setting.UpdateModelRatioByJSONString(fmt.Sprintf(`{"%s":1}`, offlineImageTestModel)))

	fixture := &offlineImageFixture{db: database, t: t}
	fixture.upstream = httptest.NewServer(http.HandlerFunc(fixture.handleUpstream))
	t.Cleanup(func() {
		fixture.upstream.Close()
		model.DB = previousDB
		model.LOG_DB = previousLogDB
		common.SetDatabaseTypes(previousMainType, previousLogType)
		common.MemoryCacheEnabled = previousMemoryCache
		common.BatchUpdateEnabled = previousBatchUpdate
		common.LogConsumeEnabled = previousLogConsume
		common.RedisEnabled = previousRedisEnabled
		common.RetryTimes = previousRetryTimes
		common.IsMasterNode = previousMasterNode
		common.SQLitePath = previousSQLitePath
		require.NoError(t, ratio_setting.UpdateModelRatioByJSONString(previousModelRatios))
		if sqlDB, err := database.DB(); err == nil {
			_ = sqlDB.Close()
		}
	})

	priority := int64(0)
	weight := uint(100)
	baseURL := fixture.upstream.URL
	require.NoError(t, database.Create(&model.Channel{
		Id:       1,
		Type:     constant.ChannelTypeOpenAI,
		Key:      "sk-offline-test",
		BaseURL:  &baseURL,
		Status:   common.ChannelStatusEnabled,
		Name:     "offline-image-channel",
		Weight:   &weight,
		Models:   offlineImageTestModel,
		Group:    offlineImageTestGroup,
		Priority: &priority,
	}).Error)
	require.NoError(t, database.Create(&model.Ability{
		Group:     offlineImageTestGroup,
		Model:     offlineImageTestModel,
		ChannelId: 1,
		Enabled:   true,
		Priority:  &priority,
		Weight:    weight,
	}).Error)

	for _, user := range []struct {
		id   int
		name string
	}{
		{offlineImageTestUserID, "offline-image-owner"},
		{offlineImageTestOtherID, "offline-image-other"},
	} {
		require.NoError(t, database.Create(&model.User{
			Id:       user.id,
			Username: user.name,
			Password: "placeholder",
			Group:    offlineImageTestGroup,
			Quota:    1_000_000,
			Status:   common.UserStatusEnabled,
			AffCode:  user.name,
		}).Error)
	}
	return fixture
}

// offlineImageContext builds the gin context for one management API call.
func offlineImageContext(t *testing.T, method string, target string, body []byte, contentType string, userId int, role int) (*gin.Context, *httptest.ResponseRecorder) {
	t.Helper()
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(method, target, bytes.NewReader(body))
	if contentType != "" {
		c.Request.Header.Set("Content-Type", contentType)
	}
	c.Request.ContentLength = int64(len(body))
	c.Set("id", userId)
	c.Set("role", role)
	return c, recorder
}

func offlineImageAPIFailed(t *testing.T, recorder *httptest.ResponseRecorder) string {
	t.Helper()
	payload := struct {
		Success bool   `json:"success"`
		Message string `json:"message"`
	}{}
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &payload))
	require.False(t, payload.Success, "expected the API call to fail, body: %s", recorder.Body.String())
	return payload.Message
}

func createOfflineImageTaskRecord(t *testing.T, task *model.OfflineImageTask) *model.OfflineImageTask {
	t.Helper()
	if task.Status == "" {
		task.Status = model.OfflineImageStatusQueued
	}
	if task.SubmittedAt == 0 {
		task.SubmittedAt = common.GetTimestamp()
	}
	require.NoError(t, model.CreateOfflineImageTask(task))
	return task
}

func TestOfflineImageTaskRunsAndStoresGeneratedImages(t *testing.T) {
	fixture := newOfflineImageFixture(t)
	decoded, err := base64.StdEncoding.DecodeString(offlineImageTestPngBase64)
	require.NoError(t, err)

	task := &model.OfflineImageTask{
		UserId: offlineImageTestUserID,
		Group:  offlineImageTestGroup,
		Model:  offlineImageTestModel,
		Mode:   model.OfflineImageModeGenerate,
		Prompt: "a lighthouse at dusk",
	}
	task.SetImageParams(model.OfflineImageParams{Size: "1024x1024", Quality: "standard", N: 2})
	createOfflineImageTaskRecord(t, task)

	summary := runOfflineImageTasksOnce(context.Background())
	require.Equal(t, 1, summary.Claimed)
	require.Equal(t, 1, summary.Success)

	stored, err := model.GetOfflineImageTaskById(task.ID)
	require.NoError(t, err)
	require.Equal(t, model.OfflineImageStatusSuccess, stored.Status)
	require.Empty(t, stored.FailReason)
	require.Equal(t, 1, stored.ChannelId)
	files := stored.ResultFiles()
	require.Len(t, files, 2)

	calls := fixture.calls()
	require.Len(t, calls, 1)
	assert.Equal(t, "/v1/images/generations", calls[0].path)
	assert.Contains(t, calls[0].contentType, "application/json")
	assert.JSONEq(t, `{"model":"offline-image-test","prompt":"a lighthouse at dusk","size":"1024x1024","quality":"standard","n":2}`, string(calls[0].body))

	// The owner can read every stored image back through the API.
	for index, file := range files {
		c, recorder := offlineImageContext(t, http.MethodGet, fmt.Sprintf("/api/offline-image/tasks/%d/images/%d", stored.ID, index), nil, "", offlineImageTestUserID, common.RoleCommonUser)
		c.Params = gin.Params{{Key: "id", Value: fmt.Sprint(stored.ID)}, {Key: "index", Value: fmt.Sprint(index)}}
		GetOfflineImageTaskImage(c)
		require.Equal(t, http.StatusOK, recorder.Code)
		assert.Equal(t, file.Mime, recorder.Header().Get("Content-Type"))
		assert.Equal(t, decoded, recorder.Body.Bytes())
	}

	// The bytes are private: another user gets a failure response, not the image.
	c, recorder := offlineImageContext(t, http.MethodGet, fmt.Sprintf("/api/offline-image/tasks/%d/images/0", stored.ID), nil, "", offlineImageTestOtherID, common.RoleCommonUser)
	c.Params = gin.Params{{Key: "id", Value: fmt.Sprint(stored.ID)}, {Key: "index", Value: "0"}}
	GetOfflineImageTaskImage(c)
	offlineImageAPIFailed(t, recorder)

	// A second claim pass finds nothing: the task was already handed out.
	claimed, err := model.ClaimOfflineImageTasks(5)
	require.NoError(t, err)
	assert.Empty(t, claimed)
}

func TestOfflineImageEditTaskUploadsReferenceImage(t *testing.T) {
	fixture := newOfflineImageFixture(t)
	reference, err := base64.StdEncoding.DecodeString(offlineImageTestPngBase64)
	require.NoError(t, err)

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	require.NoError(t, writer.WriteField("group", offlineImageTestGroup))
	require.NoError(t, writer.WriteField("model", offlineImageTestModel))
	require.NoError(t, writer.WriteField("prompt", "turn it into watercolor"))
	require.NoError(t, writer.WriteField("n", "1"))
	part, err := writer.CreateFormFile("image", "reference.png")
	require.NoError(t, err)
	_, err = part.Write(reference)
	require.NoError(t, err)
	require.NoError(t, writer.Close())

	c, recorder := offlineImageContext(t, http.MethodPost, "/api/offline-image/tasks", body.Bytes(), writer.FormDataContentType(), offlineImageTestUserID, common.RoleCommonUser)
	CreateOfflineImageTask(c)
	require.Contains(t, recorder.Body.String(), `"success":true`, "create failed: %s", recorder.Body.String())

	created, _, err := model.ListOfflineImageTasks(offlineImageTestUserID, 0, 1)
	require.NoError(t, err)
	require.Len(t, created, 1)
	require.Equal(t, model.OfflineImageModeEdit, created[0].Mode)
	require.NotEmpty(t, created[0].Reference)
	require.Equal(t, model.OfflineImageStatusQueued, created[0].Status)

	summary := runOfflineImageTasksOnce(context.Background())
	require.Equal(t, 1, summary.Success)

	calls := fixture.calls()
	require.Len(t, calls, 1)
	assert.Equal(t, "/v1/images/edits", calls[0].path)
	require.Contains(t, calls[0].contentType, "multipart/form-data")

	// The reference bytes reach the upstream unchanged, under the image field.
	form, err := multipart.NewReader(bytes.NewReader(calls[0].body), boundaryOf(t, calls[0].contentType)).ReadForm(1 << 20)
	require.NoError(t, err)
	require.Len(t, form.File["image"], 1)
	uploaded, err := form.File["image"][0].Open()
	require.NoError(t, err)
	defer uploaded.Close()
	content, err := io.ReadAll(uploaded)
	require.NoError(t, err)
	assert.Equal(t, reference, content)
	assert.Equal(t, "turn it into watercolor", form.Value["prompt"][0])
}

func TestCreateOfflineImageTaskRejectsInvalidRequests(t *testing.T) {
	newOfflineImageFixture(t)

	cases := []struct {
		name string
		body string
	}{
		{
			name: "generation count above the billing bound",
			body: fmt.Sprintf(`{"group":%q,"model":%q,"prompt":"hi","n":%d}`, offlineImageTestGroup, offlineImageTestModel, relaydto.MaxImageN+1),
		},
		{
			name: "edit without a reference image",
			body: fmt.Sprintf(`{"group":%q,"model":%q,"mode":"edit","prompt":"hi"}`, offlineImageTestGroup, offlineImageTestModel),
		},
		{
			name: "empty prompt",
			body: fmt.Sprintf(`{"group":%q,"model":%q,"prompt":"  "}`, offlineImageTestGroup, offlineImageTestModel),
		},
		{
			name: "group the user cannot use",
			body: fmt.Sprintf(`{"group":"not-my-group","model":%q,"prompt":"hi"}`, offlineImageTestModel),
		},
		{
			name: "model outside the group",
			body: fmt.Sprintf(`{"group":%q,"model":"not-in-group","prompt":"hi"}`, offlineImageTestGroup),
		},
		{
			name: "unbounded size scalar",
			body: fmt.Sprintf(`{"group":%q,"model":%q,"prompt":"hi","size":%q}`, offlineImageTestGroup, offlineImageTestModel, strings.Repeat("9", 64)),
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			c, recorder := offlineImageContext(t, http.MethodPost, "/api/offline-image/tasks", []byte(testCase.body), "application/json", offlineImageTestUserID, common.RoleCommonUser)
			CreateOfflineImageTask(c)
			assert.NotEmpty(t, offlineImageAPIFailed(t, recorder))
		})
	}

	tasks, total, err := model.ListOfflineImageTasks(offlineImageTestUserID, 0, 10)
	require.NoError(t, err)
	assert.Zero(t, total)
	assert.Empty(t, tasks)
}

func TestOfflineImageTaskFailureReportsUpstreamReason(t *testing.T) {
	fixture := newOfflineImageFixture(t)
	fixture.replyWith(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_, _ = io.WriteString(w, `{"error":{"message":"content policy violation","type":"invalid_request_error"}}`)
	})

	task := createOfflineImageTaskRecord(t, &model.OfflineImageTask{
		UserId: offlineImageTestUserID,
		Group:  offlineImageTestGroup,
		Model:  offlineImageTestModel,
		Mode:   model.OfflineImageModeGenerate,
		Prompt: "blocked prompt",
	})

	summary := runOfflineImageTasksOnce(context.Background())
	require.Equal(t, 1, summary.Claimed)
	require.Equal(t, 1, summary.Failed)

	stored, err := model.GetOfflineImageTaskById(task.ID)
	require.NoError(t, err)
	assert.Equal(t, model.OfflineImageStatusFailure, stored.Status)
	assert.Contains(t, stored.FailReason, "content policy violation")
	assert.Empty(t, stored.ResultFiles())
}

func TestOfflineImageTaskWithoutChannelReportsAnActionableReason(t *testing.T) {
	fixture := newOfflineImageFixture(t)

	task := createOfflineImageTaskRecord(t, &model.OfflineImageTask{
		UserId: offlineImageTestUserID,
		Group:  offlineImageTestGroup,
		Model:  "model-without-channel",
		Mode:   model.OfflineImageModeGenerate,
		Prompt: "hi",
	})

	summary := runOfflineImageTasksOnce(context.Background())
	require.Equal(t, 1, summary.Failed)

	stored, err := model.GetOfflineImageTaskById(task.ID)
	require.NoError(t, err)
	assert.Equal(t, model.OfflineImageStatusFailure, stored.Status)
	// A reasonless rejection still has to tell the user which model failed.
	assert.Contains(t, stored.FailReason, "model-without-channel")
	assert.NotEmpty(t, strings.TrimSpace(stored.FailReason))
	assert.Empty(t, fixture.calls())
}

func TestOfflineImageStoreRefusesEscapingNames(t *testing.T) {
	fixture := newOfflineImageFixture(t)
	secret := filepath.Join(filepath.Dir(service.OfflineImageRootDir()), "secret.txt")
	require.NoError(t, os.WriteFile(secret, []byte("do not serve"), 0o644))
	t.Cleanup(func() { _ = os.Remove(secret) })

	task := createOfflineImageTaskRecord(t, &model.OfflineImageTask{
		UserId: offlineImageTestUserID,
		Group:  offlineImageTestGroup,
		Model:  offlineImageTestModel,
		Mode:   model.OfflineImageModeGenerate,
		Prompt: "hi",
	})
	// Simulate a tampered row: the stored name points outside the task directory.
	require.NoError(t, fixture.db.Model(&model.OfflineImageTask{}).Where("id = ?", task.ID).
		Update("reference", "../secret.txt").Error)

	c, recorder := offlineImageContext(t, http.MethodGet, fmt.Sprintf("/api/offline-image/tasks/%d/reference", task.ID), nil, "", offlineImageTestUserID, common.RoleCommonUser)
	c.Params = gin.Params{{Key: "id", Value: fmt.Sprint(task.ID)}}
	GetOfflineImageTaskReference(c)
	c.Writer.WriteHeaderNow()
	assert.Equal(t, http.StatusNotFound, recorder.Code)
	assert.NotContains(t, recorder.Body.String(), "do not serve")
}

func boundaryOf(t *testing.T, contentType string) string {
	t.Helper()
	_, after, found := strings.Cut(contentType, "boundary=")
	require.True(t, found, "content type %q carries no boundary", contentType)
	return after
}
