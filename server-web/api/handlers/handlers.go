package handlers

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"gorm.io/gorm"

	appalert "server-web/alert"
	authpkg "server-web/auth"
	appcache "server-web/cache"
	eventbus "server-web/kafka"
	promclient "server-web/prometheus"
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
	cacheService   *appcache.Service
	alertService   *appalert.Service
	mysqlClient    mysqlClient
	authService    AuthService
	readyTimeout   time.Duration
	requestTimeout time.Duration
	cacheTimeout   time.Duration
	ruleSync       AlertRuleSyncConfig
	websocketHub   *ws.Hub
}

type response struct {
	Status string      `json:"status"`
	Data   interface{} `json:"data,omitempty"`
	Error  string      `json:"error,omitempty"`
}

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

var validAlertEventStatuses = map[string]struct{}{
	"firing":   {},
	"resolved": {},
}

var validAlertEventSeverities = map[string]struct{}{
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
		promClient:  promClient,
		db:          cfg.DB,
		cacheClient: cacheClient,
		cacheService: appcache.NewService(cacheClient, appcache.Options{
			HostsTTL:     cfg.HostsTTL,
			DashboardTTL: cfg.DashboardTTL,
		}),
		alertService: appalert.NewService(cacheClient, appalert.Options{
			DedupeTTL: cfg.DedupeTTL,
			DB:        cfg.DB,
			Producer:  cfg.AlertProducer,
		}),
		mysqlClient:    cfg.MySQLClient,
		authService:    cfg.AuthService,
		readyTimeout:   cfg.ReadyTimeout,
		requestTimeout: cfg.RequestTimeout,
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

	if cachedHosts, ok := h.cacheService.GetHosts(ctx); ok {
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
	h.cacheService.CacheHosts(cacheCtx, hosts)

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

	if cached, ok := h.cacheService.GetDashboardOverview(ctx); ok {
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
	activeAlerts, degraded := h.cacheService.CountActiveAlerts(ctx)
	overview.ActiveAlerts = activeAlerts
	overview.AlertDegraded = degraded
	overview.GeneratedAt = time.Now().UTC().Format(time.RFC3339)

	cacheCtx, cacheCancel := context.WithTimeout(context.Background(), h.cacheTimeout)
	defer cacheCancel()
	if !degraded {
		h.cacheService.CacheDashboardOverview(cacheCtx, overview)
	}

	c.JSON(http.StatusOK, response{
		Status: "success",
		Data:   overview,
	})
}

func (h *Handler) AlertmanagerWebhook(c *gin.Context) {
	if !h.alertService.Enabled() {
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

	if err := h.alertService.HandleWebhook(ctx, payload, receivedAt); err != nil {
		writeAlertServiceError(c, err)
		return
	}

	c.JSON(http.StatusAccepted, response{
		Status: "accepted",
	})
}

func (h *Handler) ActiveAlerts(c *gin.Context) {
	if !h.alertService.Enabled() {
		c.JSON(http.StatusServiceUnavailable, response{
			Status: "error",
			Error:  "redis is required for active alerts query",
		})
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), h.requestTimeout)
	defer cancel()

	alerts, err := h.alertService.ActiveAlerts(ctx, appalert.ParseActiveSeverityFilter(c.Query("severity")))
	if err != nil {
		writeAlertServiceError(c, err)
		return
	}

	c.JSON(http.StatusOK, response{
		Status: "success",
		Data:   alerts,
	})
}

func (h *Handler) AlertEvents(c *gin.Context) {
	if !h.alertService.Enabled() {
		c.JSON(http.StatusServiceUnavailable, response{
			Status: "error",
			Error:  "redis is required for alert events query",
		})
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), h.requestTimeout)
	defer cancel()

	events, err := h.alertService.AlertEvents(
		ctx,
		appalert.ParseEventsLimit(c.Query("limit")),
		appalert.ParseEventFilter(c.Query("status")),
		appalert.ParseEventSeverityFilter(c.Query("severity")),
	)
	if err != nil {
		writeAlertServiceError(c, err)
		return
	}

	c.JSON(http.StatusOK, response{
		Status: "success",
		Data:   events,
	})
}

func writeAlertServiceError(c *gin.Context, err error) {
	var serviceErr *appalert.ServiceError
	if !errors.As(err, &serviceErr) {
		c.JSON(http.StatusInternalServerError, response{Status: "error", Error: err.Error()})
		return
	}

	switch serviceErr.Kind {
	case appalert.ErrorUnsupportedStatus:
		c.JSON(http.StatusBadRequest, response{
			Status: "error",
			Error:  fmt.Sprintf("unsupported alert status %q", serviceErr.Status),
		})
	case appalert.ErrorMarshalAlert:
		c.JSON(http.StatusInternalServerError, response{
			Status: "error",
			Error:  fmt.Sprintf("marshal alert payload failed: %v", serviceErr.Err),
		})
	case appalert.ErrorStoreActiveAlert:
		c.JSON(http.StatusBadGateway, response{
			Status: "error",
			Error:  fmt.Sprintf("store active alert failed: %v", serviceErr.Err),
		})
	case appalert.ErrorDeleteActiveAlert:
		c.JSON(http.StatusBadGateway, response{
			Status: "error",
			Error:  fmt.Sprintf("delete active alert failed: %v", serviceErr.Err),
		})
	case appalert.ErrorMarshalAlertEvent:
		c.JSON(http.StatusInternalServerError, response{
			Status: "error",
			Error:  fmt.Sprintf("marshal alert event failed: %v", serviceErr.Err),
		})
	case appalert.ErrorStoreAlertEvent:
		c.JSON(http.StatusBadGateway, response{
			Status: "error",
			Error:  fmt.Sprintf("store alert event failed: %v", serviceErr.Err),
		})
	case appalert.ErrorLoadActiveAlerts:
		c.JSON(http.StatusBadGateway, response{
			Status: "error",
			Error:  fmt.Sprintf("load active alerts failed: %v", serviceErr.Err),
		})
	case appalert.ErrorLoadAlertEvents:
		c.JSON(http.StatusBadGateway, response{
			Status: "error",
			Error:  fmt.Sprintf("load alert events failed: %v", serviceErr.Err),
		})
	default:
		c.JSON(http.StatusInternalServerError, response{Status: "error", Error: err.Error()})
	}
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

func parseAlertEventFilter(raw string, allowed map[string]struct{}) string {
	if _, ok := allowed[raw]; !ok {
		return ""
	}

	return raw
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

func buildDashboardOverview(hosts []promclient.Host) appcache.DashboardOverview {
	overview := appcache.DashboardOverview{
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
