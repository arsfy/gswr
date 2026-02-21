package parser

import (
	"go/ast"
	"go/token"
	"sort"
	"strconv"
	"strings"
)

func normalizeEchoPath(path string) (string, []string) {
	parts := strings.Split(path, "/")
	params := make([]string, 0, 2)
	for i, part := range parts {
		if part == "" {
			continue
		}
		if strings.HasPrefix(part, ":") {
			name := strings.TrimPrefix(part, ":")
			if name == "" {
				name = "param"
			}
			parts[i] = "{" + name + "}"
			params = append(params, name)
			continue
		}
		if strings.HasPrefix(part, "*") {
			name := strings.TrimPrefix(part, "*")
			if name == "" {
				name = "wildcard"
			}
			parts[i] = "{" + name + "}"
			params = append(params, name)
		}
	}
	if len(parts) == 0 || parts[0] != "" {
		return "/" + strings.Join(parts, "/"), params
	}
	return strings.Join(parts, "/"), params
}

func componentName(pkg, typeName string) string {
	if pkg == "main" {
		return typeName
	}
	return pkg + "_" + typeName
}

func jsonFieldName(defaultName string, tag *ast.BasicLit) (string, bool) {
	if tag == nil {
		return lowerFirst(defaultName), false
	}
	jsonTag, ok := tagLookup(tag, "json")
	if !ok {
		return lowerFirst(defaultName), false
	}
	if jsonTag == "-" {
		return "", true
	}
	if jsonTag == "" {
		return lowerFirst(defaultName), false
	}
	return jsonTag, false
}

func jsonHasOmitEmpty(tag *ast.BasicLit) bool {
	raw, ok := tagLookupRaw(tag, "json")
	if !ok {
		return false
	}
	for _, item := range strings.Split(raw, ",") {
		if strings.TrimSpace(item) == "omitempty" {
			return true
		}
	}
	return false
}

func tagLookup(tag *ast.BasicLit, key string) (string, bool) {
	raw, ok := tagLookupRaw(tag, key)
	if !ok {
		return "", false
	}
	items := strings.Split(raw, ",")
	if len(items) == 0 {
		return "", true
	}
	return items[0], true
}

func tagLookupRaw(tag *ast.BasicLit, key string) (string, bool) {
	if tag == nil {
		return "", false
	}
	tagRaw, err := strconv.Unquote(tag.Value)
	if err != nil {
		return "", false
	}
	parts := strings.Split(tagRaw, " ")
	prefix := key + ":"
	for _, p := range parts {
		if !strings.HasPrefix(p, prefix) {
			continue
		}
		v, err := strconv.Unquote(strings.TrimPrefix(p, prefix))
		if err != nil {
			return "", false
		}
		return v, true
	}
	return "", false
}

func fieldRequired(tag *ast.BasicLit) bool {
	for _, key := range []string{"validate", "binding"} {
		v, ok := tagLookupRaw(tag, key)
		if !ok {
			continue
		}
		for _, item := range strings.Split(v, ",") {
			if strings.TrimSpace(item) == "required" {
				return true
			}
		}
	}
	return false
}

func dedupeSorted(in []string) []string {
	if len(in) == 0 {
		return nil
	}
	set := map[string]bool{}
	for _, v := range in {
		if v == "" {
			continue
		}
		set[v] = true
	}
	if len(set) == 0 {
		return nil
	}
	out := make([]string, 0, len(set))
	for v := range set {
		out = append(out, v)
	}
	sort.Strings(out)
	return out
}

func lowerFirst(s string) string {
	if s == "" {
		return s
	}
	return strings.ToLower(s[:1]) + s[1:]
}

func isEchoGroupType(t ast.Expr) bool {
	star, ok := t.(*ast.StarExpr)
	if !ok {
		return false
	}
	sel, ok := star.X.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	id, ok := sel.X.(*ast.Ident)
	if !ok {
		return false
	}
	return id.Name == "echo" && sel.Sel.Name == "Group"
}

func stringLiteral(expr ast.Expr) (string, bool) {
	b, ok := expr.(*ast.BasicLit)
	if !ok || b.Kind != token.STRING {
		return "", false
	}
	v, err := strconv.Unquote(b.Value)
	if err != nil {
		return "", false
	}
	return v, true
}

func intLiteral(expr ast.Expr) (int, bool) {
	b, ok := expr.(*ast.BasicLit)
	if !ok || b.Kind != token.INT {
		return 0, false
	}
	v, err := strconv.Atoi(b.Value)
	if err != nil {
		return 0, false
	}
	return v, true
}

func joinPath(prefix, suffix string) string {
	if prefix == "" {
		if strings.HasPrefix(suffix, "/") {
			return suffix
		}
		return "/" + suffix
	}
	left := strings.TrimRight(prefix, "/")
	right := strings.TrimLeft(suffix, "/")
	return left + "/" + right
}

func cloneGroupStateMap(in map[string]groupState) map[string]groupState {
	out := map[string]groupState{}
	for k, v := range in {
		if len(v.middlewares) > 0 {
			v.middlewares = append([]string(nil), v.middlewares...)
		}
		out[k] = v
	}
	return out
}

func resolveGroupState(expr ast.Expr, env map[string]groupState) (groupState, bool) {
	switch n := expr.(type) {
	case *ast.Ident:
		v, ok := env[n.Name]
		return v, ok
	case *ast.CallExpr:
		sel, ok := n.Fun.(*ast.SelectorExpr)
		if !ok || sel.Sel.Name != "Group" || len(n.Args) == 0 {
			return groupState{}, false
		}
		base, ok := resolveGroupState(sel.X, env)
		if !ok {
			return groupState{}, false
		}
		p, ok := stringLiteral(n.Args[0])
		if !ok {
			return groupState{}, false
		}
		return groupState{
			prefix:       joinPath(base.prefix, p),
			authRequired: base.authRequired,
		}, true
	default:
		return groupState{}, false
	}
}

func inlineCommentForNode(fset *token.FileSet, f *ast.File, n ast.Node) string {
	if fset == nil || f == nil || n == nil {
		return ""
	}
	line := fset.Position(n.End()).Line
	for _, cg := range f.Comments {
		if len(cg.List) != 1 {
			continue
		}
		c := cg.List[0]
		if !strings.HasPrefix(c.Text, "//") {
			continue
		}
		if fset.Position(c.Pos()).Line != line {
			continue
		}
		return strings.TrimSpace(strings.TrimPrefix(c.Text, "//"))
	}
	return ""
}
