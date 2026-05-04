package alert

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"
)

const (
	StatusFiring   = "firing"
	StatusResolved = "resolved"
)

type Event struct {
	Type         string            `json:"type"`
	Fingerprint  string            `json:"fingerprint"`
	Status       string            `json:"status"`
	Labels       map[string]string `json:"labels"`
	Annotations  map[string]string `json:"annotations"`
	StartsAt     time.Time         `json:"startsAt"`
	EndsAt       time.Time         `json:"endsAt"`
	GeneratorURL string            `json:"generatorURL,omitempty"`
	ReceivedAt   time.Time         `json:"receivedAt"`
}

type ActiveAlert struct {
	Event        Event     `json:"event"`
	AggregateKey string    `json:"aggregate_key"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type Processor struct {
	mu           sync.Mutex
	seen         map[string]struct{}
	activeAlerts map[string]ActiveAlert
	stats        map[string]int64
}

func NewProcessor() *Processor {
	return &Processor{
		seen:         make(map[string]struct{}),
		activeAlerts: make(map[string]ActiveAlert),
		stats:        make(map[string]int64),
	}
}

func (p *Processor) Process(_ context.Context, event Event) error {
	if p == nil {
		return errors.New("alert processor is nil")
	}
	if err := validateEvent(event); err != nil {
		return err
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	dedupKey := DedupKey(event)
	if _, ok := p.seen[dedupKey]; ok {
		return nil
	}
	p.seen[dedupKey] = struct{}{}

	switch event.Status {
	case StatusFiring:
		p.activeAlerts[event.Fingerprint] = ActiveAlert{
			Event:        event,
			AggregateKey: AggregateKey(event),
			UpdatedAt:    receivedAtOrNow(event),
		}
		p.stats[alertNameOrFallback(event)]++
	case StatusResolved:
		delete(p.activeAlerts, event.Fingerprint)
	}
	return nil
}

func validateEvent(event Event) error {
	if event.Fingerprint == "" {
		return errors.New("alert fingerprint is required")
	}
	if event.Status != StatusFiring && event.Status != StatusResolved {
		return fmt.Errorf("unsupported alert status %q", event.Status)
	}
	return nil
}

func (p *Processor) ActiveAlerts() map[string]ActiveAlert {
	p.mu.Lock()
	defer p.mu.Unlock()

	result := make(map[string]ActiveAlert, len(p.activeAlerts))
	for key, value := range p.activeAlerts {
		result[key] = value
	}
	return result
}

func (p *Processor) Stats() map[string]int64 {
	p.mu.Lock()
	defer p.mu.Unlock()

	result := make(map[string]int64, len(p.stats))
	for key, value := range p.stats {
		result[key] = value
	}
	return result
}

func DedupKey(event Event) string {
	return strings.Join([]string{
		"alert",
		"dedup",
		event.Fingerprint,
		event.Status,
	}, ":")
}

func AggregateKey(event Event) string {
	alertName := strings.TrimSpace(event.Labels["alertname"])
	instance := strings.TrimSpace(event.Labels["instance"])
	switch {
	case alertName != "" && instance != "":
		return alertName + ":" + instance
	case alertName != "":
		return alertName
	default:
		return event.Fingerprint
	}
}

func alertNameOrFallback(event Event) string {
	alertName := strings.TrimSpace(event.Labels["alertname"])
	if alertName != "" {
		return alertName
	}
	return event.Fingerprint
}

func receivedAtOrNow(event Event) time.Time {
	if !event.ReceivedAt.IsZero() {
		return event.ReceivedAt
	}
	return time.Now().UTC()
}
