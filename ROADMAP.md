# Nullspace Roadmap: CMS + TUI Content Backend

## Phase 1 — Query Foundations

The data layer returns everything or nothing. Before any frontend (web or terminal) is viable, reads need to be filterable, sortable, and pageable.

### 1.1 Pagination, filtering, and sorting on `data.list`

Support query parameters on the built-in list handler:

- `?page=1&per_page=20` — offset pagination
- `?sort=-created_at` — sort by meta field, `-` prefix for descending
- `?filter[status]=published` — filter by meta field equality
- Return pagination metadata in response (`total`, `page`, `per_page`, `pages`)

Wire filtering through a `data.before_list` hook so modules can extend filter logic (e.g., draft/publish visibility).

### 1.2 Entity timestamps

Expose `created_at` and `modified_at` in entity meta. File backend derives these from filesystem timestamps on first read and persists them in frontmatter on write. SQL backend uses column defaults. These fields power sorting, cache validation, and incremental sync later.

### 1.3 Request body parsing middleware

Add a middleware that decodes the request body based on `Content-Type` and attaches the result to the request context:

- `application/json` — JSON decode
- `application/x-www-form-urlencoded` — form decode
- `multipart/form-data` — multipart parse (fields + files)

Register as a named middleware (`body.parse`) so write routes can opt in.

---

## Phase 2 — Write Path Hardening

With reads in shape, make writes robust enough for real content management.

### 2.1 Input validation

A validation layer that runs before `data.create` / `data.update`:

- Per-collection schema defined in TOML (field names, types, required, constraints)
- Programmatic validators registered via `data.Validate("collection", fn)`
- Structured error responses: `{ "errors": [{ "field": "title", "message": "required" }] }`
- Fires via `data.before_write` hook so it composes with other pre-write logic

### 2.2 Draft/publish workflow

Convention-based content lifecycle:

- `status` meta field: `draft`, `published`, `archived`
- `data.list` defaults to `status=published` for unauthenticated requests
- `data.list` returns all statuses for authenticated requests (or with `?drafts=true` + auth)
- `published_at` timestamp set automatically on first publish
- Implemented as a hook module so it can be disabled for projects that don't need it

### 2.3 Content versioning

Store previous versions on write:

- File backend: `.revisions/{collection}/{id}/{timestamp}.md`
- SQL backend: `revisions` table with entity ID, timestamp, and full snapshot
- Configurable retention (`max_revisions = 10`)
- `GET /api/{collection}/{id}/revisions` — list versions
- `GET /api/{collection}/{id}/revisions/{timestamp}` — get specific version
- Triggered via `data.after_write` hook

### 2.4 Bulk operations

Batch endpoints for collections:

- `POST /api/{collection}/batch` — create/update multiple entities
- `DELETE /api/{collection}/batch` — delete by ID list
- Atomic (all-or-nothing) with structured error reporting per item
- Fires individual hooks per entity so validation and versioning still apply

---

## Phase 3 — Transport Parity

The TCP and IPC transports exist but can't access content. Bridge them so TUI clients get first-class access without HTTP.

### 3.1 Data handler bridge for TCP/IPC

Map built-in data handlers to TCP/IPC commands:

- `{"cmd":"data.list", "collection":"posts", "page":1, "per_page":20}`
- `{"cmd":"data.get", "collection":"posts", "id":"hello-world"}`
- `{"cmd":"data.create", "collection":"posts", "body":{...}}`
- `{"cmd":"data.update", "collection":"posts", "id":"hello-world", "body":{...}}`
- `{"cmd":"data.delete", "collection":"posts", "id":"hello-world"}`

Reuse the same handler logic, validation, hooks, and versioning. The transport layer translates between wire format and the internal handler interface.

### 3.2 Plain-text response formatter

Add `text` and `ansi` formatters to the response pipeline:

- `text` — structured plain text (key: value headers, blank line, body)
- `ansi` — same as text with ANSI escape codes for color/bold
- Selectable via `?format=text`, `Accept: text/plain`, or route config
- Useful for TUIs, CLI tools, and `curl` debugging

### 3.3 Streaming / incremental responses

Support chunked delivery for large result sets:

