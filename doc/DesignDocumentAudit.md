# 设计文档审查与收录

## 范围与判定

审查日期：2026-10-13。实现基线：`pre-release/v0.1` 分支 `de44eda`。
本轮只整理和提交文档，不修改实现，不修改与实现不符的原文，不推送。

盘点 `doc/` 原有 82 份 Markdown：19 份已跟踪设计／开发记录、62 份未跟踪文档，
另有 `README_CN.md` 属于发布介绍。62 份待收录文档中，14 份收录、44 份因差异跳过、
4 份暂缓。两份已跟踪文档发现漂移，仅在本文登记。本文不计入原有 82 份。

核对重点为公共 API、单位与坐标、职责与资源归属、当前输入／布局／样式／缓存路径，
以及文中“已实现／当前／待实施”的表述。发现明确矛盾后即停止将该篇作为收录候选，
不声称穷举了跳过文档的每个历史章节。未修改源码以适配旧设计。

判定规则：

- 有指导价值且当前契约与实现一致的文档，按独立主题分别提交。
- 明确标注日期或切片的历史验证保留，不将过去失败或验收范围当作今天的结果。
- 明确未实现的后续方案不会仅因“代码里没有新增 API”被判为漂移；但其现状基线、
  所依赖接口或实施边界也需要与当前框架一致。
- 有真实差异的整篇跳过，不摘取部分章节后冒充已经同步的全文。
- 收录文档指向未收录旧草案的链接改为当前源码或本审查记录；不更新旧草案正文。

## 本轮收录

| 文档 | 核对结论 | 证据入口 |
| --- | --- | --- |
| [DesignApplicationIdentity.md](DesignApplicationIdentity.md) | App ID 校验归平台入口，GUI 单例不另存 ID，三平台身份/图标资源契约匹配。 | `platform/platform.go`；`gui/application.go`；`platform/linux/x11/platform_linux.go`；`platform/windows/win32/platform_windows.go`；`platform/darwin/cocoa/platform_darwin.go` |
| [DesignDropDown.md](DesignDropDown.md) | 模型/Selected/独立委托、Popover Placement 和留白及禁用入口吻合；关联旧规划需注明未收录。 | `widgets/drop_down.go`；`widgets/drop_down_popup.go`；`widgets/ui/drop_down.go` |
| [DesignExternalOpen.md](DesignExternalOpen.md) | OpenURL/OpenPath 输入检查及返回错误、UI Sync 和原生调用匹配。 | `platform/internal/desktopopen/input.go`；`gui/application.go`；`ui/app.go` |
| [DesignGesture.md](DesignGesture.md) | 参与协议、同步裁决、取消及 rootBase 归属匹配代码；整理指向旧草案的引用。 | `gui/gesture.go`；`gui/input_controller.go`；`gui/root.go` |
| [DesignGraphicsColor.md](DesignGraphicsColor.md) | Color 预乘契约及原生边界反预乘与实现吻合。 | `platform/graphics/brush.go`；`platform/graphics/opengl`；`platform/graphics/direct2d`；`platform/typography/pango` |
| [DesignImage.md](DesignImage.md) | Image Size/Update/Destroy、Painter 所有权与帧内延迟释放吻合。 | `platform/graphics/image.go`；`ui/image.go`；`gui/image.go` |
| [DesignRenderWidget.md](DesignRenderWidget.md) | 公开 RenderWidget/RenderImage、allocation/帧外/预乘契约吻合。 | `gui/widget.go:1029`；`platform/graphics/painter.go:22` |
| [DesignShortcuts.md](DesignShortcuts.md) | 事实键/GUI Key与Modifiers转换、组合匹配、消费通道及链式描述符吻合。 | `gui/key.go`；`gui/shortcut.go`；`ui/shortcut.go`；`platform/events/input_event.go` |
| [DesignSVG.md](DesignSVG.md) | 公开 Parse/Image/WithRawColors、受限profile与固定fork及 tint 语义匹配；依赖文档需处理。 | `icon/svg/svg.go`；`icon/svg/normalize.go`；`go.mod` |
| [DesignTimer.md](DesignTimer.md) | Application 隔离调度器、到期堆/单 Timer/单 goroutine、UI runtime timer 与代码吻合；历史失败记录不等于当前失败。 | `gui/timer.go`；`gui/timer_scheduler.go`；`ui/timer.go` |
| [DesignWidgetEnabled.md](DesignWidgetEnabled.md) | 自身 Enabled + IsEnabled、状态提交后取消输入、延迟 StyleChanged、跨 Root DropDown 防护吻合。 | `gui/widget.go`；`gui/event.go`；`widgets/drop_down_popup.go`；`ui/view.go` |
| [DesignWidgetIdentity.md](DesignWidgetIdentity.md) | ID/Name 独立、挂载目标内 ID/类型协调、Root 查询和 Captions 与实现匹配。 | `gui/widget.go`；`ui/root.go`；`ui/view.go`；`ui/header_bar.go` |
| [DesignWidgetSnapshot.md](DesignWidgetSnapshot.md) | Attributes/SetAttribute、独立 widgets 信息及 JSON 失败处理匹配。 | `gui/info.go`；`widgets/info.go`；`gui/dev/server.go` |
| [Testing.md](Testing.md) | 仅两类测试、既有测试不迁移及启动线程要求与规范/入口相符。 | `AGENTS.md`；`platform/platform.go`；`tests/window` |

