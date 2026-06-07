---
title: Middleware
weight: 4
---

Nullspace has two ways to put behaviour in front of a handler: middleware
and hooks. The previous page covered hooks. This one is the other side of
that coin: when middleware is the right tool, what it can do that hooks
cannot, and why the framework keeps both rather than picking one.

## The standard Go pattern

Middleware is the idiomatic Go pattern with no surprises:

```go
type HandlerFunc func(ctx *Context) error
type Middleware  func(HandlerFunc) HandlerFunc

func timing(next request.HandlerFunc) request.HandlerFunc {
    return func(ctx *request.Context) error {
        start := time.Now()
        err := next(ctx)
        ctx.Logger().Info("handled", "duration", time.Since(start))
        return err
    }
}

adapter.Use(timing)
```

A middleware wraps a handler and returns a new handler. The chain runs in
registration order: `adapter.Use(A); adapter.Use(B); adapter.Use(C)` with
handler `H` produces

```
A-before -> B-before -> C-before -> H -> C-after -> B-after -> A-after
```

Route-specific middleware sits between global middleware and the handler:

```
G-before -> R-before -> H -> R-after -> G-after
```

There is nothing exotic in this. It is the same pattern as `net/http` and
every other Go web framework. Nullspace's `request.Context` adds a few
conveniences (per-request logger, config snapshot, state bag) but the
shape is unchanged.

## What middleware can do that hooks cannot

Three things, all related to the HTTP response:

1. **Read and write the response stream.** A middleware has the
   `http.ResponseWriter`. It can set headers, write the body, change the
   status code. A hook only has `context.Context` — it cannot reach the
   writer.
2. **Short-circuit the request.** A middleware can decide not to call
   `next` and return early with a 401 or a 429. A hook returning an
   error aborts the chain, but it cannot say "I have already produced
   the response, stop here cleanly."
3. **Wrap behaviour around the handler.** Middleware has a "before" and
   an "after" in the same function — they share local variables and the
   defer stack. Two separate hooks at `request.before` and
   `request.after` cannot trivially share state.

Authentication, CORS, rate limiting, request body parsing, response
compression, request logging — all of these need at least one of the
above. They are middleware.

## What hooks do that middleware cannot

Three things, all related to *not* being on the HTTP path:

1. **Fire at non-request lifecycle points.** Hooks run at
   `kernel.after_init`, `data.before_read`, `websocket.connected`,
   `tcp.message`. There is no request, no `http.ResponseWriter`,
   nothing for middleware to wrap.
2. **Be invoked from non-HTTP transports.** The TCP and IPC adapters
   route messages through their own command router; they do not run the
   HTTP middleware chain. The same goes for WebSocket messages and
   internal data operations. If you want behaviour to run regardless of
   transport, it has to be a hook.
3. **Participate in resolution chains.** Format resolution is a
   resolution hook because it asks a question and expects an answer.
   Middleware cannot return a typed value to a caller; it can only call
   `next` and return an error.

Logging, metrics, audit trails, policy enforcement, format negotiation —
these are hooks, because they need to run at points where there is no
HTTP middleware chain to register with.

## The spectrum

It is useful to think of these as positions on a single axis from
narrowest scope to widest:

```
                 narrow                                 wide
                   |                                      |
    middleware    request hooks      application       custom
                                       hooks           modules
   (HTTP-only)   (HTTP lifecycle    (data, websocket,  (capability +
                  events)            cross-component)   its own state)
```

Picking from this spectrum is a question of "how much of the system needs
to know about this behaviour":

- **Only the HTTP path needs to know.** Middleware. Auth on `/admin`, CORS,
  compression, body parsing.
- **The HTTP path plus the lifecycle around it.** Request hook plus a
  middleware. A request ID middleware that sets a header and an
  `request.complete` hook that logs the duration.
- **Multiple components need to coordinate.** Application hooks. Policy
  on `data.before_read`, audit on `data.after_write`, cache
  invalidation on `data.after_write`.
- **Behaviour is its own capability with state and config.** Custom
  module. The module owns its lifecycle and registers whatever
  combination of hooks, middleware, and routes it needs to do its job.

Most non-trivial features cross at least two of these. Sessions, for
example, are a module (lifecycle, store management, configuration), a
middleware (loading and saving on each request), and at least one hook
(cleanup on shutdown). The module is the home and the others hang off it.

## A worked example: an audit module

Suppose you want to record every successful write to the audit log. Where
does that live?

You could put it in a middleware on the write routes. That works for the
HTTP path, but if anyone writes through the TCP data bridge, or through
the WebSocket connection, or through the SQL module's own internal
calls, the middleware does not run.

You could put it on `data.after_write`. That catches every write through
any path — HTTP, TCP, IPC, internal. It runs after the write has
succeeded, with the entity already saved. The hook does not need to know
which transport produced it.

The audit module is therefore a module that registers a single hook on
`data.after_write` at priority 90. It does not register any middleware.
It does not register any routes. It does not appear on the HTTP request
path at all. But every write, from every transport, runs through it.

If you needed to *prevent* a write — "users without `write:posts` cannot
update posts" — you would use `data.before_write` instead, and return an
error if the policy fails. The hook bus aborts on error, which is the
short-circuit mechanism for non-HTTP code.

## A worked example: an auth middleware

Now suppose you want HTTP requests with a missing or invalid token to be
rejected with a 401, with the user identity attached to the context for
valid requests.

This is a middleware. The reason is item 1 in the list above: it needs
to write a 401 response. A hook on `request.before` could read the
header and put the user in the context, but it has no way to write a
401 — it can return an error, which the framework will format into an
error response, but you have given up control of the status code and
body. Middleware can write the response directly.

You could package this middleware as a module so it can be
enabled/disabled via config, declare its own configuration section, and
log its setup through the kernel logger. That is the recommended shape
for non-trivial middleware. The module wires the middleware into the
request adapter during `Init`; the rest of the request path does not
need to know.

## Named middleware and TOML routes

Middleware can be registered by name and referenced from declarative
routes:

```go
reg.Middleware("auth", authMiddleware)
```

```toml
[[routing.routes]]
path = "/api/admin/users"
handler = "data.list"
collection = "users"
middleware = ["auth"]
```

This lets the TOML route declaration name middleware without the routing
TOML knowing anything about Go. The middleware registry sits between
them. The same trick works for handlers — TOML routes name a handler,
and the registry maps the name to a function.

This is the pragmatic reason middleware survived even though hooks could
have done most of its work: TOML routes need to name behaviour, and the
named-middleware registry is the natural place for that.

## Where to read next

- [The hook bus]({{< relref "/explanation/hooks" >}}) — the other half
  of this comparison.
- [Multi-transport]({{< relref "/explanation/transports" >}}) — why
  behaviour that needs to apply across HTTP, TCP, and IPC has to be a
  hook.
- [Routing TOML]({{< relref "/reference" >}}) — how named middleware is
  referenced from declarative routes.
