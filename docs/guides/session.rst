Sessions
========

The ``session`` package provides server-side session management. It is an
opt-in module — disabled by default — that registers three named middleware
entries on the routing registry.

Enabling Sessions
-----------------

Add the session module to your kernel **after** the routing module:

.. code-block:: go

    import "github.com/frob/nullspace/module/session"

    k.Use(nslog.New())
    k.Use(request.NewAdapter())
    k.Use(routing.New())
    k.Use(session.New())   // after routing

Enable it in ``nullspace.toml``:

.. code-block:: toml

    [modules]
    session = true

Configuration
-------------

.. code-block:: toml

    [session]
    store  = "memory"      # "memory" (default) or "sql"
    cookie = "ns_session"  # cookie name
    ttl    = "24h"         # session lifetime
    secure = true          # set Secure attribute on cookie (use in production)
    path   = "/"           # cookie path

**Store options:**

- ``memory`` — in-process, non-persistent. Sessions are lost on restart.
  Good for development.
- ``sql`` — SQL-backed, survives restarts. Requires the ``data.sql`` module.

To use the SQL store:

.. code-block:: toml

    [modules]
    session   = true
    "data.sql" = true

    [session]
    store = "sql"

    [data.sql]
    driver = "sqlite"
    dsn    = "./app.db"

Middleware
----------

Three named middleware entries are registered on the routing registry:

``session.load``
~~~~~~~~~~~~~~~~

Loads the session from the cookie if present. Passes through without error if
no cookie exists or the session has expired. Automatically saves the session
after the handler if it was modified.

Use this on routes where a session is optional — the session enriches the
request if present but is not required.

.. code-block:: toml

    [[routing.routes]]
    path       = "/home"
    handler    = "pages.home"
    middleware = ["session.load"]

``session.require``
~~~~~~~~~~~~~~~~~~~

Loads the session and enforces that a valid session exists. Returns ``401
Unauthorized`` if no valid session is found, or redirects to a login URL if
a ``session.login_url`` resolver hook is registered (see `Login Redirects`_).

Use this on routes that require authentication.

.. code-block:: toml

    [routing.groups.authenticated]
    prefix     = "/app"
    middleware = ["session.require"]

    [[routing.routes]]
    group   = "authenticated"
    path    = "/dashboard"
    handler = "app.dashboard"

    [[routing.routes]]
    group   = "authenticated"
    path    = "/profile"
    handler = "app.profile"

``session.ignore``
~~~~~~~~~~~~~~~~~~

A no-op middleware. Its presence in a middleware list is documentation-friendly,
but the actual opt-out mechanism is the route-level ``session = "ignore"`` field
(see `Opting Out Inside Authenticated Groups`_).

Opting Out Inside Authenticated Groups
---------------------------------------

A route inside an authenticated group can bypass session enforcement by setting
``session = "ignore"`` in its route definition:

.. code-block:: toml

    [routing.groups.app]
    prefix     = "/app"
    middleware = ["session.require"]

    [[routing.routes]]
    group   = "app"
    path    = "/dashboard"
    handler = "app.dashboard"

    [[routing.routes]]
    group   = "app"
    path    = "/health"
    handler = "health.check"
    session = "ignore"        # bypasses session.require for this route

The ``session.require`` middleware reads this metadata before enforcing, so
the route is served even without a valid session.

This works regardless of middleware ordering because the route match metadata
is set before the middleware chain executes.

Reading Session Data
---------------------

Use ``session.From`` to retrieve the loaded session inside a handler:

.. code-block:: go

    import "github.com/frob/nullspace/module/session"

    func profileHandler(ctx *request.Context) error {
        sess, ok := session.From(ctx)
        if !ok {
            return nil // no session (use session.require middleware to enforce)
        }

        userID, _ := sess.Get("user_id")
        // ...
    }

Creating a Session
------------------

Call ``module.Create`` after successful authentication. This writes the session
cookie and fires the ``session.created`` hook:

.. code-block:: go

    // In your login handler:
    func loginHandler(sessMod *session.Module) request.HandlerFunc {
        return func(ctx *request.Context) error {
            // ... validate credentials ...

            sess, err := sessMod.Create(ctx)
            if err != nil {
                return err
            }
            sess.Set("user_id", user.ID)
            sess.Set("role", user.Role)

            http.Redirect(ctx.Writer, ctx.Request, "/app/dashboard", http.StatusSeeOther)
            return nil
        }
    }

The session module is available via the service locator after ``Init``:

.. code-block:: go

    sessMod, err := kernel.GetResource[*session.Module](k, "session")

Destroying a Session
--------------------

Call ``module.Destroy`` from a logout handler. This deletes the session from
the store, clears the cookie, and fires the ``session.destroyed`` hook:

.. code-block:: go

    func logoutHandler(sessMod *session.Module) request.HandlerFunc {
        return func(ctx *request.Context) error {
            if err := sessMod.Destroy(ctx); err != nil {
                return err
            }
            http.Redirect(ctx.Writer, ctx.Request, "/login", http.StatusSeeOther)
            return nil
        }
    }

Login Redirects
---------------

By default, unauthenticated requests receive ``401 Unauthorized``. To redirect
to a login page instead, register a ``session.login_url`` resolver hook:

.. code-block:: go

    k.HookResolve("session.login_url", 10, func(ctx context.Context) (any, bool, error) {
        return "/login", true, nil
    })

The hook can be dynamic — return different URLs based on request context,
tenant, or other factors.

Hooks
-----

The session module fires the following hooks:

.. list-table::
   :header-rows: 1
   :widths: 25 75

   * - Hook
     - Fired when
   * - ``session.created``
     - A new session is created via ``module.Create``
   * - ``session.loaded``
     - An existing session is loaded from the store
   * - ``session.destroyed``
     - A session is deleted via ``module.Destroy``

These are standard ``HookFunc`` hooks that receive a ``context.Context``.

.. code-block:: go

    k.Hook("session.created", 10, func(ctx context.Context) error {
        // e.g., log new sessions, audit trail
        return nil
    })

Working with OIDC
-----------------

For OIDC authentication, the stateless approach (Option A) is supported out of
the box without the session module. An OIDC middleware validates the token and
stores claims in the request state bag:

.. code-block:: go

    func oidcMiddleware(next request.HandlerFunc) request.HandlerFunc {
        return func(ctx *request.Context) error {
            token := ctx.Request.Header.Get("Authorization")
            claims, err := validateToken(token)
            if err != nil {
                ctx.Writer.WriteHeader(http.StatusUnauthorized)
                return nil
            }
            ctx.SetState("claims", claims)
            return next(ctx)
        }
    }

To add server-side session state on top of OIDC (shopping carts, wizard flows,
per-user preferences), use the session module alongside your OIDC middleware:

.. code-block:: toml

    [[routing.routes]]
    group      = "authenticated"
    path       = "/checkout"
    handler    = "shop.checkout"
    middleware = ["oidc", "session.load"]

The OIDC middleware validates identity; the session module carries application
state. They are independent and complementary.
