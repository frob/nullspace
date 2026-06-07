---
title: The hook bus
weight: 3
---

The hook bus is the answer to a single question: how does a module add
behaviour to a part of the system it does not own? Without it, every
cross-cutting concern — auth, metrics, audit, policy, format negotiation —
would have to be coded into the module that produces the event it cares
about. With it, that module fires a named hook and the cross-cutting
modules opt in.

This page explains why the hook bus exists at all, the difference between
fire-and-forget hooks and resolution hooks, what "priority" actually
means in practice, how the per-request config snapshot changes dispatch,
and — the question that comes up most — when to use a hook versus a
middleware versus writing a whole new module.

## What it is and why

There are two ways for module A to call into module B. The direct way is
an import: A imports B and calls a function. The indirect way is a named
event: A fires `"something.happened"` and any module that registered for
that name runs.

The direct way is fine when A genuinely depends on B. The request adapter
genuinely depends on the response pipeline. They are tightly coupled and
the import expresses that.

The indirect way is necessary when A produces an event that anyone might
care about and A does not want to know who. The file data module reads an
entity. Should it call into a policy module to check authorisation? An
audit module to log the read? A cache module to refresh a TTL? None of
these things are the file module's business. So it fires
`data.before_read` and `data.after_read` and lets whoever is listening
respond. If nothing is listening, nothing happens. The file module is the
same in either case.

This is what people mean by "aspect-oriented" — behaviour that cuts across
component boundaries is registered at a point in the lifecycle rather than
embedded in the components themselves. The hook bus is that registration
point.

## Two flavours: fire-and-forget and resolution

There are two kinds of hooks, and they look almost the same but mean very
different things.

### Fire-and-forget

```go
k.Hook("request.complete", 80, func(ctx context.Context) error {
    metrics.Inc("requests_total")
    return nil
})
```

Every registered handler runs, in priority order, until either all of
them have run or one returns an error. There is no value to return. The
hook just happens.

This is the right shape when the producer is announcing something — "a
request finished", "a record was written" — and listeners can react
however they like. None of the listeners can change what the producer
does next, because they have no return channel.

### Resolution

```go
k.HookResolve("response.format.resolve", 20,
    func(ctx context.Context) (any, bool, error) {
        if format := formatFromQuery(ctx); format != "" {
            return format, true, nil   // resolved, stop the chain
        }
        return nil, false, nil          // not resolved, try next
    })
```

Handlers run in priority order, but each returns `(value, resolved, err)`.
The first one to return `resolved == true` wins; its value is what the
caller receives, and no further handlers run. If nobody resolves, the
caller gets `nil`.

This is the right shape when the producer is asking a question — "what
format should this response be?" — and wants a single answer drawn from a
list of strategies, in order of preference. Format resolution is the
canonical example: route override beats query parameter beats `Accept`
header beats default. Each is a separate module with its own priority,
and adding a new strategy is a matter of registering at the right number.

The distinction matters because the two cannot be mixed. A
fire-and-forget hook cannot stop the chain (returning an error aborts,
but you cannot say "I handled this, stop"). A resolution hook is read for
its return value (firing one with `Fire` instead of `Resolve` would lose
the answer). Pick the one that matches the question you are asking.

## Priority, in practice

Priority is an `int`. Lower runs first. There is no magic in any specific
number — these are just sort keys.

The hook bus stores hooks in a slice sorted by priority. When `Fire` or
`Resolve` runs, it walks that slice in order. Equal priorities run in
registration order, but you should not depend on that.

The unwritten convention used by built-in modules:

| Range  | Use                                     |
|--------|-----------------------------------------|
| 1-19   | Framework internals, early-stage setup  |
| 20-49  | Application middleware, normal behaviour|
| 50-79  | Application logic, business rules       |
| 80-99  | Cleanup, late-stage processing          |

These are conventions, not constraints. You can register at priority
1000 and the kernel does not care. The convention exists so that two
modules added independently do not collide on the exact same number.

Why is priority a number and not a "before X / after Y" relationship?
Numbers are simple to reason about in isolation. A relationship-based
system would require every module to know the names of every other module
it wants to order against. With numbers, a new module picks a slot and
slides in; it does not need to coordinate with anyone.

The cost is that two modules can legitimately want to run at the same
point and have no good way to argue about which goes first. In practice
this rarely matters — most hook points have a handful of registrations,
each at a different priority band, and ordering is obvious.

