package bridge

import (
	"bytes"
	"context"
	"encoding/json"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/frob/nullspace/core/nslog"
	"github.com/frob/nullspace/core/tcp"
	"github.com/frob/nullspace/kernel"
	"github.com/frob/nullspace/module/data/file"
)

// tcpResponse is the wire format for a JSON-lines response.
type tcpResponse struct {
	Command string          `json:"command"`
	Payload json.RawMessage `json:"payload"`
}

// setup creates a kernel with file module, TCP adapter, and bridge module.
// It returns the TCP address and a cleanup function.
func setup(t *testing.T) (string, func()) {
	t.Helper()

	// Create content directory with a test entity.
	contentDir := t.TempDir()
	postsDir := filepath.Join(contentDir, "posts")
	os.MkdirAll(postsDir, 0755)
	os.WriteFile(filepath.Join(postsDir, "hello.json"), []byte(`{"title":"Hello World"}`), 0644)

	// Create config file.
	configDir := t.TempDir()
	tomlPath := filepath.Join(configDir, "nullspace.toml")
	os.WriteFile(tomlPath, []byte(`
[modules]
tcp = true
"data.file" = true
"data.bridge" = true

[tcp]
codec = "json-lines"

[data.file]
dir = "`+contentDir+`"
`), 0644)

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	addr := ln.Addr().String()

	k := kernel.New(kernel.WithConfigFile(tomlPath))
	k.Use(nslog.New())
	k.Use(file.New())
	k.Use(tcp.NewAdapterWithListener(ln))
	k.Use(New())

	ctx := context.Background()
	if err := k.Init(ctx); err != nil {
		t.Fatalf("init: %v", err)
	}
	if err := k.Start(ctx); err != nil {
		t.Fatalf("start: %v", err)
	}

	return addr, func() { k.Stop(ctx) }
}

// sendCommand sends a JSON-lines command and reads the response.
func sendCommand(t *testing.T, conn net.Conn, command string, payload any) tcpResponse {
	t.Helper()

	var payloadBytes []byte
	if payload != nil {
		var err error
		payloadBytes, err = json.Marshal(payload)
		if err != nil {
			t.Fatalf("marshal payload: %v", err)
		}
	}

	codec := tcp.JSONLinesCodec{}
	msg, err := codec.Encode(command, payloadBytes)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	if _, err := conn.Write(msg); err != nil {
		t.Fatalf("write: %v", err)
	}

	conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	buf := make([]byte, 65536)
	n, err := conn.Read(buf)
	if err != nil {
		t.Fatalf("read: %v", err)
	}

	var resp tcpResponse
	if err := json.Unmarshal(buf[:n], &resp); err != nil {
		t.Fatalf("unmarshal response: %v (data: %q)", err, buf[:n])
	}
	return resp
}

// payloadMap parses a response payload as a map.
func payloadMap(t *testing.T, resp tcpResponse) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal(resp.Payload, &m); err != nil {
		t.Fatalf("unmarshal payload: %v (data: %q)", err, resp.Payload)
	}
	return m
}

func dial(t *testing.T, addr string) net.Conn {
	t.Helper()
	conn, err := net.DialTimeout("tcp", addr, time.Second)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	return conn
}

// --- List Tests ---

func TestListEntities(t *testing.T) {
	addr, cleanup := setup(t)
	defer cleanup()

	conn := dial(t, addr)
	defer conn.Close()

	resp := sendCommand(t, conn, "data.list", map[string]string{"collection": "posts"})
	if resp.Command != "data.list" {
		t.Fatalf("command = %q, want data.list", resp.Command)
	}

	m := payloadMap(t, resp)
	items, ok := m["Items"].([]any)
	if !ok {
		t.Fatalf("Items not a slice: %v", m)
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(items))
	}
	if m["Collection"] != "posts" {
		t.Fatalf("Collection = %v, want posts", m["Collection"])
	}
}

