package httpsecurity

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/frob/nullspace/core/nslog"
	"github.com/frob/nullspace/core/request"
	"github.com/frob/nullspace/core/routing"
	"github.com/frob/nullspace/kernel"
)

func setupKernel(t *testing.T, tomlContent string) (*request.Adapter, *Module, *kernel.Kernel) {
	t.Helper()
	dir := t.TempDir()
	tomlPath := filepath.Join(dir, "nullspace.toml")
	if err := os.WriteFile(tomlPath, []byte(tomlContent), 0644); err != nil {
		t.Fatal(err)
	}

	k := kernel.New(kernel.WithConfigFile(tomlPath))
	logMod := nslog.New()
	adapter := request.NewAdapter()
	routingMod := routing.New()
	secMod := New()

	k.Use(logMod)
	k.Use(adapter)
	k.Use(routingMod)
	k.Use(secMod)

	if err := k.Init(context.Background()); err != nil {
		t.Fatalf("Init: %v", err)
	}

	return adapter, secMod, k
}

func okHandler(ctx *request.Context) error {
	ctx.Writer.WriteHeader(http.StatusOK)
	_, _ = ctx.Writer.Write([]byte("ok"))
	return nil
}

// — Module lifecycle tests —

func TestModuleRegistersMiddleware(t *testing.T) {
	_, _, k := setupKernel(t, `
[modules]
http-security = true
`)

	reg, err := kernel.GetResource[*routing.Registry](k, "routing.registry")
	if err != nil {
		t.Fatalf("get registry: %v", err)
	}

	for _, name := range []string{"security.redirect", "security.csrf", "security.headers"} {
		if _, err := reg.LookupMiddleware(name); err != nil {
			t.Errorf("expected middleware %q to be registered: %v", name, err)
		}
	}
}

func TestModuleProvidedOnServiceLocator(t *testing.T) {
	_, _, k := setupKernel(t, `
[modules]
http-security = true
`)

	mod, err := kernel.GetResource[*Module](k, "http-security")
	if err != nil {
		t.Fatalf("get http-security: %v", err)
	}
	if mod.Name() != "http-security" {
		t.Fatalf("expected name http-security, got %s", mod.Name())
	}
}

// — Security headers tests —

func TestHeadersAppliedGlobally(t *testing.T) {
	adapter, _, _ := setupKernel(t, `
[modules]
http-security = true

[http-security]
csp = "default-src 'self'"
frame_options = "SAMEORIGIN"
referrer_policy = "no-referrer"
`)

	adapter.Router().Get("/page", okHandler)

	req := httptest.NewRequest("GET", "/page", nil)
	w := httptest.NewRecorder()
	adapter.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	tests := map[string]string{
		"X-Content-Type-Options":  "nosniff",
		"Content-Security-Policy": "default-src 'self'",
		"X-Frame-Options":         "SAMEORIGIN",
		"Referrer-Policy":         "no-referrer",
	}
	for header, expected := range tests {
		if got := w.Header().Get(header); got != expected {
			t.Errorf("%s: expected %q, got %q", header, expected, got)
		}
	}
}

func TestHeadersHSTS(t *testing.T) {
	adapter, _, _ := setupKernel(t, `
[modules]
http-security = true

[http-security]
hsts = true
hsts_max_age = 86400
hsts_include_subs = true
hsts_preload = true
`)

	adapter.Router().Get("/secure", okHandler)

	req := httptest.NewRequest("GET", "/secure", nil)
	w := httptest.NewRecorder()
	adapter.ServeHTTP(w, req)

	hsts := w.Header().Get("Strict-Transport-Security")
	if hsts == "" {
		t.Fatal("expected HSTS header")
	}
	if !strings.Contains(hsts, "max-age=86400") {
		t.Errorf("expected max-age=86400, got %q", hsts)
	}
	if !strings.Contains(hsts, "includeSubDomains") {
		t.Errorf("expected includeSubDomains in %q", hsts)
	}
	if !strings.Contains(hsts, "preload") {
		t.Errorf("expected preload in %q", hsts)
	}
}

