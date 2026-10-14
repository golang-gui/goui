# DesignScroll — ScrollView 增强 + ListView 虚拟化

> 2026-10-07 补齐 ScrollView.Direction（默认 Both，Vertical/Horizontal 为单轴），
> 修正双条相互占位。普通内容单轴测量、输入与虚拟内容的最新契约见
> [滚动布局切片](DesignScrollLayout.md)，本文件的历史实现示意不覆盖该契约。

## 目标

1. 补齐 `ScrollView`：滚轮交互、内建滚动条 thumb、滚动边界 clamp。
2. 引入「内容方抽象」`Scrollable`，让支持滚动的内容（ListView 等）
   与不支持滚动的普通 widget 树都能被 ScrollView 承载。
3. 在 Scrollable 上实现 `ListView`（虚拟化：只布局可见 item）。

范围：gui 命令式控件与 ui 薄适配均已实现。

## 当前实现

ScrollView 已支持滚轮、双轴滚动条、范围钳制、普通内容 viewport 和 Scrollable。
Direction 默认 Both；Vertical／Horizontal 对普通内容提供有限交叉轴约束。
ListView 已实现测量式变高行、行壳复用、宽度／高度缓存、Refresh／Reveal。
GUI 与 UI 接入均已完成；当前源码入口为
[ScrollView](../gui/scroll_view.go)、[ListView](../gui/list_view.go) 和 [UI ListView](../ui/list_view.go)。

## 业界参考

| 框架 | 抽象 | 模型 |
|---|---|---|
| GTK | `GtkScrollable` 接口 | 内容、滚动容器与滚动条共享 adjustment；容器处理滚轮等交互，内容更新范围并按位置显示。不实现 = 用 `GtkViewport` 包普通 widget |
| Qt | `QAbstractScrollArea` 基类 | `QAbstractItemView` 继承它自带滚动 + 虚拟化；`QScrollArea` 是另一个子类包普通 widget |
| Flutter | `Sliver` 接口 + `SliverConstraints` | `Scrollable`（位置）→ `Viewport`（位置→约束）→ sliver 按约束只构建可见子项 |

**共同点**：滚动位置是共享状态；内容知道自己的总尺寸并能按偏移只布局可见
部分；宿主（滚动条/裁剪）与内容（布局/绘制）解耦。

## 后续扩展优先评估：独立滚动模型

当前保留 `Scrollable` 和 `ScrollIntoViewRequester`，不在本轮修改公共接口。
后续扩展滚动同步、可变高度虚拟化、视口锚定或自动滚动时，优先评估独立滚动
模型，而不是持续向“让矩形可见”的请求接口叠加职责；这是一项评估方向，不是
已经确定名称或承诺实现的公共 API。

### GTK4 的参考设计与证据

- API 契约：`GtkScrollable` 接入横向、纵向 `GtkAdjustment`。内容在布局时更新
  范围与页面大小，监听 `value-changed` 更新显示。Adjustment 是包含当前位置、
  上下界、页面大小和步进的独立数值模型。
- 源码行为（GTK 4.20.0，GTK 控件层，不限定桌面平台或绘制后端）：
  `GtkScrolledWindow` 将同一组 Adjustment 交给内容和滚动条；`GtkTextView`
  计算目标位置后修改 Adjustment，并非向父容器发送显示矩形请求。
- 布局时序仍然需要处理：`scroll_to_iter()` 使用当前已计算的行高；官方文档
  提醒布局未完成时结果可能不符合预期，`scroll_to_mark()` 可以保存目标，在
  行布局校验后滚动。共享模型不会自动消除延迟测量与定位的协调问题。

