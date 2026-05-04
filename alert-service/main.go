package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"go.uber.org/zap"

	"alert-service/config"
	"alert-service/health"
	"alert-service/logger"
	"alert-service/tracer"
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

	healthHandler := health.NewHandler()
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", healthHandler.Healthz)
	mux.HandleFunc("/readyz", healthHandler.Readyz)

	server := &http.Server{
		Addr:              cfg.ListenAddr,
		Handler:           mux,
		ReadHeaderTimeout: cfg.HTTPReadHeaderTimeout,
		ReadTimeout:       cfg.HTTPReadTimeout,
		WriteTimeout:      cfg.HTTPWriteTimeout,
		IdleTimeout:       cfg.HTTPIdleTimeout,
	}

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
	}

	zap.L().Info("alert-service shutting down")
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
	if err := server.Shutdown(shutdownCtx); err != nil {
		zap.L().Error("alert-service shutdown error", zap.Error(err))
	}
	shutdownCancel()

	traceShutdownCtx, traceShutdownCancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
	if err := shutdownTracer(traceShutdownCtx); err != nil {
		zap.L().Warn("tracer shutdown failed", zap.Error(err))
	}
	traceShutdownCancel()

	zap.L().Info("alert-service stopped")
	if exitCode != 0 {
		os.Exit(exitCode)
	}
}
