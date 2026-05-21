## Context

couswee 当前是 Go 后端加 SvelteKit 静态前端：`npm run build` 输出 `web/dist`，Go 入口为 `cmd/couswee/main.go`，本地服务默认读取 `COUSWEE_STATIC_DIR=web/dist`。项目已有 `package.json` 的 `0.1.0`，但 Go 二进制没有运行时版本包、`--version` 输出或 API 版本信息，也没有 `.github/workflows`。

参考项目 `paraspeech` 的做法很轻量：`internal/version/version.go` 内置 `Version/Commit/BuildTime` 默认值，`Makefile` 用 `-ldflags -X` 注入 tag、commit 和构建时间，GitHub Actions 在 tag push 时运行 `make build VERSION=<tag>`，再打包 tar.gz 和 sha256 并创建 Release。couswee 可以复用这个思路，但构建流程需要先生成前端静态文件，再用 Go `embed` 将 `web/dist` 放入 Go 二进制。

## Goals / Non-Goals

**Goals:**

- 为 couswee 增加 Go 运行时版本信息，发布构建能注入 tag、commit 和构建时间。
- 提供一个清晰的版本查询面：命令行 `--version` 和本地 API `/api/version` 至少实现一种，优先两者都做以覆盖终端和 UI/诊断场景。
- 增加本地构建入口，串联 `npm ci` / `npm run build` 与 Go build，产出单个 Linux amd64 二进制。
- 增加 GitHub Actions tag release workflow，自动上传 `couswee-<version>-linux-amd64.tar.gz` 与 `.sha256`。
- 更新 README，说明如何查看版本、如何打 tag 发布、产物包含什么。

**Non-Goals:**

- 不迁移 GitLab 仓库到 GitHub，也不处理 GitHub/GitLab 镜像同步。
- 不提交 `web/dist`、`.svelte-kit`、`node_modules` 或发布产物到仓库。
- 不在本 change 中做 Windows/macOS、多架构矩阵或安装包格式。
- 不改变现有账号切换、剩余流量采集和 UI 语义。

## Decisions

### Decision: 使用 Go 版本包加 `-ldflags` 注入

采用类似 `paraspeech/internal/version` 的模式，在 couswee 中新增 `internal/version`，默认值为 `dev`、`none`、`unknown`。发布构建通过 `-X 'couswee/internal/version.Version=$(VERSION)'` 等参数注入。

拒绝方案：只依赖 `package.json` 的 `version`。原因是最终发布产物是 Go 二进制，运行时无法自然读取 npm metadata，且 tag/commit/build time 不是 npm 版本字段能完整表达的。

### Decision: 本地构建入口优先使用 `Makefile`

新增或扩展 `Makefile`，保持与 `paraspeech` 类似的 `make build VERSION=vX.Y.Z` 使用方式，并针对 couswee 增加前端构建步骤。建议目标包括 `frontend`、`build`、`test`、`clean`、`version`。

拒绝方案：只把构建命令写进 GitHub Actions。原因是本地和 CI 构建入口会分叉，后续难以确认发布产物与本机验证是否一致。

### Decision: GitHub release workflow 触发条件为 tag push

新增 `.github/workflows/release.yml`，在 `push.tags: ["v*"]` 或 `["*"]` 触发。为了避免非版本 tag 误触发，建议使用 `v*`，并从 `${{ github.ref_name }}` 作为 `VERSION` 传入 Makefile。

拒绝方案：main 分支每次 push 都发布 Release。原因是 couswee 目前更适合手动决定发版点，tag 是清晰的发布边界。

### Decision: 发布包包含二进制和最小说明

CI 构建 Linux amd64 产物，打包为 `dist/couswee-<version>-linux-amd64.tar.gz`，内部至少包含 `couswee` 二进制。若 README 中需要离线说明，可以额外放入 `README.md` 或 `LICENSE`，但不要把开发缓存和前端中间产物单独发布。

拒绝方案：发布 `web/dist` 目录让用户自行运行 Go 源码。原因是目标是可追踪的运行时二进制；前端静态内容应在构建时嵌入二进制，避免单文件运行时退回占位页。

### Decision: 发布构建使用 Go `embed` 携带静态前端

couswee 开发态仍可从 `web/dist` 读取静态文件，并允许通过 `COUSWEE_STATIC_DIR` 覆盖静态文件目录。发布构建使用 Go `embed` 将 `web/dist` 嵌入二进制；当外部目录不存在时，服务端回退到嵌入式前端。`go:embed` 使用 `all:dist`，确保 SvelteKit 的 `_app` 资源目录被包含。

拒绝方案：发布包依赖用户手动运行 `npm run build`。原因是 Release 产物应开箱可运行，不能把前端构建责任留给最终用户。

## Risks / Trade-offs

- [Risk] GitHub Actions 需要仓库实际托管或镜像到 GitHub 才会运行。→ Mitigation：OpenSpec 和代码只加入标准 workflow 文件；README 明确 tag 发布依赖 GitHub Actions 环境。
- [Risk] `embed` 会改变静态文件服务路径和构建流程。→ Mitigation：施工时先用测试覆盖静态文件 fallback 或服务启动路径，再切换默认静态来源。
- [Risk] tag 与 `package.json` 版本可能不一致。→ Mitigation：发布版本以 Git tag 为准，`package.json` 版本仅作为前端包 metadata；README 说明发版流程。
- [Risk] 构建时间使用本地时区会影响可复现性。→ Mitigation：沿用 `paraspeech` 的 Asia/Shanghai 人类可读时间，或在施工时选择 UTC；无论选择哪种，都固定在 Makefile 中。

## Migration Plan

1. 新增版本包和版本查询入口，默认开发态输出不影响现有启动。
2. 新增本地构建入口并保持 `npm run build` 与 `go test ./...` 可用。
3. 新增 GitHub Actions release workflow 和 README 发布说明。
4. 施工完成后用本地 `make build VERSION=v0.1.0-test` 验证注入结果，再运行 `npm test -- --run` 或等价构建测试。
5. 如 workflow 后续在 GitHub 上失败，可先回滚 `.github/workflows/release.yml`，本地版本功能仍可保留。

## Open Questions

- 发布二进制是否必须是完全单文件？若是，施工时应优先实现 Go `embed` 静态前端。
- tag 规则采用 `v*` 还是任意 tag？本设计推荐 `v*`。
- 首个正式版本是否沿用 `package.json` 的 `0.1.0` 对应 tag `v0.1.0`？
