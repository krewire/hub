package redis_test

import (
	"bufio"
	"context"
	"fmt"
	"net"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/krewire/hub/redis"
)

func startMockRedisServer(t *testing.T) (string, func()) {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}

	var mu sync.Mutex
	kvs := make(map[string]string)
	hashes := make(map[string]map[string]string)
	lists := make(map[string][]string)

	stopCh := make(chan struct{})

	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}

			go func(c net.Conn) {
				defer c.Close()
				reader := bufio.NewReader(c)

				for {
					line, err := reader.ReadString('\n')
					if err != nil {
						return
					}
					line = strings.TrimSpace(line)
					if strings.HasPrefix(line, "*") {
						var count int
						_, _ = fmt.Sscanf(line, "*%d", &count)
						args := make([]string, count)
						for i := 0; i < count; i++ {
							_, _ = reader.ReadString('\n')
							v, _ := reader.ReadString('\n')
							args[i] = strings.TrimSpace(v)
						}

						if len(args) == 0 {
							continue
						}

						cmd := strings.ToUpper(args[0])
						mu.Lock()
						switch cmd {
						case "PING":
							_, _ = c.Write([]byte("+PONG\r\n"))
						case "SET", "SETEX":
							key := args[1]
							var val string
							if cmd == "SETEX" {
								val = args[3]
							} else {
								val = args[2]
							}
							kvs[key] = val
							_, _ = c.Write([]byte("+OK\r\n"))
						case "GET":
							key := args[1]
							val, ok := kvs[key]
							if !ok {
								_, _ = c.Write([]byte("$-1\r\n"))
							} else {
								_, _ = c.Write([]byte(fmt.Sprintf("$%d\r\n%s\r\n", len(val), val)))
							}
						case "DEL":
							deleted := 0
							for _, k := range args[1:] {
								if _, ok := kvs[k]; ok {
									delete(kvs, k)
									deleted++
								}
							}
							_, _ = c.Write([]byte(fmt.Sprintf(":%d\r\n", deleted)))
						case "EXISTS":
							exists := 0
							for _, k := range args[1:] {
								if _, ok := kvs[k]; ok {
									exists++
								}
							}
							_, _ = c.Write([]byte(fmt.Sprintf(":%d\r\n", exists)))
						case "INCR":
							key := args[1]
							num, _ := strconv.ParseInt(kvs[key], 10, 64)
							num++
							kvs[key] = strconv.FormatInt(num, 10)
							_, _ = c.Write([]byte(fmt.Sprintf(":%d\r\n", num)))
						case "HSET":
							key := args[1]
							if hashes[key] == nil {
								hashes[key] = make(map[string]string)
							}
							hashes[key][args[2]] = args[3]
							_, _ = c.Write([]byte(":1\r\n"))
						case "HGET":
							key := args[1]
							m := hashes[key]
							val, ok := m[args[2]]
							if !ok {
								_, _ = c.Write([]byte("$-1\r\n"))
							} else {
								_, _ = c.Write([]byte(fmt.Sprintf("$%d\r\n%s\r\n", len(val), val)))
							}
						case "HGETALL":
							key := args[1]
							m := hashes[key]
							var b strings.Builder
							b.WriteString(fmt.Sprintf("*%d\r\n", len(m)*2))
							for k, v := range m {
								b.WriteString(fmt.Sprintf("$%d\r\n%s\r\n$%d\r\n%s\r\n", len(k), k, len(v), v))
							}
							_, _ = c.Write([]byte(b.String()))
						case "LPUSH":
							key := args[1]
							for i := 2; i < len(args); i++ {
								lists[key] = append([]string{args[i]}, lists[key]...)
							}
							_, _ = c.Write([]byte(fmt.Sprintf(":%d\r\n", len(lists[key]))))
						case "RPOP":
							key := args[1]
							l := lists[key]
							if len(l) == 0 {
								_, _ = c.Write([]byte("$-1\r\n"))
							} else {
								item := l[len(l)-1]
								lists[key] = l[:len(l)-1]
								_, _ = c.Write([]byte(fmt.Sprintf("$%d\r\n%s\r\n", len(item), item)))
							}
						case "PUBLISH":
							_, _ = c.Write([]byte(":1\r\n"))
						default:
							_, _ = c.Write([]byte("+OK\r\n"))
						}
						mu.Unlock()
					}
				}
			}(conn)
		}
	}()

	return listener.Addr().String(), func() {
		close(stopCh)
		_ = listener.Close()
	}
}

func TestRedisClient_Operations(t *testing.T) {
	addr, cleanup := startMockRedisServer(t)
	defer cleanup()

	client, err := redis.NewClient(redis.Config{Address: addr})
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()

	ctx := context.Background()

	// 1. Ping
	if err := client.Ping(ctx); err != nil {
		t.Fatalf("Ping failed: %v", err)
	}

	// 2. String Get & Set
	err = client.Set(ctx, "session:abc", "token_value_xyz", 10*time.Minute)
	if err != nil {
		t.Fatalf("Set failed: %v", err)
	}

	val, err := client.Get(ctx, "session:abc")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if val != "token_value_xyz" {
		t.Errorf("expected token_value_xyz, got %s", val)
	}

	// 3. Incr
	n, err := client.Incr(ctx, "visit_count")
	if err != nil || n != 1 {
		t.Errorf("expected incr 1, got %d (err: %v)", n, err)
	}

	// 4. Hashes (HSet, HGet, HGetAll)
	err = client.HSet(ctx, "user:1", "name", "Antigravity")
	if err != nil {
		t.Fatalf("HSet failed: %v", err)
	}
	err = client.HSet(ctx, "user:1", "role", "AI Assistant")
	if err != nil {
		t.Fatalf("HSet role failed: %v", err)
	}

	name, err := client.HGet(ctx, "user:1", "name")
	if err != nil || name != "Antigravity" {
		t.Errorf("expected Antigravity, got %s", name)
	}

	all, err := client.HGetAll(ctx, "user:1")
	if err != nil || len(all) != 2 {
		t.Errorf("expected 2 hash fields, got %v", all)
	}

	// 5. Lists (LPush, RPop)
	count, err := client.LPush(ctx, "queue:jobs", "job-1", "job-2")
	if err != nil || count != 2 {
		t.Errorf("expected 2 items, got %d", count)
	}

	popped, err := client.RPop(ctx, "queue:jobs")
	if err != nil || popped != "job-1" {
		t.Errorf("expected popped job-1, got %s", popped)
	}

	// 6. Pub/Sub Publish
	pubCount, err := client.Publish(ctx, "notifications", "Hello Subscribers")
	if err != nil || pubCount != 1 {
		t.Errorf("expected 1 published subscriber, got %d", pubCount)
	}
}
