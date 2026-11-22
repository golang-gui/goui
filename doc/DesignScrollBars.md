# ScrollView 内外滚动条与布局配置

> 状态：2026-11-19 首批 A 已交付 ScrollBarOptions、TableView 转发和边框适配。
> 2026-11-21 补充外部 GUI 绑定、PageSize 与 TableView；2026-11-22 声明式入口
> 收敛为 Target／ScrollBar，GUI 协议命名为 ScrollClient／BindScroll。
> 完整契约、实施进度与验证范围见 Plan。

## 1. 目标与边界

ScrollView 继续内置横、纵滚动条，并拥有内容范围、视口、偏移、钳制、滚轮、
Reveal 和虚拟化协调。内置滚动条保持既有默认尺寸，可逐轴配置厚度、留白或隐藏。
隐藏内置条不禁止滚动、不要求创建外部条，也不把视口强制撑成完整内容尺寸。

外部滚动条只关联控制，不由 ScrollView 挂载、布局、改写样式或销毁。关联协议
不依赖 Widget，标准 ScrollBar、自定义控件及 Slider 适配器都可接入。

保持现有 Scrollable、ScrollIntoViewRequester、ListView 与 UI 协调边界；不引入
共享 ScrollModel、ScrollAxis、Binder 控件体系或新的样式尺寸字段。普通的内容收紧
继续通过 Measure、父约束和布局表达，不增加“控件填满但条追随末列／末行”的定位模式。
TableView 维持当前有界表体填满的策略，不通过隐藏条暗中修改测量契约。

默认保持既有外观：8 DIP 轨道、8 DIP 滑块、0 DIP 间距，滑块铺满轨道横轴。
默认配置不增加占位或两侧留白；只有应用显式修改 Options 时才改变这些几何。

## 2. 命名与公共配置

统一使用 H/V 方向前缀，与 HBox、VBox 等既有命名保持一致：

- GUI：HScrollBarOptions / SetHScrollBarOptions、VScrollBarOptions / SetVScrollBarOptions。
- UI：.HScrollBarOptions(...)、.VScrollBarOptions(...)。
- 两轴共用一个 ScrollBarOptions 值类型；不分别定义 H/V 配置类型。
- 不另设 HorizontalScrollBarOptions / VerticalScrollBarOptions 长名称别名。

首批已实现的 API：

```go
type ScrollBarPolicy uint8

const (
    ScrollBarAuto ScrollBarPolicy = iota
    ScrollBarAlways
    ScrollBarHidden
)

type ScrollBarOptions struct {
    Policy         ScrollBarPolicy
    Thickness      float32
    ThumbThickness float32
    Spacing        float32
}

func (sv *ScrollView) HScrollBarOptions() ScrollBarOptions
func (sv *ScrollView) SetHScrollBarOptions(ScrollBarOptions)
func (sv *ScrollView) VScrollBarOptions() ScrollBarOptions
func (sv *ScrollView) SetVScrollBarOptions(ScrollBarOptions)

func (v *TableView) HScrollBarOptions() gui.ScrollBarOptions
func (v *TableView) SetHScrollBarOptions(gui.ScrollBarOptions)
func (v *TableView) VScrollBarOptions() gui.ScrollBarOptions
func (v *TableView) SetVScrollBarOptions(gui.ScrollBarOptions)
```

TableView 提供上述四个方法，薄转发到内部 ScrollView，不要求应用访问
控件子树、保存内部 ScrollView 或自行扣减滚动条宽度。

### 2.1 尺寸与间隔

所有值使用 DIP，方向改变只改变横轴的解释：纵条对应宽度，横条对应高度。

| 字段 | 含义 | 默认值 |
| --- | --- | --- |
| Thickness | 条的期望横轴厚度，也是内置条的布局占位基础 | 8 DIP |
| ThumbThickness | 滑块绘制厚度，在实际条宽中居中 | 8 DIP |
| Spacing | 内容视口与条之间额外的布局间距 | 0 DIP |

默认没有滑块两侧留白。应用可显式设置较大的 Thickness 或较小的 ThumbThickness
让滑块在轨道内部居中，也可设置 Spacing 将轨道与内容分开。二者同时设置时留白相加，
例如 Thickness=14、ThumbThickness=8、Spacing=2 时，内容侧可见留白为 5 DIP。
轨道背景只画在条的矩形内，Spacing 区域保留容器背景，不虚构数据或轨道。

