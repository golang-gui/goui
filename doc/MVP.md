# GUI MVP Scope

> 状态（2026-10-14）：已完成的 GUI 基线与设计边界；不作为当前功能范围或待办清单。现行原则见 [Design](Design.md)，原始大草案见 GuiDesign 的历史区。\
> 目标：明确当前阶段必须稳定的 GUI 内核，避免在基础闭环未完成前扩展过多设计。

## 核心目标

GUI MVP 的目标不是完整控件库，也不是完整声明式框架。当前阶段只验证一条最小闭环：

```text
platform event -> Window.DispatchEvent -> EventDispatcher -> EventController -> Widget state/signal -> layout/paint -> semantic snapshot
```

完成后应能用命令式 GUI 构建一个包含文本、按钮、图片和输入框的窗口，并能通过语义快照和平台输入事件模拟驱动它。

## 必须确定

### Widget 树模型

- 父子关系；
- `Measure` / `Arrange` / `Paint`；
- `Widget.Rect()` 使用父局部坐标；
- `WidgetInfo.Bounds` 使用窗口局部绝对坐标；
- `Snapshot()` 输出可用于自动化和大模型理解的语义树。

这是 GUI 层骨架，后续声明式 View 层也建立在这套树模型上。

`Widget` 应保持为面向控件行为的接口，不包含公开的 `Base()`、`IsWidget()` 或其它用于模拟继承的 self 回指针。当前阶段显式要求控件嵌入 `WidgetBase`：`Widget` 包含包内未导出的 `base() *WidgetBase` 方法，外部控件通过嵌入 `gui.WidgetBase` 自动满足这个约束。`WidgetBase` 包办 ID、可见性、矩形、事件控制器、布局管理和树节点状态，但它仍然只是控件开发 helper，不是面向应用代码的公共基类协议。`WidgetBase` 使用 Go 零值作为有效状态，默认可见，不需要 `Init()`。

`Window` 不实现 `Widget`，而是实现 `Root`。`Root` 是一棵 Widget 树挂载到宿主后的最小边界，提供根控件、布局／绘制请求和帧前通知。root widget 内部挂载到 `Root`，但公开的 `rootWidget.Parent()` 返回 `nil`；普通 widget 的 parent 是另一个 `Widget`。窗口之间未来的 owner/transient/child window 关系不属于 Widget containment tree，应单独设计。

```go
type Root interface {
    Widget() Widget
    RequestPaint() error
    RequestLayout()
    ConnectFrame(func(time.Time)) signal.Handle
}
```

`SetWidget` 是宿主内容管理能力，属于 `Window`，不放进 `Root`。`EventDispatcher` 是宿主内部实现细节，对外通过 `Window.DispatchEvent` / `Popover.DispatchEvent` 投递平台事件；已确认文本通过宿主的 `CommitText` 进入现有 IMContext 路径。dev 自行枚举公开对象并管理引用，不在 Application 按 ID 寻址。

因为 Go embedding 的 receiver 无法自动拿到外层控件，树操作不依赖 `Init(owner)` 或长期保存 self 回指针。`Widget` 只表达控件树节点，`Container` 额外表达可承载子控件的能力：

```go
type Widget interface {
    base() *WidgetBase
    Parent() Widget
    Root() Root
    Window() Window
    // ID / Visible / Rect / layout / paint / event controller / lifecycle...
}

type Container interface {
    Widget
    AddChild(Widget)
    RemoveChild(Widget)
    Children() []Widget
}
```

`WidgetBase` 默认实现 `Parent()`、`Root()`、`Window()`、`RemoveChild()` 和 `Children()`。`AddChild` 需要知道真正的外层 parent，因此由具体容器显式实现：

```go
func (p *Panel) AddChild(child Widget) {
    p.WidgetBase.AddChild(p, child)
} // 自定义多子容器；Button 是 Bin，使用 SetChild
```

不保留包级 `Parent`、`GetChildren`、`AddChild`、`RemoveChild`、`GetRoot` 或 `GetWindow` 这类树操作函数。调用方直接使用接口方法：

