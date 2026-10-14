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
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/i18n"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/middleware"
	"github.com/QuantumNous/new-api/model"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/QuantumNous/new-api/service"

	"github.com/gin-gonic/gin"
)

const (
	offlineImageDefaultConcurrency = 2
	// offlineImageTaskTimeout bounds one generation so a stuck upstream cannot
	// hold the runner open forever.
	offlineImageTaskTimeout = 15 * time.Minute
	// offlineImageStaleMinutes is how long a RUNNING row may stay untouched
	// before the runner declares its worker gone.
	offlineImageStaleMinutes = 30
	// offlineImageMaxImageBytes bounds one stored image.
	offlineImageMaxImageBytes = 32 << 20

	offlineImageInterruptedReason = "任务执行被中断（服务重启或超时），额度以用量日志为准"
)

// offlineImageHandler runs the queued offline image tasks on the master
// instance. The system task framework supplies the cross-instance lease, so two
// masters never execute the same queue pass.
type offlineImageHandler struct{}

func (offlineImageHandler) Type() string { return model.SystemTaskTypeOfflineImage }

// Enabled folds the queue check in, so an idle queue schedules no runs at all.
func (offlineImageHandler) Enabled() bool { return model.HasUnfinishedOfflineImageTasks() }

func (offlineImageHandler) Interval() time.Duration { return 5 * time.Second }

func (offlineImageHandler) NewPayload() any { return nil }

func (offlineImageHandler) Run(ctx context.Context, task *model.SystemTask, runnerID string) {
	summary := runOfflineImageTasksOnce(ctx)
	finishSystemTaskHandler(task, runnerID, model.SystemTaskStatusSucceeded, summary, nil)
}

type offlineImageRunSummary struct {
	Reaped  int64 `json:"reaped"`
	Claimed int   `json:"claimed"`
	Success int   `json:"success"`
	Failed  int   `json:"failed"`
}

func offlineImageConcurrency() int {
	concurrency := common.GetEnvOrDefault("OFFLINE_IMAGE_MAX_CONCURRENCY", offlineImageDefaultConcurrency)
	if concurrency <= 0 {
		return offlineImageDefaultConcurrency
	}
	return concurrency
}

// runOfflineImageTasksOnce claims one batch and runs it to completion. Callers
// hold the system task lease, so passes never overlap.
func runOfflineImageTasksOnce(ctx context.Context) offlineImageRunSummary {
	summary := offlineImageRunSummary{}

	staleCutoff := common.GetTimestamp() - offlineImageStaleMinutes*60
	reaped, err := model.FailStaleOfflineImageTasks(staleCutoff, offlineImageInterruptedReason)
	if err != nil {
		logger.LogWarn(ctx, fmt.Sprintf("offline image: failed to reap stale tasks: %v", err))
	} else if reaped > 0 {
		logger.LogWarn(ctx, fmt.Sprintf("offline image: marked %d interrupted task(s) as failed", reaped))
	}
	summary.Reaped = reaped

	limit := offlineImageConcurrency()
	running, err := model.CountRunningOfflineImageTasks()
	if err != nil {
		logger.LogWarn(ctx, fmt.Sprintf("offline image: failed to count running tasks: %v", err))
	} else if remaining := limit - int(running); remaining < limit {
		limit = max(remaining, 0)
	}

	tasks, err := model.ClaimOfflineImageTasks(limit)
	if err != nil {
		logger.LogWarn(ctx, fmt.Sprintf("offline image: failed to claim queued tasks: %v", err))
		return summary
	}
	summary.Claimed = len(tasks)
	if len(tasks) == 0 {
		return summary
	}

	var wg sync.WaitGroup
	var counter sync.Mutex
	for _, task := range tasks {
		wg.Go(func() {
			// One accounting path for every outcome, including a panic inside the
			// relay: the run summary must never hide a lost task.
			var (
				files      []model.OfflineImageFile
				channelId  int
				failReason string
			)
			func() {
				defer func() {
					if recovered := recover(); recovered != nil {
						common.SysError(fmt.Sprintf("offline image: task %d panicked: %v", task.ID, recovered))
						files = nil
						channelId = 0
						failReason = fmt.Sprintf("任务执行异常：%v", recovered)
					}
				}()
				files, channelId, failReason = executeOfflineImageTask(ctx, task)
			}()

			status := model.OfflineImageStatusSuccess
			if failReason != "" {
				status = model.OfflineImageStatusFailure
			}
			logger.LogInfo(ctx, fmt.Sprintf("offline image: task %d status=%s channel=%d images=%d reason=%s",
				task.ID, status, channelId, len(files), failReason))
			finishOfflineImageTask(task, status, failReason, channelId, files)

			counter.Lock()
			if status == model.OfflineImageStatusSuccess {
				summary.Success++
			} else {
				summary.Failed++
			}
			counter.Unlock()
		})
	}
	wg.Wait()
	return summary
}

