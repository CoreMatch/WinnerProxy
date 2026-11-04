# 0004 — Upstream drift and dead outbound calls

## Status

Accepted (documents the implemented state; drift findings verified against
the upstream repositories as of 2026-11-04)

## Date

2026-11-04

## Context

WinnerProxy's HRPAuth client was written against the old
HRPAuth-Backend-Go monolith layout (its comments still cite
`HRPAuth-Backend-Go/main.go`). The HA stack since split the Yggdrasil
surface into the HRPAuth-Yggdrasil-API microservice behind the
`/yggdrasil-api` relay prefix, and the registration endpoints changed
shape. Several outbound calls no longer succeed.

## Decision (implemented state)

Verified drift items:

1. **Batch query path is wrong (double prefix).**
   `BatchQuery` posts to `{base}/yggdrasil/api/profiles/minecraft`. With
   the default base `http://127.0.0.1:2778/yggdrasil-api/` this becomes
   `/yggdrasil-api/yggdrasil/api/profiles/minecraft`. The relay strips one
   `/yggdrasil-api` prefix; the microservice then receives
   `/yggdrasil/api/profiles/minecraft`, which is not a registered route —
   the call 404s and the handler answers 500. The upstream route is
   `POST /api/profiles/minecraft` (no extra segment).
2. **Proxy registration does not succeed.**
   Stage 3 posts `{base}/register` with body
   `{username, password, email:"", mojang_uuid}` and expects
   `{success, uid, message, profile_id, cbh}` plus 409
   `username_already_bound` / 400 `invalid_mojang_uuid` semantics. The
   current upstream `POST /register` binds
   `{identifier (required), password (required), mojang_uuid}` — the
   `username` field does not satisfy `identifier`, so the request fails
   400 (invalid body) and the response shape/error codes differ from what
   the client maps. The purpose-built endpoint is
   `POST /internal/proxy-register`, which requires an `X-Internal-Key`
   header that WinnerProxy never sends (it authenticates with OAuth2
   Bearer instead). Net effect: stage 3 fails → hasJoined returns 503 for
   Mojang players.
3. **Dead client methods.** `DeclareEmail` (`POST /user/declare-email`)
   and `EnableMojangBind` (`POST /user/mojang-bind-enable`) have no call
   sites, and the corresponding core endpoints no longer exist.
4. **Stale code comment.** The `HasJoined` client comment claims "HA's
   router does NOT use a /yggdrasil prefix" and cites the old monolith —
   true only for the pre-split layout; the current deployment reaches the
   Yggdrasil surface through the `/yggdrasil-api` relay prefix.
5. **Deployment-config question (not verified).** The compose template sets
   `HRPAUTH_YGG_URL: http://hrpauth-yggdrasil-api:2770/yggdrasil-api/`
   (direct container address *with* the relay prefix). The microservice
   serves prefix-less routes and the HRPAuth relay is what strips the
   prefix, so this value looks inconsistent with the design; it may be
   compensated by the live deployment layout. **Verify against the running
   deployment** before treating batch/hasJoined failures as code bugs.

## Consequences

- Today, only hasJoined stages 1-2, profile query, and root metadata work
  end-to-end against the current upstream; batch query and proxy
  registration fail as described. HA-Contract's WinnerProxy contract
  documents the same findings (see
  [0005](./0005-contract-status.md)).
- Fixing stage 3 requires a decision humans must make: either teach
  WinnerProxy the `/internal/proxy-register` + X-Internal-Key contract
  (key distribution needed), or re-shape the upstream `/register` to
  accept the proxy flow. Both change security-relevant interfaces —
  **pending author confirmation** on which direction is intended.
- Until fixed, operators should expect 500 on `POST /api/profiles/minecraft`
  and 503 for Mojang players at join time; monitoring should not treat
  these as outages of the proxy itself.

## Evidence

- Batch query path: `internal/hrpauth/endpoints.go:96-112` (URL at `:97`),
  HEAD of `main` `86866128`; handler error mapping
  `internal/handler/handler.go:194-214`.
- Upstream batch route: repo HRPAuth-Yggdrasil-API (@ `124b2fa1`),
  `main.go:181-183` (`api.POST("/profiles/minecraft", ...)` under
  `r.Group("/api")`).
- Relay prefix stripping: repo HRPAuth (@ `8dd67593`),
  `controllers/relay_controller.go:172-199,283-316`.
- Register body sent: `internal/hrpauth/types.go:23-29`; response
  expectation `:31-40`; client mapping `internal/hrpauth/endpoints.go:114-151`.
- Upstream register contract: repo HRPAuth-Yggdrasil-API,
  `controllers/register_controller.go:21-25` (`identifier`/`password`
  required), `:28-48` (400 on bind failure, 403 on service error);
  route `main.go:148`.
- Proxy-register endpoint + X-Internal-Key: repo HRPAuth-Yggdrasil-API,
  `controllers/internal_controller.go:31-46`, route `main.go:149`.
- Dead methods: `internal/hrpauth/endpoints.go:189-221` (no call sites —
  verified 2026-11-04).
- Stale comment: `internal/hrpauth/endpoints.go:14-20`.
- Compose value: repo HA-Services-Deployer (@ `657bcb88`),
  `deploy/docker-compose.yml:136`; microservice serves prefix-less routes:
  repo HRPAuth-Yggdrasil-API, `main.go:147-200`; relay prefix is a
  core-side concept: `config/config.go:36-40,127` (`RelayPathPrefix` =
  "the path prefix on the Core side that relays to this service").
- Contract-side record of the same drift: repo HA-Contract (@ `21c7147d`),
  `docs/dev/WinnerProxy/README.md`, "Outbound Calls (current
  implementation)" table.
