# Architecture — Krewire Hub

`github.com/krewire/hub` is the sole **Port & Adapter** boundary where third-party database drivers, RESP protocol clients, and frontend compiler toolchains enter the Krewire ecosystem at **Layer 2**.

## Architectural Position

```text
Layer 4–5: Application Engines & Portals (krew, cloud, auth, dash)
               │
               ▼
Layer 2:   github.com/krewire/hub/*   (Port & Adapter Boundary)
           ├── Database Adapters: sqlite, postgres, mysql  ──► krewire/orm
           ├── Cache Client:      redis                    ──► krewire/errs
           ├── Plugin Contract:   plugin                   ──► krewire/version
           ├── Resolution Chain:  resolver                 ──► hub/plugin, svelte, tailwind, vue
           └── Frontend Adapters: svelte, tailwind, vue    ──► hub/plugin, krewire/version
               │
               ▼
Layer 1:   krewire/errs, krewire/version
Layer 0:   Go Standard Library (database/sql, net, os/exec, bufio)
```

## Architectural Invariants

1. **Third-Party Isolation Guard**:
   The foundational `krewire` monorepo remains 100% Go standard library. External drivers (`modernc.org/sqlite`, `github.com/jackc/pgx/v5`, `github.com/go-sql-driver/mysql`) and frontend CLI bridges (`vite`, `tailwindcss`, `svelte`, `vue`) live exclusively in `hub/*`.
2. **Pure Multi-Module Layout**:
   There is no root `go.mod`. Each of the 9 modules owns its own `<module>/go.mod`, coordinated locally by `hub/go.work`. Importing `hub/sqlite` never pulls `pgx` or `go-sql-driver/mysql` into your dependency graph.
3. **Pure Go (`CGO_ENABLED=0`)**:
   Every module compiles cleanly with `CGO_ENABLED=0`, preserving single-binary cross-compilation.
4. **Reversible Provider Swapping**:
   Database adapters return `*orm.DB` from `github.com/krewire/krewire/orm` and accept `orm.SQLiteConfig`, `orm.PostgresConfig`, or `orm.MySQLConfig`, allowing applications to switch engines without rewriting domain queries.
5. **Downward-Only Dependencies**:
   `hub` modules may depend on Layer 1/Layer 2 `krewire/*` packages (`errs`, `version`, `orm`) and sibling `hub/plugin`, but never on Layer 3+ tools (`krew`, `cloud`, `auth`, `dash`). In `hub/redis`, `dependency_test.go` enforces that production code only imports the Go standard library and `krewire/errs` or `krewire/model`.

## Module Inventory

- **Database Drivers**: `mysql/`, `postgres/`, `sqlite/`
- **Cache & Wire Protocol**: `redis/`
- **Plugin & Resolution**: `plugin/`, `resolver/`
- **Frontend Compilers**: `svelte/`, `tailwind/`, `vue/`
