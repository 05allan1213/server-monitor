package redisstore

import (
	"context"
	"errors"
	"time"

	"github.com/redis/go-redis/v9"
)

type Client struct {
	client  *redis.Client
	enabled bool
}

type Options struct {
	Addr            string
	Password        string
	DB              int
	DialTimeout     time.Duration
	ReadTimeout     time.Duration
	WriteTimeout    time.Duration
	ConnMaxLifetime time.Duration
	ConnMaxIdleTime time.Duration
}

func NewClient(options Options) *Client {
	if options.Addr == "" {
		return &Client{}
	}

	return &Client{
		client: redis.NewClient(&redis.Options{
			Addr:            options.Addr,
			Password:        options.Password,
			DB:              options.DB,
			DialTimeout:     options.DialTimeout,
			ReadTimeout:     options.ReadTimeout,
			WriteTimeout:    options.WriteTimeout,
			ConnMaxLifetime: options.ConnMaxLifetime,
			ConnMaxIdleTime: options.ConnMaxIdleTime,
		}),
		enabled: true,
	}
}

func (c *Client) Enabled() bool {
	return c != nil && c.enabled
}

func (c *Client) Close() error {
	if !c.Enabled() {
		return nil
	}
	return c.client.Close()
}

func (c *Client) Ping(ctx context.Context) error {
	if !c.Enabled() {
		return errors.New("redis is not enabled")
	}
	return c.client.Ping(ctx).Err()
}

func (c *Client) SetNX(ctx context.Context, key string, value []byte, ttl time.Duration) (bool, error) {
	if !c.Enabled() {
		return false, errors.New("redis is not enabled")
	}
	return c.client.SetNX(ctx, key, value, ttl).Result()
}

func (c *Client) HSet(ctx context.Context, key, field string, value []byte) error {
	if !c.Enabled() {
		return errors.New("redis is not enabled")
	}
	return c.client.HSet(ctx, key, field, value).Err()
}

func (c *Client) HGet(ctx context.Context, key, field string) ([]byte, bool, error) {
	if !c.Enabled() {
		return nil, false, errors.New("redis is not enabled")
	}

	value, err := c.client.HGet(ctx, key, field).Bytes()
	if err == nil {
		return value, true, nil
	}
	if errors.Is(err, redis.Nil) {
		return nil, false, nil
	}
	return nil, false, err
}

func (c *Client) HDel(ctx context.Context, key, field string) error {
	if !c.Enabled() {
		return errors.New("redis is not enabled")
	}
	return c.client.HDel(ctx, key, field).Err()
}

func (c *Client) HIncrBy(ctx context.Context, key, field string, incr int64) error {
	if !c.Enabled() {
		return errors.New("redis is not enabled")
	}
	return c.client.HIncrBy(ctx, key, field, incr).Err()
}

func (c *Client) Del(ctx context.Context, key string) error {
	if !c.Enabled() {
		return errors.New("redis is not enabled")
	}
	return c.client.Del(ctx, key).Err()
}
