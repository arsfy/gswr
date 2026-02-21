package parser

import (
	"fmt"
	"go/ast"
	"path/filepath"
	"strings"

	"github.com/arsfy/gswr/internal/model"
)

func (s *parserState) parseFunction(fn *funcMeta, argGroups map[string]groupState, callKey string) {
	if fn.decl.Body == nil {
		return
	}
	if callKey != "" {
		if s.visitingKey[callKey] {
			return
		}
		s.visitingKey[callKey] = true
		defer delete(s.visitingKey, callKey)
	}

	env := map[string]groupState{}
	for k, v := range argGroups {
		env[k] = v
	}
	s.walkStmts(fn, fn.decl.Body.List, env)
}

func (s *parserState) walkStmts(owner *funcMeta, stmts []ast.Stmt, env map[string]groupState) {
	for _, st := range stmts {
		switch n := st.(type) {
		case *ast.AssignStmt:
			s.handleAssign(owner, n, env)
		case *ast.ExprStmt:
			s.handleCallExpr(owner, n.X, env)
		case *ast.BlockStmt:
			s.walkStmts(owner, n.List, cloneGroupStateMap(env))
		case *ast.IfStmt:
			if n.Init != nil {
				s.walkStmts(owner, []ast.Stmt{n.Init}, env)
			}
			s.walkStmts(owner, n.Body.List, cloneGroupStateMap(env))
			if n.Else != nil {
				s.handleElse(owner, n.Else, cloneGroupStateMap(env))
			}
		}
	}
}

func (s *parserState) handleElse(owner *funcMeta, st ast.Stmt, env map[string]groupState) {
	switch e := st.(type) {
	case *ast.BlockStmt:
		s.walkStmts(owner, e.List, env)
	case *ast.IfStmt:
		s.walkStmts(owner, []ast.Stmt{e}, env)
	}
}

func (s *parserState) handleAssign(owner *funcMeta, st *ast.AssignStmt, env map[string]groupState) {
	if len(st.Lhs) != 1 || len(st.Rhs) != 1 {
		return
	}
	lhs, ok := st.Lhs[0].(*ast.Ident)
	if !ok {
		return
	}
	call, ok := st.Rhs[0].(*ast.CallExpr)
	if !ok {
		return
	}
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return
	}
	recv, ok := sel.X.(*ast.Ident)
	if !ok {
		return
	}
	if sel.Sel.Name == "New" {
		framework := detectFrameworkFromSelector(owner.file, sel)
		if framework == "" {
			return
		}
		env[lhs.Name] = groupState{
			prefix:         "",
			callConvention: routeCallConventionForFramework(framework),
		}
		return
	}
	if sel.Sel.Name != "Group" || len(call.Args) == 0 {
		return
	}
	base, ok := env[recv.Name]
	if !ok {
		return
	}
	p, ok := stringLiteral(call.Args[0])
	if !ok {
		return
	}
	env[lhs.Name] = groupState{
		prefix:         joinPath(base.prefix, p),
		callConvention: base.callConvention,
		authRequired:   base.authRequired || hasAuthMiddleware(call.Args[1:]),
		authSchemes:    mergeMiddlewareNames(base.authSchemes, inferAuthSchemesFromNames(middlewareNamesFromArgs(call.Args[1:]))),
		middlewares:    mergeMiddlewareNames(base.middlewares, middlewareNamesFromArgs(call.Args[1:])),
	}
}

func (s *parserState) handleCallExpr(owner *funcMeta, expr ast.Expr, env map[string]groupState) {
	call, ok := expr.(*ast.CallExpr)
	if !ok {
		return
	}
	if s.tryGroupUse(call, env) {
		return
	}
	if s.tryRoute(owner, call, env) {
		return
	}
	if s.tryRouterCall(owner, call, env) {
		return
	}
	s.tryBootstrapRouterCall(owner, call)
}

