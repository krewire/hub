package redis

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/krewire/krewire/errs"
)

// ErrNil is returned when a key does not exist.
var ErrNil = errors.New("redis: nil")

const (
	// DefaultAddress is the standard fallback Redis listen address.
	DefaultAddress = "127.0.0.1:6379"

	// DefaultTimeout is the standard network I/O timeout.
	DefaultTimeout = 5 * time.Second
)

// Config contains options for connecting to Redis/KeyDB/Valkey servers.
type Config struct {
	Address  string // "host:port", default: DefaultAddress
	Password string
	DB       int // Database index (default: 0)
	Timeout  time.Duration
}

// Client represents a pure-Go Redis client.
type Client struct {
	mu      sync.Mutex
	config  Config
	conn    net.Conn
	reader  *bufio.Reader
	timeout time.Duration
	closed  bool
}

// NewClient initializes a new Redis Client.
func NewClient(config Config) (*Client, error) {
	if config.Address == "" {
		config.Address = DefaultAddress
	}
	if config.Timeout <= 0 {
		config.Timeout = DefaultTimeout
	}

	return &Client{
		config:  config,
		timeout: config.Timeout,
	}, nil
}

func (c *Client) getConn() (net.Conn, *bufio.Reader, error) {
	if c.closed {
		return nil, nil, errs.FailureError("redis: client is closed")
	}

	if c.conn != nil {
		return c.conn, c.reader, nil
	}

	conn, err := net.DialTimeout("tcp", c.config.Address, c.timeout)
	if err != nil {
		return nil, nil, errs.FailureError(fmt.Sprintf("redis: failed to connect to %s: %v", c.config.Address, err))
	}

	c.conn = conn
	c.reader = bufio.NewReader(conn)

	if c.config.Password != "" {
		if _, err := c.execRaw("AUTH", c.config.Password); err != nil {
			c.closeConn()
			return nil, nil, errs.FailureError(fmt.Sprintf("redis: auth failed: %v", err))
		}
	}

	if c.config.DB > 0 {
		if _, err := c.execRaw("SELECT", strconv.Itoa(c.config.DB)); err != nil {
			c.closeConn()
			return nil, nil, errs.FailureError(fmt.Sprintf("redis: select db failed: %v", err))
		}
	}

	return c.conn, c.reader, nil
}

func (c *Client) closeConn() {
	if c.conn != nil {
		_ = c.conn.Close()
		c.conn = nil
		c.reader = nil
	}
}

func (c *Client) execRaw(args ...string) (interface{}, error) {
	conn, reader, err := c.getConn()
	if err != nil {
		return nil, err
	}

	_ = conn.SetDeadline(time.Now().Add(c.timeout))
	cmd := EncodeRESP(args...)
	if _, err := conn.Write(cmd); err != nil {
		c.closeConn()
		return nil, err
	}

	res, err := ReadRESP(reader)
	if err != nil {
		c.closeConn()
		return nil, err
	}
	return res, nil
}

// Do executes an arbitrary Redis command.
func (c *Client) Do(ctx context.Context, args ...string) (interface{}, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.execRaw(args...)
}

// Ping checks if the Redis server is alive.
func (c *Client) Ping(ctx context.Context) error {
	res, err := c.Do(ctx, "PING")
	if err != nil {
		return err
	}
	if str, ok := res.(string); ok && strings.ToUpper(str) == "PONG" {
		return nil
	}
	return nil
}

// Close closes the connection pool.
func (c *Client) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.closed = true
	c.closeConn()
	return nil
}

// ----------------------------------------------------------------------------
// Key-Value Strings
// ----------------------------------------------------------------------------

func (c *Client) Get(ctx context.Context, key string) (string, error) {
	res, err := c.Do(ctx, "GET", key)
	if err != nil {
		return "", err
	}
	if res == nil {
		return "", ErrNil
	}
	str, ok := res.(string)
	if !ok {
		return "", ErrNil
	}
	return str, nil
}