```go
widget.Parent()
widget.Root()
widget.Window()
container.AddChild(child)
container.RemoveChild(child)
container.Children()
```

`WidgetBase.AddChild(parent, child)` 是给自定义容器实现 `Container.AddChild(child)` 使用的公开 helper，因为外部包无法调用私有方法。其它树操作仍由 `WidgetBase` 内部私有 helper 统一处理 reparent、同 root 保留生命周期、跨 root mount/unmount 等树语义。这样既避免重新引入 `owner` 或 `self` 字段，也不会让公开 API 同时存在函数式和方法式两套树操作。

`Parent()` 只读取直接 widget parent，代价最小；`Root()` 和 `Window()` 需要沿 parent 链追溯；`Container.Children()` 返回 children 快照，会 clone 内部 slice。

后续如果树节点逻辑继续膨胀，可以把 `WidgetBase` 内的节点状态拆成独立的 `node.TreeNode` 或类似组合式实现。

### 窗口绘制闭环

- `Window.RequestPaint()` 主动请求平台重绘；
- layout dirty 和 paint dirty 可以合并到平台 `PaintEvent`；
- 绘制前统一执行必要的测量和安排；
- 控件绘制使用相对自身原点的局部坐标。

### 事件分发闭环

- `Window.DispatchEvent` 是真实输入和模拟输入的统一入口；
- `EventDispatcher` 负责 hit-test、传播路径和三阶段传播；
- pointer/focus 的路径变化由 `EventDispatcher` 计算为内部 crossing notification，状态语义由对应 `EventController` 维护；
- `EventController` 把平台输入事件解释为控件状态和信号；
- 控件应用层通常监听信号，而不是直接处理原始平台事件。

这也是后续大模型集成的核心：模拟操作必须投递平台输入事件，不能走 `Click(id)` 这类旁路 API。

### 最小 Focus 模型

- `Window` 记录平台窗口是否拥有系统输入焦点；
- `Window` 内部持有 `FocusedWidget`；
- `WidgetBase` 提供 `Focusable`、`Focused`、`ContainsFocus` 和对应信号；
- `KeyEvent` 发送给 focused widget；没有 focused widget 时退回 root widget；
- pointer down 命中控件后，沿父链查找最近的 focusable widget 并设置焦点；
- 控件隐藏、移除或销毁时，如果焦点位于该控件子树内，则窗口清除 focused widget；
- 窗口失去系统输入焦点时不清除 focused widget，重新获得窗口焦点后继续使用原控件焦点；
- `SetFocusedWidget(nil)` 表示清除窗口内 focused widget，不提供有歧义的 `ClearFocus()`；
- 不在 `WidgetBase` 上提供 `RequestFocus()`，自定义控件需要焦点时由外层控件显式传入 self，例如 `win.SetFocusedWidget(button)`；
- Tab／Shift+Tab 按现有树顺序及 Focusable 执行默认焦点转移，禁用／隐藏子树跳过；FocusVisible 区分键盘提示，控件内导航由控件负责。平台 RequestFocus、显式 focus chain 等不属于当前契约。

没有最小焦点模型，`TextInput` 无法正确处理键盘输入。

MVP 接口边界：

```go
type Window interface {
    Focused() bool
    FocusedWidget() Widget
    SetFocusedWidget(Widget) bool
    ConnectFocus(func(bool)) signal.Handle
}

type Widget interface {
    Focusable() bool
    SetFocusable(bool)
    Focused() bool
    ContainsFocus() bool
    ConnectFocused(func(bool)) signal.Handle
    ConnectContainsFocus(func(bool)) signal.Handle
}
```

### 最小生命周期

- 控件进入窗口；
- 控件离开窗口；
- 窗口销毁时递归清理控件树。

生命周期先只暴露挂载和卸载信号：

```go
ConnectMount(func()) signal.Handle
ConnectUnmount(func()) signal.Handle
```

定义：

