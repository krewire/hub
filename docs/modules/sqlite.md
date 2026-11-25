# `sqlite` Module Reference

Import: `github.com/krewire/hub/sqlite`

## Purpose & Domain Responsibility

`sqlite` bundles a pure-Go, CGO-free SQLite driver (`modernc.org/sqlite`) for `github.com/krewire/krewire/orm`. Importing it registers the `"sqlite"` `database/sql` driver (`DriverName`) and configures SQLite with concurrency-safe defaults out of the box, alongside hot online backup, WAL checkpointing, integrity verification, and busy-lock retry helpers.

## Architectural Position

- **Layer**: Layer 2 (Port & Adapter)
- **Module Path**: `sqlite/go.mod` (`github.com/krewire/hub/sqlite`)
- **Pure-Go Invariant**: Compiles cleanly with `CGO_ENABLED=0` (no `gcc` or `libsqlite3` required)
- **Dependencies**:
  - Third-party: `modernc.org/sqlite v1.36.0`
  - Ecosystem: `github.com/krewire/krewire/orm`, `github.com/krewire/krewire/errs`

## Core Concepts & APIs

### Configuration & Safe Concurrency Defaults

- **Constants**: `DriverName` (`"sqlite"`) and `DefaultTxLock` (`"immediate"`).
- **`Config(path string) orm.SQLiteConfig`**: Configures `JournalMode: "WAL"`, `ForeignKeys: true`, `TxLock: "immediate"`, `BusyTimeout: 5s`, `Synchronous: "NORMAL"`, and pool limits (`MaxOpenConns: 10`, `MaxIdleConns: 5`, `ConnMaxLifetime: 10m`, `ConnMaxIdleTime: 2m`). Passing `""` or `":memory:"` selects a private in-memory database.
- **`ProductionConfig(path string) orm.SQLiteConfig`**: Extends `Config` with a 64 MB page cache (`CacheSize: -64000`), `TempStore: "MEMORY"`, `MmapSize: 256 * 1024 * 1024` (256 MB), and `AutoVacuum: "INCREMENTAL"`.
- **`DSN(path string) string`**, **`Open(path string) (*orm.DB, error)`**, **`OpenProduction(path string) (*orm.DB, error)`**, and **`OpenConfig(config orm.SQLiteConfig) (*orm.DB, error)`**: Build DSNs and open `*orm.DB` handles via `orm.NewSQLite`. `OpenConfig` defaults empty `DriverName` and `TxLock` fields automatically.

### WAL, Maintenance & Hot Backups

- **`CheckpointMode`**: Typed string constants `CheckpointPassive` (`"PASSIVE"`), `CheckpointFull` (`"FULL"`), `CheckpointRestart` (`"RESTART"`), and `CheckpointTruncate` (`"TRUNCATE"`).
- **`Checkpoint(ctx, db, mode)`**: Runs `PRAGMA wal_checkpoint(<mode>)` (defaults empty mode to `CheckpointPassive`).
- **`Backup(ctx, srcDB, destPath)`**: Performs an online hot copy via `VACUUM INTO '<destPath>'` without blocking concurrent readers or writers.
- **`Vacuum(ctx, db)`**, **`IncrementalVacuum(ctx, db, pages)`**, and **`Optimize(ctx, db)`**: Reclaim space and refresh planner statistics (`PRAGMA optimize`).
- **`IntegrityCheck(ctx, db)`** and **`QuickCheck(ctx, db)`**: Execute `PRAGMA integrity_check` / `PRAGMA quick_check` and return an error if the engine reports anything other than `"ok"`.

### Introspection, PRAGMA Controls & Retry

- **Types**:
  - `PageStats`: `{ PageCount, PageSize, TotalBytes, FreeListPages, UserVersion, SchemaVersion }`
  - `ColumnInfo`: `{ CID, Name, Type, NotNull, DefaultValue *string, PK }`
- **Helpers**: `GetPageStats`, `DatabaseSize`, `GetUserVersion`, `SetUserVersion`, `ForeignKeysEnabled`, `SetJournalMode`, `SetSynchronous`, `ListTables`, `TableExists`, `TableInfo`, and `ListIndices`.
- **`RetryOnBusy(ctx context.Context, maxRetries int, initialDelay time.Duration, fn func() error) error`**: Retries `fn` with exponential backoff (`delay *= 2`) when `err.Error()` contains `"database is locked"` or `"busy"`, respecting `ctx.Done()`.

## Usage Example

```go
package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/krewire/hub/sqlite"
)

func main() {
	db, err := sqlite.OpenProduction("data/app.db")
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	ctx := context.Background()
	err = sqlite.RetryOnBusy(ctx, 5, 10*time.Millisecond, func() error {
		_, execErr := db.Exec(ctx, "CREATE TABLE IF NOT EXISTS items (id INTEGER PRIMARY KEY, name TEXT)")
		return execErr
	})
	if err != nil {
		log.Fatal(err)
	}

	stats, err := sqlite.GetPageStats(ctx, db)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("DB size: %d bytes (%d pages)\n", stats.TotalBytes, stats.PageCount)
}
```

## Verification

```bash
cd /home/krewire/workspace/krewire/hub/sqlite
gofmt -l .
go vet ./...
CGO_ENABLED=0 go test -v ./...
CGO_ENABLED=0 go build ./...
```
