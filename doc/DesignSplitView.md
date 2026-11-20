# SplitView：两区分割视图

## 范围与归属

命令式控件位于widgets，声明式薄封装位于widgets/ui；Basic和Modern提供线条样式。
Horizontal表示左右、Vertical表示上下，Start为左／上，End为右／下。允许嵌套。
本控件只调整自己的布局和子控件顺序，不修改platform、全局命中、光标或手势仲裁。

## 公共接口与尺寸偏好

```go
func NewSplitView(direction layout.Direction) *SplitView
func (*SplitView) Direction() layout.Direction
func (*SplitView) SetDirection(layout.Direction)
func (*SplitView) HandleSize() float32
func (*SplitView) SetHandleSize(float32)
func (*SplitView) StartChild() gui.Widget
func (*SplitView) SetStartChild(gui.Widget)
func (*SplitView) EndChild() gui.Widget
func (*SplitView) SetEndChild(gui.Widget)
func (*SplitView) SetStartSize(float32)
func (*SplitView) SetEndSize(float32)
func (*SplitView) SetRatio(float32)
func (*SplitView) Sizes() (start, end float32)
func (*SplitView) ConnectResize(func(start, end float32)) signal.Handle
```

HandleSize仅表示指针抓取带的期望厚度，单位DIP，默认5。0恢复默认；负数和非有限值
忽略，正小数有效。抓取带以分隔线为中心，受本SplitView边界裁剪，不平移补足。
它不增加面板间距，也不限制线宽。线宽可大于抓取宽度；额外线条没有拖动动作。

StartSize／EndSize／Ratio互斥，最后setter生效；固定尺寸指面板自身，Ratio只针对
扣除常态线宽后的可用空间。负面板尺寸归零，比例钳制到0..1，非有限值忽略。
Sizes返回当前分配；首选值独立保存，约束解除后恢复。Measure不提交或重置比例；
首次双侧非空Arrange按两侧自然测量建立比例，两侧均零则取0.5。

仅用户拖动、键盘调整及取消时恢复大小发送Resize。程序配置、测量、样式、窗口变化
不发送用户调整；修改方向、HandleSize或内容时先取消活动拖动。回调可以卸载或销毁
控件，之后检查生命周期和Root，不用清空信号代替保护，不递归发送取消恢复通知。
新内容须未挂载、未销毁且不能形成循环；替换只卸载旧内容，不销毁它。

## 面板布局与抓取几何

设主轴尺寸为m，常态线宽为w，g=min(w,max(0,m))，a=max(0,m-g)：

```text
[start pane][line g DIP][end pane]
center = start + g/2
handleStart = max(0, center-HandleSize/2)
handleEnd   = min(m, center+HandleSize/2)
```

默认w=1、HandleSize=5，抓取带覆盖两侧面板各2 DIP。改变HandleSize不减少面板空间。
线宽0时两侧直接相接，透明抓取带仍有效；单侧缺失或隐藏时，线和把手均隐藏且不占位，
剩余面板填满。恢复双侧后继续使用原尺寸偏好。

主轴约束来自内容根控件的MinSize／MaxSize，MaxSize的0表示无上限；自然测量不是
硬最小值。最小值之和超过空间时同比例压缩，父约束优先；最大值不足时放松并填满。
交叉轴填满；先自然测量，再按实际主轴分配重测。默认MainWeight=1。
私有pane维持内容裁剪，通过layout.Child保留测量缓存和样式失效路径。

**SplitView不会为了抓取带给面板增加Padding、缩小内容或移动滚动条。**
需要完整接收指针的按钮、滚动条、编辑器或其他拖动目标，由应用显式避让抓取带。
默认可以在对应面板内侧留2 DIP；非默认配置按实际线宽和抓取带计算。
只内缩内容，不给嵌套SplitView整体加空槽，才能保持其分隔线与外层线直接相接。

## 输入所有权与嵌套交点

抓取带是最后一个内部子控件，沿正常逆序命中获得本SplitView内最高目标优先级。
同一矩形决定光标、Hover、指针按下和Drag；覆盖区不穿透激活下面的按钮，不向下面
的滚动条或文本拖选让步。按下经既有DragEventController立即认领，保留抓取偏移。
没有父级后备候选、额外阈值或按控件类型判断的特殊优先级。

