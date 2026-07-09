# WinnerProxy

> A thin protocol-translation layer that lets Mojang-authenticated Minecraft players enter a Yggdrasil-only server, with [HRPAuth](https://example.com/hrpauth) as the identity source of truth.

## What is this?

WinnerProxy is a small Go service that sits between a Minecraft server (which only knows Yggdrasil) and the real authentication world (Mojang official + HRPAuth). It speaks Yggdrasil on the front, and translates to Mojang sessionserver + HRPAuth on the back.

The single design rule:

> **All of the online players should be also Yggdrasil API players. The UUID and profile of the online players will be linked to the Yggdrasil API service.**

In practice: HRPAuth is the source of truth for every player's in-game identity. Whether a player logs in with their HRPAuth password or with a Mojang/Microsoft account, they appear as the same player to the Minecraft server.

## How it works

```
┌──────────┐                       ┌──────────────┐                  ┌─────────┐
│ Minecraft│ ──hasJoined(u, sId)──► │              │ ──HasJoined────► │ HRPAuth │
│ server   │                       │              │                  │ (公开端点)│
│ (Yggdra- │ ◄──── profile ─────── │ WinnerProxy  │ ◄── 200/204 ─── │         │
│  sil     │                       │              │                  └─────────┘
│  only)   │                       │              │
└──────────┘                       │              │ ──HasJoined────► ┌─────────┐
                                   │              │                  │ Mojang  │
                                   │              │ ◄── 200/204 ─── │ sessionserver│
                                   │              │                  └─────────┘
                                   │              │
                                   │              │ ──Register─────► ┌─────────┐
                                   │              │   (M.T. 鉴权)   │ HRPAuth │
                                   │              │ ◄── profile_id─ │ /register│
                                   │              │                  └─────────┘
                                   └──────────────┘
```

Three-stage `hasJoined` flow:

1. **HRPAuth auth path** — try HRPAuth's public `hasJoined` first. If the player has an active HRPAuth session, return HRPAuth's profile (HRPAuth skin, HRPAuth UUID).
2. **Mojang auth path** — on 204, try Mojang's official `hasJoined`. If the player has a valid Mojang session, take the returned Mojang profile and forward it to stage 3.
3. **Proxy registration** — call HRPAuth's `POST /register` with HRPAuth's M.T. (Manage Token), passing the Mojang UUID. HRPAuth handles all internal binding/upsert logic and returns a `profile_id`. WinnerProxy returns `{id: profile_id, name, properties: mojangProperties}` — i.e., HRPAuth identity with Mojang skin.

All binding, unbinding, cleanup, and direct database operations are HRPAuth's internal concerns. WinnerProxy is a stateless client of HRPAuth.

## Features

- **Thin Yggdrasil proxy** — `hasJoined` / `profile/:uuid` / `api/profiles/minecraft` all transparently backed by HRPAuth
- **Seamless Mojang → HRPAuth registration** — first-time Mojang players are auto-registered into HRPAuth (with `cbh=0` and a placeholder email)
- **Single source of truth** — every in-game player identity is owned by HRPAuth, never by WinnerProxy
- **Single static binary** — no DB, no Redis, no external dependencies
- **Hot-configurable upstream** — Mojang and HRPAuth timeouts set in `config.yml`

## Requirements

- Go 1.26.4 or later
- A reachable HRPAuth instance (recommended: same operator, same machine)
- Outbound HTTPS to `sessionserver.mojang.com` and `api.minecraftservices.com`

## Build & Run

```bash
go build -o build/winnerproxy .
./build/winnerproxy
```

On first launch, WinnerProxy writes a default `config.yml` next to the executable. Edit it to point at your HRPAuth instance, then restart.

## Quick Example

Configure `config.yml`:

```yaml
upstreams:
  official:
    enabled: true
  hrpauth:
    url: "http://127.0.0.1:2880"
    manage_token: "<your HRPAuth manage.token>"
    enabled: true
```

Point your Minecraft server's `server.properties`:

```properties
online-mode=true
yggdrasil-api-url=http://localhost:2779/yggdrasil
```

Done. Mojang players will get auto-registered into HRPAuth on first join; HRPAuth users keep their identity regardless of how they log in.

## Documentation

The full documentation lives in [`docs/wiki/`](./docs/wiki/):

- [Architecture](./docs/wiki/architecture.md) — components, data flow, identity model
- [Configuration](./docs/wiki/configuration.md) — every config key explained
- [Deployment](./docs/wiki/deployment.md) — single instance, Docker, production checklist
- [API Reference](./docs/wiki/api.md) — every endpoint
- [Data Flow](./docs/wiki/data-flow.md) — the three-stage `hasJoined` in detail
- [Troubleshooting](./docs/wiki/troubleshooting.md) — common issues

Companion documents:

- [HA Change Roadmap](./docs/HA-ROADMAP.md) — what HRPAuth needs to do to support this
- [Development Roadmap](./docs/DEVELOPMENT-ROADMAP.md) — how WinnerProxy is being refactored

## License

See [LICENSE](LICENSE).
