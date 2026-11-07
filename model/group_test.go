package model

import (
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func groupTestPtr[T any](value T) *T {
	return &value
}

// groupTestDialects returns every database this test can reach. SQLite always
// runs; MySQL and PostgreSQL join when their DSN is configured, which is how the
// repository's other cross-database model tests select engines.
func groupTestDialects(t *testing.T) []struct {
	name     string
	database common.DatabaseType
	newDB    func() (*gorm.DB, error)
} {
	t.Helper()
	quiet := &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)}
	dialects := []struct {
		name     string
		database common.DatabaseType
		newDB    func() (*gorm.DB, error)
	}{
		{
			name:     "sqlite",
			database: common.DatabaseTypeSQLite,
			newDB: func() (*gorm.DB, error) {
				return gorm.Open(sqlite.Open(":memory:"), quiet)
			},
		},
	}
	if dsn := strings.TrimSpace(os.Getenv("TEST_MYSQL_DSN")); dsn != "" {
		dialects = append(dialects, struct {
			name     string
			database common.DatabaseType
			newDB    func() (*gorm.DB, error)
		}{
			name:     "mysql",
			database: common.DatabaseTypeMySQL,
			newDB: func() (*gorm.DB, error) {
				return gorm.Open(mysql.Open(dsn), quiet)
			},
		})
	}
	if dsn := strings.TrimSpace(os.Getenv("TEST_POSTGRES_DSN")); dsn != "" {
		dialects = append(dialects, struct {
			name     string
			database common.DatabaseType
			newDB    func() (*gorm.DB, error)
		}{
			name:     "postgres",
			database: common.DatabaseTypePostgreSQL,
			newDB: func() (*gorm.DB, error) {
				return gorm.Open(postgres.New(postgres.Config{DSN: dsn, PreferSimpleProtocol: true}), quiet)
			},
		})
	}
	return dialects
}

// withGroupTestDB installs a database as the package global for the duration of
// one subtest, migrates the tables a group lifecycle touches, and clears them so
// the engines cannot influence each other.
func withGroupTestDB(t *testing.T, database common.DatabaseType, open func() (*gorm.DB, error)) {
	t.Helper()
	db, err := open()
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	// SQLite in-memory needs a single connection or each query gets a fresh,
	// empty database.
	sqlDB.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = sqlDB.Close() })

	previousDB := DB
	previousLogDB := LOG_DB
	DB = db
	LOG_DB = db
	common.SetDatabaseTypes(database, database)
	// The quoting of the reserved "group" column depends on the dialect.
	initCol()
	t.Cleanup(func() {
		DB = previousDB
		LOG_DB = previousLogDB
		common.SetDatabaseTypes(common.DatabaseTypeSQLite, common.DatabaseTypeSQLite)
		initCol()
	})

	if common.OptionMap == nil {
		common.OptionMap = make(map[string]string)
	}

	require.NoError(t, db.AutoMigrate(&Group{}, &Option{}, &Channel{}, &Token{}, &User{}, &Ability{}))
	for _, table := range []string{"groups", "channels", "tokens", "abilities", "users", "options"} {
		require.NoError(t, db.Exec(fmt.Sprintf("DELETE FROM %s", quoteGroupTestTable(database, table))).Error)
	}
}

// quoteGroupTestTable quotes a table name for the active dialect. "groups" is a
// reserved word on recent PostgreSQL and MySQL releases, so the cleanup delete
// cannot rely on the unquoted name.
func quoteGroupTestTable(database common.DatabaseType, table string) string {
	if database == common.DatabaseTypePostgreSQL {
		return `"` + table + `"`
	}
	return "`" + table + "`"
}

func TestValidateGroupCode(t *testing.T) {
	// Codes are opaque: real deployments encode price tiers, languages and
	// brackets into them, so all of those must be accepted.
	for _, code := range []string{
		"default", "vip", "gpt-4o", "team_a", "A1",
		"ccmax0.9", "管key0.25", "plus稳定0.08", "Gemini Ultra 0.3", "[用户]测试111",
	} {
		assert.NoError(t, ValidateGroupCode(code), "code %q should be accepted", code)
	}

	for _, code := range []string{"", "a,b", " lead", "trail ", strings.Repeat("x", 65)} {
		assert.Error(t, ValidateGroupCode(code), "code %q should be rejected", code)
	}
}

