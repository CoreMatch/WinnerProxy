# WinnerProxy Knowledge Base

This directory is the project's own memo book: Architecture Decision
Records (ADRs) that explain **why the code looks the way it does**, kept for
self-hosting and maintenance.

## Positioning

- These notes serve the people who operate and maintain WinnerProxy. They
  record decisions, their costs, and the evidence behind them.
- **They do not define externally visible behavior.** External behavior is
  defined by [HA-Contract](https://github.com/CoreMatch/HA-Contract) —
  for this service primarily `docs/dev/WinnerProxy/README.md` (the
  "WinnerProxy Service Contract", including the inbound surface and the
  outbound-call table), plus the Yggdrasil protocol documentation under
  `docs/dev/HRPAuth/` and `docs/api/openapi/hrpauth-yggdrasil-business.yaml`
  for the upstream semantics.
- If an entry here conflicts with HA-Contract, **fix the entry, not the
  contract**. If the code conflicts with the contract, record the drift in
  an entry and resolve it through HA-Contract's change process.

## Rules

- Entries are numbered `NNNN-<slug>.md`, written in English, and follow:
  Status / Date / Context / Decision / Consequences / Evidence.
- Evidence must be checked against the repository before writing
  (`file:line`, commit hash). Anything that could not be verified is
  marked `not verified`. Gaps are stated honestly, never papered over.
- New entries get the next free number; superseded entries change Status
  to `Superseded by NNNN` instead of being deleted.

## Entries

| # | Title | Status | Date | One-line assertion |
|---|-------|--------|------|--------------------|
| [0001](./0001-service-scope-dispatcher.md) | Service scope: dispatcher, not proxy pool | Accepted | 2026-07-09 | WinnerProxy answers Yggdrasil join/profile queries by dispatching per account origin; only `hasJoined` ever fans out to Mojang — everything else is HRPAuth-only. |
| [0002](./0002-three-stage-hasjoined.md) | Three-stage hasJoined flow | Accepted | 2026-07-09 | `hasJoined` tries HA first, then Mojang, then proxy-registers the Mojang player in HA; ambiguous/negative outcomes return 204, and `username_already_bound` rejects with 204. |
| [0003](./0003-hrpauth-coupling-points.md) | Coupling points with HRPAuth core | Accepted | 2026-10-22 | WinnerProxy depends on HRPAuth OAuth2 client-credentials plus a fixed set of endpoints (Yggdrasil surface, `/register`, `/services/*`); two clients (proxy vs management) with credential fallback. |
| [0004](./0004-upstream-drift-and-dead-calls.md) | Upstream drift and dead outbound calls | Accepted | 2026-11-04 | Batch query (double `/yggdrasil` prefix) and proxy registration (`/register` body mismatch; the real endpoint is `/internal/proxy-register` with X-Internal-Key) do not succeed against the current upstream; declare-email/mojang-bind client methods are dead code. |
| [0005](./0005-contract-status.md) | Contract status | Accepted | 2026-11-04 | There is no OpenAPI file for WinnerProxy; its external surface is documented by HA-Contract `docs/dev/WinnerProxy/README.md`, which already records the same broken outbound calls. |
