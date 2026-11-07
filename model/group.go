package model

import (
	"database/sql/driver"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/setting/group_region_setting"
	"github.com/QuantumNous/new-api/setting/ratio_setting"

	"gorm.io/gorm"
)

// Group is a first-class, renameable group definition.
//
// Why this type exists: historically a group was only ever a bare string. The
// same string was written into channel.group, token.group, user.group,
// ability.group and every ratio/usable-group map, so renaming a group meant
// rewriting every reference by hand. Miss one and the channels and keys that
// still held the old string silently stopped resolving.
//
// Code is that stable identifier. It is what every existing string reference
// stores, and it never changes when an operator renames a group. Renaming only
// touches Names, which holds one display name per language, so a rename is
// purely cosmetic and cannot invalidate a channel or a token.
//
// Changing Code itself is still possible but is a deliberate migration: see
// RenameGroupCode, which rewrites every reference in one transaction.
type Group struct {
	Id   int    `json:"id" gorm:"primaryKey"`
	Code string `json:"code" gorm:"type:varchar(64);uniqueIndex;not null"`
	// Names maps a language tag ("en", "zh", "zh-TW", "fr", "ru", "ja", "vi")
	// to the display name in that language. Any subset is allowed; DisplayName
	// falls back through the language list and finally to Code.
	Names GroupNames `json:"names" gorm:"type:text"`
	// Ratio is the billing multiplier for this group. It mirrors
	// ratio_setting's group ratio map, which stays the value the relay reads.
	Ratio float64 `json:"ratio" gorm:"type:decimal(20,10)"`
	// Region mirrors setting/group_region_setting: "overseas" or "domestic".
	Region    string    `json:"region" gorm:"type:varchar(32)"`
	Enabled   bool      `json:"enabled"`
	Sort      int       `json:"sort"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (Group) TableName() string {
	return "groups"
}

// Language tags a group display name may be translated into. This is the
// frontend locale set from web/src/i18n; "en" is the source language and also
// the fallback used when a translation is missing.
const (
	GroupLanguageEn   = "en"
	GroupLanguageZh   = "zh"
	GroupLanguageZhTW = "zh-TW"
	GroupLanguageFr   = "fr"
	GroupLanguageRu   = "ru"
	GroupLanguageJa   = "ja"
	GroupLanguageVi   = "vi"
)

// GroupNameLanguages lists every language a display name may be provided in,
// in fallback order.
var GroupNameLanguages = []string{
	GroupLanguageEn,
	GroupLanguageZh,
	GroupLanguageZhTW,
	GroupLanguageFr,
	GroupLanguageRu,
	GroupLanguageJa,
	GroupLanguageVi,
}

// DefaultGroupCode is the group every user and channel starts in. It cannot be
// deleted because users, tokens and channels default to it.
const DefaultGroupCode = "default"

// maxGroupCodeLength matches the varchar(64) the reference columns use.
const maxGroupCodeLength = 64

// ValidateGroupCode accepts any identifier that can round-trip through the
// existing comma separated group lists.
//
// A code is opaque, not a slug. Real deployments already use names such as
// "ccmax0.9", "鎺ョ爜", "Gemini Ultra 0.3" and "[鐢ㄦ埛]娴嬭瘯111" 鈥?they encode price
// tiers, languages and brackets. Rejecting those would silently leave every such
// channel unregistered, so the only hard requirements are the ones the storage
// format itself imposes: non-empty, no comma (it separates the list), no
// surrounding whitespace (it would be trimmed away and break the round trip),
// and within the column width.
func ValidateGroupCode(code string) error {
	if code == "" {
		return errors.New("group code is required")
	}
	if code != strings.TrimSpace(code) {
		return errors.New("group code must not start or end with whitespace")
	}
	if strings.Contains(code, ",") {
		return errors.New("group code must not contain a comma, which separates group lists")
	}
	if len([]rune(code)) > maxGroupCodeLength {
		return fmt.Errorf("group code must be at most %d characters", maxGroupCodeLength)
	}
	return nil
}

// GroupNames is a language tag to display name map persisted as JSON text.
type GroupNames map[string]string

// Value implements driver.Valuer. It returns a string rather than []byte so the
// PostgreSQL simple protocol does not treat the value as bytea.
func (n GroupNames) Value() (driver.Value, error) {
	if len(n) == 0 {
		return "{}", nil
	}
	encoded, err := common.Marshal(map[string]string(n))
	if err != nil {
		return nil, err
	}
	return string(encoded), nil
}

// Scan implements sql.Scanner, accepting the driver types used by SQLite,
// MySQL and PostgreSQL.
func (n *GroupNames) Scan(value any) error {
	var raw []byte
	switch typed := value.(type) {
	case nil:
		*n = GroupNames{}
		return nil
	case []byte:
		raw = typed
	case string:
		raw = []byte(typed)
	default:
		encoded, err := common.Marshal(typed)
		if err != nil {
			return err
		}
		raw = encoded
	}
	if len(strings.TrimSpace(string(raw))) == 0 {
		*n = GroupNames{}
		return nil
	}
	decoded := map[string]string{}
	if err := common.Unmarshal(raw, &decoded); err != nil {
		return err
	}
	*n = GroupNames(decoded)
	return nil
}

// normalizeGroupLanguage maps a requested language to the tag used in Names.
// "zh-CN" and "zh-Hans-CN" both collapse to "zh"; "zh-TW"/"zh-Hant" stay
// distinct because they are separate translations.
func normalizeGroupLanguage(language string) string {
	trimmed := strings.TrimSpace(language)
	if trimmed == "" {
		return GroupLanguageEn
	}
	lowered := strings.ToLower(trimmed)
	switch {
	case strings.HasPrefix(lowered, "zh-hant"), strings.HasPrefix(lowered, "zh-tw"),
		strings.HasPrefix(lowered, "zh-hk"):
		return GroupLanguageZhTW
	case strings.HasPrefix(lowered, "zh"):
		return GroupLanguageZh
	}
	if index := strings.IndexAny(lowered, "-_"); index > 0 {
		lowered = lowered[:index]
	}
	return lowered
}

// DisplayName returns the best display name for the requested language. It
// never returns an empty string: when no translation exists it falls back to
// English, then any available translation, then the code itself.
func (g *Group) DisplayName(language string) string {
	if g == nil {
		return ""
	}
	if len(g.Names) > 0 {
		if name := strings.TrimSpace(g.Names[normalizeGroupLanguage(language)]); name != "" {
			return name
		}
		if name := strings.TrimSpace(g.Names[GroupLanguageEn]); name != "" {
			return name
		}
		if name := strings.TrimSpace(g.Names[GroupLanguageZh]); name != "" {
			return name
		}
		for _, tag := range GroupNameLanguages {
			if name := strings.TrimSpace(g.Names[tag]); name != "" {
				return name
			}
		}
	}
	return g.Code
}

// GroupRequest is the payload accepted when creating or updating a group.
type GroupRequest struct {
	Code    string            `json:"code"`
	Names   map[string]string `json:"names"`
	Ratio   *float64          `json:"ratio"`
	Region  string            `json:"region"`
	Enabled *bool             `json:"enabled"`
	Sort    *int              `json:"sort"`
}

// sanitizeGroupNames keeps only known language tags, trims values, and drops
// blank entries so a display name can never be whitespace.
func sanitizeGroupNames(names map[string]string) GroupNames {
	sanitized := GroupNames{}
	for tag, name := range names {
		normalized := normalizeGroupLanguage(tag)
		trimmed := strings.TrimSpace(name)
		if trimmed == "" {
			continue
		}
		// An explicit translation always wins over a value inferred from an
		// alias tag, so iterate deterministically by preferring exact matches.
		if _, exists := sanitized[normalized]; exists && tag != normalized {
			continue
		}
		sanitized[normalized] = trimmed
	}
	return sanitized
}

func validateGroupRatio(ratio float64) error {
	if ratio < 0 {
		return errors.New("group ratio must not be less than 0")
	}
	return nil
}

func validateGroupRegion(region string) error {
	switch region {
	case "", group_region_setting.RegionOverseas, group_region_setting.RegionDomestic:
		return nil
	default:
		return fmt.Errorf("group region must be %q or %q", group_region_setting.RegionOverseas, group_region_setting.RegionDomestic)
	}
}

// splitGroupCodes turns a stored comma separated group list into trimmed
// codes, dropping empties and duplicates while preserving order.
func splitGroupCodes(raw string) []string {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	codes := make([]string, 0, 4)
	seen := make(map[string]struct{}, 4)
	for _, part := range strings.Split(raw, ",") {
		code := strings.TrimSpace(part)
		if code == "" {
			continue
		}
		if _, exists := seen[code]; exists {
			continue
		}
		seen[code] = struct{}{}
		codes = append(codes, code)
	}
	return codes
}

func joinGroupCodes(codes []string) string {
	return strings.Join(codes, ",")
}

// replaceGroupCodeInList rewrites one code inside a comma separated list and
// reports whether anything changed.
func replaceGroupCodeInList(raw, oldCode, newCode string) (string, bool) {
	codes := splitGroupCodes(raw)
	changed := false
	for index, code := range codes {
		if code == oldCode {
			codes[index] = newCode
			changed = true
		}
	}
	if !changed {
		return raw, false
	}
	return joinGroupCodes(codes), true
}

// GetGroups returns every group ordered for display.
func GetGroups() ([]*Group, error) {
	groups := make([]*Group, 0)
	err := DB.Order("sort asc, id asc").Find(&groups).Error
	return groups, err
}

// GetGroupByCode loads one group by its stable code.
func GetGroupByCode(code string) (*Group, error) {
	group := &Group{}
	err := DB.Where("code = ?", code).First(group).Error
	return group, err
}

// groupCodeExists reports whether a code is already taken. A count query keeps
// the "is it free?" check free of the record-not-found log noise a First call
// would produce on every legitimate create.
func groupCodeExists(code string) (bool, error) {
	var count int64
	if err := DB.Model(&Group{}).Where("code = ?", code).Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

// GetGroupById loads one group by primary key.
func GetGroupById(id int) (*Group, error) {
	group := &Group{}
	err := DB.First(group, id).Error
	return group, err
}

// GetGroupCodes returns every registered code. Relay code compares group
// strings, so this is the authoritative list of codes that still exist.
func GetGroupCodes() ([]string, error) {
	codes := make([]string, 0)
	err := DB.Model(&Group{}).Order("sort asc, id asc").Pluck("code", &codes).Error
	return codes, err
}

// syncGroupSettingsFromGroups mirrors group rows into the runtime maps the
// relay actually reads. The database is the source of truth for names; the
// ratio and region maps stay authoritative for pricing so every existing
// caller keeps working unchanged.
func syncGroupSettingsFromGroups(groups []*Group) {
	ratioSetting := ratio_setting.GetGroupRatioSetting()
	usableGroups := setting.GetUserUsableGroupsCopy()
	regionSetting := group_region_setting.GetGroupRegionSetting()

	for _, group := range groups {
		if group == nil || group.Code == "" {
			continue
		}
		ratioSetting.GroupRatio.Set(group.Code, group.Ratio)
		usableGroups[group.Code] = group.DisplayName(GroupLanguageEn)
		if group.Region != "" {
			regionSetting.Groups.Set(group.Code, group.Region)
		} else {
			regionSetting.Groups.Delete(group.Code)
		}
	}
	if err := setting.UpdateUserUsableGroupsByJSONString(mustMarshalString(usableGroups)); err != nil {
		common.SysLog(common.LogText("error syncing usable groups: %s", err.Error()))
	}
}

func mustMarshalString(value map[string]string) string {
	encoded, err := common.Marshal(value)
	if err != nil {
		common.SysLog(common.LogText("error marshalling group settings: %s", err.Error()))
		return "{}"
	}
	return string(encoded)
}

// persistGroupOptions writes the mirrored maps back through the option store so
// a restart and the admin console both observe the same values.
func persistGroupOptions() error {
	if err := UpdateOption("GroupRatio", ratio_setting.GroupRatio2JSONString()); err != nil {
		return err
	}
	if err := UpdateOption("UserUsableGroups", setting.UserUsableGroups2JSONString()); err != nil {
		return err
	}
	regionSetting := group_region_setting.GetGroupRegionSetting()
	if err := UpdateOption(group_region_setting.GroupsKey, regionSetting.Groups.MarshalJSONString()); err != nil {
		// A group region is optional configuration; a failure here must not roll
		// back a rename that already rewrote real references.
		common.SysLog(common.LogText("error persisting group regions: %s", err.Error()))
	}
	return nil
}

// applyGroups rewrites the runtime maps for the given groups and persists them.
func applyGroups(groups ...*Group) error {
	syncGroupSettingsFromGroups(groups)
	return persistGroupOptions()
}

// CreateGroup registers a new group. The code must be unused.
func CreateGroup(request GroupRequest) (*Group, error) {
	if err := ValidateGroupCode(request.Code); err != nil {
		return nil, err
	}
	names := sanitizeGroupNames(request.Names)
	if len(names) == 0 {
		return nil, errors.New("at least one group display name is required")
	}
	ratio := 1.0
	if request.Ratio != nil {
		ratio = *request.Ratio
	}
	if err := validateGroupRatio(ratio); err != nil {
		return nil, err
	}
	if err := validateGroupRegion(request.Region); err != nil {
		return nil, err
	}
	enabled := true
	if request.Enabled != nil {
		enabled = *request.Enabled
	}
	sortOrder := 0
	if request.Sort != nil {
		sortOrder = *request.Sort
	}

	alreadyExists, err := groupCodeExists(request.Code)
	if err != nil {
		return nil, err
	}
	if alreadyExists {
		return nil, fmt.Errorf("group code %q already exists", request.Code)
	}

	group := &Group{
		Code:    request.Code,
		Names:   names,
		Ratio:   ratio,
		Region:  request.Region,
		Enabled: enabled,
		Sort:    sortOrder,
	}
	if err := DB.Create(group).Error; err != nil {
		return nil, err
	}
	if err := applyGroups(group); err != nil {
		return nil, err
	}
	return group, nil
}

// UpdateGroup edits the display names and metadata of an existing group. The
// code is deliberately not editable here: changing it would invalidate every
// channel and token that references it. Use RenameGroupCode for that.
func UpdateGroup(id int, request GroupRequest) (*Group, error) {
	group, err := GetGroupById(id)
	if err != nil {
		return nil, err
	}
	if request.Names != nil {
		names := sanitizeGroupNames(request.Names)
		if len(names) == 0 {
			return nil, errors.New("at least one group display name is required")
		}
		group.Names = names
	}
	if request.Ratio != nil {
		if err := validateGroupRatio(*request.Ratio); err != nil {
			return nil, err
		}
		group.Ratio = *request.Ratio
	}
	if err := validateGroupRegion(request.Region); err != nil {
		return nil, err
	}
	group.Region = request.Region
	if request.Enabled != nil {
		group.Enabled = *request.Enabled
	}
	if request.Sort != nil {
		group.Sort = *request.Sort
	}
	if err := DB.Save(group).Error; err != nil {
		return nil, err
	}
	if err := applyGroups(group); err != nil {
		return nil, err
	}
	return group, nil
}

// GroupRenameResult reports how many rows a code rename rewrote.
type GroupRenameResult struct {
	Channels  int `json:"channels"`
	Tokens    int `json:"tokens"`
	Users     int `json:"users"`
	Abilities int `json:"abilities"`
}

// RenameGroupCode changes a group's stable code and rewrites every reference to
// it in the same transaction. This is the only supported way to change a code;
// it exists so the operation either fully succeeds or leaves nothing behind.
func RenameGroupCode(id int, newCode string) (*GroupRenameResult, error) {
	if err := ValidateGroupCode(newCode); err != nil {
		return nil, err
	}
	group, err := GetGroupById(id)
	if err != nil {
		return nil, err
	}
	oldCode := group.Code
	if oldCode == newCode {
		return &GroupRenameResult{}, nil
	}
	alreadyExists, err := groupCodeExists(newCode)
	if err != nil {
		return nil, err
	}
	if alreadyExists {
		return nil, fmt.Errorf("group code %q already exists", newCode)
	}

	result := &GroupRenameResult{}
	err = DB.Transaction(func(tx *gorm.DB) error {
		channels := make([]*Channel, 0)
		if err := tx.Find(&channels).Error; err != nil {
			return err
		}
		for _, channel := range channels {
			updated, changed := replaceGroupCodeInList(channel.Group, oldCode, newCode)
			if !changed {
				continue
			}
			if err := tx.Model(&Channel{}).Where("id = ?", channel.Id).Update("group", updated).Error; err != nil {
				return err
			}
			result.Channels++
		}

		tokens := make([]*Token, 0)
		if err := tx.Find(&tokens).Error; err != nil {
			return err
		}
		for _, token := range tokens {
			updated, changed := replaceGroupCodeInList(token.Group, oldCode, newCode)
			if !changed {
				continue
			}
			if err := tx.Model(&Token{}).Where("id = ?", token.Id).Update("group", updated).Error; err != nil {
				return err
			}
			result.Tokens++
		}

		userUpdate := tx.Model(&User{}).Where(commonGroupCol+" = ?", oldCode).Updates(map[string]any{"group": newCode})
		if userUpdate.Error != nil {
			return userUpdate.Error
		}
		result.Users = int(userUpdate.RowsAffected)

		abilityUpdate := tx.Model(&Ability{}).Where(commonGroupCol+" = ?", oldCode).Updates(map[string]any{"group": newCode})
		if abilityUpdate.Error != nil {
			return abilityUpdate.Error
		}
		result.Abilities = int(abilityUpdate.RowsAffected)

		return tx.Model(&Group{}).Where("id = ?", group.Id).Update("code", newCode).Error
	})
	if err != nil {
		return nil, err
	}

	group.Code = newCode
	if err := renameGroupInSettings(oldCode, newCode, group); err != nil {
		return result, err
	}
	return result, nil
}

// renameGroupInSettings rewrites the option-backed maps that are keyed by group
// name: ratios, usable groups, per-user special overrides, auto groups and
// regions.
func renameGroupInSettings(oldCode, newCode string, group *Group) error {
	ratioSetting := ratio_setting.GetGroupRatioSetting()

	if ratio, exists := ratioSetting.GroupRatio.Get(oldCode); exists {
		ratioSetting.GroupRatio.Delete(oldCode)
		ratioSetting.GroupRatio.Set(newCode, ratio)
	}

	usableGroups := setting.GetUserUsableGroupsCopy()
	if description, exists := usableGroups[oldCode]; exists {
		delete(usableGroups, oldCode)
		usableGroups[newCode] = description
	}
	if err := setting.UpdateUserUsableGroupsByJSONString(mustMarshalString(usableGroups)); err != nil {
		return err
	}

	for userGroup, overrides := range ratioSetting.GroupSpecialUsableGroup.ReadAll() {
		if overrides == nil {
			continue
		}
		updated := make(map[string]string, len(overrides))
		changed := false
		for key, description := range overrides {
			nextKey := key
			if key == oldCode {
				nextKey = newCode
				changed = true
			} else if trimmed, ok := strings.CutPrefix(key, "-:"); ok && trimmed == oldCode {
				nextKey = "-:" + newCode
				changed = true
			} else if trimmed, ok := strings.CutPrefix(key, "+:"); ok && trimmed == oldCode {
				nextKey = "+:" + newCode
				changed = true
			}
			updated[nextKey] = description
		}
		if changed {
			ratioSetting.GroupSpecialUsableGroup.Set(userGroup, updated)
		}
	}

	if autoGroups := setting.GetAutoGroups(); slices.Contains(autoGroups, oldCode) {
		updated := make([]string, 0, len(autoGroups))
		for _, code := range autoGroups {
			if code == oldCode {
				updated = append(updated, newCode)
				continue
			}
			updated = append(updated, code)
		}
		if err := setting.UpdateAutoGroupsByJsonString(mustMarshalStringSlice(updated)); err != nil {
			common.SysLog(common.LogText("error renaming auto groups: %s", err.Error()))
		}
	}

	regionSetting := group_region_setting.GetGroupRegionSetting()
	if region, exists := regionSetting.Groups.Get(oldCode); exists {
		regionSetting.Groups.Delete(oldCode)
		regionSetting.Groups.Set(newCode, region)
	}
	if group != nil {
		regionSetting.Groups.Set(newCode, group.Region)
	}

	if err := UpdateOption("GroupRatio", ratio_setting.GroupRatio2JSONString()); err != nil {
		return err
	}
	if err := UpdateOption(GroupSpecialUsableGroupsOptionKey, ratioSetting.GroupSpecialUsableGroup.MarshalJSONString()); err != nil {
		common.SysLog(common.LogText("error persisting group special usable groups: %s", err.Error()))
	}
	if err := UpdateOption("AutoGroups", setting.AutoGroups2JsonString()); err != nil {
		common.SysLog(common.LogText("error persisting auto groups: %s", err.Error()))
	}
	return persistGroupOptions()
}

func mustMarshalStringSlice(values []string) string {
	encoded, err := common.Marshal(values)
	if err != nil {
		common.SysLog(common.LogText("error marshalling group list: %s", err.Error()))
		return "[]"
	}
	return string(encoded)
}

// GroupSpecialUsableGroupsOptionKey is the option that stores per-user-group
// usable-group overrides (entries prefixed with "+:" or "-:" add or remove one).
const GroupSpecialUsableGroupsOptionKey = "group_ratio_setting.group_special_usable_group"

// mapsKeys is a small helper so slices.Collect can range over map keys without
// allocating an intermediate slice at every call site.
func mapsKeys[V any](source map[string]V) func(func(string) bool) {
	return func(yield func(string) bool) {
		for key := range source {
			if !yield(key) {
				return
			}
		}
	}
}

// countGroupReferences reports whether any channel, token, user or ability still
// references a code.
func countGroupReferences(code string) (int, error) {
	total := 0

	channels := make([]*Channel, 0)
	if err := DB.Find(&channels).Error; err != nil {
		return 0, err
	}
	for _, channel := range channels {
		if slices.Contains(splitGroupCodes(channel.Group), code) {
			total++
		}
	}

	tokens := make([]*Token, 0)
	if err := DB.Find(&tokens).Error; err != nil {
		return 0, err
	}
	for _, token := range tokens {
		if slices.Contains(splitGroupCodes(token.Group), code) {
			total++
		}
	}

	var userCount int64
	if err := DB.Model(&User{}).Where(commonGroupCol+" = ?", code).Count(&userCount).Error; err != nil {
		return 0, err
	}
	total += int(userCount)

	var abilityCount int64
	if err := DB.Model(&Ability{}).Where(commonGroupCol+" = ?", code).Count(&abilityCount).Error; err != nil {
		return 0, err
	}
	return total + int(abilityCount), nil
}

// DeleteGroup removes an unreferenced group. The default group and any group
// still in use are protected so a delete can never strand a channel or a key.
func DeleteGroup(id int) error {
	group, err := GetGroupById(id)
	if err != nil {
		return err
	}
	if group.Code == DefaultGroupCode {
		return fmt.Errorf("the %q group cannot be deleted", DefaultGroupCode)
	}
	references, err := countGroupReferences(group.Code)
	if err != nil {
		return err
	}
	if references > 0 {
		return fmt.Errorf("group %q is still referenced by %d channel, token, user or ability record(s)", group.Code, references)
	}
	if err := DB.Delete(&Group{}, id).Error; err != nil {
		return err
	}

	ratioSetting := ratio_setting.GetGroupRatioSetting()
	ratioSetting.GroupRatio.Delete(group.Code)
	usableGroups := setting.GetUserUsableGroupsCopy()
	delete(usableGroups, group.Code)
	if err := setting.UpdateUserUsableGroupsByJSONString(mustMarshalString(usableGroups)); err != nil {
		return err
	}
	group_region_setting.GetGroupRegionSetting().Groups.Delete(group.Code)
	return persistGroupOptions()
}

// EnsureGroupsSeeded registers a Group row for every group name that already
// exists in option maps or in real channel/token/user/ability data. It is
// idempotent and runs at startup, so an existing installation gains group rows
// without an explicit migration step.
func EnsureGroupsSeeded() (int, error) {
	existing, err := GetGroupCodes()
	if err != nil {
		return 0, err
	}
	known := make(map[string]struct{}, len(existing)+8)
	for _, code := range existing {
		known[code] = struct{}{}
	}

	ratioCopy := ratio_setting.GetGroupRatioCopy()
	usableCopy := setting.GetUserUsableGroupsCopy()
	regions := group_region_setting.GetGroupRegionSetting().Groups.ReadAll()

	pending := make(map[string]struct{}, len(ratioCopy)+len(usableCopy))
	for code := range ratioCopy {
		pending[code] = struct{}{}
	}
	for code := range usableCopy {
		pending[code] = struct{}{}
	}
	pending[DefaultGroupCode] = struct{}{}

	// Include names that only appear in live data, so an old deployment whose
	// option maps were edited by hand is still captured.
	channelGroups := make([]string, 0)
	if err := DB.Model(&Channel{}).Pluck("group", &channelGroups).Error; err == nil {
		for _, raw := range channelGroups {
			for _, code := range splitGroupCodes(raw) {
				pending[code] = struct{}{}
			}
		}
	}
	tokenGroups := make([]string, 0)
	if err := DB.Model(&Token{}).Pluck("group", &tokenGroups).Error; err == nil {
		for _, raw := range tokenGroups {
			for _, code := range splitGroupCodes(raw) {
				pending[code] = struct{}{}
			}
		}
	}
	userGroups := make([]string, 0)
	if err := DB.Model(&User{}).Pluck("group", &userGroups).Error; err == nil {
		for _, raw := range userGroups {
			for _, code := range splitGroupCodes(raw) {
				pending[code] = struct{}{}
			}
		}
	}

	codes := make([]string, 0, len(pending))
	for code := range pending {
		if _, exists := known[code]; exists {
			continue
		}
		if ValidateGroupCode(code) != nil {
			common.SysLog(common.LogText("skipping group with unsupported code: %s", code))
			continue
		}
		codes = append(codes, code)
	}
	slices.Sort(codes)

	created := 0
	for index, code := range codes {
		names := GroupNames{GroupLanguageEn: code}
		if description := strings.TrimSpace(usableCopy[code]); description != "" {
			names[GroupLanguageEn] = description
		}
		ratio := ratioCopy[code]
		group := &Group{
			Code:    code,
			Names:   names,
			Ratio:   ratio,
			Region:  regions[code],
			Enabled: true,
			Sort:    index,
		}
		if err := DB.Create(group).Error; err != nil {
			return created, err
		}
		known[code] = struct{}{}
		created++
	}

	groups, err := GetGroups()
	if err != nil {
		return created, err
	}
	syncGroupSettingsFromGroups(groups)
	return created, nil
}
