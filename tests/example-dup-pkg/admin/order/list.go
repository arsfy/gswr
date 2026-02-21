package order

import (
	"net/http"

	"github.com/labstack/echo/v4"
)

func list(c echo.Context) error {
	return c.JSON(http.StatusOK, map[string]string{"scope": "admin"})
}
