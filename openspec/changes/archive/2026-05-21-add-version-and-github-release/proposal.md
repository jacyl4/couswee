## Why

couswee 目前缺少可追踪的运行时版本信息，也没有自动化发布产物；用户和后续运维无法直接确认当前二进制来自哪个 tag、commit 或构建时间。参考 `paraspeech` 的发布方式，本次变更要为 couswee 建立轻量、可复现的版本注入与 GitHub Release 自动构建流程。

## What Changes

- 增加 couswee 运行时版本能力：提供默认开发态版本值，并在发布构建时注入 `Version`、`Commit`、`BuildTime`。
- 增加可查询版本入口：CLI 启动参数和/或本地 API 能返回当前版本、commit、构建时间，便于定位用户运行的具体产物。
- 增加项目构建入口：用适合 couswee 的脚本或 Makefile 串联 SvelteKit 静态构建与 Go 二进制构建。
- 增加 GitHub Actions 发布 workflow：tag push 触发构建，打包 Linux amd64 发布包并生成 sha256 校验文件，再创建 GitHub Release。
- 保留 couswee 本地优先定位：不引入运行时外部服务依赖，自动构建只负责构建与发布产物。

## Capabilities

### New Capabilities
- `application-versioning`: 记录并暴露 couswee 当前运行版本、commit 和构建时间。
- `github-release-automation`: 基于 GitHub tag 自动构建、打包、校验并发布 couswee 产物。

### Modified Capabilities

无。

## Impact

- 受影响代码：`cmd/couswee/main.go`、新增版本包、可能新增本地构建脚本或 `Makefile`。
- 受影响前端：发布构建需要先生成 `web/dist`，但不要求在仓库中提交该目录。
- 受影响配置：新增 `.github/workflows/release.yml`，可能新增 `.gitignore` 条目以忽略发布二进制和 `dist/`。
- 受影响文档：`README.md` 需要补充版本查看、发布 tag 和产物说明。
