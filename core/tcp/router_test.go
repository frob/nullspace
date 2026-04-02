package tcp

import (
	"context"
	"testing"

	"github.com/frob/nullspace/kernel"
)

func testConn() *Conn {
	ctx, cancel := context.WithCancel(context.Background())
	return &Conn{
		ID:     "test-conn",
		ctx:    ctx,
		cancel: cancel,
		logger: kernel.NewSlogLogger(),
		state:  make(map[string]any),
	}
}

func TestRouterDispatch(t *testing.T) {
	r := NewRouter()

	var called bool
	var gotCmd string
	var gotPayload []byte

	r.Handle("echo", func(conn *Conn, cmd string, payload []byte) error {
		called = true
		gotCmd = cmd
		gotPayload = payload
		return nil
	})

	conn := testConn()
	defer conn.cancel()

	err := r.Dispatch(conn, "echo", []byte("hello"))
	if err != nil {
		t.Fatalf("dispatch: %v", err)
	}
	if !called {
		t.Fatal("handler not called")
	}
	if gotCmd != "echo" {
		t.Errorf("command = %q, want %q", gotCmd, "echo")
	}
	if string(gotPayload) != "hello" {
		t.Errorf("payload = %q, want %q", string(gotPayload), "hello")
	}
}

func TestRouterDispatchUnknown(t *testing.T) {
	r := NewRouter()
	conn := testConn()
	defer conn.cancel()

	err := r.Dispatch(conn, "nope", nil)
	if err == nil {
		t.Fatal("expected error for unknown command")
	}
}

func TestRouterMiddleware(t *testing.T) {
	r := NewRouter()

	var order []string

	r.Use(func(next HandlerFunc) HandlerFunc {
		return func(conn *Conn, cmd string, payload []byte) error {
			order = append(order, "mw1-before")
			err := next(conn, cmd, payload)
			order = append(order, "mw1-after")
			return err
		}
	})

	r.Handle("test", func(conn *Conn, cmd string, payload []byte) error {
		order = append(order, "handler")
		return nil
	})

	conn := testConn()
	defer conn.cancel()

	if err := r.Dispatch(conn, "test", nil); err != nil {
		t.Fatalf("dispatch: %v", err)
	}

	want := []string{"mw1-before", "handler", "mw1-after"}
	if len(order) != len(want) {
		t.Fatalf("order = %v, want %v", order, want)
	}
	for i, s := range want {
		if order[i] != s {
			t.Errorf("order[%d] = %q, want %q", i, order[i], s)
		}
	}
}

func TestRouterCommands(t *testing.T) {
	r := NewRouter()
	r.Handle("a", func(*Conn, string, []byte) error { return nil })
	r.Handle("b", func(*Conn, string, []byte) error { return nil })

	cmds := r.Commands()
	if len(cmds) != 2 {
		t.Fatalf("commands = %v, want 2 entries", cmds)
	}

	got := make(map[string]bool)
	for _, c := range cmds {
		got[c] = true
	}
	if !got["a"] || !got["b"] {
		t.Errorf("commands = %v, want a and b", cmds)
	}
}
