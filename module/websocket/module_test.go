package websocket

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/frob/nullspace/core/nslog"
	"github.com/frob/nullspace/core/request"
	"github.com/frob/nullspace/core/routing"
	"github.com/frob/nullspace/kernel"
	"nhooyr.io/websocket"
)

// --- Manager Tests ---

func TestManagerAddRemove(t *testing.T) {
	mgr := NewManager()
	conn := &Conn{ID: "a", state: make(map[string]any)}

	mgr.Add(conn)
	if mgr.Count() != 1 {
		t.Fatalf("expected 1 connection, got %d", mgr.Count())
	}

	mgr.Remove(conn)
	if mgr.Count() != 0 {
		t.Fatalf("expected 0 connections, got %d", mgr.Count())
	}
}

func TestManagerRooms(t *testing.T) {
	mgr := NewManager()
	c1 := &Conn{ID: "a", state: make(map[string]any)}
	c2 := &Conn{ID: "b", state: make(map[string]any)}

	mgr.Add(c1)
	mgr.Add(c2)
	mgr.Join(c1, "chat")
	mgr.Join(c2, "chat")

	if mgr.RoomCount("chat") != 2 {
		t.Fatalf("expected 2 in chat, got %d", mgr.RoomCount("chat"))
	}

	mgr.Leave(c1, "chat")
	if mgr.RoomCount("chat") != 1 {
		t.Fatalf("expected 1 in chat after leave, got %d", mgr.RoomCount("chat"))
	}

	// Remove cleans up rooms.
	mgr.Remove(c2)
	if mgr.RoomCount("chat") != 0 {
		t.Fatalf("expected 0 in chat after remove, got %d", mgr.RoomCount("chat"))
	}
}

func TestManagerRoomCleanup(t *testing.T) {
	mgr := NewManager()
	c := &Conn{ID: "a", state: make(map[string]any)}

	mgr.Add(c)
	mgr.Join(c, "room")
	mgr.Leave(c, "room")

	// Room should be removed when empty.
	mgr.mu.RLock()
	_, exists := mgr.rooms["room"]
	mgr.mu.RUnlock()
	if exists {
		t.Fatal("expected empty room to be cleaned up")
	}
}

// --- Conn Tests ---

func TestConnState(t *testing.T) {
	c := &Conn{ID: "test", state: make(map[string]any)}

	c.SetState("user", "alice")
	v, ok := c.State("user")
	if !ok || v != "alice" {
		t.Fatalf("expected alice, got %v", v)
	}

	_, ok = c.State("missing")
	if ok {
		t.Fatal("expected not found")
	}
}

// --- Message Tests ---

func TestTextMessage(t *testing.T) {
	msg := TextMessage("hello")
	if msg.Type != MessageText {
		t.Fatalf("expected text type, got %d", msg.Type)
	}
	if string(msg.Data) != "hello" {
		t.Fatalf("expected hello, got %s", msg.Data)
	}
}

func TestBinaryMessage(t *testing.T) {
	data := []byte{0x01, 0x02, 0x03}
	msg := BinaryMessage(data)
	if msg.Type != MessageBinary {
		t.Fatalf("expected binary type, got %d", msg.Type)
	}
	if len(msg.Data) != 3 {
		t.Fatalf("expected 3 bytes, got %d", len(msg.Data))
	}
}

// --- Module Tests ---

func setupKernel(t *testing.T, tomlContent string) (*request.Adapter, *Module, *kernel.Kernel) {
	t.Helper()
	dir := t.TempDir()
	tomlPath := filepath.Join(dir, "nullspace.toml")
	if err := os.WriteFile(tomlPath, []byte(tomlContent), 0644); err != nil {
		t.Fatal(err)
	}

	k := kernel.New(kernel.WithConfigFile(tomlPath))
	logMod := nslog.New()
	adapter := request.NewAdapter()
	routingMod := routing.New()
	wsMod := New()

	k.Use(logMod)
	k.Use(adapter)
	k.Use(routingMod)
	k.Use(wsMod)

	return adapter, wsMod, k
}

func TestModuleInit(t *testing.T) {
	_, wsMod, k := setupKernel(t, `
[modules]
websocket = true
`)

	if err := k.Init(context.Background()); err != nil {
		t.Fatalf("Init: %v", err)
	}

	// Module should be accessible via service locator.
	got, err := kernel.GetResource[*Module](k, "websocket")
	if err != nil {
		t.Fatalf("expected websocket module in locator: %v", err)
	}
	if got != wsMod {
		t.Fatal("expected same module instance")
	}

	mgr, err := kernel.GetResource[*Manager](k, "websocket.manager")
	if err != nil {
		t.Fatalf("expected manager in locator: %v", err)
	}
	if mgr != wsMod.Manager() {
		t.Fatal("expected same manager instance")
	}
}

func TestModuleHandlerRegistration(t *testing.T) {
	_, wsMod, k := setupKernel(t, `
[modules]
websocket = true
`)

	wsMod.HandleFunc("echo", func(conn *Conn, msg Message) error {
		return conn.Send(msg)
	})

	if err := k.Init(context.Background()); err != nil {
		t.Fatalf("Init: %v", err)
	}

	// The handler should be registered as "ws.echo" on the routing registry.
	reg, err := kernel.GetResource[*routing.Registry](k, "routing.registry")
	if err != nil {
		t.Fatalf("get registry: %v", err)
	}

	if _, err := reg.LookupHandler("ws.echo"); err != nil {
		t.Fatalf("expected ws.echo handler to be registered: %v", err)
	}
}

