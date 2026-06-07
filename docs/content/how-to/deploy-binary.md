---
title: Build a static binary
weight: 20
---

Use this guide when you want a single executable to copy onto a server (no
runtime, no container).

## Solution

Build the `nullspace` binary with the Taskfile:

```bash
task dist
ls bin/   # nullspace
```

Or cross-compile for a different target:

```bash
GOOS=linux GOARCH=amd64 task dist
```

The result is statically linked (`CGO_ENABLED=0`) and has no runtime
dependencies — SQLite is pure Go via `modernc.org/sqlite`.

Copy and run on the host:

```bash
scp bin/nullspace myserver:/usr/local/bin/
scp -r myproject/ myserver:/opt/myapp/

ssh myserver
cd /opt/myapp
nullspace
```

## systemd unit

Drop this in `/etc/systemd/system/nullspace.service`:

```ini
[Unit]
Description=Nullspace
After=network.target

[Service]
Type=simple
User=nullspace
WorkingDirectory=/opt/myapp
ExecStart=/usr/local/bin/nullspace
Restart=on-failure
RestartSec=5

Environment=NULLSPACE_LOG_LEVEL=info
Environment=NULLSPACE_LOG_FORMAT=json
Environment=NULLSPACE_REQUEST_ADDR=:8080

[Install]
WantedBy=multi-user.target
```

```bash
sudo systemctl daemon-reload
sudo systemctl enable --now nullspace
sudo journalctl -u nullspace -f
```

## Variations

### Custom binary (library usage)

If your project has its own `main.go`, build it with the standard Go
toolchain — no Taskfile needed:

```bash
CGO_ENABLED=0 go build -o bin/myapp .
```

### Cross-compile for multiple platforms

```bash
task dist:all      # macOS, Linux, arm64, amd64
```

### Use the published packages

A signed `.deb`, `.rpm`, `.pkg.tar.zst`, or Homebrew cask ships with each
release:

```bash
sudo dpkg -i nullspace_<version>_linux_amd64.deb
# Binary installs to /usr/bin/nullspace
# Config installs to /etc/nullspace/nullspace.toml
```

## Reverse proxy

Nullspace serves plain HTTP. Put it behind Caddy or Nginx for TLS
termination — see the [Docker deploy guide]({{< relref "deploy-docker" >}})
for sample configs. The `http-security` middleware honours
`X-Forwarded-Proto` so HTTPS redirect works through the proxy.

## See also

- [Deploy with Docker]({{< relref "deploy-docker" >}})
- [Configure logging output]({{< relref "logging" >}})
