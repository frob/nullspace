---
title: Build a JSON API
weight: 2
---

This tutorial builds a JSON API for a `books` collection without
writing any handler code. You will use the `nullspace` binary,
declarative TOML routes, the built-in `data.*` handlers, and the
streaming response mode for large lists.

## What you will build

An API at `/api/books` that supports the full CRUD lifecycle: list,
get, create, update, delete. You will exercise it with `curl` and
finish by streaming a few hundred records as NDJSON.

## Prerequisites

- The `nullspace` binary installed (`go install
  github.com/frob/nullspace/cmd/nullspace@latest`).
- Completion of the [blog tutorial]({{< relref "blog" >}}) is helpful
  but not required.
- `curl` and `jq` for testing.

## Step 1 — Scaffold

```bash
mkdir bookstore && cd bookstore
nullspace init
```

You do not need the HTML pieces for this tutorial. The default
scaffold includes them; ignore them or delete `templates/` if you want
a leaner project.

## Step 2 — Replace the config

Open `nullspace.toml` and replace it with the following. Two changes
matter: a routing group for `/api`, and a collection that only emits
API routes (no `html_prefix`).

```toml
[request]
addr = ":8080"

[log]
level = "info"
format = "text"

[response]
default_format = "json"

[data.file]
dir = "./content"
format = "markdown"

# Shared settings for every API route.
[routing.groups.api]
prefix = "/api"
format = "json"

# Collection that auto-generates CRUD routes.
[[routing.collections]]
name = "books"
source = "data.file"
api_prefix = "/api"
# html_prefix omitted → no HTML routes
```

That collection block generates:

- `GET    /api/books`
- `GET    /api/books/:id`
- `POST   /api/books`
- `PUT    /api/books/:id`
- `DELETE /api/books/:id`

Every route uses a different built-in handler (`data.list`,
`data.get`, `data.create`, `data.update`, `data.delete`). All of them
read and write files under `content/books/`.

## Step 3 — Seed some data

Create the collection directory:

```bash
mkdir content/books
```

Add two records by hand:

```bash
cat > content/books/the-pragmatic-programmer.md <<'EOF'
---
title: The Pragmatic Programmer
author: Andrew Hunt, David Thomas
year: 1999
tags:
  - classic
  - software
---
EOF

cat > content/books/structure-and-interpretation.md <<'EOF'
---
title: Structure and Interpretation of Computer Programs
author: Harold Abelson, Gerald Sussman
year: 1985
tags:
  - classic
  - scheme
---
EOF
```

The filename (without `.md`) is the entity ID.

## Step 4 — Verify the route table

```bash
nullspace routes
```

```
METHOD  PATH              HANDLER       FORMAT
DELETE  /api/books/:id    data.delete   json
GET     /api/books        data.list     json
GET     /api/books/:id    data.get      json
POST    /api/books        data.create   json
PUT     /api/books/:id    data.update   json
```

Every route was generated from the single `[[routing.collections]]`
block.

## Step 5 — Read

Start the server in one terminal:

```bash
nullspace
```

In another, list the books:

```bash
curl -s http://localhost:8080/api/books | jq
```

```json
[
  {
    "ID": "structure-and-interpretation",
    "Body": "",
    "title": "Structure and Interpretation of Computer Programs",
    "author": "Harold Abelson, Gerald Sussman",
    "year": 1985,
    "tags": ["classic", "scheme"]
  },
  {
    "ID": "the-pragmatic-programmer",
    "Body": "",
    "title": "The Pragmatic Programmer",
    "author": "Andrew Hunt, David Thomas",
    "year": 1999,
    "tags": ["classic", "software"]
  }
]
```

Fetch a single record:

```bash
curl -s http://localhost:8080/api/books/the-pragmatic-programmer | jq
```

Pretty-print is built in via `?pretty=true`:

```bash
curl -s 'http://localhost:8080/api/books?pretty=true'
```

## Step 6 — Create

`POST` a new book. The body is JSON; frontmatter fields go at the top
level alongside `ID` and `Body`.

```bash
curl -s -X POST http://localhost:8080/api/books \
  -H 'Content-Type: application/json' \
  -d '{
    "ID": "the-mythical-man-month",
    "title": "The Mythical Man-Month",
    "author": "Fred Brooks",
    "year": 1975,
    "tags": ["classic", "management"]
  }' | jq
```

