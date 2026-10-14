package controller

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	relaydto "github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/service"

	"github.com/gin-gonic/gin"
)

const (
	offlineImageDefaultMaxQueued = 10
	offlineImageMaxPromptLength  = 4000
	// offlineImageSizePattern keeps the provider scalars bounded; the relay
	// validates them against the selected model's own settings.
	offlineImageScalarPattern = `^[A-Za-z0-9x:*._-]{1,32}$`
)

var offlineImageScalarRegexp = regexp.MustCompile(offlineImageScalarPattern)

func offlineImageMaxQueued() int {
	if queued := common.GetEnvOrDefault("OFFLINE_IMAGE_MAX_QUEUED", offlineImageDefaultMaxQueued); queued > 0 {
		return queued
	}
	return offlineImageDefaultMaxQueued
}

type offlineImageCreateInput struct {
	group         string
	model         string
	mode          string
	prompt        string
	size          string
	quality       string
	count         int
	reference     []byte
	referenceMime string
}

// CreateOfflineImageTask queues an image job for the logged-in user. The row is
// created PENDING and only becomes claimable once its reference image is on
// disk, so the runner never picks up a half-written task.
func CreateOfflineImageTask(c *gin.Context) {
	userId := c.GetInt("id")
	input, err := parseOfflineImageCreateInput(c)
	if err != nil {
		common.ApiErrorMsg(c, err.Error())
		return
	}
	userCache, err := model.GetUserCache(userId)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if err := validateOfflineImageCreateInput(input, userCache); err != nil {
		common.ApiErrorMsg(c, err.Error())
		return
	}
	queued, err := model.CountActiveOfflineImageTasks(userId)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if queued >= int64(offlineImageMaxQueued()) {
		common.ApiErrorMsg(c, fmt.Sprintf("最多同时排队 %d 个任务，请等待已有任务完成", offlineImageMaxQueued()))
		return
	}

	task := &model.OfflineImageTask{
		UserId:      userId,
		Group:       input.group,
		Model:       input.model,
		Mode:        input.mode,
		Prompt:      input.prompt,
		Status:      model.OfflineImageStatusPending,
		SubmittedAt: common.GetTimestamp(),
		CreatedAt:   common.GetTimestamp(),
		UpdatedAt:   common.GetTimestamp(),
	}
	task.SetImageParams(model.OfflineImageParams{Size: input.size, Quality: input.quality, N: input.count})
	if err := model.CreateOfflineImageTask(task); err != nil {
		common.ApiError(c, err)
		return
	}

	if input.mode == model.OfflineImageModeEdit {
		reference, storeErr := service.StoreOfflineImageFile(userId, task.ID, "ref", 0, input.referenceMime, input.reference)
		if storeErr != nil {
			_ = model.DeleteOfflineImageTask(task.ID)
			common.ApiErrorMsg(c, "保存参考图失败："+storeErr.Error())
			return
		}
		if err := model.SetOfflineImageTaskReference(task.ID, reference.Name); err != nil {
			_ = model.DeleteOfflineImageTask(task.ID)
			common.ApiError(c, err)
			return
		}
		task.Reference = reference.Name
	}

	task.Status = model.OfflineImageStatusQueued
	if err := model.MarkOfflineImageTaskQueued(task.ID); err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, offlineImageTaskToDto(task))
}

func parseOfflineImageCreateInput(c *gin.Context) (offlineImageCreateInput, error) {
	input := offlineImageCreateInput{}
	if strings.Contains(c.Request.Header.Get("Content-Type"), "multipart/form-data") {
		form, err := common.ParseMultipartFormReusable(c)
		if err != nil {
			return input, errors.New("解析表单失败：" + err.Error())
		}
		input.group = strings.TrimSpace(valueOrEmpty(form.Value["group"]))
		input.model = strings.TrimSpace(valueOrEmpty(form.Value["model"]))
		input.prompt = strings.TrimSpace(valueOrEmpty(form.Value["prompt"]))
		input.size = strings.TrimSpace(valueOrEmpty(form.Value["size"]))
		input.quality = strings.TrimSpace(valueOrEmpty(form.Value["quality"]))
		input.mode = strings.TrimSpace(valueOrEmpty(form.Value["mode"]))
		if raw := strings.TrimSpace(valueOrEmpty(form.Value["n"])); raw != "" {
			count, convErr := strconv.Atoi(raw)
			if convErr != nil {
				return input, errors.New("生成数量必须是整数")
			}
			input.count = count
		}
		files := form.File["image"]
		if len(files) > 0 {
			header := files[0]
			if header.Size > service.MaxOfflineReferenceBytes {
				return input, fmt.Errorf("参考图不能超过 %d MB", service.MaxOfflineReferenceBytes>>20)
			}
			file, openErr := header.Open()
			if openErr != nil {
				return input, errors.New("读取参考图失败")
			}
			defer file.Close()
			data, readErr := readLimited(file, service.MaxOfflineReferenceBytes)
			if readErr != nil {
				return input, readErr
			}
			input.reference = data
			input.referenceMime = header.Header.Get("Content-Type")
		}
		if input.mode == "" {
			if len(input.reference) > 0 {
				input.mode = model.OfflineImageModeEdit
			} else {
				input.mode = model.OfflineImageModeGenerate
			}
		}
		return input, nil
	}

	var body struct {
		Group   string `json:"group"`
		Model   string `json:"model"`
		Mode    string `json:"mode"`
		Prompt  string `json:"prompt"`
		Size    string `json:"size"`
		Quality string `json:"quality"`
		N       int    `json:"n"`
	}
	if err := common.UnmarshalBodyReusable(c, &body); err != nil {
		return input, errors.New("参数错误")
	}
	input.group = strings.TrimSpace(body.Group)
	input.model = strings.TrimSpace(body.Model)
	input.prompt = strings.TrimSpace(body.Prompt)
	input.size = strings.TrimSpace(body.Size)
	input.quality = strings.TrimSpace(body.Quality)
	input.count = body.N
	input.mode = model.OfflineImageModeGenerate
	if strings.TrimSpace(body.Mode) == model.OfflineImageModeEdit {
		input.mode = model.OfflineImageModeEdit
	}
	return input, nil
}