func TestGroupDisplayNameFallback(t *testing.T) {
	group := &Group{
		Code: "vip",
		Names: GroupNames{
			GroupLanguageEn: "VIP",
			GroupLanguageZh: "会员",
		},
	}

	assert.Equal(t, "VIP", group.DisplayName("en"))
	assert.Equal(t, "会员", group.DisplayName("zh"))
	// A regional or script variant collapses onto its base language.
	assert.Equal(t, "会员", group.DisplayName("zh-CN"))
	assert.Equal(t, "会员", group.DisplayName("zh-Hans"))
	// A missing translation falls back to English rather than the raw code.
	assert.Equal(t, "VIP", group.DisplayName("ja"))

	noEnglish := &Group{Code: "x", Names: GroupNames{GroupLanguageZh: "中文名"}}
	assert.Equal(t, "中文名", noEnglish.DisplayName("fr"), "should fall back to any translation")

	noNames := &Group{Code: "bare"}
	assert.Equal(t, "bare", noNames.DisplayName("en"), "should fall back to the code")
}

func TestSanitizeGroupNames(t *testing.T) {
	names := sanitizeGroupNames(map[string]string{
		"en":    "  Default group  ",
		"zh-CN": "默认分组",
		"zh-TW": "預設分組",
		"ja":    "   ",
	})

	assert.Equal(t, "Default group", names[GroupLanguageEn], "values must be trimmed")
	assert.Equal(t, "默认分组", names[GroupLanguageZh], "zh-CN must collapse onto zh")
	assert.Equal(t, "預設分組", names[GroupLanguageZhTW])
	_, hasJapanese := names[GroupLanguageJa]
	assert.False(t, hasJapanese, "a whitespace-only name must be dropped")
}

func TestReplaceGroupCodeInList(t *testing.T) {
	updated, changed := replaceGroupCodeInList("default,vip,team", "vip", "premium")
	assert.True(t, changed)
	assert.Equal(t, "default,premium,team", updated)

	unchanged, changed := replaceGroupCodeInList("default,team", "vip", "premium")
	assert.False(t, changed)
	assert.Equal(t, "default,team", unchanged, "an unrelated list must be returned untouched")

	// A code that only appears as a substring must not be rewritten.
	guarded, changed := replaceGroupCodeInList("vip-plus,default", "vip", "premium")
	assert.False(t, changed)
	assert.Equal(t, "vip-plus,default", guarded)
}

// TestGroupRenameKeepsReferences is the regression test for the reported bug:
// renaming a group broke every channel and key that referenced it. A display
// name change must leave references untouched, and a code change must migrate
// all of them in one step.
func TestGroupRenameKeepsReferences(t *testing.T) {
	for _, dialect := range groupTestDialects(t) {
		t.Run(dialect.name, func(t *testing.T) {
			withGroupTestDB(t, dialect.database, dialect.newDB)

			group, err := CreateGroup(GroupRequest{
				Code:  "vip",
				Names: map[string]string{GroupLanguageEn: "VIP", GroupLanguageZh: "会员"},
				Ratio: groupTestPtr(0.9),
			})
			require.NoError(t, err)
			require.NotZero(t, group.Id)

			require.NoError(t, DB.Create(&Channel{Id: 1, Name: "channel-a", Key: "key-a", Group: "default,vip", Models: "gpt-4o"}).Error)
			require.NoError(t, DB.Create(&Token{Id: 1, UserId: 1, Name: "token-a", Key: "token-key", Group: "vip"}).Error)
			require.NoError(t, DB.Create(&User{Id: 1, Username: "user-a", Password: "x", Group: "vip"}).Error)
			require.NoError(t, DB.Create(&Ability{Group: "vip", Model: "gpt-4o", ChannelId: 1, Enabled: true}).Error)

			// 1. A display-name rename must not disturb any reference.
			updated, err := UpdateGroup(group.Id, GroupRequest{
				Names: map[string]string{GroupLanguageEn: "Premium", GroupLanguageZh: "高级会员"},
			})
			require.NoError(t, err)
			assert.Equal(t, "vip", updated.Code, "editing names must not change the code")
			assert.Equal(t, "Premium", updated.DisplayName("en"))

			var channel Channel
			require.NoError(t, DB.First(&channel, 1).Error)
			assert.Equal(t, "default,vip", channel.Group)

			var token Token
			require.NoError(t, DB.First(&token, 1).Error)
			assert.Equal(t, "vip", token.Group)

			// 2. Renaming the code must migrate every reference.
			result, err := RenameGroupCode(group.Id, "premium")
			require.NoError(t, err)
			assert.Equal(t, 1, result.Channels)
			assert.Equal(t, 1, result.Tokens)
			assert.Equal(t, 1, result.Users)
			assert.Equal(t, 1, result.Abilities)

			require.NoError(t, DB.First(&channel, 1).Error)
			assert.Equal(t, "default,premium", channel.Group)
			require.NoError(t, DB.First(&token, 1).Error)
			assert.Equal(t, "premium", token.Group)

			var user User
			require.NoError(t, DB.First(&user, 1).Error)
			assert.Equal(t, "premium", user.Group)

			var ability Ability
			require.NoError(t, DB.First(&ability).Error)
			assert.Equal(t, "premium", ability.Group)

			reloaded, err := GetGroupById(group.Id)
			require.NoError(t, err)
			assert.Equal(t, "premium", reloaded.Code)
			assert.Equal(t, "Premium", reloaded.DisplayName("en"), "the display name must survive a code rename")
		})
	}
}

