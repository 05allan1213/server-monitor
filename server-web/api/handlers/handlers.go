package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	promclient "server-web/prometheus"
	rediscache "server-web/redis"
	"server-web/webhook"
	ws "server-web/websocket"
)

type cacheClient interface {
	Enabled() bool
	Ping(ctx context.Context) error
	Get(ctx context.Context, key string) ([]byte, bool)
	Set(ctx context.Context, key string, value []byte, ttl time.Duration) error
	HSet(ctx context.Context, key, field string, value []byte) error
	HDel(ctx context.Context, key, field string) error
	HGetAll(ctx context.Context, key string) (map[string]string, error)
	LPushTrim(ctx context.Context, key string, maxLen int64, value []byte) error
	LRange(ctx context.Context, key string, start, stop int64) ([]string, error)
	Publish(ctx context.Context, channel string, message []byte) error
}

type Handler struct {
	promClient     *promclient.Client
	cacheClient    cacheClient
	readyTimeout   time.Duration
	requestTimeout time.Duration
	hostsTTL       time.Duration
	websocketHub   *ws.Hub
}

type response struct {
	Status string      `json:"status"`
	Data   interface{} `json:"data,omitempty"`
	Error  string      `json:"error,omitempty"`
}

const defaultAlertEventsLimit int64 = 8

var validAlertEventStatuses = map[string]struct{}{
	"firing":   {},
	"resolved": {},
}

var validAlertEventSeverities = map[string]struct{}{
	"critical": {},
	"warning":  {},
	"info":     {},
}

var validActiveAlertSeverities = map[string]struct{}{
	"critical": {},
	"warning":  {},
	"info":     {},
}

var validHostStatuses = map[string]struct{}{
	"up":   {},
	"down": {},
}

func NewHandler(promClient *promclient.Client, cacheClient cacheClient, readyTimeout time.Duration, requestTimeout time.Duration, hostsTTL time.Duration, websocketHub *ws.Hub) *Handler {
	return &Handler{
		promClient:     promClient,
		cacheClient:    cacheClient,
		readyTimeout:   readyTimeout,
		requestTimeout: requestTimeout,
		hostsTTL:       hostsTTL,
		websocketHub:   websocketHub,
	}
}

func (h *Handler) Healthz(c *gin.Context) {
	c.JSON(http.StatusOK, response{
		Status: "success",
		Data: gin.H{
			"healthy": true,
		},
	})
}

func (h *Handler) Readyz(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), h.readyTimeout)
	defer cancel()

	dependencies := gin.H{
		"prometheus": "ok",
		"redis":      "disabled",
	}

	var errors []string

	if err := h.promClient.Ready(ctx); err != nil {
		dependencies["prometheus"] = "unreachable"
		errors = append(errors, err.Error())
	}

	if h.cacheClient != nil && h.cacheClient.Enabled() {
		if err := h.cacheClient.Ping(ctx); err != nil {
			dependencies["redis"] = "unreachable"
			errors = append(errors, err.Error())
		} else {
			dependencies["redis"] = "ok"
		}
	}

	if len(errors) > 0 {
		c.JSON(http.StatusServiceUnavailable, response{
			Status: "error",
			Error:  fmt.Sprintf("readiness check failed: %s", strings.Join(errors, "; ")),
			Data: gin.H{
				"ready":        false,
				"dependencies": dependencies,
			},
		})
		return
	}

	c.JSON(http.StatusOK, response{
		Status: "success",
		Data: gin.H{
			"ready":        true,
			"dependencies": dependencies,
		},
	})
}

func (h *Handler) Hosts(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), h.requestTimeout)
	defer cancel()

	statusFilter := parseAlertEventFilter(c.Query("status"), validHostStatuses)
	queryFilter := normalizeHostQuery(c.Query("q"))

	if cachedHosts, ok := h.getCachedHosts(ctx); ok {
		c.JSON(http.StatusOK, response{
			Status: "success",
			Data:   filterHosts(cachedHosts, statusFilter, queryFilter),
		})
		return
	}

	hosts, err := h.promClient.GetHosts(ctx)
	if err != nil {
		c.JSON(http.StatusBadGateway, response{
			Status: "error",
			Error:  err.Error(),
		})
		return
	}

	cacheCtx, cacheCancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cacheCancel()
	h.cacheHosts(cacheCtx, hosts)

	c.JSON(http.StatusOK, response{
		Status: "success",
		Data:   filterHosts(hosts, statusFilter, queryFilter),
	})
}

