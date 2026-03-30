// Package response provides the response pipeline: format resolution,
// formatter registration, and serialization.
package response

// Response is the value returned by handlers. The response pipeline
// serializes Data into the resolved format and writes it to the client.
type Response struct {
	// Status is the HTTP status code. Defaults to 200 if not set.
	Status int

	// Headers are additional HTTP headers to set on the response.
	Headers map[string]string

	// Data is the payload to serialize (struct, map, slice, etc.).
	Data any

	// Template is an optional template name for HTML rendering.
	Template string

	// Error, if non-nil, triggers error handling instead of normal serialization.
	Error error
}

// NewResponse creates a response with the given status and data.
func NewResponse(status int, data any) *Response {
	return &Response{
		Status: status,
		Data:   data,
	}
}

// WithTemplate sets the template name for HTML rendering.
func (r *Response) WithTemplate(name string) *Response {
	r.Template = name
	return r
}

// WithHeader adds a header to the response.
func (r *Response) WithHeader(key, value string) *Response {
	if r.Headers == nil {
		r.Headers = make(map[string]string)
	}
	r.Headers[key] = value
	return r
}
