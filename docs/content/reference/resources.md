---
title: Service locator keys
weight: 4
---

Resources are registered with `k.Provide(key, value)` and retrieved with
`k.Resource(key)` or the generic helper `kernel.GetResource[T](k, key)`.
The table lists every key registered by the core framework. Application
modules may register additional keys.

| Key                    | Type                                  | Registered by              | Retrieved with                                                          |
| ---------------------- | ------------------------------------- | -------------------------- | ----------------------------------------------------------------------- |
| `router`               | `*request.Router`                     | `core/request` adapter     | `kernel.GetResource[*request.Router](k, "router")`                      |
| `request.adapter`      | `*request.Adapter`                    | `core/request` adapter     | `kernel.GetResource[*request.Adapter](k, "request.adapter")`            |
| `transport.http`       | `*request.Adapter`                    | `core/request` adapter     | `kernel.GetResource[*request.Adapter](k, "transport.http")`             |
| `response.pipeline`    | `*response.Pipeline`                  | `core/response` pipeline   | `kernel.GetResource[*response.Pipeline](k, "response.pipeline")`        |
| `logger`               | `kernel.Logger`                       | `core/nslog`               | `kernel.GetResource[kernel.Logger](k, "logger")`                        |
| `routing`              | `*routing.Module`                     | `core/routing`             | `kernel.GetResource[*routing.Module](k, "routing")`                     |
| `routing.registry`     | `*routing.Registry`                   | `core/routing`             | `kernel.GetResource[*routing.Registry](k, "routing.registry")`          |
| `tcp.router`           | `*tcp.Router`                         | `core/tcp` adapter         | `kernel.GetResource[*tcp.Router](k, "tcp.router")`                      |
| `transport.tcp`        | `*tcp.Adapter`                        | `core/tcp` adapter         | `kernel.GetResource[*tcp.Adapter](k, "transport.tcp")`                  |
| `transport.ipc`        | `*ipc.Adapter`                        | `core/ipc` adapter         | `kernel.GetResource[*ipc.Adapter](k, "transport.ipc")`                  |
| `data.static`          | `*static.Module`                      | `module/data/static`       | `kernel.GetResource[*static.Module](k, "data.static")`                  |
| `data.file`            | `*file.Module`                        | `module/data/file`         | `kernel.GetResource[*file.Module](k, "data.file")`                      |
| `data.sql`             | `*sql.Module`                         | `module/data/sql`          | `kernel.GetResource[*sql.Module](k, "data.sql")`                        |
| `db`                   | `*database/sql.DB`                    | `module/data/sql`          | `kernel.GetResource[*sql.DB](k, "db")`                                  |
| `data.sql.migrations`  | `*sql.MigrationRegistry`              | `module/data/sql`          | `kernel.GetResource[*sql.MigrationRegistry](k, "data.sql.migrations")`  |
| `session`              | `*session.Module`                     | `module/session`           | `kernel.GetResource[*session.Module](k, "session")`                     |
| `session.store`        | `session.Store`                       | `module/session`           | `kernel.GetResource[session.Store](k, "session.store")`                 |
| `websocket`            | `*websocket.Module`                   | `module/websocket`         | `kernel.GetResource[*websocket.Module](k, "websocket")`                 |
| `websocket.manager`    | `*websocket.Manager`                  | `module/websocket`         | `kernel.GetResource[*websocket.Manager](k, "websocket.manager")`        |
| `http-security`        | `*httpsecurity.Module`                | `module/httpsecurity`      | `kernel.GetResource[*httpsecurity.Module](k, "http-security")`          |

## Discovery interfaces

Adapters that listen on a network address provide themselves under a
`transport.<protocol>` key and implement `transport.Listener`. To enumerate
active listeners, iterate the known keys (`transport.http`, `transport.tcp`,
`transport.ipc`) and call `.Protocol()` / `.Addr()`.
