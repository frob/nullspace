# Nullspace Example Application

This is a reference application that demonstrates how to use the Nullspace framework as a Go library. It wires all core modules together and includes two custom example modules (auth and forms) that show how to extend the framework.

If you're looking for the convention-based binary that serves projects from a directory (like Hugo), see `cmd/nullspace/` instead. This example is for developers who need full control over module wiring, route definitions, and middleware.

## Run

```bash
cd cmd/examples/kitchen-sink
go run .
```

Or from the project root:

```bash
task run
```

## Endpoints

| URL | Method | Format | Description |
|-----|--------|--------|-------------|
| `/` | GET | HTML | Home page with links to all features |
| `/posts` | GET | HTML | Post list rendered with templates |
| `/posts/:id` | GET | HTML | Single post |
| `/forms/contact` | GET | HTML | Contact form |
| `/forms/contact` | POST | HTML/JSON | Submit the contact form |
| `/api/posts` | GET | JSON | Post list |
| `/api/posts/:id` | GET | JSON | Single post |
| `/api/posts?pretty=true` | GET | JSON | Pretty-printed |
| `/api/admin/posts` | GET | JSON | Post list (requires Basic Auth) |
| `/api/forms/contact/submissions` | GET | JSON | Stored form submissions |
| `/api/health` | GET | JSON | Health check |
| `/chat` | GET | HTML | Chat room UI |
| `/ws/chat` | GET | WebSocket | Chat endpoint (upgrade) |
| `/css/style.css` | GET | CSS | Static file |

## Project Structure

```
cmd/examples/kitchen-sink/
├── main.go                  Application entry point — module wiring
├── nullspace.toml           Project config + project-level routes
├── modules/
│   ├── auth/
│   │   ├── module.go        Basic Auth middleware module
│   │   └── routes.toml      Auth module's route definitions
│   ├── forms/
│   │   ├── module.go        Webform handler module
│   │   └── routes.toml      Forms module's route definitions
│   └── chat/
│       └── module.go        WebSocket chat module
├── content/
│   └── posts/
│       ├── hello-world.md   Sample post (markdown + YAML frontmatter)
│       └── architecture.md  Sample post
├── templates/
│   ├── home.html            Home page template
│   ├── posts.html           Post list template
│   ├── post.html            Single post template
│   ├── form.html            Form rendering template
│   ├── form_success.html    Form submission success template
│   └── chat.html            Chat room template (WebSocket client)
└── public/
    └── css/
        └── style.css        Static stylesheet
```

## What This Demonstrates

### Module Wiring (`main.go`)

The entry point registers modules in dependency order. This order matters — modules that provide resources must be registered before modules that consume them:

```go
k.Use(nslog.New())              // 1. Logging (needed by everything)
k.Use(adapter)                  // 2. Request adapter (provides router)
k.Use(pipeline)                 // 3. Response pipeline (provides formatters)
k.Use(response.NewFormat...())  // 4. Format resolvers
k.Use(static.New())             // 5. Static file fallback
k.Use(file.New())               // 6. File data (provides content)
k.Use(routing.New())            // 7. Routing (provides registry, resolves TOML routes)
k.Use(websocket.New())          // 8. WebSocket (registers WS handlers on registry)
k.Use(auth.New())               // 9. Auth (registers middleware + loads routes.toml)
k.Use(forms.New())              // 10. Forms (registers form routes + loads routes.toml)
k.Use(chat.New())               // 11. Chat (registers WS chat handler)
k.Use(&appModule{})             // 12. App (registers custom handlers)
```

### Declarative Routing

Routes are defined in three places:

- **`nullspace.toml`** — project-level routes, groups, and collections
- **`modules/auth/routes.toml`** — admin routes (owned by the auth module)
- **`modules/forms/routes.toml`** — form submission API (owned by the forms module)

All routes are merged at startup during `kernel.after_init`. List them:

```bash
nullspace routes
```

### Format Resolution

HTML and JSON routes serve the same data in different formats. The posts collection auto-generates both:

```go
// HTML — format forced via route metadata
router.Get("/posts", listPosts, request.WithMeta("format", "html"))

// JSON — same data, different format
router.Get("/api/posts", listPosts, request.WithMeta("format", "json"))
```

### Auth Module (`modules/auth/`)

A middleware-as-module that protects routes under a configurable prefix. Shows:

- Registering middleware via the service locator
- Config-driven credentials and path prefix
- Passing identity to handlers via `ctx.SetState("auth.user", username)`

Config in `nullspace.toml`:

```toml
[auth]
prefix = "/api/admin"

[auth.users]
admin = "secret"
```

Test:

```bash
curl http://localhost:8080/api/admin/posts              # 401
curl -u admin:secret http://localhost:8080/api/admin/posts  # 200
```

### Forms Module (`modules/forms/`)

A declarative webform handler. Shows:

- Registering routes from a module via the service locator
- Parsing both form-encoded and JSON request bodies
- Field validation with error feedback
- Storing submissions via the file data module
- Firing custom hooks (`form.before_submit`, `form.after_submit`) for extensibility
- Using the response pipeline for format-aware output

Forms are defined entirely in TOML — adding a form requires zero code changes:

```toml
[forms.forms.contact]
title = "Contact Us"
success_message = "Thanks!"
store = "form-submissions"

[[forms.forms.contact.fields]]
name = "email"
label = "Email"
type = "email"
required = true
```

Test:

```bash
# HTML form
curl http://localhost:8080/forms/contact

# JSON submission
curl -X POST http://localhost:8080/forms/contact \
  -H 'Content-Type: application/json' \
  -H 'Accept: application/json' \
  -d '{"name":"Alice","email":"alice@example.com","subject":"General Inquiry","message":"Hello"}'

# View submissions
curl http://localhost:8080/api/forms/contact/submissions?pretty=true
```

### Chat Module (`modules/chat/`)

A WebSocket chat room. Shows:

- Registering WebSocket handlers via `wsMod.HandleFunc`
- Room-based broadcast via the connection manager
- Passing query parameters into connection state via middleware
- Using `extra` route metadata for room auto-join

Open two browser tabs to `http://localhost:8080/chat` and send messages between them.

### File-Based Content

Posts are stored as markdown files with YAML frontmatter in `content/posts/`. The file data module parses them into entities with structured metadata and body content. No database needed.

### Static File Fallback

Static files in `public/` are served when no dynamic route matches. Routes always take precedence — a route at `/css/style.css` would override the static file.

### Per-Request Logging

Every request gets a unique ID and structured log output with method, path, status, and duration:

```
level=INFO msg="request received" request_id=a1b2c3 method=GET path=/api/posts
level=INFO msg="request complete" request_id=a1b2c3 method=GET path=/api/posts status=200 duration_ms=1
```

### Graceful Shutdown

The app listens for SIGINT/SIGTERM and stops all modules in reverse registration order.

## Configuration

All configuration is in `nullspace.toml`. See the comments in that file for available options. Environment variables override TOML values:

```bash
NULLSPACE_REQUEST_ADDR=:3000 go run .
NULLSPACE_LOG_LEVEL=warn go run .
```

## Further Reading

- [Architecture docs](../../docs/explanation/architecture.rst) — hexagonal architecture and request lifecycle
- [Custom modules guide](../../docs/how-to/custom-modules.rst) — how to write your own modules
- [Example modules guide](../../docs/explanation/example-modules.rst) — detailed walkthrough of the auth and forms modules
