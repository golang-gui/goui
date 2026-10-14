# goui GUI MVP 设计

> 状态：早期 GUI MVP 历史草案，不是当前公共 API 文档。
>
> 2026-10-14 复核：Application.Style／SetStyle、无参数 NewWindow、局部／树级
> 样式和早期最小 TextInput 等提案均已被后续设计取代，不列入未来路线。
> 当前接口使用 StyleSheet、NewWindow(options)、共享编辑核心、Root 帧通知和
> 明确 Widget 生命周期。以下原始章节保留设计推演，不应直接复制代码示意。
>
> 当前阅读入口：[架构原则](Design.md)、[应用入口](DesignApp.md)、
> [样式](DesignStyle.md)、[Widget Painter](DesignWidgetPainter.md)、
> [布局](DesignLayout.md)、[文本编辑](DesignTextEditing.md)、
> [手势](DesignGesture.md)、[未来路线](Roadmap.md)。\
> 适用范围：命令式 GUI 层 MVP\
> 核心目标：先验证控件树、布局、绘制、事件传播、信号和大模型集成入口，不在 MVP 阶段引入完整属性系统。

## 1. 设计目标

GUI MVP 层建立在 `platform` 之上，提供应用可直接使用的命令式控件 API。第一阶段重点不是做完整控件库，而是打通以下闭环：

- `Application` 创建平台、事件循环、typography 上下文和窗口；
- `Window` 持有顶层 `Widget`，负责布局、绘制、事件分发和生命周期；
- `Widget` 组成树结构，支持子控件、布局管理、绘制和语义描述；
- `EventDispatcher` 在窗口内执行三阶段事件传播；
- `EventController` 添加到控件上，将平台输入事件解释为控件信号；
- 信号用于向应用暴露控件语义事件；
- 平台外观和输入配置统一抽象为 GUI 可消费的应用环境；
- 样式系统服务于 GUI 绘制阶段，提供轻量的树级样式和控件本地样式；
- 预留大模型集成入口，用同一套控件树描述和事件分发机制驱动 UI。

MVP 阶段暂不实现完整属性系统。控件状态使用普通 Go 字段保存，公开 API 使用 `SetXXX` 方法；setter 内部负责触发布局和绘制失效。

声明式层使用 `View` 概念，位于命令式 GUI 之上：

```text
View -> Widget -> platform
```

`View` 不直接操作平台，也不替代 `Widget`。最小声明式层只负责把声明式树 reconcile 到命令式 `Widget` 树，复用现有 setter、信号、布局、绘制和事件分发能力。

## 2. Platform 补充

`platform/common.Window` 增加主动请求重绘的方法：

```go
type Window interface {
    // RequestPaint asks the platform to schedule a PaintEvent.
    // It does not draw immediately, and multiple calls may be coalesced.
    RequestPaint() error
}
```

命名与 `RequestClose` 保持一致。语义是请求平台窗口在后续产生 `PaintEvent`，真正绘制仍发生在 `PaintEvent` 处理中。

平台映射：

- Windows：`InvalidateRect`
- macOS：`setNeedsDisplay:`
- X11：触发等价的 Expose/重绘请求

GUI 层的 `Widget.RequestLayout` 和 `Window.RequestPaint` 最终都收敛到 platform `RequestPaint`。

## 3. Signal

保留信号概念，用于控件向应用暴露高层语义事件，例如按钮点击、窗口关闭请求和窗口销毁。

信号同步触发，不支持 queued。跨 goroutine 更新 UI 必须显式通过 `Application.Post` 进入 UI 线程。

信号连接返回 `Handle`：

```go
type Handle interface {
    Block()
    Unblock()
    Disconnect()
}
```

MVP 提供最多三个入参的信号类型：

```go
type Signal0 struct{}
type Signal1[T any] struct{}
type Signal2[T1, T2 any] struct{}
type Signal3[T1, T2, T3 any] struct{}
```

更多入参通过结构体承载。信号不支持返回值。需要调用方影响流程时，通过入参对象表达，例如关闭请求。

`core/signal` 不感知 GUI 对象的生命周期，也不以批量断开连接替代销毁安全检查。窗口的状态、焦点和关闭请求通知在每个用户回调入口检查窗口是否已销毁；前一个回调销毁窗口后，后续普通通知回调不再进入。连接仍由订阅方通过 `Handle` 管理，跳过回调不承诺立即释放闭包或其捕获对象；不再需要时应断开连接并释放 handle。

`Window.Destroy` 是生命周期通知，已连接的销毁回调仍按顺序执行，保留 `Handle` 的阻塞和断开语义。销毁开始后，所有窗口 `Connect` 方法只返回无操作的 handle。上述保护针对 UI 线程内的同步重入，不使窗口 API 支持并发操作。

## 4. 主要 GUI 类型

### 4.1 Application

`Application` 是命令式 GUI 层的单例入口，负责：

- 创建和持有 `platform.Platform`；
- 创建和持有 `platform.EventLoop`；
- 创建和持有应用级 `typography.Context`；
- 创建和持有应用级 `Settings`；
- 持有应用级全局 `Style`；
- 管理所有 GUI `Window`；
- 提供 UI 线程调度入口；
- 预留大模型集成的查询和事件投递方法。

草案接口：

```go
type Application interface {
    Platform() platform.Platform
    Typography() typography.Context
    Settings() Settings
    Style() Style
    SetStyle(Style)
    ConnectStyleChanged(func()) signal.Handle
    NewWindow() (Window, error)
    Run()
    Quit()
    Post(func())

    Windows() []Window
    Snapshot() ApplicationInfo
}
```

HTTP 开发协议由独立 gui/dev 提供。它通过 Windows / Window.Popovers 枚举宿主，包装 Snapshot 并自行维护 ref；输入经宿主 DispatchEvent / CommitText 派发，不在 Application 增加协议寻址能力。详见 [DevAutomation](DesignDevAutomation.md)。

`Settings` 是 GUI 层消费平台和用户设置的入口。它由 `Application` 持有，窗口、控件和事件控制器通过所属应用读取。设置值在启动时从平台层读取，平台设置变化时由 platform 层通知到 GUI 层。MVP 当前只定义接口边界，不急于实现每个平台的设置读取。

### 4.2 Window

`Window` 不实现 `Widget`。它是平台窗口和 GUI 控件树之间的边界，负责：

- 持有 `platform.Window`；
- 持有本窗口的 `graphics.Painter`；
- 持有顶层 `Widget`；
- 持有本窗口的 `EventDispatcher`；
- 管理窗口尺寸、缩放和绘制；
- 管理关闭请求和销毁生命周期；
- 为大模型集成提供窗口级快照和事件投递入口。

