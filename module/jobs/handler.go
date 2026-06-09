package jobs

import (
	"context"
	"fmt"
	"sort"
	"sync"
)

// Handler processes a single job.
type Handler func(ctx context.Context, job *Job) error

// HandlerRegistry maps job type names to their handlers.
// It is safe for concurrent use.
type HandlerRegistry struct {
	mu       sync.RWMutex
	handlers map[string]Handler
}

// NewHandlerRegistry creates an empty HandlerRegistry.
func NewHandlerRegistry() *HandlerRegistry {
	return &HandlerRegistry{
		handlers: make(map[string]Handler),
	}
}

// Handle registers h under name. Panics if name is empty or h is nil.
// Re-registering an existing name replaces the previous handler.
func (r *HandlerRegistry) Handle(name string, h Handler) {
	if name == "" {
		panic("jobs: handler name must not be empty")
	}
	if h == nil {
		panic("jobs: handler must not be nil")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.handlers[name] = h
}

// Lookup returns the handler registered under name, or a non-nil error if absent.
func (r *HandlerRegistry) Lookup(name string) (Handler, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	h, ok := r.handlers[name]
	if !ok {
		return nil, fmt.Errorf("jobs: handler not found: %q", name)
	}
	return h, nil
}

// Names returns all registered handler names in ascending order.
func (r *HandlerRegistry) Names() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	names := make([]string, 0, len(r.handlers))
	for name := range r.handlers {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// Has reports whether a handler is registered under name.
func (r *HandlerRegistry) Has(name string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	_, ok := r.handlers[name]
	return ok
}
