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
