// Package nslog provides the framework's logging module.
//
// It configures the kernel logger based on TOML/env config (level, format),
// registers request lifecycle hooks for automatic request/response logging,
// and provides per-request logger enrichment with request-scoped fields.
package nslog

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/frob/nullspace/kernel"
)

// Config holds the logging module's configuration.
type Config struct {
	Level  string `json:"level" toml:"level"`   // debug, info, warn, error
	Format string `json:"format" toml:"format"` // text, json
}

// Module is the logging module. It reconfigures the kernel logger during Init
// and registers hooks for per-request logging.
type Module struct {
	logger kernel.Logger
}

// New creates a new logging module.
func New() *Module {
	return &Module{}
}

func (m *Module) Name() string { return "log" }

func (m *Module) Config() kernel.ModuleConfig {
	return kernel.ModuleConfig{
		Key: "log",
		Default: Config{
			Level:  "info",
			Format: "text",
		},
		DefaultEnabled: true,
	}
}

func (m *Module) Init(k *kernel.Kernel) error {
	var cfg Config
	if err := k.Config().Decode("log", &cfg); err != nil {
		// Fall back to defaults if decode fails.
		cfg = Config{Level: "info", Format: "text"}
	}

	level, err := parseLevel(cfg.Level)
	if err != nil {
		return fmt.Errorf("log config: %w", err)
	}

	handler := buildHandler(cfg.Format, level)
	m.logger = kernel.NewSlogLoggerFrom(slog.New(handler))

	k.SetLogger(m.logger)
	k.Provide("logger", m.logger)

	// Register request lifecycle hooks.
	k.Hook("request.received", 10, m.onRequestReceived)
	k.Hook("request.complete", 90, m.onRequestComplete)

	return nil
}

func (m *Module) Start(ctx context.Context) error { return nil }
func (m *Module) Stop(ctx context.Context) error  { return nil }

// onRequestReceived logs the incoming request. The request adapter is
// responsible for creating the enriched per-request logger and storing
// it in context before this hook fires.
func (m *Module) onRequestReceived(ctx context.Context) error {
	l := FromContext(ctx)
	if l == nil {
		l = m.logger
	}
	l.Info("request received")
	return nil
}

// onRequestComplete logs the response with status and duration.
func (m *Module) onRequestComplete(ctx context.Context) error {
	reqLogger := FromContext(ctx)
	if reqLogger == nil {
		reqLogger = m.logger
	}

	var args []any
	if status := responseStatus(ctx); status != 0 {
		args = append(args, "status", status)
	}
	if start := requestStart(ctx); !start.IsZero() {
		args = append(args, "duration_ms", time.Since(start).Milliseconds())
	}

	reqLogger.Info("request complete", args...)
	return nil
}

func parseLevel(s string) (slog.Level, error) {
	switch strings.ToLower(s) {
	case "debug":
		return slog.LevelDebug, nil
	case "info", "":
		return slog.LevelInfo, nil
	case "warn", "warning":
		return slog.LevelWarn, nil
	case "error":
		return slog.LevelError, nil
	default:
		return slog.LevelInfo, fmt.Errorf("unknown log level: %q", s)
	}
}

func buildHandler(format string, level slog.Level) slog.Handler {
	opts := &slog.HandlerOptions{Level: level}
	switch strings.ToLower(format) {
	case "json":
		return slog.NewJSONHandler(os.Stderr, opts)
	default:
		return slog.NewTextHandler(os.Stderr, opts)
	}
}
