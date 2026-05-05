package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"gorm.io/gorm"

	"server-monitor/pkg/logger"

	authpkg "server-web/auth"
	eventbus "server-web/kafka"
	"server-web/model"
	promclient "server-web/prometheus"
	rediscache "server-web/redis"
	"server-web/webhook"
	ws "server-web/websocket"
)

type AuthService interface {
	Login(ctx context.Context, username string, password string) (authpkg.LoginResult, error)
	AuthenticateBearer(authHeader string) (authpkg.Identity, error)
	AuthenticateToken(token string) (authpkg.Identity, error)
	Register(ctx context.Context, username, password, role string) (authpkg.Identity, error)
	ListUsers(ctx context.Context) ([]authpkg.Identity, error)
	DeleteUser(ctx context.Context, id uint64) error
}

type cacheClient interface {
	Enabled() bool
	Ping(ctx context.Context) error
	Get(ctx context.Context, key string) ([]byte, bool)
	Set(ctx context.Context, key string, value []byte, ttl time.Duration) error
	HSet(ctx context.Context, key, field string, value []byte) error
	HDel(ctx context.Context, key, field string) error
	HGetAll(ctx context.Context, key string) (map[string]string, error)
	AddAlertEventOnce(ctx context.Context, streamKey, dedupeKey string, maxLen int64, value, dedupeValue []byte, ttl time.Duration) (bool, error)
	XRevRangeN(ctx context.Context, key string, count int64) ([]string, error)
	Publish(ctx context.Context, channel string, message []byte) error
}

type mysqlClient interface {
	Enabled() bool
	Ping(ctx context.Context) error
}

type alertProducer interface {
	SendAlertEvent(eventbus.AlertEvent) error
}

type Handler struct {
	promClient     *promclient.Client
	db             *gorm.DB
	cacheClient    cacheClient
	mysqlClient    mysqlClient
	authService    AuthService
	alertProducer  alertProducer
	readyTimeout   time.Duration
	requestTimeout time.Duration
	hostsTTL       time.Duration
	dashboardTTL   time.Duration
	dedupeTTL      time.Duration
	cacheTimeout   time.Duration
	ruleSync       AlertRuleSyncConfig
	websocketHub   *ws.Hub
}

type response struct {
	Status string      `json:"status"`
	Data   interface{} `json:"data,omitempty"`
	Error  string      `json:"error,omitempty"`
}

const defaultAlertEventsLimit int64 = 8

type hostMetricsRange struct {
	duration time.Duration
	step     time.Duration
}

type hostMetricQuery struct {
	name   string
	metric string
	params map[string]string
}

type hostMetricsResponse struct {
	Instance    string                              `json:"instance"`
	Range       string                              `json:"range"`
	StepSeconds int64                               `json:"stepSeconds"`
	Metrics     map[string][]promclient.RangeSeries `json:"metrics"`
}

type dashboardOverview struct {
	TotalHosts    int     `json:"total_hosts"`
	HealthyHosts  int     `json:"healthy_hosts"`
	DownHosts     int     `json:"down_hosts"`
	ActiveAlerts  int     `json:"active_alerts"`
	AvgCPU        float64 `json:"avg_cpu"`
	AvgMemory     float64 `json:"avg_memory"`
	GeneratedAt   string  `json:"generated_at"`
	AlertDegraded bool    `json:"alert_degraded,omitempty"`
}

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

var validHostSorts = map[string]struct{}{
	"instance":    {},
	"cpu_desc":    {},
	"memory_desc": {},
}

var validHostRisks = map[string]struct{}{
	"high_cpu":    {},
	"high_memory": {},
}

var validHostMetricsRanges = map[string]hostMetricsRange{
	"15m": {
		duration: 15 * time.Minute,
		step:     15 * time.Second,
	},
	"1h": {
		duration: time.Hour,
		step:     time.Minute,
	},
	"6h": {
		duration: 6 * time.Hour,
		step:     5 * time.Minute,
	},
	"24h": {
		duration: 24 * time.Hour,
		step:     15 * time.Minute,
	},
}