各文档按主题独立提交；审查记录另作一笔提交，不混入实现、测试代码或发布 README。

## 因差异跳过

下列原文件保持原样、未加入提交。证据中的文档行号对应审查时工作区原文，
源码入口用来核对当前契约；列举的是阻止收录的具体差异，不是整篇文档价值的否定。

| 文档 | 核对结论 | 证据入口 |
| --- | --- | --- |
| `Clipboard.md` | 复制/粘贴及 TextInput 快捷键仍写为待实施；当前共享编辑核心已实现 copy/cut/paste。INCR 不支持的旧限制仍属实，不将 DnD 的 INCR 能力误归剪贴板。 | `doc/Clipboard.md:94`；`gui/text_editor.go`；`platform/linux/x11/clipboard_linux.go` |
| `DesignApp.md` | 公开 Dev 端点仍宣称只有 SnapshotPath/EventPath，遗漏已增加 TextPath；旧 UI State 自动节点保留的说明需要重新核对。 | `doc/DesignApp.md:215`；`gui/dev/server.go:19`；`ui/state.go` |
| `DesignCheckButton.md` | 兜底样式归属/文件及单选配色仍按旧 widgets/style 方案；现主题已迁到 Basic，圆角覆盖也更新。 | `doc/DesignCheckButton.md:87,124`；`theme/basic/check_button.go`；`widgets/style/style.go` |
| `DesignConventions.md` | FileDialog 的错误签名迁移仍是后续事项，与已实现访问资源/错误回调不符。 | `doc/DesignConventions.md:40`；`gui/dialog.go` |
| `DesignCursor.md` | Cursor getter默认写 CursorDefault 而实际保留 nil；光标解析仍只按 hover，当前接受手势的捕获目标优先。 | `doc/DesignCursor.md:93,136`；`gui/widget.go:272`；`gui/window.go:910` |
| `DesignDragAndDrop.md` | 源反馈仍公开 SetFallbackFeedback(bool)/SourceFallback；现实现已拆为 DragFeedback 和 SetFeedback。 | `doc/DesignDragAndDrop.md:148,215`；`gui/drag_drop.go:250` |
| `DesignFileDialog.md` | 回调返回 []string、ErrCancelled 旧契约；当前为 []FileAccess/error，并提供 RestoreAccess，取消为空结果。 | `doc/DesignFileDialog.md:70`；`gui/dialog.go`；`platform/common/dialog.go` |
| `DesignGraphicsClipping.md` | 当前选择仍写不调整布局、根从 (0,0) 铺满；实现已采用 Linux Integrated 安全内容区域布局。 | `doc/DesignGraphicsClipping.md:15`；`gui/window.go:801` |
| `DesignIcon.md` | SVG 原色配置仍写 WithOriginalColors，实际已统一为 WithRawColors。 | `doc/DesignIcon.md:75`；`icon/svg/svg.go:WithRawColors` |
| `DesignIME.md` | 仍称没有真正文本输入，并规划 Popover 借用 owner IME；当前模态暂停 owner、尚不支持 Popover 原生 IME。 | `doc/DesignIME.md:15,126`；`gui/window.go`；`gui/im_context.go`；`doc/DesignDevAutomation.md:55` |
| `DesignKeyboardNavigation.md` | 当前焦点配色节继续把 Primary 当作普通按钮柔化主题色；已单独改为按亮暗模式深浅灰。 | `doc/DesignKeyboardNavigation.md:7.3`；`theme/modern/modern.go` |
| `DesignLayout.md` | 创建 API 示例仍按 width/height 分参，并保留 OpenGL priming-swap 方案；实际 Size/Options 与 GLX 当前实现不符。 | `doc/DesignLayout.md:86,92`；`platform/platform.go`；`platform/graphics/opengl/opengl_linux.go` |
| `DesignLinuxTransparency.md` | 历史草案保留已放弃输入区域，透明查询返回 (bool,error)，并将 Color 描述为非预乘；与现契约冲突。 | `doc/DesignLinuxTransparency.md:4,43,163`；`platform/common/window.go`；`platform/graphics/brush.go` |
| `DesignMenu.md` | 明确“不增加 MenuModel”与现有具名 MenuModel 冲突；行激活 Action 后 Hide 与现有先 Hide 再 Action 相反。 | `doc/DesignMenu.md:17,112`；`gui/menu.go:16,227` |
| `DesignModernStyle.md` | Linux Integrated 仍描述只裁剪不调整内容布局，并规划已放弃的 Menu/ComboBox Overlay 迁移。 | `doc/DesignModernStyle.md:40,125`；`gui/window.go:contentBounds` |
| `DesignOverlay.md` | UI 覆盖列表仍规定类型匹配前缀、首个不匹配后重建尾部；现实现按挂载目标内 ID/类型协调并支持重排。 | `doc/DesignOverlay.md:230`；`ui/root.go:416` |
| `DesignPaint.md` | 性能断言未附可重现测量；示意 Label 的 invalidateLayout 调用与当前实现不符，并列出已经落地功能为待实施。 | `doc/DesignPaint.md:29`；`gui/label.go` |
| `DesignPlatformSettings.md` | AccentColor 返回 graphics.Color，与实际 color.Color 不符；接口未含 Language。 | `doc/DesignPlatformSettings.md:31`；`platform/common/settings.go:27` |
| `DesignPopoverMenuAppearance.md` | 首段继续承诺菜单/下拉迁移 Overlay，兜底只作矩形外壳；当前保留 Popover 并有完整 Basic 菜单外观。 | `doc/DesignPopoverMenuAppearance.md:6`；`theme/basic/basic.go`；`gui/menu.go` |
| `DesignPopup.md` | 当前路线仍写 Menu/Tooltip/ComboBox 已决定迁移 Overlay；原生工厂/API 示例也未同步 Size/Options。 | `doc/DesignPopup.md:4,41`；`platform/platform.go`；`gui/menu.go`；`widgets/drop_down.go` |
| `DesignPopupAppearance.md` | 明确废弃的历史方案；其“当前决定”也保留已放弃的输入区域和菜单 Overlay 迁移。 | `doc/DesignPopupAppearance.md:3`；`doc/DesignTransparency.md`；`gui/popover.go` |
| `DesignPopupPlacement.md` | 仍写菜单无默认最小宽度、GUI 未公开 WorkAreaAt；现在菜单 160 DIP 下限，gui.Window 已公开工作区查询。 | `doc/DesignPopupPlacement.md:30,61,260`；`gui/menu.go`；`gui/window.go` |
| `DesignProgressBar.md` | 样式声称 GUI 不包含兜底，忙等待使用 NewTimer；当前默认 Basic 已包含且按 ConnectFrame 驱动。 | `doc/DesignProgressBar.md:55,66`；`widgets/progress_bar.go:225`；`theme/basic` |
| `DesignScroll.md` | 除前言轴策略更新外，正文仍将固定 itemHeight 作为虚拟化前提，保留旧私有数据布局；实际支持变高行和宽度缓存。 | `doc/DesignScroll.md:91`；`gui/list_view.go` |
| `DesignShadow.md` | 不区分透明和不透明表面的 swap chain 选择，仍称首选 flip；当前不透明 HWND 采用 sequential/discard，resize 不再清空轮廓缓存。 | `doc/DesignShadow.md:91`；`platform/graphics/direct2d/painter_windows.go:247,592` |
| `DesignSplitView.md` | 兜底声明为 widgets/style 所有，实际该包只剩兼容转发；具体规则与实现文件已迁 Basic。 | `doc/DesignSplitView.md:5`；`theme/basic`；`widgets/style/style.go` |
| `DesignStyle.md` | 仍称 Modern 无 GUI 依赖、PopoverMenu 下一切片；当前 FromSettings 接入及菜单已实现，完整 Basic 替代最小兜底。 | `doc/DesignStyle.md:87`；`theme/modern/modern.go`；`theme/basic` |
| `DesignSwitch.md` | 短过渡仍描述 GUI Timer 及显式组合兜底；当前使用 animation.Tween 和完整 Basic。 | `doc/DesignSwitch.md:70`；`widgets/switch.go`；`theme/basic/switch.go` |
| `DesignTableView.md` | 兜底组织未同步 Basic；Modern 当前项轮廓描述未同步 FocusVisible，仅部分主题及来源章节漂移，模型/Cell填满章节仍有价值。 | `doc/DesignTableView.md:481,540`；`widgets/table_row.go`；`theme/modern/modern.go` |
| `DesignTextEditing.md` | 编辑上下文菜单仍描述 Shift+F10 用 Target 阶段 ShortcutController；当前已改为共享 ContextMenuEventController。其独立编辑包/缓存/绑定章节仍有价值。 | `doc/DesignTextEditing.md:315`；`gui/text_editor.go:216` |
| `DesignTransform.md` | §8 声称 strokeWidth/radius 保持物理像素且不随变换；实际为逻辑坐标并随 DIP/Transform 处理，违反当前绘制契约。 | `doc/DesignTransform.md:214`；`platform/graphics/software/painter.go` |
| `DesignTreeView.md` | 兜底组织未同步 Basic；Modern 当前项边框仍写获得键盘焦点就显示，现默认仅 FocusVisible 显示。 | `doc/DesignTreeView.md:582,588`；`widgets/tree_row.go`；`theme/modern/modern.go` |
| `DesignWidgetPainter.md` | 公开接口及说明仍使用 RasterScale，实际已经改名 PixelScale。 | `doc/DesignWidgetPainter.md:28`；`gui/painter.go:31` |
| `DesignWindowChrome.md` | 首段透明未实施/UI 绑定后续与已实现情况冲突；WindowControls 普通自由布局控件已改为 Window 所有；Windows版本策略已变。 | `doc/DesignWindowChrome.md:3,20,87`；`gui/window_controls.go`；`platform/windows/win32/window_windows.go` |
| `DesignWindowHitTest.md` | 同文先支持 left-held motion 发起移动，后文却称首版不支持阈值启动；当前 Chrome 确实按阈值请求 BeginMove。 | `doc/DesignWindowHitTest.md:86,105`；`gui/window_chrome.go` |
| `DesignWindowManagement.md` | 保留 ClientSize/PrepareLayout 已删除接口，macOS BeginResize 仍写 Unsupported；新位置/居中部分有效但整篇需整理。 | `doc/DesignWindowManagement.md:41,383`；`platform/common/window.go`；`gui/window.go` |
| `DesignWrapBox.md` | 称普通内容两轴无界且无自动视口宽度适配；当前 ScrollView.Direction 支持 Vertical/Horizontal 的有限交叉轴。 | `doc/DesignWrapBox.md:106`；`gui/scroll_view.go:36` |
| `GuiDesign.md` | 草案保留 Application.Style/SetStyle、无参数 NewWindow 等旧接口及树级/局部样式。 | `doc/GuiDesign.md:114`；`gui/application.go`；`gui/style.go` |
| `HiDPI.md` | X11 缩放仍指 GTK monitor 查询/监听，现实现为 GOUI_PLAT_SCALE 优先、GDK_SCALE、gtk-xft-dpi 的一次查询，动态变化尚未接入。 | `doc/HiDPI.md:107,121`；`platform/linux/x11/scale_linux.go` |
| `InputTODO.md` | 后续顺序和修饰键清单仍停留早期阶段，未同步已实现的 Key/IME/Clipboard 及独立 Win/Command/Option；Windows Pointer 不查询 GetKeyState 的描述仍属实。 | `doc/InputTODO.md:109,128`；`platform/events/input_event.go`；`platform/windows/win32/input_windows.go` |
| `MVP.md` | Root 少 RequestLayout/ConnectFrame，旧 Button AddChild、ConnectFocusChanged 与当前 Bin/ConnectFocus 不符。 | `doc/MVP.md:34,64,133`；`gui/root.go`；`gui/button.go`；`gui/window.go` |
| `Plan.md` | 混有未明确废弃的 Position 外框语义、旧焦点配色和已完成仍待实施事项；不宜直接把全部历史任务发布为当前计划。 | `doc/Plan.md:414,568,710`；`gui/window.go`；`gui/menu.go`；`gui/dialog.go` |
| `Roadmap.md` | 当前主线章节保留 ProgressBar Timer、全部Windows留1px等旧策略；已完成能力仍标为后续。 | `doc/Roadmap.md:273,643,791`；`widgets/progress_bar.go`；`platform/windows/win32/window_windows.go` |
| `Settings.md` | watch 描述为 ticker goroutine + EventLoop.Post；实现已用 Application.NewTimer；接口遗漏已接入 Language。 | `doc/Settings.md:62`；`gui/settings.go:128` |

