## Why

couswee 已经托管每个 Codex 账号的 auth 文件，并通过 SQLite 管理账号状态，因此用量查询应从“active 账号的全局日志/外部缓存兜底”升级为“按账号 auth token 查询并持久回写 SQLite”的系统化能力。

现有用量逻辑仍包含 abtop cache、`~/.cache` 默认路径和 session log fallback 等历史过渡方案；这些来源要么缺少非 active 账号归属，要么会产生项目外运行残留，不符合当前多账号管理模型。

## What Changes

- 将多账号用量查询主路径改为读取每个账号自己的 `auth_path`，提取 `tokens.access_token`，调用 usage/rate-limit endpoint。
- 内置当前 Codex CLI 使用的 ChatGPT usage endpoint 默认值，避免部署时空配置导致 API collector 永远不可用；仍允许通过 `COUSWEE_USAGE_API_URL` 覆盖，或用 `COUSWEE_USAGE_API_ENABLED=false` 禁用。
- 保持 `剩余流量` 语义：当上游返回 used percent 时，统一转换为 `100 - used_percent`，并保留 `usage_basis: remaining`。
- 将 SQLite 确认为唯一持久 usage 状态，成功查询的百分比、reset time、refresh metadata 和错误状态都应回写 SQLite。
- 清理 abtop cache collector、`~/.cache/abtop/codex-rate-limits.json` 默认值、`COUSWEE_USAGE_CACHE_PATH` 默认路径，以及任何新的 `~/.cache/couswee/*` 持久缓存规划。
- 将 session log collector 限定为 active-only 辅助来源；禁止把 `payload.rate_limits` 套用到非 active 账号。
- 拆分刷新能力：增加单账号刷新与全量刷新边界，使切换账号时优先刷新新的 active 账号，其他账号后台或定时刷新。
- 保持现有本地 API 和前端账户列表语义兼容，必要时新增手动刷新 API；SSE/WebSocket 只作为后续可选增强。
- 按 OpenSpec 规划完成后，在用户明确指令下进入代码施工。

## Capabilities

### New Capabilities
- `codex-account-auth-usage`: 定义基于每个账号托管 auth 文件的 Codex 用量查询能力，覆盖 access token 读取、endpoint 调用、账号归属校验和 SQLite 持久回写。

### Modified Capabilities
- `codex-usage-collection`: 将收集来源从 API/command/session/abtop 混合 fallback 调整为账号 auth API 优先，session log 仅 active-only，abtop/cache 文件逻辑删除。
- `codex-usage-cache`: 将持久缓存边界收紧到 SQLite，进程内 cache 只作为运行时快照，不落地到 `~/.cache`。
- `codex-usage-api`: 允许后续新增手动刷新 API，并保持 usage 响应的 remaining 字段和兼容字段语义稳定。
- `local-api`: 调整切换账号后的 usage 刷新要求，从“完整刷新后返回”优化为“切换成功后优先刷新新的 active 账号”。
- `codex-usage-dashboard`: 增强前端对 stale/error/刷新中状态的展示要求，以配合 per-account refresh 与 SQLite 兜底。

## Impact

- Backend usage collector 需要改为按账号 auth 文件查询，并对 token/account_id 归属做校验。
- Usage service 需要拆分 `RefreshAccount` 与 `RefreshAll`，并支持切换时 active 优先刷新。
- SQLite account schema 或现有字段使用需要覆盖 refresh metadata、stale/error 状态；若字段不足，需要迁移。
- abtop cache 相关配置、默认路径、collector 和文档需要移除。
- session log fallback 需要保留 active-only 保护或降级为诊断用途。
- 前端不应新增项目外持久缓存依赖；账号列表继续展示 remaining percentage、reset time、stale/error 状态。
- 测试需要覆盖多账号 auth 查询、非 active 账号不使用 session log、abtop/cache 清理、SQLite 持久兜底和切换后的单账号刷新。
- 需要覆盖 ChatGPT `/backend-api/wham/usage` 响应格式：`rate_limit.primary_window` 映射为 5h，`rate_limit.secondary_window` 映射为 weekly，并保持 `100 - used_percent` 的剩余流量语义。