func finishOfflineImageTask(task *model.OfflineImageTask, status model.OfflineImageStatus, failReason string, channelId int, files []model.OfflineImageFile) {
	if err := model.FinishOfflineImageTask(task.ID, status, failReason, channelId, files); err != nil {
		common.SysError(fmt.Sprintf("offline image: failed to persist task %d result: %v", task.ID, err))
	}
}

// executeOfflineImageTask replays the task through the ordinary image relay as
// the task owner. Relay format, billing, channel selection and retries are the
// production code paths; only the transport is synthesized.
func executeOfflineImageTask(ctx context.Context, task *model.OfflineImageTask) ([]model.OfflineImageFile, int, string) {
	taskCtx, cancel := context.WithTimeout(ctx, offlineImageTaskTimeout)
	defer cancel()

	request, err := buildOfflineImageRelayRequest(task)
	if err != nil {
		return nil, 0, err.Error()
	}
	c, recorder, err := newOfflineImageRelayContext(taskCtx, task, request)
	if err != nil {
		return nil, 0, "初始化任务上下文失败：" + err.Error()
	}

	retryParam := &service.RetryParam{
		Ctx:         c,
		ModelName:   task.Model,
		TokenGroup:  task.Group,
		RequestPath: request.path,
		Retry:       common.GetPointer(0),
	}
	channel, _, selectErr := service.SelectChannelWithLimits(c, task.Model, retryParam)
	if selectErr != nil {
		// Mirror the relay's own reporting: a localized message id wins over the
		// raw text, and a reasonless rejection still says something useful.
		reason := selectErr.Message
		if selectErr.MessageID != "" {
			reason = i18n.T(c, selectErr.MessageID, selectErr.Params)
		}
		if strings.TrimSpace(reason) == "" {
			reason = fmt.Sprintf("分组 %s 下没有可用于模型 %s 的渠道", task.Group, task.Model)
		}
		return nil, 0, reason
	}
	if setupErr := middleware.SetupContextForSelectedChannel(c, channel, task.Model); setupErr != nil {
		return nil, 0, "渠道初始化失败：" + setupErr.Error()
	}
	channelId := channel.Id

	Relay(c, types.RelayFormatOpenAIImage)

	usedChannels := c.GetStringSlice("use_channel")
	if len(usedChannels) > 0 {
		if last, convErr := strconv.Atoi(usedChannels[len(usedChannels)-1]); convErr == nil {
			channelId = last
		}
	}

	if recorder.Code != http.StatusOK {
		return nil, channelId, offlineImageRelayError(recorder.Body.Bytes(), recorder.Code)
	}

	payloads, err := parseOfflineImagePayloads(recorder.Body.Bytes())
	if err != nil {
		return nil, channelId, "解析上游响应失败：" + err.Error()
	}
	if len(payloads) == 0 {
		return nil, channelId, "上游没有返回图片"
	}

	files := make([]model.OfflineImageFile, 0, len(payloads))
	storeFailures := make([]string, 0)
	for index, payload := range payloads {
		data, fetchErr := readOfflineImagePayload(taskCtx, payload)
		if fetchErr != nil {
			storeFailures = append(storeFailures, fetchErr.Error())
			continue
		}
		file, storeErr := service.StoreOfflineImageFile(task.UserId, task.ID, "out", index, http.DetectContentType(data), data)
		if storeErr != nil {
			storeFailures = append(storeFailures, storeErr.Error())
			continue
		}
		files = append(files, file)
	}
	if len(files) == 0 {
		return nil, channelId, "保存生成结果失败：" + strings.Join(storeFailures, "; ")
	}
	if len(storeFailures) > 0 {
		common.SysError(fmt.Sprintf("offline image: task %d stored %d/%d images, failures: %s",
			task.ID, len(files), len(payloads), strings.Join(storeFailures, "; ")))
	}
	return files, channelId, ""
}