## 暂缓收录

这些文件不删除，后续可整理为明确的历史材料、开发指南或独立未来方案后再审核。

| 文档 | 核对结论 | 证据入口 |
| --- | --- | --- |
| `DesignDrag.md` | 明确已被手势竞争取代，仅保留历史阶段，当前阅读应使用实际手势协议。 | `doc/DesignDrag.md:3`；`gui/gesture.go` |
| `DesignMenuSubmenu.md` | 未实现的后续规划仍有价值；gui/info专用字段应按Attributes复核，且计划提取的 gui/popover_placement.go 实际尚不存在。 | `doc/DesignMenuSubmenu.md:456,506`；`gui/info.go`；`gui/popover.go` |
| `DesignUI.md` | 核心 Bin/Container、ID 协调和移交已同步；仍混有附着浮层/Key/UpdateOverlays 的未确认草案，暂不作为统一现行接口文档提交。 | `doc/DesignUI.md:315`；`ui/view.go` |
| `PlatformDev.md` | 开发入口本身一致，但主要链接到尚未跟踪的 AGENTS 与待跳过设计文档，不宜单独发布。 | `doc/PlatformDev.md:6` |

### 保留的后续规划

- 子菜单的父子拥有、根菜单唯一模态登记、显示分支与键盘活动分支分离仍有价值；
  实施前重新核对 Attributes 快照、既有定位算法所在文件及按键消费边界。
