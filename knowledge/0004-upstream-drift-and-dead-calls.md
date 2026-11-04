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
   the client maps. Net effect: stage 3 fails → hasJoined returns 503 for
   Mojang players.
   **Contract-sanctioned path (reported only; no code change yet):** the
   contract's purpose-built endpoint is `POST /internal/proxy-register`
   (repo HA-Contract @ `21c7147d`,
   `docs/api/openapi/hrpauth-yggdrasil-business.yaml:322-345`):
   authenticated via the `X-Internal-Key` header (parameter definition at
   `:357-363` — "Temporary internal key issued by HRPAuth Core
   (`POST /services/auth-key`)"), body `{username, password,
   mojang_uuid}`, response `data: {account_id, profile_id}`. The internal
   key itself is obtained from HRPAuth via `POST /services/auth-key`
   authenticated with a client-credentials Bearer
   (`docs/dev/HRPAuth/microservices.md:97-118`). Additional mismatches a
   future fix must handle: the contract response carries `profile_id` but
   not `uid`/`cbh` (the `cbh` mechanism was removed from HRPAuth by commit
   `2efc6e2` "feat: add yggdrasil account claim and remove cbh code",
   2026-10-07, while `RegisterResponse.CBH` still exists in
   `internal/hrpauth/types.go:34-40`), and the contract's error set
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
     `docs/dev/HRPAuth/tokens.md:43`).
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
     Bearer-authenticated, `hrpauth-yggdrasil-business.yaml:266-293`),
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
- Fixing stage 3: the contract already names the intended path —
  `POST /internal/proxy-register` with `X-Internal-Key` obtained from
  `POST /services/auth-key` (see item 2 for citations). Implementing it
  still requires decisions humans must make (key acquisition/refresh
  lifecycle in WinnerProxy, response-shape adaptation, and whether the
  declare-email/mojang-bind follow-up flow is revived at all) — **pending
  author confirmation**; no code is changed in this knowledge pass.
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
- Unwired methods: `internal/hrpauth/endpoints.go:189-221`; archaeology
  evidence (repo HRPAuth @ `8dd67593`): introduction commits `8ca74f3`,
  `5ba9776` (2026-07-29, declare-email), `12f22bc` (2026-07-09,
  mojang-bind), removal commit `9ab4e64` (2026-10-06 21:15 +0800, route
  deletions visible in its `main.go` diff); WinnerProxy-side introduction
  `f63f62b9a868f25b3a3cd0a7468b81b197090d62` (2026-08-07, methods + types
  only, no call site ever — `git log --all -S".DeclareEmail("` empty);
  `AuthType` added in `f63f62b9`, removed by `05ffc49` (2026-08-23).
  Contract scope `user.declare-email`: repo HA-Contract (@ `21c7147d`),
  `docs/dev/HRPAuth/tokens.md:43`. Current mojang-bind successor:
  repo HRPAuth-Yggdrasil-API, `main.go:157-158`
  (`/player/mojang-bind-enable|disable`, player self-service).
- Stale comment: `internal/hrpauth/endpoints.go:14-20`.
- Compose value: repo HA-Services-Deployer (@ `657bcb88`),
  `deploy/docker-compose.yml:136`; microservice serves prefix-less routes:
  repo HRPAuth-Yggdrasil-API, `main.go:147-200`; relay prefix is a
  core-side concept: `config/config.go:36-40,127` (`RelayPathPrefix` =
  "the path prefix on the Core side that relays to this service").
- Contract-side record of the same drift: repo HA-Contract (@ `21c7147d`),
  `docs/dev/WinnerProxy/README.md`, "Outbound Calls (current
  implementation)" table.
