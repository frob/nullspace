package request

// HandlerFunc is the framework's handler signature. Handlers receive a
// framework Context and return an error. If the handler returns a non-nil
// error and has not written a response, the adapter writes a 500.
type HandlerFunc func(ctx *Context) error

// Middleware wraps a HandlerFunc, adding behavior before and/or after
// the next handler in the chain. This is the standard functional
// middleware pattern used throughout the framework.
type Middleware func(HandlerFunc) HandlerFunc

// buildChain wraps a handler with middleware in reverse order so that
// the first middleware in the slice executes first.
//
// Given middleware [A, B, C] and handler H, the resulting call order is:
// A -> B -> C -> H -> C -> B -> A
func buildChain(handler HandlerFunc, mw []Middleware) HandlerFunc {
	for i := len(mw) - 1; i >= 0; i-- {
		handler = mw[i](handler)
	}
	return handler
}
