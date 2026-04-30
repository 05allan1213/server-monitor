package main

import (
	"context"
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

	updateCollectors(collectors)

	go func() {
		ticker := time.NewTicker(cfg.ScrapeInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				updateCollectors(collectors)
			}
		}
	}()

	mux := http.NewServeMux()
	mux.Handle(cfg.MetricsPath, promhttp.HandlerFor(registry, promhttp.HandlerOpts{}))
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"healthy":true}`))
	})

	srv := &http.Server{
		Addr:         cfg.ListenAddr,
		Handler:      mux,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
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

func updateCollectors(collectors []collector.Collector) {
	for _, c := range collectors {
		if err := c.Update(); err != nil {
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
