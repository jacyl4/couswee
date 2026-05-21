## 1. 数据源边界清理

- [x] 1.1 移除 abtop cache collector 及其单元测试，确保目标系统不再读取 `~/.cache/abtop/codex-rate-limits.json`
- [x] 1.2 移除 couswee 默认 `COUSWEE_USAGE_CACHE_PATH` / `~/.cache` usage cache 路径逻辑
- [x] 1.3 将 session log collector 限定为 active-only，或降级为诊断用途并禁止写入非 active 账号
- [x] 1.4 更新配置和文档说明，明确 SQLite 是唯一持久 usage 兜底来源

## 2. 账号 Auth API Collector

- [x] 2.1 调整 API collector，使每个账号默认读取自己的 SQLite `auth_path`
- [x] 2.2 保留 active 账号使用 live `~/.codex/auth.json` 的路径，但必须校验其归属到当前 active 账号
- [x] 2.3 读取 `tokens.access_token` / `tokens.account_id` 时禁止将 token 写入日志、错误文本或 API 响应
- [x] 2.4 调用 usage/rate-limit endpoint 并按账号归属校验响应
- [x] 2.5 为 token 缺失、auth 文件不可读、响应账号不匹配、endpoint 失败添加测试

## 3. Usage 持久化与模型

- [x] 3.1 评估 SQLite account 表现有 usage 字段是否覆盖 `last_refresh`、`source`、`stale`、`error`
- [x] 3.2 如字段不足，添加向后兼容 SQLite migration
- [x] 3.3 成功查询时写入 5h/weekly remaining、reset time 和 refresh metadata
- [x] 3.4 查询失败时保留上一次成功值，并只更新 stale/error 状态
- [x] 3.5 确保 account fallback 不会被当作新的成功 live collection 回写

## 4. 刷新服务拆分

- [x] 4.1 增加 `RefreshAccount(ctx, accountID)` 或等价单账号刷新入口
- [x] 4.2 调整全量刷新逻辑，使 `RefreshAll(ctx)` 复用单账号刷新并隔离单账号失败
- [x] 4.3 为定时刷新增加有限并发或保持可控串行策略，并保留单账号 timeout
- [x] 4.4 切换账号后优先刷新新的 active 账号，其他账号后台或下次定时刷新
- [x] 4.5 添加切换时 active 账号刷新不被其他账号阻塞的测试

## 5. API 与前端体验

- [x] 5.1 保持 `GET /api/codex/usage` 返回进程内快照，不在请求内触发全量 live collection
- [x] 5.2 必要时新增手动全量刷新 API，并确保不破坏现有 usage endpoint
- [x] 5.3 必要时新增手动单账号刷新 API，用于前端或调试触发
- [x] 5.4 前端账号列表展示 stale/error/刷新中状态，并保留旧 remaining 值
- [x] 5.5 切换成功后刷新账号和 usage 数据，使新 active 账号尽快展示最新 remaining

## 6. 验证与收尾

- [x] 6.1 添加或更新 collector、service、SQLite migration、server handler 测试
- [x] 6.2 运行 `gofmt` 和 `go test ./...`
- [x] 6.3 运行前端 build，确认账号列表状态展示无类型或构建错误
- [x] 6.4 运行 `openspec validate systematize-codex-usage-collection --strict`
- [x] 6.5 更新 `docs/codex_usage_collection.md`，使当前实现说明与系统化方案保持一致

## 7. 默认 endpoint 与 live 格式修缮

- [x] 7.1 用本机托管账号 auth token 对 Codex ChatGPT usage endpoint 做只读 live probe，确认 endpoint 和响应结构
- [x] 7.2 将已验证 endpoint 作为默认 `COUSWEE_USAGE_API_URL`，保留环境变量覆盖和 `COUSWEE_USAGE_API_ENABLED=false` 禁用语义
- [x] 7.3 支持解析 `/backend-api/wham/usage` 的 `rate_limit.primary_window` / `secondary_window` 响应格式
- [x] 7.4 补充默认配置、wham 解析、零用量百分比和多账号 token 请求测试
- [x] 7.5 更新 README、usage 实现说明和 OpenSpec specs，使配置与实现一致
- [x] 7.6 运行 `gofmt`、Go 测试、前端 build、OpenSpec strict validate，并做 live API smoke 验证
