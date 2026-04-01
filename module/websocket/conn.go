package websocket

import (
	"context"
	"sync"

	"github.com/frob/nullspace/kernel"
	"nhooyr.io/websocket"
)

// MessageType identifies the type of a WebSocket message.
type MessageType = websocket.MessageType

const (
	// MessageText is a UTF-8 text message.
	MessageText = websocket.MessageText
	// MessageBinary is a binary message.
	MessageBinary = websocket.MessageBinary
)

// Message holds a single WebSocket message.
type Message struct {
	Type MessageType
	Data []byte
}

// TextMessage creates a text Message from a string.
func TextMessage(s string) Message {
	return Message{Type: MessageText, Data: []byte(s)}
}

// BinaryMessage creates a binary Message from bytes.
func BinaryMessage(b []byte) Message {
	return Message{Type: MessageBinary, Data: b}
}

// Conn wraps a WebSocket connection with framework context. It carries
// the per-request logger, config snapshot, and a connection-scoped state
// bag populated by middleware that ran before the upgrade.
type Conn struct {
	ID     string
	ws     *websocket.Conn
	ctx    context.Context
	cancel context.CancelFunc
	logger kernel.Logger

	mu    sync.RWMutex
	state map[string]any
}

// Send writes a message to the WebSocket peer.
func (c *Conn) Send(msg Message) error {
	return c.ws.Write(c.ctx, msg.Type, msg.Data)
}

// Close performs a graceful WebSocket close handshake.
func (c *Conn) Close(code websocket.StatusCode, reason string) error {
	return c.ws.Close(code, reason)
}

// Logger returns the per-connection logger (inherited from the upgrade request).
func (c *Conn) Logger() kernel.Logger {
	return c.logger
}

// Context returns the connection's context, which is cancelled when the
// connection closes.
func (c *Conn) Context() context.Context {
	return c.ctx
}

// State retrieves a value from the connection-scoped state bag.
func (c *Conn) State(key string) (any, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	v, ok := c.state[key]
	return v, ok
}

// SetState stores a value in the connection-scoped state bag.
func (c *Conn) SetState(key string, val any) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.state[key] = val
}
