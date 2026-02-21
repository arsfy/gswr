package integration

import (
	"os"
	"path/filepath"
	"testing"

	"golang-openapi/internal/parser"
	"golang-openapi/internal/renderer"
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
		t.Fatalf("golden mismatch: %s\nregenerate with: GOCACHE=/tmp/go-build-cache go run ./cmd/openapi-gen --entry tests/example-echo/main.go --out docs/openapi.yaml && cp docs/openapi.yaml internal/integration/testdata/openapi.golden.yaml", wantPath)
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
