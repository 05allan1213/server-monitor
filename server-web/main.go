package main

import (
	"log"

	"github.com/gin-gonic/gin"

	"server-web/api"
	"server-web/config"
	promclient "server-web/prometheus"
)

func main() {
	cfg := config.Load()
	gin.SetMode(cfg.GinMode)

	prometheusClient := promclient.NewClient(cfg.PrometheusURL, cfg.RequestTimeout)

	router, err := api.NewRouter(cfg, prometheusClient)
	if err != nil {
		log.Fatalf("create router failed: %v", err)
	}

	log.Printf("server-web listening on %s", cfg.ListenAddr)
	if err := router.Run(cfg.ListenAddr); err != nil {
		log.Fatalf("server-web exited: %v", err)
	}
}