func TestListEmptyCollection(t *testing.T) {
	addr, cleanup := setup(t)
	defer cleanup()

	conn := dial(t, addr)
	defer conn.Close()

	resp := sendCommand(t, conn, "data.list", map[string]string{"collection": "nonexistent"})
	m := payloadMap(t, resp)
	items, ok := m["Items"].([]any)
	if !ok {
		t.Fatalf("Items not a slice: %v", m)
	}
	if len(items) != 0 {
		t.Fatalf("expected 0 items, got %d", len(items))
	}
}

func TestListMissingCollection(t *testing.T) {
	addr, cleanup := setup(t)
	defer cleanup()

	conn := dial(t, addr)
	defer conn.Close()

	resp := sendCommand(t, conn, "data.list", map[string]string{})
	m := payloadMap(t, resp)
	if _, ok := m["error"]; !ok {
		t.Fatal("expected error response")
	}
}

// --- Get Tests ---

func TestGetEntity(t *testing.T) {
	addr, cleanup := setup(t)
	defer cleanup()

	conn := dial(t, addr)
	defer conn.Close()

	resp := sendCommand(t, conn, "data.get", map[string]string{
		"collection": "posts",
		"id":         "hello",
	})
	m := payloadMap(t, resp)
	if m["ID"] != "hello" {
		t.Fatalf("ID = %v, want hello", m["ID"])
	}
}

func TestGetNotFound(t *testing.T) {
	addr, cleanup := setup(t)
	defer cleanup()

	conn := dial(t, addr)
	defer conn.Close()

	resp := sendCommand(t, conn, "data.get", map[string]string{
		"collection": "posts",
		"id":         "missing",
	})
	m := payloadMap(t, resp)
	if _, ok := m["error"]; !ok {
		t.Fatal("expected error for missing entity")
	}
	if m["status"] != float64(404) {
		t.Fatalf("status = %v, want 404", m["status"])
	}
}

func TestGetMissingID(t *testing.T) {
	addr, cleanup := setup(t)
	defer cleanup()

	conn := dial(t, addr)
	defer conn.Close()

	resp := sendCommand(t, conn, "data.get", map[string]string{"collection": "posts"})
	m := payloadMap(t, resp)
	if _, ok := m["error"]; !ok {
		t.Fatal("expected error for missing id")
	}
}

// --- Create Tests ---

func TestCreateEntity(t *testing.T) {
	addr, cleanup := setup(t)
	defer cleanup()

	conn := dial(t, addr)
	defer conn.Close()

	resp := sendCommand(t, conn, "data.create", map[string]any{
		"collection": "posts",
		"body":       map[string]any{"id": "new-post", "title": "New Post"},
	})
	m := payloadMap(t, resp)
	if m["id"] != "new-post" {
		t.Fatalf("id = %v, want new-post", m["id"])
	}
	if m["status"] != "created" {
		t.Fatalf("status = %v, want created", m["status"])
	}

	// Verify it can be read back.
	resp = sendCommand(t, conn, "data.get", map[string]string{
		"collection": "posts",
		"id":         "new-post",
	})
	m = payloadMap(t, resp)
	if m["ID"] != "new-post" {
		t.Fatalf("read-back ID = %v, want new-post", m["ID"])
	}
}

func TestCreateMissingBody(t *testing.T) {
	addr, cleanup := setup(t)
	defer cleanup()

	conn := dial(t, addr)
	defer conn.Close()

	resp := sendCommand(t, conn, "data.create", map[string]any{"collection": "posts"})
	m := payloadMap(t, resp)
	if _, ok := m["error"]; !ok {
		t.Fatal("expected error for missing body")
	}
}

func TestCreateMissingIDInBody(t *testing.T) {
	addr, cleanup := setup(t)
	defer cleanup()

	conn := dial(t, addr)
	defer conn.Close()

	resp := sendCommand(t, conn, "data.create", map[string]any{
		"collection": "posts",
		"body":       map[string]any{"title": "No ID"},
	})
	m := payloadMap(t, resp)
	if _, ok := m["error"]; !ok {
		t.Fatal("expected error for missing id in body")
	}
}

// --- Update Tests ---

