package internal

import (
	"example-echo-v4/internal/user"

	"github.com/labstack/echo/v4"
)

func Start() {
	e := echo.New()
	api := e.Group("/api")
	v1 := api.Group("/v1")
	user.Route(v1.Group("/user"))
}
