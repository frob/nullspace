---
title: Broadcast to rooms
weight: 18
---

Use this guide when you want a group of WebSocket clients to receive the same
messages — chat rooms, live dashboards, multiplayer state.

## Solution

Join connections to a named room with the connection manager, then broadcast
to that room from any handler.

### Auto-join via route metadata

The simplest route-level recipe: set `extra.ws_rooms` and every new
connection joins automatically.

```toml
[[routing.routes]]
path    = "/ws/chat"
handler = "ws.chat"
extra   = { ws_rooms = "chat" }
```

### Broadcast from a handler

Get the manager from the service locator (or from `wsMod.Manager()`) during
`Init` and call `BroadcastTo`:

```go
import (
    ws "github.com/frob/nullspace/module/websocket"
    "github.com/frob/nullspace/kernel"
)

func (m *Module) Init(k *kernel.Kernel) error {
    wsMod, _ := kernel.GetResource[*ws.Module](k, "websocket")
    mgr := wsMod.Manager()

    wsMod.HandleFunc("chat", func(conn *ws.Conn, msg ws.Message) error {
        mgr.BroadcastTo("chat", msg)
        return nil
    })
    return nil
}
```

Every connection currently in the `chat` room receives `msg`. Connections
are removed from all rooms automatically when they disconnect.

## Manager API

```go
mgr.Join(conn, "room")        // add to room
mgr.Leave(conn, "room")       // remove from room
mgr.BroadcastTo("room", msg)  // send to all in room
mgr.RoomCount("room")         // count
mgr.Broadcast(msg)            // every connection
mgr.Count()                   // total connections
mgr.CloseAll(code, reason)    // shutdown
```

## Variations

### Join from a handler

Skip `ws_rooms` and join explicitly — useful when room membership depends on
user state:

```go
wsMod.HandleFunc("chat", func(conn *ws.Conn, msg ws.Message) error {
    if _, joined := conn.State("joined"); !joined {
        mgr.Join(conn, "chat")
        conn.SetState("joined", true)
    }
    mgr.BroadcastTo("chat", msg)
    return nil
})
```

### Wrap messages in JSON

```go
type chatMsg struct {
    User string `json:"user"`
    Text string `json:"text"`
}

wsMod.HandleFunc("chat", func(conn *ws.Conn, msg ws.Message) error {
    user, _ := conn.State("chat.user")
    out, _ := json.Marshal(chatMsg{User: fmt.Sprint(user), Text: string(msg.Data)})
    mgr.BroadcastTo("chat", ws.TextMessage(string(out)))
    return nil
})
```

### Multiple rooms per connection

`ws_rooms` accepts a comma-separated list. A connection can also belong to
several rooms after calling `Join` repeatedly.

```toml
extra = { ws_rooms = "chat,global" }
```

## See also

- [Upgrade to a WebSocket connection]({{< relref "websocket" >}})