归一化在 setter 比较前完成：Thickness、ThumbThickness 非正或非有限时取默认值；
Spacing 负值或非有限时取 0；非法 Policy 归一化为 Auto。
零值 ScrollBarOptions 因而表示默认配置，Spacing=0 是有效值。
getter 返回存储的归一化配置，不返回父布局钳制后的实际矩形。
滑块绘制厚度最终钳制到实际条厚，极窄分配不能越界。

这是布局参数，不加入 style.Style。颜色、边框、圆角继续由 trough/thumb/corner Part 决定，
Basic 与 Modern 使用相同默认几何。悬停、按下只改变既有外观，不改变视口占位。

### 2.2 策略

| 策略 | 内置条 | 占位 | 滚动功能 |
| --- | --- | --- | --- |
| Auto | 对应轴存在滚动范围时显示 | 显示时计入厚度与间距 | 正常工作 |
| Always | 始终显示 | 始终计入厚度与间距 | 无范围时完整滑块，不接受无效滚动 |
| Hidden | 隐藏 | 厚度与间距均不占位 | 滚轮、键盘、Reveal、程序及外部控制继续工作 |

Direction 决定允许滚动的轴；策略仅控制允许轴的内置条。Direction 禁止的轴不分配条，
即使该轴配置 Always。Hidden 不等于禁止滚动，也不等于以完整内容尺寸撑开控件。
隐藏后视口可扩大，范围必须按扩大后的视口重算，而不是冻结旧范围。

## 3. ScrollBar 与 ScrollView 的布局

标准 ScrollBar 增加 Thickness / SetThickness、ThumbThickness / SetThumbThickness
普通属性。Thickness 提供期望横轴尺寸，仍遵守 Widget MinSize/MaxSize 及父约束；
ScrollView 内置条由对应 Options 配置。整个条宽保留点击及拖动能力，槽内留白不
缩小操作区域。拖动区域与悬停反馈对同一有效滑块带保持一致。

ScrollView 不再在测量、占位、Arrange 等路径分别使用固定 8 DIP：

1. 先扣除既有稳定 frame 安全边距，保留无界约束。
2. Auto 从无条开始，Always 从占位开始，Hidden 不参与占位；联合计算两轴。
3. 每个实际存在的槽位占用对应厚度与 Spacing；按最终视口测量或布局内容。
4. 保留普通内容重测、虚拟内容范围更新及现有收敛检查。
5. 最终提交视口、条与交汇角几何，再同步滚动条状态。

不能用当前隐藏 Widget 的零测量代替其潜在槽位厚度，也不能从旧条的显隐状态
启动计算。极小尺寸先钳制条厚，再钳制可用间距，确保所有矩形非负且在内区内。
Spacing 不进入内容范围、PageSize、Reveal 或命中区域；正文继续由结构视口裁剪。

切换策略隐藏条时，复用正常可见性与手势取消；正在拖动的条改变几何时，
使用既有取消机制结束旧拖动，不能沿用失效的抓取位置。

### 3.1 供组合控件读取的实际几何

首批即提供生产布局所需的只读结果：

```go
type ScrollGeometry struct {
    Viewport   geometry.Rectangle
    HScrollBar geometry.Rectangle
    VScrollBar geometry.Rectangle
    Corner     geometry.Rectangle
}

func (sv *ScrollView) ScrollGeometry() ScrollGeometry
```

字段全部为 ScrollView 局部 DIP 坐标，表示最近一次完成的 Arrange。
未分配的条、无双条的交汇角均为空矩形。Corner 为两条轨道的实际交汇矩形，
不包括额外 Spacing 色带。Measure 不覆盖已提交几何，getter 不触发同步布局。

TableView 在布局中继续使用 LayoutVisible 得到的实际视口；绘制读取已提交结果。
不能从 MaxScrollX/Y、Scrollable() 或某个固定尺寸反推槽位是否存在。

通用交汇角由ScrollView在自己的Paint中按有效样式名的 `corner` Part绘制一次，
支持背景、边框及圆角，与frame/条/Spacing的区域分离。Basic／Modern的普通
ScrollView与text-view组合默认透明；TableView主题显式提供与trough一致的方形底色，
应用可分别定制。角区没有Widget或语义动作，样式不改变布局、命中或滚动范围。
只有正面积才提交绘制，避免退化矩形的抗锯齿残线。

只覆盖trough不再改变交汇角。自定义完整样式名遵循既有Part默认值规则；需要留白时
显式为corner设置透明背景、BorderWidth=0及Radius=0，需要连续底色时单独指定背景。

## 4. TableView：选项暴露与边框适配

Options 是当前应用缺口，首批必须同时覆盖 TableView 和 widgets/ui，不能只交付
普通 ScrollView。表格继续复用内置 ScrollView 管理滚动，不复制其状态机。

