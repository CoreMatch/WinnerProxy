# 0002 — Three-stage hasJoined flow

## Status

Accepted

## Date

2026-07-09

## Context

Game servers call `hasJoined` after a player joins. HA wants HA-authenticated
players to use HA skins/identities, official Mojang players to still be able
to join, and — where possible — Mojang players to be absorbed into HA as
proxy-registered accounts so they get HA identities going forward.

## Decision

`hasJoined` executes a three-stage flow (query parameters are passed through
verbatim to HA, so future HA-side parameters need no WinnerProxy change):

1. **Stage 1 — HA**: `GET {hrpauth}/sessionserver/session/minecraft/hasJoined`.
   - 200 with a non-empty profile → return it; warm the HA profile cache
     (uuid-keyed) for later profile queries.
   - 200 with `{}` or 204 → treat as "no profile" (`ErrNoProfile`) and
     continue. HA's "not found" shape is 200+`{}`, which must not leak to
     the game server as an empty identity.
   - 5xx/network error → log and fall through to Mojang (availability
     choice: a sick HA should not block Mojang players).
2. **Stage 2 — Mojang**: `GET {official}/session/minecraft/hasJoined`
   (only when `upstreams.official.enabled`). Positive results are cached
   username-keyed so repeated join attempts within the TTL avoid a Mojang
   roundtrip. Cache hit or fresh hit proceeds to stage 3.
3. **Stage 3 — proxy-register**: `POST {hrpauth}/internal/proxy-register`
   with the service's `X-Internal-Key`, the Mojang name/UUID and a random
   16-byte password (base64url; never used by the player). On success the
   response is the **HA profile_id** combined with the **Mojang skin
   properties**.
   - HA 409 `username_already_bound` → reject the player with **204** (the
     name belongs to a non-bindable HA account).
   - Other failures → 503 "auth backend unavailable".
   - Mojang upstream disabled or Mojang said no → **204**.

Caching: HA profiles under uuid keys, Mojang profiles under username keys
(freecache; default 100 MiB / 300 s TTL; `cache.size=0` disables via a noop
implementation).

## Consequences

- Benefit: HA players get HA skins; Mojang players keep official skins and
  are gradually absorbed into HA; game servers see one uniform Yggdrasil
  answer.
- Cost: 204 is overloaded (no session / rejected bind); game servers treat
  both as "verification failed" by Yggdrasil convention — intentional.
- Constraint: the flow returns Mojang properties with an HA profile id —
  any signature-validation expectations on the server side must tolerate
  this mix (documented behavior in HA-Contract's WinnerProxy contract).

## Evidence

- Flow implementation: `internal/handler/handler.go::HasJoined`; stage
  comments at `internal/handler/handler.go:L49-L53`.
- HA empty-200 → ErrNoProfile: `internal/hrpauth/endpoints.go::HasJoined`
  (empty-profile check in the 200 branch).
- Mojang leg: `internal/proxy/mojang.go::HasJoined` (204/404 →
  ErrNoProfile).
- Mojang username cache: `internal/handler/handler.go::GetMojangProfile`,
  `internal/handler/handler.go::SetMojangProfile`.
- Proxy-register call and error mapping:
  `internal/handler/handler.go::RegisterByProxy`; client
  `internal/hrpauth/endpoints.go::RegisterByProxy`; random password
  `internal/handler/handler.go::generatePassword`.
- 204 on username_already_bound:
  `internal/handler/handler.go::ErrUsernameBound`; sentinel
  `internal/hrpauth/client.go::ErrUsernameBound`.
- Cache construction/defaults: `main.go::buildCache`;
  `config/config.go::Default`; noop/freecache implementations
  `internal/cache/noop.go::NewNoop`,
  `internal/cache/freecache.go::NewFreeCache`.
