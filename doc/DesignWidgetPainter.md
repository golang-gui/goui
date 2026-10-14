# Widget Painter 设计

> 状态：已实现（2026-09）。本文是 `gui.Painter`、Widget 自绘和 GUI
> 子树遍历的权威说明。Surface 帧与平台图元见 `platform/graphics.Painter`；
> Transform 的矩阵约定见 `DesignTransform.md`。

## 1. 定位

GOUI 将绘图上下文和 Widget 树遍历严格分开：

- `platform/graphics.Painter` 绑定 Window/Popup surface，负责
  `Begin(width, height, scale)`、`End`、提交帧和平台资源。
- `gui.Painter` 是一次 `Widget.Paint` 中已经准备好的局部绘图上下文。
- GUI 内核负责遍历 Widget 树、建立局部原点、限制结构裁剪并恢复状态。

这与 Qt 的职责划分一致：`QPainter` 管理绘图状态和图元，QWidget 绘制体系
管理子控件，而不是让 Painter 认识 Widget。GOUI 不复制 QPainter 的全部可变
pen/brush 状态；Brush 和 stroke width 仍显式随绘制调用传入。

## 2. 公共接口

```go
type Painter interface {
    Save()
    Restore()

    NewImage(src image.Image) (graphics.Image, error)
    PixelScale() float32
    SetClipRect(rect geometry.Rectangle)
    SetTransform(matrix geometry.Transform)

    FillRect(rect geometry.Rectangle, brush graphics.Brush)
    FillRoundRect(rect geometry.Rectangle, radius float32, brush graphics.Brush)
    FillEllipse(center geometry.Point, xRadius, yRadius float32, brush graphics.Brush)
    FillPath(path graphics.Path, brush graphics.Brush)
    DrawLine(p0, p1 geometry.Point, strokeWidth float32, brush graphics.Brush)
    DrawRect(rect geometry.Rectangle, strokeWidth float32, brush graphics.Brush)
    DrawRoundRect(rect geometry.Rectangle, radius, strokeWidth float32, brush graphics.Brush)
    DrawEllipse(center geometry.Point, xRadius, yRadius, strokeWidth float32, brush graphics.Brush)
    DrawPath(path graphics.Path, strokeWidth float32, brush graphics.Brush)
    DrawTextLayout(origin geometry.Point, layout typography.TextLayout)
    DrawImage(rect geometry.Rectangle, img graphics.Image)
}
```

`gui.Painter` 不提供以下能力：

- 无参数 `Begin/End`：Widget 不拥有帧或绘制作用域的生命周期。
- `PaintChild` / `PaintChildren`：Painter 不知道 Widget 树。
- `Clear`：清理 surface 是 host 的帧操作，不是局部 Widget 操作。
- 单独的 `DeviceScale`：栅格内容使用包含当前变换的 PixelScale，避免只处理 DPI。

Painter 只能在同步执行的 `Widget.Paint` 内使用，不能保存到字段、异步调用或
并发调用。`NewImage` 返回的 `graphics.Image` 除外：Image 是独立的后端资源，
可以按其生命周期契约跨帧持有，但创建它时使用的 Painter 仍不能被持有。

PixelScale 返回物理像素/DIP 比例：surface HiDPI 乘当前变换的最大伸长，包含
非均匀缩放与剪切；退化/非有限比例为零。平移不影响它。Icon 等资源使用实际
分配尺寸与该比例决定栅格尺寸，而不是使用声明 Size 或仅屏幕 DPI。

内部 `newPainter(base, bounds, scale)` 显式接收本帧的基础缩放比例，与平台
Painter.Begin 使用同一个值；构造后不再修改。普通测试显式传 `1`。

图片的帧内释放属于 platform Painter 的资源生命周期：Image.Destroy 立即逻辑失效，
底层资源延迟到 End 消费或丢弃命令后释放。gui.Painter 不提供 AfterFrame，控件
替换资源时直接 Destroy。Icon 从已准备资源获取图像，不通过 Painter 或 App.Post
发送渲染错误通知；Post 不表示 GPU 已完成，也不用来释放绘制资源。

## 3. Widget 的使用方式

### 兄弟顺序与相对重排

