package main

import (
	"example-echo/user"
	"fmt"
	"log/slog"
	"os"

	"github.com/labstack/echo/v5"
)

// @title Example Echo v5
// @description Example Project
// @version 0.0.1
// @BasePath /api/
// @schemes http
func main() {
	e := echo.New()

	// Logger
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(logger)

	// API
	api := e.Group("/api")
	{
		v1 := api.Group("/v1")

		v1.GET("/status", status)

		user.Router(v1.Group("/user"))
	}

	// Start
	if err := e.Start(fmt.Sprintf("%s:%d", "127.0.0.1", 9875)); err != nil {
		logger.Error("Failed to start server", "error", err)
		os.Exit(1)
	}
}

// @summary Get service status
// @description Returns health and version information of the service.
// @Tags system
func status(c *echo.Context) error {
	return c.JSON(200, map[string]any{
		"code":    "ok",
		"healthy": true,
		"version": 1,
	})
}
