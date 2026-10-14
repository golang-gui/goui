# 自定义 Widget 与 View

本文面向使用 GOUI 编写应用或控件库的开发者，说明当前公开接口的六种扩展方式。
以应用实际依赖版本的源码为准；这里不新增框架 API，也不将 Go 嵌入解释成继承体系。

Widget 是持续存在的命令式对象，负责行为、布局、绘制和语义；View 是可以反复创建
的界面声明，负责把本次参数协调到 Widget。自定义 Widget 不要求同时编写 View，
组合 View 也不要求新增 Widget 类型。

## 选择方式

| 方式 | 适用场景 | 保留／复用什么 |
| --- | --- | --- |
| 组合 Widget | 表单行、工具栏、业务面板 | 已有 Widget 和容器的行为、布局与 Snapshot |
| 扩展 Widget | 为已有控件增加少量属性或绘制 | 原控件初始化与实现，按需重写方法 |
| 实现 Widget | 自绘控件、新容器、新交互 | WidgetBase 的树、状态、失效与基础语义 |
| 组合 View | 参数化界面片段、应用组件 | 已有 View 展开后的 WidgetView |
| 扩展 View | 为已有声明增加业务参数或更新逻辑 | 原 View 的 Mount／Update／Unmount |
| 实现 View | 自定义 Widget 的声明绑定、独立协调节点 | ViewBase 的通用修饰器与 BuildContext |

一般从组合开始。只换外观先用 StyleName 和样式表，不为一段界面增加全局管理器；
需要不同的行为或独立资源生命周期时再定义类型。

下面带 `package custom` 的代码块可分别保存为同一 `custom` 包中的不同 Go 文件。
`SurfaceView` 使用第 3 节的 `SurfaceWidget`；其他示例可单独取用。
接口断言用于检查公开契约，不要求读者复制框架私有方法。

## 1. 组合 Widget

组合已有 Widget 时，不需要先定义新类型。下例将标签和输入框组合成表单行，
仍返回普通 LinearBox；可直接交给 `gui.Window.SetWidget` 或加入其他容器。

```go
package custom

import (
    "github.com/golang-gui/goui/gui"
    "github.com/golang-gui/goui/layout"
)

func NewFieldRow(caption string, input *gui.TextInput) *gui.LinearBox {
    row := gui.NewLinearBox(layout.DirectionHorizontal)
    row.SetSpacing(8)
    row.SetCrossAlign(layout.CrossBaseline)
    row.AddChild(gui.NewLabel(caption))
    if input != nil {
        input.SetMainWeight(1)
        row.AddChild(input)
    }
    return row
}
```

输入框由调用方创建，便于连接信号、设置内容和读取状态；加入行后它只能属于
这一棵树。不要把同一个 Widget 同时加入两个面板，也不要用替换指针假装复制控件。
复合业务组件可以持有根容器和内部控件，向外提供明确 setter／signal；这个普通 Go
管理对象本身不一定是 Widget。

容器已经负责布局、子树绘制和 Snapshot，无需额外转发。父控件的 StyleName 不会
隐式设置孩子样式；必要时分别设置 Label 和输入框的样式名。

## 2. 扩展 Widget

用 Go 嵌入复用已有控件时，通过其构造函数初始化，避免把零值当作完整控件。
下例扩展不含子树的 Label：增加强调背景，继续使用 Label 的文字排版、测量和资源清理。

```go
package custom

import (
    "image/color"

    "github.com/golang-gui/goui/core/geometry"
    "github.com/golang-gui/goui/gui"
    "github.com/golang-gui/goui/platform/graphics"
)

type AttentionLabel struct {
    *gui.Label
    highlighted bool
}

func NewAttentionLabel(text string) *AttentionLabel {
    return &AttentionLabel{Label: gui.NewLabel(text)}
}

func (w *AttentionLabel) SetHighlighted(value bool) {
    if w.highlighted == value {
        return
    }
    w.highlighted = value
    w.RequestPaint()
}

func (w *AttentionLabel) Paint(p gui.Painter) {
    if w.highlighted {
        rect := geometry.Rectangle{Size: w.Rect().Size}
        brush := graphics.ColorOf(color.NRGBA{R: 55, G: 132, B: 196, A: 40})
        p.FillRoundRect(rect, 4, brush)
    }
    w.Label.Paint(p)
}

func (w *AttentionLabel) Snapshot() gui.WidgetInfo {
    info := w.Label.Snapshot()
    info.SetAttribute("example.attention", w.highlighted)
    return info
}

var _ gui.Widget = (*AttentionLabel)(nil)
```

