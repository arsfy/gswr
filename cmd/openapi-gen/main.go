package main

import (
	"flag"
	"fmt"
	"os"

	"golang-openapi/internal/parser"
	"golang-openapi/internal/renderer"
)

func main() {
	entry := flag.String("entry", "cmd/main.go", "entry go file")
	out := flag.String("out", "docs/openapi.yaml", "output openapi yaml")
	flag.Parse()

	ir, err := parser.ParseEchoProject(*entry)
	if err != nil {
		fmt.Fprintf(os.Stderr, "parse failed: %v\n", err)
		os.Exit(1)
	}
	if err := renderer.WriteYAML(ir, *out); err != nil {
		fmt.Fprintf(os.Stderr, "render failed: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("generated %s from %s (%d routes)\n", *out, *entry, len(ir.Routes))
}
