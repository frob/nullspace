package session

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/frob/nullspace/kernel"
	"github.com/frob/nullspace/nslog"
	"github.com/frob/nullspace/request"
	"github.com/frob/nullspace/routing"
)

// — MemoryStore tests —

func TestMemoryStoreCreateAndLoad(t *testing.T) {
	ctx := context.Background()
	store := NewMemoryStore(time.Hour)

	sess, err := store.Create(ctx)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if sess.ID == "" {
		t.Fatal("expected non-empty session ID")
	}

	loaded, err := store.Load(ctx, sess.ID)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if loaded == nil {
		t.Fatal("expected loaded session, got nil")
	}
	if loaded.ID != sess.ID {
		t.Fatalf("expected ID %s, got %s", sess.ID, loaded.ID)
	}
}

func TestMemoryStoreMissing(t *testing.T) {
	ctx := context.Background()
	store := NewMemoryStore(time.Hour)

	loaded, err := store.Load(ctx, "nonexistent")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if loaded != nil {
		t.Fatal("expected nil for missing session")
	}
}

func TestMemoryStoreExpiry(t *testing.T) {
	ctx := context.Background()
	store := NewMemoryStore(1 * time.Millisecond)

	sess, err := store.Create(ctx)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	time.Sleep(5 * time.Millisecond)

	loaded, err := store.Load(ctx, sess.ID)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if loaded != nil {
		t.Fatal("expected nil for expired session")
	}
}

func TestMemoryStoreSave(t *testing.T) {
	ctx := context.Background()
	store := NewMemoryStore(time.Hour)

	sess, _ := store.Create(ctx)
	sess.Set("role", "admin")

	if err := store.Save(ctx, sess); err != nil {
		t.Fatalf("Save: %v", err)
	}

	loaded, _ := store.Load(ctx, sess.ID)
	if loaded == nil {
		t.Fatal("expected session after save")
	}
	role, ok := loaded.Get("role")
	if !ok || role != "admin" {
		t.Fatalf("expected role=admin, got %v", role)
	}
}

func TestMemoryStoreDelete(t *testing.T) {
	ctx := context.Background()
	store := NewMemoryStore(time.Hour)

	sess, _ := store.Create(ctx)

	if err := store.Delete(ctx, sess.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	loaded, _ := store.Load(ctx, sess.ID)
	if loaded != nil {
		t.Fatal("expected nil after delete")
	}
}

func TestMemoryStoreLoadReturnsCopy(t *testing.T) {
	ctx := context.Background()
	store := NewMemoryStore(time.Hour)

	sess, _ := store.Create(ctx)
	sess.Set("key", "original")
	store.Save(ctx, sess)

	loaded, _ := store.Load(ctx, sess.ID)
	loaded.Set("key", "mutated")

	// Re-load should still have the original value (Load returns a copy).
	reloaded, _ := store.Load(ctx, sess.ID)
	v, _ := reloaded.Get("key")
	if v != "original" {
		t.Fatalf("Load should return a copy; got %v", v)
	}
}

// — Session value tests —

func TestSessionSetGetDelete(t *testing.T) {
	s := &Session{Values: make(map[string]any)}

	if s.IsDirty() {
		t.Fatal("new session should not be dirty")
	}

	s.Set("user", "alice")
	if !s.IsDirty() {
		t.Fatal("expected dirty after Set")
	}

	v, ok := s.Get("user")
	if !ok || v != "alice" {
		t.Fatalf("expected alice, got %v", v)
	}

	s.Delete("user")
	_, ok = s.Get("user")
	if ok {
		t.Fatal("expected key to be deleted")
	}
}

// — Integration tests —

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
	sessMod := New()

	k.Use(logMod)
	k.Use(adapter)
	k.Use(routingMod)
	k.Use(sessMod)

	if err := k.Init(context.Background()); err != nil {
		t.Fatalf("Init: %v", err)
	}

	return adapter, sessMod, k
}

func TestModuleRegistersMiddleware(t *testing.T) {
	_, _, k := setupKernel(t, `
[modules]
session = true
`)

	reg, err := kernel.GetResource[*routing.Registry](k, "routing.registry")
	if err != nil {
		t.Fatalf("get registry: %v", err)
	}

	for _, name := range []string{"session.load", "session.require", "session.ignore"} {
		if _, err := reg.LookupMiddleware(name); err != nil {
			t.Errorf("expected middleware %q to be registered: %v", name, err)
		}
	}
}

