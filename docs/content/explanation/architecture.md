---
title: Architecture
weight: 1
---

Nullspace is built around a single design decision: there is no application
core. There is a kernel — a tiny registry that knows how to start, stop, and
coordinate modules — and everything else is a module that plugs into it. HTTP
serving is a module. Logging is a module. JSON formatting is a module.
Sometimes a module is two lines long; sometimes it is a transport adapter
with a connection manager. The kernel does not distinguish.

This page explains the shape that choice produces, what the words "port" and
"adapter" mean in this codebase, and how the result compares to a more
typical layered web framework.

## The hexagonal shape

The conventional Go web framework — Gin, Echo, Fiber, Chi — is built bottom
up from `net/http`. There is a router, a middleware chain, a context object,
and a handler signature. Everything above is application code; everything
below is `net/http`. The framework occupies a thin slice in the middle and
gets out of your way.

Nullspace puts the kernel in the centre and pushes `net/http` to the edge,
alongside TCP listeners, Unix sockets, the SQL driver, the template engine,
the file system, and the logger. None of those things sit in a privileged
position. They are all adapters on the outside of a hexagon, and the kernel
sits inside.

```
                          +-------------------+
                          |      Kernel       |
                          |                   |
              .---- ports +  - module reg.   + ports ----.
              |           |  - hook bus      |           |
              |           |  - config        |           |
              |           |  - svc locator   |           |
              |           +-------------------+           |
              |                                           |
   +----------+----------+                   +-----------+----------+
   |   request adapter   |                   |  response pipeline   |
   |   (net/http bridge) |                   |  (format resolvers,  |
   +----------+----------+                   |   formatters, IO)    |
              |                              +-----------+----------+
              |                                          |
   +----------+----------+                   +-----------+----------+
   |    TCP / IPC        |                   |   Data providers     |
   |    transports       |                   |   (file, sql, static)|
   +----------+----------+                   +-----------+----------+
              |                                          |
   +----------+----------+                   +-----------+----------+
   |    WebSocket        |                   |   Logging adapter    |
   |    module           |                   |   (slog by default)  |
   +---------------------+                   +----------------------+
```

The arrows go inward. `net/http` calls into the kernel through the request
adapter; the kernel never calls out to `net/http` directly. The SQL driver
is reached through a `DataProvider` interface; the kernel never imports
`database/sql`. Each adapter implements an interface the kernel can name
without knowing what is on the other end of it.

## What "port" means here

In Cockburn's original hexagonal architecture, *ports* are the abstract
interfaces the application core exposes to the outside world, and *adapters*
are the concrete implementations that bridge those ports to a real
technology. Nullspace uses the words the same way, but the application core
is unusually small.

The ports live in `kernel/port.go` and there are not many of them:

- `Module` — the lifecycle contract every component must implement.
- `Configurable` — the optional contract for modules that declare a TOML
  section.
- `DataProvider` — `Module` plus a `Healthy` check, for data backends.
- `Logger` — leveled, structured, `slog`-shaped logging.

That is the entire surface area the kernel insists on. Everything else —
HTTP routing, response formatting, the hook signatures, the SQL contract,
the WebSocket lifecycle — is defined by the modules themselves and shared
through the [service locator]({{< relref "/explanation/modules" >}}). The
ports the kernel itself owns are deliberately few, because every port the
kernel owns is a place a future module cannot reshape.

## What it gives up

Hexagonal is not free. The cost is indirection. In a layered framework the
HTTP handler is the centre of gravity: you read the route table and follow
the call chain directly into the handler. In Nullspace the route table is in
TOML, the handler is looked up by name from a registry, the response is
serialised by a formatter chosen by a resolution chain, and several hooks
fire around all of it. There are more moving parts to keep in your head, and
the call graph is not as straight.

The benefit is that each of those moving parts is replaceable without
touching the others. You can disable the route-override format resolver and
keep the rest. You can swap the slog logger for zerolog. You can put a
policy module on `data.before_read` without modifying any data module. You
can put the same data layer behind HTTP, TCP, and IPC without writing the
handlers twice. None of that is unique to hexagonal architecture, but it is
much easier when the structure is built for it from the start than when it
is retrofitted onto a layered design.

## How a request actually flows

The kernel does not handle HTTP requests directly. It only knows about
modules and hooks. The chain looks like this:

```
   net/http   ->   request adapter   ->   middleware chain   ->   handler
                          |                                          |
                          v                                          v
                   request.received                         data.before_read
                   request.routed                           data.after_read
                   request.before                       response.format.resolve
                   request.after                        response.before_write
                   request.complete                     response.after_write
                          |                                          |
                          +------- hook bus (priority-ordered) ------+
```

The hook column on the right is what makes the hexagonal claim more than
decoration. Any module can register at any of those points. The data module
fires `data.before_read` without knowing whether a policy module, an audit
module, or nothing at all is listening. The response pipeline fires
`response.format.resolve` and takes whatever the first resolved format hook
returns. Nothing along that path needs to be modified to add a new
behaviour; you only add a module that registers in the right place.

The lifecycle hooks (`kernel.before_init`, `kernel.after_start`, and so on)
let modules participate in startup and shutdown the same way. The SQL module
runs pending migrations on `kernel.after_init` not because the kernel knows
about migrations, but because it knows about hooks.

## Compared to a layered framework

A layered framework — handler, service, repository, with middleware around
the handler — is organised by *what* the code does. A request goes down the
stack to the database and back up.

Nullspace is organised by *who owns the code*. The kernel owns lifecycle and
coordination. Each module owns one bounded concern. Communication is by
named contract (hook point, service locator key, configuration section)
rather than by import path. That changes the shape of the dependency graph:
modules depend on the kernel, but not on each other in any structural sense.
The data bridge module pulls the file module out of the service locator, but
the file module does not know the bridge exists.

The practical difference shows up when you want to change something. In a
layered framework you usually edit the handler or one of the layers it calls
through. In Nullspace you usually add a module — sometimes very small — and
register it at the right hook point. The existing code does not have to know
you did it. That is the property the hexagonal shape is buying.

## Where to read next

- [Modules and the kernel]({{< relref "/explanation/modules" >}}) — the
  lifecycle in detail, registration order, and why the service locator
  is not dependency injection in the Java sense.
- [The hook bus]({{< relref "/explanation/hooks" >}}) — fire-and-forget
  versus resolution hooks, priority ordering, and config-aware dispatch.
- [Multi-transport]({{< relref "/explanation/transports" >}}) — how the
  same kernel serves HTTP, TCP, IPC, and WebSocket.
