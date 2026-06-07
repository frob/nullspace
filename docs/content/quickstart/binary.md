---
title: Using the binary
weight: 1
---

The `nullspace` binary serves a project directory by convention. No code
required to ship the first version.

## Install

```bash
# macOS (Homebrew)
brew install frob/tap/nullspace

# Debian / Ubuntu
sudo dpkg -i nullspace_*_linux_amd64.deb

# Rocky / RHEL / Fedora
sudo rpm -i nullspace_*_linux_amd64.rpm

# Arch Linux
sudo pacman -U nullspace_*_linux_amd64.pkg.tar.zst

# Anywhere with a Go toolchain
go install github.com/frob/nullspace/cmd/nullspace@latest
```

## Scaffold and serve

```bash
mkdir mysite && cd mysite
nullspace init
nullspace
```

Visit <http://localhost:8080>. The `init` command creates this layout:

```
mysite/
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

Subdirectories of `content/` become auto-discovered collections. Each
collection gets HTML and JSON routes for free:

| Route                  | Description                              |
| ---------------------- | ---------------------------------------- |
| `/posts`, `/posts/:id` | HTML, rendered with `templates/posts.html` and `templates/post.html` |
| `/api/posts`, `/api/posts/:id` | JSON                              |
| `/api/health`          | Health check                             |

List every registered route:

```bash
nullspace routes
```

## Add content

```bash
cat > content/posts/second.md <<'EOF'
---
title: Second post
date: 2026-06-06
---

Hello from Nullspace.
EOF
```

Restart `nullspace` and visit <http://localhost:8080/posts/second>.

## Override config

`nullspace.toml` lives at the project root:

```toml
[request]
addr = ":3000"

[log]
level = "debug"
format = "text"
```

Environment variables override TOML values with the convention
`NULLSPACE_SECTION_KEY`:

```bash
NULLSPACE_REQUEST_ADDR=:9090 nullspace
```

## Next

- [Build a markdown blog]({{< relref "/tutorials/blog" >}}) — same flow with
  custom templates and a second collection.
- [Configuration reference]({{< relref "/reference/configuration" >}}) —
  every supported key.
- [Switch to the library form]({{< relref "library" >}}) when you outgrow
  the conventions.
