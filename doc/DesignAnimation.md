# 帧更新与数值动画

状态：首版实现及指定三平台验收完成（2026-10-07）。以本文件为本切片的开发与验收契约。

## 设计与分层

Root 在布局、绘制前发送 ConnectFrame(time.Time)，连接本身不启动绘制。
唯一绘制请求仍是 RequestPaint。动画在启动时请求首帧，收到帧通知后计算值、
发送 Update，尚未结束则请求下一帧；结束或 Stop 时断开 signal.Handle。
多个动画共用 Root 的时间与绘制请求合并，不引入帧租约、取消函数或活动动画表。

animation 是独立包，只依赖标准库和 core，不依赖 gui/ui/platform。
GUI 的 Root 隐式满足 animation.FrameSource。数值动画在播放时接收帧源，
便于 UI 在窗口创建前保存动画，以及 Widget 转移后显式使用新 Root。

参考 GTK4 4.24.1 的 Update → Layout → Paint 顺序：
https://docs.gtk.org/gtk4/drawing-model.html 。GTK tick 注册持续驱动时钟，
GOUI 采用显式 RequestPaint：https://docs.gtk.org/gtk4/method.Widget.add_tick_callback.html 。
隐藏期间停止通知但时间继续经过的选择参考 Flutter AnimationController：
https://api.flutter.dev/flutter/animation/AnimationController-class.html 。
这些是官方 API 契约参考，本次不移植其渲染器或 TickerProvider。

## Root 契约

```go
ConnectFrame(func(now time.Time)) signal.Handle
```

Window、Popover 分别拥有帧信号；同帧所有回调收到同一个带单调时间信息的
time.Now 采样值。它不是实际或预测呈现时间。帧回调允许更新 Widget、请求布局、
替换内容或关闭窗口；回调后重新取得内容并检查存活及 surfaceEpoch。
销毁后连接无效，当前帧剩余回调不得操作已销毁对象。复用 signal.Handle，
不增加 DisconnectAll。RenderWidget 不发送帧信号，Paint 只读动画缓存值。

请求、更新、布局、绘制都在 GUI 线程。帧内 RequestPaint 只保留后续需求，
当前绘制返回后才安排平台请求。dirty 在帧开始消费，布局/绘制中新产生的
请求不得在帧结束清除。重入 PaintEvent 只记录需求，不递归执行帧。

GUI 主动请求按 time.Second/60 合并、节流；空闲首帧立即请求，下一次最早为
上次帧开始时间加间隔。每 Root 至多一个待处理平台请求或单次计时器任务，
复用 Application 的到期堆，不增加动画 goroutine。原生曝光/尺寸 PaintEvent
不被节流丢弃，提前到达会满足并取消延迟任务。慢帧不补播、不积压。
这不是原生 vsync，不承诺实际呈现帧率。

已知 Hidden/Minimized、Popover Hide 停止主动唤醒，保留 dirty；Show/恢复
请求一帧。Unknown 不等同隐藏，不推断完全遮挡。Root 销毁/表面释放取消任务，
旧表面的异步任务不能访问新表面。RequestPaint 返回调度受理结果，不承诺呈现；
异步平台提交失败停止重试、保留 dirty，使用 GUI 既有诊断方式，不退出应用。

## animation API

```go
type FrameSource interface {
    ConnectFrame(func(time.Time)) signal.Handle
    RequestPaint() error
}
type Interpolator[T any] func(from, to T, progress float64) T
type Curve func(progress float64) float64

func New[T any](initial T, interpolate Interpolator[T]) *Animation[T]
func (*Animation[T]) CurrentValue() T
func (*Animation[T]) SetValue(T)
func (*Animation[T]) AnimateTo(FrameSource, T, time.Duration, Curve) error
func (*Animation[T]) Running() bool
func (*Animation[T]) Stop()
func (*Animation[T]) Finish()
func (*Animation[T]) ConnectUpdate(func(T)) signal.Handle
func (*Animation[T]) ConnectFinished(func(error)) signal.Handle
```

New 只创建状态；nil 插值函数属于编程错误。所有操作与通知在帧源线程，
GUI 使用时即 GUI 线程，跨线程经 App.Post。使用后不可复制 Animation。
插值与曲线应是纯函数。内置 Float32/Float64 及 Linear/EaseInCubic/
EaseOutCubic/EaseInOutCubic；复合值、颜色由使用方提供插值。

