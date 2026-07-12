package integration

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/arsfy/gswr/internal/model"
	"github.com/arsfy/gswr/internal/parser"
)

func TestParseSelectorHandlerAndIgnoreInnerClosureResponses(t *testing.T) {
	root := t.TempDir()
	mustWriteFile(t, filepath.Join(root, "go.mod"), `module example.com/edge

go 1.24
`)
	mustWriteFile(t, filepath.Join(root, "main.go"), `package main

import (
	"example.com/edge/api"
	e4 "github.com/labstack/echo/v4"
)

func main() {
	e := e4.New()
	g := e.Group("/api")
	api.Register(g)
}
`)
	mustWriteFile(t, filepath.Join(root, "api", "router.go"), `package api

import (
	h "example.com/edge/handlers"
	e4 "github.com/labstack/echo/v4"
)

func Register(g *e4.Group) {
	g.GET("/selector", h.SelectorHandler)
	g.GET("/closure", closureHandler)
}

func closureHandler(c e4.Context) error {
	inner := func() error {
		return c.JSON(418, map[string]string{"code": "teapot"})
	}
	_ = inner
	return c.JSON(200, map[string]string{"code": "ok"})
}
`)
	mustWriteFile(t, filepath.Join(root, "handlers", "handler.go"), `package handlers

import "github.com/labstack/echo/v4"

// @summary selector summary
func SelectorHandler(c echo.Context) error {
	return c.JSON(201, map[string]string{"code": "created"})
}
`)

	ir, err := parser.ParseEchoProject(filepath.Join(root, "main.go"))
	if err != nil {
		t.Fatalf("parse project: %v", err)
	}

	var selectorFound bool
	var closureFound bool
	for _, r := range ir.Routes {
		if r.Method == "GET" && r.Path == "/api/selector" {
			selectorFound = true
			if len(r.Responses) == 0 || r.Responses[0].StatusCode != 201 {
				t.Fatalf("selector route should keep handler response 201, got %#v", r.Responses)
			}
		}
		if r.Method == "GET" && r.Path == "/api/closure" {
			closureFound = true
			has200 := false
			has418 := false
			for _, resp := range r.Responses {
				if resp.StatusCode == 200 {
					has200 = true
				}
				if resp.StatusCode == 418 {
					has418 = true
				}
			}
			if !has200 {
				t.Fatalf("closure route should include 200 response, got %#v", r.Responses)
			}
			if has418 {
				t.Fatalf("closure route should ignore nested closure response 418, got %#v", r.Responses)
			}
		}
	}

	if !selectorFound {
		t.Fatalf("missing GET /api/selector")
	}
	if !closureFound {
		t.Fatalf("missing GET /api/closure")
	}
}

func TestParseParameterizedRouterFactoryNestedInHTTPServerHandler(t *testing.T) {
	root := t.TempDir()
	mustWriteFile(t, filepath.Join(root, "go.mod"), `module example.com/nestedentry

go 1.24
`)
	mustWriteFile(t, filepath.Join(root, "cmd", "control-api", "main.go"), `package main

import (
	"net/http"

	"example.com/nestedentry/internal/httpapi"
)

func main() {
	server := &http.Server{
		Addr:    ":8080",
		Handler: httpapi.New("dependency"),
	}
	_ = server
}
`)
	mustWriteFile(t, filepath.Join(root, "internal", "httpapi", "server.go"), `package httpapi

import (
	"example.com/nestedentry/internal/httpapi/health"
	"github.com/labstack/echo/v5"
)

func New(dependency string) *echo.Echo {
	e := echo.New()
	health.Register(e, dependency)
	return e
}
`)
	mustWriteFile(t, filepath.Join(root, "internal", "httpapi", "health", "routes.go"), `package health

import "github.com/labstack/echo/v5"

func Register(e *echo.Echo, dependency string) {
	e.GET("/health/live", live)
}

func live(c echo.Context) error {
	return c.JSON(200, map[string]string{"status": "ok"})
}
`)

	ir, err := parser.ParseEchoProject(filepath.Join(root, "cmd", "control-api", "main.go"))
	if err != nil {
		t.Fatalf("parse project: %v", err)
	}

	if len(ir.Routes) != 1 || ir.Routes[0].Method != "GET" || ir.Routes[0].Path != "/health/live" {
		t.Fatalf("expected only GET /health/live route, got %#v", ir.Routes)
	}
}

