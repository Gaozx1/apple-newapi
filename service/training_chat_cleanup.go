package service

import (
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
)

// StartTrainingChatCleanup deletes expired chat data on startup and every minute.
func StartTrainingChatCleanup() {
	if !common.IsMasterNode {
		return
	}
	go func() {
		ticker := time.NewTicker(time.Minute)
		defer ticker.Stop()
		for {
			if err := model.DeleteExpiredTrainingChats(time.Now()); err != nil {
				common.SysError(common.LogText("failed to delete expired training chats"))
			}
			<-ticker.C
		}
	}()
}
