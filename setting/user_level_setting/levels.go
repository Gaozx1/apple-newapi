package user_level_setting

import (
	"fmt"
	"math"
	"slices"
	"strings"
	"sync/atomic"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/setting/config"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
)

// User levels are a loyalty ladder over the cumulative amount a user has paid
// (wallet top-ups, subscription purchases and redemption codes). Each level may
// lower the ratio of several groups and raise their request rate limits.
//
// Option keys:
//   - user_level_setting.enabled : whether the ladder is applied at all
//   - user_level_setting.levels  : JSON array of levels, see UserLevel
const (
	EnabledKey = "user_level_setting.enabled"
	LevelsKey  = "user_level_setting.levels"
)

// maxThresholdCents bounds a configured threshold. Money carries two decimals,
// so this keeps the cents conversion inside int64 and well inside the range
// where float64 still represents integers exactly.
const maxThresholdCents int64 = 1e15

// maxRatio bounds a configured group ratio.
const maxRatio = 1e6

// UserLevel is one rung of the ladder. Threshold is a cumulative paid amount in
// the same money unit the dashboard displays (USD-equivalent, i.e. quota /
// QuotaPerUnit). GroupRatios and GroupRateLimits are explicit per-level grants:
// a level never inherits the grants of a lower level.
type UserLevel struct {
	// Name is the level name shown to users.
	Name string `json:"name"`
	// Threshold is the cumulative paid amount that unlocks this level.
	Threshold float64 `json:"threshold"`
	// GroupRatios maps a group name to the ratio this level grants for it.
	GroupRatios map[string]float64 `json:"group_ratios"`
	// GroupRateLimits maps a group name to [max requests including failures,
	// max successful requests] over the configured rate-limit window. 0 means
	// unlimited.
	GroupRateLimits map[string][2]int `json:"group_rate_limits"`
}

// UserLevelSetting is managed by config.GlobalConfig.Register. It is the
// persisted form only: request paths read the atomic index built by
// RebuildLevelIndex, so a configuration write can never tear a live read.
type UserLevelSetting struct {
	Enabled bool        `json:"enabled"`
	Levels  []UserLevel `json:"levels"`
}

// userLevelSetting carries no levels until an administrator configures them;
// the empty slice keeps the exported option value an array instead of null.
var userLevelSetting = UserLevelSetting{Levels: []UserLevel{}}

func init() {
	config.GlobalConfig.Register("user_level_setting", &userLevelSetting)
	RebuildLevelIndex()
}

// GetUserLevelSetting exposes the registered configuration for writers (option
// updates and tests). Request paths must use the accessors below.
func GetUserLevelSetting() *UserLevelSetting {
	return &userLevelSetting
}

// level is the normalized, immutable form of one UserLevel.
type level struct {
	number          int
	name            string
	thresholdCents  int64
	groupRatios     map[string]float64
	groupRateLimits map[string][2]int
}

type levelIndex struct {
	enabled bool
	// levels is sorted by thresholdCents ascending; level number N is levels[N-1].
	levels []level
}

var currentIndex atomic.Pointer[levelIndex]

// LevelStatus is the user-facing level status of a cumulative recharge amount.
type LevelStatus struct {
	Enabled    bool    `json:"enabled"`
	HasLevel   bool    `json:"has_level"`
	Number     int     `json:"number"`
	Name       string  `json:"name"`
	Threshold  float64 `json:"threshold"`
	HasNext    bool    `json:"has_next"`
	NextNumber int     `json:"next_number"`
	NextName   string  `json:"next_name"`
	// NextThreshold is the cumulative amount that unlocks the next level.
	NextThreshold float64 `json:"next_threshold"`
}

// moneyToCents converts a money amount to integer cents. Comparisons run on
// cents so a sum of two-decimal payments can never miss a threshold by a float
// rounding error.
func moneyToCents(amount float64) (int64, bool) {
	if math.IsNaN(amount) || math.IsInf(amount, 0) {
		return 0, false
	}
	cents := math.Round(amount * 100)
	if cents > float64(maxThresholdCents) || cents < float64(-maxThresholdCents) {
		return 0, false
	}
	return int64(cents), true
}

