<div align="center">
  <h1>WinnerProxy</h1>
  <p><strong>HA 与 Mojang 玩家混用，同一条加入路径。</strong></p>
  <p><a href="./README.md">English</a></p>
</div>

当 HA 与 Mojang 账号共用一台服务器时，WinnerProxy 负责路由 Minecraft 服务器加入校验。

它保留 Mojang UUID 与皮肤，同时避免身份碰撞危及 HA 档案。

## 为什么选择 WinnerProxy

- **三阶段路由。** 在接受加入前，执行完整的 hasJoined 路由序列。
- **只在预期位置回退。** 仅对 hasJoined 请求使用 Mojang 认证回退。
- **保留官方身份。** 让 Mojang 玩家的 UUID 与皮肤始终保持绑定。
- **拒绝身份碰撞。** 遇到不明确的档案匹配时直接拒绝，避免绑定错误身份。
- **缓存判断结果。** 在内存中复用加入结果，减少重复上游请求。

## 进一步了解

- [WinnerProxy 部署](https://wiki.mcnb.dev/deploy/winnerproxy) — 服务指南
- [HA Wiki](https://wiki.mcnb.dev/) — 生态文档
- [HA-Contract](https://github.com/CoreMatch/HA-Contract) — 生态 API 契约

## 许可证与来源

[AGPL-3.0](LICENSE)。来源：[CoreMatch/WinnerProxy](https://github.com/CoreMatch/WinnerProxy)。
