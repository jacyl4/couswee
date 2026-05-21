## ADDED Requirements

### Requirement: 本地发布构建入口
couswee SHALL 提供单一构建入口，用于生成带版本信息的发布产物，并复用现有前端与 Go 构建链路。

#### Scenario: 构建发布二进制
- **WHEN** 维护者执行发布构建命令并传入 `VERSION`
- **THEN** 构建入口 SHALL 先生成 SvelteKit 静态前端产物，再构建 `cmd/couswee` 对应的 Linux amd64 二进制

#### Scenario: 查询构建入口版本
- **WHEN** 维护者执行构建入口的版本目标
- **THEN** 构建入口 SHALL 输出当前默认发布版本值

### Requirement: GitHub tag 发布流程
couswee SHALL 提供 GitHub Actions workflow，在版本 tag 推送时自动构建并创建 Release。

#### Scenario: tag 触发发布
- **WHEN** GitHub 仓库收到符合版本规则的 tag push
- **THEN** workflow SHALL checkout 源码、安装 Go、安装 Node 依赖、构建前端和后端，并使用 tag 名称作为发布版本

#### Scenario: 创建 Release
- **WHEN** workflow 成功构建发布包
- **THEN** workflow SHALL 创建 GitHub Release，并上传 `couswee-<version>-linux-amd64.tar.gz` 和对应 `.sha256` 文件

### Requirement: 发布包可校验
couswee SHALL 为发布包生成 SHA-256 校验文件，便于用户确认下载完整性。

#### Scenario: 生成校验文件
- **WHEN** workflow 打包 `couswee-<version>-linux-amd64.tar.gz`
- **THEN** workflow SHALL 生成同名 `.sha256` 文件，内容可用于校验 tar.gz

### Requirement: 发布流程文档
couswee SHALL 在 README 中记录 GitHub 自动构建和发布步骤。

#### Scenario: 阅读发布说明
- **WHEN** 维护者查看 README 的发布章节
- **THEN** 文档 SHALL 说明如何创建版本 tag、如何触发 GitHub Release、产物命名规则和校验文件用途
