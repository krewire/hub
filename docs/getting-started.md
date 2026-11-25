# Getting Started with Krewire Hub

`hub` is organized as a pure multi-module Go workspace (`go.work` at the repository root, with no root `go.mod`). Each adapter lives in its own Go module (`<module>/go.mod`) so downstream applications only link the specific third-party drivers they import.

## Prerequisites

- **Go**: Version 1.27.3+ (matching `go.work` and module `go.mod` files)
- **CGO**: Disabled (`CGO_ENABLED=0`) — all modules compile as pure Go without C toolchains
- **Optional Frontend Tooling**: Node.js (`npm`/`pnpm`/`yarn`), `bun`, or the standalone `tailwindcss` binary when using `svelte`, `tailwind`, or `vue` build adapters

## Importing a Database Adapter

Database adapters in `hub` implement opt-in driver registration and configuration helpers on top of `github.com/krewire/krewire/orm`. Import the adapter you need in your application or `main` package:

```go
package main

import (
	"context"
	"log"

	"github.com/krewire/hub/sqlite"
)

func main() {
	db, err := sqlite.Open("app.db")
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	if err := sqlite.Optimize(context.Background(), db); err != nil {
		log.Fatal(err)
	}
}
```

To swap to PostgreSQL or MySQL, replace the adapter configuration with `postgres.OpenProduction(...)` or `mysql.OpenProduction(...)` while keeping your `orm.DB` queries unchanged.

## Using the Redis Client

`github.com/krewire/hub/redis` provides a standalone, pure-Go RESP client with zero third-party dependencies:

```go
client, err := redis.NewClient(redis.Config{
	Address: "127.0.0.1:6379",
})
if err != nil {
	log.Fatal(err)
}
defer client.Close()

if err := client.Ping(context.Background()); err != nil {
	log.Fatal(err)
}
```

## Using Plugins and the Resolver

Frontend adapters (`svelte`, `tailwind`, `vue`) self-register with `github.com/krewire/hub/plugin` via `init()`. Importing `github.com/krewire/hub/resolver` automatically registers all three built-in plugins and exposes `resolver.ParseSpec` and `resolver.DefaultChain()` to resolve `plugin`, `gomod`, and `npm` packages uniformly.

## Running Tests & Quality Gates

Run formatting, static analysis, unit tests, and pure-Go builds across all 9 modules:

```bash
cd /home/krewire/workspace/krewire/hub
gofmt -l .
for m in mysql plugin postgres redis resolver sqlite svelte tailwind vue; do
  (cd "$m" && go vet ./... && CGO_ENABLED=0 go test -v ./... && CGO_ENABLED=0 go build ./...)
done
```
