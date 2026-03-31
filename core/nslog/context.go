package nslog

import (
	"context"
	"time"

	"github.com/frob/nullspace/kernel"
)

type contextKey struct{ name string }

var (
	loggerKey    = contextKey{"nslog.logger"}
	requestIDKey = contextKey{"nslog.request_id"}
	methodKey    = contextKey{"nslog.method"}
	pathKey      = contextKey{"nslog.path"}
	statusKey    = contextKey{"nslog.status"}
	startKey     = contextKey{"nslog.start"}
)

// FromContext returns the per-request logger from the context.
// Returns nil if no logger is attached.
func FromContext(ctx context.Context) kernel.Logger {
	l, _ := ctx.Value(loggerKey).(kernel.Logger)
	return l
}

// WithLogger returns a new context carrying the given logger.
func WithLogger(ctx context.Context, l kernel.Logger) context.Context {
	return context.WithValue(ctx, loggerKey, l)
}

// WithRequestID returns a new context carrying the given request ID.
func WithRequestID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, requestIDKey, id)
}

// WithRequestInfo returns a new context carrying request method, path, and start time.
func WithRequestInfo(ctx context.Context, method, path string) context.Context {
	ctx = context.WithValue(ctx, methodKey, method)
	ctx = context.WithValue(ctx, pathKey, path)
	ctx = context.WithValue(ctx, startKey, time.Now())
	return ctx
}

// WithResponseStatus returns a new context carrying the response status code.
func WithResponseStatus(ctx context.Context, status int) context.Context {
	return context.WithValue(ctx, statusKey, status)
}

// Helper functions to extract values from context.

func requestID(ctx context.Context) string {
	s, _ := ctx.Value(requestIDKey).(string)
	return s
}

func requestMethod(ctx context.Context) string {
	s, _ := ctx.Value(methodKey).(string)
	return s
}

func requestPath(ctx context.Context) string {
	s, _ := ctx.Value(pathKey).(string)
	return s
}

func responseStatus(ctx context.Context) int {
	n, _ := ctx.Value(statusKey).(int)
	return n
}

func requestStart(ctx context.Context) time.Time {
	t, _ := ctx.Value(startKey).(time.Time)
	return t
}
