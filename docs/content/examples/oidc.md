---
title: OIDC
weight: 3
---

OpenID Connect authentication using the `nullspace-oidc` contributed module
and Keycloak as the identity provider. Public home page, protected admin
area, login and logout routes.

- **Source:** [`cmd/examples/oidc/`](https://github.com/frob/nullspace/tree/0.0.x/cmd/examples/oidc)
- **Form:** library
- **Prerequisites:** Docker (Keycloak runs in a container)

## Run

Bring Keycloak up, then start the app:

```bash
cd cmd/examples/oidc
docker compose up -d              # imports the realm on first run (~30s)
go run .
```

Or from the repo root:

```bash
task run:oidc                     # depends on `oidc:up`, waits for Keycloak
```

The app listens on <http://localhost:8888>. Keycloak listens on
<http://localhost:8180>.

## Endpoints

| URL            | Description                            |
| -------------- | -------------------------------------- |
| `/`            | Public home                            |
| `/admin`       | Protected admin (requires login)       |
| `/oidc/login`  | Starts the OIDC Authorization Code flow|
| `/oidc/logout` | Clears the session                     |

## Test credentials

Pre-seeded by the imported Keycloak realm:

```
Username: admin
Password: admin123
```

## What it demonstrates

**Contrib modules** — `nullspace-oidc` is a separate Go module
(`github.com/frob/nullspace-oidc`). It registers a `oidc` middleware that
TOML routes can pick up by name.

**Middleware via TOML** — the admin group declares `middleware = ["oidc"]`
and every route in the group is protected without code changes.

**Auto-derived redirect URI** — the OIDC module reads the kernel's
`[request] addr` if `redirect_uri` is omitted, so dev and prod configs
share the same TOML.

**Realm import** — `cmd/examples/oidc/keycloak/` contains a Keycloak realm
export that defines the client, scopes, and the seed user. Bringing the
container up imports it on first boot.

## Reset Keycloak

```bash
task oidc:reset                   # destroys volumes and re-imports
```

## See also

- [How-to: add OIDC authentication]({{< relref "/how-to/oidc" >}})
- [How-to: add middleware to a route group]({{< relref "/how-to/middleware" >}})
