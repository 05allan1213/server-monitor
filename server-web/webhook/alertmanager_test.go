package webhook

import (
	"testing"
	"time"
)

func TestNewAlertEvent(t *testing.T) {
	startsAt := time.Date(2026, time.April, 29, 5, 0, 0, 0, time.UTC)
	endsAt := startsAt.Add(2 * time.Minute)
	receivedAt := startsAt.Add(30 * time.Second)

	alert := AlertRecord{
		Status:      "firing",
		Fingerprint: "fp-1",
		Labels: map[string]string{
			"alertname": "HighCPU",
			"severity":  "warning",
		},
		Annotations: map[string]string{
			"summary": "CPU high",
		},
		StartsAt:     startsAt,
		EndsAt:       endsAt,
		GeneratorURL: "http://prometheus:9090/graph",
	}

	event := NewAlertEvent(alert, receivedAt)

	if event.Status != alert.Status {
		t.Fatalf("status mismatch: got %q want %q", event.Status, alert.Status)
	}
	if event.Fingerprint != alert.Fingerprint {
		t.Fatalf("fingerprint mismatch: got %q want %q", event.Fingerprint, alert.Fingerprint)
	}
	if !event.StartsAt.Equal(alert.StartsAt) {
		t.Fatalf("startsAt mismatch: got %v want %v", event.StartsAt, alert.StartsAt)
	}
	if !event.EndsAt.Equal(alert.EndsAt) {
		t.Fatalf("endsAt mismatch: got %v want %v", event.EndsAt, alert.EndsAt)
	}
	if !event.ReceivedAt.Equal(receivedAt) {
		t.Fatalf("receivedAt mismatch: got %v want %v", event.ReceivedAt, receivedAt)
	}
	if event.Labels["alertname"] != "HighCPU" {
		t.Fatalf("labels not preserved: got %q", event.Labels["alertname"])
	}
	if event.Annotations["summary"] != "CPU high" {
		t.Fatalf("annotations not preserved: got %q", event.Annotations["summary"])
	}
}