嵌套交点的重叠区域由外层把手优先命中。已经从内层或其他控件开始的捕获仍属于
原目标，移动经过交点不抢夺、不切换方向；这不是跨窗口或高于模态层的全局优先级。
方向键移动1 DIP、Shift方向键10 DIP，Home／End到有效端点，Esc恢复拖动前偏好。
卸载、捕获丢失或相关配置改变取消拖动，松手不重新应用已取消的位置。

默认零圆角时，内层横线延伸到其面板边界，与外层竖线的实际边缘相接，反向嵌套同理；
左右两侧横线对齐时可组成十字。框架不自动联动两个独立分隔位置，也不提供同时拖动两轴的手柄。

Snapshot保留两个真实内容子树和一个RoleSeparator；其Bounds为实际抓取带，允许
与内容边界重叠，保留焦点、范围及Focus动作。私有线条绘制节点无额外交互语义。
线条与抓取使用不同内部节点，线宽不会被抓取矩形裁去，抓取带也不绘制背景。

## 单一separator Part

默认StyleName为split-view；自定义StyleName同时选择separator，遵守最近StyleScope。
不读取或继承内容背景，不提供handle-start／handle-end Part或公共背景填充层。

| 字段 | 契约 |
| --- | --- |
| BorderWidth（Normal） | 实际线宽及布局占位；默认1 DIP。0隐藏，负数／非有限值按0处理；缺省使用1 DIP。 |
| ForegroundColor（状态） | 线条颜色，使用把手的Hovered／Pressed／Disabled／FocusVisible状态。 |
| Radius／CornerRadius（状态） | 填充轮廓的统一／独立四角半径，单位DIP，默认0；不影响布局和矩形抓取。 |

BorderWidth只读取Normal；状态规则里的线宽不改变布局或绘制厚度，防止反馈造成位移。
Basic与Modern默认使用中性色，FocusVisible使用已有焦点色；普通鼠标焦点不保留高亮。
Hover／Pressed默认不变色，应用可以覆盖。隐藏线条时应用需另行提供所需的焦点提示。

Radius是CornerRadius的统一半径简写，沿用样式系统的完整四角替换规则。四角按物理
左上、右上、右下、左下解释，不随HSplit／VSplit方向旋转。负数及非有限半径按0处理，
超大半径按实际分配尺寸归一化；均匀半径不超过短边的一半，非均匀半径同比例缩小
直至相邻圆角适合边长。状态可以改变圆角和颜色，BorderWidth仍只读取Normal。

圆角仅改变可见填充，不将透明角排除在抓取矩形之外，也不产生新的语义节点或动作。
复用GUI的圆角背景绘制路径，以ForegroundColor作为填充；BorderWidth只表示条带厚度，
不作为矩形描边再次绘制，不读取BackgroundColor／BorderColor。0半径仍沿用原矩形绘制路径。
默认0半径维持横竖线完整衔接；显式圆角会使端部收口，不承诺任意嵌套交点仍无缺口。

```go
wui.HSplit(sidebar, content).HandleSize(5).Style("sidebar-split")

style.Name("sidebar-split").Part("separator").ForegroundColor(separatorColor).BorderWidth(1)
style.Name("sidebar-split").Part("separator").State(style.Hovered).ForegroundColor(hoverColor)

// An optional wide rounded separator; the grab band remains independent.
style.Name("rounded-split").Part("separator").ForegroundColor(separatorColor).BorderWidth(6).Radius(3)
```

## 声明式与迁移

widgets/ui的HandleSize只转发相同setter；省略或0恢复5 DIP。Start／End双Bin、
OnResize连接、受控／非受控尺寸行为不变，应用受控尺寸仍应随OnResize更新模型。

原先整个3／5 DIP分隔带占位的规则改为仅常态线宽占位，移除两侧背景Part。
应用迁移到separator线色／线宽，并显式避让有指针交互的相邻控件。
不保留旧split-handle样式名、两背景Part或自动保护相邻控件的兼容路径。

## 参考与验证

