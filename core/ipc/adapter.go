// Package ipc provides a Unix domain socket transport adapter for the
// nullspace framework. It reuses the TCP adapter's connection handling,
// codec, and router — the only difference is the listener type.
//
// Enable in nullspace.toml:
//
//	[modules]
//	ipc = true
//
//	[ipc]
//	path             = "/var/run/myapp.sock"
//	codec            = "json-lines"
//	max_message_size = 1048576
//
// Register handlers on the shared TCP router:
//
//	ipcMod, _ := kernel.GetResource[*ipc.Adapter](k, "transport.ipc")
//	ipcMod.Router().Handle("ping", func(conn *tcp.Conn, cmd string, payload []byte) error {
//	    return conn.Send("pong", nil)
//	})
//
// Hooks fired: same as tcp.* (tcp.connected, tcp.message, etc.)
package ipc

import (
	"context"
	"fmt"
	"net"
	"os"

	"github.com/frob/nullspace/core/tcp"
	"github.com/frob/nullspace/kernel"
)

// AdapterConfig holds the IPC adapter's configuration.
type AdapterConfig struct {
	Path           string `json:"path" toml:"path"`
	Codec          string `json:"codec" toml:"codec"`
	MaxMessageSize int    `json:"max_message_size" toml:"max_message_size"`
}

// Adapter is the IPC (Unix domain socket) transport adapter module.
// It delegates connection handling to a tcp.Adapter with a Unix listener.
type Adapter struct {
	inner *tcp.Adapter
	path  string
}

// New creates a new IPC adapter module.
func New() *Adapter {
	return &Adapter{
		inner: tcp.NewAdapter(),
	}
}

func (a *Adapter) Name() string { return "ipc" }

// Protocol implements transport.Listener.
func (a *Adapter) Protocol() string { return "ipc" }

// Addr implements transport.Listener.
func (a *Adapter) Addr() string { return a.path }

// Router returns the underlying TCP command router.
func (a *Adapter) Router() *tcp.Router {
	return a.inner.Router()
}

func (a *Adapter) Config() kernel.ModuleConfig {
	return kernel.ModuleConfig{
		Key: "ipc",
		Default: AdapterConfig{
			Path:           "/tmp/nullspace.sock",
			Codec:          "json-lines",
			MaxMessageSize: 1048576,
		},
		DefaultEnabled: false,
	}
}

func (a *Adapter) Init(k *kernel.Kernel) error {
	var cfg AdapterConfig
	if err := k.Config().Decode("ipc", &cfg); err != nil {
		cfg = AdapterConfig{Path: "/tmp/nullspace.sock", Codec: "json-lines", MaxMessageSize: 1048576}
	}
	a.path = cfg.Path

	codec, err := tcp.CodecByName(cfg.Codec)
	if err != nil {
		return err
	}
	a.inner.SetCodec(codec)

	// Create the Unix socket listener.
	// Remove stale socket file if it exists.
	if _, err := os.Stat(a.path); err == nil {
		if err := os.Remove(a.path); err != nil {
			return fmt.Errorf("ipc: remove stale socket %s: %w", a.path, err)
		}
	}

	ln, err := net.Listen("unix", a.path)
	if err != nil {
		return fmt.Errorf("ipc: listen %s: %w", a.path, err)
	}

	// Set the Unix socket listener on the inner TCP adapter.
	a.inner.SetListener(ln)

	// Initialize the inner TCP adapter (registers router on service locator).
	if err := a.inner.Init(k); err != nil {
		_ = ln.Close()
		return err
	}

	// Also register ourselves for IPC-specific discovery.
	k.Provide("transport.ipc", a)

	return nil
}

func (a *Adapter) Start(ctx context.Context) error {
	return a.inner.Start(ctx)
}

func (a *Adapter) Stop(ctx context.Context) error {
	err := a.inner.Stop(ctx)
	// Clean up the socket file.
	_ = os.Remove(a.path)
	return err
}
