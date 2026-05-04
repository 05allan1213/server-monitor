package config

import (
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	ListenAddr            string
	HTTPReadHeaderTimeout time.Duration
	HTTPReadTimeout       time.Duration
	HTTPWriteTimeout      time.Duration
	HTTPIdleTimeout       time.Duration
	ShutdownTimeout       time.Duration
	KafkaBrokers          []string
	KafkaGroupID          string
	RedisAddr             string
	RedisPassword         string
	LogLevel              string
	TraceOTLPEndpoint     string
	TraceSampleRate       float64
}

func Load() Config {
	return Config{
		ListenAddr:            getEnv("LISTEN_ADDR", ":8081"),
		HTTPReadHeaderTimeout: getEnvDurationSeconds("HTTP_READ_HEADER_TIMEOUT_SECONDS", 5),
		HTTPReadTimeout:       getEnvDurationSeconds("HTTP_READ_TIMEOUT_SECONDS", 15),
		HTTPWriteTimeout:      getEnvDurationSeconds("HTTP_WRITE_TIMEOUT_SECONDS", 30),
		HTTPIdleTimeout:       getEnvDurationSeconds("HTTP_IDLE_TIMEOUT_SECONDS", 120),
		ShutdownTimeout:       getEnvDurationSeconds("SHUTDOWN_TIMEOUT_SECONDS", 10),
		KafkaBrokers:          getEnvList("KAFKA_BROKERS", []string{"kafka:9092"}),
		KafkaGroupID:          getEnvNonEmpty("KAFKA_GROUP_ID", "alert-service"),
		RedisAddr:             getEnvNonEmpty("REDIS_ADDR", "redis:6379"),
		RedisPassword:         getEnv("REDIS_PASSWORD", ""),
		LogLevel:              getEnvNonEmpty("LOG_LEVEL", "info"),
		TraceOTLPEndpoint:     getEnvNonEmpty("TRACE_OTLP_ENDPOINT", "jaeger:4317"),
		TraceSampleRate:       getEnvFloatRange("TRACE_SAMPLE_RATE", 1.0, 0, 1),
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

func getEnvDurationSeconds(key string, fallback int) time.Duration {
	return time.Duration(getEnvPositiveInt(key, fallback)) * time.Second
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

func getEnvList(key string, fallback []string) []string {
	value, exists := os.LookupEnv(key)
	if !exists {
		return append([]string(nil), fallback...)
	}

	parts := strings.Split(strings.TrimSpace(value), ",")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}
