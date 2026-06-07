---
title: Add real-time chat with WebSockets
weight: 3
---

This tutorial adds a multi-user chat room to a Nullspace project. You
will write a custom Go module that registers a WebSocket handler,
broadcasts every message to a room of connected clients, and exposes
an HTML page that connects from the browser. By the end you can open
two browser tabs and chat between them.

## What you will build

- A library-style project (`main.go` plus a module package).
- A `chat` module that registers a WebSocket handler.
- An HTML page served at `/chat` with a tiny JavaScript client.
- A WebSocket endpoint at `/ws/chat` that joins every connection to
  the `chat` room and broadcasts received messages.

The kitchen-sink example has a working version of this module at
`cmd/examples/kitchen-sink/modules/chat/`. This tutorial walks through
building it from scratch in your own project.

## Prerequisites

- Go 1.25+.
- Familiarity with `main.go` wiring. If you have not seen Nullspace as
  a library yet, skim the [library tutorial]({{< relref "library" >}}).

## Step 1 — Scaffold the project

```bash
mkdir nschat && cd nschat
go mod init nschat
go get github.com/frob/nullspace
```

Create the directory layout:

```bash
mkdir -p modules/chat templates
```

## Step 2 — Write the configuration

`nullspace.toml`:

```toml
[request]
addr = ":8080"

[log]
level = "debug"
format = "text"

[response]
default_format = "html"
template_dir = "./templates"

# WebSocket is opt-in.
[modules]
websocket = true

# Skip origin verification for local development. In production set
# allowed_origins to your front-end host(s) instead.
[websocket]
insecure_skip_verify = true

# HTML page that hosts the chat UI.
[[routing.routes]]
path = "/chat"
handler = "chat.page"
format = "html"

# WebSocket endpoint. The `ws_rooms` meta key auto-joins every new
# connection to the named room.
[[routing.routes]]
path = "/ws/chat"
handler = "ws.chat"
middleware = ["chat.name"]
extra = { ws_rooms = "chat" }
```

Two routes; both reference handlers that we will register in code.
The `ws.chat` handler name is the convention the WebSocket module
uses: a handler registered as `chat` becomes the route handler
`ws.chat`. The middleware `chat.name` will capture the user's display
name from the upgrade request's query string.

## Step 3 — Write the chat module

`modules/chat/module.go`:

```go
// Package chat provides a WebSocket chat room.
package chat

import (
	"context"
	"encoding/json"
	"time"

	"github.com/frob/nullspace/core/request"
	"github.com/frob/nullspace/core/response"
	"github.com/frob/nullspace/core/routing"
	"github.com/frob/nullspace/kernel"
	ws "github.com/frob/nullspace/module/websocket"
)

// message is the JSON envelope broadcast to every client.
type message struct {
	User      string `json:"user"`
	Text      string `json:"text"`
	Timestamp string `json:"timestamp"`
}

// Module is the chat module.
type Module struct {
	wsMod    *ws.Module
	pipeline *response.Pipeline
}

// New constructs a chat module.
func New() *Module { return &Module{} }

func (m *Module) Name() string                    { return "chat" }
func (m *Module) Start(ctx context.Context) error { return nil }
func (m *Module) Stop(ctx context.Context) error  { return nil }

func (m *Module) Init(k *kernel.Kernel) error {
	// Look up the dependencies we need.
	var err error
	m.wsMod, err = kernel.GetResource[*ws.Module](k, "websocket")
	if err != nil {
		return err
	}
	m.pipeline, err = kernel.GetResource[*response.Pipeline](k, "response.pipeline")
	if err != nil {
		return err
	}
	reg, err := kernel.GetResource[*routing.Registry](k, "routing.registry")
	if err != nil {
		return err
	}

	mgr := m.wsMod.Manager()

	// Middleware that copies the ?name= query param into request state
	// before the WebSocket upgrade. The upgrade copies request state
	// onto the connection, so the handler can read it back later.
	reg.Middleware("chat.name", func(next request.HandlerFunc) request.HandlerFunc {
		return func(ctx *request.Context) error {
			if n := ctx.Request.URL.Query().Get("name"); n != "" {
				ctx.SetState("chat.user", n)
			}
			return next(ctx)
		}
	})

	// The WebSocket handler. Called for every received message.
	m.wsMod.HandleFunc("chat", func(conn *ws.Conn, msg ws.Message) error {
		user := "anonymous"
		if v, ok := conn.State("chat.user"); ok {
			if s, _ := v.(string); s != "" {
				user = s
			}
		}

		payload, _ := json.Marshal(message{
			User:      user,
			Text:      string(msg.Data),
			Timestamp: time.Now().Format(time.RFC3339),
		})

		// BroadcastTo sends to every connection in the named room. The
		// `ws_rooms = "chat"` meta on the route added this connection
		// (and every other one) to that room automatically.
		mgr.BroadcastTo("chat", ws.TextMessage(string(payload)))
		return nil
	})

	// HTML handler for the /chat page.
	reg.HandleFunc("chat.page", func(ctx *request.Context) error {
		resp := response.NewResponse(200, map[string]any{"Title": "Chat"})
		resp.Template = "chat.html"
		return m.pipeline.Write(ctx.Context(), ctx.Writer, resp)
	})

	return nil
}
```