type Config struct {
	ReadyTimeout   time.Duration
	RequestTimeout time.Duration
	HostsTTL       time.Duration
	DashboardTTL   time.Duration
	DedupeTTL      time.Duration
	CacheTimeout   time.Duration
	RuleSync       AlertRuleSyncConfig
	AlertProducer  alertProducer
	MySQLClient    mysqlClient
	DB             *gorm.DB
	AuthService    AuthService
}

func NewHandler(promClient *promclient.Client, cacheClient cacheClient, cfg Config, websocketHub *ws.Hub) (*Handler, error) {
	if promClient == nil {
		return nil, errors.New("prometheus client is required")
	}
	return &Handler{
		promClient:     promClient,
		db:             cfg.DB,
		cacheClient:    cacheClient,
		mysqlClient:    cfg.MySQLClient,
		authService:    cfg.AuthService,
		alertProducer:  cfg.AlertProducer,
		readyTimeout:   cfg.ReadyTimeout,
		requestTimeout: cfg.RequestTimeout,
		hostsTTL:       cfg.HostsTTL,
		dashboardTTL:   cfg.DashboardTTL,
		dedupeTTL:      cfg.DedupeTTL,
		cacheTimeout:   cfg.CacheTimeout,
		ruleSync:       cfg.RuleSync,
		websocketHub:   websocketHub,
	}, nil
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

func (h *Handler) ReadyzFull(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), h.readyTimeout)
	defer cancel()

	dependencies := gin.H{
		"prometheus": "ok",
		"redis":      "disabled",
		"mysql":      "disabled",
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

	if h.mysqlClient != nil && h.mysqlClient.Enabled() {
		if err := h.mysqlClient.Ping(ctx); err != nil {
			dependencies["mysql"] = "unreachable"
			errors = append(errors, err.Error())
		} else {
			dependencies["mysql"] = "ok"
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
	sortBy := parseHostSort(c.Query("sort"))
	riskFilter := parseHostRisk(c.Query("risk"))
	groupInstances, groupFiltered, ok := h.parseHostGroupFilter(c)
	if !ok {
		return
	}
	if groupFiltered && len(groupInstances) == 0 {
		c.JSON(http.StatusOK, response{
			Status: "success",
			Data:   []promclient.Host{},
		})
		return
	}

	if cachedHosts, ok := h.getCachedHosts(ctx); ok {
		if groupFiltered {
			cachedHosts = filterHostsByInstances(cachedHosts, groupInstances)
		}
		c.JSON(http.StatusOK, response{
			Status: "success",
			Data:   sortHosts(filterHosts(cachedHosts, statusFilter, queryFilter, riskFilter), sortBy),
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

	cacheCtx, cacheCancel := context.WithTimeout(context.Background(), h.cacheTimeout)
	defer cacheCancel()
	h.cacheHosts(cacheCtx, hosts)

	if groupFiltered {
		hosts = filterHostsByInstances(hosts, groupInstances)
	}

	c.JSON(http.StatusOK, response{
		Status: "success",
		Data:   sortHosts(filterHosts(hosts, statusFilter, queryFilter, riskFilter), sortBy),
	})
}

func (h *Handler) HostMetrics(c *gin.Context) {
	instance := strings.TrimSpace(c.Param("instance"))
	if instance == "" {
		c.JSON(http.StatusBadRequest, response{
			Status: "error",
			Error:  "instance is required",
		})
		return
	}

	rangeName, rangeConfig, ok := parseHostMetricsRange(c.Query("range"))
	if !ok {
		c.JSON(http.StatusBadRequest, response{
			Status: "error",
			Error:  "invalid range, allowed values: 15m, 1h, 6h, 24h",
		})
		return
	}

	end := time.Now().UTC()
	start := end.Add(-rangeConfig.duration)
	queries := buildHostMetricQueries(c.Query("mountpoint"))

	metrics, err := h.queryHostMetrics(c.Request.Context(), queries, instance, start, end, rangeConfig.step)
	if err != nil {
		c.JSON(http.StatusBadGateway, response{
			Status: "error",
			Error:  err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, response{
		Status: "success",
		Data: hostMetricsResponse{
			Instance:    instance,
			Range:       rangeName,
			StepSeconds: int64(rangeConfig.step.Seconds()),
			Metrics:     metrics,
		},
	})
}

func (h *Handler) queryHostMetrics(ctx context.Context, queries []hostMetricQuery, instance string, start, end time.Time, step time.Duration) (map[string][]promclient.RangeSeries, error) {
	queryCtx, cancelAll := context.WithCancel(ctx)
	defer cancelAll()

	metrics := make(map[string][]promclient.RangeSeries, len(queries))
	var mu sync.Mutex
	var wg sync.WaitGroup
	var firstErr error

	for _, query := range queries {
		query := query
		wg.Add(1)
		go func() {
			defer wg.Done()

			ctx, cancel := context.WithTimeout(queryCtx, h.requestTimeout)
			defer cancel()

			series, err := h.promClient.QueryRange(ctx, query.metric, instance, query.params, start, end, step)

			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				if firstErr == nil {
					firstErr = fmt.Errorf("query host metric %s failed: %w", query.name, err)
					cancelAll()
				}
				return
			}
			metrics[query.name] = series
		}()
	}

	wg.Wait()
	if firstErr != nil {
		return nil, firstErr
	}

	return metrics, nil
}

func (h *Handler) DashboardOverview(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), h.requestTimeout)
	defer cancel()

	if cached, ok := h.getCachedDashboardOverview(ctx); ok {
		c.JSON(http.StatusOK, response{
			Status: "success",
			Data:   cached,
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

	overview := buildDashboardOverview(hosts)
	activeAlerts, degraded := h.countActiveAlerts(ctx)
	overview.ActiveAlerts = activeAlerts
	overview.AlertDegraded = degraded
	overview.GeneratedAt = time.Now().UTC().Format(time.RFC3339)

	cacheCtx, cacheCancel := context.WithTimeout(context.Background(), h.cacheTimeout)
	defer cacheCancel()
	if !degraded {
		h.cacheDashboardOverview(cacheCtx, overview)
	}

	c.JSON(http.StatusOK, response{
		Status: "success",
		Data:   overview,
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
		status := http.StatusBadRequest
		message := fmt.Sprintf("invalid alertmanager payload: %v", err)
		var maxBytesErr *http.MaxBytesError
		if errors.As(err, &maxBytesErr) {
			status = http.StatusRequestEntityTooLarge
			message = "alertmanager payload too large"
		}
		c.JSON(status, response{
			Status: "error",
			Error:  message,
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
		if !isAlertStatusSupported(alert.Status) {
			c.JSON(http.StatusBadRequest, response{
				Status: "error",
				Error:  fmt.Sprintf("unsupported alert status %q", alert.Status),
			})
			return
		}
	}

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

		stored, err := h.cacheClient.AddAlertEventOnce(
			ctx,
			rediscache.AlertEventsKey,
			alertEventDedupeKey(alert),
			rediscache.AlertEventsMax,
			event,
			[]byte(receivedAt.Format(time.RFC3339Nano)),
			h.dedupeTTL,
		)
		if err != nil {
			c.JSON(http.StatusBadGateway, response{
				Status: "error",
				Error:  fmt.Sprintf("store alert event failed: %v", err),
			})
			return
		}
		if !stored {
			continue
		}

		h.archiveAlertHistory(ctx, alert)

		if err := h.cacheClient.Publish(ctx, rediscache.AlertChannel, event); err != nil {
			// Active state and history are already stored; failing the webhook here
			// would trigger retries and duplicate history entries.
			zap.L().Warn("publish alert event failed",
				zap.String("fingerprint", alert.Fingerprint),
				zap.String("status", alert.Status),
				zap.Error(err),
			)
		}
		h.sendAlertEventToKafka(c.Request.Context(), alert, receivedAt)
	}

	c.JSON(http.StatusAccepted, response{
		Status: "accepted",
	})
}

func (h *Handler) archiveAlertHistory(ctx context.Context, alert webhook.AlertRecord) {
	if h.db == nil {
		return
	}

	history, err := buildAlertHistory(alert)
	if err != nil {
		logger.FromContext(ctx).Warn("build alert history failed",
			zap.String("fingerprint", alert.Fingerprint),
			zap.String("status", alert.Status),
			zap.Error(err),
		)
		return
	}

	err = h.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var existing model.AlertHistory
		findErr := tx.
			Where("fingerprint = ? AND fired_at = ?", history.Fingerprint, history.FiredAt).
			Order("id DESC").
			First(&existing).Error
		if errors.Is(findErr, gorm.ErrRecordNotFound) {
			return tx.Create(&history).Error
		}
		if findErr != nil {
			return findErr
		}

		updates := map[string]interface{}{
			"alert_name":  history.AlertName,
			"instance":    history.Instance,
			"severity":    history.Severity,
			"summary":     history.Summary,
			"labels_json": history.LabelsJSON,
		}
		if history.Status == "resolved" {
			updates["status"] = "resolved"
			updates["resolved_at"] = history.ResolvedAt
		}
		return tx.Model(&model.AlertHistory{}).Where("id = ?", existing.ID).Updates(updates).Error
	})
	if err != nil {
		logger.FromContext(ctx).Warn("archive alert history failed",
			zap.String("fingerprint", alert.Fingerprint),
			zap.String("status", alert.Status),
			zap.Error(err),
		)
	}
}

func (h *Handler) sendAlertEventToKafka(ctx context.Context, alert webhook.AlertRecord, receivedAt time.Time) {
	if h.alertProducer == nil {
		return
	}

	event := eventbus.AlertEvent{
		Type:         "alert",
		Fingerprint:  alert.Fingerprint,
		Status:       alert.Status,
		Labels:       alert.Labels,
		Annotations:  alert.Annotations,
		StartsAt:     alert.StartsAt,
		EndsAt:       alert.EndsAt,
		GeneratorURL: alert.GeneratorURL,
		ReceivedAt:   receivedAt,
	}
	if err := h.alertProducer.SendAlertEvent(event); err != nil {
		logger.FromContext(ctx).Warn("kafka produce alert event failed",
			zap.String("fingerprint", alert.Fingerprint),
			zap.String("status", alert.Status),
			zap.Error(err),
		)
	}
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

	values, err := h.cacheClient.XRevRangeN(ctx, rediscache.AlertEventsKey, limit)
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
		zap.L().Warn("websocket upgrade failed", zap.Error(err))
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
		zap.L().Error("cache hosts marshal failed", zap.Error(err))
		return
	}

	if err := h.cacheClient.Set(ctx, rediscache.HostsListKey, value, h.hostsTTL); err != nil {
		zap.L().Error("cache hosts set failed", zap.Error(err))
	}
}

func (h *Handler) getCachedDashboardOverview(ctx context.Context) (dashboardOverview, bool) {
	if h.cacheClient == nil || !h.cacheClient.Enabled() {
		return dashboardOverview{}, false
	}

	value, ok := h.cacheClient.Get(ctx, rediscache.DashboardOverviewKey)
	if !ok {
		return dashboardOverview{}, false
	}

	var overview dashboardOverview
	if err := json.Unmarshal(value, &overview); err != nil {
		return dashboardOverview{}, false
	}

	return overview, true
}

func (h *Handler) cacheDashboardOverview(ctx context.Context, overview dashboardOverview) {
	if h.cacheClient == nil || !h.cacheClient.Enabled() {
		return
	}

	value, err := json.Marshal(overview)
	if err != nil {
		zap.L().Error("cache dashboard overview marshal failed", zap.Error(err))
		return
	}

	if err := h.cacheClient.Set(ctx, rediscache.DashboardOverviewKey, value, h.dashboardTTL); err != nil {
		zap.L().Error("cache dashboard overview set failed", zap.Error(err))
	}
}

func (h *Handler) countActiveAlerts(ctx context.Context) (int, bool) {
	if h.cacheClient == nil || !h.cacheClient.Enabled() {
		return 0, false
	}

	values, err := h.cacheClient.HGetAll(ctx, rediscache.ActiveAlertsKey)
	if err != nil {
		zap.L().Warn("dashboard overview active alerts degraded", zap.Error(err))
		return 0, true
	}

	return len(values), false
}

func decodeActiveAlerts(values map[string]string) []webhook.AlertRecord {
	alerts := make([]webhook.AlertRecord, 0, len(values))
	for _, value := range values {
		var alert webhook.AlertRecord
		if err := json.Unmarshal([]byte(value), &alert); err != nil {
			zap.L().Warn("skip corrupted alert data", zap.Error(err))
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
			zap.L().Warn("skip corrupted alert event", zap.Error(err))
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

func isAlertStatusSupported(status string) bool {
	_, ok := validAlertEventStatuses[status]
	return ok
}

func alertEventDedupeKey(alert webhook.AlertRecord) string {
	return fmt.Sprintf("%s:%s:%s:%s:%s",
		rediscache.AlertEventDedupeKey,
		alert.Fingerprint,
		alert.Status,
		alert.StartsAt.UTC().Format(time.RFC3339Nano),
		alert.EndsAt.UTC().Format(time.RFC3339Nano),
	)
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
		isUp := host.Status == "up"
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

func filterHostsByRisk(hosts []promclient.Host, riskFilter string) []promclient.Host {
	if riskFilter == "" {
		return hosts
	}

	filtered := make([]promclient.Host, 0, len(hosts))
	for _, host := range hosts {
		switch riskFilter {
		case "high_cpu":
			if host.CPU >= 80 {
				filtered = append(filtered, host)
			}
		case "high_memory":
			if host.Memory >= 85 {
				filtered = append(filtered, host)
			}
		}
	}

	return filtered
}

func filterHosts(hosts []promclient.Host, statusFilter, queryFilter, riskFilter string) []promclient.Host {
	return filterHostsByRisk(filterHostsByQuery(filterHostsByStatus(hosts, statusFilter), queryFilter), riskFilter)
}

func normalizeHostQuery(raw string) string {
	return strings.ToLower(strings.TrimSpace(raw))
}

func parseHostSort(raw string) string {
	if _, ok := validHostSorts[raw]; !ok {
		return "instance"
	}

	return raw
}

func parseHostRisk(raw string) string {
	if _, ok := validHostRisks[raw]; !ok {
		return ""
	}

	return raw
}

func parseHostMetricsRange(raw string) (string, hostMetricsRange, bool) {
	if raw == "" {
		raw = "1h"
	}

	parsed, ok := validHostMetricsRanges[raw]
	return raw, parsed, ok
}

func buildHostMetricQueries(mountpoint string) []hostMetricQuery {
	mountpoint = strings.TrimSpace(mountpoint)
	diskParams := map[string]string{}
	if mountpoint != "" {
		diskParams["mountpoint"] = mountpoint
	}

	return []hostMetricQuery{
		{name: "cpu", metric: promclient.MetricCPUUsage},
		{name: "memory", metric: promclient.MetricMemoryUsage},
		{name: "disk", metric: promclient.MetricDiskUsage, params: diskParams},
		{name: "network_recv", metric: promclient.MetricNetworkRecv},
		{name: "network_sent", metric: promclient.MetricNetworkSent},
		{name: "load1", metric: promclient.MetricLoad1},
		{name: "process_count", metric: promclient.MetricProcessCount},
		{name: "uptime", metric: promclient.MetricUptime},
	}
}

func buildDashboardOverview(hosts []promclient.Host) dashboardOverview {
	overview := dashboardOverview{
		TotalHosts: len(hosts),
	}
	if len(hosts) == 0 {
		return overview
	}

	var totalCPU float64
	var totalMemory float64
	var healthyHosts int
	for _, host := range hosts {
		if host.Status == "up" {
			overview.HealthyHosts++
			healthyHosts++
			totalCPU += host.CPU
			totalMemory += host.Memory
		} else {
			overview.DownHosts++
		}
	}

	if healthyHosts > 0 {
		overview.AvgCPU = totalCPU / float64(healthyHosts)
		overview.AvgMemory = totalMemory / float64(healthyHosts)
	}

	return overview
}

func sortHosts(hosts []promclient.Host, sortBy string) []promclient.Host {
	sorted := append([]promclient.Host(nil), hosts...)

	switch sortBy {
	case "cpu_desc":
		sort.SliceStable(sorted, func(i, j int) bool {
			if sorted[i].CPU == sorted[j].CPU {
				return sorted[i].Instance < sorted[j].Instance
			}
			return sorted[i].CPU > sorted[j].CPU
		})
	case "memory_desc":
		sort.SliceStable(sorted, func(i, j int) bool {
			if sorted[i].Memory == sorted[j].Memory {
				return sorted[i].Instance < sorted[j].Instance
			}
			return sorted[i].Memory > sorted[j].Memory
		})
	default:
		sort.SliceStable(sorted, func(i, j int) bool {
			return sorted[i].Instance < sorted[j].Instance
		})
	}

	return sorted
}
