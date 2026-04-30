package middleware

import (
	"log/slog"
	"strconv"
	"sync/atomic"
	"time"

	"github.com/gin-gonic/gin"
)

const requestIDHeader = "X-Request-ID"

var requestIDCounter uint64

func Logging() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		requestID := c.GetHeader(requestIDHeader)
		if requestID == "" {
			requestID = newRequestID(start)
		}
		c.Header(requestIDHeader, requestID)

		c.Next()

		path := c.FullPath()
		if path == "" {
			path = c.Request.URL.Path
		}

		slog.Info("http request",
			"request_id", requestID,
			"method", c.Request.Method,
			"path", path,
			"status", c.Writer.Status(),
			"latency", time.Since(start).String(),
			"client_ip", c.ClientIP(),
		)
	}
}

func newRequestID(now time.Time) string {
	seq := atomic.AddUint64(&requestIDCounter, 1)
	return strconv.FormatInt(now.UnixNano(), 36) + "-" + strconv.FormatUint(seq, 36)
}
