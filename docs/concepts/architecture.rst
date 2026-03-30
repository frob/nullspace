Architecture
============

Nullspace uses hexagonal architecture (ports and adapters) with aspect-oriented
programming to create a modular, extensible framework.

Overview
--------

::

                  +-------------------------+
                  |         Kernel          |
                  | (registry, hook bus,    |
                  |  config, lifecycle,     |
                  |  service locator)       |
                  +-------------------------+
                 /      |       |        \
           Request   Response   Data    Logging
           (HTTP     (format    (SQL,   (slog
            adapter,  resolve,  file,    adapter,
            router,   JSON,     static)  per-request)
            mw)       HTML)

The **kernel** sits at the center. It manages module lifecycle, provides the
hook bus for cross-cutting concerns, owns configuration as a primitive, and
offers a service locator for resource sharing between modules.

Everything else is a **module** that plugs into the kernel through well-defined
**ports** (interfaces) and **adapters** (implementations).

Hexagonal Architecture
----------------------

In hexagonal architecture, the application core defines **ports** -- interfaces
that describe what the system needs -- and **adapters** implement those ports
for specific technologies.

In Nullspace:

- **Ports** are Go interfaces defined in ``kernel/port.go``: ``Module``,
  ``Configurable``, ``DataProvider``, ``Logger``
- **Adapters** are concrete implementations: the HTTP adapter, slog logger,
  SQLite driver, file parser, etc.

This separation means you can swap any adapter without changing the core. For
example, replace the slog logger with zerolog by implementing the ``Logger``
interface.

Aspect-Oriented Programming
----------------------------

Cross-cutting concerns (logging, auth, metrics, format resolution) are handled
through two complementary mechanisms:

**Functional middleware** for the request/response pipeline::

    func timing(next HandlerFunc) HandlerFunc {
        return func(ctx *Context) error {
            start := time.Now()
            err := next(ctx)
            log(time.Since(start))
            return err
        }
    }

**Hook bus** for concerns that span multiple components::

    k.Hook("data.before_read", 10, checkAccessPolicy)
    k.Hook("request.complete", 90, recordMetrics)

Middleware is best for request-scoped behavior (auth, CORS, compression). The
hook bus is best for cross-component behavior (policy enforcement, lifecycle
events, format resolution).

Request Lifecycle
-----------------

A request flows through the framework in this order:

1. **Config snapshot** -- The live config is frozen for this request
2. **Logger enrichment** -- A per-request logger is created with request ID,
   method, and path
3. ``request.received`` hooks fire
4. **Route matching** -- Dynamic routes are checked first
5. **Fallback** -- If no route matches, fallback handlers run (static files)
6. **404** -- If nothing handles it, return 404
7. ``request.routed`` hooks fire
8. **Middleware chain** -- Global middleware wraps route middleware wraps handler
9. ``request.before`` hooks fire
10. **Handler executes**
11. ``request.after`` hooks fire
12. ``request.complete`` hooks fire

Config Snapshot Isolation
-------------------------

Configuration is mutable at runtime, but each request sees an immutable
snapshot taken at step 1. Two concurrent requests may run under different
configurations if the live config changed between their starts. This guarantees
consistent behavior within a single request.
