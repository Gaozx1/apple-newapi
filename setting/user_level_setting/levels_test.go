package user_level_setting

import (
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// installLevels replaces the running ladder for one test and restores it after.
func installLevels(t *testing.T, enabled bool, levels []UserLevel) {
	t.Helper()
	config := GetUserLevelSetting()
	previousEnabled := config.Enabled
	previousLevels := slices.Clone(config.Levels)
	t.Cleanup(func() {
		config.Enabled = previousEnabled
		config.Levels = previousLevels
		RebuildLevelIndex()
	})
	config.Enabled = enabled
	config.Levels = levels
	RebuildLevelIndex()
}

func TestResolveStatusFollowsThresholdsInMonetaryOrder(t *testing.T) {
	installLevels(t, true, []UserLevel{
		{Name: "青铜", Threshold: 0},
		{Name: "白银", Threshold: 100},
		{Name: "黄金", Threshold: 500.5},
	})

	for _, tc := range []struct {
		name          string
		amount        float64
		wantNumber    int
		wantName      string
		wantHasLevel  bool
		wantNext      string
		wantNextValue float64
	}{
		{"zero reaches the first rung", 0, 1, "青铜", true, "白银", 100},
		{"just below the second threshold stays on the first", 99.99, 1, "青铜", true, "白银", 100},
		{"exactly on the threshold reaches it", 100, 2, "白银", true, "黄金", 500.5},
		{"just below the top threshold stays on the second rung", 500.49, 2, "白银", true, "黄金", 500.5},
		{"above the top rung has no next rung", 1000, 3, "黄金", true, "", 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			status := ResolveStatus(tc.amount)
			require.True(t, status.Enabled)
			assert.Equal(t, tc.wantHasLevel, status.HasLevel)
			assert.Equal(t, tc.wantNumber, status.Number)
			assert.Equal(t, tc.wantName, status.Name)
			assert.Equal(t, tc.wantNext, status.NextName)
			assert.Equal(t, tc.wantNextValue, status.NextThreshold)
		})
	}
}

// A lifetime sum of two-decimal payments can miss an exact threshold by one ULP
// in float64 (0.7 + 0.1 = 0.7999999999999999). Thresholds are compared in cents
// so the rung is still reached.
func TestResolveStatusComparesThresholdsInCents(t *testing.T) {
	installLevels(t, true, []UserLevel{{Name: "青铜", Threshold: 0.8}})

	status := ResolveStatus(0.7 + 0.1)
	require.True(t, status.HasLevel)
	assert.Equal(t, "青铜", status.Name)
	assert.Equal(t, 1, status.Number)
}

func TestResolveStatusReportsTheNextRungForAnUnleveledUser(t *testing.T) {
	installLevels(t, true, []UserLevel{{Name: "白银", Threshold: 100}})

	status := ResolveStatus(5)
	require.True(t, status.Enabled)
	assert.False(t, status.HasLevel)
	assert.Empty(t, status.Name)
	require.True(t, status.HasNext)
	assert.Equal(t, "白银", status.NextName)
	assert.Equal(t, 100.0, status.NextThreshold)
}

func TestResolveStatusIsInertWhileDisabled(t *testing.T) {
	installLevels(t, false, []UserLevel{{Name: "白银", Threshold: 100}})

	status := ResolveStatus(500)
	assert.False(t, status.Enabled)
	assert.False(t, status.HasLevel)
	assert.False(t, status.HasNext)
}

func TestEffectiveGroupRatioKeepsTheMoreFavorableRatio(t *testing.T) {
	installLevels(t, true, []UserLevel{{
		Name:        "白银",
		Threshold:   100,
		GroupRatios: map[string]float64{"vip": 0.8, "free": 0},
	}})

	assert.Equal(t, 0.5, EffectiveGroupRatio(100, "vip", 0.5), "a higher grant must not raise the billed ratio")
	assert.Equal(t, 0.8, EffectiveGroupRatio(100, "vip", 1), "a lower grant wins")
	assert.Equal(t, 0.0, EffectiveGroupRatio(100, "free", 1), "a free grant is the most favorable ratio")
	assert.Equal(t, 1.0, EffectiveGroupRatio(100, "default", 1), "groups without a grant keep their ratio")
	assert.Equal(t, 1.0, EffectiveGroupRatio(99.99, "vip", 1), "a user below the threshold keeps the group ratio")
}