A few things to notice:

- The module pulls its dependencies from the kernel's service locator
  using `kernel.GetResource[T]`. The keys (`"websocket"`,
  `"response.pipeline"`, `"routing.registry"`) are documented per
  module.
- `HandleFunc("chat", ...)` registers a WebSocket handler. The
  WebSocket module wraps it in a `request.HandlerFunc` named
  `ws.chat` after every module has finished `Init`, so a TOML route
  can reference it.
- `mgr.BroadcastTo("chat", ...)` sends to every connection in the
  `chat` room. Connections joined the room via the route meta
  `ws_rooms = "chat"`, so no explicit `Join` call is needed.

## Step 4 — Write the HTML client

`templates/chat.html`:

```html
<!DOCTYPE html>
<html>
<head>
  <title>Chat</title>
  <style>
    body { font-family: system-ui, sans-serif; max-width: 600px; margin: 2rem auto; }
    #log { border: 1px solid #ccc; padding: 1rem; height: 320px; overflow-y: auto; }
    .msg { margin: 0.25rem 0; }
    .user { font-weight: bold; }
    .time { color: #999; font-size: 0.85em; margin-left: 0.5rem; }
    form { display: flex; gap: 0.5rem; margin-top: 0.5rem; }
    input[type=text] { flex: 1; padding: 0.5rem; }
    button { padding: 0.5rem 1rem; }
  </style>
</head>
<body>
  <h1>{{.Title}}</h1>

  <div id="join">
    <label>Name: <input type="text" id="name" autofocus></label>
    <button onclick="connect()">Join</button>
  </div>

  <div id="room" style="display:none">
    <div id="log"></div>
    <form onsubmit="event.preventDefault(); send();">
      <input type="text" id="text" placeholder="Say something" autocomplete="off">
      <button type="submit">Send</button>
    </form>
  </div>

  <script>
    let ws;

    function connect() {
      const name = document.getElementById('name').value.trim() || 'anonymous';
      document.getElementById('join').style.display = 'none';
      document.getElementById('room').style.display = 'block';

      const proto = location.protocol === 'https:' ? 'wss:' : 'ws:';
      ws = new WebSocket(proto + '//' + location.host + '/ws/chat?name=' + encodeURIComponent(name));

      ws.onmessage = (e) => {
        const m = JSON.parse(e.data);
        const div = document.createElement('div');
        div.className = 'msg';
        const t = new Date(m.timestamp).toLocaleTimeString();
        div.innerHTML = '<span class="user"></span>: <span class="text"></span>' +
                        '<span class="time"></span>';
        div.querySelector('.user').textContent = m.user;
        div.querySelector('.text').textContent = m.text;
        div.querySelector('.time').textContent = t;
        const log = document.getElementById('log');
        log.appendChild(div);
        log.scrollTop = log.scrollHeight;
      };
    }

    function send() {
      const input = document.getElementById('text');
      const text = input.value.trim();
      if (text && ws && ws.readyState === WebSocket.OPEN) {
        ws.send(text);
        input.value = '';
      }
    }
  </script>
</body>
</html>
```

The page asks for a name, opens a WebSocket to
`/ws/chat?name=<name>`, and renders each incoming JSON message.

