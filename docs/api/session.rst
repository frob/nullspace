session
=======

``import "github.com/frob/nullspace/module/session"``

The session package provides server-side session management. It is an opt-in
module (``DefaultEnabled: false``) that registers named middleware on the
routing registry and fires lifecycle hooks.

Types
-----

Session
~~~~~~~

.. code-block:: go

    type Session struct {
        ID     string
        Values map[string]any
    }

    func (s *Session) Get(key string) (any, bool)
    func (s *Session) Set(key string, val any)
    func (s *Session) Delete(key string)
    func (s *Session) IsDirty() bool

Per-user state container. ``Set`` and ``Delete`` mark the session as dirty;
``session.load`` and ``session.require`` automatically save dirty sessions
after the handler returns.

Store
~~~~~

.. code-block:: go

    type Store interface {
        Create(ctx context.Context) (*Session, error)
        Load(ctx context.Context, id string) (*Session, error)
        Save(ctx context.Context, s *Session) error
        Delete(ctx context.Context, id string) error
    }

Session persistence interface. ``Load`` returns ``(nil, nil)`` for missing or
expired sessions.

MemoryStore
~~~~~~~~~~~

.. code-block:: go

    func NewMemoryStore(ttl time.Duration) *MemoryStore

In-process store. Sessions are lost on restart. Use for development.

SQLStore
~~~~~~~~

.. code-block:: go

    func NewSQLStore(db *sql.DB, ttl time.Duration) *SQLStore

SQL-backed store. Requires the ``data.sql`` module. Use for production.

The ``sessions`` table is created via the migration registry during
``kernel.after_init`` — the session module registers a version 1 migration
automatically when the SQL store is selected.

Module
------

.. code-block:: go

    func New() *Module

    func (m *Module) Name() string           // "session"
    func (m *Module) Store() Store
    func (m *Module) Create(ctx *request.Context) (*Session, error)
    func (m *Module) Destroy(ctx *request.Context) error

``Create`` generates a new session, sets the session cookie, and fires
``session.created``.

``Destroy`` deletes the session from the store, clears the cookie, and fires
``session.destroyed``.

Config
~~~~~~

.. code-block:: go

    type Config struct {
        Store  string // "memory" or "sql"
        Cookie string // cookie name, default "ns_session"
        TTL    string // duration string, default "24h"
        Secure bool   // Secure cookie attribute
        Path   string // cookie path, default "/"
    }

TOML key: ``[session]``

Context Helper
--------------

.. code-block:: go

    func From(ctx *request.Context) (*Session, bool)

Retrieves the current session from the request state bag. Returns ``nil,
false`` if no session has been loaded. Use inside handlers after
``session.load`` or ``session.require`` middleware.

Middleware
----------

Three named middleware entries are registered on the routing registry during
``Init``:

.. list-table::
   :header-rows: 1
   :widths: 20 80

   * - Name
     - Behavior
   * - ``session.load``
     - Loads session from cookie if present; no enforcement. Saves on dirty.
   * - ``session.require``
     - Enforces valid session; 401 or redirect if absent. Saves on dirty.
   * - ``session.ignore``
     - No-op. See ``session = "ignore"`` route field for opt-out mechanism.

Route Field
-----------

.. code-block:: toml

    [[routing.routes]]
    group   = "authenticated"
    path    = "/health"
    handler = "health.check"
    session = "ignore"

Setting ``session = "ignore"`` on a route causes ``session.load`` and
``session.require`` to skip enforcement for that route, even when the route
belongs to a group with session middleware. The check is based on route
metadata, which is resolved before the middleware chain executes.

Hooks
-----

.. list-table::
   :header-rows: 1
   :widths: 25 75

   * - Hook point
     - Fired when
   * - ``session.created``
     - ``module.Create`` succeeds
   * - ``session.loaded``
     - A session is loaded from the store
   * - ``session.destroyed``
     - ``module.Destroy`` succeeds

Resolver Hook
~~~~~~~~~~~~~

.. code-block:: go

    // session.login_url — return a redirect URL for unauthenticated requests.
    k.HookResolve("session.login_url", 10, func(ctx context.Context) (any, bool, error) {
        return "/login", true, nil
    })

If no resolver is registered, ``session.require`` returns ``401 Unauthorized``.

Service Locator
---------------

After ``Init``, the following resources are available:

.. list-table::
   :header-rows: 1
   :widths: 20 30 50

   * - Key
     - Type
     - Description
   * - ``session``
     - ``*session.Module``
     - The session module (Create, Destroy)
   * - ``session.store``
     - ``session.Store``
     - The configured store (direct store access)

.. code-block:: go

    sessMod, err := kernel.GetResource[*session.Module](k, "session")
    store, err   := kernel.GetResource[session.Store](k, "session.store")

Registration Order
------------------

The session module must be registered **after** the routing module, which
provides the ``routing.registry`` resource that session uses to register its
middleware:

.. code-block:: go

    k.Use(nslog.New())
    k.Use(request.NewAdapter())
    k.Use(routing.New())   // must come before session
    k.Use(session.New())
