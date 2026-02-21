package resp

import (
	"example-gin/types"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

func Success(c *gin.Context, data any) {
	c.JSON(http.StatusOK, types.Response{
		Code: "ok",
		Data: data,
	})
}

func BadRequest(c *gin.Context, msg string) {
	c.JSON(http.StatusBadRequest, types.Response{
		Code: msg,
	})
}

func ParseIDParam(c *gin.Context, paramName string) (int64, error) {
	id, err := strconv.ParseInt(c.Param(paramName), 10, 64)
	if err != nil || id <= 0 {
		return 0, err
	}
	return id, nil
}

func ParseIntQuery(c *gin.Context, name string, defaultVal int) int {
	if v := c.Query(name); v != "" {
		if parsed, err := strconv.Atoi(v); err == nil && parsed >= 0 {
			return parsed
		}
	}
	return defaultVal
}

func ParseIntForm(c *gin.Context, name string, defaultVal int) int {
	if v := c.PostForm(name); v != "" {
		if parsed, err := strconv.Atoi(v); err == nil && parsed >= 0 {
			return parsed
		}
	}
	return defaultVal
}

func BindJSON(c *gin.Context, req any) error {
	if err := c.ShouldBindJSON(req); err != nil {
		BadRequest(c, "invalid request body")
		return err
	}
	return nil
}
