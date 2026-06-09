package jobs

import (
	"bytes"
	"context"
	"encoding/json"
	"net"
	"os"
	"path/filepath"
	"sort"
	"testing"
	"time"

	"github.com/frob/nullspace/core/nslog"
	"github.com/frob/nullspace/core/routing"
	"github.com/frob/nullspace/core/tcp"
	"github.com/frob/nullspace/kernel"
	"github.com/frob/nullspace/module/data/bridge"
	datafile "github.com/frob/nullspace/module/data/file"
)

// bridgeResponse is the wire format for a JSON-lines TCP response.
type bridgeResponse struct {
	Command string          `json:"command"`
	Payload json.RawMessage `json:"payload"`
}

// setupBridgeKernel boots a kernel with logging + routing + tcp + data.file +
// data.bridge + jobs (memory store). It returns the jobs module, kernel, tcp
// adapter, and the listen address of the live TCP server.
func setupBridgeKernel(t *testing.T) (*Module, *kernel.Kernel, *tcp.Adapter, string) {
	t.Helper()

	contentDir := t.TempDir()
	// data.file requires a directory; an empty one is fine.
	_ = os.MkdirAll(filepath.Join(contentDir, "noop"), 0755)

	dir := t.TempDir()
	tomlPath := filepath.Join(dir, "nullspace.toml")
	toml := `
[modules]
tcp = true
"data.file" = true
"data.bridge" = true
jobs = true

[tcp]
codec = "json-lines"

[data.file]
dir = "` + contentDir + `"

[jobs]
store = "memory"
poll_interval = "10s"
`
	if err := os.WriteFile(tomlPath, []byte(toml), 0644); err != nil {
		t.Fatalf("write toml: %v", err)
	}

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	addr := ln.Addr().String()

	k := kernel.New(kernel.WithConfigFile(tomlPath))
	k.Use(nslog.New())
	k.Use(routing.New())
	tcpAdapter := tcp.NewAdapterWithListener(ln)
	k.Use(tcpAdapter)
	k.Use(datafile.New())
	k.Use(bridge.New())
	jobsMod := New()
	k.Use(jobsMod)

	ctx := context.Background()
	if err := k.Init(ctx); err != nil {
		t.Fatalf("Init: %v", err)
	}
	if err := k.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}
	t.Cleanup(func() {
		stopCtx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		_ = k.Stop(stopCtx)
	})

	return jobsMod, k, tcpAdapter, addr
}

// setupBridgeKernelNoBridge boots a kernel with jobs but WITHOUT data.bridge.
// Used to confirm jobs handles missing bridge gracefully.
func setupBridgeKernelNoBridge(t *testing.T) (*Module, *kernel.Kernel) {
	t.Helper()

	dir := t.TempDir()
	tomlPath := filepath.Join(dir, "nullspace.toml")
	toml := `
[modules]
jobs = true

[jobs]
store = "memory"
poll_interval = "10s"
`
	if err := os.WriteFile(tomlPath, []byte(toml), 0644); err != nil {
		t.Fatalf("write toml: %v", err)
	}

	k := kernel.New(kernel.WithConfigFile(tomlPath))
	k.Use(nslog.New())
	k.Use(routing.New())
	jobsMod := New()
	k.Use(jobsMod)

	if err := k.Init(context.Background()); err != nil {
		t.Fatalf("Init: %v", err)
	}
	t.Cleanup(func() {
		stopCtx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		_ = k.Stop(stopCtx)
	})

	return jobsMod, k
}

func dialBridge(t *testing.T, addr string) net.Conn {
	t.Helper()
	conn, err := net.DialTimeout("tcp", addr, time.Second)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	return conn
}

func sendBridgeCommand(t *testing.T, conn net.Conn, command string, payload any) bridgeResponse {
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

	// In json-lines a single response ends with \n; trim trailing newlines.
	raw := bytes.TrimRight(buf[:n], "\n")
	// If multiple lines arrived, take the first.
	if idx := bytes.IndexByte(raw, '\n'); idx >= 0 {
		raw = raw[:idx]
	}

	var resp bridgeResponse
	if err := json.Unmarshal(raw, &resp); err != nil {
		t.Fatalf("unmarshal response: %v (data: %q)", err, raw)
	}
	return resp
}

func bridgePayloadMap(t *testing.T, resp bridgeResponse) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal(resp.Payload, &m); err != nil {
		t.Fatalf("unmarshal payload: %v (data: %q)", err, resp.Payload)
	}
	return m
}