`Window` 实现 `Root`，但不实现 `Widget`。`Root` 是一棵 Widget 树挂载到宿主后的最小边界，只提供查询当前 root widget 和请求重绘的能力。root widget 内部挂载到 `Root`，但公开的 `rootWidget.Parent()` 返回 `nil`；普通 widget 的 parent 是另一个 `Widget`。窗口之间未来的 owner/transient/child window 关系不属于 Widget containment tree，应单独设计。

草案接口：

```go
type Root interface {
    Widget() Widget
    RequestPaint() error
}

type Window interface {
    Root

    SetWidget(Widget)

    ID() string
    SetID(string)

    Title() string
    SetTitle(string) error

    Focused() bool
    FocusedWidget() Widget
    SetFocusedWidget(Widget) bool

    Show() error
    RequestClose() error
    Destroy()

    Snapshot() WindowInfo
    DispatchEvent(event events.Event) error

    ConnectCloseRequest(func(*bool)) signal.Handle
    ConnectDestroy(func()) signal.Handle
    ConnectFocusChanged(func(bool)) signal.Handle
}
```

窗口关闭流程：

1. 收到 platform `CloseEvent`；
2. 创建 `allow := true`；
3. 触发 `CloseRequest` 信号；
4. 如果监听方禁止关闭，则流程结束；
5. 触发 `Destroy` 信号；
6. 释放 Widget 兜底资源；
7. 释放 painter、platform window 等窗口资源。

`CloseRequest` 示例：

```go
win.ConnectCloseRequest(func(allow *bool) {
    *allow = false
})
```

没有监听者时窗口默认允许关闭。

窗口自身 focus 和控件 focus 分开处理：

- `Window.Focused()` 表示平台窗口是否拥有系统输入焦点；
- 窗口内控件焦点通过 `Window.SetFocusedWidget(widget)` 显式设置；
- `Window.SetFocusedWidget(nil)` 表示清除窗口内控件焦点，不提供有歧义的 `ClearFocus()`；
- MVP 暂不实现平台 `Window.RequestFocus()`，窗口系统焦点只由 platform `FocusEvent` 更新。

### 4.3 Widget

`Widget` 是控件树节点。一个窗口有一个顶层 `Widget`，每个 `Widget` 可以有多个子控件、多个 `EventController` 和一个 `layout.LayoutManager`。

草案接口：

```go
type Widget interface {
    base() *WidgetBase
    Parent() Widget
    Root() Root
    Window() Window

    ID() string
    SetID(string)

    Visible() bool
    SetVisible(bool)

    Focusable() bool
    SetFocusable(bool)
    Focused() bool
    ContainsFocus() bool
    ConnectFocused(func(bool)) signal.Handle
    ConnectContainsFocus(func(bool)) signal.Handle

    Rect() geometry.Rectangle

    EventControllers() []EventController
    AddEventController(EventController)
    RemoveEventController(EventController)

    LayoutManager() layout.LayoutManager
    SetLayoutManager(layout.LayoutManager)

    Measure(c layout.Constraint) layout.Measurement
    Arrange(rect geometry.Rectangle)
    Paint(p Painter)
    RequestLayout()
    ConnectMount(func()) signal.Handle
    ConnectUnmount(func()) signal.Handle

    Snapshot() WidgetInfo
}

type Container interface {
    Widget

    AddChild(Widget)
    RemoveChild(Widget)
    Children() []Widget
}
```

MVP 显式要求控件嵌入 `WidgetBase`。`Widget` 包含包内未导出的 `base() *WidgetBase` 方法，外部控件通过嵌入 `gui.WidgetBase` 自动满足这个约束。`WidgetBase` 承载通用字段、默认实现和树节点状态；它是控件开发 helper，不是面向应用代码的公开基类协议。`WidgetBase` 使用 Go 零值作为有效状态，默认可见，不需要 `Init()`。`Widget` 不暴露公开的 `Base()`、`IsWidget()` 或 owner/self 回指针。

树操作只使用接口方法，不保留包级 `Parent`、`GetChildren`、`AddChild`、`RemoveChild`、`GetRoot` 或 `GetWindow` 这类函数。`WidgetBase` 默认实现 `Parent()`、`Root()`、`Window()`、`RemoveChild()` 和 `Children()`；`AddChild` 需要知道真正的外层 parent，因此由具体容器显式实现：

```go
func (b *Button) AddChild(child Widget) {
    b.WidgetBase.AddChild(b, child)
}
```

`WidgetBase.AddChild(parent, child)` 是给自定义容器实现 `Container.AddChild(child)` 使用的公开 helper，因为外部包无法调用私有方法。其它树操作仍由 `WidgetBase` 内部私有 helper 统一处理 reparent、同 root 保留生命周期、跨 root mount/unmount 等树语义。`Parent()` 只读取直接 widget parent；`Root()` 和 `Window()` 需要沿 parent 链追溯；`Container.Children()` 返回 children 快照，会 clone 内部 slice。

后续如果树节点逻辑继续增长，可以把 `WidgetBase` 中的节点状态拆成独立的 `node.TreeNode` 或其它组合式实现。

`EventControllers()` 返回控件当前挂载的事件控制器快照，供 `EventDispatcher` 和调试工具读取。应用代码也可以通过 `AddEventController` 增加自己的控制器，并通过 `RemoveEventController` 移除控制器。后续如果控件需要保护内部控制器，例如 `Button` 内部的 click controller，可以覆盖 `RemoveEventController` 来拒绝删除。

控件 setter 负责触发失效：

```go
func (l *label) SetText(text string) {
    if l.text == text {
        return
    }
    l.text = text
    l.clearTextLayout()
    l.RequestLayout()
}
```

`Widget.RequestLayout()` 表示控件请求重新测量和安排，并隐含一次重绘请求。它不是同步布局调用；MVP 阶段采用窗口级全量布局 dirty flag，多次控件属性或布局变化会合并到下一次 platform `PaintEvent`，并在绘制开始前统一执行顶层 `Widget` 的 `Measure` 和 `Arrange`。

纯视觉状态变化，例如 hover 或 pressed，不需要重新布局时，可以通过 `widget.Window().RequestPaint()` 请求窗口重绘。

`Widget` 必须能一路追溯到所属 `Window`，用于请求重绘、连接窗口销毁信号和获取窗口上下文。

坐标语义：

- `Widget.Rect()` 是布局系统内部使用的矩形，坐标相对父 `Widget`；
- 顶层 `Widget.Rect()` 通常是窗口局部坐标 `(0,0,width,height)`；
- `WidgetInfo.Bounds` 是语义快照使用的窗口局部绝对坐标，便于自动化和大模型直接合成窗口坐标系下的输入事件。

控件焦点规则：