| 装饰 | 职责 | Options 引入后的依据 |
| --- | --- | --- |
| outer-horizontal/vertical | 完整 TableView 的整体外框 | 保持完整分配区域及独立线带 |
| grid-horizontal/vertical | 实际单元格之间的网格 | 保持单元格边界，不伸入间距、槽位及空白 |
| grid-top/bottom/left/right | 实际数据四边 | 随内容滚动裁剪，不包含外部控制器 |
| outline-horizontal/vertical | 可见数据及实际内置槽位的紧包围包络 | 根据 ScrollGeometry 的实际槽位计算 |
| header-separator 及表头补位 | 固定表头与所属包围框的衔接 | 根据实际视口与槽位计算，不根据滚动范围猜测 |
| 交汇角 | 内置双条的通用背景 | 改由 ScrollView 绘制，移除 TableView 重复绘制 |

### 4.1 紧包围与空白区域

- 无内置槽位时，outline 包围可见的真实列宽、表头与已分配行内容高度；
  即使有滚动范围而条为 Hidden，也不能按“有范围就有槽”扩张轮廓。
- 有任意实际内置槽位时，沿现有规则包围数据与固定槽位的矩形包络。
  Always 即使无范围，也因存在真实槽位而计入包络。
- 外部滚动条与关联控制器不计入包络，也不使表格增加表头补位或角区。
- 短表需要横向内置条时，条仍固定在视口底部；中间空白不绘网格或斑马色。
- 无可见列时不生成数据紧包围轮廓；保留已有空表和极小分配行为。

整体外框与 outline 继续预留独立线带，颜色透明不释放线宽；横纵边的交点继续
只合成一次，不改变半透明边框契约。没有新增外框模式或“追随末列／末行”属性。

### 4.2 表头、列宽和输入

表头与正文按同一个实际内容视口分配列宽，横向偏移保持同步。显式更改 Options
引起的占位变化按正常视口分配处理；列 Width 基础值不改写，不发出用户 OnResize。
保留既有手动调整上下文：只改滑块厚度不重分配列，槽位改变继续在右侧调整空间，
不能让条的自动显隐或配置变化覆盖已确认的列边缘。

paintHeaderScrollGap 保留在 TableView，因为固定表头不属于 ScrollView 内容。
它改用实际几何：数据边线止于单元格；对应包围框存在时才延续分隔线；只补未绘制
区域，不覆盖真实表头状态色。隐藏纵条后不虚构槽位，关闭线宽后不提交空矩形。

表头、正文、滚动槽与 Spacing 的命中边界必须一致；点击条或间距不得排序、选择或
打开记录菜单。Snapshot 保持真实单元格边界，不为槽位、补位、角区伪造业务行列。

## 5. 首批声明式 API

ui 提供 ScrollBarOptions / ScrollBarPolicy 类型和策略常量的薄别名，
ScrollViewView 与 widgets/ui.TableViewView 都提供 HScrollBarOptions、VScrollBarOptions。
每次协调只向既有 Widget setter 应用值；省略时恢复默认，不重建内部条或复制状态机。

```go
ui.ScrollView(content).
    VScrollBarOptions(ui.ScrollBarOptions{
        Thickness:      14,
        ThumbThickness: 8,
        Spacing:        2,
    })

wui.TableView(model, columns...).
    VScrollBarOptions(ui.ScrollBarOptions{
        Thickness:      14,
        ThumbThickness: 8,
        Spacing:        2,
    }).
    HScrollBarOptions(ui.ScrollBarOptions{
        Policy: ui.ScrollBarHidden,
    })
```

应用可以先定制条宽、滑块与间距，或隐藏默认条并使用已有程序滚动入口；
不必等待后续外部绑定功能。TableView 仍只暴露必要转发，不公开完整内部 ScrollView。

## 6. 外部控制协议

本节为外部 GUI 绑定契约，第 7 节为声明式 Target 契约。

```go
type ScrollClient interface {
    SetValue(float32)
    SetMax(float32)
    SetPageSize(float32)
    ConnectChange(func(float32)) signal.Handle
}

func (sv *ScrollView) BindScroll(
    axis layout.Direction,
    client ScrollClient,
) signal.Handle
```

