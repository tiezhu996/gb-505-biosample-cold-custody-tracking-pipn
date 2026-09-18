package middleware

import (
	"fmt"
	"log/slog"
	"net/http"
	"regexp"
	"runtime/debug"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

var sensitiveLogValue = regexp.MustCompile(`(?i)(password|secret|token|authorization|access[_-]?key|database_url)(\s*[:=]\s*)([^\s,;]+)`)

func RequestContext() gin.HandlerFunc {
	return func(c *gin.Context) {
		requestID := strings.TrimSpace(c.GetHeader("X-Request-ID"))
		if requestID == "" || len(requestID) > 64 || strings.ContainsAny(requestID, "\r\n\t") {
			requestID = uuid.NewString()
		}
		c.Set("requestId", requestID)
		c.Header("X-Request-ID", requestID)
		started := time.Now()
		c.Next()
		slog.Info("request",
			"requestId", requestID,
			"method", c.Request.Method,
			"path", c.Request.URL.Path,
			"status", c.Writer.Status(),
			"durationMs", time.Since(started).Milliseconds(),
		)
	}
}

func ErrorHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if recovered := recover(); recovered != nil {
				slog.Error("panic recovered",
					"error", redactLogValue(fmt.Sprint(recovered)),
					"requestId", c.GetString("requestId"),
					"stack", string(debug.Stack()),
				)
				c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": gin.H{"code": "INTERNAL_ERROR", "message": "服务暂时不可用"}, "requestId": c.GetString("requestId")})
			}
		}()
		c.Next()
		if len(c.Errors) > 0 && !c.Writer.Written() {
			err := c.Errors.Last().Err
			slog.Error("request failed", "error", redactLogValue(err.Error()), "errorType", fmt.Sprintf("%T", err), "requestId", c.GetString("requestId"))
			c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"code": "INTERNAL_ERROR", "message": "服务器处理请求失败"}, "requestId": c.GetString("requestId")})
		}
	}
}

func redactLogValue(value string) string {
	value = sensitiveLogValue.ReplaceAllString(value, "$1$2[REDACTED]")
	if strings.Contains(strings.ToLower(value), "bearer ") {
		parts := strings.Split(value, " ")
		for index := range parts {
			if strings.EqualFold(parts[index], "Bearer") && index+1 < len(parts) {
				parts[index+1] = "[REDACTED]"
			}
		}
		value = strings.Join(parts, " ")
	}
	return value
}
