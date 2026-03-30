# Logging Specification

Logging is a cross-cutting concern implemented as a port with a default `log/slog` adapter.

## Logger Port

```go
type Logger interface {
    Debug(msg string, args ...any)
    Info(msg string, args ...any)
    Warn(msg string, args ...any)
    Error(msg string, args ...any)
    With(args ...any) Logger   // returns a child logger with additional fields
}
```

The interface mirrors `slog` semantics — structured key-value pairs, leveled output.

## Default Adapter

The default adapter wraps `log/slog`:

- Text format for development, JSON format for production (configurable)
- Level configurable via config

```toml
[log]
level = "info"        # debug, info, warn, error
format = "text"       # text, json
```

## Two Logger Scopes

### Kernel Logger
- Created at kernel construction
- Used for lifecycle events: module init, start, stop, hook registration
- Available via `kernel.Logger()`

### Per-Request Logger
- Derived from kernel logger during request start
- Enriched with request-scoped fields:
  - Request ID (generated or from header)
  - Method, path
  - Remote address
- Attached to request context
- Available via `ctx.Logger`

```go
// Kernel lifecycle logging
k.Logger().Info("module started", "module", m.Name())

// Per-request logging
ctx.Logger.Info("route matched", "pattern", route.Pattern)
ctx.Logger.Debug("static fallback", "path", filePath)
```

## Default Logged Events

The framework logs these events automatically:

| Event | Level | Fields |
|-------|-------|--------|
| Request received | info | method, path, request_id |
| Route matched | debug | pattern, handler |
| Static file fallback | debug | path, exists |
| Route not found (404) | debug | path |
| Format resolved | debug | format, resolver |
| Response sent | info | status, duration_ms |
| Module init | info | module |
| Module start | info | module |
| Module stop | info | module |
| Hook registered | debug | hook_point, module, priority |
| Error | error | error, stack (debug only) |

## Swapping the Logger

Developers replace the logger by providing their own `Logger` implementation:

```go
k := kernel.New(
    kernel.WithLogger(myZerologAdapter),
)
```

All framework internals use the port interface, so the adapter is fully swappable.
