package integration

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/arsfy/gswr/internal/model"
	"github.com/arsfy/gswr/internal/parser"
	"github.com/arsfy/gswr/internal/renderer"
)

func TestOpenAPIGenerationGolden(t *testing.T) {
	entry := filepath.Join("..", "..", "tests", "example-echo", "main.go")
	ir, err := parser.ParseEchoProject(entry)
	if err != nil {
		t.Fatalf("parse project: %v", err)
	}

	out := filepath.Join(t.TempDir(), "openapi.yaml")
	if err := renderer.WriteYAML(ir, out); err != nil {
		t.Fatalf("render yaml: %v", err)
	}

	got, err := os.ReadFile(out)
	if err != nil {
		t.Fatalf("read generated yaml: %v", err)
	}
	wantPath := filepath.Join("testdata", "openapi.golden.yaml")
	want, err := os.ReadFile(wantPath)
	if err != nil {
		t.Fatalf("read golden yaml: %v", err)
	}

	if string(got) != string(want) {
		t.Fatalf("golden mismatch: %s\nregenerate with: GOCACHE=/tmp/go-build-cache go run ./cmd/gswr --entry tests/example-echo/main.go --out docs/openapi.yaml && cp docs/openapi.yaml internal/integration/testdata/openapi.golden.yaml", wantPath)
	}
}

func TestParseEchoV4NestedRouteBootstrap(t *testing.T) {
	entry := filepath.Join("..", "..", "tests", "example-echo-v4", "cmd", "main.go")
	ir, err := parser.ParseEchoProject(entry)
	if err != nil {
		t.Fatalf("parse project: %v", err)
	}

	var found bool
	for _, r := range ir.Routes {
		if r.Method == "GET" && r.Path == "/api/v1/user/{id}" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected route GET /api/v1/user/{id}, got routes: %#v", ir.Routes)
	}
}

func TestParseDuplicatePackageNames(t *testing.T) {
	entry := filepath.Join("..", "..", "tests", "example-dup-pkg", "main.go")
	ir, err := parser.ParseEchoProject(entry)
	if err != nil {
		t.Fatalf("parse project: %v", err)
	}

	want := map[string]bool{
		"GET /api/general/order/list": false,
		"GET /api/admin/order/list":   false,
	}
	for _, r := range ir.Routes {
		key := r.Method + " " + r.Path
		if _, ok := want[key]; ok {
			want[key] = true
		}
	}
	for k, ok := range want {
		if !ok {
			t.Fatalf("expected route %s, got routes: %#v", k, ir.Routes)
		}
	}
}

func TestParseGinProject(t *testing.T) {
	entry := filepath.Join("..", "..", "tests", "example-gin", "main.go")
	ir, err := parser.ParseEchoProject(entry)
	if err != nil {
		t.Fatalf("parse project: %v", err)
	}
	if len(ir.Routes) != 8 {
		t.Fatalf("expected 8 routes, got %d", len(ir.Routes))
	}

	var foundStatus bool
	var foundUserList bool
	var listRoute *model.Route
	for i := range ir.Routes {
		r := ir.Routes[i]
		if r.Method == "GET" && r.Path == "/api/v1/status" {
			foundStatus = true
		}
		if r.Method == "GET" && r.Path == "/api/v1/user/list" {
			foundUserList = true
			listRoute = &ir.Routes[i]
			if len(r.Tags) == 0 || r.Tags[0] != "user" {
				t.Fatalf("expected user tag for list route, got %#v", r.Tags)
			}
			if !r.AuthRequired {
				t.Fatalf("expected auth required for list route")
			}
		}
	}
	if !foundStatus {
		t.Fatalf("missing GET /api/v1/status route")
	}
	if !foundUserList {
		t.Fatalf("missing GET /api/v1/user/list route")
	}
	if listRoute == nil {
		t.Fatalf("missing list route object")
	}
	found200 := false
	found400 := false
	for _, resp := range listRoute.Responses {
		if resp.StatusCode == 200 {
			found200 = true
			if resp.Schema == nil || resp.Schema.Properties["data"] == nil {
				t.Fatalf("expected list 200 response data schema")
			}
		}
		if resp.StatusCode == 400 {
			found400 = true
		}
	}
	if !found200 || !found400 {
		t.Fatalf("expected list responses to contain 200 and 400, got %#v", listRoute.Responses)
	}

	var editRoute *model.Route
	for i := range ir.Routes {
		r := &ir.Routes[i]
		if r.Method == "POST" && r.Path == "/api/v1/user/{id}" {
			editRoute = r
			break
		}
	}
	if editRoute == nil {
		t.Fatalf("missing POST /api/v1/user/{id} route")
	}
	paramTypes := map[string]string{}
	for _, p := range editRoute.Parameters {
		if p.Schema != nil {
			paramTypes[p.Name] = p.Schema.Type
		}
	}
	if paramTypes["id"] != "number" {
		t.Fatalf("expected id parameter type number, got %q", paramTypes["id"])
	}
	if paramTypes["age"] != "number" {
		t.Fatalf("expected age parameter type number, got %q", paramTypes["age"])
	}
}
