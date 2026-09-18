package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"biosample-cold-custody-tracking/backend/internal/util"
)

const (
	ContextUserID      = "userId"
	ContextUsername    = "username"
	ContextDisplayName = "displayName"
	ContextRole        = "role"
)

func Auth(secret string) gin.HandlerFunc {
	return func(c *gin.Context) {
		header := strings.TrimSpace(c.GetHeader("Authorization"))
		parts := strings.SplitN(header, " ", 2)
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") || strings.TrimSpace(parts[1]) == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": gin.H{"code": "UNAUTHORIZED", "message": "请先登录"}, "requestId": c.GetString("requestId")})
			return
		}
		claims, err := util.ParseToken(secret, strings.TrimSpace(parts[1]))
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": gin.H{"code": "INVALID_TOKEN", "message": "登录状态已失效"}, "requestId": c.GetString("requestId")})
			return
		}
		c.Set(ContextUserID, claims.UserID)
		c.Set(ContextUsername, claims.Username)
		c.Set(ContextDisplayName, claims.DisplayName)
		c.Set(ContextRole, claims.Role)
		c.Next()
	}
}