- `Window` 持有当前 `FocusedWidget`；
- Pointer down 命中控件后，从命中控件向父级查找最近的 `Focusable` 控件并设置焦点；
- 点击 `Button` 内部的 `Label` 时，焦点应落到 `Button`，而不是 `Label`；
- `KeyEvent` 以当前 `FocusedWidget` 为目标；没有 focused widget 时可以退回顶层 `Widget`；
- 窗口失去系统焦点时不清除 `FocusedWidget`，窗口重新获得系统焦点后继续使用原控件焦点；
- 控件被隐藏、移除或销毁时，如果它或其子树持有焦点，则窗口清除焦点。

## 5. 测量、布局和绘制

### 5.1 Measure 和 Arrange

控件布局分为测量和安排两个阶段：

```go
type Widget interface {
    Measure(c layout.Constraint) layout.Measurement
    Arrange(rect geometry.Rectangle)
}
```

语义：

- `Measure` 在父级给出的 `Constraint{Min, Max}` 下返回控件期望尺寸；
- `Measurement.Size` 必须落在约束内；可选的 `Baseline` 是相对控件顶边的
  第一条文字基线，并通过 `HasBaseline` 区分“不存在”和数值 0；
- `Arrange` 写入控件最终矩形，并安排子控件的位置和大小；
- 尺寸单位是逻辑像素；
- `Measure` 不应改变控件树结构，但可以更新可复用的测量缓存；
- `Arrange` 是正式写入 `Rect()` 的阶段。

`WidgetBase` 缓存最近一个 `(Constraint, Measurement)`。框架内部的
`layout.Child` adapter、Window/Popover host 和专用容器统一通过缓存入口测量；
`RequestLayout` 会失效当前控件和祖先，全局样式变化会失效整棵控件树。
缓存不跳过 `Arrange`，因此不会把上一次位置误当成当前布局结果。

使用 `Arrange` 而不是 GTK 的 `Allocate`，避免在 Go 语境中和内存分配产生歧义。

### 5.2 Layout

布局算法从 `gui` 包中分离到独立 `layout` 包。布局算法不依赖 `Widget`，只关心可以测量和安排矩形的布局子项。

```go
package layout

type Child interface {
    Measure(c Constraint) Measurement
    Arrange(rect geometry.Rectangle)
    MainWeight() float32
}

type LayoutManager interface {
    Measure(children []Child, c Constraint) Measurement
    Arrange(children []Child, rect geometry.Rectangle)
}
```

`WidgetBase` 在执行布局时负责把可见子控件转换为内部 adapter 组成的
`[]layout.Child`。布局算法只处理可测量、可安排的布局子项，不知道 Widget 的
parent、window、controller 或语义信息。`LayoutManager.Arrange` 接收的 `rect`
是父控件局部坐标系中的内容区域；子项 `Arrange` 得到的矩形也在同一坐标系。

当前基础布局包括：

```go
type LinearLayout struct {
    Direction  Direction
    Spacing    float32
    Padding    float32
    MainAlign  MainAlign
    CrossAlign CrossAlign
}

type FillLayout struct{}
```

`LinearLayout` 是单主轴有序布局。无 weight 的子项保持自然尺寸；
默认 `MainStart`，`CrossDefault` 在水平盒解析为 Center、垂直盒解析为 Start；
显式设置的 `CrossStart` 不受方向切换影响。
`MainWeight > 0` 的子项以自然尺寸为基线分配额外空间，并在最终主轴尺寸下
重新测量，以支持文字换行产生的 height-for-width。`CrossBaseline` 在水平盒中
使用 child 的第一文字基线计算共同 ascent/descent；垂直盒中退化为 Start。
这不等同于 CSS Flexbox：暂不提供独立 basis/grow/shrink/fit。

`FillLayout` 把所有子项安排到同一矩形，适合显式要求填满的内容和 overlay 式叠放。
单个 child 需要在较大槽位中保持自然尺寸并定位时，复用只有一个 child 的
`LinearBox`：HBox 的 `MainAlign` 控制水平轴、`CrossAlign` 控制垂直轴，VBox
则相反。主轴拉伸使用 child `MainWeight`，交叉轴拉伸使用 `CrossStretch`。
不提供独立 `AlignLayout` / `AlignBox`，避免在 Main/Cross 之外再引入一套绝对
水平/垂直对齐枚举。Button 等复合控件的内容位置由控件自身实现；Grid、Overlay
等未来容器需要逐项定位时，再由相应布局管理明确的专属数据。

后续多行流式布局使用独立 `WrapBox`，二维轨道布局使用 `Grid`；具体布局专属
数据仍由对应容器管理。只有实际需求出现 basis/grow/shrink/fit 时才重新评估
是否需要单独的 `FlexBox`。

### 5.3 Paint

`Window` 收到 platform `PaintEvent` 后执行 GUI 绘制流程：

1. 如果 layout dirty，则对顶层 Widget 执行 `Measure` 和 `Arrange`；
2. 调用 `graphics.Painter.Begin(width, height, scale)`；
3. 清理窗口背景；
4. 从顶层 Widget 开始递归 `Paint`；
5. 调用 `graphics.Painter.End()`。

控件绘制使用相对自身原点的局部坐标。GUI 内核在控件自绘返回后自动遍历
子节点，为每个 Widget 累加窗口原点并与祖先结构区域叠加裁剪；Painter 本身
不知道 Widget 树。

```go
func (b *button) Paint(p Painter) {
    size := b.Rect().Size
    p.FillRoundRect(geometry.Rect(0, 0, size.Width, size.Height), 6, buttonBrush)
}
```

需要在同一次 `Paint` 内临时改变 clip 或 transform 时使用成对的
`Painter.Save/Restore`。完整契约见 `doc/DesignWidgetPainter.md`。

`SetPaintFunc` 可以作为后续自定义控件便利 API，但不是 MVP 的主要机制。MVP 使用 Go 接口方法覆盖：`WidgetBase` 提供默认 `Measure`、`Arrange`、`Paint`，具体控件按需实现自己的方法。

控件绘制时应通过所属窗口或应用获取当前环境和计算后的样式，而不是直接读取平台 API。平台主题色、暗色模式、控件状态和本地样式在绘制前被解析为 GUI 层可消费的值，控件只关心“当前应该使用什么颜色、圆角、边距或描边”。

## 6. EventDispatcher 和 EventController

### 6.1 传播模型

每个 `Window` 包含一个 `EventDispatcher`。`EventDispatcher` 负责在本窗口的 `Widget` 树内执行三阶段事件传播。

```go
type EventDispatcher struct{}

func (d *EventDispatcher) DispatchEvent(window Window, event events.Event) error
```

传播阶段：

```go
type PropagationPhase int

const (
    PhaseCapture PropagationPhase = iota
    PhaseTarget
    PhaseBubble
)
```

传播流程：

1. 根据事件确定目标控件；
   - Pointer/Wheel：根据坐标 hit-test；
   - Key：以窗口当前 `FocusedWidget` 为目标；没有 focused widget 时退回顶层 `Widget`；
   - Focus：根据窗口当前 `FocusedWidget` 同步 focus crossing，不作为普通 widget event 三阶段传播；
   - 其他窗口事件由 `Window` 自身处理。
