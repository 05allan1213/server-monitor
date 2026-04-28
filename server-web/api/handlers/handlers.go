package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	promclient "server-web/prometheus"
	rediscache "server-web/redis"
	"server-web/webhook"
)

type Handler struct {
	promClient   *promclient.Client
	cacheClient  *rediscache.Client
	readyTimeout time.Duration
	hostsTTL     time.Duration
}

type response struct {
	Status string      `json:"status"`
	Data   interface{} `json:"data,omitempty"`
	Error  string      `json:"error,omitempty"`
}

func NewHandler(promClient *promclient.Client, cacheClient *rediscache.Client, readyTimeout time.Duration, hostsTTL time.Duration) *Handler {
	return &Handler{
		promClient:   promClient,
		cacheClient:  cacheClient,
		readyTimeout: readyTimeout,
		hostsTTL:     hostsTTL,
	}
}

func (h *Handler) Root(c *gin.Context) {
	c.JSON(http.StatusOK, response{
		Status: "success",
		Data: gin.H{
			"message": "server-web is running",
		},
	})
}

func (h *Handler) Healthz(c *gin.Context) {
	c.JSON(http.StatusOK, response{
		Status: "success",
		Data: gin.H{
			"healthy": true,
		},
	})
}

func (h *Handler) Readyz(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), h.readyTimeout)
	defer cancel()

	dependencies := gin.H{
		"prometheus": "ok",
		"redis":      "disabled",
	}

	var errors []string

	if err := h.promClient.Ready(ctx); err != nil {
		dependencies["prometheus"] = "unreachable"
		errors = append(errors, err.Error())
	}

	if h.cacheClient != nil && h.cacheClient.Enabled() {
		if err := h.cacheClient.Ping(ctx); err != nil {
			dependencies["redis"] = "unreachable"
			errors = append(errors, err.Error())
		} else {
			dependencies["redis"] = "ok"
		}
	}

	if len(errors) > 0 {
		c.JSON(http.StatusServiceUnavailable, response{
			Status: "error",
			Error:  fmt.Sprintf("readiness check failed: %s", strings.Join(errors, "; ")),
			Data: gin.H{
				"ready":        false,
				"dependencies": dependencies,
			},
		})
		return
	}

	c.JSON(http.StatusOK, response{
		Status: "success",
		Data: gin.H{
			"ready":        true,
			"dependencies": dependencies,
		},
	})
}

func (h *Handler) Hosts(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer cancel()

	if cachedHosts, ok := h.getCachedHosts(ctx); ok {
		c.JSON(http.StatusOK, response{
			Status: "success",
			Data:   cachedHosts,
		})
		return
	}

	hosts, err := h.promClient.GetHosts(ctx)
	if err != nil {
		c.JSON(http.StatusBadGateway, response{
			Status: "error",
			Error:  err.Error(),
		})
		return
	}

	h.cacheHosts(ctx, hosts)

	c.JSON(http.StatusOK, response{
		Status: "success",
		Data:   hosts,
	})
}

func (h *Handler) AlertmanagerWebhook(c *gin.Context) {
	if h.cacheClient == nil || !h.cacheClient.Enabled() {
		c.JSON(http.StatusServiceUnavailable, response{
			Status: "error",
			Error:  "redis is required for alert webhook handling",
		})
		return
	}

	var payload webhook.AlertmanagerWebhookRequest
	if err := c.ShouldBindJSON(&payload); err != nil {
		c.JSON(http.StatusBadRequest, response{
			Status: "error",
			Error:  fmt.Sprintf("invalid alertmanager payload: %v", err),
		})
		return
	}

	for _, alert := range payload.Alerts {
		if alert.Fingerprint == "" {
			continue
		}

		switch alert.Status {
		case "firing":
			value, err := json.Marshal(alert)
			if err != nil {
				c.JSON(http.StatusInternalServerError, response{
					Status: "error",
					Error:  fmt.Sprintf("marshal alert payload failed: %v", err),
				})
				return
			}

			if err := h.cacheClient.HSet(c.Request.Context(), rediscache.ActiveAlertsKey, alert.Fingerprint, value); err != nil {
				c.JSON(http.StatusBadGateway, response{
					Status: "error",
					Error:  fmt.Sprintf("store active alert failed: %v", err),
				})
				return
			}
		case "resolved":
			if err := h.cacheClient.HDel(c.Request.Context(), rediscache.ActiveAlertsKey, alert.Fingerprint); err != nil {
				c.JSON(http.StatusBadGateway, response{
					Status: "error",
					Error:  fmt.Sprintf("delete active alert failed: %v", err),
				})
				return
			}
		}
	}

	c.JSON(http.StatusAccepted, response{
		Status: "accepted",
	})
}

func (h *Handler) getCachedHosts(ctx context.Context) ([]promclient.Host, bool) {
	if h.cacheClient == nil || !h.cacheClient.Enabled() {
		return nil, false
	}

	value, ok := h.cacheClient.Get(ctx, rediscache.HostsListKey)
	if !ok {
		return nil, false
	}

	var hosts []promclient.Host
	if err := json.Unmarshal(value, &hosts); err != nil {
		return nil, false
	}

	return hosts, true
}

func (h *Handler) cacheHosts(ctx context.Context, hosts []promclient.Host) {
	if h.cacheClient == nil || !h.cacheClient.Enabled() {
		return
	}

	value, err := json.Marshal(hosts)
	if err != nil {
		return
	}

	_ = h.cacheClient.Set(ctx, rediscache.HostsListKey, value, h.hostsTTL)
}
