package config

import (
	"os"
	"strconv"
	"time"
)

type Config struct {
	ScrapeInterval time.Duration
	MetricsPath    string
	ListenAddr     string
	Hostname       string
}

func Load() Config {
	return Config{
		ScrapeInterval: time.Duration(getEnvInt("SCRAPE_INTERVAL", 5)) * time.Second,
		MetricsPath:    getEnv("METRICS_PATH", "/metrics"),
		ListenAddr:     getEnv("LISTEN_ADDR", ":9090"),
		Hostname:       getHostname(),
	}
}

func getEnv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func getEnvInt(key string, fallback int) int {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}

	parsed, err := strconv.Atoi(value)
	if err != nil || parsed <= 0 {
		return fallback
	}
	return parsed
}

func getHostname() string {
	hostname, err := os.Hostname()
	if err != nil || hostname == "" {
		return "unknown"
	}
	return hostname
}
