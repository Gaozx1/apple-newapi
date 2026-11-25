package controller

import (
	"net/http"
	"strconv"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
)

func GetPublicUserBans(c *gin.Context) {
	const pageSize = 6
	page, err := strconv.ParseUint(c.DefaultQuery("p", "1"), 10, 16)
	if err != nil || page == 0 {
		common.ApiErrorT(c, "Invalid parameters")
		return
	}
	bans, hasMore, err := model.ListPublicUserBans((int(page)-1)*pageSize, pageSize)
	if err != nil {
		common.SysError("failed to load public user bans: " + err.Error())
		c.JSON(http.StatusInternalServerError, gin.H{"success": false})
		return
	}
	common.ApiSuccess(c, gin.H{"items": bans, "has_more": hasMore})
}
