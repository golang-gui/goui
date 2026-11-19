# SplitView：两区分割视图

## 范围与归属

命令式控件位于 widgets，声明式薄封装位于 widgets/ui，Basic 与 Modern 提供完整样式。
两区可嵌套；Horizontal 表示左右、Vertical 表示上下，Start 为左／上，End 为右／下。
不修改平台、GUI 命中、手势优先级或 Root，不引入 Docking、吸附和持久化。

## 接口与尺寸偏好

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

HandleSize 是真实分隔区域沿分割主轴的厚度，单位 DIP，默认3。0恢复默认，负数及
非有限值忽略，正数允许小数。左右分区占宽，上下分区占高。硬父约束可以减少实际
分配，不改写期望值。单侧缺失／隐藏时不显示或占位，恢复后继续采用原尺寸偏好。

StartSize／EndSize／Ratio 互斥，最后 setter 生效。固定尺寸表示面板本身，不包含
分隔区域；比例只针对扣除分隔区域后的可用空间。负面板尺寸归零，比例钳制到0..1，
非有限值忽略。Sizes 是当前分配结果。首选尺寸与实际分配分开，约束解除后恢复。

没有尺寸声明时，首次非空双侧 Arrange 按两侧 Loose 测量主轴尺寸建立比例；两侧
均零时取0.5。Measure、内容重测、样式及窗口缩放不改写已经建立的偏好。
拖动按当前模式更新比例或固定尺寸。只有用户调整和取消拖动的恢复发送 Resize；
配置、Measure、Arrange 不发送。修改 HandleSize、方向或内容时先按原规则取消拖动，
取消导致的恢复通知仍保留。回调可能卸载或销毁，继续访问前须重新检查有效期。

新子控件须未挂载、未销毁且不能形成循环；替换只卸载旧内容，不销毁。内容子树与
结构 pane 不混为样式继承体系；私有 pane 提供布局和裁剪，应用内容保留自身样式。

## 布局、绘制与输入使用同一个区域

双侧存在时：g=min(HandleSize, max(0, extent))，a=max(0, extent-g)。分配关系为
[start pane][handle g DIP][end pane]。三个矩形不重叠，覆盖整个主轴。

```text
lo = max(startMin, a-endMax)
hi = min(startMax, a-endMin)
start = clamp(preferredStart, lo, hi)
end = a-start
```

主轴约束来自两侧内容根控件的 MinSize／MaxSize，MaxSize 的0表示无上限；自然测量
尺寸不是硬最小值。最小尺寸之和超过空间时同比例压缩，父约束优先；最大尺寸之和不足时放松上限并填满。
单侧 min 大于 max 时以 min 为下限。交叉轴填满；先自然测量，再按实际主轴分配重测。
默认 MainWeight=1。布局继续经过 layout.Child，保留测量缓存和样式失效机制。

整个 handle 矩形都是可拖动区域，包括线两侧的背景。悬停立即显示方向对应的调整
光标；按下经既有 DragEventController 立即认领，保留抓取偏移；捕获后移出仍可拖动。
删除原来扩出布局的命中范围、父级后备手势及依赖该手势的 Cursor 覆盖。
滚动条、按钮、文本拖选在各自面板内正常接收输入，不按控件类型设置特殊优先级。

方向键1 DIP、Shift方向键10 DIP，Home／End 到有效范围端点，Esc 恢复拖动前偏好。
卸载、丢失捕获、方向或尺寸配置变化都会清理活动拖动；松手不重新应用取消的位置。
Snapshot 保留一个 RoleSeparator，其 Bounds 就是完整 handle，含焦点、范围和 Focus
动作。两色背景和细线都是绘制 Part，不增加语义节点或交互动作。

## 三个样式 Part

默认样式名 split-view。自定义 StyleName 同时选择三个 Part；通过 ResolveWidgetStyle
遵守最近的 StyleScope，不读取或继承面板控件的背景。不再使用旧的 split-handle 规则。

| Part | 字段 | 几何 |
| --- | --- | --- |
| handle-start | BackgroundColor | 起始侧半区 |
| handle-end | BackgroundColor | 结束侧半区 |
| separator | ForegroundColor、BorderWidth | 居中的细线 |

两个半区共用同一个中心边界，直接各绘制一次，没有 handle 底色和隐式回退层。
统一背景时配置相同颜色；侧栏场景复用应用的侧栏／内容背景色，各自填满半区，
中间细线最后覆盖。半透明颜色按 source-over 合成，透明度不影响命中。

默认线宽1 DIP。默认3 DIP区域下，线两侧各有1 DIP可见背景。实际线宽钳制到区域
厚度内；0、负值或非有限线宽不绘制细线，但不改变分隔占位或拖动范围。
样式中的线宽只影响绘制，不参与面板布局。半区只消费背景色，separator 只消费
线色和线宽，不借用 BorderSides、Shadow 或 Radius 扩展范围。

三个 Part 使用同一 handle 的 Hovered／Pressed／Disabled／FocusVisible 状态。
普通鼠标焦点不保留高亮；键盘焦点通过 separator 的 FocusVisible 色提供反馈。
Basic 两侧用已有窗口底色和中性线，Modern 用窗口底色、分隔色与已有焦点色；默认
悬停／按下不增加新配色，应用可按 Part 状态覆盖。侧栏的两侧背景不被统一底色盖住。

```go
wui.HSplit(sidebar, content).HandleSize(3).Style("sidebar-split")

style.Name("sidebar-split").Part("handle-start").BackgroundColor(sidebarBackground)
style.Name("sidebar-split").Part("handle-end").BackgroundColor(contentBackground)
style.Name("sidebar-split").Part("separator").ForegroundColor(separatorColor).BorderWidth(1)
```

## 声明式与迁移

widgets/ui 增加 HandleSize，Update 调用相同 setter；省略或0恢复3 DIP，不引入另一套
状态机。Start／End 双 Bin、OnResize 连接和受控／非受控尺寸契约不变。应用受控
尺寸仍应在 OnResize 更新模型，重建不声明面板尺寸时保留用户调整。

旧1 DIP占位改为默认3 DIP，面板可用空间因此减少2 DIP；应用需要其他厚度时显式设置。
旧 split-handle.ForegroundColor 样式迁到 split-view 的 separator Part，并配置
两个背景 Part。不保留两套样式名称查找路径。

## 参考与验证

Qt 6.8.3 的 [handleWidth](https://github.com/qt/qtbase/blob/v6.8.3/src/widgets/widgets/qsplitter.cpp#L1604)
区分把手布局厚度，0／1的 tiny 模式另有重叠抓取。GOUI 采用实际占位、自定义外观的
思路，默认3 DIP是本项目的取舍，不采用越过布局边界的重叠抓取，也不复制 Qt 子类接口。

widgets/split_view_test.go 保留1 DIP显式配置下既有分配／约束／键盘断言，另独立覆盖
默认3 DIP、1／1.5／3／5／9厚度、全区域命中与区域外输入、双向及嵌套、动态取消、
焦点、语义和三Part作用域。Software 以独立物理边界和预乘合成公式覆盖整数／小数
位置及1／1.25／1.5／2x，不能只比较两条可能同时出错的渲染路径。
widgets/ui/split_test.go 验证厚度声明及默认恢复，并保留稳定子树与受控／非受控行为。
窗口入口 tests/window/widgets/split_view 覆盖 GUI／UI、侧栏异色、动态厚度、细线、
两侧滚动和真实悬停光标；本轮结果见 Plan，不把编译或仅窗口启动记为操作验收。

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
