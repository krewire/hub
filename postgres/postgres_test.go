package postgres_test

import (
	"context"
	"database/sql"
	"fmt"
	"net"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/krewire/hub/postgres"
	"github.com/krewire/krewire/orm"
)

func TestDriverRegistration(t *testing.T) {
	var found bool
	for _, d := range sql.Drivers() {
		if d == postgres.DriverName {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("driver %q was not registered in database/sql", postgres.DriverName)
	}
}

func TestConfigAndDSN(t *testing.T) {
	config := postgres.Config("pg.production.local", 5433, "krew_app", "p@ssword!#", "krewire_iam")
	config.SearchPath = "krewire_auth,public"
	config.ApplicationName = "krewire-auth-server"
	config.ConnectTimeout = 8 * time.Second
	config.StatementTimeout = 25 * time.Second

	dsn := postgres.DSN(config)

	expectedPrefix := "postgres://krew_app:p%40ssword%21%23@pg.production.local:5433/krewire_iam?"
	if !strings.HasPrefix(dsn, expectedPrefix) {
		t.Errorf("expected DSN prefix %s, got %s", expectedPrefix, dsn)
	}

	for _, token := range []string{
		"sslmode=disable",
		"timezone=UTC",
		"search_path=krewire_auth%2Cpublic",
		"application_name=krewire-auth-server",
		"connect_timeout=8",
		"statement_timeout=25000",
	} {
		if !strings.Contains(dsn, token) {
			t.Errorf("DSN missing expected token %q: %s", token, dsn)
		}
	}
}

func TestProductionConfig(t *testing.T) {
	prod := postgres.ProductionConfig("db-primary.internal", 0, "admin", "adm_pass", "krewire_cloud")
	if prod.Port != 5432 {
		t.Errorf("expected default port 5432, got %d", prod.Port)
	}
	if prod.SSLMode != "require" {
		t.Errorf("expected SSLMode require in production, got %s", prod.SSLMode)
	}
	if prod.MaxOpenConns != 50 {
		t.Errorf("expected MaxOpenConns 50, got %d", prod.MaxOpenConns)
	}
	if prod.MaxIdleConns != 15 {
		t.Errorf("expected MaxIdleConns 15, got %d", prod.MaxIdleConns)
	}
	if prod.ConnMaxLifetime != 15*time.Minute {
		t.Errorf("expected ConnMaxLifetime 15m, got %v", prod.ConnMaxLifetime)
	}
}

func TestPostgresDialectCompatibility(t *testing.T) {
	dialect := orm.GetDialect(postgres.DriverName)
	if dialect.Name() != "postgres" {
		t.Errorf("expected dialect postgres, got %s", dialect.Name())
	}
	if quoted := dialect.QuoteIdentifier("accounts"); quoted != `"accounts"` {
		t.Errorf("expected \"accounts\", got %s", quoted)
	}
	if ph1 := dialect.Placeholder(1); ph1 != "$1" {
		t.Errorf("expected $1, got %s", ph1)
	}
	if ph5 := dialect.Placeholder(5); ph5 != "$5" {
		t.Errorf("expected $5, got %s", ph5)
	}
	if pk := dialect.AutoIncrementPrimaryKeyClause(); pk != "BIGSERIAL PRIMARY KEY" {
		t.Errorf("unexpected PK clause: %s", pk)
	}
}

func getLivePostgresDSN() string {
	if dsn := os.Getenv("TEST_POSTGRES_DSN"); dsn != "" {
		return dsn
	}
	// Check if local krewire-db container is reachable on 127.0.0.1:5432
	conn, err := net.DialTimeout("tcp", "127.0.0.1:5432", 500*time.Millisecond)
	if err != nil {
		return ""
	}
	conn.Close()
	return "postgres://postgres:d51aec7bc227224f060d3a205c6b6816@127.0.0.1:5432/krewire_forge?sslmode=disable"
}

func TestPostgres_LiveOperations(t *testing.T) {
	dsn := getLivePostgresDSN()
	if dsn == "" {
		t.Skip("skipping live PostgreSQL test: 127.0.0.1:5432 is not reachable")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	db, err := postgres.Open(dsn)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer db.Close()

	if err := db.Ping(ctx); err != nil {
		t.Fatalf("Ping failed: %v", err)
	}

	// 1. ServerVersion & CurrentDatabase
	version, err := postgres.ServerVersion(ctx, db)
	if err != nil || !strings.Contains(strings.ToLower(version), "postgresql") {
		t.Errorf("expected PostgreSQL version, got %q (err: %v)", version, err)
	}

	curDB, err := postgres.CurrentDatabase(ctx, db)
	if err != nil || curDB != "krewire_forge" {
		t.Errorf("expected database krewire_forge, got %q (err: %v)", curDB, err)
	}

	// 2. SchemaExists
	schemaExists, err := postgres.SchemaExists(ctx, db, "public")
	if err != nil || !schemaExists {
		t.Errorf("expected public schema to exist, got %v (err: %v)", schemaExists, err)
	}
	ghostExists, err := postgres.SchemaExists(ctx, db, "non_existent_schema_xyz")
	if err != nil || ghostExists {
		t.Errorf("non_existent_schema should not exist")
	}

	// 3. Advisory Locks (Session-level & Transaction-level)
	lockKey := int64(888123)
	acquired, err := postgres.TryAdvisoryLock(ctx, db, lockKey)
	if err != nil || !acquired {
		t.Errorf("expected advisory lock acquired: %v, err: %v", acquired, err)
	}
	// Release lock
	released, err := postgres.AdvisoryUnlock(ctx, db, lockKey)
	if err != nil || !released {
		t.Errorf("expected advisory lock released: %v, err: %v", released, err)
	}

	// Transaction-level advisory lock
	err = db.Transaction(ctx, func(tx *orm.Tx) error {
		return postgres.AdvisoryXactLock(ctx, tx, int64(999456))
	})
	if err != nil {
		t.Errorf("AdvisoryXactLock inside transaction failed: %v", err)
	}

	// 4. Pub/Sub Notification (pg_notify)
	if err := postgres.Notify(ctx, db, "krewire_channel", "ping_message"); err != nil {
		t.Errorf("Notify failed: %v", err)
	}

	// 5. Connection Activity & Counts
	stats, err := postgres.ConnectionStats(ctx, db)
	if err != nil || len(stats) == 0 {
		t.Errorf("expected connection stats, got %v (err: %v)", stats, err)
	}
	activeCount, err := postgres.ActiveConnections(ctx, db)
	if err != nil || activeCount <= 0 {
		t.Errorf("expected active connections > 0, got %d (err: %v)", activeCount, err)
	}

	// 6. Table Lifecycle, Introspection & Maintenance
	tableName := "test_pg_mature_ops"
	_, _ = db.Exec(ctx, fmt.Sprintf(`DROP TABLE IF EXISTS "%s"`, tableName))

	_, err = db.Exec(ctx, fmt.Sprintf(`
		CREATE TABLE "%s" (
			id BIGSERIAL PRIMARY KEY,
			title VARCHAR(128) NOT NULL,
			content TEXT DEFAULT 'empty'
		);
		CREATE INDEX "idx_pg_mature_title" ON "%s" (title);
	`, tableName, tableName))
	if err != nil {
		t.Fatalf("create test table failed: %v", err)
	}
	defer func() {
		_, _ = db.Exec(ctx, fmt.Sprintf(`DROP TABLE IF EXISTS "%s"`, tableName))
	}()

	// TableExists
	exists, err := postgres.TableExists(ctx, db, tableName)
	if err != nil || !exists {
		t.Errorf("expected table %s to exist, err: %v", tableName, err)
	}

	// ListTables
	tables, err := postgres.ListTables(ctx, db)
	if err != nil {
		t.Fatalf("ListTables failed: %v", err)
	}
	var foundTable bool
	for _, tName := range tables {
		if tName == tableName {
			foundTable = true
			break
		}
	}
	if !foundTable {
		t.Errorf("expected %s in ListTables, got %v", tableName, tables)
	}

	// ListColumns
	cols, err := postgres.ListColumns(ctx, db, tableName)
	if err != nil {
		t.Fatalf("ListColumns failed: %v", err)
	}
	if len(cols) != 3 {
		t.Fatalf("expected 3 columns, got %d", len(cols))
	}
	if cols[0].Name != "id" || cols[1].Name != "title" || cols[1].IsNullable {
		t.Errorf("unexpected columns: %+v", cols)
	}

	// IndexExists
	idxExists, err := postgres.IndexExists(ctx, db, "idx_pg_mature_title")
	if err != nil || !idxExists {
		t.Errorf("expected index idx_pg_mature_title to exist, err: %v", err)
	}

	// Insert rows
	for i := 1; i <= 20; i++ {
		_, err := db.Exec(ctx, fmt.Sprintf(`INSERT INTO "%s" (title, content) VALUES ($1, $2)`, tableName),
			fmt.Sprintf("Item %d", i), "Some content payload")
		if err != nil {
			t.Fatalf("insert failed: %v", err)
		}
	}

	// TableSize
	size, err := postgres.TableSize(ctx, db, tableName)
	if err != nil || size <= 0 {
		t.Errorf("expected table size > 0, got %d (err: %v)", size, err)
	}

	// VacuumAnalyze
	if err := postgres.VacuumAnalyze(ctx, db, tableName); err != nil {
		t.Errorf("VacuumAnalyze failed: %v", err)
	}

	// TruncateTable
	if err := postgres.TruncateTable(ctx, db, tableName, false); err != nil {
		t.Fatalf("TruncateTable failed: %v", err)
	}

	var rowCount int
	if err := db.QueryRow(ctx, fmt.Sprintf(`SELECT COUNT(*) FROM "%s"`, tableName)).Scan(&rowCount); err != nil {
		t.Fatalf("count failed: %v", err)
	}
	if rowCount != 0 {
		t.Errorf("expected 0 rows after truncate, got %d", rowCount)
	}
}
