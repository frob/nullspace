package static

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/frob/nullspace/core/nslog"
	"github.com/frob/nullspace/core/request"
	"github.com/frob/nullspace/kernel"
)

func setupTestAdapter(t *testing.T, staticDir string) (*request.Adapter, *kernel.Kernel) {
	t.Helper()

	tomlPath := filepath.Join(t.TempDir(), "nullspace.toml")
	os.WriteFile(tomlPath, []byte(`
[data.static]
dir = "`+staticDir+`"
`), 0644)

	k := kernel.New(kernel.WithConfigFile(tomlPath))
	logMod := nslog.New()
	adapter := request.NewAdapter()
	staticMod := New()

	k.Use(logMod)
	k.Use(adapter)
	k.Use(staticMod)

	if err := k.Init(context.Background()); err != nil {
		t.Fatalf("Init: %v", err)
	}

	return adapter, k
}

func TestServeStaticFile(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "style.css"), []byte("body { color: red; }"), 0644)

	adapter, _ := setupTestAdapter(t, dir)

	req := httptest.NewRequest("GET", "/style.css", nil)
	w := httptest.NewRecorder()
	adapter.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if w.Body.String() != "body { color: red; }" {
		t.Fatalf("unexpected body: %q", w.Body.String())
	}
	ct := w.Header().Get("Content-Type")
	if ct != "text/css; charset=utf-8" {
		t.Fatalf("expected text/css, got %q", ct)
	}
}

func TestServeStaticHTML(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "page.html"), []byte("<h1>Hello</h1>"), 0644)

	adapter, _ := setupTestAdapter(t, dir)

	req := httptest.NewRequest("GET", "/page.html", nil)
	w := httptest.NewRecorder()
	adapter.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if w.Body.String() != "<h1>Hello</h1>" {
		t.Fatalf("unexpected body: %q", w.Body.String())
	}
}

func TestServeIndexHTML(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "docs"), 0755)
	os.WriteFile(filepath.Join(dir, "docs", "index.html"), []byte("docs index"), 0644)

	adapter, _ := setupTestAdapter(t, dir)

	req := httptest.NewRequest("GET", "/docs/", nil)
	w := httptest.NewRecorder()
	adapter.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if w.Body.String() != "docs index" {
		t.Fatalf("unexpected body: %q", w.Body.String())
	}
}

func TestStaticFileNotFound(t *testing.T) {
	dir := t.TempDir()
	adapter, _ := setupTestAdapter(t, dir)

	req := httptest.NewRequest("GET", "/nonexistent.js", nil)
	w := httptest.NewRecorder()
	adapter.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestRoutesTakePrecedence(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "api"), []byte("static file"), 0644)

	adapter, _ := setupTestAdapter(t, dir)

	// Register a dynamic route at the same path.
	adapter.Router().Get("/api", func(ctx *request.Context) error {
		ctx.Writer.WriteHeader(http.StatusOK)
		ctx.Writer.Write([]byte("dynamic route"))
		return nil
	})

	req := httptest.NewRequest("GET", "/api", nil)
	w := httptest.NewRecorder()
	adapter.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if w.Body.String() != "dynamic route" {
		t.Fatalf("expected dynamic route to win, got %q", w.Body.String())
	}
}

func TestDirectoryTraversal(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "safe.txt"), []byte("safe"), 0644)

	adapter, _ := setupTestAdapter(t, dir)

	req := httptest.NewRequest("GET", "/../../../etc/passwd", nil)
	w := httptest.NewRecorder()
	adapter.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for traversal, got %d", w.Code)
	}
}
