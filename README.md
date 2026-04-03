# Nullspace

A multi-transport application framework in Go for serving APIs, HTML, WebSockets, and more over HTTP, TCP, and Unix sockets. Built on hexagonal architecture with aspect-oriented programming via a hook bus and functional middleware.

## Install

```bash
# macOS (Homebrew)
brew install frob/tap/nullspace

# Debian / Ubuntu
sudo dpkg -i nullspace_*_linux_amd64.deb

# Rocky / RHEL / Fedora
sudo rpm -i nullspace_*_linux_amd64.rpm

# Arch Linux
sudo pacman -U nullspace_*_linux_amd64.pkg.tar.zst

# Go install
go install github.com/frob/nullspace/cmd/nullspace@latest
```

## Quickstart

Scaffold a new project and start serving:

```bash
mkdir mysite && cd mysite
nullspace init
nullspace
```

Visit http://localhost:8080.

The `init` command creates a project with this structure:

```
mysite/
├── nullspace.toml       Configuration
├── content/
│   └── posts/
│       └── hello-world.md
├── templates/
│   ├── home.html
│   ├── posts.html
│   └── post.html
└── public/
    └── css/
        └── style.css
```

Content collections are auto-discovered from `content/` subdirectories. Each collection gets HTML and JSON routes:

- `/posts` and `/posts/:id` — HTML (rendered with templates)
- `/api/posts` and `/api/posts/:id` — JSON
- `/api/health` — Health check

List all registered routes:

```bash
nullspace routes
```

### Use as a library

For full control, use Nullspace as a Go library:

```bash
go get github.com/frob/nullspace
```

```go
package main

import (
    "context"
    "net/http"
    "os"
    "os/signal"
    "syscall"

    "github.com/frob/nullspace/kernel"
    "github.com/frob/nullspace/core/nslog"
    "github.com/frob/nullspace/core/request"
    "github.com/frob/nullspace/core/response"
)

func main() {
    k := kernel.New()

    adapter := request.NewAdapter()
    pipeline := response.NewPipeline()

    k.Use(nslog.New())
    k.Use(adapter)
    k.Use(pipeline)
    k.Use(response.NewFormatDefault())

    ctx := context.Background()
    k.Init(ctx)

    adapter.Router().Get("/hello", func(ctx *request.Context) error {
        resp := response.NewResponse(http.StatusOK, map[string]string{
            "message": "hello world",
        })
        return pipeline.Write(ctx.Context(), ctx.Writer, resp)
    })

    k.Start(ctx)

    quit := make(chan os.Signal, 1)
    signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
    <-quit
    k.Stop(ctx)
}
```

## Declarative Routing

Routes are defined in TOML — either in `nullspace.toml` or in per-module `routes.toml` files. TOML routes are additive; you can always define routes in Go code alongside them.

```toml
# nullspace.toml

# Named groups for shared settings.
[routing.groups.api]
prefix = "/api"
format = "json"

# Individual routes.
[[routing.routes]]
group = "api"
path = "/health"
handler = "health.check"

# Collections auto-generate CRUD routes.
[[routing.collections]]
name = "posts"
api_prefix = "/api"
html_prefix = ""
list_template = "posts.html"
item_template = "post.html"
write_middleware = ["auth"]
```

### Per-module route files

Each module can own its routes via an embedded `routes.toml`:

```go
//go:embed routes.toml
var routesData []byte

func (m *Module) Init(k *kernel.Kernel) error {
    routingMod, _ := kernel.GetResource[*routing.Module](k, "routing")  // core/routing
    routingMod.LoadRoutes(routesData)
    return nil
}
```

```toml
# modules/auth/routes.toml
[groups.admin]
prefix = "/api/admin"
format = "json"
middleware = ["auth"]

[[routes]]
group = "admin"
path = "/posts"
handler = "data.list"
collection = "posts"
```

### Built-in handlers

| Handler | Description |
|---------|-------------|
| `data.list` | List entities from a collection |
| `data.get` | Get a single entity by ID |
| `data.create` | Create entity from request body |
| `data.update` | Update entity from request body |
| `data.delete` | Delete entity by ID |
| `template` | Render a template (no data fetching) |
| `redirect` | HTTP redirect |

Custom handlers are registered by name and referenced in TOML:

```go
reg.HandleFunc("health.check", myHandler)
```

### Route table

List all registered routes:

```
$ nullspace routes

METHOD  PATH                 HANDLER       FORMAT  MIDDLEWARE  TEMPLATE
GET     /                    template      html                home.html
GET     /api/posts           data.list     json
GET     /api/posts/:id       data.get      json
POST    /api/posts           data.create   json    auth
GET     /posts               data.list     html                posts.html
GET     /posts/:id           data.get      html                post.html
```

