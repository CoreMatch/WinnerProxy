# 0004 — Outbound call status and open deployment question

## Status

Accepted

## Context

WinnerProxy's HRPAuth client speaks to two planes: the Yggdrasil surface
through the `/yggdrasil-api` relay prefix (the HRPAuth core relay strips
the prefix before forwarding to HRPAuth-Yggdrasil-API, which serves the
routes prefix-less), and the management plane at the HRPAuth root. This
entry records the status of the outbound calls and one open
deployment-side question.

## Decision (implemented state)

1. **Batch query**: `POST /api/profiles/minecraft`, prefix-less — the
   configured base URL already carries the `/yggdrasil-api` relay prefix
   and the relay strips it (a base URL pointing directly at the
   microservice also works, since it serves the same prefix-less route).
   The upstream route is documented in HA-Contract
   `/home/lnb/Desktop/HA/hrpauth/HRPAuth-Yggdrasil-API/knowledge/0010-yggdrasil-api-surface.md`
   §6. Upstream failures are mapped explicitly: upstream 404 → 502
   `{"error": "hrpauth batch endpoint missing"}`, other upstream/network
   failures → 502 `{"error": "hrpauth upstream unavailable"}`. No bare
   500 on this path.
2. **Proxy registration**: `POST /internal/proxy-register` guarded by
   `X-Internal-Key`. The key is fetched from the core's
   `POST /services/auth-key` with the client-credentials service token
   (after the presence handshake, on demand, and again after any 401).
   The body is `{username, password, mojang_uuid}`; the client consumes
   `profile_id` from the envelope response. The endpoint's error codes
   (409 `username_already_bound`, 400 `invalid_mojang_uuid`) map onto the
   client's sentinels. Contract citations:
   `/home/lnb/Desktop/HA/HA-Contract/docs/api/openapi/hrpauth-yggdrasil-business.yaml (field: /internal/proxy-register)`,
   key parameter
   `/home/lnb/Desktop/HA/HA-Contract/docs/api/openapi/hrpauth-yggdrasil-business.yaml (field: InternalKeyHeader)`,
   key issuance
   `/home/lnb/Desktop/HA/hrpauth/HRPAuth/knowledge/0008-microservice-extension-layer.md (field: /services/auth-key)`.
3. **Deployment-config question (not verified).** The compose template sets
   [xref] `HRPAUTH_YGG_URL: http://hrpauth-yggdrasil-api:2770/yggdrasil-api/`
   (direct container address *with* the relay prefix). The microservice
   serves prefix-less routes and the HRPAuth relay is what strips the
   prefix, so this value looks inconsistent with the design; it may be
   compensated by the live deployment layout. **Verify against the running
   deployment** before treating batch/hasJoined failures as code bugs.

## Consequences

- All outbound calls work end-to-end against the current upstream:
  hasJoined stages 1-3, profile query, root metadata, batch query, and
  proxy registration. HA-Contract's WinnerProxy contract documents the
  same state (see [0005](./0005-contract-status.md)).
- Batch-query failures surface as 502 with an explicit
  `{"error": ...}` reason (`hrpauth batch endpoint missing` on upstream
  404, `hrpauth upstream unavailable` otherwise) instead of a bare 500.
- Item 3 (compose `HRPAUTH_YGG_URL` with the relay prefix pointing at the
  microservice container) is a deployment-side property: with that value
  the batch call 404s and surfaces as the explicit 502 `hrpauth batch
  endpoint missing`. The compose template belongs to the Deployer
  repository; the deployment-side value still needs verification.

## Evidence

- Batch query path: `internal/hrpauth/endpoints.go::BatchQuery`; handler
  error mapping `internal/handler/handler.go::BatchQuery`.
- Upstream batch route:
  `/home/lnb/Desktop/HA/hrpauth/HRPAuth-Yggdrasil-API/main.go::BatchProfiles`
  (`api.POST("/profiles/minecraft", ...)` under `r.Group("/api")`).
- Relay prefix stripping:
  `/home/lnb/Desktop/HA/hrpauth/HRPAuth/controllers/relay_controller.go::Match`,
  `/home/lnb/Desktop/HA/hrpauth/HRPAuth/controllers/relay_controller.go::RelayMiddleware`.
- Register body sent: `internal/hrpauth/types.go::RegisterRequest`;
  response expectation `internal/hrpauth/types.go::RegisterResponse`;
  client mapping `internal/hrpauth/endpoints.go::RegisterByProxy`.
- Proxy-register endpoint + X-Internal-Key:
  `/home/lnb/Desktop/HA/hrpauth/HRPAuth-Yggdrasil-API/controllers/internal_controller.go::ProxyRegister`,
  route
  `/home/lnb/Desktop/HA/hrpauth/HRPAuth-Yggdrasil-API/main.go::ProxyRegister`.
- Internal-key fetch: `internal/hrpauth/client.go::FetchInternalKey`.
- Compose value:
  `/home/lnb/Desktop/HA/maintanance/HA-Services-Deployer/deploy/docker-compose.yml (field: HRPAUTH_YGG_URL)`;
  relay prefix is a core-side concept:
  `/home/lnb/Desktop/HA/hrpauth/HRPAuth-Yggdrasil-API/config/config.go::RelayPathPrefix`,
  `/home/lnb/Desktop/HA/hrpauth/HRPAuth-Yggdrasil-API/config/config.go::DefaultRelayPathPrefix`
  (`RelayPathPrefix` =
  "the path prefix on the Core side that relays to this service").
- Contract-side record of the same state:
  `/home/lnb/Desktop/HA/hrpauth/WinnerProxy/knowledge/0006-service-contract.md (field: Outbound Calls)`.
