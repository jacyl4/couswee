## 背景

`design-logic.md` 要求 couswee 支持“前端 SvelteKit 交互 + 后端 Go 写入 profile/auth.json + CLI 可使用”的 Codex 账号添加机制。当前实现更接近手工 registry：账号记录保存在 JSON，用户需要知道 auth 文件路径，前端只能做简单新增/删除/切换。这不足以支撑真实登录流程、登录状态、账号编辑和后续多账号维护。

本方案将账号系统拆成三层：

1. SQLite 元数据层：保存账号/profile 的可展示和可编辑信息。
2. Auth/Profile 文件层：保存 Codex/prodex 可消费的 auth.json，负责权限和切换。
3. 登录流程层：提供统一的 Codex 设备码登录状态机，供前端驱动。

SQLite 驱动明确使用 `modernc.org/sqlite`，原因是 Go 后端可以在无 CGO 条件下构建和测试，避免 `github.com/mattn/go-sqlite3` 的系统依赖。

## 目标 / 非目标

**目标：**
- 使用 SQLite 替代 JSON 作为账号元数据主存储。
- 使用 `database/sql` + `modernc.org/sqlite` 实现纯 Go SQLite repository。
- 移除旧账号文件路径，避免 SQLite 与 JSON 双源不一致。
- 设计 Codex 账号登录状态机，覆盖 CLI 设备码授权流程。
- 明确 auth token 的安全边界：token 写入受管 auth 文件，不返回前端，不写日志。
- 前端支持添加账号、发起 Codex 登录、展示设备码/登录状态、编辑显示信息、删除账号和切换账号。
- 后端 API 支撑账号 CRUD、登录发起、回调/轮询、切换 active profile、状态查询。

**非目标：**
- 不引入独立 OAuth token 交换路径；当前以真实 Codex CLI 设备码行为为准。
- 不把 token 或 refresh token 存进 SQLite 明文字段。
- 不引入远程服务或多用户公网部署；仍然是本机 local-only 管理工具。

## 决策

### 1. SQLite 作为账号元数据主存储
新增 `~/.couswee/couswee.db`，通过 `database/sql` 打开，driver import 使用 `_ "modernc.org/sqlite"`。账号/profile 数据进入 SQLite，前端列表、编辑、删除都读写 SQLite，不再使用旧文件作为账号源。

备选方案：继续扩展 JSON。拒绝原因：登录状态、profile 元数据、迁移版本、设备码状态和编辑删除会让 JSON 并发与 schema 演进复杂化。

### 2. token 不进入前端，也默认不进 SQLite
SQLite 只保存 profile/auth 文件路径、状态和展示信息。Codex 设备码登录得到的 token 写入受管 auth 文件，例如 `~/.couswee/profiles/<profile>/auth.json` 或与实际 Codex/prodex 兼容的 profile 目录。文件权限 0600，目录权限 0700。

备选方案：SQLite 直接保存 token。拒绝原因：前端 CRUD 和调试日志更容易误触 token，安全边界不清晰。

### 3. 登录流程使用显式状态机
新增 login_sessions 表或等价表结构，状态包括 `pending`、`waiting_user`、`exchanging`、`succeeded`、`failed`、`expired`、`cancelled`。前端通过开始登录和轮询状态驱动 UI，不假设登录同步完成。

备选方案：后端启动 CLI 后阻塞 HTTP 请求直到登录结束。拒绝原因：设备码授权需要用户外部操作，阻塞请求不可靠。

### 4. 登录入口合并为单一 Codex 登录
当前真实可用的 Codex 登录方式是 CLI 设备码授权，因此前端只暴露一个 Codex 登录入口。旧的 `/oauth/start` 与 `/device/start` API 可以作为兼容别名转发到同一实现，但核心 service 不再维护两套流程。

备选方案：为两种登录方式维护两套账号模型。拒绝原因：会让列表、切换、删除、用量采集重复分支。

### 5. SQLite 是唯一账号 registry
启动时后端只初始化并读取 `~/.couswee/couswee.db`。旧账号文件不再读取、不再写入、不再自动导入，避免账号数据出现 JSON/SQLite 双源不一致。