2. 构造 root 到 target 的路径；
3. Capture：root 到 target；
4. Target：target；
5. Bubble：target 到 root；
6. 任一 controller 停止传播后，后续传播终止。

### 6.2 EventController

普通应用代码通常不直接处理 platform event，而是监听控件信号。`EventController` 是控件内部的输入解释器，负责把平台事件转为控件状态和信号。

```go
type EventController interface {
    Phase() PropagationPhase
    Reset()
    HandleEvent(ctx EventContext)
    HandleCrossing(ctx CrossingContext)
}
```

`EventController` 接口不暴露 `Widget()` 或 `SetWidget()`。控制器可以有内部状态，例如 hovered、pressed、lastClickTime，但不应该在接口层拥有控件。控件开发者通过连接 controller 的信号或回调闭包来更新自身状态：

```go
motion := NewMotionEventController()
motion.ConnectContainsHover(func(hover bool) {
    b.hovered = hover
    _ = b.Window().RequestPaint()
})
```

框架内部如果需要 controller attach/detach 生命周期，应通过非公开机制或可选接口处理，避免把 controller 和控件树所有权绑定到公共 API。

传播控制放在上下文中：

```go
type EventContext interface {
    Event() events.Event
    Position() (geometry.Point, bool)
    StopPropagation()
    PropagationStopped() bool
}
```

GUI 层不额外定义公开通用 `Event` 类型。事件输入沿用 `platform/events` 的只读事件结构，传播控制由 `EventContext` 承担。`Position()` 返回当前 controller 所属 widget 的局部坐标；Key 和 Focus 这类无坐标事件返回 `false`。`EventContext` 不暴露 `Phase()`，controller 已经通过自身 `Phase()` 声明传播阶段。

提供一个轻量的 `EventControllerBase` 辅助类型保存 phase 等通用字段，并提供空的 `Reset`、`HandleEvent`、`HandleCrossing` 实现，减少简单控制器的重复代码；它不是接口要求，用户自定义控制器只需要实现 `EventController`。

### 6.3 Crossing notification

`crossing notification` 是 `EventDispatcher` 在路由层计算出的内部通知，表示 pointer 或 focus 的命中路径从 `oldPath` 变化到 `newPath`。它不是公开的 GUI 事件，也不是 `platform/events.Event`，应用代码不能直接投递 crossing。

路径统一定义为：

```text
path = root widget -> ... -> target widget
```

pointer 路径来自 hit-test：

```text
pointerPath = hit-test(pointer position)
```

focus 路径来自窗口内逻辑焦点：

```text
focusPath = path(window.FocusedWidget())
```

`EventDispatcher` 的职责是维护旧路径、计算新路径、比较差异，并把路径关系变化通知给 `WidgetBase` 和 `EventController`。`EventDispatcher` 不保存、读取或直接修改 `Hover`、`ContainsHover`、`Focused`、`ContainsFocus` 这类语义状态。

crossing 使用三个维度描述：

```go
type CrossingType int

const (
    CrossingPointer CrossingType = iota
    CrossingFocus
)

type CrossingMode int

const (
    CrossingTarget CrossingMode = iota
    CrossingContains
)

type CrossingDirection int

const (
    CrossingEnter CrossingDirection = iota
    CrossingLeave
)

type CrossingContext interface {
    Type() CrossingType
    Mode() CrossingMode
    Direction() CrossingDirection
    Position() (geometry.Point, bool)
}
```

含义：

- `CrossingType`：pointer 路径变化还是 focus 路径变化；
- `CrossingMode`：直接 target 关系变化还是 contains 关系变化；
- `CrossingDirection`：进入还是离开；
- `Position()`：pointer crossing 返回当前 widget 局部坐标，focus crossing 返回 `false`。

状态不变性由控制器维护：

```text
MotionEventController.Hover()        == pointerPath.target == controller widget
MotionEventController.ContainsHover()  == pointerPath contains controller widget

Widget.Focused()                      == focusPath.target == widget
Widget.ContainsFocus()                == focusPath contains widget
```

因此，当子控件获得 hover 或 focus 时，父控件必须获得 `ContainsHover` 或 `ContainsFocus`，但父控件不应因此获得 `Hover` 或 `Focused`。

crossing notification 不支持 `StopPropagation()`。它维护的是 widget tree 的全局路由状态不变性，不能因为某个控制器消费了普通输入事件而跳过父控件状态更新。普通输入事件仍然按三阶段传播，并且仍然可以停止传播。

带坐标的 pointer 输入需要先同步 pointer path。不能假设平台在 `PointerDown` 前一定发送过 `PointerMove`，因此 `PointerMove`、`PointerDown`、`PointerUp` 和平台 `PointerEnter` 都会触发 pointer crossing 计算。

pointer 输入的处理顺序：

```text
PointerMove / PointerDown / PointerUp
  -> EventDispatcher hit-test
  -> old pointerPath / new pointerPath 生成 pointer crossing notification
  -> MotionEventController 更新 Hover / ContainsHover
  -> 输入事件本身按 Capture / Target / Bubble 传播
  -> StopPropagation 只影响本次输入事件后续控制器
```

平台层 `PointerEnter`、`PointerLeave` 只表示指针进入或离开平台窗口。GUI 层不会把 hover crossing 暴露为普通 widget 级 `PointerEnter` / `PointerLeave` 事件；需要 hover/enter/leave 语义的控件应组合 `MotionEventController` 并监听其信号。

pointer crossing 的顺序：

```text
离开：
1. old target 收到 `CrossingTarget + CrossingLeave`
2. oldPath 从 target -> root 发送 `CrossingContains + CrossingLeave`

进入：
1. newPath 从 root -> target 发送 `CrossingContains + CrossingEnter`
2. new target 收到 `CrossingTarget + CrossingEnter`
```

例如从 `Button` 移入其子控件 `Label`：

```text
oldPath = root -> Button
newPath = root -> Button -> Label
```

变化为：

```text
Button: CrossingTarget + CrossingLeave
Label:  CrossingContains + CrossingEnter
Label:  CrossingTarget + CrossingEnter
```

最终状态：

```text
Button.Hover = false
Button.ContainsHover = true
Label.Hover = true
Label.ContainsHover = true
```

focus 采用同一套 crossing 模型：

```text
Window.SetFocusedWidget(widget)
  -> old focusPath / new focusPath
  -> WidgetBase 更新 Focused / ContainsFocus
  -> EventController.HandleCrossing 收到 CrossingFocus
```

平台窗口 `FocusEvent(false)` 不清空 `FocusedWidget`，只表示窗口失去系统输入焦点；窗口重新获得系统焦点后继续使用原逻辑焦点。

### 6.4 MotionEventController 和 hover 状态

