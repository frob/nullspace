# Incident Postmortem: Cascading Circuit Breaker Failure

**Date:** 2026-03-18
**Severity:** SEV-2
**Duration:** 47 minutes
**Impact:** 12% of requests returned 503 during the degradation window

## Summary

A transient DNS resolution failure in the upstream identity provider caused the
token introspection middleware to exhaust its connection pool. The circuit breaker
tripped as expected, but the fallback path inadvertently issued synchronous retries
against the same failing endpoint, amplifying backpressure across the service mesh.

## Root Cause

The retry policy in the OAuth2 token exchange client was configured with exponential
backoff but lacked a jitter component. Under correlated failure, all retries
synchronized on the same interval boundaries, creating a thundering herd against
the recovering upstream. The circuit breaker's half-open probe window (5s) was
shorter than the upstream's recovery SLA (30s), causing repeated open/half-open
oscillation.

## Remediation

1. Added decorrelated jitter to the retry policy (full jitter algorithm)
2. Extended the half-open probe window to 60s with adaptive backoff
3. Deployed a local token cache with a 30s TTL to absorb transient upstream failures
4. Added a synthetic canary endpoint to the IDP health check aggregation
