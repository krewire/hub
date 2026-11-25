# Krewire Hub (`github.com/krewire/hub`)

Official Port & Adapter repository, third-party integrations, and ecosystem extension hub for the Krewire digital SDLC ecosystem.

## Role & Architecture

Krewire Hub operates as the official boundary where external third-party software, database drivers, and frontend framework compilers integrate into the Krewire ecosystem.

By isolating third-party code in `krewire/hub`, the foundational monorepo (`krewire`) remains **100% Go Standard Library** (pure-Go, CGO-free, zero external dependencies).

Krewire Hub is organized as a domain-first **Pure Multi-Module Monorepo** with 9 independent modules:

```text
hub/
├── go.work       # Go workspace linking all hub modules
├── plugin/       # Base Plugin, Installer, and AssetProvider lifecycle contracts & registry
├── resolver/     # Package and plugin resolution chain (PluginResolver, GoResolver, NpmResolver)
├── redis/        # Pure-Go Redis client (RESP protocol) and cache adapter
├── sqlite/       # Pure-Go SQLite database/sql driver adapter (modernc.org/sqlite)
├── postgres/     # PostgreSQL driver adapter (github.com/jackc/pgx/v5)
├── mysql/        # MySQL driver adapter (github.com/go-sql-driver/mysql)
├── svelte/       # Svelte 5 reactive island compiler and hydration bridge
├── tailwind/     # Standalone and npm Tailwind CSS compiler adapter
└── vue/          # Vue 3 Composition island compiler and hydration bridge
```

## Features

- **Database Drivers**: Opt-in pure-Go SQL driver packages registering standard `database/sql` drivers (`sqlite`, `postgres`, `mysql`) with production-friendly connection pooling and WAL defaults.
- **Frontend Islands & Compilers**: Zero-config bridges compiling Svelte 5, Vue 3, and Tailwind CSS into static assets or client-hydrated islands without polluting Go monorepo dependencies.
- **Independent Modules**: Import only the adapter you need:
  ```go
  import (
      _ "github.com/krewire/hub/sqlite"
      _ "github.com/krewire/hub/postgres"
      "github.com/krewire/hub/redis"
  )
  ```

## Quality Gates

Before contributing changes to `hub`, run the mandatory quality gates across all 9 adapter modules:
```bash
# 1. Formatting
gofmt -l .        # Must return 0 lines

# 2. Per-module validation (or via CI matrix)
for m in mysql plugin postgres redis resolver sqlite svelte tailwind vue; do
  echo "==> Verifying module: $m"
  (cd "$m" && go vet ./... && go test -v -race ./... && CGO_ENABLED=0 go build ./...)
done
```

## License

[MIT License](LICENSE) © 2026 Krewire Contributors