func TestHeadersDisabledViaDirective(t *testing.T) {
	adapter, _, _ := setupKernel(t, `
[modules]
http-security = true

[http-security]
frame_options = "DENY"
`)

	adapter.Router().Get("/health", okHandler, request.WithMeta("http-security", "none"))

	req := httptest.NewRequest("GET", "/health", nil)
	w := httptest.NewRecorder()
	adapter.ServeHTTP(w, req)

	if got := w.Header().Get("X-Frame-Options"); got != "" {
		t.Errorf("expected no X-Frame-Options on disabled route, got %q", got)
	}
	if got := w.Header().Get("X-Content-Type-Options"); got != "" {
		t.Errorf("expected no X-Content-Type-Options on disabled route, got %q", got)
	}
}

func TestHeadersPermissionsPolicy(t *testing.T) {
	adapter, _, _ := setupKernel(t, `
[modules]
http-security = true

[http-security]
permissions_policy = "camera=(), microphone=()"
`)

	adapter.Router().Get("/app", okHandler)

	req := httptest.NewRequest("GET", "/app", nil)
	w := httptest.NewRecorder()
	adapter.ServeHTTP(w, req)

	if got := w.Header().Get("Permissions-Policy"); got != "camera=(), microphone=()" {
		t.Errorf("expected permissions policy, got %q", got)
	}
}

// — HTTPS redirect tests —

func TestRedirectHTTPToHTTPS(t *testing.T) {
	adapter, _, _ := setupKernel(t, `
[modules]
http-security = true
`)

	adapter.Router().Get("/secure-page", okHandler, request.WithMeta("https_redirect", "true"))

	req := httptest.NewRequest("GET", "/secure-page", nil)
	req.Host = "example.com"
	w := httptest.NewRecorder()
	adapter.ServeHTTP(w, req)

	if w.Code != http.StatusMovedPermanently {
		t.Fatalf("expected 301, got %d", w.Code)
	}
	loc := w.Header().Get("Location")
	if loc != "https://example.com/secure-page" {
		t.Fatalf("expected redirect to https://example.com/secure-page, got %q", loc)
	}
}

func TestRedirectSkippedForHTTPS(t *testing.T) {
	adapter, _, _ := setupKernel(t, `
[modules]
http-security = true
`)

	adapter.Router().Get("/secure-page", okHandler, request.WithMeta("https_redirect", "true"))

	req := httptest.NewRequest("GET", "/secure-page", nil)
	req.Header.Set("X-Forwarded-Proto", "https")
	w := httptest.NewRecorder()
	adapter.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 for HTTPS request, got %d", w.Code)
	}
}