func TestDeleteGroupProtectsReferencedGroups(t *testing.T) {
	for _, dialect := range groupTestDialects(t) {
		t.Run(dialect.name, func(t *testing.T) {
			withGroupTestDB(t, dialect.database, dialect.newDB)

			defaultGroup, err := CreateGroup(GroupRequest{
				Code:  DefaultGroupCode,
				Names: map[string]string{GroupLanguageEn: "Default"},
			})
			require.NoError(t, err)
			require.Error(t, DeleteGroup(defaultGroup.Id), "the default group must not be deletable")

			used, err := CreateGroup(GroupRequest{
				Code:  "used-group",
				Names: map[string]string{GroupLanguageEn: "Used"},
			})
			require.NoError(t, err)
			require.NoError(t, DB.Create(&Channel{Id: 1, Name: "c", Key: "k", Group: "used-group"}).Error)
			require.Error(t, DeleteGroup(used.Id), "a referenced group must not be deletable")

			unused, err := CreateGroup(GroupRequest{
				Code:  "unused-group",
				Names: map[string]string{GroupLanguageEn: "Unused"},
			})
			require.NoError(t, err)
			require.NoError(t, DeleteGroup(unused.Id))
			_, err = GetGroupById(unused.Id)
			require.Error(t, err, "the row must be gone")
		})
	}
}

// TestEnsureGroupsSeededIsIdempotent covers the upgrade path: an existing
// installation already has group names in option maps and live data but no group
// rows. Seeding must register exactly those, and a second startup must be a
// no-op.
func TestEnsureGroupsSeededIsIdempotent(t *testing.T) {
	for _, dialect := range groupTestDialects(t) {
		t.Run(dialect.name, func(t *testing.T) {
			withGroupTestDB(t, dialect.database, dialect.newDB)

			require.NoError(t, DB.Create(&Channel{Id: 1, Name: "c", Key: "k", Group: "default,legacy-team,ccmax0.9,接码"}).Error)
			require.NoError(t, DB.Create(&Token{Id: 1, UserId: 1, Name: "t", Key: "tk", Group: "token-only"}).Error)

			created, err := EnsureGroupsSeeded()
			require.NoError(t, err)
			assert.Positive(t, created, "seeding must register the names already in use")

			codes, err := GetGroupCodes()
			require.NoError(t, err)
			assert.Contains(t, codes, DefaultGroupCode)
			assert.Contains(t, codes, "legacy-team", "a channel-only name must be captured")
			assert.Contains(t, codes, "token-only", "a token-only name must be captured")
			// Existing deployments encode price tiers and languages into group
			// names; those must be adopted rather than skipped.
			assert.Contains(t, codes, "ccmax0.9")
			assert.Contains(t, codes, "接码")

			again, err := EnsureGroupsSeeded()
			require.NoError(t, err)
			assert.Zero(t, again, "a repeated startup must not insert duplicate rows")
		})
	}
}

func TestCreateGroupRejectsDuplicatesAndBadInput(t *testing.T) {
	for _, dialect := range groupTestDialects(t) {
		t.Run(dialect.name, func(t *testing.T) {
			withGroupTestDB(t, dialect.database, dialect.newDB)

			_, err := CreateGroup(GroupRequest{Code: "vip", Names: map[string]string{GroupLanguageEn: "VIP"}})
			require.NoError(t, err)

			_, err = CreateGroup(GroupRequest{Code: "vip", Names: map[string]string{GroupLanguageEn: "VIP again"}})
			require.Error(t, err, "a duplicate code must be rejected")

			_, err = CreateGroup(GroupRequest{Code: "no-names"})
			require.Error(t, err, "a group without a display name must be rejected")

			_, err = CreateGroup(GroupRequest{
				Code:  "bad-ratio",
				Names: map[string]string{GroupLanguageEn: "Bad"},
				Ratio: groupTestPtr(-1.0),
			})
			require.Error(t, err, "a negative ratio must be rejected")

			_, err = CreateGroup(GroupRequest{
				Code:   "bad-region",
				Names:  map[string]string{GroupLanguageEn: "Bad"},
				Region: "mars",
			})
			require.Error(t, err, "an unknown region must be rejected")
		})
	}
}
