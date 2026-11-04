# WinnerProxy

本项目是 HA-Contract 的代理服务组件。

## 配置

- 配置文件：工作目录下的 `config.yml`，首次启动自动生成；当 stdin 是终端
  时会交互式提示输入 OAuth2 client secret。
- 默认监听地址 `:2777`。
- 主要配置段：
  - `server`：`addr`（监听地址）、`public_url`（对外可达地址）
  - `upstreams.hrpauth`：`url`（HA 上游地址，默认含 `/yggdrasil-api` relay
    前缀）、`client_id`、`client_secret`（OAuth2 凭据）
  - Mojang 上游设置、link-token 校验缓存设置

## 文档

所有接口文档和开发规范已统一收口至根目录：
- [API 标准与规范](https://github.com/CoreMatch/HA-Contract/blob/main/docs/api/README.md)
- [错误码表](https://github.com/CoreMatch/HA-Contract/blob/main/docs/api/error-codes.md)

旧文档已归档至 [archive/docs/WinnerProxy](https://github.com/CoreMatch/HA-Contract/tree/main/archive/docs/WinnerProxy)。
