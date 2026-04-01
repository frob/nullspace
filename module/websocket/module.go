// Package websocket provides WebSocket support for the nullspace framework.
//
// The module is opt-in. Enable it in nullspace.toml:
//
//	[modules]
//	websocket = true
//
//	[websocket]
//	max_message_size    = 65536
//	allowed_origins     = ["example.com"]
//	insecure_skip_verify = false
//
// Register WebSocket handlers in your application module's Init:
//
//	wsMod, _ := kernel.GetResource[*websocket.Module](k, "websocket")
//	wsMod.HandleFunc("chat", func(conn *websocket.Conn, msg websocket.Message) error {
//	    mgr, _ := kernel.GetResource[*websocket.Manager](k, "websocket.manager")
//	    mgr.BroadcastTo("chat", msg)
//	    return nil
//	})
//
// Then reference the handler in a route:
//
//	[[routing.routes]]
//	path = "/ws/chat"
//	handler = "ws.chat"
//	middleware = ["session.load"]
//	ws = "true"
//	ws_rooms = "chat"
//
// Hooks fired:
//   - websocket.connected    — new connection established
//   - websocket.message      — message received
//   - websocket.disconnected — connection closed
//   - websocket.error        — non-clean close error
package websocket

import (
	"context"
	"fmt"
	"sync"

	"github.com/frob/nullspace/core/routing"
	"github.com/frob/nullspace/kernel"
	"nhooyr.io/websocket"
)

// Config holds the websocket module's configuration.
type Config struct {
	// MaxMessageSize is the maximum message size in bytes. 0 means no limit.
	MaxMessageSize int64 `json:"max_message_size" toml:"max_message_size"`
	// AllowedOrigins is a list of allowed origin patterns for the upgrade handshake.
	// If empty and InsecureSkipVerify is false, only same-origin requests are accepted.
	AllowedOrigins []string `json:"allowed_origins" toml:"allowed_origins"`
	// InsecureSkipVerify disables origin verification. Use only for development.
	InsecureSkipVerify bool `json:"insecure_skip_verify" toml:"insecure_skip_verify"`
}

// Module provides WebSocket support. It maintains a handler registry and
// connection manager, and registers named handlers on the routing registry
// during Init.
type Module struct {
	kernel  *kernel.Kernel
	cfg     Config
	manager *Manager

	mu       sync.RWMutex
	handlers map[string]HandlerFunc
}

// New creates a new websocket module.
func New() *Module {
	return &Module{
		manager:  NewManager(),
		handlers: make(map[string]HandlerFunc),
	}
}

func (m *Module) Name() string { return "websocket" }

func (m *Module) Config() kernel.ModuleConfig {
	return kernel.ModuleConfig{
		Key: "websocket",
		Default: Config{
			MaxMessageSize: 65536,
		},
		DefaultEnabled: false,
	}
}

func (m *Module) Init(k *kernel.Kernel) error {
	m.kernel = k

	if err := k.Config().Decode("websocket", &m.cfg); err != nil {
		m.cfg = Config{MaxMessageSize: 65536}
	}

	k.Provide("websocket", m)
	k.Provide("websocket.manager", m.manager)

	// Register already-added handlers on the routing registry after all
	// modules have finished Init, so application modules can call
	// HandleFunc during their own Init.
	k.Hook("kernel.after_init", 5, func(ctx context.Context) error {
		return m.registerHandlers(k)
	})

	k.Logger().Info("websocket module initialized",
		"max_message_size", m.cfg.MaxMessageSize,
		"insecure_skip_verify", m.cfg.InsecureSkipVerify)

	return nil
}

func (m *Module) Start(ctx context.Context) error { return nil }

func (m *Module) Stop(ctx context.Context) error {
	m.manager.CloseAll(websocket.StatusGoingAway, "server shutting down")
	m.kernel.Logger().Info("websocket connections closed", "count", m.manager.Count())
	return nil
}

// HandleFunc registers a named WebSocket handler. Call this during your
// module's Init. The handler is registered on the routing registry as
// "ws.<name>" after all modules have initialized.
func (m *Module) HandleFunc(name string, h HandlerFunc) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.handlers[name] = h
}

// Manager returns the connection manager.
func (m *Module) Manager() *Manager {
	return m.manager
}

// registerHandlers wires each named WS handler into the routing registry
// as a standard request.HandlerFunc that performs the upgrade.
func (m *Module) registerHandlers(k *kernel.Kernel) error {
	reg, err := kernel.GetResource[*routing.Registry](k, "routing.registry")
	if err != nil {
		return fmt.Errorf("websocket: routing registry not found — register routing module before websocket: %w", err)
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	for name, h := range m.handlers {
		regName := "ws." + name
		reg.HandleFunc(regName, m.upgradeHandler(name, h))
		k.Logger().Info("websocket handler registered", "name", regName)
	}

	return nil
}
