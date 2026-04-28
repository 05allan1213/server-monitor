package handlers

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	promclient "server-web/prometheus"
)

type Handler struct {
	promClient   *promclient.Client
	readyTimeout time.Duration
}

type response struct {
	Status string      `json:"status"`
	Data   interface{} `json:"data,omitempty"`
	Error  string      `json:"error,omitempty"`
}

func NewHandler(promClient *promclient.Client, readyTimeout time.Duration) *Handler {
	return &Handler{
		promClient:   promClient,
		readyTimeout: readyTimeout,
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

	if err := h.promClient.Ready(ctx); err != nil {
		c.JSON(http.StatusServiceUnavailable, response{
			Status: "error",
			Error:  err.Error(),
			Data: gin.H{
				"ready":      false,
				"prometheus": "unreachable",
			},
		})
		return
	}

	c.JSON(http.StatusOK, response{
		Status: "success",
		Data: gin.H{
			"ready":      true,
			"prometheus": "ok",
		},
	})
}

func (h *Handler) Hosts(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer cancel()

	hosts, err := h.promClient.GetHosts(ctx)
	if err != nil {
		c.JSON(http.StatusBadGateway, response{
			Status: "error",
			Error:  err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, response{
		Status: "success",
		Data:   hosts,
	})
}
