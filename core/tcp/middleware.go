package tcp

// HandlerFunc is the callback invoked for each message received on a TCP
// connection. The command is the dispatched command name extracted by the
// codec. Return a non-nil error to close the connection.
type HandlerFunc func(conn *Conn, command string, payload []byte) error

// Middleware wraps a HandlerFunc, allowing pre/post processing of TCP
// messages (e.g., authentication, logging, rate limiting).
type Middleware func(HandlerFunc) HandlerFunc

// buildChain wraps a handler with middleware in reverse order so that
// the first middleware in the slice executes first.
func buildChain(h HandlerFunc, mw []Middleware) HandlerFunc {
	for i := len(mw) - 1; i >= 0; i-- {
		h = mw[i](h)
	}
	return h
}
