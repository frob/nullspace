---
title: Build a markdown blog
weight: 1
---

This tutorial walks through building a markdown blog from a blank
directory. You will use the `nullspace` binary, which serves a project
by convention from the current working directory — no Go code required.
By the end you will have HTML pages and a JSON API for posts plus a
second content collection of your own.

## What you will build

A site with two collections, `posts` and `notes`. Each collection gets
a list page, a detail page, and a JSON endpoint. The home page links
into them. A custom template displays tags on the post list.

## Prerequisites

- Go 1.25+ installed.
- A terminal.
- `curl` for testing JSON endpoints.

## Step 1 — Install the binary

Install the `nullspace` binary into `$GOPATH/bin`:

```bash
go install github.com/frob/nullspace/cmd/nullspace@latest
```

Verify the install:

```bash
nullspace version
```

You should see something like `nullspace dev`. Packaged releases (deb,
rpm, Homebrew, Arch) print the tagged version instead.

{{< hint info >}}
If `nullspace` is not on your `PATH`, add `$(go env GOPATH)/bin` to it.
{{< /hint >}}

## Step 2 — Scaffold a project

Create an empty directory and run `init`:

```bash
mkdir myblog && cd myblog
nullspace init
```

The command creates this layout:

```
myblog/
├── nullspace.toml
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

`nullspace init` is idempotent: existing files are never overwritten.

## Step 3 — Start the server

Run the binary with no arguments:

```bash
nullspace
```

You should see a log line like:

```
level=INFO msg="nullspace running" version=dev addr=:8080
```

Visit [http://localhost:8080](http://localhost:8080). The home page
lists collections discovered under `content/`. Right now that is just
`posts`.

Click through to `/posts`, then to `/posts/hello-world`. These are the
auto-generated HTML routes.

## Step 4 — View the JSON output

The same data is served as JSON under `/api`. In another terminal:

```bash
curl -s http://localhost:8080/api/posts | head
```

```json
[{"ID":"hello-world","Body":"Welcome to your new Nullspace site.\n","title":"Hello World","date":"2025-01-01","tags":["welcome"]}]
```

Each post is one object. Frontmatter keys appear at the top level
alongside the framework's `ID` and `Body` fields.

A single post:

```bash
curl -s http://localhost:8080/api/posts/hello-world
```

Format selection works two other ways: an `Accept` header or a `?format=`
query parameter.

```bash
curl -s -H 'Accept: application/json' http://localhost:8080/posts/hello-world
curl -s 'http://localhost:8080/posts/hello-world?format=json'
```

Both return the JSON representation even though the path is the HTML one.
See [Format resolution]({{< relref "/reference" >}}) for the priority order.

## Step 5 — Write a new post

Stop the server (`Ctrl+C`) and create another post:

```bash
cat > content/posts/second-post.md <<'EOF'
---
title: Second Post
date: 2026-06-06
tags:
  - update
  - tutorial
---

This post was added to the running blog without touching any code.
EOF
```

Restart `nullspace`. The new post appears at `/posts/second-post` and
in the JSON list at `/api/posts`. The file data module reads the
`content/posts/` directory on every list request — there is no build
step.

## Step 6 — Add a second collection

Create a new collection by adding a subdirectory under `content/`:

```bash
mkdir content/notes
cat > content/notes/today.md <<'EOF'
---
title: Today
date: 2026-06-06
---

Short-form thoughts go in notes.
EOF
```

Restart `nullspace`. The binary auto-discovers any subdirectory of
`content/` and registers HTML and JSON routes for it:

- `/notes` and `/notes/:id` — HTML
- `/api/notes` and `/api/notes/:id` — JSON

Visit `/notes`. You will get a "template not found" error: `notes.html`
does not exist yet. The convention is `<collection>.html` for the list
and the singular form (collection name with the trailing `s` removed)
for the item. For `notes` that means `notes.html` and `note.html`.

Create them:

```bash
cat > templates/notes.html <<'EOF'
<!DOCTYPE html>
<html>
<head><title>Notes</title><link rel="stylesheet" href="/css/style.css"></head>
<body>
  <nav><a href="/">Home</a></nav>
  <h1>Notes</h1>
  {{range .Items}}
  <article>
    <h2><a href="/notes/{{.ID}}">{{.title}}</a></h2>
    <p>{{.date}}</p>
  </article>
  {{end}}
</body>
</html>
EOF

cat > templates/note.html <<'EOF'
<!DOCTYPE html>
<html>
<head><title>{{.title}}</title><link rel="stylesheet" href="/css/style.css"></head>
<body>
  <nav><a href="/">Home</a> / <a href="/notes">Notes</a></nav>
  <article>
    <h1>{{.title}}</h1>
    <p>{{.date}}</p>
    <div>{{.Body}}</div>
  </article>
</body>
</html>
EOF
```

Restart and visit `/notes`. The list renders. Click through to
`/notes/today`.

{{< hint info >}}
Template data: list templates receive `{{.Items}}` (a slice of maps),
`{{.Collection}}`, and any custom keys you add. Item templates receive
the entity itself — `{{.ID}}`, `{{.Body}}`, and every frontmatter key
flattened to the top level (so `{{.title}}` and `{{.tags}}` work).
{{< /hint >}}

## Step 7 — Customize the posts template

Edit `templates/posts.html` and replace it with something richer:

```html
<!DOCTYPE html>
<html>
<head><title>Posts</title><link rel="stylesheet" href="/css/style.css"></head>
<body>
  <nav><a href="/">Home</a> / <a href="/notes">Notes</a></nav>
  <h1>Posts</h1>
  {{range .Items}}
  <article>
    <h2><a href="/posts/{{.ID}}">{{.title}}</a></h2>
    <p class="meta">
      {{.date}}
      {{range .tags}}<span class="tag">{{.}}</span>{{end}}
    </p>
  </article>
  {{end}}
</body>
</html>
```

Add a tag style to `public/css/style.css`:

```css
.tag {
  display: inline-block;
  padding: 2px 8px;
  margin-left: 4px;
  background: #eef;
  border-radius: 3px;
  font-size: 0.85em;
}
```

Hot-edit: templates are read fresh on each request, so you can refresh
the browser without restarting. Static files in `public/` are served as
a fallback whenever no dynamic route matches.

## Step 8 — Show the route table

Stop the server. The binary can print every registered route without
serving traffic:

```bash
nullspace routes
```

You should see something like:

```
METHOD  PATH                 HANDLER       FORMAT  MIDDLEWARE  TEMPLATE
GET     /                    template      html                home.html
GET     /api/health          health.check  json
GET     /api/notes           data.list     json
GET     /api/notes/:id       data.get      json
GET     /api/posts           data.list     json
GET     /api/posts/:id       data.get      json
GET     /notes               data.list     html                notes.html
GET     /notes/:id           data.get      html                note.html
GET     /posts               data.list     html                posts.html
GET     /posts/:id           data.get      html                post.html
```

Every collection emits four routes. The `data.list` and `data.get`
handlers are built in — see [the routing reference]({{< relref "/reference" >}})
for the full list.

## Where to go next

- [Build a JSON API]({{< relref "json-api" >}}) — keep using the binary
  but write data as well as read it, and stream large lists as NDJSON.
- [How-to guides]({{< relref "/how-to" >}}) — recipes for adding
  middleware, custom formatters, sessions, and more.
- [Wire Nullspace as a library]({{< relref "library" >}}) — when you
  outgrow conventions and want full control over module wiring.
