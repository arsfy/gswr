package resp

import (
	"example-echo/types"
	"net/http"
	"strconv"

	"github.com/labstack/echo/v5"
)

func Success(c *echo.Context, data any) error {
	return c.JSON(http.StatusOK, types.Response{
		Code: "ok",
		Data: data,
	})
}

func BadRequest(c *echo.Context, msg string) error {
	return c.JSON(http.StatusBadRequest, types.Response{
		Code: msg,
	})
}

func ParseIDParam(c *echo.Context, paramName string) (int64, error) {
	id, err := strconv.ParseInt(c.Param(paramName), 10, 64)
	if err != nil || id <= 0 {
		return 0, err
	}
	return id, nil
}

func RequireIDParam(c *echo.Context, paramName string) (int64, error) {
	id, err := ParseIDParam(c, paramName)
	if err != nil || id <= 0 {
		return 0, BadRequest(c, "invalid "+paramName)
	}
	return id, nil
}

func ParseIntQuery(c *echo.Context, name string, defaultVal int) int {
	if v := c.QueryParam(name); v != "" {
		if parsed, err := strconv.Atoi(v); err == nil && parsed >= 0 {
			return parsed
		}
	}
	return defaultVal
}

func ParseIntForm(c *echo.Context, name string, defaultVal int) int {
	if v := c.FormValue(name); v != "" {
		if parsed, err := strconv.Atoi(v); err == nil && parsed >= 0 {
			return parsed
		}
	}
	return defaultVal
}

func BindJSON(c *echo.Context, req any) error {
	if err := c.Bind(req); err != nil {
		return BadRequest(c, "invalid request body")
	}
	return nil
}
