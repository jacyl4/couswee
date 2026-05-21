## ADDED Requirements

### Requirement: 统一后端用量刷新入口
系统 SHALL 通过统一的用量刷新管理模块执行所有写入型 Codex 用量刷新动作，包括全账号刷新与单账号刷新。

#### Scenario: 启动触发全量刷新
- **WHEN** couswee 服务启动并启用用量服务
- **THEN** 系统 SHALL 通过统一刷新管理模块触发一次全账号用量刷新

#### Scenario: 周期触发全量刷新
- **WHEN** 用量刷新间隔到达
- **THEN** 系统 SHALL 通过统一刷新管理模块触发全账号用量刷新

#### Scenario: 账号新增触发单账号刷新
- **WHEN** 新账号成功加入账号库
- **THEN** 系统 SHALL 通过统一刷新管理模块仅刷新该账号的用量

#### Scenario: 账号切换触发单账号刷新
- **WHEN** 本地切换 API 成功切换当前账号
- **THEN** 系统 SHALL 通过统一刷新管理模块仅刷新新激活账号的用量

#### Scenario: 登录成功触发单账号刷新
- **WHEN** Codex 登录 session 首次进入成功状态且关联账号存在
- **THEN** 系统 SHALL 通过统一刷新管理模块仅刷新该登录账号的用量

### Requirement: 刷新动作携带来源语义
系统 SHALL 为内部刷新动作保留来源语义，以区分启动、周期、账号新增、账号切换、登录成功和显式全量刷新等触发原因。

#### Scenario: 刷新动作被测试或记录
- **WHEN** 用量刷新管理模块执行刷新动作
- **THEN** 调用方 SHALL 能以稳定的 reason/action 标识表达触发来源，而不需要重复编写刷新流程

### Requirement: 刷新执行保持单一写入路径
系统 SHALL 通过同一条路径更新用量 cache、stale/error 元数据和 SQLite 中的最新成功用量，不得在 server 或 frontend 层绕过该路径写入用量状态。

#### Scenario: 刷新成功
- **WHEN** 任一刷新动作采集到成功用量记录
- **THEN** 系统 SHALL 通过统一刷新路径更新内存 cache 并持久化最新成功剩余流量和 reset 时间

#### Scenario: 刷新失败
- **WHEN** 任一刷新动作采集失败
- **THEN** 系统 SHALL 通过统一刷新路径保留最后可用值并标记 stale/error

### Requirement: 读取接口不隐式刷新
系统 SHALL 将缓存读取与 live collection 触发分离，读取用量快照不得隐式执行写入型刷新动作。

#### Scenario: 用量接口读取缓存
- **WHEN** 客户端请求 `GET /api/codex/usage`
- **THEN** 系统 SHALL 返回当前缓存快照，且 SHALL NOT 因该 GET 请求隐式调用 live collector

