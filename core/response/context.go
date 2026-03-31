package response

import "context"

type contextKey struct{ name string }

var (
	routeFormatKey  = contextKey{"response.route_format"}
	queryFormatKey  = contextKey{"response.query_format"}
	acceptHeaderKey = contextKey{"response.accept_header"}
)

// WithRouteFormat attaches the route's format metadata to a context.
// Called by the request adapter after route matching.
func WithRouteFormat(ctx context.Context, format string) context.Context {
	return context.WithValue(ctx, routeFormatKey, format)
}

// WithQueryFormat attaches the ?format= query param value to a context.
func WithQueryFormat(ctx context.Context, format string) context.Context {
	return context.WithValue(ctx, queryFormatKey, format)
}

// WithAcceptHeader attaches the Accept header value to a context.
func WithAcceptHeader(ctx context.Context, accept string) context.Context {
	return context.WithValue(ctx, acceptHeaderKey, accept)
}

func routeFormatFromContext(ctx context.Context) string {
	s, _ := ctx.Value(routeFormatKey).(string)
	return s
}

func queryFormatFromContext(ctx context.Context) string {
	s, _ := ctx.Value(queryFormatKey).(string)
	return s
}

func acceptHeaderFromContext(ctx context.Context) string {
	s, _ := ctx.Value(acceptHeaderKey).(string)
	return s
}
