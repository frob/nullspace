# ADR-0042: Event Sourcing for Aggregate State Hydration

## Status: Accepted

## Context

The current CQRS implementation materializes read models via synchronous projections
triggered by domain events. Under load, the write-side latency spikes because the
projection handlers contend on the same connection pool as the command handlers.

We evaluated three approaches for decoupling the write path from the read-side
materialization pipeline:

1. Async projections with an outbox table and a polling consumer
2. Change data capture (CDC) via Debezium piped into a Kafka topic
3. Full event sourcing with an append-only event store and snapshot compaction

## Decision

We will adopt event sourcing with snapshotting every 100 events per aggregate.
The event store will use a single-writer append model with optimistic concurrency
on the aggregate version column. Snapshots are stored as JSONB in a side table
and hydrated lazily on cache miss.

## Consequences

- Write-side throughput improves ~3x due to append-only semantics
- Read models are eventually consistent (p99 propagation < 200ms)
- Aggregate reconstitution from snapshot + tail events adds ~2ms cold-start overhead
- The operations team must monitor the compaction job and snapshot storage growth
