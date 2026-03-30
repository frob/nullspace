Example Modules
===============

The example application at ``cmd/example/`` includes two custom modules that
demonstrate how to extend the framework. These are reference implementations
— study them to understand the patterns, then adapt for your own modules.

Auth Module
-----------

``cmd/example/modules/auth/``

A Basic Auth middleware module that protects routes under a configurable path
prefix.

What It Demonstrates
~~~~~~~~~~~~~~~~~~~~

- **Middleware as a module** — registers global middleware via the service locator
- **Config-driven behavior** — realm, prefix, and credentials from TOML
- **Context state** — passes the authenticated user to downstream handlers
- **Per-module routes** — owns its route definitions via embedded ``routes.toml``
- **Named middleware** — registers ``auth`` middleware on the routing registry
- **Opt-in module** — disabled by default, enabled via ``[modules]``

Configuration
~~~~~~~~~~~~~

.. code-block:: toml

    [modules]
    "auth" = true

    [auth]
    realm = "Nullspace Admin"
    prefix = "/api/admin"

    [auth.users]
    admin = "secret"
    editor = "changeme"

``prefix``
    Only requests whose path starts with this value require authentication.
    All other requests pass through unprotected.

``realm``
    The HTTP Basic Auth realm shown in the browser's login prompt.

``users``
    Maps usernames to passwords. This is plaintext for demonstration — in
    production, use hashed passwords or an external auth provider.

How It Works
~~~~~~~~~~~~

During ``Init()``, the module reads its config, retrieves the request adapter
from the service locator, and registers itself as global middleware:

.. code-block:: go

    func (m *Module) Init(k *kernel.Kernel) error {
        // Read config.
        k.Config().Decode("auth", &m.config)

        // Get the adapter and register middleware.
        adapter, _ := kernel.GetResource[*request.Adapter](k, "request.adapter")
        adapter.Use(m.middleware)
        return nil
    }

The middleware checks whether the request path matches the configured prefix.
Non-matching requests pass through. Matching requests are challenged for
Basic Auth credentials:

.. code-block:: go

    func (m *Module) middleware(next request.HandlerFunc) request.HandlerFunc {
        return func(ctx *request.Context) error {
            if !strings.HasPrefix(ctx.Request.URL.Path, m.config.Prefix) {
                return next(ctx)  // not protected
            }

            username, password, ok := ctx.Request.BasicAuth()
            if !ok || !validCredentials(username, password) {
                // Challenge the client.
                ctx.Writer.Header().Set("WWW-Authenticate", `Basic realm="..."`)
                return unauthorizedResponse(ctx)
            }

            // Set user in state for downstream handlers.
            ctx.SetState("auth.user", username)
            return next(ctx)
        }
    }

Downstream handlers access the authenticated user via the context state bag:

.. code-block:: go

    func adminHandler(ctx *request.Context) error {
        user, _ := ctx.State("auth.user")
        // user == "admin"
    }

Testing
~~~~~~~

.. code-block:: bash

    # No credentials — 401
    curl http://localhost:8080/api/admin/posts
    # {"error":"unauthorized","status":401}

    # With credentials — 200
    curl -u admin:secret http://localhost:8080/api/admin/posts
    # {"posts":[...],"user":"admin"}

    # Non-protected route — no auth needed
    curl http://localhost:8080/api/posts
    # {"posts":[...]}

Module Route File
~~~~~~~~~~~~~~~~~

The auth module owns its route definitions in ``modules/auth/routes.toml``,
embedded into the binary at compile time:

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

The module loads this file and registers the ``auth`` middleware by name
during Init:

.. code-block:: go

    //go:embed routes.toml
    var routesData []byte

    func (m *Module) Init(k *kernel.Kernel) error {
        // ... config, middleware setup ...

        // Register middleware by name for TOML route references.
        reg, _ := kernel.GetResource[*routing.Registry](k, "routing.registry")
        reg.Middleware("auth", m.middleware)

        // Load this module's routes.
        routingMod, _ := kernel.GetResource[*routing.Module](k, "routing")
        routingMod.LoadRoutes(routesData)

        return nil
    }

This means the admin route and its ``auth`` middleware requirement are
defined alongside the auth module code, not in the project's main config.

Key Patterns
~~~~~~~~~~~~

1. **Service locator for wiring** — the module doesn't import the adapter
   directly. It retrieves it by key during Init, keeping coupling loose.

