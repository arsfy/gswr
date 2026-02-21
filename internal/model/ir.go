package model

type Route struct {
	Method       string
	Path         string
	OperationID  string
	Summary      string
	Description  string
	AuthRequired bool
	AuthSchemes  []string
	Tags         []string
	Middlewares  []string
	Parameters   []Parameter
	RequestBody  *Schema
	Responses    []Response
}

type Response struct {
	StatusCode  int
	Description string
	Schema      *Schema
}

type Parameter struct {
	Name        string
	In          string
	Required    bool
	Description string
	Schema      *Schema
}

type IR struct {
	Title       string
	Version     string
	Description string
	Servers     []string
	Tags        []Tag
	Routes      []Route
	Components  map[string]*Schema
}

type Tag struct {
	Name        string
	Description string
}

type Schema struct {
	Ref                  string
	Type                 string
	Properties           map[string]*Schema
	Required             []string
	Enum                 []any
	Items                *Schema
	AdditionalProperties *Schema
}
