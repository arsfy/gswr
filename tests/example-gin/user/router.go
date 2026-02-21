package user

import "github.com/gin-gonic/gin"

func Router(e *gin.RouterGroup) {
	e.Use(AuthMiddleware())
	e.GET("/list", ListMiddleware(), list)
	e.GET("/:id", detail)
	e.POST("/:id", edit)
	e.POST("/create", create)
	e.POST("/create/form", createForm)
	e.GET("/search", search)
	e.PUT("/:id/profile", updateProfile)
}
