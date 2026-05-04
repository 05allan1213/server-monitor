package health

import (
	"encoding/json"
	"net/http"
	"sync/atomic"
)

type Handler struct {
	ready atomic.Bool
}

func NewHandler() *Handler {
	h := &Handler{}
	h.ready.Store(true)
	return h
}

func (h *Handler) SetReady(ready bool) {
	h.ready.Store(ready)
}

func (h *Handler) Healthz(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"status": "success",
		"data": map[string]bool{
			"healthy": true,
		},
	})
}

func (h *Handler) Readyz(w http.ResponseWriter, _ *http.Request) {
	if !h.ready.Load() {
		writeJSON(w, http.StatusServiceUnavailable, map[string]interface{}{
			"status": "error",
			"data": map[string]bool{
				"ready": false,
			},
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"status": "success",
		"data": map[string]bool{
			"ready": true,
		},
	})
}

func writeJSON(w http.ResponseWriter, status int, payload interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}