- HTTP: `Transfer-Encoding: chunked` with NDJSON (one JSON object per line)
- TCP/IPC: already line-delimited, stream entities as they're read
- Opt-in via `?stream=true` or route config
- Add a `StreamFormatter` interface alongside the existing `Formatter`

---

## Phase 4 — Live Updates

Connect the internal hook bus to external clients so frontends and TUIs get real-time content changes.

### 4.1 Content watch via WebSocket

A `content.watch` WebSocket handler:

- Client subscribes: `{"action":"subscribe", "collection":"posts"}`
- Server pushes on content change: `{"event":"updated", "collection":"posts", "id":"hello-world"}`
- Events sourced from `data.after_write` / `data.after_delete` hooks
- Optional: subscribe to specific IDs for single-document editing

### 4.2 Content watch via TCP/IPC

Same subscription model over TCP and IPC transports:

- `{"cmd":"content.subscribe", "collection":"posts"}`
- Push events use the same format as WebSocket
- Connection-scoped subscriptions, cleaned up on disconnect

### 4.3 Incremental sync endpoint

For clients that poll rather than subscribe:

- `GET /api/{collection}?since=2026-03-30T00:00:00Z` — entities modified after timestamp
- ETag support: `ETag` header on responses, `If-None-Match` on requests
- `304 Not Modified` when nothing changed
- Powers offline-first TUI clients that cache locally and sync on connect

---

## Phase 5 — Auth and Access Control

Move beyond session-only auth to support headless clients and fine-grained permissions.

### 5.1 Token-based authentication

API token middleware:

- `Authorization: Bearer <token>` header validation
- Support both opaque tokens (database-backed) and JWTs (stateless)
- Populate request context with user identity and roles
- Register as named middleware (`auth.token`) for route-level opt-in
- Token management endpoints: create, list, revoke

### 5.2 Role-based access control

Permission layer on top of authentication:

- Roles defined in config: `admin`, `editor`, `viewer` (customizable)
- Per-collection permissions: `posts.create = ["admin", "editor"]`
- Middleware: `auth.require("editor")` or declarative in route config
- Hook-based so modules can extend (e.g., "authors can only edit own posts")

---

## Phase 6 — Media and External Integration

Round out the CMS feature set with file handling and external system notifications.

### 6.1 Media upload module

Handle binary content:

- `POST /api/media` — multipart upload
- Configurable storage backend (local disk, S3-compatible)
- Auto-generate metadata: MIME type, dimensions (images), file size
- Thumbnail generation for images (configurable sizes)
- Media entities accessible via the same data layer (`data.get` with collection `media`)

### 6.2 Outbound webhooks

Notify external systems on content changes:

- Configure webhook URLs per event type in TOML
- Events: `content.created`, `content.updated`, `content.deleted`, `content.published`
- Retry with exponential backoff on failure
- HMAC signature on payloads for verification
- Powered by `data.after_write` / `data.after_delete` hooks

### 6.3 Relationship / reference resolution

Support references between content types:

- `ref` field type in entity meta: `author: { "$ref": "authors/jane" }`
- Resolve on read with `?expand=author` query parameter
- Circular reference protection (max depth)
- Batch resolution to avoid N+1 reads

---

## Phase 7 — Operational Maturity

Production readiness for long-running deployments.

### 7.1 Caching layer

Multi-level caching:

- In-memory LRU cache for entity reads (configurable TTL and size)
- HTTP cache headers (`Cache-Control`, `ETag`, `Last-Modified`)
- Cache invalidation via `data.after_write` / `data.after_delete` hooks
- Per-collection cache policy in config

### 7.2 Structured query support for SQL backend

Thin query builder for the SQL data module:

- Structured query type: `Collection`, `Filters`, `Sort`, `Limit`, `Offset`
- Same query interface as file backend so handlers are backend-agnostic
- Migration support: versioned schema changes with up/down
- Connection pooling configuration

### 7.3 Observability

Beyond request logging:

- Prometheus-compatible metrics endpoint (`/metrics`)
- Request duration, status code distribution, active connections
- Per-collection read/write counters
- Optional OpenTelemetry trace propagation
- Health check endpoint (`/health`) with module status
