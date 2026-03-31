package routing

import (
	"fmt"
	"sync"

	"github.com/frob/nullspace/core/request"
)

// Registry holds named handlers and middleware that TOML routes reference.
// Modules register their handlers/middleware by name during Init.
// The routing module resolves names to functions after all modules have initialized.
type Registry struct {
	handlers   map[string]request.HandlerFunc
	middleware map[string]request.Middleware
	mu         sync.RWMutex
}

// NewRegistry creates an empty registry.
func NewRegistry() *Registry {
	return &Registry{
		handlers:   make(map[string]request.HandlerFunc),
		middleware: make(map[string]request.Middleware),
	}
}

// HandleFunc registers a named handler.
func (r *Registry) HandleFunc(name string, h request.HandlerFunc) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.handlers[name] = h
}

// Middleware registers a named middleware.
func (r *Registry) Middleware(name string, m request.Middleware) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.middleware[name] = m
}

// LookupHandler returns the handler registered under the given name.
func (r *Registry) LookupHandler(name string) (request.HandlerFunc, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	h, ok := r.handlers[name]
	if !ok {
		return nil, fmt.Errorf("handler not found: %q", name)
	}
	return h, nil
}

// LookupMiddleware returns the middleware registered under the given name.
func (r *Registry) LookupMiddleware(name string) (request.Middleware, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	m, ok := r.middleware[name]
	if !ok {
		return nil, fmt.Errorf("middleware not found: %q", name)
	}
	return m, nil
}

// ResolveMiddleware resolves a list of middleware names to functions.
func (r *Registry) ResolveMiddleware(names []string) ([]request.Middleware, error) {
	mws := make([]request.Middleware, 0, len(names))
	for _, name := range names {
		m, err := r.LookupMiddleware(name)
		if err != nil {
			return nil, err
		}
		mws = append(mws, m)
	}
	return mws, nil
}

// HandlerNames returns all registered handler names (sorted).
func (r *Registry) HandlerNames() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	names := make([]string, 0, len(r.handlers))
	for name := range r.handlers {
		names = append(names, name)
	}
	return names
}

// MiddlewareNames returns all registered middleware names.
func (r *Registry) MiddlewareNames() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	names := make([]string, 0, len(r.middleware))
	for name := range r.middleware {
		names = append(names, name)
	}
	return names
}
