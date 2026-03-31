package nslog

import (
	"bytes"
	"context"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/frob/nullspace/kernel"
)

func TestModuleName(t *testing.T) {
	m := New()
	if m.Name() != "log" {
		t.Fatalf("expected 'log', got %q", m.Name())
	}
}

func TestModuleConfig(t *testing.T) {
	m := New()
	cfg := m.Config()
	if cfg.Key != "log" {
		t.Fatalf("expected key 'log', got %q", cfg.Key)
	}
	if !cfg.DefaultEnabled {
		t.Fatal("expected default enabled")
	}
	def, ok := cfg.Default.(Config)
	if !ok {
		t.Fatalf("expected Config type, got %T", cfg.Default)
	}
	if def.Level != "info" || def.Format != "text" {
		t.Fatalf("unexpected defaults: %+v", def)
	}
}

func TestModuleInitDefaultConfig(t *testing.T) {
	k := kernel.New()
	m := New()
	k.Use(m)

	ctx := context.Background()
	if err := k.Init(ctx); err != nil {
		t.Fatalf("Init: %v", err)
	}

	// Logger should be available as a resource.
	_, ok := k.Resource("logger")
	if !ok {
		t.Fatal("expected logger resource to be provided")
	}
}

func TestModuleInitWithConfig(t *testing.T) {
	k := kernel.New()
	m := New()
	k.Use(m)

	// Pre-set config to simulate TOML loading.
	k.Config().Set("log", map[string]any{
		"level":  "debug",
		"format": "json",
	})

	ctx := context.Background()
	if err := k.Init(ctx); err != nil {
		t.Fatalf("Init: %v", err)
	}

	// Verify the kernel logger was replaced.
	l := k.Logger()
	if l == nil {
		t.Fatal("expected non-nil logger")
	}
}

func TestModuleInitInvalidLevel(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/nullspace.toml"
	os.WriteFile(path, []byte(`
[log]
level = "bogus"
`), 0644)

	k := kernel.New(kernel.WithConfigFile(path))
	m := New()
	k.Use(m)

	ctx := context.Background()
	err := k.Init(ctx)
	if err == nil {
		t.Fatal("expected error for invalid log level")
	}
}

func TestParseLevel(t *testing.T) {
	tests := []struct {
		input string
		want  slog.Level
		err   bool
	}{
		{"debug", slog.LevelDebug, false},
		{"info", slog.LevelInfo, false},
		{"warn", slog.LevelWarn, false},
		{"warning", slog.LevelWarn, false},
		{"error", slog.LevelError, false},
		{"DEBUG", slog.LevelDebug, false},
		{"", slog.LevelInfo, false},
		{"bogus", slog.LevelInfo, true},
	}

	for _, tt := range tests {
		got, err := parseLevel(tt.input)
		if tt.err && err == nil {
			t.Errorf("parseLevel(%q): expected error", tt.input)
		}
		if !tt.err && err != nil {
			t.Errorf("parseLevel(%q): unexpected error: %v", tt.input, err)
		}
		if !tt.err && got != tt.want {
			t.Errorf("parseLevel(%q) = %v, want %v", tt.input, got, tt.want)
		}
	}
}

func TestBuildHandler(t *testing.T) {
	// Text handler.
	h := buildHandler("text", slog.LevelInfo)
	if _, ok := h.(*slog.TextHandler); !ok {
		t.Fatalf("expected TextHandler, got %T", h)
	}

	// JSON handler.
	h = buildHandler("json", slog.LevelDebug)
	if _, ok := h.(*slog.JSONHandler); !ok {
		t.Fatalf("expected JSONHandler, got %T", h)
	}

	// Unknown defaults to text.
	h = buildHandler("unknown", slog.LevelInfo)
	if _, ok := h.(*slog.TextHandler); !ok {
		t.Fatalf("expected TextHandler for unknown format, got %T", h)
	}
}

func TestContextLogger(t *testing.T) {
	var buf bytes.Buffer
	handler := slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})
	logger := kernel.NewSlogLoggerFrom(slog.New(handler))

	ctx := context.Background()

	// No logger in plain context.
	if l := FromContext(ctx); l != nil {
		t.Fatal("expected nil logger from plain context")
	}

	// Store and retrieve.
	ctx = WithLogger(ctx, logger)
	l := FromContext(ctx)
	if l == nil {
		t.Fatal("expected logger from context")
	}

	l.Info("test message")
	if !bytes.Contains(buf.Bytes(), []byte("test message")) {
		t.Fatalf("expected log output, got: %s", buf.String())
	}
}

func TestContextRequestInfo(t *testing.T) {
	ctx := context.Background()
	ctx = WithRequestID(ctx, "req-123")
	ctx = WithRequestInfo(ctx, "GET", "/api/test")

	if got := requestID(ctx); got != "req-123" {
		t.Fatalf("expected req-123, got %q", got)
	}
	if got := requestMethod(ctx); got != "GET" {
		t.Fatalf("expected GET, got %q", got)
	}
	if got := requestPath(ctx); got != "/api/test" {
		t.Fatalf("expected /api/test, got %q", got)
	}
	if start := requestStart(ctx); start.IsZero() {
		t.Fatal("expected non-zero start time")
	}
}

func TestContextResponseStatus(t *testing.T) {
	ctx := context.Background()
	ctx = WithResponseStatus(ctx, 200)

	if got := responseStatus(ctx); got != 200 {
		t.Fatalf("expected 200, got %d", got)
	}
}

func TestOnRequestComplete(t *testing.T) {
	var buf bytes.Buffer
	handler := slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})
	logger := kernel.NewSlogLoggerFrom(slog.New(handler))

	m := &Module{logger: logger}

	ctx := context.Background()
	ctx = WithLogger(ctx, logger)
	ctx = WithResponseStatus(ctx, 201)
	ctx = context.WithValue(ctx, startKey, time.Now().Add(-50*time.Millisecond))

	if err := m.onRequestComplete(ctx); err != nil {
		t.Fatalf("onRequestComplete: %v", err)
	}

	output := buf.String()
	if !bytes.Contains(buf.Bytes(), []byte("request complete")) {
		t.Fatalf("expected 'request complete' in output: %s", output)
	}
	if !bytes.Contains(buf.Bytes(), []byte("status=201")) {
		t.Fatalf("expected status=201 in output: %s", output)
	}
	if !bytes.Contains(buf.Bytes(), []byte("duration_ms=")) {
		t.Fatalf("expected duration_ms in output: %s", output)
	}
}
