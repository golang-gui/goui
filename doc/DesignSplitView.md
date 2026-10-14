# SplitView：两区分割视图

## 范围与归属

首版提供可嵌套的两区容器，复杂控件位于 `widgets`，声明绑定位于
`widgets/ui`，完整兜底样式位于 `theme/basic` 并由 GUI 自动使用，Modern 自带完整样式。
不添加平台能力，不修改 Root、BuildContext、手势仲裁或基础布局接口。
不实现 Docking、自动折叠、双击恢复、吸附及尺寸持久化。

`Horizontal` 表示左右分区，`Vertical` 表示上下分区；Start 为左／上，End 为右／下。
分割视图自身默认 `MainWeight=1`，参与父级剩余空间分配，但 `Measure` 仍由内容撑开。

## 公共接口

```go
func NewSplitView(direction layout.Direction) *SplitView
func (*SplitView) Direction() layout.Direction
func (*SplitView) SetDirection(layout.Direction)
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

尺寸和位置为局部 DIP。三个尺寸 setter 互斥，最后一次调用生效。负尺寸归零，比例
限制在 0..1，非有限值忽略。`Sizes` 为当前分配结果，首次布局前为零；不是尺寸偏好。

默认不指定比例：首次两侧可见且具有正可用空间的 Arrange，根据两侧在相同 Loose
约束下的测量主轴尺寸建立比例。例如 240:560 建立 0.3；两侧皆零则使用 0.5。
Measure 不提交该比例。后续内容重测、样式变化或窗口缩放不覆盖它。
`SetRatio` 显式建立比例，窗口缩放保持比例；`SetStartSize`／`SetEndSize` 固定对应一侧
的首选 DIP 尺寸。拖动分别更新当前模式的比例或固定尺寸。

尺寸偏好与实际分配分开保存。约束导致临时压缩时，不丢失偏好；空间恢复后重新采用。
仅用户拖动、键盘调整及取消拖动时的恢复发出 Resize；程序 setter、测量及窗口缩放不发出。
通知前更新 Sizes 并请求布局，子控件矩形在下一次布局生效。
回调期间因关闭宿主或修改内容而产生的取消恢复不递归发出 Resize；对象销毁或跨 Root
迁移后跳过尚未开始的回调。不以清空信号代替生命周期保护。

新子控件必须未挂载、未销毁、不同于另一侧且不能形成祖先循环。替换只卸载旧控件，
不销毁。nil 或自身隐藏的侧不占空间，另一侧填满且无分隔条；保存的尺寸偏好不变。
两侧可收缩至零，但不由容器改变内容的 Visible，分隔条保留以便恢复。

## 布局与约束

私有 pane 是实际 Widget 子树的结构裁剪边界，内部 FillLayout 通过 `layout.Child`
走 GUI 的测量缓存及 StyleChanged 通知。私有 splitLayout 和纯分配函数承担分区计算，
不扩充通用 `layout.Child`。

分隔间隙 1 DIP，可用主轴 `a=max(0, extent-1)`。内容根 Widget 的 MinSize／MaxSize
约束主轴分配；期望测量尺寸不是硬最小值。MaxSize 的零表示无上限。

```text
lo = max(startMin, a-endMax)
hi = min(startMax, a-endMin)
start = clamp(preferredStart, lo, hi)
end = a-start
```

两侧最小值之和超过可用空间时按最小值比例压缩，父级硬尺寸优先，结果非负。
两侧最大值之和不足以填满时放松上限、保留最小值，再按尺寸策略填满。
同一侧 min 超过 max 时以 min 为分隔条范围下限；内容自身仍遵守基础布局约束规则。
交叉轴填满 pane，子内容仍使用自身布局。测量先得到自然主轴尺寸，再按分配后的主轴
测量换行内容；Arrange 以最终尺寸重测、布置两侧。首版不承诺 SplitView 自身的文字 baseline。

## 输入、外观与语义

分隔条是实际最上层 Widget：所有状态的视觉线宽均为 1 DIP；命中
宽度 9 DIP，居中覆盖两侧各 4 DIP，边界处裁到容器范围。不会用装饰 Widget 抢占更大区域。
独立使用 `split-handle` StyleName，ForegroundColor 控制线色；Modern 和 widgets 兜底
默认使用中性色，不随悬停、拖动或焦点改变。应用可覆盖 Hovered／Pressed 的颜色来
启用指针反馈，线宽仍为 1 DIP，不新增高亮开关或专用宽度 API。
焦点只支持键盘操作与语义状态，不参与分隔线绘制状态选择；移开鼠标即恢复普通线色，
不能因仍有键盘焦点而保持高亮。不清除焦点来掩盖绘制问题。
不向内容传递样式，不增加窗口装饰或 Painter 专用能力。

既有 DragEventController 阈值零，参加正常手势竞争及捕获；坐标转换到 SplitView 局部后
保留抓取偏移，避免 handle 在重布局后移动引发跳动。使用既有水平／垂直缩放光标。
鼠标按下经标准分发取得焦点，主轴方向键移动 1 DIP，Shift+方向键 10 DIP，Home／End
到有效范围端点，Esc 恢复活动拖动前的尺寸策略。丢失捕获、卸载、改变方向或内容时取消
活动拖动。回调可能销毁控件，后续操作检查生命周期。

Snapshot 的根 Role 为 `splitview`，只列真实内容和可见 `separator`，不暴露 pane 包装。
separator Bounds 是实际命中区域，记录 Focusable／Focused 和 ActionFocus；通用可选
`WidgetInfo.Range` 提供 Value／Min／Max（DIP）及值增加的 Direction，不是线的朝向。
不添加绕过正常输入派发的“调整值”动作。本切片未引入全局 Tab 键焦点导航或 OS 无障碍桥接。

## 声明式绑定

```go
wui.HSplit(start, end).
    StartSize(sidebarWidth).
    OnResize(func(start, end float32) {
        sidebarWidth = start
        app.RequestUpdate()
    })

