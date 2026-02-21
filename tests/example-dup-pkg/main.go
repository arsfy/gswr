package main

import (
	adminOrder "example-dup-pkg/admin/order"
	generalOrder "example-dup-pkg/general/order"

	"github.com/labstack/echo/v4"
)

func main() {
	e := echo.New()
	api := e.Group("/api")
	generalOrder.Router(api.Group("/general/order"))
	adminOrder.Router(api.Group("/admin/order"))
}
