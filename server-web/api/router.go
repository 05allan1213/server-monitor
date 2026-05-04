package api

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin"

	"server-web/api/handlers"
	"server-web/api/middleware"
	"server-web/config"
	promclient "server-web/prometheus"
	rediscache "server-web/redis"
	ws "server-web/websocket"
)

func NewRouter(cfg config.Config, promClient *promclient.Client, cacheClient *rediscache.Client, websocketHub *ws.Hub) (*gin.Engine, error) {
	router := gin.New()
	metrics := middleware.NewMetrics()
	if websocketHub != nil {
		websocketHub.SetConnectionObserver(metrics.SetWebSocketConnections)
	}
	router.Use(
		middleware.CORS(cfg.CORSOrigins),
		otelgin.Middleware("server-web"),
		middleware.Logging(),
		middleware.Recovery(),
		metrics.Handler(),
		middleware.RateLimit(cacheClient, cfg.RateLimit),
	)

	if err := router.SetTrustedProxies(cfg.TrustedProxies); err != nil {
		return nil, err
	}

	handler, err := handlers.NewHandler(promClient, cacheClient, handlers.Config{
		ReadyTimeout:   cfg.ReadyTimeout,
		RequestTimeout: cfg.RequestTimeout,
		HostsTTL:       cfg.HostsCacheTTL,
		DashboardTTL:   cfg.DashboardOverviewTTL,
		DedupeTTL:      cfg.AlertEventDedupeTTL,
		CacheTimeout:   cfg.CacheWriteTimeout,
	}, websocketHub)
	if err != nil {
		return nil, err
	}

	router.GET("/metrics", gin.WrapH(metrics.HTTPHandler()))
	router.GET("/healthz", handler.Healthz)
	router.GET("/readyz", handler.Readyz)
	router.GET("/api/v1/hosts", handler.Hosts)
	router.GET("/api/v1/hosts/:instance/metrics", handler.HostMetrics)
	router.GET("/api/v1/dashboard/overview", handler.DashboardOverview)
	router.GET("/api/v1/alerts/active", handler.ActiveAlerts)
	router.GET("/api/v1/alerts/events", handler.AlertEvents)
	router.GET("/ws/alerts", handler.AlertsWebSocket)
	router.POST(
		"/api/v1/webhook/alertmanager",
		limitRequestBody(cfg.AlertmanagerWebhookMaxBodyBytes),
		handler.AlertmanagerWebhook,
	)

	staticDir := cfg.StaticDir
	if staticDir != "" {
		if _, err := os.Stat(staticDir); err == nil {
			staticHandler, err := serveStatic(staticDir)
			if err != nil {
				return nil, err
			}
			router.Use(staticHandler)
		}
	}

	return router, nil
}

func limitRequestBody(maxBytes int64) gin.HandlerFunc {
	return func(c *gin.Context) {
		if maxBytes > 0 {
			c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxBytes)
		}
		c.Next()
	}
}

func serveStatic(staticDir string) (gin.HandlerFunc, error) {
	fileServer := http.FileServer(http.Dir(staticDir))
	absStaticDir, err := filepath.Abs(staticDir)
	if err != nil {
		return nil, err
	}

	return func(c *gin.Context) {
		path := c.Request.URL.Path

		if c.Request.Method != http.MethodGet && c.Request.Method != http.MethodHead {
			c.Next()
			return
		}

		if len(path) >= 5 && path[:5] == "/api/" {
			c.Next()
			return
		}
		if len(path) >= 4 && path[:4] == "/ws/" {
			c.Next()
			return
		}
		if path == "/healthz" || path == "/readyz" {
			c.Next()
			return
		}

		filePath := filepath.Join(absStaticDir, filepath.Clean(path))
		if !strings.HasPrefix(filePath, absStaticDir+string(os.PathSeparator)) && filePath != absStaticDir {
			c.Next()
			return
		}

		if info, err := os.Stat(filePath); err == nil && !info.IsDir() {
			fileServer.ServeHTTP(c.Writer, c.Request)
			c.Abort()
			return
		}

		indexPath := filepath.Join(absStaticDir, "index.html")
		if _, err := os.Stat(indexPath); err == nil {
			c.Request.URL.Path = "/"
			fileServer.ServeHTTP(c.Writer, c.Request)
			c.Abort()
			return
		}

		c.Next()
	}, nil
}
