Routing
=======

Nullspace supports three ways to define routes, all usable together:

1. **TOML routes** — declarative, in ``nullspace.toml`` or per-module ``routes.toml``
2. **TOML collections** — auto-generated CRUD routes from a single declaration
3. **Go routes** — programmatic, via the router API

TOML Routes
-----------

Define routes in the ``[routing]`` section of ``nullspace.toml``:

.. code-block:: toml

    # Named groups for shared settings.
    [routing.groups.api]
    prefix = "/api"
    format = "json"

    [routing.groups.pages]
    prefix = ""
    format = "html"

    # Individual routes reference groups by name.
    [[routing.routes]]
    group = "api"
    path = "/health"
    handler = "health.check"

    [[routing.routes]]
    group = "pages"
    path = "/"
    handler = "template"
    template = "home.html"

Route Fields
~~~~~~~~~~~~

============================  ==========================================================
Field                         Description
============================  ==========================================================
``path``                      URL pattern (e.g., ``/posts/:id``)
``handler``                   Named handler (e.g., ``data.list``, ``redirect``)
``group``                     Named group for shared prefix/format/middleware
``methods``                   HTTP methods (defaults to ``["GET"]``)
``format``                    Response format (overrides group)
``template``                  Template name for HTML rendering
``middleware``                Additional named middleware (added to group middleware)
``collection``                Data collection for built-in handlers
``data_param``                Route param for entity lookup (defaults to ``"id"``)
``redirect``                  Target URL for the ``redirect`` handler
``status_code``               HTTP status for redirects (defaults to 303)
============================  ==========================================================

Route Groups
~~~~~~~~~~~~

Groups define shared settings applied to all routes that reference them:

.. code-block:: toml

    [routing.groups.admin]
    prefix = "/api/admin"
    format = "json"
    middleware = ["auth"]

    [[routing.routes]]
    group = "admin"
    path = "/users"           # becomes /api/admin/users
    handler = "users.list"    # inherits json format + auth middleware

A route's own ``format`` and ``middleware`` override or extend the group
settings.

Collections
-----------

A collection auto-generates CRUD routes for a data source:

.. code-block:: toml

    [[routing.collections]]
    name = "posts"
    source = "data.file"
    api_prefix = "/api"
    html_prefix = ""
    list_template = "posts.html"
    item_template = "post.html"
    write_middleware = ["auth"]

This single declaration generates:

==========  ============================  ================  ==========
Method      Path                          Handler           Middleware
==========  ============================  ================  ==========
GET         ``/api/posts``                ``data.list``
GET         ``/api/posts/:id``            ``data.get``
POST        ``/api/posts``                ``data.create``   auth
PUT         ``/api/posts/:id``            ``data.update``   auth
DELETE      ``/api/posts/:id``            ``data.delete``   auth
GET         ``/posts``                    ``data.list``
GET         ``/posts/:id``                ``data.get``
==========  ============================  ================  ==========

API routes use JSON; HTML routes use the specified templates.
``write_middleware`` applies only to POST/PUT/DELETE.
``read_middleware`` applies only to GET.

Per-Module Route Files
-----------------------

Modules can own their route definitions via an embedded ``routes.toml``.
This keeps routes co-located with the module code and compiled into the
binary:

.. code-block:: go

    //go:embed routes.toml
    var routesData []byte

    func (m *Module) Init(k *kernel.Kernel) error {
        routingMod, _ := kernel.GetResource[*routing.Module](k, "routing")
        routingMod.LoadRoutes(routesData)
        return nil
    }

The module's ``routes.toml`` uses the same format as the ``[routing]``
section in ``nullspace.toml``:

.. code-block:: toml

    # modules/auth/routes.toml

    [groups.admin]
    prefix = "/api/admin"
    format = "json"
    middleware = ["auth"]

    [[routes]]
    group = "admin"
    path = "/posts"
    handler = "data.list"
    collection = "posts"

Routes from all sources (``nullspace.toml``, module ``routes.toml``, and Go
code) are merged at startup. Duplicates are detected by path + handler and
silently deduplicated.

Built-in Handlers
-----------------

These handlers are available without registration:

