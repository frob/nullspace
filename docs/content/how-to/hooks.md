---
title: Hook into the request lifecycle
weight: 11
---

Use this guide when you want to observe (or short-circuit) requests without
wrapping every handler in middleware.

## Solution

Register hooks during your module's `Init`. Lower priority numbers run first.

```go
func (m *Module) Init(k *kernel.Kernel) error {
    k.Hook("request.received", 10, m.onReceived)
    k.Hook("request.complete", 90, m.onComplete)
    return nil
}

func (m *Module) onReceived(ctx context.Context) error {
    nslog.FromContext(ctx).Info("request received")
    return nil
}

func (m *Module) onComplete(ctx context.Context) error {
    // record metrics, send traces, etc.
    return nil
}
```

A hook returning an error stops the chain at that point.

## Available hook points

| Hook                  | When it fires                              |
| --------------------- | ------------------------------------------ |
| `request.received`    | Adapter accepted the request               |
| `request.routed`      | Route matched (or fallback selected)       |
| `request.before`      | Before the handler runs                    |
| `request.after`       | After the handler runs                     |
| `request.complete`    | Response written, request finished         |
| `request.error`       | Pipeline wrote an error response           |
| `response.before_write` | Pipeline is about to write the response |
| `response.after_write`  | Pipeline finished writing the response  |

Kernel lifecycle:

| Hook                   | When it fires                       |
| ---------------------- | ----------------------------------- |
| `kernel.before_init`   | Before any module's `Init`          |
| `kernel.after_init`    | After every module's `Init`         |
| `kernel.before_start`  | Before any module's `Start`         |
| `kernel.after_start`   | After every module's `Start`        |
| `kernel.before_stop`   | Before any module's `Stop`          |
| `kernel.after_stop`    | After every module's `Stop`         |

Data, session, websocket, and TCP modules fire their own hooks (`data.before_read`,
`session.created`, `websocket.connected`, `tcp.message`, etc.).

## Variations

### Resolution hooks

Use `HookResolve` when you want the *first* registered handler to produce a
value. The chain stops at the first one that returns `ok = true`.

```go
k.HookResolve("session.login_url", 10, func(ctx context.Context) (any, bool, error) {
    return "/login", true, nil
})
```

The resolver chain that picks a response format is the canonical example —
each resolver runs in priority order, the first hit wins.

### Cancel a request from a hook

```go
k.Hook("request.before", 10, func(ctx context.Context) error {
    if blocked(ctx) {
        return fmt.Errorf("blocked")
    }
    return nil
})
```

The pipeline turns the returned error into an error response.

## See also

- [Add middleware to a route group]({{< relref "middleware" >}})
- [Configure logging output]({{< relref "logging" >}})