- 独立滚动模型仍是后续扩展的评估方向。现阶段保留 ScrollView 的偏移所有权和
  ScrollIntoViewRequester；若扩展同步滚动、视口锚定或自动滚动，应分别审视目标可见性、
  范围更新和位置控制，不把一切职责继续叠进矩形可见请求。
- UI 的 Bin／Container、局部 ID 协调和显式移交是已实现的通用机制；附着浮层的
  Key／UpdateOverlays 等草案不据此成为已确认的公共接口。
- 圆角 clip／擦除规划需重新以当前 Linux Integrated 安全内容布局为基线，
  不恢复已经撤销的窗口输入区域或 Menu／DropDown 迁移 Overlay 路线。

## 已跟踪文档

不重复提交。以下核对结论针对当前契约及相关实现入口，不把历史验证追认为本轮验收。

| 文档 | 核对结论 | 证据入口 |
| --- | --- | --- |
| [DemoFrameworkNotes.md](DemoFrameworkNotes.md) | Demo框架反馈/已实现修正与待优化区分明确，已有跟踪，不重复提交。 | `example/demo`；`gui/scroll_view.go`；`theme/modern/modern.go` |
| [Design.md](Design.md) | 核心三层原则有效；UI身份仍称只按位置/类型、ID以后再加，已落后当前本地ID协调。 | `doc/Design.md:132`；`ui/root.go` |
| [DesignAnimation.md](DesignAnimation.md) | Root ConnectFrame/RequestPaint与纯animation包、Switch/Progress/Tab迁移匹配。 | `gui/root.go`；`animation`；`widgets/progress_bar.go` |
| [DesignBorderBox.md](DesignBorderBox.md) | 单子容器、None根缩放热区、ResizeWidth/样式安全留白契约匹配。 | `gui/border_box.go`；`ui/border_box.go` |
| [DesignBuildPackaging.md](DesignBuildPackaging.md) | TOML、build/generate/package产物与macOS沙箱签名契约匹配。 | `cmd/goui` |
| [DesignDemo.md](DesignDemo.md) | Demo分类、资源与初始Center描述匹配；已有跟踪。 | `example/demo` |
| [DesignDevAutomation.md](DesignDevAutomation.md) | dev自有ref、宿主DispatchEvent/CommitText、Popover枚举匹配。 | `gui/dev`；`gui/popover.go`；`gui/im_context.go` |
| [DesignFileAccess.md](DesignFileAccess.md) | FileAccess凭据/Close及业务错误交付匹配。 | `gui/dialog.go`；`platform/common/dialog.go`；`platform/internal/fileaccess` |
| [DesignGUIWindowChrome.md](DesignGUIWindowChrome.md) | Window所有Controls、Chrome信号、HeaderBar避让及Linux安全布局匹配。 | `gui/window_chrome.go`；`gui/header_bar.go`；`gui/window.go` |
| [DesignInteractionExtensions.md](DesignInteractionExtensions.md) | ContextMenu/TreeExpander/RowAt/DragMotion与自动滚动匹配。 | `gui/context_menu.go`；`gui/drag_motion.go`；`widgets/tree_expander.go` |
| [DesignLocalization.md](DesignLocalization.md) | 纯mlang资源、显式选语言与GUI内置文本接入匹配。 | `mlang`；`gui/text_editor.go` |
| [DesignModernPresets.md](DesignModernPresets.md) | Modern中性预设、Primary实底/单独灰色焦点及Radio实底现方案匹配。 | `theme/modern/modern.go`；`theme/modern/check_button.go` |
| [DesignScrollLayout.md](DesignScrollLayout.md) | 单轴有限交叉约束、Linear自然尺寸与剩余分配匹配。 | `gui/scroll_view.go`；`layout/linear.go` |
| [DesignSystemLanguage.md](DesignSystemLanguage.md) | 平台语言事实、GUI规范化快照与UI自动更新匹配。 | `platform/common/settings.go`；`gui/settings.go`；`ui/settings.go` |
| [DesignTabs.md](DesignTabs.md) | 仍要求显式组合widgets/style且Tab动画使用循环Timer；已迁完整Basic和ConnectFrame。 | `doc/DesignTabs.md:28,211`；`widgets/style/style.go`；`widgets/tab_bar.go` |
| [DesignTabTransfer.md](DesignTabTransfer.md) | 第31节明确废弃旧隐藏布局/反馈API，当前转移方案匹配；早期各节是历史记录。 | `widgets/tab_transfer.go`；`ui/transfer.go` |
| [DesignThemeFallback.md](DesignThemeFallback.md) | 完整Basic及Modern.FromSettings一行接入匹配。 | `theme/basic`；`theme/modern/modern.go`；`gui/style.go` |
| [DesignTransparency.md](DesignTransparency.md) | 不可变alpha选项与三后端呈现、背景归样式/清空归宿主匹配。 | `platform/common/window.go`；`gui/popover.go`；`gui/window.go` |
| [DesignWindowTaskbar.md](DesignWindowTaskbar.md) | SkipTaskbar创建偏好与三平台映射匹配；记录为本切片不含BorderBox，非当前不存在。 | `platform/common/window.go`；`gui/window.go`；`ui/window.go` |

