package user

import (
	"net/http"

	"github.com/labstack/echo/v4"
)

func get(c echo.Context) error {
	id := c.Param("id")
	return c.JSON(http.StatusOK, map[string]any{
		"id": id,
	})
}
