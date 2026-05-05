package config

import (
	"os"
	"strings"
	"time"

	"go.uber.org/zap"

	"server-monitor/pkg/configutil"
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
		ListenAddr:                  configutil.NonEmptyString("LISTEN_ADDR", ":9090"),
		MetricsPath:                 getEnvPath("METRICS_PATH", "/metrics"),
		ScrapeInterval:              configutil.DurationSeconds("SCRAPE_INTERVAL", 5),
		PromHTTPMaxRequestsInFlight: configutil.PositiveInt("PROMHTTP_MAX_REQUESTS_IN_FLIGHT", 5),
		PromHTTPTimeout:             configutil.DurationSeconds("PROMHTTP_TIMEOUT", 5),
		HTTPReadTimeout:             configutil.DurationSeconds("HTTP_READ_TIMEOUT", 10),
		HTTPWriteTimeout:            configutil.DurationSeconds("HTTP_WRITE_TIMEOUT", 10),
		HTTPIdleTimeout:             configutil.DurationSeconds("HTTP_IDLE_TIMEOUT", 60),
		ShutdownTimeout:             configutil.DurationSeconds("SHUTDOWN_TIMEOUT", 5),
		Hostname:                    configutil.String("HOSTNAME", getHostname()),
		HostProc:                    configutil.String("HOST_PROC", ""),
		HostSys:                     configutil.String("HOST_SYS", ""),
		TraceOTLPEndpoint:           configutil.NonEmptyString("TRACE_OTLP_ENDPOINT", ""),
		TraceSampleRate:             configutil.FloatRange("TRACE_SAMPLE_RATE", 1.0, 0, 1),
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

func getEnvPath(key, fallback string) string {
	value := configutil.NonEmptyString(key, fallback)
	if !strings.HasPrefix(value, "/") {
		return fallback
	}
	return value
}
