package httpsecurity

import (
	"fmt"

	"github.com/frob/nullspace/core/request"
)

// headersMiddleware returns middleware that sets security response headers.
// It runs globally on all responses. Individual routes can opt out by
// setting http-security = "none" in their TOML definition.
func (m *Module) headersMiddleware() request.Middleware {
	return func(next request.HandlerFunc) request.HandlerFunc {
		return func(ctx *request.Context) error {
			if headersDisabled(ctx) {
				return next(ctx)
			}

			h := ctx.Writer.Header()

			// Always set these baseline headers.
			h.Set("X-Content-Type-Options", "nosniff")

			// HSTS — only meaningful over HTTPS but safe to always set.
			if m.cfg.HSTS {
				hsts := fmt.Sprintf("max-age=%d", m.cfg.HSTSMaxAge)
				if m.cfg.HSTSIncludeSubs {
					hsts += "; includeSubDomains"
				}
				if m.cfg.HSTSPreload {
					hsts += "; preload"
				}
				h.Set("Strict-Transport-Security", hsts)
			}

			// Content-Security-Policy.
			if m.cfg.CSP != "" {
				h.Set("Content-Security-Policy", m.cfg.CSP)
			}

			// X-Frame-Options.
			if m.cfg.FrameOptions != "" {
				h.Set("X-Frame-Options", m.cfg.FrameOptions)
			}

			// Referrer-Policy.
			if m.cfg.ReferrerPolicy != "" {
				h.Set("Referrer-Policy", m.cfg.ReferrerPolicy)
			}

			// Permissions-Policy.
			if m.cfg.PermissionsPolicy != "" {
				h.Set("Permissions-Policy", m.cfg.PermissionsPolicy)
			}

			return next(ctx)
		}
	}
}

// headersDisabled checks if the route has opted out of security headers
// via the http-security = "none" directive.
func headersDisabled(ctx *request.Context) bool {
	r := ctx.Route()
	if r == nil {
		return false
	}
	return r.Meta["http-security"] == "none"
}
