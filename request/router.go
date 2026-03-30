package request

import (
	"strings"
)

// RouteOption configures a route during registration.
type RouteOption func(*routeEntry)

// WithMeta attaches metadata to a route. Metadata is available via
// RouteMatch.Meta and can be used by modules (e.g., format override).
func WithMeta(key, value string) RouteOption {
	return func(e *routeEntry) {
		e.meta[key] = value
	}
}

// WithRouteMiddleware attaches middleware specific to this route.
// Route middleware executes after global middleware.
func WithRouteMiddleware(mw ...Middleware) RouteOption {
	return func(e *routeEntry) {
		e.mw = append(e.mw, mw...)
	}
}

// segment represents one part of a parsed URL pattern.
type segment struct {
	literal string // non-empty for literal segments
	param   string // non-empty for parameter segments (e.g., ":id" -> "id")
}

// routeEntry is a registered route.
type routeEntry struct {
	method   string
	pattern  string
	handler  HandlerFunc
	meta     map[string]string
	segments []segment
	mw       []Middleware
}

// RouteMatch holds the result of a successful route match.
type RouteMatch struct {
	Pattern string
	Params  map[string]string
	Meta    map[string]string
	handler HandlerFunc
	mw      []Middleware
}

// Router holds registered routes and matches incoming requests.
type Router struct {
	routes []routeEntry
}

// NewRouter creates an empty router.
func NewRouter() *Router {
	return &Router{}
}

// Handle registers a route for the given HTTP method and URL pattern.
// Pattern segments starting with ":" are treated as path parameters.
func (r *Router) Handle(method, pattern string, handler HandlerFunc, opts ...RouteOption) {
	entry := routeEntry{
		method:   strings.ToUpper(method),
		pattern:  pattern,
		handler:  handler,
		meta:     make(map[string]string),
		segments: parsePattern(pattern),
	}

	for _, opt := range opts {
		opt(&entry)
	}

	r.routes = append(r.routes, entry)
}

// Get registers a GET route.
func (r *Router) Get(pattern string, handler HandlerFunc, opts ...RouteOption) {
	r.Handle("GET", pattern, handler, opts...)
}

// Post registers a POST route.
func (r *Router) Post(pattern string, handler HandlerFunc, opts ...RouteOption) {
	r.Handle("POST", pattern, handler, opts...)
}

// Put registers a PUT route.
func (r *Router) Put(pattern string, handler HandlerFunc, opts ...RouteOption) {
	r.Handle("PUT", pattern, handler, opts...)
}

// Delete registers a DELETE route.
func (r *Router) Delete(pattern string, handler HandlerFunc, opts ...RouteOption) {
	r.Handle("DELETE", pattern, handler, opts...)
}

// Patch registers a PATCH route.
func (r *Router) Patch(pattern string, handler HandlerFunc, opts ...RouteOption) {
	r.Handle("PATCH", pattern, handler, opts...)
}

// Match finds the first route matching the given method and path.
// Returns nil if no route matches.
func (r *Router) Match(method, path string) *RouteMatch {
	pathSegments := splitPath(path)
	method = strings.ToUpper(method)

	for _, entry := range r.routes {
		if entry.method != method {
			continue
		}
		if params, ok := matchSegments(entry.segments, pathSegments); ok {
			meta := make(map[string]string, len(entry.meta))
			for k, v := range entry.meta {
				meta[k] = v
			}
			return &RouteMatch{
				Pattern: entry.pattern,
				Params:  params,
				Meta:    meta,
				handler: entry.handler,
				mw:      entry.mw,
			}
		}
	}
	return nil
}

// parsePattern splits a URL pattern into typed segments.
func parsePattern(pattern string) []segment {
	parts := splitPath(pattern)
	segments := make([]segment, len(parts))

	for i, part := range parts {
		if strings.HasPrefix(part, ":") {
			segments[i] = segment{param: part[1:]}
		} else {
			segments[i] = segment{literal: part}
		}
	}
	return segments
}

// splitPath splits a URL path into non-empty segments.
func splitPath(path string) []string {
	var parts []string
	for _, p := range strings.Split(path, "/") {
		if p != "" {
			parts = append(parts, p)
		}
	}
	return parts
}

// matchSegments checks if path segments match a route's pattern segments,
// extracting path parameters along the way.
func matchSegments(pattern []segment, path []string) (map[string]string, bool) {
	if len(pattern) != len(path) {
		return nil, false
	}

	params := make(map[string]string)
	for i, seg := range pattern {
		if seg.param != "" {
			params[seg.param] = path[i]
		} else if seg.literal != path[i] {
			return nil, false
		}
	}
	return params, true
}
