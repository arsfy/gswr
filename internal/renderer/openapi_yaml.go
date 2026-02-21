package renderer

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/arsfy/gswr/internal/model"
	"gopkg.in/yaml.v3"
)

type openAPIDoc struct {
	OpenAPI    string              `yaml:"openapi" json:"openapi"`
	Info       info                `yaml:"info" json:"info"`
	Servers    []serverObject      `yaml:"servers,omitempty" json:"servers,omitempty"`
	Tags       []tagObject         `yaml:"tags,omitempty" json:"tags,omitempty"`
	Paths      map[string]pathItem `yaml:"paths" json:"paths"`
	Components components          `yaml:"components,omitempty" json:"components,omitempty"`
}

type info struct {
	Title       string `yaml:"title" json:"title"`
	Version     string `yaml:"version" json:"version"`
	Description string `yaml:"description,omitempty" json:"description,omitempty"`
}

type serverObject struct {
	URL string `yaml:"url" json:"url"`
}

type components struct {
	Schemas         map[string]schemaObject   `yaml:"schemas,omitempty" json:"schemas,omitempty"`
	SecuritySchemes map[string]securityScheme `yaml:"securitySchemes,omitempty" json:"securitySchemes,omitempty"`
}

type pathItem struct {
	Get    *operation `yaml:"get,omitempty" json:"get,omitempty"`
	Post   *operation `yaml:"post,omitempty" json:"post,omitempty"`
	Put    *operation `yaml:"put,omitempty" json:"put,omitempty"`
	Patch  *operation `yaml:"patch,omitempty" json:"patch,omitempty"`
	Delete *operation `yaml:"delete,omitempty" json:"delete,omitempty"`
}

type operation struct {
	OperationID  string                `yaml:"operationId,omitempty" json:"operationId,omitempty"`
	Summary      string                `yaml:"summary,omitempty" json:"summary,omitempty"`
	Description  string                `yaml:"description,omitempty" json:"description,omitempty"`
	Tags         []string              `yaml:"tags,omitempty" json:"tags,omitempty"`
	Security     []map[string][]string `yaml:"security,omitempty" json:"security,omitempty"`
	XMiddlewares []string              `yaml:"x-middlewares,omitempty" json:"x-middlewares,omitempty"`
	Parameters   []parameterObject     `yaml:"parameters,omitempty" json:"parameters,omitempty"`
	RequestBody  *requestBodyObject    `yaml:"requestBody,omitempty" json:"requestBody,omitempty"`
	Responses    map[string]response   `yaml:"responses" json:"responses"`
}

type tagObject struct {
	Name        string `yaml:"name" json:"name"`
	Description string `yaml:"description,omitempty" json:"description,omitempty"`
}

type securityScheme struct {
	Type         string `yaml:"type" json:"type"`
	In           string `yaml:"in,omitempty" json:"in,omitempty"`
	Name         string `yaml:"name,omitempty" json:"name,omitempty"`
	Scheme       string `yaml:"scheme,omitempty" json:"scheme,omitempty"`
	BearerFormat string `yaml:"bearerFormat,omitempty" json:"bearerFormat,omitempty"`
}

type parameterObject struct {
	Name        string       `yaml:"name" json:"name"`
	In          string       `yaml:"in" json:"in"`
	Required    bool         `yaml:"required,omitempty" json:"required,omitempty"`
	Description string       `yaml:"description,omitempty" json:"description,omitempty"`
	Schema      schemaObject `yaml:"schema" json:"schema"`
}

type requestBodyObject struct {
	Required bool        `yaml:"required,omitempty" json:"required,omitempty"`
	Content  contentType `yaml:"content" json:"content"`
}

type response struct {
	Description string       `yaml:"description" json:"description"`
	Content     *contentType `yaml:"content,omitempty" json:"content,omitempty"`
}

type contentType struct {
	ApplicationJSON mediaType `yaml:"application/json" json:"application/json"`
}

type mediaType struct {
	Schema schemaObject `yaml:"schema" json:"schema"`
}

