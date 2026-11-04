# 0001 — Service scope: dispatcher, not proxy pool

## Status

Accepted

## Date

2026-07-09 (three-stage HA-first rewrite, commit `41f5ff14`; rebranded to
WinnerProxy in `a729f4e6`, 2026-10-22)

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
  (`docs/dev/WinnerProxy/README.md`); this entry records the internal
  dispatch design.

## Evidence

- Route binding (dual prefix): `internal/router/router.go:13-32` (HEAD of
  `main`, `86866128`).
- Handler dispatch comments and per-route upstream choice:
  `internal/handler/handler.go:20-27` (overview), `:172-189` (QueryProfile
  → HRPAuth), `:194-214` (BatchQuery → HRPAuth), `:220-227` (YggdrasilRoot
  → HRPAuth meta).
- Mojang used only in hasJoined: `internal/handler/handler.go:106-115`
  (`findMojang` called only here); `:150-157` (`findMojang`).
- Unused Mojang methods: `internal/proxy/mojang.go:63-124`
  (QueryProfile/BatchQuery implemented, no handler call site — verified by
  searching call sites, 2026-11-04).
- Interface symmetry note: `internal/proxy/upstream.go:12-19`.
- Mojang toggle: `main.go:106-114`; default upstream config
  `config/config.go:173-186` (`official` = `https://api.minecraftservices.com`,
  `hrpauth` = `http://127.0.0.1:2778/yggdrasil-api/`).
- Commits: `41f5ff14048b0b04ad236889fd6a9bab4d59c85f` ("feat: replace
  HasJoined with three-stage HA-first flow", 2026-07-09);
  `a729f4e6f3fcff45591481da6d4e7fad999161fc` ("feat: rebrand to winnerproxy
  and add custom sdk url config", 2026-10-22).
