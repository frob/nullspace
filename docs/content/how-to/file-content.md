---
title: Serve markdown content from files
weight: 4
---

Use this guide when you want to store records as files on disk — for example,
blog posts written in markdown with frontmatter.

## Solution

Enable the file data module and put files in a collection directory.
Subdirectory name = collection; filename = entity ID.

```toml
[data.file]
dir    = "./content"
format = "markdown"
```

```
content/
└── posts/
    ├── hello-world.md
    └── second.md
```

A markdown file with YAML frontmatter:

```markdown
---
title: Hello World
date: 2026-06-06
tags:
  - intro
---

Body content goes here.
```

The frontmatter populates the entity `Meta` map; the content after the
frontmatter becomes the entity `Body`.

Expose the collection through generated routes:

```toml
[[routing.collections]]
name          = "posts"
source        = "data.file"
api_prefix    = "/api"
html_prefix   = ""
list_template = "posts.html"
item_template = "post.html"
```

`GET /posts/hello-world` now renders `templates/post.html` with the parsed
entity; `GET /api/posts` returns the list as JSON.

## Variations

### TOML frontmatter

```markdown
+++
title = "Hello World"
date  = 2026-06-06
+++

Body content goes here.
```

### Plain JSON or TOML records

The module also accepts `.json` and `.toml` files. The reserved `body` key
becomes the entity body; all other keys go into `Meta`.

```toml
# content/authors/jane.toml
title = "Jane"
body  = "Bio text."
email = "jane@example.com"
```

### Reading entities from Go

```go
fileMod, _ := kernel.GetResource[*file.Module](k, "data.file")

entity, err := fileMod.Read(ctx, "posts", "hello-world")
list,  err := fileMod.List(ctx, "posts")

err = fileMod.Write(ctx, "posts", "new-post", &file.Entity{
    ID:     "new-post",
    Meta:   map[string]any{"title": "New Post"},
    Body:   "Content here.",
    Format: "markdown",
})

err = fileMod.Delete(ctx, "posts", "old-post")
```

## Lookup rules

`Read(ctx, "posts", "hello-world")` tries the following filenames in order:

1. `posts/hello-world.md`
2. `posts/hello-world.markdown`
3. `posts/hello-world.json`
4. `posts/hello-world.toml`

## See also

- [Generate CRUD routes for a collection]({{< relref "collections" >}})
- [Use SQL storage with migrations]({{< relref "sql-data" >}})
- [Expose data over TCP/IPC]({{< relref "data-bridge" >}})
