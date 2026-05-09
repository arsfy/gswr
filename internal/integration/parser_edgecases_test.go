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
