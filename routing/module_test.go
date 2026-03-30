package routing

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/frob/nullspace/data/file"
	"github.com/frob/nullspace/kernel"
	"github.com/frob/nullspace/nslog"
	"github.com/frob/nullspace/request"
	"github.com/frob/nullspace/response"
)

// --- Registry Tests ---

func TestRegistryHandler(t *testing.T) {
	reg := NewRegistry()
	called := false
	reg.HandleFunc("test.handler", func(ctx *request.Context) error {
		called = true
		return nil
	})

	h, err := reg.LookupHandler("test.handler")
	if err != nil {
		t.Fatalf("LookupHandler: %v", err)
	}
	h(nil)
	if !called {
		t.Fatal("handler was not called")
	}
}

func TestRegistryHandlerNotFound(t *testing.T) {
	reg := NewRegistry()
	_, err := reg.LookupHandler("nonexistent")
	if err == nil {
		t.Fatal("expected error for missing handler")
	}
}

func TestRegistryMiddleware(t *testing.T) {
	reg := NewRegistry()
	reg.Middleware("test.mw", func(next request.HandlerFunc) request.HandlerFunc {
		return func(ctx *request.Context) error {
			ctx.SetState("mw", true)
			return next(ctx)
		}
	})

	m, err := reg.LookupMiddleware("test.mw")
	if err != nil {
		t.Fatalf("LookupMiddleware: %v", err)
	}
	if m == nil {
		t.Fatal("expected non-nil middleware")
	}
}

func TestRegistryResolveMiddleware(t *testing.T) {
	reg := NewRegistry()
	reg.Middleware("a", func(next request.HandlerFunc) request.HandlerFunc { return next })
	reg.Middleware("b", func(next request.HandlerFunc) request.HandlerFunc { return next })

	mws, err := reg.ResolveMiddleware([]string{"a", "b"})
	if err != nil {
		t.Fatalf("ResolveMiddleware: %v", err)
	}
	if len(mws) != 2 {
		t.Fatalf("expected 2 middleware, got %d", len(mws))
	}
}

func TestRegistryResolveMiddlewareMissing(t *testing.T) {
	reg := NewRegistry()
	reg.Middleware("a", func(next request.HandlerFunc) request.HandlerFunc { return next })

	_, err := reg.ResolveMiddleware([]string{"a", "missing"})
	if err == nil {
		t.Fatal("expected error for missing middleware")
	}
}

// --- Config Expansion Tests ---

func TestExpandRouteDefaults(t *testing.T) {
	m := &Module{config: Config{Groups: map[string]Group{}}}

	routes := m.expandRoute(Route{
		Path:    "/test",
		Handler: "my.handler",
	})

	if len(routes) != 1 {
		t.Fatalf("expected 1 route, got %d", len(routes))
	}
	if routes[0].Method != "GET" {
		t.Fatalf("expected GET, got %s", routes[0].Method)
	}
	if routes[0].Path != "/test" {
		t.Fatalf("expected /test, got %s", routes[0].Path)
	}
	if routes[0].DataParam != "id" {
		t.Fatalf("expected default data_param=id, got %s", routes[0].DataParam)
	}
}

func TestExpandRouteWithGroup(t *testing.T) {
	m := &Module{config: Config{
		Groups: map[string]Group{
			"api": {Prefix: "/api", Format: "json", Middleware: []string{"auth"}},
		},
	}}

	routes := m.expandRoute(Route{
		Group:   "api",
		Path:    "/posts",
		Handler: "posts.list",
	})

	if routes[0].Path != "/api/posts" {
		t.Fatalf("expected /api/posts, got %s", routes[0].Path)
	}
	if routes[0].Format != "json" {
		t.Fatalf("expected json, got %s", routes[0].Format)
	}
	if len(routes[0].Middleware) != 1 || routes[0].Middleware[0] != "auth" {
		t.Fatalf("expected [auth], got %v", routes[0].Middleware)
	}
}

func TestExpandRouteFormatOverride(t *testing.T) {
	m := &Module{config: Config{
		Groups: map[string]Group{
			"api": {Prefix: "/api", Format: "json"},
		},
	}}

	routes := m.expandRoute(Route{
		Group:   "api",
		Path:    "/special",
		Handler: "special",
		Format:  "html",
	})

	if routes[0].Format != "html" {
		t.Fatalf("route format should override group, got %s", routes[0].Format)
	}
}