## Config-aware dispatch

The hook bus reads the config snapshot off the context before it dispatches:

```
Fire(name, ctx)
  -> snap = SnapshotFromContext(ctx)
  -> for entry in hooks[name] sorted by priority:
       if snap != nil and not snap.ModuleEnabled(entry.module):
           skip
       else:
           entry.handler(ctx)
```

Every hook is tagged with the name of the module that registered it. If
that module is disabled in the snapshot, the handler does not run. If the
snapshot is `nil` (no request context, or a kernel-lifecycle hook), every
handler runs.

This is what makes it safe to disable modules at runtime. A request that
started before the change still sees the module enabled, because its
snapshot was taken at the start of the request and is immutable for the
rest of it. A request that starts after the change does not. Two
concurrent requests can produce different hook chains; neither needs to
know that the other is happening.

Note the wrinkle: a hook registered outside any module — for example,
directly on the kernel before any module's `Init` runs — is tagged with
the `"kernel"` module. The kernel module is always enabled. There is no
way to disable it.

## Hook versus middleware versus module

The three mechanisms overlap. All of them can run code before or after a
request. The differences matter when the overlap is real.

| Property                       | Hook                    | Middleware            | New module             |
|--------------------------------|-------------------------|-----------------------|------------------------|
| Scope                          | Any lifecycle point     | Request only          | Anything               |
| Access to `http.ResponseWriter`| No (only `context.Context`) | Yes               | Indirect, via locator  |
| Can short-circuit the request  | No (errors abort)       | Yes (skip `next`)     | Depends on what it does|
| Can modify the response writer | No                      | Yes                   | Indirect               |
| Config-aware disable           | Built in                | Only as a module      | Built in               |
| Best at                        | Cross-component events  | Request behaviour     | New capability         |

The rough decision tree:

- **Does it need to write to the HTTP response stream?** Use a middleware.
  Auth that returns 401, CORS that adds headers, compression that wraps
  the writer — all middleware. Hooks only see `context.Context`; they
  cannot reach the `http.ResponseWriter`.
- **Does it react to something that happens outside HTTP?** Use a hook.
  Module init, data writes, response format resolution, websocket
  events, TCP messages — none of those are HTTP-shaped. Middleware does
  not see them.
- **Is it a self-contained capability with its own lifecycle, config, and
  state?** Make it a module. A module can hold a connection pool, expose
  resources, register hooks, register middleware, register routes. A
  hook handler that needs all of that wants to be the `Init` method of a
  module instead.

A common pattern is a module that registers both: a hook on
`kernel.after_init` to wire up state, and a middleware on the request
adapter for the per-request behaviour. The session module does this; the
HTTP security module does this. The module is the home of the wiring; the
hook and the middleware are the two places it shows up.

## Standard hook points

The framework defines hook points at the boundaries it understands. They
are documented in full in the [reference]({{< relref "/reference" >}}).
The headline ones:

- `kernel.before_init`, `kernel.after_init`, `kernel.before_start`,
  `kernel.after_start`, `kernel.before_stop`, `kernel.after_stop` — the
  lifecycle of the framework itself.
- `request.received`, `request.routed`, `request.before`,
  `request.after`, `request.error`, `request.complete` — the lifecycle
  of an HTTP request.
- `response.format.resolve` (resolution), `response.before_write`,
  `response.after_write` — the response pipeline.
- `data.before_read`, `data.after_read`, `data.before_write`,
  `data.after_write` — generic data operations.
- `websocket.connected`, `websocket.message`, `websocket.disconnected`,
  `websocket.error` — WebSocket connections.
- `tcp.connected`, `tcp.message`, `tcp.disconnected`, `tcp.error` —
  TCP connections.

Modules are free to define their own. The naming convention is
`component.event`. A module that wants to expose a customisable point
fires the hook itself and trusts that listeners will follow the
convention.

## Where to read next

- [Middleware]({{< relref "/explanation/middleware" >}}) — the other
  half of the hook-versus-middleware decision.
- [Format resolution]({{< relref "/explanation/format-resolution" >}}) —
  the canonical resolution-hook chain, in detail.
- [Modules]({{< relref "/explanation/modules" >}}) — when a hook handler
  has outgrown being a function and wants to become a module.
