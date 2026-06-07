---
title: Modules and the kernel
weight: 2
---

The module is the only unit of composition in Nullspace. The framework has
no notion of "plugin" or "extension" or "service" as separate categories;
the request adapter and a one-line audit handler implement the same
interface and go through the same lifecycle. This page explains what that
lifecycle is, why registration order matters more than it looks, what the
service locator is doing, and — most importantly — why this is not
dependency injection in the Java sense.

## The interface, all of it

```go
type Module interface {
    Name() string
    Init(k *Kernel) error
    Start(ctx context.Context) error
    Stop(ctx context.Context) error
}
```

Four methods. `Name` is the identifier used for enablement, hook
attribution, and logs. `Init` wires the module into the kernel. `Start`
opens whatever needs opening — sockets, goroutines, connection pools.
`Stop` closes them.

Modules that need configuration also implement `Configurable`:

```go
type Configurable interface {
    Config() ModuleConfig
}
```

The kernel calls `Config` before any `Init` so it can collect every TOML
section that exists, apply environment overrides, decide which modules are
enabled, and only then begin initialising. That ordering is deliberate:
modules cannot inspect configuration during construction because they have
not been told anything yet. They inspect it during `Init`.

## The lifecycle in order

```
   kernel.New(...)
        v
   kernel.Use(module)        register, in caller-defined order
        v
   kernel.Init(ctx)
        |
        |--- collect Configurable.Config() declarations
        |--- load TOML file, apply NULLSPACE_* env overrides
        |--- read [modules] section, set enabled/disabled
        |--- fire kernel.before_init
        |--- for each enabled module:
        |        log "initializing"
        |        module.Init(kernel)
        |--- fire kernel.after_init
        v
   kernel.Start(ctx)
        |
        |--- fire kernel.before_start
        |--- for each enabled module: module.Start(ctx)
        |--- fire kernel.after_start
        v
   ... serving ...
        v
   kernel.Stop(ctx)
        |
        |--- fire kernel.before_stop
        |--- for each enabled module in REVERSE order: module.Stop(ctx)
        |--- fire kernel.after_stop
```

`Init` is where modules register hooks, expose resources via the service
locator, and read their configuration. It is also the only place where the
kernel will attribute hook registrations to the calling module — the kernel
tracks which module is currently initialising and tags any hook registered
during that window. After `Init`, hooks register as `"kernel"` and are
always enabled.

`Start` runs after every enabled module has initialised. This is when the
TCP listener starts accepting connections, when the HTTP server binds, when
goroutines spin up. By the time `Start` runs, everything that any module
might look up in the service locator is already there.

`Stop` runs in reverse registration order — last in, first out. The TCP
listener that started last shuts down first so that the dependencies it was
using (logger, hook bus, data modules) are still alive while it drains.

## Why registration order matters

The order in which `kernel.Use` is called determines the order of `Init`,
the order of `Start`, and the reverse of `Stop`. That is a lot of
behaviour pinned to call order, and it is the easiest thing to get wrong.

The reason it has to work this way is the service locator. If module B
calls `kernel.GetResource[*A](k, "a")` during its `Init`, module A's
`Init` had to have already run. The kernel does not topologically sort
modules — it executes them in the order you registered them. There is no
hidden dependency graph.

Concretely, this is the canonical order for an HTTP application:

```
1. Logging                    (so everything else can log)
2. Request adapter            (provides "request.adapter", "router")
3. Response pipeline          (provides "response.pipeline")
4. Format resolvers           (register HookResolve handlers)
5. Routing module             (provides "routing", reads route TOML)
6. Data modules               (provide "data.file", "data.sql", etc.)
7. WebSocket / transports     (look up router, data modules)
8. Application modules        (your code)
```

Deviating from this is fine as long as later modules do not depend on
earlier ones. But "obviously the application module goes last" is the rule
worth remembering. If you ever find yourself debugging a `resource not
found` error during `Init`, the first thing to check is the registration
order in `main`.

## Enabled and disabled

Each module declares a `DefaultEnabled` value in its `ModuleConfig`. Core
modules default to `true`; opt-in modules (SQL, sessions, WebSocket, TCP)
default to `false`. The `[modules]` section of `nullspace.toml` overrides
the default:

```toml
[modules]
"data.sql" = true            # opt in
"format.query_param" = false # opt out
```

Disabled modules are not initialised, not started, not stopped. None of
their `Init` code runs, so they cannot register hooks, expose resources,
or read config. From the rest of the system's perspective they may as well
not exist.

