package httpmiddleware

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"sync/atomic"
	"time"

	"go.uber.org/zap"

	"server-monitor/pkg/logger"
)

const RequestIDHeader = "X-Request-ID"

type requestIDContextKey struct{}

var requestIDCounter uint64

type RecoveryFields func(*http.Request) []zap.Field

type StatusRecorder struct {
	http.ResponseWriter
	status int
}

func NewStatusRecorder(w http.ResponseWriter) *StatusRecorder {
	return &StatusRecorder{ResponseWriter: w}
}

func (r *StatusRecorder) WriteHeader(status int) {
	if r.status != 0 {
		return
	}
	r.status = status
	r.ResponseWriter.WriteHeader(status)
}

func (r *StatusRecorder) Write(body []byte) (int, error) {
	if r.status == 0 {
		r.WriteHeader(http.StatusOK)
	}
	return r.ResponseWriter.Write(body)
}

func (r *StatusRecorder) Status() int {
	if r.status == 0 {
		return http.StatusOK
	}
	return r.status
}

func Recovery(next http.Handler, fields RecoveryFields) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if recovered := recover(); recovered != nil {
				logFields := []zap.Field{zap.Any("error", recovered)}
				if fields != nil {
					logFields = append(fields(r), logFields...)
				}
				logger.FromContext(r.Context()).Error("http request panic recovered", logFields...)
				WriteErrorJSON(w, http.StatusInternalServerError)
			}
		}()

		next.ServeHTTP(w, r)
	})
}

func WriteErrorJSON(w http.ResponseWriter, status int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{
		"status": "error",
		"error":  http.StatusText(status),
	})
}

func EnsureRequestID(w http.ResponseWriter, r *http.Request, now time.Time) (*http.Request, string) {
	requestID := r.Header.Get(RequestIDHeader)
	if requestID == "" {
		requestID = NewRequestID(now)
	}
	w.Header().Set(RequestIDHeader, requestID)
	return r.WithContext(context.WithValue(r.Context(), requestIDContextKey{}, requestID)), requestID
}

func NewRequestID(now time.Time) string {
	seq := atomic.AddUint64(&requestIDCounter, 1)
	return strconv.FormatInt(now.UnixNano(), 36) + "-" + strconv.FormatUint(seq, 36)
}

func RequestIDFromRequest(r *http.Request) string {
	if r == nil {
		return ""
	}
	if value, ok := r.Context().Value(requestIDContextKey{}).(string); ok {
		return value
	}
	return r.Header.Get(RequestIDHeader)
}
