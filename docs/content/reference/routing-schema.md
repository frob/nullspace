---
title: Routing TOML schema
weight: 5
---

The routing module reads routes from two sources:

- The `[routing]` section of `nullspace.toml`.
- Per-module `routes.toml` files loaded with `routing.Module.LoadRoutes`.

Both sources accept the same record shapes, but **the top-level keys
differ**: per-module files omit the `routing.` prefix. See
[Per-module `routes.toml`](#per-module-routestoml) below.

## `[routing.groups.<name>]`

A named group whose settings are merged into each route that references it
via the route's `group` field.

| Key        | Type     | Default | Description                                                       |
| ---------- | -------- | ------- | ----------------------------------------------------------------- |
| prefix     | string   | `""`    | Prepended to every member route's `path`.                         |
| format     | string   | `""`    | Default response format for member routes; route-level `format` wins. |
| middleware | []string | `[]`    | Names resolved via the routing registry. Prepended to each member route's middleware list. |

```toml
[routing.groups.api]
prefix     = "/api"
format     = "json"
middleware = ["session.require"]
```

## `[[routing.routes]]`

A single route declaration. Repeat the section for each route.

| Key            | Type              | Default        | Description                                                                  |
| -------------- | ----------------- | -------------- | ---------------------------------------------------------------------------- |
| group          | string            | `""`           | Name of a group whose settings are merged.                                   |
| path           | string            | *(required)*    | URL pattern. `:name` denotes a path parameter.                              |
| methods        | []string          | `["GET"]`      | HTTP methods. Each method generates its own route entry.                     |
| handler        | string            | *(required)*    | Named handler resolved via the routing registry, or the special `redirect`. |
| format         | string            | `""`           | Response format override for this route.                                     |
| template       | string            | `""`           | Template name (HTML formatter).                                              |
| middleware     | []string          | `[]`           | Per-route middleware names. Appended to any group middleware.                |
| collection     | string            | `""`           | Data collection consumed by `data.*` handlers.                               |
| data_param     | string            | `"id"`         | Path-parameter name read by `data.get`/`data.update`/`data.delete`.          |
| redirect       | string            | `""`           | Target URL for the `redirect` handler.                                       |
| status_code    | int               | `303`          | HTTP status for the `redirect` handler.                                      |
| session        | string            | `""`           | Set to `"ignore"` to bypass `session.load` / `session.require`.              |
| csrf           | string            | `""`           | Set to `"true"` to require CSRF token validation.                            |
| https_redirect | string            | `""`           | Set to `"true"` to redirect HTTP requests to HTTPS.                          |
| extra          | map[string]string | `{}`           | Arbitrary route metadata, propagated to `Route.Meta` as-is.                  |

### Path parameters

A path segment that begins with `:` becomes a named parameter. Parameters
are matched as a single non-empty segment and made available via
`ctx.Param("name")`.

```toml
[[routing.routes]]
path    = "/posts/:id"
handler = "data.get"
```

### Reserved `extra` keys

Some modules read keys from `extra` (which becomes part of `Route.Meta`):

| Key        | Used by      | Effect                                                                      |
| ---------- | ------------ | --------------------------------------------------------------------------- |
| `format`   | request adapter / response pipeline | Sets the resolved response format for the route.            |
| `stream`   | request adapter | Setting `stream = "true"` enables stream mode regardless of `?stream` query. |
| `ws_rooms` | websocket    | Comma-separated room names auto-joined after upgrade.                       |
| `http-security` | http-security | Setting `"none"` skips the global security headers middleware.         |

## `[[routing.collections]]`

A collection generates CRUD routes for a data source.

| Key              | Type     | Default      | Description                                                                          |
| ---------------- | -------- | ------------ | ------------------------------------------------------------------------------------ |
| name             | string   | *(required)* | Collection name. Used in URLs and passed to data handlers.                           |
| source           | string   | `"data.file"` | Data module name. Currently only the file module is wired into expansion.           |
| api_prefix       | string   | `""`         | If set, generates `GET/POST` on `<api_prefix>/<name>` and `GET/PUT/DELETE` on `<api_prefix>/<name>/:id`. |
| html_prefix      | string   | `""`         | If set together with `list_template` or `item_template`, generates HTML list / item routes. |
| list_template    | string   | `""`         | Template for the HTML list page.                                                     |
| item_template    | string   | `""`         | Template for the HTML single-item page.                                              |
| read_middleware  | []string | `[]`         | Middleware applied to `GET` routes.                                                  |
| write_middleware | []string | `[]`         | Middleware applied to `POST`/`PUT`/`DELETE` routes.                                  |

Generated routes for a collection `posts` with `api_prefix = "/api"`:

| Method | Path             | Handler       |
| ------ | ---------------- | ------------- |
| GET    | `/api/posts`     | `data.list`   |
| GET    | `/api/posts/:id` | `data.get`    |
| POST   | `/api/posts`     | `data.create` |
| PUT    | `/api/posts/:id` | `data.update` |
| DELETE | `/api/posts/:id` | `data.delete` |

## Per-module `routes.toml`

Modules ship route definitions in a sibling `routes.toml` file and load it
during `Init`:

```go
//go:embed routes.toml
var routesData []byte

routingMod, _ := kernel.GetResource[*routing.Module](k, "routing")
routingMod.LoadRoutes(routesData)
```

The schema is identical to `[routing]` in `nullspace.toml`, but the
top-level prefix is dropped — each table is rooted at the module file.

| `nullspace.toml`              | Per-module `routes.toml`     |
| ----------------------------- | ---------------------------- |
| `[routing.groups.api]`        | `[groups.api]`               |
| `[[routing.routes]]`          | `[[routes]]`                 |
| `[[routing.collections]]`     | `[[collections]]`            |

Loaded routes are deduplicated by effective path + handler against routes
already in the config. Groups are merged by name; the first definition
wins. Collections are merged by name; the first wins.
