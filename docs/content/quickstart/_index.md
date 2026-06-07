---
title: Quickstart
weight: 1
bookCollapseSection: false
---

# Quickstart

Get a Nullspace project serving HTML and JSON in under a minute. Two paths
exist depending on how much control you want:

- **Binary** — `nullspace init` scaffolds a project; the binary serves it.
  Best for content sites, static + dynamic mixes, and prototypes.
- **Library** — `go get github.com/frob/nullspace`, wire modules in your own
  `main`. Best when you need custom transports, business logic, or
  integration with an existing Go codebase.

Start with the path that fits your goal. You can always move to the library
form later — the binary is just a pre-wired set of modules.
