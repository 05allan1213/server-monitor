package main

import (
	"context"
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"server-web/config"
	promclient "server-web/prometheus"
)

type app struct {
	promClient *promclient.Client
}

type response struct {
	Status string      `json:"status"`
	Data   interface{} `json:"data,omitempty"`
	Error  string      `json:"error,omitempty"`
}

func main() {
	cfg := config.Load()
	prometheusClient := promclient.NewClient(cfg.PrometheusURL, cfg.RequestTimeout)

	application := &app{
		promClient: prometheusClient,
	}

	router := gin.Default()
	router.GET("/", application.handleRoot)
	router.GET("/healthz", application.handleHealthz)
	router.GET("/readyz", application.handleReadyz)
	router.GET("/api/v1/hosts", application.handleHosts)

	log.Printf("server-web listening on %s", cfg.ListenAddr)
	if err := router.Run(cfg.ListenAddr); err != nil {
		log.Fatalf("server-web exited: %v", err)
	}
}

func (a *app) handleRoot(c *gin.Context) {
	c.JSON(http.StatusOK, response{
		Status: "success",
		Data: gin.H{
			"message": "server-web is running",
		},
	})
}

func (a *app) handleHealthz(c *gin.Context) {
	c.JSON(http.StatusOK, response{
		Status: "success",
		Data: gin.H{
			"healthy": true,
		},
	})
}

func (a *app) handleReadyz(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 3*time.Second)
	defer cancel()

	if err := a.promClient.Ready(ctx); err != nil {
		c.JSON(http.StatusServiceUnavailable, response{
			Status: "error",
			Error:  err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, response{
		Status: "success",
		Data: gin.H{
			"ready": true,
		},
	})
}

func (a *app) handleHosts(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer cancel()

	hosts, err := a.promClient.GetHosts(ctx)
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
