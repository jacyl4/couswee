## 1. 版本信息实现

- [x] 1.1 新增 `internal/version` 包，提供 `Version`、`Commit`、`BuildTime` 默认值和结构化输出方法
- [x] 1.2 在 `cmd/couswee` 中增加版本查询参数，查询时输出版本信息并直接退出
- [x] 1.3 在本地 API 中增加 `/api/version` 或等价版本查询接口，返回 `version`、`commit`、`build_time`
- [x] 1.4 为版本查询参数和 API 响应增加针对性测试

## 2. 构建入口

- [x] 2.1 新增 `Makefile` 或等价脚本，参考 `paraspeech` 用 `-ldflags -X` 注入版本、commit 和构建时间
- [x] 2.2 让发布构建入口先执行前端静态构建，再构建 `cmd/couswee` Linux amd64 二进制
- [x] 2.3 使用 Go `embed` 将发布构建生成的 `web/dist` 嵌入 couswee 二进制，保留 `COUSWEE_STATIC_DIR` 外部覆盖能力
- [x] 2.4 更新 `.gitignore`，确保发布二进制、`dist/`、前端构建输出和缓存不被误提交

## 3. GitHub 自动发布

- [x] 3.1 新增 `.github/workflows/release.yml`，在版本 tag push 时触发
- [x] 3.2 在 workflow 中安装 Go 与 Node 依赖，执行统一发布构建入口并传入 `${{ github.ref_name }}`
- [x] 3.3 打包 `couswee-<version>-linux-amd64.tar.gz` 并生成对应 `.sha256`
- [x] 3.4 使用 GitHub Release action 创建 Release，并上传 tar.gz 与 sha256 文件

## 4. 文档与验证

- [x] 4.1 更新 README，说明版本查询、tag 发布流程、产物命名和 sha256 校验
- [x] 4.2 本地执行 `make build VERSION=v0.1.0-test` 或等价命令，验证版本注入结果
- [x] 4.3 运行 `npm test -- --run` 和 `go test ./...`，确认现有功能未回归
- [x] 4.4 使用 OpenSpec 严格校验本 change，确认 proposal/design/specs/tasks 均通过
