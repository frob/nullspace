// gRPC example: HTTP + TCP + gRPC transports in a single kernel.
//
// This demonstrates the multi-transport architecture: all three adapters
// share the same kernel, hook bus, config, and service locator. Business
// logic lives in shared functions; transport handlers are thin adapters
// that format I/O for their protocol.
//
// Run from the cmd/examples/grpc directory:
//
//	go run .
//
// HTTP endpoints (port 8080):
//   - GET  /api/time    — current server time (JSON)
//   - GET  /api/echo    — echo query params (JSON)
//   - GET  /api/health  — health check (JSON)
//
// TCP commands (port 9090, JSON-lines codec):
//   - {"command":"time"}                           — current server time
//   - {"command":"echo","payload":"hello"}          — echo payload back
//   - {"command":"health"}                          — health check
//
// gRPC (port 50051, JSON codec):
//   - echo.Echo/Say     — echo message with timestamp
//   - echo.Echo/Health  — health check
//
// Test TCP with netcat:
//
//	echo '{"command":"time"}' | nc localhost 9090
//	echo '{"command":"echo","payload":"hello world"}' | nc localhost 9090
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	nsgrpc "github.com/frob/nullspace-grpc"
	"github.com/frob/nullspace-grpc/proto/echopb"
	"github.com/frob/nullspace/core/nslog"
	"github.com/frob/nullspace/core/request"
	"github.com/frob/nullspace/core/response"
	"github.com/frob/nullspace/core/routing"
	"github.com/frob/nullspace/core/tcp"
	"github.com/frob/nullspace/kernel"
)

func main() {
	k := kernel.New(
		kernel.WithConfigFile("nullspace.toml"),
	)

	// Core modules.
	pipeline := response.NewPipeline()

	k.Use(nslog.New())
	k.Use(request.NewAdapter())
	k.Use(pipeline)
	k.Use(response.NewFormatQueryParam())
	k.Use(response.NewFormatDefault())

	// Routing (HTTP).
	k.Use(routing.New())

	// TCP transport.
	k.Use(tcp.NewAdapter())

	// gRPC transport.
	k.Use(nsgrpc.New())

	// Application module — registers handlers on all three transports.
	k.Use(&appModule{pipeline: pipeline})

	ctx := context.Background()
	if err := k.Init(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "init: %v\n", err)
		os.Exit(1)
	}
	if err := k.Start(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "start: %v\n", err)
		os.Exit(1)
	}

	k.Logger().Info("mixed-mode app running",
		"http", "http://localhost:8080",
		"tcp", "localhost:9090",
		"grpc", "localhost:50051",
	)

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	k.Logger().Info("shutting down")
	if err := k.Stop(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "stop: %v\n", err)
	}
}

// --- Shared business logic (transport-agnostic) ---

func getTime() map[string]string {
	return map[string]string{
		"time": time.Now().Format(time.RFC3339),
	}
}

func echoPayload(input string) map[string]string {
	return map[string]string{
		"echo": input,
	}
}

func healthCheck() map[string]string {
	return map[string]string{
		"status": "ok",
	}
}

// --- Application module ---

type appModule struct {
	pipeline *response.Pipeline
}

func (m *appModule) Name() string                    { return "app" }
func (m *appModule) Start(ctx context.Context) error { return nil }
func (m *appModule) Stop(ctx context.Context) error  { return nil }

func (m *appModule) Init(k *kernel.Kernel) error {
	// --- HTTP handlers ---
	reg, err := kernel.GetResource[*routing.Registry](k, "routing.registry")
	if err != nil {
		return err
	}

	reg.HandleFunc("time", func(ctx *request.Context) error {
		return m.pipeline.Write(ctx.Context(), ctx.Writer, response.NewResponse(http.StatusOK, getTime()))
	})

	reg.HandleFunc("echo", func(ctx *request.Context) error {
		input := ctx.Request.URL.Query().Get("msg")
		return m.pipeline.Write(ctx.Context(), ctx.Writer, response.NewResponse(http.StatusOK, echoPayload(input)))
	})

	reg.HandleFunc("health.check", func(ctx *request.Context) error {
		return m.pipeline.Write(ctx.Context(), ctx.Writer, response.NewResponse(http.StatusOK, healthCheck()))
	})

	// --- TCP handlers ---
	tcpRouter, err := kernel.GetResource[*tcp.Router](k, "tcp.router")
	if err != nil {
		return err
	}

	tcpRouter.Handle("time", func(conn *tcp.Conn, cmd string, payload []byte) error {
		data, _ := json.Marshal(getTime())
		return conn.Send("time", data)
	})

	tcpRouter.Handle("echo", func(conn *tcp.Conn, cmd string, payload []byte) error {
		data, _ := json.Marshal(echoPayload(string(payload)))
		return conn.Send("echo", data)
	})

	tcpRouter.Handle("health", func(conn *tcp.Conn, cmd string, payload []byte) error {
		data, _ := json.Marshal(healthCheck())
		return conn.Send("health", data)
	})

	// --- gRPC service ---
	grpcAdapter, err := kernel.GetResource[*nsgrpc.Adapter](k, "transport.grpc")
	if err != nil {
		return err
	}

	echopb.RegisterEchoServer(grpcAdapter.Server(), &echoService{})

	return nil
}

// echoService implements the gRPC Echo service using shared business logic.
type echoService struct {
	echopb.UnimplementedEchoServer
}

func (s *echoService) Say(ctx context.Context, req *echopb.SayRequest) (*echopb.SayReply, error) {
	result := echoPayload(req.Message)
	t := getTime()
	return &echopb.SayReply{
		Message: result["echo"],
		Time:    t["time"],
	}, nil
}

func (s *echoService) Health(ctx context.Context, req *echopb.HealthRequest) (*echopb.HealthReply, error) {
	result := healthCheck()
	return &echopb.HealthReply{
		Status: result["status"],
	}, nil
}
