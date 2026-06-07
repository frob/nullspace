---
title: Multi-transport
weight: 7
---

The conventional Go web framework is single-transport: it serves HTTP.
Adding TCP, a Unix socket, or WebSockets is left to your code, and
sharing code between them is left to your imagination. Nullspace treats
HTTP, TCP, IPC, and WebSockets as peers — four adapters around the same
kernel, with the same hook bus and the same data layer behind all of
them.

This page explains what that buys you, what it costs, and where the
seams are.

## The shape

```
                       +-------------------+
                       |     Kernel        |
                       |  - hook bus       |
                       |  - config         |
                       |  - service loc.   |
                       +-------------------+
            /             /            \              \
   +--------+----+  +-----+------+  +--+--------+  +---+------+
   |    HTTP    |  |    TCP     |  |    IPC    |  | WebSocket|
   |  adapter   |  |  adapter   |  |  adapter  |  |   module |
   +------------+  +------------+  +-----------+  +----------+
       |                |                |              |
       v                v                v              v
   net/http          net.Listener   net.Listener   (HTTP upgrade)
                     (TCP)          (Unix socket)
```

Each adapter is a module. Each opens its own listener during `Start` and
closes it during `Stop`. Each tags its inbound work with hooks
(`request.received`, `tcp.connected`, `websocket.message`, and so on).
The kernel does not know how many adapters are loaded; it knows about
modules and hooks.

This means a Nullspace server can serve HTTP on `:8080`, TCP on `:9090`,
and a Unix socket at `/tmp/myapp.sock`, all from one process, with one
configuration file, and the modules backing each one share state through
the service locator.

## Why share a kernel

The case for one kernel across transports is the case against rewriting
your business logic for each one.

Imagine the alternative: one Go binary that exposes the same content
collection over HTTP, TCP, and IPC, each implemented separately. The
HTTP code parses the URL, picks the collection, calls the data layer.
The TCP code parses the command, picks the collection, calls the data
layer. The IPC code parses the command, picks the collection, calls the
data layer. The "calls the data layer" part is identical. The framing,
parsing, and routing are different. If you add validation to the HTTP
write path, you have to remember to add it to the TCP write path too.

Nullspace's `data.bridge` module addresses this by registering the same
handlers on the TCP and IPC routers as the HTTP routing module uses
internally. The bridge knows nothing about HTTP; it just looks up the
file module from the service locator and exposes its CRUD operations as
TCP commands. Validation lives on the `data.before_write` hook, which
fires regardless of transport. Authorisation, audit, and cache
invalidation work the same way.

The benefit is structural: a feature added at the data layer is
available over every transport without coordination. The handler logic
is written once.

## Transports as adapters

Each transport adapter implements the `Module` interface and exposes a
listener-shaped behaviour:

- **HTTP** (`core/request`) — wraps `net/http`. Routes are URL-matched.
  The request adapter is the entry point; it fires
  `request.received`, runs middleware, fires `request.routed`, calls
  the matched handler, and fires `request.complete`.
- **TCP** (`core/tcp`) — listens on a TCP address. Each connection
  reads framed messages via a codec, dispatches to a command-named
  handler, and fires `tcp.connected`, `tcp.message`, and
  `tcp.disconnected`.
- **IPC** (`core/ipc`) — a thin wrapper that builds a TCP adapter on a
  Unix socket listener instead of a TCP listener. Same router, same
  codec, same hooks, different path semantics. Configuration looks
  almost identical except for `path` versus `addr`.
- **WebSocket** (`module/websocket`) — opt-in module that registers a
  handler with the HTTP routing module. The upgrade happens on an HTTP
  route; after upgrade, the connection moves to a per-message loop with
  `websocket.connected`, `websocket.message`, and
  `websocket.disconnected` hooks.

WebSocket is interesting because it is not a separate listener — it is
an HTTP route that hijacks the connection at upgrade time. It still
sits in this list because, from the application's perspective, it is a
distinct transport with its own message-shaped lifecycle. The kernel
treats it that way.

## TCP codec choices

TCP needs framing — without it, the receiver has no way to know where
one message ends and the next begins. The HTTP and WebSocket adapters
solve framing through their protocols. TCP and IPC have to choose.

The TCP adapter supports two codecs out of the box:

### `json-lines`

Each message is a JSON object terminated by a newline:

