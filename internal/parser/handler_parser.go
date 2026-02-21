package parser

import (
	"go/ast"
	"go/token"
	"path/filepath"
	"sort"
	"strings"

	"github.com/arsfy/gswr/internal/model"
)

func (s *parserState) parseHandlerSemantics(pkg string, file *fileCtx, body *ast.BlockStmt, inheritedParams []model.Parameter, contextTypes map[string]ast.Expr) handlerSemantics {
	result := handlerSemantics{responses: []model.Response{{StatusCode: 200, Description: "OK", Schema: &model.Schema{Type: "object"}}}}
	if body == nil {
		result.parameters = inheritedParams
		return result
	}

	varTypes, varValues := s.collectVarContext(pkg, file, body, contextTypes)
	result.responses = s.parseHandlerResponses(pkg, file, body, varTypes, varValues)
	result.parameters, result.requestBody = s.parseHandlerInputs(pkg, file, body, varTypes)
	for _, p := range inheritedParams {
		mergeParameterIntoSlice(&result.parameters, p)
	}
	return result
}

func (s *parserState) parseHandlerResponses(pkg string, file *fileCtx, body *ast.BlockStmt, varTypes map[string]ast.Expr, varValues map[string]ast.Expr) []model.Response {
	byCode := map[int]model.Response{}
	order := make([]int, 0, 4)

	ast.Inspect(body, func(n ast.Node) bool {
		ret, ok := n.(*ast.ReturnStmt)
		if !ok || len(ret.Results) != 1 {
			return true
		}
		resps := s.extractResponsesFromExpr(pkg, file, ret.Results[0], varTypes, varValues, nil, 0)
		if len(resps) == 0 {
			return true
		}
		desc := ""
		if file != nil && s.fset != nil {
			desc = inlineCommentForNode(s.fset, file.astFile, ret)
		}
		for _, r := range resps {
			code := r.StatusCode
			if desc != "" {
				r.Description = desc
			}
			if r.Description == "" {
				r.Description = responseDescription(code)
			}
			if _, exists := byCode[code]; !exists {
				order = append(order, code)
			}
			byCode[code] = r
		}
		return true
	})

	if len(order) == 0 {
		return []model.Response{{StatusCode: 200, Description: "OK", Schema: &model.Schema{Type: "object"}}}
	}
	sort.Ints(order)
	out := make([]model.Response, 0, len(order))
	for _, code := range order {
		out = append(out, byCode[code])
	}
	return out
}

func (s *parserState) extractResponsesFromExpr(pkg string, file *fileCtx, expr ast.Expr, varTypes map[string]ast.Expr, varValues map[string]ast.Expr, bindings map[string]ast.Expr, depth int) []model.Response {
	if depth > 6 {
		return nil
	}
	call, ok := expr.(*ast.CallExpr)
	if !ok {
		return nil
	}
	if r, ok := s.extractJSONResponseFromCall(pkg, file, call, varTypes, varValues, bindings); ok {
		return []model.Response{r}
	}

	fm := s.resolveCalleeByPkg(pkg, file, call.Fun)
	if fm == nil || fm.decl == nil || fm.decl.Body == nil || fm.decl.Type.Params == nil {
		return nil
	}

	localBindings := map[string]ast.Expr{}
	for k, v := range bindings {
		localBindings[k] = v
	}
	argIdx := 0
	for _, p := range fm.decl.Type.Params.List {
		for _, name := range p.Names {
			if argIdx < len(call.Args) {
				localBindings[name.Name] = resolveBindingExpr(call.Args[argIdx], bindings)
			}
			argIdx++
		}
	}

	out := make([]model.Response, 0, 2)
	for _, st := range fm.decl.Body.List {
		ret, ok := st.(*ast.ReturnStmt)
		if !ok || len(ret.Results) == 0 {
			continue
		}
		for _, r := range ret.Results {
			out = append(out, s.extractResponsesFromExpr(fm.pkg, fm.file, resolveBindingExpr(r, localBindings), varTypes, varValues, localBindings, depth+1)...)
		}
	}
	return dedupeResponsesByCode(out)
}

func (s *parserState) extractJSONResponseFromCall(pkg string, file *fileCtx, call *ast.CallExpr, varTypes map[string]ast.Expr, varValues map[string]ast.Expr, bindings map[string]ast.Expr) (model.Response, bool) {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok || sel.Sel.Name != "JSON" || len(call.Args) < 2 {
		return model.Response{}, false
	}
	statusExpr := resolveBindingExpr(call.Args[0], bindings)
	bodyExpr := resolveBindingExpr(call.Args[1], bindings)
	code, ok := resolveStatusCode(statusExpr, varValues, map[string]bool{})
	if !ok {
		code = 200
	}
	return model.Response{
		StatusCode:  code,
		Description: responseDescription(code),
		Schema:      s.schemaFromExprWithVarsAndBindings(pkg, file, bodyExpr, varTypes, bindings),
	}, true
}

