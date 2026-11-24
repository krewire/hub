# Contributing to Krewire Hub

Welcome! This guide explains how to contribute adapters, drivers, and plugins to **Krewire Hub** (`github.com/krewire/hub`).

> All code, comments, docstrings, commit messages, and documentation are strictly in **English**.

---

## Architecture & Conventions

1. **Independent Submodules**: Each integration must live in its own directory with a dedicated `go.mod` (e.g. `sqlite/go.mod`, `redis/go.mod`).
2. **Standard Library Interface Conformance**:
   - Database drivers must register standard `database/sql` driver names and implement `Open` helpers returning `*orm.DB`.
   - Frontend tools must implement `plugin.Plugin`, `plugin.Installer`, or `plugin.AssetProvider`.
3. **Zero CGO**: All modules must compile cleanly with `CGO_ENABLED=0`.

## Commit Standards

Format commit messages using Conventional Commits:
```text
feat(sqlite): add WAL auto-checkpoint tuning parameter
fix(tailwind): resolve asset output path in nested directories
```

## Quality Gates

Every module must independently pass:
```bash
gofmt -l .
go vet ./...
go test ./...
CGO_ENABLED=0 go build ./...
```
