# GOUI v0.5.0 发布说明

比较基线为 v0.4.0。本轮计划同步发布 Linux amd64/X11、Windows amd64、macOS
arm64 Demo，版本为 0.5.0、build 1。当前处于发布准备阶段；正式候选提交、最终
构建、桌面验收、签名公证及下载结果以 Release 附件 `BUILD-INFO.md` 和
[Plan](Plan.md) 的本轮记录为准，不将此前切片验收视为正式附件已通过。

## 主要变化

- **声明式局部更新。** `ui.Build` 和自定义 ComposeView 在同步构建期间自动
  追踪 `ui.State.Get()`／`ui.Read()`，依赖变化只更新相应子树；属性绑定直接更新
  对应 Widget 属性。`ui.Modify(view)` 为返回 `ui.View` 的组合提供公共修饰，
  不增加 Widget 或布局边界。
- **格式化与可选择 Label。** 使用 `\{format\}` 前缀开启命名样式范围，支持
  嵌套、颜色、字体和字重等文字格式。`Selectable(true)` 支持单个 Label 的
  鼠标／键盘选择、纯文本复制及选区绑定；普通文案和编辑器正文保持原样。
- **模态、Dialog 与 Toast。** `gui.ModalScope` 隔离后台的内容输入、焦点、
  Tab、快捷键、IME 和拖放；声明式通过 `OverlayItem(...).Modal(true)` 接入。
  `widgets.Dialog` 和 `widgets.Toast` 可组合到普通 Overlay；ToastHandle 支持
  显示、替换、关闭及超时，Dialog 的关闭请求与业务结果由应用处理。
- **按钮分组。** 新增 ButtonGroup 连排容器；MenuButtonGroup 改为局部水平
  容器，后代菜单按钮自动协作。CheckButtonGroup 的声明式后代自动关联最近组，
  保留显式对象关联及独立选项，不再要求重复填写字符串组名。
- **绘图与外观。** DrawingArea 显式生成并缓存 Software 绘制内容，同尺寸
  重画复用 `graphics.Image.Update`。Painter 增加通用 ClearPath，三后端支持
  擦除路径区域；Linux Integrated 内容铺满客户区，并保留边缘缩放命中。
  统一 Widget 状态与显式样式绘制辅助，Box 可绘制自己的背景和边框。
- **布局与数据视图。** 容器和文本控件支持四边 Padding；TableView 支持列
  显隐和按剩余宽度伸展，TreeView 完善行激活语义。修正 ScrollView 重测后
  视口几何不同步、窗口缩放后留白异常等问题。
- **桌面与系统集成。** Window.Show 请求显示、前置和激活已有窗口；初始居中
  在工作区能力缺失时保留系统默认位置。原生窗口被销毁时及时停止绘制并清理
  宿主。文件对话框显式关联所属窗口，异步等待期间丢弃后台输入，其他窗口
  继续工作；Linux 优先通过 D-Bus 使用 XDG Desktop Portal 文件选择器。
- **Demo 与验证入口。** Demo 按功能持有稳定状态和生命周期，展示局部更新、
  按钮分组、格式化／可选择文本、Dialog／Toast、DrawingArea 和数据列配置。
  修正 DrawingArea 样例提前重绘时使用旧版本的问题，重新生成立即绘制当前状态。
  新增三平台原生输入诊断例程，便于记录触摸板等真实硬件的消息；这不代表
  已完成触摸板或触摸手势适配。

同时修正 Software 栅格与像素对齐、SVG 兼容性、圆角切点缺口，以及遮罩显示时
HeaderBar 高度查询导致窗口控制按钮跳动和关闭图标的像素对齐问题。

## 升级影响

### State 与局部构建

`State.Set`／`Update` 不再自动重建全应用。将需要随状态变化的声明放入
`ui.Build` 或自定义 ComposeView.Build，并在同步构建期间读取状态：

```go
func counterText(count *ui.State[int]) ui.View {
    return ui.Build(func() ui.View {
        return ui.Label(fmt.Sprintf("Count: %d", count.Get()))
    })
}

// At the composition site:
ui.Modify(counterText(&count)).MainWeight(1)
```

State 由宿主或稳定的功能对象持有。提前执行 Get 再捕获其结果只得到普通 Go
快照；事件回调和异步任务中的 Get 不建立构建依赖。需要不追踪的读取时使用
Peek；外部 Observable 仍使用 `.Observe(source)`，自动与显式依赖可以组合。
确需全应用更新时调用 `App.RequestUpdate()`。完整契约见
[声明式更新设计](DesignUIUpdates.md)。

### Padding

GUI 和 layout 的 Padding 字段／setter 改用 `geometry.Insets`：

```go
box.SetPadding(geometry.UniformInsets(12))
box.SetPadding(geometry.Insets{Left: 16, Top: 8, Right: 16, Bottom: 8})
```

UI 保留 `.Padding(12)` 表示四边相同，增加
`.Paddings(geometry.Insets{...})`。Padding 属于布局属性，不是样式字段。
自定义控件应同步测量、排列、基线及 Snapshot 中的内边距表达。

### 按钮组

MenuButtonGroup 使用普通子声明，成员不再调用 `.Group("id")`：

