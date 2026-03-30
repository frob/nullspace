package request

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/frob/nullspace/kernel"
	"github.com/frob/nullspace/nslog"
)

// --- Router Tests ---

func TestRouterExactMatch(t *testing.T) {
	r := NewRouter()
	r.Get("/api/posts", func(ctx *Context) error { return nil })

	match := r.Match("GET", "/api/posts")
	if match == nil {
		t.Fatal("expected match")
	}
	if match.Pattern != "/api/posts" {
		t.Fatalf("expected pattern /api/posts, got %s", match.Pattern)
	}
}

func TestRouterNoMatch(t *testing.T) {
	r := NewRouter()
	r.Get("/api/posts", func(ctx *Context) error { return nil })

	if r.Match("GET", "/api/users") != nil {
		t.Fatal("expected no match")
	}
}

func TestRouterMethodMismatch(t *testing.T) {
	r := NewRouter()
	r.Get("/api/posts", func(ctx *Context) error { return nil })

	if r.Match("POST", "/api/posts") != nil {
		t.Fatal("expected no match for wrong method")
	}
}

func TestRouterPathParams(t *testing.T) {
	r := NewRouter()
	r.Get("/api/posts/:id", func(ctx *Context) error { return nil })

	match := r.Match("GET", "/api/posts/42")
	if match == nil {
		t.Fatal("expected match")
	}
	if match.Params["id"] != "42" {
		t.Fatalf("expected id=42, got %s", match.Params["id"])
	}
}

func TestRouterMultipleParams(t *testing.T) {
	r := NewRouter()
	r.Get("/api/posts/:postID/comments/:commentID", func(ctx *Context) error { return nil })

	match := r.Match("GET", "/api/posts/5/comments/99")
	if match == nil {
		t.Fatal("expected match")
	}
	if match.Params["postID"] != "5" {
		t.Fatalf("expected postID=5, got %s", match.Params["postID"])
	}
	if match.Params["commentID"] != "99" {
		t.Fatalf("expected commentID=99, got %s", match.Params["commentID"])
	}
}

func TestRouterRootPath(t *testing.T) {
	r := NewRouter()
	r.Get("/", func(ctx *Context) error { return nil })

	match := r.Match("GET", "/")
	if match == nil {
		t.Fatal("expected match for root")
	}
}

func TestRouterMeta(t *testing.T) {
	r := NewRouter()
	r.Get("/api/data", func(ctx *Context) error { return nil },
		WithMeta("format", "json"),
	)

	match := r.Match("GET", "/api/data")
	if match == nil {
		t.Fatal("expected match")
	}
	if match.Meta["format"] != "json" {
		t.Fatalf("expected format=json, got %s", match.Meta["format"])
	}
}

func TestRouterAllMethods(t *testing.T) {
	r := NewRouter()
	h := func(ctx *Context) error { return nil }

	r.Get("/r", h)
	r.Post("/r", h)
	r.Put("/r", h)
	r.Delete("/r", h)
	r.Patch("/r", h)

	for _, method := range []string{"GET", "POST", "PUT", "DELETE", "PATCH"} {
		if r.Match(method, "/r") == nil {
			t.Fatalf("expected match for %s", method)
		}
	}
}

// --- Middleware Tests ---

func TestBuildChain(t *testing.T) {
	var order []string

	mwA := func(next HandlerFunc) HandlerFunc {
		return func(ctx *Context) error {
			order = append(order, "A-before")
			err := next(ctx)
			order = append(order, "A-after")
			return err
		}
	}

	mwB := func(next HandlerFunc) HandlerFunc {
		return func(ctx *Context) error {
			order = append(order, "B-before")
			err := next(ctx)
			order = append(order, "B-after")
			return err
		}
	}

	handler := func(ctx *Context) error {
		order = append(order, "handler")
		return nil
	}

	chain := buildChain(handler, []Middleware{mwA, mwB})
	chain(newContext(nil, nil, context.Background()))

	expected := []string{"A-before", "B-before", "handler", "B-after", "A-after"}
	if len(order) != len(expected) {
		t.Fatalf("expected %v, got %v", expected, order)
	}
	for i, v := range expected {
		if order[i] != v {
			t.Fatalf("position %d: expected %s, got %s", i, v, order[i])
		}
	}
}

func TestBuildChainEmpty(t *testing.T) {
	called := false
	handler := func(ctx *Context) error {
		called = true
		return nil
	}

	chain := buildChain(handler, nil)
	chain(newContext(nil, nil, context.Background()))

	if !called {
		t.Fatal("handler should be called with empty middleware")
	}
}

