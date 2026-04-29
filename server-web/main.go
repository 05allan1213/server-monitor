package main

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"

	"server-web/api"
	"server-web/config"
	promclient "server-web/prometheus"
	"server-web/pubsub"
	rediscache "server-web/redis"
	ws "server-web/websocket"
)

type wsMessage struct {
	Type string      `json:"type"`
	Data interface{} `json:"data"`
}

func main() {
	cfg := config.Load()
	gin.SetMode(cfg.GinMode)

	prometheusClient := promclient.NewClient(cfg.PrometheusURL, cfg.RequestTimeout)
	redisClient := rediscache.NewClient(cfg.RedisAddr, cfg.RedisPassword, cfg.RedisDB)
	alertHub := pubsub.NewHub(64)
	websocketHub := ws.NewHub()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go websocketHub.Run(ctx)

	if redisClient.Enabled() {
		if err := redisClient.Ping(context.Background()); err != nil {
			slog.Error("redis ping failed at startup", "addr", cfg.RedisAddr, "error", err)
		}

		subscriber := pubsub.NewSubscriber(redisClient, alertHub, rediscache.AlertChannel)
		go subscriber.Run(ctx)
	}

	alertHubConsumers := make(chan struct{})
	go func() {
		defer close(alertHubConsumers)
		for message := range alertHub.Messages() {
			websocketHub.Broadcast(message)
		}
	}()

	go broadcastHosts(ctx, prometheusClient, websocketHub, cfg.RequestTimeout)

	router, err := api.NewRouter(cfg, prometheusClient, redisClient, websocketHub)
	if err != nil {
		slog.Error("create router failed", "error", err)
		os.Exit(1)
	}

	srv := &http.Server{
		Addr:    cfg.ListenAddr,
		Handler: router,
	}

	go func() {
		slog.Info("server-web listening", "addr", cfg.ListenAddr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("server-web exited", "error", err)
			os.Exit(1)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	slog.Info("server-web shutting down...")

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		slog.Error("server-web shutdown error", "error", err)
	}

	cancel()
	alertHub.Close()
	<-alertHubConsumers

	slog.Info("server-web stopped")
}

func broadcastHosts(ctx context.Context, promClient *promclient.Client, hub *ws.Hub, timeout time.Duration) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			queryCtx, cancel := context.WithTimeout(ctx, timeout)
			hosts, err := promClient.GetHosts(queryCtx)
			cancel()
			if err != nil {
				slog.Warn("broadcast hosts query failed", "error", err)
				continue
			}

			payload, err := json.Marshal(wsMessage{Type: "hosts", Data: hosts})
			if err != nil {
				slog.Warn("broadcast hosts marshal failed", "error", err)
				continue
			}

			hub.Broadcast(payload)
		}
	}
}
