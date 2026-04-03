// OIDC example: HTTP server with public pages and an OIDC-protected admin area.
//
// This demonstrates the nullspace-oidc contributed module for OpenID Connect
// authentication using the Authorization Code + PKCE flow.
//
// Prerequisites:
//   - Docker (for Keycloak IDP)
//
// Run:
//
//	docker-compose up -d     # Start Keycloak (first run takes ~30s to import realm)
//	go run .                 # Start the app
//
// Endpoints:
//   - GET  /            — Public home page
//   - GET  /admin       — Protected admin page (directory listing)
//   - GET  /oidc/login  — Initiates OIDC login flow
//   - GET  /oidc/logout — Clears session and logs out
//
// Test credentials (Keycloak):
//   - Username: admin
//   - Password: admin123
package main

import (
	"context"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	nsoidc "github.com/frob/nullspace-oidc"
	"github.com/frob/nullspace/core/nslog"
	"github.com/frob/nullspace/core/request"
	"github.com/frob/nullspace/core/response"
	"github.com/frob/nullspace/core/routing"
	"github.com/frob/nullspace/kernel"
	"github.com/frob/nullspace/module/data/static"
)

func main() {
	k := kernel.New(
		kernel.WithConfigFile("nullspace.toml"),
	)

	// Core modules.
	pipeline := response.NewPipeline()

	k.Use(nslog.New())
	k.Use(request.NewAdapter())
	k.Use(pipeline)
	k.Use(response.NewFormatRouteOverride())
	k.Use(response.NewFormatQueryParam())
	k.Use(response.NewFormatContentNegotiate())
	k.Use(response.NewFormatDefault())
	k.Use(static.New())

	// Routing.
	k.Use(routing.New())

	// OIDC module.
	k.Use(nsoidc.New())

	// Application module.
	k.Use(&appModule{pipeline: pipeline})

	ctx := context.Background()
	if err := k.Init(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "init: %v\n", err)
		os.Exit(1)
	}
	if err := k.Start(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "start: %v\n", err)
		os.Exit(1)
	}

	k.Logger().Info("oidc example running",
		"url", "http://localhost:8080",
		"keycloak", "http://localhost:8180",
	)

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	k.Logger().Info("shutting down")
	if err := k.Stop(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "stop: %v\n", err)
	}
}

// --- Application module ---

type appModule struct {
	pipeline *response.Pipeline
}

func (m *appModule) Name() string                    { return "app" }
func (m *appModule) Start(ctx context.Context) error { return nil }
func (m *appModule) Stop(ctx context.Context) error  { return nil }

func (m *appModule) Init(k *kernel.Kernel) error {
	reg, err := kernel.GetResource[*routing.Registry](k, "routing.registry")
	if err != nil {
		return err
	}

	// Public home page.
	reg.HandleFunc("home.page", func(ctx *request.Context) error {
		return m.pipeline.Write(ctx.Context(), ctx.Writer, &response.Response{
			Status:   http.StatusOK,
			Template: "home.html",
			Data:     nil,
		})
	})

	// Protected admin page — lists files in public/files/.
	reg.HandleFunc("admin.files", func(ctx *request.Context) error {
		user, _ := nsoidc.From(ctx)

		type fileEntry struct {
			Name string
			Size int64
		}

		var files []fileEntry
		_ = filepath.WalkDir("./public/files", func(path string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return nil
			}
			info, err := d.Info()
			if err != nil {
				return nil
			}
			files = append(files, fileEntry{
				Name: d.Name(),
				Size: info.Size(),
			})
			return nil
		})

		return m.pipeline.Write(ctx.Context(), ctx.Writer, &response.Response{
			Status:   http.StatusOK,
			Template: "admin.html",
			Data: map[string]any{
				"User":  user,
				"Files": files,
			},
		})
	})

	return nil
}
