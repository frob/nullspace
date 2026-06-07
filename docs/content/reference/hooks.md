---
title: Hook points
weight: 3
---

Hook points are named extension sites fired by the framework. Handlers
registered with `k.Hook(name, priority, fn)` run in ascending priority order;
handlers registered with `k.HookResolve(name, priority, fn)` run in priority
order until one returns `resolved = true`. Hooks owned by a disabled module
are skipped based on the request's config snapshot.

All hook handlers receive a `context.Context`. The context contains the
config snapshot (`kernel.SnapshotFromContext`), the per-request logger
(`nslog.FromContext`), and any framework values set earlier in the request.

## Kernel lifecycle

Type: fire. Fired by the kernel during `Init`, `Start`, and `Stop`.

| Name                  | When                                                                       |
| --------------------- | -------------------------------------------------------------------------- |
| `kernel.before_init`  | After config is loaded; before any module's `Init` runs.                   |
| `kernel.after_init`   | After every enabled module's `Init` has returned.                          |
| `kernel.before_start` | Before any module's `Start` runs.                                          |
| `kernel.after_start`  | After every enabled module's `Start` has returned.                         |
| `kernel.before_stop`  | Before any module's `Stop` runs.                                           |
| `kernel.after_stop`   | After every enabled module's `Stop` has returned.                          |

## HTTP request lifecycle

Type: fire. Fired by `core/request.Adapter.ServeHTTP`.

| Name                | When                                                              |
| ------------------- | ----------------------------------------------------------------- |
| `request.received`  | Right after the per-request logger and config snapshot are attached. |
| `request.routed`    | After route matching (or after fallbacks fail).                   |
| `request.before`    | Before the matched handler runs.                                  |
| `request.after`     | After the handler returns (skipped on hijacked connections).      |
| `request.complete`  | Final hook fired for every request, with response status in context. |
| `request.error`     | Fired by the response pipeline when writing an error response.    |

## Response pipeline

| Name                       | Type    | Arguments / contract                                       | When                                                                  |
| -------------------------- | ------- | ---------------------------------------------------------- | --------------------------------------------------------------------- |
| `response.before_write`    | fire    | —                                                          | Just before format resolution and serialization.                      |
| `response.after_write`     | fire    | —                                                          | After the response body has been written.                             |
| `response.format.resolve`  | resolve | Return `string` format name, `resolved=true` to short-circuit. | Each call to `Pipeline.Write` / `WriteStream`. Built-in resolvers run at priorities 10/20/30/40. |

## Data layer

Type: fire. Fired by `data.file` and `data.sql` around CRUD operations.

| Name                | When                                                    |
| ------------------- | ------------------------------------------------------- |
| `data.before_read`  | Before `Read`, `List`, `ListIter`, or `Query`.          |
| `data.after_read`   | After a successful read.                                |
| `data.before_write` | Before `Write`, `Delete`, or `Exec`.                    |
| `data.after_write`  | After a successful write.                               |

## Session module

| Name                | Type    | Arguments / contract                                  | When                                              |
| ------------------- | ------- | ----------------------------------------------------- | ------------------------------------------------- |
| `session.created`   | fire    | —                                                     | After `Module.Create` creates a new session.      |
| `session.loaded`    | fire    | —                                                     | After `session.load` or `session.require` loads a valid session into context. |
| `session.destroyed` | fire    | —                                                     | After `Module.Destroy` deletes a session.         |
| `session.login_url` | resolve | Return `string` URL with `resolved=true` to override 401 with a redirect. | When `session.require` rejects an unauthenticated request. |

## TCP / IPC transports

Type: fire. Fired by `core/tcp.Adapter`. IPC reuses the same hook names.

| Name              | When                                                    |
| ----------------- | ------------------------------------------------------- |
| `tcp.connected`   | New TCP connection accepted.                            |
| `tcp.message`     | A framed message was successfully decoded and is about to be dispatched. |
| `tcp.disconnected` | After a connection is closed (clean or otherwise).     |
| `tcp.error`       | Non-clean termination of a connection.                  |

## WebSocket

Type: fire. Fired by the `websocket` module's `upgradeHandler`.

| Name                     | When                                                  |
| ------------------------ | ----------------------------------------------------- |
| `websocket.connected`    | After the WebSocket upgrade completes and the connection is registered. |
| `websocket.message`      | Each message received before the user handler runs.   |
| `websocket.disconnected` | After the connection is removed from the manager.     |
| `websocket.error`        | Non-clean close (`websocket.CloseStatus == -1`).      |
