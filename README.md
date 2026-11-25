<div align="center">
  <h1>Armorial Winner Proxy</h1>
  <p><strong>Mixed Armorial and Mojang players, one join path.</strong></p>
  <p><a href="./README.zh-CN.md">简体中文</a></p>
</div>

Armorial Winner Proxy routes Minecraft server join checks for deployments where Armorial and Mojang accounts share a server.

It preserves Mojang UUIDs and skins while protecting Armorial profiles from identity collisions.

## Why Armorial Winner Proxy

- **Route in three stages.** Apply the hasJoined routing sequence before accepting a join.
- **Fall back only where intended.** Use Mojang authentication only for hasJoined requests.
- **Preserve official identities.** Keep Mojang UUIDs and skins attached to those players.
- **Reject collisions.** Refuse ambiguous profile matches rather than bind the wrong identity.
- **Cache decisions.** Reuse join results in memory to reduce repeated upstream work.

## Learn more

- [Armorial Winner Proxy deployment](https://wiki.mcnb.dev/deploy/winnerproxy) — service guide
- [Armorial Wiki](https://wiki.mcnb.dev/) — ecosystem documentation
- [Armorial API Contracts](https://github.com/CoreMatch/HA-Contract) — ecosystem API contracts

## License & provenance

[AGPL-3.0](LICENSE). Source: [CoreMatch/WinnerProxy](https://github.com/CoreMatch/WinnerProxy).