func TestUpdateEntity(t *testing.T) {
	addr, cleanup := setup(t)
	defer cleanup()

	conn := dial(t, addr)
	defer conn.Close()

	resp := sendCommand(t, conn, "data.update", map[string]any{
		"collection": "posts",
		"id":         "hello",
		"body":       map[string]any{"title": "Updated Title"},
	})
	m := payloadMap(t, resp)
	if m["status"] != "updated" {
		t.Fatalf("status = %v, want updated", m["status"])
	}
}

func TestUpdateMissingID(t *testing.T) {
	addr, cleanup := setup(t)
	defer cleanup()

	conn := dial(t, addr)
	defer conn.Close()

	resp := sendCommand(t, conn, "data.update", map[string]any{
		"collection": "posts",
		"body":       map[string]any{"title": "No ID"},
	})
	m := payloadMap(t, resp)
	if _, ok := m["error"]; !ok {
		t.Fatal("expected error for missing id")
	}
}

// --- Delete Tests ---

func TestDeleteEntity(t *testing.T) {
	addr, cleanup := setup(t)
	defer cleanup()

	conn := dial(t, addr)
	defer conn.Close()

	resp := sendCommand(t, conn, "data.delete", map[string]string{
		"collection": "posts",
		"id":         "hello",
	})
	m := payloadMap(t, resp)
	if m["status"] != "deleted" {
		t.Fatalf("status = %v, want deleted", m["status"])
	}

	// Verify it's gone.
	resp = sendCommand(t, conn, "data.get", map[string]string{
		"collection": "posts",
		"id":         "hello",
	})
	m = payloadMap(t, resp)
	if _, ok := m["error"]; !ok {
		t.Fatal("expected 404 after delete")
	}
}

func TestDeleteNotFound(t *testing.T) {
	addr, cleanup := setup(t)
	defer cleanup()

	conn := dial(t, addr)
	defer conn.Close()

	resp := sendCommand(t, conn, "data.delete", map[string]string{
		"collection": "posts",
		"id":         "nonexistent",
	})
	m := payloadMap(t, resp)
	if _, ok := m["error"]; !ok {
		t.Fatal("expected error for missing entity")
	}
	if m["status"] != float64(404) {
		t.Fatalf("status = %v, want 404", m["status"])
	}
}

// --- Validation Tests ---

func TestPathTraversalInID(t *testing.T) {
	addr, cleanup := setup(t)
	defer cleanup()

	conn := dial(t, addr)
	defer conn.Close()

	resp := sendCommand(t, conn, "data.get", map[string]string{
		"collection": "posts",
		"id":         "../secret",
	})
	m := payloadMap(t, resp)
	if _, ok := m["error"]; !ok {
		t.Fatal("expected error for path traversal")
	}
	if m["status"] != float64(400) {
		t.Fatalf("status = %v, want 400", m["status"])
	}
}

func TestPathTraversalInCollection(t *testing.T) {
	addr, cleanup := setup(t)
	defer cleanup()

	conn := dial(t, addr)
	defer conn.Close()

	resp := sendCommand(t, conn, "data.list", map[string]string{
		"collection": "../etc",
	})
	m := payloadMap(t, resp)
	if _, ok := m["error"]; !ok {
		t.Fatal("expected error for path traversal in collection")
	}
}

func TestInvalidPayload(t *testing.T) {
	addr, cleanup := setup(t)
	defer cleanup()

	conn := dial(t, addr)
	defer conn.Close()

	// Send a JSON string instead of the expected object.
	resp := sendCommand(t, conn, "data.get", "not an object")
	m := payloadMap(t, resp)
	if _, ok := m["error"]; !ok {
		t.Fatal("expected error for invalid payload structure")
	}
}

// --- Streaming Tests ---

