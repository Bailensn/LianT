# 任务记录

每次任务的简要总结：目标、完成内容、验证方式、遗留问题与提交。
按时间倒序。

## 2026-10-20 · 私聊 / 话题 / 消息操作 / 附件发送

**目标**：把私聊加进聊天页；Telegram 论坛话题和不同会话分开上下文；去掉长按，
改成显式操作按钮；后台支持图片/文件发送，并修附件下载。

**完成**

- **会话模型**：`conversations` 表 + `messages.conv_key/thread_id`（schema v5）；
  会话键为 `chat_id`（群/私聊）或 `chat_id::thread_id`（话题）。
- **私聊**：Telegram 私聊落库并出现在会话列表；QQ C2C 新增持久化与发送。
- **话题**：Telegram 论坛话题按 `message_thread_id` 拆成独立会话，发送/上传带 thread。
  `forum_topic_created` 只更新会话标题，不生成空气泡。
- **操作菜单**：去掉全部长按逻辑；每条消息与头像都有「⋯」，点开统一菜单：
  查看图片 / 引用 / 复制文本 / 下载附件 / 撤回。
- **附件发送**：输入条新增回形针，支持图片/视频/音频/文件；Telegram 走 multipart，
  QQ 富媒体未接入时隐藏按钮并返回 409。
- **附件下载**：文件卡片改成按钮，带鉴权 `fetch` → Blob → 下载；失败展示平台错误，
  不再依赖 WebView 的 `<a download>`。
- **UI**：会话列表显示话题/私聊；消息流只在必要时显示时间，`⋯` 常驻但低对比。

**验证**

- `node --check internal/admin/web/admin.js`；CSS 括号平衡。
- tree-sitter-go 全项目 41 个 Go 文件，0 语法错误。
- Go Playground 单文件类型检查：`store/messages`、bot/qqbot 新增函数通过。
- sql.js：v4→v5 迁移、会话目录回填、消息按 `conv_key` 查询通过。
- 应用内 AI 浏览器 + mock API 实测：群/话题/私聊切换、独立历史、操作菜单、
  文件下载按钮、附件按钮。
- 仍未跑项目内 `go build`（本机 Go 工具链 argv 问题）。

**遗留**

- QQ 频道 runtime 未接入（模型已保留 `kind=channel`）。
- QQ 私聊撤回、QQ 富媒体上传需要平台接口/权限。
- 命令引擎的 `RecentList` 仍是群级，不按话题拆分。
- v4 之前的历史消息没有 `thread_id`，无法自动回溯拆成话题。

**提交**：本次 commit（见 `git log --oneline -1`）。

## 2026-10-20 · 聊天页面重做

**目标**：把像后台表格的聊天页改成真正的聊天软件界面，不要什么都套卡片。

**完成**：全屏聊天布局、平面会话列表、消息气泡与日期胶囊、同人合并、
单条圆角输入、默认文本/可选卡片、Enter 发送、自动增高、移动端会话抽屉、
会话列表最近消息预览（`store.LastMessage`）、上传按钮与操作菜单的基础结构。

**验证**：1680/1920 与 390×844 视口实际渲染检查；隐藏 WebView 合成残影用 DOM 尺寸复核。
**提交**：`08c85f4`（handoff 记录 `89bae87`）。

## 2026-10-20 · 聊天记录持久化

**目标**：消息上限改为无限，历史写入数据库文件。

**完成**：`messages` 表（schema v4），删除内存 `recent` map 与 200 条上限；
bot/qqbot 历史查询走 SQLite；撤回/媒体状态持久化；SSE 用索引签名；
API/CLI 默认返回全部，`limit` 可选。

**验证**：tree-sitter、Go Playground、sql.js 迁移/upsert/签名测试。
**提交**：`9cb8d9a`（handoff 记录 `2bdb344`）。

## 2026-10-20 · 聊天媒体

**目标**：采集、取回、展示聊天媒体。

**完成**：媒体引用入库；`GET /api/media`（Telegram getFile / QQ 代理）；
`available/expired/gone/unknown`；图片懒加载 + dialog、文件卡片、语音视频控件、表情 emoji。

**验证**：Go Playground 类型检查、JS 语法检查。
**提交**：`65629b7`（handoff 记录 `7991c50`）。