Widget 是有序树。`Children()` 返回直接子节点的兄弟顺序：正序绘制、逆序命中；
默认 Snapshot 保持同序。使用顺序的布局（如 LinearBox）按此排列；具体控件也可
以自身的页面列表等数据决定布局，兄弟顺序不是通用内容索引，更不是键盘焦点顺序。
WidgetBase 的挂载列表是共同来源，不能只覆盖 Children 的返回顺序而不改变该列表。

控件实现者通过下列 helper 重排，不向 Widget 或 Container 接口增加自由排序要求：

```go
func (w *WidgetBase) MoveChildBefore(child, sibling Widget)
func (w *WidgetBase) MoveChildAfter(child, sibling Widget)
```

- Before 放到 sibling 紧前方；sibling 为 nil 时放到末尾（默认绘制最上层）。
- After 放到 sibling 紧后方；sibling 为 nil 时放到开头（默认绘制最下层）。
- child 与非 nil sibling 必须是同一接收者的直接子节点。child 为 nil、参数相同、
  位置已满足、节点已销毁或归属不符均无操作，不返回错误。
- 不接收未挂载节点，不跨父节点移动。AddChild/RemoveChild 继续负责挂载与卸载。
- 不改变归属、Root、可见性、焦点、控制器，不触发 mount/unmount；实际变化请求
  布局、绘制和语义更新。现有语义更新 hook 尚为保留入口，Snapshot 查询直接读取新顺序。
- 此操作可能影响布局，不是 paint-only 的 ZIndex。应用应使用具体控件的内容 API，
  如 TabView.MovePage；不再保留按绝对索引操作挂载列表的 MoveChild。

