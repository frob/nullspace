package httpsecurity

import (
	"net/http"

	"github.com/frob/nullspace/core/request"
)

// redirectMiddleware returns middleware that redirects HTTP requests to HTTPS.
// It activates on routes with https_redirect = "true" in their TOML definition,
// or when used explicitly as named middleware "security.redirect".
func (m *Module) redirectMiddleware() request.Middleware {
	return func(next request.HandlerFunc) request.HandlerFunc {
		return func(ctx *request.Context) error {
			if !shouldRedirect(ctx) {
				return next(ctx)
			}

			if isTLS(ctx) {
				return next(ctx)
			}

			target := "https://" + ctx.Request.Host + ctx.Request.URL.RequestURI()
			http.Redirect(ctx.Writer, ctx.Request, target, http.StatusMovedPermanently)
			return nil
		}
	}
}

// shouldRedirect checks the route directive for https_redirect = "true".
func shouldRedirect(ctx *request.Context) bool {
	r := ctx.Route()
	if r == nil {
		return false
	}
	return r.Meta["https_redirect"] == "true"
}

// isTLS reports whether the request arrived over TLS. It checks both the
// native TLS state and the X-Forwarded-Proto header set by reverse proxies.
func isTLS(ctx *request.Context) bool {
	if ctx.Request.TLS != nil {
		return true
	}
	return ctx.Request.Header.Get("X-Forwarded-Proto") == "https"
}