MVP 提供更接近 GTK4 `GtkEventControllerMotion` 的 `MotionEventController`：

```go
type MotionInfo struct {
    Position  geometry.Point
    Modifiers events.Modifiers
}

func NewMotionEventController() *MotionEventController

func (c *MotionEventController) Phase() PropagationPhase
func (c *MotionEventController) SetPhase(PropagationPhase)
func (c *MotionEventController) Hover() bool
func (c *MotionEventController) ContainsHover() bool
func (c *MotionEventController) ConnectMotion(func(MotionInfo)) signal.Handle
func (c *MotionEventController) ConnectHover(func(bool)) signal.Handle
func (c *MotionEventController) ConnectContainsHover(func(bool)) signal.Handle
```

`MotionEventController.ConnectMotion` 处理真实 `PointerMove`，走正常事件传播阶段并受 `StopPropagation()` 影响；`Hover` / `ContainsHover` 状态更新来自 pointer crossing notification，不受传播停止影响。

### 6.5 ClickEventController

`ClickEventController` 作为通用控制器，而不是 `Button` 的私有实现。它识别基础 pointer down/up click，并暴露 pressed 状态变化；系统双击间隔、移动阈值、double click 和 pointer capture 后续再补。

传播阶段改为 `Bubble: target -> root` 后，`Button` 可以只挂一个默认 bubble 阶段的 click controller：命中 Button 自身时会在 bubble 阶段处理，命中 Button 子内容时会冒泡到 Button；如果子控件停止传播，Button 不触发 click。

`ClickEventController` 自身不主动停止传播。`ConnectPressed` 和 `ConnectClicked` 都传入 `EventContext`，由信号处理者决定是否调用 `StopPropagation()`。

```go
func NewClickEventController() *ClickEventController

func (c *ClickEventController) Phase() PropagationPhase
func (c *ClickEventController) SetPhase(PropagationPhase)
func (c *ClickEventController) Button() events.PointerButton
func (c *ClickEventController) SetButton(events.PointerButton)
func (c *ClickEventController) Pressed() bool
func (c *ClickEventController) Reset()
func (c *ClickEventController) ConnectPressed(func(EventContext, bool)) signal.Handle
func (c *ClickEventController) ConnectClicked(func(EventContext)) signal.Handle
```

pointer capture、触摸和更复杂手势后续再扩展到独立控制器。

### 6.6 KeyEventController

`KeyEventController` 是基础键盘事件控制器。它不负责文本转换、快捷键或 IME，只把 `events.KeyEvent` 按传播阶段转为 key down/up 信号。控件自行决定是否消费事件：

```go
func NewKeyEventController() *KeyEventController

func (c *KeyEventController) Phase() PropagationPhase
func (c *KeyEventController) SetPhase(PropagationPhase)
func (c *KeyEventController) Reset()
func (c *KeyEventController) ConnectKeyDown(func(EventContext, events.KeyEvent)) signal.Handle
func (c *KeyEventController) ConnectKeyUp(func(EventContext, events.KeyEvent)) signal.Handle
```

默认阶段为 `PhaseTarget`。`TextInput` 使用它监听 key down，并在实际修改文本或移动 caret 后调用 `StopPropagation()`。

### 6.7 控件信号示例

`Button` 在语义上是一块可点击并触发动作的区域。它不拥有固定的文本、图片或样式；
按钮内是文本、图片还是更复杂的组合，由子控件和布局决定。Button/MenuButton
默认使用水平 LinearLayout 的 `MainCenter`/`CrossCenter` 将自然尺寸的内容居中。
Button 的骨架下限、用户 Min/Max 和父约束在测量 child 前合并，上传的 baseline
包含实际居中偏移。需要填满时显式设置 FillLayout；复杂内容可以放入 LinearBox
等容器后再作为按钮的唯一内容。

MVP 阶段 `Button` 绘制一个简单圆角矩形背景：正常状态为灰色，hover 状态变亮，pressed 状态变暗。真实输入和模拟输入都先进入 `EventDispatcher`，后续由按钮内部组合的公共 `MotionEventController` 和 `ClickEventController` 根据 pointer move/down/up 更新状态，并在 click 成立时触发 `Button.Clicked` 信号。

```go
type Button interface {
    Widget
    ConnectClicked(func()) signal.Handle
}
```

应用层不需要处理原始 pointer event。

### 6.8 TextInput MVP

`TextInput` MVP 是单行基础文本输入控件，用于验证 focus、key event、文字测量绘制、caret 和语义快照闭环。它暂不实现完整文本编辑器能力。

边界：

- 只维护普通字符串文本和 caret 位置；
- 内部 caret 使用 UTF-8 byte offset，与 `TextCluster.Start` / `TextCluster.Length` 对齐；
- 只处理 `KeyDown`；
- `Control`、`Alt` 或 `Super` 按下时不插入字符，避免吞掉快捷键；
- 基于当前 `events.KeyEvent` 的 `Key` 和 `Modifiers` 做有限 ASCII 映射；
- 支持 Backspace、Delete、ArrowLeft、ArrowRight、Home、End；
- 不支持选择区、剪贴板、撤销、多行、密码模式、placeholder、滚动、IME 组合态、死键和完整键盘布局。

接口草案：

```go
type TextInput struct {
    WidgetBase
}

func NewTextInput() *TextInput
func (t *TextInput) Text() string
func (t *TextInput) SetText(string)
func (t *TextInput) TextFormat() typography.TextFormat
func (t *TextInput) SetTextFormat(typography.TextFormat)
func (t *TextInput) ConnectText(func(string)) signal.Handle
```

`TextInput` 默认 focusable，并内置一个 `KeyEventController`。文本改变触发布局、绘制和语义更新；caret 移动只触发绘制。

绘制：

- 简单白色背景和灰色边框；
- focused 时使用更明显的边框；
- 文本使用 `typography.TextLayout` 绘制；
- caret MVP 不做闪烁；
- caret 位置从 `TextLayout.MeasureMetrics()` 返回的 `TextCluster` 信息计算，不通过重复测量前缀文本实现。

语义：

- `Role = RoleTextInput`；
- `Text = t.Text()`；
- `Focusable`、`Focused` 和 `ContainsFocus` 由 `WidgetBase` 提供。

## 7. Settings 和样式系统

### 7.1 Settings

平台和用户设置通过 `Application` 持有的 `Settings` 暴露给 GUI 层。控件和事件控制器不直接调用平台 API，而是读取 GUI 层归一化后的设置。

设置读取方式应下沉到 platform 层，因为不同平台的数据来源不同：

- Windows：系统参数、注册表、DWM、WinRT 或消息通知；
- macOS：`NSUserDefaults`、`NSAppearance`、系统通知；
- Linux：XSettings、settings portal、桌面环境配置或 toolkit fallback。

GUI 层只关心启动时读取到的设置快照，以及设置变化时收到通知。MVP 当前不实现各平台读取，只先确定边界。

