package request

import (
	"context"
	"net/http"

	"github.com/frob/nullspace/kernel"
	"github.com/frob/nullspace/nslog"
)

// Context is the framework's request context. It wraps the standard
// http.Request and ResponseWriter, and provides typed accessors for
// the config snapshot, per-request logger, route parameters, and
// arbitrary state shared between middleware.
type Context struct {
	Request *http.Request
	Writer  http.ResponseWriter

	ctx    context.Context
	params map[string]string
	route  *RouteMatch
	state  map[string]any
}

// newContext creates a framework Context from an HTTP request.
func newContext(w http.ResponseWriter, r *http.Request, ctx context.Context) *Context {
	return &Context{
		Request: r,
		Writer:  w,
		ctx:     ctx,
		params:  make(map[string]string),
		state:   make(map[string]any),
	}
}

// Context returns the underlying context.Context, which carries the config
// snapshot, per-request logger, and other framework values. This is the
// same context passed to hooks.
func (c *Context) Context() context.Context {
	return c.ctx
}

// Logger returns the per-request logger from the context.
// Falls back to a default slog logger if none is attached.
func (c *Context) Logger() kernel.Logger {
	if l := nslog.FromContext(c.ctx); l != nil {
		return l
	}
	return kernel.NewSlogLogger()
}

// Snapshot returns the immutable config snapshot for this request.
func (c *Context) Snapshot() *kernel.Snapshot {
	return kernel.SnapshotFromContext(c.ctx)
}

// Param returns a route path parameter by name (e.g., ":id" -> Param("id")).
// Returns empty string if the parameter does not exist.
func (c *Context) Param(name string) string {
	return c.params[name]
}

// Route returns the matched route information, or nil if no route matched.
func (c *Context) Route() *RouteMatch {
	return c.route
}

// State retrieves a value from the request-scoped state bag.
// Middleware can use this to pass data to downstream handlers.
func (c *Context) State(key string) (any, bool) {
	v, ok := c.state[key]
	return v, ok
}

// SetState stores a value in the request-scoped state bag.
func (c *Context) SetState(key string, val any) {
	c.state[key] = val
}
