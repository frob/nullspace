package response

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
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

// --- Text Formatter Tests ---

func TestTextFormatterMap(t *testing.T) {
	f := &TextFormatter{}
	resp := &Response{Data: map[string]string{"ID": "hello", "Name": "world"}}
	data, err := f.Format(context.Background(), resp)
	if err != nil {
		t.Fatalf("Format: %v", err)
	}

	expected := "ID: hello\nName: world\n"
	if string(data) != expected {
		t.Fatalf("expected:\n%s\ngot:\n%s", expected, data)
	}
}

func TestTextFormatterMapWithBody(t *testing.T) {
	f := &TextFormatter{}
	resp := &Response{Data: map[string]any{
		"ID":   "hello",
		"Body": "This is the content.",
	}}
	data, err := f.Format(context.Background(), resp)
	if err != nil {
		t.Fatalf("Format: %v", err)
	}

	expected := "ID: hello\n\nThis is the content.\n"
	if string(data) != expected {
		t.Fatalf("expected:\n%q\ngot:\n%q", expected, string(data))
	}
}

func TestTextFormatterStruct(t *testing.T) {
	type article struct {
		Title  string `json:"title"`
		Status string `json:"status"`
	}
	f := &TextFormatter{}
	resp := &Response{Data: article{Title: "Hello", Status: "published"}}
	data, err := f.Format(context.Background(), resp)
	if err != nil {
		t.Fatalf("Format: %v", err)
	}

	expected := "title: Hello\nstatus: published\n"
	if string(data) != expected {
		t.Fatalf("expected:\n%s\ngot:\n%s", expected, data)
	}
}

func TestTextFormatterSlice(t *testing.T) {
	f := &TextFormatter{}
	resp := &Response{Data: []map[string]string{
		{"ID": "a", "Title": "First"},
		{"ID": "b", "Title": "Second"},
	}}
	data, err := f.Format(context.Background(), resp)
	if err != nil {
		t.Fatalf("Format: %v", err)
	}

	out := string(data)
	if !strings.Contains(out, "Count: 2") {
		t.Fatalf("expected Count field, got:\n%s", out)
	}
	if !strings.Contains(out, "[1]") || !strings.Contains(out, "[2]") {
		t.Fatalf("expected numbered items, got:\n%s", out)
	}
}

func TestTextFormatterPrimitive(t *testing.T) {
	f := &TextFormatter{}
	resp := &Response{Data: "just a string"}
	data, err := f.Format(context.Background(), resp)
	if err != nil {
		t.Fatalf("Format: %v", err)
	}

	if string(data) != "just a string\n" {
		t.Fatalf("expected primitive body, got: %q", string(data))
	}
}

func TestTextFormatterNil(t *testing.T) {
	f := &TextFormatter{}
	resp := &Response{Data: nil}
	data, err := f.Format(context.Background(), resp)
	if err != nil {
		t.Fatalf("Format: %v", err)
	}

	if len(data) != 0 {
		t.Fatalf("expected empty output for nil, got: %q", string(data))
	}
}

func TestTextFormatterNestedMap(t *testing.T) {
	f := &TextFormatter{}
	resp := &Response{Data: map[string]any{
		"ID":   "hello",
		"Meta": map[string]any{"author": "jane", "tags": "go"},
	}}
	data, err := f.Format(context.Background(), resp)
	if err != nil {
		t.Fatalf("Format: %v", err)
	}

	out := string(data)
	if !strings.Contains(out, "Meta: {") {
		t.Fatalf("expected inline map for Meta, got:\n%s", out)
	}
}

func TestTextFormatterError(t *testing.T) {
	f := &TextFormatter{}
	resp := &Response{Data: map[string]any{"error": "not found", "status": 404}}
	data, err := f.Format(context.Background(), resp)
	if err != nil {
		t.Fatalf("Format: %v", err)
	}

	out := string(data)
	if !strings.Contains(out, "error: not found") {
		t.Fatalf("expected error field, got:\n%s", out)
	}
	if !strings.Contains(out, "status: 404") {
		t.Fatalf("expected status field, got:\n%s", out)
	}
}

func TestTextFormatterDeterministic(t *testing.T) {
	f := &TextFormatter{}
	resp := &Response{Data: map[string]string{"Z": "last", "A": "first", "M": "middle"}}

	// Run multiple times to verify sorted output.
	var prev string
	for range 10 {
		data, _ := f.Format(context.Background(), resp)
		s := string(data)
		if prev != "" && s != prev {
			t.Fatalf("non-deterministic output:\n%s\nvs:\n%s", prev, s)
		}
		prev = s
	}

	expected := "A: first\nM: middle\nZ: last\n"
	if prev != expected {
		t.Fatalf("expected sorted:\n%s\ngot:\n%s", expected, prev)
	}
}

// --- ANSI Formatter Tests ---