```
{"command":"data.list","payload":{"collection":"posts"}}\n
{"command":"data.get","payload":{"collection":"posts","id":"hello"}}\n
```

This is the default and the easiest to use. It is readable in a terminal
with `nc`, debuggable with `tcpdump`, and natively serialisable on
every platform that has JSON. The cost is bandwidth — JSON has
overhead, and large binary payloads either need base64 encoding (more
overhead) or a different codec.

### `length-prefix`

Each message is a 4-byte big-endian length followed by that many bytes
of payload:

```
[len: 00 00 00 23][payload: {"command":"data.list",...}]
[len: 00 00 00 31][payload: {"command":"data.get",...}]
```

The payload can be anything — JSON, msgpack, protobuf, raw bytes. The
codec only frames; it does not parse. The cost is human-readability:
this is not a format you can prod with `telnet`.

The right choice depends on what you are doing. `json-lines` for
debuggable interactive protocols, configuration, and admin
consoles. `length-prefix` for high-throughput binary protocols where
overhead matters.

A third codec is straightforward to add: implement the `Codec`
interface and register it. The TCP adapter looks the codec up by name
from configuration.

## The data bridge in detail

The `data.bridge` module is the canonical example of code shared across
transports. When enabled, it:

1. Looks up the file module from the service locator.
2. Hooks `kernel.after_init`, so that all transport adapters have
   already provided their routers.
3. After init, looks up the TCP and IPC routers (if those modules are
   enabled) and registers data CRUD commands on each.

The result is that the same data operations available at
`GET /api/posts` over HTTP are available as `{"command":"data.list",
"payload":{"collection":"posts"}}` over TCP. Hooks on
`data.before_read` and `data.before_write` fire regardless of which
transport invoked the operation. A policy module written without
knowledge of TCP automatically enforces its policy on TCP requests.

The streaming envelope format (`data.list.start`, `data.list.item`,
`data.list.end`) is the TCP analogue of NDJSON over HTTP. Both stream
the same data; the framing differs because the transports differ.

## What it costs

Multi-transport is not free.

**More moving parts.** A pure-HTTP Nullspace deployment ignores most of
this and behaves like any other Go web framework. A multi-transport
deployment has more configuration, more failure modes (listeners can
fail independently), and more places to look when something goes
wrong.

**Hook contract widens.** Because the `data.before_read` hook fires for
HTTP, TCP, and IPC requests, the hook handler cannot assume an HTTP
context. The `context.Context` it receives may not have a
`*http.Request` attached. Policy modules and audit modules need to
write to the lowest common denominator.

**Format resolution is HTTP-flavoured.** The format resolution chain
was designed for HTTP: route override, query param, `Accept` header,
default. Over TCP, only "route override" (well, the equivalent at the
command level) and "default" apply. The TCP adapter does not run the
full format chain; it serialises responses through the same formatter
registry but with simpler selection logic. This is by design — TCP
clients can specify `format` in their payload if they want — but it
means the two transports do not behave identically when you ask them
to.

**Auth is your problem.** HTTP has cookies, sessions, bearer tokens. TCP
and IPC have neither. If you want authenticated TCP, you have to
design a handshake; the framework does not impose one. The hooks are
there to plug into (`tcp.connected` is the obvious place), but the
authentication protocol is yours.

## When to use what

A rough guide.

- **HTTP** — anything a browser or HTTP client will hit. Public APIs,
  HTML pages, webhooks. This is the default and what most applications
  need.
- **WebSocket** — bidirectional, low-latency push to browser clients.
  Live updates, chat, collaborative editing. Sits on top of HTTP and
  needs HTTP to be enabled.
- **TCP** — services on the same network, custom protocols, TUI
  clients. The data bridge over TCP is useful when you want the
  content backend addressable from a desktop tool without standing up
  HTTP for it.
- **IPC** (Unix socket) — same machine, often same user. Admin
  consoles, sidecars, control planes. The Unix socket filesystem
  permission is the auth boundary in many cases.

If you only need HTTP, leave the other transports disabled (they
default to off). The kernel will not load them; they cost nothing.

## Where to read next

- [Format resolution]({{< relref "/explanation/format-resolution" >}}) —
  how response format selection differs across transports.
- [The hook bus]({{< relref "/explanation/hooks" >}}) — the layer
  that makes cross-transport behaviour possible.
- [Architecture]({{< relref "/explanation/architecture" >}}) — why the
  kernel is shaped to make this kind of layering natural.