Qt 6.8的[handleWidth公开契约](https://doc.qt.io/qt-6.8/qsplitter.html#handleWidth-prop)
允许0／1宽度时扩大实际抓取区；[Qt 6.8.3源码](https://github.com/qt/qtbase/blob/v6.8.3/src/widgets/widgets/qsplitter.cpp#L204)
通过内容边距和mask区分绘制与抓取。GTK 4.18.6的
[get_handle_area](https://github.com/GNOME/gtk/blob/4.18.6/gtk/gtkpaned.c#L303)
在非wide-handle模式扩大拖动判断范围。以上为公开契约及源码行为，未在本轮运行这些框架。
GOUI采用细占位与宽抓取分开的思路，使用本身的Widget子序及事件捕获，不移植原生mask
或修改全局事件系统；默认5 DIP抓取宽度和明确的应用避让是本项目的选择。

圆角外观另参考[Avalonia 11.3.9 Fluent GridSplitter模板](https://github.com/AvaloniaUI/Avalonia/blob/11.3.9/src/Avalonia.Themes.Fluent/Controls/GridSplitter.xaml)
将CornerRadius用于可见轮廓的源码做法。本轮未运行Avalonia；GOUI沿用自己的
ForegroundColor填充和BorderWidth厚度契约，不引入其完整Border模板或新handle Part。

包内回归保留布局、约束、测量缓存、键盘、取消、生命周期和声明式协调覆盖；新增
抓取覆盖优先、区域外控件独立操作、嵌套交点及跨交点捕获、线宽和抓取独立、零线宽、
极小父约束和样式反馈不移动布局的断言。Software离屏验证预乘RGBA、透明抓取带、
整数／小数位置和1／1.25／1.5／2x。窗口入口tests/window/widgets/split_view通过
真实输入检查光标、动态配置、显式避让，并对实际后端及桌面交点做像素检查。
本轮结果见Plan；编译、离屏绘制或仅启动窗口均不代替桌面操作验收。

## 历史记录

以下保留原提交背景；旧的1 DIP占位、9 DIP重叠命中及后备手势已由上文契约替代。

### 2026-10-03 按原生重入链路收敛修复

- 最终修复保留 GetMessage、PeekMessage、TranslateMessage、DispatchMessage
  的 Proc.Call，以及明确同步调用窗口过程的 CreateWindowEx、BeginPaint、
  GetWindowText、SetWindowText；只有八个 WinAPI 入口改变调用方式。
  普通 COM/SDK 查询及绘图恢复原 Syscall/CallRaw，结构体按值和浮点等原有
  cgo.Call 不变，没有新增绘图热路径的反射参数转换。独立的 HitTestPoint BOOL*
  签名修正及真实原生测试仍保留。
- EventLoop.Run 和 helper 消息循环各自在循环外分配、复用一个 MSG，
  每次 GetMessage/PeekMessage 前用零值清空缓冲区，避免沿用上一条消息字段。
  Windows Go 1.25.8 逃逸分析确认两个 `new(winapi.MSG)` 均逃逸到堆上；
  稳定地址由关键调用边界保证，不依赖 new 语法或碰巧不增长的 Go 栈。
- 窗口重入程序分别在 GetMessage、PeekMessage 接收函数中声明缓冲区，
  避免 GetMessage 的逃逸替 PeekMessage 掩盖错误；增加真实 DispatchMessage
  同步窗口回调检查。撤回初轮自行新增的 GetMetrics 假 vtable 回调测试：
  它强制模拟了该原生查询不具备的同步 Go 回调，不能据此要求所有绘图查询
  都换成 cgo.Call。真实 HitTestPoint 原生断言及所有既有测试保持。
- 收敛后 Linux amd64 隔离 Xvnc :98 / Xfwm4 / Software 的全仓测试通过：
  46 个有测试包、1694 个测试/子测试、14 个既有人工/opt-in 跳过，无失败。
  Windows amd64 全仓测试目标编译通过；不是原生运行结果。
- Windows 原生重入新/旧对照及 dwrite 测试产物已传输、SHA256 一致，
  首次启动被 Defender ASR 1121 拦截，当时未修改安全策略。
  RDP Session 1 的桌面 Split 用例在 Direct2D / Software 启动配置下也均
  返回拒绝访问，启动器退出码 5；没有创建窗口或执行拖动，不算场景通过。
  随后按用户明确要求临时关闭 Defender 实时防护，只复测消息重入；关闭状态
  为 RealTimeProtectionEnabled=False，测试后恢复原 DisableRealtimeMonitoring=False，
  等待并确认 RealTimeProtectionEnabled=True，没有调整 ASR 规则。
- 最终原生消息复测在 Windows 10 19045 x64、SSH Session 0 创建隐藏 HWND，
  旧 WinAPI overlay 构建以 `MSG moved during native callback` 失败，退出码 2；
  修正版的 GetMessage / PeekMessage 各八次独立缓冲区回调、DispatchMessage
  回调和 GetMessage 错误/WM_QUIT 契约均通过，退出码 0。
  最终修正版 SHA256 为
  `feb381aeaa56a1ab5e778ba72aefb9ee8c3be5904c583e58e4a4e73a148c8eda`，
  旧绑定对照为 `4731307d97dcd03f547a92a642974cb6ae8761d543cef71f266da271a51b8aaa`；
  远端传输哈希与本地一致。日志在远端同名任务目录的 `message-*-retest.out/err/exit`，
  防护前后状态在 `defender-before.json` / `defender-after.json`。
  本次没有重跑 HitTestPoint 或交互桌面的 Split 拖动；初轮相关结果见下一节，
  属于收敛前构建，不能代替最终构建的这些场景验收。
  本轮产物、全仓日志与截图在 `/tmp/goui-win-msg.hQYVQV`。

### 2026-10-03 初轮崩溃修复与验证（调用方式已由上节收敛）

- 旧程序在 Windows 10 19045 x64、Direct2D、Native 1x 快速拖动时两次退出
  `0xC0000005`。原生异常位于 USER32，调用链指向 GetMessageW 写入 MSG。
  Go 1.25.8 逃逸分析及原生消息回调回归共同确认：原绑定把栈上 MSG 地址作为
  uintptr 传入 SyscallN，同步窗口回调增长 Go 栈后，原生端仍持有旧地址。
- 修正发生在 Windows SDK 调用边界，不修改 SplitView、布局或拖动行为。
  借用 Go 指针的 WinAPI 使用既有 Proc.Call；COM 输出/缓冲区保留类型交给
  cgo.Call/CallRet；必须保留原始调用的少量入口显式固定 Go 缓冲区。
  原生指针、标量及已有固定生命周期的 OLE 对象不机械改写。
  同类审查还修正 DirectWrite HitTestPoint 的两个 BOOL* 输出参数；这是独立
  潜在内存错误，没有证据表明它触发了本次 Split 崩溃。
- 新增 `tests/window/platform/message_reentry`：真实 GetMessage/PeekMessage
  同步重入、强制栈增长、消息内容及退出/错误契约断言。用旧 WinAPI 文件的
  Go overlay 构建，对照程序以 `MSG moved during native callback` 失败；
  修正版通过。测试显式创建隐藏 HWND，在 Windows SSH Session 0 执行，
  不把它等同于交互桌面验收。
- Windows 原生 SDK 的 winapi、d2d1、dwrite、dxgi、dcomp、shell 六包共八个
  测试通过，包含新增 COM 回调输出及真实 DirectWrite HitTestPoint 测试。
  额外的 graphics/direct2d 和 typography/directwrite 整包测试已编译、传输
  并校验哈希，但启动返回拒绝访问，Defender 日志确认 ASR 1121；未运行、
  不算通过，也没有调整安全策略。
- RDP Session 1、Native 1x：Direct2D 与 Software 分别执行横向 1200 次和
  纵向 300 次快速指针移动；窗口存活，松手后的两个独立分区几何断言通过，
  正常关闭退出码为 0。最终构建再次验证 Direct2D 横向快速拖动；进程加载
  d2d1/d3d11/dxgi，未加载 OpenGL，结合后端选择路径排除 Software 回退。
  OpenGL 在此 RDP 环境报无法选择可用像素格式，旧/新程序一致，交互仍待验收。
- Linux amd64 隔离 Xvnc :98 / Xfwm4 / Software 下 `go test ./...` 通过：
  46 个有测试包、1694 个测试/子测试、14 个既有人工/opt-in 跳过，无失败。
  Windows amd64 `go test -exec=/bin/true ./...` 全仓目标编译通过，不等同原生运行。
  既有图像测试仍产生图片；本轮日志、截图及二进制保留在仓库外
  `/tmp/goui-win-stack.b7GlPO`，未操作用户已有窗口或更改系统配置。

### 2026-10-03 外观修正

- 分隔线统一 1 DIP，默认不高亮；应用可覆盖 Hovered／Pressed 颜色。
  焦点保留键盘与 Snapshot 语义，不参与分隔线的绘制状态选择。
- `TestSplitViewHandlePointerFeedbackAndFocus` 经 EventDispatcher 验证悬停、按下、
  拖动、松开、移出及后续键盘调整；覆盖亮暗 Modern、兜底样式、两种方向和
  自定义颜色。移出后仍有焦点但恢复普通颜色，扩展拖动范围仍为线两侧各 4 DIP，实际 Widget 边界为 1 DIP。
- `go test ./...` 和 `go test -race ./widgets/... ./theme/modern` 通过。
- 本机 Linux amd64、独立 Xvnc/X11、Xfwm4 合成开启、Software 1x、Native 装饰：
  UI 水平拖动后移出，线保持细中性色；随后方向键仍调整 1 DIP，几何断言通过。
  截图采样验证方向键前后线色不变、位置移动 1 个物理像素。
  本轮没有重跑 OpenGL、Direct2D 或 Windows/macOS 桌面验收，不修改平台绘制代码。

### 2026-10-02 实现与验证

- 两区控件、私有分配／拖动实现、双 Bin UI 绑定、widgets 兜底及 Modern 样式、
  通用 RangeInfo 和窗口程序均已实现。
- `go test ./...`、`go test -race ./widgets/... ./theme/modern` 通过；
  `go test ./widgets -run TestSplitView -count=10` 通过。初轮新增生命周期用例失败，
  核对 Popover 销毁只卸载内容的现有契约后补齐宿主失效和重入取消防护，重跑通过。
- 本机 Linux amd64、独立 Xvnc/X11、Xfwm4 合成开启、Native 装饰：
  Software 1x 的 UI 拖动、1／10 DIP 键盘步进、Esc、嵌套上下拖动、受控尺寸、
  普通重建、隐藏／恢复、窄窗口恢复、亮暗及几何／语义断言通过。
  OpenGL 2x（Xvnc/Mesa 软件渲染）的纯 GUI 水平拖动、步进、Esc 和两区／嵌套
  几何断言通过；另以 Software 1x 验证显式兜底样式、Home／End 和几何断言。
  没有性能结论，没有穷举所有后端与缩放组合；不将光标属性检查宣称为所有主题的视觉验收。
- Windows amd64／macOS arm64 窗口程序和 widgets／widgets/ui 测试目标交叉编译通过。
  不代表原生运行或桌面验收，本轮不访问远端；两平台桌面交互留给后续验收。
- 额外尝试 Linux arm64 窗口程序交叉编译，受既有
  `platform/linux/libs/libc/syscall_linux.go` 的 `syscall.SYS_PIPE`／`SYS_POLL`
  未定义阻塞。失败命令为
  `GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go build ./tests/window/widgets/split_view`，
  错误位置为该文件 29／61 行；核对 HEAD 中已有相同引用。此文件没有本轮改动，
  不在 SplitView 切片中修改核心绑定。
- 窗口验收只操作独立测试桌面，不操作用户应用或剪贴板。产物留在仓库外临时目录；
  既有全仓测试的图片输出仍按现有行为执行，没有迁移或清理用户文件。

### 相邻控件输入回归

扩展候选不在按下时抢占；获胜后复用 handle 的调整、聚焦、Esc 回滚和生命周期取消。
方向、首选尺寸或子项变更时同时清理待认领和活动手势；滚动条默认几何不变。
逻辑用例独立检查双向相邻滚动条、按钮点击、子级拖动、空白扩展拖动和取消。
Qt 的小尺寸 handle 会扩展抓取区，但这不是相邻输入安全的保证；GOUI 采用已有仲裁。
