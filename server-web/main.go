package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"server-web/api"
	"server-web/config"
	"server-web/logger"
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
	log, err := logger.Init("server-web")
	if err != nil {
		fmt.Fprintf(os.Stderr, "logger init failed: %v\n", err)
		os.Exit(1)
	}
	defer logger.Sync(log)

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
			zap.L().Error("redis ping failed at startup",
				zap.String("addr", cfg.RedisAddr),
				zap.Error(err),
			)
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
			if err := websocketHub.BroadcastBlocking(ctx, message); err != nil {
				if ctx.Err() != nil {
					return
				}
				zap.L().Warn("broadcast alert failed", zap.Error(err))
			}
		}
	}()

	go broadcastHosts(ctx, prometheusClient, websocketHub, cfg.RequestTimeout, cfg.HostsBroadcastInterval)

	router, err := api.NewRouter(cfg, prometheusClient, redisClient, websocketHub)
	if err != nil {
		zap.L().Error("create router failed", zap.Error(err))
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
		zap.L().Info("server-web listening", zap.String("addr", cfg.ListenAddr))
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			serverErr <- err
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	exitCode := 0
	select {
	case sig := <-quit:
		zap.L().Info("server-web received shutdown signal", zap.String("signal", sig.String()))
	case err := <-serverErr:
		exitCode = 1
		zap.L().Error("server-web exited", zap.Error(err))
	}

	zap.L().Info("server-web shutting down")

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)

	if err := srv.Shutdown(shutdownCtx); err != nil {
		zap.L().Error("server-web shutdown error", zap.Error(err))
	}
	shutdownCancel()

	cancel()
	if subscriberDone != nil {
		<-subscriberDone
	}
	if err := redisClient.Close(); err != nil {
		zap.L().Error("redis close failed", zap.Error(err))
	}

	alertHub.Close()
	<-alertHubConsumers

	zap.L().Info("server-web stopped")
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
				zap.L().Warn("broadcast hosts query failed", zap.Error(err))
				continue
			}

			payload, err := json.Marshal(wsMessage{Type: "hosts", Data: hosts})
			if err != nil {
				zap.L().Warn("broadcast hosts marshal failed", zap.Error(err))
				continue
			}

			hub.Broadcast(payload)
		}
	}
}
