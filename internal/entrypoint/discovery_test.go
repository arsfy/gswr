package entrypoint

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDiscoverSelectsOnlyMainThatProducesRoutes(t *testing.T) {
	root := t.TempDir()
	write(t, filepath.Join(root, "go.mod"), "module example.com/discovery\n\ngo 1.24\n")
	write(t, filepath.Join(root, "cmd", "agent", "main.go"), `package main
func main() {}
`)
	write(t, filepath.Join(root, "cmd", "api", "main.go"), `package main
import "github.com/labstack/echo/v5"
func main() {
	e := echo.New()
	e.GET("/health", health)
}
func health(c echo.Context) error { return c.NoContent(204) }
`)

	got, err := Discover(root)
	if err != nil {
		t.Fatalf("discover: %v", err)
	}
	want := filepath.Join(root, "cmd", "api", "main.go")
	if got != want {
		t.Fatalf("entry mismatch: got %q want %q", got, want)
	}
}

func TestDiscoverReportsMultipleAPIMains(t *testing.T) {
	root := t.TempDir()
	write(t, filepath.Join(root, "go.mod"), "module example.com/ambiguous\n\ngo 1.24\n")
	for _, name := range []string{"admin", "public"} {
		write(t, filepath.Join(root, "cmd", name, "main.go"), `package main
import "github.com/labstack/echo/v5"
func main() { e := echo.New(); e.GET("/health", health) }
func health(c echo.Context) error { return c.NoContent(204) }
`)
	}

	_, err := Discover(root)
	if err == nil || !strings.Contains(err.Error(), "cmd/admin/main.go") || !strings.Contains(err.Error(), "cmd/public/main.go") {
		t.Fatalf("expected candidate list, got %v", err)
	}
}

func write(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