func TestANSIFormatterMap(t *testing.T) {
	f := &ANSIFormatter{}
	resp := &Response{Data: map[string]string{"ID": "hello"}}
	data, err := f.Format(context.Background(), resp)
	if err != nil {
		t.Fatalf("Format: %v", err)
	}

	out := string(data)
	if !strings.Contains(out, ansiBold+"ID"+ansiReset) {
		t.Fatalf("expected bold key, got:\n%q", out)
	}

	// Verify stripped output matches plain text.
	stripped := ansiStrip(data)
	if stripped != "ID: hello\n" {
		t.Fatalf("stripped output mismatch: %q", stripped)
	}
}

func TestANSIFormatterError(t *testing.T) {
	f := &ANSIFormatter{}
	resp := &Response{Data: map[string]any{"error": "bad request"}}
	data, err := f.Format(context.Background(), resp)
	if err != nil {
		t.Fatalf("Format: %v", err)
	}

	if !strings.Contains(string(data), ansiBoldRed) {
		t.Fatal("expected bold-red for error value")
	}
}

func TestANSIFormatterStatusColors(t *testing.T) {
	f := &ANSIFormatter{}

	tests := []struct {
		status int
		color  string
	}{
		{200, ""},        // no color
		{404, ansiYellow}, // yellow
		{500, ansiRed},    // red
	}

	for _, tt := range tests {
		resp := &Response{Data: map[string]any{"status": tt.status}}
		data, _ := f.Format(context.Background(), resp)
		out := string(data)

		if tt.color == "" {
			if strings.Contains(out, ansiYellow) || strings.Contains(out, ansiRed) {
				t.Errorf("status %d: unexpected color in: %q", tt.status, out)
			}
		} else {
			if !strings.Contains(out, tt.color) {
				t.Errorf("status %d: expected %q in: %q", tt.status, tt.color, out)
			}
		}
	}
}

func TestANSIFormatterResetBalance(t *testing.T) {
	f := &ANSIFormatter{}
	resp := &Response{Data: map[string]any{
		"ID":     "hello",
		"error":  "fail",
		"status": 500,
	}}
	data, _ := f.Format(context.Background(), resp)

	// Every ANSI open should have a matching reset.
	opens := ansiFieldCount(data, ansiBold) +
		ansiFieldCount(data, ansiBoldRed) +
		ansiFieldCount(data, ansiRed) +
		ansiFieldCount(data, ansiYellow) +
		ansiFieldCount(data, ansiCyan)
	resets := ansiFieldCount(data, ansiReset)

	if opens != resets {
		t.Fatalf("ANSI open/reset mismatch: %d opens, %d resets\ndata: %q", opens, resets, string(data))
	}
}

func TestANSIFormatterListIndices(t *testing.T) {
	f := &ANSIFormatter{}
	resp := &Response{Data: []string{"one", "two"}}
	data, _ := f.Format(context.Background(), resp)

	if !strings.Contains(string(data), ansiCyan+"[1]"+ansiReset) {
		t.Fatalf("expected cyan list index, got:\n%q", string(data))
	}
}

// --- Pipeline Write Tests for Text/ANSI ---

func TestPipelineWriteText(t *testing.T) {
	pipeline, _ := setupPipeline(t)

	w := httptest.NewRecorder()
	ctx := context.Background()
	ctx = WithQueryFormat(ctx, "text")

	resp := NewResponse(http.StatusOK, map[string]string{"msg": "ok"})
	if err := pipeline.Write(ctx, w, resp); err != nil {
		t.Fatalf("Write: %v", err)
	}

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if ct := w.Header().Get("Content-Type"); ct != "text/plain; charset=utf-8" {
		t.Fatalf("expected text/plain content type, got %s", ct)
	}
	if !strings.Contains(w.Body.String(), "msg: ok") {
		t.Fatalf("unexpected body: %s", w.Body.String())
	}
}

func TestPipelineWriteANSI(t *testing.T) {
	pipeline, _ := setupPipeline(t)

	w := httptest.NewRecorder()
	ctx := context.Background()
	ctx = WithQueryFormat(ctx, "ansi")

	resp := NewResponse(http.StatusOK, map[string]string{"msg": "ok"})
	if err := pipeline.Write(ctx, w, resp); err != nil {
		t.Fatalf("Write: %v", err)
	}

	if ct := w.Header().Get("Content-Type"); ct != "text/plain; charset=utf-8" {
		t.Fatalf("expected text/plain content type, got %s", ct)
	}
	if !strings.Contains(w.Body.String(), "\033[") {
		t.Fatal("expected ANSI escape codes in output")
	}
}

