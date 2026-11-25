# `mysql` Module Reference

Import: `github.com/krewire/hub/mysql`

## Purpose & Domain Responsibility

`mysql` bundles a pure-Go MySQL and MariaDB driver for `github.com/krewire/krewire/orm`. Importing this package registers `github.com/go-sql-driver/mysql` under `database/sql` with the driver name `"mysql"` (`DriverName`) and provides opinionated configuration builders, DSN formatting, and operational maintenance helpers.

Because `krewire/orm` never imports database drivers directly, applications only link MySQL when they explicitly import `github.com/krewire/hub/mysql`.

## Architectural Position

- **Layer**: Layer 2 (Port & Adapter)
- **Module Path**: `mysql/go.mod` (`github.com/krewire/hub/mysql`)
- **Pure-Go Invariant**: Compiles cleanly with `CGO_ENABLED=0`
- **Dependencies**:
  - Third-party: `github.com/go-sql-driver/mysql v1.9.0`
  - Ecosystem: `github.com/krewire/krewire/orm`, `github.com/krewire/krewire/errs`

## Core Concepts & APIs

### Configuration & Connection Lifecycle

- **Constants**: `DriverName` (`"mysql"`) and `DefaultPort` (`3306`).
- **`Config(host string, port int, user, password, database string) orm.MySQLConfig`**: Returns a baseline `orm.MySQLConfig` with safe defaults: `127.0.0.1:3306`, user `root`, `utf8mb4` / `utf8mb4_unicode_ci`, `ParseTime: true`, `Loc: "Local"`, `InterpolateParams: true`, `MultiStatements: false` (defense-in-depth against SQL injection), `AllowNativePasswords: true`, `CheckConnLiveness: true`, 5s dial timeout, 30s read/write timeouts, and pool limits (`MaxOpenConns: 25`, `MaxIdleConns: 5`, `ConnMaxLifetime: 10m`, `ConnMaxIdleTime: 2m`).
- **`ProductionConfig(host string, port int, user, password, database string) orm.MySQLConfig`**: Scales the pool for production workloads (`MaxOpenConns: 50`, `MaxIdleConns: 15`, `ConnMaxLifetime: 15m`, `ConnMaxIdleTime: 3m`).
- **`DSN(config orm.MySQLConfig) string`**: Defaults `config.DriverName` to `DriverName` if empty and returns `config.FormatDSN()`.
- **`Open(dsn string) (*orm.DB, error)`**: Opens an `*orm.DB` from a formatted DSN string using `orm.OpenWithConfig`.
- **`OpenConfig(config orm.MySQLConfig) (*orm.DB, error)`**: Opens an `*orm.DB` from a structured `orm.MySQLConfig` via `orm.NewMySQL`, ensuring `DriverName` defaults to `"mysql"`.
- **`OpenProduction(host string, port int, user, password, database string) (*orm.DB, error)`**: Convenience wrapper combining `ProductionConfig` and `OpenConfig`.

### Introspection & Operations

All operational helpers validate that `db != nil` (returning `errs.UsageError` if nil) and wrap query errors in `errs.FailureError`:

- **`ServerVersion(ctx context.Context, db *orm.DB) (string, error)`**: Queries `SELECT VERSION()`.
- **`TableStatus`**: Struct holding `Name string`, `Engine string`, `Rows int64`, `DataSize int64`, and `IndexSize int64`.
- **`GetTableStatus(ctx context.Context, db *orm.DB, database, table string) (*TableStatus, error)`**: Queries `information_schema.TABLES` for `TABLE_NAME`, `ENGINE`, `TABLE_ROWS`, `DATA_LENGTH`, and `INDEX_LENGTH`.
- **`KillProcess(ctx context.Context, db *orm.DB, processID int64) error`**: Executes `KILL <processID>` to terminate a hung MySQL thread.

## Usage Example

```go
package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/krewire/hub/mysql"
)

func main() {
	db, err := mysql.OpenProduction(
		"127.0.0.1",
		mysql.DefaultPort,
		os.Getenv("DB_USER"),
		os.Getenv("DB_PASS"),
		"app_prod",
	)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	ctx := context.Background()
	ver, err := mysql.ServerVersion(ctx, db)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("MySQL version:", ver)

	status, err := mysql.GetTableStatus(ctx, db, "app_prod", "users")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Table %s (%s): %d rows, %d data bytes\n",
		status.Name, status.Engine, status.Rows, status.DataSize)
}
```

## Verification

```bash
cd /home/krewire/workspace/krewire/hub/mysql
gofmt -l .
go vet ./...
CGO_ENABLED=0 go test -v ./...
CGO_ENABLED=0 go build ./...
```
