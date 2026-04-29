package main

import (
	"context"
	"log"

	"github.com/gin-gonic/gin"

	"server-web/api"
	"server-web/config"
	promclient "server-web/prometheus"
	"server-web/pubsub"
	rediscache "server-web/redis"
)

func main() {
	cfg := config.Load()
	gin.SetMode(cfg.GinMode)

	prometheusClient := promclient.NewClient(cfg.PrometheusURL, cfg.RequestTimeout)
	redisClient := rediscache.NewClient(cfg.RedisAddr, cfg.RedisPassword, cfg.RedisDB)
	alertHub := pubsub.NewHub(64)

	if redisClient.Enabled() {
		subscriber := pubsub.NewSubscriber(redisClient, alertHub, rediscache.AlertChannel)
		go subscriber.Run(context.Background())
	}

	router, err := api.NewRouter(cfg, prometheusClient, redisClient)
	if err != nil {
		log.Fatalf("create router failed: %v", err)
	}

	log.Printf("server-web listening on %s", cfg.ListenAddr)
	if err := router.Run(cfg.ListenAddr); err != nil {
		log.Fatalf("server-web exited: %v", err)
	}
}
