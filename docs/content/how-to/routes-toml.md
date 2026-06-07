---
title: Define routes in TOML
weight: 1
---

Use this guide when you want to declare HTTP routes without writing Go code.

## Solution

Add a `[routing]` section to `nullspace.toml` (or a per-module `routes.toml`).
A route needs a `path` and a `handler`; groups give a set of routes a shared
prefix, format, and middleware list.

```toml
[routing.groups.api]
prefix = "/api"
format = "json"

[routing.groups.pages]
prefix = ""
format = "html"

[[routing.routes]]
group   = "api"
path    = "/health"
handler = "health.check"

[[routing.routes]]
group    = "pages"
path     = "/"
handler  = "template"
template = "home.html"

[[routing.routes]]
path     = "/blog"
handler  = "redirect"
redirect = "/posts"
```

Restart the server. `nullspace routes` lists every registered route.

## Route fields

| Field          | Purpose                                              |
| -------------- | ---------------------------------------------------- |
| `path`         | URL pattern (`/posts/:id`)                           |
| `handler`      | Named handler — built-in or registered by a module   |
| `group`        | Inherit prefix, format, and middleware               |
| `methods`      | HTTP methods (defaults to `["GET"]`)                 |
| `format`       | Response format override                             |
| `template`     | Template name for the `template` handler             |
| `middleware`   | Additional named middleware, appended to the group's |
| `collection`   | Collection name for `data.*` handlers                |
| `redirect`     | Target URL for the `redirect` handler                |
| `status_code`  | Redirect status (defaults to 303)                    |
| `extra`        | Inline table of module-specific metadata             |

## Built-in handlers

| Handler        | Purpose                                |
| -------------- | -------------------------------------- |
| `data.list`    | List entities from `collection`        |
| `data.get`     | Get a single entity by route param     |
| `data.create`  | Create an entity from the request body |
| `data.update`  | Update an entity                       |
| `data.delete`  | Delete an entity                       |
| `template`     | Render a template, no data fetching    |
| `redirect`     | HTTP redirect                          |

## Variations

### Per-module route file

A module can own its routes via an embedded `routes.toml`. The file uses the
same syntax minus the `routing.` prefix.

```go
//go:embed routes.toml
var routesData []byte

func (m *Module) Init(k *kernel.Kernel) error {
    routingMod, _ := kernel.GetResource[*routing.Module](k, "routing")
    return routingMod.LoadRoutes(routesData)
}
```

```toml
# routes.toml
[groups.admin]
prefix     = "/api/admin"
format     = "json"
middleware = ["auth"]

[[routes]]
group   = "admin"
path    = "/users"
handler = "users.list"
```

### Inline route metadata

Use `extra` to pass arbitrary key-value metadata into a route. Modules read it
through `ctx.Route().Meta`.

```toml
[[routing.routes]]
path    = "/api/posts"
handler = "data.list"
collection = "posts"
extra   = { stream = "true" }
```

## See also

- [Register a custom handler]({{< relref "custom-handler" >}})
- [Generate CRUD routes for a collection]({{< relref "collections" >}})
- [Add middleware to a route group]({{< relref "middleware" >}})
