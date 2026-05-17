package user

import (
	"errors"
	"example-echo/resp"
	"example-echo/types"
	"net/http"
	"time"

	"github.com/labstack/echo/v5"
)

type Label struct {
	Display string `json:"display" validate:"required"`
}

type LabelMap map[string]Label

type Profile struct {
	Nickname string   `json:"nickname" validate:"required"`
	Labels   LabelMap `json:"labels"`
}

type CreateUserRequest struct {
	Name    string  `json:"name" validate:"required"`
	Profile Profile `json:"profile"`
}

type UserDetailResponse struct {
	ID      string   `json:"id"`
	Profile Profile  `json:"profile"`
	Extras  LabelMap `json:"extras"`
}

type SearchUsersInput struct {
	Page    int      `query:"page" validate:"required"`
	Active  bool     `query:"active"`
	Keyword string   `query:"keyword"`
	TraceID string   `header:"X-Trace-Id"`
	Tags    []string `query:"tag"`
}

type UpdateProfileInput struct {
	UserID  string   `param:"id"`
	TraceID string   `header:"X-Trace-Id"`
	Profile Profile  `json:"profile" validate:"required"`
	Labels  LabelMap `json:"labels"`
}

// @summary Get user list
// @description Returns user list data with pagination parameters page and size.
// @Tags user
func list(c *echo.Context) error {
	page := c.Get("page").(int)
	size := c.Get("size").(int)

	if page == 0 {
		return c.JSON(http.StatusBadRequest, types.Response{
			Code: "page == 0",
		}) // Empty
	}

	return c.JSON(http.StatusOK, types.Response{
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
func detail(c *echo.Context) error {
	id := c.Param("id")
	traceID := c.Request().Header.Get("X-Trace-Id")

	return c.JSON(200, UserDetailResponse{
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

func create(c *echo.Context) error {
	var req CreateUserRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(400, map[string]string{"code": "bad_request"})
	}

	return c.JSON(200, UserDetailResponse{
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

func parseMonth(c *echo.Context) (int, int, time.Time, time.Time, error) {
	now := time.Now().UTC()
	year, err := echo.QueryParamOr[int](c, "year", now.Year())
	if err != nil {
		return 0, 0, time.Time{}, time.Time{}, errors.New("year is invalid")
	}
	month, err := echo.QueryParamOr[int](c, "month", int(now.Month()))
	if err != nil {
		return 0, 0, time.Time{}, time.Time{}, errors.New("month is invalid")
	}
	if year < 2000 || year > 2100 {
		return 0, 0, time.Time{}, time.Time{}, errors.New("year is invalid")
	}
	if month < 1 || month > 12 {
		return 0, 0, time.Time{}, time.Time{}, errors.New("month is invalid")
	}

	start := time.Date(year, time.Month(month), 1, 0, 0, 0, 0, time.UTC)
	end := start.AddDate(0, 1, 0)
	return year, month, start, end, nil
}

func createForm(c *echo.Context) error {
	name := c.FormValue("name")
	email := c.FormValueOr("email", "default@example.com")

	year, month, _, _, err := parseMonth(c)
	if err != nil {
		return c.JSON(400, map[string]string{"code": "invalid_month"})
	}

	return c.JSON(200, types.Response{
		Code: "A",
		Data: map[string]any{
			"name": name,
			"email": []string{
				email,
			},
			"year":  year,
			"month": month,
		},
	})
}

// @summary Edit user
// @description Edits user profile fields with helper-based parsing.
// @Tags user
func edit(c *echo.Context) error {
	id, _ := resp.ParseIDParam(c, "id")
	age := resp.ParseIntForm(c, "age", 18)
	email := c.FormValueOr("email", "default@example.com") // Description 🎉

	if id <= 0 {
		return resp.BadRequest(c, "id <= 0")
	}

	return resp.Success(c, map[string]any{
		"id":  id,
		"age": age,
		"email": []string{
			email,
		},
	}) // Response Description 🎉
}

// @summary Search users
// @description Demonstrates user search using query and header input parameters.
// @Tags user
func search(c *echo.Context) error {
	var input SearchUsersInput
	if err := c.Bind(&input); err != nil {
		return c.JSON(400, map[string]string{"code": "bad_request"})
	}

	return c.JSON(200, map[string]any{
		"page":    input.Page,
		"active":  input.Active,
		"keyword": input.Keyword,
	})
}

func updateProfile(c *echo.Context) error {
	var input UpdateProfileInput
	if err := c.Bind(&input); err != nil {
		return c.JSON(400, map[string]string{"code": "bad_request"})
	}

	return c.JSON(200, UserDetailResponse{
		ID:      input.UserID,
		Profile: input.Profile,
		Extras:  input.Labels,
	})
}
