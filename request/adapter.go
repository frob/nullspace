package request

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/http"

	"github.com/frob/nullspace/kernel"
	"github.com/frob/nullspace/nslog"
	"github.com/frob/nullspace/response"
)

// AdapterConfig holds the HTTP adapter's configuration.
type AdapterConfig struct {
	Addr string `json:"addr" toml:"addr"`
}

// Adapter is the HTTP request adapter module. It bridges net/http to the
// framework, managing the full request lifecycle: config snapshots, logger
// enrichment, middleware chains, routing, and hook firing.
// ErrNotHandled is returned by a fallback handler to indicate it did not
// handle the request (e.g., static file not found). The adapter continues
// to the next fallback or returns 404.
var ErrNotHandled = fmt.Errorf("not handled")

type Adapter struct {
	kernel    *kernel.Kernel
	router    *Router
	mw        []Middleware
	fallbacks []HandlerFunc
	server    *http.Server
	addr      string
}

// NewAdapter creates a new HTTP adapter module.
func NewAdapter() *Adapter {
	return &Adapter{
		router: NewRouter(),
	}
}

func (a *Adapter) Name() string { return "request" }

func (a *Adapter) Config() kernel.ModuleConfig {
	return kernel.ModuleConfig{
		Key: "request",
		Default: AdapterConfig{
			Addr: ":8080",
		},
		DefaultEnabled: true,
	}
}

func (a *Adapter) Init(k *kernel.Kernel) error {
	a.kernel = k

	var cfg AdapterConfig
	if err := k.Config().Decode("request", &cfg); err != nil {
		cfg = AdapterConfig{Addr: ":8080"}
	}
	a.addr = cfg.Addr

	k.Provide("router", a.router)
	k.Provide("request.adapter", a)

	return nil
}

func (a *Adapter) Start(ctx context.Context) error {
	a.server = &http.Server{
		Addr:    a.addr,
		Handler: a,
	}

	go func() {
		a.kernel.Logger().Info("http server listening", "addr", a.addr)
		if err := a.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			a.kernel.Logger().Error("http server error", "error", err)
		}
	}()

	return nil
}

func (a *Adapter) Stop(ctx context.Context) error {
	if a.server != nil {
		return a.server.Shutdown(ctx)
	}
	return nil
}

// Use adds global middleware to the adapter. Global middleware wraps
// every request, executing before route-specific middleware.
func (a *Adapter) Use(mw ...Middleware) {
	a.mw = append(a.mw, mw...)
}

// Router returns the adapter's router for route registration.
func (a *Adapter) Router() *Router {
	return a.router
}

// Fallback registers a fallback handler that is tried when no dynamic route
// matches. Fallbacks are tried in registration order. A fallback should
// return ErrNotHandled if it cannot serve the request, allowing the next
// fallback to try. If all fallbacks return ErrNotHandled, the adapter
// returns 404.
func (a *Adapter) Fallback(handler HandlerFunc) {
	a.fallbacks = append(a.fallbacks, handler)
}

