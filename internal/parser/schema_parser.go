package parser

import (
	"go/ast"
	"go/token"
	"path/filepath"
	"sort"
	"strconv"

	"github.com/arsfy/gswr/internal/model"
)

func (s *parserState) schemaFromExpr(pkg string, file *fileCtx, expr ast.Expr) *model.Schema {
	return s.schemaFromExprWithVars(pkg, file, expr, nil)
}

func (s *parserState) schemaFromExprWithVars(pkg string, file *fileCtx, expr ast.Expr, varTypes map[string]ast.Expr) *model.Schema {
	return s.schemaFromExprWithVarsAndBindings(pkg, file, expr, varTypes, nil)
}

func (s *parserState) schemaFromExprWithVarsAndBindings(pkg string, file *fileCtx, expr ast.Expr, varTypes map[string]ast.Expr, bindings map[string]ast.Expr) *model.Schema {
	switch n := expr.(type) {
	case *ast.CompositeLit:
		return s.schemaFromCompositeLit(pkg, file, n, varTypes, bindings)
	case *ast.BasicLit:
		return primitiveSchemaFromBasicLit(n)
	case *ast.Ident:
		if bindings != nil {
			if bound, ok := bindings[n.Name]; ok && bound != nil && bound != expr {
				return s.schemaFromExprWithVarsAndBindings(pkg, file, bound, varTypes, bindings)
			}
		}
		if n.Name == "nil" {
			return &model.Schema{Type: "object"}
		}
		if n.Name == "true" || n.Name == "false" {
			return &model.Schema{Type: "boolean", Enum: []any{n.Name == "true"}}
		}
		if varTypes != nil {
			if t, ok := varTypes[n.Name]; ok {
				return s.schemaFromTypeExpr(pkg, file, t)
			}
		}
		if meta, ok := s.resolveNamedTypeInScope(pkg, file, n.Name); ok {
			return s.schemaFromNamedMeta(meta)
		}
		return &model.Schema{Type: "object"}
	case *ast.SelectorExpr:
		if resolved := s.schemaFromSelectorValue(pkg, file, n, varTypes); resolved != nil {
			return resolved
		}
		return &model.Schema{Type: "object"}
	default:
		return &model.Schema{Type: "object"}
	}
}

func (s *parserState) schemaFromSelectorValue(pkg string, file *fileCtx, sel *ast.SelectorExpr, varTypes map[string]ast.Expr) *model.Schema {
	resolvedPkg, resolvedFile, resolvedType, ok := s.resolveExprTypeWithVars(pkg, file, sel, varTypes)
	if !ok {
		return nil
	}
	return s.schemaFromTypeExpr(resolvedPkg, resolvedFile, resolvedType)
}

func (s *parserState) resolveExprTypeWithVars(pkg string, file *fileCtx, expr ast.Expr, varTypes map[string]ast.Expr) (string, *fileCtx, ast.Expr, bool) {
	switch n := expr.(type) {
	case *ast.Ident:
		if varTypes != nil {
			if t, ok := varTypes[n.Name]; ok {
				return pkg, file, t, true
			}
		}
		if meta, ok := s.resolveNamedTypeInScope(pkg, file, n.Name); ok {
			return meta.pkg, meta.file, meta.typeExpr, true
		}
		return "", nil, nil, false
	case *ast.SelectorExpr:
		// Imported named type, e.g. `types.Response`.
		if xid, ok := n.X.(*ast.Ident); ok && file != nil {
			if importPath := file.imports[xid.Name]; importPath != "" {
				targetPkg := filepath.Base(importPath)
				if meta := s.namedTypesByPkg[targetPkg][n.Sel.Name]; meta != nil {
					return meta.pkg, meta.file, meta.typeExpr, true
				}
			}
		}
		// Field access chain, e.g. `input.Page`.
		return s.resolveSelectorFieldType(pkg, file, n, varTypes)
	case *ast.StarExpr:
		return s.resolveExprTypeWithVars(pkg, file, n.X, varTypes)
	default:
		return "", nil, nil, false
	}
}

func (s *parserState) resolveSelectorFieldType(pkg string, file *fileCtx, sel *ast.SelectorExpr, varTypes map[string]ast.Expr) (string, *fileCtx, ast.Expr, bool) {
	basePkg, baseFile, baseType, ok := s.resolveExprTypeWithVars(pkg, file, sel.X, varTypes)
	if !ok {
		return "", nil, nil, false
	}
	resolvedPkg, resolvedFile, st := s.resolveStructType(basePkg, baseFile, baseType)
	if st == nil {
		return "", nil, nil, false
	}
	for _, f := range st.Fields.List {
		if len(f.Names) == 0 {
			continue
		}
		if f.Names[0].Name == sel.Sel.Name {
			return resolvedPkg, resolvedFile, f.Type, true
		}
	}
	return "", nil, nil, false
}