func TestExpandRouteMultipleMethods(t *testing.T) {
	m := &Module{config: Config{Groups: map[string]Group{}}}

	routes := m.expandRoute(Route{
		Path:    "/posts",
		Methods: []string{"GET", "POST"},
		Handler: "posts",
	})

	if len(routes) != 2 {
		t.Fatalf("expected 2 routes, got %d", len(routes))
	}
	if routes[0].Method != "GET" || routes[1].Method != "POST" {
		t.Fatalf("expected GET and POST, got %s and %s", routes[0].Method, routes[1].Method)
	}
}

func TestExpandCollection(t *testing.T) {
	m := &Module{config: Config{}}

	routes := m.expandCollection(Collection{
		Name:         "posts",
		Source:       "data.file",
		APIPrefix:    "/api",
		HTMLPrefix:   "",
		ListTemplate: "posts.html",
		ItemTemplate: "post.html",
	})

	// API: GET list, GET item, POST create, PUT update, DELETE delete = 5
	// HTML: GET list, GET item = 2
	if len(routes) != 7 {
		t.Fatalf("expected 7 routes, got %d", len(routes))
	}

	// Check API routes.
	apiRoutes := filterRoutes(routes, "/api/")
	if len(apiRoutes) != 5 {
		t.Fatalf("expected 5 API routes, got %d", len(apiRoutes))
	}

	// Check HTML routes.
	htmlRoutes := filterRoutes(routes, "/posts")
	htmlOnly := []resolvedRoute{}
	for _, r := range htmlRoutes {
		if r.Format == "html" {
			htmlOnly = append(htmlOnly, r)
		}
	}
	if len(htmlOnly) != 2 {
		t.Fatalf("expected 2 HTML routes, got %d", len(htmlOnly))
	}
}

func TestExpandCollectionWriteMiddleware(t *testing.T) {
	m := &Module{config: Config{}}

	routes := m.expandCollection(Collection{
		Name:            "posts",
		APIPrefix:       "/api",
		WriteMiddleware: []string{"auth"},
	})

	for _, r := range routes {
		switch r.Method {
		case "POST", "PUT", "DELETE":
			if len(r.Middleware) == 0 || r.Middleware[0] != "auth" {
				t.Fatalf("%s %s should have auth middleware", r.Method, r.Path)
			}
		case "GET":
			if len(r.Middleware) != 0 {
				t.Fatalf("GET %s should not have write middleware", r.Path)
			}
		}
	}
}

// --- Route Table Tests ---

func TestRouteTable(t *testing.T) {
	table := &RouteTable{}
	table.Add(TableEntry{Method: "GET", Path: "/api/posts", Handler: "data.list", Format: "json"})
	table.Add(TableEntry{Method: "POST", Path: "/api/posts", Handler: "data.create", Format: "json", Middleware: []string{"auth"}})

	out := table.String()
	if !strings.Contains(out, "GET") || !strings.Contains(out, "/api/posts") {
		t.Fatalf("table missing expected content: %s", out)
	}
	if !strings.Contains(out, "auth") {
		t.Fatalf("table missing middleware: %s", out)
	}
}

// --- Integration Tests ---

func setupIntegration(t *testing.T, toml string) (*request.Adapter, *Module, *kernel.Kernel) {
	t.Helper()

	contentDir := t.TempDir()
	postsDir := filepath.Join(contentDir, "posts")
	os.MkdirAll(postsDir, 0755)
	os.WriteFile(filepath.Join(postsDir, "hello.md"), []byte("---\ntitle: Hello\n---\nWorld"), 0644)
	os.WriteFile(filepath.Join(postsDir, "second.md"), []byte("---\ntitle: Second\n---\nPost"), 0644)

	toml = strings.ReplaceAll(toml, "CONTENT_DIR", contentDir)

	configPath := filepath.Join(t.TempDir(), "nullspace.toml")
	os.WriteFile(configPath, []byte(toml), 0644)

	k := kernel.New(kernel.WithConfigFile(configPath))
	adapter := request.NewAdapter()
	pipeline := response.NewPipeline()
	fileMod := file.New()
	routingMod := New()

	k.Use(nslog.New())
	k.Use(adapter)
	k.Use(pipeline)
	k.Use(response.NewFormatRouteOverride())
	k.Use(response.NewFormatDefault())
	k.Use(fileMod)
	k.Use(routingMod)

	if err := k.Init(context.Background()); err != nil {
		t.Fatalf("Init: %v", err)
	}

	return adapter, routingMod, k
}

