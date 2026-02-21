package user

import "github.com/labstack/echo/v4"

func Route(e *echo.Group) {
	e.GET("/:id", get)
}
