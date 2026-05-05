package config

import (
	"fmt"
	"strings"
	"time"

	"server-monitor/pkg/configutil"
)

type Config struct {
	ListenAddr                      string
	PrometheusURL                   string
	PrometheusReloadURL             string
	AlertRulesFilePath              string
	AlertRuleSyncEnabled            bool
	PromtoolPath                    string
	AlertRuleSyncTimeout            time.Duration
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
	JWTSecret                       string
	JWTExpireHours                  int
	AuthEnabled                     bool
	AdminPassword                   string
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
	prometheusURL := configutil.String("PROMETHEUS_URL", "http://prometheus:9090")
	return Config{
		ListenAddr:                      configutil.String("LISTEN_ADDR", ":8080"),
		PrometheusURL:                   prometheusURL,
		PrometheusReloadURL:             configutil.NonEmptyString("PROMETHEUS_RELOAD_URL", strings.TrimRight(prometheusURL, "/")+"/-/reload"),
		AlertRulesFilePath:              configutil.String("ALERT_RULES_FILE_PATH", ""),
		AlertRuleSyncEnabled:            configutil.Bool("ALERT_RULE_SYNC_ENABLED", true),
		PromtoolPath:                    configutil.String("PROMTOOL_PATH", "promtool"),
		AlertRuleSyncTimeout:            configutil.DurationSeconds("ALERT_RULE_SYNC_TIMEOUT_SECONDS", 10),
		RequestTimeout:                  configutil.DurationSeconds("REQUEST_TIMEOUT_SECONDS", 5),
		ReadyTimeout:                    configutil.DurationSeconds("READY_TIMEOUT_SECONDS", 3),
		HTTPReadHeaderTimeout:           configutil.DurationSeconds("HTTP_READ_HEADER_TIMEOUT_SECONDS", 5),
		HTTPReadTimeout:                 configutil.DurationSeconds("HTTP_READ_TIMEOUT_SECONDS", 15),
		HTTPWriteTimeout:                configutil.DurationSeconds("HTTP_WRITE_TIMEOUT_SECONDS", 30),
		HTTPIdleTimeout:                 configutil.DurationSeconds("HTTP_IDLE_TIMEOUT_SECONDS", 120),
		ShutdownTimeout:                 configutil.DurationSeconds("SHUTDOWN_TIMEOUT_SECONDS", 5),
		HostsBroadcastInterval:          configutil.DurationSeconds("HOSTS_BROADCAST_INTERVAL_SECONDS", 5),
		HostsCacheTTL:                   configutil.DurationSeconds("HOSTS_CACHE_TTL_SECONDS", 30),
		DashboardOverviewTTL:            configutil.DurationSeconds("DASHBOARD_OVERVIEW_TTL_SECONDS", 10),
		AlertEventDedupeTTL:             configutil.DurationSeconds("ALERT_EVENT_DEDUPE_TTL_SECONDS", 86400),
		AlertmanagerWebhookMaxBodyBytes: int64(configutil.PositiveInt("ALERTMANAGER_WEBHOOK_MAX_BODY_BYTES", 1048576)),
		CacheWriteTimeout:               configutil.DurationSeconds("CACHE_WRITE_TIMEOUT_SECONDS", 3),
		GinMode:                         configutil.String("GIN_MODE", "debug"),
		TrustedProxies:                  configutil.List("TRUSTED_PROXIES"),
		CORSOrigins:                     configutil.List("CORS_ALLOWED_ORIGINS"),
		RateLimit: RateLimitConfig{
			Enabled:          configutil.Bool("RATE_LIMIT_ENABLED", false),
			Requests:         int64(configutil.PositiveInt("RATE_LIMIT_REQUESTS", 120)),
			Window:           configutil.DurationSeconds("RATE_LIMIT_WINDOW_SECONDS", 60),
			OperationTimeout: configutil.DurationMilliseconds("RATE_LIMIT_OPERATION_TIMEOUT_MILLISECONDS", 500),
		},
		RedisAddr:            configutil.String("REDIS_ADDR", ""),
		RedisPassword:        configutil.String("REDIS_PASSWORD", ""),
		RedisDB:              configutil.NonNegativeInt("REDIS_DB", 0),
		RedisStartupTimeout:  configutil.DurationSeconds("REDIS_STARTUP_TIMEOUT_SECONDS", 5),
		RedisDialTimeout:     configutil.DurationSeconds("REDIS_DIAL_TIMEOUT_SECONDS", 5),
		RedisReadTimeout:     configutil.DurationSeconds("REDIS_READ_TIMEOUT_SECONDS", 3),
		RedisWriteTimeout:    configutil.DurationSeconds("REDIS_WRITE_TIMEOUT_SECONDS", 3),
		RedisConnMaxLifetime: configutil.DurationSeconds("REDIS_CONN_MAX_LIFETIME_SECONDS", 1800),
		RedisConnMaxIdleTime: configutil.DurationSeconds("REDIS_CONN_MAX_IDLE_TIME_SECONDS", 300),
		MySQLHost:            configutil.String("MYSQL_HOST", ""),
		MySQLPort:            configutil.String("MYSQL_PORT", "3306"),
		MySQLUser:            configutil.String("MYSQL_USER", ""),
		MySQLPassword:        configutil.String("MYSQL_PASSWORD", ""),
		MySQLDatabase:        configutil.String("MYSQL_DATABASE", ""),
		MySQLStartupTimeout:  configutil.DurationSeconds("MYSQL_STARTUP_TIMEOUT_SECONDS", 5),
		MySQLPingTimeout:     configutil.DurationSeconds("MYSQL_PING_TIMEOUT_SECONDS", 3),
		JWTSecret:            configutil.String("JWT_SECRET", ""),
		JWTExpireHours:       configutil.PositiveInt("JWT_EXPIRE_HOURS", 24),
		AuthEnabled:          configutil.Bool("AUTH_ENABLED", true),
		AdminPassword:        configutil.String("ADMIN_PASSWORD", ""),
		StaticDir:            configutil.String("STATIC_DIR", ""),
		TraceOTLPEndpoint:    configutil.NonEmptyString("TRACE_OTLP_ENDPOINT", ""),
		TraceSampleRate:      configutil.FloatRange("TRACE_SAMPLE_RATE", 1.0, 0, 1),
		KafkaBrokers:         configutil.List("KAFKA_BROKERS"),
	}
}

func (c Config) Validate() error {
	if c.AuthEnabled && len(strings.TrimSpace(c.JWTSecret)) < 32 {
		return fmt.Errorf("JWT_SECRET must be at least 32 bytes when auth is enabled, got %d", len(strings.TrimSpace(c.JWTSecret)))
	}
	return nil
}
