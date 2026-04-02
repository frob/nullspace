// Package transport defines minimal interfaces for cross-transport discovery
// and connection handling. Transport adapters (HTTP, TCP, IPC) implement these
// interfaces so that cross-cutting modules (metrics, health dashboards) can
// discover active listeners and interact with connections generically.
//
// These interfaces are opt-in. Transport adapters provide richer,
// protocol-specific types for their handlers. The interfaces here exist
// only for the common denominator needed by framework infrastructure.
package transport

import (
	"context"

	"github.com/frob/nullspace/kernel"
)

// Listener is an optional discovery interface for transport adapter modules.
// Modules that manage a network listener (HTTP, TCP, IPC, gRPC) implement
// this so that auxiliary modules can enumerate active transports.
type Listener interface {
	kernel.Module

	// Protocol returns the transport identifier (e.g., "http", "tcp", "ipc", "grpc").
	Protocol() string

	// Addr returns the listen address (e.g., ":8080", "/var/run/app.sock").
	Addr() string
}

// Conn is a minimal interface for a transport-level connection. Each transport
// adapter provides a richer, protocol-specific connection type; this interface
// captures only the common surface needed by cross-cutting concerns such as
// metrics, authentication, and logging.
//
// Read/Write are intentionally omitted — they are protocol-specific. Use the
// concrete connection type (tcp.Conn, websocket.Conn, etc.) for I/O.
type Conn interface {
	// Context returns the connection-scoped context, which is cancelled
	// when the connection closes.
	Context() context.Context

	// Logger returns the per-connection logger.
	Logger() kernel.Logger

	// State retrieves a value from the connection-scoped state bag.
	State(key string) (any, bool)

	// SetState stores a value in the connection-scoped state bag.
	SetState(key string, val any)

	// Close closes the connection.
	Close() error
}