ScrollClient 表示单轴滚动控制端，不包含 Widget；普通 Go 对象也可实现。
接口及 BindScroll、私有连接实现集中在 gui/scroll_view.go，与宿主状态和生命周期一起维护；
ScrollBar 的实现断言放在 gui/scroll_bar.go。现有 *ScrollBar 保持为具体控件，只增加必要的 PageSize
输入；TableView 薄转发 BindScroll。合法方向为 Horizontal/Vertical，
非法方向作为 API 误用触发 panic；nil 客户端或已销毁宿主不建立连接，返回 nil。

### 6.1 状态与请求

绑定时立即同步 Max、PageSize、最终 Value；三个 setter 不回发位置请求。
ConnectChange 表达用户位置请求，统一进入 ScrollView 的有限值检查、取整、
钳制与正常内容更新路径，再回传实际位置。非有限请求不进入布局。

范围或视口改变，即使位置不变也同步；Direction、SetChild(nil)、Reveal、
布局后钳制及程序 SetScrollX/Y 均覆盖。内置条与外部客户端使用同一同步路径。
支持多个客户端连接同一轴；一个客户端只关联一个宿主轴，调用者避免重复建立相同关联。
所有操作在 GUI 线程进行，非 Widget 客户端不意味着允许跨线程调用。

标准 ScrollBar 的比例使用 PageSize / (Max + PageSize)，再按自身轨道长度计算
滑块与可移动距离，保留最小长度和精确端点。PageSize 是实际视口长度，不能用
外部条的长度代替。独立使用 ScrollBar 的调用者需显式提供页面大小。

### 6.2 生命周期与重入

返回句柄只解除关联，不移动、隐藏或销毁客户端，也不改写其方向、尺寸或样式。
调用者管理使用周期，宿主销毁兜底断开全部连接。Disconnect 幂等；Block/Unblock
沿 signal.Handle 语义处理，恢复连接时补发当前完整状态。

同步基于已提交的几何和连接快照；每次外部调用后检查宿主及连接版本。客户端
可以解绑、重入或销毁宿主，旧同步不能继续覆盖更新后的状态，不靠清空全部信号
代替生命周期保护。标准 Widget 客户端卸载时的连接释放由其关联适配器负责。失去滚动范围会通过
ScrollBar 自身的手势取消路径结束旧拖动，不临时改写 Visible 或触发隐藏／显示。

## 7. 声明式 Target 与外部滚动条

ScrollBar 通过 Target(id) 关联同一 UI Root 的有效 Widget 输出；不需要 ScrollGroup，
ScrollView、TableView 和第三方宿主也不登记组成员。通用协调契约见
[Target、Group 与 Scope](DesignUIRelations.md)。GUI 保留 BindScroll 直接对象入口。

```go
ui.HBox(
    ui.ScrollView(content).ID("content").
        VScrollBarOptions(ui.ScrollBarOptions{Policy: ui.ScrollBarHidden}),
    ui.ScrollBar(layout.DirectionVertical, "content").Thickness(14).ThumbThickness(8),
).Spacing(6)
```

一个宿主允许多个方向和多个外部条，同一 Root 内多个具名宿主独立关联。
目标暂时不存在时清除范围、页面和偏移；被引用 ID 重复或不满足 ScrollHost 时报告
声明配置错误。隐藏宿主仍是存活目标，显隐不是卸载。

BindTarget 在整树、局部输出替换和延迟构建完成后提交；即使外部条没有 Update，
目标的删除、恢复或替换也会刷新连接。普通 AfterUpdate 运行时关联已完成。
相同宿主和轴保留连接；目标或轴改变先断开旧连接、取消旧手势，再连接新目标。
引用随显式移交进入新 Root；目标和条一起迁移且连接身份不变时不重连。

ui.ScrollBar 暴露 Target、Orientation、Thickness、ThumbThickness 及普通布局、样式
修饰器。空 Target 不关联，独立数值条仍使用 GUI ScrollBar。绑定不修改宿主内置
Options，不改变布局、颜色、可见性和 Widget 所有权。外部条在自身语义位置报告
Range 与方向，不进入 TableView 的内容包络，输入继续正常派发。

ScrollHost 是 BindScroll 能力接口；Target 支持按接口解析，第三方控件无需
向 Root 或 BuildContext 增加类型特例。不再暴露 ScrollGroupBinding。

## 8. 分阶段实施与验收

### A. 优先交付 Options 与 TableView 定制

1. gui/scroll_bar.go、gui/scroll_view.go：尺寸、滑块、三种策略、间距、统一占位、
   ScrollGeometry 与通用角区。无外部协议也必须能完整使用本批。
2. widgets/table_view.go、table_layout.go、table_paint.go：H/V Options 转发，
   实际槽位驱动包络／表头补位，移除重复角区绘制，保留列分配和所有框线能力。