The server writes `content/books/the-mythical-man-month.md` and
returns the stored entity. Verify on disk:

```bash
cat content/books/the-mythical-man-month.md
```

```
---
ID: the-mythical-man-month
author: Fred Brooks
tags:
- classic
- management
title: The Mythical Man-Month
year: 1975
---
```

## Step 7 — Update

`PUT` overwrites the record at a given ID:

```bash
curl -s -X PUT http://localhost:8080/api/books/the-mythical-man-month \
  -H 'Content-Type: application/json' \
  -d '{
    "title": "The Mythical Man-Month: Essays on Software Engineering",
    "author": "Fred Brooks",
    "year": 1975,
    "tags": ["classic", "management", "essays"]
  }' | jq
```

## Step 8 — Delete

```bash
curl -s -X DELETE http://localhost:8080/api/books/the-mythical-man-month -i
```

```
HTTP/1.1 204 No Content
```

The file is gone from disk.

## Step 9 — Stream large lists

`GET /api/books?stream=true` returns NDJSON (one JSON object per line)
with `Content-Type: application/x-ndjson`. The file module iterates
lazily, so memory usage stays flat regardless of collection size.

First, generate enough data to make streaming interesting. Stop the
server, then:

```bash
for i in $(seq 1 500); do
  cat > "content/books/book-$i.md" <<EOF
---
title: Book $i
author: Author $i
year: $((1900 + RANDOM % 125))
---
EOF
done
```

Restart `nullspace`. Then:

```bash
curl -s 'http://localhost:8080/api/books?stream=true' | head -3
```

```
{"ID":"book-1","Body":"","author":"Author 1","title":"Book 1","year":1972}
{"ID":"book-10","Body":"","author":"Author 10","title":"Book 10","year":2009}
{"ID":"book-100","Body":"","author":"Author 100","title":"Book 100","year":1955}
```

Compare the Content-Type:

```bash
curl -s -I 'http://localhost:8080/api/books?stream=true' | grep -i content-type
# Content-Type: application/x-ndjson
```

Streaming can also be forced per-route in TOML using the `extra` table,
so callers do not need to opt in:

```toml
[[routing.routes]]
group = "api"
path = "/books"
handler = "data.list"
collection = "books"
extra = { stream = "true" }
```

## Step 10 — End-to-end curl session

Stop the server, clean up the test data, and run through the full
lifecycle in one terminal session:

```bash
rm content/books/book-*.md
nullspace &
sleep 1

BASE=http://localhost:8080/api/books

# List
curl -s $BASE | jq 'length'

# Create
curl -s -X POST $BASE -H 'Content-Type: application/json' \
  -d '{"ID":"crafting-interpreters","title":"Crafting Interpreters","author":"Robert Nystrom","year":2021}' | jq

# Read
curl -s $BASE/crafting-interpreters | jq

# Update
curl -s -X PUT $BASE/crafting-interpreters -H 'Content-Type: application/json' \
  -d '{"title":"Crafting Interpreters","author":"Robert Nystrom","year":2021,"tags":["compilers"]}' | jq

# Delete
curl -s -X DELETE $BASE/crafting-interpreters -i | head -1

# Confirm gone
curl -s -o /dev/null -w '%{http_code}\n' $BASE/crafting-interpreters

kill %1
```

You should see `2` (initial count), the created record, the updated
record with a `tags` field, `HTTP/1.1 204 No Content`, and `404`.

{{< hint info >}}
The same operations are available over TCP and Unix sockets via the
`data.bridge` module — useful for CLIs, daemons, and machine-to-machine
clients that should not use HTTP. See
[Expose data over TCP/IPC]({{< relref "/how-to" >}}).
{{< /hint >}}

## Where to go next

- [Add real-time chat with WebSockets]({{< relref "websocket-chat" >}}) —
  the third transport in the same kernel.
- [Wire Nullspace as a library]({{< relref "library" >}}) — for
  validation, business logic, or anything beyond CRUD over files.
- [How-to guides]({{< relref "/how-to" >}}) — SQL storage, custom
  middleware, hooks into the request lifecycle.