这里的固定颜色只是强调属性的示范；若应跟随主题，改用独立 StyleName／Part
解析该背景，不能去改全局 Label 规则。颜色从标准非预乘 NRGBA 经 ColorOf 转成
绘图需要的预乘值，不再手工乘一次 Alpha。

需要注意 Go 的实际调用规则：

- 外层重写 Paint 后，GUI 对外层 Widget 的调用会进入它；但原类型方法内部
  对自身方法的调用，不会自动变成对外层类型的“虚调用”。
- 重写 StyleChanged、Measure 等方法时，不能丢掉原控件依赖的缓存失效、约束
  或排版行为；需要保留的部分显式委托，如上例的 Label.Paint。
- 扩展含子树的控件更需谨慎。原类型的 AddChild／SetChild 或构造函数可能把
  原对象作为孩子的 Parent，并不会因嵌入而自动改成外层对象。父链、mount 通知、
  事件和身份都要核对；不能仅认为“接口能编译”就等同于正确的控件树。
- 若需要改动原控件的内部结构或多个行为，通常将它作为自定义容器的真实孩子
  更清楚，不要通过反射修改私有状态。

## 3. 实现 Widget

外部类型嵌入 `gui.WidgetBase` 获得 Widget 的基础契约。它是 helper，不要求应用
重新实现焦点、树操作和输入分发，也没有需要用户维护的全局“继承注册表”。

下面实现一个有样式背景的单子容器。它用 FillLayout 复用测量与安排逻辑，
只在 Paint 中绘制自身背景；孩子由 GUI 遍历绘制。

```go
package custom

import (
    "github.com/golang-gui/goui/core/geometry"
    "github.com/golang-gui/goui/gui"
    "github.com/golang-gui/goui/layout"
    "github.com/golang-gui/goui/platform/graphics"
    "github.com/golang-gui/goui/style"
)

type SurfaceWidget struct {
    gui.WidgetBase
    child gui.Widget
}

func NewSurfaceWidget() *SurfaceWidget {
    w := new(SurfaceWidget)
    w.SetLayoutManager(layout.NewFillLayout())
    return w
}

func (w *SurfaceWidget) Child() gui.Widget { return w.child }

func (w *SurfaceWidget) SetChild(child gui.Widget) {
    if w.child == child {
        return
    }
    if w.child != nil {
        w.WidgetBase.RemoveChild(w.child)
    }
    // 先登记单子槽，再挂接；WidgetBase 对 Bin 有一致性防护。
    w.child = child
    if child != nil {
        w.WidgetBase.AddChild(w, child)
    }
    w.RequestLayout()
}

func (w *SurfaceWidget) Paint(p gui.Painter) {
    name := w.StyleName()
    if name == "" {
        name = "widget"
    }
    state := style.Normal
    if !gui.IsEnabled(w) {
        state = style.Disabled
    }
    s := gui.ResolveStyle(name, style.PartDefault, state)
    background, ok := s.BackgroundColor()
    if !ok || background == nil {
        return
    }
    radius, _ := s.Radius()
    rect := geometry.Rectangle{Size: w.Rect().Size}
    p.FillRoundRect(rect, radius, graphics.ColorOf(background))
}

func (w *SurfaceWidget) Snapshot() gui.WidgetInfo {
    info := w.WidgetBase.Snapshot()
    info.Role = gui.RoleBox
    return info
}

var _ gui.Widget = (*SurfaceWidget)(nil)
var _ gui.Bin = (*SurfaceWidget)(nil)
```

Surface 没有默认装饰，样式未设置背景时不画底色；应用可选择 Modern 的 Card
或自己的样式。它不实现 Border／Shadow 等字段的绘制，不能仅因设了这些样式字段
就期待出现边框／阴影。单纯的圆角背景也不等于圆角裁剪：孩子仍受矩形结构裁剪，
需要安全留白时给内部容器设置 Padding，不宣称存在尚未提供的圆角 Clip 接口。

