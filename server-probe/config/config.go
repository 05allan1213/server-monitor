package config

import (
	"os"
	"strconv"
	"strings"
	"time"

	"go.uber.org/zap"
)

type Config struct {
	ListenAddr                  string
	MetricsPath                 string
	ScrapeInterval              time.Duration
	PromHTTPMaxRequestsInFlight int
	PromHTTPTimeout             time.Duration
	HTTPReadTimeout             time.Duration
	HTTPWriteTimeout            time.Duration
	HTTPIdleTimeout             time.Duration
	ShutdownTimeout             time.Duration
	Hostname                    string
	HostProc                    string
	HostSys                     string
	TraceOTLPEndpoint           string
	TraceSampleRate             float64
}

func Load() Config {
	return Config{
		ListenAddr:                  getEnvNonEmpty("LISTEN_ADDR", ":9090"),
		MetricsPath:                 getEnvPath("METRICS_PATH", "/metrics"),
		ScrapeInterval:              getEnvDurationSeconds("SCRAPE_INTERVAL", 5),
		PromHTTPMaxRequestsInFlight: getEnvInt("PROMHTTP_MAX_REQUESTS_IN_FLIGHT", 5),
		PromHTTPTimeout:             getEnvDurationSeconds("PROMHTTP_TIMEOUT", 5),
		HTTPReadTimeout:             getEnvDurationSeconds("HTTP_READ_TIMEOUT", 10),
		HTTPWriteTimeout:            getEnvDurationSeconds("HTTP_WRITE_TIMEOUT", 10),
		HTTPIdleTimeout:             getEnvDurationSeconds("HTTP_IDLE_TIMEOUT", 60),
		ShutdownTimeout:             getEnvDurationSeconds("SHUTDOWN_TIMEOUT", 5),
		Hostname:                    getEnv("HOSTNAME", getHostname()),
		HostProc:                    getEnv("HOST_PROC", ""),
		HostSys:                     getEnv("HOST_SYS", ""),
		TraceOTLPEndpoint:           getEnvNonEmpty("TRACE_OTLP_ENDPOINT", ""),
		TraceSampleRate:             getEnvFloatRange("TRACE_SAMPLE_RATE", 1.0, 0, 1),
	}
}

func getHostname() string {
	h, err := os.Hostname()
	if err != nil {
		zap.L().Warn("hostname lookup failed", zap.Error(err))
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

func getEnvDurationSeconds(key string, fallback int) time.Duration {
	return time.Duration(getEnvInt(key, fallback)) * time.Second
}

func getEnvFloatRange(key string, fallback, minValue, maxValue float64) float64 {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	parsed, err := strconv.ParseFloat(value, 64)
	if err != nil || parsed < minValue || parsed > maxValue {
		return fallback
	}
	return parsed
}
