WebSockets
==========

The ``websocket`` package provides WebSocket support as an opt-in module. It
handles the HTTP upgrade, manages connections, and provides room-based message
broadcast. Application code registers named handlers that receive messages on
individual connections.

Enabling WebSockets
-------------------

Register the module after the routing module and before any modules that use it:

.. code-block:: go

    import "github.com/frob/nullspace/module/websocket"

    k.Use(routing.New())
    k.Use(websocket.New())   // after routing
    k.Use(myapp.New())       // app registers WS handlers during Init

Enable it in ``nullspace.toml``:

.. code-block:: toml

    [modules]
    websocket = true

Configuration
-------------

.. code-block:: toml

    [websocket]
    max_message_size     = 65536    # bytes; 0 = no limit
    allowed_origins      = ["example.com", "*.example.com"]
    insecure_skip_verify = false    # skip origin checks (development only)

**Origin checking:**

- By default, only same-origin requests are accepted.
- Set ``allowed_origins`` to a list of host patterns for cross-origin access.
- Set ``insecure_skip_verify = true`` to disable origin checks entirely
  (useful for local development, never use in production).

Registering Handlers
--------------------

Register WebSocket handlers during your module's ``Init`` using the module's
``HandleFunc`` method. Each handler receives messages on a single connection:

.. code-block:: go

    func (m *myModule) Init(k *kernel.Kernel) error {
        wsMod, err := kernel.GetResource[*websocket.Module](k, "websocket")
        if err != nil {
            return err
        }

        wsMod.HandleFunc("echo", func(conn *websocket.Conn, msg websocket.Message) error {
            return conn.Send(msg)  // echo the message back
        })

        return nil
    }

Handlers are registered on the routing registry as ``ws.<name>`` (e.g.,
``ws.echo``) after all modules initialize. Reference them in TOML routes:

.. code-block:: toml

    [[routing.routes]]
    path    = "/ws/echo"
    handler = "ws.echo"

Route Metadata
--------------

Use the ``extra`` field to pass module-specific metadata to WebSocket routes:

.. code-block:: toml

    [[routing.routes]]
    path       = "/ws/chat"
    handler    = "ws.chat"
    middleware = ["session.load"]
    extra      = { ws_rooms = "chat" }

The ``ws_rooms`` key auto-joins new connections to the named room. Modules
can read any ``extra`` values from ``ctx.Route().Meta``.

Connection Lifecycle
--------------------

When a client connects to a WebSocket route:

1. Request enters the normal middleware chain (session, auth, etc.)
2. The upgrade handler calls ``websocket.Accept`` to complete the handshake
3. The framework marks the connection as hijacked (``ctx.Hijack()``)
4. The ``websocket.connected`` hook fires
5. If ``ws_rooms`` metadata is set, the connection auto-joins that room
6. The message loop begins — each message invokes the registered handler
7. When the connection closes, the ``websocket.disconnected`` hook fires
8. The connection is removed from the manager

Because ``ctx.Hijack()`` is called after a successful upgrade, the adapter
skips post-handler hooks (``request.after``, ``request.complete``) and does not
attempt to write error responses. Middleware that ran before the upgrade
(session loading, authentication) works normally — state set by middleware is
carried into the ``Conn`` object.

If the upgrade fails (bad request, origin rejection), the WebSocket library
writes its own HTTP error response and the normal request lifecycle completes.

The Conn Object
---------------

Each connection is wrapped in a ``*websocket.Conn`` that provides:

.. code-block:: go

    conn.ID                        // unique connection ID (hex string)
    conn.Send(msg)                 // write a message to the peer
    conn.Close(code, reason)       // graceful close handshake
    conn.Logger()                  // per-connection logger (from upgrade request)
    conn.Context()                 // connection context (cancelled on close)
    conn.State(key)                // read from connection state bag
    conn.SetState(key, val)        // write to connection state bag

The state bag is initialized with all request state from middleware that ran
before the upgrade. For example, if ``session.load`` middleware ran, the
session is available via ``conn.State("session")``.

Messages
--------

.. code-block:: go

    // Message types
    websocket.MessageText     // UTF-8 text frame
    websocket.MessageBinary   // binary frame

    // Constructors
    msg := websocket.TextMessage("hello")
    msg := websocket.BinaryMessage(data)

    // Reading in a handler
    func handler(conn *websocket.Conn, msg websocket.Message) error {
        fmt.Println(msg.Type, string(msg.Data))
        return conn.Send(msg)  // echo
    }

Connection Manager
------------------

The manager tracks all active connections and provides room-based broadcast.
Retrieve it from the service locator:

.. code-block:: go

    mgr, _ := kernel.GetResource[*websocket.Manager](k, "websocket.manager")

**Rooms:**

.. code-block:: go

    mgr.Join(conn, "chat")              // add to room
    mgr.Leave(conn, "chat")             // remove from room
    mgr.BroadcastTo("chat", msg)        // send to all in room
    mgr.RoomCount("chat")              // connections in room

**Global:**

.. code-block:: go

    mgr.Broadcast(msg)                  // send to all connections
    mgr.Count()                         // total active connections
    mgr.CloseAll(code, reason)          // graceful shutdown

Connections are automatically removed from all rooms when they disconnect.

Hooks
-----

The websocket module fires the following hooks:

.. list-table::
   :header-rows: 1
   :widths: 30 70

   * - Hook
     - Fired when
   * - ``websocket.connected``
     - Connection established after successful upgrade
   * - ``websocket.message``
     - Message received (before handler invoked)
   * - ``websocket.disconnected``
     - Connection closed (clean or otherwise)
   * - ``websocket.error``
     - Non-clean close (read/write error, not normal closure)

.. code-block:: go

    k.Hook("websocket.connected", 10, func(ctx context.Context) error {
        // e.g., log connection count, rate-limit checks
        return nil
    })

Graceful Shutdown
-----------------

When the kernel stops, the websocket module sends a ``GoingAway`` close frame
to all active connections before shutting down. Modules stop in reverse
registration order, so application modules that use WebSocket connections
stop first.

Working with Sessions
---------------------

WebSocket routes can use session middleware normally. The session is loaded
during the HTTP upgrade request (before hijack) and carried into the
connection state:

.. code-block:: toml

    [[routing.routes]]
    path       = "/ws/app"
    handler    = "ws.app"
    middleware = ["session.load"]

.. code-block:: go

    wsMod.HandleFunc("app", func(conn *websocket.Conn, msg websocket.Message) error {
        if sess, ok := conn.State("session"); ok {
            // session was loaded before upgrade
        }
        return nil
    })

Note that cookie writes are skipped on hijacked connections. If your handler
calls ``session.Create`` or ``session.Destroy`` after the upgrade, the store
operation succeeds but no ``Set-Cookie`` header is sent (the HTTP response is
already complete). Set session state before the upgrade or use a separate HTTP
endpoint for authentication.

Example: Chat Room
------------------

The example application at ``cmd/examples/kitchen-sink/`` includes a chat module that
demonstrates a complete WebSocket integration:

.. code-block:: toml

    [modules]
    websocket = true
    chat = true

    [websocket]
    insecure_skip_verify = true

    [[routing.routes]]
    path       = "/ws/chat"
    handler    = "ws.chat"
    middleware = ["chat.name"]
    extra      = { ws_rooms = "chat" }

The chat module registers a handler that broadcasts JSON messages to all
connections in the ``chat`` room, and a middleware that reads the ``?name=``
query parameter into the connection state. See ``cmd/examples/kitchen-sink/modules/chat/``
for the full implementation.
