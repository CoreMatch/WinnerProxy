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

1. **Batch query path is wrong (double prefix). — FIXED 2026-11-05,
   commit `4ded866`.**
   `BatchQuery` used to post to `{base}/yggdrasil/api/profiles/minecraft`.
   [xref] With the default base `http://127.0.0.1:2778/yggdrasil-api/` this
   became
   `/yggdrasil-api/yggdrasil/api/profiles/minecraft`. The relay strips one
   `/yggdrasil-api` prefix; the microservice then received
   `/yggdrasil/api/profiles/minecraft`, which is not a registered route —
   the call 404'd and the handler answered a bare 500. The upstream route
   is `POST /api/profiles/minecraft` (no extra segment, HA-Contract
   `/home/lnb/Desktop/HA/HA-Contract/docs/dev/HRPAuth/yggdrasil-api.md`
   §6). The fix drops the extra
   `/yggdrasil` segment (the base URL already carries the relay prefix and
   the relay strips it, so no double prefix occurs; a base URL pointing
   directly at the microservice also works because it serves the same
   prefix-less route) and maps upstream failures explicitly: upstream 404
   → 502 `{"error": "hrpauth batch endpoint missing"}`, other
   upstream/network failures → 502 `{"error": "hrpauth upstream
   unavailable"}`. No bare 500 remains on this path.