func centsToMoney(cents int64) float64 {
	return float64(cents) / 100
}

func validRatio(ratio float64) bool {
	return ratio >= 0 && ratio <= maxRatio && !math.IsNaN(ratio) && !math.IsInf(ratio, 0)
}

// buildLevelIndex validates and normalizes a configuration snapshot. Invalid
// rungs are dropped and logged: a bad saved value must never poison the request
// path, and dropping whole rungs keeps every remaining level number stable.
func buildLevelIndex(cfg UserLevelSetting) *levelIndex {
	idx := &levelIndex{enabled: cfg.Enabled}
	if !cfg.Enabled {
		return idx
	}

	for i := range cfg.Levels {
		raw := cfg.Levels[i]
		name := strings.TrimSpace(raw.Name)
		if name == "" {
			common.SysError("user level dropped: empty name")
			continue
		}
		thresholdCents, ok := moneyToCents(raw.Threshold)
		if !ok || thresholdCents < 0 {
			common.SysError(fmt.Sprintf("user level %q dropped: invalid threshold %v", name, raw.Threshold))
			continue
		}
		ratios := make(map[string]float64, len(raw.GroupRatios))
		for group, ratio := range raw.GroupRatios {
			group = strings.TrimSpace(group)
			if group == "" || !validRatio(ratio) {
				common.SysError(fmt.Sprintf("user level %q dropped group ratio %q: invalid value %v", name, group, ratio))
				continue
			}
			ratios[group] = ratio
		}
		limits := make(map[string][2]int, len(raw.GroupRateLimits))
		for group, pair := range raw.GroupRateLimits {
			group = strings.TrimSpace(group)
			if group == "" || pair[0] < 0 || pair[1] < 0 {
				common.SysError(fmt.Sprintf("user level %q dropped group rate limit %q: invalid value %v", name, group, pair))
				continue
			}
			limits[group] = pair
		}
		idx.levels = append(idx.levels, level{
			name:            name,
			thresholdCents:  thresholdCents,
			groupRatios:     ratios,
			groupRateLimits: limits,
		})
	}

	slices.SortStableFunc(idx.levels, func(a, b level) int {
		return int(a.thresholdCents - b.thresholdCents)
	})
	for i := range idx.levels {
		idx.levels[i].number = i + 1
	}
	return idx
}

func currentLevelIndex() *levelIndex {
	idx := currentIndex.Load()
	if idx != nil {
		return idx
	}
	RebuildLevelIndex()
	return currentIndex.Load()
}

// RebuildLevelIndex republishes the configured ladder. Called at startup and
// after every option write (see model.handleConfigUpdate).
func RebuildLevelIndex() {
	currentIndex.Store(buildLevelIndex(userLevelSetting))
}

// levelFor returns the highest rung whose threshold the amount reached.
func (idx *levelIndex) levelFor(amount float64) *level {
	if idx == nil || !idx.enabled || len(idx.levels) == 0 {
		return nil
	}
	amountCents, ok := moneyToCents(amount)
	if !ok {
		return nil
	}
	reached := -1
	for i := range idx.levels {
		if idx.levels[i].thresholdCents <= amountCents {
			reached = i
		}
	}
	if reached < 0 {
		return nil
	}
	return &idx.levels[reached]
}

// IsEnabled reports whether the level ladder is active.
func IsEnabled() bool {
	idx := currentLevelIndex()
	return idx != nil && idx.enabled
}

// ResolveStatus returns the level a cumulative recharge amount unlocked, plus
// the next rung when one exists.
func ResolveStatus(amount float64) LevelStatus {
	idx := currentLevelIndex()
	if idx == nil || !idx.enabled {
		return LevelStatus{}
	}
	amountCents, ok := moneyToCents(amount)
	if !ok {
		amountCents = 0
	}

	status := LevelStatus{Enabled: true}
	if reached := idx.levelFor(amount); reached != nil {
		status.HasLevel = true
		status.Number = reached.number
		status.Name = reached.name
		status.Threshold = centsToMoney(reached.thresholdCents)
	}
	for i := range idx.levels {
		if idx.levels[i].thresholdCents > amountCents {
			status.HasNext = true
			status.NextNumber = idx.levels[i].number
			status.NextName = idx.levels[i].name
			status.NextThreshold = centsToMoney(idx.levels[i].thresholdCents)
			break
		}
	}
	return status
}

