## Context

couswee 已经完成 SQLite 账号管理、Codex 登录、账号 auth 文件托管、账号切换、用量字段持久化和账号列表集成展示。当前 usage collector 仍保留过渡期来源：API collector、command fallback、session log fallback 和 account fallback。

新的系统化目标来自 `docs/couswee_codex_usage_systematized.md`：每个账号的 auth 文件由 couswee 管理，因此多账号用量查询应直接读取每个账号自己的 `auth_path`，使用其中的 `tokens.access_token` 调用 usage/rate-limit endpoint。SQLite 是唯一持久状态，`~/.cache` 和 abtop cache 不再适合作为 couswee 运行依赖。

关键约束：

- 用量语义必须是 `剩余流量`，不是已消耗百分比。
- session log 只能代表当前 live auth，不能用于非 active 账号。
- token 不得写入日志、API 响应或前端状态。
- 本 change 已在用户明确指令后进入代码施工。

## Goals / Non-Goals

**Goals:**

- 让所有有有效 `auth_path` 和 access token 的账号都可独立查询 Codex 用量。
- 将 Account auth API collector 作为主路径，按账号读取 token 并查询 usage/rate-limit endpoint。
- 把成功查询结果和 refresh metadata 持久化到 SQLite。
- 移除 abtop cache collector 和 `~/.cache` 默认持久缓存路径。
- 将 session log collector 限定为 active-only 辅助来源，或降级为诊断用途。
- 拆分单账号刷新与全量刷新，使账号切换可以优先刷新新 active 账号。
- 保持 `/api/accounts`、`/api/codex/usage` 和账号列表 remaining 显示兼容。

**Non-Goals:**

- 不在本 change 中引入公开远程服务。
- 不新增项目外持久 cache 文件。
- 不把 session log 作为非 active 账号用量来源。
- 不强制第一阶段实现 SSE/WebSocket；事件流是体验增强，不是系统化查询的前置条件。
- 不改变本地单用户部署模型或引入远程公共服务。

## Decisions

### Decision: 以账号 auth 文件作为主查询凭证

每次查询以 SQLite account 的 `auth_path` 为入口，读取 `tokens.access_token` 和 `tokens.account_id`，再调用 usage/rate-limit endpoint。

Rationale: couswee 已经托管账号 auth 文件，也由 couswee 完成登录；这比从全局 session log 或外部 cache 推断账号更准确。

Alternative considered: 继续只读取 `~/.codex/auth.json`。Rejected: 只能覆盖 active 账号，无法可靠查询非 active 账号。

### Decision: 内置 Codex ChatGPT usage endpoint 默认值

默认 endpoint 使用当前 Codex CLI 二进制暴露并经 live probe 验证可用的 ChatGPT usage 地址：`https://chatgpt.com/backend-api/wham/usage`。用户仍可通过 `COUSWEE_USAGE_API_URL` 覆盖；如需要完全禁止外部 usage 查询，可设置 `COUSWEE_USAGE_API_ENABLED=false`。

Rationale: 当前实现虽然支持 `COUSWEE_USAGE_API_URL`，但默认值为空会让最关键的 account-auth API 主路径永远不启用，实际运行退回 stale SQLite/account fallback。把已验证 endpoint 默认化，可以让“每个账号用自己的 auth token 查询”成为默认行为，而不是部署后隐性失效。

Alternative considered: 继续要求用户手动配置 endpoint。Rejected: 这个地址是系统化用量查询的必要主路径；空配置会重复触发同一类错误。

### Decision: 解析 ChatGPT wham usage 响应

`/backend-api/wham/usage` 返回的 `rate_limit.primary_window.used_percent/reset_at` 表示 5 小时窗口，`rate_limit.secondary_window.used_percent/reset_at` 表示 weekly 窗口。collector 必须将 used percent 转成剩余百分比，并接受 0% used 作为 100% remaining，而不是误判为无数据。

Rationale: live probe 显示该 endpoint 对两个本地托管账号 auth token 均返回 HTTP 200，响应格式与现有 abtop/session fixture 不完全相同，需要一等解析支持。

Alternative considered: 让用户通过 command fallback 转换 wham 响应。Rejected: 增加外部脚本依赖，违背本 change 的系统化目标。

### Decision: wham 顶层 account_id 不作为 auth account_id 严格相等依据

live probe 显示 `/backend-api/wham/usage` 响应中的顶层 `account_id` 与本地 auth JSON 的 `tokens.account_id` 不是同一个 ID 命名空间。对 wham 响应，账号归属以发起请求所用的 per-account Bearer token 为准；仅对明确同命名空间的响应身份字段继续做严格冲突拒绝。

Rationale: 如果把 wham 顶层 `account_id` 与 auth `tokens.account_id` 直接比较，会让两个已验证 token 都被误判为 mismatch，导致实际运行退回 stale SQLite/account fallback。

