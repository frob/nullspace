---
title: Upgrade to a WebSocket connection
weight: 17
---

Use this guide when you want a route to handle a WebSocket connection
instead of a single HTTP request.

## Solution

Enable the `websocket` module, register a named handler during `Init`, and
point a TOML route at `ws.<name>`.

```toml
[modules]
websocket = true

[websocket]
insecure_skip_verify = true     # dev only; locks origin check in prod
# allowed_origins    = ["example.com", "*.example.com"]
# max_message_size   = 65536
```

```go
import (
    ws "github.com/frob/nullspace/module/websocket"
    "github.com/frob/nullspace/kernel"
)

func (m *Module) Init(k *kernel.Kernel) error {
    wsMod, err := kernel.GetResource[*ws.Module](k, "websocket")
    if err != nil {
        return err
    }

    wsMod.HandleFunc("echo", func(conn *ws.Conn, msg ws.Message) error {
        return conn.Send(msg)
    })
    return nil
}
```

```toml
[[routing.routes]]
path    = "/ws/echo"
handler = "ws.echo"
```

Connect with any WebSocket client:

```
wscat -c ws://localhost:8080/ws/echo
> hello
< hello
```

## Module registration order

`websocket.New()` must come after `routing.New()` and before any module that
registers WS handlers.

```go
k.Use(routing.New())
k.Use(websocket.New())
k.Use(myapp.New())     // calls wsMod.HandleFunc("...")
```

## The Conn object

```go
conn.ID                   // unique connection ID
conn.Send(msg)            // write a message
conn.Close(code, reason)  // graceful close
conn.Logger()             // per-connection logger
conn.Context()            // cancelled on close
conn.State(key)           // request state from middleware that ran pre-upgrade
conn.SetState(key, val)
```

State set by middleware before the upgrade (session, auth) is copied into
the connection.

## Messages

```go
ws.TextMessage("hello")     // ws.MessageText
ws.BinaryMessage(data)      // ws.MessageBinary

func handler(conn *ws.Conn, msg ws.Message) error {
    fmt.Println(msg.Type, string(msg.Data))
    return conn.Send(msg)
}
```

## Variations

### Apply middleware before the upgrade

Middleware runs as normal during the upgrade request:

```toml
[[routing.routes]]
path       = "/ws/app"
handler    = "ws.app"
middleware = ["session.load"]
```

```go
wsMod.HandleFunc("app", func(conn *ws.Conn, msg ws.Message) error {
    if sess, ok := conn.State("session"); ok {
        // session was loaded before upgrade
    }
    return nil
})
```

Cookie writes after the upgrade are silently dropped — set state before the
upgrade or use a separate HTTP endpoint for auth.

### Allowed origins

```toml
[websocket]
allowed_origins = ["example.com", "*.example.com"]
```

## See also

- [Broadcast to rooms]({{< relref "rooms" >}})
- [Add middleware to a route group]({{< relref "middleware" >}})
