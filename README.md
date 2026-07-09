# WinnerProxy

> 一个轻量的 Yggdrasil 协议翻译层，让 Mojang 玩家和 HRPAuth 玩家能在同一台 Minecraft 服务器上共用一套身份。  
> A thin Yggdrasil protocol-translation layer that lets Mojang-authenticated and HRPAuth-authenticated players share a single in-game identity.

---

## 中文

### 它是什么？

WinnerProxy 是一个 Go 写的微小服务，架在 Minecraft 服务端（只认 Yggdrasil）和真实身份世界（Mojang 官方 + HRPAuth）之间。前端讲 Yggdrasil，后端讲 Mojang sessionserver + HRPAuth。

**单一设计原则**：

> 所有在线玩家都必须是 Yggdrasil API 玩家；他们的 UUID 和 profile 始终挂在 Yggdrasil API 服务上。

无论玩家用 HRPAuth 密码进服还是用 Mojang/Microsoft 账号进服，服务器看到的都是同一个 HRPAuth 身份。

### 三段式 `hasJoined`

1. **HRPAuth 优先** —— 先打 HRPAuth 公开 `hasJoined`。命中即返回（HRPAuth 皮肤 + HRPAuth UUID）。
2. **Mojang 兜底** —— 204 时打 Mojang 官方 `hasJoined`。命中拿 Mojang profile 进阶段 3。
3. **代注册** —— 用 HRPAuth 的 M.T. 调 `POST /register`，把 Mojang UUID 透传。HRPAuth 内部负责所有 binding/upsert 逻辑，返回 `profile_id`。WinnerProxy 返回 `{id: profile_id, name, properties: mojangProperties}` —— 即 **HRPAuth 身份 + Mojang 皮肤**。

绑定、解绑、清理、数据库写入 **全部在 HRPAuth 内部** 完成，WinnerProxy 永远是无状态的 HRPAuth 客户端。

### 特性

- **HA-First Yggdrasil 代理** —— `hasJoined` / `profile/:uuid` / `api/profiles/minecraft` 全部由 HRPAuth 支撑
- **Mojang → HRPAuth 透明代注册** —— 首次进服的 Mojang 玩家自动落入 HRPAuth（`cbh=0` + 占位邮箱）
- **单一数据源** —— 所有玩家身份归 HRPAuth 所有，WinnerProxy 不持久化任何身份数据
- **内置 freecache 缓存层** —— 5 分钟 TTL，挡掉重复的 `QueryProfile` 和 Mojang 兜底
- **单文件二进制** —— 无 DB / 无 Redis / 无外部服务依赖
- **交互式 M.T. 配置** —— 首次启动时在 TTY 下输入 HRPAuth Manage Token

### 快速开始

```bash
# 1. 编译
go build -o build/winnerproxy .

# 2. 启动（首次会在可执行文件同目录生成 config.yml，并在 TTY 下提示输入 M.T.）
./build/winnerproxy

# 3. 编辑 config.yml（如果上面没填 M.T.）
$EDITOR ./config.yml   # 填 upstreams.hrpauth.manage_token

# 4. Minecraft server.properties
online-mode=true
yggdrasil-api-url=http://localhost:2779/yggdrasil

# 5. 重新启动
./build/winnerproxy
```

### 配置示例

```yaml
server:
  addr: ":2779"

cache:
  size: 104857600        # 100 MiB
  ttl_sec: 300           # 5 分钟

upstreams:
  official:
    enabled: true        # Mojang 兜底
  hrpauth:
    url: "http://127.0.0.1:2880"
    manage_token: "<your HRPAuth manage.token>"
    enabled: true
```

### 协议 / License

见 [LICENSE](LICENSE)。

---

## English

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

# 4. Minecraft server.properties
online-mode=true
yggdrasil-api-url=http://localhost:2779/yggdrasil

# 5. Restart
./build/winnerproxy
```

### Configuration example

```yaml
server:
  addr: ":2779"

cache:
  size: 104857600        # 100 MiB
  ttl_sec: 300           # 5 minutes

upstreams:
  official:
    enabled: true        # Mojang fallback
  hrpauth:
    url: "http://127.0.0.1:2880"
    manage_token: "<your HRPAuth manage.token>"
    enabled: true
```

### License

See [LICENSE](LICENSE).
