> 来源：HA-Contract@bab2a05（2026-11-07），原路径 docs/dev/WinnerProxy/README.md（契约仓知识库回迁）。

# WinnerProxy Service Contract

WinnerProxy is the server-join proxy for the HA ecosystem. It answers
Yggdrasil `hasJoined` / profile queries for Minecraft servers, falling back
from the HA upstream to Mojang, and can proxy-register Mojang players.

## Inbound HTTP Surface

The last three routes are bound both at the root and under the `/winnerproxy`
prefix.

| Method | Path | Purpose |
|---|---|---|
| GET | `/health` | Liveness probe |
| GET | `/` | Service banner / status page |
| GET | `/winnerproxy` | Service banner / status page (alias) |
| GET | `/sessionserver/session/minecraft/hasJoined` | Three-step join verification: HA upstream first, then Mojang, then proxy registration |
| GET | `/sessionserver/session/minecraft/profile/{uuid}` | Profile query (HA upstream, Mojang fallback) |
| POST | `/api/profiles/minecraft` | Batch username → profile summary |

## Outbound Calls (current implementation)

Yggdrasil-plane calls go to the configured upstream base URL
(`upstreams.hrpauth.url`, default includes the `/yggdrasil-api` relay
prefix); management-plane calls go to the management base URL
(`hrpauth.url`, the core root). Business calls carry an OAuth2
client-credentials Bearer token; proxy registration additionally carries
the `X-Internal-Key`. Startup credential resolution is the exception: the
presence bonjour and `POST /oauth/register` go out without a Bearer token
(the register call authenticates with `client_secret_basic`), because the
configured credentials may still be the super client and the super secret
must never ride along on anything but the exchange itself.

| Call | Used by | Notes |
|---|---|---|
| `GET /sessionserver/session/minecraft/hasJoined` | hasJoined step 1 | HA skin path |
| `GET /sessionserver/session/minecraft/profile/{uuid}` | profile query | |
| `POST /api/profiles/minecraft` | batch query | The base URL carries the `/yggdrasil-api` relay prefix and the relay strips it. Upstream failures surface as 502 with an explicit `{"error": ...}` reason |
| `POST /internal/proxy-register` | proxy registration (hasJoined step 3) | Body `{username, password, mojang_uuid}`; authenticated with `X-Internal-Key` (contract `hrpauth-yggdrasil-business.yaml` `/internal/proxy-register`). Consumes `profile_id` from the envelope response |
| `POST /oauth/register` | startup credential resolution | RFC 7591 protected registration (`client_secret_basic`): classifies the configured credentials — when they are the core's built-in super client they are exchanged for WinnerProxy's own low-privilege client (stable `client_id`/`client_secret` via get-or-create) and the config file is rewritten with it; already-low-privilege credentials are verified via `POST /oauth/token` and reused unchanged. Resolution failure aborts startup (contract `hrpauth-business.yaml` `/oauth/register`) |
| `POST /services/auth-key` | startup, and after any 401 from proxy registration | Fetches/re-fetches the service's `X-Internal-Key` (core re-issues the key on every call) |
| `GET /` | startup | Fetches Yggdrasil metadata published via presence `root_metadata` |
| `POST /services/presence` | startup | Registers with `security_level: 0` and `root_metadata.yggdrasil` |
| `POST /services/relay` | startup | Declares relay rules |
| `POST /services/sdk-packages` (and `DELETE /services/sdk-packages/{name}`) | startup | Compile-time SDK package upload for the WebUI |

## Configuration

Runtime configuration of the WinnerProxy service (configuration file, listen
address defaults, upstream credentials) is documented in the
[WinnerProxy repository README](https://github.com/CoreMatch/WinnerProxy).

## Behavior Notes

- Single-instance design: in-process freecache, no shared state, no leader
  election.
- hasJoined flow: HA 200 → return HA profile; else Mojang hasJoined → return
  Mojang profile (Mojang-signed textures); else optional proxy registration
  path.
- On startup the service registers presence on the core and contributes a
  `yggdrasil` metadata block to the core `GET /` response via `root_metadata`.
