package controller

import (
	"strconv"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"

	"github.com/gin-gonic/gin"
)

// GetJailbreakRecords lists the anti-jailbreak verdicts for administrators: who
// was judged, on which channel, and what they actually sent.
func GetJailbreakRecords(c *gin.Context) {
	pageInfo := common.GetPageQuery(c)
	userId, _ := strconv.Atoi(c.Query("user_id"))
	channelId, _ := strconv.Atoi(c.Query("channel_id"))
	records, total, err := model.GetJailbreakRecords(
		pageInfo.GetPage(),
		pageInfo.GetPageSize(),
		userId,
		channelId,
		c.Query("keyword"),
	)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(records)
	common.ApiSuccess(c, pageInfo)
}