func TestAssignmentRHSDoesNotExpandRecursiveRouterCalls(t *testing.T) {
	root := t.TempDir()
	mustWriteFile(t, filepath.Join(root, "go.mod"), "module example.com/recursive\n\ngo 1.24\n")
	mustWriteFile(t, filepath.Join(root, "main.go"), `package main

import "github.com/labstack/echo/v5"

func main() {
	e := echo.New()
	register(e.Group("/root"), 2)
}

func register(g *echo.Group, depth int) *echo.Group {
	if depth == 0 {
		g.GET("/leaf", handler)
		return g
	}
	child := register(g.Group("/nested"), depth-1)
	return child
}

func handler(c echo.Context) error { return c.NoContent(204) }
`)

	ir, err := parser.ParseEchoProject(filepath.Join(root, "main.go"))
	if err != nil {
		t.Fatalf("parse project: %v", err)
	}
	if len(ir.Routes) != 1 || ir.Routes[0].Path != "/root/leaf" {
		t.Fatalf("expected one finite route, got %#v", ir.Routes)
	}
}

func TestUnusedClosureRoutesAreIgnored(t *testing.T) {
	root := t.TempDir()
	mustWriteFile(t, filepath.Join(root, "go.mod"), "module example.com/closure\n\ngo 1.24\n")
	mustWriteFile(t, filepath.Join(root, "main.go"), `package main

import "github.com/labstack/echo/v5"

func main() {
	e := echo.New()
	e.GET("/ok", handler)
	unused := func() {
		e.GET("/never-registered", handler)
	}
	_ = unused
}

func handler(c echo.Context) error { return c.NoContent(204) }
`)

	ir, err := parser.ParseEchoProject(filepath.Join(root, "main.go"))
	if err != nil {
		t.Fatalf("parse project: %v", err)
	}
	if len(ir.Routes) != 1 || ir.Routes[0].Path != "/ok" {
		t.Fatalf("unused closure must not register routes, got %#v", ir.Routes)
	}
}

func TestParameterizedOrdinaryHelperIsNotBootstrapped(t *testing.T) {
	root := t.TempDir()
	mustWriteFile(t, filepath.Join(root, "go.mod"), "module example.com/helper\n\ngo 1.24\n")
	mustWriteFile(t, filepath.Join(root, "main.go"), `package main

import "github.com/labstack/echo/v5"

func helper(_ string) {
	e := echo.New()
	e.GET("/not-an-api", handler)
}

func main() {
	e := echo.New()
	e.GET("/ok", handler)
	helper("ordinary-call")
}

func handler(c echo.Context) error { return c.NoContent(204) }
`)

	ir, err := parser.ParseEchoProject(filepath.Join(root, "main.go"))
	if err != nil {
		t.Fatalf("parse project: %v", err)
	}
	if len(ir.Routes) != 1 || ir.Routes[0].Path != "/ok" {
		t.Fatalf("ordinary helper must not be bootstrapped, got %#v", ir.Routes)
	}
}

func TestNestedHTTPHandlerFactoryArgumentIsNotParsedTwice(t *testing.T) {
	root := t.TempDir()
	mustWriteFile(t, filepath.Join(root, "go.mod"), "module example.com/dedup\n\ngo 1.24\n")
	mustWriteFile(t, filepath.Join(root, "main.go"), `package main

import (
	"net/http"
	"github.com/labstack/echo/v5"
)

func inner() *echo.Echo {
	e := echo.New()
	e.GET("/inner", handler)
	return e
}

func outer(_ *echo.Echo) *echo.Echo {
	inner()
	e := echo.New()
	e.GET("/outer", handler)
	return e
}

func main() {
	server := &http.Server{Handler: outer(inner())}
	_ = server
}

func handler(c echo.Context) error { return c.NoContent(204) }
`)

	ir, err := parser.ParseEchoProject(filepath.Join(root, "main.go"))
	if err != nil {
		t.Fatalf("parse project: %v", err)
	}
	counts := map[string]int{}
	for _, route := range ir.Routes {
		counts[route.Path]++
	}
	if len(ir.Routes) != 2 || counts["/inner"] != 1 || counts["/outer"] != 1 {
		t.Fatalf("nested factory calls should be parsed once, got %#v", ir.Routes)
	}
}