### 控件树与布局

`WidgetBase.AddChild(parent, child)` 是供控件实现使用的低层 helper。上例传入的是
实际 SurfaceWidget，不能传 `&w.WidgetBase` 或另一个根容器。应用使用 Surface
时只调用 SetChild，不绕过单子槽。

公开单子容器实现 `gui.Bin`；公开多子容器实现 `gui.Container`，其 AddChild 包装
`WidgetBase.AddChild(w, child)`。Widget 本身不等于 Container。实现内部结构孩子时
可使用 AddStructuralChild，但它不是给用户绕过 Bin／Container 内容 API 的入口。

默认 WidgetBase.Measure／Arrange 已处理显隐、控件自己的 min／max、父约束和
LayoutManager。上例没有重写它们，继承的是这些 helper 方法的正常 Go 提升。
自绘叶子没有孩子或布局策略时不会凭绘图代码产生自然尺寸：简单图形可显式设
MinSize，更完整的尺寸策略可实现独立 LayoutManager。

确需自己重写 Measure 时，返回 `layout.Measurement`，同时考虑自己的 MinSize／
MaxSize 和传入的 Constraint，父约束最终优先。Widget 的 MaxSize 中 0 表示不限，
Constraint 的 Max 中 0 则是硬零；不能拿空 Constraint 代替 Unbounded。
有真实文字基线时返回相应 Baseline，不虚构几何中心作为 baseline。
重写 Arrange 时先通过 WidgetBase.Arrange 保存自身矩形，再正确安排局部坐标孩子；
若已配置 LayoutManager，不要重复执行同一套孩子布局。

### 绘制、交互与失效

- Paint 只在同步调用中借用 Painter，不能缓存 Painter 或在后台 goroutine 使用。
  原点是自身左上角；使用 Rect().Size，而不是把 Rect().Pos 再加到绘图坐标。
- 子树遍历、每个控件的状态隔离由 GUI 负责，不手工调用 child.Paint。
  同一个 Paint 内需要临时变换时用 Save／Restore 成对管理；父 Transform
  不会自动变成孩子的布局变换。
- setter 改变期望尺寸、文字或布局时 RequestLayout；只改绘制参数时 RequestPaint。
  StyleChanged 是失效钩子：释放／重建自身派生缓存，不能改树或全局样式。
  本例每次 Paint 解析样式，没有额外样式缓存，因此默认空钩子足够。
- 新的交互行为复用 EventController，将低层输入转成 core/signal 信号，再由应用
  连接通知。不要在 Widget 中绕开 EventDispatcher 建另一套命中与事件广播。
  状态、禁用、键盘操作和 Snapshot 的 Actions 必须与实际交互一致。
- Snapshot 从 WidgetBase.Snapshot 起步，保留孩子与标准状态，再补角色、文本和
  Attributes。Surface 是装饰容器，不伪造 click 动作；属性只放脱离可变对象的
  JSON 数据，不把 Widget、原生句柄或回调放进快照。

### 原生资源与生命周期

控件按 ConnectMount／ConnectUnmount 管理能明确释放的资源；unmount 不等于最终
销毁，控件可能重新挂载或迁到另一个窗口。Window 负责树的最终兜底清理。
Painter.NewImage 产生的资源属于相应绘图后端实例，不能跨 Painter 继续复用。
在 unmount 时释放并清空这类缓存，下次需要时重新创建；资源创建错误要按操作
语义处理，不无条件忽略或在每帧重复报错。StyleChanged 不负责替代所有资源归属判断。

## 4. 组合 View

最简单的组件是返回已有 View 的普通 Go 函数。参数是本次声明，业务状态由调用方
持有；这里不引入独立协调节点。

