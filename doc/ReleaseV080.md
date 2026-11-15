# GOUI v0.8.0 发布准备

状态：**尚未发布**。比较基线为 v0.7.0；Demo 源码配置为 **0.8.0 / build 1**。
本轮先完成 pre-release 分支的版本、Demo 和文档更新，由维护者发起合入 main 的 PR。
合并后按 [发布流程](Release.md) 冻结完整候选 SHA，再重新生成并验收正式附件。
本文件不是当前版本已发布、已公证或全部平台验收通过的声明。

## 主要变化

- 新增 ListRow 及声明式入口，组合图标、标题、多行说明与独立尾部控件；
  Basic 保持清楚的朴素边界，Modern 使用中性表面与主次文字。
- TableView 区分余宽伸展、空间不足时收缩与用户手动列宽。表格自有样式 Part
  控制表头、滚动槽、内部网格、实际单元格四边、含滚动槽的紧包围轮廓和视口外框；
  修正小数滚动范围末端、表头分隔线及网格接缝。
- 树提供 RowStyle、内容装饰层级、RowInsets 与相邻选中信息。背景与命中范围
  适配视口，内容独立横滚；连续选区接合，单行边框仍独立。稳定树持有拖拽源时，
  虚拟化源行回收不再等同于所属树卸载。
- 控制器使用类型化输入回调，在按下时立即报告连续次数。TextInput／TextView
  支持三击全选／逻辑行选择，并可继续整行拖选；文本光标闲置后停止闪烁。
- 内置菜单、DropDown、Tooltip 按所属 Surface 持续复用同一 Popup 与 Painter，
  普通 Popover 保持独立。原生拖拽预览创建失败可降级为无预览拖放，数据与会话
  错误仍正常报告。
- X11 通过 XSETTINGS 读取运行期外观设置，不接入 GTK 事件循环。修正失活窗口
  快速拖动、待布局时标题区域拖动、源窗口关闭后的拖拽会话清理，以及 Win32
  鼠标 Alt 状态、进度动画、圆形进度接缝和 Switch 像素对齐。
- 共享 Surface 呈现内核、激活／退出及 Android／iOS 原生 UI 线程、宿主与绘制
  生命周期继续收敛。完整移动触摸、IME、安全区、Popup 和物理设备交付仍有待办；
  不附移动安装包，不把桌面 Demo 当作已完成移动适配的示例。

## Demo 更新

