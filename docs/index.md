# Krewire Hub Documentation

> **Public Port & Adapter boundary isolating third-party database drivers, RESP cache clients, and frontend compiler integrations from the 100% Go standard-library core.**

Central engineering documentation for [`github.com/krewire/hub`](https://github.com/krewire/hub).

---

## 1. Overview & Architecture

- [Architecture & Third-Party Isolation](./architecture.md) — Layer 2 Port & Adapter boundary, pure-Go (`CGO_ENABLED=0`) invariant, dependency rules, and reversible provider swapping.
- [Getting Started](./getting-started.md) — Multi-module `go.work` setup, importing adapters, configuring drivers, and running verification gates.

## 2. Module Reference (9 Modules)

- [Module Catalog](./modules/index.md) — Complete inventory of all 9 independent Go modules in `hub`.
- **Database & Cache Drivers**:
  - [`mysql`](./modules/mysql.md) — Pure-Go MySQL/MariaDB driver adapter (`go-sql-driver/mysql`) for `krewire/orm`.
  - [`postgres`](./modules/postgres.md) — Pure-Go PostgreSQL driver adapter (`jackc/pgx/v5`) with advisory locks, `NOTIFY`, and schema introspection.
  - [`sqlite`](./modules/sqlite.md) — Pure-Go SQLite driver adapter (`modernc.org/sqlite`) with WAL defaults, hot backups (`VACUUM INTO`), and busy-retry helpers.
  - [`redis`](./modules/redis.md) — Zero-external-dependency pure-Go RESP client for Redis, KeyDB, Dragonfly, and Valkey.
- **Plugin & Dependency Resolution**:
  - [`plugin`](./modules/plugin.md) — Minimal `Plugin`, `Installer`, and `AssetProvider` contracts, global registry, and package-manager/Vite helpers.
  - [`resolver`](./modules/resolver.md) — Cascading `package@version` resolver (`PluginResolver` → `GoResolver` → `NpmResolver`) with input validation.
- **Frontend Compiler Adapters**:
  - [`svelte`](./modules/svelte.md) — Svelte 5 + Vite build and scaffolding adapter.
  - [`tailwind`](./modules/tailwind.md) — Tailwind CSS v3/v4 CLI detection, build, and scaffolding adapter.
  - [`vue`](./modules/vue.md) — Vue 3 + Vite build and scaffolding adapter.
