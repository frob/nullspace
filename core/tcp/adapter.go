// Package tcp provides a TCP transport adapter for the nullspace framework.
//
// The adapter is a kernel.Module that listens for TCP connections, decodes
// messages using a configurable codec, and dispatches them to registered
// handlers via a command-name router.
//
// Enable in nullspace.toml:
//
//	[modules]
//	tcp = true
//
//	[tcp]
//	addr             = ":9090"
//	codec            = "json-lines"     # or "length-prefix"
//	max_message_size = 1048576
//
// Register handlers in your application module's Init:
//
//	tcpRouter, _ := kernel.GetResource[*tcp.Router](k, "tcp.router")
//	tcpRouter.Handle("echo", func(conn *tcp.Conn, cmd string, payload []byte) error {
//	    return conn.Send(cmd, payload)
//	})
//
// Hooks fired:
//   - tcp.connected    — new connection accepted
//   - tcp.message      — message received and dispatched
//   - tcp.disconnected — connection closed
//   - tcp.error        — connection error (non-clean close)
package tcp

import (
	"bufio"
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net"
	"sync"

	"github.com/frob/nullspace/core/nslog"
	"github.com/frob/nullspace/kernel"
)

// AdapterConfig holds the TCP adapter's configuration.
type AdapterConfig struct {
	Addr           string `json:"addr" toml:"addr"`
	Codec          string `json:"codec" toml:"codec"`
	MaxMessageSize int    `json:"max_message_size" toml:"max_message_size"`
}

// Adapter is the TCP transport adapter module. It manages a TCP listener,
// accepts connections, and dispatches decoded messages to handlers.
type Adapter struct {
	kernel   *kernel.Kernel
	router   *Router
	codec    Codec
	listener net.Listener
	addr     string

	mu    sync.Mutex
	conns map[string]*Conn
	wg    sync.WaitGroup
}

// NewAdapter creates a new TCP adapter module with an empty router.
func NewAdapter() *Adapter {
	return &Adapter{
		router: NewRouter(),
		conns:  make(map[string]*Conn),
	}
}

// NewAdapterWithListener creates a TCP adapter using a pre-existing listener.
// This is useful for IPC (Unix sockets) or testing.
func NewAdapterWithListener(ln net.Listener) *Adapter {
	return &Adapter{
		router:   NewRouter(),
		conns:    make(map[string]*Conn),
		listener: ln,
	}
}

func (a *Adapter) Name() string { return "tcp" }

// Protocol implements transport.Listener.
func (a *Adapter) Protocol() string { return "tcp" }

// Addr implements transport.Listener.
func (a *Adapter) Addr() string { return a.addr }

func (a *Adapter) Config() kernel.ModuleConfig {
	return kernel.ModuleConfig{
		Key: "tcp",
		Default: AdapterConfig{
			Addr:           ":9090",
			Codec:          "json-lines",
			MaxMessageSize: 1048576,
		},
		DefaultEnabled: false,
	}
}

func (a *Adapter) Init(k *kernel.Kernel) error {
	a.kernel = k

	var cfg AdapterConfig
	if err := k.Config().Decode("tcp", &cfg); err != nil {
		cfg = AdapterConfig{Addr: ":9090", Codec: "json-lines", MaxMessageSize: 1048576}
	}
	a.addr = cfg.Addr

	// Only set codec from config if not already set (e.g., by NewAdapterWithListener).
	if a.codec == nil {
		codec, err := CodecByName(cfg.Codec)
		if err != nil {
			return err
		}
		a.codec = codec
	}

	k.Provide("tcp.router", a.router)
	k.Provide("transport.tcp", a)

	return nil
}

func (a *Adapter) Start(ctx context.Context) error {
	// If no listener was provided (normal case), create one.
	if a.listener == nil {
		ln, err := net.Listen("tcp", a.addr)
		if err != nil {
			return fmt.Errorf("tcp: listen %s: %w", a.addr, err)
		}
		a.listener = ln
	}
	a.addr = a.listener.Addr().String()

	a.wg.Add(1)
	go a.acceptLoop()

	a.kernel.Logger().Info("tcp server listening", "addr", a.addr)
	return nil
}

func (a *Adapter) Stop(ctx context.Context) error {
	// Close the listener to stop accepting new connections.
	if a.listener != nil {
		_ = a.listener.Close()
	}

	// Close all active connections.
	a.mu.Lock()
	for _, conn := range a.conns {
		_ = conn.Close()
	}
	a.mu.Unlock()

	// Wait for all connection goroutines to finish.
	a.wg.Wait()

	a.kernel.Logger().Info("tcp server stopped")
	return nil
}

// Router returns the adapter's command router.
func (a *Adapter) Router() *Router {
	return a.router
}

// SetCodec sets the codec used for framing. Call before Init if you want
// to override the config-driven codec.
func (a *Adapter) SetCodec(c Codec) {
	a.codec = c
}

// SetListener sets a pre-existing listener. Call before Start. This is
// useful for IPC adapters that create their own listener type.
func (a *Adapter) SetListener(ln net.Listener) {
	a.listener = ln
}

func (a *Adapter) acceptLoop() {
	defer a.wg.Done()

	for {
		raw, err := a.listener.Accept()
		if err != nil {
			// Listener closed during shutdown.
			if errors.Is(err, net.ErrClosed) {
				return
			}
			a.kernel.Logger().Error("tcp accept error", "error", err)
			continue
		}

		a.wg.Add(1)
		go a.handleConn(raw)
	}
}

func (a *Adapter) handleConn(raw net.Conn) {
	defer a.wg.Done()

	connID := generateID()
	connCtx, cancel := context.WithCancel(context.Background())

	// Attach config snapshot and logger to context.
	snap := a.kernel.Config().Snapshot()
	connCtx = kernel.ContextWithSnapshot(connCtx, snap)
	connLogger := a.kernel.Logger().With(
		"conn_id", connID,
		"remote", raw.RemoteAddr().String(),
	)
	connCtx = nslog.WithLogger(connCtx, connLogger)

	conn := &Conn{
		ID:     connID,
		raw:    raw,
		reader: bufio.NewReader(raw),
		ctx:    connCtx,
		cancel: cancel,
		logger: connLogger,
		codec:  a.codec,
		state:  make(map[string]any),
	}

	a.mu.Lock()
	a.conns[connID] = conn
	a.mu.Unlock()

	_ = a.kernel.Fire("tcp.connected", connCtx)
	connLogger.Info("tcp connected")

	// Message loop.
	var loopErr error
	for {
		command, payload, err := a.codec.Decode(conn.reader)
		if err != nil {
			if err != io.EOF && !errors.Is(err, net.ErrClosed) && connCtx.Err() == nil {
				loopErr = err
			}
			break
		}

		_ = a.kernel.Fire("tcp.message", connCtx)

		if err := a.router.Dispatch(conn, command, payload); err != nil {
			connLogger.Error("tcp handler error", "command", command, "error", err)
		}
	}

	// Cleanup.
	a.mu.Lock()
	delete(a.conns, connID)
	a.mu.Unlock()
	cancel()
	_ = raw.Close()

	if loopErr != nil {
		_ = a.kernel.Fire("tcp.error", connCtx)
		connLogger.Warn("tcp connection error", "error", loopErr)
	}

	_ = a.kernel.Fire("tcp.disconnected", connCtx)
	connLogger.Info("tcp disconnected")
}

// ConnectionCount returns the number of active connections.
func (a *Adapter) ConnectionCount() int {
	a.mu.Lock()
	defer a.mu.Unlock()
	return len(a.conns)
}

func generateID() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
