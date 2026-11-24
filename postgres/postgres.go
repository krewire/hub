// Package postgres bundles a pure-Go PostgreSQL driver for pkg/orm based on pgx.
//
// Importing this package registers github.com/jackc/pgx/v5/stdlib under the
// driver name "pgx" (pure Go, CGO_ENABLED=0 compliant) and provides
// production-ready configuration, connection pooling, and maintenance helpers.
//
// The driver is opt-in: pkg/orm itself never imports it, preserving independent
// modularity (Pillar 2: Sustainable).
package postgres

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/krewire/krewire/errs"
	"github.com/krewire/krewire/orm"

	_ "github.com/jackc/pgx/v5/stdlib" // registers the "pgx" database/sql driver
)

// DriverName is the database/sql driver name registered by pgx/v5/stdlib.
const DriverName = "pgx"

// DefaultPort is the standard PostgreSQL port.
const DefaultPort = 5432

// Config returns a baseline orm.PostgresConfig with safe defaults:
// SSLMode disabled (for local dev), UTC timezone, public search path.
func Config(host string, port int, user, password, database string) orm.PostgresConfig {
	if port <= 0 {
		port = DefaultPort
	}
	if host == "" {
		host = "localhost"
	}
	if user == "" {
		user = "postgres"
	}
	if database == "" {
		database = "postgres"
	}

	return orm.PostgresConfig{
		Host:                            host,
		Port:                            port,
		Database:                        database,
		User:                            user,
		Password:                        password,
		SSLMode:                         "disable",
		TimeZone:                        "UTC",
		DriverName:                      DriverName,
		SearchPath:                      "public",
		ApplicationName:                 "krewire",
		ConnectTimeout:                  5 * time.Second,
		StatementTimeout:                30 * time.Second,
		IdleInTransactionSessionTimeout: 60 * time.Second,
		MaxOpenConns:                    25,
		MaxIdleConns:                    5,
		ConnMaxLifetime:                 10 * time.Minute,
		ConnMaxIdleTime:                 2 * time.Minute,
	}
}

// ProductionConfig returns an enterprise-tuned PostgreSQL configuration:
// SSLMode require, pool limits, prepared statement caching, and bounded timeouts.
func ProductionConfig(host string, port int, user, password, database string) orm.PostgresConfig {
	config := Config(host, port, user, password, database)
	config.SSLMode = "require"
	config.MaxOpenConns = 50
	config.MaxIdleConns = 15
	config.ConnMaxLifetime = 15 * time.Minute
	config.ConnMaxIdleTime = 3 * time.Minute
	config.ConnectTimeout = 10 * time.Second
	config.StatementTimeout = 45 * time.Second
	config.IdleInTransactionSessionTimeout = 30 * time.Second
	return config
}

// DSN returns the formatted PostgreSQL connection URL string from config.
func DSN(config orm.PostgresConfig) string {
	if strings.TrimSpace(config.DriverName) == "" {
		config.DriverName = DriverName
	}
	return config.FormatDSN()
}

// Open opens a PostgreSQL database connection using a formatted DSN string.
func Open(dsn string) (*orm.DB, error) {
	return orm.OpenWithConfig(orm.Config{
		DriverName: DriverName,
		DSN:        dsn,
	})
}

// OpenConfig opens a PostgreSQL database connection using a structured orm.PostgresConfig.
func OpenConfig(config orm.PostgresConfig) (*orm.DB, error) {
	if strings.TrimSpace(config.DriverName) == "" {
		config.DriverName = DriverName
	}
	return orm.NewPostgres(config)
}

// OpenProduction opens an enterprise-grade connection pool for PostgreSQL.
func OpenProduction(host string, port int, user, password, database string) (*orm.DB, error) {
	return OpenConfig(ProductionConfig(host, port, user, password, database))
}

