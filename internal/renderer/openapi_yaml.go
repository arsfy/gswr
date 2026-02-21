package renderer

import (
	"os"
	"path/filepath"
	"sort"
	"strconv"

	"golang-openapi/internal/model"
	"gopkg.in/yaml.v3"
)

type openAPIDoc struct {
	OpenAPI    string              `yaml:"openapi"`
	Info       info                `yaml:"info"`
	Servers    []serverObject      `yaml:"servers,omitempty"`
	Tags       []tagObject         `yaml:"tags,omitempty"`
	Paths      map[string]pathItem `yaml:"paths"`
	Components components          `yaml:"components,omitempty"`
}

type info struct {
	Title       string `yaml:"title"`
	Version     string `yaml:"version"`
	Description string `yaml:"description,omitempty"`
}

type serverObject struct {
	URL string `yaml:"url"`
}

type components struct {
	Schemas         map[string]schemaObject   `yaml:"schemas,omitempty"`
	SecuritySchemes map[string]securityScheme `yaml:"securitySchemes,omitempty"`
}

type pathItem struct {
	Get    *operation `yaml:"get,omitempty"`
	Post   *operation `yaml:"post,omitempty"`
	Put    *operation `yaml:"put,omitempty"`
	Patch  *operation `yaml:"patch,omitempty"`
	Delete *operation `yaml:"delete,omitempty"`
}

type operation struct {
	OperationID  string                `yaml:"operationId,omitempty"`
	Summary      string                `yaml:"summary,omitempty"`
	Description  string                `yaml:"description,omitempty"`
	Tags         []string              `yaml:"tags,omitempty"`
	Security     []map[string][]string `yaml:"security,omitempty"`
	XMiddlewares []string              `yaml:"x-middlewares,omitempty"`
	Parameters   []parameterObject     `yaml:"parameters,omitempty"`
	RequestBody  *requestBodyObject    `yaml:"requestBody,omitempty"`
	Responses    map[string]response   `yaml:"responses"`
}

type tagObject struct {
	Name        string `yaml:"name"`
	Description string `yaml:"description,omitempty"`
}

type securityScheme struct {
	Type         string `yaml:"type"`
	Scheme       string `yaml:"scheme,omitempty"`
	BearerFormat string `yaml:"bearerFormat,omitempty"`
}

type parameterObject struct {
	Name        string       `yaml:"name"`
	In          string       `yaml:"in"`
	Required    bool         `yaml:"required,omitempty"`
	Description string       `yaml:"description,omitempty"`
	Schema      schemaObject `yaml:"schema"`
}

type requestBodyObject struct {
	Required bool        `yaml:"required,omitempty"`
	Content  contentType `yaml:"content"`
}

type response struct {
	Description string       `yaml:"description"`
	Content     *contentType `yaml:"content,omitempty"`
}

type contentType struct {
	ApplicationJSON mediaType `yaml:"application/json"`
}

type mediaType struct {
	Schema schemaObject `yaml:"schema"`
}

type schemaObject struct {
	Ref                  string                  `yaml:"$ref,omitempty"`
	Type                 string                  `yaml:"type,omitempty"`
	Properties           map[string]schemaObject `yaml:"properties,omitempty"`
	Required             []string                `yaml:"required,omitempty"`
	Enum                 []any                   `yaml:"enum,omitempty"`
	Items                *schemaObject           `yaml:"items,omitempty"`
	AdditionalProperties *schemaObject           `yaml:"additionalProperties,omitempty"`
}

func WriteYAML(ir *model.IR, outPath string) error {
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

	hasAuth := false
	for _, r := range ir.Routes {
		if r.AuthRequired {
			hasAuth = true
			break
		}
	}
	if hasAuth {
		doc.Components.SecuritySchemes = map[string]securityScheme{
			"bearerAuth": {
				Type:         "http",
				Scheme:       "bearer",
				BearerFormat: "JWT",
			},
		}
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
		if r.AuthRequired {
			op.Security = []map[string][]string{{"bearerAuth": []string{}}}
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

	b, err := yaml.Marshal(doc)
	if err != nil {
		return err
	}

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

func itoaOrDefault(code int) string {
	if code <= 0 {
		return "200"
	}
	return strconv.Itoa(code)
}
