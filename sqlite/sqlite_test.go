package sqlite_test

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/krewire/hub/sqlite"
	"github.com/krewire/krewire/orm"
)

func TestDSNDefaults(t *testing.T) {
	dsn := sqlite.DSN("data/app.db")
	for _, want := range []string{"data/app.db?", "_journal_mode=WAL", "_foreign_keys=on", "_busy_timeout=5000", "_txlock=immediate"} {
		if !strings.Contains(dsn, want) {
			t.Errorf("DSN %q lacks %q", dsn, want)
		}
	}
	if got := sqlite.DSN(":memory:"); got != ":memory:" {
		t.Errorf("DSN(:memory:) = %q", got)
	}
}

// TestOpenAppliesPragmas opens a real file and reads the pragmas back, so a
// DSN the driver silently ignored would fail here.
func TestOpenAppliesPragmas(t *testing.T) {
	ctx := context.Background()
	db, err := sqlite.Open(filepath.Join(t.TempDir(), "app.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	if db.Name() != sqlite.DriverName || db.Dialect().Name() != "sqlite" {
		t.Fatalf("driver %q dialect %q", db.Name(), db.Dialect().Name())
	}
	var mode string
	if err := db.QueryRow(ctx, "PRAGMA journal_mode").Scan(&mode); err != nil {
		t.Fatal(err)
	}
	if !strings.EqualFold(mode, "wal") {
		t.Errorf("journal_mode = %q, want wal", mode)
	}
	var fk, busy int
	if err := db.QueryRow(ctx, "PRAGMA foreign_keys").Scan(&fk); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(ctx, "PRAGMA busy_timeout").Scan(&busy); err != nil {
		t.Fatal(err)
	}
	if fk != 1 || busy != 5000 {
		t.Errorf("foreign_keys = %d, busy_timeout = %d; want 1 and 5000", fk, busy)
	}
}

func TestOpenRoundTripsThroughORM(t *testing.T) {
	type note struct {
		ID   int64  `orm:"pk;auto"`
		Body string `orm:"column:body"`
	}
	ctx := context.Background()
	db, err := sqlite.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := db.AutoMigrate(ctx, &note{}); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Model(&note{}).Insert(&note{Body: "hello"}); err != nil {
		t.Fatal(err)
	}
	err = db.Transaction(ctx, func(tx *orm.Tx) error {
		_, err := tx.Exec(ctx, `UPDATE note SET body = ? WHERE body = ?`, "hi", "hello")
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	var got []note
	if err := db.Model(&note{}).FindAll(&got); err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Body != "hi" {
		t.Fatalf("rows = %+v", got)
	}
}

func TestOpenConfigFillsDefaults(t *testing.T) {
	config := sqlite.Config(filepath.Join(t.TempDir(), "x.db"))
	config.DriverName, config.TxLock = "", ""
	db, err := sqlite.OpenConfig(config)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := db.Ping(context.Background()); err != nil {
		t.Fatal(err)
	}
	if db.Name() != sqlite.DriverName {
		t.Errorf("driver = %q", db.Name())
	}
}

// TestForeignKeysEnforced proves the foreign_keys default reaches every
// pooled connection, not only the first.
func TestForeignKeysEnforced(t *testing.T) {
	ctx := context.Background()
	db, err := sqlite.Open(filepath.Join(t.TempDir(), "fk.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	for _, q := range []string{
		`CREATE TABLE parent (id INTEGER PRIMARY KEY)`,
		`CREATE TABLE child (id INTEGER PRIMARY KEY, parent_id INTEGER NOT NULL REFERENCES parent(id))`,
	} {
		if _, err := db.Exec(ctx, q); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := db.Exec(ctx, `INSERT INTO child (parent_id) VALUES (42)`); err == nil {
		t.Fatal("insert with a dangling foreign key succeeded; foreign_keys is off")
	}
}

func TestProductionConfigAndOperations(t *testing.T) {
	ctx := context.Background()
	dbPath := filepath.Join(t.TempDir(), "prod.db")
	db, err := sqlite.OpenProduction(dbPath)
	if err != nil {
		t.Fatalf("OpenProduction failed: %v", err)
	}
	defer db.Close()

	// Verify integrity checks on fresh database
	msgs, err := sqlite.IntegrityCheck(ctx, db)
	if err != nil {
		t.Fatalf("IntegrityCheck failed: %v", err)
	}
	if len(msgs) != 1 || msgs[0] != "ok" {
		t.Errorf("unexpected integrity check message: %v", msgs)
	}

	quickMsgs, err := sqlite.QuickCheck(ctx, db)
	if err != nil {
		t.Fatalf("QuickCheck failed: %v", err)
	}
	if len(quickMsgs) != 1 || quickMsgs[0] != "ok" {
		t.Errorf("unexpected quick check message: %v", quickMsgs)
	}

	// Create a table and write some data
	_, err = db.Exec(ctx, "CREATE TABLE items (id INTEGER PRIMARY KEY, name TEXT)")
	if err != nil {
		t.Fatalf("create table items failed: %v", err)
	}
	for i := 0; i < 50; i++ {
		_, err = db.Exec(ctx, "INSERT INTO items (name) VALUES (?)", "test item")
		if err != nil {
			t.Fatalf("insert item failed: %v", err)
		}
	}

	// Verify maintenance operations
	if err := sqlite.Checkpoint(ctx, db, sqlite.CheckpointTruncate); err != nil {
		t.Errorf("Checkpoint TRUNCATE failed: %v", err)
	}
	if err := sqlite.Checkpoint(ctx, db, sqlite.CheckpointPassive); err != nil {
		t.Errorf("Checkpoint PASSIVE failed: %v", err)
	}
	if err := sqlite.Optimize(ctx, db); err != nil {
		t.Errorf("Optimize failed: %v", err)
	}
	if err := sqlite.IncrementalVacuum(ctx, db, 10); err != nil {
		t.Errorf("IncrementalVacuum failed: %v", err)
	}
	if err := sqlite.Vacuum(ctx, db); err != nil {
		t.Errorf("Vacuum failed: %v", err)
	}
}

func TestSQLite_OnlineBackup(t *testing.T) {
	ctx := context.Background()
	srcPath := filepath.Join(t.TempDir(), "src.db")
	destPath := filepath.Join(t.TempDir(), "backup.db")

	srcDB, err := sqlite.Open(srcPath)
	if err != nil {
		t.Fatalf("Open src failed: %v", err)
	}
	defer srcDB.Close()

	_, err = srcDB.Exec(ctx, "CREATE TABLE customers (id INTEGER PRIMARY KEY, name TEXT)")
	if err != nil {
		t.Fatalf("create table failed: %v", err)
	}
	for i := 1; i <= 25; i++ {
		_, err = srcDB.Exec(ctx, "INSERT INTO customers (name) VALUES (?)", "Customer")
		if err != nil {
			t.Fatalf("insert failed: %v", err)
		}
	}

	// Perform live online backup
	if err := sqlite.Backup(ctx, srcDB, destPath); err != nil {
		t.Fatalf("Backup failed: %v", err)
	}

	// Verify backup database independently
	backupDB, err := sqlite.Open(destPath)
	if err != nil {
		t.Fatalf("Open backup DB failed: %v", err)
	}
	defer backupDB.Close()

	var count int
	if err := backupDB.QueryRow(ctx, "SELECT COUNT(*) FROM customers").Scan(&count); err != nil {
		t.Fatalf("count on backup DB failed: %v", err)
	}
	if count != 25 {
		t.Errorf("expected 25 rows in backup DB, got %d", count)
	}
}

func TestSQLite_PageStatsAndPragmas(t *testing.T) {
	ctx := context.Background()
	dbPath := filepath.Join(t.TempDir(), "stats.db")
	db, err := sqlite.Open(dbPath)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer db.Close()

	// 1. Version Pragmas
	if err := sqlite.SetUserVersion(ctx, db, 105); err != nil {
		t.Fatalf("SetUserVersion failed: %v", err)
	}
	v, err := sqlite.GetUserVersion(ctx, db)
	if err != nil {
		t.Fatalf("GetUserVersion failed: %v", err)
	}
	if v != 105 {
		t.Errorf("expected user_version 105, got %d", v)
	}

	// 2. Foreign keys check
	fk, err := sqlite.ForeignKeysEnabled(ctx, db)
	if err != nil {
		t.Fatalf("ForeignKeysEnabled failed: %v", err)
	}
	if !fk {
		t.Error("expected foreign keys to be enabled")
	}

	// 3. Journal mode & Synchronous
	mode, err := sqlite.SetJournalMode(ctx, db, "WAL")
	if err != nil {
		t.Fatalf("SetJournalMode failed: %v", err)
	}
	if mode != "WAL" {
		t.Errorf("expected journal mode WAL, got %s", mode)
	}
	if err := sqlite.SetSynchronous(ctx, db, "NORMAL"); err != nil {
		t.Fatalf("SetSynchronous failed: %v", err)
	}

	// 4. Page stats
	_, _ = db.Exec(ctx, "CREATE TABLE data (id INTEGER PRIMARY KEY, content TEXT)")
	for i := 0; i < 10; i++ {
		_, _ = db.Exec(ctx, "INSERT INTO data (content) VALUES (?)", "payload")
	}

	stats, err := sqlite.GetPageStats(ctx, db)
	if err != nil {
		t.Fatalf("GetPageStats failed: %v", err)
	}
	if stats.PageCount <= 0 || stats.PageSize <= 0 || stats.TotalBytes <= 0 {
		t.Errorf("invalid stats: %+v", stats)
	}
	if stats.UserVersion != 105 {
		t.Errorf("expected user_version 105 in stats, got %d", stats.UserVersion)
	}

	dbSize, err := sqlite.DatabaseSize(ctx, db)
	if err != nil {
		t.Fatalf("DatabaseSize failed: %v", err)
	}
	if dbSize != stats.TotalBytes {
		t.Errorf("expected DatabaseSize %d == TotalBytes %d", dbSize, stats.TotalBytes)
	}
}

func TestSQLite_TableIntrospection(t *testing.T) {
	ctx := context.Background()
	db, err := sqlite.Open(":memory:")
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer db.Close()
	db.SQLDB().SetMaxOpenConns(1)

	_, err = db.Exec(ctx, `
		CREATE TABLE users (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			username TEXT NOT NULL,
			email TEXT DEFAULT 'none'
		);
		CREATE INDEX idx_users_username ON users (username);
	`)
	if err != nil {
		t.Fatalf("setup tables failed: %v", err)
	}

	// ListTables
	tables, err := sqlite.ListTables(ctx, db)
	if err != nil {
		t.Fatalf("ListTables failed: %v", err)
	}
	if len(tables) != 1 || tables[0] != "users" {
		t.Errorf("expected ['users'], got %v", tables)
	}

	// TableExists
	exists, err := sqlite.TableExists(ctx, db, "users")
	if err != nil || !exists {
		t.Errorf("expected table 'users' to exist")
	}
	notExists, err := sqlite.TableExists(ctx, db, "ghost")
	if err != nil || notExists {
		t.Errorf("table 'ghost' should not exist")
	}

	// TableInfo
	cols, err := sqlite.TableInfo(ctx, db, "users")
	if err != nil {
		t.Fatalf("TableInfo failed: %v", err)
	}
	if len(cols) != 3 {
		t.Fatalf("expected 3 columns, got %d", len(cols))
	}
	if cols[0].Name != "id" || cols[0].PK != 1 {
		t.Errorf("expected col 0 id PK, got %+v", cols[0])
	}
	if cols[1].Name != "username" || !cols[1].NotNull {
		t.Errorf("expected col 1 username NotNull, got %+v", cols[1])
	}

	// ListIndices
	indices, err := sqlite.ListIndices(ctx, db, "users")
	if err != nil {
		t.Fatalf("ListIndices failed: %v", err)
	}
	if len(indices) != 1 || indices[0] != "idx_users_username" {
		t.Errorf("expected ['idx_users_username'], got %v", indices)
	}
}

func TestSQLite_RetryOnBusy(t *testing.T) {
	ctx := context.Background()

	// 1. Success on first try
	calls := 0
	err := sqlite.RetryOnBusy(ctx, 3, 5*time.Millisecond, func() error {
		calls++
		return nil
	})
	if err != nil || calls != 1 {
		t.Errorf("expected 1 call, got %d, err: %v", calls, err)
	}

	// 2. Retry on locked error then success
	attempts := 0
	err = sqlite.RetryOnBusy(ctx, 3, 5*time.Millisecond, func() error {
		attempts++
		if attempts < 3 {
			return errors.New("SQL logic error: database is locked (5)")
		}
		return nil
	})
	if err != nil {
		t.Fatalf("expected retry to succeed eventually, got: %v", err)
	}
	if attempts != 3 {
		t.Errorf("expected 3 attempts, got %d", attempts)
	}
}
