package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"server-probe/collector"
	"server-probe/config"
)

func main() {
	cfg := config.Load()
	if err := applyHostPaths(cfg); err != nil {
		slog.Error("apply host paths failed", "error", err)
		os.Exit(1)
	}

	registry := prometheus.NewRegistry()

	collectors := []collector.Collector{
		collector.NewCPUCollector(cfg.Hostname),
		collector.NewMemoryCollector(cfg.Hostname),
		collector.NewDiskCollector(cfg.Hostname),
		collector.NewNetworkCollector(cfg.Hostname),
		collector.NewLoadCollector(cfg.Hostname),
		collector.NewProcessCollector(cfg.Hostname),
	}

	for _, c := range collectors {
		c.Register(registry)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	updateCollectors(ctx, collectors)

	go func() {
		ticker := time.NewTicker(cfg.ScrapeInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				updateCollectors(ctx, collectors)
			}
		}
	}()

	mux := http.NewServeMux()
	mux.Handle(cfg.MetricsPath, promhttp.HandlerFor(registry, promhttp.HandlerOpts{
		MaxRequestsInFlight: 5,
		Timeout:             5 * time.Second,
	}))
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		if _, err := w.Write([]byte(`{"healthy":true}`)); err != nil {
			slog.Error("healthz response write failed", "error", err)
		}
	})
	mux.HandleFunc("/readyz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		if _, err := w.Write([]byte(`{"ready":true}`)); err != nil {
			slog.Error("readyz response write failed", "error", err)
		}
	})

	srv := &http.Server{
		Addr:         cfg.ListenAddr,
		Handler:      loggingMiddleware(recoveryMiddleware(mux)),
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	serverErr := make(chan error, 1)
	go func() {
		slog.Info("server-probe listening", "addr", cfg.ListenAddr, "metrics_path", cfg.MetricsPath)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			serverErr <- err
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(quit)

	exitCode := 0
	select {
	case sig := <-quit:
		slog.Info("server-probe shutting down...", "signal", sig.String())
	case err := <-serverErr:
		slog.Error("server-probe exited", "error", err)
		exitCode = 1
	}

	cancel()

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)

	if err := srv.Shutdown(shutdownCtx); err != nil {
		slog.Error("server-probe shutdown error", "error", err)
	}
	shutdownCancel()

	slog.Info("server-probe stopped")
	if exitCode != 0 {
		os.Exit(exitCode)
	}
}

func updateCollectors(ctx context.Context, collectors []collector.Collector) {
	for _, c := range collectors {
		if err := ctx.Err(); err != nil {
			return
		}
		if err := c.Update(ctx); err != nil {
			if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
				return
			}
			slog.Error("collector update failed", "collector", c.Name(), "error", err)
		}
	}
}

func applyHostPaths(cfg config.Config) error {
	if cfg.HostProc != "" {
		if err := os.Setenv("HOST_PROC", cfg.HostProc); err != nil {
			return err
		}
	}
	if cfg.HostSys != "" {
		if err := os.Setenv("HOST_SYS", cfg.HostSys); err != nil {
			return err
		}
	}
	return nil
}

type statusResponseWriter struct {
	http.ResponseWriter
	status int
}

func (w *statusResponseWriter) WriteHeader(status int) {
	if w.status != 0 {
		return
	}
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}

func (w *statusResponseWriter) Write(data []byte) (int, error) {
	if w.status == 0 {
		w.WriteHeader(http.StatusOK)
	}
	return w.ResponseWriter.Write(data)
}

func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		recorder := &statusResponseWriter{ResponseWriter: w}

		next.ServeHTTP(recorder, r)

		status := recorder.status
		if status == 0 {
			status = http.StatusOK
		}
		slog.Info("http request completed",
			"method", r.Method,
			"path", r.URL.Path,
			"status", status,
			"latency", time.Since(start),
		)
	})
}

func recoveryMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if recovered := recover(); recovered != nil {
				slog.Error("http request panic recovered",
					"method", r.Method,
					"path", r.URL.Path,
					"error", recovered,
				)
				http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			}
		}()

		next.ServeHTTP(w, r)
	})
}