func (s *parserState) tryRoute(owner *funcMeta, call *ast.CallExpr, env map[string]groupState) bool {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	recv, ok := sel.X.(*ast.Ident)
	if !ok {
		return false
	}
	if !httpMethods[sel.Sel.Name] || len(call.Args) < 2 {
		return false
	}
	state, ok := env[recv.Name]
	if !ok {
		return false
	}
	p, ok := stringLiteral(call.Args[0])
	if !ok {
		return false
	}
	echoPath := joinPath(state.prefix, p)
	openapiPath, pathParamNames := normalizeEchoPath(echoPath)
	handlerArg, routeMwArgs, ok := splitRouteHandlerAndMiddlewareArgs(call.Args, state.callConvention)
	if !ok {
		return false
	}
	routeMiddlewares := mergeMiddlewareNames(state.middlewares, middlewareNamesFromArgs(routeMwArgs))
	mwParams, mwContextTypes, mwAuthSchemes := s.collectMiddlewareSemanticsWithAuth(owner.pkg, owner.file, routeMiddlewares)

	semantics := handlerSemantics{responses: []model.Response{{StatusCode: 200, Description: "OK", Schema: &model.Schema{Type: "object"}}}}
	handlerName := normalizeOperationID(sel.Sel.Name, openapiPath)
	summary := strings.ToLower(sel.Sel.Name) + " " + openapiPath
	description := ""
	tags := []string{}
	if h, ok := handlerArg.(*ast.FuncLit); ok {
		semantics = s.parseHandlerSemantics(owner.pkg, owner.file, h.Body, mwParams, mwContextTypes)
	}
	if fm := s.resolveCallee(owner, handlerArg); fm != nil {
		semantics = s.parseHandlerSemantics(fm.pkg, fm.file, fm.decl.Body, mwParams, mwContextTypes)
		handlerName = fm.name
		doc := parseRouteDoc(fm.decl.Doc)
		if doc.summary != "" || doc.description != "" || len(doc.tags) > 0 {
			if doc.summary != "" {
				summary = doc.summary
			}
			description = doc.description
			tags = doc.tags
			for _, t := range tags {
				if _, ok := s.tagDescriptions[t]; !ok {
					s.tagDescriptions[t] = ""
				}
			}
		}
	}
	if len(tags) == 0 {
		if inferred := inferTagFromPath(openapiPath); inferred != "" {
			tags = []string{inferred}
			if _, ok := s.tagDescriptions[inferred]; !ok {
				s.tagDescriptions[inferred] = ""
			}
		}
	}

	s.routes = append(s.routes, model.Route{
		Method:       sel.Sel.Name,
		Path:         openapiPath,
		OperationID:  handlerName,
		Summary:      summary,
		Description:  description,
		Tags:         tags,
		AuthRequired: state.authRequired || hasAuthMiddleware(routeMwArgs),
		AuthSchemes:  dedupeStrings(mergeMiddlewareNames(state.authSchemes, mwAuthSchemes)),
		Middlewares:  routeMiddlewares,
		Parameters:   mergeParameters(pathParamNames, semantics.parameters),
		RequestBody:  semantics.requestBody,
		Responses:    semantics.responses,
	})
	return true
}

func splitRouteHandlerAndMiddlewareArgs(args []ast.Expr, convention routeCallConvention) (ast.Expr, []ast.Expr, bool) {
	if len(args) < 2 {
		return nil, nil, false
	}
	if convention == routeCallConventionHandlerLast {
		if len(args) == 2 {
			return args[1], nil, true
		}
		return args[len(args)-1], args[1 : len(args)-1], true
	}
	if len(args) == 2 {
		return args[1], nil, true
	}
	return args[1], args[2:], true
}

func routeCallConventionForFramework(framework string) routeCallConvention {
	switch framework {
	case "gin":
		return routeCallConventionHandlerLast
	default:
		return routeCallConventionHandlerSecond
	}
}

func detectFrameworkFromSelector(file *fileCtx, sel *ast.SelectorExpr) string {
	if sel == nil {
		return ""
	}
	xid, ok := sel.X.(*ast.Ident)
	if !ok {
		return ""
	}
	if file != nil && file.imports != nil {
		if importPath := file.imports[xid.Name]; importPath != "" {
			switch {
			case strings.Contains(importPath, "github.com/labstack/echo"):
				return "echo"
			case strings.Contains(importPath, "github.com/gin-gonic/gin"):
				return "gin"
			}
		}
	}
	switch xid.Name {
	case "echo", "gin":
		return xid.Name
	default:
		return ""
	}
}

func (s *parserState) tryRouterCall(owner *funcMeta, call *ast.CallExpr, env map[string]groupState) bool {
	if len(call.Args) == 0 {
		return false
	}
	callee := s.resolveCallee(owner, call.Fun)
	if callee == nil || callee.decl.Type.Params == nil || len(callee.decl.Type.Params.List) == 0 {
		return false
	}
	firstParam := callee.decl.Type.Params.List[0]
	if !isEchoRouterType(firstParam.Type) || len(firstParam.Names) == 0 {
		return false
	}
	argState, ok := resolveGroupState(call.Args[0], env)
	if !ok {
		return false
	}
	paramName := firstParam.Names[0].Name
	key := fmt.Sprintf("%s.%s@%s#%t", callee.pkg, callee.name, argState.prefix, argState.authRequired)
	s.parseFunction(callee, map[string]groupState{paramName: argState}, key)
	return true
}

func (s *parserState) tryBootstrapRouterCall(owner *funcMeta, call *ast.CallExpr) {
	if len(call.Args) != 0 {
		return
	}
	callee := s.resolveCallee(owner, call.Fun)
	if callee == nil || callee.decl == nil || callee.decl.Body == nil {
		return
	}
	if callee.decl.Type.Params != nil && len(callee.decl.Type.Params.List) > 0 {
		return
	}
	if !s.looksLikeRouteSetup(callee) {
		return
	}
	key := fmt.Sprintf("%s.%s@bootstrap", callee.pkg, callee.name)
	s.parseFunction(callee, map[string]groupState{}, key)
}

