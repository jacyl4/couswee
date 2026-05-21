## ADDED Requirements

### Requirement: 登录式新增账号界面
系统 SHALL 将新增账号入口升级为登录式流程，而不是只要求用户填写 auth 文件路径。

#### Scenario: 打开新增账号
- **WHEN** 用户点击账号管理栏中的 `新增账号`
- **THEN** 前端 SHALL 显示一个 Codex 登录入口和手动导入兼容入口

#### Scenario: Codex 登录展示
- **WHEN** 用户选择 Codex 登录且后端返回 verification URL 和 code
- **THEN** 前端 SHALL 显示验证 URL、用户 code、过期时间和等待状态

### Requirement: SQLite 账号编辑与删除界面
系统 SHALL 允许用户基于 SQLite 账号记录编辑显示信息并删除账号。

#### Scenario: 编辑账号显示信息
- **WHEN** 用户编辑账号 nickname、display name 或 subscription 等非秘密字段
- **THEN** 前端 SHALL 调用账号编辑 API 并刷新账号列表

#### Scenario: 删除账号
- **WHEN** 用户删除一个或多个账号
- **THEN** 前端 SHALL 调用删除 API，并在成功后移除对应账号卡片

### Requirement: 登录状态展示
系统 SHALL 在账号列表和新增账号流程中展示登录状态。

#### Scenario: 账号登录中
- **WHEN** 一个账号或登录 session 处于 pending 或 waiting_user 状态
- **THEN** 前端 SHALL 展示等待用户授权或登录中的状态

#### Scenario: 账号登录失败
- **WHEN** 一个账号或登录 session 处于 failed 状态
- **THEN** 前端 SHALL 展示可读错误信息，并提供重新登录入口