参考 GTK4 的 [insert_before](https://docs.gtk.org/gtk4/method.Widget.insert_before.html)
和 [insert_after](https://docs.gtk.org/gtk4/method.Widget.insert_after.html) 文档（查阅版本
4.23.4）：相对兄弟定位、nil 首尾语义及“用于控件实现者”的分层。GOUI 不复制其
插入与重排合并行为，只允许已有同父节点重排。GTK 4.18.6 的
[默认 snapshot 源码](https://github.com/GNOME/gtk/blob/4.18.6/gtk/gtkwidget.c#L735-L744)
正序遍历兄弟，但 GTK snapshot 是绘制过程，不等同于 GOUI 语义快照。
Flutter [move](https://api.flutter.dev/flutter/rendering/ContainerRenderObjectMixin/move.html)
同样提供同父节点相对移动，默认 [paint](https://api.flutter.dev/flutter/rendering/RenderBoxContainerDefaultsMixin/defaultPaint.html)
与 [hit test](https://api.flutter.dev/flutter/rendering/RenderBoxContainerDefaultsMixin/defaultHitTestChildren.html)
分别正序、逆序；GOUI 采用这一默认配对，不引入独立叠放属性或新的渲染树。

### 局部绘制

`Widget.Paint` 只绘制当前 Widget 自身。普通控件不需要保存状态：

```go
func (b *Badge) Paint(p gui.Painter) {
    bounds := geometry.Rect(0, 0, b.Rect().Width, b.Rect().Height)
    p.FillRoundRect(bounds, 6, b.background)
    p.DrawTextLayout(geometry.Point{X: 8, Y: 4}, b.textLayout)
}
```

需要在同一次 Paint 内临时修改状态时，使用成对的 `Save/Restore`：

```go
func (c *Chart) Paint(p gui.Painter) {
    bounds := geometry.Rect(0, 0, c.Rect().Width, c.Rect().Height)
    p.FillRect(bounds, c.background)

    p.Save()
    p.SetClipRect(geometry.Rect(8, 8, bounds.Width-16, bounds.Height-16))
    p.SetTransform(c.dataTransform)
    c.drawSeries(p)
    p.Restore()

    p.DrawTextLayout(geometry.Point{X: 8, Y: 8}, c.caption)
}
```

容器也只画自身背景或装饰；子节点在返回后由框架绘制：

```go
func (p *Panel) Paint(painter gui.Painter) {
    bounds := geometry.Rect(0, 0, p.Rect().Width, p.Rect().Height)
    painter.FillRoundRect(bounds, 8, p.background)
}
```

子节点按照添加顺序绘制，最后一个子节点位于最上方；命中测试反向遍历同一
顺序。需要覆盖其他子节点的装饰应表示为最后一个装饰/Overlay Widget，不能在
父 Widget 中手工调用子节点的 `Paint`。

## 4. 自动遍历

每个 Window 和原生 Popup 在一帧中只创建一个 GUI `painter`。内部遍历等价于：

```text
paintWidget(widget):
    忽略 nil 或不可见 Widget
    保存父结构状态
    建立 Widget 窗口原点和结构 clip
    调用 widget.Paint
    恢复 Widget 入口绘图状态
    按顺序 paintWidget(visible child)
    恢复父结构状态
```

遍历直接读取 `WidgetBase` 的规范 children slice，不调用会复制 slice 的公开
`Children()`。因此 `Paint` 必须只读 Widget 状态，不能在绘制过程中修改树。

父 Widget 的显式 clip 和 transform 在进入第一个子节点前已经恢复。子 Widget
只继承由 Widget 层级形成的窗口原点和结构 clip，不继承父 Widget 的绘图状态。

## 5. 坐标、Transform 与 Clip

Painter 内部状态为：

```go
type painterState struct {
    scopeClip geometry.Rectangle // Widget 树确定的窗口坐标裁剪
    clip      geometry.Rectangle // 当前有效窗口坐标裁剪
    offset    geometry.Point     // Widget local -> window 的原点
    userXform geometry.Transform // 当前 Widget 的局部绝对变换
}
```

进入 Widget 时：

```text
offset    = parent.offset + widget.Rect().Pos
scopeClip = parent.scopeClip ∩ Translate(widget local bounds, offset)
clip      = scopeClip
userXform = Identity
```

绘制图元仍接收 Widget-local 坐标，有效矩阵为：

```go
geometry.Translate(offset.X, offset.Y).Multiply(userXform)
```

`SetTransform` 是替换操作，只影响当前 Widget。本设计不把 paint transform 传播
给子树，因为布局、命中测试和语义边界目前都没有相应变换。未来若需要整体缩放
或旋转控件树，应增加统一影响绘制、输入和语义的 Widget/scene 能力。

`SetClipRect` 也接收 Widget-local 矩形，但它属于布局空间：先平移到窗口坐标，
再与不可扩大的 `scopeClip` 求交。它不随 `userXform` 旋转或缩放。当前底层只提供
轴对齐矩形裁剪；精确的变换路径裁剪留给单独的 ClipPath 设计。

连续调用 `SetClipRect` 会在 `scopeClip` 内替换前一次显式 clip，而不是持续求交。
要临时缩小后恢复，应使用 `Save/Restore`。空有效 clip 会在 GUI 层抑制所有绘制，
因为底层 `graphics.Painter` 使用空矩形表达“取消平台裁剪”。

## 6. 状态平衡与诊断

- `Save` 保存当前 clip 和 transform。
- `Restore` 只能弹出当前 Widget 自己保存的状态；越过 Widget 入口立即 panic。
- Paint 正常返回但遗漏 Restore 时，框架先恢复入口状态，再用包含 Widget 类型和
  ID 的信息 panic。
- Paint 自身 panic 时，框架恢复 clip、transform、保存栈和父状态，然后继续传播
  原始 panic，不用附带诊断覆盖它。

状态恢复既是调试检查，也是隔离保证：一个错误 Widget 不会把绘图状态留给后续
兄弟节点或下一次子树绘制。

## 7. ScrollView viewport

ScrollView 使用真实的内部结构节点表达裁剪，而不是特殊 Painter 调用：

```text
ScrollView
├── scrollViewport
│   └── content
├── vertical ScrollBar
└── horizontal ScrollBar
```

- viewport 的矩形等于扣除可见滚动条后的 content area。
- 普通内容以负 scroll offset 排列在 viewport 内；虚拟内容仍通过
  `Scrollable.LayoutVisible` 接收 offset。
- viewport 的 Widget 边界自然成为内容及后代的结构 clip。
- 滚动条是后置兄弟节点，因而绘制和命中顺序都位于内容之上。
- `ScrollView.Child()` 仍返回 content，但 content 的真实 `Parent()` 是 viewport。
- 语义快照过滤 viewport 和滚动条，只暴露应用内容。

## 8. 验证要点

- Widget-local 坐标、嵌套窗口原点和祖先结构裁剪。
- 父 Paint 状态不影响子节点，兄弟节点相互隔离。
- 可见性、树顺序、空 clip 和连续替换 clip。
- Save/Restore 嵌套、越界、遗漏以及 Paint panic 后恢复。
- ScrollView viewport 的绘制裁剪、滚动条命中顺序和语义过滤。
- Window 与 Popup host 使用同一个自动遍历入口。