// TestBridge_RegistersJobsHandlers verifies that when data.bridge + tcp + jobs
// are all enabled the jobs module registers jobs.submit/list/cancel on the
// TCP router during kernel.after_init.
func TestBridge_RegistersJobsHandlers(t *testing.T) {
	_, _, tcpAdapter, _ := setupBridgeKernel(t)

	router := tcpAdapter.Router()
	cmds := router.Commands()
	sort.Strings(cmds)

	required := []string{"jobs.cancel", "jobs.list", "jobs.submit"}
	for _, want := range required {
		found := false
		for _, c := range cmds {
			if c == want {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("router does not have handler %q; commands=%v", want, cmds)
		}
	}

	// Sanity: RegisterBridgeHandlers (or whatever symbol the GREEN step
	// exports) must exist so opportunistic registration is callable from
	// the kernel.after_init hook. Compilation of this file will fail until
	// the GREEN step provides it.
	_ = RegisterBridgeHandlers
}

// TestBridge_SkippedWhenBridgeAbsent verifies that without data.bridge enabled
// the jobs module initializes cleanly and does not panic. Registration of
// bridge handlers is opportunistic, not required.
func TestBridge_SkippedWhenBridgeAbsent(t *testing.T) {
	t.Parallel()

	m, _ := setupBridgeKernelNoBridge(t)
	if m == nil {
		t.Fatal("expected non-nil jobs module")
	}
	// If we got here without a panic during Init / after_init the module
	// behaved correctly.
}

// TestBridge_SubmitInvokesEnqueue invokes the registered jobs.submit handler
// (via the live TCP server) and asserts the job is enqueued in the store.
func TestBridge_SubmitInvokesEnqueue(t *testing.T) {
	jobsMod, _, _, addr := setupBridgeKernel(t)

	// Register a handler so Submit accepts the type.
	jobsMod.Handlers().Handle("h", func(ctx context.Context, j *Job) error { return nil })

	conn := dialBridge(t, addr)
	defer conn.Close()

	resp := sendBridgeCommand(t, conn, "jobs.submit", map[string]any{
		"type":    "h",
		"payload": map[string]any{"n": 1},
	})

	if resp.Command != "jobs.submit" {
		t.Fatalf("command = %q, want jobs.submit", resp.Command)
	}
	m := bridgePayloadMap(t, resp)
	if _, ok := m["error"]; ok {
		t.Fatalf("unexpected error response: %v", m)
	}

	// Verify the job is in the store.
	all, err := jobsMod.Store().List(context.Background(), ListFilter{})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(all) != 1 {
		t.Fatalf("expected 1 job enqueued, got %d", len(all))
	}
	if all[0].Type != "h" {
		t.Errorf("job type: got %q, want %q", all[0].Type, "h")
	}
}

// TestBridge_ListReturnsJobs enqueues 3 jobs and verifies jobs.list returns
// all of them.
func TestBridge_ListReturnsJobs(t *testing.T) {
	jobsMod, _, _, addr := setupBridgeKernel(t)

	jobsMod.Handlers().Handle("h", func(ctx context.Context, j *Job) error { return nil })

	for i := 0; i < 3; i++ {
		if _, err := jobsMod.Submit(context.Background(), JobSpec{
			Type:  "h",
			RunAt: time.Now().Add(time.Hour),
		}); err != nil {
			t.Fatalf("Submit %d: %v", i, err)
		}
	}

	conn := dialBridge(t, addr)
	defer conn.Close()

	resp := sendBridgeCommand(t, conn, "jobs.list", map[string]any{})
	if resp.Command != "jobs.list" {
		t.Fatalf("command = %q, want jobs.list", resp.Command)
	}

	m := bridgePayloadMap(t, resp)
	items, ok := m["Items"].([]any)
	if !ok {
		// Tolerate "items" lowercase for flexibility.
		if v, ok2 := m["items"].([]any); ok2 {
			items = v
		} else {
			t.Fatalf("Items not a slice in response: %v", m)
		}
	}
	if len(items) != 3 {
		t.Errorf("expected 3 items, got %d", len(items))
	}
}

// TestBridge_CancelRemovesJob submits a future job and invokes jobs.cancel
// via the bridge, verifying the job ends up in StatusCancelled.
func TestBridge_CancelRemovesJob(t *testing.T) {
	jobsMod, _, _, addr := setupBridgeKernel(t)

	jobsMod.Handlers().Handle("h", func(ctx context.Context, j *Job) error { return nil })

	id, err := jobsMod.Submit(context.Background(), JobSpec{
		Type:  "h",
		RunAt: time.Now().Add(time.Hour),
	})
	if err != nil {
		t.Fatalf("Submit: %v", err)
	}

	conn := dialBridge(t, addr)
	defer conn.Close()

	resp := sendBridgeCommand(t, conn, "jobs.cancel", map[string]any{"id": id})
	if resp.Command != "jobs.cancel" {
		t.Fatalf("command = %q, want jobs.cancel", resp.Command)
	}
	m := bridgePayloadMap(t, resp)
	if _, ok := m["error"]; ok {
		t.Fatalf("unexpected error response: %v", m)
	}

	got, err := jobsMod.Store().Get(context.Background(), id)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Status != StatusCancelled {
		t.Errorf("Status: got %q, want %q", got.Status, StatusCancelled)
	}
}
