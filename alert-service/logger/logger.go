package logger

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"syscall"
	"time"

	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

const defaultLevel = zapcore.InfoLevel

func Init(service, rawLevel string) (*zap.Logger, error) {
	level, err := parseLevel(rawLevel)
	if err != nil {
		return nil, err
	}

	cfg := zap.Config{
		Level:            zap.NewAtomicLevelAt(level),
		Development:      false,
		Encoding:         "json",
		EncoderConfig:    newEncoderConfig(),
		OutputPaths:      []string{"stdout"},
		ErrorOutputPaths: []string{"stderr"},
	}

	base, err := cfg.Build()
	if err != nil {
		return nil, err
	}

	log := base.With(
		zap.String("service", service),
		zap.String("instance", instance()),
	)
	zap.ReplaceGlobals(log)
	return log, nil
}

func Sync(log *zap.Logger) {
	if log == nil {
		return
	}
	if err := log.Sync(); err != nil && !isIgnorableSyncError(err) {
		fmt.Fprintf(os.Stderr, "logger sync failed: %v\n", err)
	}
}

func FromContext(ctx context.Context) *zap.Logger {
	log := zap.L()
	spanCtx := trace.SpanContextFromContext(ctx)
	if !spanCtx.IsValid() {
		return log
	}
	return log.With(
		zap.String("trace_id", spanCtx.TraceID().String()),
		zap.String("span_id", spanCtx.SpanID().String()),
	)
}

func parseLevel(raw string) (zapcore.Level, error) {
	value := strings.TrimSpace(strings.ToLower(raw))
	if value == "" {
		return defaultLevel, nil
	}

	var level zapcore.Level
	if err := level.UnmarshalText([]byte(value)); err != nil {
		return defaultLevel, fmt.Errorf("invalid LOG_LEVEL %q", raw)
	}
	return level, nil
}

func instance() string {
	for _, key := range []string{"INSTANCE", "HOSTNAME"} {
		if value := strings.TrimSpace(os.Getenv(key)); value != "" {
			return value
		}
	}

	hostname, err := os.Hostname()
	if err == nil && strings.TrimSpace(hostname) != "" {
		return hostname
	}
	return "unknown"
}

func isIgnorableSyncError(err error) bool {
	return errors.Is(err, syscall.EINVAL) ||
		errors.Is(err, syscall.ENOTTY) ||
		errors.Is(err, syscall.EBADF)
}

func newEncoderConfig() zapcore.EncoderConfig {
	return zapcore.EncoderConfig{
		TimeKey:        "ts",
		LevelKey:       "level",
		NameKey:        "logger",
		CallerKey:      "caller",
		FunctionKey:    zapcore.OmitKey,
		MessageKey:     "msg",
		StacktraceKey:  "stacktrace",
		LineEnding:     zapcore.DefaultLineEnding,
		EncodeLevel:    zapcore.LowercaseLevelEncoder,
		EncodeTime:     utcMillisTimeEncoder,
		EncodeDuration: zapcore.MillisDurationEncoder,
		EncodeCaller:   zapcore.ShortCallerEncoder,
		EncodeName:     zapcore.FullNameEncoder,
	}
}

func utcMillisTimeEncoder(t time.Time, enc zapcore.PrimitiveArrayEncoder) {
	enc.AppendString(t.UTC().Format("2006-01-02T15:04:05.000Z"))
}
