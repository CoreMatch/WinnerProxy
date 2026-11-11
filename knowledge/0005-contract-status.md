# 0005 — Contract status

## Status

Accepted

## Context

HA-Contract keeps per-service OpenAPI files under `docs/api/openapi/`.
WinnerProxy's external surface is largely the standard Yggdrasil protocol
(defined externally), which HA-Contract documents for the whole ecosystem.

## Decision

1. WinnerProxy's external behavior is documented by:
   - HA-Contract
     `/home/lnb/Desktop/HA/HA-Contract/docs/api/openapi/winnerproxy-business.yaml`
     — wired into HA-Contract's method-level drift check.
   - HA-Contract
     `/home/lnb/Desktop/HA/hrpauth/WinnerProxy/knowledge/0006-service-contract.md`
     — "WinnerProxy Service Contract": inbound route table, outbound call
     table, configuration notes.
   - HA-Contract Yggdrasil documentation
     (`/home/lnb/Desktop/HA/hrpauth/HRPAuth-Yggdrasil-API/knowledge/0010-yggdrasil-api-surface.md`,
     `/home/lnb/Desktop/HA/HA-Contract/docs/api/openapi/hrpauth-yggdrasil-business.yaml`)
     for the upstream semantics WinnerProxy passes through.
2. **Handling policy**:
   - The implementation reference is the code plus the HA-Contract dev
     contract; this repo's README stays deployment-oriented (config file
     behavior, ports, links) and does not define API behavior.
   - Gaps are fixed **in HA-Contract**. This knowledge base never defines
     external behavior to fill a gap.
   - Keep code-vs-contract drift at zero: land the contract change and the
     code change together.

## Consequences

- The inbound surface has a machine-checkable OpenAPI file that the
  HA-Contract drift script compares against the registered routes.
- The outbound-call state is documented in the dev contract's outbound
  table, not in the OpenAPI file, which only describes WinnerProxy's own
  inbound surface.

## Evidence

- OpenAPI file: repo HA-Contract,
  `/home/lnb/Desktop/HA/HA-Contract/docs/api/openapi/winnerproxy-business.yaml`.
- Dev contract: repo HA-Contract,
  `/home/lnb/Desktop/HA/hrpauth/WinnerProxy/knowledge/0006-service-contract.md`
  (inbound surface table; outbound call table).
- This repo's README scope (deployment/config only): `README.md:L1-L18 fp=2dc13979 ~ WinnerProxy`.
- Yggdrasil protocol documentation pointers: repo HA-Contract,
  `/home/lnb/Desktop/HA/HA-Contract/docs/api/README.md (field: External Protocol Compliance)`.
