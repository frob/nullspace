---
title: Named handlers
weight: 2
---

Handler names are referenced by the `handler` field of a route. The routing
module's `Registry` maps names to `request.HandlerFunc` values. Modules
register handlers during `Init`. Handlers below are registered by the core
framework when the corresponding module is loaded.

## Built-in handlers

Registered by the routing module (`core/routing`) when it initializes.

| Name          | Description                                                                | Route metadata consumed                                  | Default format / output                                                  |
| ------------- | -------------------------------------------------------------------------- | --------------------------------------------------------- | ------------------------------------------------------------------------- |
| `data.list`   | List all entities in `collection`. Streams when `?stream=true` or `stream` meta is set. | `_collection` (required), `_template` (optional)         | Resolved format; `{Items, Collection}` payload (plus `Total` for streams).|
| `data.get`    | Read one entity by ID.                                                     | `_collection` (required), `_data_param` (default `"id"`), `_template` | Resolved format; entity as map. Returns 404 if not found.                |
| `data.create` | Create an entity from the request body (JSON or form-encoded). Requires `id` in the body. | `_collection` (required)                                  | `{id, status:"created"}` with HTTP 201.                                  |
| `data.update` | Replace an existing entity. Body merges into the entity at `_data_param`.  | `_collection` (required), `_data_param` (default `"id"`)  | `{id, status:"updated"}` with HTTP 200.                                  |
| `data.delete` | Delete an entity by ID.                                                    | `_collection` (required), `_data_param` (default `"id"`)  | `{id, status:"deleted"}` with HTTP 200; 404 if not found.                |
| `template`    | Render a template with `{Path, Params}` and no data fetching.              | `_template` (required)                                    | HTML output via the named template.                                      |
| `redirect`    | Issue an HTTP redirect.                                                    | Read from `Route.Redirect` and `Route.StatusCode`.        | `Location` header; status defaults to 303 (See Other).                   |

The `data.*` handlers operate on the file data module (`module/data/file`).
The `_collection`, `_template`, `_data_param` keys are set automatically by
the routing module from the TOML route fields `collection`, `template`,
`data_param`.

## Conventional handler (binary only)

Registered by the `nullspace` binary's `conventionalHandlers` module.

| Name           | Description                          | Route metadata consumed | Output                                |
| -------------- | ------------------------------------ | ------------------------ | ------------------------------------- |
| `health.check` | Returns a static health-check JSON. | None.                    | `{"status":"ok"}` JSON, HTTP 200.    |

## WebSocket handlers

The WebSocket module exposes app-registered handlers under a prefix.

| Name pattern | Description                                                           | Route metadata consumed | Output                |
| ------------ | --------------------------------------------------------------------- | ------------------------ | --------------------- |
| `ws.<name>`  | Upgrade the request to WebSocket and run the `HandlerFunc` registered via `Module.HandleFunc("<name>", ...)`. | `ws_rooms` (auto-join comma-separated rooms). | None — connection is hijacked. |

## Data bridge commands

The data bridge module (`module/data/bridge`) does not register HTTP
handlers — instead it registers TCP/IPC command handlers under the same
names:

| Command       | Payload (JSON)                                  | Reply                                                                       |
| ------------- | ----------------------------------------------- | --------------------------------------------------------------------------- |
| `data.list`   | `{"collection":"posts","stream":false}`         | `{"Items":[...],"Collection":"posts"}`. Streaming yields `data.list.start`, repeated `data.list.item`, `data.list.end`. |
| `data.get`    | `{"collection":"posts","id":"hello"}`           | Entity as JSON object. Error response on 404.                               |
| `data.create` | `{"collection":"posts","body":{...}}`           | `{"id":"…","status":"created"}`.                                            |
| `data.update` | `{"collection":"posts","id":"…","body":{...}}` | `{"id":"…","status":"updated"}`.                                            |
| `data.delete` | `{"collection":"posts","id":"…"}`              | `{"id":"…","status":"deleted"}`.                                            |

## Named middleware

Registered by modules on the same `Registry` and resolved by the route's
`middleware` field.

| Name                | Registered by   | Behavior                                                                                |
| ------------------- | --------------- | --------------------------------------------------------------------------------------- |
| `session.load`      | `session`       | Load session from cookie if present. Auto-saves dirty sessions after the handler.       |
| `session.require`   | `session`       | Enforce a valid session. Returns 401 or redirects via the `session.login_url` resolver. |
| `session.ignore`    | `session`       | No-op. Companion to the `session = "ignore"` route directive.                           |
| `security.redirect` | `http-security` | Redirect HTTP to HTTPS when `https_redirect = "true"` is set on the route.              |
| `security.csrf`     | `http-security` | CSRF double-submit validation when `csrf = "true"` is set on the route.                 |
| `security.headers`  | `http-security` | Apply security response headers. Skipped when route sets `http-security = "none"`.      |

The OIDC contrib module registers a middleware named `oidc`.
