# Data Specification

The data layer manages data provider lifecycle and policy enforcement. The framework does not dictate data access patterns — modules define their own ports. Two data modules ship out of the box.

## Data Provider Port

All data modules implement the DataProvider interface, which extends Module:

```go
type DataProvider interface {
    Module
    Healthy(ctx context.Context) error
}
```

The kernel manages DataProvider lifecycle (connect, disconnect) and can health-check all registered providers.

## Policy Aspect

Data access control is handled through hook-based policies, not baked into the data interfaces:

- `data.before_read` — policy modules check authorization before reads
- `data.before_write` — policy modules check authorization before writes
- Policy modules are configured and enabled/disabled like any other module

```go
func (p *PolicyModule) Init(k *kernel.Kernel) error {
    k.Hook("data.before_read", 10, p.checkReadAccess)
    k.Hook("data.before_write", 10, p.checkWriteAccess)
    return nil
}
```

## Built-in Module: SQL (`data/sql`)

### Purpose
SQL-based data access with pluggable database drivers.

### Driver Architecture
- The module provides the SQL interface (port)
- Drivers are adapters that plug in (SQLite, Postgres, MySQL)
- SQLite ships as the default — zero external infrastructure required

### Config

```toml
[data.sql]
enabled = true
driver = "sqlite"
dsn = "./data.db"
```

### Interface

```go
type SQLProvider interface {
    DataProvider
    Query(ctx context.Context, query string, args ...interface{}) (*Rows, error)
    Exec(ctx context.Context, query string, args ...interface{}) (Result, error)
    Begin(ctx context.Context) (Tx, error)
}
```

### Migrations
- Migration support via a kernel lifecycle hook (`kernel.after_start`)
- Migrations are SQL files in a configurable directory
- Migration state tracked in the database itself

### Driver Adapters
- **SQLite** (default) — `modernc.org/sqlite` (pure Go, no CGO)
- **PostgreSQL** — `lib/pq` or `jackc/pgx` (separate package)
- **MySQL** — `go-sql-driver/mysql` (separate package)

## Built-in Module: File (`data/file`)

### Purpose
File-based entity storage where each record is a file on disk.

### Storage Format

Supports three file formats:

**Markdown** (with frontmatter):
```markdown
---
title: "My First Post"
date: 2025-01-15
tags: ["go", "framework"]
---

The body content goes here.
```

**JSON:**
```json
{
    "title": "My First Post",
    "date": "2025-01-15",
    "tags": ["go", "framework"],
    "body": "The body content goes here."
}
```

**TOML:**
```toml
title = "My First Post"
date = 2025-01-15
tags = ["go", "framework"]
body = "The body content goes here."
```

### Directory Structure

Entity type maps to directory, filename is the entity ID:

```
content/
├── posts/
│   ├── my-first-post.md
│   └── another-post.md
├── pages/
│   └── about.md
└── authors/
    └── jane.toml
```

### Config

```toml
[data.file]
enabled = true
dir = "./content"
format = "markdown"   # default format for new files
```

### Interface

```go
type FileProvider interface {
    DataProvider
    Read(ctx context.Context, collection string, id string) (*Entity, error)
    List(ctx context.Context, collection string) ([]*Entity, error)
    Write(ctx context.Context, collection string, id string, entity *Entity) error
    Delete(ctx context.Context, collection string, id string) error
}
```

### Entity

```go
type Entity struct {
    ID       string
    Meta     map[string]interface{}   // frontmatter / structured fields
    Body     string                   // body content (markdown body, or empty for JSON/TOML)
    Format   string                   // "markdown", "json", "toml"
}
```

### Performance

- Parse-on-read, no caching (optimize later with data)
- Suitable for content-heavy sites with moderate read volumes
- Git-friendly: files are diffable, mergeable, reviewable

## Built-in Module: Static Files (`data/static`)

### Purpose
Serve static files (CSS, JS, images) from a directory.

### Config

```toml
[data.static]
enabled = true
dir = "./public"
```

### Behavior
- Serves files from the configured directory
- Only used as a fallback when no dynamic route matches (route precedence)
- Sets appropriate Content-Type headers based on file extension
- No directory listing by default