其中 `Design.md` 的 UI 身份描述和 `DesignTabs.md` 的兜底／时间驱动描述有差异，
本轮不修改原文。读取这两处时，以当前源码及已跟踪的
[完整兜底契约](DesignThemeFallback.md)、[帧驱动契约](DesignAnimation.md) 为准。

## 其它材料与范围保护

- `theme/modern/README.md` 仍包含 Source.Render／WithOriginalColors 旧名，以及菜单
  无默认最小宽度、工作区退避未实现的表述；与 `gui/icon.go`、`icon/svg/svg.go`、
  `gui/menu.go`、`gui/popover.go` 不符，保持原样、不提交。
- 根 README、`doc/README_CN.md`、截图、各级 AGENTS、包／示例的其它使用说明，
  以及工作区原有测试、示例、二进制和图片产物，不混入本批设计文档提交。
- 跳过文件保留在工作区；无文件删除，无回滚用户内容。
- `Plan.md` 和 `Roadmap.md` 自身需要先整理历史与当前状态，所以本轮进度只记于
  本审查记录，不绕过用户的“有出入就跳过”要求回写它们。

## 验证

本机：Linux amd64，Go 1.25.8。仅文档整理，未改平台或绘制实现。

- `go test ./...`：通过，包含缓存结果；没有迁移测试、扩大 skip 或削弱断言。
  全仓沿用既有桌面条件与 PNG 输出规则，不把它表述为完全无桌面副作用的测试。
- 文档本地链接检查：检查本轮收录集合与已跟踪文件，避免链接指向未收录草案。
- 提交范围与原文件校验：每笔只暂存指定 Markdown，核对跳过文件及 README 未变化。
- `git diff --check`／暂存差异检查：提交前执行。
- 未执行新的独立窗口、远端桌面、交叉编译或 race 检查；本轮没有实现变更，
  不借文档整理扩大已有功能的跨平台验收结论。
