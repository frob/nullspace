package tcp

import (
	"context"
	"encoding/json"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/frob/nullspace/kernel"
)

func setupTCPKernel(t *testing.T) (*Adapter, *kernel.Kernel, string) {
	t.Helper()

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}

	dir := t.TempDir()
	tomlPath := filepath.Join(dir, "nullspace.toml")
	toml := `
[modules]
tcp = true
`
	if err := os.WriteFile(tomlPath, []byte(toml), 0644); err != nil {
		t.Fatal(err)
	}

	k := kernel.New(kernel.WithConfigFile(tomlPath))
	adapter := NewAdapterWithListener(ln)
	k.Use(adapter)

	return adapter, k, ln.Addr().String()
}

func TestAdapterLifecycle(t *testing.T) {
	adapter, k, addr := setupTCPKernel(t)

	// Register an echo handler.
	adapter.Router().Handle("echo", func(conn *Conn, cmd string, payload []byte) error {
		return conn.Send(cmd, payload)
	})

	ctx := context.Background()
	if err := k.Init(ctx); err != nil {
		t.Fatalf("init: %v", err)
	}
	if err := k.Start(ctx); err != nil {
		t.Fatalf("start: %v", err)
	}

	// Connect a client.
	clientConn, err := net.DialTimeout("tcp", addr, time.Second)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}

	// Send an echo message using JSON lines.
	codec := JSONLinesCodec{}
	msg, _ := codec.Encode("echo", []byte(`"hello"`))
	if _, err := clientConn.Write(msg); err != nil {
		t.Fatalf("write: %v", err)
	}

	// Read the response.
	clientConn.SetReadDeadline(time.Now().Add(2 * time.Second))
	buf := make([]byte, 1024)
	n, err := clientConn.Read(buf)
	if err != nil {
		t.Fatalf("read: %v", err)
	}

	// Parse the response.
	var resp struct {
		Command string          `json:"command"`
		Payload json.RawMessage `json:"payload"`
	}
	if err := json.Unmarshal(buf[:n], &resp); err != nil {
		t.Fatalf("unmarshal: %v (data: %q)", err, buf[:n])
	}
	if resp.Command != "echo" {
		t.Errorf("response command = %q, want %q", resp.Command, "echo")
	}
	if string(resp.Payload) != `"hello"` {
		t.Errorf("response payload = %q, want %q", string(resp.Payload), `"hello"`)
	}

	clientConn.Close()
	time.Sleep(50 * time.Millisecond)

	if err := k.Stop(ctx); err != nil {
		t.Fatalf("stop: %v", err)
	}
}

func TestAdapterConnectionCount(t *testing.T) {
	adapter, k, addr := setupTCPKernel(t)
	adapter.Router().Handle("noop", func(*Conn, string, []byte) error { return nil })

	ctx := context.Background()
	if err := k.Init(ctx); err != nil {
		t.Fatalf("init: %v", err)
	}
	if err := k.Start(ctx); err != nil {
		t.Fatalf("start: %v", err)
	}

	// Open two connections.
	c1, err := net.DialTimeout("tcp", addr, time.Second)
	if err != nil {
		t.Fatalf("dial c1: %v", err)
	}
	c2, err := net.DialTimeout("tcp", addr, time.Second)
	if err != nil {
		t.Fatalf("dial c2: %v", err)
	}

	// Give the server a moment to register connections.
	time.Sleep(50 * time.Millisecond)

	if n := adapter.ConnectionCount(); n != 2 {
		t.Errorf("connection count = %d, want 2", n)
	}

	// Close one.
	c1.Close()
	time.Sleep(50 * time.Millisecond)

	if n := adapter.ConnectionCount(); n != 1 {
		t.Errorf("connection count = %d, want 1", n)
	}

	c2.Close()
	time.Sleep(50 * time.Millisecond)

	if err := k.Stop(ctx); err != nil {
		t.Fatalf("stop: %v", err)
	}
}
