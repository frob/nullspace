// Example application demonstrating the Nullspace framework.
//
// Run from the cmd/example directory:
//
//	go run .
//
// Routes are defined in nullspace.toml. Custom handlers are registered
// by example modules (auth, forms) and by this main package.
//
// Endpoints:
//   - http://localhost:8080/                     — Home page (HTML)
//   - http://localhost:8080/posts                — Post list (HTML, from collection)
//   - http://localhost:8080/posts/:id            — Single post (HTML, from collection)
//   - http://localhost:8080/forms/contact        — Contact form (HTML, from forms module)
//   - http://localhost:8080/api/posts            — Post list (JSON, from collection)
//   - http://localhost:8080/api/posts/:id        — Single post (JSON, from collection)
//   - http://localhost:8080/api/admin/posts      — Protected by Basic Auth
//   - http://localhost:8080/api/health           — Health check (JSON)
//   - http://localhost:8080/blog                 — Redirects to /posts
//   - http://localhost:8080/css/style.css        — Static file
package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/frob/nullspace/cmd/example/modules/auth"
	"github.com/frob/nullspace/cmd/example/modules/forms"
	"github.com/frob/nullspace/core/nslog"
	"github.com/frob/nullspace/core/request"
	"github.com/frob/nullspace/core/response"
	"github.com/frob/nullspace/core/routing"
	"github.com/frob/nullspace/kernel"
	"github.com/frob/nullspace/module/data/file"
	"github.com/frob/nullspace/module/data/static"
)

func main() {
	k := kernel.New(
		kernel.WithConfigFile("nullspace.toml"),
	)

	// Register core modules in dependency order.
	adapter := request.NewAdapter()
	pipeline := response.NewPipeline()

	k.Use(nslog.New())
	k.Use(adapter)
	k.Use(pipeline)
	k.Use(response.NewFormatRouteOverride())
	k.Use(response.NewFormatQueryParam())
	k.Use(response.NewFormatContentNegotiate())
	k.Use(response.NewFormatDefault())
	k.Use(static.New())
	k.Use(file.New())

	// Routing module — provides the handler/middleware registry.
	// Must be registered BEFORE modules that register handlers on it.
	k.Use(routing.New())

	// Example modules — register their own handlers and middleware.
	k.Use(auth.New())
	k.Use(forms.New())

	// Application module — registers custom handlers on the routing registry.
	k.Use(&appModule{})

	// Initialize (TOML routes are resolved in kernel.after_init hook).
	ctx := context.Background()
	if err := k.Init(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "init: %v\n", err)
		os.Exit(1)
	}

	// Start.
	if err := k.Start(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "start: %v\n", err)
		os.Exit(1)
	}

	k.Logger().Info("example app running", "url", "http://localhost:8080")

	// Wait for shutdown signal.
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	k.Logger().Info("shutting down")
	if err := k.Stop(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "stop: %v\n", err)
	}
}

// appModule registers custom handlers that TOML routes reference.
// This demonstrates how application code provides handler implementations
// that the declarative routing config wires up.
type appModule struct{}

func (m *appModule) Name() string                    { return "app" }
func (m *appModule) Start(ctx context.Context) error { return nil }
func (m *appModule) Stop(ctx context.Context) error  { return nil }

func (m *appModule) Init(k *kernel.Kernel) error {
	reg, err := kernel.GetResource[*routing.Registry](k, "routing.registry")
	if err != nil {
		return err
	}
	pipeline, err := kernel.GetResource[*response.Pipeline](k, "response.pipeline")
	if err != nil {
		return err
	}

	// Register the health check handler referenced in nullspace.toml.
	reg.HandleFunc("health.check", func(ctx *request.Context) error {
		resp := response.NewResponse(http.StatusOK, map[string]string{
			"status": "ok",
		})
		return pipeline.Write(ctx.Context(), ctx.Writer, resp)
	})

	return nil
}
