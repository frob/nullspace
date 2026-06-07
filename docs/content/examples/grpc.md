---
title: gRPC
weight: 2
---

Three transports — HTTP, TCP, gRPC — sharing one kernel. Demonstrates the
multi-transport architecture: business logic lives in shared functions and
each transport handler is a thin adapter that formats I/O for its protocol.

- **Source:** [`cmd/examples/grpc/`](https://github.com/frob/nullspace/tree/0.0.x/cmd/examples/grpc)
- **Form:** library

## Run

```bash
cd cmd/examples/grpc
go run .
```

Or:

```bash
task run:grpc
```

The kernel binds three listeners:

| Port  | Transport | Codec        |
| ----- | --------- | ------------ |
| 8080  | HTTP      | JSON         |
| 9090  | TCP       | JSON-lines   |
| 50051 | gRPC      | JSON         |

## Endpoints

| Transport | Endpoint                            | Description       |
| --------- | ----------------------------------- | ----------------- |
| HTTP      | `GET /api/time`                     | Server time       |
| HTTP      | `GET /api/echo?msg=hi`              | Echo query param  |
| HTTP      | `GET /api/health`                   | Health check      |
| TCP       | `{"command":"time"}`                | Server time       |
| TCP       | `{"command":"echo","payload":"x"}`  | Echo              |
| TCP       | `{"command":"health"}`              | Health check      |
| gRPC      | `echo.Echo/Say`                     | Echo + timestamp  |
| gRPC      | `echo.Echo/Health`                  | Health check      |

## What it demonstrates

**Multi-transport kernel** — registers `request.NewAdapter`,
`tcp.NewAdapter`, and `nsgrpc.New` against the same kernel. Each adapter
provides its own router on the service locator; the application module
fans out registrations.

**Contrib modules** — the gRPC adapter is not part of core. It lives in
`github.com/frob/nullspace-grpc` and demonstrates how an external module
can add a new transport without forking the framework.

**Codec selection** — `[tcp] codec = "json-lines"` switches the TCP
transport from binary length-prefix to newline-delimited JSON for easy
debugging with `nc`.

## Test it

```bash
# HTTP
curl http://localhost:8080/api/time

# TCP
echo '{"command":"time"}' | nc localhost 9090
echo '{"command":"echo","payload":"hello"}' | nc localhost 9090

# gRPC (use grpcurl or any gRPC client)
grpcurl -plaintext -d '{"message":"hi"}' localhost:50051 echo.Echo/Say
```

## See also

- [Explanation: multi-transport]({{< relref "/explanation/transports" >}})
- [Reference: configuration — [tcp] and [grpc]]({{< relref "/reference/configuration" >}})
