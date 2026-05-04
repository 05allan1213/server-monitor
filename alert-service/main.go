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

	"go.uber.org/zap"

	"alert-service/alert"
	"alert-service/config"
	"alert-service/health"
	"alert-service/kafka"
	"alert-service/logger"
	servicemetrics "alert-service/metrics"
	redisstore "alert-service/redis"
	"alert-service/tracer"
)

const (
	redisDialTimeout  = 3 * time.Second
	redisReadTimeout  = 2 * time.Second
	redisWriteTimeout = 2 * time.Second
	readinessTimeout  = time.Second
	shutdownPhases    = 3
)

func main() {
	cfg := config.Load()

	log, err := logger.Init("alert-service", cfg.LogLevel)
	if err != nil {
		fmt.Fprintf(os.Stderr, "logger init failed: %v\n", err)
		os.Exit(1)
	}
	defer logger.Sync(log)

	shutdownTracer, err := tracer.Init(context.Background(), tracer.Config{
		ServiceName:  "alert-service",
		OTLPEndpoint: cfg.TraceOTLPEndpoint,
		SampleRate:   cfg.TraceSampleRate,
	})
	if err != nil {
		zap.L().Warn("tracer init failed; tracing disabled",
			zap.String("endpoint", cfg.TraceOTLPEndpoint),
			zap.Error(err),
		)
		shutdownTracer = func(context.Context) error { return nil }
	} else if cfg.TraceOTLPEndpoint != "" {
		zap.L().Info("tracer initialized",
			zap.String("endpoint", cfg.TraceOTLPEndpoint),
			zap.Float64("sample_rate", cfg.TraceSampleRate),
		)
	}

	redisClient := redisstore.NewClient(redisstore.Options{
		Addr:         cfg.RedisAddr,
		Password:     cfg.RedisPassword,
		DialTimeout:  redisDialTimeout,
		ReadTimeout:  redisReadTimeout,
		WriteTimeout: redisWriteTimeout,
	})
	defer func() {
		if err := redisClient.Close(); err != nil {
			zap.L().Warn("redis close failed", zap.Error(err))
		}
	}()

	serviceMetrics := servicemetrics.New()
	store := alert.NewStore(redisClient, alert.DefaultDedupTTL, serviceMetrics)
	consumer, err := kafka.NewConsumer(cfg.KafkaBrokers, cfg.KafkaGroupID, store)
	if err != nil {
		zap.L().Fatal("kafka consumer init failed", zap.Error(err))
	}
	consumer.SetObserver(serviceMetrics)

	healthHandler := health.NewHandler(redisClient, readinessTimeout)
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", healthHandler.Healthz)
	mux.HandleFunc("/readyz", healthHandler.Readyz)
	mux.Handle("/metrics", serviceMetrics.HTTPHandler())

	server := &http.Server{
		Addr:              cfg.ListenAddr,
		Handler:           loggingMiddleware(recoveryMiddleware(mux)),
		ReadHeaderTimeout: cfg.HTTPReadHeaderTimeout,
		ReadTimeout:       cfg.HTTPReadTimeout,
		WriteTimeout:      cfg.HTTPWriteTimeout,
		IdleTimeout:       cfg.HTTPIdleTimeout,
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	consumerErr := make(chan error, 1)
	consumerDone := make(chan struct{})
	go func() {
		defer close(consumerDone)
		if err := consumer.Consume(ctx,
			func() {
				healthHandler.SetKafkaReady(true)
				serviceMetrics.SetKafkaReady(true)
				zap.L().Info("kafka consumer ready", zap.Strings("brokers", cfg.KafkaBrokers), zap.String("group_id", cfg.KafkaGroupID))
			},
			func() {
				healthHandler.SetKafkaReady(false)
				serviceMetrics.SetKafkaReady(false)
			},
		); err != nil && ctx.Err() == nil {
			consumerErr <- err
		}
	}()

	serverErr := make(chan error, 1)
	go func() {
		zap.L().Info("alert-service listening", zap.String("addr", cfg.ListenAddr))
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			serverErr <- err
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	exitCode := 0
	select {
	case sig := <-quit:
		zap.L().Info("alert-service received shutdown signal", zap.String("signal", sig.String()))
	case err := <-serverErr:
		exitCode = 1
		zap.L().Error("alert-service exited", zap.Error(err))
	case err := <-consumerErr:
		exitCode = 1
		zap.L().Error("alert-service consumer exited", zap.Error(err))
	}

	zap.L().Info("alert-service shutting down")
	cancel()
	healthHandler.SetKafkaReady(false)
	serviceMetrics.SetKafkaReady(false)
	consumerShutdownTimeout := phaseTimeout(cfg.ShutdownTimeout, shutdownPhases)
	if err := consumer.Close(); err != nil {
		zap.L().Warn("kafka consumer close failed", zap.Error(err))
	}
	select {
	case <-consumerDone:
	case <-time.After(consumerShutdownTimeout):
		zap.L().Warn("kafka consumer shutdown timed out")
	}

	serverShutdownTimeout := phaseTimeout(cfg.ShutdownTimeout, shutdownPhases)
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), serverShutdownTimeout)
	if err := server.Shutdown(shutdownCtx); err != nil {
		zap.L().Error("alert-service shutdown error", zap.Error(err))
	}
	shutdownCancel()

	traceShutdownTimeout := phaseTimeout(cfg.ShutdownTimeout, shutdownPhases)
	traceShutdownCtx, traceShutdownCancel := context.WithTimeout(context.Background(), traceShutdownTimeout)
	if err := shutdownTracer(traceShutdownCtx); err != nil {
		zap.L().Warn("tracer shutdown failed", zap.Error(err))
	}
	traceShutdownCancel()

	zap.L().Info("alert-service stopped")
	if exitCode != 0 {
		os.Exit(exitCode)
	}
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(status int) {
	r.status = status
	r.ResponseWriter.WriteHeader(status)
}

func (r *statusRecorder) Write(body []byte) (int, error) {
	if r.status == 0 {
		r.status = http.StatusOK
	}
	return r.ResponseWriter.Write(body)
}

func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		recorder := &statusRecorder{ResponseWriter: w}
		next.ServeHTTP(recorder, r)

		status := recorder.status
		if status == 0 {
			status = http.StatusOK
		}

		zap.L().Info("http request completed",
			zap.String("module", "http"),
			zap.String("method", r.Method),
			zap.String("path", r.URL.Path),
			zap.Int("status", status),
			zap.Float64("latency_ms", float64(time.Since(start).Microseconds())/1000),
			zap.String("client_ip", r.RemoteAddr),
		)
	})
}

func recoveryMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if recovered := recover(); recovered != nil {
				zap.L().Error("http request panic recovered",
					zap.String("module", "http"),
					zap.String("method", r.Method),
					zap.String("path", r.URL.Path),
					zap.Any("error", recovered),
				)
				writeErrorJSON(w, http.StatusInternalServerError)
			}
		}()

		next.ServeHTTP(w, r)
	})
}

func writeErrorJSON(w http.ResponseWriter, status int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{
		"status": "error",
		"error":  http.StatusText(status),
	})
}

func phaseTimeout(total time.Duration, phases int) time.Duration {
	if total <= 0 || phases <= 1 {
		return total
	}

	perPhase := total / time.Duration(phases)
	if perPhase <= 0 {
		return total
	}
	return perPhase
}