初期设置包括：

- 当前解析后的颜色方案：亮色或暗色；
- 应用颜色偏好：跟随系统、强制亮色或强制暗色；
- 系统强调色；
- 双击间隔；
- 后续可增加光标闪烁间隔、键盘重复配置、指针移动阈值、减少动画等辅助功能设置。

草案：

```go
type Settings interface {
    Appearance() AppearanceSettings
    Input() InputSettings
    Accessibility() AccessibilitySettings
    ConnectChanged(func(SettingsChange)) signal.Handle
}

type AppearanceSettings struct {
    ColorScheme ColorScheme
    AccentColor color.Color
}

type InputSettings struct {
    DoubleClickInterval time.Duration
    CursorBlinkInterval time.Duration
}

type AccessibilitySettings struct {
    HighContrast bool
    ReduceMotion bool
}

type SettingsChange struct {
    AppearanceChanged   bool
    InputChanged        bool
    AccessibilityChanged bool
    LayoutChanged       bool
}

type ColorScheme int

const (
    ColorSchemeLight ColorScheme = iota
    ColorSchemeDark
)
```

设置变化处理：

- 颜色方案、强调色、字体等视觉设置变化会触发窗口重绘；
- 影响测量的设置变化，例如默认字体变化，需要触发布局失效；
- 双击间隔这类输入设置不触发布局或绘制，只影响 `ClickEventController` 等控制器的判定逻辑。

### 7.2 Style

样式系统只服务 GUI 层，不强行兼容 CSS。它可以吸取 CSS 的优点：树级继承、样式可单独设置、按控件角色和状态匹配，但不引入完整 selector engine、CSS box model 或浏览器布局语义。

`Style` 是值语义对象。应用可以设置全局 `Style`，也可以给某个 Widget 设置局部 `Style`。控件绘制时读取当前样式；如果实现为了速度做缓存，缓存必须是内部机制，对外表现仍然是样式实时生效。全局样式改变后，只需要发出样式变化通知并请求相关窗口重绘或重新布局。

MVP 样式边界：

- 样式在 GUI 绘制阶段被读取；
- 布局仍由 `LayoutManager` 决定，样式不替代布局系统；
- MVP 优先支持颜色、字体、圆角、描边、内边距等少量值；
- 影响绘制的样式变化触发 `RequestPaint`；
- 影响测量或安排的样式变化必须触发 `RequestLayout`；
- 样式值是普通 Go 类型，避免把字符串解析作为核心路径。

颜色使用标准库 `color.Color`。需要跟随系统变化的颜色可以用实现了 `color.Color` 的系统颜色引用表达，引用在绘制时根据当前 `Settings` 动态解析：

```go
func SystemColor(name SystemColorName) color.Color

type SystemColorName string

const (
    SystemColorWindowBackground SystemColorName = "window.background"
    SystemColorText             SystemColorName = "text"
    SystemColorAccent           SystemColorName = "accent"
    SystemColorControl          SystemColorName = "control"
    SystemColorControlHover     SystemColorName = "control.hover"
    SystemColorControlPressed   SystemColorName = "control.pressed"
    SystemColorFocusRing        SystemColorName = "focus.ring"
)
```

`Style` 本身保持简单：

```go
type Style struct {
    BackgroundColor color.Color
    TextColor       color.Color
    BorderColor     color.Color
    BorderWidth     float32
    BorderRadius    float32
    Padding         Edges
}

type Edges struct {
    Left   float32
    Top    float32
    Right  float32
    Bottom float32
}
```

控件参与样式匹配的基础信息：

- `Role`：控件语义角色，例如 `button`、`label`、`hbox`；
- `ID`：应用指定的稳定标识；
- `Class`：应用或控件附加的样式类；
- `State`：hovered、pressed、focused、disabled 等运行时状态；
- 树结构：父子关系用于继承全局样式和局部样式。

相关 API 可以保持很小：

```go
type Widget interface {
    Style() Style
    SetStyle(Style)
    StyleClasses() []string
    AddStyleClass(string)
    RemoveStyleClass(string)
}
```

样式继承、系统颜色解析和缓存都属于实现细节。对应用和控件开发者来说，`Style` 就是绘制时读取的当前样式。

## 8. 声明式 View 层

声明式层位于 `gui` 之上，使用 `View` 表达 UI 结构：

```text
View -> Widget -> platform
```

最小 View 层只负责把声明式树更新到命令式 `Widget` 树：

- 根据 View 类型创建对应 Widget；
- 根据同父级位置和 View 具体类型复用已有 Widget；
- 调用 Widget setter 更新属性；
- 用 `signal.Handle` 管理回调连接的替换和断开；
- 对不再使用的声明层节点断开信号连接，并从容器中移除对应 Widget；
- 不绕过 `DispatchEvent` 直接触发控件动作。

View 层不应该在 MVP 早期引入完整状态管理、属性绑定或复杂组件生命周期。命令式 GUI 层先提供稳定的 Focus、EventController、Widget 生命周期、样式和语义快照，View 层再薄薄地建立在这些能力之上。

MVP 实现采用独立 `ui` 包，位于 `gui` 之上，避免 `gui` 反向依赖声明式层。`View` 是普通 Go 值，`Root` 保存上一轮命令式 widget 树和信号连接状态。

```go
type View interface {
    Build(ctx BuildContext, old gui.Widget) gui.Widget
}

type BuildContext interface {
    State() any
    SetState(any)
    UpdateChild(target Bin, child View) gui.Widget
    UpdateChildren(target Container, children []View) []gui.Widget
}

type Root struct{}

func NewRoot() *Root
func MountWindow(window gui.Window, build func() View) *Root
func (r *Root) Update(View) gui.Widget
func (r *Root) UpdateWindow(gui.Window, View) gui.Widget
func (r *Root) MountWindow(window gui.Window, build func() View)
func (r *Root) RequestUpdate()
func (r *Root) UpdateNow() gui.Widget
func (r *Root) Widget() gui.Widget
func (r *Root) Unmount()
```

`Root` 是有状态的声明式 UI 挂载根。一个 `gui.Window` 通常对应一个 `ui.Root`，同一个窗口内容更新时必须复用同一个 `Root`，否则无法复用旧 `Widget`。

`Update` 和 `UpdateWindow` 是底层立即更新入口，适合测试或手动驱动。应用代码通常使用 `MountWindow` 挂载一次，然后在状态变化后调用 `RequestUpdate`：

```go
var root *ui.Root
count := 0

root = ui.MountWindow(win, func() ui.View {
    return ui.VBox(
        ui.Label(fmt.Sprintf("count: %d", count)),
        ui.Button(ui.Label("Add")).OnClick(func() {
            count++
            root.RequestUpdate()
        }),
    )
})
```