func (s *parserState) schemaFromCompositeLit(pkg string, file *fileCtx, lit *ast.CompositeLit, varTypes map[string]ast.Expr, bindings map[string]ast.Expr) *model.Schema {
	if mt, ok := lit.Type.(*ast.MapType); ok {
		return s.schemaFromMapLiteral(pkg, file, mt, lit.Elts, varTypes, bindings)
	}
	if st, ok := lit.Type.(*ast.StructType); ok {
		return s.schemaFromStructLiteral(pkg, file, st, lit.Elts, varTypes, bindings)
	}

	resolvedPkg, resolvedFile, st := s.resolveStructType(pkg, file, lit.Type)
	if st != nil {
		return s.schemaFromStructLiteral(resolvedPkg, resolvedFile, st, lit.Elts, varTypes, bindings)
	}
	if keyed := s.schemaFromKeyedObjectLiteral(pkg, file, lit.Elts, varTypes, bindings); keyed != nil {
		return keyed
	}
	return s.schemaFromTypeExpr(pkg, file, lit.Type)
}

func (s *parserState) schemaFromStructLiteral(pkg string, file *fileCtx, st *ast.StructType, elts []ast.Expr, varTypes map[string]ast.Expr, bindings map[string]ast.Expr) *model.Schema {
	base := s.schemaFromStruct(pkg, file, st)
	if len(elts) == 0 {
		return base
	}

	fieldMap := structFieldMeta(st)
	present := map[string]bool{}
	allKeyed := true
	for _, elt := range elts {
		kv, ok := elt.(*ast.KeyValueExpr)
		if !ok {
			allKeyed = false
			return base
		}
		key, ok := kv.Key.(*ast.Ident)
		if !ok {
			continue
		}
		fm, ok := fieldMap[key.Name]
		if !ok {
			continue
		}
		present[fm.jsonName] = true
		resolvedVal := kv.Value
		if id, ok := kv.Value.(*ast.Ident); ok {
			if bindings != nil {
				if bound, ok := bindings[id.Name]; ok && bound != nil {
					resolvedVal = bound
				}
			}
		}
		if id, ok := resolvedVal.(*ast.Ident); ok && id.Name == "nil" && fm.omitempty {
			delete(base.Properties, fm.jsonName)
			base.Required = removeRequiredField(base.Required, fm.jsonName)
			continue
		}
		literal := s.schemaFromLiteralValueExpr(pkg, file, resolvedVal, varTypes, bindings)
		declared := s.schemaFromTypeExpr(pkg, file, fm.typ)
		base.Properties[fm.jsonName] = mergeLiteralSchema(declared, literal)
	}
	if allKeyed {
		for _, fm := range fieldMap {
			if fm.omitempty && !present[fm.jsonName] {
				delete(base.Properties, fm.jsonName)
				base.Required = removeRequiredField(base.Required, fm.jsonName)
			}
		}
	}
	return base
}

type fieldMeta struct {
	jsonName  string
	typ       ast.Expr
	omitempty bool
}

func structFieldMeta(st *ast.StructType) map[string]fieldMeta {
	out := map[string]fieldMeta{}
	for _, f := range st.Fields.List {
		if len(f.Names) == 0 {
			continue
		}
		name := f.Names[0].Name
		jsonName, ignore := jsonFieldName(name, f.Tag)
		if ignore {
			continue
		}
		out[name] = fieldMeta{jsonName: jsonName, typ: f.Type, omitempty: jsonHasOmitEmpty(f.Tag)}
	}
	return out
}

func removeRequiredField(fields []string, target string) []string {
	if len(fields) == 0 {
		return fields
	}
	out := make([]string, 0, len(fields))
	for _, f := range fields {
		if f != target {
			out = append(out, f)
		}
	}
	return out
}

func mergeLiteralSchema(declared, literal *model.Schema) *model.Schema {
	if literal == nil {
		return declared
	}
	if declared == nil {
		return literal
	}
	if len(literal.Enum) > 0 {
		if literal.Type == "" {
			literal.Type = declared.Type
		}
		return literal
	}
	if literal.Type == "object" && len(literal.Properties) == 0 && literal.AdditionalProperties == nil && declared.Type != "object" {
		return declared
	}
	if literal.Type == "object" && literal.Ref == "" && (len(literal.Properties) > 0 || literal.AdditionalProperties != nil) {
		return literal
	}
	return preferSchema(declared, literal)
}

func primitiveSchemaFromBasicLit(n *ast.BasicLit) *model.Schema {
	switch n.Kind {
	case token.STRING:
		v, err := strconv.Unquote(n.Value)
		if err == nil {
			return &model.Schema{Type: "string", Enum: []any{v}}
		}
		return &model.Schema{Type: "string"}
	case token.INT, token.FLOAT:
		if v, err := strconv.ParseFloat(n.Value, 64); err == nil {
			return &model.Schema{Type: "number", Enum: []any{v}}
		}
		return &model.Schema{Type: "number"}
	default:
		return &model.Schema{Type: "string"}
	}
}