func TestParseGinLocalChannelTypeInference(t *testing.T) {
	root := t.TempDir()
	mustWriteFile(t, filepath.Join(root, "go.mod"), `module example.com/ginedge

go 1.24
`)
	mustWriteFile(t, filepath.Join(root, "main.go"), `package main

import "github.com/gin-gonic/gin"

func main() {
	r := gin.New()
	g := r.Group("/api")
	g.GET("/book/:id", book)
}

func book(c *gin.Context) {
	type translateResult struct {
		title string
	}
	ch := make(chan translateResult, 1)
	go func() {
		ch <- translateResult{title: "ok"}
	}()
	res := <-ch
	recommend := []int64{1, 2}
	uid := c.GetInt64("uid")
	c.JSON(200, gin.H{
		"translate": res.title,
		"recommend": recommend,
		"uid":       uid,
	})
}
`)

	ir, err := parser.ParseEchoProject(filepath.Join(root, "main.go"))
	if err != nil {
		t.Fatalf("parse project: %v", err)
	}

	var found bool
	for _, r := range ir.Routes {
		if r.Method != "GET" || r.Path != "/api/book/{id}" || len(r.Responses) == 0 {
			continue
		}
		found = true
		s := r.Responses[0].Schema
		if s == nil || s.Properties["translate"] == nil || s.Properties["recommend"] == nil || s.Properties["uid"] == nil {
			t.Fatalf("missing response properties: %#v", s)
		}
		if s.Properties["translate"].Type != "string" {
			t.Fatalf("translate type mismatch: %#v", s.Properties["translate"])
		}
		if s.Properties["recommend"].Type != "array" {
			t.Fatalf("recommend type mismatch: %#v", s.Properties["recommend"])
		}
		if s.Properties["uid"].Type != "number" {
			t.Fatalf("uid type mismatch: %#v", s.Properties["uid"])
		}
	}
	if !found {
		t.Fatalf("missing GET /api/book/{id} route")
	}
}

func TestParseEmbeddedStructFields(t *testing.T) {
	root := t.TempDir()
	mustWriteFile(t, filepath.Join(root, "go.mod"), `module example.com/embededge

go 1.24
`)
	mustWriteFile(t, filepath.Join(root, "main.go"), `package main

import (
	_type "example.com/embededge/types"
	"github.com/gin-gonic/gin"
)

type ProductGroupResponse struct {
	_type.ProductGroup
	Products []_type.Product `+"`json:\"products\"`"+`
}

type CreateProductGroupRequest struct {
	_type.ProductGroup
	Products []_type.Product `+"`json:\"products\" binding:\"required\"`"+`
}

func main() {
	r := gin.New()
	g := r.Group("/api")
	g.GET("/products", listProducts)
	g.POST("/products", createProducts)
}

func listProducts(c *gin.Context) {
	c.JSON(200, ProductGroupResponse{})
}

func createProducts(c *gin.Context) {
	var req CreateProductGroupRequest
	_ = c.ShouldBindJSON(&req)
	c.JSON(200, req)
}
`)
	mustWriteFile(t, filepath.Join(root, "types", "product.go"), `package _type

type ProductGroup struct {
	GroupID string `+"`json:\"group_id\" binding:\"required\"`"+`
	Name    string `+"`json:\"name\"`"+`
}

type Product struct {
	SKU string `+"`json:\"sku\"`"+`
}
`)

	ir, err := parser.ParseEchoProject(filepath.Join(root, "main.go"))
	if err != nil {
		t.Fatalf("parse project: %v", err)
	}

	var getSchema *model.Schema
	var postSchema *model.Schema
	for i := range ir.Routes {
		r := ir.Routes[i]
		switch {
		case r.Method == "GET" && r.Path == "/api/products":
			if len(r.Responses) > 0 {
				getSchema = r.Responses[0].Schema
			}
		case r.Method == "POST" && r.Path == "/api/products":
			postSchema = r.RequestBody
		}
	}
	if getSchema == nil {
		t.Fatalf("missing GET /api/products route")
	}
	if postSchema == nil {
		t.Fatalf("missing POST /api/products route")
	}

	assertEmbeddedProductGroupSchema(t, "response", getSchema)
	assertEmbeddedProductGroupSchema(t, "request body", postSchema)
}