func TestRedirectNotAppliedWithoutDirective(t *testing.T) {
	adapter, _, _ := setupKernel(t, `
[modules]
http-security = true
`)

	adapter.Router().Get("/normal", okHandler)

	req := httptest.NewRequest("GET", "/normal", nil)
	w := httptest.NewRecorder()
	adapter.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestRedirectPreservesQueryString(t *testing.T) {
	adapter, _, _ := setupKernel(t, `
[modules]
http-security = true
`)

	adapter.Router().Get("/search", okHandler, request.WithMeta("https_redirect", "true"))

	req := httptest.NewRequest("GET", "/search?q=test&page=2", nil)
	req.Host = "example.com"
	w := httptest.NewRecorder()
	adapter.ServeHTTP(w, req)

	if w.Code != http.StatusMovedPermanently {
		t.Fatalf("expected 301, got %d", w.Code)
	}
	loc := w.Header().Get("Location")
	if loc != "https://example.com/search?q=test&page=2" {
		t.Fatalf("expected full URL with query, got %q", loc)
	}
}

// — CSRF tests —

func TestCSRFGeneratesTokenOnGET(t *testing.T) {
	adapter, _, _ := setupKernel(t, `
[modules]
http-security = true
`)

	var token string
	adapter.Router().Get("/form", func(ctx *request.Context) error {
		token = CSRFToken(ctx)
		ctx.Writer.WriteHeader(http.StatusOK)
		return nil
	}, request.WithMeta("csrf", "true"))

	req := httptest.NewRequest("GET", "/form", nil)
	w := httptest.NewRecorder()
	adapter.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if token == "" {
		t.Fatal("expected CSRF token to be set in state")
	}
	if len(token) != 64 { // 32 bytes hex-encoded
		t.Fatalf("expected 64-char token, got %d chars", len(token))
	}

	// Verify the CSRF cookie was set.
	var csrfCookie *http.Cookie
	for _, c := range w.Result().Cookies() {
		if c.Name == "ns_csrf" {
			csrfCookie = c
		}
	}
	if csrfCookie == nil {
		t.Fatal("expected ns_csrf cookie")
	}
	if csrfCookie.Value != token {
		t.Fatalf("cookie value %q != state token %q", csrfCookie.Value, token)
	}
}

func TestCSRFRejectsPostWithoutToken(t *testing.T) {
	adapter, _, _ := setupKernel(t, `
[modules]
http-security = true
`)

	adapter.Router().Post("/submit", okHandler, request.WithMeta("csrf", "true"))

	req := httptest.NewRequest("POST", "/submit", nil)
	req.AddCookie(&http.Cookie{Name: "ns_csrf", Value: "known-token"})
	w := httptest.NewRecorder()
	adapter.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", w.Code)
	}
}

