package user

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/labstack/echo/v5"
)

// AuthMiddleware validates Bearer token for user routes.
func AuthMiddleware(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c *echo.Context) error {
		auth := c.Request().Header.Get("Authorization")
		if !strings.HasPrefix(auth, "Bearer ") {
			return c.JSON(http.StatusUnauthorized, map[string]string{"code": "unauthorized"}) // Missing bearer token
		}
		return next(c)
	}
}

func ListMiddleware(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c *echo.Context) error {
		page, _ := strconv.Atoi(c.QueryParamOr("page", "1"))
		size, _ := strconv.Atoi(c.QueryParamOr("size", "20")) // Per page size

		c.Set("page", page)
		c.Set("size", size)

		return next(c)
	}
}
