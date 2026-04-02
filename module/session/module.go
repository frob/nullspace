package session

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"time"

	"github.com/frob/nullspace/core/request"
	"github.com/frob/nullspace/core/routing"
	"github.com/frob/nullspace/kernel"
	datasql "github.com/frob/nullspace/module/data/sql"
)

// Config holds the session module's configuration.
type Config struct {
	// Store selects the backing store: "memory" (default) or "sql".
	Store string `json:"store" toml:"store"`
	// Cookie is the name of the session cookie. Defaults to "ns_session".
	Cookie string `json:"cookie" toml:"cookie"`
	// TTL is the session lifetime as a duration string (e.g. "24h", "30m").
	TTL string `json:"ttl" toml:"ttl"`
	// Secure sets the Secure attribute on the session cookie.
	Secure bool `json:"secure" toml:"secure"`
	// Path sets the cookie path. Defaults to "/".
	Path string `json:"path" toml:"path"`
}

// Module provides session management via three named middleware entries
// registered on the routing registry:
//
//   - session.load    — loads session from cookie; no enforcement
//   - session.require — enforces valid session; 401 or redirect if absent
//   - session.ignore  — no-op; routes use session = "ignore" in TOML to
//     bypass enforcement even when inside an authenticated group
//
// DefaultEnabled is false — apps opt in via [modules] session = true.
type Module struct {
	kernel *kernel.Kernel
	store  Store
	cfg    Config
}

// New creates a new session module.
func New() *Module {
	return &Module{}
}

func (m *Module) Name() string { return "session" }

func (m *Module) Config() kernel.ModuleConfig {
	return kernel.ModuleConfig{
		Key: "session",
		Default: Config{
			Store:  "memory",
			Cookie: "ns_session",
			TTL:    "24h",
			Secure: false,
			Path:   "/",
		},
		DefaultEnabled: false,
	}
}

func (m *Module) Init(k *kernel.Kernel) error {
	m.kernel = k

	if err := k.Config().Decode("session", &m.cfg); err != nil {
		m.cfg = Config{Store: "memory", Cookie: "ns_session", TTL: "24h", Path: "/"}
	}

	ttl, err := time.ParseDuration(m.cfg.TTL)
	if err != nil {
		ttl = 24 * time.Hour
	}

	switch m.cfg.Store {
	case "sql":
		db, err := kernel.GetResource[*sql.DB](k, "db")
		if err != nil {
			return fmt.Errorf("session: sql store requires data.sql module: %w", err)
		}

		reg, err := kernel.GetResource[*datasql.MigrationRegistry](k, "data.sql.migrations")
		if err != nil {
			return fmt.Errorf("session: migration registry not found: %w", err)
		}
		reg.Register("session", datasql.Migration{
			Version:     1,
			Description: "create sessions table",
			Up: func(ctx context.Context, tx *sql.Tx) error {
				_, err := tx.ExecContext(ctx, `
					CREATE TABLE IF NOT EXISTS sessions (
						id         TEXT PRIMARY KEY,
						data       TEXT NOT NULL DEFAULT '{}',
						expires_at INTEGER NOT NULL
					)
				`)
				return err
			},
		})

		m.store = NewSQLStore(db, ttl)
	default:
		m.store = NewMemoryStore(ttl)
	}

	k.Provide("session.store", m.store)
	k.Provide("session", m)

	reg, err := kernel.GetResource[*routing.Registry](k, "routing.registry")
	if err != nil {
		return fmt.Errorf("session: routing registry not found — register routing module before session: %w", err)
	}

	reg.Middleware("session.load", m.loadMiddleware())
	reg.Middleware("session.require", m.requireMiddleware())
	reg.Middleware("session.ignore", m.ignoreMiddleware())

	return nil
}

func (m *Module) Start(ctx context.Context) error { return nil }
func (m *Module) Stop(ctx context.Context) error  { return nil }

// Create creates a new session and sets the session cookie on the response.
// Call this after successful authentication in a login handler.
//
//	sess, err := sessionMod.Create(ctx)
//	sess.Set("user_id", user.ID)
func (m *Module) Create(ctx *request.Context) (*Session, error) {
	sess, err := m.store.Create(ctx.Context())
	if err != nil {
		return nil, fmt.Errorf("create session: %w", err)
	}
	if !ctx.Hijacked() {
		m.setCookie(ctx, sess.ID)
	}
	ctx.SetState(stateKey, sess)
	_ = m.kernel.Fire("session.created", ctx.Context())
	return sess, nil
}

