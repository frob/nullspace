# Build the Nullspace documentation site.
#
# Three stages:
#  1. `api-gen` — Go + gomarkdoc; regenerates content/api/*.md from the
#     Go source.
#  2. `builder` — official hugomods/hugo:exts image (Hugo extended +
#     Go preinstalled); fetches the theme module and builds the site.
#  3. `serve`   — a tiny Caddy that serves the rendered HTML.
#
# The build expects the *project root* as its build context, not the docs
# directory, so it can read the Go source for gomarkdoc. The Taskfile sets
# this up; if invoking docker manually:
#
#     docker build -f docs/Dockerfile -t nullspace-docs .
#
# To get just the rendered site without serving it, target the `builder`
# stage and copy out /docs-output:
#
#     docker build -f docs/Dockerfile --target builder -o docs/_build/html .

ARG GO_VERSION=1.25
ARG HUGO_TAG=debian-dart-sass-go-git-0.162.1

# Stage 1: generate API markdown from Go source.
FROM golang:${GO_VERSION}-alpine AS api-gen

RUN apk add --no-cache bash git ca-certificates

RUN go install github.com/princjef/gomarkdoc/cmd/gomarkdoc@latest

WORKDIR /src
COPY . .

RUN bash /src/docs/gen-api.sh


# Stage 2: build the Hugo site.
#
# `debian-dart-sass-go-git-0.162.1` bundles Hugo 0.162.1 with the
# dart-sass-embedded binary (needed by the Hugo Book theme's `css.Sass`
# call), Go (needed by `hugo mod get`), and Git (needed to fetch theme
# modules from GitHub).
FROM hugomods/hugo:${HUGO_TAG} AS builder

WORKDIR /src
COPY --from=api-gen /src /src

WORKDIR /src/docs

# Resolve the Hugo Book theme module pinned in go.mod (network required).
RUN hugo mod get github.com/alex-shpak/hugo-book

# Build the static site.
RUN hugo --minify --destination /docs-output


# Stage 3: serve the rendered HTML.
FROM caddy:2-alpine AS serve
COPY --from=builder /docs-output /usr/share/caddy
EXPOSE 80
