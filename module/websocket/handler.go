package websocket

import (
	"context"
	"crypto/rand"
	"encoding/hex"

	"github.com/frob/nullspace/core/request"
	"nhooyr.io/websocket"
)

// HandlerFunc is the callback invoked for each message received on a
// WebSocket connection. Return a non-nil error to close the connection.
type HandlerFunc func(conn *Conn, msg Message) error

// upgradeHandler returns a request.HandlerFunc that upgrades the HTTP
// connection to WebSocket and runs the message loop with the named handler.
func (m *Module) upgradeHandler(name string, wsHandler HandlerFunc) request.HandlerFunc {
	return func(ctx *request.Context) error {
		opts := &websocket.AcceptOptions{
			InsecureSkipVerify: m.cfg.InsecureSkipVerify,
		}
		if len(m.cfg.AllowedOrigins) > 0 && !m.cfg.InsecureSkipVerify {
			opts.OriginPatterns = m.cfg.AllowedOrigins
		}

		ws, err := websocket.Accept(ctx.Writer, ctx.Request, opts)
		if err != nil {
			ctx.Logger().Error("websocket upgrade failed", "handler", name, "error", err)
			return nil // Accept already wrote the HTTP error response
		}

		// Mark the connection as hijacked after a successful upgrade.
		// The adapter will skip post-handler hooks and error responses.
		ctx.Hijack()

		if m.cfg.MaxMessageSize > 0 {
			ws.SetReadLimit(m.cfg.MaxMessageSize)
		}

		connCtx, cancel := newConnContext(ctx)

		conn := &Conn{
			ID:     generateConnID(),
			ws:     ws,
			ctx:    connCtx,
			cancel: cancel,
			logger: ctx.Logger().With("ws_conn", name),
			state:  copyState(ctx),
		}

		m.manager.Add(conn)

		// Auto-join rooms from route metadata.
		if route := ctx.Route(); route != nil {
			if rooms := route.Meta["ws_rooms"]; rooms != "" {
				m.manager.Join(conn, rooms)
			}
		}

		_ = m.kernel.Fire("websocket.connected", connCtx)
		conn.logger.Info("websocket connected", "conn_id", conn.ID)

		// Message loop.
		var loopErr error
		for {
			typ, data, err := ws.Read(connCtx)
			if err != nil {
				loopErr = err
				break
			}

			msg := Message{Type: typ, Data: data}
			_ = m.kernel.Fire("websocket.message", connCtx)

			if err := wsHandler(conn, msg); err != nil {
				loopErr = err
				break
			}
		}

		// Cleanup.
		m.manager.Remove(conn)
		cancel()

		if loopErr != nil && websocket.CloseStatus(loopErr) == -1 {
			// Not a clean close — fire error hook.
			_ = m.kernel.Fire("websocket.error", connCtx)
			conn.logger.Warn("websocket error", "conn_id", conn.ID, "error", loopErr)
		}

		_ = m.kernel.Fire("websocket.disconnected", connCtx)
		conn.logger.Info("websocket disconnected", "conn_id", conn.ID)

		return nil
	}
}

// newConnContext creates a cancellable context derived from the request context.
func newConnContext(ctx *request.Context) (context.Context, context.CancelFunc) {
	return context.WithCancel(ctx.Context())
}

// copyState copies the request state bag into a new map for the connection.
// This captures all state set by middleware that ran before the upgrade.
func copyState(ctx *request.Context) map[string]any {
	return ctx.StateAll()
}

// generateConnID creates a short random hex ID for connection tracing.
func generateConnID() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