## Configuration

Create a `nullspace.toml` in your working directory:

```toml
[request]
addr = ":8080"

[log]
level = "info"    # debug, info, warn, error
format = "text"   # text, json

[response]
default_format = "json"
template_dir = "./templates"

[data.static]
dir = "./public"

[data.file]
dir = "./content"
format = "markdown"

[data.sql]
driver = "sqlite"
dsn = "./data.db"

[tcp]
addr = ":9090"
codec = "json-lines"      # or "length-prefix"

[ipc]
path = "/tmp/myapp.sock"

[modules]
"data.sql" = false        # disabled by default, opt in here
tcp = false               # TCP transport
ipc = false               # Unix socket transport
"data.bridge" = false     # TCP/IPC data commands
```

Environment variables override TOML values. The convention is `NULLSPACE_SECTION_KEY`:

```bash
NULLSPACE_LOG_LEVEL=debug
NULLSPACE_REQUEST_ADDR=:9090
NULLSPACE_DATA_SQL_DSN=postgres://localhost/mydb
```

## Architecture

```
                  +---------------------+
                  |       Kernel        |
                  | (registry, hooks,   |
                  |  config, lifecycle) |
                  +---------------------+
              /      |       |      |       \
         Request  Response  Data  Transport  Logging
         (HTTP    (format   (SQL, (TCP, IPC, (slog
          adapter, resolve, file,  bridge,    adapter,
          router,  JSON,    static) codec)   per-request)
          routing) HTML,
                   text,
                   ANSI)
```

### Modules

Every component implements the `Module` interface:

```go
type Module interface {
    Name() string
    Init(k *Kernel) error
    Start(ctx context.Context) error
    Stop(ctx context.Context) error
}
```

Modules are registered with `kernel.Use()` and managed through a lifecycle: Init, Start, Stop. Each module can declare configuration and be enabled/disabled via TOML config.

### Hook bus

Cross-cutting concerns are handled by a prioritized hook bus. Hooks fire at named lifecycle points and respect module enabled/disabled state per request.

```go
k.Hook("request.before", 10, func(ctx context.Context) error {
    return nil
})

k.HookResolve("response.format.resolve", 20, func(ctx context.Context) (any, bool, error) {
    return "json", true, nil
})
```

### Middleware

Request middleware uses the standard functional pattern:

```go
func timing(next request.HandlerFunc) request.HandlerFunc {
    return func(ctx *request.Context) error {
        start := time.Now()
        err := next(ctx)
        ctx.Logger().Info("request handled", "duration", time.Since(start))
        return err
    }
}

adapter.Use(timing)
```

Middleware can also be registered by name and referenced in TOML routes:

```go
reg.Middleware("auth", myAuthMiddleware)
```

### Format resolution

Response format is resolved through a prioritized module chain:

| Priority | Module | Source |
|----------|--------|--------|
| 10 | `format.route_override` | Route metadata |
| 20 | `format.query_param` | `?format=json` |
| 30 | `format.content_negotiate` | `Accept` header |
| 40 | `format.default` | Configured default |

Built-in formatters:

| Format | Content-Type | Selection |
|--------|-------------|-----------|
| `json` | `application/json` | `Accept: application/json`, `?format=json` |
| `html` | `text/html` | `Accept: text/html`, `?format=html` |
| `text` | `text/plain` | `Accept: text/plain`, `?format=text` |
| `ansi` | `text/plain` | `?format=ansi` (terminal color output) |

The `text` formatter renders structured plain text (key-value headers, blank line, body). The `ansi` formatter adds ANSI escape codes for bold keys, colored status codes, and cyan list indices — useful for TUIs and `curl` debugging.

### TCP and IPC transports

Nullspace supports TCP and Unix socket (IPC) transports alongside HTTP. Both use a command-based router with configurable codecs (JSON-lines or binary length-prefix).

```toml
[modules]
tcp = true
ipc = true

[tcp]
addr = ":9090"
codec = "json-lines"

[ipc]
path = "/tmp/myapp.sock"
```

Register TCP handlers in your module:

```go
tcpRouter, _ := kernel.GetResource[*tcp.Router](k, "tcp.router")
tcpRouter.Handle("ping", func(conn *tcp.Conn, cmd string, payload []byte) error {
    return conn.Send("pong", nil)
})
```

### Data bridge (TCP/IPC)

