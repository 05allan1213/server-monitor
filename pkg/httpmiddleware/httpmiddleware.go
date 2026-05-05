package httpmiddleware

import (
	"encoding/json"
	"net/http"

	"go.uber.org/zap"

	"server-monitor/pkg/logger"
)

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