func dedupeResponsesByCode(in []model.Response) []model.Response {
	if len(in) == 0 {
		return nil
	}
	m := map[int]model.Response{}
	keys := make([]int, 0, len(in))
	for _, r := range in {
		if _, ok := m[r.StatusCode]; !ok {
			keys = append(keys, r.StatusCode)
		}
		m[r.StatusCode] = r
	}
	sort.Ints(keys)
	out := make([]model.Response, 0, len(keys))
	for _, k := range keys {
		out = append(out, m[k])
	}
	return out
}

func resolveStatusCode(expr ast.Expr, values map[string]ast.Expr, visiting map[string]bool) (int, bool) {
	if code, ok := intLiteral(expr); ok {
		return code, true
	}
	switch n := expr.(type) {
	case *ast.SelectorExpr:
		return statusCodeFromName(n.Sel.Name)
	case *ast.Ident:
		if values == nil {
			return 0, false
		}
		if visiting[n.Name] {
			return 0, false
		}
		v, ok := values[n.Name]
		if !ok {
			return 0, false
		}
		visiting[n.Name] = true
		code, ok := resolveStatusCode(v, values, visiting)
		delete(visiting, n.Name)
		return code, ok
	default:
		return 0, false
	}
}

func statusCodeFromName(name string) (int, bool) {
	switch name {
	case "StatusOK":
		return 200, true
	case "StatusCreated":
		return 201, true
	case "StatusAccepted":
		return 202, true
	case "StatusNoContent":
		return 204, true
	case "StatusBadRequest":
		return 400, true
	case "StatusUnauthorized":
		return 401, true
	case "StatusForbidden":
		return 403, true
	case "StatusNotFound":
		return 404, true
	case "StatusConflict":
		return 409, true
	case "StatusUnprocessableEntity":
		return 422, true
	case "StatusTooManyRequests":
		return 429, true
	case "StatusInternalServerError":
		return 500, true
	case "StatusBadGateway":
		return 502, true
	case "StatusServiceUnavailable":
		return 503, true
	case "StatusGatewayTimeout":
		return 504, true
	default:
		return 0, false
	}
}

func responseDescription(code int) string {
	if code >= 200 && code < 300 {
		return "OK"
	}
	if code >= 400 && code < 500 {
		return "Client Error"
	}
	if code >= 500 {
		return "Server Error"
	}
	return "Response"
}

func (s *parserState) parseHandlerInputs(pkg string, file *fileCtx, body *ast.BlockStmt, varTypes map[string]ast.Expr) ([]model.Parameter, *model.Schema) {
	params := map[string]model.Parameter{}
	var requestBody *model.Schema

	ast.Inspect(body, func(n ast.Node) bool {
		switch x := n.(type) {
		case *ast.AssignStmt:
			s.collectInputParameterFromAssign(params, file, x)
		case *ast.CallExpr:
			s.collectInputParameter(params, x)
			s.collectInputParameterFromHelper(pkg, file, params, x, 0, nil)
			bindParams, bindBody := s.tryParseBindSemantics(pkg, file, x, varTypes)
			for _, p := range bindParams {
				mergeParameter(params, p)
			}
			if requestBody == nil && bindBody != nil {
				requestBody = bindBody
			}
		}
		return true
	})

	out := make([]model.Parameter, 0, len(params))
	for _, p := range params {
		out = append(out, p)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].In == out[j].In {
			return out[i].Name < out[j].Name
		}
		return paramOrder(out[i].In) < paramOrder(out[j].In)
	})
	return out, requestBody
}

func (s *parserState) collectInputParameterFromAssign(params map[string]model.Parameter, file *fileCtx, st *ast.AssignStmt) {
	if file == nil || s.fset == nil {
		return
	}
	desc := inlineCommentForNode(s.fset, file.astFile, st)
	calls := parameterCallsInExpr(st.Rhs)
	for _, call := range calls {
		mergeParameter(params, model.Parameter{
			Name:        call.name,
			In:          call.in,
			Required:    call.required,
			Description: desc,
			Schema:      &model.Schema{Type: "string"},
		})
	}
}

type parameterCall struct {
	name     string
	in       string
	required bool
}

