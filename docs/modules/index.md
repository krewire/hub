# Hub Module Catalog

All 9 independent Go modules in `github.com/krewire/hub`:

| Module | Import Path | External Dependencies | Focus |
|---|---|---|---|
| [`mysql`](./mysql.md) | `github.com/krewire/hub/mysql` | `github.com/go-sql-driver/mysql` | Pure-Go MySQL/MariaDB driver adapter for `krewire/orm` with config builders, DSN formatting, table status, and process management. |
| [`plugin`](./plugin.md) | `github.com/krewire/hub/plugin` | None (stdlib + `krewire/version`) | Site build plugin contracts (`Plugin`, `Installer`, `AssetProvider`), global registry, package manager detection, and Vite command builder. |
| [`postgres`](./postgres.md) | `github.com/krewire/hub/postgres` | `github.com/jackc/pgx/v5` | Pure-Go PostgreSQL driver adapter for `krewire/orm` with advisory locks, `NOTIFY`, connection stats, and schema introspection. |
| [`redis`](./redis.md) | `github.com/krewire/hub/redis` | None (stdlib + `krewire/errs`) | Pure-Go RESP client (`Client`, `EncodeRESP`, `ReadRESP`) supporting strings, hashes, lists, and pub/sub for Redis, KeyDB, Dragonfly, and Valkey. |
| [`resolver`](./resolver.md) | `github.com/krewire/hub/resolver` | None (via `hub/*` & `krewire/version`) | Cascading `package@version` resolver (`DefaultChain`: `PluginResolver` → `GoResolver` → `NpmResolver`) with `ParseSpec` security validation. |
| [`sqlite`](./sqlite.md) | `github.com/krewire/hub/sqlite` | `modernc.org/sqlite` | Pure-Go, CGO-free SQLite driver adapter for `krewire/orm` with WAL defaults, hot backups (`VACUUM INTO`), PRAGMA stats, and `RetryOnBusy`. |
| [`svelte`](./svelte.md) | `github.com/krewire/hub/svelte` | None (shells out to Vite) | Svelte 5 + Vite compiler adapter implementing `Plugin`, `Installer`, and `AssetProvider` with config and starter scaffolding. |
| [`tailwind`](./tailwind.md) | `github.com/krewire/hub/tailwind` | None (shells out to Tailwind CLI) | Tailwind CSS v3/v4 adapter implementing `Plugin`, `Installer`, and `AssetProvider` with multi-runner discovery and `--minify` builds. |
| [`vue`](./vue.md) | `github.com/krewire/hub/vue` | None (shells out to Vite) | Vue 3 + Vite compiler adapter implementing `Plugin`, `Installer`, and `AssetProvider` with config and starter scaffolding. |
