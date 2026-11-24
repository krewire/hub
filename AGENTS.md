# AGENTS.md — Krewire Hub

Operational guide for autonomous AI agents working inside the `krewire/hub` repository (`github.com/krewire/hub`).

## Repository Purpose

This repository is the dedicated **Port & Adapter layer** for all external third-party software in the Krewire digital SDLC ecosystem:
- Database drivers: `hub/sqlite`, `hub/postgres`, `hub/mysql`, `hub/redis`.
- Frontend compilers: `hub/svelte`, `hub/tailwind`, `hub/vue`.
- Hub kernel: `hub/plugin`, `hub/resolver`.

## Invariant Rules

1. **Port & Adapter Boundary**: All third-party dependencies required by Krewire applications (such as `modernc.org/sqlite`, `pgx`, `mysql`, `redis`) must live here and never in `krewire` foundational monorepo.
2. **Pure Multi-Module Monorepo**: Each adapter is an independent Go module in its own subfolder with its own `go.mod`. Never add a `go.mod` to the repository root.
3. **Pure-Go Compliance**: All adapters must compile with `CGO_ENABLED=0`.
4. **English Language**: All code, docstrings, and commit messages must be strictly in English.

## Quality Gates

Run all 4 quality gates prior to finishing work:
```bash
gofmt -l .
go vet ./...
go test ./...
CGO_ENABLED=0 go build ./...
```