// EffectiveGroupRatio returns the ratio a request on group is billed with: the
// more favorable (lower) of the configured group ratio and the level grant. A
// level never makes a group more expensive.
func EffectiveGroupRatio(amount float64, group string, baseRatio float64) float64 {
	if group == "" {
		return baseRatio
	}
	lv := currentLevelIndex().levelFor(amount)
	if lv == nil {
		return baseRatio
	}
	grant, ok := lv.groupRatios[group]
	if !ok || grant >= baseRatio {
		return baseRatio
	}
	return grant
}

// EffectiveGroupRateLimit returns the request cap a user on group may use: the
// more favorable (higher) of the configured group limit and the level grant,
// where 0 means unlimited and therefore always wins.
func EffectiveGroupRateLimit(amount float64, group string, totalMax, successMax int) (int, int) {
	if group == "" {
		return totalMax, successMax
	}
	lv := currentLevelIndex().levelFor(amount)
	if lv == nil {
		return totalMax, successMax
	}
	grant, ok := lv.groupRateLimits[group]
	if !ok {
		return totalMax, successMax
	}
	return morePermissiveLimit(totalMax, grant[0]), morePermissiveLimit(successMax, grant[1])
}

// morePermissiveLimit picks the looser of two request caps. Zero disables the
// cap entirely, so it is always the looser value.
func morePermissiveLimit(configured, granted int) int {
	if configured == 0 || granted == 0 {
		return 0
	}
	return max(configured, granted)
}

// ValidateLevelsJSON validates an operator-supplied level ladder. It is the save
// gate for user_level_setting.levels, so admins get immediate feedback instead
// of silently dropped rungs.
func ValidateLevelsJSON(value string) error {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	var levels []UserLevel
	if err := common.UnmarshalJsonStr(value, &levels); err != nil {
		return fmt.Errorf("等级配置必须是 JSON 数组: %w", err)
	}

	seenThresholds := make(map[int64]string, len(levels))
	for i := range levels {
		raw := levels[i]
		position := fmt.Sprintf("第 %d 个等级", i+1)
		name := strings.TrimSpace(raw.Name)
		if name == "" {
			return fmt.Errorf("%s 的等级名称不能为空", position)
		}
		if len(name) > 64 {
			return fmt.Errorf("%s 的等级名称不能超过 64 个字符", position)
		}
		thresholdCents, ok := moneyToCents(raw.Threshold)
		if !ok || thresholdCents < 0 {
			return fmt.Errorf("%s 的累计充值金额门槛必须是不小于 0 的数字", position)
		}
		if other, duplicated := seenThresholds[thresholdCents]; duplicated {
			return fmt.Errorf("%s 的累计充值金额门槛与等级 %q 重复，每个等级的门槛必须不同", position, other)
		}
		seenThresholds[thresholdCents] = name

		for group, ratio := range raw.GroupRatios {
			if strings.TrimSpace(group) == "" {
				return fmt.Errorf("%s 的分组倍率中存在空分组名", position)
			}
			if !validRatio(ratio) {
				return fmt.Errorf("%s 分组 %s 的倍率必须是 0 到 %v 之间的数字", position, group, maxRatio)
			}
			if !ratio_setting.ContainsGroupRatio(group) {
				return fmt.Errorf("%s 引用了不存在的分组 %s", position, group)
			}
		}
		for group, pair := range raw.GroupRateLimits {
			if strings.TrimSpace(group) == "" {
				return fmt.Errorf("%s 的速率限制中存在空分组名", position)
			}
			if pair[0] < 0 || pair[1] < 0 {
				return fmt.Errorf("%s 分组 %s 的速率限制不能为负数，0 表示不限制", position, group)
			}
			if int64(pair[0]) > setting.MaxModelRequestRateLimitCount || int64(pair[1]) > setting.MaxModelRequestRateLimitCount {
				return fmt.Errorf("%s 分组 %s 的速率限制不能超过 %d", position, group, setting.MaxModelRequestRateLimitCount)
			}
		}
	}
	return nil
}
