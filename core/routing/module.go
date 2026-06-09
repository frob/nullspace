package routing

import (
	"context"
	"fmt"
	"strings"

	toml "github.com/pelletier/go-toml/v2"

	"github.com/frob/nullspace/core/request"
	"github.com/frob/nullspace/core/response"
	"github.com/frob/nullspace/kernel"
	"github.com/frob/nullspace/module/data/file"
)

// Module provides declarative TOML-based route configuration.
// It reads route definitions from config, resolves handler and middleware
// names via the registry, and registers routes on the router.
//
// The module registers its registry early in Init so other modules can
// register handlers and middleware. Route resolution happens in a
// kernel.after_init hook, after all modules have initialized.
type Module struct {
	kernel   *kernel.Kernel
	config   Config
	registry *Registry
	table    *RouteTable
	deps     *deps
}

// New creates a new routing module.
func New() *Module {
	return &Module{}
}

func (m *Module) Name() string { return "routing" }

func (m *Module) Config() kernel.ModuleConfig {
	return kernel.ModuleConfig{
		Key: "routing",
		Default: Config{
			Groups:      map[string]Group{},
			Routes:      []Route{},
			Collections: []Collection{},
		},
		DefaultEnabled: true,
	}
}

func (m *Module) Init(k *kernel.Kernel) error {
	m.kernel = k
	m.table = &RouteTable{}

	// Create and provide the registry immediately so other modules can register.
	m.registry = NewRegistry()
	k.Provide("routing.registry", m.registry)

	// Read config.
	if err := k.Config().Decode("routing", &m.config); err != nil {
		m.config = Config{Groups: map[string]Group{}, Routes: []Route{}, Collections: []Collection{}}
	}

	// Gather dependencies for built-in handlers.
	pipeline, _ := kernel.GetResource[*response.Pipeline](k, "response.pipeline")
	fileMod, _ := kernel.GetResource[*file.Module](k, "data.file")
	m.deps = &deps{kernel: k, pipeline: pipeline, fileMod: fileMod}

	// Register built-in handlers.
	registerBuiltins(m.registry, m.deps)

	// Provide the module itself so other modules can add routes/collections.
	k.Provide("routing", m)

	// Defer route resolution until all modules have registered their handlers.
	k.Hook("kernel.after_init", 10, m.resolveRoutes)

	return nil
}

// AddRoute adds a route to the config if no route with the same effective
// path and handler already exists. Must be called before kernel.after_init.
func (m *Module) AddRoute(r Route) {
	newPath := m.effectivePath(r)
	for _, existing := range m.config.Routes {
		if m.effectivePath(existing) == newPath && existing.Handler == r.Handler {
			return // already defined in TOML config
		}
	}
	m.config.Routes = append(m.config.Routes, r)
}

// LoadRoutes merges route definitions from embedded TOML data into the
// routing config. Modules call this during Init to register their routes:
//
//	//go:embed routes.toml
//	var routesData []byte
//
//	func (m *Module) Init(k *kernel.Kernel) error {
//	    routingMod, _ := kernel.GetResource[*routing.Module](k, "routing")
//	    routingMod.LoadRoutes(routesData)
//	}
//
// The TOML format is the same as the [routing] section in nullspace.toml.
// Groups, routes, and collections are merged into the main config.
// Must be called before kernel.after_init (i.e., during module Init).
func (m *Module) LoadRoutes(data []byte) error {
	var cfg Config
	if err := toml.Unmarshal(data, &cfg); err != nil {
		return fmt.Errorf("parse routes: %w", err)
	}

	// Merge groups.
	for name, group := range cfg.Groups {
		if _, exists := m.config.Groups[name]; !exists {
			m.config.Groups[name] = group
		}
	}

	// Merge routes (dedup by effective path + handler).
	for _, r := range cfg.Routes {
		m.AddRoute(r)
	}

	// Merge collections (dedup by name).
	for _, c := range cfg.Collections {
		m.AddCollection(c)
	}

	return nil
}

func (m *Module) effectivePath(r Route) string {
	if r.Group != "" {
		if g, ok := m.config.Groups[r.Group]; ok {
			return g.Prefix + r.Path
		}
	}
	return r.Path
}