type schemaObject struct {
	Ref                  string                  `yaml:"$ref,omitempty" json:"$ref,omitempty"`
	Type                 string                  `yaml:"type,omitempty" json:"type,omitempty"`
	Properties           map[string]schemaObject `yaml:"properties,omitempty" json:"properties,omitempty"`
	Required             []string                `yaml:"required,omitempty" json:"required,omitempty"`
	Enum                 []any                   `yaml:"enum,omitempty" json:"enum,omitempty"`
	Items                *schemaObject           `yaml:"items,omitempty" json:"items,omitempty"`
	AdditionalProperties *schemaObject           `yaml:"additionalProperties,omitempty" json:"additionalProperties,omitempty"`
}

func WriteYAML(ir *model.IR, outPath string) error {
	doc := buildDoc(ir)
	b, err := yaml.Marshal(doc)
	if err != nil {
		return err
	}
	return writeOutput(outPath, b)
}

func WriteJSON(ir *model.IR, outPath string) error {
	doc := buildDoc(ir)
	b, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return err
	}
	b = append(b, '\n')
	return writeOutput(outPath, b)
}

func buildDoc(ir *model.IR) openAPIDoc {
	doc := openAPIDoc{
		OpenAPI: "3.0.3",
		Info: info{
			Title:       ir.Title,
			Version:     ir.Version,
			Description: ir.Description,
		},
		Paths: map[string]pathItem{},
	}
	if len(ir.Servers) > 0 {
		doc.Servers = make([]serverObject, 0, len(ir.Servers))
		for _, s := range ir.Servers {
			doc.Servers = append(doc.Servers, serverObject{URL: s})
		}
	}

	if len(ir.Components) > 0 {
		doc.Components.Schemas = map[string]schemaObject{}
		keys := make([]string, 0, len(ir.Components))
		for k := range ir.Components {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			doc.Components.Schemas[k] = toSchema(*ir.Components[k])
		}
	}
	if len(ir.Tags) > 0 {
		doc.Tags = make([]tagObject, 0, len(ir.Tags))
		for _, t := range ir.Tags {
			doc.Tags = append(doc.Tags, tagObject{Name: t.Name, Description: t.Description})
		}
	}

	usedSchemes := map[string]securityScheme{}
	for _, r := range ir.Routes {
		ids, defs := resolveSecuritySchemes(r.AuthRequired, r.AuthSchemes)
		for _, id := range ids {
			if def, ok := defs[id]; ok {
				usedSchemes[id] = def
			}
		}
	}
	if len(usedSchemes) > 0 {
		doc.Components.SecuritySchemes = usedSchemes
	}

	for _, r := range ir.Routes {
		item := doc.Paths[r.Path]
		opResponses := map[string]response{}
		for _, rr := range r.Responses {
			respSchema := model.Schema{Type: "object"}
			if rr.Schema != nil {
				respSchema = *rr.Schema
			}
			desc := rr.Description
			if desc == "" {
				desc = "Response"
			}
			opResponses[itoaOrDefault(rr.StatusCode)] = response{
				Description: desc,
				Content:     &contentType{ApplicationJSON: mediaType{Schema: toSchema(respSchema)}},
			}
		}
		if len(opResponses) == 0 {
			opResponses["200"] = response{
				Description: "OK",
				Content:     &contentType{ApplicationJSON: mediaType{Schema: toSchema(model.Schema{Type: "object"})}},
			}
		}
		op := &operation{
			OperationID:  r.OperationID,
			Summary:      r.Summary,
			Description:  r.Description,
			Tags:         r.Tags,
			XMiddlewares: r.Middlewares,
			Responses:    opResponses,
		}
		if ids, _ := resolveSecuritySchemes(r.AuthRequired, r.AuthSchemes); len(ids) > 0 {
			security := make([]map[string][]string, 0, len(ids))
			for _, id := range ids {
				security = append(security, map[string][]string{id: []string{}})
			}
			op.Security = security
		}
		if len(r.Parameters) > 0 {
			op.Parameters = make([]parameterObject, 0, len(r.Parameters))
			for _, p := range r.Parameters {
				paramSchema := model.Schema{Type: "string"}
				if p.Schema != nil {
					paramSchema = *p.Schema
				}
				op.Parameters = append(op.Parameters, parameterObject{
					Name:        p.Name,
					In:          p.In,
					Required:    p.Required,
					Description: p.Description,
					Schema:      toSchema(paramSchema),
				})
			}
		}
		if r.RequestBody != nil {
			op.RequestBody = &requestBodyObject{
				Required: true,
				Content:  contentType{ApplicationJSON: mediaType{Schema: toSchema(*r.RequestBody)}},
			}
		}

		switch r.Method {
		case "GET":
			item.Get = op
		case "POST":
			item.Post = op
		case "PUT":
			item.Put = op
		case "PATCH":
			item.Patch = op
		case "DELETE":
			item.Delete = op
		}
		doc.Paths[r.Path] = item
	}
	return doc
}

