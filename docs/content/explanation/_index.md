---
title: Explanation
weight: 5
bookCollapseSection: true
---

# Explanation

Background and rationale. Explanation pages are **understanding-oriented**:
they exist to help you build a correct mental model. They answer the "why"
that the rest of the docs do not.

- [Architecture]({{< relref "architecture" >}}) — hexagonal design and the
  module/kernel relationship.
- [The hook bus]({{< relref "hooks" >}}) — why hooks exist and how
  resolution hooks differ from fire-and-forget hooks.
- [Modules and the kernel]({{< relref "modules" >}}) — lifecycle, ordering,
  enabled/disabled state, the service locator.
- [Configuration model]({{< relref "configuration" >}}) — TOML, env
  overrides, and per-request snapshots.
- [Middleware]({{< relref "middleware" >}}) — when to choose middleware
  over hooks.
- [Format resolution]({{< relref "format-resolution" >}}) — the priority
  chain that turns one handler into many representations.
- [Multi-transport]({{< relref "transports" >}}) — sharing one kernel
  across HTTP, TCP, IPC, and WebSocket.
- [Comparisons]({{< relref "comparisons" >}}) — how Nullspace differs from
  Echo, Chi, Gin, and Fiber.
