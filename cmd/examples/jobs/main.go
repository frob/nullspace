// Jobs example: enqueues one "hello" job on startup and processes it.
// Uses the in-memory store; no database required.
//
// Run from this directory:
//
//	go run .
package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/frob/nullspace/core/nslog"
	"github.com/frob/nullspace/kernel"
	"github.com/frob/nullspace/module/jobs"
)

func main() {
	k := kernel.New(kernel.WithConfigFile("nullspace.toml"))
	k.Use(nslog.New())
	j := jobs.New()
	k.Use(j)

	j.Handlers().Handle("hello", func(ctx context.Context, job *jobs.Job) error {
		fmt.Printf("hello job executed: payload=%s\n", string(job.Payload))
		return nil
	})

	ctx := context.Background()
	if err := k.Init(ctx); err != nil {
		log.Fatal(err)
	}
	if err := k.Start(ctx); err != nil {
		log.Fatal(err)
	}

	if _, err := j.Submit(ctx, jobs.JobSpec{Type: "hello", Payload: "world"}); err != nil {
		log.Fatal(err)
	}

	time.Sleep(2 * time.Second)

	if err := k.Stop(ctx); err != nil {
		log.Printf("stop: %v", err)
	}
}