==============================  ================================================
Handler                         Description
==============================  ================================================
``data.list``                   List entities from a collection
``data.get``                    Get a single entity by route param (default ``:id``)
``data.create``                 Create entity from JSON or form body
``data.update``                 Update entity from JSON or form body
``data.delete``                 Delete entity by route param
``template``                    Render a template with no data fetching
``redirect``                    HTTP redirect (set ``redirect`` and ``status_code``)
==============================  ================================================

Example using built-in handlers:

.. code-block:: toml

    # Render a template.
    [[routing.routes]]
    path = "/"
    handler = "template"
    format = "html"
    template = "home.html"

    # Redirect.
    [[routing.routes]]
    path = "/blog"
    handler = "redirect"
    redirect = "/posts"

    # Data list with explicit collection.
    [[routing.routes]]
    path = "/api/authors"
    handler = "data.list"
    format = "json"
    collection = "authors"

Named Handlers and Middleware
------------------------------

Custom handlers and middleware are registered by name on the routing
registry, then referenced in TOML:

.. code-block:: go

    func (m *Module) Init(k *kernel.Kernel) error {
        reg, _ := kernel.GetResource[*routing.Registry](k, "routing.registry")

        // Register a handler.
        reg.HandleFunc("health.check", func(ctx *request.Context) error {
            resp := response.NewResponse(http.StatusOK, map[string]string{"status": "ok"})
            return pipeline.Write(ctx.Context(), ctx.Writer, resp)
        })

        // Register middleware.
        reg.Middleware("auth", myAuthMiddleware)

        return nil
    }

.. code-block:: toml

    [[routing.routes]]
    path = "/api/health"
    handler = "health.check"
    format = "json"

    [[routing.routes]]
    path = "/api/admin/data"
    handler = "data.list"
    middleware = ["auth"]
    collection = "data"

Data Injection
--------------

When a TOML route specifies a ``collection`` and uses a custom (non-built-in)
handler, the routing module automatically injects data into the request
context before the handler runs:

- For routes with a path parameter: the entity is loaded and stored in
  ``ctx.State("data.entity")``
- For routes without a parameter: the entity list is loaded and stored in
  ``ctx.State("data.entities")``

.. code-block:: toml

    [[routing.routes]]
    path = "/custom/:id"
    handler = "my.handler"
    collection = "posts"

.. code-block:: go

    reg.HandleFunc("my.handler", func(ctx *request.Context) error {
        entity, _ := ctx.State("data.entity")
        // entity is pre-loaded from the "posts" collection
    })

Go Routes
---------

Routes defined in Go code work alongside TOML routes:

.. code-block:: go

    router := adapter.Router()

    router.Get("/api/posts", listPosts)
    router.Get("/api/posts/:id", getPost)
    router.Post("/api/posts", createPost,
        request.WithMeta("format", "json"),
        request.WithRouteMiddleware(authMiddleware),
    )

Path Parameters
~~~~~~~~~~~~~~~

Segments starting with ``:`` capture path parameters:

.. code-block:: go

    router.Get("/posts/:id", func(ctx *request.Context) error {
        id := ctx.Param("id")
    })

Route Metadata
~~~~~~~~~~~~~~

.. code-block:: go

    router.Get("/api/data", handler, request.WithMeta("format", "json"))

Per-Route Middleware
~~~~~~~~~~~~~~~~~~~~

.. code-block:: go

    router.Post("/admin/users", handler, request.WithRouteMiddleware(adminOnly))

Route Precedence
-----------------

1. **Dynamic routes** — checked first (both TOML and Go routes)
2. **Fallback handlers** — static files
3. **404** — nothing matched

Dynamic routes always take precedence over static files.

Route Table
-----------

List all registered routes with the ``routes`` command::

    $ nullspace routes

    METHOD  PATH                 HANDLER       FORMAT  MIDDLEWARE  TEMPLATE
    ------  ----                 -------       ------  ----------  --------
    GET     /                    template      html                home.html
    GET     /api/admin/posts     data.list     json    auth
    GET     /api/health          health.check  json
    GET     /api/posts           data.list     json
    POST    /api/posts           data.create   json    auth
    GET     /posts               data.list     html                posts.html
    GET     /posts/:id           data.get      html                post.html

This shows TOML-defined routes. Go-defined routes also appear in the router
but are not tracked in the route table.
