package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/frob/nullspace/core/nslog"
	"github.com/frob/nullspace/core/routing"
	"github.com/frob/nullspace/kernel"
	"github.com/frob/nullspace/module/jobs"
)

// buildJobsKernel builds a kernel with logging + routing + jobs (memory store)
// and registers any provided handler names. The jobs module is started so the
// worker pool is live.
func buildJobsKernel(t *testing.T, handlerNames ...string) (*kernel.Kernel, *jobs.Module) {
	t.Helper()

	dir := t.TempDir()
	tomlPath := filepath.Join(dir, "nullspace.toml")
	toml := `
[modules]
jobs = true

[jobs]
store = "memory"
poll_interval = "10s"
`
	if err := os.WriteFile(tomlPath, []byte(toml), 0644); err != nil {
		t.Fatalf("write toml: %v", err)
	}

	k := kernel.New(kernel.WithConfigFile(tomlPath))
	k.Use(nslog.New())
	k.Use(routing.New())
	jobsMod := jobs.New()
	k.Use(jobsMod)

	ctx := context.Background()
	if err := k.Init(ctx); err != nil {
		t.Fatalf("Init: %v", err)
	}

	for _, name := range handlerNames {
		jobsMod.Handlers().Handle(name, func(ctx context.Context, j *jobs.Job) error { return nil })
	}

	if err := k.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}
	t.Cleanup(func() { _ = k.Stop(context.Background()) })

	return k, jobsMod
}

// TestShowJobs_ListsRegisteredHandlers verifies that showJobs writes every
// registered handler name to the writer.
func TestShowJobs_ListsRegisteredHandlers(t *testing.T) {
	k, _ := buildJobsKernel(t, "charlie", "alpha", "bravo")

	var buf bytes.Buffer
	if err := showJobs(k, &buf); err != nil {
		t.Fatalf("showJobs: %v", err)
	}

	out := buf.String()
	for _, name := range []string{"alpha", "bravo", "charlie"} {
		if !strings.Contains(out, name) {
			t.Errorf("output missing handler %q\noutput:\n%s", name, out)
		}
	}

	// Order is deterministic (alphabetical).
	aIdx := strings.Index(out, "alpha")
	bIdx := strings.Index(out, "bravo")
	cIdx := strings.Index(out, "charlie")
	if !(aIdx < bIdx && bIdx < cIdx) {
		t.Errorf("handler names not in alphabetical order: alpha=%d bravo=%d charlie=%d\noutput:\n%s",
			aIdx, bIdx, cIdx, out)
	}
}

// TestShowJobs_NoHandlers verifies showJobs handles an empty registry without
// error. Output should either be an empty table or an explicit "no handlers"
// notice — both are acceptable.
func TestShowJobs_NoHandlers(t *testing.T) {
	k, _ := buildJobsKernel(t)

	var buf bytes.Buffer
	if err := showJobs(k, &buf); err != nil {
		t.Fatalf("showJobs: %v", err)
	}

	out := buf.String()
	// Must not contain any phantom handler name.
	for _, junk := range []string{"alpha", "bravo", "charlie"} {
		if strings.Contains(out, junk) {
			t.Errorf("output unexpectedly contains %q: %s", junk, out)
		}
	}
}

// TestShowJobs_TableFormat verifies the output is a simple aligned table with
// at least NAME and QUEUE columns and contains the default queue name.
func TestShowJobs_TableFormat(t *testing.T) {
	k, _ := buildJobsKernel(t, "alpha")

	var buf bytes.Buffer
	if err := showJobs(k, &buf); err != nil {
		t.Fatalf("showJobs: %v", err)
	}

	out := buf.String()
	upper := strings.ToUpper(out)

	if !strings.Contains(upper, "NAME") {
		t.Errorf("output missing NAME column header\noutput:\n%s", out)
	}
	if !strings.Contains(upper, "QUEUE") {
		t.Errorf("output missing QUEUE column header\noutput:\n%s", out)
	}
	if !strings.Contains(out, "default") {
		t.Errorf("output missing default queue name\noutput:\n%s", out)
	}
	if !strings.Contains(out, "alpha") {
		t.Errorf("output missing handler name alpha\noutput:\n%s", out)
	}
}
