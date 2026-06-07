---
title: Add OIDC authentication
weight: 16
---

Use this guide when you want users to sign in through an OpenID Connect
provider (Keycloak, Auth0, Google, etc.) and gate routes behind that login.

## Solution

Add the `nullspace-oidc` contrib module to your project, configure the
issuer, and apply the `oidc` middleware to protected routes.

```bash
go get github.com/frob/nullspace-oidc
```

```go
import (
    nsoidc "github.com/frob/nullspace-oidc"
    "github.com/frob/nullspace/core/routing"
)

k.Use(routing.New())
k.Use(nsoidc.New())
```

```toml
[modules]
oidc = true

[oidc]
issuer        = "http://localhost:8180/realms/myrealm"
client_id     = "my-app"
client_secret = ""                                         # empty = public + PKCE
scopes        = ["openid", "profile", "email"]
cookie_secret = "a1b2c3d4e5f6a7b8c9d0e1f2a3b4c5d6"          # 32 or 64 hex chars
cookie_secure = true
post_login_url  = "/admin"
post_logout_url = "/"
```

Three routes register automatically:

| Path             | Purpose                                 |
| ---------------- | --------------------------------------- |
| `/oidc/login`    | Redirects to the IDP with PKCE          |
| `/oidc/callback` | Handles the authorization code response |
| `/oidc/logout`   | Clears the session cookie and redirects |

Protect routes by applying the `oidc` middleware:

```toml
[routing.groups.admin]
prefix     = "/admin"
middleware = ["oidc"]

[[routing.routes]]
group   = "admin"
path    = "/dashboard"
handler = "admin.dashboard"
```

Unauthenticated requests are redirected to `/oidc/login`. After a successful
login, the user lands on `post_login_url`.

## Access user info

```go
import nsoidc "github.com/frob/nullspace-oidc"

func dashboard(ctx *request.Context) error {
    user, ok := nsoidc.From(ctx)
    if !ok {
        return fmt.Errorf("not authenticated")
    }
    // user.Subject, user.Email, user.Name, user.PreferredUsername
    return nil
}
```

## Variations

### Confidential client

Provide a `client_secret` instead of relying on PKCE alone:

```toml
[oidc]
client_id     = "my-app"
client_secret = "shhhh"
```

### Custom redirect URI

By default the module derives `redirect_uri` from `[request] addr` plus
`/oidc/callback`. Override it when running behind a proxy:

```toml
[oidc]
redirect_uri = "https://app.example.com/oidc/callback"
```

### Combine with the session module

OIDC carries identity; the `session` module carries application state
(shopping cart, wizard step). Apply both middlewares:

```toml
[[routing.routes]]
group      = "authenticated"
path       = "/checkout"
handler    = "shop.checkout"
middleware = ["oidc", "session.load"]
```

## See also

- [Add CSRF protection]({{< relref "csrf" >}})
- [Hook into the request lifecycle]({{< relref "hooks" >}})