func valueOrEmpty(values []string) string {
	if len(values) == 0 {
		return ""
	}
	return values[0]
}

// readLimited reads at most limit bytes and rejects anything larger instead of
// silently truncating an upload.
func readLimited(reader io.Reader, limit int64) ([]byte, error) {
	data, err := io.ReadAll(io.LimitReader(reader, limit+1))
	if err != nil {
		return nil, errors.New("读取文件失败")
	}
	if int64(len(data)) > limit {
		return nil, fmt.Errorf("文件不能超过 %d MB", limit>>20)
	}
	return data, nil
}

func validateOfflineImageCreateInput(input offlineImageCreateInput, userCache *model.UserBase) error {
	if input.mode != model.OfflineImageModeGenerate && input.mode != model.OfflineImageModeEdit {
		return errors.New("任务类型无效")
	}
	if input.mode == model.OfflineImageModeEdit && len(input.reference) == 0 {
		return errors.New("图生图任务需要上传参考图")
	}
	if input.mode == model.OfflineImageModeGenerate && len(input.reference) > 0 {
		return errors.New("文生图任务不接受参考图")
	}
	if input.prompt == "" {
		return errors.New("提示词不能为空")
	}
	if len([]rune(input.prompt)) > offlineImageMaxPromptLength {
		return fmt.Errorf("提示词不能超过 %d 字", offlineImageMaxPromptLength)
	}
	if input.model == "" {
		return errors.New("请选择模型")
	}
	if len(input.model) > 191 {
		return errors.New("模型名称过长")
	}
	if input.group == "" {
		return errors.New("请选择分组")
	}
	if input.count < 0 || input.count > relaydto.MaxImageN {
		return fmt.Errorf("生成数量必须在 1 到 %d 之间", relaydto.MaxImageN)
	}
	if input.mode == model.OfflineImageModeEdit && input.count > 1 {
		return errors.New("图生图任务一次只能生成一张")
	}
	for _, scalar := range []string{input.size, input.quality} {
		if scalar != "" && !offlineImageScalarRegexp.MatchString(scalar) {
			return errors.New("尺寸或品质参数无效")
		}
	}
	if !service.GroupInUserUsableGroups(userCache.Group, input.group) {
		return errors.New("没有该分组的访问权限")
	}
	if blocked := service.OverseasDeclarationBlockedGroup(userCache.Group, input.group, userCache.OverseasConfirmedAt > 0); blocked != "" {
		return fmt.Errorf("分组 %s 为海外模型，请先在「个人设置」里完成外国人声明", blocked)
	}
	models := service.GetGroupsEnabledModels([]string{input.group})
	if !slices.Contains(models, input.model) {
		return fmt.Errorf("分组 %s 下没有模型 %s", input.group, input.model)
	}
	if len(input.reference) > 0 {
		mime := http.DetectContentType(input.reference)
		if !strings.HasPrefix(mime, "image/") {
			return errors.New("参考图必须是图片文件")
		}
	}
	return nil
}

func ListOfflineImageTasks(c *gin.Context) {
	pageInfo := common.GetPageQuery(c)
	userId := c.GetInt("id")
	listAll := c.Query("all") == "1" && c.GetInt("role") >= common.RoleAdminUser
	queryUserId := userId
	if listAll {
		queryUserId = 0
	}
	tasks, total, err := model.ListOfflineImageTasks(queryUserId, pageInfo.GetStartIdx(), pageInfo.GetPageSize())
	if err != nil {
		common.ApiError(c, err)
		return
	}
	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(offlineImageTasksToDto(tasks, listAll))
	common.ApiSuccess(c, pageInfo)
}