// ServerVersion queries the connected PostgreSQL server version string.
func ServerVersion(ctx context.Context, db *orm.DB) (string, error) {
	if db == nil {
		return "", errs.UsageError("postgres: db connection must not be nil")
	}
	var version string
	if err := db.QueryRow(ctx, "SELECT version()").Scan(&version); err != nil {
		return "", errs.FailureError(fmt.Sprintf("postgres select version failed: %v", err))
	}
	return version, nil
}

// CurrentDatabase queries the active database name.
func CurrentDatabase(ctx context.Context, db *orm.DB) (string, error) {
	if db == nil {
		return "", errs.UsageError("postgres: db connection must not be nil")
	}
	var currentDB string
	if err := db.QueryRow(ctx, "SELECT current_database()").Scan(&currentDB); err != nil {
		return "", errs.FailureError(fmt.Sprintf("postgres current_database failed: %v", err))
	}
	return currentDB, nil
}

// SchemaExists checks if a schema exists in information_schema.schemata.
func SchemaExists(ctx context.Context, db *orm.DB, schemaName string) (bool, error) {
	if db == nil {
		return false, errs.UsageError("postgres: db connection must not be nil")
	}
	query := `SELECT EXISTS(SELECT 1 FROM information_schema.schemata WHERE schema_name = $1)`
	var exists bool
	if err := db.QueryRow(ctx, query, schemaName).Scan(&exists); err != nil {
		return false, errs.FailureError(fmt.Sprintf("postgres schema_exists check failed: %v", err))
	}
	return exists, nil
}

// TableSize queries the total size on disk for a table including its indexes.
func TableSize(ctx context.Context, db *orm.DB, tableName string) (int64, error) {
	if db == nil {
		return 0, errs.UsageError("postgres: db connection must not be nil")
	}
	query := `SELECT pg_total_relation_size($1)`
	var size int64
	if err := db.QueryRow(ctx, query, tableName).Scan(&size); err != nil {
		return 0, errs.FailureError(fmt.Sprintf("postgres table_size failed: %v", err))
	}
	return size, nil
}

// VacuumAnalyze executes VACUUM (ANALYZE) on a target table to reclaim storage and update planner statistics.
func VacuumAnalyze(ctx context.Context, db *orm.DB, tableName string) error {
	if db == nil {
		return errs.UsageError("postgres: db connection must not be nil")
	}
	var query string
	if tableName != "" {
		query = fmt.Sprintf(`VACUUM (ANALYZE) "%s"`, strings.ReplaceAll(tableName, `"`, `""`))
	} else {
		query = "VACUUM (ANALYZE)"
	}
	_, err := db.Exec(ctx, query)
	if err != nil {
		return errs.FailureError(fmt.Sprintf("postgres vacuum analyze failed: %v", err))
	}
	return nil
}

// TerminateBackend calls pg_terminate_backend(pid) to terminate a runaway query or hung connection.
func TerminateBackend(ctx context.Context, db *orm.DB, pid int) (bool, error) {
	if db == nil {
		return false, errs.UsageError("postgres: db connection must not be nil")
	}
	query := `SELECT pg_terminate_backend($1)`
	var terminated bool
	if err := db.QueryRow(ctx, query, pid).Scan(&terminated); err != nil {
		return false, errs.FailureError(fmt.Sprintf("postgres terminate_backend (%d) failed: %v", pid, err))
	}
	return terminated, nil
}

// TryAdvisoryLock attempts to acquire a session-level advisory lock using pg_try_advisory_lock.
// It returns true if the lock was acquired immediately, or false if already held by another session.
func TryAdvisoryLock(ctx context.Context, db *orm.DB, key int64) (bool, error) {
	if db == nil {
		return false, errs.UsageError("postgres: db connection must not be nil")
	}
	var acquired bool
	if err := db.QueryRow(ctx, "SELECT pg_try_advisory_lock($1)", key).Scan(&acquired); err != nil {
		return false, errs.FailureError(fmt.Sprintf("postgres try_advisory_lock failed: %v", err))
	}
	return acquired, nil
}

