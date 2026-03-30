# Kernel Specification

The kernel is the central component of the framework. It manages module lifecycle, provides the hook bus for aspect-oriented behavior, and holds configuration as a primitive.

## Responsibilities

- Register and manage modules
- Provide the hook bus for cross-cutting concerns
- Own configuration as a kernel primitive (not a module)
- Manage lifecycle: Init -> Start -> Stop
- Provide a service locator for cross-module resource sharing

## Module Interface

Every component in the framework implements the Module interface:

```go
type Module interface {
    Name() string
    Init(k *Kernel) error
    Start(ctx context.Context) error
    Stop(ctx context.Context) error
}
```

### Module Registration

Modules self-register with the kernel via `kernel.Use()`. During `Init`, modules register their hooks, config sections, and expose resources via the service locator.

```go
k := kernel.New()
k.Use(dataModule)
k.Use(requestModule)
k.Use(responseModule)
```

### Module Lifecycle

```
Register (kernel.Use)
    -> Config Load (kernel primitive, before any module init)
    -> Check Enabled (per-module, from config)
    -> Init (module wires itself into kernel: hooks, resources)
    -> Start (module begins serving)
    -> Stop (module shuts down gracefully)
```

### Module Enable/Disable

- Each module declares its own default enabled state
- Config overrides this per-module:

```toml
[modules]
format.query_param = true
format.content_negotiate = true
format.route_override = false
```

- Modules that are disabled are not initialized or started
- Runtime config changes affect module enablement only at request boundaries (see config spec)

## Service Locator

The kernel provides a typed service locator for cross-module resource sharing:

```go
kernel.Provide("data.users", userRepo)
repo := kernel.Get[Repository]("data.users")
```

- Keys are typed constants (not raw strings) to prevent typos
- Resources are registered during module Init phase
- The locator is read-only after Start

## Hook Bus

See [02-hooks.md](02-hooks.md) for full specification.

The kernel owns the hook bus and exposes it to modules during Init:

```go
func (m *MyModule) Init(k *Kernel) error {
    k.Hook("request.before", 10, m.myHandler)
    return nil
}
```