```go
package custom

import "github.com/golang-gui/goui/ui"

func FieldRow(caption string, text ui.BindState[string]) *ui.BoxView {
    return ui.HBox(
        ui.Label(caption),
        ui.TextInput().BindText(text).MainWeight(1),
    ).Spacing(8)
}

type ProfileView struct {
    ui.ViewBase[ProfileView]
    title string
    child ui.View
}

func Profile(title string, child ui.View) *ProfileView {
    v := &ProfileView{title: title, child: child}
    v.Self = v
    return v
}

func (v *ProfileView) Build() ui.View {
    return ui.VBox(ui.Label(v.title), v.child).Spacing(8).Padding(12)
}

var _ ui.View = (*ProfileView)(nil)
```

FieldRow 返回的是实际 BoxView，所以 `FieldRow(...).ID("name-row")` 作用于这个根。
Profile 只实现 View，不实现 WidgetView；协调器调用 Build，直到得到 WidgetView。
它不能返回自身形成循环，也没有 Mount／Unmount 或 BuildContext.State 槽。

**当前纯 Build 组件的外层通用修饰器不会自动传播。** 例如给 Profile 调用
`.Style("card")`、`.ID("profile")`，不会自动把这两个设置施加到 Build 返回的 VBox。
展开后用于协调的身份是最终 WidgetView 的具体类型和 ID，而不是 Profile 的类型。
不要以外层类型变化就推定内层 Widget 或状态一定会重建。

因此简单组件优先像 FieldRow 一样直接返回实际根 View。需要参数化结构类型时，
可以显式增加业务参数，在 Build 中设置返回根的属性；需要自动通用修饰器、独立
状态或生命周期时，实现 WidgetView，如第 6 节，而不是改协调器传播规则。

## 5. 扩展 View

直接嵌入已有 `*ui.LabelView` 可以提升其方法，但 `.ID`、`.Text` 等仍返回
`*ui.LabelView`。链式调用之后返回内层对象，若把它提交给 UI，就不再使用外层
类型的扩展实现。原 ViewBase.Self 的类型也是 LabelView，不能强行改成外层类型。
同时，已经通过方法提升实现 WidgetView 的类型会被直接识别，不能只重写 Build
就指望它再展开成另一棵树。

推荐需要完整链式扩展时：外层嵌入自己的 `ViewBase[外层类型]`，以**命名字段**持有
要复用的 View，并显式委托其生命周期。下例为 Label 增加数值和单位参数：

```go
package custom

import (
    "fmt"

    "github.com/golang-gui/goui/gui"
    "github.com/golang-gui/goui/ui"
)

type UnitLabelView struct {
    ui.ViewBase[UnitLabelView]
    label *ui.LabelView
    value float64
    unit  string
}

func UnitLabel(value float64) *UnitLabelView {
    v := &UnitLabelView{label: ui.Label(""), value: value}
    v.Self = v
    return v
}

func (v *UnitLabelView) Unit(unit string) *UnitLabelView {
    v.unit = unit
    return v
}

func (v *UnitLabelView) Build() ui.View { return v }

func (v *UnitLabelView) Mount(ctx ui.BuildContext) gui.Widget {
    return v.label.Mount(ctx)
}

func (v *UnitLabelView) Update(ctx ui.BuildContext, widget gui.Widget) {
    v.label.Text(fmt.Sprintf("%.1f %s", v.value, v.unit))
    v.label.Update(ctx, widget)
}

func (v *UnitLabelView) Unmount(ctx ui.BuildContext, widget gui.Widget) {
    v.label.Unmount(ctx, widget)
}

var _ ui.WidgetView = (*UnitLabelView)(nil)
```

`UnitLabel(23.5).Unit("°C").ID("temperature").Style("label")` 保持外层类型。
通用修饰器由协调器统一应用外层 ViewBase；这里不把 Style／ID 等设置到作为代理
的内层 LabelView，因为协调器不会另外替它应用一遍共享修饰器。

**一个协调节点只有一个 State 槽。** 本例的 LabelView 不占用它，故委托简单。
若复用 ButtonView／TextViewView 等会 SetState 的实现，不能在原 Mount 后再
用自己的 SetState 覆盖，也不能更换槽的类型后直接调用原 Update／Unmount。
需要独立状态时，将原 View 作为通过 UpdateChild 协调的真实子 View，各自获得
节点；确需同节点委托才显式桥接原上下文状态，不能凭私有字段或猜测实现。

