# GOUI v0.8.0 发布说明

[v0.8.0 已正式发布](https://github.com/golang-gui/goui/releases/tag/v0.8.0)，Demo 为 **0.8.0 / build 1**。
标签与三平台附件冻结于 main 提交 `bc47eb006836fce9fed3cfe695f6345995513199`；发布后文档收尾不移动标签。
以 v0.7.0 为比较基线，完整来源、摘要与实际验收边界见 Release 的
[BUILD-INFO.md](https://github.com/golang-gui/goui/releases/download/v0.8.0/BUILD-INFO.md) 和
[SHA256SUMS](https://github.com/golang-gui/goui/releases/download/v0.8.0/SHA256SUMS)。

```sh
go get github.com/golang-gui/goui@v0.8.0
```

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

公开下载与双语 README 已更新为 v0.8.0，正式附件经 Draft 和匿名公开下载逐字节
比较通过。现有 README 截图继续明确标注 v0.6.0，不将旧图改标为新版。
Windows EXE 未做 Authenticode 签名；Linux/X11 下载后需赋予执行权限；macOS
Apple Silicon 解压打开 `GOUI Demo.app`，已使用 Developer ID、Hardened Runtime
及安全时间戳签名。公证 `4147ea42-173d-4fb8-b9bf-ddca94023432` 为 Accepted，
最终 ZIP 解压后的签名、票据、Gatekeeper 与实际操作通过。

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

## 发布与验证

正式分发目标为 Windows amd64、Linux amd64/X11、macOS arm64。Linux arm64
的 pipe／poll 编译修正已合入，但完整 ARM64 桌面验收未完成，不附该架构附件。

- [x] 准备提交经 PR #113 合入 main，再冻结候选、从干净归档构建同源工具和附件。
- [x] Go1.25.8／CGO_ENABLED=0 全仓非缓存 **5816 通过／11 既有跳过／0 失败**；
  Demo 和控件相关 race／vet 通过。三目标桌面包构建通过，两个移动目录例外保留。
- [x] 最终分发程序经正常输入核对启动、按钮、文本提交、三击整行拖选、五种表格
  外观、备注与选区保持、树连续选区、末行完整显示、菜单／下拉、窗口调整和关闭。
  Linux Xvnc／Xfwm4／OpenGL 1x，Windows10／Direct2D 1x；macOS15.7.7／OpenGL
  使用默认原生缩放、运行最终 ZIP 解压件，不把 VNC 尺寸当作 Retina 像素专项验收。
- [x] Mac 正式签名、公证 Accepted、装订、Gatekeeper 和最终 ZIP 解压复验通过。
- [x] 五份附件的服务端 digest、Draft 下载校验及原件比较通过；正式发布并设为 Latest，
  随后匿名公开下载五份附件，校验与逐字节比较全部通过。
- [x] Go proxy 的 Origin SHA 与候选一致；1067 份公开模块 Go 文件与冻结归档一致。
  Go1.24.13 独立 README 计数器消费项目无 replace／workspace，三个桌面目标编译通过。
- [x] 双语 README、Demo 使用说明及本记录更新；发布后只提交文档，不改标签或附件。

桌面 `go build ./...` 仍因既有 `mobile_start`／`mobile_surface` 没有桌面 main 失败，
排除移动专用入口及无生产源码的测试目录后，211／204／202 个桌面包分别构建通过。
原工作区全仓测试受已有未跟踪旧例程／测试编译失败阻塞，诊断集合与准备阶段相同；
未跟踪文件没有进入发布归档，也没有删除或弱化其断言。

Software／2x、Windows OpenGL、混合 DPI、真实 Retina 像素、全量 IME／DnD、
Linux ARM64 桌面和物理移动设备未在最终附件重新验收。发布准备与正式附件验证
分别记录在 [Plan](Plan.md)；完整命令、环境和限制以 BUILD-INFO 为准。

## English summary

- Add composable ListRow settings and action rows.
- Separate table stretching, shrinking and manual widths; make headers, scrollbars and each border scope explicit style parts.
- Add declarative tree row appearance, viewport-sized interaction regions and continuous selection groups.
- Unify typed input callbacks with immediate consecutive-press counts; support triple-click logical-line selection and dragging.
- Reuse popup/painter resources across built-in transient presenters; continue native dragging when optional preview creation fails.
- Correct X11 live appearance and window dragging, Win32 modifiers, progress animation and Switch geometry.
- Advance shared Surface hosting and Android/iOS lifecycle foundations without claiming complete mobile support.
- Release Demo 0.8.0 / build 1 for Windows amd64, Linux amd64/X11 and notarized macOS arm64 from one frozen main commit. See BUILD-INFO.md for provenance, native validation and retained build exceptions.
