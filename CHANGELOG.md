# Changelog

All notable changes to this project are recorded here. This file follows
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and the project
adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

For longer-horizon planning see [ROADMAP.md](ROADMAP.md).

## [Unreleased]

### Added

- `module/jobs`: background-job runner with pluggable stores. Apps submit
  work via a typed `JobSpec`; a worker pool leases jobs, runs registered
  handlers, and acks or retries with exponential backoff. Pluggable
  storage behind a `Store` interface — the memory store covers
  single-process apps, the SQL store persists across restarts and uses
  `SELECT ... FOR UPDATE SKIP LOCKED` on Postgres with a `BEGIN IMMEDIATE`
  fallback on SQLite. Lifecycle hooks (`jobs.before`, `jobs.after`,
  `jobs.error`, `jobs.retry`, `jobs.dead`) plug in tracing, custom
  backoff, and alerting. `data.bridge` registers `jobs.submit`,
  `jobs.list`, and `jobs.cancel` commands on TCP and IPC transports.
- `nullspace jobs` CLI subcommand prints the registered handler table.
- `cmd/examples/jobs/` worked example.
- `docs/content/how-to/jobs.md` how-to guide.

### Changed

- `core/routing`: the `kernel.after_init` hook now skips the router
  lookup when no routes need registering, so kernels that use the
  registry without an HTTP adapter can boot cleanly.
- Licensed under the Mozilla Public License 2.0 (`LICENSE` added;
  release packages now declare MPL-2.0).
- `cmd/examples/grpc` and `cmd/examples/oidc` moved into their own Go
  modules so the main module no longer depends on sibling contrib
  checkouts. Fresh clones build and test without a `go.work`; the
  examples require `nullspace-grpc` / `nullspace-oidc` checked out as
  siblings.
- `nhooyr.io/websocket` is now a direct dependency in `go.mod`
  (was mislabeled `// indirect`).

### Removed

- Stray `example` and `oidc` binaries committed at the repo root, and
  the committed `go.work`/`go.work.sum` (both now gitignored). History
  was rewritten to purge the binaries.

## [0.0.x] - 2026-06-07

First development line. Established the hexagonal architecture, the
kernel + module + hook bus contract, and the initial set of core and
optional modules.

### Added

- Kernel: module registry, lifecycle (`Init`/`Start`/`Stop`), hook bus
  (`Hook`/`HookResolve`/`Fire`/`Resolve`), TOML config with
  `NULLSPACE_*` env overrides, service locator (`Provide` +
  generic `GetResource[T]`).
- `core/nslog`: slog adapter, per-request loggers attached via context.
- `core/request`: HTTP adapter, router, middleware, request context.
- `core/response`: format resolution and a streaming pipeline with
  JSON, HTML, text, and ANSI formatters.
- `core/routing`: declarative TOML routing, handler registry, built-in
  handlers.
- `core/tcp`: TCP transport adapter with command router and codecs.
- `core/ipc`: Unix-socket transport adapter wrapping the TCP adapter.
- `module/data/static`: static file serving.
- `module/data/file`: file-based entity storage with lazy iteration.
- `module/data/sql`: SQL store with SQLite default, migration registry
  run from `kernel.after_init`.
- `module/data/bridge`: TCP/IPC bridge exposing `data.*` commands.
- `module/session`: session management with memory and SQL stores.
- `module/httpsecurity`: security headers, CSRF, HTTPS redirect.
- `module/websocket`: WebSocket upgrade, connection manager, rooms,
  broadcast.
- Contrib integration: `nullspace-grpc` adapter (gRPC transport) and
  `nullspace-oidc` middleware (Authorization Code + PKCE).
- Worked examples in `cmd/examples/`: `kitchen-sink` (reference app),
  `grpc` (HTTP + TCP + gRPC), `oidc` (Keycloak-backed OIDC).
- `cmd/nullspace` installable binary with `serve`, `init`, and
  `routes` subcommands.
- Documentation site under `docs/`, organized by the Diataxis
  framework.

### Changed

- Reorganized modules to separate required core modules from optional,
  pluggable ones.
- Reorganized examples and broadened the core library so additional
  transport adapters (gRPC, OIDC) can be added without forking.
- File and transport handling refactored to be transport-agnostic.

### Fixed

- Path traversal in the file data module.

### Security

- Hardened data handlers and increased session ID entropy.
