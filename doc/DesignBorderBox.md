# BorderBox 单子边框容器

## 定位与边界

BorderBox 是 GUI 单子容器，不是 Window 或 Popup 的新类别。装饰与布局始终
可用；只有作为 `WindowChromeNone` 窗口的根 Widget 时提供自动缩放边缘。
嵌套、Native/Integrated 与 Popover 中只做布局和绘制，不改变宿主交互。
移动由 HeaderBar 的 Caption 查询提供。平台层、Painter、事件传播和手势机制
均不增加 BorderBox 特例，原生透明绘制仍由创建选项决定。验收发现并修正 macOS
既有 None 状态误报，范围见下节；这不是 BorderBox 专属的平台分支。

## 公共接口

```go
NewBorderBox() *BorderBox
Child() Widget
SetChild(Widget)
Resizable() bool
SetResizable(bool)
ResizeWidth() float32
SetResizeWidth(float32)
```

实现 `gui.Bin`。默认 Resizable=true、ResizeWidth=6 DIP、不取得键盘焦点。
ResizeWidth 同时作为命中宽度和最小布局留白；负数/NaN/Inf 归零，零关闭边缘。
Resizable 关闭不会收回留白，避免状态切换导致内容跳动。

默认 StyleName 为 `border-box`，Basic/Modern **透明、无描边、零圆角**。
自定义 StyleName 使用 BackgroundColor、BorderColor、BorderWidth、Radius；
不隐式给子控件样式，不添加阴影或 Hover 外观。禁用使用既有 Disabled 样式。

## 布局与绘制

内容留白为 `max(ResizeWidth, BorderWidth, Radius*(1-1/sqrt(2))+1 DIP)`；
无圆角不附加抗锯齿余量。此矩形落在圆角曲线内部，无需新圆角 clip。
Measure 将 inset 从父/自身约束中扣除，保留无界哨兵，添加外框尺寸并调整 baseline。
Arrange 填充内矩形，通过既有测量路径尊重约束；小尺寸内区域收缩到零。
Paint 只绘制自身，子树遍历和结构矩形裁剪由 GUI 内核完成；子控件自身的
Transform/SetClipRect 不能扩大结构裁剪范围。不同背景时会看到预留边缘，这是设计行为。

透明圆角需要 `WindowOptions.Transparent=true` **以及透明的 Window 背景样式**。
只让 BorderBox 透明不等于清除宿主已绘制的背景。透明角仍在原生矩形输入区域内，
没有鼠标穿透承诺，也不提供通用擦除/圆角 clip 能力。

## 窗口交互与生命周期

Mount 连接 `Chrome.ConnectQueryRegion`，Unmount 断开。Window.SetWidget 在挂载后
才写根槽，因此根归属在查询时检查。查询只读当前几何，要求可见、启用、可缩放、
None/Normal 状态且仍是根。四角优先，沿边 16 DIP 扩展，不侵入内容矩形；小框以
最近边解决重叠。未命中不改结果；后连接者仍可返回 Client 或其他区域。

Windows 使用已有原生 HitTest；Linux/macOS 通过已有 Chrome host controller 和
手势竞争移交 BeginResize，不独立捕获指针、不做位置/尺寸模拟。
BorderBox 的 Cursor 方法参与既有 hover/capture 光标解析，使用同一个最终有序区域
查询，不缓存 hover 几何、不增加控制器；显式 SetCursor 可覆盖自动反馈。
禁用、最大化、卸载、后连接的 Client 覆盖、modal 与 capture 均不会残留独立状态。

Snapshot 使用 RoleBox，保留实际子树、窗口局部边界和 Enabled，不伪造点击动作。

## UI 声明

```go
ui.BorderBox(ui.VBox(ui.HeaderBar(ui.Label("工具")), content)).
    ResizeWidth(6).Resizable(true).StyleName("tool-frame")
```

支持 Child，复用 `ui.Bin` / UpdateChild，不增加 BuildContext 类型特例。
省略属性恢复 GUI 构造默认值，ID 仍服从既有声明身份/重建规则。

## 参考与取舍

