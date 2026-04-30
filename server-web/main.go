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
	redisClient := rediscache.NewClient(rediscache.Options{
		Addr:            cfg.RedisAddr,
		Password:        cfg.RedisPassword,
		DB:              cfg.RedisDB,
		DialTimeout:     cfg.RedisDialTimeout,
		ReadTimeout:     cfg.RedisReadTimeout,
		WriteTimeout:    cfg.RedisWriteTimeout,
		ConnMaxLifetime: cfg.RedisConnMaxLifetime,
		ConnMaxIdleTime: cfg.RedisConnMaxIdleTime,
	})
	alertHub := pubsub.NewHub(64)
	websocketHub := ws.NewHub(cfg.CORSOrigins)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go websocketHub.Run(ctx)

	var subscriberDone <-chan struct{}
	if redisClient.Enabled() {
		pingCtx, pingCancel := context.WithTimeout(context.Background(), cfg.RedisStartupTimeout)
		if err := redisClient.Ping(pingCtx); err != nil {
			slog.Error("redis ping failed at startup", "addr", cfg.RedisAddr, "error", err)
		}
		pingCancel()

		subscriber := pubsub.NewSubscriber(redisClient, alertHub, rediscache.AlertChannel)
		done := make(chan struct{})
		subscriberDone = done
		go func() {
			defer close(done)
			subscriber.Run(ctx)
		}()
	}

	alertHubConsumers := make(chan struct{})
	go func() {
		defer close(alertHubConsumers)
		for message := range alertHub.Messages() {
			websocketHub.Broadcast(message)
		}
	}()

	go broadcastHosts(ctx, prometheusClient, websocketHub, cfg.RequestTimeout, cfg.HostsBroadcastInterval)

	router, err := api.NewRouter(cfg, prometheusClient, redisClient, websocketHub)
	if err != nil {
		slog.Error("create router failed", "error", err)
		os.Exit(1)
	}

	srv := &http.Server{
		Addr:              cfg.ListenAddr,
		Handler:           router,
		ReadHeaderTimeout: cfg.HTTPReadHeaderTimeout,
		ReadTimeout:       cfg.HTTPReadTimeout,
		WriteTimeout:      cfg.HTTPWriteTimeout,
		IdleTimeout:       cfg.HTTPIdleTimeout,
	}

	serverErr := make(chan error, 1)
	go func() {
		slog.Info("server-web listening", "addr", cfg.ListenAddr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			serverErr <- err
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	exitCode := 0
	select {
	case sig := <-quit:
		slog.Info("server-web received shutdown signal", "signal", sig.String())
	case err := <-serverErr:
		exitCode = 1
		slog.Error("server-web exited", "error", err)
	}

	slog.Info("server-web shutting down...")

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)

	if err := srv.Shutdown(shutdownCtx); err != nil {
		slog.Error("server-web shutdown error", "error", err)
	}
	shutdownCancel()

	cancel()
	if subscriberDone != nil {
		<-subscriberDone
	}
	if err := redisClient.Close(); err != nil {
		slog.Error("redis close failed", "error", err)
	}

	alertHub.Close()
	<-alertHubConsumers

	slog.Info("server-web stopped")
	if exitCode != 0 {
		os.Exit(exitCode)
	}
}

func broadcastHosts(ctx context.Context, promClient *promclient.Client, hub *ws.Hub, timeout time.Duration, interval time.Duration) {
	ticker := time.NewTicker(interval)
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
