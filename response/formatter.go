package response

import (
	"context"
	"net/http"
)

// Formatter serializes response data into a specific format (JSON, HTML, etc.).
type Formatter interface {
	// Name returns the formatter's identifier (e.g., "json", "html").
	Name() string

	// ContentType returns the MIME type for the HTTP Content-Type header.
	ContentType() string

	// Format serializes the response data into bytes.
	Format(ctx context.Context, resp *Response) ([]byte, error)
}

var httpRequestKey = contextKey{"response.http_request"}

// WithHTTPRequest attaches the HTTP request to a context so formatters
// can access request data (e.g., query parameters).
func WithHTTPRequest(ctx context.Context, r *http.Request) context.Context {
	return context.WithValue(ctx, httpRequestKey, r)
}

// HTTPRequestFromContext retrieves the HTTP request from context.
func HTTPRequestFromContext(ctx context.Context) *http.Request {
	r, _ := ctx.Value(httpRequestKey).(*http.Request)
	return r
}