// AdvisoryUnlock releases a session-level advisory lock previously acquired by TryAdvisoryLock.
func AdvisoryUnlock(ctx context.Context, db *orm.DB, key int64) (bool, error) {
	if db == nil {
		return false, errs.UsageError("postgres: db connection must not be nil")
	}
	var released bool
	if err := db.QueryRow(ctx, "SELECT pg_advisory_unlock($1)", key).Scan(&released); err != nil {
		return false, errs.FailureError(fmt.Sprintf("postgres advisory_unlock failed: %v", err))
	}
	return released, nil
}

// AdvisoryXactLock acquires a transaction-level advisory lock using pg_advisory_xact_lock.
// The lock is automatically released by PostgreSQL when the transaction commits or aborts.
func AdvisoryXactLock(ctx context.Context, tx *orm.Tx, key int64) error {
	if tx == nil {
		return errs.UsageError("postgres: tx must not be nil")
	}
	if _, err := tx.Exec(ctx, "SELECT pg_advisory_xact_lock($1)", key); err != nil {
		return errs.FailureError(fmt.Sprintf("postgres advisory_xact_lock failed: %v", err))
	}
	return nil
}

// Notify emits an asynchronous pub/sub notification message to channel via pg_notify.
func Notify(ctx context.Context, db *orm.DB, channel, payload string) error {
	if db == nil {
		return errs.UsageError("postgres: db connection must not be nil")
	}
	if strings.TrimSpace(channel) == "" {
		return errs.UsageError("postgres: notify channel must not be empty")
	}
	if _, err := db.Exec(ctx, "SELECT pg_notify($1, $2)", channel, payload); err != nil {
		return errs.FailureError(fmt.Sprintf("postgres notify (%s) failed: %v", channel, err))
	}
	return nil
}

// ConnectionStat details backend connection activity grouped by state from pg_stat_activity.
type ConnectionStat struct {
	State string `json:"state"`
	Count int    `json:"count"`
}

// ConnectionStats returns connection counts grouped by state from pg_stat_activity.
func ConnectionStats(ctx context.Context, db *orm.DB) ([]ConnectionStat, error) {
	if db == nil {
		return nil, errs.UsageError("postgres: db connection must not be nil")
	}
	query := `SELECT COALESCE(state, 'unknown'), count(*) FROM pg_stat_activity WHERE datname = current_database() GROUP BY state ORDER BY count DESC`
	rows, err := db.Query(ctx, query)
	if err != nil {
		return nil, errs.FailureError(fmt.Sprintf("postgres connection_stats failed: %v", err))
	}
	defer rows.Close()

	var stats []ConnectionStat
	for rows.Next() {
		var s ConnectionStat
		if err := rows.Scan(&s.State, &s.Count); err != nil {
			return nil, err
		}
		stats = append(stats, s)
	}
	return stats, rows.Err()
}

// ActiveConnections returns the total number of connected backends for the active database.
func ActiveConnections(ctx context.Context, db *orm.DB) (int, error) {
	if db == nil {
		return 0, errs.UsageError("postgres: db connection must not be nil")
	}
	var count int
	query := `SELECT count(*) FROM pg_stat_activity WHERE datname = current_database()`
	if err := db.QueryRow(ctx, query).Scan(&count); err != nil {
		return 0, errs.FailureError(fmt.Sprintf("postgres active_connections count failed: %v", err))
	}
	return count, nil
}

// ColumnInfo holds column schema introspection metadata.
type ColumnInfo struct {
	Name         string `json:"name"`
	DataType     string `json:"data_type"`
	IsNullable   bool   `json:"is_nullable"`
	DefaultValue string `json:"default_value"`
}