3. ui/scroll_view.go、widgets/ui/table_view.go：H/V Options 薄封装，重建恢复默认。
4. 扩展既有滚动布局、表格边框和表格契约窗口用例；同步更新相关设计和进度。

A 的验收至少覆盖：默认 8/8、两轴不同尺寸、正／零间距、非法值归一化、
Auto/Always/Hidden、允许轴变化、双条相互占位、极小分配、末端完整可见、
手动列调整与列收缩、表头／正文对齐、全部框线组合及 UI 默认恢复。
不能以只完成 ScrollView、缺 TableView 或声明式入口的结果交付 A。

### B. 外部 GUI 协议

增加 ScrollClient、PageSize、BindScroll 和 TableView 转发。验证非 Widget
客户端、不同轨道长度、内外条同时控制、无请求回声、范围变化、解绑、重入和销毁。
验证客户端存在与否均不影响 Hidden 策略下的滚轮、Reveal 和程序滚动。

### C. 外部标准条的声明式入口

ui.ScrollBar.Target 使用通用 BindTarget；宿主无需登记成员。
验证目标切换／缺失／局部替换、Root 隔离、同轴多条、重名、移交与释放。
窗口入口 tests/window/ui/scroll_target 同时覆盖 TableView、ScrollView 与 TabBar。

### 8.1 测试组织与交付

优先扩展 gui/scroll_bar_test.go、scroll_view_test.go、scroll_frame_test.go、
widgets 的现有表格布局／边框／滚动样式测试及 ui/scroll_view_test.go、
widgets/ui/table_view_test.go。共享层使用公共契约与必要替身，不引入具体原生后端。
几何按独立预期断言；保留原 8 DIP 默认占位及显式自定义配置的覆盖。

绘制归属调整后，像素回归定位到实际绘制节点或整树绘制，保留独立颜色与区域预期，
不能因 paintScrollCorner 移动而删除角区检查。像素测试覆盖 1x/分数缩放/2x、
半透明交点、裁剪及零线宽；注明预乘 sRGB、采样位置和有依据的误差。

每批完成后运行 go test ./...，按风险追加相关 race、本机代表性 Software/OpenGL
窗口及受影响目标交叉编译。仅修改这些共享契约不自动追加其他平台桌面验收；
编译、跳过和原生运行结果分别记录。窗口程序使用正常输入，不依赖 DevServer。
首批 A 的实际测试、窗口操作及跨平台验证边界见 Plan，不将 B/C 记为已交付。

提交按可独立审查的层次拆分，相关回归与文档随实现提交。A 可按 GUI、widgets、
UI 与窗口用例组织，不机械增加“记录验证”提交；B/C 按 GUI、TableView、声明式
和窗口用例拆分，回归及对应设计随实现提交。

## 9. 参考与取舍

- GTK4 的 [PolicyType](https://docs.gtk.org/gtk4/enum.PolicyType.html) 将 EXTERNAL
  与 NEVER 区分；[ScrolledWindow](https://docs.gtk.org/gtk4/class.ScrolledWindow.html)
  文档允许用外部 Scrollbar 与普通布局组合。采用隐藏默认条但保留受约束滚动的契约，
  不复制 Adjustment/GObject 属性系统。
- GTK 4.20.0 [条布局源码](https://github.com/GNOME/gtk/blob/4.20.0/gtk/gtkscrolledwindow.c#L3177)
  按分配的内容区域与条测量结果安排位置；采用按真实槽位布局，不按末列／末行定位。
- Qt 6.8.3 [滚动区域布局](https://github.com/qt/qtbase/blob/v6.8.3/src/widgets/widgets/qabstractscrollarea.cpp#L295)
  将 frame、viewport 和条的几何集中处理，表格内容范围用于滚动范围。
  采用职责分离，不引入 QScrollBar 继承或 style hint 布局分支。
- Flutter 3.38.1 [ScrollbarPainter](https://github.com/flutter/flutter/blob/3.38.1/packages/flutter/lib/src/widgets/scrollbar.dart#L572)
  在自身绘制区域边缘定位滑块，区分 thickness 与 crossAxisMargin；
  [shrinkWrap](https://api.flutter.dev/flutter/widgets/ScrollView/shrinkWrap.html)
  是滚动方向的尺寸策略且有布局成本。本方案不以虚拟内容全量测量获得紧包围效果。

以上是公共文档与固定版本源码依据，位于控件层，不代表各主题、平台或后端的像素
完全一致，也不表示 GOUI 已执行这些框架的运行对照。具体默认值是本库的布局选择。
