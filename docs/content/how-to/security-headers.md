---
title: Set security headers
weight: 15
---

Use this guide when you want HSTS, CSP, frame protection, and friends applied
to every response.

## Solution

Enable the `http-security` module. Once registered, every response is
augmented with the configured security headers.

```toml
[modules]
http-security = true

[http-security]
hsts               = true
hsts_max_age       = 31536000         # 1 year
hsts_include_subs  = true
hsts_preload       = false
csp                = "default-src 'self'"
frame_options      = "DENY"
referrer_policy    = "strict-origin-when-cross-origin"
permissions_policy = "geolocation=(), camera=()"
```

Register the module after the routing module:

```go
k.Use(routing.New())
k.Use(httpsecurity.New())
```

## Headers applied

| Header                        | When                              |
| ----------------------------- | --------------------------------- |
| `X-Content-Type-Options`      | Always (`nosniff`)                |
| `Strict-Transport-Security`   | `hsts = true`                     |
| `Content-Security-Policy`     | `csp` non-empty                   |
| `X-Frame-Options`             | Default `DENY`                    |
| `Referrer-Policy`             | Default `strict-origin-when-cross-origin` |
| `Permissions-Policy`          | `permissions_policy` non-empty    |

## Variations

### Opt a route out

Set `http-security = "none"` in the route's metadata to skip the headers (for
example, on a health check probed by a load balancer):

```toml
[[routing.routes]]
path    = "/health"
handler = "health.check"

[routing.routes.meta]
http-security = "none"
```

### Force HTTPS

Add `https_redirect = "true"` to a route. HTTP requests are redirected with
`301 Moved Permanently`. The middleware detects TLS via the native connection
state or `X-Forwarded-Proto`, so it works behind a proxy.

```toml
[[routing.routes]]
path           = "/account"
handler        = "account.dashboard"
https_redirect = "true"
```

### Different CSP per environment

Override the CSP from the environment:

```bash
NULLSPACE_HTTP_SECURITY_CSP="default-src 'self'; script-src 'self' cdn.example.com"
```

## See also

- [Add CSRF protection]({{< relref "csrf" >}})
- [Deploy with Docker]({{< relref "deploy-docker" >}})