The `data.bridge` module exposes the same CRUD operations available over HTTP as TCP/IPC commands. Enable it to give non-HTTP clients first-class data access:

```toml
[modules]
"data.bridge" = true
```

Commands use the existing codec framing:

```json
{"command":"data.list","payload":{"collection":"posts"}}
{"command":"data.get","payload":{"collection":"posts","id":"hello-world"}}
{"command":"data.create","payload":{"collection":"posts","body":{"id":"new","title":"Hi"}}}
{"command":"data.update","payload":{"collection":"posts","id":"hello-world","body":{"title":"Updated"}}}
{"command":"data.delete","payload":{"collection":"posts","id":"hello-world"}}
```

All commands reuse the same file module, hooks, and validation as the HTTP handlers.

### Streaming responses

List endpoints support opt-in streaming for large collections. Request streaming with `?stream=true` (HTTP) or `"stream": true` (TCP):

**HTTP** — returns NDJSON (`application/x-ndjson`), one JSON object per line:

```
GET /api/posts?stream=true

{"ID":"hello","Body":"...","title":"Hello World","Meta":{...}}
{"ID":"goodbye","Body":"...","title":"Goodbye","Meta":{...}}
```

**TCP/IPC** — uses envelope messages:

```json
{"command":"data.list.start","payload":{"collection":"posts","total":2}}
{"command":"data.list.item","payload":{"ID":"hello",...}}
{"command":"data.list.item","payload":{"ID":"goodbye",...}}
{"command":"data.list.end","payload":{"collection":"posts","count":2}}
```

Streaming can also be enabled per-route in TOML config:

```toml
[[routing.routes]]
path = "/api/posts"
handler = "data.list"
collection = "posts"
extra = { stream = "true" }
```

### Data modules

**Static files** — serves from a directory as a fallback when no route matches.

**File entities** — stores records as files (markdown with YAML/TOML frontmatter, JSON, or TOML). Directory = collection, filename = ID.

**SQL** — wraps `database/sql` with SQLite as the default driver (pure Go, no CGO). Opt-in via config.

### Config snapshots

Configuration is snapshotted at the start of each request. A request always completes with the same config it started with, even if the live config changes mid-flight.

## Package structure

```
nullspace/
├── cmd/
│   ├── nullspace/      Installable binary (serve / init / routes)
│   └── example/        Example application (library usage)
├── kernel/             Core: module registry, hook bus, config, service locator
├── core/               Required framework modules
│   ├── nslog/          Logging module (slog adapter, per-request loggers)
│   ├── request/        HTTP adapter, router, middleware, context
│   ├── response/       Format resolution, formatters (JSON/HTML/text/ANSI), streaming pipeline
│   ├── routing/        Declarative TOML routing, handler registry, built-in handlers
│   ├── tcp/            TCP transport adapter, command router, codecs
│   └── ipc/            Unix socket transport (wraps TCP)
├── module/             Optional, pluggable modules
│   ├── data/
│   │   ├── static/     Static file serving
│   │   ├── file/       File-based entity storage (with lazy iterator)
│   │   ├── sql/        SQL with SQLite default
│   │   └── bridge/     TCP/IPC data command bridge
│   └── session/        Session management (memory and SQL stores)
├── docs/               Documentation (Sphinx / Read the Docs)
└── specs/              Architecture specifications
```

## Development

This project uses [Task](https://taskfile.dev) as a task runner.

```bash
task test               # Run all tests
task check              # Run fmt check + vet + tests
task run                # Run the example app
task run:nullspace      # Run the nullspace binary from source
task docs:serve         # Build and serve docs at localhost:8000
```

### Building

```bash
task install            # Install to $GOPATH/bin
task dist               # Build release binary to ./bin/
task dist:all           # Cross-compile for all platforms
task docker:build       # Build Docker image
task docker:binary      # Build binary via Docker, copy to ./bin/
```

### Releasing

Releases are managed with [GoReleaser](https://goreleaser.com). A release produces tarballs (macOS, Linux), `.deb` (Debian/Ubuntu), `.rpm` (Rocky/RHEL/Fedora), `.pkg.tar.zst` (Arch Linux), a Homebrew cask, and multi-arch Docker images.

```bash
task release:check      # Validate goreleaser config
task release:snapshot   # Full build locally without publishing
task release            # Tag, build, and publish
```

### Running tests

```bash
task test               # Local
task docker:test        # In a container
task test:coverage      # With coverage report
```

## Documentation

Full documentation is in the `docs/` directory, formatted for Read the Docs.

```bash
task docs:build         # Build HTML docs in a container
task docs:serve         # Build and serve at localhost:8000
```
