## 1. OpenSpec 与现状锁定

- [x] 1.1 用 graphify 更新代码图并记录用量刷新入口：启动、周期、账号新增、账号切换、登录成功、前端轮询
- [x] 1.2 校验 `centralize-usage-refresh-management` change 的 proposal/design/specs/tasks 均可被 OpenSpec 识别

## 2. 后端刷新管理模块

- [x] 2.1 在 `internal/usage` 中引入刷新 reason/action 与统一 refresh manager 边界
- [x] 2.2 将全账号刷新、单账号刷新、超时、cache 更新、SQLite 回写和失败 stale 保留统一到 manager/service 路径
- [x] 2.3 串行化刷新执行，避免周期刷新与业务触发刷新同时写 cache/SQLite
- [x] 2.4 保持 collector 只负责给定账号采集，不承担刷新触发来源判断

## 3. 启动与 API 入口收敛

- [x] 3.1 调整 `cmd/couswee/main.go`，启动刷新与周期刷新通过统一刷新管理入口执行
- [x] 3.2 调整 `internal/server/server.go`，账号新增、账号切换、登录成功刷新均通过统一入口执行并删除重复超时上下文逻辑
- [x] 3.3 确认 `GET /api/codex/usage` 只返回缓存快照，不隐式触发 live collection

## 4. 前端视图刷新整理

- [x] 4.1 整理 `web/src/routes/+page.svelte` 的账号/用量加载函数，减少重复的 `refreshAll` 调用语义
- [x] 4.2 确保前端轮询只做缓存读取和视图合并，不引入新的后端写入刷新动作

## 5. 验证

- [x] 5.1 增加或调整 Go 单元测试，覆盖统一 manager 的全量刷新、单账号刷新、失败 stale 保留和串行化行为
- [x] 5.2 运行 `openspec validate centralize-usage-refresh-management --strict`
- [x] 5.3 运行后端测试与前端构建/测试，确认现有发布改动和用量改动没有互相破坏
