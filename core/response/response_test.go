package response

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/frob/nullspace/core/nslog"
	"github.com/frob/nullspace/kernel"
)

func setupPipeline(t *testing.T) (*Pipeline, *kernel.Kernel) {
	t.Helper()

	k := kernel.New()
	logMod := nslog.New()
	pipeline := NewPipeline()

	k.Use(logMod)
	k.Use(pipeline)
	k.Use(NewFormatRouteOverride())
	k.Use(NewFormatQueryParam())
	k.Use(NewFormatContentNegotiate())
	k.Use(NewFormatDefault())

	if err := k.Init(context.Background()); err != nil {
		t.Fatalf("Init: %v", err)
	}

	return pipeline, k
}

// --- JSON Formatter Tests ---

func TestJSONFormatter(t *testing.T) {
	f := &JSONFormatter{}
	ctx := context.Background()

	resp := &Response{Data: map[string]string{"hello": "world"}}
	data, err := f.Format(ctx, resp)
	if err != nil {
		t.Fatalf("Format: %v", err)
	}

	if string(data) != `{"hello":"world"}` {
		t.Fatalf("unexpected JSON: %s", data)
	}
}

func TestJSONFormatterPretty(t *testing.T) {
	f := &JSONFormatter{}
	r := httptest.NewRequest("GET", "/test?pretty=true", nil)
	ctx := WithHTTPRequest(context.Background(), r)

	resp := &Response{Data: map[string]string{"a": "b"}}
	data, err := f.Format(ctx, resp)
	if err != nil {
		t.Fatalf("Format: %v", err)
	}

	expected := "{\n  \"a\": \"b\"\n}"
	if string(data) != expected {
		t.Fatalf("expected pretty JSON:\n%s\ngot:\n%s", expected, data)
	}
}

// --- HTML Formatter Tests ---

func TestHTMLFormatter(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "hello.html"), []byte("<h1>{{.Title}}</h1>"), 0644)

	f := NewHTMLFormatter(dir)
	ctx := context.Background()

	resp := &Response{
		Data:     map[string]string{"Title": "Hello"},
		Template: "hello.html",
	}
	data, err := f.Format(ctx, resp)
	if err != nil {
		t.Fatalf("Format: %v", err)
	}

	if string(data) != "<h1>Hello</h1>" {
		t.Fatalf("unexpected HTML: %s", data)
	}
}

func TestHTMLFormatterPathTraversal(t *testing.T) {
	// Set up two dirs: the template dir and a "secret" dir outside it.
	templateDir := t.TempDir()
	secretDir := t.TempDir()
	os.WriteFile(filepath.Join(secretDir, "secret.txt"), []byte("secret contents"), 0644)

	f := NewHTMLFormatter(templateDir)
	ctx := context.Background()

	// Construct a traversal name that points at the secret file.
	traversal := "../" + filepath.Base(secretDir) + "/secret.txt"
	resp := &Response{Template: traversal}

	_, err := f.Format(ctx, resp)
	if err == nil {
		t.Fatal("expected error for path traversal attempt, got nil")
	}
}

func TestHTMLFormatterNoTemplate(t *testing.T) {
	f := NewHTMLFormatter(t.TempDir())
	resp := &Response{Data: nil}

	_, err := f.Format(context.Background(), resp)
	if err == nil {
		t.Fatal("expected error for missing template")
	}
}

// --- Content Negotiation Tests ---

func TestNegotiateFormat(t *testing.T) {
	tests := []struct {
		accept string
		want   string
	}{
		{"application/json", "json"},
		{"text/html", "html"},
		{"text/html, application/json", "html"},
		{"application/json, text/html", "json"},
		{"text/plain", "text"},
		{"*/*", ""},
		{"", ""},
		{"application/xml", ""},
	}

	for _, tt := range tests {
		got := negotiateFormat(tt.accept)
		if got != tt.want {
			t.Errorf("negotiateFormat(%q) = %q, want %q", tt.accept, got, tt.want)
		}
	}
}

// --- Format Resolution Tests ---

func TestFormatResolutionRouteOverride(t *testing.T) {
	pipeline, _ := setupPipeline(t)

	ctx := context.Background()
	ctx = WithRouteFormat(ctx, "html")

	format, err := pipeline.resolveFormat(ctx)
	if err != nil {
		t.Fatalf("resolveFormat: %v", err)
	}
	if format != "html" {
		t.Fatalf("expected html, got %s", format)
	}
}

