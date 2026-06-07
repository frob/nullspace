---
title: Comparisons
weight: 8
---

Nullspace is not the only Go web framework, and for many projects it is
not the right one. This page compares Nullspace honestly with the
mainstream alternatives — Chi, Gin, Echo, Fiber, and Buffalo. The goal
is to help you decide quickly whether to read further, and to set
expectations about what Nullspace is prescriptive about that the others
are not.

## At a glance

| Framework | Style                   | Routing      | Transports         | Opinions on data | Configuration |
|-----------|-------------------------|--------------|--------------------|------------------|---------------|
| Chi       | `net/http` router       | Code         | HTTP               | None             | Yours         |
| Gin       | Performance-first       | Code         | HTTP               | None             | Yours         |
| Echo      | Batteries-included HTTP | Code         | HTTP, WS           | None             | Yours         |
| Fiber     | fasthttp-based          | Code         | HTTP               | None             | Yours         |
| Buffalo   | Rails-shaped            | Code         | HTTP               | ORM, migrations  | Conventions   |
| Nullspace | Hexagonal, multi-transport | TOML + code | HTTP, TCP, IPC, WS | File + SQL, hookable | TOML + env  |

Two of these stand apart in different directions. Buffalo and Nullspace
both make opinionated choices about content and project shape; the
others sit closer to "just an HTTP framework, plug in what you need."

## Chi

Chi is a thin, idiomatic `net/http` router with stdlib-compatible
middleware. It is the closest thing the Go ecosystem has to a default.

**What Chi does well.** Stays out of the way. Composes with `net/http`
without ceremony. Middleware is `func(http.Handler) http.Handler`,
which is the standard signature, so anything that works with `net/http`
works with Chi. No magic. The code reads top to bottom.

**Where Nullspace is different.** Chi is a router; Nullspace is a
framework. Chi gives you the routing layer and leaves everything else
(data, configuration, response shaping, transports beyond HTTP) to you
or to other libraries. Nullspace ships those layers as modules and
expects you to use the kernel to coordinate them.

If you already have a working application architecture and just want a
clean router, Chi is the right answer. If you are starting from nothing
and want the architecture decided for you, Nullspace is making more
choices.

## Gin

Gin is performance-oriented. It has its own context type, a fast radix
router, and a large ecosystem of middleware.

**What Gin does well.** Speed. The benchmarks consistently put it near
the top. The middleware ecosystem is mature. The context API is
ergonomic.

**Where Nullspace is different.** Gin's context object is the centre
of gravity — handlers receive `*gin.Context` and write to it directly:
`c.JSON(200, data)`, `c.String(200, "...")`, `c.HTML(200, "tpl",
data)`. The format decision is the handler's. Nullspace handlers
return a `*Response` and the [format resolution chain]({{< relref
"/explanation/format-resolution" >}}) decides what to serialise as.

Gin is also single-transport. Nullspace's multi-transport story
(TCP, IPC, WebSocket sharing one kernel) does not have a Gin
equivalent — you would assemble it yourself from `net.Listen` and
glue code.

Choose Gin when speed is a primary criterion and you are happy to wire
the rest of the stack yourself. Choose Nullspace when the cross-cutting
structure (hooks, modules, multi-transport) is worth more than raw
throughput.

## Echo

Echo is closer in spirit to Gin — a context-centric HTTP framework with
built-in WebSocket support, middleware, and decent ergonomics. It is
slightly more opinionated than Chi, slightly less than Buffalo.

**What Echo does well.** Built-in goodies (JWT, CORS, rate limit, gzip)
that work out of the box. WebSocket is integrated, not an afterthought.
The HTTP/2 server push API is exposed.

**Where Nullspace is different.** Same as Gin: the handler decides the
format. Echo also does not have a hook bus — middleware is the only
extension point. There is no formal lifecycle for an "Echo module" as
distinct from "code in `main`."

If your project is HTTP-only and you want batteries included, Echo is a
strong choice. Nullspace adds a hook bus and module lifecycle to a
similar baseline; whether that is worth the extra structure depends on
how much cross-cutting code you expect to write.

## Fiber

Fiber is built on `fasthttp` instead of `net/http`. It is the fastest of
the mainstream choices in many benchmarks, with an Express-like API.

**What Fiber does well.** Throughput and memory usage. Familiar API for
people coming from Node. Active development.

