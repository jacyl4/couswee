## ADDED Requirements

### Requirement: 运行时版本元数据
couswee SHALL 在 Go 运行时维护版本号、commit 和构建时间三项元数据，并在开发构建中提供稳定默认值。

#### Scenario: 开发态默认值
- **WHEN** 未通过构建参数注入版本信息的 couswee 二进制启动或查询版本
- **THEN** 系统 SHALL 返回 `Version=dev`、`Commit=none`、`BuildTime=unknown` 或等价字段值

#### Scenario: 发布态注入值
- **WHEN** 使用发布构建入口传入 `VERSION` 并执行 Go build
- **THEN** 系统 SHALL 将 tag 版本号、当前短 commit 和构建时间注入运行时版本元数据

### Requirement: 版本查询入口
couswee SHALL 提供可自动化读取的版本查询入口，用于定位当前运行产物。

#### Scenario: 命令行查询版本
- **WHEN** 用户执行 couswee 二进制并传入版本查询参数
- **THEN** 程序 SHALL 输出当前版本号、commit 和构建时间，并在不启动 HTTP 服务的情况下退出

#### Scenario: API 查询版本
- **WHEN** 用户请求本地服务的版本查询 API
- **THEN** 服务 SHALL 返回 JSON 对象，包含 `version`、`commit` 和 `build_time` 字段

### Requirement: 文档说明版本语义
couswee SHALL 在项目文档中说明版本字段来源和开发态默认值。

#### Scenario: 阅读版本文档
- **WHEN** 用户查看 README 的版本说明
- **THEN** 文档 SHALL 说明发布版本来自 Git tag，commit 来自构建时 Git commit，构建时间来自构建入口