wui.VSplit(editor, terminal).EndSize(160)
```

提供 Start／End、StartSize／EndSize／Ratio、OnResize 链式声明。两侧使用各自稳定的
`ui.Bin` 目标，复用普通协调机制；不向 BuildContext 添加类型特例。不声明尺寸时，重建
保留现有用户尺寸；声明时每次重建应用该值，属于受控用法，用户在 OnResize 更新模型。
没有另一套 UI 拖拽或布局状态机。连接在 Mount 建立、Update 更新回调、Unmount 断开。

## 参考与取舍

- [GTK4 Paned](https://docs.gtk.org/gtk4/class.Paned.html)（查阅文档 4.23.4）明确两区、
  默认由两侧 size request 决定分区、单侧缺失无分隔条。采用其两区及按内容初始化思路，
  不引入独立 resize/shrink 布尔组合或 RTL 新能力。
- [Qt QSplitter](https://doc.qt.io/qt-6/qsplitter.html)（查阅文档 Qt 6.12）说明细 handle
  可扩展实际抓取区域。采用视觉宽度与命中范围分离，不引入多区列表、折叠或保存状态 API。
- [Avalonia GridSplitter 11.3.0 源码](https://github.com/AvaloniaUI/Avalonia/blob/11.3.0/src/Avalonia.Controls/GridSplitter.cs)
  的键盘调整、CancelResize 和约束检查帮助核对交互设计。GOUI 自行选择 1／10 DIP 步进，
  不复制 Grid 定义、Preview 或行列重写机制。

以上是参考契约／源码；GOUI 的首次 Arrange 记忆、约束冲突退化、三种尺寸模式及
Resize 信号语义是本设计的明确选择，不宣称与参考框架所有边缘行为一致。

## 验证

包内 `widgets/split_view_test.go` 集中验证比例初始化、内容重测、首选尺寸恢复、
min/max 冲突、单侧与嵌套、Snapshot、命中、真实派发路径拖动、重布局、键盘及取消。
`widgets/ui/split_test.go` 验证稳定双 Bin、受控／非受控更新及连接释放。
窗口程序为 `tests/window/widgets/split_view`，含 GUI 和 UI 两条入口，中文步骤见入口注释。

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
  自定义颜色。移出后仍有焦点但恢复普通颜色，命中宽度仍为 9 DIP。
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
