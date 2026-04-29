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

func (c *Client) HGetAll(ctx context.Context, key string) (map[string]string, error) {
	if !c.Enabled() {
		return map[string]string{}, nil
	}

	result, err := c.client.HGetAll(ctx, key).Result()
	if err != nil {
		return nil, err
	}

	return result, nil
}

func (c *Client) Publish(ctx context.Context, channel string, payload []byte) error {
	if !c.Enabled() {
		return nil
	}

	return c.client.Publish(ctx, channel, payload).Err()
}

func (c *Client) Subscribe(ctx context.Context, channel string) (<-chan string, error) {
	if !c.Enabled() {
		return nil, nil
	}

	pubsub := c.client.Subscribe(ctx, channel)
	if _, err := pubsub.Receive(ctx); err != nil {
		return nil, err
	}

	source := pubsub.Channel()
	output := make(chan string)

	go func() {
		defer close(output)
		defer pubsub.Close()

		for {
			select {
			case <-ctx.Done():
				return
			case message, ok := <-source:
				if !ok {
					return
				}
				select {
				case <-ctx.Done():
					return
				case output <- message.Payload:
				}
			}
		}
	}()

	return output, nil
}