```go
ui.MenuButtonGroup(
    ui.MenuButton("File").Menu(/* items */),
    ui.MenuButton("Edit").Menu(/* items */),
).Spacing(8)
```

CheckButtonGroup 不再接收字符串 ID 参数，后代默认关联最近组：

```go
wui.CheckButtonGroup(ui.HBox(
    wui.CheckButton("A"),
    wui.CheckButton("B"),
))
```

这里 wui 表示 `github.com/golang-gui/goui/widgets/ui`。需要跨区域关联时，向
CheckButton 的 Group 传显式 `*widgets.CheckButtonGroup`；`.Group(nil)` 声明
独立成员。ButtonGroup 仅改变连排布局和外观，不增加互斥选择行为。

### 文件对话框

GUI 的 OpenFile／OpenDirectory／SaveFile 首参为所属 `gui.Window`，nil 表示
独立显示；UI 首参为 windowID，空字符串表示独立显示，并返回调度受理错误。
非空 ID 不存在时通过结果回调报告，不自动降级为无所属窗口。

```go
err := app.FileDialog().OpenFile("main", ui.DialogOptions{},
    func(files []ui.FileAccess, err error) {
        // Handle cancellation, errors and selected files; close each FileAccess
        // after all file operations finish.
    })
```

同一窗口重复打开通过结果回调报告 ErrDialogBusy。用户取消仍是空结果和 nil error；业务与
系统错误交给调用方。GUI 正在运行时，已受理请求的结果在 GUI 线程回调。
打开和关闭对话框不更改 Widget.Enabled，也不在关闭后补执行积压输入。

### 窗口最小尺寸与显示

窗口的原生最小尺寸只来自显式 SetMinSize／声明式窗口最小尺寸设置，不再把根
Widget 的自然测量结果隐式转换为窗口限制。Widget.MinSize 继续约束布局；需要
限制用户缩小窗口的应用须明确声明窗口最小尺寸，设置零值可清除该应用限制。

再次调用 Window.Show 会显示、请求取消最小化并前置激活已有窗口，同时保留
最大化／全屏状态；实际结果由窗口系统决定。显式状态请求继续使用 RequestState。

### 自定义控件与绘制后端

- Widget 新增 State 查询，状态变化复用 StyleChanged 通知；嵌入 WidgetBase
  的实现可复用公共状态。自定义绘制按需使用 WidgetStyle／PaintWidgetStyle 或
  分离的背景、边框辅助，不要求调用 WidgetBase.Paint，也不统一插入额外绘制指令。
- 样式状态改为可组合标志；按实际状态组合匹配，Disabled 优先。自行保存
  状态整数或实现 StyleSheet 的使用方需复核，不依赖旧枚举的连续数值。
- 自定义 Painter 实现需增加 ClearPath；自定义 Root 实现需接入
  ConnectAfterPaint。回调中的 Painter 为借用对象，限制与 Widget.Paint 相同。
- `graphics.Image.Update` 可在帧内首次非空 DrawImage 之前调用；该图片被
  绘制后到 End 之前禁止更新。不同图片独立判断，下一帧重新允许更新。

## 范围与限制

Linux 正式目标仍为 amd64/X11；原生 Wayland 和 Linux arm64 未在本轮完成。
Portal 需要可用的 session bus、FileChooser 服务及桌面后端；能力不可用时按
既定规则回退文件选择工具，原生模态行为受桌面服务影响。Settings 与输入法
未迁移到 D-Bus。GTK 仍用于现有系统设置接入，不宣称完全没有系统库依赖。

窗口前置与定位是请求，系统可限制激活或调整位置。Label 选择限于单控件，
不包含跨控件选择、触摸手柄或富文本剪贴板。DrawingArea 同步在 GUI 线程
绘制，应用决定重画频率；不是异步 Canvas 或通用 GPU 绘图替代品。
子菜单、原生 Popover IME、系统通知和触摸支持仍不属于本轮新增能力。

SourceView 是独立项目，不包含在 GOUI 模块或 Demo 中。Windows EXE 不宣称
经过 Authenticode 签名。macOS Demo 的 Developer ID 签名、公证票据和
Gatekeeper 状态须以本轮最终 ZIP 的验证为准。

## English summary

- UI compositions track State reads automatically. Property bindings and subtree
  updates replace implicit application-wide State updates; `ui.Modify` configures
  interface-typed composition results without introducing layout widgets.
- Add formatted and selectable Labels, modal input scopes, Dialog/Toast widgets,
  connected button groups, cached DrawingArea painting, per-edge padding, and
  TableView column visibility/expansion.
- Improve full-client Linux Integrated rendering, native window destruction,
  owner-modal asynchronous file pickers, and Linux desktop portal integration.
- The refreshed Demo demonstrates stable feature ownership and local state
  updates. Review the migration notes for State, padding, button groups, file
  dialog ownership, and custom widget/painter implementations before upgrading.
- Demo 0.5.0 build 1 is planned for Windows amd64, macOS arm64 and Linux amd64/X11.
  Final artifact verification and macOS notarization are tracked separately;
  release preparation does not imply those steps have completed.