## Step 5 — Wire it together in main.go

`main.go`:

```go
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"nschat/modules/chat"

	"github.com/frob/nullspace/core/nslog"
	"github.com/frob/nullspace/core/request"
	"github.com/frob/nullspace/core/response"
	"github.com/frob/nullspace/core/routing"
	"github.com/frob/nullspace/kernel"
	"github.com/frob/nullspace/module/websocket"
)

func main() {
	k := kernel.New(kernel.WithConfigFile("nullspace.toml"))

	// Order matters: each module may depend on resources provided by
	// modules registered before it.
	k.Use(nslog.New())
	k.Use(request.NewAdapter())
	k.Use(response.NewPipeline())
	k.Use(response.NewFormatRouteOverride())
	k.Use(response.NewFormatQueryParam())
	k.Use(response.NewFormatContentNegotiate())
	k.Use(response.NewFormatDefault())
	k.Use(routing.New())   // routing must come before websocket
	k.Use(websocket.New()) // websocket must come before modules that use it
	k.Use(chat.New())

	ctx := context.Background()
	if err := k.Init(ctx); err != nil {
		fmt.Fprintln(os.Stderr, "init:", err)
		os.Exit(1)
	}
	if err := k.Start(ctx); err != nil {
		fmt.Fprintln(os.Stderr, "start:", err)
		os.Exit(1)
	}

	k.Logger().Info("chat running", "url", "http://localhost:8080/chat")

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	_ = k.Stop(ctx)
}
```

{{< hint info >}}
The registration order is not arbitrary. The `chat` module's `Init`
calls `kernel.GetResource[*websocket.Module]` — that resource only
exists because `websocket.New()` was registered first and provided
itself under the key `"websocket"`. The same logic applies to every
other dependency. See the [library tutorial]({{< relref "library" >}})
for a deeper dive on registration order.
{{< /hint >}}

## Step 6 — Run it

```bash
go run .
```

Verify the routes:

```bash
nullspace routes
```

Wait — `nullspace routes` reads from a different kernel. For a
library project, log the table from your own code or just check the
log output. You should see two relevant log lines on startup:

```
level=INFO msg="websocket handler registered" name=ws.chat
level=INFO msg="chat running" url=http://localhost:8080/chat
```

## Step 7 — Chat from two browser tabs

Open [http://localhost:8080/chat](http://localhost:8080/chat) in two
browser tabs. Enter different names in each (`alice` and `bob`),
click **Join**, and send messages. Each tab sees its own and the
other's messages, prefixed with the sender's name and a timestamp.

Behind the scenes:

1. The browser issues `GET /ws/chat?name=alice` with the WebSocket
   upgrade headers.
2. The `chat.name` middleware copies `alice` into request state under
   the key `chat.user`.
3. The WebSocket module upgrades the connection and copies the
   request state bag onto the connection, then auto-joins it to the
   `chat` room (from `extra = { ws_rooms = "chat" }`).
4. When Alice types a message, your handler reads `chat.user` from
   the connection state, wraps the text in a JSON envelope, and calls
   `mgr.BroadcastTo("chat", ...)`.
5. The connection manager iterates every connection in the room and
   writes the envelope to each, so every browser tab receives it.

## Step 8 — Tail the connection count

The WebSocket module logs every connect and disconnect. Watch the
log while you open and close tabs:

```
level=INFO msg="websocket connected" ws_conn=chat conn_id=a1b2c3d4
level=INFO msg="websocket disconnected" ws_conn=chat conn_id=a1b2c3d4
```

You can hook into these lifecycle events from any module:

```go
k.Hook("websocket.connected", 10, func(ctx context.Context) error {
    k.Logger().Info("someone joined")
    return nil
})
```

The hook bus fires `websocket.connected`, `websocket.message`,
`websocket.disconnected`, and `websocket.error` at the appropriate
points.

## Where to go next

- [Wire Nullspace as a library]({{< relref "library" >}}) — the
  module pattern, registration order, and graceful shutdown explained
  in full.
- [How-to: Broadcast to rooms]({{< relref "/how-to" >}}) — joining,
  leaving, and multi-room patterns.
- [How-to: Hook into the request lifecycle]({{< relref "/how-to" >}}) —
  use `websocket.connected` and friends.
