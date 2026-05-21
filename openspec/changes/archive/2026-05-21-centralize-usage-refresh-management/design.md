## Context

graphify 更新后的代码图显示，用量相关节点集中在 `internal/usage` 的 collection/cache/service 社区、`internal/server` 的 API 触发社区、`web/src/routes/+page.svelte` 的 dashboard 轮询社区，以及 OpenSpec 中的 usage cache/API/dashboard 规格社区。当前刷新动作跨越这些社区：

- `cmd/couswee/main.go` 在启动时直接 `RefreshAll` 并启动周期刷新。
- `internal/server/server.go` 在账号新增、账号切换、登录成功查询中直接调用单账号刷新，并各自创建超时上下文。
- `web/src/routes/+page.svelte` 分别轮询账号列表和用量列表，并在切换、登录、增删改后重复调用 `refreshAll`。
- `internal/usage.Service` 同时承担采集、缓存更新、持久化、定时调度和错误兜底，缺少明确的刷新动作边界。

这使得后续修改容易变成“在哪个入口补一段刷新”的补丁式演进。统一管理后，collection 仍负责采集，cache 仍负责快照，SQLite 仍是持久用量状态，但所有写入型刷新动作都通过一个刷新管理层执行。

## Goals / Non-Goals

**Goals:**

- 给所有后端用量刷新动作建立统一入口，支持全账号刷新和单账号刷新。
- 为刷新来源建立可读的 reason/action 概念，方便测试和后续排查。
- 让启动刷新、周期刷新、账号新增、账号切换、登录成功刷新复用同一套超时、缓存更新、SQLite 回写和错误保留逻辑。
- 保持 `GET /api/codex/usage` 为缓存读取，不隐式触发 live collection。
- 简化前端视图刷新，把账号和用量读取的组合逻辑集中到少数函数，避免多个入口重复表达同一件事。

**Non-Goals:**

- 不改变 Codex usage endpoint、auth 读取或 rate-limit payload 解析语义。
- 不新增外部依赖、队列、后台进程或额外服务。
- 不把用量刷新暴露成必须存在的新公开 API；现有 API 合同保持兼容。
- 不改变“剩余流量”字段语义，不恢复 abtop cache 作为持久状态。

## Decisions

### Decision: 新增 usage refresh manager 作为动作入口

在 `internal/usage` 中引入轻量的刷新管理模块，包装现有 `Service` 的采集/缓存/持久化能力，并提供 `RefreshAll(ctx, reason)`、`RefreshAccount(ctx, selector, reason)`、`Start(ctx)` 等动作入口。

Rationale: 现有 `Service` 已经掌握 collector、cache、account source、account sink 和 interval，适合保留为核心实现；新增管理边界比把逻辑继续散在 server/main 更可测试，也比大规模重写 collector 风险低。

Rejected: 在 `internal/server` 中维护刷新状态机。原因是 server 应只表达 HTTP/use-case 触发，不应持有用量刷新策略和周期调度。

Rejected: 引入任务队列或 worker pool。原因是当前刷新频率低、账号数量小，队列会增加状态和并发复杂度。

### Decision: collection 与 refresh orchestration 分离

collector 只负责“给定账号，产出一个 `UsageRecord` 或错误”；manager/service 负责“何时刷新、刷新哪些账号、如何更新 cache/SQLite、失败时如何保留 stale 值”。

Rationale: graphify 显示 collection 与 cache/API/dashboard 已经是不同社区；把触发策略从 collector 中剥离可以减少跨层耦合。

Rejected: 让 collector 读取全局账号列表并决定刷新范围。原因是这会让单账号刷新、活跃账号刷新和测试替身更难控制。

### Decision: API 读取不触发写入刷新

`GET /api/codex/usage` 继续返回缓存快照。需要刷新时由启动、周期、账号新增、切换、登录成功等明确动作触发，而不是 GET 读取时顺手采集。

Rationale: 读取接口保持快速、稳定、可缓存，避免用户打开页面时阻塞在网络 usage endpoint 上。

Rejected: 每次 GET 都尝试 live collection。原因是这会让页面轮询变成高频写入动作，也容易放大 endpoint 失败和超时。

### Decision: 前端只统一视图刷新，不承担后端刷新决策

前端保留账号与用量数据读取，但将 `loadAccounts`/`loadUsage` 组合为更清晰的 view refresh helper。账号切换、登录完成、增删改后只刷新视图，后端在对应业务动作内负责写入型用量刷新。

Rationale: 前端没有 auth token 和 collector 细节，不能成为刷新策略来源；它只需要展示后端缓存和账号状态。

Rejected: 在前端增加手动刷新写入接口调用。原因是本次目标是统一管理现有刷新动作，不引入新的用户操作面。

## Risks / Trade-offs

- [Risk] 新增 manager 名称可能与现有 `Service` 职责重叠 → Mitigation: 保持 manager 轻量，把现有采集/缓存逻辑最小迁移，并用测试锁住行为。
- [Risk] 单账号刷新后 account list 返回值可能仍是刷新前对象 → Mitigation: server 在业务动作后通过统一 helper 重新读取账号快照。
- [Risk] 周期刷新与手动业务刷新并发写 cache/SQLite → Mitigation: manager/service 内部保持现有 cache 原子更新，并用互斥锁串行化刷新执行。
- [Risk] 过度改动影响当前 dirty 工作树中的发布/版本变更 → Mitigation: 只改用量刷新相关文件和新 change 文档，不触碰发布 change 的职责。