func (h *Handler) AlertmanagerWebhook(c *gin.Context) {
	if h.cacheClient == nil || !h.cacheClient.Enabled() {
		c.JSON(http.StatusServiceUnavailable, response{
			Status: "error",
			Error:  "redis is required for alert webhook handling",
		})
		return
	}

	var payload webhook.AlertmanagerWebhookRequest
	if err := c.ShouldBindJSON(&payload); err != nil {
		c.JSON(http.StatusBadRequest, response{
			Status: "error",
			Error:  fmt.Sprintf("invalid alertmanager payload: %v", err),
		})
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), h.requestTimeout)
	defer cancel()

	receivedAt := time.Now().UTC()

	for _, alert := range payload.Alerts {
		if alert.Fingerprint == "" {
			continue
		}

		message, err := json.Marshal(alert)
		if err != nil {
			c.JSON(http.StatusInternalServerError, response{
				Status: "error",
				Error:  fmt.Sprintf("marshal alert payload failed: %v", err),
			})
			return
		}

		switch alert.Status {
		case "firing":
			if err := h.cacheClient.HSet(ctx, rediscache.ActiveAlertsKey, alert.Fingerprint, message); err != nil {
				c.JSON(http.StatusBadGateway, response{
					Status: "error",
					Error:  fmt.Sprintf("store active alert failed: %v", err),
				})
				return
			}
		case "resolved":
			if err := h.cacheClient.HDel(ctx, rediscache.ActiveAlertsKey, alert.Fingerprint); err != nil {
				c.JSON(http.StatusBadGateway, response{
					Status: "error",
					Error:  fmt.Sprintf("delete active alert failed: %v", err),
				})
				return
			}
		}

		event, err := json.Marshal(webhook.NewAlertEvent(alert, receivedAt))
		if err != nil {
			c.JSON(http.StatusInternalServerError, response{
				Status: "error",
				Error:  fmt.Sprintf("marshal alert event failed: %v", err),
			})
			return
		}

		if err := h.cacheClient.LPushTrim(ctx, rediscache.AlertEventsKey, rediscache.AlertEventsMax, event); err != nil {
			c.JSON(http.StatusBadGateway, response{
				Status: "error",
				Error:  fmt.Sprintf("store alert event failed: %v", err),
			})
			return
		}

		if err := h.cacheClient.Publish(ctx, rediscache.AlertChannel, event); err != nil {
			// Active state and history are already stored; failing the webhook here
			// would trigger retries and duplicate history entries.
			slog.Warn("publish alert event failed", "fingerprint", alert.Fingerprint, "status", alert.Status, "error", err)
		}
	}

	c.JSON(http.StatusAccepted, response{
		Status: "accepted",
	})
}

func (h *Handler) ActiveAlerts(c *gin.Context) {
	if h.cacheClient == nil || !h.cacheClient.Enabled() {
		c.JSON(http.StatusServiceUnavailable, response{
			Status: "error",
			Error:  "redis is required for active alerts query",
		})
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), h.requestTimeout)
	defer cancel()

	severityFilter := parseAlertEventFilter(c.Query("severity"), validActiveAlertSeverities)

	values, err := h.cacheClient.HGetAll(ctx, rediscache.ActiveAlertsKey)
	if err != nil {
		c.JSON(http.StatusBadGateway, response{
			Status: "error",
			Error:  fmt.Sprintf("load active alerts failed: %v", err),
		})
		return
	}

	alerts := decodeActiveAlerts(values)
	alerts = filterActiveAlerts(alerts, severityFilter)

	sort.Slice(alerts, func(i, j int) bool {
		return alerts[i].StartsAt.After(alerts[j].StartsAt)
	})

	c.JSON(http.StatusOK, response{
		Status: "success",
		Data:   alerts,
	})
}

func (h *Handler) AlertEvents(c *gin.Context) {
	if h.cacheClient == nil || !h.cacheClient.Enabled() {
		c.JSON(http.StatusServiceUnavailable, response{
			Status: "error",
			Error:  "redis is required for alert events query",
		})
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), h.requestTimeout)
	defer cancel()

	limit := parseAlertEventsLimit(c.Query("limit"))
	statusFilter := parseAlertEventFilter(c.Query("status"), validAlertEventStatuses)
	severityFilter := parseAlertEventFilter(c.Query("severity"), validAlertEventSeverities)

	values, err := h.cacheClient.LRange(ctx, rediscache.AlertEventsKey, 0, limit-1)
	if err != nil {
		c.JSON(http.StatusBadGateway, response{
			Status: "error",
			Error:  fmt.Sprintf("load alert events failed: %v", err),
		})
		return
	}

	events := decodeAlertEvents(values)
	events = filterAlertEvents(events, statusFilter, severityFilter)

	c.JSON(http.StatusOK, response{
		Status: "success",
		Data:   events,
	})
}

