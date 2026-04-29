package api

import (
	"net/http"
	"os"
	"path/filepath"

	"github.com/gin-gonic/gin"

	"server-web/api/handlers"
	"server-web/config"
	promclient "server-web/prometheus"
	rediscache "server-web/redis"
	ws "server-web/websocket"
)

func NewRouter(cfg config.Config, promClient *promclient.Client, cacheClient *rediscache.Client, websocketHub *ws.Hub) (*gin.Engine, error) {
	router := gin.New()
	router.Use(gin.Logger(), gin.Recovery())

	if err := router.SetTrustedProxies(cfg.TrustedProxies); err != nil {
		return nil, err
	}

	handler := handlers.NewHandler(promClient, cacheClient, cfg.ReadyTimeout, cfg.HostsCacheTTL, websocketHub)

	router.GET("/healthz", handler.Healthz)
	router.GET("/readyz", handler.Readyz)
	router.GET("/api/v1/hosts", handler.Hosts)
	router.GET("/api/v1/alerts/active", handler.ActiveAlerts)
	router.GET("/ws/alerts", handler.AlertsWebSocket)
	router.POST("/api/v1/webhook/alertmanager", handler.AlertmanagerWebhook)

	staticDir := cfg.StaticDir
	if staticDir != "" {
		if _, err := os.Stat(staticDir); err == nil {
			router.Use(serveStatic(staticDir))
		}
	}

	return router, nil
}

func serveStatic(staticDir string) gin.HandlerFunc {
	fileServer := http.FileServer(http.Dir(staticDir))

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

		filePath := filepath.Join(staticDir, filepath.Clean(path))
		if info, err := os.Stat(filePath); err == nil && !info.IsDir() {
			fileServer.ServeHTTP(c.Writer, c.Request)
			c.Abort()
			return
		}

		indexPath := filepath.Join(staticDir, "index.html")
		if _, err := os.Stat(indexPath); err == nil {
			c.Request.URL.Path = "/"
			fileServer.ServeHTTP(c.Writer, c.Request)
			c.Abort()
			return
		}

		c.Next()
	}
}
