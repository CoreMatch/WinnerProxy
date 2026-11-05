# 0003 — Coupling points with HRPAuth core

## Status

Accepted

## Context

WinnerProxy is intentionally stateless about identities — all account data
lives in HRPAuth. The proxy therefore has a wide but shallow coupling
surface with the HRPAuth core, and operators need a single list of what
breaks if HRPAuth changes.

## Decision (implemented coupling surface)

1. **Authentication**: OAuth2 client_credentials against
   `{host}/oauth/token` (host taken from the configured base URL, path
   always the root). Every outbound call carries the service Bearer token.
   At startup the configured credentials go through **credential
   resolution** (`internal/hrpauth/selfregister.go::ResolveServiceCredentials`):
   they are presented to HRPAuth's RFC 7591 protected registration
   endpoint `POST /oauth/register` (client_secret_basic), which only
   authenticates the built-in super client —
   - `201`: the credentials are the super client and are exchanged for
     WinnerProxy's own low-privilege client (`client_id` = presence name,
     stable secret via HRPAuth get-or-create); the super secret is
     discarded from memory (`main.go::applyResolvedCredentials`) and the
     config file is rewritten with the per-service pair (see item 6);
   - `400 invalid_client_metadata`: super credentials, but registration
     rejected (presence missing, scope not allowed) — startup aborts;
   - `401`: not super; a plain `client_credentials` token request
     verifies the pair is an ordinary service client, which is then used
     as-is (idempotent path — no exchange, no write-back).
   Any resolution failure aborts startup (`log.Fatalf`): super
   credentials must never reach business requests, and there is no
   fallback to unverified credentials. The presence bonjour goes first
   (registration requires a presence-registered name) as a token-less
   plain request (`internal/hrpauth/selfregister.go::RegisterPresencePlain`).
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
6. **Credential write-back** after a successful super exchange
   (`config/configwrite.go::RewriteClientCredentials`): every
   `hrpauth` / `upstreams.hrpauth` section carrying the super pair is
   rewritten to the per-service pair — atomically (temp file + rename),
   with the previous content kept as `config.yml.bak`, permissions
   tightened to `0600`, and the new bytes validated by re-parsing before
   any file is touched. Two honest limitations:
   - Entrypoint-rendered deployments
     (`config/configwrite.go::IsRenderedDeployment`: `CONFIG_TEMPLATE` +
     `RENDERED_CONFIG`, the HA container convention) re-render the config
     from template + environment on every start, so nothing is written
     and a WARN points at the template/environment side;
   - a failed write-back is not fatal — the service keeps running with
     the in-memory per-service credentials and logs an ERROR stating the
     next startup will exchange again.
   The exchanged secret is stable server-side (HRPAuth get-or-create), so
   even without write-back a restart re-derives the same client; the
   write-back merely skips the exchange round-trip. The config file keeps
   whatever credentials were written — WinnerProxy never deletes
   credentials from anywhere but its own memory.

## Consequences

- Any HRPAuth change to token exchange, presence/relay schemas, or the
  Yggdrasil relay prefix breaks WinnerProxy at runtime: presence/relay
  failures are logged and never block startup, while credential
  resolution failures abort startup by design (item 1).
- The `security_level: 0` declaration is load-bearing: it is what lets
  unauthenticated Minecraft traffic pass the relay. Raising it requires a
  coordinated change.
- Config fallback (item 2) means a half-configed deployment silently reuses
  proxy credentials for management; the startup log says so, but operators
  should configure the `hrpauth` section explicitly.
- Credential resolution makes HRPAuth reachability a **startup
  requirement** whenever credentials are configured: an unreachable or
  rejecting HRPAuth aborts the process instead of degrading to
  super-credential operation.
- WinnerProxy needs **no specific OAuth scopes**: every endpoint it calls
  is either unauthenticated (`/services/presence`), internal-key
  authenticated (`/internal/proxy-register`), ops-level — which checks
  service-token subject type, not the scope list
  (`/services/auth-key`, `/services/relay`, `/services/sdk-packages`) —
  or Yggdrasil-plane, where HRPAuth OAuth tokens are ignored. Its
  HRPAuth-side scope allowlist entry is therefore correctly empty.

## Evidence

- OAuth2 wiring: `internal/hrpauth/client.go::New` (token URL derivation
  at `internal/hrpauth/client.go::tokenURL`).
- Credential resolution / super exchange:
  `internal/hrpauth/selfregister.go::ResolveServiceCredentials`,
  `internal/hrpauth/selfregister.go::RegisterPresencePlain`; startup
  wiring `main.go::applyResolvedCredentials`.
- Write-back: `config/configwrite.go::RewriteClientCredentials`,
  rendered-deployment detection `config/configwrite.go::IsRenderedDeployment`
  (env convention set by `docker-entrypoint.sh`).
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