func TestLoadMiddlewareNoSession(t *testing.T) {
	adapter, _, k := setupKernel(t, `
[modules]
session = true
`)

	reg, _ := kernel.GetResource[*routing.Registry](k, "routing.registry")
	loadMW, _ := reg.LookupMiddleware("session.load")

	var gotSession *Session
	adapter.Router().Get("/test", func(ctx *request.Context) error {
		gotSession, _ = From(ctx)
		ctx.Writer.WriteHeader(http.StatusOK)
		return nil
	}, request.WithRouteMiddleware(loadMW))

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	adapter.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if gotSession != nil {
		t.Fatal("expected no session without cookie")
	}
}

func TestLoadMiddlewareWithSession(t *testing.T) {
	adapter, sessMod, k := setupKernel(t, `
[modules]
session = true
`)

	reg, _ := kernel.GetResource[*routing.Registry](k, "routing.registry")
	loadMW, _ := reg.LookupMiddleware("session.load")

	// Pre-create a session in the store.
	ctx := context.Background()
	stored, _ := sessMod.Store().Create(ctx)
	stored.Set("user", "alice")
	sessMod.Store().Save(ctx, stored)

	var gotSession *Session
	adapter.Router().Get("/profile", func(ctx *request.Context) error {
		gotSession, _ = From(ctx)
		ctx.Writer.WriteHeader(http.StatusOK)
		return nil
	}, request.WithRouteMiddleware(loadMW))

	req := httptest.NewRequest("GET", "/profile", nil)
	req.AddCookie(&http.Cookie{Name: "ns_session", Value: stored.ID})
	w := httptest.NewRecorder()
	adapter.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if gotSession == nil {
		t.Fatal("expected session to be loaded")
	}
	v, ok := gotSession.Get("user")
	if !ok || v != "alice" {
		t.Fatalf("expected user=alice, got %v", v)
	}
}

func TestLoadMiddlewareSavesDirtySession(t *testing.T) {
	adapter, sessMod, k := setupKernel(t, `
[modules]
session = true
`)

	reg, _ := kernel.GetResource[*routing.Registry](k, "routing.registry")
	loadMW, _ := reg.LookupMiddleware("session.load")

	ctx := context.Background()
	stored, _ := sessMod.Store().Create(ctx)

	adapter.Router().Get("/update", func(ctx *request.Context) error {
		sess, _ := From(ctx)
		sess.Set("updated", true)
		ctx.Writer.WriteHeader(http.StatusOK)
		return nil
	}, request.WithRouteMiddleware(loadMW))

	req := httptest.NewRequest("GET", "/update", nil)
	req.AddCookie(&http.Cookie{Name: "ns_session", Value: stored.ID})
	w := httptest.NewRecorder()
	adapter.ServeHTTP(w, req)

	// Verify mutation was persisted.
	reloaded, _ := sessMod.Store().Load(ctx, stored.ID)
	if reloaded == nil {
		t.Fatal("expected session after handler")
	}
	v, ok := reloaded.Get("updated")
	if !ok || v != true {
		t.Fatalf("expected updated=true, got %v", v)
	}
}

func TestRequireMiddlewareNoSession(t *testing.T) {
	adapter, _, k := setupKernel(t, `
[modules]
session = true
`)

	reg, _ := kernel.GetResource[*routing.Registry](k, "routing.registry")
	requireMW, _ := reg.LookupMiddleware("session.require")

	adapter.Router().Get("/protected", func(ctx *request.Context) error {
		ctx.Writer.WriteHeader(http.StatusOK)
		return nil
	}, request.WithRouteMiddleware(requireMW))

	req := httptest.NewRequest("GET", "/protected", nil)
	w := httptest.NewRecorder()
	adapter.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}

func TestRequireMiddlewareWithValidSession(t *testing.T) {
	adapter, sessMod, k := setupKernel(t, `
[modules]
session = true
`)

	reg, _ := kernel.GetResource[*routing.Registry](k, "routing.registry")
	requireMW, _ := reg.LookupMiddleware("session.require")

	ctx := context.Background()
	stored, _ := sessMod.Store().Create(ctx)

	adapter.Router().Get("/dashboard", func(ctx *request.Context) error {
		ctx.Writer.WriteHeader(http.StatusOK)
		return nil
	}, request.WithRouteMiddleware(requireMW))

	req := httptest.NewRequest("GET", "/dashboard", nil)
	req.AddCookie(&http.Cookie{Name: "ns_session", Value: stored.ID})
	w := httptest.NewRecorder()
	adapter.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestRequireMiddlewareExpiredSession(t *testing.T) {
	adapter, _, k := setupKernel(t, `
[modules]
session = true

[session]
ttl = "1ms"
`)

	reg, _ := kernel.GetResource[*routing.Registry](k, "routing.registry")
	requireMW, _ := reg.LookupMiddleware("session.require")

	ctx := context.Background()
	store, _ := kernel.GetResource[Store](k, "session.store")
	stored, _ := store.Create(ctx)

	time.Sleep(5 * time.Millisecond)

	adapter.Router().Get("/secured", func(ctx *request.Context) error {
		ctx.Writer.WriteHeader(http.StatusOK)
		return nil
	}, request.WithRouteMiddleware(requireMW))

	req := httptest.NewRequest("GET", "/secured", nil)
	req.AddCookie(&http.Cookie{Name: "ns_session", Value: stored.ID})
	w := httptest.NewRecorder()
	adapter.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for expired session, got %d", w.Code)
	}
}

