## Why

当前 couswee 账号管理仍以本地 JSON/auth 文件路径为核心，无法支撑完整的 Codex 用户账号登录、可编辑账号资料、登录状态追踪和后续多账号管理。根据 `design-logic.md`，需要把“添加账号”从手工填写 auth 文件路径升级为前后端协同的登录流程，并用 SQLite 持久化账号/profile 元数据，供界面显示、编辑、删除和切换使用。

## What Changes

- 新增 Codex 用户账号登录机制，前端提供添加账号入口，后端负责登录会话、profile 生成、auth 写入和状态持久化。
- 登录方式合并为一条 Codex 设备码授权路径；该路径生成本地 profile 与 auth 文件。
- 后端新增 SQLite 账号存储，使用 Go 纯 Go 驱动 `modernc.org/sqlite`，不使用 CGO SQLite 驱动。
- SQLite 存储账号/profile 元数据、显示名称、登录方式、auth/profile 路径、active 状态、登录状态、最后使用时间、创建/更新时间；token 仍只落在受权限保护的 auth 文件中，不直接暴露给前端。
- 账号列表、添加、编辑、删除、切换统一使用 SQLite backed repository；旧文件不再作为读写源，也不再自动导入。
- 前端新增添加账号流程 UI：提供单一 Codex 登录入口、显示登录状态、设备码/验证 URL、登录成功后刷新账号列表。
- 后端新增 REST API 支撑账号登录、登录状态轮询、账号 CRUD、切换 active profile、旧 registry 迁移与错误反馈。
- 本 change 只规划，不实施代码改动。

## Capabilities

### New Capabilities
- `codex-account-login`: 定义 Codex 账号登录流程、profile/auth 文件写入、登录状态追踪和安全边界。

### Modified Capabilities
- `account-registry`: 使用 SQLite 作为账号/profile 元数据的唯一读写源。
- `account-switching`: 从复制任意 auth_path 切换升级为基于 SQLite active profile 与受管 auth 文件的切换。
- `local-api`: 增加账号登录、账号 CRUD、登录状态轮询、profile 切换等本地 API。
- `account-dashboard-ui`: 增加登录式添加账号、编辑/删除账号和登录状态展示的前端交互要求。

## Impact

- 后端 Go 模块：新增 SQLite repository、迁移器、登录服务、profile/auth 文件服务、API handler。
- 依赖：新增 `modernc.org/sqlite` 作为 SQLite database/sql 驱动。
- 数据路径：新增 `~/.couswee/couswee.db` 或等价可配置 DB 路径；继续使用受管 profile/auth 文件目录。
- 前端 SvelteKit：新增添加账号登录流程、设备码状态、账号编辑表单、账号删除确认和状态刷新。
- 安全：token 不进前端，不写入普通日志；auth 文件权限 0600，目录权限 0700。
- 测试：repository migration、API handler、登录状态机、前端构建和关键 UI 状态。
