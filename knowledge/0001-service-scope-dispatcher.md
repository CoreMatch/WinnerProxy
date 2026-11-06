# 0001 — Service scope: dispatcher, not proxy pool

## Status

Accepted

## Context

Minecraft servers behind the HA stack verify joining players via Yggdrasil
`hasJoined` and profile queries. Players may authenticate either against HA
itself or with official Mojang accounts. The service needed a clear answer
to: which upstream serves which request?

## Decision

WinnerProxy (Gin, default listen `:2777`) is a **request dispatcher keyed on
account origin** — explicitly *not* a generic multi-upstream proxy pool:

- Inbound surface (each Yggdrasil route bound twice: prefix-less for
  launchers/legacy probes, and under `/winnerproxy` for HA clients):
  - `GET /sessionserver/session/minecraft/hasJoined` — the only route with
    multi-upstream logic (see
    [0002](./0002-three-stage-hasjoined.md)).
  - `GET /sessionserver/session/minecraft/profile/{uuid}` — HRPAuth only
    (with an in-process profile cache).
  - `POST /api/profiles/minecraft` — HRPAuth only (summary `{id, name}`
    output; textures dropped per Yggdrasil convention).
  - `GET /` and `/winnerproxy` — pass through HRPAuth's Yggdrasil root
    metadata (skinDomains, signature public key); minimal fallback on
    failure.
  - `GET /health` — liveness.
- The Mojang upstream is consulted **only** inside `hasJoined` stage 2.
  `MojangService` implements the full `UpstreamService` interface
  (QueryProfile/BatchQuery included), but the handler never calls those
  methods — they exist for interface symmetry, not use.
- Upstream selection is config-driven: `upstreams.official.enabled` toggles
  the Mojang leg entirely; when disabled, `hasJoined` degrades to HA-only
  (204 on miss).

## Consequences

- Benefit: predictable upstream load — profile/batch traffic never leaves
  the HA perimeter; Mojang is only touched for join verification of
  non-HA players.
- Cost: dead surface in `MojangService` (QueryProfile/BatchQuery) can
  mislead readers into thinking profile queries fall back to Mojang; they
  do not.
- Constraint: adding Mojang fallback to profile/batch routes would change
  externally visible behavior and belongs to HA-Contract's process, not a
  local decision.
- The authoritative inbound/outbound table lives in HA-Contract
  (`/home/lnb/Desktop/HA/WinnerProxy/knowledge/0006-service-contract.md`);
  this entry records the internal dispatch design.

## Evidence

- Route binding (dual prefix): `internal/router/router.go::bindYggdrasil`.
- Handler dispatch comments and per-route upstream choice:
  `internal/handler/handler.go::Handler` (overview),
  `internal/handler/handler.go::QueryProfile` (QueryProfile → HRPAuth),
  `internal/handler/handler.go::BatchQuery` (BatchQuery → HRPAuth),
  `internal/handler/handler.go::YggdrasilRoot` (YggdrasilRoot → HRPAuth
  meta).
- Mojang used only in hasJoined: `internal/handler/handler.go::HasJoined`
  (`findMojang` called only here); `internal/handler/handler.go::findMojang`
  (`findMojang`).
- Unused Mojang methods: `internal/proxy/mojang.go::QueryProfile`,
  `internal/proxy/mojang.go::BatchQuery`
  (QueryProfile/BatchQuery implemented, no handler call site).
- Interface symmetry note: `internal/proxy/upstream.go::UpstreamService`.
- Mojang toggle: `main.go:L106-L114`; default upstream config
  `config/config.go::Default` (`official` = `https://api.minecraftservices.com`,
  [xref] `hrpauth` = `http://127.0.0.1:2778/yggdrasil-api/`).