func TestIntegrationDataList(t *testing.T) {
	adapter, _, _ := setupIntegration(t, `
[data.file]
dir = "CONTENT_DIR"

[[routing.routes]]
path = "/api/posts"
handler = "data.list"
format = "json"
collection = "posts"
`)

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/api/posts", nil)
	adapter.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "Hello") {
		t.Fatalf("expected Hello in response: %s", w.Body.String())
	}
}

func TestIntegrationDataGet(t *testing.T) {
	adapter, _, _ := setupIntegration(t, `
[data.file]
dir = "CONTENT_DIR"

[[routing.routes]]
path = "/api/posts/:id"
handler = "data.get"
format = "json"
collection = "posts"
`)

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/api/posts/hello", nil)
	adapter.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "Hello") {
		t.Fatalf("expected Hello in response: %s", w.Body.String())
	}
}

func TestIntegrationRedirect(t *testing.T) {
	adapter, _, _ := setupIntegration(t, `
[data.file]
dir = "CONTENT_DIR"

[[routing.routes]]
path = "/old"
handler = "redirect"
redirect = "/new"
status_code = 301
`)

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/old", nil)
	adapter.ServeHTTP(w, r)

	if w.Code != http.StatusMovedPermanently {
		t.Fatalf("expected 301, got %d", w.Code)
	}
	if w.Header().Get("Location") != "/new" {
		t.Fatalf("expected Location: /new, got %s", w.Header().Get("Location"))
	}
}

