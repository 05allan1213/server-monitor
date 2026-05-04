package kafka

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"

	"github.com/IBM/sarama"
	"go.uber.org/zap"
)

type AlertProcessor interface {
	Process(ctx context.Context, event AlertEvent) error
}

type ConsumerObserver interface {
	ObserveKafkaMessage(result string)
}

const (
	MessageProcessed    = "processed"
	MessageInvalidJSON  = "invalid_json"
	MessagePermanentErr = "permanent_error"
	MessageProcessError = "process_error"
)

type permanentError struct {
	err error
}

func (e permanentError) Error() string {
	return e.err.Error()
}

func (e permanentError) Unwrap() error {
	return e.err
}

func Permanent(err error) error {
	if err == nil {
		return nil
	}
	if IsPermanent(err) {
		return err
	}
	return permanentError{err: err}
}

func IsPermanent(err error) bool {
	var target permanentError
	return errors.As(err, &target)
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

func (c *Consumer) Consume(ctx context.Context, onReady, onNotReady func()) error {
	if c == nil || c.group == nil {
		return errors.New("kafka consumer is not initialized")
	}

	c.handler.onReady = onReady
	c.handler.onNotReady = onNotReady
	for ctx.Err() == nil {
		if err := c.group.Consume(ctx, c.topics, c.handler); err != nil {
			c.handler.notifyNotReady()
			return fmt.Errorf("consume kafka topics: %w", err)
		}
	}
	c.handler.notifyNotReady()
	return nil
}

func (c *Consumer) Close() error {
	if c == nil || c.group == nil {
		return nil
	}
	return c.group.Close()
}

type consumerGroupHandler struct {
	processor  AlertProcessor
	observer   ConsumerObserver
	observerMu sync.RWMutex
	onReady    func()
	onNotReady func()
}

func (c *Consumer) SetObserver(observer ConsumerObserver) {
	if c == nil || c.handler == nil {
		return
	}
	c.handler.observerMu.Lock()
	defer c.handler.observerMu.Unlock()
	c.handler.observer = observer
}

func (h *consumerGroupHandler) Setup(sarama.ConsumerGroupSession) error {
	if h.onReady != nil {
		h.onReady()
	}
	return nil
}

func (h *consumerGroupHandler) Cleanup(sarama.ConsumerGroupSession) error {
	h.notifyNotReady()
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
	defer func() {
		if recovered := recover(); recovered != nil {
			zap.L().Error("process alert event panic recovered, skipping offset commit",
				zap.String("topic", msg.Topic),
				zap.Int32("partition", msg.Partition),
				zap.Int64("offset", msg.Offset),
				zap.String("fingerprint", event.Fingerprint),
				zap.String("status", event.Status),
				zap.Any("panic", recovered),
			)
			h.observe(MessageProcessError)
		}
	}()

	if err := json.Unmarshal(msg.Value, &event); err != nil {
		zap.L().Warn("unmarshal alert event failed",
			zap.String("topic", msg.Topic),
			zap.Int32("partition", msg.Partition),
			zap.Int64("offset", msg.Offset),
			zap.Error(err),
		)
		h.observe(MessageInvalidJSON)
		marker.MarkMessage(msg, "")
		return
	}

	if err := h.processor.Process(ctx, event); err != nil {
		if IsPermanent(err) {
			zap.L().Warn("process alert event failed permanently, committing offset",
				zap.String("topic", msg.Topic),
				zap.Int32("partition", msg.Partition),
				zap.Int64("offset", msg.Offset),
				zap.String("fingerprint", event.Fingerprint),
				zap.String("status", event.Status),
				zap.Error(err),
			)
			h.observe(MessagePermanentErr)
			marker.MarkMessage(msg, "")
			return
		}

		zap.L().Error("process alert event failed, skipping offset commit",
			zap.String("topic", msg.Topic),
			zap.Int32("partition", msg.Partition),
			zap.Int64("offset", msg.Offset),
			zap.String("fingerprint", event.Fingerprint),
			zap.String("status", event.Status),
			zap.Error(err),
		)
		h.observe(MessageProcessError)
		return
	}

	h.observe(MessageProcessed)
	marker.MarkMessage(msg, "")
}

func (h *consumerGroupHandler) notifyNotReady() {
	if h.onNotReady != nil {
		h.onNotReady()
	}
}

func (h *consumerGroupHandler) observe(result string) {
	h.observerMu.RLock()
	observer := h.observer
	h.observerMu.RUnlock()
	if observer == nil {
		return
	}
	observer.ObserveKafkaMessage(result)
}
