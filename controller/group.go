package controller

import (
	"net/http"
	"strconv"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/setting/group_region_setting"
	"github.com/QuantumNous/new-api/setting/ratio_setting"

	"github.com/gin-gonic/gin"
)

func GetGroups(c *gin.Context) {
	groupNames := make([]string, 0)
	for groupName := range ratio_setting.GetGroupRatioCopy() {
		groupNames = append(groupNames, groupName)
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    groupNames,
	})
}

// groupNamesByCode indexes registered group definitions by their stable code so
// a caller can attach display names without one query per group.
func groupNamesByCode() map[string]model.GroupNames {
	groups, err := model.GetGroups()
	if err != nil {
		common.SysLog(common.LogText("failed to load group definitions: %s", err.Error()))
		return map[string]model.GroupNames{}
	}
	indexed := make(map[string]model.GroupNames, len(groups))
	for _, group := range groups {
		indexed[group.Code] = group.Names
	}
	return indexed
}

// groupDisplayNamesFor resolves the multilingual display names of a group, so the
// web console can render the name in the language the user picked instead of
// relying on the single description string.
func groupDisplayNamesFor(code string, names model.GroupNames) map[string]string {
	if len(names) > 0 {
		return map[string]string(names)
	}
	if description := setting.GetUsableGroupDescription(code); description != "" {
		return map[string]string{model.GroupLanguageEn: description}
	}
	return map[string]string{model.GroupLanguageEn: code}
}

func GetUserGroups(c *gin.Context) {
	usableGroups := make(map[string]map[string]any)
	userGroup := ""
	rechargeAmount := model.GetContextUserRechargeAmount(c)
	userId := c.GetInt("id")
	userGroup, _ = model.GetUserGroup(userId, false)
	userUsableGroups := model.FilterOwnedOrClaimedUserGroups(service.GetUserUsableGroups(userGroup), userId)
	displayNames := groupNamesByCode()
	for groupName := range ratio_setting.GetGroupRatioCopy() {
		// UserUsableGroups contains the groups that the user can use
		if desc, ok := userUsableGroups[groupName]; ok {
			usableGroups[groupName] = map[string]any{
				"ratio": service.GetUserGroupRatio(userGroup, groupName, rechargeAmount),
				"desc":  desc,
				"names": groupDisplayNamesFor(groupName, displayNames[groupName]),
				// 海外分组需要用户先确认自己是外国人（见 setting/group_region_setting）
				"region": group_region_setting.RegionOf(groupName),
			}
		}
	}
	if _, ok := userUsableGroups["auto"]; ok {
		usableGroups["auto"] = map[string]any{
			"ratio": "auto",
			"desc":  setting.GetUsableGroupDescription("auto"),
		}
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    usableGroups,
	})
}

// GetGroupDefinitions lists every group with its stable code, multilingual
// display names and metadata. The admin console manages groups through this.
func GetGroupDefinitions(c *gin.Context) {
	groups, err := model.GetGroups()
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data": gin.H{
			"groups":    groups,
			"languages": model.GroupNameLanguages,
		},
	})
}

// CreateGroupDefinition registers a new group. Its code is the identifier that
// channels, tokens and users reference, so it is validated strictly.
func CreateGroupDefinition(c *gin.Context) {
	var request model.GroupRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
		return
	}
	group, err := model.CreateGroup(request)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "", "data": group})
}

// UpdateGroupDefinition edits a group's display names, ratio, region, enabled
// flag and ordering. The code is immutable here: changing it would invalidate
// every channel and token that already references it. Use the rename endpoint
// for that, which migrates the references in one transaction.
func UpdateGroupDefinition(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "invalid group id"})
		return
	}
	var request model.GroupRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
		return
	}
	group, err := model.UpdateGroup(id, request)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "", "data": group})
}

// RenameGroupDefinition changes a group's stable code and rewrites every
// reference in one transaction, so nothing is left pointing at the old code.
func RenameGroupDefinition(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "invalid group id"})
		return
	}
	var request struct {
		Code string `json:"code"`
	}
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
		return
	}
	result, err := model.RenameGroupCode(id, request.Code)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "", "data": result})
}

// DeleteGroupDefinition removes a group that nothing references any more. The
// default group and any group still in use are rejected.
func DeleteGroupDefinition(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "invalid group id"})
		return
	}
	if err := model.DeleteGroup(id); err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": ""})
}