- `Mount` 表示 widget 子树已经进入某个 `Root`。mount 信号触发时，当前 widget 的 `Root()` 和 `Window()` 已经返回新宿主；root widget 的 `Parent()` 为 `nil`，普通 child 的 `Parent()` 已经是新 parent。
- `Unmount` 表示 widget 子树即将离开当前 `Root`。unmount 信号触发时，当前 widget 的 `Root()` 和 `Window()` 仍然返回旧宿主，`Parent()` 仍然返回当前 parent；信号处理结束后，框架才断开 root 或 parent 关系。
- `Mount` 递归顺序是 parent 先于 child，便于父控件先建立上下文。
- `Unmount` 递归顺序是 child 先于 parent，便于子控件先释放依赖父级或窗口的资源。
- root widget 被 `Window.SetWidget(nil)` 或窗口销毁移出时，unmount 信号期间 `Window()` 仍然有效；信号结束后 root widget 的 `Root()` / `Window()` 变为 `nil`，但子树内部的 parent/children 关系保持不变。
- 普通 child 被 `Container.RemoveChild` 移出时，unmount 信号期间 `Parent()` 和 `Window()` 仍然有效；信号结束后该 child 的 `Parent()`、`Root()` 和 `Window()` 变为 `nil`。
- 同一个 `Root` 内 reparent 不触发 mount/unmount；跨 `Root` reparent 先按旧宿主触发 unmount，再接入新宿主并触发 mount。

这里的 `Window()` 有效表示回调中能取得旧窗口对象，并且框架尚未释放 painter、platform window 等窗口资源，适合断开信号、释放依赖窗口的资源或读取窗口上下文。若 unmount 发生在 `Window.Destroy()` 流程中，窗口不会继续参与正常绘制，回调不应依赖新的重绘请求一定会执行。

`Destroy` 暂不作为公开 `Widget` 方法。窗口销毁和后续 View reconciler 可以通过 GUI 内部销毁流程清理控件树。当前阶段不引入复杂组件生命周期。

### 语义快照和模拟输入

- `Application.Snapshot()` / `Window.Snapshot()` 输出窗口和控件信息；
- `Window.DispatchEvent()` / `Popover.DispatchEvent()` 接受平台事件，宿主 `CommitText` 接受已确认文本；
- 自动化和大模型集成通过公开宿主能力与语义快照完成，协议及短期引用留在 dev。

### 最小控件集

- `Label`：验证文字测量和绘制；
- `Button`：验证 pointer、hover/pressed 状态、信号；
- `Image`：验证 `image.Image` 显示和绘制；
- `TextInput`：验证 focus、key event 和基础文本输入。

## 已超出早期 MVP 的已实现能力

Settings 实际读取、完整 Basic／Modern、文本编辑／IME／Clipboard、Shortcut、
Tab／FocusVisible、手势竞争／捕获、dev 协议、Overlay 与 widgets 复杂控件均已落地。
它们不再是“明确推迟”项；分平台验证范围以各自设计的带日期记录为准。

尚未实现与待评估方向统一见 [未来路线](Roadmap.md)，不在本篇重复维护 TODO。
触摸／笔、更多平台、原生无障碍桥接与检查工具等并非已经具备的能力。

## 原 MVP 实现顺序（已完成的历史）

1. 已完成：清理 `EventController` 接口，去掉 controller 对 `Widget` 的公共持有关系。
2. 已完成：收敛 `Widget` 和树操作，去掉 `owner` / `Base()` / 公开 `Destroy()`，实现最小 mount/unmount 生命周期。
3. 已完成：稳定 `Widget` / `Container` 树接口，去掉公开 `SetParent`。
4. 已完成：实现最小 Focus 模型。
5. 已完成：稳定现有 `Label`、`Button`、`LinearBox`，并公共化基础 `MotionEventController` / `ClickEventController`。
6. 已完成：增加最小 `Image`，只接收 `image.Image` 并显示出来。
7. 已完成：增加最小 `TextInput`，包含文本状态、focusable、KeyEvent 编辑逻辑、语义快照、基础绘制和 caret。
8. 已完成：增加最小 View 层，只做 View 到 Widget 的创建、复用、更新和移除，并提供 `MountWindow` / `RequestUpdate` 的合并更新调度。