func writeOutput(outPath string, b []byte) error {
	if err := os.MkdirAll(filepath.Dir(outPath), 0o755); err != nil {
		return err
	}
	return os.WriteFile(outPath, b, 0o644)
}

func toSchema(s model.Schema) schemaObject {
	if s.Ref != "" {
		return schemaObject{Ref: s.Ref}
	}
	out := schemaObject{Type: s.Type}
	if len(s.Properties) > 0 {
		out.Properties = map[string]schemaObject{}
		keys := make([]string, 0, len(s.Properties))
		for k := range s.Properties {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			out.Properties[k] = toSchema(*s.Properties[k])
		}
	}
	if len(s.Required) > 0 {
		out.Required = append([]string(nil), s.Required...)
		sort.Strings(out.Required)
	}
	if len(s.Enum) > 0 {
		out.Enum = append([]any(nil), s.Enum...)
	}
	if s.Items != nil {
		it := toSchema(*s.Items)
		out.Items = &it
	}
	if s.AdditionalProperties != nil {
		ap := toSchema(*s.AdditionalProperties)
		out.AdditionalProperties = &ap
	}
	return out
}

func resolveSecuritySchemes(authRequired bool, raw []string) ([]string, map[string]securityScheme) {
	defs := map[string]securityScheme{}
	ids := make([]string, 0, 2)
	add := func(id string, sc securityScheme) {
		if id == "" {
			return
		}
		if _, ok := defs[id]; !ok {
			defs[id] = sc
			ids = append(ids, id)
		}
	}

	for _, item := range raw {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		switch {
		case item == "bearer":
			add("bearerAuth", securityScheme{
				Type:         "http",
				Scheme:       "bearer",
				BearerFormat: "JWT",
			})
		case strings.HasPrefix(item, "cookie:"):
			name := strings.TrimSpace(strings.TrimPrefix(item, "cookie:"))
			if name == "" {
				name = "session"
			}
			add("cookie_"+sanitizeSchemeID(name), securityScheme{
				Type: "apiKey",
				In:   "cookie",
				Name: name,
			})
		case strings.HasPrefix(item, "header:"):
			name := strings.TrimSpace(strings.TrimPrefix(item, "header:"))
			if name == "" {
				name = "Authorization"
			}
			add("header_"+sanitizeSchemeID(name), securityScheme{
				Type: "apiKey",
				In:   "header",
				Name: name,
			})
		}
	}

	if len(ids) == 0 && authRequired {
		add("bearerAuth", securityScheme{
			Type:         "http",
			Scheme:       "bearer",
			BearerFormat: "JWT",
		})
	}
	sort.Strings(ids)
	return ids, defs
}

func sanitizeSchemeID(s string) string {
	if s == "" {
		return "auth"
	}
	var b strings.Builder
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
			continue
		}
		b.WriteByte('_')
	}
	out := strings.Trim(b.String(), "_")
	if out == "" {
		return "auth"
	}
	return out
}

func itoaOrDefault(code int) string {
	if code <= 0 {
		return "200"
	}
	return strconv.Itoa(code)
}
