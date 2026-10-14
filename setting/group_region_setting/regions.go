package group_region_setting

import (
	"fmt"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/config"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/QuantumNous/new-api/types"
)

// A group can be classified as serving overseas models or domestic models.
// Using a group that serves overseas models requires the user to declare that
// they are a foreign user first (see model.ConfirmOverseasDeclaration); the
// declaration gate lives in middleware.Distribute.
//
// Option key:
//   - group_region_setting.groups : JSON object mapping a group name to
//     "overseas" or "domestic". Groups without an entry behave as before — no
//     declaration is required.
const (
	// GroupsKey is the option key holding the group → region map.
	GroupsKey = "group_region_setting.groups"
	// RegionOverseas marks a group serving overseas models.
	RegionOverseas = "overseas"
	// RegionDomestic marks a group serving domestic models.
	RegionDomestic = "domestic"
)

// GroupRegionSetting is managed by config.GlobalConfig.Register. The map is
// replaced as a whole on every update, and reads go through the lock-protected
// RWMap, so a configuration write cannot tear a live lookup.
type GroupRegionSetting struct {
	Groups *types.RWMap[string, string] `json:"groups"`
}

var groupRegionSetting = GroupRegionSetting{
	Groups: types.NewRWMap[string, string](),
}

func init() {
	config.GlobalConfig.Register("group_region_setting", &groupRegionSetting)
}

// GetGroupRegionSetting exposes the registered configuration for writers
// (option updates and tests).
func GetGroupRegionSetting() *GroupRegionSetting {
	if groupRegionSetting.Groups == nil {
		groupRegionSetting.Groups = types.NewRWMap[string, string]()
	}
	return &groupRegionSetting
}

// RegionOf returns the configured region of a group. Groups without an entry
// report the domestic region, which never requires a declaration.
func RegionOf(group string) string {
	if group == "" {
		return RegionDomestic
	}
	region, ok := GetGroupRegionSetting().Groups.Get(group)
	if !ok {
		return RegionDomestic
	}
	return region
}

// IsOverseasGroup reports whether using the group requires the user's
// foreign-user declaration.
func IsOverseasGroup(group string) bool {
	return RegionOf(group) == RegionOverseas
}

// RegionsCopy returns a snapshot of the group → region map.
func RegionsCopy() map[string]string {
	return GetGroupRegionSetting().Groups.ReadAll()
}

// ValidateRegionsJSON validates an operator-supplied group → region map. It is
// the save gate for group_region_setting.groups, so admins get immediate
// feedback instead of silently ignored classifications.
func ValidateRegionsJSON(value string) error {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	regions := map[string]string{}
	if err := common.UnmarshalJsonStr(value, &regions); err != nil {
		return fmt.Errorf("分组地区配置必须是 JSON 对象: %w", err)
	}
	for group, region := range regions {
		name := strings.TrimSpace(group)
		if name == "" {
			return fmt.Errorf("分组地区配置中存在空分组名")
		}
		if region != RegionOverseas && region != RegionDomestic {
			return fmt.Errorf("分组 %s 的地区必须是 %s（海外模型）或 %s（国内模型）", group, RegionOverseas, RegionDomestic)
		}
		if region == RegionOverseas && !ratio_setting.ContainsGroupRatio(name) {
			return fmt.Errorf("分组 %s 不存在，无法标记为海外模型分组", group)
		}
	}
	return nil
}
