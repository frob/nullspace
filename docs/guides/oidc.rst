OIDC Authentication
====================

The ``nullspace-oidc`` contributed module adds OpenID Connect authentication
to your application using the Authorization Code + PKCE flow. It provides a
named middleware, automatic login/callback/logout routes, and encrypted cookie
sessions — no server-side session store required.

``nullspace-oidc`` lives in a separate repository and Go module, keeping the
OIDC dependencies (``go-oidc``, ``golang.org/x/oauth2``) out of the core
framework.

Installation
------------

Add the module to your project:

.. code-block:: shell

    go get github.com/frob/nullspace-oidc

Enabling the Module
-------------------

Register the module after the routing module:

.. code-block:: go

    import (
        nsoidc "github.com/frob/nullspace-oidc"
        "github.com/frob/nullspace/core/routing"
    )

    k.Use(routing.New())
    k.Use(nsoidc.New())

Enable it in ``nullspace.toml``:

.. code-block:: toml

    [modules]
    oidc = true

Configuration
-------------

All settings live under the ``[oidc]`` TOML section:

.. code-block:: toml

    [oidc]
    issuer          = "http://localhost:8180/realms/nullspace"
    client_id       = "my-app"
    client_secret   = ""
    scopes          = ["openid", "profile", "email"]
    cookie_name     = "ns_oidc"
    cookie_secret   = "a1b2c3d4e5f6a7b8c9d0e1f2a3b4c5d6"
    cookie_secure   = false
    post_login_url  = "/admin"
    post_logout_url = "/"
    path_prefix     = "/oidc"

============== ====================================== =========================================
Field          Default                                Description
============== ====================================== =========================================
issuer         *(required)*                           OIDC provider issuer URL
client_id      *(required)*                           OAuth2 client ID
client_secret  ``""``                                 Client secret (empty for public + PKCE)
redirect_uri   auto-derived from ``[request] addr``   Callback URL sent to the IDP
scopes         ``["openid", "profile", "email"]``     Scopes to request
cookie_name    ``ns_oidc``                            Encrypted session cookie name
cookie_secret  *(ephemeral if empty)*                 32 or 64 hex chars for AES-GCM key
cookie_secure  ``false``                              Set ``Secure`` flag on cookies
post_login_url ``/``                                  Redirect target after login
post_logout_url ``/``                                 Redirect target after logout
path_prefix    ``/oidc``                              URL prefix for login/callback/logout
============== ====================================== =========================================

.. note::

    When ``redirect_uri`` is omitted, the module derives it automatically from
    the ``[request] addr`` setting. For example, if ``addr = ":8888"``, the
    redirect URI becomes ``http://localhost:8888/oidc/callback``. This means
    you only need to change the port in one place.

.. note::

    If ``cookie_secret`` is empty, the module generates a random ephemeral key
    on startup. Sessions will not survive restarts. Always set a stable key in
    production.

Registered Routes
-----------------

The module automatically registers three routes:

================= ===================================
Path              Purpose
================= ===================================
``/oidc/login``   Redirects to the IDP with PKCE
``/oidc/callback``Handles the IDP's authorization code response
``/oidc/logout``  Clears the session cookie and redirects
================= ===================================

The path prefix is configurable via the ``path_prefix`` setting.

Protecting Routes
-----------------

Apply the ``oidc`` middleware to routes or groups that require authentication:

.. code-block:: toml

    [routing.groups.admin]
    prefix     = "/admin"
    middleware = ["oidc"]

    [[routing.routes]]
    group   = "admin"
    path    = "/dashboard"
    handler = "admin.dashboard"

Unauthenticated requests to these routes are redirected to the OIDC login
endpoint. After successful authentication, the user is redirected back to
``post_login_url``.

Accessing User Info
-------------------

Inside a handler protected by the ``oidc`` middleware, retrieve the
authenticated user with ``oidc.From()``:

.. code-block:: go

    import nsoidc "github.com/frob/nullspace-oidc"

    func adminHandler(ctx *request.Context) error {
        user, ok := nsoidc.From(ctx)
        if !ok {
            // should not happen behind the oidc middleware
            return fmt.Errorf("not authenticated")
        }

        fmt.Println(user.Subject)          // OIDC subject
        fmt.Println(user.Email)            // email claim
        fmt.Println(user.Name)             // full name
        fmt.Println(user.PreferredUsername) // username

        // pass user info to templates, etc.
    }

The middleware sets the ``UserInfo`` in the request state under the key
``oidc.user``. The ``From()`` helper is a typed convenience wrapper.

Token Refresh
-------------

The middleware automatically refreshes expired access tokens using the refresh
token (if provided by the IDP). The updated session cookie is written
transparently. If refresh fails, the user is redirected to login again.

Security
--------

- **PKCE (S256)** is used for all authorization requests, preventing code
  interception attacks.
- **Encrypted cookies** use AES-GCM to protect token data at rest. The nonce
  is random per encryption, so identical tokens produce different ciphertexts.
- **State parameter** is stored in a separate short-lived encrypted cookie (5
  minutes), preventing CSRF attacks during the login flow.
- **ID token verification** uses the IDP's published signing keys (fetched via
  OIDC discovery) to validate token signatures.

Running the Example
-------------------

The repository includes a complete example at ``cmd/examples/oidc/`` with a
public home page and an OIDC-protected admin area that shows a directory
listing.

Prerequisites:

- Docker (for Keycloak)

Start the example:

.. code-block:: shell

    task run:oidc

This starts Keycloak with a pre-configured realm (user ``admin``, password
``admin123``), waits for it to be ready, then launches the application.

Other Taskfile commands:

.. code-block:: shell

    task oidc:up      # Start Keycloak only
    task oidc:down    # Stop Keycloak
    task oidc:reset   # Destroy and recreate Keycloak (reimports realm)

Browse to ``http://localhost:8888``, then click **Admin** to trigger the OIDC
login flow through Keycloak.
