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

func (c *Client) LPushTrim(ctx context.Context, key string, maxLen int64, value []byte) error {
	if !c.Enabled() {
		return errors.New("redis is not enabled")
	}
	if maxLen <= 0 {
		return errors.New("max list length must be positive")
	}

	pipe := c.client.TxPipeline()
	pipe.LPush(ctx, key, value)
	pipe.LTrim(ctx, key, 0, maxLen-1)

	_, err := pipe.Exec(ctx)
	return err
}

func (c *Client) LRange(ctx context.Context, key string, start, stop int64) ([]string, error) {
	if !c.Enabled() {
		return nil, errors.New("redis is not enabled")
	}

	return c.client.LRange(ctx, key, start, stop).Result()
}

func (c *Client) XAddMaxLen(ctx context.Context, key string, maxLen int64, value []byte) error {
	if !c.Enabled() {
		return errors.New("redis is not enabled")
	}
	if maxLen <= 0 {
		return errors.New("max stream length must be positive")
	}

	return c.client.XAdd(ctx, &redis.XAddArgs{
		Stream: key,
		MaxLen: maxLen,
		Approx: true,
		Values: map[string]interface{}{
			AlertEventPayload: string(value),
		},
	}).Err()
}

func (c *Client) XRevRangeN(ctx context.Context, key string, count int64) ([]string, error) {
	if !c.Enabled() {
		return nil, errors.New("redis is not enabled")
	}
	if count <= 0 {
		return nil, errors.New("stream count must be positive")
	}

	messages, err := c.client.XRevRangeN(ctx, key, "+", "-", count).Result()
	if err != nil {
		return nil, err
	}

	values := make([]string, 0, len(messages))
	for _, message := range messages {
		raw, ok := message.Values[AlertEventPayload]
		if !ok {
			slog.Warn("skip alert event stream message without payload", "key", key, "id", message.ID)
			continue
		}

		switch value := raw.(type) {
		case string:
			values = append(values, value)
		case []byte:
			values = append(values, string(value))
		default:
			slog.Warn("skip alert event stream message with invalid payload", "key", key, "id", message.ID)
		}
	}

	return values, nil
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
