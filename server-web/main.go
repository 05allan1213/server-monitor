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
	ws "server-web/websocket"
)

func main() {
	cfg := config.Load()
	gin.SetMode(cfg.GinMode)

	prometheusClient := promclient.NewClient(cfg.PrometheusURL, cfg.RequestTimeout)
	redisClient := rediscache.NewClient(cfg.RedisAddr, cfg.RedisPassword, cfg.RedisDB)
	alertHub := pubsub.NewHub(64)
	websocketHub := ws.NewHub()

	go websocketHub.Run()

	if redisClient.Enabled() {
		subscriber := pubsub.NewSubscriber(redisClient, alertHub, rediscache.AlertChannel)
		go subscriber.Run(context.Background())
	}

	go func() {
		for message := range alertHub.Messages() {
			websocketHub.Broadcast(message)
		}
	}()

	router, err := api.NewRouter(cfg, prometheusClient, redisClient, websocketHub)
	if err != nil {
		log.Fatalf("create router failed: %v", err)
	}

	log.Printf("server-web listening on %s", cfg.ListenAddr)
	if err := router.Run(cfg.ListenAddr); err != nil {
		log.Fatalf("server-web exited: %v", err)
	}
}
