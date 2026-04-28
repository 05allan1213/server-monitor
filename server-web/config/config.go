package config

import (
	"os"
	"strconv"
	"time"
)

type Config struct {
	ListenAddr     string
	PrometheusURL  string
	RequestTimeout time.Duration
}

func Load() Config {
	return Config{
		ListenAddr:     getEnv("LISTEN_ADDR", ":8080"),
		PrometheusURL:  getEnv("PROMETHEUS_URL", "http://prometheus:9090"),
		RequestTimeout: time.Duration(getEnvInt("REQUEST_TIMEOUT_SECONDS", 5)) * time.Second,
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
