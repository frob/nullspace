---
title: Environment variables
weight: 7
---

Every `nullspace.toml` key can be overridden by an environment variable.
The convention is:

```
NULLSPACE_<SECTION>_<KEY>
```

Rules implemented by `applyEnvOverrides`:

- Prefix `NULLSPACE_` is stripped (case-insensitively).
- The remainder is lowercased.
- Each `_` becomes a `.`, producing a dotted config path.
- The path is set on the loaded config as a string (TOML decoding still
  applies on access; numeric and boolean fields accept their string forms).
- The prefix can be overridden via `kernel.WithEnvPrefix("MYAPP")` when
  embedding the kernel.

Examples:

| Environment variable          | Equivalent TOML                   |
| ----------------------------- | --------------------------------- |
| `NULLSPACE_LOG_LEVEL=debug`   | `[log]`<br>`level = "debug"`      |
| `NULLSPACE_REQUEST_ADDR=:9090` | `[request]`<br>`addr = ":9090"`  |
| `NULLSPACE_RESPONSE_DEFAULT_FORMAT=html` | `[response]`<br>`default_format = "html"` |

Because every `_` becomes `.`, a section name containing `.` (such as
`data.static`) maps to a path with two underscores between the
section and the key:

| Environment variable             | Equivalent TOML                  |
| -------------------------------- | -------------------------------- |
| `NULLSPACE_DATA_STATIC_DIR=/srv` | `[data.static]`<br>`dir = "/srv"` |
| `NULLSPACE_DATA_FILE_DIR=/data`  | `[data.file]`<br>`dir = "/data"` |
| `NULLSPACE_DATA_SQL_DSN=...`     | `[data.sql]`<br>`dsn = "..."`    |

## Override table

Every override key supported by the framework's built-in modules.

### `[request]`

| Variable                 | TOML key       |
| ------------------------ | -------------- |
| `NULLSPACE_REQUEST_ADDR` | `request.addr` |

### `[response]`

| Variable                                  | TOML key                  |
| ----------------------------------------- | ------------------------- |
| `NULLSPACE_RESPONSE_DEFAULT_FORMAT`       | `response.default_format` |
| `NULLSPACE_RESPONSE_TEMPLATE_DIR`         | `response.template_dir`   |

### `[log]`

| Variable                | TOML key     |
| ----------------------- | ------------ |
| `NULLSPACE_LOG_LEVEL`   | `log.level`  |
| `NULLSPACE_LOG_FORMAT`  | `log.format` |

### `[data.static]`

| Variable                      | TOML key          |
| ----------------------------- | ----------------- |
| `NULLSPACE_DATA_STATIC_DIR`   | `data.static.dir` |

### `[data.file]`

| Variable                       | TOML key             |
| ------------------------------ | -------------------- |
| `NULLSPACE_DATA_FILE_DIR`      | `data.file.dir`      |
| `NULLSPACE_DATA_FILE_FORMAT`   | `data.file.format`   |

### `[data.sql]`

| Variable                       | TOML key            |
| ------------------------------ | ------------------- |
| `NULLSPACE_DATA_SQL_DRIVER`    | `data.sql.driver`   |
| `NULLSPACE_DATA_SQL_DSN`       | `data.sql.dsn`      |

### `[tcp]`

| Variable                              | TOML key               |
| ------------------------------------- | ---------------------- |
| `NULLSPACE_TCP_ADDR`                  | `tcp.addr`             |
| `NULLSPACE_TCP_CODEC`                 | `tcp.codec`            |
| `NULLSPACE_TCP_MAX_MESSAGE_SIZE`      | `tcp.max_message_size` |

### `[ipc]`

| Variable                              | TOML key               |
| ------------------------------------- | ---------------------- |
| `NULLSPACE_IPC_PATH`                  | `ipc.path`             |
| `NULLSPACE_IPC_CODEC`                 | `ipc.codec`            |
| `NULLSPACE_IPC_MAX_MESSAGE_SIZE`      | `ipc.max_message_size` |

### `[session]`

| Variable                  | TOML key         |
| ------------------------- | ---------------- |
| `NULLSPACE_SESSION_STORE` | `session.store`  |
| `NULLSPACE_SESSION_COOKIE` | `session.cookie` |
| `NULLSPACE_SESSION_TTL`   | `session.ttl`    |
| `NULLSPACE_SESSION_SECURE` | `session.secure` |
| `NULLSPACE_SESSION_PATH`  | `session.path`   |

### `[websocket]`

| Variable                                  | TOML key                        |
| ----------------------------------------- | ------------------------------- |
| `NULLSPACE_WEBSOCKET_MAX_MESSAGE_SIZE`    | `websocket.max_message_size`    |
| `NULLSPACE_WEBSOCKET_ALLOWED_ORIGINS`     | `websocket.allowed_origins`     |
| `NULLSPACE_WEBSOCKET_INSECURE_SKIP_VERIFY` | `websocket.insecure_skip_verify` |

### `[http-security]`

Note: hyphens cannot appear in environment variable names. The
`http-security` section is reached through the dotted path; consequently
the section name is *not* overridable via env. Sub-keys are reachable when
the section is also set in TOML. *(In practice, set this section in TOML.)*

### `[oidc]`

| Variable                          | TOML key             |
| --------------------------------- | -------------------- |
| `NULLSPACE_OIDC_ISSUER`           | `oidc.issuer`        |
| `NULLSPACE_OIDC_CLIENT_ID`        | `oidc.client_id`     |
| `NULLSPACE_OIDC_CLIENT_SECRET`    | `oidc.client_secret` |
| `NULLSPACE_OIDC_REDIRECT_URI`     | `oidc.redirect_uri`  |
| `NULLSPACE_OIDC_COOKIE_SECRET`    | `oidc.cookie_secret` |
| `NULLSPACE_OIDC_POST_LOGIN_URL`   | `oidc.post_login_url` |
| `NULLSPACE_OIDC_POST_LOGOUT_URL`  | `oidc.post_logout_url` |
| `NULLSPACE_OIDC_PATH_PREFIX`      | `oidc.path_prefix`   |

### `[modules]`

| Variable                       | TOML key                  |
| ------------------------------ | ------------------------- |
| `NULLSPACE_MODULES_SESSION`    | `[modules]` → `session`   |
| `NULLSPACE_MODULES_WEBSOCKET`  | `[modules]` → `websocket` |
| `NULLSPACE_MODULES_TCP`        | `[modules]` → `tcp`       |
| `NULLSPACE_MODULES_IPC`        | `[modules]` → `ipc`       |
| `NULLSPACE_MODULES_OIDC`       | `[modules]` → `oidc`      |
