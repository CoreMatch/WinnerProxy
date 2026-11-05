# 0003 — Coupling points with HRPAuth core

## Status

Accepted

## Date

2026-10-22

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
4. **Proxy registration**: `POST /internal/proxy-register` with the
   service's `X-Internal-Key`, issued by the core's
   `POST /services/auth-key` (stage 3 of hasJoined).
5. **Management-plane calls** (root, non-blocking at startup):
   - `POST /services/presence` — name (default `WinnerProxy`), TTL,
     scope `{winnerproxy, [webui-dash]}`, `security_level: 0` (explicitly
     level 0 so the Yggdrasil relay stays anonymous), and
     `root_metadata.yggdrasil` populated from the upstream's own
     `GET /` meta so HRPAuth can aggregate it.
   - `POST /services/relay` — rules mapping `/winnerproxy` + each Yggdrasil
     path (`/`, `/authserver`, `/sessionserver`, `/api/profiles/minecraft`,
     `/textures`, `/previews`, `/texture`, `/skin`, `/skins`,
     `/minecraftservices`) to this proxy's `public_url`.
   - `POST /services/sdk-packages` (and DELETE) — compile-time SDK package
     upload for the WEBUI.

## Consequences

- Any HRPAuth change to token exchange, presence/relay schemas, or the
  Yggdrasil relay prefix breaks WinnerProxy silently at runtime (failures
  are logged; presence/relay never block startup).
- The `security_level: 0` declaration is load-bearing: it is what lets
  unauthenticated Minecraft traffic pass the relay. Raising it requires a
  coordinated change.
- Config fallback (item 2) means a half-configed deployment silently reuses
  proxy credentials for management; the startup log says so, but operators
  should configure the `hrpauth` section explicitly.

## Evidence

- OAuth2 wiring: `internal/hrpauth/client.go::New` (token URL derivation
  at `internal/hrpauth/client.go::tokenURL`).
- Two clients + fallback: `main.go::hrpauthMgmtCli`.
- Presence request fields (security_level 0, root_metadata):
  `main.go::announcePresence`; type
  `internal/hrpauth/types.go::PresenceRequest`.
- Relay path list: `main.go::buildRelayRules`.
- SDK package upload/delete:
  `internal/hrpauth/endpoints.go::UploadSDKPackage`,
  `internal/hrpauth/endpoints.go::DeleteSDKPackage`; startup hook
  `main.go::uploadSDKPackage`.
- Default URLs: `config/config.go::Default`.
