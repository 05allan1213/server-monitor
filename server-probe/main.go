package main

import (
	"context"
	"log"
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
	registry := prometheus.NewRegistry()

	collectors := []collector.Collector{
		collector.NewCPUCollector(cfg.Hostname),
		collector.NewMemoryCollector(cfg.Hostname),
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

	srv := &http.Server{
		Addr:         cfg.ListenAddr,
		Handler:      mux,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	go func() {
		log.Printf("server-probe listening on %s%s", cfg.ListenAddr, cfg.MetricsPath)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("server-probe exited: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Printf("server-probe shutting down...")
	cancel()

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Printf("server-probe shutdown error: %v", err)
	}

	log.Printf("server-probe stopped")
}

func updateCollectors(collectors []collector.Collector) {
	for _, c := range collectors {
		if err := c.Update(); err != nil {
			log.Printf("collector %s update failed: %v", c.Name(), err)
		}
	}
}