// AddCollection adds a collection to the config if one with the same name
// doesn't already exist. Must be called before kernel.after_init.
func (m *Module) AddCollection(c Collection) {
	for _, existing := range m.config.Collections {
		if existing.Name == c.Name {
			return // already defined in TOML config
		}
	}
	m.config.Collections = append(m.config.Collections, c)
}

func (m *Module) Start(ctx context.Context) error { return nil }
func (m *Module) Stop(ctx context.Context) error  { return nil }

// Table returns the route table for display (e.g., by the `routes` command).
func (m *Module) Table() *RouteTable {
	return m.table
}

// Registry returns the handler/middleware registry.
func (m *Module) Registry() *Registry {
	return m.registry
}

// resolveRoutes is called via kernel.after_init hook. All modules have
// registered their handlers by this point.
func (m *Module) resolveRoutes(ctx context.Context) error {
	// Expand all routes: merge group settings, expand collections.
	resolved := m.expandRoutes()

	// If there are no routes to register, skip router lookup entirely.
	if len(resolved) == 0 {
		m.kernel.Logger().Info("routes registered from config",
			"routes", 0,
			"collections", len(m.config.Collections),
			"total", 0)
		return nil
	}

	router, err := kernel.GetResource[*request.Router](m.kernel, "router")
	if err != nil {
		return fmt.Errorf("routing: %w", err)
	}

	// Register each route on the router.
	for _, r := range resolved {
		if err := m.registerRoute(router, r); err != nil {
			return fmt.Errorf("routing: %w", err)
		}
	}

	m.kernel.Logger().Info("routes registered from config",
		"routes", len(m.config.Routes),
		"collections", len(m.config.Collections),
		"total", len(resolved))

	return nil
}

// expandRoutes merges group settings into routes and expands collections.
func (m *Module) expandRoutes() []resolvedRoute {
	var resolved []resolvedRoute

	// Expand individual routes.
	for _, r := range m.config.Routes {
		resolved = append(resolved, m.expandRoute(r)...)
	}

	// Expand collections.
	for _, c := range m.config.Collections {
		resolved = append(resolved, m.expandCollection(c)...)
	}

	return resolved
}

// expandRoute merges group settings and expands methods.
func (m *Module) expandRoute(r Route) []resolvedRoute {
	// Merge group settings.
	prefix := ""
	format := r.Format
	var mw []string

	if r.Group != "" {
		if g, ok := m.config.Groups[r.Group]; ok {
			prefix = g.Prefix
			if format == "" {
				format = g.Format
			}
			mw = append(mw, g.Middleware...)
		}
	}
	mw = append(mw, r.Middleware...)

	// Default method.
	methods := r.Methods
	if len(methods) == 0 {
		methods = []string{"GET"}
	}

	// Default data param.
	dataParam := r.DataParam
	if dataParam == "" {
		dataParam = "id"
	}

	path := prefix + r.Path

	var resolved []resolvedRoute
	for _, method := range methods {
		resolved = append(resolved, resolvedRoute{
			Method:        strings.ToUpper(method),
			Path:          path,
			Handler:       r.Handler,
			Format:        format,
			Template:      r.Template,
			Middleware:    mw,
			Collection:    r.Collection,
			DataParam:     dataParam,
			Redirect:      r.Redirect,
			StatusCode:    r.StatusCode,
			Session:       r.Session,
			Csrf:          r.Csrf,
			HttpsRedirect: r.HttpsRedirect,
			Extra:         r.Extra,
		})
	}
	return resolved
}

