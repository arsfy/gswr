# GoSemRoute

![Golang](https://img.shields.io/badge/-Golang%201.26-17333d?style=flat-square&logo=go&logoColor=white)
![OpenAPI](https://img.shields.io/badge/-OpenAPI%203-2e6614?style=flat-square)

Semantic OpenAPI generator for Go projects.

GoSemRoute focuses on **semantic recognition**, not only annotation parsing.  
It walks routing code, follows helper wrappers, and infers request/response schemas from real code paths.

## Status

- Supported:
    - [x] `echo` v5 / v4
- Planned: 
    - [ ] `gin`

> This tool is optimized for real internal codebases, but it is still static analysis.  
> If you hit unsupported patterns, open an issue with a minimal code sample.

## Quick Start

Install CLI:

```bash
go install github.com/arsfy/gswr/cmd/gswr@latest
```

Run:

Generate YAML:

```bash
gswr --entry ./main.go --out docs/openapi.yaml
```

Generate JSON:

```bash
gswr --entry ./main.go --out docs/openapi.json
```

Force format explicitly:

```bash
gswr --entry ./main.go --out docs/openapi.out --format json
gswr --entry ./main.go --out docs/openapi.out --format yaml
```

## Why Semantic Recognition

Most generators rely heavily on doc comments.  
GoSemRoute additionally infers API shape from code semantics, so it can still produce useful docs with partial or missing annotations.

## Core Capabilities

- Route discovery with nested `Group(...)` recursion and cross-file router chaining
- Input inference from `Param`, `QueryParam`, `QueryParamOr`, `FormValue`, `FormValueOr`
- `Bind(&req)` inference via `param/query/header/json` tags and required constraints
- Response inference from direct `c.JSON(...)` returns and helper wrappers like `resp.Success(...)`
- Multi-exit response collection (`return` in different branches)
- Type inference across nested structs, map literals, and helper argument binding
- Authentication inference from middleware semantics (`bearer`, `cookie`, `header` apiKey)
- Tag support via explicit `@Tags` / `@tag` and automatic path-based fallback grouping

## Annotation Support

- Operation: `@summary` / `@Summary`, `@description` / `@Description`, `@tag` / `@tags` / `@Tags`
- Main metadata: `@title`, `@version`, `@description`, `@BasePath` / `@basepath`, `@host`, `@schemes`

## Example Pattern (Helper Wrappers)

GoSemRoute can infer response schema through helper layers:

```go
func Success(c *echo.Context, data any) error {
  return c.JSON(http.StatusOK, types.Response{Code: "ok", Data: data})
}

func List(c *echo.Context) error {
  id, _ := ParseIDParam(c, "id")
  return Success(c, map[string]any{
    "id": id,
  })
}
```

Generated `200` schema will include a typed `data.id` field instead of a generic object.

## Current Limitations

- Dynamic runtime-only patterns (reflection-heavy dispatch, generated handlers) may not be fully resolved
- Ambiguous symbols with no import/type context may degrade to generic object schema
- This is static analysis, not runtime tracing

## Development

Run tests:

```bash
go test ./...
```