func (c *Client) Set(ctx context.Context, key string, val string, ttl time.Duration) error {
	var err error
	if ttl > 0 {
		seconds := strconv.Itoa(int(ttl.Seconds()))
		_, err = c.Do(ctx, "SETEX", key, seconds, val)
	} else {
		_, err = c.Do(ctx, "SET", key, val)
	}
	return err
}

func (c *Client) Del(ctx context.Context, keys ...string) (int64, error) {
	args := append([]string{"DEL"}, keys...)
	res, err := c.Do(ctx, args...)
	if err != nil {
		return 0, err
	}
	if count, ok := res.(int64); ok {
		return count, nil
	}
	return 0, nil
}

func (c *Client) Exists(ctx context.Context, keys ...string) (int64, error) {
	args := append([]string{"EXISTS"}, keys...)
	res, err := c.Do(ctx, args...)
	if err != nil {
		return 0, err
	}
	if count, ok := res.(int64); ok {
		return count, nil
	}
	return 0, nil
}

func (c *Client) Incr(ctx context.Context, key string) (int64, error) {
	res, err := c.Do(ctx, "INCR", key)
	if err != nil {
		return 0, err
	}
	if n, ok := res.(int64); ok {
		return n, nil
	}
	return 0, nil
}

// ----------------------------------------------------------------------------
// Hashes
// ----------------------------------------------------------------------------

func (c *Client) HGet(ctx context.Context, key, field string) (string, error) {
	res, err := c.Do(ctx, "HGET", key, field)
	if err != nil {
		return "", err
	}
	if res == nil {
		return "", ErrNil
	}
	return res.(string), nil
}

func (c *Client) HSet(ctx context.Context, key, field, val string) error {
	_, err := c.Do(ctx, "HSET", key, field, val)
	return err
}

func (c *Client) HGetAll(ctx context.Context, key string) (map[string]string, error) {
	res, err := c.Do(ctx, "HGETALL", key)
	if err != nil {
		return nil, err
	}
	arr, ok := res.([]interface{})
	if !ok || len(arr) == 0 {
		return make(map[string]string), nil
	}

	result := make(map[string]string, len(arr)/2)
	for i := 0; i < len(arr)-1; i += 2 {
		k, _ := arr[i].(string)
		v, _ := arr[i+1].(string)
		result[k] = v
	}
	return result, nil
}

func (c *Client) HDel(ctx context.Context, key string, fields ...string) (int64, error) {
	args := append([]string{"HDEL", key}, fields...)
	res, err := c.Do(ctx, args...)
	if err != nil {
		return 0, err
	}
	if n, ok := res.(int64); ok {
		return n, nil
	}
	return 0, nil
}

// ----------------------------------------------------------------------------
// Lists
// ----------------------------------------------------------------------------

func (c *Client) LPush(ctx context.Context, key string, values ...string) (int64, error) {
	args := append([]string{"LPUSH", key}, values...)
	res, err := c.Do(ctx, args...)
	if err != nil {
		return 0, err
	}
	if n, ok := res.(int64); ok {
		return n, nil
	}
	return 0, nil
}

func (c *Client) RPop(ctx context.Context, key string) (string, error) {
	res, err := c.Do(ctx, "RPOP", key)
	if err != nil {
		return "", err
	}
	if res == nil {
		return "", ErrNil
	}
	return res.(string), nil
}

func (c *Client) LLen(ctx context.Context, key string) (int64, error) {
	res, err := c.Do(ctx, "LLEN", key)
	if err != nil {
		return 0, err
	}
	if n, ok := res.(int64); ok {
		return n, nil
	}
	return 0, nil
}

// ----------------------------------------------------------------------------
// Pub / Sub
// ----------------------------------------------------------------------------

func (c *Client) Publish(ctx context.Context, channel, message string) (int64, error) {
	res, err := c.Do(ctx, "PUBLISH", channel, message)
	if err != nil {
		return 0, err
	}
	if count, ok := res.(int64); ok {
		return count, nil
	}
	return 0, nil
}
