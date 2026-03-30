# Hook Bus Specification

The hook bus is the kernel's mechanism for aspect-oriented programming. It allows modules to register behavior at named lifecycle points without modifying core code.

## Core Properties

- **Priority-ordered** — hooks execute in numeric priority order (lower = earlier)
- **Short-circuit capable** — a hook can signal resolution, stopping the chain
- **Config-aware** — hook execution respects module enabled/disabled state per request config snapshot
- **Typed signatures** — hooks have typed function signatures, not `interface{}`

## Hook Registration

Modules register hooks during their Init phase:

```go
k.Hook("response.format.resolve", 10, m.resolveFromRoute)
k.Hook("response.format.resolve", 20, m.resolveFromQueryParam)
k.Hook("response.format.resolve", 30, m.resolveFromAcceptHeader)
k.Hook("response.format.resolve", 40, m.resolveDefault)
```

### Parameters

| Parameter | Type | Description |
|-----------|------|-------------|
| name | string | Hook point identifier (e.g., `request.before`) |
| priority | int | Execution order. Lower runs first. |
| handler | typed func | The hook handler function |

## Hook Points

Framework-defined hook points (modules may define additional ones):

### Kernel Lifecycle
- `kernel.before_init` — before any module Init
- `kernel.after_init` — after all modules Init
- `kernel.before_start` — before any module Start
- `kernel.after_start` — after all modules Start
- `kernel.before_stop` — before any module Stop
- `kernel.after_stop` — after all modules Stop

### Request Lifecycle
- `request.received` — raw request received, before routing
- `request.routed` — route matched (or static file fallback determined)
- `request.before` — before handler execution
- `request.after` — after handler execution, before response
- `request.error` — error occurred during handling
- `request.complete` — response sent, cleanup

### Response Lifecycle
- `response.format.resolve` — determine response format (short-circuit chain)
- `response.before_write` — before serialization
- `response.after_write` — after serialization

### Data Lifecycle
- `data.before_read` — before data read (policy checks here)
- `data.after_read` — after data read
- `data.before_write` — before data write
- `data.after_write` — after data write

## Short-Circuit Behavior

A hook handler can return a resolution signal to stop the chain:

```go
func (m *FormatRouteOverride) resolveFromRoute(ctx Context) (string, bool) {
    format, ok := ctx.Route().Meta("format")
    if ok {
        return format, true  // resolved — stop chain
    }
    return "", false  // not resolved — continue chain
}
```

The hook bus executes handlers in priority order and stops at the first one that returns `true` for the resolved flag.

## Config-Aware Execution

Before executing a hook, the bus checks whether the owning module is enabled in the current request's config snapshot:

```
For each hook at this hook point (sorted by priority):
    1. Check if owning module is enabled in config snapshot
    2. If disabled, skip
    3. Execute handler
    4. If short-circuited, stop
```

This means two concurrent requests can execute different hook chains if the runtime config changed between their starts.