Alternative considered: 删除所有响应身份校验。Rejected: 非 wham/兼容响应仍可能提供可比较的账号身份，保留严格校验能防止其他 endpoint 把错误账号数据写入 SQLite。

### Decision: SQLite 是唯一持久 usage 状态

成功查询的 remaining 百分比、reset time、source、last refresh、stale/error 等状态应进入 SQLite；进程内 cache 只作为运行时快照。

Rationale: 用户希望项目数据尽量由 SQLite 管理，并且不希望 couswee 增加项目外运行残留。

Alternative considered: 使用 `~/.cache/couswee/codex-rate-limits.json`。Rejected: 与项目数据管理方向冲突，也会增加额外清理成本。

### Decision: 删除 abtop cache 依赖

移除 abtop cache collector 和默认 `~/.cache/abtop/codex-rate-limits.json` 路径，不再把 abtop 作为目标系统 fallback。

Rationale: abtop cache 是外部工具副产物，不属于 couswee 的受控数据；当前项目已经有账号 auth 文件和 SQLite，应该独立运行。

Alternative considered: 保留 abtop 作为最后兜底。Rejected: 它只代表外部工具最近状态，且缺少项目内生命周期管理。

### Decision: session log 只允许显式启用的 active-only 诊断/临时兜底

session log 的 `payload.rate_limits` 只可在显式配置 `COUSWEE_USAGE_SESSION_GLOB` 后，在 live auth 与 active account 匹配且事件不早于账号 `last_used_at` 时用于 active 账号辅助刷新；禁止写给非 active 账号。

Rationale: session log 记录的是某个 Codex CLI 会话上下文，不携带足够的多账号归属信息；正在运行的 Codex 进程也不一定会因为 live auth 文件切换而立即切换账号。

Alternative considered: 通过最近切换时间推断 session log 属于哪个账号。Rejected: 时间推断不可靠，容易污染非 active 账号用量。

### Decision: 拆分 RefreshAccount 与 RefreshAll

Usage service 需要单账号刷新和全量刷新两个边界。切换账号时先刷新新的 active 账号，全量刷新由启动、定时任务或手动触发负责。

Rationale: 切换体验需要尽快显示新 active 账号的用量，而不应被其他账号网络错误或超时拖慢。

Alternative considered: 切换时继续完整 `Refresh` 全部账号。Rejected: 账号数量增加后延迟不可控，且不符合 active 优先体验。

### Decision: 继续保留 remaining 兼容字段

API 继续返回 `5h_usage` / `weekly_usage` 兼容字段，但这些字段必须与 `5h_remaining` / `weekly_remaining` 保持 remaining 语义，并通过 `usage_basis: remaining` 明确标记。

Rationale: 保持前端和旧调用方兼容，同时避免语义误读。

Alternative considered: 立即删除 legacy usage 字段。Rejected: 不必要地扩大破坏面。

## Risks / Trade-offs

- [Risk] usage/rate-limit endpoint 的稳定地址或响应格式仍可能变化。 -> Mitigation: 默认 endpoint 可通过环境变量覆盖，collector 用 wham fixture 覆盖解析器，并保留 `COUSWEE_USAGE_API_ENABLED=false` 禁用路径。
- [Risk] 部分账号 auth token 过期或 auth 文件缺失。 -> Mitigation: 保留 SQLite 旧值，标记 stale/error，并在 UI 中明确提示。
- [Risk] 多账号并发查询可能触发 endpoint 限制。 -> Mitigation: 使用有限 worker pool、单账号 timeout 和定时刷新间隔限制。
- [Risk] 写入 refresh metadata 可能需要 SQLite schema 迁移。 -> Mitigation: 先评估现有字段，不足时添加向后兼容迁移。
- [Risk] active live auth 与 SQLite active 状态不一致。 -> Mitigation: 查询前通过 account_id 或 auth 文件内容同步 active 归属。

## Migration Plan

1. 更新 usage collection specs，确立 account auth API 主路径。
2. 清理 abtop/cache 相关配置和 collector。
3. 将 SQLite 持久 usage 字段补齐到 refresh metadata 需求。
4. 实现 `RefreshAccount` 并让 `RefreshAll` 复用它。
5. 切换账号后只等待新 active 账号刷新，其他账号后台刷新。
6. 保留 `/api/codex/usage` cache 快照响应，必要时新增手动刷新 API。
7. 前端显示 stale/error/刷新中状态，仍在账号列表内展示 remaining。

Rollback 策略：保留 SQLite 旧 usage 字段和 account fallback；若 endpoint 查询不可用，可禁用 Account auth API collector，仅显示 SQLite 上次成功结果。

## Open Questions

- SQLite 是否需要新增 `last_refresh`、`usage_source`、`usage_stale`、`usage_error` 等字段，还是先只在 usage cache 中携带。
- command collector 是否继续作为长期 fallback，还是仅作为调试/开发入口保留。
