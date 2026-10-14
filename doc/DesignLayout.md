# GOUI 布局与尺寸设计

> 2026-10-07 自然基线修正：所有 LinearLayout 孩子（不只 weighted）以主轴
> 无界约束取得自然尺寸，再给 weighted 最终分配。旧文中“只有加权主轴无界”
> 的描述由 [滚动布局切片](DesignScrollLayout.md) 取代；其他轴/权重契约不变。

> 自动换行容器已增加 `layout.WrapLayout`、`gui.WrapBox` 与 `ui.HWrap/VWrap`，
> 具体约束和 ScrollView 组合限制见 [WrapBox 设计](DesignWrapBox.md)。

> 状态:尺寸真相源、布局约束流向、以及**具体机制(§8–§14)**的设计。本文写思路 / 哲学 / 约定 + 关键类型形态,不写实现细节。
> 已落地：创建尺寸、Constraint／Measure／Arrange、基线、默认对齐、测量缓存和 WrapBox。§14 是已完成切片的背景；后续缺口见 [未来路线](Roadmap.md#future-layout)。
> 默认值升级（2026-09-07）：`CrossDefault`、Button/MenuButton 内容居中及 GUI
> 布局参数归一化已实现，详见 §16。字体/主题度量和二维布局仍为后续工作。
> 相关:`doc/DesignPopup.md`、`doc/HiDPI.md`(逻辑/物理坐标)、`doc/DesignConventions.md`(facts-vs-policy)。

---

## 1. 核心:尺寸真相源取决于 surface 的角色

尺寸从哪来、往哪流,不是一个全局选择,而是看这是哪种 surface:

| surface | 尺寸真相源 | 流向 | 例子 |
|---|---|---|---|
| **顶层 Window** | 平台 / 合成器 / 用户 | 尺寸**向下**流成约束,Widget 填充它 | 应用主窗口 |
| **Popup** | 内容(Widget) | 尺寸**向上**流,Widget 撑开它 | 菜单、下拉、tooltip |

- **Window = 外生尺寸(extrinsic)**:窗口约束 Widget。
- **Popup = 内生尺寸(intrinsic)**:Widget 撑开 Popup。

"到底是 Widget 撑开窗口,还是窗口限制 Widget?"——**顶层窗口是后者,Popup 是前者**。两者流向相反,但用同一套约束机制统一(见 §3)。

## 2. 为什么这么分:可移植性(Wayland / Android)

- **顶层 Window 的尺寸在很多平台上不是 App 能"命令"的**:Wayland 由 client 提议、compositor 用 `configure` 说了算;Android 的 Activity 直接铺满。因此**顶层窗口不设命令式 `SetSize`**——尺寸永远是**事实**,经 `SizeEvent` 报回,gui 在该尺寸下布局。让内容去撑开 OS 顶层窗口既不可移植,WM/合成器也不答应。
- **Popup 的尺寸在所有平台都是 client 定的**:Wayland `xdg_popup` 用 `xdg_positioner` 带尺寸、Android `PopupWindow` 有显式宽高、x11/win32/cocoa 更是。所以 **Popup 创建传尺寸、`SetSize` 更新尺寸,是可移植的**。

## 3. 统一机制:Measure 接收"约束",返回"期望尺寸"

两种相反流向用一套 measure/arrange 表达(Flutter BoxConstraints / Android MeasureSpec / WPF availableSize 那套):

- **约束向下、尺寸向上、父定位子**;子返回的尺寸**必须**落在父给的约束内。
- 区别只在**喂给根 Widget 的约束是紧还是松**:

| 传入约束 | 布局行为 | 是否回写 surface |
|---|---|---|
| **Window:紧约束(max=窗口尺寸)** | 在窗口尺寸下布局,Arrange 铺满窗口 | 不回写(也回写不了) |
| **Popup:松 / 无界约束** | Measure 返回内容期望尺寸 | 回写成 popup 尺寸 |

gui 现在的 `root.Measure(size)` 本质就是"紧约束/窗口驱动"那条路;要支持 Popup,需把 `Measure` 从"吃确定 size"改为"**吃约束、吐期望 size**"。一套布局代码,两种配置。

## 4. Widget 的 min / max 尺寸

**允许 Widget 声明 min / max(及 preferred)尺寸**,它们是 Widget 的固有属性(按钮不能小于文字、侧栏有最小宽度)。关键约定:

> **父的硬约束永远赢**:Widget 自身的 min/max 只是"自我偏好",会被传入约束**夹住(clamp)**;**子的 min 永不突破父的 max。**

于是同一个"Widget min"在两种 surface 下走向不同——由**约束的松紧**决定,不是由 min 本身决定:

- **顶层窗口(紧约束)**:min 被夹到窗口尺寸,放不下就**溢出**(默认裁剪;要滚动就套 `ScrollView`,由它把紧约束在滚动方向重新放松成无界再喂给子)。**App 从不主动改窗口尺寸** → Wayland/Android 安全。
- **Popup(松约束)**:min 参与内生度量,真的把 popup 撑开。

因此"允许 min ⟹ 必须让 Widget 撑开顶层窗口"这个担心**不成立**:顶层窗口里 min 是被夹住 + 溢出,不是撑窗口。

## 5. "内容 min → 窗口"的正当通道:窗口最小尺寸 hint

内容的 min 仍可**合法地影响窗口**,但不是命令式 resize,而是把根 Widget 的 min 作为**窗口最小尺寸 hint** 报给 WM/合成器:

- X11 `WM_NORMAL_HINTS.min_size`、win32 `WM_GETMINMAXINFO`、cocoa `contentMinSize`、Wayland `xdg_toplevel.set_min_size`。
- 语义 = **"别让用户把我缩到比可用还小"**;桌面 + Wayland 原生支持,Android 忽略(本就固定/铺满)。
- 这是**咨询性下限**,不是内容任意撑开窗口;它让"内容 min"触达窗口,却不破坏"窗口尺寸是平台事实"这条线。

DesktopWindow.SetMinSize 已实现；GUI 显式最小尺寸优先，否则从根测量派生提示。换行内容的自然尺寸不等于固有最小尺寸，此限制需后续单独评估。

## 6. 创建尺寸的 API 约定(本轮落地)

- **`NewWindow(size, handler, options)`**：size 为 geometry.Size，是客户区 DIP 的**建议尺寸(hint)**——桌面尽力满足、移动端忽略;权威尺寸永远来自 `SizeEvent`。取代各后端原先的魔法默认(800×600 / 300×200 / OS 默认)。
- **`NewPopup(owner, size, handler, options)`**：size 是内容度量后的请求 DIP 尺寸，原生量化后的实际尺寸仍以 SizeEvent 为准。保留 `SetPosition`/`SetSize` 作为**更新**入口(动态内容如子菜单展开)。
- **单位一律逻辑(DIP)**,与 `SetSize` 同契约;各后端按需换算:cocoa 的 NSWindow content 本就是 points(逻辑),直接传;x11 ×GTK scale;win32 ×DPI(popup 用 owner 的 DPI,window 用系统 DPI 作估计)。

## 7. 与 OpenGL 修复的闭环

Popup 内生流程天然是:**度量内容 → 得尺寸 → 按此尺寸创建/SetSize → 再绘制**,正好是"先定尺寸、再建 painter/绘制"的顺序,天然避开 GLX back buffer "滞后一帧"的坑(见 `DesignPopup.md`)。当前 GLX 集成已不采用 priming-swap；原生 drawable／尺寸同步归平台与 OpenGL 后端，不以多交换一帧作为安全网。

## 8. `Constraint{Min,Max}`:约束的具体形态

```go
type Constraint struct { Min, Max geometry.Size }   // 约束向下流;Size = {Width, Height float32}

func Tight(s Size) Constraint       // Min==Max:必须正好这么大(Window 根、Fill 子)
func Loose(max Size) Constraint     // Min=0:随你多大,不超 Max(Popup、内容驱动)
func (c Constraint) Clamp(s Size) Size   // 把期望尺寸夹进 [Min,Max]
```
**契约(唯一一条)**:`child.Measure(c)` 返回的 `s` 必须逐轴 `c.Min ≤ s ≤ c.Max`;子自己负责夹取。

能表达:Tight / Loose / Unbounded(某轴 Max=∞)/ 逐轴混合。**Window 外生=Tight,Popup 内生=Loose,同一套 measure 代码**——这就是 §3 两种相反流向的统一。

`Constraint{}` 是 `Min=Max=0` 的严格零尺寸约束；无约束测量应使用 `Unbounded()`。

**为何选 `{Min,Max}`(而非"只传 available"或 Android MeasureSpec)**:它表达力最强——能表达"下限非 0 的区间" + tight/loose/unbounded/逐轴混合,把外生/内生统一;久经验证(Flutter)。**取舍**:4 个数;**无独立 intrinsic 查询**（内容尺寸复用 Measure；TableView 已通过稳定列几何和行委托实现，不代表增加了通用 intrinsic API）;无界 Max 用**大哨兵值**(封进 helper,别用裸 `Inf`);`Min>Max` 归一化(Max 赢)。

## 9. Widget 尺寸偏好 = 它自身的一个 Constraint(min/max,不是 Width/Height)

**布局里没有"固定大小"这回事。** `.Width(100)` 读作"钉死 100",但最终会被父约束夹,名不副实。所以暴露 **min/max**,不暴露 Width/Height。

- Widget 存一个 `sizeConstraint Constraint`(默认 `{Min:0, Max:∞}` = 无偏好);它和父传下来的约束**同一个类型**。
- Measure 的夹取顺序:
  ```
  final = 父约束.Clamp( self约束.Clamp( 固有尺寸 ) )
  ```
  **父最后夹 → 父永远赢**:你 `MinWidth(200)` 但父只给 100 → 压到 100 → 溢出/裁剪(§4),而不是撑大父。"子 min 不破父 max"免费落地,且一眼可解释。
- API:`MinWidth/MaxWidth/MinHeight/MaxHeight`(+ `MinSize/MaxSize`)。真固定 = min==max **显式写两边**(逼你意识到"仍是偏好、父能覆盖");"要 N 像素空间"用**有固有尺寸的控件**(`Spacer(n)` / `Image`),不给普通控件钉尺寸。

对于委托 LayoutManager 的 Widget，先将自身尺寸偏好与父约束合并，再交给
manager 测量子项。合并上下界仍采用“self 先夹、parent 最后夹”的规则；这样
MinSize 造成的额外空间会参与居中和 baseline 计算，MaxSize 也能提前限制换行
宽度。不能只在 manager 返回之后夹取容器尺寸，否则 Measure 的 baseline 与
Arrange 的最终位置会不一致。

### 9.1 控件固有骨架下限(≠ 用户 min/max)

**固有尺寸(intrinsic)**和**用户 min/max**是两层,别混:

- **固有尺寸**是控件 `Measure` 按内容+样式算出的自然尺寸,是控件自己的职责。**空内容不该退化到 0**:交互控件(如 `Button`)在固有尺寸上保留一个**从字体派生的骨架下限**(`floor = textLineHeight(fontSize) + padding*2`,一个行高的方块),这样空按钮/图标按钮仍是可见可点的方块,而不是塌成一条线或消失。骨架从字号派生 → 随 DPI/字号自适应,**不是硬编码魔法像素、也不是给每个控件挂一个 minSize 常量**。
- **用户 min/max**(§9 的 `SetMin/MaxSize`)是在固有尺寸**之上**的显式覆盖。夹取顺序不变:`父约束.Clamp( self约束.Clamp( max(固有, 骨架) ) )` —— 骨架属于"固有"这一层,`constrain` 最后跑,所以用户 `SetMaxSize` 仍能压过骨架、`SetMinSize` 仍能抬高。**不要用 `SetMinSize` 去补固有尺寸的塌陷**——那是补错了层。
- 只有真正会"塌成 0"的交互控件才需要骨架:`Button` 需要;`TextInput` 自带 `defaultTextInputWidth`+行高骨架、`Image` 空图=`{0,0}` 合理、`Label` 纯展示空=0 宽合理,**都不加**。骨架是控件特有逻辑,不是共享常量。
- 相关:`Button` 构造器给内容盒一个真实的 `Padding(6)`(原为 0,导致有文字也贴边)；
  它当前是控件的布局默认，不是 style 值。若日后要支持分轴 padding 或主题度量，
  再扩展对应的控件/主题 API。

## 10. 布局属性归属(与 style 的分工)

| 属性 | 性质 | 放哪 |
|---|---|---|
| 颜色 / 边框 / 圆角 / 字体 | 可解析、可主题化的视觉属性 | **style**(一套 Resolve) |
| **width / height / min / max**(尺寸约束) | app 特定、非视觉、不主题化 | **WidgetBase 字段 + ViewBase 修饰器** |
| **padding / spacing / direction / align** | 控件内容盒或容器**如何排布子节点**的结构参数 | **具体控件或容器视图字段** |

**padding 的实现落点(已落地)**:padding 是**布局标量**(不是 style),且**只属于有内容盒的 widget**,不进 `WidgetBase`(不是所有 widget 都有 padding)。落点跟着"谁负责布局":

- **`LinearLayout`** 自持 `Padding`，和 `Spacing` 同属容器结构参数；manager 在
  `Measure`/`Arrange` 内部 inset。padding=0 时行为不变。
- **`LinearBox` 保持薄包装**:`SetPadding` 只转发 `b.layout.Padding = p`(和 `SetSpacing` 一样),**不覆盖** `Measure`/`Arrange`;`WidgetBase` 委托给 manager,padding 自动生效。
- **`Button`** 因为需要同时处理内容 inset 和骨架下限，自持 `padding`，并在其
  `Measure`/`Arrange` 中显式应用。
- **`TextInput`** 自绘、**无 LayoutManager**，同样自持 `padding` 字段(文字
  origin/高度)。
- **`FillLayout` 和 `WidgetBase` 无 padding**；`Label`/`Image` 自然也没有 padding。
- **`layout` 包不读 style**(padding 是纯几何标量,这条分层不破)。

声明式设法:`.Padding(n)` 是**布局修饰器**,只在容器视图(`BoxView`/`ButtonView`/`TextInputView`)上,叶子(`Label`/`Image`)没有;和 `.MinWidth()` 同为覆盖语义。margin(盒外纯间距)若做则进 `layout` 的 Child 契约,**不进 style**(与 padding 分界见 §9)。

## 11. 各布局的约束派生(布局 = 约束变换 + 定位规则)

- **FillLayout**(叠放/填充):子给 `Loose(c.Max)`；自身 =
  `clamp(max(各子), c)`；arrange 时所有子填满同一个最终矩形。需要保留自然尺寸并
  对齐单个子时使用只有一个 child 的 `LinearLayout`，通过既有
  `MainAlign`/`CrossAlign` 组合表达，不在 FillLayout 内增加第二套模式。
- **LinearLayout**(竖为例,主轴=高、交叉轴=宽):
  - 交叉轴按容器 `CrossAlign`:`Stretch` → 给 Tight 交叉轴 `c.Max`;否则 Loose(0..c.Max)。
  - 主轴 → Loose 高(MVP 给 `Loose(剩余)`;flex/递减剩余后做)。
  - 自身:主轴 = `clamp(Σ子 + 间距, c)`,交叉 = `clamp(max子, c)`。
  - arrange:沿主轴按 `MainAlign` 摆,交叉轴按容器 `CrossAlign` 定位。**横向对调,一套代码。**
- **单子定位**：使用单子 LinearBox 的 `MainAlign`/`CrossAlign`。Start、Center、
  End 保留 child 自然尺寸；父给 tight 或显式 min 时才产生额外对齐空间，
  无界/普通 loose 时保持 hug。主轴扩张使用 child `MainWeight`，交叉轴填充使用
  `CrossStretch`。
- **ScrollView**(竖):子给 `{Min:0, Max:{c.Max.W, ∞}}`(**滚动轴无界**),量出完整内容高;自身保留 Child 宽度,垂直固有高度为 0,再由父级主轴分配或紧约束得到视口;arrange 把子排在 `(0,-offset)` 并裁剪。→ **无界是一等状态,ScrollView 几乎白得**,正是选 `{Min,Max}` 的回报。

## 12. 对齐与 MainWeight:相对主轴/交叉轴,别学 GTK 的绝对 h/v

学 SwiftUI 方向,不学 GTK。GTK 的 `halign/valign/hexpand/vexpand` 迷惑有两根因,GOUI 消解:

1. **相对 Main/Cross,不用绝对 h/v** → 消灭"VBox 里 halign 管哪根轴"的脑内映射。`CrossAlign` 值永远是"横跨盒子流动方向"的语义,H/V 盒同义。
2. **主轴的"多占空间"与交叉轴的"占据方式"是两码事,分开表达**(不像 GTK 一个 `expand` 布尔扛两轴)。

**旋钮(三个,全按 Main/Cross 词汇):**
```
MainAlign   主轴:一堆子整体怎么摆     Start / Center / End / SpaceBetween   容器级
CrossAlign  交叉轴:容器如何安排所有子项  Default / Start / Center / End / Stretch / Baseline  容器级
MainWeight  主轴:单个子分多少多余空间  float 权重(0=hug)                    子级
```
```go
type MainAlign int  ; const ( MainStart MainAlign = iota; MainCenter; MainEnd; MainSpaceBetween )
type CrossAlign int ; const ( CrossDefault CrossAlign = iota; CrossStart; CrossCenter; CrossEnd; CrossStretch; CrossBaseline )
```

**关键约定与取舍(逐条,来自评审):**

- **默认 `MainStart` + `CrossDefault`**:`CrossDefault` 根据方向解析，HBox 为
  `CrossCenter`，VBox 为 `CrossStart`。普通横向控件行默认垂直居中，纵向内容仍
  保持顺阅读方向靠左；需要文字基线、拉伸或明确靠边时分别显式使用
  `CrossBaseline`、`CrossStretch` 或 `CrossStart/End`。默认值不直接等于
  `CrossCenter`，以免方向切换后让 VBox 内容意外居中。
- **hug 默认、填充 opt-in**(内容驱动、可预测):普通控件默认 = 内容尺寸(Loose→hug);要铺满 → 容器显式 `CrossAlign=CrossStretch`。等宽表单靠容器 Stretch 一句话,不靠"默认 FILL 的暗魔法"。ScrollView 是特殊 viewport:有限父级高度直接作为其 viewport,不需要 Widget 级对齐属性。
- **交叉轴没有 weight,是结构使然**:主轴子**顺序排、竞争同一条长度** → 有剩余要按比例分 → `MainWeight`;普通交叉轴子**并排、各自独占整条交叉宽** → 不竞争、无剩余可分 → 只有容器 `CrossStretch`(拉满)+ 对齐 + min/max。ScrollView 的垂直 viewport 高度由自身的有限高度约束提供,在 VBox 中则由 `MainWeight` 分配。**顺序=竞争→权重;并排=各占满→拉伸。**
- **`MainWeight` 是 float 不是布尔**:要支持"1:2 分屏 / 等宽栅格"这类**按比例**分配;布尔 `Expand` 只能均分。名字用 `MainWeight`(不用 `Weight` 太泛、不用 `StretchWeight` 撞交叉轴 stretch、不用 `Grow` 撞 Go 缓冲区、不用 `Flex` 套 CSS)。
- **`CrossStretch` ≠ `MainSpaceBetween`**:前者=每个子各自拉满交叉宽(尺寸、单子);后者=子保持尺寸、剩余主轴均摊成子间间隙(定位、多子)。**不是一回事,别合并。**
- **`MainAlign`/`CrossAlign` 保持两个类型不合并**:虽共享 Start/Center/End,但 `Stretch`(仅交叉)、`SpaceBetween`(仅主轴)是轴特有;分成两类型 → `SetMainAlign(Stretch)`/`SetCrossAlign(SpaceBetween)` 这类无意义组合**编译不过**,换来类型安全 + 自文档,值。
- **`CrossStretch` 进 `CrossAlign` 是对的**(修正早期"align 里禁止一切 FILL"的过绝说法):它是交叉轴"如何占据"的单一旋钮,相对轴、清楚(同 Flutter `CrossAxisAlignment.stretch`、CSS `align-items:stretch`)。GTK 的病根是**绝对 h/v + expand 一布尔扛两轴**,不是"stretch 不能进对齐枚举"。

**LinearBox 给自身 children 提供布局策略**。`LinearLayout.CrossAlign` 的配置
默认值是 `CrossDefault`，由方向解析成有效值；父级 Widget 不需要携带
CrossAlign。一个 Widget 的外部矩形完全由父级 LinearLayout 的策略决定。
`MainAlign` 与 `CrossAlign` 都是 LinearBox 的容器属性。

**弹性容器(按轴测量 + 默认权重,2025 落地)**:容器型 widget(如 `ScrollView`)
包裹虚拟内容(`Scrollable`)时保留 Child 的固有宽度,有限父级高度作为 viewport,
高度无界时无固有 viewport 高度,并默认 `SetMainWeight(1)` 让 LinearBox 的剩余空间分配机制生效:

- `HBox(ScrollView, ScrollView)` → Child 宽度作为基线,`MainWeight(1)` 均分剩余主轴;
  有限父级高度成为 ScrollView 的 viewport。
- 若返回 `c.Max`,两个 child 各申报整个可用主轴,求和溢出后无收缩逻辑,
  后者会被排到容器外(曾致 `HBox` 双列表第二个不可见)。
- `VBox(ScrollView, ScrollView)` → Child 宽度作为交叉轴尺寸,`MainWeight(1)` 均分高度。
- `VBox(Label, ScrollView)` → Label 保自然尺寸,ScrollView 拿剩余主轴空间并保留 Child 宽度。
- Window 根(`Tight`) → `c.Min = c.Max` = 窗口尺寸,照常填满。

**职责边界**: `MainWeight` 只影响主轴剩余空间分配,`CrossAlign` 是 LinearBox
安排自身 children 的容器策略。LinearLayout 不识别具体的 ScrollView、ListView
类型;它只对 weighted child 使用主轴无界约束计算 flex basis,因此 ScrollView 在
HBox/VBox 中都不需要知道父容器方向。

## 13. host 喂什么约束 + measure/arrange 两相

- **Window 根**:`Tight(窗口逻辑尺寸)`;**Popup 根**:`Loose(上界)`(屏幕/owner 尺寸或大哨兵),取代现在的 `1<<14` hack。
- 两相：`Measure` 算出的子尺寸在 `Arrange` 定位时还会使用。GUI 框架在
  `WidgetBase` 缓存最近一个 `(Constraint, Measurement)`，通过内部 child
  adapter 复用相同约束的结果；Arrange 本身仍始终执行。

## 14. 实现路线(小步、可验证)

1. ~~**Constraint 化 Measure**(地基):`Constraint` 类型 + `Measure(Constraint)` 全线改 + host 喂紧/松 + popover 去 `1<<14`。**行为不变**,测试跟改。~~ **已完成。**
2. ~~**Widget min/max + ViewBase 尺寸修饰器**:`sizeConstraint` 字段 + `.MinWidth/...`;夹取顺序(§9)。~~ **已完成。** `WidgetBase` 加 `minWidth/minHeight/maxWidth/maxHeight`(0 max = 无上限)+ `selfConstraint()`/`constrain()`(`c.Clamp(self.Clamp(intrinsic))`,父约束最后夹取=胜)+ 6 个 setter 上到 `Widget` 接口;四个叶子(Label/Button/Image/TextInput)在固有尺寸上套 `constrain`。ui:`viewBase` 尺寸字段 + `apply()` 下发 + `ViewBase[T].MinWidth/MaxWidth/MinHeight/MaxHeight/MinSize/MaxSize`。
3. ~~**对齐**:LinearLayout 的 `MainAlign` / `CrossAlign`(§12)。~~ **已完成。** `LinearLayout` 保留容器级 `MainAlign`/`CrossAlign`，按相对主/交叉轴(`mainSize`/`crossSize`/`makeSize`)进行 Measure 与 Arrange。`CrossStart/Center/End` 保持 child 固有交叉尺寸，`CrossStretch` 在有限交叉轴上传递紧约束并填满交叉轴；`MainWeight` child 先以主轴无界约束计算 flex basis,再瓜分主轴余量。`WidgetBase`/`Widget` 不携带 CrossAlign，`LinearBox`/`BoxView` 提供容器级对齐属性。
4. **已完成：溢出 + ScrollView**：默认结构裁剪，Direction 选择放松的滚动轴；普通内容与虚拟内容分别处理。
5. **已完成：窗口 min-size hint**：显式客户区最小尺寸或根测量派生提示 → DesktopWindow.SetMinSize。自然尺寸与固有最小尺寸的区别仍待评估。

---

**一句话**:顶层 Window 外生(平台约束 Widget、无 `SetSize`、尺寸经 `SizeEvent`、创建尺寸=hint);Popup 内生(Widget 撑开、创建尺寸=权威 + `SetSize`);二者用"Measure 吃约束、吐期望尺寸"统一;Widget min/max 由约束松紧决定是撑大 surface(Popup)还是夹住+溢出+给 WM 最小尺寸 hint(Window)。Wayland/Android 均不破。

## 15. 实用化升级：baseline、对齐与布局缓存

### 15.1 保留 LinearBox，不新增 FlexBox

`LinearBox` 的语义是“子项在一条主轴上按顺序排列”。支持
`MainWeight` 不会把它变成 CSS Flexbox：weight 只是线性盒对剩余主轴
空间的可选分配策略。

- `MainWeight == 0`：保持子项自然尺寸。
- `MainWeight > 0`：在自然尺寸基础上按权重分配额外空间；空间
  不足时，weighted child 是优先重新约束和测量的对象。
- 子项始终在最终分配到的约束下重新测量，使换行文本的高度和
  baseline 与最终宽度一致。
- 多行流式排列将来使用独立 `WrapBox`；二维轨道使用 `Grid`。
  只有出现明确的 basis/grow/shrink/fit 需求时才重新评估 `FlexBox`。

### 15.2 Measure 同时返回尺寸和 baseline

baseline 是与本次约束度量绑定的几何结果，不是 Widget 上一个脱离
Measure 的可变属性：

```go
type Measurement struct {
    Size        geometry.Size
    Baseline    float32 // 相对顶边的第一条排版基线
    HasBaseline bool
}

type Child interface {
    Measure(Constraint) Measurement
    Arrange(geometry.Rectangle)
    MainWeight() float32
}
```

- `Label` 报告首行 `TextLine.Baseline`，空文本无 baseline。
- `TextInput` 报告内边距加稳定文字样本的 baseline。
- `Button` / `MenuButton` / 单子布局把 child baseline 加上实际布局偏移后上传，
  包括内边距和内容居中产生的纵向偏移。
- `Image` / `ScrollView` / `ListView` 等没有文本行语义的控件不报告 baseline。
- 容器可以把第一个有效子 baseline 转换到自己的坐标系后上传。

不增加依赖“上次 Measure”隐藏状态的 `Baseline()` 方法；这样可避免约束
变化后读到过期基线，也避免再次调用平台文字排版。

### 15.3 LinearBox baseline 对齐

`CrossAlign` 增加 `CrossBaseline`：

- 仅水平 LinearBox 使用；垂直 LinearBox 按 `CrossStart` 处理。
- 公共 baseline 是所有有效 child baseline 的最大值。
- 自然高度由最大 ascent 与最大 descent 之和决定。
- 无 baseline 的 child 靠上对齐，与 Flutter Row 的行为一致。
- 容器比自然高度更高时，额外空间默认留在下方；暂不引入 GTK 的
  BaselinePosition。

### 15.4 独立控件对齐复用 LinearBox

LinearBox 的 `MainAlign` / `CrossAlign` 既能安排一组 child，也能安排单个
child。单个控件需要在较大槽位中定位时，使用单子 LinearBox，不新增
`AlignLayout` / `AlignBox`，也不引入第三套 Start/Center/End 枚举：

```go
// 水平居中、垂直靠下；HBox 的主轴为水平，交叉轴为垂直。
ui.HBox(child).
    MainAlign(layout.MainCenter).
    CrossAlign(layout.CrossEnd)
```

- LinearBox 必须先从父约束、MinSize 或 MainWeight 获得大于 child 自然尺寸的
  槽位，对齐才会产生可见偏移。
- 主轴拉伸使用 child `MainWeight`，交叉轴拉伸使用 `CrossStretch`；双轴填充也可
  直接使用 FillLayout。
- Button 等复合控件的默认内容位置属于控件自身语义，不靠公共 AlignBox 包装。
- Grid、Overlay 等未来布局若需要逐项定位，由对应容器管理明确的 parent-data；
  不提前给所有 Widget 增加 `HAlign` / `VAlign`。

这样布局系统始终只有 Main/Cross 相对轴模型：`MainCenter` 和 `CrossCenter` 的
使用位置由容器方向明确，不再出现无法判断属于哪根轴的 `AlignCenter`。

### 15.5 测量缓存与失效

- 缓存属于 `gui.WidgetBase`，不属于纯算法 `layout`。
- 每个 Widget 先只缓存最近一个 `(Constraint, Measurement)`，不引入 map/LRU。
- `RequestLayout` 使当前 Widget 和祖先的测量失效；全局字体/样式变化
  使整个子树失效。
- framework 通过内部 child adapter 统一进入缓存，不给 `WidgetBase`
  增加 owner/self 回指。
- 首版仍执行 Arrange，仅去掉重复的昂贵 Measure；不提前做布局子树跳过。

### 15.6 实施顺序

1. `Measurement` 升级与全链路迁移。
2. LinearLayout 的最终约束重测、空间不足策略和 `CrossBaseline`。
3. ~~独立控件对齐。~~ **已完成：复用单子 LinearBox，不新增第二套对齐类型。**
4. 单条测量缓存、失效传播和计数型性能测试。
5. 再根据真实需求增加 `Insets`、`SpaceAround` / `SpaceEvenly`、
   `WrapBox` 和 `Grid`。

## 16. 默认参数审视

> 状态：前三项已实现（2026-09-07）：`CrossDefault`、Button/MenuButton 内容
> 居中、GUI setter 参数归一化。主题度量和 Grid/FormLayout 保留为后续工作。

### 16.1 默认值的分层原则

默认值不应在所有层使用同一套“看起来方便”的数值，而应由抽象所处层级决定：

1. **`layout` 算法保持中性。** 默认不产生额外间距、内边距或扩张，输入相同就
   得到稳定、可推导的几何结果。
2. **`gui` 复合控件提供符合控件语义的默认行为。** 例如 Button 内容默认应
   居中，ScrollView 默认应形成可用的视口。
3. **`ui` 的“未设置”必须保留 `gui` 默认。** 声明式层只协调用户明确给出的值，
   不能用字段零值无条件覆盖命令式控件构造器的语义默认。
4. **主题值与结构值分开。** 通用布局的 spacing/padding 保持 0；控件视觉间距、
   推荐尺寸等以后可由 style/theme 提供，而不是写进底层布局算法。

由此，零值的目标不是让所有控件都呈现同一种外观，而是做到安全、确定，并允许
上层区分“未设置”和“显式设置为某个值”。

### 16.2 当前默认值结论

| 项目 | 当前行为 | 结论 | 后续方向 |
|---|---|---|---|
| `Constraint{}` | `Min=Max=0`，严格零尺寸 | 保留 | 注释中明确它不是 Unbounded |
| Widget MinSize | 0，无固有下限覆盖 | 保留 | 交互控件的骨架下限仍由控件 Measure 负责 |
| Widget MaxSize | 0 表示用户未设置上限 | 保留 | 注释说明其语义不同于 `Constraint.Max=0` |
| Linear direction | 零值为 Horizontal | 可保留 | 正常入口仍应使用带 direction 的构造器 |
| `MainAlign` | `MainStart` | 保留 | Center/End/SpaceBetween 显式选择 |
| `CrossAlign` | `CrossDefault`：HBox Center、VBox Start | 已调整 | 显式 Start/Center/Baseline 等不随方向改写配置值 |
| Linear spacing | 0 | 保留 | 由调用方或未来主题组合显式提供 |
| LinearBox padding | 0 | 保留 | 结构容器不隐式制造留白 |
| 普通 Widget `MainWeight` | 0，即 hug | 保留 | 扩张必须显式选择 |
| 单子定位 | 单子 LinearBox，显式 Main/Cross | 保留 | 不增加绝对水平/垂直对齐类型 |
| FillLayout | 子项填满内容区域 | 保留 | 这是该布局本身的完整语义 |
| ScrollView `MainWeight` | 构造时显式设为 1 | 有意保留的例外 | 文档化，并允许显式设回 0 |
| Label | 不换行、文字 Begin 对齐 | 保留 | 换行会改变 height-for-width，应显式选择 |
| TextInput | 建议宽度 160 DIP | 过渡性保留 | 后续改为字体或主题度量 |
| Button/MenuButton 内容布局 | 内部 LinearLayout，MainCenter/CrossCenter | 已调整 | baseline 包含居中偏移；需要填满时显式设置 FillLayout |

`Constraint{}` 与 Widget `MaxSize=0` 看似不一致，但它们属于不同层次：前者是
父级给出的实际可用区间，零就是没有空间；后者是控件的可选尺寸偏好，零表示用户
没有声明上限。保留这种差异比为所有尺寸字段引入 Optional 更简洁，但必须通过
API 注释消除歧义。

### 16.3 `CrossDefault` 的精确定义

`CrossDefault` 是配置值，不是 `CrossCenter` 的别名：

```text
Horizontal + CrossDefault -> CrossCenter
Vertical   + CrossDefault -> CrossStart
```

- HBox 的常见用途是按钮、标签、输入框组成的一行，默认垂直居中比靠上更符合
  视觉直觉。
- VBox 中的子项通常宽度不同，默认靠左（主书写方向的 Start）比水平居中稳定。
- `CrossBaseline` 仍然显式使用。它依赖子项是否存在 baseline，也会受多行文字
  首行基线影响，不适合作为所有 HBox 的隐式默认。
- 显式 `CrossStart` 始终表示 Start；容器改变方向时，只有仍配置为
  `CrossDefault` 的实例重新解析有效值。
- getter 应返回配置值，以保证 setter/getter 往返一致；布局内部使用解析后的
  effective value。

声明式 `BoxView` 必须把零值解释为 `CrossDefault`，不能通过无条件下发
`CrossStart` 覆盖 `gui.NewLinearBox` 的默认行为。未来若其他 View 包装了具有非零
语义默认的 Widget，也应使用“是否显式设置”状态或可表达默认态的枚举。

### 16.4 特殊控件默认

**ScrollView。** 普通 Widget 继续以 `MainWeight=0` 为默认；ScrollView 显式使用
1 是视口语义带来的例外。它避免滚动区域在有限 LinearBox 中收缩到接近零尺寸，
代价是多个 ScrollView 会自动分配剩余空间、在内容驱动 Popup 中可能需要显式
MinSize。当前收益更大，因此保留，并保证命令式和声明式 API 都能显式设回 0。

**Button / MenuButton。** 默认 manager 使用水平 `LinearLayout`，显式配置
`MainCenter` 和 `CrossCenter`，不增加 Widget 包装层或公共对齐类型。
`Button` 先把字号派生的骨架下限夹入用户尺寸偏好与父约束的有效范围，再去掉
padding，把完整的内容约束传给 manager；MenuButton 复用相同流程，保留原有
内容驱动的固有尺寸。最终 baseline 由 manager 的布局结果加上实际内边距偏移
得到，即使按钮被分配的高度小于两倍 padding，也与 Arrange 使用的矩形一致。

需要内容填满按钮时，可以显式调用 `SetLayoutManager(layout.NewFillLayout())`。
默认 manager 下 child 的 `MainWeight` 仍可用于主轴空间分配；复杂内容保持通过
单个子容器组合的方式提供。

**TextInput。** 160 DIP 能给松约束下的输入框提供可用宽度，短期删除会导致控件
过度收缩；但固定 DIP 不随字体、字号、语言和主题变化。后续应按一定字符容量的
advance 计算，或由主题提供 preferred width，同时保留用户 min/max 的最终覆盖权。

### 16.5 参数合法性

GUI 的公开 setter 对负数、NaN 和正负 Infinity 统一归一到 0，适用于：

- Widget 的 `MainWeight`、MinSize/MaxSize 各分量；
- LinearBox 的 spacing 和 padding；
- Button、TextInput 的 padding。

归一化发生在新旧值比较之前，getter 返回归一后的值；重复传入同一个异常值
不会反复触发布局失效。MaxSize 的 0 仍表示清除对应轴的上限。`ui` 通过 GUI
setter 应用相同规则，未提供 padding/weight 修饰器时继续恢复控件构造默认值。

底层 `layout` 是普通值模型；直接构造 Constraint、设置 LinearLayout 字段或
实现 `layout.Child` 的调用方应提供符合各字段契约的几何数据。归一化没有增加
跨层依赖或第二套公共配置 API。重叠布局使用明确的 Stack/Transform 语义。

约束内部仍遵循“`Min > Max` 时 Max 赢”和“父约束最后夹取”的既有规则。

### 16.6 实施优先级

1. **已完成**：增加 `CrossDefault`，同步 `layout`、`gui`、`ui` 与方向切换测试。
2. **已完成**：Button/MenuButton 内容居中，提前合并布局约束并保证 baseline
   与最终位置一致；保留显式 FillLayout。
3. **已完成**：在 GUI setter 边界归一化 weight、spacing、padding、尺寸偏好
   中的负数和非有限数值。
4. **待评估**：TextInput 建议宽度与字体容量度量；Padding 继续是控件／布局属性，不因主题成熟自动迁入样式。
5. **需求触发**：Grid／FormLayout 的跨行列对齐；不改变 LinearBox 默认值或提前增加完整 FlexBox。见 [未来路线](Roadmap.md#future-layout)。

验证覆盖 HBox/VBox 默认值、方向切换与配置值回读、声明式修饰器撤销、Button
骨架/MinSize/MaxSize/父级 tight 下的内容位置与 baseline、按钮内换行内容、显式
FillLayout、异常参数的归一化及失效次数，以及 ScrollView 默认权重的关闭与恢复。