AnimateTo 从最后已计算的 CurrentValue 开始，以 time.Now 记录开始时间；
替换旧运行不发送旧 Finished。nil 源、负时长参数错误保留原运行；nil 曲线
表示 Linear，零时长立即到终值并请求一次绘制。同步首帧请求失败返回 error，
不重复 Finished。运行中请求失败停止并通过 Finished(error) 一次报告。
Finished(nil) 仅代表数值到达目标，不代表呈现成功；Stop/替换不是完成。

CurrentValue 仅读取缓存。SetValue 停止、保存并发送 Update，不发送 Finished。
Stop 断开连接保留缓存，不重新按当前时间采样。Finish 对运行中的动画断开、
精确到目标、Update 后请求最终绘制并发送 Finished（请求失败为 error）。
Running 表示播放意图，不表示可见。
进度 clamp(elapsed/duration,0,1)，曲线可超出范围，终点直接赋 target。
曲线非有限结果作为该次运行错误报告。泛型值不使用反射猜测相等。

每次播放有版本号。调用插值/曲线/Update 后检查版本，防止回调 Stop、
重定向或销毁导致旧帧请求、旧完成或清理新连接。最终通知前先标记停止、
断开；最终 Update 改变运行后跳过旧 Finished。

Widget 卸载显式 Stop；重挂载按自身状态启动，Window/应用状态由拥有者在
销毁中 Stop。隐藏不冻结时间，恢复按实际经过时间采样，可能直接完成。
首版不包含 Pause/Resume/Repeat、时间轴、动画组、隐式属性动画。

## UI 与控件迁移

UI 持久保存 Animation，一次性 ConnectUpdate 到 ui.State.Set 或
App.RequestUpdate；声明读取 State.Get/CurrentValue。播放时用 FindWindow
或已有 Widget.Root 获取源。禁止每次 Build 创建动画或重复连接。
现有 RequestUpdate 经 Post 异步协调，首版不承诺动画更新参与当前 UI 绘制帧。
后续严格同帧协调另设通用调度切片；本次验证最终值、合并和生命周期。

Switch 使用 Animation[float32]；ProgressBar 以单 Frame 回调缓存相位；
TabBar 以单 Frame 回调推进现有位置、宽度、占位与滚动算法。
Paint/Snapshot/RenderWidget 不推进时间。保持样式、时长、输入与业务语义，
只调整帧驱动及明确的缓存读取。Timer 继续服务普通超时。

## 开发切片与验收

1. gui/root.go、frame.go、window.go、popover.go：帧信号、合并、单次唤醒、
   生命周期；包内 fake 时钟/调度测试，不依赖 Sleep。
2. animation 包：数值、曲线、重定向、Stop/Finish、错误、重入、多动画假帧测试。
3. widgets Switch/ProgressBar/TabBar：迁移并保留原运动公式与行为覆盖。
4. tests/window/gui/animation 与 ui/animation：中文入口说明、可见状态、
   本机后再 Windows/macOS 真机验收，检查结束空闲、隐藏恢复、尺寸拖动、
   多动画和离屏读值稳定。无需 DevServer。

每阶段运行针对性测试，最终 go test ./...、相关 race 与目标编译。
记录 OS、后端、缩放、通过/失败/跳过与未测场景；窗口启动不等于操作验收。
阶段记录同步 doc/Plan.md / Roadmap.md。

## 使用方式

命令式消费者在 Update 中调用已有 setter；一个 Root 可驱动任意多个动画：

```go
motion := animation.New(float32(24), animation.Float32)
motion.ConnectUpdate(icon.SetSize)
window.ConnectDestroy(motion.Stop)
err := motion.AnimateTo(window, 80, 400*time.Millisecond, animation.EaseOutCubic)
```

声明式消费者在 Build 外保存 motion / State，并仅连接一次：

```go
size := ui.MakeState(float32(24))
motion := animation.New(size.Get(), animation.Float32)
motion.ConnectUpdate(size.Set)
defer motion.Stop()
// Build 中读取：ui.Icon(source).Size(size.Get())
// 事件中播放：motion.AnimateTo(app.FindWindow("main"), 80, duration, curve)
```

