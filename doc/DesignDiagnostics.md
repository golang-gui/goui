# 可选绘制与资源诊断

## 目的与边界

区分框架测量缓存查询、Paint 调用、文字栅格化和图片操作，提供 GUI 线程帧阶段耗时。
默认关闭；不因诊断修改失效、缓存、布局、绘制、输入或帧调度策略。
基础采集不同时优化缓存，不引入绘制指令缓存、局部重绘或原生 vsync。
2026-11-04 增加显式开启的 HUD，设计见下文；只读采集保持原有行为。

本次参考 Flutter main 的 [RenderObject.layout](https://api.flutter.dev/flutter/rendering/RenderObject/layout.html)
及其内嵌实现：布局入口与 performLayout 分工明确，追踪发生在已知调用边界。
[逐对象布局追踪说明](https://api.flutter.dev/flutter/rendering/debugProfileLayoutsEnabled.html)
明确指出详细追踪的成本可能显著改变耗时。因此采用分阶段、可选详细计数的思路，
不为观测照搬 Flutter 的布局接口或给 GOUI 增加强制基类调用约束。
GTK/GDK 4.24.1 的 [FrameClock.get_fps](https://docs.gtk.org/gdk4/method.FrameClock.get_fps.html)
根据帧时序计算频率；GOUI 目前只掌握 CPU 帧边界，不能声称获得真实呈现频率。
以上是参考文档／实现行为；下文是 GOUI 自身的统计契约。

## 公共接入

公开入口位于 `gui/diagnostics`，不向 Application、Root、Widget、Painter 接口增加成员：

```go
capture, err := diagnostics.Start(app, diagnostics.Options{Widgets: true})
if err != nil {
    return err
}
// Later, on the GUI thread:
snapshot := capture.Snapshot()
_ = snapshot
capture.Stop()
```

Start 接收已通过 gui.NewApplication 建立的当前应用，可在 Run 前调用。
一个进程同时只有一个活动采集。默认选项只记录帧与资源；Widgets 显式开启逐控件
调用计数，不逐控件计时。Stop 幂等停止并冻结结果，重启使用新的 Capture。
Application.Run 返回时停止属于该应用的采集，旧句柄不能停止新采集。
Start／Snapshot／Stop 均在 GUI 线程使用，原生资源操作计数可来自其它线程。

仅导入包不启用采集。不启动诊断 Timer、goroutine 或窗口，不连接帧信号，
不调用 RequestPaint／RequestLayout。启用后也只随已有工作记录。
Snapshot 会整理缓存元数据并复制结果，建议按需或低频查询，不在每次 Paint 中查询。
启用统计有开销，尤其逐 Widget 计数；不宣称零成本。

## Go 标准诊断工具接入（2026-11-05）

保持现有 Capture、HUD 和精确资源计数，补充 `runtime/trace` 的帧语义标记；
公共 Widget／Root／Painter 接口、平台后端和 Go 1.24 最低版本不变。

- 标准 CPU、堆、锁和阻塞分析继续使用 `runtime/pprof`；应用控制采集生命周期，
  GOUI 不自动开启 CPU profiling、HTTP handler、监听端口或全局 trace。
- `runtime/trace` 由应用调用 Start／Stop，或由 `go test -trace` 开启。其开关与
  diagnostics.Capture 独立；只有 trace 开启时也能看到帧，不隐式开启逐 Widget 统计。
- 每次被接受的窗口／Popover 帧尝试对应固定类型 `goui.frame` Task。阶段 Region 为
  `goui.callbacks`、`goui.styles`、`goui.layout`、`goui.begin`、`goui.paint`、`goui.end`。
  布局只在实际执行时记录，可一帧多次；Paint 包含内容、装饰和 AfterPaint，End 单列。
  帧前的输入与等待、RenderWidget 离屏绘制不伪装成窗口帧；Task 结束不代表呈现成功。
- `goui.root` 日志使用进程内 rootBase 地址区分根；不要求用户 ID，不引入全局编号表，
  不调用窗口／Widget getter、不遍历树，也不把动态标识拼进 Task 或 Region 类型名。
  地址在对象回收后可能复用；它只是追踪辅助信息，与 dev ref／Capture ref 无关，
  不应持久化为应用身份。
- 回调取消／销毁、布局或绘制 panic 均须结束已经开始的 Region／Task，不吞掉 panic，
  不改变 Painter.End 的接收者或执行次数。不同根的同步嵌套帧保持同 goroutine 区域嵌套。
  帧中途开启 trace 从下一帧标记；中途停止可能截断当前记录，正常清理仍继续执行。
- 关闭路径先检查 trace.IsEnabled，不创建 Task／Context／格式化元数据；后续阶段仅
  检查本帧的私有 trace 状态。Go 1.24 的 NewTask 在关闭时仍会分配，不能无条件调用。
  不新增 trace Timer、goroutine、帧请求或默认逐 Widget／资源事件流。

语义埋点放在 rootBase 已有阶段，私有记录辅助复用 gui/diagnostics.go。原资源计数
继续用原子数值；`runtime/metrics` 只提供 Go 运行时指标，不作为自定义指标注册表。
pprof 的自定义 Profile.Add 会保活登记值，不用于替代现有缓存／资源计数。
Go 堆和调度数据不冒充原生库／GPU 显存或呈现时间。

应用可写出标准 trace 文件并用 `go tool trace <path>` 分析。既有独立例程提供
`-trace <path>`，例如：

```bash
go run ./tests/window/gui/diagnostics -trace /tmp/goui.trace
go tool trace /tmp/goui.trace
# 不启动 Capture，手工操作后正常关闭，同样记录窗口帧：
go run ./tests/window/gui/diagnostics -auto=false -trace /tmp/goui.trace
```

验证包括标准工具实际解析输出、任务／阶段顺序及闭合、Window／Popover、独立与组合
采集、取消／panic、采集中途启停、关闭时零额外分配与同条件前后基准。标准 trace
测试在子进程中控制全局开关，不能与父测试的 `go test -trace` 冲突或通过 skip 绕过。
实际执行范围记录在 [Plan](Plan.md)。

依据：[Go 诊断文档](https://go.dev/doc/diagnostics)、
[Go 1.24 trace 标记源码](https://github.com/golang/go/blob/go1.24.0/src/runtime/trace/annotation.go)、
[runtime/metrics](https://pkg.go.dev/runtime/metrics@go1.24.0)。参考 GTK 4.18.0 的
[GDK profiler](https://github.com/GNOME/gtk/blob/4.18.0/gdk/gdkprofilerprivate.h)、
Qt 6.8.0 的 [tracepoint 开关](https://github.com/qt/qtbase/blob/v6.8.0/src/corelib/global/qtrace_p.h)、
Flutter 3.32.0 的 [帧报告](https://github.com/flutter/flutter/blob/3.32.0/packages/flutter/lib/src/scheduler/binding.dart#L249)
和 Avalonia 11.3.0 的 [阶段计时](https://github.com/AvaloniaUI/Avalonia/blob/11.3.0/src/Avalonia.Base/Diagnostics/Diagnostic.Metrics.cs#L59)。
采用实际入口记录和按需开关，不引入这些框架的 tracing 后端、构建模式或完整工具体系。

## 分层与实现位置

| 位置 | 职责 |
|---|---|
| gui/diagnostics | 对外 Capture、Options、Snapshot 等 API，验证应用归属 |
| internal/diagnostic | 采集会话开关、原子资源计数、通用记录模型、帧历史与频率计算、冻结结果 |
| gui/diagnostics.go | 私有弱引用映射、读取 GUI 缓存元数据、关联本次会话的 GUI 采集器 |
| gui/root.go 等稳定入口 | 在真实工作发生处计数／计时；rootBase 封装帧阶段 |
| 绘制／排版后端及共享文字位图缓存 | 在既有成功操作处记录，不认识 Widget |
| gui/dev | 使用公开 diagnostics API 实现 HTTP 启停、查询 |

`gui/diagnostics -> gui`；`gui`、`gui/diagnostics`、`platform` 使用无上层依赖的
`internal/diagnostic`。gui 不反向导入 gui/diagnostics 或 gui/dev。
公开结果与内部记录共用纯值模型，不复制两套字段，也不包含 GUI／原生资源对象。

GUI 采集器只在启用后首次需要时创建，以内部 Source 查询接口关联到会话。
该接口仅在 Snapshot／Stop 查询，固定由框架内部实现，不是用户观察者或信号回调。
没有 init 注册或公共内核桥接 API。GUI 保留必要适配，是因为类型化弱引用和私有缓存
无法在外部直接读取；不为了分包额外暴露这些内部状态。
查询不调用用户重写的 Parent／Root／Rect／Snapshot，也不通过原生接口查询窗口信息。

## 统计口径

### 帧

每次根帧记录开始时间、总耗时及回调、状态样式更新、布局、Painter.Begin、Paint、
Painter.End 的耗时。这是经过的时间，包含阻塞等待，不等同 CPU 占用时间。
总耗时从 GUI 根帧入口到退出，不包含此前全部原生事件处理。
Styles 只计 updateStateStyles 阶段；布局／绘制中发生的样式解析仍计入相应阶段。
一次帧可有多次布局，累计布局时段；不叠加子控件的递归耗时。
极短阶段可能低于当前时钟的有效分辨率而记录为零，不能据此判断该阶段没有执行。

EndReturned 仅表示 End 返回。Completed 还要求绘制正常完成，没有中途取消或销毁。
Paint panic 仍照常传播，清理时返回的 End 不能使其计为完成帧。
End 的原始接收者及调用、释放顺序不因采集而改变。
RenderWidget 离屏请求、离屏 Widget.Paint 单列，不计入完成帧率。

每根 FPS 是近期 CPU 完成帧间隔的估计，不是 GPU／合成器／显示器刷新率。
缺少样本时为零；超过一秒未完成帧或根已暂停时 Idle，频率为零。恢复绘制后重新采样，
不让此前空闲拉低新的活动频率，也不从低频事件驱动刷新推断掉帧。

Window、Popover 不修改统计记录字段。rootBase 提供私有布局阶段值对象；Popover
在独立的 layoutContent 函数内结束阶段，避免 defer 把后续 Paint 算入布局。
关闭时阶段对象不读取诊断时钟，不在热路径创建回调函数或堆对象。

### Widget

- MeasureQueries／MeasureHits／MeasureCalls：measureWidget 缓存入口的查询、命中、
  实际调用 Measure 的次数，不宣称包含任意直接调用 Widget.Measure 的行为。
- PaintCalls／OffscreenPaintCalls：GUI 绘制遍历实际调用的 Paint，包含正常与离屏分类。
- StyleChangedCalls：框架消费样式失效、实际调用 StyleChanged 的次数，不是样式表修改次数。
- PaintRequests／LayoutRequests：WidgetBase 收到的请求次数，不等同合并后的帧数。

首版不统计逐控件 Arrange 次数。当前公开 Widget.Arrange 可以被自定义控件、widgets
包直接调用，私有包装函数无法完整拦截；WidgetBase.Arrange 是带 LayoutManager 的默认
实现，不是通用派发边界。矩形 setter 调用次数也不能代表 Arrange 或成功分配次数。
不为诊断新增 setter／派发 API，不要求控件手工报告，不改造原有布局行为。
以后若布局机制因独立需求统一了派发边界，再考虑加入可靠的 Arrange 指标。

### 资源

成功的 TextLayout 创建、尺寸调整、逻辑销毁和实际栅格化；graphics.Image 创建、更新、
逻辑销毁；文字位图缓存命中／未命中／失效；Icon 源调用和 DrawingArea 生成。
不包装或替换资源，不改变具体类型、线程亲和、延迟释放和失败回退。

资源按采集期间的进程活动汇总，含离屏及其它线程，不冒充某个 Widget／窗口独占开销。
PixelBytes 是提供的 RGBA 像素载荷，不是 GPU 显存；Software 的相同操作也会计数。
采集前已有资源的销毁也可观察，不能用创建减销毁推断当前存活量。
各资源字段单独原子读取，不承诺跨线程操作的一致事务快照；停止后的公开结果固定。
Direct2D 原生文字绘制不走文字位图缓存，零计数不意味着缓存不良。

## 容量、身份和生命周期

最近 120 帧、最多 64 个根与 4096 个 Widget。GUI 适配器使用类型化弱引用，
不保活控件或窗口；会话停止后释放适配器及应用归属，仅保留冻结的纯值结果。
引用只在该次采集中有效，独立于应用可选 ID 和 DevServer 的 ref。

元数据包括根类型、可用 ID、标题、DIP 尺寸、后端，以及 Widget 的类型、名称、
ID、根归属和根局部 DIP 矩形。只读缓存，不在每帧遍历树来补充信息。

记录满后仍更新已有项，直接跳过新项并累计 DroppedRoots／DroppedWidgets。
这些值是创建记录被跳过的次数，不是不同对象的数量。未记录不能解释为没有发生工作。
记录路径不在每次达到容量时扫描整张表。Snapshot 清理失效 Widget；根达到容量时，
查询先发布已关闭根的最后值，再回收其记录。历史引用的元数据可能因回收而缺失。

## 开发协议

仍需显式导入 gui/dev 并设置 GOUI_DEV_ADDR。路径 `/goui/dev/diagnostics`：

- GET 返回该端点开启的当前或已停止快照，未开启时 active=false。
- POST `{"enabled":true,"widgets":true}` 开始；已开启时不重置或更换选项。
- POST `{"enabled":false}` 停止并保留冻结结果。

协议适配通过已有 app.Post 进入 GUI 线程，不读取 gui 私有字段。
不随查询开启，不随语义 Snapshot 自动采集，不要求业务对象设置 ID。
JSON 的 Widget 字段使用 measureCalls／paintCalls／offscreenPaintCalls／styleChangedCalls，
不包含语义不可靠的 arranges。

## 验证

- 精确计数、缓存命中与实际执行分离、完成／中断、停止／重启、冻结副本与 GC 不保活。
- Arrange 原有行为不变，重复调用不产生诊断记录；容量满时记录不扫描清理，查询才回收。
- 查询不调用自定义 getter，Popover 布局计时不覆盖 Paint，失败时保留中断阶段。
- 开关前后 Software／Direct2D WARP 像素一致，资源创建／更新／延迟销毁语义不变。
- `tests/window/gui/diagnostics` 通过公开 API 验证主窗口／Popover、离屏、资源、空闲与停止。
  默认自动检查并退出；`-auto=false` 用于手工及 HTTP 协议检查，操作说明见程序入口。
- 同环境前后基准比较关闭状态及可选详细计数，检查分配；不外推应用实际 FPS。
- 完整测试、相关 race／vet、受影响目标编译及本机／远端验证；实际结果记录在 Plan。

本切片不含 UI Build 追踪、逐 Widget 资源归因和 GPU 计时，不以桌面启动或交叉编译
代替实际操作验收。后续缓存优化必须基于同环境、同工作量的测量证据。

## 显式性能浮层 HUD（2026-11-04）

### 定位与接入

HUD 属于 `gui/diagnostics`，由现有 Capture 持有，不扩展 Widget、Root、Painter
公共接口，不参与 Widget 树、布局、输入、焦点或语义快照。它通过已有
`Window.ConnectAfterPaint` 在内容和装饰后绘制。默认未启用。

```go
capture, err := diagnostics.Start(app, diagnostics.Options{})
if err != nil {
    return err
}
hud, err := capture.ShowHUD(window)
if err != nil {
    capture.Stop()
    return err
}
hud.SetPosition(geometry.Point{X: 12, Y: 48})
// hud.Destroy() removes only the overlay; capture.Stop() also releases it.
```

公共 API：

```go
func (c *Capture) ShowHUD(window gui.Window) (*HUD, error)
func (h *HUD) SetPosition(position geometry.Point)
func (h *HUD) Destroy()
```

所有调用在 GUI 线程；窗口必须是该次采集所属应用的存活窗口。同一窗口重复 ShowHUD
返回已有 HUD。初始位置为窗口局部 `(12, 12)` DIP，调用方可调整以避让标题栏按钮。
位置可为负，超出窗口的部分由现有绘制裁剪处理；非有限值 panic。
ShowHUD 报告初始文字资源创建及首次请求绘制的错误，中途失败释放已取得的资源。

只读 Snapshot、Start 的原有无重绘契约不变。显式显示、移动或移除 HUD 会请求一次
正常重绘以应用外观变化。HUD 使用 Application 的单次空闲 Timer，不新增 goroutine、
动画或持续 RequestPaint；普通采集与只读查询仍不启动 Timer。

### 指标、更新与开销

显示 Draw FPS、Frame／Layout／Paint／End 毫秒数及后端名称：

- Draw FPS 复用 Root 最近完成帧的间隔估计，不是显示刷新率。
- 阶段耗时取该窗口最近一份已完成帧，不读取正在绘制且尚未 End 的当前帧。
- Frame 是 GUI 帧的墙钟耗时，包含等待，不是 CPU 忙碌时间；End 不证明 GPU 完成或
  合成器实际呈现。不虚构 GPU 耗时、原生同步来源、掉帧率或高刷状态。
- 首次没有足够样本时显示 `--`，不显示虚假的零耗时或稳定帧率。
- HUD 文案注明 recent CPU frames。完成绘制后连续一秒没有新绘制时，额外重绘一次
  显示 `0 FPS`，保留最近完成帧耗时。初始样本不足仍显示 `--`。
  清零重绘自身不重新预约空闲 Timer；下一次普通绘制恢复采样和空闲检测。
  这次清零也是实际 CPU 帧，照常计入采集，不删除或伪造统计记录。

每次正常帧复用已有 TextLayout，最多每 250 ms 读取一次统计并尝试更新文字。
文字未变化时不重建布局；文字不可变的 TextLayout 仅在文本变化时替换。
空闲切换和恢复采样立即更新文字。普通帧只记录活动时刻，不逐帧重启 Timer；
单次超时检查最近绘制及已完成帧的实际完成时刻，必要时预约剩余延时，避免较慢的
Painter.End 导致过早清零。清零请求与原生绘制可能合并，后续普通帧恢复采样，
无需给 GUI 输入或帧分发增加诊断专用标记。
字体为 9 pt，使用固定深色半透明背景和浅色文字以便在不同主题下阅读，使用预乘颜色。
已有文字栅格与 Painter 图像缓存继续生效。暂时的更新失败保留上次可读资源。

通过 internal/diagnostic 的内部 Source 桥读取指定根的现有记录，不复制完整 Snapshot、
不遍历 Widget、不调用原生窗口 getter，也不为查询创建记录。RootRecord 额外保存最近
完成 Frame 的纯值，内存上界仍受现有根容量限制；普通记录路径不新增外部观察回调。

HUD 自身的排版、栅格化、图像和绘制操作计入采集，不人为扣除。需要测量应用本身时
关闭 HUD，继续使用手动 Snapshot；逐控件统计仍需显式开启 Widgets。

### 资源与结束

Capture 持有每个窗口一个 HUD，HUD 持有 AfterPaint／Destroy／Timeout 信号 handle、
一个 TextLayout 和一个 Application Timer。Destroy 幂等停止 Timer、断开信号，
释放文字并从 Capture 移除；已失效的超时通知不得请求绘制。
窗口销毁时不再请求重绘；显式 Destroy 和采集停止时请求一次重绘擦除浮层。
内部 Session 的停止信号保证 Application.Run 返回引发 StopOwner 时也完成清理，
不只依赖用户调用公开 Capture.Stop。停止后的采集结果继续冻结。
首版不为 Popover 增加 HUD；Popover 的统计仍在 Snapshot 中。

### 验证与示例

- 包内替身测试：重复显示、位置、所有权／失败清理、字体失败保留旧资源、窗口销毁、
  应用停止清理、无主动帧请求、250 ms 查询节流和相同文字布局复用。
- GUI 适配器测试：单根读取隔离，不创建记录／快照，返回最近完成帧，避免当前帧及
  其它窗口帧混入；采集停止后不可读取。
- `tests/window/gui/diagnostics -hud`：真实窗口检查透传点击、显示／移动／移除、
  连续帧、闲置仅一次清零帧／文字更新、之后帧与资源稳定、恢复绘制、
  RenderWidget 不包含 HUD、停止冻结及资源释放。
- Demo 的“窗口与检查”使用已有采集，提供显示 HUD 开关，跨页面保留。

GTK/GDK 4.24.1 的 FPS 定义见本文开头。Flutter
[PerformanceOverlay](https://api.flutter.dev/flutter/widgets/PerformanceOverlay-class.html)
展示帧阶段预算；GOUI 当前只显示实际具备的 GUI 阶段统计，不照搬其双线程渲染指标。

## 后续帧调度规划（未实现）

当前 `gui/root.go` 的主动重绘仍以 60 Hz 节流，本次 HUD 不改变帧调度。
后续独立切片保留 RequestPaint／ConnectFrame／animation 使用方式：

1. 将固定间隔改为窗口刷新节奏感知；未知时才用 60 Hz 兼容值，标明估算来源。
2. 接入原生绘制机会，按需预约、合并请求，隐藏／销毁停止，过期通知不得访问窗口。
3. platform 负责原生机会，GUI 负责失效与绘制阶段，Painter 继续只负责绘制／提交。
   具体平台接口在该切片前细化，不把 HUD 当作调度驱动器。

候选路径：macOS 14+ 的窗口／View 关联 CADisplayLink；Windows 11 的合成器时钟需
核对混合刷新率，Windows 10 先采用刷新周期感知定时降级；Linux/X11 评估 Present MSC
通知，缺失时以 RandR 信息定时降级。原生接口失败不能阻止正常绘制。

参考：[Apple NSWindow DisplayLink](https://developer.apple.com/documentation/appkit/nswindow/displaylink(target:selector:))、
[DCompositionWaitForCompositorClock](https://learn.microsoft.com/en-us/windows/win32/api/dcomp/nf-dcomp-dcompositionwaitforcompositorclock)
（最低 Build 22000，阻塞调用线程）、[XCB Present](https://xcb.freedesktop.org/manual/group__XCB__Present__API.html)。
调度机会不等同实际呈现反馈；不简单统一 SwapInterval(1)／Present(1)，也不在 GUI 线程
阻塞等待垂直消隐。真实高刷、跨屏和混合刷新率需实际硬件验证，VNC／RDP 只验证功能。