**Where Nullspace is different.** Fiber sits outside the standard
library because `fasthttp` is not `net/http`-compatible. Standard
middleware does not work; you use Fiber's ecosystem instead.
Nullspace stays on `net/http` and benefits from everything that
ecosystem provides — sessions, profiling, tracing, anything that
wraps `http.Handler`.

The format-resolution split, hook bus, and multi-transport story are
also Nullspace-specific and have no direct Fiber equivalent.

Choose Fiber when throughput dominates and you are happy off the
`net/http` path. Choose Nullspace when interoperability with the
standard library matters more.

## Buffalo

Buffalo is the most opinionated of the mainstream Go frameworks. It is
Rails-shaped: it generates project structure, includes an ORM (Pop),
ships database migrations, has asset pipelines, and prescribes how an
application is organised.

**What Buffalo does well.** Convention. A `buffalo new` project has
everything in known locations. Generators exist for resources,
migrations, and tests. The ORM, the asset pipeline, and the web
framework agree on how things fit together.

**Where Nullspace is different.** Buffalo and Nullspace are both
opinionated, but they make different choices. Buffalo follows Rails:
ORM-first, full asset pipeline, deep code generation. Nullspace
follows a different lineage: hexagonal architecture, data-as-files as
the default backend, content-aware routing through collections,
multi-transport as a first-class feature.

The dividing question: do you want a Go web framework that resembles
Rails, or a Go web framework that resembles a content/CMS backend with
HTTP, TCP, and TUI surfaces? They are different products. Buffalo
optimises for a CRUD web application backed by Postgres; Nullspace
optimises for a content-driven service with multiple client types.

## What Nullspace is prescriptive about

The other frameworks on this list are mostly libraries you compose. They
have opinions about how their own pieces fit together, but they do not
have opinions about your project's architecture. Nullspace does:

1. **You will have modules.** Every component implements the `Module`
   interface and goes through the same lifecycle. There is no
   alternative shape.
2. **You will use the hook bus.** Cross-cutting concerns go on the
   hook bus. Putting them anywhere else fights the framework.
3. **You will configure through TOML.** Not YAML, not flags, not
   bespoke loaders. TOML plus environment variables, full stop.
4. **Routes can be declared in TOML.** They do not have to be — you
   can register routes in Go code alongside TOML routes — but the TOML
   route file is the recommended entry point and is what the
   `nullspace init` scaffolding produces.
5. **Format is decided by the framework.** Handlers return data, not
   bytes. The format resolution chain picks the wire format.
6. **Data is files by default.** The file data module ships with the
   framework and is enabled by default. SQL is opt-in. Content as
   files is the assumed primary model.

If any of these are bad fits for your project, the friction will be
real. Use Chi, Gin, Echo, or Fiber, and assemble what you need from
libraries. That is also a perfectly good way to build a Go web
application.

## When Nullspace is the right answer

A short list of project shapes where Nullspace's choices pay off:

- **Content sites with API and HTML clients.** The format resolution
  chain serves both from one handler. The file data module makes
  content git-trackable.
- **Applications that want a TUI sibling.** TCP, IPC, and the data
  bridge make it possible to expose the same operations to a terminal
  client without re-implementing them.
- **Services with policy, audit, or metrics aspects.** The hook bus
  was built for these. Adding cross-cutting behaviour without
  modifying the original module is the use case.
- **Applications you expect to outlive their first transport.** If you
  start with HTTP and might add gRPC, TCP, or WebSocket later, the
  multi-transport shape pays off.

And the inverse — when Nullspace is the wrong answer:

- **Pure HTTP, performance-critical, every microsecond matters.** Use
  Gin or Fiber. Nullspace's resolution chain, hook bus, and per-request
  snapshot all cost a small amount per request; the cost is worth it
  for what they enable, but not if raw throughput is the primary
  metric.
- **Microservices already wired into a service mesh and DI container.**
  The mesh handles cross-cutting; Nullspace's hook bus duplicates it.
- **You already have a working project and want to migrate a router.**
  Use Chi. Nullspace is shaped for new projects, not for replacing the
  router in an existing one.

## Where to read next

- [Architecture]({{< relref "/explanation/architecture" >}}) — the
  hexagonal shape behind the prescriptive choices.
- [Modules]({{< relref "/explanation/modules" >}}) — what writing a
  Nullspace module actually looks like.
- [Quickstart]({{< relref "/quickstart" >}}) — the fastest way to
  decide for yourself.
