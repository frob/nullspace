# Runbook: OIDC Signing Key Rotation

## Overview

This runbook covers the procedure for rotating the OIDC signing keys on the
identity provider without causing token validation failures in downstream
services. The process uses a two-phase commit: publish the new key to the JWKS
endpoint before any tokens are signed with it, then retire the old key after
the longest-lived token has expired.

## Prerequisites

- Access to the IDP admin console (Keycloak realm admin or equivalent)
- Permissions to restart the token cache flush CronJob in the platform namespace
- The current signing key's `kid` value (check `/realms/{realm}/protocol/openid-connect/certs`)

## Procedure

### Phase 1: Publish New Key

1. Generate a new RSA-4096 or EC P-256 key pair in the IDP key store
2. Mark the new key as **active for signing** but do not yet disable the old key
3. Verify the JWKS endpoint now returns both keys
4. Flush the JWKS cache in all downstream services:
   ```
   kubectl rollout restart deployment/token-validator -n platform
   ```
5. Wait for at least one full cache TTL cycle (default: 5 minutes)

### Phase 2: Retire Old Key

1. Confirm no tokens signed with the old `kid` are in active circulation
   (check the access token max lifetime — typically 5 minutes)
2. Disable the old key in the IDP key store
3. Verify the JWKS endpoint returns only the new key
4. Monitor error rates for 15 minutes; roll back if validation failures spike

## Rollback

Re-enable the old key in the IDP key store. The JWKS endpoint will immediately
serve both keys again. Downstream caches will pick up the change within one TTL.
