package integration

import (
	"os"
	"path/filepath"
	"testing"

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

func mustWriteFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}
