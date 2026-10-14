package model

import (
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// userProfileColumnsMigrationTable is the `users` schema under a dedicated test
// table, so the migration can be exercised on a shared database without touching
// the real table.
type userProfileColumnsMigrationTable struct {
	User
}

func (userProfileColumnsMigrationTable) TableName() string {
	return "users_profile_columns_migration_test"
}

// userProfileColumnsMigrationRow reads the migrated row through the columns that
// must survive the upgrade.
type userProfileColumnsMigrationRow struct {
	Username            string
	Quota               int
	Group               string
	RechargeAmount      float64
	OverseasConfirmedAt int64
}

// Upgrade path for the two columns added by the user-level and
// overseas-declaration features. The table starts as the current release creates
// it, loses those columns to look like the previous release, and must then gain
// them back without losing data. A repeated startup migration must not repeat DDL.
func testUserProfileColumnsMigration(t *testing.T, db *gorm.DB, recorder *migrationSQLRecorder) {
	t.Helper()
	const tableName = "users_profile_columns_migration_test"
	t.Cleanup(func() { _ = db.Migrator().DropTable(tableName) })
	require.NoError(t, db.Migrator().DropTable(tableName))

	require.NoError(t, db.AutoMigrate(&userProfileColumnsMigrationTable{}))
	require.NoError(t, db.Table(tableName).Create(&User{
		Id:       1,
		Username: "legacy-user",
		Quota:    100,
		Group:    "vip",
	}).Error)
	// The released table predates the two columns.
	for _, column := range []string{"recharge_amount", "overseas_confirmed_at"} {
		require.NoError(t, db.Exec("ALTER TABLE ? DROP COLUMN ?",
			clause.Table{Name: tableName},
			clause.Column{Name: column},
		).Error)
	}
	require.False(t, db.Migrator().HasColumn(&userProfileColumnsMigrationTable{}, "recharge_amount"))

	require.NoError(t, db.AutoMigrate(&userProfileColumnsMigrationTable{}))
	assert.True(t, db.Migrator().HasColumn(&userProfileColumnsMigrationTable{}, "recharge_amount"))
	assert.True(t, db.Migrator().HasColumn(&userProfileColumnsMigrationTable{}, "overseas_confirmed_at"))

	var upgraded userProfileColumnsMigrationRow
	require.NoError(t, db.Table(tableName).Where("id = ?", 1).Take(&upgraded).Error)
	assert.Equal(t, "legacy-user", upgraded.Username)
	assert.Equal(t, 100, upgraded.Quota)
	assert.Equal(t, "vip", upgraded.Group)
	assert.Zero(t, upgraded.RechargeAmount, "a legacy row must start from the zero default")
	assert.Zero(t, upgraded.OverseasConfirmedAt)

	// The accumulated amount and the declaration must survive a restart
	// migration unchanged.
	require.NoError(t, db.Table(tableName).Where("id = ?", 1).UpdateColumns(map[string]any{
		"recharge_amount":       12.5,
		"overseas_confirmed_at": 1700000000,
	}).Error)
	recorder.reset()
	require.NoError(t, db.AutoMigrate(&userProfileColumnsMigrationTable{}))
	assert.Empty(t, recorder.schemaMutations(), "a second startup migration must not repeat DDL")
	require.NoError(t, db.Table(tableName).Where("id = ?", 1).Take(&upgraded).Error)
	assert.Equal(t, 12.5, upgraded.RechargeAmount)
	assert.Equal(t, int64(1700000000), upgraded.OverseasConfirmedAt)
}

func TestUserProfileColumnsMigrationSQLite(t *testing.T) {
	recorder := &migrationSQLRecorder{}
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{Logger: recorder})
	require.NoError(t, err)
	testUserProfileColumnsMigration(t, db, recorder)
}

func TestUserProfileColumnsMigrationConfiguredDatabases(t *testing.T) {
	tests := []struct {
		name      string
		env       string
		dialector func(string) gorm.Dialector
	}{
		{name: "mysql", env: "TEST_MYSQL_DSN", dialector: func(dsn string) gorm.Dialector { return mysql.Open(dsn) }},
		{name: "postgres", env: "TEST_POSTGRES_DSN", dialector: func(dsn string) gorm.Dialector {
			return postgres.New(postgres.Config{DSN: dsn, PreferSimpleProtocol: true})
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			dsn := strings.TrimSpace(os.Getenv(test.env))
			if dsn == "" {
				t.Skip(test.env + " is not configured")
			}
			recorder := &migrationSQLRecorder{}
			db, err := gorm.Open(test.dialector(dsn), &gorm.Config{Logger: recorder})
			require.NoError(t, err)
			sqlDB, err := db.DB()
			require.NoError(t, err)
			t.Cleanup(func() { _ = sqlDB.Close() })
			testUserProfileColumnsMigration(t, db, recorder)
		})
	}
}
