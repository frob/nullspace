// Package routing provides declarative TOML-based route configuration.
//
// Routes can be defined in nullspace.toml alongside code-based routes.
// TOML routes are additive — they don't replace the Go API, they layer on top.
package routing

// Config holds the routing module's configuration.
type Config struct {
	// Groups defines named route groups with shared settings.
	Groups map[string]Group `json:"groups" toml:"groups"`

	// Routes defines individual routes.
	Routes []Route `json:"routes" toml:"routes"`

	// Collections defines auto-generated CRUD route sets.
	Collections []Collection `json:"collections" toml:"collections"`
}

// Group defines shared settings for a set of routes.
type Group struct {
	// Prefix is prepended to all route paths in this group.
	Prefix string `json:"prefix" toml:"prefix"`

	// Format is the default response format for routes in this group.
	Format string `json:"format" toml:"format"`

	// Middleware lists named middleware to apply to all routes in this group.
	Middleware []string `json:"middleware" toml:"middleware"`
}

// Route defines a single route.
type Route struct {
	// Group references a named group for shared settings.
	Group string `json:"group" toml:"group"`

	// Path is the URL pattern (e.g., "/posts/:id").
	Path string `json:"path" toml:"path"`

	// Methods lists HTTP methods for this route. Defaults to ["GET"].
	Methods []string `json:"methods" toml:"methods"`

	// Handler is the named handler (e.g., "posts.list", "data.list", "redirect").
	Handler string `json:"handler" toml:"handler"`

	// Format overrides the group's format for this route.
	Format string `json:"format" toml:"format"`

	// Template is the template name for HTML rendering.
	Template string `json:"template" toml:"template"`

	// Middleware lists additional named middleware for this route.
	Middleware []string `json:"middleware" toml:"middleware"`

	// Collection is the data collection name for built-in data handlers.
	Collection string `json:"collection" toml:"collection"`

	// DataParam is the route parameter name for single-entity lookups.
	// Defaults to "id".
	DataParam string `json:"data_param" toml:"data_param"`

	// Redirect is the target URL for the "redirect" handler.
	Redirect string `json:"redirect" toml:"redirect"`

	// StatusCode is the HTTP status for redirects. Defaults to 303.
	StatusCode int `json:"status_code" toml:"status_code"`

	// Session controls session enforcement for this route.
	// Set to "ignore" to bypass session.load and session.require middleware
	// on this route even when the route belongs to an authenticated group.
	Session string `json:"session" toml:"session"`

	// Csrf controls CSRF token enforcement for this route.
	// Set to "true" to require CSRF token validation on state-changing methods.
	// Requires the http-security module to be enabled.
	Csrf string `json:"csrf" toml:"csrf"`

	// HttpsRedirect controls HTTPS enforcement for this route.
	// Set to "true" to redirect HTTP requests to HTTPS.
	// Requires the http-security module to be enabled.
	HttpsRedirect string `json:"https_redirect" toml:"https_redirect"`
}

// Collection defines auto-generated CRUD routes for a data collection.
type Collection struct {
	// Name is the collection name (e.g., "posts").
	Name string `json:"name" toml:"name"`

	// Source is the data module to read from. Defaults to "data.file".
	Source string `json:"source" toml:"source"`

	// APIPrefix is the URL prefix for JSON API routes (e.g., "/api").
	APIPrefix string `json:"api_prefix" toml:"api_prefix"`

	// HTMLPrefix is the URL prefix for HTML routes (e.g., "").
	HTMLPrefix string `json:"html_prefix" toml:"html_prefix"`

	// ListTemplate is the template for the HTML list page.
	ListTemplate string `json:"list_template" toml:"list_template"`

	// ItemTemplate is the template for the HTML single-item page.
	ItemTemplate string `json:"item_template" toml:"item_template"`

	// ReadMiddleware lists named middleware for read operations (GET).
	ReadMiddleware []string `json:"read_middleware" toml:"read_middleware"`

	// WriteMiddleware lists named middleware for write operations (POST/PUT/DELETE).
	WriteMiddleware []string `json:"write_middleware" toml:"write_middleware"`
}

// resolvedRoute is an internal representation after group settings are merged
// and collection routes are expanded.
type resolvedRoute struct {
	Method        string
	Path          string
	Handler       string
	Format        string
	Template      string
	Middleware    []string
	Collection    string
	DataParam     string
	Redirect      string
	StatusCode    int
	Session       string
	Csrf          string
	HttpsRedirect string
}
