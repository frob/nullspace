---
title: Deploy with Docker
weight: 19
---

Use this guide when you want to run Nullspace in a container.

## Solution

Mount your project directory into the official multi-arch image:

```bash
docker run -d \
  --name nullspace \
  -p 8080:8080 \
  -v /opt/myapp:/app \
  ghcr.io/frob/nullspace
```

`/app` is the working directory inside the image. The binary reads
`nullspace.toml` and serves `content/`, `templates/`, and `public/` from
there.

## Docker Compose

```yaml
services:
  nullspace:
    image: ghcr.io/frob/nullspace:latest
    ports:
      - "8080:8080"
    volumes:
      - ./myproject:/app
    environment:
      - NULLSPACE_LOG_FORMAT=json
      - NULLSPACE_LOG_LEVEL=info
```

Bring it up:

```bash
docker compose up -d
docker compose logs -f nullspace
```

## Variations

### Custom binary (library usage)

If your application has its own `main.go`, build a static binary in one
stage and copy it into a small runtime image:

```dockerfile
FROM golang:1.25-alpine AS builder
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /bin/myapp .

FROM alpine:3.21
RUN apk add --no-cache ca-certificates
COPY --from=builder /bin/myapp /usr/local/bin/myapp
COPY templates /app/templates
COPY public    /app/public
COPY content   /app/content
COPY nullspace.toml /app/nullspace.toml
WORKDIR /app
EXPOSE 8080
ENTRYPOINT ["myapp"]
```

### Behind a reverse proxy

Terminate TLS at the proxy and forward to Nullspace on `:8080`. The
`http-security` middleware honours `X-Forwarded-Proto`.

Caddy:

```text
myapp.example.com {
    reverse_proxy localhost:8080
}
```

Nginx:

```nginx
server {
    listen 443 ssl;
    server_name myapp.example.com;
    ssl_certificate     /etc/ssl/certs/myapp.pem;
    ssl_certificate_key /etc/ssl/private/myapp.key;

    location / {
        proxy_pass http://127.0.0.1:8080;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }
}
```

### Health check probe

```yaml
healthcheck:
  test: ["CMD", "wget", "-qO-", "http://localhost:8080/api/health"]
  interval: 30s
  timeout: 5s
  retries: 3
```

## Configuration tips

- Put non-secret defaults in `nullspace.toml` (committed to the repo).
- Set secrets via environment variables: `NULLSPACE_DATA_SQL_DSN`,
  `NULLSPACE_OIDC_CLIENT_SECRET`, etc.

## See also

- [Build a static binary]({{< relref "deploy-binary" >}})
- [Set security headers]({{< relref "security-headers" >}})