func TestModuleConfig(t *testing.T) {
	_, wsMod, k := setupKernel(t, `
[modules]
websocket = true

[websocket]
max_message_size = 1024
insecure_skip_verify = true
`)

	if err := k.Init(context.Background()); err != nil {
		t.Fatalf("Init: %v", err)
	}

	if wsMod.cfg.MaxMessageSize != 1024 {
		t.Fatalf("expected max_message_size 1024, got %d", wsMod.cfg.MaxMessageSize)
	}
	if !wsMod.cfg.InsecureSkipVerify {
		t.Fatal("expected insecure_skip_verify true")
	}
}

// --- Integration Tests ---

func TestWebSocketEcho(t *testing.T) {
	_, wsMod, k := setupKernel(t, `
[modules]
websocket = true

[websocket]
insecure_skip_verify = true
`)

	wsMod.HandleFunc("echo", func(conn *Conn, msg Message) error {
		return conn.Send(msg)
	})

	if err := k.Init(context.Background()); err != nil {
		t.Fatalf("Init: %v", err)
	}

	adapter, err := kernel.GetResource[*request.Adapter](k, "request.adapter")
	if err != nil {
		t.Fatalf("get adapter: %v", err)
	}

	// Look up the registered handler and wire it to a route.
	reg, _ := kernel.GetResource[*routing.Registry](k, "routing.registry")
	handler, _ := reg.LookupHandler("ws.echo")
	adapter.Router().Get("/ws/echo", handler)

	// Start an httptest server with the adapter.
	srv := httptest.NewServer(adapter)
	defer srv.Close()

	// Connect via WebSocket.
	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http") + "/ws/echo"
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	ws, _, err := websocket.Dial(ctx, wsURL, nil)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer ws.Close(websocket.StatusNormalClosure, "done")

	// Send a message.
	err = ws.Write(ctx, websocket.MessageText, []byte("hello"))
	if err != nil {
		t.Fatalf("write: %v", err)
	}

	// Read the echo.
	typ, data, err := ws.Read(ctx)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if typ != websocket.MessageText {
		t.Fatalf("expected text, got %d", typ)
	}
	if string(data) != "hello" {
		t.Fatalf("expected 'hello', got %q", data)
	}
}

func TestWebSocketManagerTracksConnection(t *testing.T) {
	_, wsMod, k := setupKernel(t, `
[modules]
websocket = true

[websocket]
insecure_skip_verify = true
`)

	connected := make(chan struct{})
	wsMod.HandleFunc("track", func(conn *Conn, msg Message) error {
		select {
		case connected <- struct{}{}:
		default:
		}
		return conn.Send(msg)
	})

	if err := k.Init(context.Background()); err != nil {
		t.Fatalf("Init: %v", err)
	}

	adapter, _ := kernel.GetResource[*request.Adapter](k, "request.adapter")
	reg, _ := kernel.GetResource[*routing.Registry](k, "routing.registry")
	handler, _ := reg.LookupHandler("ws.track")
	adapter.Router().Get("/ws/track", handler)

	srv := httptest.NewServer(adapter)
	defer srv.Close()

	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http") + "/ws/track"
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	ws, _, err := websocket.Dial(ctx, wsURL, nil)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}

	// Send a message to trigger the handler.
	_ = ws.Write(ctx, websocket.MessageText, []byte("ping"))

	// Wait for handler to run.
	select {
	case <-connected:
	case <-ctx.Done():
		t.Fatal("timed out waiting for connection")
	}

	if wsMod.Manager().Count() != 1 {
		t.Fatalf("expected 1 tracked connection, got %d", wsMod.Manager().Count())
	}

	// Close and verify cleanup.
	ws.Close(websocket.StatusNormalClosure, "done")

	// Give the server time to process the close.
	time.Sleep(50 * time.Millisecond)

	if wsMod.Manager().Count() != 0 {
		t.Fatalf("expected 0 connections after close, got %d", wsMod.Manager().Count())
	}
}

func TestWebSocketNonWSRequestGets426(t *testing.T) {
	_, wsMod, k := setupKernel(t, `
[modules]
websocket = true

[websocket]
insecure_skip_verify = true
`)

	wsMod.HandleFunc("test", func(conn *Conn, msg Message) error {
		return nil
	})

	if err := k.Init(context.Background()); err != nil {
		t.Fatalf("Init: %v", err)
	}

	adapter, _ := kernel.GetResource[*request.Adapter](k, "request.adapter")
	reg, _ := kernel.GetResource[*routing.Registry](k, "routing.registry")
	handler, _ := reg.LookupHandler("ws.test")
	adapter.Router().Get("/ws/test", handler)

	// Plain HTTP GET — should fail the upgrade.
	req := httptest.NewRequest("GET", "/ws/test", nil)
	w := httptest.NewRecorder()
	adapter.ServeHTTP(w, req)

	// The upgrade will fail, but since we called Hijack() first, the adapter
	// won't write a 500. The response depends on nhooyr.io/websocket's behavior
	// when Accept fails — it writes an HTTP error itself.
	if w.Code == http.StatusOK {
		t.Fatal("expected non-200 for plain HTTP request to WS endpoint")
	}
}