func TestFormatResolutionAcceptTextPlain(t *testing.T) {
	pipeline, _ := setupPipeline(t)

	ctx := context.Background()
	ctx = WithAcceptHeader(ctx, "text/plain")

	format, err := pipeline.resolveFormat(ctx)
	if err != nil {
		t.Fatalf("resolveFormat: %v", err)
	}
	if format != "text" {
		t.Fatalf("expected text, got %s", format)
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

// --- JSON StreamFormatter Tests ---

func TestJSONStreamContentType(t *testing.T) {
	f := &JSONFormatter{}
	if ct := f.StreamContentType(); ct != "application/x-ndjson" {
		t.Fatalf("StreamContentType = %q, want application/x-ndjson", ct)
	}
}

func TestJSONWriteStreamItem(t *testing.T) {
	f := &JSONFormatter{}
	var buf strings.Builder
	item := map[string]string{"ID": "hello", "title": "Hello"}
	if err := f.WriteStreamItem(context.Background(), &buf, item); err != nil {
		t.Fatalf("WriteStreamItem: %v", err)
	}

	line := buf.String()
	if line[len(line)-1] != '\n' {
		t.Fatal("expected trailing newline")
	}

	var parsed map[string]string
	if err := json.Unmarshal([]byte(line), &parsed); err != nil {
		t.Fatalf("line is not valid JSON: %v", err)
	}
	if parsed["ID"] != "hello" {
		t.Fatalf("ID = %q, want hello", parsed["ID"])
	}
}

func TestJSONWriteStreamMultipleItems(t *testing.T) {
	f := &JSONFormatter{}
	var buf strings.Builder

	for i := range 3 {
		f.WriteStreamItem(context.Background(), &buf, map[string]int{"n": i})
	}

	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(lines) != 3 {
		t.Fatalf("expected 3 lines, got %d", len(lines))
	}
	for _, line := range lines {
		var m map[string]int
		if err := json.Unmarshal([]byte(line), &m); err != nil {
			t.Fatalf("invalid NDJSON line %q: %v", line, err)
		}
	}
}

// --- Stream Context Tests ---

func TestStreamContext(t *testing.T) {
	ctx := context.Background()
	if StreamFromContext(ctx) {
		t.Fatal("expected false for empty context")
	}

	ctx = WithStream(ctx, true)
	if !StreamFromContext(ctx) {
		t.Fatal("expected true after WithStream")
	}
}

// --- Pipeline WriteStream Tests ---

// testIter is a mock StreamIter for testing.
type testIter struct {
	items []any
	pos   int
}

func (it *testIter) Next() (any, error) {
	if it.pos >= len(it.items) {
		return nil, nil
	}
	item := it.items[it.pos]
	it.pos++
	return item, nil
}

func (it *testIter) Close() error { return nil }

func TestPipelineWriteStreamNDJSON(t *testing.T) {
	pipeline, _ := setupPipeline(t)

	w := httptest.NewRecorder()
	ctx := context.Background()
	ctx = WithQueryFormat(ctx, "json")

	sr := &StreamResponse{
		Status: http.StatusOK,
		Meta:   map[string]any{"Collection": "posts"},
		Iter: &testIter{items: []any{
			map[string]string{"ID": "a"},
			map[string]string{"ID": "b"},
		}},
	}

	if err := pipeline.WriteStream(ctx, w, sr); err != nil {
		t.Fatalf("WriteStream: %v", err)
	}

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if ct := w.Header().Get("Content-Type"); ct != "application/x-ndjson" {
		t.Fatalf("Content-Type = %q, want application/x-ndjson", ct)
	}

	lines := strings.Split(strings.TrimSpace(w.Body.String()), "\n")
	if len(lines) != 2 {
		t.Fatalf("expected 2 NDJSON lines, got %d: %q", len(lines), w.Body.String())
	}
}

func TestPipelineWriteStreamFallbackToBatch(t *testing.T) {
	pipeline, _ := setupPipeline(t)

	w := httptest.NewRecorder()
	ctx := context.Background()
	ctx = WithQueryFormat(ctx, "text") // Text formatter doesn't implement StreamFormatter.

	sr := &StreamResponse{
		Status: http.StatusOK,
		Meta:   map[string]any{"Collection": "posts"},
		Iter: &testIter{items: []any{
			map[string]string{"ID": "a"},
		}},
	}

	if err := pipeline.WriteStream(ctx, w, sr); err != nil {
		t.Fatalf("WriteStream: %v", err)
	}

	if ct := w.Header().Get("Content-Type"); ct != "text/plain; charset=utf-8" {
		t.Fatalf("Content-Type = %q, want text/plain fallback", ct)
	}

	// Should contain the items in batch format.
	if !strings.Contains(w.Body.String(), "Collection: posts") {
		t.Fatalf("expected batch format with Collection, got: %s", w.Body.String())
	}
}

func TestPipelineWriteStreamEmpty(t *testing.T) {
	pipeline, _ := setupPipeline(t)

	w := httptest.NewRecorder()
	ctx := context.Background()
	ctx = WithQueryFormat(ctx, "json")

	sr := &StreamResponse{
		Status: http.StatusOK,
		Meta:   map[string]any{"Collection": "empty"},
		Iter:   &testIter{items: nil},
	}

	if err := pipeline.WriteStream(ctx, w, sr); err != nil {
		t.Fatalf("WriteStream: %v", err)
	}

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	// Empty stream = empty body (no NDJSON lines).
	if body := strings.TrimSpace(w.Body.String()); body != "" {
		t.Fatalf("expected empty body, got: %q", body)
	}
}

