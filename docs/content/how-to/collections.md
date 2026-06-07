---
title: Generate CRUD routes for a collection
weight: 3
---

Use this guide when you want list/get/create/update/delete routes for a data
collection without writing each route by hand.

## Solution

Add a `[[routing.collections]]` block. One declaration generates JSON and HTML
routes for the named collection.

```toml
[[routing.collections]]
name             = "posts"
source           = "data.file"
api_prefix       = "/api"
html_prefix      = ""
list_template    = "posts.html"
item_template    = "post.html"
write_middleware = ["auth"]
```

This generates:

| Method | Path             | Handler        | Middleware |
| ------ | ---------------- | -------------- | ---------- |
| GET    | `/api/posts`     | `data.list`    |            |
| GET    | `/api/posts/:id` | `data.get`     |            |
| POST   | `/api/posts`     | `data.create`  | `auth`     |
| PUT    | `/api/posts/:id` | `data.update`  | `auth`     |
| DELETE | `/api/posts/:id` | `data.delete`  | `auth`     |
| GET    | `/posts`         | `data.list`    |            |
| GET    | `/posts/:id`     | `data.get`     |            |

API routes use JSON; HTML routes use the supplied templates.
`write_middleware` applies to POST/PUT/DELETE only; `read_middleware`
applies to GET only.

Run `nullspace routes` to confirm the routes were generated.

## Variations

### API only, no HTML

Omit `html_prefix` (or set it to a non-empty value the project doesn't expose
to browsers).

```toml
[[routing.collections]]
name       = "events"
source     = "data.file"
api_prefix = "/api"
```

### Read auth, write auth

```toml
[[routing.collections]]
name             = "posts"
source           = "data.file"
api_prefix       = "/api"
read_middleware  = ["session.require"]
write_middleware = ["session.require", "csrf"]
```

### Backed by SQL instead of files

Set `source` to the SQL data module. The same routes are generated, but
handlers read from the configured database.

```toml
[modules]
"data.sql" = true

[[routing.collections]]
name       = "users"
source     = "data.sql"
api_prefix = "/api"
```

## Content directory layout

When `source = "data.file"`, the file data module looks for the collection at
`{data.file.dir}/{name}/`. The default `dir` is `./content`.

```
content/
└── posts/
    ├── hello-world.md
    └── second.md
```

## See also

- [Serve markdown content from files]({{< relref "file-content" >}})
- [Use SQL storage with migrations]({{< relref "sql-data" >}})
- [Stream large lists as NDJSON]({{< relref "streaming" >}})
