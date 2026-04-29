package api

import (
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

	router.GET("/", handler.Root)
	router.GET("/healthz", handler.Healthz)
	router.GET("/readyz", handler.Readyz)
	router.GET("/api/v1/hosts", handler.Hosts)
	router.GET("/api/v1/alerts/active", handler.ActiveAlerts)
	router.GET("/ws/alerts", handler.AlertsWebSocket)
	router.POST("/api/v1/webhook/alertmanager", handler.AlertmanagerWebhook)

	return router, nil
}
