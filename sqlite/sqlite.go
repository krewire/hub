// Package sqlite bundles a pure-Go SQLite driver for pkg/orm.
//
// Importing this package registers modernc.org/sqlite under the driver name
// "sqlite" (no cgo, so CGO_ENABLED=0 cross-compiles keep working) and offers
// Open and DSN helpers with production-friendly defaults:
//
//   - journal_mode=WAL: readers do not block the writer;
//   - foreign_keys=on: REFERENCES clauses are enforced;
//   - busy_timeout=5000: a locked database is retried for 5s instead of
//     failing at once with "database is locked";
//   - _txlock=immediate: BEGIN takes the write lock up front, so busy_timeout
//     also covers transactions that read before they write;
//   - synchronous=NORMAL: zero data loss in WAL mode while maximizing disk IOPS;
//   - cache_size=-64000: 64 MB thread-safe in-memory page cache;
//   - temp_store=MEMORY: transient sorting and indices reside in memory.
//
// The driver is opt-in: pkg/orm itself never imports it, so programs that use
// another database do not link SQLite.
package sqlite

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/krewire/krewire/errs"
	"github.com/krewire/krewire/orm"

	_ "modernc.org/sqlite" // registers the "sqlite" database/sql driver
)

// DriverName is the database/sql driver name registered by this package.
const DriverName = "sqlite"

// DefaultTxLock is the BEGIN mode Config applies when TxLock is empty.
const DefaultTxLock = "immediate"

// CheckpointMode defines the WAL checkpoint mode.
type CheckpointMode string

const (
	// CheckpointPassive checkpoints as many frames as possible without blocking.
	CheckpointPassive CheckpointMode = "PASSIVE"

	// CheckpointFull blocks until all current readers finish and checkpoints all frames.
	CheckpointFull CheckpointMode = "FULL"

	// CheckpointRestart blocks like FULL and ensures subsequent writers start at frame 1.
	CheckpointRestart CheckpointMode = "RESTART"

	// CheckpointTruncate blocks, checkpoints everything, and truncates the WAL file to 0 bytes.
	CheckpointTruncate CheckpointMode = "TRUNCATE"
)

// Config returns the orm.SQLiteConfig for path with standard safe defaults:
// the bundled driver, WAL, foreign keys on, a 5s busy timeout, and immediate
// transactions. Adjust the returned value before passing it to OpenConfig.
// A path of "" or ":memory:" selects a private in-memory database.
func Config(path string) orm.SQLiteConfig {
	return orm.SQLiteConfig{
		Path:            path,
		JournalMode:     "WAL",
		ForeignKeys:     true,
		DriverName:      DriverName,
		TxLock:          DefaultTxLock,
		BusyTimeout:     5 * time.Second,
		Synchronous:     "NORMAL",
		MaxOpenConns:    10,
		MaxIdleConns:    5,
		ConnMaxLifetime: 10 * time.Minute,
		ConnMaxIdleTime: 2 * time.Minute,
	}
}

// ProductionConfig returns an enterprise-tuned orm.SQLiteConfig:
// WAL mode, synchronous NORMAL, 64MB memory page cache, in-memory temp tables,
// 256MB memory-mapped I/O, and incremental auto-vacuum.
func ProductionConfig(path string) orm.SQLiteConfig {
	config := Config(path)
	config.CacheSize = -64000           // 64 MB page cache
	config.TempStore = "MEMORY"         // memory temp store
	config.MmapSize = 256 * 1024 * 1024 // 256 MB mmap
	config.AutoVacuum = "INCREMENTAL"
	return config
}

// DSN returns the connection string for path with the defaults of Config,
// for use with orm.Open(sqlite.DriverName, dsn) or database/sql directly.
func DSN(path string) string {
	return Config(path).FormatDSN()
}

// Open opens (creating it when missing) the SQLite database at path with the
// defaults of Config.
func Open(path string) (*orm.DB, error) {
	return OpenConfig(Config(path))
}

// OpenProduction opens a SQLite database optimized for heavy production workloads.
func OpenProduction(path string) (*orm.DB, error) {
	return OpenConfig(ProductionConfig(path))
}

// OpenConfig opens a SQLite database from config. An empty DriverName selects the
// bundled driver; an empty TxLock selects DefaultTxLock.
func OpenConfig(config orm.SQLiteConfig) (*orm.DB, error) {
	if strings.TrimSpace(config.DriverName) == "" {
		config.DriverName = DriverName
	}
	if strings.TrimSpace(config.TxLock) == "" {
		config.TxLock = DefaultTxLock
	}
	return orm.NewSQLite(config)
}