func TestParseEchoV5GenericQueryParamOrInHelper(t *testing.T) {
	root := t.TempDir()
	mustWriteFile(t, filepath.Join(root, "go.mod"), `module example.com/echov5edge

go 1.24
`)
	mustWriteFile(t, filepath.Join(root, "main.go"), `package main

import (
	"errors"
	"time"

	"github.com/labstack/echo/v5"
)

func main() {
	e := echo.New()
	e.GET("/billing", billing)
}

func billing(c echo.Context) error {
	year, month, start, end, err := parseBillingMonth(&c)
	if err != nil {
		return c.JSON(400, map[string]string{"error": err.Error()})
	}
	typ := c.QueryParams()["type"]
	return c.JSON(200, map[string]any{"year": year, "month": month, "start": start, "end": end, "now": time.Now(), "type": typ})
}

func parseBillingMonth(c *echo.Context) (int, int, time.Time, time.Time, error) {
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
`)

	ir, err := parser.ParseEchoProject(filepath.Join(root, "main.go"))
	if err != nil {
		t.Fatalf("parse project: %v", err)
	}

	var route *model.Route
	for i := range ir.Routes {
		if ir.Routes[i].Method == "GET" && ir.Routes[i].Path == "/billing" {
			route = &ir.Routes[i]
			break
		}
	}
	if route == nil {
		t.Fatalf("missing GET /billing route, got routes: %#v", ir.Routes)
	}

	paramTypes := map[string]string{}
	var typeParam *model.Parameter
	for _, p := range route.Parameters {
		if p.In == "query" && p.Schema != nil {
			paramTypes[p.Name] = p.Schema.Type
		}
		if p.In == "query" && p.Name == "type" {
			typeParam = &p
		}
	}
	for _, name := range []string{"year", "month"} {
		if paramTypes[name] != "number" {
			t.Fatalf("expected %s query parameter type number, got %q in params %#v", name, paramTypes[name], route.Parameters)
		}
	}
	if typeParam == nil || typeParam.Schema == nil || typeParam.Schema.Type != "array" || typeParam.Schema.Items == nil || typeParam.Schema.Items.Type != "string" {
		t.Fatalf("expected type query parameter schema []string, got %#v in params %#v", typeParam, route.Parameters)
	}
	if len(route.Responses) == 0 || route.Responses[0].Schema == nil || route.Responses[0].Schema.Properties["type"] == nil {
		t.Fatalf("missing type response schema: %#v", route.Responses)
	}
	typeSchema := route.Responses[0].Schema.Properties["type"]
	if typeSchema.Type != "array" || typeSchema.Items == nil || typeSchema.Items.Type != "string" {
		t.Fatalf("expected type response schema []string, got %#v", typeSchema)
	}
	for _, name := range []string{"start", "end", "now"} {
		schema := route.Responses[0].Schema.Properties[name]
		if schema == nil || schema.Type != "string" || schema.Format != "date-time" || schema.Example != "2026-01-02T15:04:05Z" {
			t.Fatalf("expected %s response schema date-time string for time.Time, got %#v", name, schema)
		}
	}
}

func TestParseMultiReturnAssignedResponseType(t *testing.T) {
	root := t.TempDir()
	mustWriteFile(t, filepath.Join(root, "go.mod"), `module example.com/multireturnedge

go 1.24
`)
	mustWriteFile(t, filepath.Join(root, "main.go"), `package main

import "github.com/labstack/echo/v5"

type FindResponse struct {
	ID   int64  `+"`json:\"id\"`"+`
	Name string `+"`json:\"name\"`"+`
}

func main() {
	e := echo.New()
	e.GET("/find", findHandler)
}

func lookup(t int) (int, []FindResponse, FindResponse, error) {
	return 1, nil, FindResponse{ID: int64(t), Name: "ok"}, nil
}

func findHandler(c echo.Context) error {
	_, _, find, _ := lookup(1)
	return c.JSON(200, map[string]any{
		"find": map[string]interface{}{
			"find": find,
		},
	})
}
`)

	ir, err := parser.ParseEchoProject(filepath.Join(root, "main.go"))
	if err != nil {
		t.Fatalf("parse project: %v", err)
	}

	var route *model.Route
	for i := range ir.Routes {
		if ir.Routes[i].Method == "GET" && ir.Routes[i].Path == "/find" {
			route = &ir.Routes[i]
			break
		}
	}
	if route == nil || len(route.Responses) == 0 || route.Responses[0].Schema == nil {
		t.Fatalf("missing GET /find response schema, got route %#v", route)
	}
	outer := route.Responses[0].Schema.Properties["find"]
	if outer == nil || outer.Properties["find"] == nil {
		t.Fatalf("missing nested find schema: %#v", route.Responses[0].Schema)
	}
	inner := outer.Properties["find"]
	if inner.Ref == "" {
		t.Fatalf("expected nested find schema to resolve to FindResponse ref, got %#v", inner)
	}
}

func assertEmbeddedProductGroupSchema(t *testing.T, label string, s *model.Schema) {
	t.Helper()
	if s == nil {
		t.Fatalf("%s schema missing", label)
	}
	for _, name := range []string{"group_id", "name", "products"} {
		if s.Properties[name] == nil {
			t.Fatalf("%s schema missing property %q: %#v", label, name, s.Properties)
		}
	}
	if s.Properties["productGroup"] != nil {
		t.Fatalf("%s schema should not keep embedded field as productGroup: %#v", label, s.Properties)
	}
	if s.Properties["products"].Type != "array" {
		t.Fatalf("%s products should be array, got %#v", label, s.Properties["products"])
	}
	if !containsString(s.Required, "group_id") {
		t.Fatalf("%s should inherit required group_id, got %#v", label, s.Required)
	}
}

func containsString(items []string, target string) bool {
	for _, item := range items {
		if item == target {
			return true
		}
	}
	return false
}

func mustWriteFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}
