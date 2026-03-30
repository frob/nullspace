# Request Specification

The request layer accepts HTTP input and passes it to the kernel. It bridges `net/http` to the framework's internal types using functional middleware.

## Responsibilities

- Accept HTTP requests via `net/http`
- Wrap requests in a framework context
- Execute the functional middleware chain
- Route to dynamic handlers or fall through to static files
- Fire hook bus events at request lifecycle points

## HTTP Adapter

The HTTP adapter implements `net/http.Handler` and bridges to the framework:

```go
type Adapter struct {
    kernel *kernel.Kernel
    chain  MiddlewareChain
}

func (a *Adapter) ServeHTTP(w http.ResponseWriter, r *http.Request) {
    // 1. Snapshot config
    // 2. Build request context
    // 3. Execute middleware chain -> handler
    // 4. Write response
}
```

## Request Context

The framework wraps the standard request into a context that carries:

- The original `*http.Request`
- Config snapshot (immutable for this request)
- Logger (per-request, enriched with request ID)
- Route metadata (after routing)
- Mutable state bag for middleware to share data

```go
type Context struct {
    Request    *http.Request
    Response   http.ResponseWriter
    Config     *config.Snapshot
    Logger     log.Logger
    Route      *RouteMatch
    State      map[string]interface{}
}
```

## Functional Middleware

Middleware follows the standard Go pattern:

```go
type HandlerFunc func(ctx *Context) error
type Middleware  func(HandlerFunc) HandlerFunc
```

### Registration

Middleware is registered on the request adapter:

```go
adapter.Use(logging)       // runs first
adapter.Use(recovery)      // runs second
adapter.Use(auth)          // runs third
```

Middleware can also be registered per-route or per-group.

### Middleware as Modules

Each middleware can be packaged as a module, registering itself during Init:

```go
func (m *AuthMiddleware) Init(k *kernel.Kernel) error {
    adapter := k.Get[*request.Adapter]("request.adapter")
    adapter.Use(m.handler)
    return nil
}
```

This allows middleware to be enabled/disabled through config like any other module.

## Routing

### Route Precedence

1. Dynamic routes (checked first)
2. Static files (fallback)
3. 404 (nothing matched)

This is logged at debug level to aid troubleshooting.

### Route Definition

```go
router.Get("/api/posts", listPosts)
router.Post("/api/posts", createPost)
router.Get("/api/posts/:id", getPost)
```

Routes support:
- Path parameters (`:id`)
- Route metadata (format override, middleware groups)
- Method-based routing

### Static File Fallback

If no dynamic route matches, the request adapter checks the static file provider (data.static module). If a file exists at the requested path, it is served. Otherwise, 404.

## Hook Integration

The request adapter fires hooks at lifecycle points:

| Hook Point | When |
|------------|------|
| `request.received` | Raw request received, before config snapshot |
| `request.routed` | After route match or static fallback determined |
| `request.before` | Before handler execution (after middleware) |
| `request.after` | After handler, before response write |
| `request.error` | Error occurred during handling |
| `request.complete` | Response sent, cleanup phase |

Hooks execute within the request's config snapshot — disabled modules are skipped.