// Checkpoint executes a PRAGMA wal_checkpoint command to flush WAL frames back to the main DB.
func Checkpoint(ctx context.Context, db *orm.DB, mode CheckpointMode) error {
	if db == nil {
		return errs.UsageError("sqlite: db connection must not be nil")
	}
	if mode == "" {
		mode = CheckpointPassive
	}
	query := fmt.Sprintf("PRAGMA wal_checkpoint(%s)", mode)
	_, err := db.Exec(ctx, query)
	if err != nil {
		return errs.FailureError(fmt.Sprintf("sqlite checkpoint (%s) failed: %v", mode, err))
	}
	return nil
}

// Vacuum executes the VACUUM command to defragment and optimize the database file.
func Vacuum(ctx context.Context, db *orm.DB) error {
	if db == nil {
		return errs.UsageError("sqlite: db connection must not be nil")
	}
	_, err := db.Exec(ctx, "VACUUM")
	if err != nil {
		return errs.FailureError(fmt.Sprintf("sqlite vacuum failed: %v", err))
	}
	return nil
}

// IncrementalVacuum frees unused database pages back to the filesystem.
func IncrementalVacuum(ctx context.Context, db *orm.DB, pages int) error {
	if db == nil {
		return errs.UsageError("sqlite: db connection must not be nil")
	}
	var query string
	if pages > 0 {
		query = fmt.Sprintf("PRAGMA incremental_vacuum(%d)", pages)
	} else {
		query = "PRAGMA incremental_vacuum"
	}
	_, err := db.Exec(ctx, query)
	if err != nil {
		return errs.FailureError(fmt.Sprintf("sqlite incremental vacuum failed: %v", err))
	}
	return nil
}

// Optimize executes PRAGMA optimize to refresh query planner statistics.
func Optimize(ctx context.Context, db *orm.DB) error {
	if db == nil {
		return errs.UsageError("sqlite: db connection must not be nil")
	}
	_, err := db.Exec(ctx, "PRAGMA optimize")
	if err != nil {
		return errs.FailureError(fmt.Sprintf("sqlite optimize failed: %v", err))
	}
	return nil
}

// IntegrityCheck runs PRAGMA integrity_check and returns any diagnostics or errors.
func IntegrityCheck(ctx context.Context, db *orm.DB) ([]string, error) {
	if db == nil {
		return nil, errs.UsageError("sqlite: db connection must not be nil")
	}
	rows, err := db.Query(ctx, "PRAGMA integrity_check")
	if err != nil {
		return nil, errs.FailureError(fmt.Sprintf("sqlite integrity check query failed: %v", err))
	}
	defer rows.Close()

	var messages []string
	for rows.Next() {
		var msg string
		if err := rows.Scan(&msg); err != nil {
			return nil, errs.FailureError(fmt.Sprintf("sqlite scan integrity error: %v", err))
		}
		messages = append(messages, msg)
	}

	if len(messages) == 1 && messages[0] == "ok" {
		return messages, nil
	}
	return messages, errs.FailureError(fmt.Sprintf("sqlite integrity issues detected: %s", strings.Join(messages, "; ")))
}

// QuickCheck runs a faster integrity scan (skipping index consistency verify).
func QuickCheck(ctx context.Context, db *orm.DB) ([]string, error) {
	if db == nil {
		return nil, errs.UsageError("sqlite: db connection must not be nil")
	}
	rows, err := db.Query(ctx, "PRAGMA quick_check")
	if err != nil {
		return nil, errs.FailureError(fmt.Sprintf("sqlite quick check query failed: %v", err))
	}
	defer rows.Close()

	var messages []string
	for rows.Next() {
		var msg string
		if err := rows.Scan(&msg); err != nil {
			return nil, errs.FailureError(fmt.Sprintf("sqlite scan quick check error: %v", err))
		}
		messages = append(messages, msg)
	}

	if len(messages) == 1 && messages[0] == "ok" {
		return messages, nil
	}
	return messages, errs.FailureError(fmt.Sprintf("sqlite quick check issues detected: %s", strings.Join(messages, "; ")))
}

// PageStats holds memory and page allocation metrics for a SQLite database.
type PageStats struct {
	PageCount     int64 `json:"page_count"`
	PageSize      int64 `json:"page_size"`
	TotalBytes    int64 `json:"total_bytes"`
	FreeListPages int64 `json:"freelist_pages"`
	UserVersion   int   `json:"user_version"`
	SchemaVersion int   `json:"schema_version"`
}