`RequestUpdate` 只标记需要更新，并通过 `gui.App.Post` 投递到 GUI 线程；连续多次调用会合并为一次待处理更新。真正构建发生在 `UpdateNow` 中：调用 `build()` 生成新的 `View` 树，再把它更新到已挂载的 `gui.Window`。如果不存在 `gui.App`，`RequestUpdate` 会退化为立即 `UpdateNow`，主要用于无事件循环的测试场景。

`MountWindow` 会连接窗口 `Destroy` 信号。窗口销毁时，`Root` 只断开声明层保存的信号连接并清空自身节点状态，不提前从窗口移除 Widget 树；Widget 资源释放仍由 `Window.Destroy` 兜底处理。用户主动调用 `Unmount` 时，`Root` 会断开信号连接、释放声明层节点状态，并在仍挂载同一个 root widget 时调用 `Window.SetWidget(nil)` 解除窗口内容。

`Root` 内部使用 `reflect.TypeOf(view)` 判断同位置的新旧 View 是否为同一类型。MVP 不提供 `ID()`、`Key()`、`Identity` 或 `Kind` 概念；View 树和 Widget 树按父级位置对应。动态列表中间插入、删除或重排的稳定状态保留不是 MVP 目标，后续通过 `Key()` 或 `ForEach` 设计解决。

UI 层使用 `Name()` 而不是 `ID()`：

```go
ui.Label("Name").Name("name-label")
```

`Name` 用于调试可读性、语义快照、自动化和大模型定位，不参与 diff identity。底层当前可以映射到 `gui.Widget.SetID(name)`，但 UI 层不向用户暴露唯一 ID 语义。

内置 View 值先覆盖 MVP 控件：

```go
func Label(text string) LabelView
func Button(child View) ButtonView
func TextInput() TextInputView
func Image(img image.Image) ImageView
func HBox(children ...View) BoxView
func VBox(children ...View) BoxView
```

推荐链式写法：

```go
ui.VBox().
    Spacing(8).
    Children(
        ui.Label("Name").Name("name-label"),
        ui.TextInput().Name("name").Text(state.Name).OnText(func(text string) {
            state.Name = text
            root.RequestUpdate()
        }),
        ui.Button(ui.Label("OK")).Name("ok").OnClick(submit),
    )
```

也支持短写：

```go
ui.VBox(ui.Label("A"), ui.Label("B")).Spacing(8)
```

内置 View 不公开 `Props`，不使用 `ViewBase`，不嵌入通用结构，也不使用 unsafe 自引用技巧。每个 View 显式持有自己的 `name`、`hidden` 和控件字段，并显式提供链式方法。少量重复优先于复杂抽象。

