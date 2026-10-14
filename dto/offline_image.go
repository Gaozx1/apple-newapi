package dto

// OfflineImageTask is the API view of a queued image job. Stored file names stay
// server-side; clients address the bytes through the task's content endpoints.
type OfflineImageTask struct {
	ID           int64  `json:"id"`
	CreatedAt    int64  `json:"created_at"`
	UserId       int    `json:"user_id"`
	Username     string `json:"username,omitempty"`
	Group        string `json:"group"`
	Model        string `json:"model"`
	Mode         string `json:"mode"`
	Prompt       string `json:"prompt"`
	Size         string `json:"size,omitempty"`
	Quality      string `json:"quality,omitempty"`
	N            int    `json:"n"`
	Status       string `json:"status"`
	FailReason   string `json:"fail_reason,omitempty"`
	ChannelId    int    `json:"channel_id,omitempty"`
	ImageCount   int    `json:"image_count"`
	HasReference bool   `json:"has_reference"`
	SubmittedAt  int64  `json:"submitted_at"`
	StartedAt    int64  `json:"started_at"`
	FinishedAt   int64  `json:"finished_at"`
}
