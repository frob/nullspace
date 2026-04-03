OIDC
====

.. code-block:: go

    import nsoidc "github.com/frob/nullspace-oidc"

The ``nullspace-oidc`` package provides OIDC authentication as a contributed
module. It implements the Authorization Code + PKCE flow with encrypted cookie
sessions.

Module
------

.. code-block:: go

    func New() *Module

Creates a new OIDC module. Register it with ``k.Use(nsoidc.New())``.

The module implements ``kernel.Module`` and ``kernel.Configurable``.

- **Name**: ``oidc``
- **Config key**: ``oidc``
- **Default enabled**: ``false``
- **Provided resource**: ``"oidc"`` (``*Module``)

Lifecycle:

- **Init**: Reads config, initializes encryption, registers middleware
  (``"oidc"``), handlers (``"oidc.login"``, ``"oidc.callback"``,
  ``"oidc.logout"``), and loads embedded routes.
- **Start**: Performs OIDC discovery against the configured issuer. Returns an
  error if the IDP is unreachable.
- **Stop**: No-op.

Config
------

.. code-block:: go

    type Config struct {
        Issuer        string   `toml:"issuer"`
        ClientID      string   `toml:"client_id"`
        ClientSecret  string   `toml:"client_secret"`
        RedirectURI   string   `toml:"redirect_uri"`
        Scopes        []string `toml:"scopes"`
        CookieName    string   `toml:"cookie_name"`
        CookieSecret  string   `toml:"cookie_secret"`
        CookieSecure  bool     `toml:"cookie_secure"`
        PostLoginURL  string   `toml:"post_login_url"`
        PostLogoutURL string   `toml:"post_logout_url"`
        PathPrefix    string   `toml:"path_prefix"`
    }

See :doc:`../guides/oidc` for field descriptions and defaults.

UserInfo
--------

.. code-block:: go

    type UserInfo struct {
        Subject           string `json:"sub"`
        Email             string `json:"email"`
        EmailVerified     bool   `json:"email_verified"`
        Name              string `json:"name"`
        PreferredUsername string `json:"preferred_username"`
    }

Claims extracted from the OIDC ID token. Stored in the request state under
key ``oidc.user`` by the middleware.

From
----

.. code-block:: go

    func From(ctx *request.Context) (*UserInfo, bool)

Retrieves the authenticated user from the request context. Returns ``nil,
false`` if the request is not authenticated via the OIDC middleware.

Middleware
----------

The module registers a named middleware ``"oidc"`` on the routing registry.
Apply it to routes via TOML:

.. code-block:: toml

    [[routing.routes]]
    path       = "/admin"
    handler    = "admin.page"
    middleware = ["oidc"]

The middleware:

1. Reads and decrypts the session cookie.
2. Refreshes expired tokens if a refresh token is available.
3. Verifies the ID token and extracts ``UserInfo``.
4. Sets ``ctx.SetState("oidc.user", userInfo)`` for downstream handlers.
5. Redirects to the login endpoint on failure.

Handlers
--------

Three handlers are registered on the routing registry:

``oidc.login``
    Generates PKCE verifier and state, stores them in an encrypted cookie,
    and redirects to the IDP's authorization endpoint.

``oidc.callback``
    Validates the state parameter, exchanges the authorization code for
    tokens (with PKCE verifier), verifies the ID token, and sets the
    encrypted session cookie.

``oidc.logout``
    Clears the session cookie and redirects to ``post_logout_url``.
