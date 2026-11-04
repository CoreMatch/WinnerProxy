# 0003 — Coupling points with HRPAuth core

## Status

Accepted

## Date

2026-10-22 (management/SDK integration, commit `53e6352b`; OAuth2
client-credentials since `05ffc498`, 2026-08-23)

## Context

WinnerProxy is intentionally stateless about identities — all account data
lives in HRPAuth. The proxy therefore has a wide but shallow coupling
surface with the HRPAuth core, and operators need a single list of what
breaks if HRPAuth changes.

## Decision (implemented coupling surface)

1. **Authentication**: OAuth2 client_credentials against
   `{host}/oauth/token` (host taken from the configured base URL, path
   always the root). Every outbound call carries the service Bearer token.
2. **Two clients** are constructed at startup:
   - *proxy client* on `upstreams.hrpauth.url` (default includes the
     `/yggdrasil-api` relay prefix) for Yggdrasil-plane calls;
   - *management client* on `hrpauth.url` (root) for presence/relay/SDK.
   If management credentials are unset, they **fall back** to the proxy
   client's credentials, and the management URL is derived by stripping a
   `/yggdrasil-api` suffix from the proxy URL.
3. **Yggdrasil-plane calls** (via relay prefix): `hasJoined`, profile by
   uuid, batch query, root metadata `GET /` (see
   [0004](./0004-upstream-drift-and-dead-calls.md) for which of these
   currently succeed).
4. **Proxy registration**: `POST /register` (stage 3 of hasJoined).
5. **Management-plane calls** (root, non-blocking at startup):
   - `POST /services/presence` — name (default `WinnerProxy`), TTL,
     scope `{winnerproxy, [webui-dash]}`, `security_level: 0` (explicitly
     level 0 so the Yggdrasil relay stays anonymous), `sdk_url`, and
     `root_metadata.yggdrasil` populated from the upstream's own
     `GET /` meta so HRPAuth can aggregate it.
   - `POST /services/relay` — rules mapping `/winnerproxy` + each Yggdrasil
     path (`/`, `/authserver`, `/sessionserver`, `/api/profiles/minecraft`,
     `/textures`, `/previews`, `/texture`, `/skin`, `/skins`,
     `/minecraftservices`, `/register`) to this proxy's `public_url`.
   - `POST /services/sdk-packages` (and DELETE) — compile-time SDK package
     upload for the WEBUI.
6. **Unwired client methods with removed upstream endpoints**:
   `DeclareEmail` (`POST /user/declare-email`) and `EnableMojangBind`
   (`POST /user/mojang-bind-enable`) exist in the client but have never
   had call sites. They are the remains of an unfinished post-registration
   flow; see entry
   [0004](./0004-upstream-drift-and-dead-calls.md) item 3 for the full
   archaeology. **Update 2026-11-05: both methods (and the dead
   `RegisterResponse.CBH` field) were deleted after the author confirmed
   declare-email deprecated with no successor; player-side email setting is
   the claim flow (`/ha reg` → `/admin/claim-user` on HRPAuth-Yggdrasil-API).**

## Consequences

- Any HRPAuth change to token exchange, presence/relay schemas, or the
  Yggdrasil relay prefix breaks WinnerProxy silently at runtime (failures
  are logged; presence/relay never block startup).
- The `security_level: 0` declaration is load-bearing: it is what lets
  unauthenticated Minecraft traffic pass the relay. Raising it requires a
  coordinated change.
- Item 6 is **not ordinary dead code**: the upstream endpoints once existed
  in HRPAuth core and were removed by the 2026-10-06 decoupling commit
  `9ab4e64`; the WinnerProxy side was added 2026-08-07 (`f63f62b9`) but
  never wired into any handler. Whether the flow is revived (and via which
  upstream path) is **pending author confirmation**; until then the methods
  stay in place as the only remaining record of the intended flow.
- Config fallback (item 2) means a half-configed deployment silently reuses
  proxy credentials for management; the startup log says so, but operators
  should configure the `hrpauth` section explicitly.

## Evidence

- OAuth2 wiring: `internal/hrpauth/client.go:58-89` (token URL derivation
  at `:71-77`); introduced by `05ffc498e6de294865e1d5934a177a5b9317025a`
  ("refactor(hrpauth): use OAuth2 client credentials", 2026-08-23).
- Two clients + fallback: `main.go:44-88` (HEAD of `main`, `86866128`).
- Presence request fields (security_level 0, sdk_url, root_metadata):
  `main.go:129-155`; type `internal/hrpauth/types.go:60-69`.
- Relay path list: `main.go:172-198`.
- SDK package upload/delete: `internal/hrpauth/endpoints.go:270-340`;
  startup hook `main.go:90-95`; introduced by
  `53e6352bbc3e69ca29d4a49109a9a3f47ee8d393` ("feat: add WinnerProxy SDK
  and HRPAuth management", 2026-10-22).
- Unwired methods: `internal/hrpauth/endpoints.go:189-221`; `git log
  --all -S".DeclareEmail("` returns no commit — a call site has never
  existed (verified 2026-11-04). Added by commit
  `f63f62b9a868f25b3a3cd0a7468b81b197090d62` ("feat(hrpauth): add email and
  mojang bind support", 2026-08-07), which per its commit message added
  "supporting type definitions" and the client methods only.
- Upstream-side archaeology (repo HRPAuth @ `8dd67593`): routes
  `POST /user/declare-email` and `POST /user/mojang-bind-enable` existed
  (added `5ba9776`/`8ca74f3` 2026-07-29 and `12f22bc` 2026-07-09) and were
  removed by `9ab4e64` ("refactor: strip legacy Yggdrasil stack and unused
  cleanup services", 2026-10-06 21:15 +0800). Contract scope
  `user.declare-email` for service-to-service calls: repo HA-Contract
  (@ `21c7147d`), `docs/dev/HRPAuth/tokens.md:43`.
- Default URLs: `config/config.go:156-186`.
