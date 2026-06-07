---
title: Configuration keys
weight: 1
---

Every TOML section recognized by the Nullspace kernel and its built-in
modules. Keys are read from `nullspace.toml` and overridden by environment
variables using the `NULLSPACE_SECTION_KEY` convention (see
[Environment variables]({{< relref "env" >}})).

Module-level config sections also accept a corresponding `[modules]` entry
to toggle the module on or off. Modules with `DefaultEnabled = false` must
be explicitly enabled.

## `[modules]`

Per-module enable/disable map. Keys are module names; values are booleans.
Unknown modules default to enabled. Modules with `DefaultEnabled = false`
require an explicit `true`.

```toml
[modules]
session       = true
"data.sql"    = true
tcp           = false
```

## `[request]`

HTTP request adapter.

| Key  | Type   | Default  | Description                |
| ---- | ------ | -------- | -------------------------- |
| addr | string | `":8080"` | Listen address for the HTTP server. |

## `[response]`

Response pipeline.

| Key             | Type   | Default        | Description                                  |
| --------------- | ------ | -------------- | -------------------------------------------- |
| default_format  | string | `"json"`       | Fallback formatter when no resolver wins.    |
| template_dir    | string | `"./templates"` | Directory loaded by the HTML formatter.     |

## `[log]`

Logging module.

| Key    | Type   | Default | Description                                    |
| ------ | ------ | ------- | ---------------------------------------------- |
| level  | string | `"info"` | Log level: `debug`, `info`, `warn`, `error`.  |
| format | string | `"text"` | Handler format: `text` or `json`.             |

## `[data.static]`

Static file fallback module.

| Key | Type   | Default     | Description                                |
| --- | ------ | ----------- | ------------------------------------------ |
| dir | string | `"./public"` | Directory served as a fallback handler.   |

## `[data.file]`

File-based entity store.

| Key    | Type   | Default       | Description                                                  |
| ------ | ------ | ------------- | ------------------------------------------------------------ |
| dir    | string | `"./content"`  | Root directory; each subdirectory is a collection.          |
| format | string | `"markdown"`   | Default serialization for new files (`markdown`, `json`, `toml`). |

## `[data.sql]`

SQL data module. `DefaultEnabled = false`.

| Key    | Type   | Default      | Description                                       |
| ------ | ------ | ------------ | ------------------------------------------------- |
| driver | string | `"sqlite"`   | `database/sql` driver name.                       |
| dsn    | string | `"./data.db"` | Driver-specific data source name.                 |

## `[tcp]`

TCP transport adapter. `DefaultEnabled = false`.

| Key              | Type   | Default        | Description                                              |
| ---------------- | ------ | -------------- | -------------------------------------------------------- |
| addr             | string | `":9090"`       | TCP listen address.                                     |
| codec            | string | `"json-lines"`  | Frame codec: `json-lines` (aka `json`) or `length-prefix` (aka `binary`). |
| max_message_size | int    | `1048576`       | Maximum message size in bytes.                          |

## `[ipc]`

Unix domain socket transport adapter. `DefaultEnabled = false`.

| Key              | Type   | Default                 | Description                              |
| ---------------- | ------ | ----------------------- | ---------------------------------------- |
| path             | string | `"/tmp/nullspace.sock"` | Unix socket path.                        |
| codec            | string | `"json-lines"`          | Frame codec (same options as `[tcp]`).   |
| max_message_size | int    | `1048576`               | Maximum message size in bytes.           |

## `[session]`

Session module. `DefaultEnabled = false`.

| Key    | Type   | Default       | Description                                                       |
| ------ | ------ | ------------- | ----------------------------------------------------------------- |
| store  | string | `"memory"`    | Backing store: `memory` or `sql` (requires `data.sql`).           |
| cookie | string | `"ns_session"` | Session cookie name.                                              |
| ttl    | string | `"24h"`       | Session lifetime as a Go duration string.                         |
| secure | bool   | `false`       | Set the `Secure` attribute on the session cookie.                 |
| path   | string | `"/"`         | Cookie path.                                                      |

