package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.uber.org/zap"

	"server-probe/collector"
	"server-probe/config"

	"server-monitor/pkg/httpmiddleware"
	"server-monitor/pkg/logger"
	"server-monitor/pkg/tracer"
)

const requestIDHeader = "X-Request-ID"

type requestIDContextKey struct{}

var requestIDCounter uint64

func main() {
	log, err := logger.Init("server-probe")
	if err != nil {
		fmt.Fprintf(os.Stderr, "logger init failed: %v\n", err)
		os.Exit(1)
	}
	defer logger.Sync(log)

	cfg := config.Load()
	shutdownTracer, err := tracer.Init(context.Background(), tracer.Config{
		ServiceName:  "server-probe",
		OTLPEndpoint: cfg.TraceOTLPEndpoint,
		SampleRate:   cfg.TraceSampleRate,
	})
	if err != nil {
		zap.L().Warn("tracer init failed; tracing disabled",
			zap.String("endpoint", cfg.TraceOTLPEndpoint),
			zap.Error(err),
		)
		shutdownTracer = func(context.Context) error { return nil }
	} else if cfg.TraceOTLPEndpoint != "" {
		zap.L().Info("tracer initialized",
			zap.String("endpoint", cfg.TraceOTLPEndpoint),
			zap.Float64("sample_rate", cfg.TraceSampleRate),
		)
	}

	if err := applyHostPaths(cfg); err != nil {
		zap.L().Error("apply host paths failed", zap.Error(err))
		os.Exit(1)
	}

	registry := prometheus.NewRegistry()

	collectors := []collector.Collector{
		collector.NewCPUCollector(cfg.Hostname),
		collector.NewMemoryCollector(cfg.Hostname),
		collector.NewDiskCollector(cfg.Hostname),
		collector.NewNetworkCollector(cfg.Hostname),
		collector.NewLoadCollector(cfg.Hostname),
		collector.NewProcessCollector(cfg.Hostname),
	}

	for _, c := range collectors {
		c.Register(registry)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	updateCollectors(ctx, collectors)

	go func() {
		ticker := time.NewTicker(cfg.ScrapeInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				updateCollectors(ctx, collectors)
			}
		}
	}()

	mux := http.NewServeMux()
	mux.Handle(cfg.MetricsPath, promhttp.HandlerFor(registry, promhttp.HandlerOpts{
		MaxRequestsInFlight: cfg.PromHTTPMaxRequestsInFlight,
		Timeout:             cfg.PromHTTPTimeout,
	}))
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		if _, err := w.Write([]byte(`{"healthy":true}`)); err != nil {
			zap.L().Error("healthz response write failed", zap.Error(err))
		}
	})
	mux.HandleFunc("/readyz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		if _, err := w.Write([]byte(`{"ready":true}`)); err != nil {
			zap.L().Error("readyz response write failed", zap.Error(err))
		}
	})

	srv := &http.Server{
		Addr:         cfg.ListenAddr,
		Handler:      tracedHandler(loggingMiddleware(recoveryMiddleware(mux))),
		ReadTimeout:  cfg.HTTPReadTimeout,
		WriteTimeout: cfg.HTTPWriteTimeout,
		IdleTimeout:  cfg.HTTPIdleTimeout,
	}

	serverErr := make(chan error, 1)
	go func() {
		zap.L().Info("server-probe listening",
			zap.String("addr", cfg.ListenAddr),
			zap.String("metrics_path", cfg.MetricsPath),
		)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			serverErr <- err
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(quit)

	exitCode := 0
	select {
	case sig := <-quit:
		zap.L().Info("server-probe shutting down", zap.String("signal", sig.String()))
	case err := <-serverErr:
		zap.L().Error("server-probe exited", zap.Error(err))
		exitCode = 1
	}

	cancel()

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)

	if err := srv.Shutdown(shutdownCtx); err != nil {
		zap.L().Error("server-probe shutdown error", zap.Error(err))
	}
	shutdownCancel()

	traceShutdownCtx, traceShutdownCancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
	if err := shutdownTracer(traceShutdownCtx); err != nil {
		zap.L().Warn("tracer shutdown failed", zap.Error(err))
	}
	traceShutdownCancel()

	zap.L().Info("server-probe stopped")
	if exitCode != 0 {
		os.Exit(exitCode)
	}
}

func updateCollectors(ctx context.Context, collectors []collector.Collector) {
	var wg sync.WaitGroup

	for _, c := range collectors {
		if err := ctx.Err(); err != nil {
			break
		}

		wg.Add(1)
		go func(c collector.Collector) {
			defer wg.Done()

			if err := c.Update(ctx); err != nil {
				if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
					return
				}
				zap.L().Error("collector update failed",
					zap.String("collector", c.Name()),
					zap.Error(err),
				)
			}
		}(c)
	}

	wg.Wait()
}

func applyHostPaths(cfg config.Config) error {
	if cfg.HostProc != "" {
		if err := os.Setenv("HOST_PROC", cfg.HostProc); err != nil {
			return err
		}
	}
	if cfg.HostSys != "" {
		if err := os.Setenv("HOST_SYS", cfg.HostSys); err != nil {
			return err
		}
	}
	return nil
}

func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		recorder := httpmiddleware.NewStatusRecorder(w)
		requestID := r.Header.Get(requestIDHeader)
		if requestID == "" {
			requestID = newRequestID(start)
		}
		recorder.Header().Set(requestIDHeader, requestID)
		r = r.WithContext(context.WithValue(r.Context(), requestIDContextKey{}, requestID))

		next.ServeHTTP(recorder, r)

		latency := time.Since(start)
		logger.FromContext(r.Context()).Info("http request completed",
			zap.String("request_id", requestID),
			zap.String("method", r.Method),
			zap.String("path", r.URL.Path),
			zap.Int("status", recorder.Status()),
			zap.Float64("latency_ms", float64(latency.Microseconds())/1000),
			zap.String("client_ip", r.RemoteAddr),
		)
	})
}

func tracedHandler(next http.Handler) http.Handler {
	return otelhttp.NewHandler(next, "server-probe")
}

func newRequestID(now time.Time) string {
	seq := atomic.AddUint64(&requestIDCounter, 1)
	return strconv.FormatInt(now.UnixNano(), 36) + "-" + strconv.FormatUint(seq, 36)
}

func requestIDFromRequest(r *http.Request) string {
	if r == nil {
		return ""
	}
	if value, ok := r.Context().Value(requestIDContextKey{}).(string); ok {
		return value
	}
	return r.Header.Get(requestIDHeader)
}

func recoveryMiddleware(next http.Handler) http.Handler {
	return httpmiddleware.Recovery(next, func(r *http.Request) []zap.Field {
		return []zap.Field{
			zap.String("request_id", requestIDFromRequest(r)),
			zap.String("method", r.Method),
			zap.String("path", r.URL.Path),
		}
	})
}