2. **Path-scoped protection** — instead of tagging individual routes, the
   module uses a prefix to protect a whole section. This is simpler and
   doesn't require route-level cooperation.

3. **State bag for identity** — ``ctx.SetState("auth.user", username)``
   lets any handler check who's authenticated without importing the auth
   module.

4. **Constant-time comparison** — credentials are compared using
   ``crypto/subtle`` to prevent timing attacks.

5. **Per-module route file** — the admin routes live in
   ``modules/auth/routes.toml``, embedded in the binary. The module owns
   its routes instead of relying on the project's central config.

Forms Module
------------

``cmd/example/modules/forms/``

A configurable webform handler that defines forms declaratively in TOML,
renders them as HTML, validates submissions, stores them via the file data
module, and fires hooks for extensibility.

What It Demonstrates
~~~~~~~~~~~~~~~~~~~~

- **Route registration via service locator** — the module registers its own routes
- **Per-module route file** — owns submission API routes via embedded ``routes.toml``
- **Request body parsing** — handles both form-encoded and JSON submissions
- **Data layer integration** — stores submissions using the file module
- **Custom hook points** — fires ``form.before_submit`` and ``form.after_submit``
- **Response pipeline** — uses formatters for both HTML and JSON responses
- **Validation** — field-level validation with error feedback
- **Config-driven** — forms defined entirely in TOML

Configuration
~~~~~~~~~~~~~

.. code-block:: toml

    [modules]
    "forms" = true

    [forms.forms.contact]
    title = "Contact Us"
    success_message = "Thanks for reaching out!"
    store = "form-submissions"

    [[forms.forms.contact.fields]]
    name = "name"
    label = "Your Name"
    type = "text"
    required = true

    [[forms.forms.contact.fields]]
    name = "email"
    label = "Email Address"
    type = "email"
    required = true

    [[forms.forms.contact.fields]]
    name = "subject"
    label = "Subject"
    type = "select"
    required = true
    options = ["General Inquiry", "Bug Report", "Feature Request"]

    [[forms.forms.contact.fields]]
    name = "message"
    label = "Message"
    type = "textarea"
    required = true

``forms.forms.<name>``
    Each key under ``forms.forms`` defines a form. The key becomes the URL
    slug: ``contact`` creates routes at ``/forms/contact``.

``store``
    The file data collection name for persisting submissions. If empty,
    submissions are not stored. The file module saves each submission as
    a JSON file in ``content/<store>/``.

``fields``
    An ordered list of field definitions. Supported types: ``text``,
    ``email``, ``textarea``, ``select``, ``hidden``.

How It Works
~~~~~~~~~~~~

During ``Init()``, the module reads its config, retrieves the router and
response pipeline from the service locator, and registers routes for each
form:

.. code-block:: go

    func (m *Module) Init(k *kernel.Kernel) error {
        k.Config().Decode("forms", &m.config)

        m.pipeline, _ = kernel.GetResource[*response.Pipeline](k, "response.pipeline")
        m.fileMod, _  = kernel.GetResource[*file.Module](k, "data.file")
        router, _     = kernel.GetResource[*request.Router](k, "router")

        for name, def := range m.config.Forms {
            router.Get("/forms/"+name, renderHandler)
            router.Post("/forms/"+name, submitHandler)
            router.Get("/api/forms/"+name+"/submissions", listHandler)
        }
        return nil
    }

Each form gets three routes:

====================================  =======  =================================
Route                                 Method   Purpose
====================================  =======  =================================
``/forms/<name>``                     GET      Render the HTML form
``/forms/<name>``                     POST     Handle submission
``/api/forms/<name>/submissions``     GET      List stored submissions (JSON)
====================================  =======  =================================

Submission Flow
~~~~~~~~~~~~~~~

When a form is submitted:

1. **Parse body** — detects ``Content-Type`` and parses either form-encoded or JSON
2. **Validate** — checks required fields, collects errors
3. **If invalid** — re-renders the form with errors (HTML) or returns 422 (JSON)
4. **Fire** ``form.before_submit`` **hook** — other modules can reject or modify
5. **Store** — saves submission via the file data module as a JSON entity
6. **Fire** ``form.after_submit`` **hook** — other modules can send email, etc.
7. **Respond** — redirect, success template (HTML), or JSON success

The module accepts both form-encoded and JSON bodies on the same endpoint:

.. code-block:: bash

    # HTML form (browser)
    curl -X POST http://localhost:8080/forms/contact \
      -d "name=Alice&email=alice@example.com&subject=General+Inquiry&message=Hello"

    # JSON API
    curl -X POST http://localhost:8080/forms/contact \
      -H 'Content-Type: application/json' \
      -H 'Accept: application/json' \
      -d '{"name":"Alice","email":"alice@example.com","subject":"General Inquiry","message":"Hello"}'

Custom Hooks
~~~~~~~~~~~~

The forms module fires two hook points that other modules can use:

``form.before_submit``
    Fires after validation passes, before storage. Returning an error rejects
    the submission. Use for spam filtering, rate limiting, or additional
    validation.

``form.after_submit``
    Fires after successful storage. Use for sending email notifications,
    updating caches, or triggering external integrations.

Hook handlers can read the form name and submitted values from context:

.. code-block:: go

    func (m *NotifyModule) Init(k *kernel.Kernel) error {
        k.Hook("form.after_submit", 50, m.onSubmit)
        return nil
    }

    func (m *NotifyModule) onSubmit(ctx context.Context) error {
        name := forms.FormNameFromContext(ctx)
        values := forms.FormValuesFromContext(ctx)

        if name == "contact" {
            email := values["email"].(string)
            // send confirmation email...
        }
        return nil
    }

Stored Submissions
~~~~~~~~~~~~~~~~~~

When ``store`` is configured, submissions are saved as JSON files via the
file data module:

.. code-block:: text

    content/form-submissions/contact-1706400000000.json

Each file contains the submitted fields plus metadata:

.. code-block:: json

    {
        "email": "alice@example.com",
        "form": "contact",
        "message": "Hello!",
        "name": "Alice",
        "subject": "General Inquiry",
        "timestamp": "2025-01-28T10:00:00-07:00"
    }

Query stored submissions via the API:

.. code-block:: bash

    curl http://localhost:8080/api/form-submissions?pretty=true

Module Route File
~~~~~~~~~~~~~~~~~

The forms module owns its submission API routes via
``modules/forms/routes.toml``:

.. code-block:: toml

    # modules/forms/routes.toml

    # Auto-generate CRUD API routes for all stored submissions.
    [[collections]]
    name = "form-submissions"
    source = "data.file"
    api_prefix = "/api"

This generates ``/api/form-submissions`` and ``/api/form-submissions/:id``
endpoints. The form GET/POST routes (``/forms/contact``) remain code-generated
since they depend on the dynamic form definitions in config.

Templates
~~~~~~~~~

The forms module uses two templates:

``form.html``
    Renders the form. Receives ``FormName``, ``Title``, ``Fields``,
    ``Values`` (for re-population), ``Errors`` (per-field), and ``Action``
    (POST URL).

``form_success.html``
    Shown after successful submission. Receives ``FormName``, ``Title``,
    and ``Message``.

Both templates are in the standard ``templates/`` directory and can be
customized per-project.

Adding a New Form
~~~~~~~~~~~~~~~~~

Add another form by extending the TOML config — no code changes needed:

.. code-block:: toml

    [forms.forms.feedback]
    title = "Product Feedback"
    success_message = "Thank you for your feedback!"
    store = "form-submissions"

    [[forms.forms.feedback.fields]]
    name = "rating"
    label = "Rating"
    type = "select"
    required = true
    options = ["1 - Poor", "2 - Fair", "3 - Good", "4 - Great", "5 - Excellent"]

    [[forms.forms.feedback.fields]]
    name = "comments"
    label = "Comments"
    type = "textarea"
    required = false

Restart the server and visit ``/forms/feedback``.

Key Patterns
~~~~~~~~~~~~

1. **Declarative over imperative** — forms are defined in config, not code.
   Adding a form requires zero Go changes.

2. **Dual content type** — the same POST endpoint handles both browser
   form submissions and JSON API calls, choosing the response format
   accordingly.

3. **Hook-based extensibility** — instead of hardcoding email sending or
   spam filtering, the module fires hooks that other modules subscribe to.
   This keeps the forms module focused and composable.

4. **Data module reuse** — submissions are stored through the existing file
   data module rather than implementing custom persistence. The file module's
   data hooks (``data.before_write``, etc.) apply to form storage too.

5. **Service locator for loose coupling** — the module retrieves the router,
   pipeline, and file module by key. It doesn't import them as hard
   dependencies — if the file module isn't available, forms still work
   (they just don't persist).