对需要长期信号或复杂子树的扩展，直接实现 WidgetView 往往比层层委托更清楚。
不要同时嵌入两个提供封印方法的 ViewBase／具体 View 来制造歧义，也不反射修改
内层回调和修饰器。

## 6. 实现 View

为新 Widget 编写声明绑定，实现 `ui.WidgetView`。下面将第 3 节的 SurfaceWidget
作为稳定单子挂载目标；新建的 View 只保存本次 child 声明。

```go
package custom

import (
    "github.com/golang-gui/goui/gui"
    "github.com/golang-gui/goui/ui"
)

type SurfaceView struct {
    ui.ViewBase[SurfaceView]
    child ui.View
}

func Surface(child ui.View) *SurfaceView {
    v := &SurfaceView{child: child}
    v.Self = v
    return v
}

func (v *SurfaceView) Build() ui.View { return v }

func (*SurfaceView) Mount(ui.BuildContext) gui.Widget {
    return NewSurfaceWidget()
}

func (v *SurfaceView) Update(ctx ui.BuildContext, widget gui.Widget) {
    ctx.UpdateChild(widget.(*SurfaceWidget), v.child)
}

func (*SurfaceView) Unmount(ui.BuildContext, gui.Widget) {}

var _ ui.WidgetView = (*SurfaceView)(nil)
```

调用 `Surface(ui.VBox(...).Padding(12)).Style(modern.Card)` 可以使用 Modern 的
背景和圆角；Style 不传给内部 VBox 或 Label。此 View 的通用修饰器会正常应用到
SurfaceWidget，所以它与只展开 Build 的 Profile 不同。

### 三个生命周期方法

| 方法 | 做什么 | 不做什么 |
| --- | --- | --- |
| Mount | 创建一次 Widget，初始化持久协调状态、连接信号 | 不捕获以后会过期的 View 回调作为永久逻辑 |
| Update | 写入专属属性和当前回调，协调本次子树 | 不重建 Widget，不重复连接相同信号，不手工应用共享修饰器 |
| Unmount | 清理自身 UI 连接、Timer 和协调资源 | 不再协调子树，不重复销毁 Window 拥有的 Widget 树 |

在同一挂载目标中，最终 WidgetView 的具体类型与 ID 相同会复用节点；未设 ID 时
按位置协调，重排容易改变状态归属。ID／类型变化或声明删除会释放旧节点。
新 View 对象不意味着新 Widget。通用修饰器由框架在 Update 之后应用；遗漏的
修饰器按首次 Mount 时的默认快照恢复，不需要自定义 View 重复调用 SetID 等。

单子目标使用 `ctx.UpdateChild(target, child)`，nil 明确清空；多子目标使用
`ctx.UpdateChildren(target, children)`，空列表明确清空，遗漏调用不会自动清空。
目标必须是稳定的非 nil 指针，不能每次 Update 新建适配目标；同一个目标不在
同节点混作 Bin 和 Container，也不由多个节点共同管理。

ui.Bin／Container 是挂载能力，不一定是 Widget。目标可保留 Widget 自己的结构
孩子，UI 只协调其管理的内容。多子适配要支持 MoveChildBefore，避免靠拆挂重排
丢失焦点、资源或 mount 状态。不要为自定义容器给 BuildContext 增加类型特例。

### 持久状态与最新回调

下面直接适配 GUI Button，展示只连接一次、每次更新当前回调的做法。数字由调用方
持有，不在瞬时 View 中自增；UI 子 Label 仍通过正常协调创建和更新。