## `[websocket]`

WebSocket module. `DefaultEnabled = false`.

| Key                  | Type     | Default | Description                                                                       |
| -------------------- | -------- | ------- | --------------------------------------------------------------------------------- |
| max_message_size     | int      | `65536` | Maximum message size in bytes. `0` disables the limit.                            |
| allowed_origins      | []string | `[]`    | Allowed origin patterns for the upgrade handshake.                                |
| insecure_skip_verify | bool     | `false` | Disable origin verification. Development only.                                    |

## `[http-security]`

HTTP security module. `DefaultEnabled = false`.

| Key                  | Type   | Default                              | Description                                                |
| -------------------- | ------ | ------------------------------------ | ---------------------------------------------------------- |
| hsts                 | bool   | `false`                              | Emit `Strict-Transport-Security` header.                   |
| hsts_max_age         | int    | `31536000`                           | HSTS `max-age` in seconds.                                 |
| hsts_include_subs    | bool   | `true`                               | Add `includeSubDomains`.                                   |
| hsts_preload         | bool   | `false`                              | Add `preload`.                                             |
| csp                  | string | `""`                                 | `Content-Security-Policy` value. Empty disables emission.  |
| frame_options        | string | `"DENY"`                             | `X-Frame-Options` value. Empty disables emission.          |
| referrer_policy      | string | `"strict-origin-when-cross-origin"`  | `Referrer-Policy` value.                                   |
| permissions_policy   | string | `""`                                 | `Permissions-Policy` value.                                |
| csrf_cookie          | string | `"ns_csrf"`                          | CSRF cookie name (double-submit pattern).                  |
| csrf_header          | string | `"X-CSRF-Token"`                     | HTTP header read on state-changing requests.               |
| csrf_field           | string | `"csrf_token"`                       | Form field read on state-changing requests.                |
| csrf_secure          | bool   | `false`                              | Set `Secure` attribute on the CSRF cookie.                 |
| csrf_path            | string | `"/"`                                | CSRF cookie path.                                          |

`X-Content-Type-Options: nosniff` is always set by the headers middleware.

## `[oidc]`

Provided by the contributed `nullspace-oidc` module. `DefaultEnabled = false`.

| Key             | Type     | Default                                | Description                                                                |
| --------------- | -------- | -------------------------------------- | -------------------------------------------------------------------------- |
| issuer          | string   | *(required)*                           | OIDC provider issuer URL.                                                  |
| client_id       | string   | *(required)*                           | OAuth2 client ID.                                                          |
| client_secret   | string   | `""`                                   | Client secret. Leave empty for public client + PKCE.                       |
| redirect_uri    | string   | derived from `[request] addr`          | Callback URL. Auto-derived when empty.                                     |
| scopes          | []string | `["openid", "profile", "email"]`       | Scopes to request from the IDP.                                            |
| cookie_name     | string   | `"ns_oidc"`                            | Encrypted session cookie name.                                             |
| cookie_secret   | string   | *(ephemeral if empty)*                 | 32 or 64 hex characters for the AES-GCM key.                               |
| cookie_secure   | bool     | `false`                                | Set `Secure` attribute on OIDC cookies.                                    |
| post_login_url  | string   | `"/"`                                  | Redirect after successful login.                                           |
| post_logout_url | string   | `"/"`                                  | Redirect after logout.                                                     |
| path_prefix     | string   | `"/oidc"`                              | URL prefix for `login`/`callback`/`logout` routes.                         |

## `[routing]`

Routing module. See [Routing TOML schema]({{< relref "routing-schema" >}})
for `[routing.groups.*]`, `[[routing.routes]]`, and `[[routing.collections]]`.

| Key         | Type    | Default | Description                                            |
| ----------- | ------- | ------- | ------------------------------------------------------ |
| groups      | table   | `{}`    | Map of named groups (`[routing.groups.<name>]`).       |
| routes      | array   | `[]`    | List of route entries (`[[routing.routes]]`).          |
| collections | array   | `[]`    | List of CRUD collection entries (`[[routing.collections]]`). |
