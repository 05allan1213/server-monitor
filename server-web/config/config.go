package config

import (
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	ListenAddr                      string
	PrometheusURL                   string
	RequestTimeout                  time.Duration
	ReadyTimeout                    time.Duration
	HTTPReadHeaderTimeout           time.Duration
	HTTPReadTimeout                 time.Duration
	HTTPWriteTimeout                time.Duration
	HTTPIdleTimeout                 time.Duration
	ShutdownTimeout                 time.Duration
	HostsBroadcastInterval          time.Duration
	HostsCacheTTL                   time.Duration
	DashboardOverviewTTL            time.Duration
	AlertEventDedupeTTL             time.Duration
	AlertmanagerWebhookMaxBodyBytes int64
	CacheWriteTimeout               time.Duration
	GinMode                         string
	TrustedProxies                  []string
	CORSOrigins                     []string
	RateLimit                       RateLimitConfig
	RedisAddr                       string
	RedisPassword                   string
	RedisDB                         int
	RedisStartupTimeout             time.Duration
	RedisDialTimeout                time.Duration
	RedisReadTimeout                time.Duration
	RedisWriteTimeout               time.Duration
	RedisConnMaxLifetime            time.Duration
	RedisConnMaxIdleTime            time.Duration
	MySQLHost                       string
	MySQLPort                       string
	MySQLUser                       string
	MySQLPassword                   string
	MySQLDatabase                   string
	MySQLStartupTimeout             time.Duration
	MySQLPingTimeout                time.Duration
	StaticDir                       string
	TraceOTLPEndpoint               string
	TraceSampleRate                 float64
	KafkaBrokers                    []string
}

type RateLimitConfig struct {
	Enabled          bool
	Requests         int64
	Window           time.Duration
	OperationTimeout time.Duration
}

func Load() Config {
	return Config{
		ListenAddr:                      getEnv("LISTEN_ADDR", ":8080"),
		PrometheusURL:                   getEnv("PROMETHEUS_URL", "http://prometheus:9090"),
		RequestTimeout:                  getEnvDurationSeconds("REQUEST_TIMEOUT_SECONDS", 5),
		ReadyTimeout:                    getEnvDurationSeconds("READY_TIMEOUT_SECONDS", 3),
		HTTPReadHeaderTimeout:           getEnvDurationSeconds("HTTP_READ_HEADER_TIMEOUT_SECONDS", 5),
		HTTPReadTimeout:                 getEnvDurationSeconds("HTTP_READ_TIMEOUT_SECONDS", 15),
		HTTPWriteTimeout:                getEnvDurationSeconds("HTTP_WRITE_TIMEOUT_SECONDS", 30),
		HTTPIdleTimeout:                 getEnvDurationSeconds("HTTP_IDLE_TIMEOUT_SECONDS", 120),
		ShutdownTimeout:                 getEnvDurationSeconds("SHUTDOWN_TIMEOUT_SECONDS", 5),
		HostsBroadcastInterval:          getEnvDurationSeconds("HOSTS_BROADCAST_INTERVAL_SECONDS", 5),
		HostsCacheTTL:                   getEnvDurationSeconds("HOSTS_CACHE_TTL_SECONDS", 30),
		DashboardOverviewTTL:            getEnvDurationSeconds("DASHBOARD_OVERVIEW_TTL_SECONDS", 10),
		AlertEventDedupeTTL:             getEnvDurationSeconds("ALERT_EVENT_DEDUPE_TTL_SECONDS", 86400),
		AlertmanagerWebhookMaxBodyBytes: int64(getEnvPositiveInt("ALERTMANAGER_WEBHOOK_MAX_BODY_BYTES", 1048576)),
		CacheWriteTimeout:               getEnvDurationSeconds("CACHE_WRITE_TIMEOUT_SECONDS", 3),
		GinMode:                         getEnv("GIN_MODE", "debug"),
		TrustedProxies:                  getEnvList("TRUSTED_PROXIES"),
		CORSOrigins:                     getEnvList("CORS_ALLOWED_ORIGINS"),
		RateLimit: RateLimitConfig{
			Enabled:          getEnvBool("RATE_LIMIT_ENABLED", false),
			Requests:         int64(getEnvPositiveInt("RATE_LIMIT_REQUESTS", 120)),
			Window:           getEnvDurationSeconds("RATE_LIMIT_WINDOW_SECONDS", 60),
			OperationTimeout: getEnvDurationMilliseconds("RATE_LIMIT_OPERATION_TIMEOUT_MILLISECONDS", 500),
		},
		RedisAddr:            getEnv("REDIS_ADDR", ""),
		RedisPassword:        getEnv("REDIS_PASSWORD", ""),
		RedisDB:              getEnvInt("REDIS_DB", 0),
		RedisStartupTimeout:  getEnvDurationSeconds("REDIS_STARTUP_TIMEOUT_SECONDS", 5),
		RedisDialTimeout:     getEnvDurationSeconds("REDIS_DIAL_TIMEOUT_SECONDS", 5),
		RedisReadTimeout:     getEnvDurationSeconds("REDIS_READ_TIMEOUT_SECONDS", 3),
		RedisWriteTimeout:    getEnvDurationSeconds("REDIS_WRITE_TIMEOUT_SECONDS", 3),
		RedisConnMaxLifetime: getEnvDurationSeconds("REDIS_CONN_MAX_LIFETIME_SECONDS", 1800),
		RedisConnMaxIdleTime: getEnvDurationSeconds("REDIS_CONN_MAX_IDLE_TIME_SECONDS", 300),
		MySQLHost:            getEnv("MYSQL_HOST", ""),
		MySQLPort:            getEnv("MYSQL_PORT", "3306"),
		MySQLUser:            getEnv("MYSQL_USER", ""),
		MySQLPassword:        getEnv("MYSQL_PASSWORD", ""),
		MySQLDatabase:        getEnv("MYSQL_DATABASE", ""),
		MySQLStartupTimeout:  getEnvDurationSeconds("MYSQL_STARTUP_TIMEOUT_SECONDS", 5),
		MySQLPingTimeout:     getEnvDurationSeconds("MYSQL_PING_TIMEOUT_SECONDS", 3),
		StaticDir:            getEnv("STATIC_DIR", ""),
		TraceOTLPEndpoint:    getEnvNonEmpty("TRACE_OTLP_ENDPOINT", ""),
		TraceSampleRate:      getEnvFloatRange("TRACE_SAMPLE_RATE", 1.0, 0, 1),
		KafkaBrokers:         getEnvList("KAFKA_BROKERS"),
	}
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

func getEnvPositiveInt(key string, fallback int) int {
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
	return time.Duration(getEnvPositiveInt(key, fallback)) * time.Second
}

func getEnvDurationMilliseconds(key string, fallback int) time.Duration {
	return time.Duration(getEnvPositiveInt(key, fallback)) * time.Millisecond
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

func getEnvBool(key string, fallback bool) bool {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	parsed, err := strconv.ParseBool(value)
	if err != nil {
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
