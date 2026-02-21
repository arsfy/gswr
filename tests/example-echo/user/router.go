package user

import "github.com/labstack/echo/v5"

func Router(e *echo.Group) {
	e.Use(AuthMiddleware)
	e.GET("/list", list, ListMiddleware)
	e.GET("/:id", detail)
	e.POST("/:id", edit)
	e.POST("/create", create)
	e.POST("create/post", createForm)
	e.GET("/search", search)
	e.PUT("/:id/profile", updateProfile)
}