// ListTables returns user tables in the given schema (default: "public").
func ListTables(ctx context.Context, db *orm.DB, schema ...string) ([]string, error) {
	if db == nil {
		return nil, errs.UsageError("postgres: db connection must not be nil")
	}
	schemaName := "public"
	if len(schema) > 0 && strings.TrimSpace(schema[0]) != "" {
		schemaName = schema[0]
	}

	query := `SELECT table_name FROM information_schema.tables WHERE table_schema = $1 AND table_type = 'BASE TABLE' ORDER BY table_name ASC`
	rows, err := db.Query(ctx, query, schemaName)
	if err != nil {
		return nil, errs.FailureError(fmt.Sprintf("postgres list tables failed: %v", err))
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

// TableExists checks if a table exists in the given schema (default: "public").
func TableExists(ctx context.Context, db *orm.DB, tableName string, schema ...string) (bool, error) {
	if db == nil {
		return false, errs.UsageError("postgres: db connection must not be nil")
	}
	schemaName := "public"
	if len(schema) > 0 && strings.TrimSpace(schema[0]) != "" {
		schemaName = schema[0]
	}

	query := `SELECT EXISTS(SELECT 1 FROM information_schema.tables WHERE table_schema = $1 AND table_name = $2)`
	var exists bool
	if err := db.QueryRow(ctx, query, schemaName, tableName).Scan(&exists); err != nil {
		return false, errs.FailureError(fmt.Sprintf("postgres table_exists check failed: %v", err))
	}
	return exists, nil
}

// ListColumns returns column metadata for a target table in the given schema (default: "public").
func ListColumns(ctx context.Context, db *orm.DB, tableName string, schema ...string) ([]ColumnInfo, error) {
	if db == nil {
		return nil, errs.UsageError("postgres: db connection must not be nil")
	}
	schemaName := "public"
	if len(schema) > 0 && strings.TrimSpace(schema[0]) != "" {
		schemaName = schema[0]
	}

	query := `
		SELECT column_name, data_type, is_nullable, COALESCE(column_default, '')
		FROM information_schema.columns
		WHERE table_schema = $1 AND table_name = $2
		ORDER BY ordinal_position ASC
	`
	rows, err := db.Query(ctx, query, schemaName, tableName)
	if err != nil {
		return nil, errs.FailureError(fmt.Sprintf("postgres list columns failed: %v", err))
	}
	defer rows.Close()

	var columns []ColumnInfo
	for rows.Next() {
		var col ColumnInfo
		var nullableStr string
		if err := rows.Scan(&col.Name, &col.DataType, &nullableStr, &col.DefaultValue); err != nil {
			return nil, err
		}
		col.IsNullable = strings.EqualFold(nullableStr, "YES")
		columns = append(columns, col)
	}
	return columns, rows.Err()
}

// IndexExists checks if an index exists in pg_indexes.
func IndexExists(ctx context.Context, db *orm.DB, indexName string) (bool, error) {
	if db == nil {
		return false, errs.UsageError("postgres: db connection must not be nil")
	}
	query := `SELECT EXISTS(SELECT 1 FROM pg_indexes WHERE indexname = $1)`
	var exists bool
	if err := db.QueryRow(ctx, query, indexName).Scan(&exists); err != nil {
		return false, errs.FailureError(fmt.Sprintf("postgres index_exists check failed: %v", err))
	}
	return exists, nil
}

// TruncateTable empties all rows from a table, optionally cascading to foreign keys.
func TruncateTable(ctx context.Context, db *orm.DB, tableName string, cascade bool) error {
	if db == nil {
		return errs.UsageError("postgres: db connection must not be nil")
	}
	quoted := fmt.Sprintf(`"%s"`, strings.ReplaceAll(tableName, `"`, `""`))
	query := fmt.Sprintf("TRUNCATE TABLE %s", quoted)
	if cascade {
		query += " CASCADE"
	}
	if _, err := db.Exec(ctx, query); err != nil {
		return errs.FailureError(fmt.Sprintf("postgres truncate table failed: %v", err))
	}
	return nil
}
