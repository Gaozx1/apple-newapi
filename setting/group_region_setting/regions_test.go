package group_region_setting

import (
	"testing"

	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// installRegions replaces the group region map for one test and restores it.
func installRegions(t *testing.T, regions map[string]string) {
	t.Helper()
	groups := GetGroupRegionSetting().Groups
	previous := groups.ReadAll()
	t.Cleanup(func() {
		groups.Clear()
		groups.AddAll(previous)
	})
	groups.Clear()
	groups.AddAll(regions)
}

func TestIsOverseasGroupOnlyMatchesConfiguredOverseasGroups(t *testing.T) {
	installRegions(t, map[string]string{
		"vip":     RegionOverseas,
		"default": RegionDomestic,
	})

	assert.True(t, IsOverseasGroup("vip"))
	assert.False(t, IsOverseasGroup("default"))
	assert.False(t, IsOverseasGroup("unclassified"))
	assert.False(t, IsOverseasGroup(""))
	assert.Equal(t, RegionOverseas, RegionOf("vip"))
	assert.Equal(t, RegionDomestic, RegionOf("unclassified"))
}

func TestValidateRegionsJSON(t *testing.T) {
	// The validation requires the classified group to exist, so make sure the
	// group the overseas cases reference is a real group ratio entry.
	previousRatios := ratio_setting.GroupRatio2JSONString()
	t.Cleanup(func() {
		require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(previousRatios))
	})
	require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(`{"default":1,"overseas-test":1}`))

	for _, tc := range []struct {
		name    string
		value   string
		wantErr string
	}{
		{name: "empty is accepted", value: ""},
		{name: "both regions are accepted", value: `{"overseas-test":"overseas","default":"domestic"}`},
		{
			name:    "an unknown region is rejected",
			value:   `{"default":"mars"}`,
			wantErr: "地区必须是",
		},
		{
			name:    "an unknown group cannot serve overseas models",
			value:   `{"missing-group":"overseas"}`,
			wantErr: "不存在",
		},
		{
			name:    "an empty group name is rejected",
			value:   `{"  ":"domestic"}`,
			wantErr: "空分组名",
		},
		{
			name:    "malformed json is rejected",
			value:   `["default"]`,
			wantErr: "必须是 JSON 对象",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateRegionsJSON(tc.value)
			if tc.wantErr == "" {
				require.NoError(t, err)
				return
			}
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.wantErr)
		})
	}
}