func TestFormatResolutionQueryParam(t *testing.T) {
	pipeline, _ := setupPipeline(t)

	ctx := context.Background()
	ctx = WithQueryFormat(ctx, "json")

	format, err := pipeline.resolveFormat(ctx)
	if err != nil {
		t.Fatalf("resolveFormat: %v", err)
	}
	if format != "json" {
		t.Fatalf("expected json, got %s", format)
	}
}

func TestFormatResolutionAcceptHeader(t *testing.T) {
	pipeline, _ := setupPipeline(t)

	ctx := context.Background()
	ctx = WithAcceptHeader(ctx, "text/html")

	format, err := pipeline.resolveFormat(ctx)
	if err != nil {
		t.Fatalf("resolveFormat: %v", err)
	}
	if format != "html" {
		t.Fatalf("expected html, got %s", format)
	}
}

func TestFormatResolutionDefault(t *testing.T) {
	pipeline, _ := setupPipeline(t)

	ctx := context.Background()
	// No format hints — should fall through to default.
	format, err := pipeline.resolveFormat(ctx)
	if err != nil {
		t.Fatalf("resolveFormat: %v", err)
	}
	if format != "json" {
		t.Fatalf("expected default json, got %s", format)
	}
}

func TestFormatResolutionPrecedence(t *testing.T) {
	pipeline, _ := setupPipeline(t)

	// Route override (priority 10) should beat query param (priority 20).
	ctx := context.Background()
	ctx = WithRouteFormat(ctx, "html")
	ctx = WithQueryFormat(ctx, "json")

	format, err := pipeline.resolveFormat(ctx)
	if err != nil {
		t.Fatalf("resolveFormat: %v", err)
	}
	if format != "html" {
		t.Fatalf("expected route override (html) to win, got %s", format)
	}
}

// --- Pipeline Write Tests ---

func TestPipelineWriteJSON(t *testing.T) {
	pipeline, _ := setupPipeline(t)

	w := httptest.NewRecorder()
	ctx := context.Background()
	ctx = WithQueryFormat(ctx, "json")

	resp := NewResponse(http.StatusOK, map[string]string{"msg": "ok"})
	if err := pipeline.Write(ctx, w, resp); err != nil {
		t.Fatalf("Write: %v", err)
	}

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if w.Header().Get("Content-Type") != "application/json" {
		t.Fatalf("expected application/json, got %s", w.Header().Get("Content-Type"))
	}
	if w.Body.String() != `{"msg":"ok"}` {
		t.Fatalf("unexpected body: %s", w.Body.String())
	}
}

func TestPipelineWriteHTML(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "test.html"), []byte("Hello {{.Name}}"), 0644)

	tomlPath := filepath.Join(t.TempDir(), "nullspace.toml")
	os.WriteFile(tomlPath, []byte(`
[response]
default_format = "json"
template_dir = "`+dir+`"
`), 0644)

	k := kernel.New(kernel.WithConfigFile(tomlPath))
	k.Use(nslog.New())
	pipeline := NewPipeline()
	k.Use(pipeline)
	k.Use(NewFormatQueryParam())
	k.Use(NewFormatDefault())

	if err := k.Init(context.Background()); err != nil {
		t.Fatalf("Init: %v", err)
	}

	w := httptest.NewRecorder()
	ctx := context.Background()
	ctx = WithQueryFormat(ctx, "html")

	resp := NewResponse(http.StatusOK, map[string]string{"Name": "World"})
	resp.Template = "test.html"

	if err := pipeline.Write(ctx, w, resp); err != nil {
		t.Fatalf("Write: %v", err)
	}

	if w.Body.String() != "Hello World" {
		t.Fatalf("unexpected body: %s", w.Body.String())
	}
}

func TestPipelineWriteError(t *testing.T) {
	pipeline, _ := setupPipeline(t)

	w := httptest.NewRecorder()
	ctx := context.Background()

	resp := &Response{
		Status: http.StatusBadRequest,
		Error:  fmt.Errorf("bad input"),
	}
	pipeline.Write(ctx, w, resp)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestPipelineWriteCustomHeaders(t *testing.T) {
	pipeline, _ := setupPipeline(t)

	w := httptest.NewRecorder()
	ctx := context.Background()

	resp := NewResponse(http.StatusOK, "data").WithHeader("X-Custom", "test")
	pipeline.Write(ctx, w, resp)

	if w.Header().Get("X-Custom") != "test" {
		t.Fatalf("expected custom header, got %q", w.Header().Get("X-Custom"))
	}
}
