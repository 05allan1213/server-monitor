package pubsub

import (
	"context"
	"log"

	rediscache "server-web/redis"
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

	messages, err := s.redisClient.Subscribe(ctx, s.channel)
	if err != nil {
		log.Printf("subscribe to %s failed: %v", s.channel, err)
		return
	}

	for {
		select {
		case <-ctx.Done():
			return
		case message, ok := <-messages:
			if !ok {
				return
			}
			s.hub.PublishLocal([]byte(message))
		}
	}
}