- [Qt QSizeGrip 公共文档（6.12）](https://doc.qt.io/qt-6/qsizegrip.html)允许嵌套句柄控制
  顶层窗；GOUI 有意只允许 root，避免普通装饰容器隐式改变宿主。
- [GTK4 GdkToplevel.begin_resize（文档 4.24.1）](https://docs.gtk.org/gdk4/method.Toplevel.begin_resize.html)
  将交互式缩放交给窗口系统。[GtkWindowHandle 4.20.0 源码](https://github.com/GNOME/gtk/blob/4.20.0/gtk/gtkwindowhandle.c)
  在手势确认后使用原生 toplevel 操作；GOUI 复用已经验证的 Chrome 手势路径。
- 这些是 API/源码证据，不代表 GOUI 三平台实测；不照搬桌面坐标 fallback 或历史后端排除。

## 验证

### macOS 状态查询修正

macOS 15.7.7 / arm64 / OpenGL / 应用 1x 的真实 None 窗口，刚显示时旧实现
报告 Maximized（日志 STATE=3），440x340 DIP 窗口并未放大，BorderBox 因此拒绝缩放。
原生 None 没有 Resizable mask，且现行 RequestState(Maximized) 明确返回 Unsupported。
在 State() 中仅对有原生 Resizable mask 的窗口采用 isZoomed；Normal 请求也避免
对没有原生 zoom 能力的窗口调用 Zoom。隐藏/最小化/全屏的原生事实和优先级不变。
修正后报告 Normal（STATE=0），真实边缘缩放生效。`-auto` 对刚显示状态作独立断言，
不能把 isZoomed 原值和框架返回值互相比当成正确性。

[wxWidgets 3.2.8 Cocoa IsMaximized 源码](https://github.com/wxWidgets/wxWidgets/blob/v3.2.8/src/osx/cocoa/nonownedwnd.mm#L1094)
也区分 Resizable mask 与不可缩放窗口，不直接信任后者的 isZoomed。
[Qt 6.8.3 Cocoa toggleMaximized](https://github.com/qt/qtbase/blob/v6.8.3/src/plugins/platforms/cocoa/qcocoawindow.mm#L779)
临时增加 Resizable mask 实现放大/恢复。GOUI 不采用 Qt 扩大 None 能力的做法，
也不以工作区大小比较来模拟 zoom 状态，保持既定 None 请求契约。

包内：默认/自定义布局、约束/Inf/baseline、小框/空框、八方向、生命周期、后序覆盖、
禁用/状态、原生与 host 移交、Snapshot、UI 重建、Basic/Modern 无外观；Software
预乘 RGBA 1x/2x 独立像素采样检查透明角、背景和子树结构裁剪。

窗口：[tests/window/gui/border_box](../tests/window/gui/border_box/main.go)，中文入口注释
规定每一步操作及预期，-auto 仅原生布局/快照检查。真实光标、缩放、输入、移动和
卸载重挂载必须用桌面操作验收。结果与未覆盖项在本节追加，不把编译当原生运行。

### 2026-10-13 验收记录

- Go 1.25.8 / Linux amd64：相关 GUI/UI/Basic/Modern 回归、race 与限定 vet 通过；
  Windows amd64/macOS arm64 全仓 build 通过。普通 GUI/UI vet 仍有既有 unkeyed
  composites 告警，限定检查仅关闭该项，不改原有代码。
- 本机 Ubuntu 22.04.5 / X11 / Xfwm4 / Mesa llvmpipe：Software 1x 和 OpenGL 2x
  的右/下边缘缩放、HeaderBar 移动、文本输入、按钮计数、自定义圆角安全边界通过；
  Software 场景还检查禁用、Resizable、嵌套及卸载重挂载。Software/OpenGL 各 1x/2x
  与 Native/Integrated 指定模式的 -auto 布局/Snapshot/Normal/样式失效断言通过。
- Windows 10 19045 / x64 / RDP 交互 Session 1 / Direct2D / 应用 1x：None 的真实
  右/下边缘缩放、文本输入、点击、自定义圆角与重挂载通过；None/Native/Integrated
  的三阶段 -auto 通过。GUI/UI 新增回归在原生测试二进制中通过。
- macOS 15.7.7 / arm64 / VNC / OpenGL / 应用 1x：旧 State=3 误报已复现；修正后
  State=0、右/下缩放、HeaderBar 移动、文本输入、点击和圆角外观通过。
  None/Native/Integrated -auto 通过；GUI/UI 新增回归、Cocoa 已有 popup 尺寸逻辑通过。
- 八方向、覆盖、capture、显式光标、禁用、leave、状态、Popover 隔离由包内断言覆盖。
  远端截图不包含可靠的光标叠加，不宣称八方向系统光标图案逐项视觉验收通过；尝试
  SSH Session 0 GetCursorInfo 返回失败，未算通过。Windows 11、其他 WM/Wayland、
  真混合 DPI、远端其他绘制后端未验收。
- 全仓早期两轮为 2474 通过 / 15 既有跳过 / 0 失败；一轮末期复测发生未改的
  `platform/typography/pango TestTextAttributesReplaceNestedRanges/strikethrough` 偶发失败，
  `bitmap dimensions differ: want=0x0/0 got=296x102/1184`，随后该测试单独 count=10 通过。
  原始日志保留于 `/tmp/goui-borderbox.tRLhJc/test-delivery.json`，不放宽断言或修改 Pango。
  最终 `go test ./... -count=1 -json` 为 **2474 通过 / 15 既有跳过 / 0 失败**，
  日志为 `/tmp/goui-borderbox.tRLhJc/test-last.json`；最终相关 race、限定 vet、
  Windows amd64/macOS arm64 全仓 build 也通过。
  既有跳过包括 macOS signing 与显式手动窗口用例，不属于 BorderBox 新增 skip；
  已有全仓测试可能读写桌面剪贴板及 PNG，未迁移或删除这些用例。
- 本轮只保留任务专属测试产物，不写入源码目录；清理自己的测试窗口、临时 Windows
  scheduled tasks、传输服务和新建桌面连接，不修改系统配置。实现与验收已完成，
  提交版本以 Git 记录为准。
