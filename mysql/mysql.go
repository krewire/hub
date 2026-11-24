// Package mysql bundles a pure-Go MySQL and MariaDB driver for pkg/orm.
//
// Importing this package registers github.com/go-sql-driver/mysql under the
// driver name "mysql" (pure Go, CGO_ENABLED=0 compliant) and provides
// production-ready configuration, connection pooling, and maintenance helpers.
//
// The driver is opt-in: pkg/orm itself never imports it, preserving independent
// modularity (Pillar 2: Sustainable).
package mysql

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/krewire/krewire/errs"
	"github.com/krewire/krewire/orm"

	_ "github.com/go-sql-driver/mysql" // registers the "mysql" database/sql driver
)

// DriverName is the database/sql driver name registered by github.com/go-sql-driver/mysql.
const DriverName = "mysql"

// DefaultPort is the standard MySQL port.
const DefaultPort = 3306

// Config returns a baseline orm.MySQLConfig with safe defaults:
// utf8mb4 encoding, parseTime enabled, and standard timeouts.
func Config(host string, port int, user, password, database string) orm.MySQLConfig {
	if port <= 0 {
		port = DefaultPort
	}
	if host == "" {
		host = "127.0.0.1"
	}
	if user == "" {
		user = "root"
	}

	return orm.MySQLConfig{
		Host:                 host,
		Port:                 port,
		Database:             database,
		User:                 user,
		Password:             password,
		Charset:              "utf8mb4",
		Collation:            "utf8mb4_unicode_ci",
		ParseTime:            true,
		Loc:                  "Local",
		DriverName:           DriverName,
		Timeout:              5 * time.Second,
		ReadTimeout:          30 * time.Second,
		WriteTimeout:         30 * time.Second,
		InterpolateParams:    true,
		MultiStatements:      false, // keep false for defense-in-depth against SQL injection
		AllowNativePasswords: true,
		CheckConnLiveness:    true,
		MaxOpenConns:         25,
		MaxIdleConns:         5,
		ConnMaxLifetime:      10 * time.Minute,
		ConnMaxIdleTime:      2 * time.Minute,
	}
}

// ProductionConfig returns an enterprise-tuned MySQL configuration:
// high concurrency connection pool, statement interpolation, and bounded idle reaps.
func ProductionConfig(host string, port int, user, password, database string) orm.MySQLConfig {
	config := Config(host, port, user, password, database)
	config.MaxOpenConns = 50
	config.MaxIdleConns = 15
	config.ConnMaxLifetime = 15 * time.Minute
	config.ConnMaxIdleTime = 3 * time.Minute
	config.InterpolateParams = true
	config.CheckConnLiveness = true
	return config
}

// DSN returns the formatted MySQL connection string from config.
func DSN(config orm.MySQLConfig) string {
	if strings.TrimSpace(config.DriverName) == "" {
		config.DriverName = DriverName
	}
	return config.FormatDSN()
}

// Open opens a MySQL database connection using a formatted DSN string.
func Open(dsn string) (*orm.DB, error) {
	return orm.OpenWithConfig(orm.Config{
		DriverName: DriverName,
		DSN:        dsn,
	})
}

// OpenConfig opens a MySQL database connection using a structured orm.MySQLConfig.
func OpenConfig(config orm.MySQLConfig) (*orm.DB, error) {
	if strings.TrimSpace(config.DriverName) == "" {
		config.DriverName = DriverName
	}
	return orm.NewMySQL(config)
}

// OpenProduction opens an enterprise-grade connection pool for MySQL.
func OpenProduction(host string, port int, user, password, database string) (*orm.DB, error) {
	return OpenConfig(ProductionConfig(host, port, user, password, database))
}

// ServerVersion queries the connected MySQL server version string.
func ServerVersion(ctx context.Context, db *orm.DB) (string, error) {
	if db == nil {
		return "", errs.UsageError("mysql: db connection must not be nil")
	}
	var version string
	if err := db.QueryRow(ctx, "SELECT VERSION()").Scan(&version); err != nil {
		return "", errs.FailureError(fmt.Sprintf("mysql select version failed: %v", err))
	}
	return version, nil
}

// TableStatus holds storage metrics for a MySQL table.
type TableStatus struct {
	Name      string
	Engine    string
	Rows      int64
	DataSize  int64
	IndexSize int64
}

// GetTableStatus queries table metrics from information_schema.TABLES.
func GetTableStatus(ctx context.Context, db *orm.DB, database, table string) (*TableStatus, error) {
	if db == nil {
		return nil, errs.UsageError("mysql: db connection must not be nil")
	}
	query := `
		SELECT TABLE_NAME, ENGINE, TABLE_ROWS, DATA_LENGTH, INDEX_LENGTH
		FROM information_schema.TABLES
		WHERE TABLE_SCHEMA = ? AND TABLE_NAME = ?
	`
	var status TableStatus
	err := db.QueryRow(ctx, query, database, table).Scan(
		&status.Name,
		&status.Engine,
		&status.Rows,
		&status.DataSize,
		&status.IndexSize,
	)
	if err != nil {
		return nil, errs.FailureError(fmt.Sprintf("mysql query table status failed: %v", err))
	}
	return &status, nil
}

// KillProcess issues a KILL statement to terminate a hung MySQL thread.
func KillProcess(ctx context.Context, db *orm.DB, processID int64) error {
	if db == nil {
		return errs.UsageError("mysql: db connection must not be nil")
	}
	query := fmt.Sprintf("KILL %d", processID)
	_, err := db.Exec(ctx, query)
	if err != nil {
		return errs.FailureError(fmt.Sprintf("mysql kill process (%d) failed: %v", processID, err))
	}
	return nil
}