备选方案：继续保留一次性 JSON 导入。拒绝原因：当前目标已经明确 SQLite 为唯一读写源，继续保留迁移入口会留下旧实现残留。

### 6. 切换语义从 auth_path 复制迁移到受管 profile
账号切换 API 使用 account/profile ID 或 nickname 找到 SQLite 记录，再把受管 auth 文件复制/链接到 Codex 当前使用的 `~/.codex/auth.json`，或调用兼容 CLI/profile 机制。迁移后的 active 状态写回 SQLite。

备选方案：继续让用户手填 auth_path。拒绝原因：登录式添加账号后 auth 文件应由 couswee 管理，避免用户理解内部路径。

### 7. 前端新增登录面板而不是只保留路径表单
账号管理栏中的新增账号应打开登录面板，提供一个 Codex 登录入口。该入口显示 verification URL/user code 和等待状态，并在登录完成后刷新账号列表。高级手动导入 auth 文件可以作为后续可选能力，不作为主路径。

备选方案：继续只显示 nickname/auth_path 表单。拒绝原因：不符合 `design-logic.md` 的登录机制目标。

## 数据模型草案

### accounts
- `id` TEXT PRIMARY KEY
- `nickname` TEXT NOT NULL UNIQUE
- `display_name` TEXT
- `profile_name` TEXT NOT NULL UNIQUE
- `auth_path` TEXT NOT NULL
- `login_method` TEXT NOT NULL (`device` / `imported`)
- `status` TEXT NOT NULL (`active` / `ready` / `login_pending` / `login_failed` / `disabled`)
- `subscription` TEXT
- `active` INTEGER NOT NULL DEFAULT 0
- `last_used_at` TEXT
- `created_at` TEXT NOT NULL
- `updated_at` TEXT NOT NULL

### login_sessions
- `id` TEXT PRIMARY KEY
- `method` TEXT NOT NULL
- `account_id` TEXT
- `profile_name` TEXT
- `status` TEXT NOT NULL
- `verification_url` TEXT
- `device_code` TEXT
- `user_code` TEXT
- `expires_at` TEXT
- `error` TEXT
- `created_at` TEXT NOT NULL
- `updated_at` TEXT NOT NULL

### schema_migrations
- `version` INTEGER PRIMARY KEY
- `applied_at` TEXT NOT NULL

## API 草案

- `GET /api/accounts`：返回 SQLite 账号列表，兼容现有前端字段并增加 id/profile/status/login_method。
- `POST /api/accounts`：保留手动导入/编辑入口；实现时可改为创建 imported 账号。
- `PATCH /api/accounts/:id`：编辑 nickname/display_name/subscription 等非 token 字段。
- `DELETE /api/accounts`：批量删除账号，删除 SQLite 记录并可选择删除受管 auth/profile 文件。
- `POST /api/accounts/:id/switch` 或兼容 `POST /api/switch`：切换 active account。
- `POST /api/codex/login/start`：创建设备码登录 session，返回 verification_url、device_code/user_code、expires_at。
- `POST /api/codex/login/oauth/start`、`POST /api/codex/login/device/start`：兼容旧前端/调用方，转发到同一登录实现。
- `GET /api/codex/login/:session_id`：返回登录状态，前端轮询。
- `POST /api/codex/login/:session_id/cancel`：取消 pending 登录。

## 风险 / 权衡

- [风险] Codex/Prodex 登录命令行为变化 → 缓解：以真实 CLI 行为做 spike，登录 runner 接口保持可替换。
- [风险] SQLite schema 设计后续还会变化 → 缓解：第一版就加入 schema_migrations，并用迁移测试覆盖。
- [风险] 删除账号误删 auth 文件 → 缓解：删除 API 区分删除记录与删除受管 auth 文件；默认只删除 couswee 管理目录内的文件，外部路径不主动删。
- [风险] token 泄漏 → 缓解：token 不入前端响应、不入 SQLite 明文字段、不写日志；文件权限 0600。
- [风险] 旧 JSON 与新 SQLite 双源不一致 → 缓解：迁移后 SQLite 成为唯一读写源，旧 JSON 只作为导入备份。
