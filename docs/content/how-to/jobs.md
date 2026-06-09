---
title: Run background jobs
weight: 21
---

Use this guide when your application needs to defer work — sending email,
processing uploads, syncing data — outside the HTTP request cycle.

## Solution

Enable the `jobs` module and register named handler functions before calling
`k.Start`.

```toml
[modules]
jobs = true

[jobs]
store         = "memory"   # or "sql"
workers       = 4
poll_interval = "1s"
```

```go
import (
    "context"

    "github.com/frob/nullspace/kernel"
    "github.com/frob/nullspace/module/jobs"
)

func (m *Module) Init(k *kernel.Kernel) error {
    j, err := kernel.GetResource[*jobs.Module](k, "jobs")
    if err != nil {
        return err
    }

    j.Handlers().Handle("send-email", func(ctx context.Context, job *jobs.Job) error {
        var payload struct{ To string }
        _ = jobs.DecodePayload(job.Payload, &payload)
        return sendEmail(ctx, payload.To)
    })

    return nil
}
```

Enqueue a job from anywhere that has access to the jobs module:

```go
id, err := j.Submit(ctx, jobs.JobSpec{
    Type:    "send-email",
    Payload: map[string]string{"to": "user@example.com"},
})
```

## Configuration

| Key | Type | Default | Description |
|---|---|---|---|
| `store` | string | `"memory"` | `"memory"` or `"sql"` (requires `data.sql`). |
| `workers` | int | `runtime.NumCPU()` | Worker pool size. |
| `poll_interval` | duration | `1s` | How often workers check for new work. |
| `lease_duration` | duration | `30s` | How long a worker holds a leased job before it is considered stuck. |
| `max_attempts` | int | `5` | Default per-job retry limit. |
| `backoff_base` | duration | `1s` | Exponential backoff base. |
| `backoff_max` | duration | `1h` | Backoff cap. |
| `backoff_jitter` | float | `0.2` | Jitter fraction applied to each backoff delay. |

## Store comparison

| Store | Persistence | Multi-process | Notes |
|---|---|---|---|
| memory | none (lost on restart) | no | Single-process only. Good for dev or ephemeral workloads. |
| sql | yes | yes | Requires `data.sql`. Postgres uses `FOR UPDATE SKIP LOCKED`; SQLite serializes via `BEGIN IMMEDIATE`. |

## Hooks

| Hook | Type | Fires |
|---|---|---|
| `jobs.before` | standard | Before the handler runs. |
| `jobs.after` | standard | After a successful handler. |
| `jobs.error` | standard | On handler error or panic, before the retry decision. |
| `jobs.retry` | resolve | Override the backoff delay (return a `time.Time`). |
| `jobs.dead` | standard | Job exhausted `max_attempts` and entered failed state. |

## Cancelling a job

```go
err := j.Cancel(ctx, jobID)
```

Jobs in `pending` status can be cancelled. Jobs that are already `leased`
(being processed) cannot be cancelled mid-flight.

## Exposing jobs over TCP

Enable the `data.bridge` module alongside `tcp` and `jobs`. The bridge
automatically registers `jobs.submit`, `jobs.list`, and `jobs.cancel`
commands on the TCP router during `kernel.after_init`.

```toml
[modules]
tcp           = true
"data.bridge" = true
jobs          = true

[tcp]
codec = "json-lines"
```

```
$ echo '{"command":"jobs.list","payload":{}}' | nc localhost 9090
```

## See also

- [Expose data over TCP/IPC]({{< relref "data-bridge" >}})
- [SQL data module]({{< relref "sql-data" >}})
