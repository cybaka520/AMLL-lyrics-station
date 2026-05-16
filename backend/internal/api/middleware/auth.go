package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

func Auth(apiKey string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if apiKey == "" || c.GetHeader("X-API-Key") != apiKey {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthorized", "code": 401})
			return
		}
		c.Next()
	}
}
