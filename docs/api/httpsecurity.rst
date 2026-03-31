httpsecurity
============

``import "github.com/frob/nullspace/module/httpsecurity"``

The httpsecurity package provides HTTP security middleware for the nullspace
framework. It is an opt-in module (``DefaultEnabled: false``) that applies
security response headers globally and provides CSRF protection and HTTPS
redirect via route directives.

Types
-----

Config
~~~~~~

.. code-block:: go

    type Config struct {
        HSTS              bool   // enable Strict-Transport-Security
        HSTSMaxAge        int    // max-age in seconds, default 31536000
        HSTSIncludeSubs   bool   // include subdomains in HSTS
        HSTSPreload       bool   // add preload directive
        CSP               string // Content-Security-Policy value
        FrameOptions      string // X-Frame-Options, default "DENY"
        ReferrerPolicy    string // Referrer-Policy, default "strict-origin-when-cross-origin"
        PermissionsPolicy string // Permissions-Policy value
        CsrfCookie        string // CSRF cookie name, default "ns_csrf"
        CsrfHeader        string // CSRF header name, default "X-CSRF-Token"
        CsrfField         string // CSRF form field name, default "csrf_token"
        CsrfSecure        bool   // Secure attribute on CSRF cookie
        CsrfPath          string // cookie path, default "/"
    }

TOML key: ``[http-security]``

Module
------

.. code-block:: go

    func New() *Module

    func (m *Module) Name() string  // "http-security"

Creates a new http-security module. During ``Init``, the module:

1. Decodes configuration from the ``[http-security]`` TOML section.
2. Registers global middleware on the request adapter (headers, redirect, CSRF).
3. Registers three named middleware on the routing registry.
4. Provides itself on the service locator as ``"http-security"``.

Context Helper
--------------

.. code-block:: go

    func CSRFToken(ctx *request.Context) string

Retrieves the CSRF token from the request state bag. Returns an empty string
if CSRF protection is not enabled for the route. Use inside handlers or
templates after the CSRF middleware has run.

Middleware
----------

Three named middleware entries are registered on the routing registry during
``Init``:

.. list-table::
   :header-rows: 1
   :widths: 25 75

   * - Name
     - Behavior
   * - ``security.headers``
     - Sets security response headers on all responses. Routes opt out with
       ``http-security = "none"`` in their metadata.
   * - ``security.redirect``
     - Redirects HTTP to HTTPS (301). Activates on routes with
       ``https_redirect = "true"``.
   * - ``security.csrf``
     - Enforces double-submit cookie CSRF validation on state-changing methods
       (POST, PUT, PATCH, DELETE). Activates on routes with ``csrf = "true"``.

All three are also registered as global middleware on the request adapter, so
they run on every request. Each middleware checks its own route directive to
decide whether to activate.

Security Headers
~~~~~~~~~~~~~~~~

The headers middleware always sets ``X-Content-Type-Options: nosniff``. The
remaining headers are set only when their corresponding config value is
non-empty or enabled:

.. list-table::
   :header-rows: 1
   :widths: 35 65

   * - Header
     - Config field
   * - ``Strict-Transport-Security``
     - ``hsts = true`` (with ``hsts_max_age``, ``hsts_include_subs``, ``hsts_preload``)
   * - ``Content-Security-Policy``
     - ``csp``
   * - ``X-Frame-Options``
     - ``frame_options`` (default ``"DENY"``)
   * - ``Referrer-Policy``
     - ``referrer_policy`` (default ``"strict-origin-when-cross-origin"``)
   * - ``Permissions-Policy``
     - ``permissions_policy``

HTTPS Redirect
~~~~~~~~~~~~~~

Detects TLS via the native ``Request.TLS`` state or the ``X-Forwarded-Proto:
https`` header (for reverse proxies). Preserves the full request URI including
query string during redirect.

CSRF Protection
~~~~~~~~~~~~~~~

Uses the double-submit cookie pattern (no server-side session state required):

- **Safe methods** (GET, HEAD, OPTIONS, TRACE): generates a 32-byte random
  token, stores it in a cookie and makes it available via
  ``ctx.State("csrf_token")``.
- **State-changing methods** (POST, PUT, PATCH, DELETE): validates the
  submitted token against the cookie. The token may be submitted via the
  configured HTTP header (default ``X-CSRF-Token``) or form field (default
  ``csrf_token``). Tokens are compared in constant time. Returns
  ``403 Forbidden`` on mismatch.

The CSRF cookie has ``HttpOnly: false`` so JavaScript can read it for AJAX
requests, and ``SameSite: Strict`` for additional protection.

Route Directives
-----------------

.. code-block:: toml

    [[routing.routes]]
    path           = "/login"
    handler        = "auth.login"
    methods        = ["GET", "POST"]
    csrf           = "true"
    https_redirect = "true"

.. list-table::
   :header-rows: 1
   :widths: 25 75

   * - Directive
     - Effect
   * - ``csrf = "true"``
     - Enables CSRF token validation on this route.
   * - ``https_redirect = "true"``
     - Redirects HTTP requests to HTTPS on this route.
   * - ``http-security = "none"``
     - Disables all security response headers on this route.

Service Locator
---------------

After ``Init``, the following resource is available:

.. list-table::
   :header-rows: 1
   :widths: 25 30 45

   * - Key
     - Type
     - Description
   * - ``http-security``
     - ``*httpsecurity.Module``
     - The http-security module instance

.. code-block:: go

    mod, err := kernel.GetResource[*httpsecurity.Module](k, "http-security")

Registration Order
------------------

The http-security module must be registered **after** both the request adapter
and routing modules, which provide the ``request.adapter`` and
``routing.registry`` resources:

.. code-block:: go

    k.Use(nslog.New())
    k.Use(request.NewAdapter())   // must come before http-security
    k.Use(routing.New())          // must come before http-security
    k.Use(httpsecurity.New())
