---
title: Configuration
weight: 5
---

Configuration in Nullspace is a kernel primitive, not a module. That
sentence carries most of the design decisions on this page. It means
configuration is loaded before any module initialises, mutated through a
narrow API, snapshotted at the start of every request, and read from that
immutable snapshot for the duration of the request. This page explains
why each of those properties matters.

## Sources, in order

There are three sources, in increasing precedence:

```
1. Module defaults       (compiled into the binary)         lowest
2. TOML config file      (nullspace.toml by default)
3. Environment vars      (NULLSPACE_SECTION_KEY pattern)    highest
```

Nothing else. There is no flag parser, no remote config, no profile
system. Anything that would have been a flag is either a TOML key or an
environment variable. Anything that would have been a profile is a
different TOML file selected via `WithConfigFile`.

This is opinionated. Other frameworks support YAML, JSON, and arbitrary
provider chains; Nullspace deliberately does not. The reason is that
every additional source is another place to look when a value is wrong,
and the kernel wants the answer to "where did this value come from" to
have at most three places to check. TOML is the human-edited primary;
env vars are for secrets and deployment overrides; defaults are the
fallback. That is the whole model.

## TOML layout

```toml
[kernel]
name = "myapp"

[log]
level = "info"
format = "text"

[request]
addr = ":8080"

[response]
default_format = "json"
template_dir = "./templates"

[data.static]
dir = "./public"

[data.file]
dir = "./content"
format = "markdown"

[data.sql]
driver = "sqlite"
dsn = "./data.db"

[modules]
"data.sql" = true
"format.query_param" = false
```

Sections map one-to-one onto modules. The `[data.file]` section
configures the file module; the `[log]` section configures logging.
There is no inheritance, no `extends`, no `include`. Each module
declares exactly one section via `ModuleConfig.Key`, and the kernel
delivers that section back to the module during `Init`.

The `[modules]` section is special. It does not configure a module; it
controls which modules are enabled. Anything listed there overrides the
module's `DefaultEnabled` value.

## Environment overrides

```
NULLSPACE_<SECTION>_<KEY>=value
```

The prefix is configurable at construction time via `WithEnvPrefix`. The
mapping is mechanical: underscores in the variable name become dots in
the config path, lowercase.

```
NULLSPACE_LOG_LEVEL=debug              -> log.level = "debug"
NULLSPACE_REQUEST_ADDR=:9090           -> request.addr = ":9090"
NULLSPACE_DATA_SQL_DSN=postgres://...  -> data.sql.dsn = "postgres://..."
```

This makes containerised deployment straightforward: bake the TOML into
the image with reasonable defaults, override the dynamic values
(database DSN, address bindings, log level) with environment variables
at runtime. The TOML file is the developer-facing surface; the env vars
are the operator-facing surface. They are intentionally orthogonal.

## Module-declared config

A module declares its config section by implementing `Configurable`:

```go
func (m *Module) Config() kernel.ModuleConfig {
    return kernel.ModuleConfig{
        Key:            "data.file",
        Default:        Config{Dir: "./content", Format: "markdown"},
        DefaultEnabled: true,
    }
}
```

During `Init`, the module reads the populated config:

```go
func (m *Module) Init(k *kernel.Kernel) error {
    var cfg Config
    if err := k.Config().Decode("data.file", &cfg); err != nil {
        cfg = Config{Dir: "./content"}
    }
    m.dir = cfg.Dir
    return nil
}
```

`Decode` is a JSON round-trip under the hood — config struct fields use
`json` tags that match the TOML keys. This is a deliberate
simplification: it means no reflection-based TOML decoding inside the
kernel, and the config structs work the same way they would in any other
Go program.

The flow during startup is:

1. Kernel collects `ModuleConfig` declarations from every module.
2. Kernel loads the TOML file into a generic map.
3. Kernel applies environment variable overrides to that map.
4. Kernel reads the `[modules]` section to set enabled/disabled.
5. Kernel merges each module's TOML section into its `Default` value and
   stores the result.
6. Kernel calls `Init` on each enabled module; the module asks for its
   section back.

Step 5 is where defaults work: a missing TOML key keeps the default
value from the `Default` struct.

## Project config versus module config

There is a distinction worth naming explicitly. The TOML file mixes two
things:

- **Project configuration** — sections owned by the application: routes
  in `[[routing.routes]]`, collections in `[[routing.collections]]`,
  domain-specific values in your own sections.
- **Module configuration** — sections owned by framework or library
  modules: `[log]`, `[request]`, `[data.file]`, `[data.sql]`.

