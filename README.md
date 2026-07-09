# WinnerProxy

> A thin Yggdrasil protocol-translation layer that lets Mojang-authenticated and HRPAuth-authenticated players share a single in-game identity.

### What is it?

WinnerProxy is a tiny Go service that sits between a Minecraft server (which only speaks Yggdrasil) and the real authentication world (Mojang official + HRPAuth). It speaks Yggdrasil on the front, and translates to Mojang sessionserver + HRPAuth on the back.

**The single design rule**:

> All of the online players should be Yggdrasil API players. Their UUIDs and profiles are always linked to the Yggdrasil API service.

Whether a player logs in with their HRPAuth password or with a Mojang/Microsoft account, the Minecraft server sees one HRPAuth-owned identity.

### Three-stage `hasJoined`

1. **HRPAuth first** — call HRPAuth's public `hasJoined`. On 200, return the HA profile (HA skin + HA UUID).
2. **Mojang fallback** — on 204, call Mojang's official `hasJoined`. On 200, take the Mojang profile and proceed to stage 3.
3. **Proxy registration** — call HRPAuth `POST /register` with the M.T. (Manage Token), passing the Mojang UUID. HRPAuth handles all binding / upsert internally and returns `profile_id`. WinnerProxy returns `{id: profile_id, name, properties: mojangProperties}` — i.e. **HA identity + Mojang skin**.

All binding, unbinding, cleanup, and direct database operations are **HRPAuth's internal concerns**. WinnerProxy is a stateless client of HRPAuth.

### Features

- **HA-first Yggdrasil proxy** — `hasJoined` / `profile/:uuid` / `api/profiles/minecraft` all backed by HRPAuth
- **Transparent Mojang → HRPAuth registration** — first-time Mojang players are auto-registered (`cbh=0`, placeholder email)
- **Single source of truth** — every in-game identity lives in HRPAuth, never in WinnerProxy
- **Built-in freecache layer** — 5-minute TTL, shields repeated `QueryProfile` and Mojang fallbacks
- **Single static binary** — no DB, no Redis, no external services
- **Interactive M.T. prompt** — first launch with a TTY asks for the HRPAuth Manage Token

### Quick start

```bash
# 1. Build
go build -o build/winnerproxy .

# 2. Run — first launch writes a default config.yml next to the
#    binary and (if stdin is a TTY) interactively asks for the M.T.
./build/winnerproxy

# 3. Edit config.yml if you skipped the prompt
$EDITOR ./config.yml   # fill upstreams.hrpauth.manage_token

# 4. Minecraft server settings
online-mode=true
yggdrasil-api-url=http://localhost:2777/

# 5. Restart
./build/winnerproxy
```

### Configuration example

```yaml
server:
  addr: ":2777"

cache:
  size: 104857600        # 100 MiB
  ttl_sec: 300           # 5 minutes

upstreams:
  official:
    enabled: true        # Mojang fallback
  hrpauth:
    url: "http://127.0.0.1:2778"
    manage_token: "<your HRPAuth manage.token>"
    enabled: true
```

### License

See [LICENSE](LICENSE).
