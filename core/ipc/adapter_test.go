package ipc

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/frob/nullspace/core/tcp"
	"github.com/frob/nullspace/kernel"
)

func TestIPCAdapterLifecycle(t *testing.T) {
	dir := t.TempDir()
	sockPath := filepath.Join(dir, "test.sock")

	// Write a temp config file.
	configPath := filepath.Join(dir, "nullspace.toml")
	configContent := fmt.Sprintf(`
[modules]
ipc = true

[ipc]
path = %q
codec = "json-lines"
`, sockPath)
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	k := kernel.New(kernel.WithConfigFile(configPath))

	adapter := New()

	// Register an echo handler.
	adapter.Router().Handle("echo", func(conn *tcp.Conn, cmd string, payload []byte) error {
		return conn.Send(cmd, payload)
	})

	k.Use(adapter)

	ctx := context.Background()
	if err := k.Init(ctx); err != nil {
		t.Fatalf("init: %v", err)
	}
	if err := k.Start(ctx); err != nil {
		t.Fatalf("start: %v", err)
	}

	// Connect via Unix socket.
	clientConn, err := net.DialTimeout("unix", sockPath, time.Second)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}

	// Send an echo message.
	codec := tcp.JSONLinesCodec{}
	msg, _ := codec.Encode("echo", []byte(`"ipc-test"`))
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
	if string(resp.Payload) != `"ipc-test"` {
		t.Errorf("response payload = %q, want %q", string(resp.Payload), `"ipc-test"`)
	}

	clientConn.Close()
	time.Sleep(50 * time.Millisecond)

	if err := k.Stop(ctx); err != nil {
		t.Fatalf("stop: %v", err)
	}

	// Socket file should be cleaned up.
	if _, err := os.Stat(sockPath); !os.IsNotExist(err) {
		t.Error("socket file should be removed after stop")
	}
}
