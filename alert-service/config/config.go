package config

import (
	"time"

	"server-monitor/pkg/configutil"
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
	TraceOTLPEndpoint     string
	TraceSampleRate       float64
}

func Load() Config {
	return Config{
		ListenAddr:            configutil.String("LISTEN_ADDR", ":8081"),
		HTTPReadHeaderTimeout: configutil.DurationSeconds("HTTP_READ_HEADER_TIMEOUT_SECONDS", 5),
		HTTPReadTimeout:       configutil.DurationSeconds("HTTP_READ_TIMEOUT_SECONDS", 15),
		HTTPWriteTimeout:      configutil.DurationSeconds("HTTP_WRITE_TIMEOUT_SECONDS", 30),
		HTTPIdleTimeout:       configutil.DurationSeconds("HTTP_IDLE_TIMEOUT_SECONDS", 120),
		ShutdownTimeout:       configutil.DurationSeconds("SHUTDOWN_TIMEOUT_SECONDS", 10),
		KafkaBrokers:          configutil.ListWithFallback("KAFKA_BROKERS", []string{"kafka:9092"}),
		KafkaGroupID:          configutil.NonEmptyString("KAFKA_GROUP_ID", "alert-service"),
		RedisAddr:             configutil.NonEmptyString("REDIS_ADDR", "redis:6379"),
		RedisPassword:         configutil.String("REDIS_PASSWORD", ""),
		TraceOTLPEndpoint:     configutil.NonEmptyString("TRACE_OTLP_ENDPOINT", "jaeger:4317"),
		TraceSampleRate:       configutil.FloatRange("TRACE_SAMPLE_RATE", 1.0, 0, 1),
	}
}