// GetPageStats queries page metrics and storage allocations from SQLite PRAGMAs.
func GetPageStats(ctx context.Context, db *orm.DB) (*PageStats, error) {
	if db == nil {
		return nil, errs.UsageError("sqlite: db connection must not be nil")
	}

	stats := &PageStats{}
	if err := db.QueryRow(ctx, "PRAGMA page_count").Scan(&stats.PageCount); err != nil {
		return nil, errs.FailureError(fmt.Sprintf("sqlite page_count failed: %v", err))
	}
	if err := db.QueryRow(ctx, "PRAGMA page_size").Scan(&stats.PageSize); err != nil {
		return nil, errs.FailureError(fmt.Sprintf("sqlite page_size failed: %v", err))
	}
	stats.TotalBytes = stats.PageCount * stats.PageSize

	if err := db.QueryRow(ctx, "PRAGMA freelist_count").Scan(&stats.FreeListPages); err != nil {
		return nil, errs.FailureError(fmt.Sprintf("sqlite freelist_count failed: %v", err))
	}
	if err := db.QueryRow(ctx, "PRAGMA user_version").Scan(&stats.UserVersion); err != nil {
		return nil, errs.FailureError(fmt.Sprintf("sqlite user_version failed: %v", err))
	}
	if err := db.QueryRow(ctx, "PRAGMA schema_version").Scan(&stats.SchemaVersion); err != nil {
		return nil, errs.FailureError(fmt.Sprintf("sqlite schema_version failed: %v", err))
	}

	return stats, nil
}

// DatabaseSize computes the total allocated database size in bytes.
func DatabaseSize(ctx context.Context, db *orm.DB) (int64, error) {
	stats, err := GetPageStats(ctx, db)
	if err != nil {
		return 0, err
	}
	return stats.TotalBytes, nil
}

// GetUserVersion returns the integer user_version stored in the database header.
func GetUserVersion(ctx context.Context, db *orm.DB) (int, error) {
	if db == nil {
		return 0, errs.UsageError("sqlite: db connection must not be nil")
	}
	var version int
	if err := db.QueryRow(ctx, "PRAGMA user_version").Scan(&version); err != nil {
		return 0, errs.FailureError(fmt.Sprintf("sqlite get user_version failed: %v", err))
	}
	return version, nil
}

// SetUserVersion updates the integer user_version stored in the database header.
func SetUserVersion(ctx context.Context, db *orm.DB, version int) error {
	if db == nil {
		return errs.UsageError("sqlite: db connection must not be nil")
	}
	query := fmt.Sprintf("PRAGMA user_version = %d", version)
	if _, err := db.Exec(ctx, query); err != nil {
		return errs.FailureError(fmt.Sprintf("sqlite set user_version failed: %v", err))
	}
	return nil
}

// ForeignKeysEnabled checks if foreign key constraints are currently enforced.
func ForeignKeysEnabled(ctx context.Context, db *orm.DB) (bool, error) {
	if db == nil {
		return false, errs.UsageError("sqlite: db connection must not be nil")
	}
	var fk int
	if err := db.QueryRow(ctx, "PRAGMA foreign_keys").Scan(&fk); err != nil {
		return false, errs.FailureError(fmt.Sprintf("sqlite check foreign_keys failed: %v", err))
	}
	return fk == 1, nil
}

// SetJournalMode changes the active journal mode and returns the mode configured by the engine.
func SetJournalMode(ctx context.Context, db *orm.DB, mode string) (string, error) {
	if db == nil {
		return "", errs.UsageError("sqlite: db connection must not be nil")
	}
	query := fmt.Sprintf("PRAGMA journal_mode = %s", strings.ToUpper(strings.TrimSpace(mode)))
	var result string
	if err := db.QueryRow(ctx, query).Scan(&result); err != nil {
		return "", errs.FailureError(fmt.Sprintf("sqlite set journal_mode failed: %v", err))
	}
	return strings.ToUpper(result), nil
}

// SetSynchronous sets the PRAGMA synchronous flag (e.g. "NORMAL", "FULL", "OFF").
func SetSynchronous(ctx context.Context, db *orm.DB, mode string) error {
	if db == nil {
		return errs.UsageError("sqlite: db connection must not be nil")
	}
	query := fmt.Sprintf("PRAGMA synchronous = %s", strings.ToUpper(strings.TrimSpace(mode)))
	if _, err := db.Exec(ctx, query); err != nil {
		return errs.FailureError(fmt.Sprintf("sqlite set synchronous failed: %v", err))
	}
	return nil
}

// Backup creates a clean online hot copy of the database at destPath without blocking concurrent reads/writes.
func Backup(ctx context.Context, srcDB *orm.DB, destPath string) error {
	if srcDB == nil {
		return errs.UsageError("sqlite: srcDB must not be nil")
	}
	if strings.TrimSpace(destPath) == "" {
		return errs.UsageError("sqlite: destPath must not be empty")
	}
	escaped := strings.ReplaceAll(destPath, `'`, `''`)
	query := fmt.Sprintf("VACUUM INTO '%s'", escaped)
	if _, err := srcDB.Exec(ctx, query); err != nil {
		return errs.FailureError(fmt.Sprintf("sqlite backup (VACUUM INTO) failed: %v", err))
	}
	return nil
}

