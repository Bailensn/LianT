# UI 设计规范（Manager / Android）

来源：[Ludi-Herry/mobile-design-system-skill](https://github.com/Ludi-Herry/mobile-design-system-skill)
（移动端 iOS HIG / Android Material 3 执行级 Skill）。本文只记录**本项目如何落地**，
规范原文与推理在那边，不复制结论。

## 硬门禁与本仓库的对应实现

| 规范条款 | Compose 落地 |
| --- | --- |
| 顶部避让状态栏/灵动岛 | 根布局 `Modifier.statusBarsPadding()`；`Scaffold` 的 `TopAppBar` 自带状态栏 insets |
| 底部避让 Home Bar / 手势条 | `BottomTabBar` 与输入条用 `navigationBarsPadding()`；输入条改用 `WindowInsets.safeDrawing.only(Bottom)`，避免与 `imePadding` 叠加 |
| 软键盘不遮挡输入 | `enableEdgeToEdge` + `windowSoftInputMode="adjustResize"` + 输入条 `windowInsetsPadding(safeDrawing.only(Bottom))`；配对页可滚动 |
| 最小触控热区 48×48dp | 所有按钮/可点项 `heightIn(min = 48.dp)`；列表行 `padding(vertical = 10.dp)` 后 ≥64dp |
| 相邻可点项留白 ≥8dp | 顶部操作 `Arrangement.spacedBy(8.dp)` / TabBar 等分 |
| 0.5dp 微边框 | `HorizontalDivider(thickness = 0.5.dp)`，颜色用 `outlineVariant` |
| 3–5 槽位 TabBar | `BottomTabBar`：消息 / Bot / 设置 |
| 空态与失败出口 | 会话空态给说明、Bot 空态给「重新加载」、错误条常驻底部 |
| 同心圆角 | 气泡圆角按方向收角（外角 12dp、贴边角 2dp） |
| 单一主 CTA | 配对页只有一个全宽「配对」按钮；设置页的「解除配对」是唯一危险操作 |

## 新增界面时必须自查

1. 是否避让了顶部状态栏与底部手势条？
2. 每个可点元素热区是否 ≥48dp？相邻是否 ≥8dp？
3. 键盘弹起时输入框与主按钮是否仍可见/可点？
4. 分割线是否 0.5dp？圆角是否内外同心？
5. 空列表、断网、超时是否都有明确文案与操作出口？
6. 一屏是否只有 1 个全宽主按钮？

## 已按规范落地

- **图片消息**：`MediaThumb` 只接受 `1:1 / 4:3 / 16:9` 三档，加载前后占位同尺寸，避免布局跳动；
  消息类型由 `messages.kind` 区分（DB v2）。
- **底部抽屉**：`ChatDetailSheet` 使用 `ModalBottomSheet`，36×4dp 拖拽条、内容 60% 高度，
  遮罩吞掉手势、底层列表不会跟着滚；信息类允许点遮罩关闭，表单类必须改成只能点「取消/保存」。
- **失败出口**：监听断开、未配对、Bot 空态都有可点的重试/重新加载入口。
- **媒体加载**：`media.fetch` 后经同一条钉指纹连接下载，解码后按 4:3 裁切填进占位盒；
  失败保留占位，不打断聊天。

## 待补

- 图片预览若用全屏浮层，需按 30%/60%/80% 三档做二级抽屉。
- `media_expired` / `media_evicted` 现在统一显示「待下载」，后续在占位盒上区分文案。