// Destroy deletes the current session and clears the session cookie.
// Call this from logout handlers.
func (m *Module) Destroy(ctx *request.Context) error {
	cookie, err := ctx.Request.Cookie(m.cfg.Cookie)
	if err != nil {
		return nil // no session to destroy
	}
	if err := m.store.Delete(ctx.Context(), cookie.Value); err != nil {
		return fmt.Errorf("destroy session: %w", err)
	}
	if !ctx.Hijacked() {
		http.SetCookie(ctx.Writer, &http.Cookie{
			Name:     m.cfg.Cookie,
			Value:    "",
			Path:     m.cfg.Path,
			MaxAge:   -1,
			HttpOnly: true,
		})
	}
	ctx.SetState(stateKey, nil)
	_ = m.kernel.Fire("session.destroyed", ctx.Context())
	return nil
}

// Store returns the underlying session store.
// Use this to interact with sessions outside of the standard middleware flow.
func (m *Module) Store() Store {
	return m.store
}

// loadMiddleware loads the session from the cookie if present. Passes through
// without error if there is no cookie or the session has expired. Automatically
// saves the session after the handler if it was modified.
func (m *Module) loadMiddleware() request.Middleware {
	return func(next request.HandlerFunc) request.HandlerFunc {
		return func(ctx *request.Context) error {
			if isIgnored(ctx) {
				return next(ctx)
			}

			cookie, err := ctx.Request.Cookie(m.cfg.Cookie)
			if err != nil {
				return next(ctx)
			}

			sess, err := m.store.Load(ctx.Context(), cookie.Value)
			if err != nil {
				m.kernel.Logger().Warn("session load error", "error", err)
				return next(ctx)
			}
			if sess != nil {
				ctx.SetState(stateKey, sess)
				_ = m.kernel.Fire("session.loaded", ctx.Context())
			}

			handlerErr := next(ctx)

			if sess != nil && sess.IsDirty() {
				if saveErr := m.store.Save(ctx.Context(), sess); saveErr != nil {
					m.kernel.Logger().Error("session save error", "error", saveErr)
				}
			}

			return handlerErr
		}
	}
}

// requireMiddleware enforces that a valid session exists. Returns 401 if no
// session is found, or redirects to the URL provided by a session.login_url
// resolver hook if one is registered.
//
// Routes inside authenticated groups can opt out by setting session = "ignore"
// in their TOML route definition.
func (m *Module) requireMiddleware() request.Middleware {
	return func(next request.HandlerFunc) request.HandlerFunc {
		return func(ctx *request.Context) error {
			if isIgnored(ctx) {
				return next(ctx)
			}

			cookie, err := ctx.Request.Cookie(m.cfg.Cookie)
			if err != nil {
				return m.unauthorized(ctx)
			}

			sess, err := m.store.Load(ctx.Context(), cookie.Value)
			if err != nil {
				m.kernel.Logger().Warn("session load error", "error", err)
				return m.unauthorized(ctx)
			}
			if sess == nil {
				return m.unauthorized(ctx)
			}

			ctx.SetState(stateKey, sess)
			_ = m.kernel.Fire("session.loaded", ctx.Context())

			handlerErr := next(ctx)

			if sess.IsDirty() {
				if saveErr := m.store.Save(ctx.Context(), sess); saveErr != nil {
					m.kernel.Logger().Error("session save error", "error", saveErr)
				}
			}

			return handlerErr
		}
	}
}

// ignoreMiddleware is a no-op. Its presence in a middleware list is purely
// documentation-friendly. The actual opt-out mechanism is the route-level
// session = "ignore" TOML field, which sets route metadata read by
// session.load and session.require via isIgnored().
func (m *Module) ignoreMiddleware() request.Middleware {
	return func(next request.HandlerFunc) request.HandlerFunc {
		return func(ctx *request.Context) error {
			return next(ctx)
		}
	}
}

func (m *Module) setCookie(ctx *request.Context, id string) {
	http.SetCookie(ctx.Writer, &http.Cookie{
		Name:     m.cfg.Cookie,
		Value:    id,
		Path:     m.cfg.Path,
		HttpOnly: true,
		Secure:   m.cfg.Secure,
		SameSite: http.SameSiteLaxMode,
	})
}

// unauthorized writes a 401 or redirects to a login URL if a resolver provides one.
// Register a session.login_url resolver hook to redirect instead of returning 401:
//
//	k.HookResolve("session.login_url", 10, func(ctx context.Context) (any, bool, error) {
//	    return "/login", true, nil
//	})
func (m *Module) unauthorized(ctx *request.Context) error {
	loginURL, err := m.kernel.Resolve("session.login_url", ctx.Context())
	if err == nil && loginURL != nil {
		if url, ok := loginURL.(string); ok {
			http.Redirect(ctx.Writer, ctx.Request, url, http.StatusSeeOther)
			return nil
		}
	}
	ctx.Writer.WriteHeader(http.StatusUnauthorized)
	_, _ = ctx.Writer.Write([]byte("401 unauthorized"))
	return nil
}
