# WrapBox 自动换行布局

## 定位与参考

WrapBox 与 LinearBox 并列，是普通、非虚拟化布局容器。按子树顺序排列，主轴
放不下时换行；垂直模式则换列。不实现选择、数据模型、排序、回填空隙、
固定列数、等宽网格、RTL、反向排列或 Flex 伸缩。保留现有 gui 包结构。

参考证据（2026-09-22 查阅，布局算法与平台绘制后端无关）：

- [Flutter Wrap API](https://api.flutter.dev/flutter/widgets/Wrap-class.html)
  区分行内 spacing/alignment 与行间 runSpacing/runAlignment。
- [Flutter 3.35.0 RenderWrap 源码](https://github.com/flutter/flutter/blob/3.35.0/packages/flutter/lib/src/rendering/wrap.dart)
  的 `_computeRuns` 顺序分行，再由 `_positionChildren` 定位。不照搬其无界
  交叉轴测量：GOUI 给子项完整内容区域的松约束，遵守自己的尺寸契约。
- [Qt 6 Flow Layout 示例](https://doc.qt.io/qt-6/qtwidgets-layouts-flowlayout-example.html)
  使用 `doLayout` 复用测量和排列计算，展示 height-for-width。
- [GTK4 FlowBox](https://docs.gtk.org/gtk4/class.FlowBox.html) 包含选择、过滤及排序，
  不适合作为本次纯布局容器的完整模型。
- [Avalonia WrapPanel](https://docs.avaloniaui.net/controls/layout/panels/wrappanel)
  支持横向/纵向流动；其统一 ItemWidth/ItemHeight 不纳入首版。

上述是参考框架文档与源码行为，不代表 GOUI 的测试结果。

## API 与默认值

算法为 `layout.WrapLayout`，构造器 `NewWrapLayout(direction)`，实现原有
`LayoutManager`；不依赖 Widget 或平台。

GUI 为 `gui.NewWrapBox(direction)`。提供 `AddChild`；移除、显隐、尺寸约束和
MainWeight 继承现有 Widget 能力。布局 getter/setter 成对提供：

| 属性 | 含义 | 默认 |
| --- | --- | --- |
| Direction | Horizontal 从左至右再向下；Vertical 从上至下再向右 | 构造参数 |
| Spacing | 行/列内子项间距 | 0 |
| LineSpacing | 行间/列间间距 | 0 |
| Padding | 四边等量内边距 | 0 |
| MainAlign | 每行沿主轴排列，包括最后一行 | MainStart |
| CrossAlign | 每行内部交叉轴对齐 | CrossDefault |

单位为 DIP。GUI 将负数、NaN、正负无穷的间距和 Padding 归零，先归一再判断
是否改变。直接使用 layout 值的调用者负责提供有限非负参数。
WrapBox 的 LayoutManager 只接受 `*layout.WrapLayout`，与 LinearBox 风格一致。

UI 为 `ui.HWrap(children...)` / `ui.VWrap(children...)`，返回 `*WrapView`。
通过 `Spacing`、`LineSpacing`、`Padding`、`MainAlign`、`CrossAlign`、`Children`
链式配置，其他能力复用 ViewBase。每次 Update 完整同步配置，省略的修饰器
恢复默认；复用既有子列表协调，不扩展 Root/BuildContext。

```go
ui.HWrap(
    ui.Label("Go"), ui.Label("Windows"), ui.Label("Linux"), ui.Label("macOS"),
).Spacing(8).LineSpacing(6).Padding(12)
```

## 测量与排列

1. 对父约束应用 Padding，为每个可见子项提供完整内容区域的松约束；不是
   当前行剩余空间。父硬约束优先于子项最小尺寸。
2. 当前行非空，且累计主轴长度 + Spacing + 子项长度超过有限主轴上限时换行。
   恰好放下不换行。主轴无界则始终单行。零尺寸可见子项仍占排列序号和间距，
   nil/隐藏子项不占位。
3. 普通行交叉尺寸取该行最大子项尺寸；Baseline 行取最大上伸 + 最大下伸，
   同时容纳无基线子项。总交叉尺寸是行尺寸之和 + 行间距。
4. 自然尺寸加回 Padding 后由父约束夹取。空容器仅贡献 Padding。松约束下
   内容收缩，紧约束服从分配。超出交叉轴的行不整体压缩，沿用 GUI 结构裁剪。
5. CrossStretch 分行后冻结成员和主轴尺寸，按该行交叉尺寸施加紧约束重新
   测量；不会反复重新分行。子项依赖尺寸的文本/布局状态与分配保持一致。
6. 根据最终尺寸逐行定位。Measure 与 Arrange 使用同一计算路径；Arrange
   按最终矩形重新计算，不能缓存并套用另一个宽度的分行结果。

时间复杂度 O(n)，临时空间 O(n)，不添加持久行缓存。复用 LinearLayout 的
私有轴映射、行对齐和基线几何帮助函数，不调用其权重分配算法、不改其行为。
Measurement 传播子树顺序中第一个有基线子项的最终基线位置。

### 对齐和权重

- MainStart/Center/End/SpaceBetween 作用于每一行；单项行的 SpaceBetween 为 Start。
- CrossDefault：水平行内 Center、垂直列内 Start，方向切换重新解释默认策略。
- CrossStart/Center/End 相对于所在行/列；CrossStretch 仅撑满该行/列。
- CrossBaseline 水平时逐行对齐，无基线子项靠行顶；垂直时退化为 Start。
- 多行整体固定从交叉轴起点排列，不提供 LineAlign。CrossCenter 不表示把所有
  行一起垂直居中；需要整体定位时用外层 LinearBox。
- 子项 MainWeight 被忽略但不修改；WrapBox 自身放在 LinearBox 中仍可带权重。

## 宿主约束与组合限制

### 窗口最小尺寸

当前 `window.updateMinSize` 在未显式设置窗口最小尺寸时，将根控件的无界
测量结果作为桌面最小尺寸 hint。WrapBox 的无界测量表示单行期望尺寸，
**不等于允许换行后的最小尺寸**；这可能导致窗口管理器阻止窗口缩窄。

本轮不改变通用窗口最小尺寸策略，也不让 WrapBox 伪造无界测量结果。
应用应显式声明合适的 `Window.MinSize`；测试例程为 400×600 DIP，初始
780×650 DIP，允许实际跨越换行阈值。未来单独设计最小尺寸与期望尺寸的
区别，不能仅以单行自然宽度作为支持换行内容的最小宽度。

### 滚动内容宽度

ScrollView 默认 `ScrollBoth`，普通内容按自然尺寸排列，此模式直接包裹 HWrap
不会按视口宽度换行。已实现的 `ScrollVertical` 使用有限视口宽度和无界高度，
适合 HWrap 随窗口宽度换行；`ScrollHorizontal` 对调两轴。例：

```go
ui.ScrollView(ui.HWrap(items...).Spacing(8).LineSpacing(8)).Direction(ui.ScrollVertical)
```

固定换行上限仍可显式设置 HWrap.MaxWidth；不要在 WrapBox 中判断父类型。
轴约束与滚动条出现后的重测契约见 [滚动布局](DesignScrollLayout.md)。

## 树、样式与语义

WrapBox 是薄包装，布局参与、样式失效、子树绘制、事件分发、焦点及生命周期
沿用 WidgetBase。布局参数不进入主题；父控件不向子控件隐式提供样式。
Snapshot 的角色为 `RoleWrapBox`（`wrapbox`），保留真实子树和窗口局部边界，
不创建行 Widget、不伪造 Grid/选择/点击动作。隐藏节点策略保持内核原样。

## 验证

- layout：方向对调、临界尺寸、空/零/超限子项、无界、间距、Padding、对齐、
  基线传播、Stretch 重测、实际宽度变化与权重忽略。
- gui：增删/显隐、快照、参数归一与失效去重、样式变化重新分行、带宽度限制
  的 ScrollView、WrapBox 在 LinearBox 中的自身权重。
- ui：协调身份、默认值恢复、Children 变更、归一化、布局与快照。
- 桌面例程：`go run ./tests/window/ui/wrap`。入口注释包含操作与预期。

具体执行结果和待验收项记入 Plan/Roadmap。