func parameterCallsInExpr(exprs []ast.Expr) []parameterCall {
	out := make([]parameterCall, 0, 2)
	for _, expr := range exprs {
		ast.Inspect(expr, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			if p, ok := parseParameterCall(call); ok {
				out = append(out, p)
			}
			return true
		})
	}
	return out
}

func parseParameterCall(call *ast.CallExpr) (parameterCall, bool) {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok || len(call.Args) == 0 {
		return parameterCall{}, false
	}
	name, ok := stringLiteral(call.Args[0])
	if !ok || name == "" {
		return parameterCall{}, false
	}
	switch sel.Sel.Name {
	case "Param":
		return parameterCall{name: name, in: "path", required: true}, true
	case "QueryParam", "QueryParamOr", "FormValue", "FormValueOr":
		return parameterCall{name: name, in: "query", required: false}, true
	case "Get":
		if isHeaderGetReceiver(sel.X) {
			return parameterCall{name: name, in: "header", required: false}, true
		}
	}
	return parameterCall{}, false
}

func (s *parserState) collectInputParameterFromHelper(pkg string, file *fileCtx, params map[string]model.Parameter, call *ast.CallExpr, depth int, bindings map[string]ast.Expr) {
	if depth > 4 {
		return
	}
	fm := s.resolveCalleeByPkg(pkg, file, call.Fun)
	if fm == nil || fm.decl == nil || fm.decl.Body == nil || fm.decl.Type.Params == nil {
		return
	}

	localBindings := map[string]ast.Expr{}
	for k, v := range bindings {
		localBindings[k] = v
	}
	argIdx := 0
	for _, p := range fm.decl.Type.Params.List {
		for _, name := range p.Names {
			if argIdx < len(call.Args) {
				localBindings[name.Name] = resolveBindingExpr(call.Args[argIdx], bindings)
			}
			argIdx++
		}
	}

	ast.Inspect(fm.decl.Body, func(n ast.Node) bool {
		innerCall, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		if p, ok := parseParameterCallWithBindings(innerCall, localBindings); ok {
			mergeParameter(params, model.Parameter{
				Name:     p.name,
				In:       p.in,
				Required: p.required,
				Schema:   &model.Schema{Type: "string"},
			})
		}
		s.collectInputParameterFromHelper(fm.pkg, fm.file, params, innerCall, depth+1, localBindings)
		return true
	})
}

func parseParameterCallWithBindings(call *ast.CallExpr, bindings map[string]ast.Expr) (parameterCall, bool) {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok || len(call.Args) == 0 {
		return parameterCall{}, false
	}
	name, ok := stringLiteral(resolveBindingExpr(call.Args[0], bindings))
	if !ok || name == "" {
		return parameterCall{}, false
	}
	switch sel.Sel.Name {
	case "Param":
		return parameterCall{name: name, in: "path", required: true}, true
	case "QueryParam", "QueryParamOr", "FormValue", "FormValueOr":
		return parameterCall{name: name, in: "query", required: false}, true
	case "Get":
		if isHeaderGetReceiver(sel.X) {
			return parameterCall{name: name, in: "header", required: false}, true
		}
	}
	return parameterCall{}, false
}

func resolveBindingExpr(expr ast.Expr, bindings map[string]ast.Expr) ast.Expr {
	id, ok := expr.(*ast.Ident)
	if !ok || bindings == nil {
		return expr
	}
	if v, ok := bindings[id.Name]; ok {
		return v
	}
	return expr
}

func (s *parserState) collectVarContext(pkg string, file *fileCtx, body *ast.BlockStmt, contextTypes map[string]ast.Expr) (map[string]ast.Expr, map[string]ast.Expr) {
	varTypes := map[string]ast.Expr{}
	varValues := map[string]ast.Expr{}
	ast.Inspect(body, func(n ast.Node) bool {
		switch x := n.(type) {
		case *ast.ValueSpec:
			if x.Type == nil {
				// keep walking to capture values below
			} else {
				for _, name := range x.Names {
					varTypes[name.Name] = x.Type
				}
			}
			for i, name := range x.Names {
				if i < len(x.Values) {
					varValues[name.Name] = x.Values[i]
				}
			}
		case *ast.AssignStmt:
			for i := range x.Lhs {
				if i >= len(x.Rhs) {
					break
				}
				lhs, ok := x.Lhs[i].(*ast.Ident)
				if !ok {
					continue
				}
				if t, ok := s.inferTypeFromExprWithResolver(pkg, file, x.Rhs[i], contextTypes); ok {
					varTypes[lhs.Name] = t
				}
				varValues[lhs.Name] = x.Rhs[i]
			}
		}
		return true
	})
	return varTypes, varValues
}