// readAllResponses reads multiple responses from a TCP connection until timeout.
func readAllResponses(t *testing.T, conn net.Conn, count int) []tcpResponse {
	t.Helper()
	var responses []tcpResponse

	conn.SetReadDeadline(time.Now().Add(3 * time.Second))
	buf := make([]byte, 65536)

	// Read all available data.
	var accumulated []byte
	for len(responses) < count {
		n, err := conn.Read(buf)
		if err != nil {
			t.Fatalf("read: %v (got %d responses so far)", err, len(responses))
		}
		accumulated = append(accumulated, buf[:n]...)

		// Try to parse complete JSON lines.
		for {
			idx := bytes.IndexByte(accumulated, '\n')
			if idx < 0 {
				break
			}
			line := accumulated[:idx]
			accumulated = accumulated[idx+1:]

			var resp tcpResponse
			if err := json.Unmarshal(line, &resp); err != nil {
				t.Fatalf("unmarshal response line: %v (data: %q)", err, line)
			}
			responses = append(responses, resp)
		}
	}

	return responses
}

func TestStreamList(t *testing.T) {
	addr, cleanup := setup(t)
	defer cleanup()

	conn := dial(t, addr)
	defer conn.Close()

	// Send a streaming list request.
	payload, _ := json.Marshal(map[string]any{
		"collection": "posts",
		"stream":     true,
	})
	codec := tcp.JSONLinesCodec{}
	msg, _ := codec.Encode("data.list", payload)
	conn.Write(msg)

	// Expect: start, 1 item, end = 3 messages.
	responses := readAllResponses(t, conn, 3)

	// Verify start envelope.
	if responses[0].Command != "data.list.start" {
		t.Fatalf("first command = %q, want data.list.start", responses[0].Command)
	}
	startPayload := payloadMap(t, responses[0])
	if startPayload["collection"] != "posts" {
		t.Fatalf("start collection = %v, want posts", startPayload["collection"])
	}
	if startPayload["total"] != float64(1) {
		t.Fatalf("start total = %v, want 1", startPayload["total"])
	}

	// Verify item.
	if responses[1].Command != "data.list.item" {
		t.Fatalf("second command = %q, want data.list.item", responses[1].Command)
	}
	itemPayload := payloadMap(t, responses[1])
	if itemPayload["ID"] != "hello" {
		t.Fatalf("item ID = %v, want hello", itemPayload["ID"])
	}

	// Verify end envelope.
	if responses[2].Command != "data.list.end" {
		t.Fatalf("third command = %q, want data.list.end", responses[2].Command)
	}
	endPayload := payloadMap(t, responses[2])
	if endPayload["count"] != float64(1) {
		t.Fatalf("end count = %v, want 1", endPayload["count"])
	}
}

func TestStreamListEmpty(t *testing.T) {
	addr, cleanup := setup(t)
	defer cleanup()

	conn := dial(t, addr)
	defer conn.Close()

	payload, _ := json.Marshal(map[string]any{
		"collection": "nonexistent",
		"stream":     true,
	})
	codec := tcp.JSONLinesCodec{}
	msg, _ := codec.Encode("data.list", payload)
	conn.Write(msg)

	// Expect: start + end = 2 messages (no items).
	responses := readAllResponses(t, conn, 2)

	if responses[0].Command != "data.list.start" {
		t.Fatalf("first command = %q, want data.list.start", responses[0].Command)
	}
	startPayload := payloadMap(t, responses[0])
	if startPayload["total"] != float64(0) {
		t.Fatalf("start total = %v, want 0", startPayload["total"])
	}

	if responses[1].Command != "data.list.end" {
		t.Fatalf("second command = %q, want data.list.end", responses[1].Command)
	}
	endPayload := payloadMap(t, responses[1])
	if endPayload["count"] != float64(0) {
		t.Fatalf("end count = %v, want 0", endPayload["count"])
	}
}

func TestStreamListNonStreamingStillWorks(t *testing.T) {
	addr, cleanup := setup(t)
	defer cleanup()

	conn := dial(t, addr)
	defer conn.Close()

	// Without stream=true, should get normal batch response.
	resp := sendCommand(t, conn, "data.list", map[string]string{"collection": "posts"})
	if resp.Command != "data.list" {
		t.Fatalf("command = %q, want data.list", resp.Command)
	}
	m := payloadMap(t, resp)
	if _, ok := m["Items"]; !ok {
		t.Fatal("expected Items in batch response")
	}
}