// expandCollection generates CRUD routes for a data collection.
func (m *Module) expandCollection(c Collection) []resolvedRoute {
	var resolved []resolvedRoute

	if c.Source == "" {
		c.Source = "data.file"
	}

	// API routes (JSON).
	if c.APIPrefix != "" {
		listPath := c.APIPrefix + "/" + c.Name
		itemPath := c.APIPrefix + "/" + c.Name + "/:id"

		// Read routes.
		resolved = append(resolved, resolvedRoute{
			Method: "GET", Path: listPath, Handler: "data.list",
			Format: "json", Collection: c.Name, Middleware: c.ReadMiddleware,
		})
		resolved = append(resolved, resolvedRoute{
			Method: "GET", Path: itemPath, Handler: "data.get",
			Format: "json", Collection: c.Name, DataParam: "id", Middleware: c.ReadMiddleware,
		})

		// Write routes.
		resolved = append(resolved, resolvedRoute{
			Method: "POST", Path: listPath, Handler: "data.create",
			Format: "json", Collection: c.Name, Middleware: c.WriteMiddleware,
		})
		resolved = append(resolved, resolvedRoute{
			Method: "PUT", Path: itemPath, Handler: "data.update",
			Format: "json", Collection: c.Name, DataParam: "id", Middleware: c.WriteMiddleware,
		})
		resolved = append(resolved, resolvedRoute{
			Method: "DELETE", Path: itemPath, Handler: "data.delete",
			Format: "json", Collection: c.Name, DataParam: "id", Middleware: c.WriteMiddleware,
		})
	}

	// HTML routes.
	if c.HTMLPrefix != "" || c.ListTemplate != "" || c.ItemTemplate != "" {
		htmlPrefix := c.HTMLPrefix

		if c.ListTemplate != "" {
			resolved = append(resolved, resolvedRoute{
				Method: "GET", Path: htmlPrefix + "/" + c.Name, Handler: "data.list",
				Format: "html", Collection: c.Name, Template: c.ListTemplate,
				Middleware: c.ReadMiddleware,
			})
		}
		if c.ItemTemplate != "" {
			resolved = append(resolved, resolvedRoute{
				Method: "GET", Path: htmlPrefix + "/" + c.Name + "/:id", Handler: "data.get",
				Format: "html", Collection: c.Name, Template: c.ItemTemplate, DataParam: "id",
				Middleware: c.ReadMiddleware,
			})
		}
	}

	return resolved
}

// registerRoute resolves a single route and registers it on the router.
func (m *Module) registerRoute(router *request.Router, r resolvedRoute) error {
	var handler request.HandlerFunc

	// Resolve handler.
	if r.Handler == "redirect" {
		handler = makeRedirectHandler(r.Redirect, r.StatusCode)
	} else {
		h, err := m.registry.LookupHandler(r.Handler)
		if err != nil {
			return fmt.Errorf("route %s %s: %w", r.Method, r.Path, err)
		}
		handler = h
	}

	// For non-builtin handlers with a collection, add data injection middleware.
	if r.Collection != "" && !isBuiltin(r.Handler) && m.deps.fileMod != nil {
		paramName := ""
		if strings.Contains(r.Path, ":") {
			paramName = r.DataParam
		}
		handler = dataInjectionMiddleware(m.deps, r.Collection, paramName)(handler)
	}

	// Build route options.
	var opts []request.RouteOption

	if r.Format != "" {
		opts = append(opts, request.WithMeta("format", r.Format))
	}
	if r.Template != "" {
		opts = append(opts, request.WithMeta("_template", r.Template))
	}
	if r.Collection != "" {
		opts = append(opts, request.WithMeta("_collection", r.Collection))
	}
	if r.DataParam != "" {
		opts = append(opts, request.WithMeta("_data_param", r.DataParam))
	}
	if r.Session != "" {
		opts = append(opts, request.WithMeta("session", r.Session))
	}
	if r.Csrf != "" {
		opts = append(opts, request.WithMeta("csrf", r.Csrf))
	}
	if r.HttpsRedirect != "" {
		opts = append(opts, request.WithMeta("https_redirect", r.HttpsRedirect))
	}
	for k, v := range r.Extra {
		opts = append(opts, request.WithMeta(k, v))
	}

	// Resolve middleware (warn and skip unknown middleware).
	if len(r.Middleware) > 0 {
		var mws []request.Middleware
		for _, name := range r.Middleware {
			mw, err := m.registry.LookupMiddleware(name)
			if err != nil {
				m.kernel.Logger().Warn("middleware not found, skipping",
					"middleware", name, "route", r.Path)
				continue
			}
			mws = append(mws, mw)
		}
		if len(mws) > 0 {
			opts = append(opts, request.WithRouteMiddleware(mws...))
		}
	}

	// Register on the router.
	router.Handle(r.Method, r.Path, handler, opts...)

	// Record in route table.
	m.table.Add(TableEntry{
		Method:     r.Method,
		Path:       r.Path,
		Handler:    r.Handler,
		Format:     r.Format,
		Middleware: r.Middleware,
		Template:   r.Template,
	})

	m.kernel.Logger().Debug("route registered",
		"method", r.Method, "path", r.Path, "handler", r.Handler)

	return nil
}