// --- Context Tests ---

func TestContextParam(t *testing.T) {
	ctx := newContext(nil, nil, context.Background())
	ctx.params["id"] = "42"

	if ctx.Param("id") != "42" {
		t.Fatalf("expected 42, got %s", ctx.Param("id"))
	}
	if ctx.Param("missing") != "" {
		t.Fatal("expected empty for missing param")
	}
}

func TestContextState(t *testing.T) {
	ctx := newContext(nil, nil, context.Background())

	ctx.SetState("user", "alice")
	v, ok := ctx.State("user")
	if !ok || v != "alice" {
		t.Fatalf("expected alice, got %v", v)
	}

	_, ok = ctx.State("missing")
	if ok {
		t.Fatal("expected not found")
	}
}

func TestContextLogger(t *testing.T) {
	logger := kernel.NewSlogLogger()
	ctx := context.Background()
	ctx = nslog.WithLogger(ctx, logger)

	fctx := newContext(nil, nil, ctx)
	if fctx.Logger() == nil {
		t.Fatal("expected non-nil logger")
	}
}

func TestContextSnapshot(t *testing.T) {
	cfg := kernel.NewConfig()
	cfg.Set("test", "value")
	snap := cfg.Snapshot()

	ctx := kernel.ContextWithSnapshot(context.Background(), snap)
	fctx := newContext(nil, nil, ctx)

	if fctx.Snapshot() == nil {
		t.Fatal("expected non-nil snapshot")
	}
}

// --- Adapter Integration Tests ---

func setupTestAdapter(t *testing.T) (*Adapter, *kernel.Kernel) {
	t.Helper()
	k := kernel.New()
	logMod := nslog.New()
	adapter := NewAdapter()

	k.Use(logMod)
	k.Use(adapter)

	ctx := context.Background()
	if err := k.Init(ctx); err != nil {
		t.Fatalf("Init: %v", err)
	}

	return adapter, k
}

func TestAdapterServeHTTP(t *testing.T) {
	adapter, _ := setupTestAdapter(t)

	adapter.Router().Get("/hello", func(ctx *Context) error {
		ctx.Writer.WriteHeader(http.StatusOK)
		fmt.Fprint(ctx.Writer, "hello world")
		return nil
	})

	req := httptest.NewRequest("GET", "/hello", nil)
	w := httptest.NewRecorder()

	adapter.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if w.Body.String() != "hello world" {
		t.Fatalf("expected 'hello world', got %q", w.Body.String())
	}
}

func TestAdapterServeHTTP404(t *testing.T) {
	adapter, _ := setupTestAdapter(t)

	req := httptest.NewRequest("GET", "/nonexistent", nil)
	w := httptest.NewRecorder()

	adapter.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestAdapterServeHTTPHandlerError(t *testing.T) {
	adapter, _ := setupTestAdapter(t)

	adapter.Router().Get("/fail", func(ctx *Context) error {
		return fmt.Errorf("something broke")
	})

	req := httptest.NewRequest("GET", "/fail", nil)
	w := httptest.NewRecorder()

	adapter.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w.Code)
	}
}

func TestAdapterPathParams(t *testing.T) {
	adapter, _ := setupTestAdapter(t)

	var capturedID string
	adapter.Router().Get("/posts/:id", func(ctx *Context) error {
		capturedID = ctx.Param("id")
		ctx.Writer.WriteHeader(http.StatusOK)
		return nil
	})

	req := httptest.NewRequest("GET", "/posts/42", nil)
	w := httptest.NewRecorder()

	adapter.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if capturedID != "42" {
		t.Fatalf("expected id=42, got %s", capturedID)
	}
}

func TestAdapterGlobalMiddleware(t *testing.T) {
	adapter, _ := setupTestAdapter(t)

	var mwCalled bool
	adapter.Use(func(next HandlerFunc) HandlerFunc {
		return func(ctx *Context) error {
			mwCalled = true
			ctx.SetState("from_mw", true)
			return next(ctx)
		}
	})

	var stateValue any
	adapter.Router().Get("/mw", func(ctx *Context) error {
		stateValue, _ = ctx.State("from_mw")
		ctx.Writer.WriteHeader(http.StatusOK)
		return nil
	})

	req := httptest.NewRequest("GET", "/mw", nil)
	w := httptest.NewRecorder()

	adapter.ServeHTTP(w, req)

	if !mwCalled {
		t.Fatal("expected middleware to run")
	}
	if stateValue != true {
		t.Fatal("expected state from middleware")
	}
}

