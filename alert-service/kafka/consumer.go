package kafka

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/IBM/sarama"
	"go.uber.org/zap"
)

type AlertProcessor interface {
	Process(ctx context.Context, event AlertEvent) error
}

type Consumer struct {
	group   sarama.ConsumerGroup
	topics  []string
	handler *consumerGroupHandler
}

func NewConsumer(brokers []string, groupID string, processor AlertProcessor) (*Consumer, error) {
	if len(brokers) == 0 {
		return nil, errors.New("kafka brokers is empty")
	}
	if groupID == "" {
		return nil, errors.New("kafka group id is empty")
	}
	if processor == nil {
		return nil, errors.New("alert processor is required")
	}

	config := sarama.NewConfig()
	config.Consumer.Group.Rebalance.GroupStrategies = []sarama.BalanceStrategy{sarama.NewBalanceStrategyRange()}
	config.Consumer.Offsets.Initial = sarama.OffsetOldest

	group, err := sarama.NewConsumerGroup(brokers, groupID, config)
	if err != nil {
		return nil, fmt.Errorf("create kafka consumer group: %w", err)
	}

	return &Consumer{
		group:   group,
		topics:  []string{TopicAlertEvents},
		handler: &consumerGroupHandler{processor: processor},
	}, nil
}

func (c *Consumer) Consume(ctx context.Context, onReady func()) error {
	if c == nil || c.group == nil {
		return errors.New("kafka consumer is not initialized")
	}

	c.handler.onReady = onReady
	for ctx.Err() == nil {
		if err := c.group.Consume(ctx, c.topics, c.handler); err != nil {
			return fmt.Errorf("consume kafka topics: %w", err)
		}
	}
	return nil
}

func (c *Consumer) Close() error {
	if c == nil || c.group == nil {
		return nil
	}
	return c.group.Close()
}

type consumerGroupHandler struct {
	processor AlertProcessor
	onReady   func()
}

func (h *consumerGroupHandler) Setup(sarama.ConsumerGroupSession) error {
	if h.onReady != nil {
		h.onReady()
	}
	return nil
}

func (h *consumerGroupHandler) Cleanup(sarama.ConsumerGroupSession) error {
	return nil
}

func (h *consumerGroupHandler) ConsumeClaim(session sarama.ConsumerGroupSession, claim sarama.ConsumerGroupClaim) error {
	for msg := range claim.Messages() {
		h.processMessage(session.Context(), session, msg)
	}
	return nil
}

type messageMarker interface {
	MarkMessage(*sarama.ConsumerMessage, string)
}

func (h *consumerGroupHandler) processMessage(ctx context.Context, marker messageMarker, msg *sarama.ConsumerMessage) {
	var event AlertEvent
	if err := json.Unmarshal(msg.Value, &event); err != nil {
		zap.L().Warn("unmarshal alert event failed",
			zap.String("topic", msg.Topic),
			zap.Int32("partition", msg.Partition),
			zap.Int64("offset", msg.Offset),
			zap.Error(err),
		)
		marker.MarkMessage(msg, "")
		return
	}

	if err := h.processor.Process(ctx, event); err != nil {
		zap.L().Error("process alert event failed, skipping offset commit",
			zap.String("fingerprint", event.Fingerprint),
			zap.String("status", event.Status),
			zap.Error(err),
		)
		return
	}

	marker.MarkMessage(msg, "")
}
