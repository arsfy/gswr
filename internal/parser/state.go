package parser

import (
	"go/ast"
	"go/token"

	"golang-openapi/internal/model"
)

var httpMethods = map[string]bool{
	"GET": true, "POST": true, "PUT": true, "PATCH": true, "DELETE": true,
}

type fileCtx struct {
	path       string
	pkg        string
	importPath string
	astFile    *ast.File
	imports    map[string]string
}

type namedTypeMeta struct {
	pkg      string
	name     string
	typeExpr ast.Expr
	file     *fileCtx
}

type parserState struct {
	fset                 *token.FileSet
	apiTitle             string
	apiVersion           string
	apiDescription       string
	apiBasePath          string
	apiSchemes           []string
	apiHost              string
	filesByPkg           map[string][]*fileCtx
	funcsByPkg           map[string]map[string]*funcMeta
	funcsByImportPath    map[string]map[string]*funcMeta
	namedTypesByPkg      map[string]map[string]*namedTypeMeta
	namedTypesByImport   map[string]map[string]*namedTypeMeta
	components           map[string]*model.Schema
	tagDescriptions      map[string]string
	visitingKey          map[string]bool
	routeSetupHints      map[string]bool
	buildingComponentRef map[string]bool
	routes               []model.Route
}

type groupState struct {
	prefix       string
	authRequired bool
	authSchemes  []string
	middlewares  []string
}

type funcMeta struct {
	pkg  string
	name string
	decl *ast.FuncDecl
	file *fileCtx
}

type handlerSemantics struct {
	responses   []model.Response
	parameters  []model.Parameter
	requestBody *model.Schema
}