func (s *parserState) inferTypeFromExprWithResolver(pkg string, file *fileCtx, expr ast.Expr, contextTypes map[string]ast.Expr) (ast.Expr, bool) {
	switch n := expr.(type) {
	case *ast.CallExpr:
		if t, ok := inferTypeFromContextGetCall(n, contextTypes); ok {
			return t, true
		}
		if t, ok := inferTypeFromCall(n); ok {
			return t, true
		}
		if fm := s.resolveCalleeByPkg(pkg, file, n.Fun); fm != nil && fm.decl.Type.Results != nil && len(fm.decl.Type.Results.List) > 0 {
			return fm.decl.Type.Results.List[0].Type, true
		}
		return nil, false
	default:
		return inferTypeFromExprWithContext(expr, contextTypes)
	}
}

func (s *parserState) collectInputParameter(params map[string]model.Parameter, call *ast.CallExpr) {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok || len(call.Args) == 0 {
		return
	}
	name, ok := stringLiteral(call.Args[0])
	if !ok || name == "" {
		return
	}
	add := func(in string, required bool) {
		mergeParameter(params, model.Parameter{Name: name, In: in, Required: required, Schema: &model.Schema{Type: "string"}})
	}

	switch sel.Sel.Name {
	case "Param":
		add("path", true)
	case "QueryParam", "QueryParamOr", "FormValue", "FormValueOr":
		add("query", false)
	case "Get":
		if isHeaderGetReceiver(sel.X) {
			add("header", false)
		}
	}
}

func (s *parserState) tryParseBindSemantics(pkg string, file *fileCtx, call *ast.CallExpr, varTypes map[string]ast.Expr) ([]model.Parameter, *model.Schema) {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok || sel.Sel.Name != "Bind" || len(call.Args) != 1 {
		return nil, nil
	}
	boundType, ok := boundTypeFromArg(call.Args[0], varTypes)
	if !ok {
		return nil, nil
	}
	return s.bindTypeSemantics(pkg, file, boundType)
}

func (s *parserState) bindTypeSemantics(pkg string, file *fileCtx, typeExpr ast.Expr) ([]model.Parameter, *model.Schema) {
	resolvedPkg, resolvedFile, st := s.resolveStructType(pkg, file, typeExpr)
	if st == nil {
		return nil, s.schemaFromTypeExpr(pkg, file, typeExpr)
	}

	params := make([]model.Parameter, 0, 4)
	bodyProps := map[string]*model.Schema{}
	bodyRequired := make([]string, 0, 2)

	for _, f := range st.Fields.List {
		if len(f.Names) == 0 {
			continue
		}
		fieldName := f.Names[0].Name
		fieldSchema := s.schemaFromTypeExpr(resolvedPkg, resolvedFile, f.Type)
		required := fieldRequired(f.Tag)

		pathName, pathOK := tagLookup(f.Tag, "param")
		if pathOK && pathName != "" && pathName != "-" {
			params = append(params, model.Parameter{Name: pathName, In: "path", Required: true, Schema: fieldSchema})
		}
		queryName, queryOK := tagLookup(f.Tag, "query")
		if queryOK && queryName != "" && queryName != "-" {
			params = append(params, model.Parameter{Name: queryName, In: "query", Required: required, Schema: fieldSchema})
		}
		headerName, headerOK := tagLookup(f.Tag, "header")
		if headerOK && headerName != "" && headerName != "-" {
			params = append(params, model.Parameter{Name: headerName, In: "header", Required: required, Schema: fieldSchema})
		}

		jsonName, hasJSON := tagLookup(f.Tag, "json")
		hasParamLike := pathOK || queryOK || headerOK
		if hasJSON {
			if jsonName == "-" {
				continue
			}
			name := jsonName
			if name == "" {
				name = lowerFirst(fieldName)
			}
			bodyProps[name] = fieldSchema
			if required {
				bodyRequired = append(bodyRequired, name)
			}
			continue
		}
		if !hasParamLike {
			name := lowerFirst(fieldName)
			bodyProps[name] = fieldSchema
			if required {
				bodyRequired = append(bodyRequired, name)
			}
		}
	}

	if len(bodyProps) == 0 {
		return params, nil
	}
	return params, &model.Schema{Type: "object", Properties: bodyProps, Required: dedupeSorted(bodyRequired)}
}

