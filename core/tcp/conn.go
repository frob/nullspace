package tcp

import (
	"bufio"
	"context"
	"net"
	"sync"

	"github.com/frob/nullspace/kernel"
)

// Conn wraps a TCP connection with framework context. It carries a
// per-connection logger, config snapshot, and a connection-scoped state bag.
type Conn struct {
	// ID is a unique identifier for this connection.
	ID string

	raw    net.Conn
	reader *bufio.Reader
	ctx    context.Context
	cancel context.CancelFunc
	logger kernel.Logger
	codec  Codec

	mu    sync.RWMutex
	state map[string]any
}

// Send encodes and writes a command/payload to the peer.
func (c *Conn) Send(command string, payload []byte) error {
	frame, err := c.codec.Encode(command, payload)
	if err != nil {
		return err
	}
	_, err = c.raw.Write(frame)
	return err
}

// Close closes the underlying TCP connection and cancels the context.
func (c *Conn) Close() error {
	c.cancel()
	return c.raw.Close()
}

// Logger returns the per-connection logger.
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

// RemoteAddr returns the remote network address.
func (c *Conn) RemoteAddr() net.Addr {
	return c.raw.RemoteAddr()
}