func (s *parserState) schemaFromMapLiteral(pkg string, file *fileCtx, mt *ast.MapType, elts []ast.Expr, varTypes map[string]ast.Expr, bindings map[string]ast.Expr) *model.Schema {
	fallback := &model.Schema{Type: "object", AdditionalProperties: s.schemaFromTypeExpr(pkg, file, mt.Value)}
	if len(elts) == 0 {
		return fallback
	}

	props := map[string]*model.Schema{}
	required := make([]string, 0, len(elts))
	for _, elt := range elts {
		kv, ok := elt.(*ast.KeyValueExpr)
		if !ok {
			return fallback
		}
		key, ok := stringLiteral(kv.Key)
		if !ok || key == "" {
			return fallback
		}
		props[key] = s.schemaFromLiteralValueExpr(pkg, file, kv.Value, varTypes, bindings)
		required = append(required, key)
	}
	return &model.Schema{Type: "object", Properties: props, Required: dedupeSorted(required)}
}

func (s *parserState) schemaFromKeyedObjectLiteral(pkg string, file *fileCtx, elts []ast.Expr, varTypes map[string]ast.Expr, bindings map[string]ast.Expr) *model.Schema {
	if len(elts) == 0 {
		return nil
	}
	props := map[string]*model.Schema{}
	required := make([]string, 0, len(elts))
	for _, elt := range elts {
		kv, ok := elt.(*ast.KeyValueExpr)
		if !ok {
			return nil
		}
		key, ok := stringLiteral(kv.Key)
		if !ok || key == "" {
			return nil
		}
		props[key] = s.schemaFromLiteralValueExpr(pkg, file, kv.Value, varTypes, bindings)
		required = append(required, key)
	}
	return &model.Schema{Type: "object", Properties: props, Required: dedupeSorted(required)}
}

func (s *parserState) schemaFromLiteralValueExpr(pkg string, file *fileCtx, expr ast.Expr, varTypes map[string]ast.Expr, bindings map[string]ast.Expr) *model.Schema {
	switch n := expr.(type) {
	case *ast.BasicLit:
		return primitiveSchemaFromBasicLit(n)
	case *ast.Ident:
		if n.Name == "true" || n.Name == "false" {
			return &model.Schema{Type: "boolean", Enum: []any{n.Name == "true"}}
		}
		return s.schemaFromExprWithVarsAndBindings(pkg, file, n, varTypes, bindings)
	case *ast.CompositeLit:
		return s.schemaFromExprWithVarsAndBindings(pkg, file, n, varTypes, bindings)
	default:
		return s.schemaFromExprWithVarsAndBindings(pkg, file, expr, varTypes, bindings)
	}
}

func (s *parserState) schemaFromNamedType(pkg, typeName string) *model.Schema {
	meta := s.namedTypesByPkg[pkg][typeName]
	if meta == nil {
		return &model.Schema{Type: "object"}
	}
	return s.schemaFromNamedMeta(meta)
}

func (s *parserState) schemaFromNamedMeta(meta *namedTypeMeta) *model.Schema {
	if meta == nil {
		return &model.Schema{Type: "object"}
	}
	comp := componentName(meta.pkg, meta.name)
	if _, ok := s.components[comp]; ok {
		return &model.Schema{Ref: "#/components/schemas/" + comp}
	}
	if s.buildingComponentRef[comp] {
		return &model.Schema{Ref: "#/components/schemas/" + comp}
	}

	s.buildingComponentRef[comp] = true
	s.components[comp] = s.schemaFromTypeExpr(meta.pkg, meta.file, meta.typeExpr)
	delete(s.buildingComponentRef, comp)
	return &model.Schema{Ref: "#/components/schemas/" + comp}
}

func (s *parserState) schemaFromStruct(pkg string, file *fileCtx, st *ast.StructType) *model.Schema {
	props := map[string]*model.Schema{}
	required := make([]string, 0, 2)
	for _, f := range st.Fields.List {
		if len(f.Names) == 0 {
			continue
		}
		name := f.Names[0].Name
		jsonName, ignore := jsonFieldName(name, f.Tag)
		if ignore {
			continue
		}
		props[jsonName] = s.schemaFromTypeExpr(pkg, file, f.Type)
		if fieldRequired(f.Tag) {
			required = append(required, jsonName)
		}
	}
	return &model.Schema{Type: "object", Properties: props, Required: dedupeSorted(required)}
}