func (s *parserState) resolveStructType(pkg string, file *fileCtx, typeExpr ast.Expr) (string, *fileCtx, *ast.StructType) {
	switch t := typeExpr.(type) {
	case *ast.StarExpr:
		return s.resolveStructType(pkg, file, t.X)
	case *ast.StructType:
		return pkg, file, t
	case *ast.Ident:
		meta, ok := s.resolveNamedTypeInScope(pkg, file, t.Name)
		if !ok || meta == nil {
			return "", nil, nil
		}
		st, ok := meta.typeExpr.(*ast.StructType)
		if !ok {
			return "", nil, nil
		}
		return meta.pkg, meta.file, st
	case *ast.SelectorExpr:
		if file == nil {
			return "", nil, nil
		}
		alias, ok := t.X.(*ast.Ident)
		if !ok {
			return "", nil, nil
		}
		importPath := file.imports[alias.Name]
		if importPath == "" {
			return "", nil, nil
		}
		targetPkg := filepath.Base(importPath)
		meta := s.namedTypesByPkg[targetPkg][t.Sel.Name]
		if meta == nil {
			return "", nil, nil
		}
		st, ok := meta.typeExpr.(*ast.StructType)
		if !ok {
			return "", nil, nil
		}
		return meta.pkg, meta.file, st
	default:
		return "", nil, nil
	}
}

func inferTypeFromExprWithContext(expr ast.Expr, contextTypes map[string]ast.Expr) (ast.Expr, bool) {
	switch n := expr.(type) {
	case *ast.CompositeLit:
		return n.Type, true
	case *ast.UnaryExpr:
		if n.Op == token.AND {
			if lit, ok := n.X.(*ast.CompositeLit); ok {
				return lit.Type, true
			}
		}
	case *ast.CallExpr:
		if t, ok := inferTypeFromContextGetCall(n, contextTypes); ok {
			return t, true
		}
		return inferTypeFromCall(n)
	case *ast.TypeAssertExpr:
		return n.Type, true
	}
	return nil, false
}

func inferTypeFromExpr(expr ast.Expr) (ast.Expr, bool) {
	return inferTypeFromExprWithContext(expr, nil)
}

func inferTypeFromContextGetCall(call *ast.CallExpr, contextTypes map[string]ast.Expr) (ast.Expr, bool) {
	if contextTypes == nil {
		return nil, false
	}
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok || sel.Sel.Name != "Get" || len(call.Args) != 1 {
		return nil, false
	}
	key, ok := stringLiteral(call.Args[0])
	if !ok || key == "" {
		return nil, false
	}
	t, ok := contextTypes[key]
	return t, ok
}

func mergeParameterIntoSlice(dst *[]model.Parameter, p model.Parameter) {
	params := map[string]model.Parameter{}
	for _, cur := range *dst {
		params[cur.In+":"+cur.Name] = cur
	}
	mergeParameter(params, p)
	out := make([]model.Parameter, 0, len(params))
	for _, v := range params {
		out = append(out, v)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].In == out[j].In {
			return out[i].Name < out[j].Name
		}
		return paramOrder(out[i].In) < paramOrder(out[j].In)
	})
	*dst = out
}

func inferTypeFromCall(call *ast.CallExpr) (ast.Expr, bool) {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return nil, false
	}
	switch sel.Sel.Name {
	case "Param", "QueryParam", "QueryParamOr", "FormValue", "FormValueOr", "Get":
		return ast.NewIdent("string"), true
	case "ParseInt", "ParseFloat", "Atoi":
		return ast.NewIdent("int64"), true
	default:
		return nil, false
	}
}

func boundTypeFromArg(arg ast.Expr, varTypes map[string]ast.Expr) (ast.Expr, bool) {
	switch n := arg.(type) {
	case *ast.Ident:
		t, ok := varTypes[n.Name]
		return t, ok
	case *ast.UnaryExpr:
		if n.Op != token.AND {
			return nil, false
		}
		switch x := n.X.(type) {
		case *ast.Ident:
			t, ok := varTypes[x.Name]
			return t, ok
		case *ast.CompositeLit:
			return x.Type, true
		}
	case *ast.CompositeLit:
		return n.Type, true
	}
	return nil, false
}

func isHeaderGetReceiver(expr ast.Expr) bool {
	sel, ok := expr.(*ast.SelectorExpr)
	if !ok || sel.Sel.Name != "Header" {
		return false
	}
	call, ok := sel.X.(*ast.CallExpr)
	if !ok {
		return false
	}
	innerSel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok || innerSel.Sel.Name != "Request" {
		return false
	}
	return true
}

func normalizeOperationID(method, path string) string {
	repl := strings.NewReplacer("/", "_", "{", "", "}", "", "-", "_")
	return strings.ToLower(method) + repl.Replace(path)
}
