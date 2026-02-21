package parser

import (
	"go/ast"
	"strings"
)

func (s *parserState) resolveFuncByName(currentPkg, name string) *funcMeta {
	if name == "" {
		return nil
	}
	if fm := s.funcsByPkg[currentPkg][name]; fm != nil {
		return fm
	}
	var found *funcMeta
	for _, byName := range s.funcsByPkg {
		fm := byName[name]
		if fm == nil {
			continue
		}
		if found != nil && found.pkg != fm.pkg {
			return nil
		}
		found = fm
	}
	return found
}

func (s *parserState) resolveFuncInScope(file *fileCtx, currentPkg, name string) *funcMeta {
	if name == "" {
		return nil
	}
	if file != nil && file.importPath != "" {
		if byName := s.funcsByImportPath[file.importPath]; byName != nil {
			if fm := byName[name]; fm != nil {
				return fm
			}
		}
	}
	return s.resolveFuncByName(currentPkg, name)
}

func inferAuthSchemesFromNames(names []string) []string {
	out := make([]string, 0, 2)
	for _, n := range names {
		v := strings.ToLower(n)
		switch {
		case strings.Contains(v, "session"), strings.Contains(v, "userauth"), strings.Contains(v, "adminauth"):
			out = append(out, "cookie:session")
		case strings.Contains(v, "clientauth"), strings.Contains(v, "apikey"), strings.Contains(v, "keyauth"):
			out = append(out, "header:Authorization")
		case strings.Contains(v, "jwt"), strings.Contains(v, "bearer"), strings.Contains(v, "token"):
			out = append(out, "bearer")
		}
	}
	return dedupeStrings(out)
}

func inferAuthSchemesFromMiddlewareBody(fm *funcMeta) []string {
	if fm == nil || fm.decl == nil || fm.decl.Body == nil {
		return nil
	}
	out := make([]string, 0, 2)
	ast.Inspect(fm.decl.Body, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		if sessName, ok := parseSessionGetCall(call); ok {
			out = append(out, "cookie:"+sessName)
			return true
		}
		if p, ok := parseParameterCall(call); ok && p.in == "header" {
			out = append(out, "header:"+p.name)
			return true
		}
		return true
	})
	if len(out) == 0 {
		out = inferAuthSchemesFromNames([]string{fm.name})
	}
	return dedupeStrings(out)
}

func parseSessionGetCall(call *ast.CallExpr) (string, bool) {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok || sel.Sel.Name != "Get" || len(call.Args) < 2 {
		return "", false
	}
	xid, ok := sel.X.(*ast.Ident)
	if !ok || xid.Name != "session" {
		return "", false
	}
	name, ok := stringLiteral(call.Args[0])
	if !ok || name == "" {
		return "session", true
	}
	return name, true
}
