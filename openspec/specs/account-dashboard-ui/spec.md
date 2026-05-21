# account-dashboard-ui Specification

## Purpose
TBD - created by archiving change align-account-dashboard-ui. Update Purpose after archive.
## Requirements
### Requirement: 新版效果图页面结构
系统 SHALL 将 couswee 账号切换监测页渲染为符合新版效果图风格的暗色玻璃拟态单页仪表盘，并在保持视觉风格的同时采用更紧凑的布局以提升多账号可见数量。

#### Scenario: 初始页面结构
- **WHEN** 仪表盘加载并取得账号数据
- **THEN** 页面显示标题 `couswee 账号切换监测`、副标题 `个人多账号用量概览`、三张概览卡、账号管理工具条和账号卡片列表

#### Scenario: 紧凑空间使用
- **WHEN** 桌面视口显示多个账号
- **THEN** 页面 SHALL 使用较小的标题、概览卡、头像、按钮、卡片内边距和纵向间距，以便同屏显示更多账号

#### Scenario: 旧 UI 要求冲突
- **WHEN** 旧的未归档 OpenSpec change 描述了不同的仪表盘布局或文案
- **THEN** 系统在本次界面实现中 SHALL 以 `account-dashboard-ui` 的新版效果图契约为准

### Requirement: 顶部概览
系统 SHALL 提供与新版效果图一致的顶部概览指标，且 SHALL NOT 在右上角显示账号搜索入口。

#### Scenario: 顶部概览指标
- **WHEN** 账号和用量数据加载完成
- **THEN** 顶部概览区显示 `可用账号`、`建议切换` 和 `最后同步` 三项指标

#### Scenario: 最后同步文案
- **WHEN** 用量数据包含最后刷新时间
- **THEN** `最后同步` 显示类似 `2 分钟前` 的短相对时间文案

### Requirement: 账号管理工具条
系统 SHALL 在顶部概览与账号卡片之间显示整齐、紧凑、一体化的账号管理工具条。

#### Scenario: 管理工具条内容
- **WHEN** 仪表盘可见
- **THEN** 页面显示 `账号管理` 标题、齿轮图标、展开/收起按钮、`新增账号` 按钮、`删除选中` 按钮和 `已选择 N 项` 计数，并且这些元素 SHALL 在同一工具条层级内整齐对齐

#### Scenario: 新增账号入口
- **WHEN** 用户点击 `新增账号`
- **THEN** 系统 SHALL 在管理工具条附近以紧凑表单进入新增账号流程

#### Scenario: 删除选中入口
- **WHEN** 没有账号被选中
- **THEN** `删除选中` SHALL 保持禁用或不可执行状态

#### Scenario: 删除选中可执行
- **WHEN** 至少一个账号被选中
- **THEN** `删除选中` SHALL 显示为可执行状态，并且 `已选择 N 项` 中的 N SHALL 等于当前选中账号数量

### Requirement: 账号选择状态
系统 SHALL 支持在账号卡片上选择或取消选择账号，并展示与新版效果图一致的选中态。

#### Scenario: 勾选账号
- **WHEN** 用户勾选某个账号卡片左侧的复选框
- **THEN** 该账号进入选中集合，复选框显示选中标记，卡片边框显示绿色高亮

#### Scenario: 取消勾选账号
- **WHEN** 用户取消勾选已选中的账号
- **THEN** 该账号从选中集合移除，复选框恢复未选中样式，选中计数同步减少

### Requirement: 账号卡片身份与操作内容
系统 SHALL 将每个账号显示为紧凑的全宽卡片，并包含身份、状态和切换操作。

#### Scenario: 账号身份可见
- **WHEN** 账号卡片渲染
- **THEN** 卡片显示圆形首字母头像、账号昵称、彩色状态徽标和 `上次切换` 元数据，且头像和文字尺寸 SHALL 不造成过高卡片

#### Scenario: 每个账号显示切换按钮
- **WHEN** 账号卡片渲染
- **THEN** 卡片右侧显示清晰但更紧凑的 `切换` 按钮，并按账号状态使用绿色、黄色或红色样式

#### Scenario: 切换按钮调用后端
- **WHEN** 用户点击某个账号卡片的 `切换` 按钮
- **THEN** 前端 SHALL 调用现有账号切换 API，并在成功后刷新账号和用量状态

### Requirement: 剩余流量展示
系统 SHALL 按新版效果图用剩余百分比、进度条和 reset 文案展示 Codex 限额。

#### Scenario: 5h 剩余流量行
- **WHEN** 账号存在 5h 剩余流量数据
- **THEN** 账号卡片显示 `5h limit:` 行、水平进度条、类似 `93% left` 的文本，以及该账号的 5h reset 时间

#### Scenario: Weekly 剩余流量行
- **WHEN** 账号存在 weekly 剩余流量数据
- **THEN** 账号卡片显示 `Weekly limit:` 行、水平进度条、类似 `99% left` 的文本，以及该账号的 weekly reset 日期/时间

#### Scenario: 保持剩余语义
- **WHEN** 前端将后端用量记录映射为展示值
- **THEN** 前端 SHALL 优先使用 `5h_remaining` 和 `weekly_remaining` 字段，并且 MUST NOT 将已消耗用量标记为剩余流量

### Requirement: 阈值视觉状态
系统 SHALL 根据剩余流量状态为账号卡片、头像、徽标、进度条和切换按钮着色。

#### Scenario: 可用状态
- **WHEN** 账号的 5h 与 weekly 剩余容量都大于 50%
- **THEN** 账号 SHALL 使用绿色样式并显示状态 `可用`

#### Scenario: 接近用尽状态
- **WHEN** 账号最低剩余容量在 10% 到 50% 之间且包含边界
- **THEN** 账号 SHALL 使用黄色样式并显示状态 `接近用尽`

#### Scenario: 冷却中状态
- **WHEN** 账号最低剩余容量小于 10%
- **THEN** 账号 SHALL 使用红色样式并显示状态 `冷却中`

### Requirement: 自动同步呈现
系统 SHALL 继续自动刷新数据，但 MUST NOT 在页面底部显示 `数据每分钟自动同步` 这类独立提示行。

#### Scenario: 无底部同步提示
- **WHEN** 仪表盘可见
- **THEN** 页面底部 SHALL NOT 渲染 `数据每分钟自动同步` 或等价的独立同步提示行

#### Scenario: 周期刷新数据
- **WHEN** 仪表盘持续打开
- **THEN** 前端 SHALL 大约每分钟刷新一次用量数据，并更新顶部 `最后同步` 概览

### Requirement: 响应式与异常状态
系统 SHALL 在桌面端保持新版效果图层级，同时在窄屏和异常数据状态下保持可用。

#### Scenario: 窄屏视口
- **WHEN** 视口宽度不足以容纳桌面横向布局
- **THEN** 账号卡片区域 SHALL 纵向堆叠，并且不隐藏昵称、剩余百分比、reset 文案或切换按钮

#### Scenario: API 错误状态
- **WHEN** 账号或用量数据加载失败
- **THEN** 页面 SHALL 在保留整体框架和已加载旧数据的同时显示可见错误提示

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
- **WHEN** 用户编辑账号 nickname 或 subscription 等非秘密字段
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
