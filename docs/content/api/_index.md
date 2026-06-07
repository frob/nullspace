---
title: API
weight: 7
bookCollapseSection: true
---

# API reference

Machine-generated documentation for every exported symbol in the Nullspace
Go packages, produced by [`gomarkdoc`](https://github.com/princjef/gomarkdoc)
straight from the source.

The pages here mirror the package layout:

- [`kernel`]({{< relref "kernel" >}}) — module registry, hook bus, config,
  service locator.
- [`core/nslog`]({{< relref "core-nslog" >}}) — structured logging adapter.
- [`core/request`]({{< relref "core-request" >}}) — HTTP adapter, router,
  context.
- [`core/response`]({{< relref "core-response" >}}) — pipeline, formatters,
  format resolvers.
- [`core/routing`]({{< relref "core-routing" >}}) — TOML route loader and
  handler registry.
- [`core/tcp`]({{< relref "core-tcp" >}}) — TCP adapter and command
  router.
- [`core/ipc`]({{< relref "core-ipc" >}}) — Unix-socket transport.
- [`module/data/file`]({{< relref "module-data-file" >}}) — file-entity
  storage.
- [`module/data/sql`]({{< relref "module-data-sql" >}}) — SQL data module.
- [`module/data/static`]({{< relref "module-data-static" >}}) — static
  file fallback.
- [`module/data/bridge`]({{< relref "module-data-bridge" >}}) — TCP/IPC
  data command bridge.
- [`module/session`]({{< relref "module-session" >}}) — session store.
- [`module/httpsecurity`]({{< relref "module-httpsecurity" >}}) — security
  headers, CSRF, HTTPS redirect.
- [`module/websocket`]({{< relref "module-websocket" >}}) — WebSocket
  upgrade and connection manager.

{{< hint info >}}
These pages are regenerated on every documentation build. Don't edit them by
hand — fix the doc comment in the source file instead.
{{< /hint >}}