// offlineImageRelayRequest is the synthesized request handed to the relay.
type offlineImageRelayRequest struct {
	path        string
	relayMode   int
	contentType string
	body        []byte
}

// buildOfflineImageRelayRequest turns the stored task back into the provider
// request the relay expects: JSON for text-to-image, multipart for edits.
func buildOfflineImageRelayRequest(task *model.OfflineImageTask) (offlineImageRelayRequest, error) {
	params := task.ImageParams()
	count := params.N
	if count > dto.MaxImageN {
		return offlineImageRelayRequest{}, fmt.Errorf("生成数量必须在 1 到 %d 之间", dto.MaxImageN)
	}
	if count < 0 {
		count = 0
	}

	if task.Mode == model.OfflineImageModeEdit {
		if task.Reference == "" {
			return offlineImageRelayRequest{}, fmt.Errorf("任务缺少参考图")
		}
		reference, err := service.ReadOfflineImageFile(task.UserId, task.ID, task.Reference)
		if err != nil {
			return offlineImageRelayRequest{}, fmt.Errorf("读取参考图失败：%w", err)
		}

		buffer := &bytes.Buffer{}
		writer := multipart.NewWriter(buffer)
		if err := writer.WriteField("model", task.Model); err != nil {
			return offlineImageRelayRequest{}, err
		}
		if err := writer.WriteField("prompt", task.Prompt); err != nil {
			return offlineImageRelayRequest{}, err
		}
		if params.Size != "" {
			_ = writer.WriteField("size", params.Size)
		}
		if params.Quality != "" {
			_ = writer.WriteField("quality", params.Quality)
		}
		// Multipart edits carry no n: the relay re-decodes form values as JSON,
		// where a numeric field arrives as a string. Edit tasks are validated to
		// request a single image instead.
		part, err := writer.CreateFormFile("image", task.Reference)
		if err != nil {
			return offlineImageRelayRequest{}, err
		}
		if _, err := part.Write(reference); err != nil {
			return offlineImageRelayRequest{}, err
		}
		if err := writer.Close(); err != nil {
			return offlineImageRelayRequest{}, err
		}

		return offlineImageRelayRequest{
			path:        "/pg/images/edits",
			relayMode:   relayconstant.RelayModeImagesEdits,
			contentType: writer.FormDataContentType(),
			body:        buffer.Bytes(),
		}, nil
	}

	payload := map[string]any{
		"model":  task.Model,
		"prompt": task.Prompt,
	}
	if params.Size != "" {
		payload["size"] = params.Size
	}
	if params.Quality != "" {
		payload["quality"] = params.Quality
	}
	if count > 0 {
		payload["n"] = count
	}
	body, err := common.Marshal(payload)
	if err != nil {
		return offlineImageRelayRequest{}, err
	}
	return offlineImageRelayRequest{
		path:        "/pg/images/generations",
		relayMode:   relayconstant.RelayModeImagesGenerations,
		contentType: "application/json",
		body:        body,
	}, nil
}