func TestCSRFAcceptsPostWithHeaderToken(t *testing.T) {
	adapter, _, _ := setupKernel(t, `
[modules]
http-security = true
`)

	adapter.Router().Post("/submit", okHandler, request.WithMeta("csrf", "true"))

	token := "a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2"
	req := httptest.NewRequest("POST", "/submit", nil)
	req.AddCookie(&http.Cookie{Name: "ns_csrf", Value: token})
	req.Header.Set("X-CSRF-Token", token)
	w := httptest.NewRecorder()
	adapter.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestCSRFAcceptsPostWithFormField(t *testing.T) {
	adapter, _, _ := setupKernel(t, `
[modules]
http-security = true
`)

	adapter.Router().Post("/submit", okHandler, request.WithMeta("csrf", "true"))

	token := "a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2"
	form := url.Values{}
	form.Set("csrf_token", token)
	req := httptest.NewRequest("POST", "/submit", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(&http.Cookie{Name: "ns_csrf", Value: token})
	w := httptest.NewRecorder()
	adapter.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestCSRFRejectsMismatchedToken(t *testing.T) {
	adapter, _, _ := setupKernel(t, `
[modules]
http-security = true
`)

	adapter.Router().Post("/submit", okHandler, request.WithMeta("csrf", "true"))

	req := httptest.NewRequest("POST", "/submit", nil)
	req.AddCookie(&http.Cookie{Name: "ns_csrf", Value: "cookie-token-aaaaaaaaaaaaaaa"})
	req.Header.Set("X-CSRF-Token", "different-token-bbbbbbbbbbbbbbb")
	w := httptest.NewRecorder()
	adapter.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for mismatched token, got %d", w.Code)
	}
}

func TestCSRFNotAppliedWithoutDirective(t *testing.T) {
	adapter, _, _ := setupKernel(t, `
[modules]
http-security = true
`)

	adapter.Router().Post("/open", okHandler)

	req := httptest.NewRequest("POST", "/open", nil)
	w := httptest.NewRecorder()
	adapter.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestCSRFSafeMethodsPassThrough(t *testing.T) {
	adapter, _, _ := setupKernel(t, `
[modules]
http-security = true
`)

	for _, method := range []string{"GET", "HEAD", "OPTIONS"} {
		adapter.Router().Handle(method, "/safe-"+strings.ToLower(method), okHandler, request.WithMeta("csrf", "true"))
	}

	for _, method := range []string{"GET", "HEAD", "OPTIONS"} {
		req := httptest.NewRequest(method, "/safe-"+strings.ToLower(method), nil)
		w := httptest.NewRecorder()
		adapter.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("%s: expected 200, got %d", method, w.Code)
		}
	}
}

func TestCSRFReusesExistingCookie(t *testing.T) {
	adapter, _, _ := setupKernel(t, `
[modules]
http-security = true
`)

	var token string
	adapter.Router().Get("/form", func(ctx *request.Context) error {
		token = CSRFToken(ctx)
		ctx.Writer.WriteHeader(http.StatusOK)
		return nil
	}, request.WithMeta("csrf", "true"))

	existingToken := "existingtoken1234567890abcdef1234567890abcdef1234567890abcdef12"
	req := httptest.NewRequest("GET", "/form", nil)
	req.AddCookie(&http.Cookie{Name: "ns_csrf", Value: existingToken})
	w := httptest.NewRecorder()
	adapter.ServeHTTP(w, req)

	if token != existingToken {
		t.Fatalf("expected existing token %q, got %q", existingToken, token)
	}
}

func TestCSRFRejectsPUTAndDELETE(t *testing.T) {
	adapter, _, _ := setupKernel(t, `
[modules]
http-security = true
`)

	adapter.Router().Handle("PUT", "/item", okHandler, request.WithMeta("csrf", "true"))
	adapter.Router().Handle("DELETE", "/item-del", okHandler, request.WithMeta("csrf", "true"))

	for _, method := range []string{"PUT", "DELETE"} {
		path := "/item"
		if method == "DELETE" {
			path = "/item-del"
		}
		req := httptest.NewRequest(method, path, nil)
		req.AddCookie(&http.Cookie{Name: "ns_csrf", Value: "token"})
		w := httptest.NewRecorder()
		adapter.ServeHTTP(w, req)

		if w.Code != http.StatusForbidden {
			t.Errorf("%s: expected 403, got %d", method, w.Code)
		}
	}
}

func TestCSRFCustomConfig(t *testing.T) {
	adapter, _, _ := setupKernel(t, `
[modules]
http-security = true

[http-security]
csrf_cookie = "my_csrf"
csrf_header = "X-My-CSRF"
csrf_field = "my_token"
`)

	adapter.Router().Post("/custom", okHandler, request.WithMeta("csrf", "true"))

	token := "a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2"
	req := httptest.NewRequest("POST", "/custom", nil)
	req.AddCookie(&http.Cookie{Name: "my_csrf", Value: token})
	req.Header.Set("X-My-CSRF", token)
	w := httptest.NewRecorder()
	adapter.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 with custom header, got %d", w.Code)
	}
}

// — Helper function tests —

func TestTokensMatch(t *testing.T) {
	tests := []struct {
		name     string
		a, b     string
		expected bool
	}{
		{"matching", "abc123", "abc123", true},
		{"different", "abc123", "xyz789", false},
		{"empty expected", "", "abc123", false},
		{"empty submitted", "abc123", "", false},
		{"both empty", "", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tokensMatch(tt.a, tt.b); got != tt.expected {
				t.Errorf("tokensMatch(%q, %q) = %v, want %v", tt.a, tt.b, got, tt.expected)
			}
		})
	}
}

func TestIsSafeMethod(t *testing.T) {
	safe := []string{"GET", "HEAD", "OPTIONS", "TRACE"}
	unsafe := []string{"POST", "PUT", "PATCH", "DELETE"}

	for _, m := range safe {
		if !isSafeMethod(m) {
			t.Errorf("expected %s to be safe", m)
		}
	}
	for _, m := range unsafe {
		if isSafeMethod(m) {
			t.Errorf("expected %s to be unsafe", m)
		}
	}
}

func TestGenerateCSRFToken(t *testing.T) {
	token, err := generateCSRFToken()
	if err != nil {
		t.Fatalf("generateCSRFToken: %v", err)
	}
	if len(token) != 64 {
		t.Fatalf("expected 64-char token, got %d", len(token))
	}

	// Tokens should be unique.
	token2, _ := generateCSRFToken()
	if token == token2 {
		t.Fatal("expected unique tokens")
	}
}
