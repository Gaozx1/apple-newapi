package model

import (
	"github.com/QuantumNous/new-api/common"

	"gorm.io/gorm"
)

type OfflineImageStatus string

const (
	// OfflineImageStatusPending is the row state between creating the task and
	// finishing its upload. Only QUEUED tasks are claimable, so the runner never
	// picks up a task whose reference image is still being written.
	OfflineImageStatusPending OfflineImageStatus = "PENDING"
	OfflineImageStatusQueued  OfflineImageStatus = "QUEUED"
	OfflineImageStatusRunning OfflineImageStatus = "RUNNING"
	OfflineImageStatusSuccess OfflineImageStatus = "SUCCESS"
	OfflineImageStatusFailure OfflineImageStatus = "FAILURE"
)

const (
	OfflineImageModeGenerate = "generate"
	OfflineImageModeEdit     = "edit"
)

// OfflineImageFile is one stored output image of an offline task. Name is
// relative to the task directory; Mime is persisted so the content endpoint
// answers with the stored type instead of sniffing the file again.
type OfflineImageFile struct {
	Name string `json:"name"`
	Mime string `json:"mime"`
}

// OfflineImageParams carries the optional provider scalars the creator picked.
// They are validated when the task is created and replayed verbatim on the
// relay request, which re-validates them against the model's own settings.
type OfflineImageParams struct {
	Size    string `json:"size,omitempty"`
	Quality string `json:"quality,omitempty"`
	N       int    `json:"n,omitempty"`
}

// OfflineImageTask is one queued image job that the server runs on its own, so
// the creator can close the browser and collect the result later.
//
// Files live outside the database: ReferenceImage and ResultImages hold names
// relative to the user's task directory (see service.OfflineImageTaskDir) and
// are never served by the list API.
type OfflineImageTask struct {
	ID          int64              `json:"id" gorm:"primary_key;AUTO_INCREMENT"`
	CreatedAt   int64              `json:"created_at" gorm:"index"`
	UpdatedAt   int64              `json:"updated_at"`
	UserId      int                `json:"user_id" gorm:"index"`
	Username    string             `json:"username" gorm:"-"`
	Group       string             `json:"group" gorm:"type:varchar(50);index"`
	Model       string             `json:"model" gorm:"type:varchar(191)"`
	Mode        string             `json:"mode" gorm:"type:varchar(20)"`
	Prompt      string             `json:"prompt" gorm:"type:text"`
	Params      string             `json:"params" gorm:"type:text"`
	Status      OfflineImageStatus `json:"status" gorm:"type:varchar(20);index"`
	FailReason  string             `json:"fail_reason" gorm:"type:text"`
	ChannelId   int                `json:"channel_id"`
	Attempts    int                `json:"attempts"`
	SubmittedAt int64              `json:"submitted_at" gorm:"index"`
	StartedAt   int64              `json:"started_at"`
	FinishedAt  int64              `json:"finished_at"`
	Reference   string             `json:"-" gorm:"type:varchar(191)"`
	Results     string             `json:"-" gorm:"type:text"`
}

// OfflineImageResult decodes the persisted scalar selections.
func (t *OfflineImageTask) ImageParams() OfflineImageParams {
	params := OfflineImageParams{}
	if t.Params == "" {
		return params
	}
	if err := common.UnmarshalJsonStr(t.Params, &params); err != nil {
		common.SysError("failed to decode offline image params: " + err.Error())
		return OfflineImageParams{}
	}
	return params
}

func (t *OfflineImageTask) SetImageParams(params OfflineImageParams) {
	t.Params = ""
	if payload, err := common.Marshal(params); err == nil {
		t.Params = string(payload)
	}
}

// ResultFiles decodes the stored output images, newest request first.
func (t *OfflineImageTask) ResultFiles() []OfflineImageFile {
	files := []OfflineImageFile{}
	if t.Results == "" {
		return files
	}
	if err := common.UnmarshalJsonStr(t.Results, &files); err != nil {
		common.SysError("failed to decode offline image results: " + err.Error())
		return []OfflineImageFile{}
	}
	return files
}

func (t *OfflineImageTask) SetResultFiles(files []OfflineImageFile) {
	t.Results = ""
	if payload, err := common.Marshal(files); err == nil {
		t.Results = string(payload)
	}
}

func CreateOfflineImageTask(task *OfflineImageTask) error {
	return DB.Create(task).Error
}

// SetOfflineImageTaskReference records the stored reference image of an edit task.
func SetOfflineImageTaskReference(id int64, name string) error {
	return DB.Model(&OfflineImageTask{}).
		Where("id = ?", id).
		Updates(map[string]any{"reference": name, "updated_at": common.GetTimestamp()}).Error
}

// MarkOfflineImageTaskQueued publishes a fully stored task to the runner.
func MarkOfflineImageTaskQueued(id int64) error {
	return DB.Model(&OfflineImageTask{}).
		Where("id = ?", id).
		Updates(map[string]any{"status": OfflineImageStatusQueued, "updated_at": common.GetTimestamp()}).Error
}

func GetOfflineImageTaskById(id int64) (*OfflineImageTask, error) {
	task := &OfflineImageTask{}
	if err := DB.Where("id = ?", id).First(task).Error; err != nil {
		return nil, err
	}
	return task, nil
}

