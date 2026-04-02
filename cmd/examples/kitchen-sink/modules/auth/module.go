// Package auth provides a basic authentication middleware module.
//
// It demonstrates how to build a middleware-as-module that:
//   - Reads credentials from TOML config
//   - Protects routes matching a configurable path prefix
//   - Sets the authenticated user in request state for downstream handlers
//   - Can be enabled/disabled via the module system
package auth

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	_ "embed"
	"encoding/base64"
	"net/http"
	"strings"

	"github.com/frob/nullspace/core/request"
	"github.com/frob/nullspace/core/response"
	"github.com/frob/nullspace/core/routing"
	"github.com/frob/nullspace/kernel"
)

//go:embed routes.toml
var routesData []byte

// Config holds the auth module's configuration.
type Config struct {
	// Realm is the HTTP Basic Auth realm shown in the browser prompt.
	Realm string `json:"realm"`

	// Prefix is the URL path prefix to protect (e.g., "/api/admin").
	// Requests not matching this prefix pass through unprotected.
	Prefix string `json:"prefix"`

	// Users maps usernames to passwords. In production, use hashed
	// passwords — this is plaintext for demonstration only.
	Users map[string]string `json:"users"`
}

// Module provides HTTP Basic Auth middleware.
type Module struct {
	config   Config
	pipeline *response.Pipeline
}

// New creates a new auth module.
func New() *Module {
	return &Module{}
}

func (m *Module) Name() string { return "auth" }

func (m *Module) Config() kernel.ModuleConfig {
	return kernel.ModuleConfig{
		Key: "auth",
		Default: Config{
			Realm:  "Restricted",
			Prefix: "/api/admin",
			Users:  map[string]string{},
		},
		DefaultEnabled: false, // opt-in
	}
}

func (m *Module) Init(k *kernel.Kernel) error {
	if err := k.Config().Decode("auth", &m.config); err != nil {
		m.config = Config{Realm: "Restricted", Prefix: "/api/admin"}
	}

	adapter, err := kernel.GetResource[*request.Adapter](k, "request.adapter")
	if err != nil {
		return err
	}

	m.pipeline, err = kernel.GetResource[*response.Pipeline](k, "response.pipeline")
	if err != nil {
		return err
	}

	adapter.Use(m.middleware)

	// Register the "auth" middleware by name so TOML routes can reference it.
	if reg, err := kernel.GetResource[*routing.Registry](k, "routing.registry"); err == nil {
		reg.Middleware("auth", m.middleware)
	}

	// Load this module's route definitions.
	if routingMod, err := kernel.GetResource[*routing.Module](k, "routing"); err == nil {
		if err := routingMod.LoadRoutes(routesData); err != nil {
			return err
		}
	}

	k.Logger().Info("auth middleware registered", "prefix", m.config.Prefix,
		"users", len(m.config.Users))
	return nil
}

func (m *Module) Start(ctx context.Context) error { return nil }
func (m *Module) Stop(ctx context.Context) error  { return nil }

// middleware checks Basic Auth credentials for requests matching the
// configured prefix. Non-matching requests pass through.
func (m *Module) middleware(next request.HandlerFunc) request.HandlerFunc {
	return func(ctx *request.Context) error {
		// Only protect paths under the configured prefix.
		if !strings.HasPrefix(ctx.Request.URL.Path, m.config.Prefix) {
			return next(ctx)
		}

		username, password, ok := ctx.Request.BasicAuth()
		if !ok {
			return m.unauthorized(ctx)
		}

		expected, exists := m.config.Users[username]
		if !exists || !secureCompare(password, expected) {
			ctx.Logger().Warn("auth failed", "username", username)
			return m.unauthorized(ctx)
		}

		// Set the authenticated user in state for downstream handlers.
		ctx.SetState("auth.user", username)
		ctx.Logger().Debug("auth success", "username", username)

		return next(ctx)
	}
}

func (m *Module) unauthorized(ctx *request.Context) error {
	ctx.Writer.Header().Set("WWW-Authenticate", `Basic realm="`+m.config.Realm+`"`)
	resp := &response.Response{
		Status: http.StatusUnauthorized,
		Error:  errUnauthorized,
	}
	return m.pipeline.Write(ctx.Context(), ctx.Writer, resp)
}

var errUnauthorized = &authError{msg: "unauthorized"}

type authError struct{ msg string }

func (e *authError) Error() string { return e.msg }

// secureCompare performs a constant-time comparison to prevent timing attacks.
func secureCompare(a, b string) bool {
	ha := sha256.Sum256([]byte(a))
	hb := sha256.Sum256([]byte(b))
	return subtle.ConstantTimeCompare(ha[:], hb[:]) == 1
}

// EncodeBasicAuth returns a base64-encoded Basic Auth header value.
// Useful for testing.
func EncodeBasicAuth(username, password string) string {
	return "Basic " + base64.StdEncoding.EncodeToString(
		[]byte(username+":"+password))
}
