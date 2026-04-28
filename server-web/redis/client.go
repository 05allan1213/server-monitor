package rediscache

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

type Client struct {
	client  *redis.Client
	enabled bool
}

func NewClient(addr, password string, db int) *Client {
	if addr == "" {
		return &Client{}
	}

	return &Client{
		client: redis.NewClient(&redis.Options{
			Addr:     addr,
			Password: password,
			DB:       db,
		}),
		enabled: true,
	}
}

func (c *Client) Enabled() bool {
	return c != nil && c.enabled && c.client != nil
}

func (c *Client) Ping(ctx context.Context) error {
	if !c.Enabled() {
		return nil
	}

	if err := c.client.Ping(ctx).Err(); err != nil {
		return fmt.Errorf("redis ping failed: %w", err)
	}

	return nil
}

func (c *Client) Get(ctx context.Context, key string) ([]byte, bool) {
	if !c.Enabled() {
		return nil, false
	}

	value, err := c.client.Get(ctx, key).Bytes()
	if err != nil {
		return nil, false
	}

	return value, true
}

func (c *Client) Set(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	if !c.Enabled() {
		return nil
	}

	return c.client.Set(ctx, key, value, ttl).Err()
}

func (c *Client) HSet(ctx context.Context, key, field string, value []byte) error {
	if !c.Enabled() {
		return nil
	}

	return c.client.HSet(ctx, key, field, value).Err()
}

func (c *Client) HDel(ctx context.Context, key string, fields ...string) error {
	if !c.Enabled() {
		return nil
	}

	return c.client.HDel(ctx, key, fields...).Err()
}
