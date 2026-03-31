package httpsecurity

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"net/http"

	"github.com/frob/nullspace/core/request"
)

const csrfStateKey = "csrf_token"

// csrfMiddleware returns middleware that enforces CSRF token validation on
// state-changing HTTP methods (POST, PUT, PATCH, DELETE). It activates on
// routes with csrf = "true" in their TOML definition, or when used
// explicitly as named middleware "security.csrf".
//
// On safe methods (GET, HEAD, OPTIONS), the middleware generates a token
// and makes it available via ctx.State("csrf_token") for templates.
//
// On state-changing methods, the middleware validates the token from either:
//   - The configured HTTP header (default: X-CSRF-Token)
//   - The configured form field (default: csrf_token)
//
// The token is stored in a cookie (default: ns_csrf) using the double-submit
// cookie pattern. This works without server-side session state.
func (m *Module) csrfMiddleware() request.Middleware {
	return func(next request.HandlerFunc) request.HandlerFunc {
		return func(ctx *request.Context) error {
			if !csrfEnabled(ctx) {
				return next(ctx)
			}

			// Read or generate the CSRF token from the cookie.
			token := m.readCSRFCookie(ctx)
			if token == "" {
				var err error
				token, err = generateCSRFToken()
				if err != nil {
					ctx.Logger().Error("csrf: failed to generate token", "error", err)
					ctx.Writer.WriteHeader(http.StatusInternalServerError)
					return nil
				}
				m.setCSRFCookie(ctx, token)
			}

			// Make the token available to handlers/templates.
			ctx.SetState(csrfStateKey, token)

			// Safe methods: skip validation.
			if isSafeMethod(ctx.Request.Method) {
				return next(ctx)
			}

			// State-changing methods: validate the submitted token.
			submitted := m.readSubmittedToken(ctx)
			if !tokensMatch(token, submitted) {
				ctx.Writer.WriteHeader(http.StatusForbidden)
				_, _ = ctx.Writer.Write([]byte("403 forbidden: invalid CSRF token"))
				return nil
			}

			return next(ctx)
		}
	}
}

// CSRFToken retrieves the CSRF token from the request context.
// Returns empty string if CSRF protection is not enabled for this route.
func CSRFToken(ctx *request.Context) string {
	v, ok := ctx.State(csrfStateKey)
	if !ok {
		return ""
	}
	s, _ := v.(string)
	return s
}

// csrfEnabled checks the route directive for csrf = "true".
func csrfEnabled(ctx *request.Context) bool {
	r := ctx.Route()
	if r == nil {
		return false
	}
	return r.Meta["csrf"] == "true"
}

// isSafeMethod reports whether the HTTP method is considered safe (no
// state change) and therefore does not require CSRF validation.
func isSafeMethod(method string) bool {
	switch method {
	case http.MethodGet, http.MethodHead, http.MethodOptions, http.MethodTrace:
		return true
	}
	return false
}

func (m *Module) readCSRFCookie(ctx *request.Context) string {
	cookie, err := ctx.Request.Cookie(m.cfg.CsrfCookie)
	if err != nil {
		return ""
	}
	return cookie.Value
}

func (m *Module) setCSRFCookie(ctx *request.Context, token string) {
	http.SetCookie(ctx.Writer, &http.Cookie{
		Name:     m.cfg.CsrfCookie,
		Value:    token,
		Path:     m.cfg.CsrfPath,
		HttpOnly: false, // Must be readable by JavaScript for AJAX
		Secure:   m.cfg.CsrfSecure,
		SameSite: http.SameSiteStrictMode,
	})
}

// readSubmittedToken reads the CSRF token from the request header or form field.
func (m *Module) readSubmittedToken(ctx *request.Context) string {
	// Check header first (preferred for AJAX).
	if token := ctx.Request.Header.Get(m.cfg.CsrfHeader); token != "" {
		return token
	}
	// Fall back to form field.
	return ctx.Request.FormValue(m.cfg.CsrfField)
}

// tokensMatch performs a constant-time comparison of two CSRF tokens.
func tokensMatch(expected, submitted string) bool {
	if expected == "" || submitted == "" {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(expected), []byte(submitted)) == 1
}

func generateCSRFToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