`BuildContext.UpdateChild`／`UpdateChildren` 分别协调 `ui.Bin`／`ui.Container` 挂载目标。
目标不要求是 Widget，按稳定指针区分，在各目标内独立复用节点；具体定义与生命周期以
[DesignUI §2.1](DesignUI.md#21-子节点挂载目标已实现) 为准。由于 Container 没有指定位置
插入 API，列表采用保守策略：连续同类型前缀原地更新，尾部新增直接 append，尾部减少
直接 remove；遇到第一个既有位置 View 类型变化时，释放旧 tail 并重建新 tail。

信号连接由各 View 在 Mount 时建立，在 State 中保存 handle 和当前回调，Update 刷新
回调，Unmount 断开连接；共享修饰器的连接由 ViewBase 管理。公开 BuildContext 不提供 Connect。

View 层不直接调用控件动作，不提供 `Click(name)` 之类旁路；真实输入和模拟输入仍统一进入 `Window.DispatchEvent`。

## 9. 语义信息和大模型集成入口

MVP 阶段不实现 HTTP 协议细节，但在 `Application` 和 `Window` 上预留公开方法。

结构化信息草案：

```go
type ApplicationInfo struct {
    Windows []WindowInfo
}

type WindowInfo struct {
    ID      string
    Title   string
    Bounds  geometry.Rectangle
    Widget  WidgetInfo
}

type WidgetInfo struct {
    ID        string
    Role      Role
    Text      string
    Bounds    geometry.Rectangle
    Visible   bool
    Enabled   bool
    Focusable bool
    Focused   bool
    ContainsFocus bool
    Actions   []Action
    Children  []WidgetInfo
}
```

角色和动作：

```go
type Role string

const (
    RoleWidget Role = "widget"
    RoleBox    Role = "box"
    RoleHBox   Role = "hbox"
    RoleVBox   Role = "vbox"
    RoleLabel  Role = "label"
    RoleButton Role = "button"
)

type Action string

const (
    ActionClick Action = "click"
    ActionFocus Action = "focus"
)
```

`Role` 是开放的字符串类型，内置常量只表示 goui 约定的标准角色，不是封闭枚举。自定义控件可以定义自己的稳定角色：

```go
const RoleColorPicker gui.Role = "color_picker"
```

MVP 阶段不需要 role 注册表。后续如果需要跨库协作，可以约定命名空间，例如 `"acme.color_picker"`。

`WidgetInfo.Bounds` 使用窗口局部绝对坐标，而不是 `Widget.Rect()` 的父局部坐标。`RoleBox` 表示普通容器，`RoleHBox` 和 `RoleVBox` 表示带明确线性方向的 `LinearBox`。

Snapshot 是用于理解界面及规划输入的语义树，不是控件类型或全部布局参数的转储。
控件可以过滤内部结构，例如 ScrollView 不暴露内部 viewport 和滚动条。普通 Overlay
使用 `RoleBox`，内容的具体语义由内容自己提供，不根据覆盖布局推断对话框或模态行为。

- `Children` 对保留的兄弟节点维持从底到顶的顺序，但语义树不保证包含完整命中结构。
- `Bounds` 是尚未扣除裁剪和遮挡的布局矩形，不代表有效可点击区域。
- `Visible` 是控件自身的可见标志；隐藏节点可以保留，祖先隐藏和遮挡不改写该标志。
- 被覆盖不等于被禁用，不应因此改写 `Enabled`；`Actions` 表达支持的动作，不保证
  在 Bounds 内任意点输入都可达。执行动作仍须走输入分发，不能直接调用控件信号。

后续可交互区域、裁剪和遮挡信息需要统一设计，不为 Overlay 添加推测性的交互状态。

2026-10-05 补充：WidgetInfo.Attributes / SetAttribute 提供具名语义扩展。
TableInfo、HierarchyInfo 归 widgets，不再为每种复杂控件追加 GUI 专用字段。
公共字段仍为明确 Go 类型；属性是独立的快照数据，不是 Widget 属性系统或活体
对象入口。归属、复制、JSON 和错误反馈见 [Widget Snapshot](DesignWidgetSnapshot.md)。

预留方法：

```go
func (a *application) Snapshot() ApplicationInfo
func (w *window) Snapshot() WindowInfo
func (w *window) DispatchEvent(event events.Event) error
```

`Window.DispatchEvent` 是窗口事件的统一入口。平台回调 `onEvent` 只负责把 platform event 转交给它；自动化或测试找到公开 Window 后也进入同一入口，Popover 同理。`DispatchEvent` 先处理 `CloseEvent`、`SizeEvent`、`ScaleEvent`、`PaintEvent` 等窗口级事件，Pointer、Wheel、Key 等输入事件再交给本窗口的 `EventDispatcher` 做控件树传播。文本走独立的 IMContext 路径，宿主 CommitText 与原生文字提交共用该路径。

后续 HTTP GET/POST 只是这些方法的协议外壳。模拟操作必须通过 `DispatchEvent` 投递输入事件，不能直接触发控件信号或调用控件动作方法，以保证真实输入和模拟输入路径一致。

## 10. 控件生命周期和资源释放

控件生命周期描述的是 `Widget` 和窗口、控件树、外部资源之间的关系，不等同于 Go 对象被 GC 回收的时机。

基础状态：

```text
Constructed -> Mounted -> Unmounted
```

- `Constructed`：控件对象已创建，但尚未进入任何窗口；
- `Mounted`：控件已经进入某个 `Window` 的控件树；
- `Unmounted`：控件从窗口控件树移除，但对象仍可复用；

公开生命周期先使用信号：

```go
type Widget interface {
    ConnectMount(func()) signal.Handle
    ConnectUnmount(func()) signal.Handle
}
```

触发规则：

- `Window.SetWidget(widget)` 或 `container.AddChild(child)` 使控件进入已有窗口时，递归触发 mount 信号；
- `Window.SetWidget(nil)`、替换 root 或 `container.RemoveChild(child)` 使控件离开窗口时，递归触发 unmount 信号；
- mount 信号触发时，控件已经接入新 `Root`，`Root()` / `Window()` 返回新宿主，root widget 的 `Parent()` 为 `nil`，普通 child 的 `Parent()` 已经是新 parent；
- unmount 信号触发时，控件尚未从旧 `Root` 断开，`Root()` / `Window()` 仍然返回旧宿主，`Parent()` 仍然返回当前 parent；信号结束后才清除 root 或 parent 关系；
- mount 顺序是 parent 先于 child，unmount 顺序是 child 先于 parent；
- root widget 离开窗口时，unmount 信号期间 `Window()` 仍然有效，信号结束后 root widget 的 `Root()` / `Window()` 变为 `nil`，但子树内部 parent/children 关系保持不变；
- 普通 child 被 `Container.RemoveChild` 移出时，unmount 信号期间 `Parent()` 和 `Window()` 仍然有效，信号结束后该 child 的 `Parent()`、`Root()` 和 `Window()` 变为 `nil`；
- 同一个 `Root` 内 reparent 不触发 mount/unmount；跨 `Root` reparent 先按旧宿主触发 unmount，再接入新宿主并触发 mount；
- 这里的 `Window()` 有效表示回调中能取得旧窗口对象，并且框架尚未释放 painter、platform window 等窗口资源，适合断开信号、释放依赖窗口的资源或读取窗口上下文；如果 unmount 发生在 `Window.Destroy()` 流程中，窗口不会继续参与正常绘制，回调不应依赖新的重绘请求一定会执行；
- 命令式 `Container.RemoveChild` 只表示 detach，不默认 destroy，避免误删仍要复用的控件；
- `Destroy` 暂不作为公开 `Widget` 方法，窗口销毁和后续 View reconciler 使用 GUI 内部销毁流程清理控件树；
- 控件被 unmount 或内部销毁时，如果它或其子树持有焦点，窗口需要清除焦点；
- 控件被 unmount 或内部销毁时，`EventDispatcher` 需要清理 hover path、pressed 等与该子树相关的临时输入状态。

控制器生命周期：

- `EventController` 公共接口不拥有 Widget；
- 公共控制器如果有内部状态，例如 hover、pressed、click count，应在控件 unmount 时通过框架内部生命周期重置；
- 控件连接到 controller 的回调由控件自身持有 handle，并在不再需要时断开；
- 声明式层替换回调时，通过旧 `signal.Handle.Disconnect()` 断开旧连接。

资源释放：

`Window` 对控件资源生命周期兜底。控件能明确释放的资源应尽早释放；无法确定释放时机的资源，可以连接窗口 `Destroy` 信号或控件 unmount 信号。后续如果 `Image`、`TextInput` 等控件证明需要区分 detach 和最终销毁，再考虑公开 destroy 信号。

窗口销毁顺序：

1. 标记窗口已销毁，并撤销原生命中测试回调；
2. 触发 `Window.Destroy` 信号，重入 `Destroy` 不重复触发或中断当前通知；
3. 递归触发控件树 unmount 信号；
4. 清理 focus、hover path 和 controller 临时状态；
5. 断开控件树和窗口关系；
6. 释放输入法、光标和 painter 等窗口资源；
7. 释放 platform window。

`Window.DispatchEvent` 忽略销毁开始后到达的事件。模态目标或窗口焦点回调返回后，也要重新检查窗口是否已销毁，再决定是否继续处理事件或进入控件树。检查不能只放在信号 `Emit` 之前，因为单次通知中的前一个回调就可能销毁窗口；销毁和 unmount 等清理通知则必须保留自身的正常派发语义。

因此 Widget 必须能找到所属 Window，但 Window 不应实现 Widget 接口。

## 11. MVP 实现步骤

建议按以下顺序实施，每一步保持可单独验证：

1. 调整 `EventController` 公共接口，去掉 `Widget()` 和 `SetWidget()`，保留传播上下文中的 `Target`、`Current` 和 `Phase`。
2. 收敛 `Widget` 和树操作，去掉 `owner` / `Base()` / 公开 `Destroy()`，实现最小 mount/unmount 生命周期。
3. 实现窗口系统 focus 和窗口内 Widget focus，KeyEvent 以 focused widget 为目标。
4. 增加 `Settings` 和全局 `Style`，先让内置控件从样式读取颜色。
5. 将当前基础 hover/click 控制器调整为 `MotionEventController` 和默认 bubble 阶段的 `ClickEventController`，改造 `Button` 使用单一 bubble click controller。
6. 增加 `Image` 控件，MVP 阶段只通过 `SetImage(image.Image)` 接收图片并绘制，不处理图标名称、资源体系、文件加载、尺寸策略或旋转信息。
7. 增加最小 `TextInput` 控件，先支持关闭输入法时的基础文本输入；IME 组合态接口保留到后续。
8. 增加最小 View 层，只做 View 到 Widget 的创建、复用、更新和销毁。
9. 在 View 层稳定后，再考虑更完整的样式规则、Inspector、HTTP 协议、IME 和剪贴板。

属性系统、复杂绑定和完整 MVVM/MVU 支持不进入当前 MVP 核心路径。
