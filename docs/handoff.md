# 现状与下一轮交接

## 项目定位

LianT 是**自托管的跨平台聊天软件**：Service（任务进程）负责平台接入、消息中继与临时存储；
Manager（主进程/管理端/CLI）负责管理与交互；机器人、规则、审计是辅助能力。详见 [design.md](design.md)。

## 已完成（最近提交）

| 提交 | 内容 |
| --- | --- |
| `c71a23e` | 消息状态语义：`received` / `sent` / `recalled`；撤回变墓碑（正文清空）；UI 显示状态 |
| `4ef9e15` | `internal/chat` 投影模型；IPC `Payload`；任务进程 `chat` 指令 |
| `acfde02` | 会话优先（默认 #chat、导航重排）；`LianT chat` CLI |
| `b088326` | `/api/events` SSE 推送消息变化（服务端快照比对，非 runtime 事件） |
| `ccc300a` | 管理端配色统一（冷灰白 + 靛蓝，单一变量集） |
| `7c47d22` | Bot 列表显示真实连接状态（`state` / `online`），不再一律「运行中」 |

更早：死代码清理、admin 页面拆 html/css/js、AGPL-3.0、文档扩充、平台校验走注册表、路径容错。

## 验证状态

⚠️ **所有 Go 改动都没有编译验证**。本机 Go 工具链损坏（`~/goroot/bin/go` 是实验性 PIE 版，Android 拒绝执行）；
QEMU 方式可跑但会让手机满载发热，已停用。改动只做了：括号平衡、import 使用、JS `node --check`、逐行复核。

## 下一轮：聊天媒体消息（图片 / 文件 / 表情）

1. **采集**：`RecentMsg` 只有 `Kind`，缺媒体引用。
   - Telegram：`Photo`（取最大 size 的 `file_id`）、`Document`（`file_name`/`mime_type`/`file_size`）、`Sticker`（`emoji`/`file_id`）、`Animation`；
   - QQ：`attachments[].url / filename / size / content_type`。
2. **取回**：新增 `GET /api/media?bot_id=&chat_id=&msg=`，管理端鉴权通过后：
   - Telegram：`getFile` → `file_path` → 流式转发（不要把带 token 的直链给浏览器）；
   - QQ：代理附件 URL；
   - 限制大小、校验消息归属，避免变成任意文件下载器。
3. **状态**：`available` / `expired`（`file_path` 过期需重取）/ `gone`（QQ 链接失效）/ `unknown`；取不到就明说，不伪造缩略图。
4. **前端**：图片 lazy + `<dialog>` 放大；文件显示名称/大小/类型 + 下载；表情按 `emoji` 渲染；
   `Classify` 补 `sticker` / `animation` 两类。

## 其他待办

- SSE 目前是服务端比对快照；下一步给 `bot`/`qqbot` 的 `remember`、`MarkRecalled` 加 notifier，走真事件；
- `admin/qbot_compat.go` 与 `admin.go` 原生路由重复（compat 会抢占多个路由），需要二选一；
- 全项目无测试；`internal/rules`、`internal/chat` 最适合先补；
- `qctx` 的 `b`/`q` 双指针待抽成平台无关接口（Discord 接入前置）。
