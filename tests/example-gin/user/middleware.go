package user

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
)

// AuthMiddleware validates Bearer token for user routes.
func AuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		auth := c.GetHeader("Authorization")
		if !strings.HasPrefix(auth, "Bearer ") {
			c.JSON(http.StatusUnauthorized, map[string]string{"code": "unauthorized"}) // Missing bearer token
			c.Abort()
			return
		}
		c.Next()
	}
}

func ListMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
		size, _ := strconv.Atoi(c.DefaultQuery("size", "20")) // Per page size

		c.Set("page", page)
		c.Set("size", size)

		c.Next()
	}
}
