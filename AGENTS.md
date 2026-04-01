# Agents

## Project

Go HTTP framework. Module path: `github.com/frob/nullspace`

## Build

```
task test          # run tests
task build         # compile
task check         # fmt + vet + test
```

## Architecture

Hexagonal. Kernel at center, everything else is a module.

- **Kernel** (`kernel/`) — module registry, hook bus, config, service locator
- **Routing** (`core/routing/`) — TOML route config, handler registry, built-in handlers
- **Request** (`core/request/`) — HTTP adapter, router, middleware, context
- **Response** (`core/response/`) — format resolution, formatters, pipeline
- **Data** (`module/data/`) — static files, file entities, SQL
- **Logging** (`core/nslog/`) — slog adapter, per-request loggers
- **Session** (`module/session/`) — session management, memory and SQL stores
- **HTTP Security** (`module/httpsecurity/`) — security headers, CSRF, HTTPS redirect
- **WebSocket** (`module/websocket/`) — WS upgrade, connection manager, rooms, broadcast

## Key patterns

- Modules implement `kernel.Module` (Name/Init/Start/Stop)
- Hooks: `k.Hook("point", priority, handler)` — lower priority runs first
- Resolution hooks: `k.HookResolve("point", priority, handler)` — first to resolve wins
- Config: TOML + env overrides, per-request immutable snapshots
- Routes: TOML in `nullspace.toml` or per-module `routes.toml` (embedded via `go:embed`)
- Handlers registered by name: `reg.HandleFunc("name", handler)`
- Service locator: `k.Provide("key", value)` / `kernel.GetResource[T](k, "key")`

## Module registration order matters

Logging → Request adapter → Response pipeline → Format resolvers → Data modules → Routing → WebSocket → Application modules

## Tests

Run `task test`. All data tests use temp dirs. No external services needed.