```go
package custom

import (
    "fmt"

    "github.com/golang-gui/goui/core/signal"
    "github.com/golang-gui/goui/gui"
    "github.com/golang-gui/goui/ui"
)

type CounterView struct {
    ui.ViewBase[CounterView]
    value       int
    onIncrement func()
}

type counterState struct {
    onIncrement func()
    click       signal.Handle
}

func Counter(value int) *CounterView {
    v := &CounterView{value: value}
    v.Self = v
    return v
}

func (v *CounterView) OnIncrement(fn func()) *CounterView {
    v.onIncrement = fn
    return v
}

func (v *CounterView) Build() ui.View { return v }

func (*CounterView) Mount(ctx ui.BuildContext) gui.Widget {
    button := gui.NewButton()
    state := new(counterState)
    ctx.SetState(state)
    state.click = button.ConnectClicked(func() {
        if state.onIncrement != nil {
            state.onIncrement()
        }
    })
    return button
}

func (v *CounterView) Update(ctx ui.BuildContext, widget gui.Widget) {
    state := ctx.State().(*counterState)
    state.onIncrement = v.onIncrement
    ctx.UpdateChild(widget.(*gui.Button), ui.Label(fmt.Sprintf("次数：%d", v.value)))
}

func (*CounterView) Unmount(ctx ui.BuildContext, _ gui.Widget) {
    state, _ := ctx.State().(*counterState)
    if state != nil {
        state.onIncrement = nil
        state.click.Disconnect()
    }
}

var _ ui.WidgetView = (*CounterView)(nil)
```

调用方在 build 外创建 `count := ui.MakeState(0)`，在 build 内声明
`Counter(count.Get()).OnIncrement(func() { count.Update(func(n int) int { return n + 1 }) })`。
GUI 的点击信号闭包捕获持久 state，不捕获第一次 Mount 的 View，故重建后使用
新的回调。生产控件如会在 Update 中调用发信号的 setter，应按实际需要阻断自己的
连接，避免声明回写变成用户操作通知；不阻断无关外部监听，不滥用清空所有信号。

Unmount 与 Widget 的 ConnectUnmount 不是同一种生命周期：UI 节点释放、GUI
移交／拆挂和窗口销毁有不同顺序。UI 先清理共享连接及受管孩子，再调用自身
Unmount；窗口销毁路径不会由 UI 擅自拆掉 GUI 树。清理逻辑应可安全执行，不能假设
Unmount 时仍能启动子树协调或访问一个可绘制的原生窗口。

AfterUpdate 是一次性的整批协调完成钩子，发生在下一次布局之前，不是帧结束、
窗口已显示或原生几何已稳定的通知。不要在其中同步触发嵌套协调；异步任务可通过
App.Post 回 GUI 线程，但任务和借用对象的有效期仍由调用方负责。

## 7. 示例与源码阅读顺序

- `example/ui_test/main.go`：当前工作区旧例程中的 DrawWidget 与 RotatingImageView，
  展示 WidgetBase、ViewBase.Self、State 和 Mount／Update／Unmount。它是旧式用法
  例程，不是生产资源管理模板：不要照搬忽略 NewImage 错误、图片身份反射判断或
  假设 Painter 资源永远有效的写法。该例程不一定包含在所选依赖版本中，本文的
  完整代码不依赖它存在；本轮不修改或顺带收录这个已有例程。
- [example/demo/surface.go](../example/demo/surface.go)：样式化单子 Widget 与 UpdateChild；
  [ui/button.go](../ui/button.go)：持久 signal handle 与当前回调；
  [ui/box.go](../ui/box.go)：多子目标与 UpdateChildren。
- [ui/view.go](../ui/view.go)：公开扩展接口、Self 与共享修饰器；
  [ui/root.go](../ui/root.go)：展开、身份、协调与释放顺序。
- [gui/widget.go](../gui/widget.go)、[gui/painter.go](../gui/painter.go)：树、布局、
  生命周期、语义与局部绘图边界。另见 [Widget Painter](DesignWidgetPainter.md)、
  [布局](DesignLayout.md) 和 [Snapshot](DesignWidgetSnapshot.md)。

## 8. 验证自定义组件

首先编译外部包中的接口断言，不能只在 gui／ui 包内验证封印方法可实现。
包内逻辑验证布局约束、Parent／Children、setter 失效和 Snapshot；UI 验证 Self
链式类型、同 ID 更新／换 ID 替换、回调更新和连接释放。依赖真实窗口、排版、
绘图后端或输入法的部分再启动独立窗口程序，步骤与结果遵循 [Testing](Testing.md)。

不通过导出私有协调器或虚构“直接点击”接口来测试。没有桌面、只做编译或使用
测试上下文手动调用生命周期时，明确这种验证不等于真实窗口／完整协调流程通过。
