websocket
=========

.. code-block:: text

    import "github.com/frob/nullspace/module/websocket"

Module
------

.. code-block:: go

    type Module struct{ ... }

    func New() *Module

    func (m *Module) Name() string                    // "websocket"
    func (m *Module) Config() kernel.ModuleConfig
    func (m *Module) Init(k *kernel.Kernel) error
    func (m *Module) Start(ctx context.Context) error
    func (m *Module) Stop(ctx context.Context) error

    // HandleFunc registers a named WebSocket handler. Call during Init.
    // Registered on the routing registry as "ws.<name>" after all modules Init.
    func (m *Module) HandleFunc(name string, h HandlerFunc)

    // Manager returns the connection manager.
    func (m *Module) Manager() *Manager

Config
------

.. code-block:: go

    type Config struct {
        MaxMessageSize     int64    `toml:"max_message_size"`
        AllowedOrigins     []string `toml:"allowed_origins"`
        InsecureSkipVerify bool     `toml:"insecure_skip_verify"`
    }

TOML key: ``websocket``. Default enabled: **no**.

HandlerFunc
-----------

.. code-block:: go

    type HandlerFunc func(conn *Conn, msg Message) error

Called for each message received on a connection. Return a non-nil error to
close the connection.

Conn
----

.. code-block:: go

    type Conn struct {
        ID string  // unique connection ID
    }

    func (c *Conn) Send(msg Message) error
    func (c *Conn) Close(code websocket.StatusCode, reason string) error
    func (c *Conn) Logger() kernel.Logger
    func (c *Conn) Context() context.Context
    func (c *Conn) State(key string) (any, bool)
    func (c *Conn) SetState(key string, val any)

Message
-------

.. code-block:: go

    type MessageType = websocket.MessageType

    const (
        MessageText   = websocket.MessageText
        MessageBinary = websocket.MessageBinary
    )

    type Message struct {
        Type MessageType
        Data []byte
    }

    func TextMessage(s string) Message
    func BinaryMessage(b []byte) Message

Manager
-------

.. code-block:: go

    type Manager struct{ ... }

    func NewManager() *Manager

    func (m *Manager) Add(c *Conn)
    func (m *Manager) Remove(c *Conn)
    func (m *Manager) Join(c *Conn, room string)
    func (m *Manager) Leave(c *Conn, room string)
    func (m *Manager) Broadcast(msg Message)
    func (m *Manager) BroadcastTo(room string, msg Message)
    func (m *Manager) Count() int
    func (m *Manager) RoomCount(room string) int
    func (m *Manager) CloseAll(code websocket.StatusCode, reason string)

Service Locator Keys
--------------------

============================  ========================
Key                           Type
============================  ========================
``"websocket"``               ``*websocket.Module``
``"websocket.manager"``       ``*websocket.Manager``
============================  ========================

Hooks
-----

============================  ==========================================
Hook Point                    When
============================  ==========================================
``websocket.connected``       Connection established after upgrade
``websocket.message``         Message received (before handler)
``websocket.disconnected``    Connection closed
``websocket.error``           Non-clean close error
============================  ==========================================