func TestEffectiveGroupRatioIsInertWhileDisabled(t *testing.T) {
	installLevels(t, false, []UserLevel{{
		Name:        "白银",
		Threshold:   100,
		GroupRatios: map[string]float64{"vip": 0.8},
	}})

	assert.Equal(t, 1.0, EffectiveGroupRatio(100, "vip", 1))
}

func TestEffectiveGroupRateLimitKeepsTheMorePermissiveLimit(t *testing.T) {
	installLevels(t, true, []UserLevel{{
		Name:            "白银",
		Threshold:       100,
		GroupRateLimits: map[string][2]int{"vip": {200, 30}, "free": {0, 0}},
	}})

	total, success := EffectiveGroupRateLimit(100, "vip", 100, 50)
	assert.Equal(t, 200, total, "the higher request cap wins")
	assert.Equal(t, 50, success, "a lower grant must not tighten the success cap")

	total, success = EffectiveGroupRateLimit(100, "free", 100, 50)
	assert.Equal(t, 0, total, "0 means unlimited and always wins")
	assert.Equal(t, 0, success)

	total, success = EffectiveGroupRateLimit(100, "default", 100, 50)
	assert.Equal(t, 100, total, "groups without a grant keep the configured limit")
	assert.Equal(t, 50, success)

	total, success = EffectiveGroupRateLimit(99.99, "vip", 100, 50)
	assert.Equal(t, 100, total, "a user below the threshold keeps the configured limit")
	assert.Equal(t, 50, success)
}

func TestValidateLevelsJSON(t *testing.T) {
	for _, tc := range []struct {
		name    string
		value   string
		wantErr string
	}{
		{name: "empty is accepted", value: ""},
		{name: "valid ladder is accepted", value: `[{"name":"青铜","threshold":0,"group_ratios":{"default":1,"vip":0.8},"group_rate_limits":{"vip":[0,1000]}}]`},
		{
			name:    "duplicate thresholds are rejected",
			value:   `[{"name":"甲","threshold":100},{"name":"乙","threshold":100}]`,
			wantErr: "重复",
		},
		{
			name:    "unknown group ratio is rejected",
			value:   `[{"name":"甲","threshold":0,"group_ratios":{"missing-group":0.5}}]`,
			wantErr: "不存在的分组",
		},
		{
			name:    "negative rate limit is rejected",
			value:   `[{"name":"甲","threshold":0,"group_rate_limits":{"default":[-1,100]}}]`,
			wantErr: "不能为负数",
		},
		{
			name:    "empty level name is rejected",
			value:   `[{"name":"  ","threshold":0}]`,
			wantErr: "等级名称不能为空",
		},
		{
			name:    "malformed json is rejected",
			value:   `{"name":"甲"}`,
			wantErr: "必须是 JSON 数组",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateLevelsJSON(tc.value)
			if tc.wantErr == "" {
				require.NoError(t, err)
				return
			}
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.wantErr)
		})
	}
}

func TestBuildLevelIndexDropsInvalidRungsAndKeepsNumbering(t *testing.T) {
	idx := buildLevelIndex(UserLevelSetting{Enabled: true, Levels: []UserLevel{
		{Name: "甲", Threshold: 100},
		{Name: "  ", Threshold: 200},
		{Name: "丙", Threshold: -1},
		{Name: "丁", Threshold: 300},
	}})

	require.Len(t, idx.levels, 2)
	assert.Equal(t, "甲", idx.levels[0].name)
	assert.Equal(t, 1, idx.levels[0].number)
	assert.Equal(t, "丁", idx.levels[1].name)
	assert.Equal(t, 2, idx.levels[1].number)
}
