package model

import (
	"fmt"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/glebarez/sqlite"
	gomysql "github.com/go-sql-driver/mysql"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// openTicketClaimTestDB runs each claim case against every supported database.
// MySQL and PostgreSQL get a throwaway database/schema because Ticket overrides
// TableName, so the naming-strategy prefix of other model tests would not
// isolate these tables.
func openTicketClaimTestDB(t *testing.T, dialect string) *gorm.DB {
	t.Helper()
	var dialector gorm.Dialector
	switch dialect {
	case "sqlite":
		dialector = sqlite.Open(":memory:")
	case "mysql":
		dsn := os.Getenv("TEST_MYSQL_DSN")
		if dsn == "" {
			t.Skip("TEST_MYSQL_DSN is not configured")
		}
		config, err := gomysql.ParseDSN(dsn)
		require.NoError(t, err)
		dbName := fmt.Sprintf("ticket_claim_%d", time.Now().UnixNano())
		adminConfig := *config
		adminConfig.DBName = ""
		admin, err := gorm.Open(mysql.Open(adminConfig.FormatDSN()), &gorm.Config{})
		require.NoError(t, err)
		require.NoError(t, admin.Exec("CREATE DATABASE `"+dbName+"` CHARACTER SET utf8mb4").Error)
		t.Cleanup(func() {
			assert.NoError(t, admin.Exec("DROP DATABASE `"+dbName+"`").Error)
			if sqlDB, err := admin.DB(); err == nil {
				_ = sqlDB.Close()
			}
		})
		config.DBName = dbName
		dialector = mysql.Open(config.FormatDSN())
	case "postgres":
		dsn := os.Getenv("TEST_POSTGRES_DSN")
		if dsn == "" {
			t.Skip("TEST_POSTGRES_DSN is not configured")
		}
		schemaName := fmt.Sprintf("ticket_claim_%d", time.Now().UnixNano())
		admin, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
		require.NoError(t, err)
		require.NoError(t, admin.Exec(`CREATE SCHEMA "`+schemaName+`"`).Error)
		t.Cleanup(func() {
			assert.NoError(t, admin.Exec(`DROP SCHEMA "`+schemaName+`" CASCADE`).Error)
			if sqlDB, err := admin.DB(); err == nil {
				_ = sqlDB.Close()
			}
		})
		dialector = postgres.Open(postgresDSNWithSchema(dsn, schemaName))
	default:
		t.Fatalf("unsupported dialect %s", dialect)
	}
	db, err := gorm.Open(dialector, &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)

	previousDB := DB
	DB = db
	t.Cleanup(func() {
		DB = previousDB
		require.NoError(t, sqlDB.Close())
	})

	var version string
	versionQuery := "SELECT version()"
	if dialect == "sqlite" {
		versionQuery = "SELECT sqlite_version()"
	}
	require.NoError(t, db.Raw(versionQuery).Scan(&version).Error)
	t.Logf("database version: %s", version)

	require.NoError(t, db.AutoMigrate(&Ticket{}, &TicketReply{}))
	return db
}

// postgresDSNWithSchema pins the connection search_path to one schema.
func postgresDSNWithSchema(dsn, schema string) string {
	if strings.HasPrefix(dsn, "postgres://") || strings.HasPrefix(dsn, "postgresql://") {
		parsed, err := url.Parse(dsn)
		if err == nil {
			query := parsed.Query()
			query.Set("search_path", schema)
			parsed.RawQuery = query.Encode()
			return parsed.String()
		}
	}
	return dsn + " search_path=" + schema
}

func seedTicketForClaimTest(t *testing.T, db *gorm.DB) *Ticket {
	t.Helper()
	ticket := &Ticket{
		UserId:    7,
		Username:  "alice",
		Subject:   "Billing question",
		Content:   "Why was I charged twice?",
		Status:    TicketStatusOpen,
		CreatedAt: common.GetTimestamp(),
		UpdatedAt: common.GetTimestamp(),
	}
	require.NoError(t, db.Create(ticket).Error)
	return ticket
}

// TestTicketClaimLifecycleDatabaseMatrix protects the ownership rules that
// make the claim meaningful: the pool is claimable once, only the assignee
// handles a claimed ticket, and releasing it returns it to the pool.
func TestTicketClaimLifecycleDatabaseMatrix(t *testing.T) {
	for _, dialect := range []string{"sqlite", "mysql", "postgres"} {
		t.Run(dialect, func(t *testing.T) {
			db := openTicketClaimTestDB(t, dialect)
			ticket := seedTicketForClaimTest(t, db)

			claimed, err := ClaimTicket(ticket.Id, 21, "admin-a")
			require.NoError(t, err)
			assert.Equal(t, 21, claimed.AssigneeId)
			assert.Equal(t, "admin-a", claimed.AssigneeName)
			assert.NotZero(t, claimed.ClaimedAt)

			// A second administrator cannot take the same ticket.
			_, err = ClaimTicket(ticket.Id, 22, "admin-b")
			assert.ErrorIs(t, err, ErrTicketClaimedByOther)

			// Only the assignee may reply to or close the ticket.
			_, err = AddTicketReply(ticket.Id, 22, true, "Handling it")
			assert.ErrorIs(t, err, ErrTicketClaimedByOther)
			_, err = AddTicketReply(ticket.Id, 21, true, "Looking into it")
			require.NoError(t, err)
			assert.ErrorIs(t, CloseTicket(ticket.Id, 22), ErrTicketClaimedByOther)

			// Someone else cannot release the claim.
			assert.ErrorIs(t, UnclaimTicket(ticket.Id, 22, false), ErrTicketClaimedByOther)

			// The assignee releases it; it returns to the pending pool and is
			// claimable again, with the previous handler fields cleared.
			require.NoError(t, UnclaimTicket(ticket.Id, 21, false))
			var released Ticket
			require.NoError(t, db.First(&released, ticket.Id).Error)
			assert.Zero(t, released.AssigneeId)
			assert.Empty(t, released.AssigneeName)
			assert.Zero(t, released.ClaimedAt)

			_, err = ClaimTicket(ticket.Id, 22, "admin-b")
			require.NoError(t, err)
			assert.NoError(t, CloseTicket(ticket.Id, 22))

			// A closed ticket stays closed for further claim attempts.
			_, err = ClaimTicket(ticket.Id, 21, "admin-a")
			assert.ErrorIs(t, err, ErrTicketClosed)
		})
	}
}

// TestTicketClaimRulesDatabaseMatrix covers the unclaimed and root overrides
// so admins cannot silently bypass the queue.
func TestTicketClaimRulesDatabaseMatrix(t *testing.T) {
	for _, dialect := range []string{"sqlite", "mysql", "postgres"} {
		t.Run(dialect, func(t *testing.T) {
			db := openTicketClaimTestDB(t, dialect)
			ticket := seedTicketForClaimTest(t, db)

			// Replying or closing before claiming is rejected.
			_, err := AddTicketReply(ticket.Id, 31, true, "Answer without claiming")
			assert.ErrorIs(t, err, ErrTicketNotClaimed)
			assert.ErrorIs(t, CloseTicket(ticket.Id, 31), ErrTicketNotClaimed)
			assert.ErrorIs(t, UnclaimTicket(ticket.Id, 31, false), ErrTicketNotClaimed)

			// Users may still reply to their own open tickets.
			_, err = AddTicketReply(ticket.Id, 7, false, "More details")
			require.NoError(t, err)

			// A root administrator may release a stale claim.
			_, err = ClaimTicket(ticket.Id, 31, "admin-a")
			require.NoError(t, err)
			require.NoError(t, UnclaimTicket(ticket.Id, 99, true))
			var released Ticket
			require.NoError(t, db.First(&released, ticket.Id).Error)
			assert.Zero(t, released.AssigneeId)

			// Unknown tickets are still reported as missing.
			assert.ErrorIs(t, UnclaimTicket(999_999, 31, false), ErrTicketNotFound)
		})
	}
}

// TestGetAllTicketsAssigneeFilterDatabaseMatrix verifies the admin list
// filters that separate the pending pool from the caller's claimed tickets.
func TestGetAllTicketsAssigneeFilterDatabaseMatrix(t *testing.T) {
	for _, dialect := range []string{"sqlite", "mysql", "postgres"} {
		t.Run(dialect, func(t *testing.T) {
			db := openTicketClaimTestDB(t, dialect)
			pending := seedTicketForClaimTest(t, db)
			claimed := seedTicketForClaimTest(t, db)
			_, err := ClaimTicket(claimed.Id, 41, "admin-a")
			require.NoError(t, err)

			tickets, total, err := GetAllTickets(TicketListFilter{Assignee: TicketAssigneePending}, 1, 20)
			require.NoError(t, err)
			assert.EqualValues(t, 1, total)
			require.Len(t, tickets, 1)
			assert.Equal(t, pending.Id, tickets[0].Id)

			tickets, total, err = GetAllTickets(TicketListFilter{Assignee: TicketAssigneeMine, UserId: 41}, 1, 20)
			require.NoError(t, err)
			assert.EqualValues(t, 1, total)
			require.Len(t, tickets, 1)
			assert.Equal(t, claimed.Id, tickets[0].Id)

			_, total, err = GetAllTickets(TicketListFilter{Assignee: TicketAssigneeMine, UserId: 42}, 1, 20)
			require.NoError(t, err)
			assert.Zero(t, total)
		})
	}
}

// legacyTicketForClaimTest is the tickets table as released before the claim
// columns existed. AutoMigrate must upgrade it in place without losing rows.
type legacyTicketForClaimTest struct {
	Id        int64  `gorm:"primaryKey;autoIncrement"`
	UserId    int    `gorm:"not null;index"`
	Username  string `gorm:"type:varchar(64);not null;default:''"`
	Subject   string `gorm:"type:varchar(255);not null"`
	Content   string `gorm:"type:text;not null"`
	Status    string `gorm:"type:varchar(16);not null;default:'open';index"`
	CreatedAt int64  `gorm:"bigint"`
	UpdatedAt int64  `gorm:"bigint"`
}

func (legacyTicketForClaimTest) TableName() string {
	return "tickets"
}

// TestTicketClaimSchemaUpgradeDatabaseMatrix upgrades a pre-claim tickets table
// twice to prove idempotency, then checks existing rows survive and become
// claimable.
func TestTicketClaimSchemaUpgradeDatabaseMatrix(t *testing.T) {
	for _, dialect := range []string{"sqlite", "mysql", "postgres"} {
		t.Run(dialect, func(t *testing.T) {
			db := openTicketClaimTestDB(t, dialect)
			require.NoError(t, db.Migrator().DropTable(&Ticket{}, &TicketReply{}))
			require.NoError(t, db.AutoMigrate(&legacyTicketForClaimTest{}))
			legacy := &legacyTicketForClaimTest{
				UserId:    7,
				Username:  "alice",
				Subject:   "Old ticket",
				Content:   "Created before the claim columns existed",
				Status:    "open",
				CreatedAt: 1756400000,
				UpdatedAt: 1756400000,
			}
			require.NoError(t, db.Create(legacy).Error)

			for range 2 {
				require.NoError(t, db.AutoMigrate(&Ticket{}, &TicketReply{}))
			}

			var upgraded Ticket
			require.NoError(t, db.First(&upgraded, legacy.Id).Error)
			assert.Equal(t, "Old ticket", upgraded.Subject)
			assert.Zero(t, upgraded.AssigneeId)
			assert.Empty(t, upgraded.AssigneeName)
			assert.Zero(t, upgraded.ClaimedAt)

			_, err := ClaimTicket(upgraded.Id, 51, "admin-a")
			require.NoError(t, err)
			var claimed Ticket
			require.NoError(t, db.First(&claimed, upgraded.Id).Error)
			assert.Equal(t, 51, claimed.AssigneeId)
			assert.Equal(t, "admin-a", claimed.AssigneeName)
		})
	}
}
