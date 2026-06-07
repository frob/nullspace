---
title: kitchen-sink
weight: 1
---

The reference application. Wires every core module and includes three
custom modules (auth, forms, chat) to show how to extend the framework.

- **Source:** [`cmd/examples/kitchen-sink/`](https://github.com/frob/nullspace/tree/0.0.x/cmd/examples/kitchen-sink)
- **Form:** library (custom `main.go`)

## Run

```bash
cd cmd/examples/kitchen-sink
go run .
```

Or from the repo root:

```bash
task run
```

The server listens on <http://localhost:8080>.

## Endpoints

| URL                                | Method | Format    | Description                       |
| ---------------------------------- | ------ | --------- | --------------------------------- |
| `/`                                | GET    | HTML      | Home page with links              |
| `/posts`                           | GET    | HTML      | Post list                         |
| `/posts/:id`                       | GET    | HTML      | Single post                       |
| `/forms/contact`                   | GET    | HTML      | Contact form                      |
| `/forms/contact`                   | POST   | HTML/JSON | Submit the form                   |
| `/api/posts`                       | GET    | JSON      | Post list                         |
| `/api/posts/:id`                   | GET    | JSON      | Single post                       |
| `/api/posts?pretty=true`           | GET    | JSON      | Pretty-printed                    |
| `/api/admin/posts`                 | GET    | JSON      | Post list, Basic Auth             |
| `/api/forms/contact/submissions`   | GET    | JSON      | Stored form submissions           |
| `/api/health`                      | GET    | JSON      | Health check                      |
| `/chat`                            | GET    | HTML      | Chat room UI                      |
| `/ws/chat`                         | GET    | WS        | Chat endpoint (upgrade)           |
| `/css/style.css`                   | GET    | CSS       | Static file                       |

## Project layout

```
cmd/examples/kitchen-sink/
├── main.go                  Module wiring
├── nullspace.toml           Project config + project-level routes
├── modules/
│   ├── auth/                Basic Auth middleware module
│   ├── forms/               TOML-defined webform handler
│   └── chat/                WebSocket chat module
├── content/posts/           Markdown posts (YAML frontmatter)
├── templates/               HTML templates
└── public/css/style.css     Static file
```

## What it demonstrates

**Module wiring** — registration order matters. Logging first, then the
request adapter, response pipeline, format resolvers, data modules,
routing, websocket, and the application modules. Each module can read
resources registered earlier through the service locator.

**Declarative routing** — routes live in three places: `nullspace.toml`
(project-level), `modules/auth/routes.toml` (admin routes), and
`modules/forms/routes.toml` (form API). All merged at `kernel.after_init`.

**Format-aware handlers** — `/posts` and `/api/posts` serve the same data;
the route metadata forces HTML or JSON.

**Custom auth middleware** — the `auth` module reads credentials from
config and registers itself by name (`reg.Middleware("auth", ...)`) so
TOML routes can reference it.

**Declarative forms** — the `forms` module reads form definitions from
TOML and produces working HTML+JSON endpoints with field validation, file
storage, and `form.before_submit` / `form.after_submit` hooks for
extensibility.

**WebSocket chat** — the `chat` module registers a WS handler, joins
connections to a room based on route metadata, and broadcasts via the
connection manager.

## Test it

```bash
# Public JSON
curl http://localhost:8080/api/posts

# Auth-protected
curl http://localhost:8080/api/admin/posts                  # 401
curl -u admin:secret http://localhost:8080/api/admin/posts  # 200

# Form submission
curl -X POST http://localhost:8080/forms/contact \
  -H 'Content-Type: application/json' \
  -H 'Accept: application/json' \
  -d '{"name":"Alice","email":"alice@example.com","subject":"Hi","message":"Hello"}'

# Chat: open two browsers on
# http://localhost:8080/chat
```

## See also

- [Tutorial: build a markdown blog]({{< relref "/tutorials/blog" >}})
- [Tutorial: WebSocket chat]({{< relref "/tutorials/websocket-chat" >}})
- [How-to: register a custom handler]({{< relref "/how-to/custom-handler" >}})
