---
title: Nullspace
type: docs
bookToc: false
---

# Nullspace

A multi-transport application framework in Go for serving APIs, HTML,
WebSockets, and more over HTTP, TCP, and Unix sockets. Built on hexagonal
architecture with aspect-oriented programming via a hook bus and functional
middleware.

```bash
go install github.com/frob/nullspace/cmd/nullspace@latest

mkdir mysite && cd mysite
nullspace init
nullspace
```

The documentation follows the [Diataxis](https://diataxis.fr) framework. Each
section answers a different question:

| Section | Question it answers |
|---------|---------------------|
| [Quickstart]({{< relref "/quickstart" >}}) | How do I get started in 60 seconds? |
| [Tutorials]({{< relref "/tutorials" >}}) | How do I learn Nullspace by building? |
| [How-to]({{< relref "/how-to" >}}) | How do I solve a specific problem? |
| [Reference]({{< relref "/reference" >}}) | What are the available options? |
| [Explanation]({{< relref "/explanation" >}}) | Why is it designed this way? |
| [Examples]({{< relref "/examples" >}}) | What does a complete project look like? |
| [API]({{< relref "/api" >}}) | What does this Go package expose? |

## Project

- Source: [github.com/frob/nullspace](https://github.com/frob/nullspace)
- License: see repository
- Status: pre-1.0, API may change

## At a glance

- **Convention or library** — run the `nullspace` binary against a project
  directory, or import the framework into your own Go program.
- **Multi-transport** — the same kernel serves HTTP, TCP, Unix-socket IPC,
  and WebSockets.
- **Declarative routing** — routes, groups, and collections in TOML, with
  per-module `routes.toml` files embedded via `go:embed`.
- **Hooks and middleware** — prioritised hook bus for cross-cutting concerns,
  functional middleware for the request path.
- **Format-aware responses** — content negotiation picks JSON, HTML, plain
  text, ANSI, or NDJSON from a single handler.
- **Per-request config snapshots** — a request always finishes with the
  configuration it started with.
