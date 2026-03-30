package session

import "github.com/frob/nullspace/request"

const stateKey = "session"

// From retrieves the current session from the request context.
// Returns nil and false if no session has been loaded for this request.
//
// Example usage in a handler:
//
//	sess, ok := session.From(ctx)
//	if ok {
//	    userID, _ := sess.Get("user_id")
//	}
func From(ctx *request.Context) (*Session, bool) {
	v, ok := ctx.State(stateKey)
	if !ok {
		return nil, false
	}
	s, ok := v.(*Session)
	return s, ok
}

// isIgnored reports whether the matched route has declared session = "ignore".
// This allows individual routes inside an authenticated group to opt out of
// session enforcement without changing group-level middleware.
func isIgnored(ctx *request.Context) bool {
	r := ctx.Route()
	if r == nil {
		return false
	}
	return r.Meta["session"] == "ignore"
}