参考：官方 [GtkScrollable](https://docs.gtk.org/gtk4/iface.Scrollable.html)、
[GtkAdjustment](https://docs.gtk.org/gtk4/class.Adjustment.html)、
[TextView.scroll_to_iter](https://docs.gtk.org/gtk4/method.TextView.scroll_to_iter.html)
（本次核对在线文档标注版本 4.23.4）；固定版本源码
[gtkscrolledwindow.c](https://github.com/GNOME/gtk/blob/4.20.0/gtk/gtkscrolledwindow.c#L3950-L4000)、
[gtktextview.c](https://github.com/GNOME/gtk/blob/4.20.0/gtk/gtktextview.c#L2657-L2696)。
以上为文档与源码证据，不代表本项目已完成 GTK 对照运行验证。

### GOUI 的取舍与评估条件

当前滚动位置由 `ScrollView` 持有，内容通过信号请求矩形可见，宿主决定偏移并
限制范围，再通过 `LayoutVisible` 通知内容。自动连接仅覆盖直接内容，不是任意
后代的冒泡机制。该方案适合当前需求，不必只为光标跟随引入完整 Adjustment。

但 TextView 目前也使用该通道协调视口锚点修正和拖选自动滚动，语义已经超出
单纯的“显示目标矩形”。后续设计应分别审视：目标可见性、滚动位置控制、内容
范围更新和布局后锚点修正，避免把它们都包装成矩形请求。

独立模型允许内容、容器和滚动条通过明确契约协调位置，并不意味着内容耦合到
具体父容器。若采用该方向，必须说明所有者、连接生命周期、范围更新职责、
双向通知及重入规则、布局完成时机和嵌套滚动边界，并验证新模型确实减少现有
协调代码；不照搬 GObject 属性系统，也不与旧接口形成两套滚动状态来源。

## 决策

| 决策 | 结论 |
|---|---|
| 内容抽象 | `Scrollable` 接口 + 双模式（Scrollable 模式 / Viewport 模式） |
| ListView 形态 | 纯内容，实现 `Scrollable`，被 ScrollView 包裹使用 |
| 接口方法 | `ContentSize() + LayoutVisible(viewport, offset)` 两方法 |
| 滚动条 | 独立公共 `ScrollBar` 控件（占布局空间、不浮动叠加），挂 `DragEventController` 支持 thumb 拖拽与 track 跳转；样式走 part（`trough`/`thumb`）+ state |
| 滚动方向 | 垂直 + 水平双轴；可滚才显示对应轴滚动条 |
| 滚轮行高 | ScrollView 的 LineHeight／默认字体行高负责步进，不要求 ListView 有固定 itemHeight |
| 虚拟化 | Bind 后实测行高；未测项估计、宽度与布局失效后重测，无固定行高前提 |

## 1. Scrollable 接口（内容方抽象）

```go
// Scrollable 由「可滚动内容」实现：内容知道自己的总尺寸，
// 并能按视口偏移只布局可见部分。
// 对应 GTK GtkScrollable / Flutter Sliver。
type Scrollable interface {
	Widget
	// ContentSize 返回内容总尺寸（用于 clamp 与滚动条比例）。
	ContentSize() geometry.Size
	// LayoutVisible 布局可见内容：viewport 是可见区域尺寸，
	// offset 是滚动偏移（内容把子项放到 -offset 位置）。
	LayoutVisible(viewport geometry.Size, offset geometry.Point)
}
```

### 与 Flutter 的对应

| GOUI | Flutter |
|---|---|
| `ScrollView`（宿主） | `Scrollable` + `Viewport` |
| `Scrollable.LayoutVisible(viewport, offset)` | `Sliver.performLayout(SliverConstraints)` |
| `ContentSize()` | `SliverGeometry.scrollExtent` |
| `ListView`（实现 Scrollable） | `SliverList` |

## 2. ScrollView（滚动宿主）

### 2.1 状态与接口

ScrollView 持有偏移、视口和内容范围；内部 viewport 是内容的真实父节点，
两个 ScrollBar 是其后置兄弟。Child／SetChild 管理唯一业务内容，不把滚动条
暴露成应用内容槽；Children 是 Widget 的真实子树接口，Snapshot 可过滤结构节点。

SetScrollX／SetScrollY 将偏移取整到整数 DIP，并钳制至实际范围，变化时沿正常
布局／绘制失效路径更新。SetLineHeight 提供 WheelDeltaLine 的步进。
滚动条宽度 12 DIP、胶囊半径 6 DIP、最短 thumb 20 DIP；实际命中宽度与绘制宽度一致。

固定字段布局不作为公共契约，缓存／失效细节以源码为准；不保留过时 struct 副本。

### 2.2 双模式（核心）

| 模式 | content 类型 | Measure | Arrange |
|---|---|---|---|
| **Scrollable 模式** | 实现 `Scrollable` | 保留 Child 的固有宽度；有限父级高度作为 viewport，高度无界时没有固有 viewport 高度；默认 `MainWeight(1)` 在主轴弹性填充 | 调 `content.LayoutVisible(viewport, offset)`，content 自己布局可见项 |
| **Viewport 模式**（现状） | 普通 widget 树 | 滚动轴 `layout.Inf` 测 content；保留内容宽度，有限父级高度作为 viewport | `content.Arrange(Rect(-offset, contentSize))`，结构 viewport 裁剪 |

普通内容按 Direction 放松滚动轴，Arrange 时按实际 viewport 重测；
虚拟内容的 ContentSize／LayoutVisible 契约不由 Direction 自动改写。
双条相互占位及尺寸收敛以 [滚动布局切片](DesignScrollLayout.md) 为准。

> **viewport 高度与内容高度分离（决策记录）**：垂直 ScrollView 的 Child 宽度
> 是有用的固有尺寸；有限父级高度是 viewport 尺寸，不是内容固有高度；高度无界时
> 才返回无固有 viewport 高度。`LinearLayout` 对 `MainWeight` child 使用主轴无界
> 约束计算 flex basis，再把剩余空间分配给 viewport，避免两个 ScrollView 各自申报
> 整个 `c.Max` 导致溢出。

### 2.3 弹性测量（无固有尺寸容器）

本节以垂直滚动轴为例描述测量基线：Child 宽度作为固有宽度，滚动高度没有固有尺寸，
有限父级高度作为 viewport，构造时默认 `SetMainWeight(1)`。
`MainWeight` 只参与父级 LinearLayout 的主轴空间分配，`CrossAlign` 仍是
LinearBox 的容器级策略；ScrollView 不需要知道父级是 HBox 还是 VBox。

| 场景 | 结果 |
|---|---|
| `HBox(ScrollView, ScrollView)` | Child 宽度作为基线，`MainWeight(1)` 均分剩余宽度；有限父级高度成为 viewport |
| `HBox(...).CrossAlign(CrossStretch)` | 容器显式让所有 children 填满交叉轴 |
| `VBox(ScrollView, ScrollView)` | 加权 child 的主轴无界基线为 0，`MainWeight(1)` 均分高度，Child 宽度作为交叉轴尺寸 |
| `VBox(Label, ScrollView)` | Label 保自然高，ScrollView 拿剩余主轴空间并保留 Child 宽度 |
| Window 根（`Tight(窗口)`） | `c.Min = c.Max` = 窗口尺寸，照常填满 |
| `SetMainWeight(0)` | 关闭主轴弹性；按自身测量结果布局 |

> **职责边界（决策记录）**：LinearLayout 不依赖具体的 ScrollView、ListView
> 类型。它读取 child 的 `MainWeight`，并使用自身的 `CrossAlign` 统一安排 children；
> weighted child 先以主轴无界约束测量 flex basis，再分配剩余主轴空间。

Arrange 根据实际内容区安排 viewport 与 ScrollBar。普通内容在 viewport 内
以负偏移布局；Scrollable 接收实际 viewport 与双轴 offset，自行决定可见行。
ListView 已处理水平偏移，不再把 offset.X 当作忽略字段。
范围变化后使用最终内容区钳制；滚动条实际占位与方向策略见最新滚动布局契约。

### 2.4 滚轮交互

`WheelEventController` 是公共 `EventController`（与 `MotionEventController`
同层，见 `gui/input_controller.go`），`PhaseBubble` 处理 `WheelEvent`，通过
`ConnectScroll(fn func(ctx EventContext, event events.WheelEvent))` 暴露。
ScrollView 构造时接入并滚动：

```go
sv.wheel = NewWheelEventController()
sv.wheel.ConnectScroll(func(_ EventContext, e events.WheelEvent) {
	dx, dy := e.DeltaX, e.DeltaY
	if dy != 0 && e.Modifiers&events.ModifierShift != 0 { // Shift+竖滚 = 水平
		dx, dy = dy, 0
	}
	sv.SetScrollX(sv.ScrollX() + sv.wheelDelta(dx, e.Mode))
	sv.SetScrollY(sv.ScrollY() + sv.wheelDelta(dy, e.Mode))
})
sv.AddEventController(sv.wheel)
```

- `wheelDelta(delta, mode)`：`WheelDeltaLine` 乘 `lineHeightPx()`，`WheelDeltaPixel`
  直接返回 delta。
- DeltaX 原生横向滚轮；Shift+竖滚转水平（与 GTK/Flutter 一致）。
- `SetScroll{X,Y}` 内部取整到整数像素，滚轮累计的小数行高不会让内容落在
  分数坐标。

### 2.5 滚动条（独立 `ScrollBar` 控件）

滚动条不是浮动在内容上的自绘局部，而是独立的公共 `ScrollBar` widget：
占自己的布局空间、参与命中测试、上面挂 `DragEventController`。

**ScrollView 与 ScrollBar 在样式系统里是整体**：共用同一样式名
（默认 `"scroll-view"`，用户给 ScrollView 设 `StyleName` 时下推给两个 bar）。
`ScrollBar` 按 **part** 解析样式：

| part | 含义 | 默认规则 |
|---|---|---|
| `trough` | 凹槽/轨道 | `BackgroundColor(rgba(0,0,0,24))`, `Radius(6)` |
| `thumb` | 滑块 | `BackgroundColor(rgba(140,140,140,255))`, `Radius(6)`；`Hovered`→较深；`Pressed`→更深 |

经 `fallbackChain`（`style.go:resolveRules`），part 规则继承基础 `Name("scroll-view")`
规则：主题只写 base 即可统一调色，也可按 part 精细覆盖。

`ScrollBar.Paint` 用自身矩形（局部坐标，分方向）：
- `trackStyle() = ResolveStyle(name, "trough", Normal)` → 填充轨道。
- `thumbStyle() = ResolveStyle(name, "thumb", thumbState)` → 填充圆角 thumb。
  `thumbState ∈ {Normal, Hovered(指针在 thumb 内), Pressed(拖拽中)}`——hover 由
  `MotionEventController` 驱动、pressed 由 `DragEventController` 驱动。

### 2.5.1 ScrollBar 控件骨架（`gui/scroll_bar.go`）

```go
// 使用 layout.Direction，不定义另一套 Orientation 枚举。
type ScrollBar struct {
	WidgetBase
	orientation layout.Direction
	value, max  float32             // 当前偏移 + 可滚总长（max<=0 不可滚）
	grabOffset  float32             // 按下点到 thumb 起始边的偏移（避免 thumb 跳到指针中心）
	change      signal.Signal1[float32]  // 请求的新 value（消费者 clamp）
	drag        *DragEventController      // 挂本控件（无条件 begin：控件矩形就是轨道）
	motion      *MotionEventController    // thumb hover
}
```

- 公共 API：`NewScrollBar(ori)`、`SetOrientation`/`SetValue`/`SetMax`/
  `ConnectChange(fn func(value float32))`。
- 样式名默认空 → 回退 `"scroll-view"`（与 ScrollView 一体）。
- **Drag 交互（jump-then-drag）**：
  - `ConnectBegin(pos)`：若按在 thumb → 记 `grabOffset`；若按在 trough 非 thumb →
    先 jump（value 设得 thumb 中心对准 pos，`grabOffset=0`），立即 emit change。
  - `ConnectUpdate(pos)`：`value=(pos-grab-troughStart)/(troughLen-thumbLen)*max`，
    emit change。
  - `ConnectEnd`：清拖拽状态。
- 挂自身的 `DragEventController` **无条件 begin**（控件矩形就是轨道，按在内容上
  不会命中 ScrollBar）——无需 `SetStartFunc` 条件启动。
- `Paint`：trough + thumb，全部 ScrollBar 局部坐标。
- `Snapshot`：`RoleScrollBar` + value/max。
- **几何**（沿用现有公式）：
  - 垂直：`trough = Rect(0,0,W,barLen)`；`thumbLen = max(minThumb, barLen²/content)`；
    `thumbStart = (barLen-thumbLen)*value/(content-barLen)`。
  - 水平：轴交换。

### 2.5.2 ScrollView 布局与裁剪

ScrollView 使用一个私有结构 viewport 加两个 ScrollBar。`Arrange` 显式布局
这些结构节点和内容：

```
contentArea = Rect(0, 0, w - vbarW?, h - hbarH?)   // 可滚才扣
viewport → contentArea
content → viewport 坐标系，含 -scrollX/-scrollY 偏移
vbar → Rect(w-barW, 0, barW, h-hbarH?)   // 可滚才 SetVisible(true)
hbar → Rect(0, h-barH, w-vbarW?, barH)
```

**可滚才显示**：某轴 `max<=0` 时 `SetVisible(false)`，空间并入内容区。

**结构裁剪**（内容不画进滚动条区）：

```text
ScrollView
├── scrollViewport
│   └── content
├── vertical ScrollBar
└── horizontal ScrollBar
```

`content.Rect()` 保留 scroll 偏移
（`Rect(-scrollX,-scrollY,contentW,contentH)`）。GUI 的自动 Widget 遍历将
content 的结构 clip 与 viewport 边界相交，因此不需要 ScrollView 手动调用
Painter 或设置特殊 clip。滚动条是 viewport 后面的兄弟节点，绘制在内容之上，
反向 hit-test 也会优先命中滚动条。

**StyleName 下推**：ScrollView 重写 `SetStyleName` 把同名同步给 vbar/hbar，
保证 `ScrollView + ScrollBar` 在样式系统里是整体：
```go
func (sv *ScrollView) SetStyleName(name string) {
	sv.WidgetBase.SetStyleName(name)
	sv.vbar.SetStyleName(name)
	sv.hbar.SetStyleName(name)
}
```

**结构节点挂入子树**：ScrollView 是 Bin（ui 层依赖 `Child/SetChild` 管理内容槽）。
viewport 与滚动条通过 WidgetBase 的内部子树管理挂入；content 挂在 viewport 下，
但 `Child()`/`SetChild()` 仍只暴露和管理 content。语义快照过滤 viewport 和
滚动条。`hitTest` 沿真实子树命中，隐藏的 bar 会被跳过。

### 2.6 clamp

```go
func (sv *ScrollView) clampScroll() {
	maxY := mathx.Floor(sv.contentHeight - sv.contentAreaSize().Height)
	if maxY < 0 { maxY = 0 }
	if sv.scrollY > maxY { sv.scrollY = maxY }
	if sv.scrollY < 0 { sv.scrollY = 0 }
	maxX := mathx.Floor(sv.contentWidth - sv.contentAreaSize().Width)
	if maxX < 0 { maxX = 0 }
	if sv.scrollX > maxX { sv.scrollX = maxX }
	if sv.scrollX < 0 { sv.scrollX = 0 }
}
```

`SetScrollY`/`SetScrollX` 与 `Arrange` 后都调用。`contentAreaSize` 已扣除可见
滚动条宽度，clamp 基于内容区而非整个控件矩形。

## 3. ListView（虚拟内容，实现 Scrollable）

### 3.0 设计原则（v2 修订）

初版把数据（数量、高度）和渲染（`SetRenderItem`）揉在 ListView 里。参考
GTK（GListModel + GtkListItemFactory）、Qt（QAbstractItemModel +
QAbstractItemDelegate）、fyne（List 的 CreateItem/UpdateItem），拆为三层：

| 角色 | 职责 | 对应 |
|---|---|---|
| `ListModel` | 数据：数量、按索引取值、变更通知 | GListModel / QAbstractItemModel |
| `ListItemDelegate` | 渲染：创建空壳、绑定、解绑 | GtkListItemFactory / fyne UpdateItem |
| `ListView` | 虚拟化：可见范围、测量、复用 | GtkListView / QListView |

- **数据层只读 + 订阅**（consumer-side interface）：CRUD 由具体实现
  （`SliceListModel[T]`）提供，接口不声明——ListView 只消费。
- **测量式高度**：每行高度来自 item 自身 `Measure`（内容自适应：多行文本、
  图片行不需要外部告知高度）；固定行高分配（`SetItemExtent`）留待后续优化。
- **变更信号无参**：`ConnectItems(f func())`，收到通知全量 reload。
  增量对比（index 平移、Update 保留实例）留待后续；届时可改为带参信号或
  ListView 内部 diff。
- **Unbind 是重要生命周期**：delegate 在 Bind 里连接了 item 内部信号时，
  Unbind 是防泄漏钩子（GTK 同样）。简单 delegate 可空实现；后续提供便捷
  包装（仅需 Setup/Bind 甚至仅 Bind）。

### 3.1 数据层：结构信息与数据访问分离

**关键设计**：`ListView` 内部只消费结构信息（`ItemsCount`/`ConnectItems`），
item 数据只有 delegate 实现者需要——所以拆成两个接口，**`ListView` 保持
非泛型**（index 是唯一通信），泛型只留在数据访问：

```go
// ListModel：结构层（非泛型）——ListView 用它驱动虚拟化
type ListModel interface {
	ItemsCount() int
	ConnectItems(f func()) signal.Handle
}

// ListData[T]：数据访问层（泛型）——delegate/声明式包装用它取 item，
// 编译期类型检查：ListData[int] 不能被 string model 满足。
type ListData[T any] interface {
	ListModel
	ItemAt(index int) T
}
```

`SliceListModel[T]` 便捷实现（**泛型** slice 适配，实现 `ListData[T]`，
含 CRUD，每次变更 emit）：

```go
func NewSliceListModel[T any](items []T) *SliceListModel[T]
func (m *SliceListModel[T]) ItemsCount() int
func (m *SliceListModel[T]) ItemAt(i int) T
func (m *SliceListModel[T]) Modify(f func(prev []T) (after []T)) // 批量：锁内改，一次 emit
func (m *SliceListModel[T]) Insert(index int, item T) // 越界 panic（slice 语义）
func (m *SliceListModel[T]) Append(item T)
func (m *SliceListModel[T]) Remove(index int)
func (m *SliceListModel[T]) Set(index int, item T)
func (m *SliceListModel[T]) SetItems(items []T) // 全量替换
func (m *SliceListModel[T]) ConnectItems(f func()) signal.Handle
```

**并发安全**：`sync.RWMutex` 保护数据（读 RLock、写 Lock）；变更通知在
**锁外 Emit**（`Signal0.Emit` 同步执行回调，锁内 Emit 会让回调调 model
方法时死锁）。`Modify` 的 `f` 在锁内执行（原子读改写），f 内不得回调
model 方法，直接改传入切片或返回新切片；批量操作（多次插删改）合并为
一次通知。

### 3.2 渲染层 ListItemDelegate

```go
// ListItemDelegate renders model items as widgets. Implementations hold a
// reference to the ListModel they render (fyne style) and read data from
// it inside Bind.
type ListItemDelegate interface {
	Setup() Widget            // 首次创建空壳 widget（虚拟化复用池，只调一次）
	Bind(index int, w Widget) // 把 index 的数据绑定到 w（滚动复用反复调用）
	Unbind(index int, w Widget) // 解绑（防泄漏钩子；简单 delegate 可空实现）
}
```

### 3.3 ListView：测量式虚拟化与失效

可见行保存在 items，脱离视口的壳先 Unbind 再放入 pool。Bind 后测量得到高度
与自然宽度；未测行用首个有效实测值／已测均值估计，范围随后续测量修正。

宽度改变或样式失效清理尺寸缓存与定位前缀，避免旧 prefix 影响反向滚动。
局部 RequestLayout 由测量有效性检查感知，不让索引缓存遮蔽子控件失效。
Refresh 保留模型和 Delegate，下一布局重绑；Reveal 通过 ScrollIntoView 请求定位，
完成需要实际测得目标行，不能仅靠离屏估计宣称可见。

SetModel 每次重新安装订阅，即使是同一实例；SetDelegate 每次解除旧绑定、清理
不再可复用的旧壳。声明层负责避免重复设置。普通重载保持同 Delegate 行壳池，
不把模型数据、绑定状态和像素资源混成一个缓存。

定位以已有 first／firstY 增量调整；高度与均值改变须使定位估计失效。
总高为已测行高之和加未测项估计，水平范围来自测得宽度；范围变动请求布局，
稳定测量不反复制造布局请求。具体字段与回调重入检查见源码，不复制私有 struct。

滚轮由 ScrollView 提供，当前 LineHeight 不自动取 ListView 的 estimate。
共享滚动模型、固定行高及增量刷新仍为 [待评估方向](Roadmap.md#future-scroll)，
不把它们写成下一步必加的公共 API。

### 3.4 使用方式

```go
type labelDelegate struct{ model gui.ListData[int] }

func (d *labelDelegate) Setup() gui.Widget { return gui.NewLabel("") }
func (d *labelDelegate) Bind(i int, w gui.Widget) {
	w.(*gui.Label).SetText(fmt.Sprintf("Item %d: %v", i, d.model.ItemAt(i)))
}
func (d *labelDelegate) Unbind(i int, w gui.Widget) {} // 简单实现可空

data := make([]int, 1000)
for i := range data { data[i] = i }
model := gui.NewSliceListModel(data)

list := gui.NewListView()
list.SetModel(model)
list.SetDelegate(&labelDelegate{model: model})

scroll := gui.NewScrollView()
scroll.SetChild(list)
```

数据变更：`model.Append(x)` / `model.Set(i, x)` 自动 emit → ListView 全量
reload（解绑并清索引测量，保留同 Delegate 的可复用壳，再 Bind／Measure 可见项）。

## 4. ui 层（声明式）

`ui` 层薄适配（已实现，SwiftUI `List` / GTK Builder 同构）。**泛型类型
检查**：`ListView[T]` 的 model 参数是 `gui.ListData[T]`，builder 直接收
`T`——`ListView[int]` 配 string model **编译不过**：

```go
model := ui.SliceList(data) // 便捷：一行得到 gui.ListData[T]
ui.ScrollView(
	ui.ListView(model, func(i int, data int) View {
		return ui.HBox(
			ui.Label(fmt.Sprintf("%4d.", i)),
			ui.Label(fmt.Sprintf("value %d", data)),
		).Spacing(8)
	}),
)
```

- `ui.ListView[T](model gui.ListData[T], item func(index int, data T) View)`：
  item 闭包返回声明式 View 树（SwiftUI `List(items) { item in ... }` 对应；
  Go 闭包替代 GTK Builder 的 XML）。链式修改器 `.Model(m)` / `.Item(fn)`。
- `ui.SliceList[T](slice []T) gui.ListData[T]`：`gui.NewSliceListModel` 的
  一行便捷。
- **协调机制**（声明式 item × 命令式复用池的桥接）：内部 `uiItemDelegate[T]`
  实现 `gui.ListItemDelegate`——
  - `Setup()` 返回空壳 `gui.NewLinearBox`（可挂 child + 测量 child）；
  - Bind 借用所属节点的 BuildContext，通过 UpdateChildren(shell, views) 协调内容；
    model.ItemAt(index) 返回 T，不另存私有 node 映射或捕获创建时的 Root；
  - Unbind 通过 UpdateChildren(shell, nil) 清理声明节点和连接；业务状态由模型持有，
    不承诺未实现的节点缓存池。
- UI 复用稳定 Delegate，更新最新 model／builder；相同非 nil 模型指针不重复安装。
  GUI SetModel／SetDelegate 每次显式调用均重新安装，不猜测接口相等；普通展示刷新
  使用 Refresh。换 T 会改变具体 View 类型，遵循通用协调规则。
- item 构造是命令式 gui 调用（ui 层薄适配定位）；声明式协调树只在
  Bind 时机工作，node 数有界（≈ 复用池大小）。

## 5. 语义快照（AI / 自动化友好）

ScrollView 与 ListView 的快照已对齐 WAI-ARIA 语义，AI/自动化可通过
`Snapshot()` 理解列表结构与滚动状态（执行滚动仍派发 `WheelEvent`）：

- **角色**：`RoleScrollView`（"scrollview"）、`RoleScrollBar`（"scrollbar"，
  ScrollView 内建）、`RoleList`（"list"）、`RoleListItem`（"listitem"，列表项容器；
  shell 的 box 噪音在快照中被改写，AI 直接看到 `list → listitem → 内容`）。
- **滚动状态**（`WidgetInfo` 字段，`omitempty`——非滚动控件不出现）：
  - `scrollY`/`scrollX`：当前滚动偏移
  - `maxScrollY`/`maxScrollX`：可滚范围（contentSize - viewportSize，≥0）
  - `itemCount`：ListView 总项数（虚拟化——AI 知道视口外还有更多行）
  - `visibleStart` / `visibleEnd`：当前可见 index 范围
- **ScrollView 快照过滤滚动条**：子级只含内容，避免 bar 噪音。
- **无 scroll Action**：滚动是连续操作，AI 经滚轮输入派发执行（AGENTS.md：
  执行操作必须派发平台输入事件）；`Action` 只表达离散动作（click/focus）。

## 6. 原实现切片的文件范围（历史）

| 文件 | 改动 |
|---|---|
| `gui/scroll_bar.go` | 新文件：独立公共 `ScrollBar` 控件（Drag + Motion + part 样式 + 几何 + Paint） |
| `gui/scroll_view.go` | 重写：SetChild、双模式、滚轮双轴、双 ScrollBar 子控件、内容区裁剪、scrollX、clamp、layout.Inf |
| `gui/style.go` | 新增 `trough`/`thumb` part 常量 + DefaultStyleRules part 规则 |
| `gui/scroll_view_test.go` | 滚轮滚动、clamp、滚动条可见性、双模式单测 |
| `gui/list_model.go` | 新文件：ListModel 接口 + 泛型 SliceListModel[T]（CRUD + ConnectItems） |
| `gui/list_view.go` | 重写：ListView（model/delegate 分离、测量式瀑布虚拟化、复用池、信号 reload） |
| `gui/list_view_test.go` | ContentSize 估算、瀑布布局、复用（Bind 不 Setup）、Unbind、信号 reload、RequestLayout 防风暴 |
| `example/gui_test/main.go` | ListView 用例改为 SetModel + SetDelegate |

## 7. 验证

- 单测：`SetScrollY` clamp、滚轮 DeltaY/Line 换算、thumb 几何、
  ListView 可见范围 `[first, last]`、滚动后 item 复用、双模式行为一致。
- 视觉：`example/gui_test` 50 个 label 滚轮滚动 + 滚动条 thumb；
  ListView 1000 个 item 滚动流畅（只布局可见项）。
- go test ./... 是验证要求；是否通过及环境见带日期记录，不由本节示意场景推断。

## 8. 不做的事

| 事项 | 不做原因 |
|---|---|
| 滚动条 `SetStartFunc` 条件启动 | 独立 `ScrollBar` 控件矩形就是轨道，hitTest 天然门禁，无需条件启动 |
| 双轴 2D 自由滚动（同时拖两 thumb） | 指针捕获单目标，一次只拖一根轴；收益低 |
| 键盘滚动（PageUp/PageDown/Home/End） | 需要焦点 + 按键处理，后续加 |
| item 回收池 | 已实现，不是待办；模型重装与 Delegate 更换有不同复用边界 |
| ui 层 ScrollView/ListView | 已实现通用挂载目标协调，不再列为未来能力 |
| 固定行高／增量模型／delegate 简化 | 未实现，按测量和真实需求评估，见未来路线 |
| 增量 diff（index 平移、Update 保留实例） | 信号无参全量 reload；后续 ListView 内部 diff 或带参信号 |
| 便捷 delegate 包装（仅 Setup/Bind） | 先保留完整 Setup/Bind/Unbind 生命周期；简单场景包装后续提供 |
