package alert

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"alert-service/kafka"
)

const (
	RedisActiveAlertsKey = "alert:active"
	// RedisStatsKey stores cumulative firing counts by alert name. Resolved
	// events only clear active state; they do not decrement these counters.
	RedisStatsKey   = "alert:stats"
	DefaultDedupTTL = 5 * time.Minute
)

type RedisClient interface {
	SetNX(ctx context.Context, key string, value []byte, ttl time.Duration) (bool, error)
	HGet(ctx context.Context, key, field string) ([]byte, bool, error)
	HSet(ctx context.Context, key, field string, value []byte) error
	HDel(ctx context.Context, key, field string) error
	HIncrBy(ctx context.Context, key, field string, incr int64) error
	Del(ctx context.Context, key string) error
}

type StoreObserver interface {
	ObserveAlertEvent(status, result string)
}

const (
	EventStored  = "stored"
	EventDeduped = "deduped"
	EventFailed  = "failed"
)

type Store struct {
	client   RedisClient
	dedupTTL time.Duration
	observer StoreObserver
}

var _ kafka.AlertProcessor = (*Store)(nil)

func NewStore(client RedisClient, dedupTTL time.Duration, observers ...StoreObserver) *Store {
	if dedupTTL <= 0 {
		dedupTTL = DefaultDedupTTL
	}
	store := &Store{
		client:   client,
		dedupTTL: dedupTTL,
	}
	if len(observers) > 0 {
		store.observer = observers[0]
	}
	return store
}

func (s *Store) Process(ctx context.Context, event kafka.AlertEvent) error {
	if s == nil {
		return errors.New("alert store is nil")
	}
	if s.client == nil {
		return errors.New("redis client is required")
	}

	if err := validateEvent(event); err != nil {
		s.observe(event.Status, EventFailed)
		return kafka.Permanent(err)
	}

	dedupKey := DedupKey(event)
	ok, err := s.client.SetNX(ctx, dedupKey, []byte("1"), s.dedupTTL)
	if err != nil {
		s.observe(event.Status, EventFailed)
		return fmt.Errorf("set alert dedup key: %w", err)
	}
	if !ok {
		s.observe(event.Status, EventDeduped)
		return nil
	}

	if err := s.apply(ctx, event); err != nil {
		rollbackErr := s.client.Del(ctx, dedupKey)
		if rollbackErr != nil {
			s.observe(event.Status, EventFailed)
			return fmt.Errorf("%w; rollback alert dedup key: %v", err, rollbackErr)
		}
		s.observe(event.Status, EventFailed)
		return err
	}
	s.observe(event.Status, EventStored)
	return nil
}

func (s *Store) apply(ctx context.Context, event Event) error {
	switch event.Status {
	case StatusFiring:
		payload, err := json.Marshal(event)
		if err != nil {
			return fmt.Errorf("marshal active alert: %w", err)
		}
		previousValue, existed, err := s.client.HGet(ctx, RedisActiveAlertsKey, event.Fingerprint)
		if err != nil {
			return fmt.Errorf("load previous active alert: %w", err)
		}
		if err := s.client.HSet(ctx, RedisActiveAlertsKey, event.Fingerprint, payload); err != nil {
			return fmt.Errorf("store active alert: %w", err)
		}
		if err := s.client.HIncrBy(ctx, RedisStatsKey, alertNameOrFallback(event), 1); err != nil {
			if rollbackErr := rollbackActiveAlert(ctx, s.client, event.Fingerprint, previousValue, existed); rollbackErr != nil {
				return fmt.Errorf("increment alert stats: %w; rollback active alert: %v", err, rollbackErr)
			}
			return fmt.Errorf("increment alert stats: %w", err)
		}
	case StatusResolved:
		if err := s.client.HDel(ctx, RedisActiveAlertsKey, event.Fingerprint); err != nil {
			return fmt.Errorf("delete active alert: %w", err)
		}
	}
	return nil
}

func rollbackActiveAlert(ctx context.Context, client RedisClient, fingerprint string, previousValue []byte, existed bool) error {
	if existed {
		return client.HSet(ctx, RedisActiveAlertsKey, fingerprint, previousValue)
	}
	return client.HDel(ctx, RedisActiveAlertsKey, fingerprint)
}

func (s *Store) observe(status, result string) {
	if s == nil || s.observer == nil {
		return
	}
	s.observer.ObserveAlertEvent(normalizeStatus(status), result)
}

func normalizeStatus(status string) string {
	switch status {
	case StatusFiring, StatusResolved:
		return status
	default:
		return "unknown"
	}
}
