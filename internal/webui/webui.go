package webui

import (
	"embed"
	"fmt"
	"io/fs"
	"net/http"
	"strings"
)

const DefaultAddress = "127.0.0.1:43877"

//go:embed dist
var embeddedAssets embed.FS

type SpecGenerator func() ([]byte, error)

func NewHandler(generate SpecGenerator) (http.Handler, error) {
	if generate == nil {
		return nil, fmt.Errorf("OpenAPI generator is required")
	}
	dist, err := fs.Sub(embeddedAssets, "dist")
	if err != nil {
		return nil, err
	}
	files := http.FileServer(http.FS(dist))

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/openapi.yaml" {
			serveOpenAPI(w, r, generate)
			return
		}

		path := strings.TrimPrefix(r.URL.Path, "/")
		if path == "" {
			path = "index.html"
		}
		if info, statErr := fs.Stat(dist, path); statErr == nil && !info.IsDir() {
			files.ServeHTTP(w, r)
			return
		}

		fallback := r.Clone(r.Context())
		fallback.URL.Path = "/"
		files.ServeHTTP(w, fallback)
	}), nil
}

func serveOpenAPI(w http.ResponseWriter, r *http.Request, generate SpecGenerator) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD")
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	data, err := generate()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/yaml; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Length", fmt.Sprintf("%d", len(data)))
	if r.Method == http.MethodGet {
		_, _ = w.Write(data)
	}
}
