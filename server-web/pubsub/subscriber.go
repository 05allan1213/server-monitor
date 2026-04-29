package pubsub

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	rediscache "server-web/redis"
)

const (
	reconnectInitialDelay = 1 * time.Second
	reconnectMaxDelay     = 30 * time.Second
)

type Subscriber struct {
	redisClient *rediscache.Client
	hub         *Hub
	channel     string
}

func NewSubscriber(redisClient *rediscache.Client, hub *Hub, channel string) *Subscriber {
	return &Subscriber{
		redisClient: redisClient,
		hub:         hub,
		channel:     channel,
	}
}

func (s *Subscriber) Run(ctx context.Context) {
	if s == nil || s.redisClient == nil || !s.redisClient.Enabled() || s.hub == nil || s.channel == "" {
		return
	}

	delay := reconnectInitialDelay

	for {
		err := s.subscribeOnce(ctx)
		if err == nil {
			return
		}

		slog.Warn("subscribe failed, reconnecting", "channel", s.channel, "error", err, "delay", delay)

		select {
		case <-ctx.Done():
			return
		case <-time.After(delay):
		}

		delay *= 2
		if delay > reconnectMaxDelay {
			delay = reconnectMaxDelay
		}
	}
}

func (s *Subscriber) subscribeOnce(ctx context.Context) error {
	messages, err := s.redisClient.Subscribe(ctx, s.channel)
	if err != nil {
		return err
	}

	for {
		select {
		case <-ctx.Done():
			return nil
		case message, ok := <-messages:
			if !ok {
				return fmt.Errorf("subscription channel closed")
			}
			s.hub.PublishLocal([]byte(message))
		}
	}
}
