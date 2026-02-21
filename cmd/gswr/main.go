package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gswr/internal/parser"
	"gswr/internal/renderer"
)

func main() {
	entry := flag.String("entry", "cmd/main.go", "entry go file")
	out := flag.String("out", "docs/openapi.yaml", "output openapi yaml")
	format := flag.String("format", "auto", "output format: auto|yaml|json")
	flag.Parse()

	ir, err := parser.ParseEchoProject(*entry)
	if err != nil {
		fmt.Fprintf(os.Stderr, "parse failed: %v\n", err)
		os.Exit(1)
	}
	actualFormat := resolveFormat(*format, *out)
	var renderErr error
	switch actualFormat {
	case "json":
		renderErr = renderer.WriteJSON(ir, *out)
	default:
		renderErr = renderer.WriteYAML(ir, *out)
	}
	if renderErr != nil {
		fmt.Fprintf(os.Stderr, "render failed: %v\n", renderErr)
		os.Exit(1)
	}

	fmt.Printf("generated %s from %s (%d routes)\n", *out, *entry, len(ir.Routes))
}

func resolveFormat(format, out string) string {
	f := strings.ToLower(strings.TrimSpace(format))
	switch f {
	case "yaml", "yml", "json":
		if f == "yml" {
			return "yaml"
		}
		return f
	case "", "auto":
		ext := strings.ToLower(filepath.Ext(out))
		if ext == ".json" {
			return "json"
		}
		return "yaml"
	default:
		ext := strings.ToLower(filepath.Ext(out))
		if ext == ".json" {
			return "json"
		}
		return "yaml"
	}
}
