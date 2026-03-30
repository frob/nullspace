FROM golang:1.25-alpine AS builder

ARG VERSION=dev
ARG GOOS
ARG GOARCH

WORKDIR /src

# Cache dependencies.
COPY go.mod go.sum ./
RUN go mod download

# Build.
COPY . .
RUN CGO_ENABLED=0 go build -ldflags "-s -w -X main.version=${VERSION}" -o /bin/nullspace ./cmd/nullspace

# Output stage — just the binary. Use with:
#   docker build --output=bin .
FROM scratch AS binary
COPY --from=builder /bin/nullspace /nullspace

# Runtime stage — for running in a container.
FROM alpine:3.21 AS runtime

RUN apk add --no-cache ca-certificates
COPY --from=builder /bin/nullspace /usr/local/bin/nullspace

WORKDIR /app
EXPOSE 8080

ENTRYPOINT ["nullspace"]
