package config

import (
	"log/slog"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	ListenAddr     string
	MetricsPath    string
	ScrapeInterval time.Duration
	Hostname       string
	HostProc       string
	HostSys        string
}

func Load() Config {
	return Config{
		ListenAddr:     getEnvNonEmpty("LISTEN_ADDR", ":9090"),
		MetricsPath:    getEnvPath("METRICS_PATH", "/metrics"),
		ScrapeInterval: time.Duration(getEnvInt("SCRAPE_INTERVAL", 5)) * time.Second,
		Hostname:       getEnv("HOSTNAME", getHostname()),
		HostProc:       getEnv("HOST_PROC", ""),
		HostSys:        getEnv("HOST_SYS", ""),
	}
}

func getHostname() string {
	h, err := os.Hostname()
	if err != nil {
		slog.Warn("hostname lookup failed", "error", err)
	}
	if h != "" {
		return h
	}
	return "unknown"
}

func getEnv(key, fallback string) string {
	value, exists := os.LookupEnv(key)
	if !exists {
		return fallback
	}
	return value
}

func getEnvNonEmpty(key, fallback string) string {
	value, exists := os.LookupEnv(key)
	if !exists || strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}

func getEnvPath(key, fallback string) string {
	value := getEnvNonEmpty(key, fallback)
	if !strings.HasPrefix(value, "/") {
		return fallback
	}
	return value
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
