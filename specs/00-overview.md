# Nullspace Framework — Architecture Overview

Nullspace is an HTTP application framework written in Go for serving APIs, HTML, WebSockets, LLM interfaces, and future modalities.

## Core Principles

- **Hexagonal architecture** — ports and adapters separate concerns
- **Aspect-oriented design** — cross-cutting concerns handled via hook bus + functional middleware
- **Module system** — self-contained, configurable, lifecycle-managed components
- **Developer experience** — simplicity and flexibility over convention

## Architecture Summary

| Component | Responsibility |
|-----------|---------------|
| Kernel | Module registry, lifecycle management, hook bus, config primitive |
| Request | HTTP adapter, functional middleware chain |
| Response | Format negotiation, formatter pipeline, serialization |
| Data | Data provider lifecycle, ships with SQL and file modules |
| Logging | Logger port with slog default, per-request context |
| Config | TOML + env overrides, module-declared sections, per-request snapshots |

## AOP Strategy

Hybrid approach:
- **Functional middleware** for the request/response pipeline (idiomatic Go, composable)
- **Hook/event registry** for cross-cutting concerns that span components (lifecycle, policy, format resolution)

## Project Layout

Library-first (`go get`) with CLI scaffolding tool planned as a later layer.

```
nullspace/
├── cmd/example/main.go
├── kernel/
│   ├── kernel.go
│   ├── hook.go
│   └── port.go
├── request/
│   ├── adapter.go
│   ├── context.go
│   └── middleware.go
├── response/
│   ├── response.go
│   ├── formatter.go
│   ├── pipeline.go
│   ├── json.go
│   └── html.go
├── data/
│   ├── port.go
│   ├── sql/
│   ├── file/
│   └── static/
├── log/
│   ├── port.go
│   └── slog.go
├── config/
│   ├── config.go
│   └── snapshot.go
├── go.mod
└── go.sum
```

## Build Order

1. Kernel — hook bus, module registry, ports
2. Config — TOML + env, module config sections, snapshots
3. Logging — logger port + slog adapter
4. Request — HTTP adapter + middleware chain
5. Response — formatter port + JSON + HTML pipeline
6. Data — static files, file module, SQL module
7. Example app — wire it all together
8. CLI scaffold tool (future)

## Dependencies

- `pelletier/go-toml` — TOML parsing (config)
- `mattn/go-sqlite3` or `modernc.org/sqlite` — SQLite (data/sql default)
- Go stdlib for everything else
