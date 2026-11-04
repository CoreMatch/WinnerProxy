# 0005 — Contract status

## Status

Accepted

## Date

2026-11-04

## Context

HA-Contract keeps per-service OpenAPI files under `docs/api/openapi/`.
WinnerProxy has **no** OpenAPI file there. Its external surface is largely
the standard Yggdrasil protocol (defined externally), which HA-Contract
documents for the whole ecosystem.

## Decision

1. **Current state (verified 2026-11-04):** WinnerProxy's external
   behavior is documented by:
   - HA-Contract `docs/dev/WinnerProxy/README.md` — "WinnerProxy Service
     Contract": inbound route table, outbound call table (including the
     two calls known not to succeed), configuration notes.
   - HA-Contract Yggdrasil documentation (`docs/dev/HRPAuth/yggdrasil-api.md`,
     `docs/api/openapi/hrpauth-yggdrasil-business.yaml`) for the upstream
     semantics WinnerProxy passes through.
   There is no `winnerproxy-business.yaml`.
2. **Handling while the contract file is missing** (policy):
   - The implementation reference is the code plus the HA-Contract dev
     contract; this repo's README stays deployment-oriented (config file
     behavior, ports, links) and does not define API behavior.
   - Gaps are fixed **in HA-Contract**. This knowledge base never defines
     external behavior to fill a gap.
   - The dev contract already matches the code's drift state (broken batch
     query / proxy registration are documented there), so code-vs-contract
     drift is currently zero; keep it that way when fixing
     [0004](./0004-upstream-drift-and-dead-calls.md) — land the contract
     change and the code change together.

## Consequences

- No machine-checkable OpenAPI for this service; reviewers compare against
  the dev contract manually.
- Because the inbound surface is standard Yggdrasil, an OpenAPI file would
  mostly restate protocol shapes; the dev-contract table may remain the
  right level of documentation — whether to ever add a YAML file is
  **pending author confirmation**.

## Evidence

- OpenAPI directory contents (no winnerproxy file): repo HA-Contract
  (@ `21c7147d`), `docs/api/openapi/` listing (verified 2026-11-04).
- Dev contract: repo HA-Contract, `docs/dev/WinnerProxy/README.md`
  (inbound surface table; outbound table with the two broken calls marked
  "does not succeed against the current upstream").
- This repo's README scope (deployment/config only): `README.md:1-19`.
- Yggdrasil protocol documentation pointers: repo HA-Contract,
  `docs/api/README.md` (External Protocol Compliance section).
