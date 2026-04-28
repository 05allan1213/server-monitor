package api

import (
	"github.com/gin-gonic/gin"

	"server-web/api/handlers"
	"server-web/config"
	promclient "server-web/prometheus"
)

func NewRouter(cfg config.Config, promClient *promclient.Client) (*gin.Engine, error) {
	router := gin.New()
	router.Use(gin.Logger(), gin.Recovery())

	if err := router.SetTrustedProxies(cfg.TrustedProxies); err != nil {
		return nil, err
	}

	handler := handlers.NewHandler(promClient, cfg.ReadyTimeout)

	router.GET("/", handler.Root)
	router.GET("/healthz", handler.Healthz)
	router.GET("/readyz", handler.Readyz)
	router.GET("/api/v1/hosts", handler.Hosts)

	return router, nil
}