2. **Proxy registration does not succeed.**
   Stage 3 posts `{base}/register` with body
   `{username, password, email:"", mojang_uuid}` and expects
   `{success, uid, message, profile_id, cbh}` plus 409
   `username_already_bound` / 400 `invalid_mojang_uuid` semantics. The
   current upstream `POST /register` binds
   `{identifier (required), password (required), mojang_uuid}` — the
   `username` field does not satisfy `identifier`, so the request fails
   400 (invalid body) and the response shape/error codes differ from what
   the client maps. Net effect: stage 3 fails → hasJoined returns 503 for
   Mojang players.
   **Contract-sanctioned path (reported only; no code change yet):** the
   contract's purpose-built endpoint is `POST /internal/proxy-register`
   (repo HA-Contract @ `21c7147d`,
   `/home/lnb/Desktop/HA/HA-Contract/docs/api/openapi/hrpauth-yggdrasil-business.yaml (field: /internal/proxy-register)`):
   authenticated via the `X-Internal-Key` header (parameter definition at
   `/home/lnb/Desktop/HA/HA-Contract/docs/api/openapi/hrpauth-yggdrasil-business.yaml (field: InternalKeyHeader)`
   — "Temporary internal key issued by HRPAuth Core
   (`POST /services/auth-key`)"), body `{username, password,
   mojang_uuid}`, response `data: {account_id, profile_id}`. The internal
   key itself is obtained from HRPAuth via `POST /services/auth-key`
   authenticated with a client-credentials Bearer
   (`/home/lnb/Desktop/HA/HA-Contract/docs/dev/HRPAuth/microservices.md (field: /services/auth-key)`).
   Additional mismatches a
   future fix must handle: the contract response carries `profile_id` but
   not `uid`/`cbh` (the `cbh` mechanism was removed from HRPAuth by commit
   `2efc6e2` "feat: add yggdrasil account claim and remove cbh code",
   2026-10-07, while `RegisterResponse.CBH` still exists in
   `internal/hrpauth/types.go:L34-L40 @ 86866128`), and the contract's
   error set
   (400/401/500) differs from the 409/400 codes the client maps.
3. **Unwired client methods — decoupling leftovers, not ordinary dead
   code.** `DeclareEmail` (`POST /user/declare-email`) and
   `EnableMojangBind` (`POST /user/mojang-bind-enable`) have no call
   sites and never had any. Archaeology (verified 2026-11-04 against repo
   HRPAuth @ `8dd67593`, the HA main project — remote
   `CoreMatch/HRPAuth.git`):
   - The upstream endpoints **did exist** in HRPAuth core:
     `POST /user/declare-email` (added by `8ca74f3`/`5ba9776`,
     2026-07-29) and `POST /user/mojang-bind-enable` (added by `12f22bc`,
     2026-07-09, together with the register-flow overhaul); both were still
     present through the OAuth2 work (`f92f9ce`, 2026-08-19, which lists
     the service-to-service scope `user.declare-email` — HA-Contract
     [xref] `docs/dev/HRPAuth/tokens.md:L43`).
   - Both were **removed on 2026-10-06** by
     `9ab4e64` ("refactor: strip legacy Yggdrasil stack and unused cleanup
     services", 21:15 +0800) — the decoupling commit that deleted the
     in-process Yggdrasil stack from the core (within the author's
     "up to and including 10-08" window; the surrounding commits of
     10-06..10-08 continue that decoupling).
   - The WinnerProxy side was added by `f63f62b9` (2026-08-07) as client
     methods + types only ("supporting type definitions", per the commit
     message); `git log --all -S".DeclareEmail("` finds no commit ever
     adding a call site. An `AuthType` field supporting the flow was added
     in the same commit and removed by the OAuth2 refactor `05ffc49`
     (2026-08-23).
   - Verdict: a mix of cases ① and ② — the upstream side was fully
     implemented and then dropped by the decoupling (no successor for
     declare-email anywhere; mojang-bind survives only as player
     self-service `POST /player/mojang-bind-enable` on Yggdrasil-API,
     Bearer-authenticated,
     `/home/lnb/Desktop/HA/HA-Contract/docs/api/openapi/hrpauth-yggdrasil-business.yaml (field: /player/mojang-bind-enable)`),
     while the WinnerProxy side was never wired up. Evidence is sufficient
     for this verdict; what is **not** recoverable from the repos is
     whether a handler-level integration was ever planned in detail —
     no such code ever existed.
   - If these calls were made today they would hit
     `{upstreams.hrpauth.url}/user/declare-email` →
     `/yggdrasil-api/user/declare-email` → relay strips prefix →
     Yggdrasil-API has no `/user/*` route → 404 (and the HRPAuth root no
     longer registers the routes either). Original auth design per
     `f63f62b9` was M.T. ("Maintenance Token") authentication; the current
     comment says OAuth2 — neither matches any live endpoint.
   - **Update 2026-11-05**: the author confirmed declare-email deprecated
     with no successor (player-side email setting is the claim flow `/ha
     reg` → `/admin/claim-user`, now implemented on HRPAuth-Yggdrasil-API).
     The dead `DeclareEmail`/`EnableMojangBind` client methods and the dead
     `RegisterResponse.CBH` field were deleted from this repository; the
     archaeology above is kept as the historical record.
4. **Stale code comment. — FIXED 2026-11-05, commit `4ded866`.** The
   `HasJoined` client comment claimed "HA's router does NOT use a
   /yggdrasil prefix" and cited the old monolith — true only for the
   pre-split layout; the current deployment reaches the Yggdrasil surface
   through the `/yggdrasil-api` relay prefix. The comment now describes
   the relay-prefix semantics.
5. **Deployment-config question (not verified).** The compose template sets
   [xref] `HRPAUTH_YGG_URL: http://hrpauth-yggdrasil-api:2770/yggdrasil-api/`
   (direct container address *with* the relay prefix). The microservice
   serves prefix-less routes and the HRPAuth relay is what strips the
   prefix, so this value looks inconsistent with the design; it may be
   compensated by the live deployment layout. **Verify against the running
   deployment** before treating batch/hasJoined failures as code bugs.

## Consequences

- Today, hasJoined stages 1-2, profile query, root metadata, and the batch
  query (fixed 2026-11-05, commit `4ded866`) work end-to-end against the
  current upstream; proxy registration still fails as described.
  HA-Contract's WinnerProxy contract documents the same state (see
  [0005](./0005-contract-status.md)).
- Fixing stage 3: the contract already names the intended path —
  `POST /internal/proxy-register` with `X-Internal-Key` obtained from
  `POST /services/auth-key` (see item 2 for citations). Implementing it
  still requires decisions humans must make (key acquisition/refresh
  lifecycle in WinnerProxy, response-shape adaptation, and whether the
  declare-email/mojang-bind follow-up flow is revived at all) — **pending
  author confirmation**; no code is changed in this knowledge pass.
- Until stage 3 is fixed, operators should expect 503 for Mojang players at
  join time; monitoring should not treat these as outages of the proxy
  itself. Batch-query failures now surface as 502 with an explicit
  `{"error": ...}` reason (`hrpauth batch endpoint missing` on upstream
  404, `hrpauth upstream unavailable` otherwise) instead of a bare 500.
- Item 5 (compose `HRPAUTH_YGG_URL` with the relay prefix pointing at the
  microservice container) is unchanged by the code fix: with that value the
  batch call would still 404, now surfacing as the explicit 502
  `hrpauth batch endpoint missing`. The compose template belongs to the
  Deployer repository; the deployment-side value still needs verification.

## Evidence

- Batch query path: `internal/hrpauth/endpoints.go:L96-L112 @ 86866128`
  (URL at `internal/hrpauth/endpoints.go:L97 @ 86866128`), HEAD of `main`
  `86866128`; handler error mapping
  `internal/handler/handler.go:L194-L214 @ 86866128`.
- Upstream batch route: repo HRPAuth-Yggdrasil-API (@ `124b2fa1`),
  `/home/lnb/Desktop/HA/HRPAuth-Yggdrasil-API/main.go::BatchProfiles`
  (`api.POST("/profiles/minecraft", ...)` under `r.Group("/api")`).
- Relay prefix stripping: repo HRPAuth (@ `8dd67593`),
  `/home/lnb/Desktop/HA/HRPAuth/controllers/relay_controller.go::Match`,
  `/home/lnb/Desktop/HA/HRPAuth/controllers/relay_controller.go::RelayMiddleware`.
- Register body sent: `internal/hrpauth/types.go::RegisterRequest`;
  response expectation `internal/hrpauth/types.go::RegisterResponse`;
  client mapping `internal/hrpauth/endpoints.go::RegisterByProxy`.
- Upstream register contract: repo HRPAuth-Yggdrasil-API,
  `/home/lnb/Desktop/HA/HRPAuth-Yggdrasil-API/controllers/register_controller.go::RegisterGameAccountRequest`
  (`identifier`/`password` required),
  `/home/lnb/Desktop/HA/HRPAuth-Yggdrasil-API/controllers/register_controller.go::Register`
  (400 on bind failure, 403 on service error); route
  `/home/lnb/Desktop/HA/HRPAuth-Yggdrasil-API/main.go::Register`.
- Proxy-register endpoint + X-Internal-Key: repo HRPAuth-Yggdrasil-API,
  `/home/lnb/Desktop/HA/HRPAuth-Yggdrasil-API/controllers/internal_controller.go::ProxyRegister`,
  route `/home/lnb/Desktop/HA/HRPAuth-Yggdrasil-API/main.go::ProxyRegister`.
- Unwired methods: `internal/hrpauth/endpoints.go:L189-L221 @ 86866128`;
  archaeology
  evidence (repo HRPAuth @ `8dd67593`): introduction commits `8ca74f3`,
  `5ba9776` (2026-07-29, declare-email), `12f22bc` (2026-07-09,
  mojang-bind), removal commit `9ab4e64` (2026-10-06 21:15 +0800, route
  deletions visible in its `main.go` diff); WinnerProxy-side introduction
  `f63f62b9a868f25b3a3cd0a7468b81b197090d62` (2026-08-07, methods + types
  only, no call site ever — `git log --all -S".DeclareEmail("` empty);
  `AuthType` added in `f63f62b9`, removed by `05ffc49` (2026-08-23).
  Contract scope `user.declare-email`: repo HA-Contract (@ `21c7147d`),
  [xref] `docs/dev/HRPAuth/tokens.md:L43`. Current mojang-bind successor:
  repo HRPAuth-Yggdrasil-API,
  `/home/lnb/Desktop/HA/HRPAuth-Yggdrasil-API/main.go::EnableMojangBind`
  (`/player/mojang-bind-enable|disable`, player self-service).
- Stale comment: `internal/hrpauth/endpoints.go:L14-L20 @ 86866128`.
- Compose value: repo HA-Services-Deployer (@ `657bcb88`),
  `/home/lnb/Desktop/HA/HA-Services-Deployer/deploy/docker-compose.yml (field: HRPAUTH_YGG_URL)`;
  microservice serves prefix-less routes:
  repo HRPAuth-Yggdrasil-API,
  `/home/lnb/Desktop/HA/HRPAuth-Yggdrasil-API/main.go:L148-L200`; relay
  prefix is a core-side concept:
  `/home/lnb/Desktop/HA/HRPAuth-Yggdrasil-API/config/config.go::RelayPathPrefix`,
  `/home/lnb/Desktop/HA/HRPAuth-Yggdrasil-API/config/config.go::DefaultRelayPathPrefix`
  (`RelayPathPrefix` =
  "the path prefix on the Core side that relays to this service").
- Contract-side record of the same drift: repo HA-Contract (@ `21c7147d`),
  `/home/lnb/Desktop/HA/HA-Contract/docs/dev/WinnerProxy/README.md (field: Outbound Calls)`,
  "Outbound Calls (current implementation)" table.
