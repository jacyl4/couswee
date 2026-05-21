## Why

当前 couswee 的用量刷新动作分散在启动流程、定时任务、账号新增、账号切换、登录轮询和前端数据刷新中，触发语义靠各处临时调用拼接，后续继续修补会让行为更难验证。

本次变更把“刷新用量”提升为统一管理的模块能力：所有写入型用量刷新都经过同一个后端管理入口，前端只读取缓存并按统一节奏刷新视图，从 graphify 审视出的用量 collection/cache/API/dashboard 社区边界中拆出清晰的刷新协调层。

## What Changes

- 新增 `usage-refresh-management` 能力，定义统一的用量刷新管理器、刷新原因、单账号/全账号刷新语义，以及与缓存和持久化的边界。
- 修改启动、定时、账号新增、账号切换、登录成功后的刷新入口，让它们调用同一个刷新管理模块，而不是在 server/main 中各自拼接超时、选择器和回写逻辑。
- 保持 `GET /api/codex/usage` 为缓存读取接口，不把读取接口变成隐式写入刷新动作。
- 统一前端刷新节奏，账号列表和用量数据的视图刷新可以合并调用，但不会在前端制造多个互相竞争的写入刷新触发。
- 保留现有剩余流量语义、活跃账号匹配规则、SQLite 持久化规则和失败保留 stale 数据行为。
- 不新增依赖，不引入额外后台服务，不恢复 abtop cache 作为持久状态。

## Capabilities

### New Capabilities

- `usage-refresh-management`: 统一管理所有后端用量刷新动作，包括启动、定时、账号新增、账号切换、登录成功、全量刷新和单账号刷新。

### Modified Capabilities

- `codex-usage-cache`: 将缓存更新与持久化收束到统一刷新管理器产生的刷新结果中。
- `codex-usage-collection`: 明确 collection 只负责按目标账号采集，不负责决定刷新触发来源。
- `codex-usage-api`: 明确读取接口返回缓存快照，不隐式触发 live collection。
- `codex-usage-dashboard`: 明确前端只做视图级轮询/合并，不直接承担后端写入型刷新决策。

## Impact

- 主要影响 `internal/usage`：新增或重组刷新管理模块，收敛 `RefreshAll`、`RefreshAccount`、定时启动、超时、刷新原因和持久化边界。
- 影响 `cmd/couswee/main.go` 与 `internal/server/server.go`：改为依赖统一刷新管理入口，删除重复的上下文超时和刷新状态管理。
- 影响 `web/src/routes/+page.svelte`：统一账号/用量视图刷新调用，减少重复轮询路径。
- 影响测试：增加用量刷新管理器的单元测试，并调整 server/frontend 相关测试以验证统一刷新触发。
