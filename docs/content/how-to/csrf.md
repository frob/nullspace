---
title: Add CSRF protection
weight: 14
---

Use this guide when a state-changing route (form POST, AJAX call) needs CSRF
validation.

## Solution

Enable the `http-security` module and mark the route with `csrf = "true"`.

```toml
[modules]
http-security = true

[[routing.routes]]
path    = "/login"
handler = "auth.login"
methods = ["GET", "POST"]
csrf    = "true"
```

The module uses the **double-submit cookie** pattern: a random token is
issued in a cookie on safe requests, and the same value must be echoed in
the request body or header on state-changing requests.

## Render the token in a form

Get the token inside a handler and pass it to your template:

```go
import "github.com/frob/nullspace/module/httpsecurity"

func login(ctx *request.Context) error {
    token := httpsecurity.CSRFToken(ctx)
    resp := response.NewResponse(200, map[string]any{
        "CSRFToken": token,
    }).WithTemplate("login.html")
    return pipeline.Write(ctx.Context(), ctx.Writer, resp)
}
```

```html
<form method="POST" action="/login">
    <input type="hidden" name="csrf_token" value="{{.CSRFToken}}">
    <!-- other fields -->
</form>
```

## Send the token from JavaScript

The cookie is readable from JavaScript (`HttpOnly: false`); send it as an
`X-CSRF-Token` header:

```javascript
fetch("/api/action", {
    method: "POST",
    headers: {
        "X-CSRF-Token": getCookie("ns_csrf"),
    },
});
```

## Configuration

All settings are optional — defaults are sensible.

```toml
[http-security]
csrf_cookie = "ns_csrf"
csrf_header = "X-CSRF-Token"
csrf_field  = "csrf_token"
csrf_secure = true       # set Secure flag (production)
csrf_path   = "/"
```

## Variations

### Combine with HTTPS redirect

```toml
[[routing.routes]]
path           = "/settings"
handler        = "settings.update"
methods        = ["GET", "POST"]
csrf           = "true"
https_redirect = "true"
```

### Opt a route out of CSRF inside a group

Set `csrf = "false"` on the route — it overrides the group's setting.

## See also

- [Set security headers]({{< relref "security-headers" >}})
- [Add OIDC authentication]({{< relref "oidc" >}})
