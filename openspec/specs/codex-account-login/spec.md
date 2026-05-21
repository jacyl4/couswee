# codex-account-login Specification

## Purpose
TBD - created by archiving change design-codex-account-login. Update Purpose after archive.
## Requirements
### Requirement: Codex 账号登录流程
系统 SHALL 支持从 couswee 前端发起 Codex 账号登录，并由 Go 后端完成登录状态管理、profile 创建和 auth 文件写入。

#### Scenario: 用户选择登录方式
- **WHEN** 用户在前端点击新增账号
- **THEN** 系统 SHALL 提供一个 Codex 登录入口，并可保留手动导入作为兼容入口

#### Scenario: 登录成功生成账号
- **WHEN** Codex 登录成功取得可用认证信息
- **THEN** 后端 SHALL 创建或更新 SQLite 账号记录，并写入对应 profile 的 auth 文件

### Requirement: Codex 设备码登录
系统 SHALL 使用 Codex CLI 的设备码授权流程作为唯一主登录路径，并允许前端轮询登录状态。

#### Scenario: 发起 Codex 登录
- **WHEN** 前端请求开始 Codex 登录
- **THEN** 后端 SHALL 返回 verification URL、用户可输入的 code、过期时间和 session ID

#### Scenario: 设备码等待用户操作
- **WHEN** 用户尚未完成设备码授权
- **THEN** 登录 session SHALL 保持 waiting_user 或 pending 状态，前端 SHALL 可查询当前状态

#### Scenario: 设备码登录成功
- **WHEN** 后端检测到设备码授权完成
- **THEN** 后端 SHALL 写入 auth 文件，创建或更新账号记录，并把 session 标记为 succeeded

### Requirement: 登录 session 状态机
系统 SHALL 用持久化或可恢复的登录 session 表示登录流程状态。

#### Scenario: 登录状态查询
- **WHEN** 前端查询登录 session
- **THEN** 后端 SHALL 返回 session id、method、status、必要的用户操作信息、错误信息和过期时间

#### Scenario: 登录失败
- **WHEN** 登录交换、轮询或文件写入失败
- **THEN** session SHALL 标记为 failed，并保留可展示错误信息但不包含 token

### Requirement: auth 文件安全边界
系统 SHALL 将认证 token 写入受权限保护的 auth 文件，而不是返回给前端。

#### Scenario: 写入 auth 文件
- **WHEN** 后端持久化登录结果
- **THEN** auth 文件 SHALL 使用 0600 权限写入，父目录 SHALL 使用 0700 权限创建

#### Scenario: 前端响应不含 token
- **WHEN** 任意登录 API 返回响应
- **THEN** 响应 MUST NOT 包含 access token、refresh token 或等价秘密字段