// ColumnInfo holds introspection details for a table column.
type ColumnInfo struct {
	CID          int     `json:"cid"`
	Name         string  `json:"name"`
	Type         string  `json:"type"`
	NotNull      bool    `json:"not_null"`
	DefaultValue *string `json:"default_value"`
	PK           int     `json:"pk"`
}

// ListTables returns all user-defined table names in the database.
func ListTables(ctx context.Context, db *orm.DB) ([]string, error) {
	if db == nil {
		return nil, errs.UsageError("sqlite: db connection must not be nil")
	}
	query := `SELECT name FROM sqlite_master WHERE type = 'table' AND name NOT LIKE 'sqlite_%' ORDER BY name ASC`
	rows, err := db.Query(ctx, query)
	if err != nil {
		return nil, errs.FailureError(fmt.Sprintf("sqlite list tables failed: %v", err))
	}
	defer rows.Close()

	var tables []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		tables = append(tables, name)
	}
	return tables, rows.Err()
}

// TableExists checks if a specific table exists in the database.
func TableExists(ctx context.Context, db *orm.DB, tableName string) (bool, error) {
	if db == nil {
		return false, errs.UsageError("sqlite: db connection must not be nil")
	}
	query := `SELECT 1 FROM sqlite_master WHERE type = 'table' AND name = ? LIMIT 1`
	var exists int
	err := db.QueryRow(ctx, query, tableName).Scan(&exists)
	if err != nil {
		return false, nil
	}
	return exists == 1, nil
}

// TableInfo returns column introspection metadata for a given table.
func TableInfo(ctx context.Context, db *orm.DB, tableName string) ([]ColumnInfo, error) {
	if db == nil {
		return nil, errs.UsageError("sqlite: db connection must not be nil")
	}
	escaped := strings.ReplaceAll(tableName, `"`, `""`)
	query := fmt.Sprintf(`PRAGMA table_info("%s")`, escaped)
	rows, err := db.Query(ctx, query)
	if err != nil {
		return nil, errs.FailureError(fmt.Sprintf("sqlite table_info failed: %v", err))
	}
	defer rows.Close()

	var cols []ColumnInfo
	for rows.Next() {
		var c ColumnInfo
		var notNullInt int
		if err := rows.Scan(&c.CID, &c.Name, &c.Type, &notNullInt, &c.DefaultValue, &c.PK); err != nil {
			return nil, errs.FailureError(fmt.Sprintf("sqlite scan table_info failed: %v", err))
		}
		c.NotNull = notNullInt == 1
		cols = append(cols, c)
	}
	return cols, rows.Err()
}

// ListIndices returns the index names attached to a specific table.
func ListIndices(ctx context.Context, db *orm.DB, tableName string) ([]string, error) {
	if db == nil {
		return nil, errs.UsageError("sqlite: db connection must not be nil")
	}
	escaped := strings.ReplaceAll(tableName, `"`, `""`)
	query := fmt.Sprintf(`PRAGMA index_list("%s")`, escaped)
	rows, err := db.Query(ctx, query)
	if err != nil {
		return nil, errs.FailureError(fmt.Sprintf("sqlite index_list failed: %v", err))
	}
	defer rows.Close()

	var indices []string
	for rows.Next() {
		var seq int
		var name string
		var unique int
		var origin string
		var partial int
		if err := rows.Scan(&seq, &name, &unique, &origin, &partial); err != nil {
			return nil, errs.FailureError(fmt.Sprintf("sqlite scan index_list failed: %v", err))
		}
		indices = append(indices, name)
	}
	return indices, rows.Err()
}

// RetryOnBusy executes fn, retrying with exponential backoff if a transient "database is locked" error occurs.
func RetryOnBusy(ctx context.Context, maxRetries int, initialDelay time.Duration, fn func() error) error {
	if maxRetries <= 0 {
		maxRetries = 3
	}
	if initialDelay <= 0 {
		initialDelay = 10 * time.Millisecond
	}

	delay := initialDelay
	var lastErr error
	for attempt := 0; attempt <= maxRetries; attempt++ {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		err := fn()
		if err == nil {
			return nil
		}
		lastErr = err

		msg := strings.ToLower(err.Error())
		if strings.Contains(msg, "database is locked") || strings.Contains(msg, "busy") {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(delay):
				delay *= 2
				continue
			}
		}
		return err
	}
	return lastErr
}
