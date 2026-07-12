package entrypoint

import (
	"fmt"
	"go/ast"
	goparser "go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"sort"
	"strings"

	projectparser "github.com/arsfy/gswr/internal/parser"
)

func Discover(root string) (string, error) {
	rootAbs, err := filepath.Abs(root)
	if err != nil {
		return "", err
	}
	candidates, err := findMainFiles(rootAbs)
	if err != nil {
		return "", err
	}
	if len(candidates) == 0 {
		return "", fmt.Errorf("no package main with func main() found under %s", root)
	}
	if len(candidates) == 1 {
		return candidates[0], nil
	}

	viable := make([]string, 0, len(candidates))
	for _, candidate := range candidates {
		if _, err := projectparser.ParseEchoProject(candidate); err == nil {
			viable = append(viable, candidate)
		}
	}
	if len(viable) == 1 {
		return viable[0], nil
	}
	if len(viable) > 1 {
		return "", ambiguousError(rootAbs, viable, "multiple API entry points found")
	}
	return "", ambiguousError(rootAbs, candidates, "multiple main entry points found, but none produced routes")
}

func findMainFiles(root string) ([]string, error) {
	fset := token.NewFileSet()
	var candidates []string
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			if path != root && (d.Name() == "vendor" || d.Name() == "testdata" || strings.HasPrefix(d.Name(), ".")) {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		file, err := goparser.ParseFile(fset, path, nil, goparser.SkipObjectResolution)
		if err != nil || file.Name.Name != "main" {
			return nil
		}
		for _, decl := range file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if ok && fn.Recv == nil && fn.Name.Name == "main" && fn.Type.Params.NumFields() == 0 {
				candidates = append(candidates, path)
				break
			}
		}
		return nil
	})
	sort.Strings(candidates)
	return candidates, err
}

func ambiguousError(root string, candidates []string, message string) error {
	relative := make([]string, 0, len(candidates))
	for _, candidate := range candidates {
		rel, err := filepath.Rel(root, candidate)
		if err != nil {
			rel = candidate
		}
		relative = append(relative, filepath.ToSlash(rel))
	}
	return fmt.Errorf("%s:\n  %s\nuse --entry to choose one", message, strings.Join(relative, "\n  "))
}