func (s *parserState) schemaFromTypeExpr(pkg string, file *fileCtx, t ast.Expr) *model.Schema {
	switch n := t.(type) {
	case *ast.Ident:
		switch n.Name {
		case "string":
			return &model.Schema{Type: "string"}
		case "bool":
			return &model.Schema{Type: "boolean"}
		case "int", "int8", "int16", "int32", "int64", "uint", "uint8", "uint16", "uint32", "uint64", "float32", "float64":
			return &model.Schema{Type: "number"}
		case "any", "interface{}":
			return &model.Schema{Type: "object"}
		default:
			if meta, ok := s.resolveNamedTypeInScope(pkg, file, n.Name); ok {
				return s.schemaFromNamedMeta(meta)
			}
			if meta, ok := s.resolveNamedTypeAnyPkg(n.Name); ok {
				return s.schemaFromNamedMeta(meta)
			}
			return &model.Schema{Type: "object"}
		}
	case *ast.ArrayType:
		return &model.Schema{Type: "array", Items: s.schemaFromTypeExpr(pkg, file, n.Elt)}
	case *ast.MapType:
		return &model.Schema{Type: "object", AdditionalProperties: s.schemaFromTypeExpr(pkg, file, n.Value)}
	case *ast.StructType:
		return s.schemaFromStruct(pkg, file, n)
	case *ast.SelectorExpr:
		if file != nil {
			if alias, ok := n.X.(*ast.Ident); ok {
				if importPath := file.imports[alias.Name]; importPath != "" {
					if byName := s.namedTypesByImport[importPath]; byName != nil {
						if meta := byName[n.Sel.Name]; meta != nil {
							return s.schemaFromNamedMeta(meta)
						}
					}
					return s.schemaFromNamedType(filepath.Base(importPath), n.Sel.Name)
				}
			}
		}
		if meta, ok := s.resolveNamedTypeAnyPkg(n.Sel.Name); ok {
			return s.schemaFromNamedMeta(meta)
		}
		return &model.Schema{Type: "object"}
	case *ast.StarExpr:
		return s.schemaFromTypeExpr(pkg, file, n.X)
	default:
		return &model.Schema{Type: "object"}
	}
}

func (s *parserState) resolveNamedTypeAnyPkg(typeName string) (*namedTypeMeta, bool) {
	var found *namedTypeMeta
	for _, byName := range s.namedTypesByPkg {
		meta := byName[typeName]
		if meta == nil {
			continue
		}
		if found != nil {
			fp := ""
			mp := ""
			if found.file != nil {
				fp = found.file.importPath
			}
			if meta.file != nil {
				mp = meta.file.importPath
			}
			if fp != mp {
				return nil, false
			}
		}
		found = meta
	}
	if found == nil {
		return nil, false
	}
	return found, true
}

func (s *parserState) resolveNamedTypeInScope(pkg string, file *fileCtx, typeName string) (*namedTypeMeta, bool) {
	if file != nil && file.importPath != "" {
		if byName := s.namedTypesByImport[file.importPath]; byName != nil {
			if meta := byName[typeName]; meta != nil {
				return meta, true
			}
		}
	}
	if meta := s.namedTypesByPkg[pkg][typeName]; meta != nil {
		if file == nil || meta.file == nil || meta.file.importPath == file.importPath {
			return meta, true
		}
	}
	if found, ok := s.resolveNamedTypeAnyPkg(typeName); ok {
		return found, true
	}
	return nil, false
}

func mergeParameters(pathParamNames []string, parsed []model.Parameter) []model.Parameter {
	params := map[string]model.Parameter{}
	for _, name := range pathParamNames {
		mergeParameter(params, model.Parameter{Name: name, In: "path", Required: true, Schema: &model.Schema{Type: "string"}})
	}
	for _, p := range parsed {
		mergeParameter(params, p)
	}

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
	return out
}

func mergeParameter(params map[string]model.Parameter, p model.Parameter) {
	key := p.In + ":" + p.Name
	existing, ok := params[key]
	if !ok {
		params[key] = p
		return
	}
	existing.Required = existing.Required || p.Required
	if existing.Description == "" {
		existing.Description = p.Description
	}
	existing.Schema = preferSchema(existing.Schema, p.Schema)
	params[key] = existing
}

func preferSchema(oldSchema, newSchema *model.Schema) *model.Schema {
	if oldSchema == nil {
		return newSchema
	}
	if newSchema == nil {
		return oldSchema
	}
	if oldSchema.Type == "string" && newSchema.Type != "string" {
		return newSchema
	}
	if oldSchema.Type == "object" && newSchema.Type != "object" {
		return newSchema
	}
	if oldSchema.Type == "" {
		return newSchema
	}
	return oldSchema
}

func paramOrder(in string) int {
	switch in {
	case "path":
		return 0
	case "query":
		return 1
	case "header":
		return 2
	case "cookie":
		return 3
	default:
		return 4
	}
}