func (s *parserState) looksLikeRouteSetup(fm *funcMeta) bool {
	if fm == nil || fm.decl == nil || fm.decl.Body == nil {
		return false
	}
	key := fm.pkg + "." + fm.name
	if hinted, ok := s.routeSetupHints[key]; ok {
		return hinted
	}

	looksLike := false
	ast.Inspect(fm.decl.Body, func(n ast.Node) bool {
		if looksLike {
			return false
		}
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		switch sel.Sel.Name {
		case "New":
			if detectFrameworkFromSelector(fm.file, sel) != "" {
				looksLike = true
				return false
			}
		case "Group", "Use":
			looksLike = true
			return false
		default:
			if httpMethods[sel.Sel.Name] {
				looksLike = true
				return false
			}
		}
		return true
	})
	s.routeSetupHints[key] = looksLike
	return looksLike
}

func (s *parserState) tryGroupUse(call *ast.CallExpr, env map[string]groupState) bool {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok || sel.Sel.Name != "Use" {
		return false
	}
	recv, ok := sel.X.(*ast.Ident)
	if !ok {
		return false
	}
	state, ok := env[recv.Name]
	if !ok {
		return false
	}
	if hasAuthMiddleware(call.Args) {
		state.authRequired = true
	}
	state.authSchemes = dedupeStrings(mergeMiddlewareNames(state.authSchemes, inferAuthSchemesFromNames(middlewareNamesFromArgs(call.Args))))
	state.middlewares = mergeMiddlewareNames(state.middlewares, middlewareNamesFromArgs(call.Args))
	env[recv.Name] = state
	return true
}

func hasAuthMiddleware(args []ast.Expr) bool {
	for _, a := range args {
		if isAuthMiddlewareExpr(a) {
			return true
		}
	}
	return false
}

func isAuthMiddlewareExpr(expr ast.Expr) bool {
	switch n := expr.(type) {
	case *ast.Ident:
		return looksLikeAuthName(n.Name)
	case *ast.SelectorExpr:
		return looksLikeAuthName(n.Sel.Name)
	case *ast.CallExpr:
		return isAuthMiddlewareExpr(n.Fun)
	default:
		return false
	}
}

func looksLikeAuthName(name string) bool {
	v := strings.ToLower(name)
	return strings.Contains(v, "auth") || strings.Contains(v, "jwt") || strings.Contains(v, "bearer") || strings.Contains(v, "apikey") || strings.Contains(v, "session")
}

func middlewareNamesFromArgs(args []ast.Expr) []string {
	out := make([]string, 0, len(args))
	for _, a := range args {
		if name, ok := middlewareName(a); ok && name != "" {
			out = append(out, name)
		}
	}
	return dedupeStrings(out)
}

func middlewareName(expr ast.Expr) (string, bool) {
	switch n := expr.(type) {
	case *ast.Ident:
		return n.Name, true
	case *ast.SelectorExpr:
		return n.Sel.Name, true
	case *ast.CallExpr:
		return middlewareName(n.Fun)
	default:
		return "", false
	}
}

func mergeMiddlewareNames(a, b []string) []string {
	out := append([]string(nil), a...)
	out = append(out, b...)
	return dedupeStrings(out)
}

func dedupeStrings(in []string) []string {
	if len(in) == 0 {
		return nil
	}
	seen := map[string]bool{}
	out := make([]string, 0, len(in))
	for _, v := range in {
		if v == "" || seen[v] {
			continue
		}
		seen[v] = true
		out = append(out, v)
	}
	return out
}

func (s *parserState) resolveCallee(owner *funcMeta, fun ast.Expr) *funcMeta {
	return s.resolveCalleeByPkg(owner.pkg, owner.file, fun)
}

func (s *parserState) resolveCalleeByPkg(pkg string, file *fileCtx, fun ast.Expr) *funcMeta {
	switch n := fun.(type) {
	case *ast.Ident:
		return s.funcsByPkg[pkg][n.Name]
	case *ast.SelectorExpr:
		if file == nil {
			return nil
		}
		pkgAlias, ok := n.X.(*ast.Ident)
		if !ok {
			return nil
		}
		importPath := file.imports[pkgAlias.Name]
		if importPath == "" {
			return nil
		}
		if byName := s.funcsByImportPath[importPath]; byName != nil {
			if fm := byName[n.Sel.Name]; fm != nil {
				return fm
			}
		}
		return s.funcsByPkg[filepath.Base(importPath)][n.Sel.Name]
	default:
		return nil
	}
}
