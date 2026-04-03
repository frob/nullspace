# Capacity Planning: Q2 2026 Scaling Projections

## Current Baseline

| Metric                  | Value         |
|-------------------------|---------------|
| Peak RPS (p99)          | 14,200        |
| Avg response latency    | 8.3ms         |
| Pod count (steady)      | 12            |
| Pod count (HPA ceiling) | 36            |
| CPU utilization (avg)   | 42%           |
| Memory headroom         | 31%           |

## Projected Growth

The product team forecasts a 2.4x increase in authenticated API traffic due to
the mobile SDK launch and the partner integration pipeline going GA. The bulk
of the new load will hit the session validation and token refresh paths, which
are currently CPU-bound on the ECDSA signature verification.

## Recommendations

1. **Pre-scale the HPA floor** from 12 to 18 pods before the mobile launch date
   to absorb the initial burst without cold-start latency from the autoscaler.

2. **Enable the JWKS key cache** with a 5-minute TTL. Currently every request
   fetches the signing key set from the IDP discovery endpoint. Caching eliminates
   ~1.2ms per request and reduces upstream fanout by 99.7%.

3. **Migrate to Ed25519 tokens** if the IDP supports it. EdDSA verification is
   ~4x faster than ES256 on our ARM64 instances, which would defer the need for
   additional compute capacity until Q4.

4. **Shard the rate limiter** by tenant ID rather than source IP to prevent
   partner traffic from consuming the global token bucket during peak windows.