不引入 ui.Animation 声明节点、属性追踪或 View 的永久对象身份假设。
完整例程见 tests/window/gui/animation 和 tests/window/ui/animation。

## 2026-10-07 实现与验证记录

- Root 共享信号、被动连接、请求合并、单次唤醒、帧内重入防护与生命周期完成；
  包内可控时间测试断言零任务空闲、曝光提前取消唤醒、隐藏恢复、销毁跳过后续连接、
  Frame → Measure → Arrange → Paint 与回调替换子树。
- animation 泛型缓存值、插值/曲线、Stop / Finish / 重定向及版本保护完成；
  明确终值和独立数值预期覆盖多动画、同步/异步错误与回调改变运行。
- Switch / ProgressBar / TabBar 不再各自创建循环 Timer；TabBar 的现有运动公式不变。
  生命周期与共享帧回归通过，Snapshot 不暴露动画内部调度状态。
- Windows 初轮窗口验收复现 `popover frame source not running`：UpdateWindow
  在原生 Show 内同步绘制，早于 Popover.Visible 提交。新建 Popover 默认暂停，
  成功 Show 后恢复，首次/再次显示一致；同步 Show 回归及三平台复验通过。
  无平台改动、额外延时或在 Paint 中启动动画的补偿。

通过的命令：

```sh
go test ./...
go test -race ./animation ./gui ./ui ./widgets ./widgets/ui
go vet ./animation
go vet -composites=false ./gui ./ui ./widgets ./widgets/ui
GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build ./animation ./gui ./widgets ./ui ./widgets/ui ./tests/window/gui/animation ./tests/window/ui/animation
GOOS=darwin GOARCH=arm64 CGO_ENABLED=0 go build ./animation ./gui ./widgets ./ui ./widgets/ui ./tests/window/gui/animation ./tests/window/ui/animation
```

默认 `go vet ./animation ./gui ./ui ./widgets ./widgets/ui` 仍报告既有未命名字段
结构体字面量（如 gui/text_editor.go、ui/text_editing_test.go），本次未修改或屏蔽
这些文件；上述关闭 composites 的命令只用于核对其余检查，不代表完整 vet 通过。

窗口验证均不用 DevServer：

| 环境 | 已通过范围 |
| --- | --- |
| Linux amd64、X11 / Xfwm4、Go 1.25.8 | GUI 用例 Software / OpenGL 配置各 1x/2x；UI 用例 Software 1x / OpenGL 2x。双动画终值、结束空闲、离屏一致、Popover 隐藏/恢复自动断言；OpenGL 1x 的 Start / Stop / Switch / Popup / 缩放及 TabBar 拖拽排序桌面操作。 |
| macOS 15.7.7 arm64、Go 1.24.9 | GUI / UI/widgets 包内测试；GUI OpenGL 默认配置及 UI 用例各 1x/2x 自动断言、进程退出码 0；桌面 Stop 保留值、Finish 精确终值、Switch、Popover 忙等待与窗口缩放。 |
| Windows 10 19045 amd64、Go 1.24.0 | GUI / UI/widgets 包内测试；Direct2D 偏好配置的 GUI / UI 用例各 1x/2x 自动断言及退出码 0；桌面 Stop / Finish / Snapshot / Switch / Popover / 窗口缩放。 |

Windows 工厂存在 D2D → OpenGL → Software fallback；这里只记录选择配置，
未新增探测接口确认运行实例类型。1x/2x 是应用缩放配置，不代表真实 Retina /
混合 DPI / 多显示器验收。未执行高刷新率、硬件 GPU 专项、Windows OpenGL、
远端 TabBar 跨窗口拖放专项与真实最小化/恢复全组合；共享暂停策略有逻辑覆盖。
本机全仓测试中的既有排版/像素测试按原行为写图片，不修改或清理用户文件。

远端传输曾遇到 PowerShell stdin 读取等待及 shell 元数据与 CMD 实际行为不符；
后续复用该 SSH 连接的 SFTP 完成上传。桌面输入需等待 Run 对话框就绪并检查结果，
不能把工具输入受理当作窗口启动。验收仅使用隔离的任务目录，没有修改系统防护配置。

当前边界：帧源是 GUI 主动请求节流，不是平台原生 vsync；UI 更新仍经异步协调，
不保证修改进入同一绘制帧。后续能力须另行设计，不将计时器或属性系统扩入本切片。
