package service

import (
	"slices"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/setting/group_region_setting"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/QuantumNous/new-api/setting/user_level_setting"
	"github.com/gin-gonic/gin"
)

func GetUserUsableGroups(userGroup string) map[string]string {
	groupsCopy := setting.GetUserUsableGroupsCopy()
	if userGroup != "" {
		specialSettings, b := ratio_setting.GetGroupRatioSetting().GroupSpecialUsableGroup.Get(userGroup)
		if b {
			// 处理特殊可用分组
			for specialGroup, desc := range specialSettings {
				if after, ok := strings.CutPrefix(specialGroup, "-:"); ok {
					// 移除分组
					groupToRemove := after
					delete(groupsCopy, groupToRemove)
				} else if after, ok := strings.CutPrefix(specialGroup, "+:"); ok {
					// 添加分组
					groupToAdd := after
					groupsCopy[groupToAdd] = desc
				} else {
					// 直接添加分组
					groupsCopy[specialGroup] = desc
				}
			}
		}
		// 如果userGroup不在UserUsableGroups中，返回UserUsableGroups + userGroup。
		// 描述是英文源文本，由 web 控制台翻译；管理员配置的描述原样显示。
		if _, ok := groupsCopy[userGroup]; !ok {
			groupsCopy[userGroup] = "User group"
		}
	}
	return groupsCopy
}

func GroupInUserUsableGroups(userGroup, groupName string) bool {
	_, ok := GetUserUsableGroups(userGroup)[groupName]
	return ok
}

func IsUserSelectableGroup(userGroup, groupName string) bool {
	if groupName == "" || groupName == "auto" {
		return false
	}
	return GroupInUserUsableGroups(userGroup, groupName) && ratio_setting.ContainsGroupRatio(groupName)
}

// GetUserAutoGroup 根据用户分组获取自动分组设置
func GetUserAutoGroup(userGroup string) []string {
	autoGroups := make([]string, 0)
	seen := make(map[string]struct{})
	for _, group := range setting.GetAutoGroups() {
		if !IsUserSelectableGroup(userGroup, group) {
			continue
		}
		if _, ok := seen[group]; ok {
			continue
		}
		seen[group] = struct{}{}
		autoGroups = append(autoGroups, group)
	}
	return autoGroups
}

// FilterUserTokenAutoGroups applies current permissions before the current
// per-token limit. It intentionally does not fall back to the global Auto list.
func FilterUserTokenAutoGroups(userGroup string, groups []string) []string {
	maxCount := setting.GetMaxTokenAutoGroups()
	filtered := make([]string, 0, min(len(groups), maxCount))
	seen := make(map[string]struct{})
	for _, group := range groups {
		if !IsUserSelectableGroup(userGroup, group) {
			continue
		}
		if _, ok := seen[group]; ok {
			continue
		}
		seen[group] = struct{}{}
		filtered = append(filtered, group)
		if len(filtered) == maxCount {
			break
		}
	}
	return filtered
}

// GetRequestAutoGroups resolves the ordered Auto groups for the current token.
// The absence of the context value means that the token inherits the complete
// global Auto list; a present (even empty) value is an explicit token snapshot.
func GetRequestAutoGroups(c *gin.Context, userGroup string) []string {
	value, ok := common.GetContextKey(c, constant.ContextKeyTokenAutoGroups)
	if !ok {
		return filterOverseasGroups(c, GetUserAutoGroup(userGroup))
	}
	groups, ok := value.([]string)
	if !ok {
		return []string{}
	}
	return filterOverseasGroups(c, FilterUserTokenAutoGroups(userGroup, groups))
}

// filterOverseasGroups drops the groups that serve overseas models while the
// user has not declared themselves a foreign user, so an Auto request falls
// back to the domestic groups instead of being rejected. The relay gate in
// middleware.Distribute still rejects a request whose only candidates are
// overseas.
func filterOverseasGroups(c *gin.Context, groups []string) []string {
	if model.GetContextUserOverseasConfirmed(c) {
		return groups
	}
	if !slices.ContainsFunc(groups, group_region_setting.IsOverseasGroup) {
		return groups
	}
	domestic := make([]string, 0, len(groups))
	for _, group := range groups {
		if !group_region_setting.IsOverseasGroup(group) {
			domestic = append(domestic, group)
		}
	}
	return domestic
}

// OverseasDeclarationBlockedGroup reports the group that rejects the request
// because it only serves overseas models and the user has not declared
// themselves a foreign user. It returns an empty string when the request may
// proceed: the user declared, no candidate serves overseas models, or at least
// one candidate is a domestic group.
func OverseasDeclarationBlockedGroup(userGroup, usingGroup string, confirmed bool) string {
	if confirmed {
		return ""
	}
	candidates := []string{usingGroup}
	if usingGroup == "auto" {
		candidates = GetUserAutoGroup(userGroup)
	}
	if !slices.ContainsFunc(candidates, group_region_setting.IsOverseasGroup) {
		return ""
	}
	if slices.ContainsFunc(candidates, func(group string) bool {
		return !group_region_setting.IsOverseasGroup(group)
	}) {
		return ""
	}
	if usingGroup == "" {
		return "auto"
	}
	return usingGroup
}

// GetGroupsEnabledModels 按 groups 顺序获取各分组启用的模型并去重
func GetGroupsEnabledModels(groups []string) []string {
	seen := make(map[string]struct{})
	models := make([]string, 0)
	for _, group := range groups {
		for _, modelName := range model.GetGroupEnabledModels(group) {
			if _, ok := seen[modelName]; !ok {
				seen[modelName] = struct{}{}
				models = append(models, modelName)
			}
		}
	}
	return models
}

// GetUserGroupRatio 获取用户使用某个分组的倍率
// userGroup 用户分组
// group 需要获取倍率的分组
// rechargeAmount 用户累计充值金额，用于叠加用户等级给出的更优惠倍率
func GetUserGroupRatio(userGroup, group string, rechargeAmount float64) float64 {
	ratio, ok := ratio_setting.GetGroupGroupRatio(userGroup, group)
	if !ok {
		ratio = ratio_setting.GetGroupRatio(group)
	}
	return user_level_setting.EffectiveGroupRatio(rechargeAmount, group, ratio)
}
