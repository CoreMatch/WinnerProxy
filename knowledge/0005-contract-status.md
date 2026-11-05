# 0005 — Contract status

## Status

Accepted

## Date

2026-11-04 (updated 2026-11-05: `winnerproxy-business.yaml` now exists)

## Context

HA-Contract keeps per-service OpenAPI files under `docs/api/openapi/`.
Until 2026-11-05 WinnerProxy had **no** OpenAPI file there; its external
surface is largely the standard Yggdrasil protocol (defined externally),
which HA-Contract documents for the whole ecosystem.

## Decision

1. **Current state (updated 2026-11-05):** WinnerProxy's external
   behavior is documented by:
   - HA-Contract
     `/home/lnb/Desktop/HA/HA-Contract/docs/api/openapi/winnerproxy-business.yaml`
     — added by the 2026-11-05 contract-coverage audit (HA-Contract
     `c72bf82`) and wired into HA-Contract's method-level drift check.
   - HA-Contract
     `/home/lnb/Desktop/HA/HA-Contract/docs/dev/WinnerProxy/README.md`
     — "WinnerProxy Service Contract": inbound route table, outbound call
     table, configuration notes.
   - HA-Contract Yggdrasil documentation
     (`/home/lnb/Desktop/HA/HA-Contract/docs/dev/HRPAuth/yggdrasil-api.md`,
     `/home/lnb/Desktop/HA/HA-Contract/docs/api/openapi/hrpauth-yggdrasil-business.yaml`)
     for the upstream semantics WinnerProxy passes through.
2. **Handling policy** (unchanged):
   - The implementation reference is the code plus the HA-Contract dev
     contract; this repo's README stays deployment-oriented (config file
     behavior, ports, links) and does not define API behavior.
   - Gaps are fixed **in HA-Contract**. This knowledge base never defines
     external behavior to fill a gap.
   - Keep code-vs-contract drift at zero: when fixing
     [0004](./0004-upstream-drift-and-dead-calls.md) items, land the
     contract change and the code change together. The batch-query fix
     (`4ded866`, 2026-11-05) was paired with the corresponding
     HA-Contract update (outbound table, `winnerproxy-business.yaml`
     error mapping, `yggdrasil-api.md` §9 note).

## Consequences

- The inbound surface now has a machine-checkable OpenAPI file that the
  HA-Contract drift script compares against the registered routes.
- The outbound-call state is documented in the dev contract's outbound
  table, not in the OpenAPI file, which only describes WinnerProxy's own
  inbound surface.

## Evidence

- OpenAPI file: repo HA-Contract,
  `/home/lnb/Desktop/HA/HA-Contract/docs/api/openapi/winnerproxy-business.yaml`,
  added by `c72bf82` ("contract: add WinnerProxy business API spec
  (coverage audit)", 2026-11-05).
- Dev contract: repo HA-Contract,
  `/home/lnb/Desktop/HA/HA-Contract/docs/dev/WinnerProxy/README.md`
  (inbound surface table; outbound call table).
- This repo's README scope (deployment/config only): `README.md:L1-L18`.
- Yggdrasil protocol documentation pointers: repo HA-Contract,
  `/home/lnb/Desktop/HA/HA-Contract/docs/api/README.md (field: External Protocol Compliance)`.