func TestAdapterRouteMiddleware(t *testing.T) {
	adapter, _ := setupTestAdapter(t)

	var routeMWCalled bool
	routeMW := func(next HandlerFunc) HandlerFunc {
		return func(ctx *Context) error {
			routeMWCalled = true
			return next(ctx)
		}
	}

	adapter.Router().Get("/with-mw", func(ctx *Context) error {
		ctx.Writer.WriteHeader(http.StatusOK)
		return nil
	}, WithRouteMiddleware(routeMW))

	req := httptest.NewRequest("GET", "/with-mw", nil)
	w := httptest.NewRecorder()

	adapter.ServeHTTP(w, req)

	if !routeMWCalled {
		t.Fatal("expected route middleware to run")
	}
}

func TestAdapterHooksFire(t *testing.T) {
	k := kernel.New()
	logMod := nslog.New()
	adapter := NewAdapter()

	k.Use(logMod)
	k.Use(adapter)

	var hooksFired []string

	// Register a test module that tracks hooks.
	testMod := &hookTracker{hooks: &hooksFired}
	k.Use(testMod)

	ctx := context.Background()
	if err := k.Init(ctx); err != nil {
		t.Fatalf("Init: %v", err)
	}

	adapter.Router().Get("/hooks", func(ctx *Context) error {
		ctx.Writer.WriteHeader(http.StatusOK)
		return nil
	})

	req := httptest.NewRequest("GET", "/hooks", nil)
	w := httptest.NewRecorder()

	adapter.ServeHTTP(w, req)

	// Should fire: received, routed, before, after, complete.
	expected := map[string]bool{
		"request.received": true,
		"request.routed":   true,
		"request.before":   true,
		"request.after":    true,
		"request.complete": true,
	}

	for _, h := range hooksFired {
		delete(expected, h)
	}
	if len(expected) > 0 {
		t.Fatalf("hooks not fired: %v (fired: %v)", expected, hooksFired)
	}
}

// hookTracker is a test module that records which hooks fire.
type hookTracker struct {
	hooks *[]string
}

func (m *hookTracker) Name() string                       { return "hook_tracker" }
func (m *hookTracker) Start(ctx context.Context) error    { return nil }
func (m *hookTracker) Stop(ctx context.Context) error     { return nil }
func (m *hookTracker) Init(k *kernel.Kernel) error {
	for _, name := range []string{
		"request.received", "request.routed", "request.before",
		"request.after", "request.complete",
	} {
		hookName := name // capture
		k.Hook(hookName, 50, func(ctx context.Context) error {
			*m.hooks = append(*m.hooks, hookName)
			return nil
		})
	}
	return nil
}

func TestAdapterConfigSnapshot(t *testing.T) {
	adapter, k := setupTestAdapter(t)

	k.Config().Set("test.key", "snapshot_value")

	var snapshotOK bool
	adapter.Router().Get("/snap", func(ctx *Context) error {
		snap := ctx.Snapshot()
		if snap != nil {
			v, ok := snap.Get("test.key")
			snapshotOK = ok && v == "snapshot_value"
		}
		ctx.Writer.WriteHeader(http.StatusOK)
		return nil
	})

	req := httptest.NewRequest("GET", "/snap", nil)
	w := httptest.NewRecorder()

	adapter.ServeHTTP(w, req)

	if !snapshotOK {
		t.Fatal("expected config snapshot to be available in handler")
	}
}

func TestAdapterRequestLogger(t *testing.T) {
	adapter, _ := setupTestAdapter(t)

	var hasLogger bool
	adapter.Router().Get("/log", func(ctx *Context) error {
		hasLogger = ctx.Logger() != nil
		ctx.Writer.WriteHeader(http.StatusOK)
		return nil
	})

	req := httptest.NewRequest("GET", "/log", nil)
	w := httptest.NewRecorder()

	adapter.ServeHTTP(w, req)

	if !hasLogger {
		t.Fatal("expected per-request logger in context")
	}
}

func TestResponseCaptureStatus(t *testing.T) {
	w := httptest.NewRecorder()
	capture := &responseCapture{ResponseWriter: w, status: http.StatusOK}

	capture.WriteHeader(http.StatusCreated)

	if capture.status != http.StatusCreated {
		t.Fatalf("expected 201, got %d", capture.status)
	}

	// Second WriteHeader should be ignored.
	capture.WriteHeader(http.StatusNotFound)
	if capture.status != http.StatusCreated {
		t.Fatalf("expected 201 after second WriteHeader, got %d", capture.status)
	}
}
