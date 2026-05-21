## 1. 方案与基线确认

- [x] 1.1 确认所有旧 change 已归档，当前只保留 `optimize-dashboard-density` active change。
- [x] 1.2 检查当前 `+page.svelte` 与 `global.css` 中造成空间浪费的结构和样式。

## 2. 前端结构调整

- [x] 2.1 从 Svelte 页面彻底移除底部 `数据每分钟自动同步` footer 结构。
- [x] 2.2 将账号管理栏调整为一体化 toolbar，减少 tab/body 割裂感。
- [x] 2.3 保留新增账号表单、删除选中、选中计数、搜索和切换功能。

## 3. CSS 密度优化

- [x] 3.1 缩小标题、副标题、顶部区域和概览卡尺寸。
- [x] 3.2 缩小账号卡片高度、padding、头像、按钮、进度条和行间距。
- [x] 3.3 调整账号管理 toolbar 和新增表单对齐，保证桌面端整齐、窄屏可换行。
- [x] 3.4 删除 `.sync-footer` 相关样式和其他冗余旧样式。

## 4. 验证

- [x] 4.1 运行 `npm run build`。
- [x] 4.2 运行 `npm run go:test` 或 `npm run test`。
- [x] 4.3 搜索确认 `数据每分钟自动同步` 和 `.sync-footer` 无残留。
- [x] 4.4 运行 `openspec validate optimize-dashboard-density --strict`。
