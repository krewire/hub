# `redis` Module Reference

Import: `github.com/krewire/hub/redis`

## Purpose & Domain Responsibility

`redis` implements a lightweight, pure-Go client speaking the Redis Serialization Protocol (RESP) over TCP. It is compatible with Redis, KeyDB, Dragonfly, and Valkey servers, providing key-value strings, hashes, lists, pub/sub publishing, and raw command execution without any third-party driver dependencies.

## Architectural Position

- **Layer**: Layer 2 (Port & Adapter)
- **Module Path**: `redis/go.mod` (`github.com/krewire/hub/redis`)
- **Pure-Go Invariant**: Compiles cleanly with `CGO_ENABLED=0`
- **Dependencies**:
  - Third-party: None (100% Go standard library)
  - Ecosystem: `github.com/krewire/krewire/errs` (enforced by `dependency_test.go`, which permits only stdlib, `krewire/errs`, and `krewire/model`)

## Core Concepts & APIs

### Client Configuration & Connection Management

- **Constants & Errors**:
  - `DefaultAddress = "127.0.0.1:6379"`
  - `DefaultTimeout = 5 * time.Second`
  - `ErrNil = errors.New("redis: nil")` (returned by `Get`, `HGet`, and `RPop` on RESP null bulk strings)
- **`Config`**: `{ Address string, Password string, DB int, Timeout time.Duration }`.
- **`NewClient(config Config) (*Client, error)`**: Initializes a thread-safe `Client` (`sync.Mutex`-guarded). Connections are dialed lazily on first command; if `Password` is set, `AUTH` is sent automatically, and if `DB > 0`, `SELECT` is issued.
- **`(c *Client) Do(ctx context.Context, args ...string) (interface{}, error)`**: Executes any arbitrary RESP command with per-call read/write deadlines.
- **`(c *Client) Ping(ctx context.Context) error`** and **`(c *Client) Close() error`**: Health check and clean connection shutdown.

### Data Structure Methods

- **Strings**: `Get(ctx, key)`, `Set(ctx, key, val, ttl)` (uses `SETEX` when `ttl > 0`, `SET` otherwise), `Del(ctx, keys...)`, `Exists(ctx, keys...)`, `Incr(ctx, key)`.
- **Hashes**: `HGet(ctx, key, field)`, `HSet(ctx, key, field, val)`, `HGetAll(ctx, key) (map[string]string, error)`, `HDel(ctx, key, fields...)`.
- **Lists**: `LPush(ctx, key, values...)`, `RPop(ctx, key)`, `LLen(ctx, key)`.
- **Pub/Sub**: `Publish(ctx, channel, message) (int64, error)`.

### RESP Wire Protocol (`resp.go`)

- **`EncodeRESP(args ...string) []byte`**: Serializes a command and arguments into a RESP array of bulk strings (`*N\r\n$len\r\narg\r\n...`).
- **`ReadRESP(r *bufio.Reader) (interface{}, error)`**: Decodes RESP simple strings (`+`), errors (`-`), integers (`:`), bulk strings (`$`, returning `nil, nil` for `$-1`), and arrays (`*`).

## Usage Example

```go
package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/krewire/hub/redis"
)

func main() {
	client, err := redis.NewClient(redis.Config{
		Address: "127.0.0.1:6379",
		Timeout: 3 * time.Second,
	})
	if err != nil {
		log.Fatal(err)
	}
	defer client.Close()

	ctx := context.Background()
	if err := client.Set(ctx, "session:42", "active", 10*time.Minute); err != nil {
		log.Fatal(err)
	}

	val, err := client.Get(ctx, "session:42")
	if errors.Is(err, redis.ErrNil) {
		fmt.Println("key expired or missing")
	} else if err != nil {
		log.Fatal(err)
	} else {
		fmt.Println("session:", val)
	}
}
```

## Verification

```bash
cd /home/krewire/workspace/krewire/hub/redis
gofmt -l .
go vet ./...
CGO_ENABLED=0 go test -v ./...
CGO_ENABLED=0 go build ./...
```