// ListOfflineImageTasks returns a page of a user's tasks. A userId of zero lists
// every user's tasks, which the controller only does for administrators.
func ListOfflineImageTasks(userId int, offset int, limit int) ([]*OfflineImageTask, int64, error) {
	query := DB.Model(&OfflineImageTask{})
	if userId > 0 {
		query = query.Where("user_id = ?", userId)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	tasks := make([]*OfflineImageTask, 0)
	if err := query.Order("id desc").Offset(offset).Limit(limit).Find(&tasks).Error; err != nil {
		return nil, 0, err
	}
	return tasks, total, nil
}

// CountActiveOfflineImageTasks counts the user's unfinished tasks, which bounds
// how much work one account can queue.
func CountActiveOfflineImageTasks(userId int) (int64, error) {
	var count int64
	err := DB.Model(&OfflineImageTask{}).
		Where("user_id = ? AND status IN ?", userId, []OfflineImageStatus{OfflineImageStatusQueued, OfflineImageStatusRunning}).
		Count(&count).Error
	return count, err
}

// CountRunningOfflineImageTasks reports how many tasks are executing right now,
// so a claim pass never exceeds the configured concurrency.
func CountRunningOfflineImageTasks() (int64, error) {
	var count int64
	err := DB.Model(&OfflineImageTask{}).
		Where("status = ?", OfflineImageStatusRunning).
		Count(&count).Error
	return count, err
}

// HasUnfinishedOfflineImageTasks reports whether any task still needs runner
// attention — waiting in the queue, being created, or stuck in RUNNING. The
// scheduler folds it into enablement so an idle system schedules no runs, while
// an interrupted run is still revisited and reaped.
func HasUnfinishedOfflineImageTasks() bool {
	tasks := make([]*OfflineImageTask, 0, 1)
	err := DB.Where("status IN ?", []OfflineImageStatus{
		OfflineImageStatusPending,
		OfflineImageStatusQueued,
		OfflineImageStatusRunning,
	}).
		Limit(1).
		Find(&tasks).Error
	return err == nil && len(tasks) > 0
}

// ClaimOfflineImageTasks moves up to limit queued tasks to RUNNING and returns
// the ones this caller won. The compare-and-set on the previous status keeps
// two runners from executing the same task.
func ClaimOfflineImageTasks(limit int) ([]*OfflineImageTask, error) {
	if limit <= 0 {
		return nil, nil
	}
	candidates := make([]*OfflineImageTask, 0, limit)
	if err := DB.Where("status = ?", OfflineImageStatusQueued).
		Order("id asc").
		Limit(limit).
		Find(&candidates).Error; err != nil {
		return nil, err
	}

	now := common.GetTimestamp()
	claimed := make([]*OfflineImageTask, 0, len(candidates))
	for _, candidate := range candidates {
		result := DB.Model(&OfflineImageTask{}).
			Where("id = ? AND status = ?", candidate.ID, OfflineImageStatusQueued).
			Updates(map[string]any{
				"status":     OfflineImageStatusRunning,
				"started_at": now,
				"updated_at": now,
				"attempts":   candidate.Attempts + 1,
			})
		if result.Error != nil {
			return claimed, result.Error
		}
		if result.RowsAffected != 1 {
			continue
		}
		candidate.Status = OfflineImageStatusRunning
		candidate.StartedAt = now
		candidate.UpdatedAt = now
		candidate.Attempts++
		claimed = append(claimed, candidate)
	}
	return claimed, nil
}

// FailStaleOfflineImageTasks fails tasks whose runner disappeared (process
// restart, crash) instead of retrying them: the relay may already have charged
// the request, and a retry would charge the user twice. It also fails rows left
// PENDING by a create request that never finished.
func FailStaleOfflineImageTasks(cutoff int64, reason string) (int64, error) {
	now := common.GetTimestamp()
	stale := func(query *gorm.DB) (int64, error) {
		result := query.Updates(map[string]any{
			"status":      OfflineImageStatusFailure,
			"fail_reason": reason,
			"finished_at": now,
			"updated_at":  now,
		})
		return result.RowsAffected, result.Error
	}

	running, err := stale(DB.Model(&OfflineImageTask{}).
		Where("status = ? AND started_at > 0 AND started_at < ?", OfflineImageStatusRunning, cutoff))
	if err != nil {
		return running, err
	}
	pending, err := stale(DB.Model(&OfflineImageTask{}).
		Where("status = ? AND created_at < ?", OfflineImageStatusPending, cutoff))
	return running + pending, err
}

func FinishOfflineImageTask(id int64, status OfflineImageStatus, failReason string, channelId int, files []OfflineImageFile) error {
	results := ""
	if payload, err := common.Marshal(files); err == nil {
		results = string(payload)
	}
	now := common.GetTimestamp()
	return DB.Model(&OfflineImageTask{}).
		Where("id = ?", id).
		Updates(map[string]any{
			"status":      status,
			"fail_reason": failReason,
			"channel_id":  channelId,
			"results":     results,
			"finished_at": now,
			"updated_at":  now,
		}).Error
}

func DeleteOfflineImageTask(id int64) error {
	return DB.Where("id = ?", id).Delete(&OfflineImageTask{}).Error
}