func DeleteOfflineImageTask(c *gin.Context) {
	task, err := offlineImageTaskFromParam(c)
	if err != nil {
		common.ApiErrorMsg(c, err.Error())
		return
	}
	if !offlineImageTaskVisible(c, task) {
		common.ApiErrorMsg(c, "无权操作该任务")
		return
	}
	if task.Status == model.OfflineImageStatusRunning {
		common.ApiErrorMsg(c, "任务正在执行，无法删除")
		return
	}
	if err := model.DeleteOfflineImageTask(task.ID); err != nil {
		common.ApiError(c, err)
		return
	}
	if err := service.RemoveOfflineImageTaskDir(task.UserId, task.ID); err != nil {
		common.SysError(fmt.Sprintf("failed to remove offline image files for task %d: %v", task.ID, err))
	}
	common.ApiSuccess(c, nil)
}

// GetOfflineImageTaskImage serves one stored output image. Access is checked
// against the task owner, so a task id alone never exposes the bytes.
func GetOfflineImageTaskImage(c *gin.Context) {
	task, err := offlineImageTaskFromParam(c)
	if err != nil {
		common.ApiErrorMsg(c, err.Error())
		return
	}
	if !offlineImageTaskVisible(c, task) {
		common.ApiErrorMsg(c, "无权访问该任务")
		return
	}
	index, err := strconv.Atoi(c.Param("index"))
	if err != nil {
		common.ApiErrorMsg(c, "图片序号无效")
		return
	}
	files := task.ResultFiles()
	if index < 0 || index >= len(files) {
		c.Status(http.StatusNotFound)
		return
	}
	writeOfflineImageFile(c, task, files[index].Name, files[index].Mime)
}

func GetOfflineImageTaskReference(c *gin.Context) {
	task, err := offlineImageTaskFromParam(c)
	if err != nil {
		common.ApiErrorMsg(c, err.Error())
		return
	}
	if !offlineImageTaskVisible(c, task) {
		common.ApiErrorMsg(c, "无权访问该任务")
		return
	}
	if task.Reference == "" {
		c.Status(http.StatusNotFound)
		return
	}
	writeOfflineImageFile(c, task, task.Reference, "image/png")
}

func writeOfflineImageFile(c *gin.Context, task *model.OfflineImageTask, name string, mime string) {
	data, err := service.ReadOfflineImageFile(task.UserId, task.ID, name)
	if err != nil {
		c.Status(http.StatusNotFound)
		return
	}
	c.Header("Cache-Control", "private, max-age=300")
	c.Data(http.StatusOK, mime, data)
}

// GetOfflineImageOptions lists what the current user may pick for a group: the
// models enabled for it plus the request bounds the create endpoint enforces.
func GetOfflineImageOptions(c *gin.Context) {
	userId := c.GetInt("id")
	userCache, err := model.GetUserCache(userId)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	group := strings.TrimSpace(c.Query("group"))
	if group == "" {
		group = userCache.Group
	}
	if !service.GroupInUserUsableGroups(userCache.Group, group) {
		common.ApiErrorMsg(c, "没有该分组的访问权限")
		return
	}
	common.ApiSuccess(c, gin.H{
		"group":             group,
		"models":            service.GetGroupsEnabledModels([]string{group}),
		"max_n":             relaydto.MaxImageN,
		"max_queued":        offlineImageMaxQueued(),
		"reference_max_mb":  service.MaxOfflineReferenceBytes >> 20,
		"prompt_max_length": offlineImageMaxPromptLength,
	})
}

func offlineImageTaskFromParam(c *gin.Context) (*model.OfflineImageTask, error) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		return nil, errors.New("任务 ID 无效")
	}
	task, err := model.GetOfflineImageTaskById(id)
	if err != nil {
		return nil, errors.New("任务不存在")
	}
	return task, nil
}

func offlineImageTaskVisible(c *gin.Context, task *model.OfflineImageTask) bool {
	if task.UserId == c.GetInt("id") {
		return true
	}
	return c.GetInt("role") >= common.RoleAdminUser
}

func offlineImageTasksToDto(tasks []*model.OfflineImageTask, fillUser bool) []*dto.OfflineImageTask {
	result := make([]*dto.OfflineImageTask, 0, len(tasks))
	for _, task := range tasks {
		item := offlineImageTaskToDto(task)
		if fillUser {
			if user, err := model.GetUserCache(task.UserId); err == nil {
				item.Username = user.Username
			}
		}
		result = append(result, item)
	}
	return result
}

func offlineImageTaskToDto(task *model.OfflineImageTask) *dto.OfflineImageTask {
	params := task.ImageParams()
	return &dto.OfflineImageTask{
		ID:           task.ID,
		CreatedAt:    task.CreatedAt,
		UserId:       task.UserId,
		Group:        task.Group,
		Model:        task.Model,
		Mode:         task.Mode,
		Prompt:       task.Prompt,
		Size:         params.Size,
		Quality:      params.Quality,
		N:            params.N,
		Status:       string(task.Status),
		FailReason:   task.FailReason,
		ChannelId:    task.ChannelId,
		ImageCount:   len(task.ResultFiles()),
		HasReference: task.Reference != "",
		SubmittedAt:  task.SubmittedAt,
		StartedAt:    task.StartedAt,
		FinishedAt:   task.FinishedAt,
	}
}
