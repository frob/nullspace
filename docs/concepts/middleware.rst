Middleware
==========

Nullspace uses the standard Go functional middleware pattern for request
processing. Middleware wraps handlers to add behavior before and after
execution.

Types
-----

.. code-block:: go

    type HandlerFunc func(ctx *Context) error
    type Middleware  func(HandlerFunc) HandlerFunc

A ``HandlerFunc`` receives a framework ``Context`` and returns an error. A
``Middleware`` takes a handler and returns a new handler that wraps it.

Writing Middleware
------------------

.. code-block:: go

    func timing(next request.HandlerFunc) request.HandlerFunc {
        return func(ctx *request.Context) error {
            start := time.Now()
            err := next(ctx)
            ctx.Logger().Info("handled", "duration", time.Since(start))
            return err
        }
    }

    func cors(next request.HandlerFunc) request.HandlerFunc {
        return func(ctx *request.Context) error {
            ctx.Writer.Header().Set("Access-Control-Allow-Origin", "*")
            return next(ctx)
        }
    }

    func auth(next request.HandlerFunc) request.HandlerFunc {
        return func(ctx *request.Context) error {
            token := ctx.Request.Header.Get("Authorization")
            if token == "" {
                ctx.Writer.WriteHeader(http.StatusUnauthorized)
                return fmt.Errorf("unauthorized")
            }
            ctx.SetState("user", validateToken(token))
            return next(ctx)
        }
    }

Global Middleware
-----------------

Global middleware runs on every request:

.. code-block:: go

    adapter.Use(timing)
    adapter.Use(cors)
    adapter.Use(auth)

Middleware is executed in registration order. Given ``[A, B, C]`` and handler
``H``, the call order is::

    A-before -> B-before -> C-before -> H -> C-after -> B-after -> A-after

Route Middleware
-----------------

Route-specific middleware runs only for matching routes:

.. code-block:: go

    router.Post("/api/admin/users", createUser,
        request.WithRouteMiddleware(adminOnly),
    )

Route middleware executes after global middleware. Given global ``[G]``, route
``[R]``, and handler ``H``::

    G-before -> R-before -> H -> R-after -> G-after

Sharing Data
------------

Middleware can pass data to downstream handlers via the context state bag:

.. code-block:: go

    // In middleware
    func userLoader(next request.HandlerFunc) request.HandlerFunc {
        return func(ctx *request.Context) error {
            user := loadUser(ctx.Request)
            ctx.SetState("user", user)
            return next(ctx)
        }
    }

    // In handler
    func myHandler(ctx *request.Context) error {
        user, _ := ctx.State("user")
        // ...
    }

Short-Circuiting
-----------------

Middleware can stop the chain by not calling ``next``:

.. code-block:: go

    func rateLimit(next request.HandlerFunc) request.HandlerFunc {
        return func(ctx *request.Context) error {
            if isRateLimited(ctx.Request) {
                ctx.Writer.WriteHeader(http.StatusTooManyRequests)
                return nil  // don't call next
            }
            return next(ctx)
        }
    }

Middleware vs Hooks
-------------------

Both middleware and hooks can run code before/after request handling. Use
this guide to choose:

============================  ============  ============
Characteristic                Middleware    Hooks
============================  ============  ============
Scope                         Request only  Any lifecycle
Can modify response writer    Yes           No
Can short-circuit request     Yes           No (errors only)
Access to framework Context   Yes           context.Context only
Config-aware disable          Via module    Built-in
Best for                      Auth, CORS    Logging, metrics
============================  ============  ============

Middleware as Modules
---------------------

Middleware can be packaged as modules for configuration and lifecycle
management:

.. code-block:: go

    type CORSModule struct {
        origins []string
    }

    func (m *CORSModule) Name() string { return "middleware.cors" }

    func (m *CORSModule) Init(k *kernel.Kernel) error {
        adapter, _ := kernel.GetResource[*request.Adapter](k, "request.adapter")
        adapter.Use(m.handler)
        return nil
    }

    func (m *CORSModule) handler(next request.HandlerFunc) request.HandlerFunc {
        return func(ctx *request.Context) error {
            ctx.Writer.Header().Set("Access-Control-Allow-Origin",
                strings.Join(m.origins, ", "))
            return next(ctx)
        }
    }

This allows the middleware to be enabled/disabled via config:

.. code-block:: toml

    [modules]
    "middleware.cors" = false
