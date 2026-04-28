package rediscache

import (
	"context"
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
