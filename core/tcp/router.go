package tcp

import (
	"fmt"
	"sync"
)

// Router dispatches incoming TCP messages to named handlers based on the
// command field extracted by the codec. Unlike the HTTP router which matches
// method + URL path, the TCP router uses simple command-name lookup.
type Router struct {
	mu       sync.RWMutex
	handlers map[string]HandlerFunc
	mw       []Middleware // global middleware applied to all handlers
}

// NewRouter creates an empty TCP router.
func NewRouter() *Router {
	return &Router{
		handlers: make(map[string]HandlerFunc),
	}
}

// Handle registers a handler for a command name.
func (r *Router) Handle(command string, h HandlerFunc) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.handlers[command] = h
}

// Use adds global middleware that wraps all handlers.
func (r *Router) Use(mw ...Middleware) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.mw = append(r.mw, mw...)
}

// Dispatch looks up the handler for a command and executes it with any
// registered middleware. Returns an error if no handler is registered.
func (r *Router) Dispatch(conn *Conn, command string, payload []byte) error {
	r.mu.RLock()
	h, ok := r.handlers[command]
	mw := r.mw
	r.mu.RUnlock()

	if !ok {
		return fmt.Errorf("tcp: unknown command %q", command)
	}

	if len(mw) > 0 {
		h = buildChain(h, mw)
	}

	return h(conn, command, payload)
}

// Commands returns a list of registered command names.
func (r *Router) Commands() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	cmds := make([]string, 0, len(r.handlers))
	for cmd := range r.handlers {
		cmds = append(cmds, cmd)
	}
	return cmds
}
