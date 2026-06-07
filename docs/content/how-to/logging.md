---
title: Configure logging output
weight: 13
---

Use this guide when you want to change the log level, switch to JSON output,
or attach extra fields to every line.

## Solution

Set the level and format in `nullspace.toml`:

```toml
[log]
level  = "info"     # debug, info, warn, error
format = "text"     # text, json
```

Override at runtime with environment variables:

```bash
NULLSPACE_LOG_LEVEL=debug
NULLSPACE_LOG_FORMAT=json
```

The default `slog` adapter writes to `stderr`.

## Use the right logger scope

| Scope          | How to obtain                              | Includes                       |
| -------------- | ------------------------------------------ | ------------------------------ |
| Kernel         | `k.Logger()`                               | Lifecycle events only          |
| Per-request    | `ctx.Logger()` inside a handler            | `request_id`, `method`, `path` |
| Hook context   | `nslog.FromContext(ctx)`                   | Same as per-request if available |

```go
func myHandler(ctx *request.Context) error {
    ctx.Logger().Info("processing", "user", "alice")
    return nil
}
```

## Automatic logging

The logging module installs hooks that log:

| Event              | Level | Fields                                          |
| ------------------ | ----- | ----------------------------------------------- |
| Request received   | INFO  | `request_id`, `method`, `path`                  |
| Request complete   | INFO  | `request_id`, `method`, `path`, `status`, `duration_ms` |
| Module init/start/stop | INFO | `module`                                    |
| Handler error      | ERROR | `error`                                         |
| Static file served | DEBUG | `path`                                          |

Turn the level down to `debug` to see route matching, hook registration, and
resource lookups too.

## Variations

### Attach permanent fields with `With`

```go
logger := ctx.Logger().With("tenant", tenantID)
logger.Info("loaded")
logger.Info("saved")
```

### Swap the adapter

`Logger` is a port — anything implementing it works. Plug in zerolog, zap,
or your own:

```go
type adapter struct{ /* ... */ }
func (a *adapter) Debug(msg string, args ...any) { /* ... */ }
func (a *adapter) Info(msg string, args ...any)  { /* ... */ }
// Warn, Error, With

k := kernel.New(kernel.WithLogger(&adapter{}))
```

All framework internals route through the `Logger` port.

### Disable automatic request logs

Disable the `nslog` automatic logging hooks if you'd rather emit your own.
Register a `request.complete` hook of your own and skip the module:

```toml
[modules]
nslog = false
```

(You still get a logger; the auto-log hooks are simply not registered.)

## See also

- [Hook into the request lifecycle]({{< relref "hooks" >}})