// ServeHTTP implements http.Handler. It runs the full request lifecycle:
//
//  1. Snapshot config
//  2. Create per-request logger with request ID, method, path
//  3. Fire request.received hooks
//  4. Match route (dynamic routes first, 404 if no match)
//  5. Fire request.routed hooks
//  6. Build middleware chain (global + route-specific)
//  7. Fire request.before hooks
//  8. Execute handler
//  9. Fire request.after hooks
//  10. Fire request.complete hooks
func (a *Adapter) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// 1. Snapshot config.
	snap := a.kernel.Config().Snapshot()

	// 2. Create per-request context with logger.
	rid := generateRequestID()
	reqLogger := a.kernel.Logger().With(
		"request_id", rid,
		"method", r.Method,
		"path", r.URL.Path,
	)

	ctx := r.Context()
	ctx = kernel.ContextWithSnapshot(ctx, snap)
	ctx = nslog.WithRequestID(ctx, rid)
	ctx = nslog.WithRequestInfo(ctx, r.Method, r.URL.Path)
	ctx = nslog.WithLogger(ctx, reqLogger)

	// Populate response format context values and HTTP request for formatters.
	ctx = response.WithHTTPRequest(ctx, r)
	ctx = response.WithQueryFormat(ctx, r.URL.Query().Get("format"))
	ctx = response.WithAcceptHeader(ctx, r.Header.Get("Accept"))

	// Wrap response writer to capture status code.
	capture := &responseCapture{ResponseWriter: w, status: http.StatusOK}

	// 3. Fire request.received.
	_ = a.kernel.Fire("request.received", ctx)

	// 4. Match route.
	match := a.router.Match(r.Method, r.URL.Path)

	if match == nil {
		reqLogger.Debug("no route matched, trying fallbacks", "path", r.URL.Path)

		// Try fallback handlers (e.g., static files).
		if a.tryFallbacks(capture, r, ctx, reqLogger) {
			ctx = nslog.WithResponseStatus(ctx, capture.status)
			_ = a.kernel.Fire("request.complete", ctx)
			return
		}

		// No fallback handled it — 404.
		_ = a.kernel.Fire("request.routed", ctx)
		capture.WriteHeader(http.StatusNotFound)
		fmt.Fprint(capture, "404 not found")

		ctx = nslog.WithResponseStatus(ctx, http.StatusNotFound)
		_ = a.kernel.Fire("request.complete", ctx)
		return
	}

	reqLogger.Debug("route matched", "pattern", match.Pattern)

	// Add route format metadata to context for format resolution.
	if fmt, ok := match.Meta["format"]; ok {
		ctx = response.WithRouteFormat(ctx, fmt)
	}

	// 5. Fire request.routed.
	_ = a.kernel.Fire("request.routed", ctx)

	// Build framework context.
	fctx := newContext(capture, r.WithContext(ctx), ctx)
	fctx.params = match.Params
	fctx.route = match

	// 6. Build middleware chain: global + route-specific.
	handler := match.handler
	if len(match.mw) > 0 {
		handler = buildChain(handler, match.mw)
	}
	if len(a.mw) > 0 {
		handler = buildChain(handler, a.mw)
	}

	// 7. Fire request.before.
	_ = a.kernel.Fire("request.before", ctx)

	// 8. Execute handler.
	err := handler(fctx)

	// 9. Fire request.after.
	_ = a.kernel.Fire("request.after", ctx)

	// Handle errors.
	if err != nil {
		reqLogger.Error("handler error", "error", err)
		if !capture.written {
			capture.WriteHeader(http.StatusInternalServerError)
			fmt.Fprint(capture, "500 internal server error")
		}
	}

	// 10. Fire request.complete.
	ctx = nslog.WithResponseStatus(ctx, capture.status)
	_ = a.kernel.Fire("request.complete", ctx)
}

// tryFallbacks attempts each registered fallback handler in order.
// Returns true if a fallback handled the request.
func (a *Adapter) tryFallbacks(w *responseCapture, r *http.Request, ctx context.Context, logger kernel.Logger) bool {
	for _, fb := range a.fallbacks {
		fctx := newContext(w, r.WithContext(ctx), ctx)
		err := fb(fctx)
		if err == nil {
			// Fallback handled the request.
			logger.Debug("fallback handled request")
			return true
		}
		if err != ErrNotHandled {
			// Fallback had a real error.
			logger.Error("fallback error", "error", err)
			if !w.written {
				w.WriteHeader(http.StatusInternalServerError)
				fmt.Fprint(w, "500 internal server error")
			}
			return true
		}
		// ErrNotHandled — try next fallback.
	}
	return false
}

// responseCapture wraps http.ResponseWriter to capture the status code.
type responseCapture struct {
	http.ResponseWriter
	status  int
	written bool
}

func (w *responseCapture) WriteHeader(code int) {
	if !w.written {
		w.status = code
		w.written = true
		w.ResponseWriter.WriteHeader(code)
	}
}

func (w *responseCapture) Write(b []byte) (int, error) {
	if !w.written {
		w.written = true
	}
	return w.ResponseWriter.Write(b)
}

// generateRequestID creates a short random hex ID for request tracing.
func generateRequestID() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
