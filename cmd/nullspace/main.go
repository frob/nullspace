// Nullspace serves a project from the current working directory.
//
// Install:
//
//	go install github.com/frob/nullspace/cmd/nullspace@latest
//
// Usage:
//
//	cd myproject
//	nullspace
//
// The project directory should contain a nullspace.toml and any combination
// of content/, templates/, and public/ directories. See the documentation
// for the expected directory layout.
package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/frob/nullspace/core/nslog"
	"github.com/frob/nullspace/core/request"
	"github.com/frob/nullspace/core/response"
	"github.com/frob/nullspace/core/routing"
	"github.com/frob/nullspace/kernel"
	"github.com/frob/nullspace/module/data/file"
	"github.com/frob/nullspace/module/data/static"
)

var version = "dev"

func main() {
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "version", "--version", "-v":
			fmt.Println("nullspace", version)
			return
		case "help", "--help", "-h":
			printUsage()
			return
		case "init":
			initProject()
			return
		case "routes":
			showRoutes()
			return
		}
	}

	if err := serve(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

// setupKernel creates and initializes the kernel with all standard modules.
func setupKernel() (*kernel.Kernel, *routing.Module, error) {
	k := kernel.New()

	adapter := request.NewAdapter()
	pipeline := response.NewPipeline()
	fileMod := file.New()
	routingMod := routing.New()

	k.Use(nslog.New())
	k.Use(adapter)
	k.Use(pipeline)
	k.Use(response.NewFormatRouteOverride())
	k.Use(response.NewFormatQueryParam())
	k.Use(response.NewFormatContentNegotiate())
	k.Use(response.NewFormatDefault())
	k.Use(static.New())
	k.Use(fileMod)
	k.Use(routingMod)

	// Register conventional route handlers on the routing registry.
	// These are resolved during kernel.after_init.
	k.Use(&conventionalHandlers{})

	ctx := context.Background()
	if err := k.Init(ctx); err != nil {
		return nil, nil, fmt.Errorf("init: %w", err)
	}

	return k, routingMod, nil
}

func serve() error {
	k, _, err := setupKernel()
	if err != nil {
		return err
	}

	ctx := context.Background()
	if err := k.Start(ctx); err != nil {
		return fmt.Errorf("start: %w", err)
	}

	var addr string
	if v, ok := k.Config().Get("request"); ok {
		if m, ok := v.(map[string]any); ok {
			if a, ok := m["addr"].(string); ok {
				addr = a
			}
		}
	}
	if addr == "" {
		addr = ":8080"
	}
	k.Logger().Info("nullspace running", "version", version, "addr", addr)

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	k.Logger().Info("shutting down")
	return k.Stop(ctx)
}

func showRoutes() {
	k, routingMod, err := setupKernel()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	_ = k

	table := routingMod.Table()
	if len(table.Entries) == 0 {
		fmt.Println("No routes defined in nullspace.toml.")
		return
	}
	table.Print(os.Stdout)
}

// conventionalHandlers auto-discovers content collections and registers them
// as routing collections + a health check handler. This gives the nullspace
// binary its convention-based behavior without hardcoded routes.
type conventionalHandlers struct{}

func (m *conventionalHandlers) Name() string                    { return "conventional" }
func (m *conventionalHandlers) Start(ctx context.Context) error { return nil }
func (m *conventionalHandlers) Stop(ctx context.Context) error  { return nil }

func (m *conventionalHandlers) Init(k *kernel.Kernel) error {
	reg, err := kernel.GetResource[*routing.Registry](k, "routing.registry")
	if err != nil {
		return err
	}
	pipeline, err := kernel.GetResource[*response.Pipeline](k, "response.pipeline")
	if err != nil {
		return err
	}

	// Health check handler.
	reg.HandleFunc("health.check", func(ctx *request.Context) error {
		return pipeline.Write(ctx.Context(), ctx.Writer,
			response.NewResponse(http.StatusOK, map[string]string{"status": "ok"}))
	})

	// Auto-discover collections and add them to the routing config.
	routingMod, err := kernel.GetResource[*routing.Module](k, "routing")
	if err != nil {
		// Routing module not available — register routes directly.
		return nil
	}

	collections := discoverCollections()
	for _, col := range collections {
		routingMod.AddCollection(routing.Collection{
			Name:         col,
			Source:       "data.file",
			APIPrefix:    "/api",
			HTMLPrefix:   "",
			ListTemplate: col + ".html",
			ItemTemplate: strings.TrimSuffix(col, "s") + ".html",
		})
	}

	// Add home page and health check if not already in config.
	routingMod.AddRoute(routing.Route{
		Path:     "/",
		Handler:  "template",
		Format:   "html",
		Template: "home.html",
	})
	routingMod.AddRoute(routing.Route{
		Path:    "/api/health",
		Handler: "health.check",
		Format:  "json",
	})

	return nil
}

// discoverCollections reads subdirectories of the content directory.
func discoverCollections() []string {
	contentDir := "./content"
	entries, err := os.ReadDir(contentDir)
	if err != nil {
		return nil
	}
	var collections []string
	for _, entry := range entries {
		if entry.IsDir() && !strings.HasPrefix(entry.Name(), ".") {
			collections = append(collections, entry.Name())
		}
	}
	return collections
}

func printUsage() {
	fmt.Print(`Nullspace - HTTP application framework

Usage:
  nullspace              Serve the project in the current directory
  nullspace init         Scaffold a new project in the current directory
  nullspace routes       List all registered routes
  nullspace version      Print version
  nullspace help         Print this help

The project directory should contain:
  nullspace.toml         Configuration (optional)
  content/               File-based content (collections as subdirectories)
  templates/             HTML templates
  public/                Static files (CSS, JS, images)

Install:
  go install github.com/frob/nullspace/cmd/nullspace@latest
`)
}

func initProject() {
	dirs := []string{"content/posts", "templates", "public/css"}
	for _, d := range dirs {
		if err := os.MkdirAll(d, 0755); err != nil {
			fmt.Fprintf(os.Stderr, "error creating %s: %v\n", d, err)
			os.Exit(1)
		}
	}

	writeIfNotExists("nullspace.toml", `[request]
addr = ":8080"

[log]
level = "info"
format = "text"

[response]
default_format = "json"
template_dir = "./templates"

[data.static]
dir = "./public"

[data.file]
dir = "./content"
format = "markdown"
`)

	writeIfNotExists("content/posts/hello-world.md", `---
title: Hello World
date: `+fmt.Sprintf("%s", "2025-01-01")+`
tags:
  - welcome
---

Welcome to your new Nullspace site.
`)

	writeIfNotExists("templates/home.html", `<!DOCTYPE html>
<html>
<head><title>Nullspace</title><link rel="stylesheet" href="/css/style.css"></head>
<body>
  <h1>Nullspace</h1>
  <ul>
  {{range .Collections}}<li><a href="/{{.}}">{{.}}</a> (<a href="/api/{{.}}">api</a>)</li>{{end}}
  </ul>
</body>
</html>
`)

	writeIfNotExists("templates/posts.html", `<!DOCTYPE html>
<html>
<head><title>{{.Collection}}</title><link rel="stylesheet" href="/css/style.css"></head>
<body>
  <nav><a href="/">Home</a></nav>
  <h1>{{.Collection}}</h1>
  {{range .Items}}
  <article>
    <h2><a href="/posts/{{.ID}}">{{.Title}}</a></h2>
    <p>{{.Date}}</p>
  </article>
  {{end}}
</body>
</html>
`)

	writeIfNotExists("templates/post.html", `<!DOCTYPE html>
<html>
<head><title>{{.Title}}</title><link rel="stylesheet" href="/css/style.css"></head>
<body>
  <nav><a href="/">Home</a> / <a href="/posts">Posts</a></nav>
  <article>
    <h1>{{.Title}}</h1>
    <p>{{.Date}}</p>
    <div>{{.Body}}</div>
  </article>
</body>
</html>
`)

	writeIfNotExists("public/css/style.css", `body {
  font-family: system-ui, sans-serif;
  max-width: 800px;
  margin: 0 auto;
  padding: 2rem;
  line-height: 1.6;
}
a { color: #0066cc; }
`)

	fmt.Println("Project initialized. Run 'nullspace' to start.")
}

func writeIfNotExists(path, content string) {
	if _, err := os.Stat(path); err == nil {
		return
	}
	os.WriteFile(path, []byte(content), 0644)
}
