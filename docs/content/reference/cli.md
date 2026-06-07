---
title: CLI commands
weight: 6
---

The `nullspace` binary (`cmd/nullspace`) serves a project directory by
convention. Run from a directory containing `nullspace.toml`,
`content/`, `templates/`, or `public/`.

## Synopsis

```
nullspace [subcommand]
```

If no subcommand is supplied, `nullspace` runs the `serve` behavior
(the default).

## Subcommands

| Subcommand                  | Description                                            |
| --------------------------- | ------------------------------------------------------ |
| *(none)*                    | Serve the project in the current directory.            |
| `init`                      | Scaffold `nullspace.toml`, `content/`, `templates/`, `public/` with sample files. Existing files are preserved. |
| `routes`                    | Print the registered route table to stdout and exit. |
| `version`, `--version`, `-v` | Print the binary version and exit.                     |
| `help`, `--help`, `-h`      | Print usage and exit.                                  |

There are no flags. All configuration is read from `nullspace.toml` and
environment variables.

## Exit codes

| Code | Meaning                                              |
| ---- | ---------------------------------------------------- |
| `0`  | Clean shutdown after `SIGINT` or `SIGTERM`.          |
| `1`  | Initialization, configuration, or startup failure.   |

## Scaffold layout

`nullspace init` writes these files only if they do not already exist:

| File                              | Purpose                                  |
| --------------------------------- | ---------------------------------------- |
| `nullspace.toml`                  | Default config (`[request]`, `[log]`, `[response]`, `[data.static]`, `[data.file]`). |
| `content/posts/hello-world.md`    | Example post.                            |
| `templates/home.html`             | Home page template.                      |
| `templates/posts.html`            | Collection list template.                |
| `templates/post.html`             | Single-item template.                    |
| `public/css/style.css`            | Example stylesheet.                      |

## Conventional routes

When run as the binary, the kernel auto-discovers content collections by
listing subdirectories of `content/`. For each subdirectory, a
`[[routing.collections]]` entry is added with `source = "data.file"`,
`api_prefix = "/api"`, `html_prefix = ""`, `list_template = "<name>.html"`,
and `item_template = "<singular>.html"` (singular = name with trailing `s`
stripped). Two more routes are added unconditionally:

| Path           | Handler        | Format | Notes                          |
| -------------- | -------------- | ------ | ------------------------------ |
| `/`            | `template`     | html   | Renders `home.html`.           |
| `/api/health`  | `health.check` | json   | Returns `{"status":"ok"}`.    |

These are appended only if no route with the same effective path + handler
is already defined in `nullspace.toml`.

## Signals

| Signal    | Behavior                                                 |
| --------- | -------------------------------------------------------- |
| `SIGINT`  | Calls `k.Stop`, closes connections, and exits with code `0`. |
| `SIGTERM` | Same as `SIGINT`.                                        |
