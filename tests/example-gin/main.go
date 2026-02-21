package main

import (
	"example-gin/user"
	"fmt"
	"log/slog"
	"os"

	"github.com/gin-gonic/gin"
)

// @title Example Gin API
// @description Example Project
// @version 0.0.1
// @BasePath /api/
// @schemes http
func main() {
	r := gin.New()

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(logger)

	api := r.Group("/api")
	{
		v1 := api.Group("/v1")

		v1.GET("/status", status)
		user.Router(v1.Group("/user"))
	}

	if err := r.Run(fmt.Sprintf("%s:%d", "127.0.0.1", 9876)); err != nil {
		logger.Error("Failed to start server", "error", err)
		os.Exit(1)
	}
}

// @summary Get service status
// @description Returns health and version information of the service.
// @Tags system
func status(c *gin.Context) {
	c.JSON(200, map[string]any{
		"code":    "ok",
		"healthy": true,
		"version": 1,
	})
}
