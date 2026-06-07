---
title: Wire Nullspace as a Go library
weight: 4
---

This tutorial drops the binary and uses Nullspace as a regular Go
dependency. You will build `main.go` step by step, learn why module
registration order matters, write a tiny custom module that
implements `kernel.Module`, pull resources from the service locator,
and handle SIGINT/SIGTERM for graceful shutdown.

## What you will build

A standalone Go program that:

- Boots a kernel with logging, HTTP request handling, response
  formatting, file-backed data, and declarative routing.
- Adds a custom `greeter` module that registers a handler.
- Reads a config snapshot, looks up the routing registry, and
  registers a handler that uses both.
- Shuts down cleanly on `Ctrl+C`.

By the end you will understand the recipe used by every Nullspace
application, including the `nullspace` binary itself.

## Prerequisites

- Go 1.25+.
- Familiarity with one of the earlier tutorials. The
  [blog tutorial]({{< relref "blog" >}}) is the gentlest.

## Step 1 — New project

```bash
mkdir greeter && cd greeter
go mod init greeter
go get github.com/frob/nullspace
```

## Step 2 — Minimum-viable main.go

Start with the smallest kernel that serves an HTTP request. Create
`main.go`:

```go
package main

import (
	"context"
	"fmt"
	"net/http"
	"os"

	"github.com/frob/nullspace/core/nslog"
	"github.com/frob/nullspace/core/request"
	"github.com/frob/nullspace/core/response"
	"github.com/frob/nullspace/kernel"
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
	if err := k.Init(ctx); err != nil {
		fmt.Fprintln(os.Stderr, "init:", err)
		os.Exit(1)
	}

	adapter.Router().Get("/hello", func(ctx *request.Context) error {
		return pipeline.Write(ctx.Context(), ctx.Writer,
			response.NewResponse(http.StatusOK, map[string]string{
				"message": "hello world",
			}))
	})

	if err := k.Start(ctx); err != nil {
		fmt.Fprintln(os.Stderr, "start:", err)
		os.Exit(1)
	}

	// Block forever for now — we add graceful shutdown later.
	select {}
}
```

Run it and test:

```bash
go run . &
curl http://localhost:8080/hello
# {"message":"hello world"}
kill %1
```

This works because the four registered modules cover the minimum set
needed to handle HTTP requests:

1. `nslog.New()` — logging.
2. `request.NewAdapter()` — the HTTP server and router.
3. `response.NewPipeline()` — turns a `*response.Response` into bytes
   on the wire.
4. `response.NewFormatDefault()` — picks a default format (JSON)
   when no other resolver fires.

## Step 3 — Module registration order

Order matters. Modules later in the list can depend on resources
provided by earlier ones. The canonical order for a full project is:

```
Logging          → nslog.New()
Request adapter  → request.NewAdapter()
Response pipeline → response.NewPipeline()
Format resolvers → response.NewFormatRouteOverride() and friends
Data modules     → static, file, sql
Routing          → routing.New()
WebSocket        → websocket.New()
Application      → your own modules
```

Why this order?

- **Logging first.** Every other module logs during `Init`. The
  logger is provided to the kernel via `nslog.New()`.
- **Request adapter before response pipeline.** The adapter does not
  depend on the pipeline, but if you ever flip the order, mistakes
  surface immediately because the adapter would not have access to
  shared response infrastructure.
- **Format resolvers before routing.** The routing module's built-in
  handlers ask the pipeline to write responses; the pipeline asks the
  resolvers what format to use. Resolvers must be registered first.
- **Data modules before routing.** The routing module's `data.*`
  handlers ask the `file` module for entities. If `file.New()` came
  later, the routing module would fail to wire the handlers.
- **Routing before WebSocket.** WebSocket handlers register
  themselves on the routing registry. The registry is provided by the
  routing module.
- **Application modules last.** Your own modules typically need
  every other resource — the routing registry, the response
  pipeline, the WebSocket manager.

The `kernel.after_init` hook fires after every module's `Init`
returns. The routing and WebSocket modules use this hook to wire up
handlers that earlier-registered application modules might have
contributed. That gives you flexibility — you do not have to
register everything in dependency order if you defer work to
`after_init` — but the order above always works.

## Step 4 — Add a config file

Reading config from a file is one line:

```go
k := kernel.New(kernel.WithConfigFile("nullspace.toml"))
```

Create `nullspace.toml`:

```toml
[request]
addr = ":3000"

[log]
level = "debug"
format = "text"

[response]
default_format = "json"

[greeter]
default_name = "world"
```

Environment variables override TOML keys. The convention is
`NULLSPACE_SECTION_KEY`:

```bash
NULLSPACE_GREETER_DEFAULT_NAME=universe go run .
```

## Step 5 — Write a custom module

A module is anything that satisfies the `kernel.Module` interface:

```go
type Module interface {
    Name() string
    Init(k *Kernel) error
    Start(ctx context.Context) error
    Stop(ctx context.Context) error
}
```

Create `greeter.go` in the same package:

```go
package main

import (
	"context"
	"net/http"

	"github.com/frob/nullspace/core/request"
	"github.com/frob/nullspace/core/response"
	"github.com/frob/nullspace/core/routing"
	"github.com/frob/nullspace/kernel"
)

// greeterModule is a tiny application module. It reads config, pulls
// dependencies from the service locator, and registers a named
// handler that TOML routes can reference.
type greeterModule struct {
	defaultName string
	pipeline    *response.Pipeline
}

// Name is the module's stable identifier in logs and the config tree.
func (m *greeterModule) Name() string { return "greeter" }

// Config tells the kernel how to load this module's config block and
// whether it's enabled by default.
func (m *greeterModule) Config() kernel.ModuleConfig {
	return kernel.ModuleConfig{
		Key: "greeter",
		Default: map[string]any{
			"default_name": "world",
		},
		DefaultEnabled: true,
	}
}

// Init runs once at startup. Look up dependencies, decode config,
// and register handlers/hooks.
func (m *greeterModule) Init(k *kernel.Kernel) error {
	// Decode our config block.
	var cfg struct {
		DefaultName string `toml:"default_name"`
	}
	_ = k.Config().Decode("greeter", &cfg)
	if cfg.DefaultName == "" {
		cfg.DefaultName = "world"
	}
	m.defaultName = cfg.DefaultName

	// Look up the response pipeline.
	pipeline, err := kernel.GetResource[*response.Pipeline](k, "response.pipeline")
	if err != nil {
		return err
	}
	m.pipeline = pipeline

	// Look up the routing registry and register a named handler.
	reg, err := kernel.GetResource[*routing.Registry](k, "routing.registry")
	if err != nil {
		return err
	}
	reg.HandleFunc("greeter.hello", m.handle)

	k.Logger().Info("greeter ready", "default_name", m.defaultName)
	return nil
}

// Start runs after every module has finished Init. Use it to open
// connections, launch goroutines, etc. We have nothing to do.
func (m *greeterModule) Start(ctx context.Context) error { return nil }

// Stop runs in reverse registration order when the kernel shuts down.
// Use it to flush, close, and cancel.
func (m *greeterModule) Stop(ctx context.Context) error { return nil }

func (m *greeterModule) handle(ctx *request.Context) error {
	name := ctx.Request.URL.Query().Get("name")
	if name == "" {
		name = m.defaultName
	}
	return m.pipeline.Write(ctx.Context(), ctx.Writer,
		response.NewResponse(http.StatusOK, map[string]string{
			"message": "hello, " + name,
		}))
}
```

Two things worth highlighting:

- **`Config()` is optional.** If a module's `Config` method returns a
  `kernel.ModuleConfig` with `DefaultEnabled: false`, it is skipped
  unless the user opts in via `[modules]` in the config file. Any
  module can be enabled or disabled this way.
- **`kernel.GetResource[T]` is the entry point to the service
  locator.** A module that provides a service calls `k.Provide(key,
  value)` in `Init`; a module that consumes one calls
  `kernel.GetResource[T](k, key)`. Built-in providers and their keys
  are listed in the [reference]({{< relref "/reference" >}}).

## Step 6 — Reference the handler from a route

Add a route to `nullspace.toml`:

```toml
[[routing.routes]]
path = "/hello"
handler = "greeter.hello"
format = "json"
```

Update `main.go` to register the routing module and the greeter:

```go
import (
	// ... existing imports
	"github.com/frob/nullspace/core/routing"
)

func main() {
	k := kernel.New(kernel.WithConfigFile("nullspace.toml"))

	adapter := request.NewAdapter()
	pipeline := response.NewPipeline()

	k.Use(nslog.New())
	k.Use(adapter)
	k.Use(pipeline)
	k.Use(response.NewFormatRouteOverride())
	k.Use(response.NewFormatQueryParam())
	k.Use(response.NewFormatContentNegotiate())
	k.Use(response.NewFormatDefault())
	k.Use(routing.New())
	k.Use(&greeterModule{})

	ctx := context.Background()
	if err := k.Init(ctx); err != nil {
		fmt.Fprintln(os.Stderr, "init:", err)
		os.Exit(1)
	}
	if err := k.Start(ctx); err != nil {
		fmt.Fprintln(os.Stderr, "start:", err)
		os.Exit(1)
	}

	k.Logger().Info("greeter running", "addr", ":3000")

	// Graceful shutdown comes next.
	select {}
}
```

Drop the inline `adapter.Router().Get("/hello", ...)` call — the
route now comes from TOML. Run it:

```bash
go run . &
curl 'http://localhost:3000/hello'
# {"message":"hello, world"}
curl 'http://localhost:3000/hello?name=Ada'
# {"message":"hello, Ada"}
kill %1
```

## Step 7 — Graceful shutdown

Replace the `select {}` at the bottom of `main` with a signal handler
that calls `k.Stop`:

```go
import (
	// ... existing
	"os/signal"
	"syscall"
	"time"
)

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	k.Logger().Info("shutting down")
	if err := k.Stop(shutdownCtx); err != nil {
		fmt.Fprintln(os.Stderr, "stop:", err)
	}
```

`k.Stop` invokes each module's `Stop` method in reverse registration
order. The HTTP request adapter waits for in-flight requests to
finish or for the context to time out. WebSocket connections receive
a graceful close.

Now `Ctrl+C` cleanly drains the server.

## Step 8 — End-to-end main.go

For reference, the final `main.go`:

```go
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/frob/nullspace/core/nslog"
	"github.com/frob/nullspace/core/request"
	"github.com/frob/nullspace/core/response"
	"github.com/frob/nullspace/core/routing"
	"github.com/frob/nullspace/kernel"
)

func main() {
	k := kernel.New(kernel.WithConfigFile("nullspace.toml"))

	// Logging
	k.Use(nslog.New())

	// Request
	k.Use(request.NewAdapter())

	// Response
	k.Use(response.NewPipeline())

	// Format resolvers (priority order: route → query → accept → default)
	k.Use(response.NewFormatRouteOverride())
	k.Use(response.NewFormatQueryParam())
	k.Use(response.NewFormatContentNegotiate())
	k.Use(response.NewFormatDefault())

	// Routing
	k.Use(routing.New())

	// Application modules
	k.Use(&greeterModule{})

	ctx := context.Background()
	if err := k.Init(ctx); err != nil {
		fmt.Fprintln(os.Stderr, "init:", err)
		os.Exit(1)
	}
	if err := k.Start(ctx); err != nil {
		fmt.Fprintln(os.Stderr, "start:", err)
		os.Exit(1)
	}
	k.Logger().Info("greeter running")

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	k.Logger().Info("shutting down")
	if err := k.Stop(shutdownCtx); err != nil {
		fmt.Fprintln(os.Stderr, "stop:", err)
	}
}
```

This template covers nearly every Nullspace application. Add data
modules between the format resolvers and the routing module; add the
WebSocket module between routing and your application modules; add
your own modules at the bottom.

{{< hint info >}}
The kitchen-sink example at
`cmd/examples/kitchen-sink/main.go` follows this exact recipe and
adds the `auth`, `forms`, and `chat` modules. Use it as a working
reference whenever the order of operations is unclear.
{{< /hint >}}

## Where to go next

- [Build a JSON API]({{< relref "json-api" >}}) — add a `[[routing.collections]]`
  block to this project and let it serve CRUD.
- [Add real-time chat with WebSockets]({{< relref "websocket-chat" >}}) —
  plug the WebSocket module into this `main.go`.
- [How-to guides]({{< relref "/how-to" >}}) — middleware, hooks,
  custom formatters, SQL storage, OIDC, and more.
- [Reference]({{< relref "/reference" >}}) — every module, every
  service-locator key, every hook point.
