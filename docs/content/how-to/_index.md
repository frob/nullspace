---
title: How-to
weight: 3
bookCollapseSection: true
---

# How-to guides

Recipes for specific problems. Unlike tutorials, how-to guides assume you
already know the basics and want to look something up. Each guide is
**task-oriented**: it solves one problem with the minimum surrounding
context.

## Routing

- [Define routes in TOML]({{< relref "routes-toml" >}})
- [Register a custom handler]({{< relref "custom-handler" >}})
- [Generate CRUD routes for a collection]({{< relref "collections" >}})

## Data

- [Serve markdown content from files]({{< relref "file-content" >}})
- [Use SQL storage with migrations]({{< relref "sql-data" >}})
- [Expose data over TCP/IPC]({{< relref "data-bridge" >}})

## Responses

- [Stream large lists as NDJSON]({{< relref "streaming" >}})
- [Force a response format on a route]({{< relref "format-override" >}})
- [Add a custom formatter]({{< relref "custom-formatter" >}})

## Cross-cutting

- [Add middleware to a route group]({{< relref "middleware" >}})
- [Hook into the request lifecycle]({{< relref "hooks" >}})
- [Run SQL migrations from a module]({{< relref "migrations" >}})
- [Configure logging output]({{< relref "logging" >}})

## Security

- [Add CSRF protection]({{< relref "csrf" >}})
- [Set security headers]({{< relref "security-headers" >}})
- [Add OIDC authentication]({{< relref "oidc" >}})

## Real-time

- [Upgrade to a WebSocket connection]({{< relref "websocket" >}})
- [Broadcast to rooms]({{< relref "rooms" >}})

## Operations

- [Deploy with Docker]({{< relref "deploy-docker" >}})
- [Build a static binary]({{< relref "deploy-binary" >}})