But there is a subtler case: a disabled module's hooks. If a module
registers hooks and is then disabled, this is a non-event because `Init`
never ran. What about a module that is enabled at startup, registers
hooks, and is then disabled at runtime via `Config().SetModuleEnabled`?
The kernel keeps the hooks registered but checks the per-request config
snapshot before firing each one. A disabled module's hooks are silently
skipped for the duration of any request whose snapshot has them disabled.
Concurrent requests that started before the change still see them enabled.
This per-request resolution is what makes the [hook bus]({{< relref
"/explanation/hooks" >}}) safe to use as a coordination point while
configuration is live.

## The service locator

The kernel has a typed map keyed by string:

```go
k.Provide("data.file", fileMod)

fileMod, err := kernel.GetResource[*file.Module](k, "data.file")
```

Modules use it to publish references that other modules might need: the
router, the response pipeline, the WebSocket manager, the SQL database
handle. It is a key-value store with a generic helper for type assertion.

Standard keys live in the [reference]({{< relref "/reference" >}}); a
short list of the most useful ones:

| Key                    | Type                |
|------------------------|---------------------|
| `"router"`             | `*request.Router`   |
| `"request.adapter"`    | `*request.Adapter`  |
| `"response.pipeline"`  | `*response.Pipeline`|
| `"data.file"`          | `*file.Module`      |
| `"data.sql"`           | `*sql.Module`       |
| `"db"`                 | `*sql.DB`           |
| `"websocket"`          | `*websocket.Module` |
| `"websocket.manager"`  | `*websocket.Manager`|
| `"transport.tcp"`      | `*tcp.Adapter`      |
| `"transport.ipc"`      | `*ipc.Adapter`      |

The locator is intentionally a flat map, not a hierarchical container. We
will come back to this in a moment.

## Why this is not Spring

In the Java ecosystem, dependency injection means a container that:

1. Reads metadata (annotations, XML, or both).
2. Builds a graph of beans and their dependencies.
3. Instantiates beans in topological order, wiring them together by type
   or qualifier.
4. Manages scopes (singleton, prototype, request, session) and proxies.

Nullspace does none of that. The service locator is a `map[string]any`
with a type-asserted getter. There is no graph, no resolution, no scope,
no proxying. The kernel does not know that the WebSocket module depends
on the router; it just knows that someone called `Provide("router", ...)`
at some point and that someone else later called `GetResource[T]`.

The tradeoff is honest:

**What DI gives you** — declarative wiring, automatic ordering, lifecycle
management for arbitrarily complex graphs, testability through automatic
mock injection.

**What the service locator gives you** — explicit, debuggable wiring. When
something is not found, the error is a string lookup failure with a
filename and line number, not a chain of reflective constructor calls.
You can grep for the key.

**What it costs** — you have to register modules in the right order. The
compiler will not catch a stale key. The "graph" lives in your `main`
function as a sequence of `k.Use` calls.

The reason Nullspace stays on this side of the line is that the kernel
itself is small and the module count in a typical application is around a
dozen. At that scale a string map with a typed getter is enough, and the
explicitness is worth more than the automation. If your application is
large enough that you would benefit from declarative wiring, Nullspace is
probably not the right framework — but you can still use one inside a
module if you want it.

## What this implies for how you write modules

A module that follows the grain of the framework looks like this:

```go
type Module struct {
    db *sql.DB
}

func (m *Module) Name() string { return "myapp.things" }

func (m *Module) Config() kernel.ModuleConfig {
    return kernel.ModuleConfig{
        Key:            "things",
        Default:        Config{Limit: 100},
        DefaultEnabled: true,
    }
}

func (m *Module) Init(k *kernel.Kernel) error {
    var cfg Config
    _ = k.Config().Decode("things", &cfg)

    db, err := kernel.GetResource[*sql.DB](k, "db")
    if err != nil {
        return err
    }
    m.db = db

    router, _ := kernel.GetResource[*request.Router](k, "router")
    router.Get("/things", m.list)

    k.Hook("data.after_write", 50, m.invalidateCache)
    return nil
}

func (m *Module) Start(ctx context.Context) error { return nil }
func (m *Module) Stop(ctx context.Context)  error { return nil }
```

`Init` is the busy method. `Start` and `Stop` are usually empty for
modules that do not own a long-lived resource. The kernel does not care.

## Where to read next

- [The hook bus]({{< relref "/explanation/hooks" >}}) — what hooks let
  modules say to each other without taking a hard dependency.
- [Configuration]({{< relref "/explanation/configuration" >}}) — how the
  `Configurable` interface participates in startup.
- [Multi-transport]({{< relref "/explanation/transports" >}}) — modules
  that listen on more than HTTP.