保留九类导航与基础页原顺序，ListRow 仍位于基础页最下方。数据页增加五种边框
外观，使用明确的列最小宽度、树行样式和视口内缩；文本页补充三击与整行拖选说明。
八种语言同步更新，设置、模型、用户文本、备注、选区与手动列宽继续按原归属保存。
操作步骤见 [Demo 使用说明](../example/demo/README.md#v080-展示)。

公开下载仍为 v0.7.0。v0.8.0 发布并下载复核后，再更新双语 README 的下载入口、
实际版本与最终截图；不提前链接尚不存在的 v0.8.0 附件，也不将旧截图改标为新版。

<a id="api-migration"></a>

## API 迁移

本版本仍处于 0.x 阶段，升级前请检查直接使用的控制器、宿主接口、快照和自定义样式。

### 控制器回调

| 旧用法 | v0.8.0 |
| --- | --- |
| Click.ConnectPressed(ctx, bool)，只更新外观 | ConnectPressedChanged(func(bool)) |
| 真实按下通知 | ConnectPressed(func(EventContext, ClickInfo))，读取 Count／Position／Buttons／Modifiers |
| ConnectClick(ctx, count) 或旧 ConnectClicked(ctx) | 统一 ConnectClicked(func(EventContext, ClickInfo)) |
| 释放／中断混用按压状态 | ConnectReleased 接收真实抬起；ConnectCanceled 接收 GestureCancelReason |
| Click.CancelPointer(ctx) | CancelPointer() |
| Motion.ConnectMotion(func(MotionInfo)) | func(EventContext, MotionInfo)，信息含 Buttons |
| Drag.Begin／Update／End(point, modifiers) | func(EventContext, DragInfo) |
| Drag.ConnectCancel(func()) | ConnectCanceled(func(GestureCancelReason)) |
| Wheel.ConnectScroll(ctx, events.WheelEvent) | func(EventContext, WheelInfo) |
| ContextMenu.ConnectRequest(ctx)、ui.OnContextMenu(ctx) | func(EventContext, ContextMenuInfo) |

Info 中的位置是控件局部 DIP；原始平台事件的位置仍为宿主局部 DIP。
菜单用 HasPosition 区分指针与键盘请求。EventContext 负责本次同步传播控制，
不保存到异步回调。Count 在 Down 时确定，完成后的第一击不会等待未来双击，
也不会被后续多击撤回；取消不伪造释放坐标。Button 的无参数业务 Clicked 与
ui.OnClick 不变。完整约束及边界见 [控制器输入回调](DesignInputCallbacks.md)。

### 应用、宿主与快照

- GUI Application.Run 现在返回 error。推荐在 ConnectActivate 中创建 GUI 对象，
  在 ConnectShutdown 清理 UI 资源；声明式 ui.Run 统一处理激活流程。桌面已有的
  Run 前创建仍支持迁移，不能据此推断移动端也允许提前创建原生 GUI 资源。
- Window 继续管理标题、位置、装饰和窗口状态；Surface 负责共享呈现与输入。
  移动端的 Application.NewSurface／ui.Surface 采用系统提供的宿主，不模拟可自由管理的窗口。
- 直接接入 platform 的代码从基础 Window 查询可选 WindowManagement；旧
  DesktopWindow 保留为迁移别名。应用普通控件无须自行查询平台能力。
- ApplicationInfo 和其 JSON 顶层使用 Surfaces／surfaces，取代 Windows／windows；
  WindowInfo 保留为 SurfaceInfo 的别名。直接解析快照的工具应同步迁移。
- 自行实现 gui.Root 的宿主需实现新增 ConnectBeforePaint；此通知用于布局完成后
  准备派生绘制数据，不在回调中绘图、改控件树或同步重建 UI。

详细生命周期及适用范围见 [呈现宿主设计](DesignPresentationHosts.md)。

### 表格、树与浮层

- TableColumn.Width 是基础宽度；不足时会收缩至 MinWidth，伸展仍由 Expand 控制。
  自动分配不改写基础宽度、不发送用户 OnResize。手动调整会保存受影响列的分配，
  声明式 Width／OnResize 应继续使用稳定列 ID 和用户保存的值。
  相邻列调整与普通重建保留当前分配；真正改变视口面积或列几何配置后恢复全表
  自动分配，不保证实际列宽始终等于此前手动调整的像素宽度。
- 旧全局 table-header／table-header-text／table-row／table-grid 样式改为表格自己的
  Name 与 header／header-text／row／grid-horizontal、grid-vertical 等 Part。
  自定义完整名称要显式提供所有部件及状态；不隐式继承 table-view。
- 内部网格与数据四边分开：grid-horizontal／vertical 不再同时控制末行／末列边线。
  grid-top／bottom／left／right 只沿实际单元格绘制；outline-horizontal／vertical
  包含固定滚动槽；outer-horizontal／vertical 包围完整分配视口。
- TreeView 的行背景与命中区域跟随视口，横滚只移动内容；普通 ListView 继续使用
  默认自然行宽。虚拟化横向范围依据已测量行估计，后续测到更宽的行仍可扩大范围。
- 虚拟化树拖拽源由稳定树持有，在 Prepare 中解析节点与数据；不要将每个会回收的
  内容 Widget 当作必须存活至原生拖放结束的源。真正卸载所属树仍请求取消。
- NewTransientPopover 表达共享宿主借用；普通 NewPopover 不限实例且保持独立。
  预览降级只处理可选视觉创建失败，不能把非法参数或业务数据错误当作成功。

详见 [TableView](DesignTableView.md)、[TreeView](DesignTreeView.md)、
[Popup](DesignPopup.md) 与 [拖放契约](DesignDragAndDrop.md)。

## 发布门槛与当前状态

正式分发目标维持 Windows amd64、Linux amd64/X11、macOS arm64。Linux arm64
的 pipe／poll 编译修正已合入，但完整 ARM64 桌面支持仍需其专项验收，不扩大发布列表。

- [ ] 版本、Demo、双语 README 与本记录的准备提交完成并合入 main。
- [ ] 在 main 冻结完整候选 SHA，所有构建工具、源码与正式附件来源一致。
- [ ] 候选独立快照非缓存全仓测试、相关 race／vet、三目标编译通过，分别登记跳过和限制。
- [ ] 三平台最终分发程序完成启动、基础输入、滚动、菜单／下拉、调整窗口及本轮场景验收。
- [ ] macOS Developer ID、Hardened Runtime、安全时间戳、公证 Accepted、装订及最终 ZIP 复验。
- [ ] BUILD-INFO、SHA256SUMS 和双语 Release Notes 就绪。
- [ ] annotated tag、Draft 附件、下载原件比较通过；随后公开、设置 Latest 并复核公开下载。
- [ ] Go proxy 与独立消费项目验证；双语 README 下载入口和本记录按实际结果收尾。

本轮准备分支的验证数据已记入 [Plan](Plan.md)，不替代合入 main 后冻结候选和正式
附件的验收。尚无本版本公证 ID、正式附件摘要或公开 Release URL。

## English summary

- Add composable ListRow settings and action rows.
- Separate table stretching, shrinking and manual widths; make headers, scrollbars and each border scope explicit style parts.
- Add declarative tree row appearance, viewport-sized interaction regions and continuous selection groups.
- Unify typed input callbacks with immediate consecutive-press counts; support triple-click logical-line selection and dragging.
- Reuse popup/painter resources across built-in transient presenters; continue native dragging when optional preview creation fails.
- Correct X11 live appearance and window dragging, Win32 modifiers, progress animation and Switch geometry.
- Advance shared Surface hosting and Android/iOS lifecycle foundations without claiming complete mobile support.
- Prepare Demo 0.8.0 / build 1. This version is not yet published; final provenance and validation will be recorded after the candidate is merged and frozen.
