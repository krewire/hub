# `postgres` Module Reference

Import: `github.com/krewire/hub/postgres`

## Purpose & Domain Responsibility

`postgres` bundles a pure-Go PostgreSQL driver for `github.com/krewire/krewire/orm` built on `github.com/jackc/pgx/v5/stdlib`. Importing this package registers the `"pgx"` `database/sql` driver (`DriverName`) and exposes production-ready configuration builders, schema introspection, session/transaction advisory locks, `NOTIFY` pub/sub emission, and maintenance helpers.

## Architectural Position

- **Layer**: Layer 2 (Port & Adapter)
- **Module Path**: `postgres/go.mod` (`github.com/krewire/hub/postgres`)
- **Pure-Go Invariant**: Compiles cleanly with `CGO_ENABLED=0`
- **Dependencies**:
  - Third-party: `github.com/jackc/pgx/v5 v5.7.2`
  - Ecosystem: `github.com/krewire/krewire/orm`, `github.com/krewire/krewire/errs`

## Core Concepts & APIs

### Configuration & Connection Lifecycle

- **Constants**: `DriverName` (`"pgx"`) and `DefaultPort` (`5432`).
- **`Config(host string, port int, user, password, database string) orm.PostgresConfig`**: Returns a local-development baseline (`localhost:5432`, user/db `postgres`, `SSLMode: "disable"`, `TimeZone: "UTC"`, `SearchPath: "public"`, `ApplicationName: "krewire"`, `ConnectTimeout: 5s`, `StatementTimeout: 30s`, `IdleInTransactionSessionTimeout: 60s`, `MaxOpenConns: 25`, `MaxIdleConns: 5`, `ConnMaxLifetime: 10m`, `ConnMaxIdleTime: 2m`).
- **`ProductionConfig(host string, port int, user, password, database string) orm.PostgresConfig`**: Enforces `SSLMode: "require"` and scales pool/timeout settings (`MaxOpenConns: 50`, `MaxIdleConns: 15`, `ConnMaxLifetime: 15m`, `ConnMaxIdleTime: 3m`, `ConnectTimeout: 10s`, `StatementTimeout: 45s`, `IdleInTransactionSessionTimeout: 30s`).
- **`DSN(config orm.PostgresConfig) string`**, **`Open(dsn string) (*orm.DB, error)`**, **`OpenConfig(config orm.PostgresConfig) (*orm.DB, error)`**, and **`OpenProduction(host string, port int, user, password, database string) (*orm.DB, error)`**: Format DSNs and open `*orm.DB` handles via `orm.OpenWithConfig` / `orm.NewPostgres`.

### Advisory Locks & Pub/Sub

- **`TryAdvisoryLock(ctx context.Context, db *orm.DB, key int64) (bool, error)`**: Non-blocking session-level lock via `pg_try_advisory_lock($1)`.
- **`AdvisoryUnlock(ctx context.Context, db *orm.DB, key int64) (bool, error)`**: Releases a session-level lock via `pg_advisory_unlock($1)`.
- **`AdvisoryXactLock(ctx context.Context, tx *orm.Tx, key int64) error`**: Acquires a transaction-scoped lock via `pg_advisory_xact_lock($1)` that PostgreSQL automatically releases on commit or rollback.
- **`Notify(ctx context.Context, db *orm.DB, channel, payload string) error`**: Sends an asynchronous notification via `SELECT pg_notify($1, $2)` (rejects empty channel names with `errs.UsageError`).

### Introspection, Diagnostics & Maintenance

- **Types**:
  - `ConnectionStat`: `{ State string, Count int }`
  - `ColumnInfo`: `{ Name string, DataType string, IsNullable bool, DefaultValue string }`
- **Diagnostics**: `ServerVersion`, `CurrentDatabase`, `ConnectionStats` (grouped by `state` from `pg_stat_activity`), `ActiveConnections`, `TableSize` (`pg_total_relation_size`).
- **Schema Introspection**: `SchemaExists`, `ListTables(ctx, db, schema...)`, `TableExists(ctx, db, tableName, schema...)`, `ListColumns(ctx, db, tableName, schema...)` (defaulting `schema` to `"public"`), and `IndexExists(ctx, db, indexName)`.
- **Maintenance**: `VacuumAnalyze(ctx, db, tableName)` (runs `VACUUM (ANALYZE)` on a quoted table or the whole database if `tableName == ""`), `TerminateBackend(ctx, db, pid)` (`pg_terminate_backend`), and `TruncateTable(ctx, db, tableName, cascade)`.

## Usage Example

```go
package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/krewire/hub/postgres"
)

func main() {
	cfg := postgres.ProductionConfig(
		"db.internal",
		postgres.DefaultPort,
		os.Getenv("PG_USER"),
		os.Getenv("PG_PASS"),
		"krewire_prod",
	)
	db, err := postgres.OpenConfig(cfg)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	ctx := context.Background()
	locked, err := postgres.TryAdvisoryLock(ctx, db, 42001)
	if err != nil {
		log.Fatal(err)
	}
	if locked {
		defer func() { _, _ = postgres.AdvisoryUnlock(ctx, db, 42001) }()
		tables, err := postgres.ListTables(ctx, db)
		if err != nil {
			log.Fatal(err)
		}
		fmt.Println("Tables in public schema:", tables)
	}
}
```

## Verification

```bash
cd /home/krewire/workspace/krewire/hub/postgres
gofmt -l .
go vet ./...
CGO_ENABLED=0 go test -v ./...
CGO_ENABLED=0 go build ./...
```
