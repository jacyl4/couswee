## 1. 依赖与数据层规划

- [x] 1.1 在实现前确认 `modernc.org/sqlite` 版本，并将其作为唯一 SQLite driver 依赖加入 Go 后端。
- [x] 1.2 设计 `internal/accounts` 或新增 `internal/store` 的 SQLite repository 边界，使用 `database/sql` 打开 couswee DB。
- [x] 1.3 定义 SQLite schema：`accounts`、`login_sessions`、`schema_migrations`，并写迁移执行顺序。
- [x] 1.4 移除旧账号文件 registry 与导入逻辑，确认 SQLite 是唯一账号读写源。

## 2. 后端账号与登录服务

- [x] 2.1 实现 SQLite-backed AccountRepository，覆盖 list/get/create/update/delete/set-active。
- [x] 2.2 实现 profile/auth 文件服务，负责创建目录、写 auth.json、权限 0700/0600、删除安全边界。
- [x] 2.3 实现登录 session service，支持 pending/waiting_user/succeeded/failed/expired/cancelled 状态。
- [x] 2.4 实现统一 Codex 登录 runner，使用 CLI 设备码授权流程并写入受管 profile。
- [x] 2.6 改造账号切换逻辑：从 SQLite 读取 account/profile，激活受管 auth 文件，并写回单 active 状态。

## 3. 后端 API

- [x] 3.1 改造 `GET /api/accounts` 以返回 SQLite 账号列表，并保留现有字段兼容前端。
- [x] 3.2 增加 `PATCH /api/accounts/:id` 用于编辑非秘密显示字段。
- [x] 3.3 保留/改造 `POST /api/accounts` 作为手动导入或兼容新增入口。
- [x] 3.4 改造 `DELETE /api/accounts` 为 SQLite 删除，并实现安全 auth/profile 清理规则。
- [x] 3.5 增加 `POST /api/codex/login/start`、`GET /api/codex/login/:session_id`、`POST /api/codex/login/:session_id/cancel`，并保留旧 start 路径兼容。
- [x] 3.7 保持 `POST /api/switch` nickname 兼容，并新增按 account id/profile 切换的内部路径。

## 4. 前端交互

- [x] 4.1 将账号管理栏的新增账号表单改为登录方式选择面板。
- [x] 4.2 实现统一 Codex 登录 UI：开始登录、打开验证 URL、显示 user code、过期时间、轮询状态和错误状态。
- [x] 4.4 增加账号编辑 UI，支持修改 nickname/display name/subscription 等非秘密字段。
- [x] 4.5 保留批量删除、勾选、切换和剩余流量显示，并改为使用 SQLite-backed API 响应；右上角账号搜索入口已移除。
- [x] 4.6 增加登录中、登录失败、已登录、已过期等状态展示和重试入口。

## 5. 测试与验证

- [x] 5.1 添加 SQLite repository 单元测试，覆盖 schema 初始化、CRUD、单 active、迁移导入。
- [x] 5.2 添加登录 session service 测试，覆盖状态转换、失败、过期、取消。
- [x] 5.3 添加 API handler 测试，覆盖账号 CRUD、登录 start/status/cancel、兼容 switch。
- [x] 5.4 添加 auth/profile 文件权限测试，确认目录 0700、auth 文件 0600。
- [x] 5.5 运行 `npm run build`、`npm run go:test`、`openspec validate design-codex-account-login --strict`。
