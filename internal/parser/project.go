package parser

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"gswr/internal/model"
)

func ParseEchoProject(entry string) (*model.IR, error) {
	entryAbs, err := filepath.Abs(entry)
	if err != nil {
		return nil, err
	}
	root := findParseRoot(entryAbs)
	fset := token.NewFileSet()

	state := &parserState{
		fset:                 fset,
		apiTitle:             "Example Echo API",
		apiVersion:           "0.1.0",
		apiDescription:       "",
		apiBasePath:          "",
		apiSchemes:           nil,
		apiHost:              "",
		filesByPkg:           map[string][]*fileCtx{},
		funcsByPkg:           map[string]map[string]*funcMeta{},
		funcsByImportPath:    map[string]map[string]*funcMeta{},
		namedTypesByPkg:      map[string]map[string]*namedTypeMeta{},
		namedTypesByImport:   map[string]map[string]*namedTypeMeta{},
		components:           map[string]*model.Schema{},
		tagDescriptions:      map[string]string{},
		visitingKey:          map[string]bool{},
		routeSetupHints:      map[string]bool{},
		buildingComponentRef: map[string]bool{},
	}
	modulePath := readModulePath(root)
	var entryMain *funcMeta

	err = filepath.WalkDir(root, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			if d.Name() == "vendor" || strings.HasPrefix(d.Name(), ".") {
				if path != root {
					return filepath.SkipDir
				}
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") {
			return nil
		}

		f, err := parser.ParseFile(fset, path, nil, parser.ParseComments)
		if err != nil {
			return err
		}
		collectTagDefinitions(f, state.tagDescriptions)
		ctx := &fileCtx{
			path:       path,
			pkg:        f.Name.Name,
			importPath: importPathForFile(root, modulePath, path),
			astFile:    f,
			imports:    parseImports(f),
		}
		state.filesByPkg[ctx.pkg] = append(state.filesByPkg[ctx.pkg], ctx)
		if state.funcsByPkg[ctx.pkg] == nil {
			state.funcsByPkg[ctx.pkg] = map[string]*funcMeta{}
		}
		if state.funcsByImportPath[ctx.importPath] == nil {
			state.funcsByImportPath[ctx.importPath] = map[string]*funcMeta{}
		}
		if state.namedTypesByPkg[ctx.pkg] == nil {
			state.namedTypesByPkg[ctx.pkg] = map[string]*namedTypeMeta{}
		}
		if state.namedTypesByImport[ctx.importPath] == nil {
			state.namedTypesByImport[ctx.importPath] = map[string]*namedTypeMeta{}
		}
		for _, decl := range f.Decls {
			if fd, ok := decl.(*ast.FuncDecl); ok && fd.Recv == nil {
				meta := &funcMeta{pkg: ctx.pkg, name: fd.Name.Name, decl: fd, file: ctx}
				state.funcsByPkg[ctx.pkg][fd.Name.Name] = meta
				state.funcsByImportPath[ctx.importPath][fd.Name.Name] = meta
				if path == entryAbs && ctx.pkg == "main" && fd.Name.Name == "main" {
					entryMain = meta
				}
				if ctx.pkg == "main" && fd.Name.Name == "main" && path == entryAbs {
					info := parseMainDocInfo(fd.Doc)
					if info.title != "" {
						state.apiTitle = info.title
					}
					if info.version != "" {
						state.apiVersion = info.version
					}
					if info.description != "" {
						state.apiDescription = info.description
					}
					if info.basePath != "" {
						state.apiBasePath = info.basePath
					}
					if len(info.schemes) > 0 {
						state.apiSchemes = info.schemes
					}
					if info.host != "" {
						state.apiHost = info.host
					}
				}
			}
			gd, ok := decl.(*ast.GenDecl)
			if !ok || gd.Tok != token.TYPE {
				continue
			}
			for _, spec := range gd.Specs {
				ts, ok := spec.(*ast.TypeSpec)
				if !ok {
					continue
				}
				state.namedTypesByPkg[ctx.pkg][ts.Name.Name] = &namedTypeMeta{pkg: ctx.pkg, name: ts.Name.Name, typeExpr: ts.Type, file: ctx}
				state.namedTypesByImport[ctx.importPath][ts.Name.Name] = &namedTypeMeta{pkg: ctx.pkg, name: ts.Name.Name, typeExpr: ts.Type, file: ctx}
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	mainFunc := entryMain
	if mainFunc == nil {
		return nil, fmt.Errorf("main.main not found from entry root: %s", root)
	}

	state.parseFunction(mainFunc, map[string]groupState{}, "")
	sort.Slice(state.routes, func(i, j int) bool {
		if state.routes[i].Path == state.routes[j].Path {
			return state.routes[i].Method < state.routes[j].Method
		}
		return state.routes[i].Path < state.routes[j].Path
	})
	if len(state.routes) == 0 {
		return nil, fmt.Errorf("no echo routes found")
	}

	tagNames := make([]string, 0, len(state.tagDescriptions))
	for name := range state.tagDescriptions {
		tagNames = append(tagNames, name)
	}
	sort.Strings(tagNames)
	tags := make([]model.Tag, 0, len(tagNames))
	for _, name := range tagNames {
		tags = append(tags, model.Tag{
			Name:        name,
			Description: state.tagDescriptions[name],
		})
	}

	return &model.IR{
		Title:       state.apiTitle,
		Version:     state.apiVersion,
		Description: state.apiDescription,
		Servers:     buildServers(state.apiBasePath, state.apiSchemes, state.apiHost),
		Tags:        tags,
		Routes:      state.routes,
		Components:  state.components,
	}, nil
}

func findParseRoot(entryAbs string) string {
	dir := filepath.Dir(entryAbs)
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return filepath.Dir(entryAbs)
		}
		dir = parent
	}
}

func readModulePath(root string) string {
	data, err := os.ReadFile(filepath.Join(root, "go.mod"))
	if err != nil {
		return ""
	}
	for _, raw := range strings.Split(string(data), "\n") {
		line := strings.TrimSpace(raw)
		if v, ok := strings.CutPrefix(line, "module "); ok {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

func importPathForFile(root, modulePath, filePath string) string {
	if modulePath == "" {
		return ""
	}
	dir := filepath.Dir(filePath)
	rel, err := filepath.Rel(root, dir)
	if err != nil {
		return modulePath
	}
	rel = filepath.ToSlash(rel)
	if rel == "." {
		return modulePath
	}
	return modulePath + "/" + rel
}

func parseImports(f *ast.File) map[string]string {
	out := map[string]string{}
	for _, imp := range f.Imports {
		p, _ := strconv.Unquote(imp.Path.Value)
		name := filepath.Base(p)
		if imp.Name != nil {
			name = imp.Name.Name
		}
		out[name] = p
	}
	return out
}
