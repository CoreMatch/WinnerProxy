# WinnerProxy

本项目是 HA-Contract 的代理服务组件。

## 配置

配置文件为工作目录下的 `config.yml`（首次启动自动生成），默认监听
`:2777`。完整配置参考见
[SamuelWiki — WinnerProxy 配置说明](https://wiki.mcnb.dev/zh/projects/winnerproxy/configuration)
（[English](https://wiki.mcnb.dev/projects/winnerproxy/configuration)）。

## 部署事实

部署事实以仓库根目录的 [`deploy-manifest.yaml`](deploy-manifest.yaml) 为准
（供 HA-Services-Deployer 编排器读取；`manager: aapanel` 表示进程由面板管理，
编排器只读不碰）：

- 监听 `:2777`（对外）；健康端点 `GET /health`。
- 依赖 HRPAuth（启动期凭据换取）与 HRPAuth-Yggdrasil-API（上游转发）。
  无数据库、无 Redis（内存缓存）。
- 配置文件 `config.yml`（模板版本 2）。

## 文档

所有接口文档和开发规范已统一收口至根目录：
- [API 标准与规范](https://github.com/CoreMatch/HA-Contract/blob/main/docs/api/README.md)
- [错误码表](https://github.com/CoreMatch/HA-Contract/blob/main/docs/api/error-codes.md)

旧文档已归档至 [archive/docs/WinnerProxy](https://github.com/CoreMatch/HA-Contract/tree/main/archive/docs/WinnerProxy)。