func TestSessionIgnoreViaRouteMeta(t *testing.T) {
	adapter, _, k := setupKernel(t, `
[modules]
session = true
`)

	reg, _ := kernel.GetResource[*routing.Registry](k, "routing.registry")
	requireMW, _ := reg.LookupMiddleware("session.require")

	// Route has session.require middleware but declares session = "ignore" via meta.
	adapter.Router().Get("/health", func(ctx *request.Context) error {
		ctx.Writer.WriteHeader(http.StatusOK)
		ctx.Writer.Write([]byte("ok"))
		return nil
	},
		request.WithRouteMiddleware(requireMW),
		request.WithMeta("session", "ignore"),
	)

	// No session cookie — should still pass due to ignore flag.
	req := httptest.NewRequest("GET", "/health", nil)
	w := httptest.NewRecorder()
	adapter.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 for ignored route, got %d", w.Code)
	}
}

func TestSessionLoginURLRedirect(t *testing.T) {
	adapter, _, k := setupKernel(t, `
[modules]
session = true
`)

	// Register a login URL resolver.
	k.HookResolve("session.login_url", 10, func(ctx context.Context) (any, bool, error) {
		return "/login", true, nil
	})

	reg, _ := kernel.GetResource[*routing.Registry](k, "routing.registry")
	requireMW, _ := reg.LookupMiddleware("session.require")

	adapter.Router().Get("/admin", func(ctx *request.Context) error {
		ctx.Writer.WriteHeader(http.StatusOK)
		return nil
	}, request.WithRouteMiddleware(requireMW))

	req := httptest.NewRequest("GET", "/admin", nil)
	w := httptest.NewRecorder()
	adapter.ServeHTTP(w, req)

	if w.Code != http.StatusSeeOther {
		t.Fatalf("expected 303 redirect, got %d", w.Code)
	}
	if loc := w.Header().Get("Location"); loc != "/login" {
		t.Fatalf("expected redirect to /login, got %q", loc)
	}
}

func TestCreateAndDestroySession(t *testing.T) {
	adapter, sessMod, k := setupKernel(t, `
[modules]
session = true
`)
	_ = k

	var createdID string

	// Login route creates a session.
	adapter.Router().Post("/login", func(ctx *request.Context) error {
		sess, err := sessMod.Create(ctx)
		if err != nil {
			return err
		}
		sess.Set("user", "bob")
		createdID = sess.ID
		ctx.Writer.WriteHeader(http.StatusOK)
		return nil
	})

	req := httptest.NewRequest("POST", "/login", nil)
	w := httptest.NewRecorder()
	adapter.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("login: expected 200, got %d", w.Code)
	}

	// Verify the session cookie was set.
	cookies := w.Result().Cookies()
	var sessionCookie *http.Cookie
	for _, c := range cookies {
		if c.Name == "ns_session" {
			sessionCookie = c
		}
	}
	if sessionCookie == nil {
		t.Fatal("expected session cookie after login")
	}
	if sessionCookie.Value != createdID {
		t.Fatalf("expected cookie value %s, got %s", createdID, sessionCookie.Value)
	}

	// Logout route destroys the session.
	adapter.Router().Post("/logout", func(ctx *request.Context) error {
		if err := sessMod.Destroy(ctx); err != nil {
			return err
		}
		ctx.Writer.WriteHeader(http.StatusOK)
		return nil
	})

	logoutReq := httptest.NewRequest("POST", "/logout", nil)
	logoutReq.AddCookie(sessionCookie)
	logoutW := httptest.NewRecorder()
	adapter.ServeHTTP(logoutW, logoutReq)

	if logoutW.Code != http.StatusOK {
		t.Fatalf("logout: expected 200, got %d", logoutW.Code)
	}

	// Session should be gone from the store.
	loaded, _ := sessMod.Store().Load(context.Background(), createdID)
	if loaded != nil {
		t.Fatal("expected session to be deleted after logout")
	}
}
