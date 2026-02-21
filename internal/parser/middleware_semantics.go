package parser

import (
	"go/ast"

	"golang-openapi/internal/model"
)

func (s *parserState) collectMiddlewareSemantics(pkg string, middlewareNames []string) ([]model.Parameter, map[string]ast.Expr) {
	params := map[string]model.Parameter{}
	contextTypes := map[string]ast.Expr{}
	seen := map[string]bool{}

	for _, name := range middlewareNames {
		if name == "" || seen[name] {
			continue
		}
		seen[name] = true
		if looksLikeAuthName(name) {
			// Auth middleware should map to `security`, not operation parameters.
			continue
		}
		fm := s.funcsByPkg[pkg][name]
		if fm == nil || fm.decl == nil || fm.decl.Body == nil {
			continue
		}

		inner := middlewareInnerBody(fm.decl.Body)
		if inner == nil {
			continue
		}
		varTypes, _ := s.collectVarContext(pkg, fm.file, inner, nil)

		ast.Inspect(inner, func(n ast.Node) bool {
			switch x := n.(type) {
			case *ast.AssignStmt:
				s.collectInputParameterFromAssign(params, fm.file, x)
			case *ast.CallExpr:
				s.collectInputParameter(params, x)
				s.captureContextSetType(contextTypes, varTypes, x)
			}
			return true
		})
	}

	outParams := make([]model.Parameter, 0, len(params))
	for _, p := range params {
		outParams = append(outParams, p)
	}
	return outParams, contextTypes
}

func middlewareInnerBody(body *ast.BlockStmt) *ast.BlockStmt {
	for _, st := range body.List {
		ret, ok := st.(*ast.ReturnStmt)
		if !ok || len(ret.Results) != 1 {
			continue
		}
		fn, ok := ret.Results[0].(*ast.FuncLit)
		if ok {
			return fn.Body
		}
	}
	return body
}

func (s *parserState) captureContextSetType(contextTypes map[string]ast.Expr, varTypes map[string]ast.Expr, call *ast.CallExpr) {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok || sel.Sel.Name != "Set" || len(call.Args) != 2 {
		return
	}
	key, ok := stringLiteral(call.Args[0])
	if !ok || key == "" {
		return
	}
	if t, ok := inferTypeFromExprWithContext(call.Args[1], contextTypes); ok {
		contextTypes[key] = t
		return
	}
	if id, ok := call.Args[1].(*ast.Ident); ok {
		if t, ok := varTypes[id.Name]; ok {
			contextTypes[key] = t
		}
	}
}