func TestIntegrationRouteGroups(t *testing.T) {
	adapter, _, _ := setupIntegration(t, `
[data.file]
dir = "CONTENT_DIR"

[routing.groups.api]
prefix = "/api"
format = "json"

[[routing.routes]]
group = "api"
path = "/posts"
handler = "data.list"
collection = "posts"
`)

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/api/posts", nil)
	adapter.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestIntegrationCollection(t *testing.T) {
	adapter, mod, _ := setupIntegration(t, `
[data.file]
dir = "CONTENT_DIR"

[[routing.collections]]
name = "posts"
api_prefix = "/api"
`)

	// List.
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/api/posts", nil)
	adapter.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("list: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	// Get.
	w = httptest.NewRecorder()
	r = httptest.NewRequest("GET", "/api/posts/hello", nil)
	adapter.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("get: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	// Route table should have entries.
	table := mod.Table()
	if len(table.Entries) < 5 {
		t.Fatalf("expected at least 5 collection routes, got %d", len(table.Entries))
	}
}

func TestIntegrationCustomHandler(t *testing.T) {
	contentDir := t.TempDir()
	postsDir := filepath.Join(contentDir, "posts")
	os.MkdirAll(postsDir, 0755)
	os.WriteFile(filepath.Join(postsDir, "test.md"), []byte("---\ntitle: Test\n---\nBody"), 0644)

	configPath := filepath.Join(t.TempDir(), "nullspace.toml")
	os.WriteFile(configPath, []byte(`
[data.file]
dir = "`+contentDir+`"

[[routing.routes]]
path = "/custom"
handler = "my.handler"
format = "json"
`), 0644)

	k := kernel.New(kernel.WithConfigFile(configPath))
	adapter := request.NewAdapter()

	k.Use(nslog.New())
	k.Use(adapter)
	k.Use(response.NewPipeline())
	k.Use(response.NewFormatDefault())
	k.Use(file.New())
	// Routing module FIRST — provides the registry.
	k.Use(New())
	// Registrar module AFTER — registers handlers on the registry.
	k.Use(&handlerRegistrar{handler: func(ctx *request.Context) error {
		ctx.Writer.WriteHeader(http.StatusOK)
		ctx.Writer.Write([]byte(`{"custom":true}`))
		return nil
	}})

	if err := k.Init(context.Background()); err != nil {
		t.Fatalf("Init: %v", err)
	}

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/custom", nil)
	adapter.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "custom") {
		t.Fatalf("expected custom handler output: %s", w.Body.String())
	}
}

// handlerRegistrar is a test module that registers a custom handler.
type handlerRegistrar struct {
	handler request.HandlerFunc
}

func (m *handlerRegistrar) Name() string                    { return "test.registrar" }
func (m *handlerRegistrar) Start(ctx context.Context) error { return nil }
func (m *handlerRegistrar) Stop(ctx context.Context) error  { return nil }
func (m *handlerRegistrar) Init(k *kernel.Kernel) error {
	reg, err := kernel.GetResource[*Registry](k, "routing.registry")
	if err != nil {
		return err
	}
	reg.HandleFunc("my.handler", m.handler)
	return nil
}

func TestIntegrationDataInjection(t *testing.T) {
	contentDir := t.TempDir()
	postsDir := filepath.Join(contentDir, "posts")
	os.MkdirAll(postsDir, 0755)
	os.WriteFile(filepath.Join(postsDir, "test.md"), []byte("---\ntitle: Injected\n---\nBody"), 0644)

	configPath := filepath.Join(t.TempDir(), "nullspace.toml")
	os.WriteFile(configPath, []byte(`
[data.file]
dir = "`+contentDir+`"

[[routing.routes]]
path = "/injected/:id"
handler = "my.handler"
format = "json"
collection = "posts"
`), 0644)

	k := kernel.New(kernel.WithConfigFile(configPath))
	adapter := request.NewAdapter()

	var injectedEntity any

	k.Use(nslog.New())
	k.Use(adapter)
	k.Use(response.NewPipeline())
	k.Use(response.NewFormatDefault())
	k.Use(file.New())
	// Routing first — provides registry.
	k.Use(New())
	// Registrar after — registers handler.
	k.Use(&handlerRegistrar{handler: func(ctx *request.Context) error {
		injectedEntity, _ = ctx.State("data.entity")
		ctx.Writer.WriteHeader(http.StatusOK)
		ctx.Writer.Write([]byte(`{"ok":true}`))
		return nil
	}})

	if err := k.Init(context.Background()); err != nil {
		t.Fatalf("Init: %v", err)
	}

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/injected/test", nil)
	adapter.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	if injectedEntity == nil {
		t.Fatal("expected data.entity to be injected into context state")
	}
}

func TestIntegrationMiddleware(t *testing.T) {
	contentDir := t.TempDir()
	postsDir := filepath.Join(contentDir, "posts")
	os.MkdirAll(postsDir, 0755)
	os.WriteFile(filepath.Join(postsDir, "test.md"), []byte("---\ntitle: Test\n---\nBody"), 0644)

	configPath := filepath.Join(t.TempDir(), "nullspace.toml")
	os.WriteFile(configPath, []byte(`
[data.file]
dir = "`+contentDir+`"

[[routing.routes]]
path = "/protected"
handler = "data.list"
format = "json"
collection = "posts"
middleware = ["block"]
`), 0644)

	k := kernel.New(kernel.WithConfigFile(configPath))
	adapter := request.NewAdapter()
	routingMod := New()

	// A middleware module that registers a "block" middleware.
	blocker := &middlewareRegistrar{name: "block", mw: func(next request.HandlerFunc) request.HandlerFunc {
		return func(ctx *request.Context) error {
			ctx.Writer.WriteHeader(http.StatusForbidden)
			ctx.Writer.Write([]byte("blocked"))
			return nil
		}
	}}

	k.Use(nslog.New())
	k.Use(adapter)
	k.Use(response.NewPipeline())
	k.Use(response.NewFormatDefault())
	k.Use(file.New())
	// Routing first — provides registry.
	k.Use(routingMod)
	// Blocker after — registers middleware on the registry.
	k.Use(blocker)

	if err := k.Init(context.Background()); err != nil {
		t.Fatalf("Init: %v", err)
	}

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/protected", nil)
	adapter.ServeHTTP(w, r)

	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403 from middleware, got %d: %s", w.Code, w.Body.String())
	}
}

// middlewareRegistrar is a test module that registers named middleware.
type middlewareRegistrar struct {
	name string
	mw   request.Middleware
}

func (m *middlewareRegistrar) Name() string                    { return "test.mw." + m.name }
func (m *middlewareRegistrar) Start(ctx context.Context) error { return nil }
func (m *middlewareRegistrar) Stop(ctx context.Context) error  { return nil }
func (m *middlewareRegistrar) Init(k *kernel.Kernel) error {
	reg, err := kernel.GetResource[*Registry](k, "routing.registry")
	if err != nil {
		return err
	}
	reg.Middleware(m.name, m.mw)
	return nil
}

// --- Helpers ---

func filterRoutes(routes []resolvedRoute, prefix string) []resolvedRoute {
	var filtered []resolvedRoute
	for _, r := range routes {
		if strings.HasPrefix(r.Path, prefix) {
			filtered = append(filtered, r)
		}
	}
	return filtered
}
