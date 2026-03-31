// Package httpsecurity provides HTTP security middleware for the nullspace framework.
//
// The module registers three named middleware on the routing registry:
//   - security.redirect  — redirects HTTP to HTTPS when https_redirect = "true"
//   - security.csrf      — validates CSRF tokens on state-changing requests when csrf = "true"
//   - security.headers   — applies configurable security response headers
//
// It also registers global middleware for security headers so all responses
// get baseline protection. The HTTPS redirect and CSRF middleware are opt-in
// via route directives.
//
// Usage in nullspace.toml:
//
//	[modules]
//	http-security = true
//
//	[http-security]
//	hsts             = true
//	hsts_max_age     = 31536000
//	csp              = "default-src 'self'"
//	frame_options    = "DENY"
//	referrer_policy  = "strict-origin-when-cross-origin"
//	csrf_cookie      = "ns_csrf"
//	csrf_header      = "X-CSRF-Token"
//	csrf_field       = "csrf_token"
//	csrf_secure      = true
//
// Route directives:
//
//	[[routing.routes]]
//	path = "/login"
//	handler = "auth.login"
//	methods = ["GET", "POST"]
//	csrf = "true"
//	https_redirect = "true"
package httpsecurity

import (
	"context"
	"fmt"

	"github.com/frob/nullspace/core/request"
	"github.com/frob/nullspace/core/routing"
	"github.com/frob/nullspace/kernel"
)

// Config holds the http-security module's configuration.
type Config struct {
	// HSTS enables Strict-Transport-Security headers.
	HSTS bool `json:"hsts" toml:"hsts"`
	// HSTSMaxAge is the max-age value in seconds. Defaults to 31536000 (1 year).
	HSTSMaxAge int `json:"hsts_max_age" toml:"hsts_max_age"`
	// HSTSIncludeSubs includes subdomains in the HSTS header.
	HSTSIncludeSubs bool `json:"hsts_include_subs" toml:"hsts_include_subs"`
	// HSTSPreload adds the preload directive to the HSTS header.
	HSTSPreload bool `json:"hsts_preload" toml:"hsts_preload"`

	// CSP is the Content-Security-Policy header value.
	CSP string `json:"csp" toml:"csp"`
	// FrameOptions is the X-Frame-Options header value. Defaults to "DENY".
	FrameOptions string `json:"frame_options" toml:"frame_options"`
	// ReferrerPolicy is the Referrer-Policy header value. Defaults to "strict-origin-when-cross-origin".
	ReferrerPolicy string `json:"referrer_policy" toml:"referrer_policy"`
	// PermissionsPolicy is the Permissions-Policy header value.
	PermissionsPolicy string `json:"permissions_policy" toml:"permissions_policy"`

	// CsrfCookie is the name of the CSRF cookie. Defaults to "ns_csrf".
	CsrfCookie string `json:"csrf_cookie" toml:"csrf_cookie"`
	// CsrfHeader is the HTTP header name for CSRF tokens. Defaults to "X-CSRF-Token".
	CsrfHeader string `json:"csrf_header" toml:"csrf_header"`
	// CsrfField is the form field name for CSRF tokens. Defaults to "csrf_token".
	CsrfField string `json:"csrf_field" toml:"csrf_field"`
	// CsrfSecure sets the Secure attribute on the CSRF cookie.
	CsrfSecure bool `json:"csrf_secure" toml:"csrf_secure"`
	// CsrfPath sets the path for the CSRF cookie. Defaults to "/".
	CsrfPath string `json:"csrf_path" toml:"csrf_path"`
}

// Module provides HTTP security middleware.
type Module struct {
	kernel *kernel.Kernel
	cfg    Config
}

// New creates a new http-security module.
func New() *Module {
	return &Module{}
}

func (m *Module) Name() string { return "http-security" }

func (m *Module) Config() kernel.ModuleConfig {
	return kernel.ModuleConfig{
		Key: "http-security",
		Default: Config{
			HSTS:            false,
			HSTSMaxAge:      31536000,
			HSTSIncludeSubs: true,
			FrameOptions:    "DENY",
			ReferrerPolicy:  "strict-origin-when-cross-origin",
			CsrfCookie:      "ns_csrf",
			CsrfHeader:      "X-CSRF-Token",
			CsrfField:       "csrf_token",
			CsrfSecure:      false,
			CsrfPath:        "/",
		},
		DefaultEnabled: false,
	}
}

func (m *Module) Init(k *kernel.Kernel) error {
	m.kernel = k

	if err := k.Config().Decode("http-security", &m.cfg); err != nil {
		m.cfg = Config{
			HSTSMaxAge:      31536000,
			HSTSIncludeSubs: true,
			FrameOptions:    "DENY",
			ReferrerPolicy:  "strict-origin-when-cross-origin",
			CsrfCookie:      "ns_csrf",
			CsrfHeader:      "X-CSRF-Token",
			CsrfField:       "csrf_token",
			CsrfPath:        "/",
		}
	}

	// Get the adapter for global middleware.
	adapter, err := kernel.GetResource[*request.Adapter](k, "request.adapter")
	if err != nil {
		return fmt.Errorf("http-security: request adapter not found — register request module before http-security: %w", err)
	}

	// Register global middleware — each checks its own route directive.
	adapter.Use(m.headersMiddleware())
	adapter.Use(m.redirectMiddleware())
	adapter.Use(m.csrfMiddleware())

	// Register all middleware by name so routes can reference them.
	reg, err := kernel.GetResource[*routing.Registry](k, "routing.registry")
	if err != nil {
		return fmt.Errorf("http-security: routing registry not found — register routing module before http-security: %w", err)
	}

	reg.Middleware("security.redirect", m.redirectMiddleware())
	reg.Middleware("security.csrf", m.csrfMiddleware())
	reg.Middleware("security.headers", m.headersMiddleware())

	k.Provide("http-security", m)

	k.Logger().Info("http-security module initialized",
		"hsts", m.cfg.HSTS,
		"csp", m.cfg.CSP != "",
		"frame_options", m.cfg.FrameOptions)

	return nil
}

func (m *Module) Start(ctx context.Context) error { return nil }
func (m *Module) Stop(ctx context.Context) error  { return nil }
