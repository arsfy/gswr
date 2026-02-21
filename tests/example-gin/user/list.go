package user

import (
	"example-gin/resp"
	"example-gin/types"
	"net/http"

	"github.com/gin-gonic/gin"
)

type Label struct {
	Display string `json:"display" binding:"required"`
}

type LabelMap map[string]Label

type Profile struct {
	Nickname string   `json:"nickname" binding:"required"`
	Labels   LabelMap `json:"labels"`
}

type CreateUserRequest struct {
	Name    string  `json:"name" binding:"required"`
	Profile Profile `json:"profile"`
}

type UserDetailResponse struct {
	ID      string   `json:"id"`
	Profile Profile  `json:"profile"`
	Extras  LabelMap `json:"extras"`
}

type SearchUsersInput struct {
	Page    int      `form:"page" binding:"required"`
	Active  bool     `form:"active"`
	Keyword string   `form:"keyword"`
	TraceID string   `header:"X-Trace-Id"`
	Tags    []string `form:"tag"`
}

type UpdateProfileInput struct {
	UserID  string   `uri:"id"`
	TraceID string   `header:"X-Trace-Id"`
	Profile Profile  `json:"profile" binding:"required"`
	Labels  LabelMap `json:"labels"`
}

// @summary Get user list
// @description Returns user list data with pagination parameters page and size.
// @Tags user
func list(c *gin.Context) {
	pageAny, _ := c.Get("page")
	sizeAny, _ := c.Get("size")
	page, _ := pageAny.(int)
	size, _ := sizeAny.(int)

	if page == 0 {
		c.JSON(http.StatusBadRequest, types.Response{
			Code: "page == 0",
		}) // Empty
		return
	}

	c.JSON(http.StatusOK, types.Response{
		Code: "ok",
		Data: map[string]any{
			"data": "a",
			"page": page,
			"size": size,
		},
	}) // 200OK
}

// @summary Get user detail
// @description Returns user detail by ID and includes trace information from request header.
// @Tags user
func detail(c *gin.Context) {
	id := c.Param("id")
	traceID := c.GetHeader("X-Trace-Id")

	c.JSON(200, UserDetailResponse{
		ID: id,
		Profile: Profile{
			Nickname: "tom",
			Labels: LabelMap{
				"main": {Display: traceID},
			},
		},
		Extras: LabelMap{
			"extra": {Display: "yes"},
		},
	})
}

func create(c *gin.Context) {
	var req CreateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, map[string]string{"code": "bad_request"})
		return
	}

	c.JSON(200, UserDetailResponse{
		ID: req.Name,
		Profile: Profile{
			Nickname: req.Profile.Nickname,
			Labels:   req.Profile.Labels,
		},
		Extras: LabelMap{
			"created": {Display: "true"},
		},
	})
}

func createForm(c *gin.Context) {
	name := c.PostForm("name")
	email := c.DefaultPostForm("email", "default@example.com")

	c.JSON(200, types.Response{
		Code: "A",
		Data: gin.H{
			"name": name,
			"email": []string{
				email,
			},
		},
	})
}

// @summary Edit user
// @description Edits user profile fields with helper-based parsing.
// @Tags user
func edit(c *gin.Context) {
	id, _ := resp.ParseIDParam(c, "id")
	age := resp.ParseIntForm(c, "age", 18)
	email := c.DefaultPostForm("email", "default@example.com")

	if id <= 0 {
		resp.BadRequest(c, "id <= 0")
		return
	}

	resp.Success(c, gin.H{
		"id":  id,
		"age": age,
		"email": []string{
			email,
		},
	})
}

// @summary Search users
// @description Demonstrates user search using query and header input parameters.
// @Tags user
func search(c *gin.Context) {
	var input SearchUsersInput
	if err := c.ShouldBindQuery(&input); err != nil {
		c.JSON(400, gin.H{"code": "bad_request"})
		return
	}

	c.JSON(200, gin.H{
		"page":    input.Page,
		"active":  input.Active,
		"keyword": input.Keyword,
	})
}

func updateProfile(c *gin.Context) {
	var input UpdateProfileInput
	if err := c.ShouldBindUri(&input); err != nil {
		c.JSON(400, gin.H{"code": "bad_request"})
		return
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(400, gin.H{"code": "bad_request"})
		return
	}

	c.JSON(200, UserDetailResponse{
		ID:      input.UserID,
		Profile: input.Profile,
		Extras:  input.Labels,
	})
}
