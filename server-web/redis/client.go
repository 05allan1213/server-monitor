package rediscache

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
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
	return c != nil && c.enabled
}

func (c *Client) Ping(ctx context.Context) error {
	if !c.Enabled() {
		return errors.New("redis is not enabled")
	}

	return c.client.Ping(ctx).Err()
}

func (c *Client) Get(ctx context.Context, key string) ([]byte, bool) {
	if !c.Enabled() {
		return nil, false
	}

	value, err := c.client.Get(ctx, key).Bytes()
	if err != nil {
		if err != redis.Nil {
			slog.Error("redis get failed", "key", key, "error", err)
		}
		return nil, false
	}

	return value, true
}

func (c *Client) Set(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	if !c.Enabled() {
		return errors.New("redis is not enabled")
	}

	return c.client.Set(ctx, key, value, ttl).Err()
}

func (c *Client) HSet(ctx context.Context, key, field string, value []byte) error {
	if !c.Enabled() {
		return errors.New("redis is not enabled")
	}

	return c.client.HSet(ctx, key, field, value).Err()
}

func (c *Client) HDel(ctx context.Context, key, field string) error {
	if !c.Enabled() {
		return errors.New("redis is not enabled")
	}

	return c.client.HDel(ctx, key, field).Err()
}

func (c *Client) HGetAll(ctx context.Context, key string) (map[string]string, error) {
	if !c.Enabled() {
		return nil, errors.New("redis is not enabled")
	}

	return c.client.HGetAll(ctx, key).Result()
}

func (c *Client) Publish(ctx context.Context, channel string, message []byte) error {
	if !c.Enabled() {
		return errors.New("redis is not enabled")
	}

	return c.client.Publish(ctx, channel, message).Err()
}

func (c *Client) Subscribe(ctx context.Context, channel string) (<-chan string, error) {
	if !c.Enabled() {
		return nil, errors.New("redis is not enabled")
	}

	pubsub := c.client.Subscribe(ctx, channel)

	if err := pubsub.Ping(ctx); err != nil {
		pubsub.Close()
		return nil, fmt.Errorf("subscribe ping failed: %w", err)
	}

	source := pubsub.Channel()
	output := make(chan string, 32)

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
