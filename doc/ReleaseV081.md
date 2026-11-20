# GOUI v0.8.1 发布说明

状态：**2026-11-20 已发布**。Demo **0.8.1 / build 1** 的三平台附件与校验文件见
[正式 Release](https://github.com/golang-gui/goui/releases/tag/v0.8.1)。

本版本以修复 v0.8.0 的桌面交互、弹层和绘制问题为主。公共 API 的调整用于明确
资源归属、修正输入边界及补齐应用无法自行处理的布局入口；升级时请检查下方迁移项。

## 主要修复

- **弹层复用与定位：** 菜单、下拉框和 Tooltip 在同一 Surface 上共享实际 Popover、
  原生 Popup 和 Painter。显示前完成尺寸与位置确认，避免交替打开不同大小内容时
  暴露上次内容或位置跳动；修正隐藏期间失效、锚点切换与屏幕边缘退避。
  普通 Popover 继续独立控制，不参与共享槽的互斥。
- **全屏与菜单：** 修正 Windows 的全屏进入、退出和原状态恢复，以及 X11 全屏时
  菜单使用错误工作区造成的退避。macOS 全屏请求按原生异步完成状态处理。
- **滚动与表格：** 后来的显式滚动覆盖较早的待执行 Reveal，避免视口被旧请求拉回。
  内置滚动条可分别设置条厚、滑块厚度、内容间距和显示策略；表格框线随真实槽位
  布局，交汇角支持独立样式。默认仍为 8 DIP 条厚、8 DIP 滑块、0 DIP 间距。
- **分隔条与焦点：** SplitView 的光标、悬停与拖动使用同一抓取范围，分隔线与抓取
  厚度独立设置，线条支持圆角。标签主体与关闭按钮显示可见的键盘焦点。
- **文本与设置行：** 聚焦的自定义文本控件可在输入能力改变后刷新输入法绑定，
  无须切换焦点。Tooltip 支持 Label 的 FormatText。ListRowGroup 修正连接行的
  边线与外侧圆角，并可按当前宽度统一直接子行的自然高度。
- **绘制质量：** 修正小数物理像素位置的裁剪边缘与 Software 图片缩放采样；
  NanoVG v0.0.16 改进填充边界覆盖率和曲线细分，减少圆环与旋转圆弧的锯齿。

## Demo 更新

保留九页导航，基础页的设置区域仍位于最下方。增加“统一行高”开关；数据页增加
默认、增加留白、隐藏三种滚动条布局，切换时保留模型、选区和用户列宽。
Tooltip 示例展示粗体与斜体；窗口页提供全屏请求，并可通过“请求普通状态”退出。
新文案随八种语言切换。菜单、文本、分割与动画继续通过原有页面体验。

详细操作见 [Demo 使用说明](../example/demo/README.md#v081-修复展示)。双语 README
下载入口已更新至本次正式附件，历史截图与资源测量继续保留原版本标注。

<a id="api-migration"></a>

## 修复所需的 API 迁移

### Popover 的所有权与当次显示目标

| v0.8.0 用法 | v0.8.1 |
| --- | --- |
| `gui.NewTransientPopover(anchor)` | 使用 `gui.NewSharedPopover(anchor)`，返回轻量 `SharedPopover` 客户端。它不实现 Root，不拥有共享宿主。 |
| `Surface.Popovers() []gui.Popover` | 返回 `[]gui.PopoverTarget`，用于查询和输入派发，不提供宿主配置或销毁操作。 |
| 修改锚点时重新构造普通 Popover | 可用 `Popover.SetAnchor(widget)`；同 Surface 复用原生资源，跨 Surface 在下次 Show 时迁移资源。 |

`PopoverTarget` 只对应一次可见显示，关闭后即失效；同一弹层重开也须重新枚举。
普通 Popover 及 SharedPopover 的 `SetAnchor` 会结束旧显示，不自动重新显示。
`SharedPopover.Destroy` 只销毁自身客户端；共享资源由 Surface 最终释放。
借用的 `Widget.Root()` 不能作为应用拥有的弹层来销毁。

### SplitView 自定义样式

旧的 `split-handle` 样式改为所属 SplitView 的 `separator` Part：

```go
style.Name("split-view").Part("separator").
    ForegroundColor(color.RGBA{R: 128, G: 128, B: 128, A: 255}).
    BorderWidth(1)
```

使用自定义 StyleName 时替换示例中的 `split-view`。常态 `BorderWidth` 控制实际布局线宽，
`Radius` / `CornerRadius` 控制线条端部；`HandleSize` 只控制抓取厚度，默认 5 DIP，
不增加面板间距。默认 1 DIP 线条的抓取范围覆盖两侧面板各 2 DIP。

抓取范围内 SplitView 优先接收悬停和拖动。相邻滚动条、按钮或文本拖选需要应用
显式内缩避让；SplitView 不自动添加 Padding。零圆角细线仍可与嵌套分隔线相接。

### 自定义平台适配

直接实现 `platform.Popup` 的适配需补充 `ClientSize` 和 `Repaint`：前者查询实际
DIP／物理尺寸，后者在原生绘制上下文中处理首帧，不等同于合成器已经显示。
`WindowManagement.WorkAreaAt` 的签名不变：普通窗口返回桌面工作区，实际全屏时
返回该窗口占用显示器上的可用全屏区域，并保留原生内容／安全区限制。
仅使用 GUI 或声明式控件的应用无需实现这些平台方法。

## 修复相关的补充入口

- `ScrollView`、`TableView` 的 `HScrollBarOptions` / `VScrollBarOptions` 及对应 setter；
  声明式同名方法接收 `ui.ScrollBarOptions`。可配置 `Thickness`、`ThumbThickness`、
  `Spacing` 和 `ScrollBarAuto` / `ScrollBarAlways` / `ScrollBarHidden`。
  隐藏条不禁用滚动；外部滚动条关联协议不属于本版本。
- 独立 `ScrollBar` 提供 `SetThickness` / `SetThumbThickness`；颜色和圆角仍由
  `trough` / `thumb` Part 控制，滚动容器的交汇角使用 `corner` Part。
- `ListRowGroup.SetUniformRows(bool)`，声明式 `.UniformRows(true)`。默认 false；
  隐藏行不参与最高自然高度计算，关闭后恢复各行自然高度，不改写行尺寸属性。
- `gui.RefreshInputMethod(widget)`：自定义 IMClient 在聚焦期间改变 IMContext 后，
  在 GUI 线程调用。普通 TextInput / TextView 已由框架处理。
- Tooltip 使用与 Label 相同的 FormatText 前缀及命名样式语法；普通文本用法不变。

## English release notes

v0.8.1 is a repair release for popup reuse, desktop input, layout and rendering.
API adjustments clarify ownership and expose controls needed to correct these behaviors.
Released on **2026-11-20**, including Demo **0.8.1 / build 1** for all three targets.

- Reuse one actual Popover, native popup and painter per Surface for built-in
  menus, dropdowns and tooltips. Stabilize geometry before showing, invalidate
  retained contents, and correct hidden updates, anchor changes and edge placement.
  Ordinary Popovers remain independently controlled.
- Fix Windows fullscreen transitions and restoration, X11 fullscreen menu work
  areas, and asynchronous macOS fullscreen requests.
- Let explicit scrolling supersede an older pending reveal. Expose per-axis
  scrollbar geometry and visibility on ScrollView and TableView, with matching
  table frames and an independent corner style. Preserve the default 8 DIP
  track, 8 DIP thumb and zero spacing.
- Align SplitView hover, cursor and drag bounds; separate separator styling from
  grab thickness and support rounded separators. Show keyboard focus on tab
  bodies and close buttons.
- Refresh a focused custom text widget's input method without changing focus.
  Support Label FormatText in tooltips and connected settings rows with optional
  uniform natural heights.
- Preserve fractional-pixel drawing edges, correct Software image scaling, and
  improve ring and rotated-arc coverage with NanoVG v0.0.16.

Migration from v0.8.0:

- Replace `NewTransientPopover` with `NewSharedPopover`. The returned client is
  not a Root and does not own the shared host. `Surface.Popovers` now returns
  borrowed `PopoverTarget` values that expire when the current showing closes.
- Move custom `split-handle` styles to the SplitView's `separator` Part.
  Normal `BorderWidth` reserves the line; `HandleSize` controls the 5 DIP default grab
  band. Applications must inset neighboring interactive content when necessary.
- Custom `platform.Popup` implementations must provide `ClientSize` and
  `Repaint`. `WorkAreaAt` observes the window's actual fullscreen presentation.
- Optional repair APIs include `HScrollBarOptions` / `VScrollBarOptions`,
  `ListRowGroup.UniformRows` and `RefreshInputMethod`; existing defaults remain.

The demo retains nine pages and adds equal-row-height, scrollbar-layout,
formatted-tooltip and fullscreen examples in their existing pages. All new
labels support the existing eight languages.

## 下载与验收范围 / Downloads and validation scope

从 [v0.8.1 Release](https://github.com/golang-gui/goui/releases/tag/v0.8.1) 下载 Windows x64
EXE、Linux x64/X11 程序或 macOS Apple Silicon ZIP。Linux 首次启动前执行
`chmod +x goui-demo-v0.8.1-linux-amd64`；macOS 解压后打开 `GOUI Demo.app`。
macOS 已完成 Developer ID 签名与 Apple 公证；Windows EXE 未做 Authenticode 签名。
库升级使用 `go get github.com/golang-gui/goui@v0.8.1`。

正式程序已在 Linux/X11 OpenGL、Windows Direct2D、macOS OpenGL 的 1x 环境完成
输入、滚动、弹层、全屏和窗口调整操作验收。不据此承诺所有后端、真实 Retina／混合
DPI 或移动设备均已验收。来源、摘要、详细范围和限制见 Release 的 BUILD-INFO.md；
SHA256SUMS 用于下载校验。

Download the Windows x64 EXE, Linux x64/X11 binary or macOS Apple Silicon ZIP from
the release above. Mark the Linux binary executable with the command above; on macOS,
extract and open `GOUI Demo.app`. The macOS app is Developer ID signed and Apple-notarized;
the Windows EXE is not Authenticode-signed. Upgrade the module with the `go get` command above.

Final programs passed interactive checks on Linux/X11 OpenGL, Windows Direct2D and
macOS OpenGL at 1x. This does not cover every backend, physical Retina/mixed-DPI or mobile
devices. See the release's BUILD-INFO.md for provenance, validation scope and limitations,
and SHA256SUMS for download integrity.
