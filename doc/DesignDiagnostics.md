# 可选绘制与资源诊断

## 目的与边界

区分框架测量缓存查询、Paint 调用、文字栅格化和图片操作，提供 GUI 线程帧阶段耗时。
默认关闭；不因诊断修改失效、缓存、布局、绘制、输入或帧调度策略。
本切片不同时优化缓存，不引入绘制指令缓存、局部重绘、HUD 或原生 vsync。

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
