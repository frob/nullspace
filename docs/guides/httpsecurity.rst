HTTP Security
=============

The ``http-security`` module adds security response headers, HTTPS redirect,
and CSRF protection to your application. All features are configurable via
TOML and activated through route directives.

Enabling the Module
-------------------

Register the module after the request adapter and routing modules:

.. code-block:: go

    import "github.com/frob/nullspace/module/httpsecurity"

    k.Use(request.NewAdapter())
    k.Use(routing.New())
    k.Use(httpsecurity.New())

Enable it in ``nullspace.toml``:

.. code-block:: toml

    [modules]
    http-security = true

Configuration
-------------

All settings live under the ``[http-security]`` TOML section. Every field has
a sensible default; you only need to set the values you want to change.

.. code-block:: toml

    [http-security]
    hsts               = true
    hsts_max_age       = 31536000       # 1 year
    hsts_include_subs  = true
    hsts_preload       = false
    csp                = "default-src 'self'"
    frame_options      = "DENY"
    referrer_policy    = "strict-origin-when-cross-origin"
    permissions_policy = "geolocation=(), camera=()"
    csrf_cookie        = "ns_csrf"
    csrf_header        = "X-CSRF-Token"
    csrf_field         = "csrf_token"
    csrf_secure        = true
    csrf_path          = "/"

Security Headers
----------------

Once the module is registered, every response automatically receives:

- ``X-Content-Type-Options: nosniff`` (always)
- ``Strict-Transport-Security`` (when ``hsts = true``)
- ``Content-Security-Policy`` (when ``csp`` is set)
- ``X-Frame-Options`` (default ``DENY``)
- ``Referrer-Policy`` (default ``strict-origin-when-cross-origin``)
- ``Permissions-Policy`` (when ``permissions_policy`` is set)

To disable headers on a specific route (e.g. an API health check), use the
route metadata:

.. code-block:: toml

    [[routing.routes]]
    path    = "/health"
    handler = "health.check"

    [routing.routes.meta]
    http-security = "none"

HTTPS Redirect
--------------

Enable per-route HTTPS redirect with the ``https_redirect`` directive:

.. code-block:: toml

    [[routing.routes]]
    path           = "/account"
    handler        = "account.dashboard"
    https_redirect = "true"

HTTP requests are redirected to HTTPS with a ``301 Moved Permanently`` status.
The middleware detects TLS via the native connection state or the
``X-Forwarded-Proto`` header, so it works behind reverse proxies.

CSRF Protection
---------------

Enable CSRF validation on individual routes:

.. code-block:: toml

    [[routing.routes]]
    path    = "/login"
    handler = "auth.login"
    methods = ["GET", "POST"]
    csrf    = "true"

How it works
~~~~~~~~~~~~

The module uses the **double-submit cookie** pattern, which requires no
server-side session state:

1. On ``GET`` (or other safe methods), the middleware generates a random token,
   sets it in a cookie (``ns_csrf``), and stores it in the request state.
2. Your template or JavaScript reads the token and includes it in subsequent
   form submissions or AJAX requests.
3. On ``POST`` (or other state-changing methods), the middleware compares the
   cookie value against the submitted token. If they don't match, the request
   is rejected with ``403 Forbidden``.

Using the token in templates
~~~~~~~~~~~~~~~~~~~~~~~~~~~~

Retrieve the token inside a handler:

.. code-block:: go

    func loginHandler(ctx *request.Context) error {
        token := httpsecurity.CSRFToken(ctx)
        // pass token to your template
    }

Include it as a hidden form field:

.. code-block:: html

    <form method="POST" action="/login">
        <input type="hidden" name="csrf_token" value="{{.CSRFToken}}">
        <!-- other fields -->
    </form>

Using the token in JavaScript
~~~~~~~~~~~~~~~~~~~~~~~~~~~~~

For AJAX requests, read the token from the cookie and send it as a header:

.. code-block:: javascript

    fetch("/api/action", {
        method: "POST",
        headers: {
            "X-CSRF-Token": getCookie("ns_csrf"),
        },
    });

The cookie has ``HttpOnly: false`` specifically so JavaScript can read it.
``SameSite: Strict`` provides additional cross-origin protection.

Combining Features
------------------

Route directives can be combined freely:

.. code-block:: toml

    [[routing.routes]]
    path           = "/settings"
    handler        = "settings.update"
    methods        = ["GET", "POST"]
    csrf           = "true"
    https_redirect = "true"

This route will redirect HTTP to HTTPS, apply security headers, and enforce
CSRF validation on POST requests.