func (h *Handler) AlertsWebSocket(c *gin.Context) {
	if h.websocketHub == nil {
		c.JSON(http.StatusServiceUnavailable, response{
			Status: "error",
			Error:  "websocket hub is unavailable",
		})
		return
	}

	if err := h.websocketHub.ServeWS(c.Writer, c.Request); err != nil {
		slog.Warn("websocket upgrade failed", "error", err)
	}
}

func (h *Handler) getCachedHosts(ctx context.Context) ([]promclient.Host, bool) {
	if h.cacheClient == nil || !h.cacheClient.Enabled() {
		return nil, false
	}

	value, ok := h.cacheClient.Get(ctx, rediscache.HostsListKey)
	if !ok {
		return nil, false
	}

	var hosts []promclient.Host
	if err := json.Unmarshal(value, &hosts); err != nil {
		return nil, false
	}

	return hosts, true
}

func (h *Handler) cacheHosts(ctx context.Context, hosts []promclient.Host) {
	if h.cacheClient == nil || !h.cacheClient.Enabled() {
		return
	}

	value, err := json.Marshal(hosts)
	if err != nil {
		slog.Error("cache hosts marshal failed", "error", err)
		return
	}

	if err := h.cacheClient.Set(ctx, rediscache.HostsListKey, value, h.hostsTTL); err != nil {
		slog.Error("cache hosts set failed", "error", err)
	}
}

func decodeActiveAlerts(values map[string]string) []webhook.AlertRecord {
	alerts := make([]webhook.AlertRecord, 0, len(values))
	for _, value := range values {
		var alert webhook.AlertRecord
		if err := json.Unmarshal([]byte(value), &alert); err != nil {
			slog.Warn("skip corrupted alert data", "error", err)
			continue
		}
		alerts = append(alerts, alert)
	}

	return alerts
}

func decodeAlertEvents(values []string) []webhook.AlertEvent {
	events := make([]webhook.AlertEvent, 0, len(values))
	for _, value := range values {
		var event webhook.AlertEvent
		if err := json.Unmarshal([]byte(value), &event); err != nil {
			slog.Warn("skip corrupted alert event", "error", err)
			continue
		}
		events = append(events, event)
	}

	return events
}

func parseAlertEventsLimit(raw string) int64 {
	if raw == "" {
		return defaultAlertEventsLimit
	}

	parsed, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || parsed <= 0 {
		return defaultAlertEventsLimit
	}
	if parsed > rediscache.AlertEventsMax {
		return rediscache.AlertEventsMax
	}

	return parsed
}

func parseAlertEventFilter(raw string, allowed map[string]struct{}) string {
	if _, ok := allowed[raw]; !ok {
		return ""
	}

	return raw
}

func filterAlertEvents(events []webhook.AlertEvent, statusFilter, severityFilter string) []webhook.AlertEvent {
	if statusFilter == "" && severityFilter == "" {
		return events
	}

	filtered := make([]webhook.AlertEvent, 0, len(events))
	for _, event := range events {
		if statusFilter != "" && event.Status != statusFilter {
			continue
		}
		if severityFilter != "" && (event.Labels["severity"] != severityFilter) {
			continue
		}
		filtered = append(filtered, event)
	}

	return filtered
}

func filterActiveAlerts(alerts []webhook.AlertRecord, severityFilter string) []webhook.AlertRecord {
	if severityFilter == "" {
		return alerts
	}

	filtered := make([]webhook.AlertRecord, 0, len(alerts))
	for _, alert := range alerts {
		if alert.Labels["severity"] != severityFilter {
			continue
		}
		filtered = append(filtered, alert)
	}

	return filtered
}

func filterHostsByStatus(hosts []promclient.Host, statusFilter string) []promclient.Host {
	if statusFilter == "" {
		return hosts
	}

	filtered := make([]promclient.Host, 0, len(hosts))
	for _, host := range hosts {
		isUp := host.Status == "up" || host.Status == "healthy"
		if statusFilter == "up" && isUp {
			filtered = append(filtered, host)
			continue
		}
		if statusFilter == "down" && !isUp {
			filtered = append(filtered, host)
		}
	}

	return filtered
}

func filterHostsByQuery(hosts []promclient.Host, queryFilter string) []promclient.Host {
	if queryFilter == "" {
		return hosts
	}

	filtered := make([]promclient.Host, 0, len(hosts))
	for _, host := range hosts {
		if strings.Contains(strings.ToLower(host.Instance), queryFilter) {
			filtered = append(filtered, host)
		}
	}

	return filtered
}

func filterHosts(hosts []promclient.Host, statusFilter, queryFilter string) []promclient.Host {
	return filterHostsByQuery(filterHostsByStatus(hosts, statusFilter), queryFilter)
}

func normalizeHostQuery(raw string) string {
	return strings.ToLower(strings.TrimSpace(raw))
}