The kernel does not distinguish between them. A section is a section; a
module either claims it via `ModuleConfig.Key` or it does not.

The pragmatic split is that *project* configuration tends to live in
`nullspace.toml` at the root of the project, and *module* configuration
tends to be a mix of the same file plus per-module `routes.toml` files
embedded via `go:embed`. The routing module loads the embedded TOML at
`Init` time:

```go
//go:embed routes.toml
var routesData []byte

func (m *Module) Init(k *kernel.Kernel) error {
    routingMod, _ := kernel.GetResource[*routing.Module](k, "routing")
    routingMod.LoadRoutes(routesData)
    return nil
}
```

This is the route by which a module can ship its own declarative routes
without polluting the top-level project file.

## Live config and snapshots

Configuration is mutable at runtime:

```go
k.Config().Set("custom.key", "value")
k.Config().SetModuleEnabled("format.query_param", false)
```

This is the *live* config. It is guarded by a mutex; multiple goroutines
can read and write it safely. But framework code never reads from it
during a request. Instead, when a request arrives, the request adapter
takes a snapshot:

```go
snap := k.Config().Snapshot()
ctx = kernel.ContextWithSnapshot(ctx, snap)
```

The snapshot is a deep copy of the modules map and the values map. It is
not safe-by-mutex; it is safe by being immutable. Two concurrent
requests can hold two different snapshots, and neither cares that the
other exists.

The hook bus reads the snapshot off the context before it dispatches.
Format resolution reads the snapshot. Anywhere the framework wants to
know "is this module enabled?" or "what's the value of `request.addr`?"
during a request, it asks the snapshot, not the live config.

## Why immutability mid-request matters

The reason for this design is subtle and worth saying directly.

Imagine a request handler that calls into the file data module, which
fires `data.before_read`. A policy module is listening on that hook.
Halfway through the request, an operator disables the policy module via
the live config. What should happen?

Option A: the disable takes effect immediately. The hook stops firing
mid-request. The request that was being checked now reads without
policy enforcement. This is a security failure — a request that started
under one policy regime ended under another. Worse, the failure mode is
silent.

Option B: the disable takes effect only at the next request boundary.
The request in flight continues to use the policy that was in place when
it started. The next request starts with a new snapshot that has the
policy disabled. This is what Nullspace does.

The same logic applies to format resolvers, route override behaviour,
and every other piece of configuration that affects how a request is
handled. The guarantee is: *a request always completes under the
configuration it started with*. That is the contract. Snapshots are the
mechanism.

The cost is that the snapshot must be taken at request start, which
means a small allocation and a small copy per request. The benefit is
that no part of the request path needs to take a lock to read
configuration. The snapshot is referentially transparent; you can read
from it from any goroutine for as long as the request lives.

## Two concurrent requests, different configurations

The implication is that two concurrent requests can be running under
different configurations. If a config change happens at time T, every
request that started before T sees the old config until it finishes;
every request that starts after T sees the new config. There is no
synchronisation point between them. This is sometimes surprising — but
it is what makes runtime config changes safe in the first place.

The runtime mutation API is intentionally small. There is
`SetModuleEnabled` for module enable state and `Set` for arbitrary
values. There is no transaction, no batching, no signal. If you change
five values, requests in flight see zero of them; the next request to
start sees all five. Atomicity at the request boundary is the only
guarantee.

## What was rejected

Some alternatives that were considered and not chosen:

- **Configuration as a module.** It would be consistent. But the kernel
  needs configuration before any module can run (to decide which modules
  are even enabled), so config-as-module would have to bootstrap itself
  in a special way. Making it a kernel primitive is simpler.
- **Hot reload from the filesystem.** Possible, but it adds a watcher
  goroutine, file-change semantics, and questions about what to do when
  a partial change is read. The same effect is achievable by calling
  `k.Config().Set` from your own code, with full control over when and
  what.
- **Profiles or environments.** "dev", "staging", "prod" profiles in
  one file. This is what env vars are for: one TOML for the application,
  env vars for the deployment.
- **Schema validation at startup.** Each module gets its config back as
  a typed struct, so the type system already validates the structure.
  Cross-field invariants are the module's responsibility, checked
  during `Init`.

## Where to read next

- [The hook bus]({{< relref "/explanation/hooks" >}}) — how snapshots
  affect hook dispatch.
- [Modules]({{< relref "/explanation/modules" >}}) — the
  `Configurable` interface and where `Init` reads config.
- [Configuration reference]({{< relref "/reference" >}}) — every TOML
  key and environment variable.
