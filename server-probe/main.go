package main

import (
	"log"
	"net/http"
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

	updateCollectors(collectors)

	go func() {
		ticker := time.NewTicker(cfg.ScrapeInterval)
		defer ticker.Stop()

		for range ticker.C {
			updateCollectors(collectors)
		}
	}()

	mux := http.NewServeMux()
	mux.Handle(cfg.MetricsPath, promhttp.HandlerFor(registry, promhttp.HandlerOpts{}))

	log.Printf("server-probe listening on %s%s", cfg.ListenAddr, cfg.MetricsPath)
	if err := http.ListenAndServe(cfg.ListenAddr, mux); err != nil {
		log.Fatalf("server-probe exited: %v", err)
	}
}

func updateCollectors(collectors []collector.Collector) {
	for _, c := range collectors {
		if err := c.Update(); err != nil {
			log.Printf("collector %s update failed: %v", c.Name(), err)
		}
	}
}
