package webhook

import "time"

type AlertmanagerWebhookRequest struct {
	Receiver string        `json:"receiver"`
	Status   string        `json:"status"`
	Alerts   []AlertRecord `json:"alerts"`
}

type AlertRecord struct {
	Status       string            `json:"status"`
	Fingerprint  string            `json:"fingerprint"`
	Labels       map[string]string `json:"labels"`
	Annotations  map[string]string `json:"annotations"`
	StartsAt     time.Time         `json:"startsAt"`
	EndsAt       time.Time         `json:"endsAt"`
	GeneratorURL string            `json:"generatorURL,omitempty"`
}
