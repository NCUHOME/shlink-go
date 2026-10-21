# 上游参考资料

本目录内容复制自 [shlinkio/shlink](https://github.com/shlinkio/shlink)（PHP 原版），仅作为 Go 重写的 API 契约与行为参考，不参与构建或运行。

- 上游版本：v5.1.7
- 上游提交：`3f17ecf9334ea3d53beba0539e6f23a42454f5a3`
- 许可证：MIT，见根目录 `LICENSE`

## 内容

### `swagger/`

官方 OpenAPI 3.1 规范，包含 `swagger.json` 以及 `paths/`、`definitions/`、`parameters/`、`examples/`。

注意：该规范覆盖原版全部接口，包含 v1/v2 和多种导入源等本仓库未全部实现的内容。实际实现范围以根目录 `README.md` 为准。

## 未搬入的内容

- PHP 实现（`module/`、`config/`、`bin/`、`public/index.php`）
- PHP 测试与 fixtures（无法直接在 Go 中运行）
- 架构决策记录（ADR）
- Doctrine migrations（schema 按 `README.md` 重新设计）
- PHP / FrankenPHP / RoadRunner 部署配置与 Dockerfile
- 安装器、导入器以及其余 CLI 命令
