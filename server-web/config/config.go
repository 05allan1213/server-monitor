package config

import (
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	ListenAddr     string
	PrometheusURL  string
	RequestTimeout time.Duration
	ReadyTimeout   time.Duration
	HostsCacheTTL  time.Duration
	GinMode        string
	TrustedProxies []string
	CORSOrigins    []string
	RedisAddr      string
	RedisPassword  string
	RedisDB        int
	StaticDir      string
}

func Load() Config {
	return Config{
		ListenAddr:     getEnv("LISTEN_ADDR", ":8080"),
		PrometheusURL:  getEnv("PROMETHEUS_URL", "http://prometheus:9090"),
		RequestTimeout: time.Duration(getEnvInt("REQUEST_TIMEOUT_SECONDS", 5)) * time.Second,
		ReadyTimeout:   time.Duration(getEnvInt("READY_TIMEOUT_SECONDS", 3)) * time.Second,
		HostsCacheTTL:  time.Duration(getEnvInt("HOSTS_CACHE_TTL_SECONDS", 30)) * time.Second,
		GinMode:        getEnv("GIN_MODE", "debug"),
		TrustedProxies: getEnvList("TRUSTED_PROXIES"),
		CORSOrigins:    getEnvList("CORS_ALLOWED_ORIGINS"),
		RedisAddr:      getEnv("REDIS_ADDR", ""),
		RedisPassword:  getEnv("REDIS_PASSWORD", ""),
		RedisDB:        getEnvInt("REDIS_DB", 0),
		StaticDir:      getEnv("STATIC_DIR", ""),
	}
}

func getEnv(key, fallback string) string {
	value, exists := os.LookupEnv(key)
	if !exists {
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
	if err != nil || parsed < 0 {
		return fallback
	}
	return parsed
}

func getEnvList(key string) []string {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return nil
	}

	parts := strings.Split(value, ",")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}
