<div align="center">
  <h1>WinnerProxy</h1>
  <p><strong>Mixed HA and Mojang players, one join path.</strong></p>
  <p><a href="./README.zh-CN.md">简体中文</a></p>
</div>

WinnerProxy routes Minecraft server join checks for deployments where HA and Mojang accounts share a server.

It preserves Mojang UUIDs and skins while protecting HA profiles from identity collisions.

## Why WinnerProxy

- **Route in three stages.** Apply the hasJoined routing sequence before accepting a join.
- **Fall back only where intended.** Use Mojang authentication only for hasJoined requests.
- **Preserve official identities.** Keep Mojang UUIDs and skins attached to those players.
- **Reject collisions.** Refuse ambiguous profile matches rather than bind the wrong identity.
- **Cache decisions.** Reuse join results in memory to reduce repeated upstream work.

## Learn more

- [WinnerProxy deployment](https://wiki.mcnb.dev/deploy/winnerproxy) — service guide
- [HA Wiki](https://wiki.mcnb.dev/) — ecosystem documentation
- [HA-Contract](https://github.com/CoreMatch/HA-Contract) — ecosystem API contracts

## License & provenance

[AGPL-3.0](LICENSE). Source: [CoreMatch/WinnerProxy](https://github.com/CoreMatch/WinnerProxy).