func referenceFileName(stored string) (string, error) {
	if stored == "" {
		return "", fmt.Errorf("缺少参考图")
	}
	return stored, nil
}

// newOfflineImageRelayContext builds the request context for one task. The
// "/pg" path makes the relay treat the caller as the playground, so the request
// is billed to the owner's wallet without involving an API token, exactly like
// the console playground.
func newOfflineImageRelayContext(ctx context.Context, task *model.OfflineImageTask, request offlineImageRelayRequest) (*gin.Context, *httptest.ResponseRecorder, error) {
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequestWithContext(ctx, http.MethodPost, request.path, bytes.NewReader(request.body))
	c.Request.Header.Set("Content-Type", request.contentType)
	c.Request.ContentLength = int64(len(request.body))
	c.Request.RemoteAddr = "127.0.0.1:0"
	c.Set("relay_mode", request.relayMode)
	common.SetContextKey(c, constant.ContextKeyOriginalModel, task.Model)

	userCache, err := model.GetUserCache(task.UserId)
	if err != nil {
		return nil, nil, err
	}
	userCache.WriteContext(c)
	common.SetContextKey(c, constant.ContextKeyUsingGroup, task.Group)
	if err := middleware.SetupContextForToken(c, &model.Token{
		UserId: task.UserId,
		Name:   "offline-image",
		Group:  task.Group,
	}); err != nil {
		return nil, nil, err
	}
	common.SetContextKey(c, common.RequestIdKey, common.NewRequestId())
	return c, recorder, nil
}

// offlineImagePayload is one generated image, either inline or behind a URL.
type offlineImagePayload struct {
	Url     string
	B64Json string
}

// parseOfflineImagePayloads keeps only entries that actually carry an image, so
// payload-less entries are not stored as empty files.
func parseOfflineImagePayloads(body []byte) ([]offlineImagePayload, error) {
	response := dto.ImageResponse{}
	if err := common.Unmarshal(body, &response); err != nil {
		return nil, err
	}
	payloads := make([]offlineImagePayload, 0, len(response.Data))
	for _, item := range response.Data {
		if item.B64Json == "" && item.Url == "" {
			continue
		}
		payloads = append(payloads, offlineImagePayload{Url: item.Url, B64Json: item.B64Json})
	}
	return payloads, nil
}

func readOfflineImagePayload(ctx context.Context, payload offlineImagePayload) ([]byte, error) {
	if payload.B64Json != "" {
		encoded := payload.B64Json
		if strings.HasPrefix(encoded, "data:") {
			if _, after, found := strings.Cut(encoded, ","); found {
				encoded = after
			}
		}
		data, err := base64.StdEncoding.DecodeString(encoded)
		if err != nil {
			return nil, fmt.Errorf("解码图片失败：%w", err)
		}
		return data, nil
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodGet, payload.Url, nil)
	if err != nil {
		return nil, fmt.Errorf("图片地址无效：%w", err)
	}
	response, err := service.GetHttpClient().Do(request)
	if err != nil {
		return nil, fmt.Errorf("下载图片失败：%w", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("下载图片失败：HTTP %d", response.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(response.Body, offlineImageMaxImageBytes+1))
	if err != nil {
		return nil, fmt.Errorf("读取图片失败：%w", err)
	}
	if len(data) > offlineImageMaxImageBytes {
		return nil, fmt.Errorf("图片超过 %d MB", offlineImageMaxImageBytes>>20)
	}
	return data, nil
}

func offlineImageRelayError(body []byte, statusCode int) string {
	message := strings.TrimSpace(string(body))
	if len(message) > 0 {
		var payload struct {
			Error struct {
				Message string `json:"message"`
			} `json:"error"`
		}
		if err := common.Unmarshal(body, &payload); err == nil && payload.Error.Message != "" {
			message = payload.Error.Message
		}
	}
	if message == "" {
		message = http.StatusText(statusCode)
	}
	if len(message) > 500 {
		message = message[:500]
	}
	return message
}
