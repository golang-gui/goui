# ListRowGroup 连接分组

状态：2026-11-17 实现；2026-11-18 增加可选统一自然行高。验证范围记录在 [Plan](Plan.md)。

## 契约与 API

`widgets.ListRowGroup` 是把**直接**包含的 ListRow 上下连成一体的普通 Widget。
它只改变布局与装饰：每行仍解析自己的样式、状态、焦点和激活回调，组不增加选择、
动作或新的业务状态。分组不是列表模型，也不接管 ListView 的虚拟化。

```go
group := widgets.NewListRowGroup()
group.AddChild(updatesRow)  // 直接子行才会相连
group.AddChild(actionRow)
group.AddChild(versionRow)
group.SetUniformRows(true) // optional; false by default
```

对应声明式接口：

```go
// import wui "github.com/golang-gui/goui/widgets/ui"
wui.ListRowGroup(
    wui.ListRow("Automatic updates").Suffix(wui.Switch()),
    wui.ListRow("Check for updates").Activatable(true).OnActivate(check),
    wui.ListRow("Version").Suffix(ui.Label("0.8.0")),
).Children(more...).UniformRows(true)
```

`UniformRows() bool` / `SetUniformRows(bool)` 控制统一行高，默认 false。
其余布局方向固定纵向，间距固定为 0，交叉轴默认拉伸，无内边距。
它继承 `WidgetBase` 的普通容器能力（`AddChild`、`RemoveChild`、`MoveChildBefore`），
由分组布局复用 `LinearLayout` 完成测量与排布。组本身不可聚焦、无点击动作，
`Snapshot.Role=vbox`。

## 布局与连接几何

子行上下紧贴、左右同宽，默认行高仍由各行内容与 MinSize 决定。
隐藏的子行不占位，也不保留接缝：隐藏中间行后，其上下两行立即贴合。

### 可选统一自然行高

开启 UniformRows 后，只对当前可见的直接子 `*ListRow` 执行以下规则：

- 按交叉轴拉伸后的实际可用宽度、无界高度测量每行，取最高的自然高度，统一分配。
  文字换行、插槽和各行尺寸偏好均参与自然测量；不使用单行的 MainWeight 分配高度。
- 同组的直接行即使被非行内容隔开，仍共享这个高度；连接边框继续遵循相邻行规则。
  非 ListRow 子控件与嵌套分组不参与最大值计算，保留自身布局与 MainWeight。
  嵌套组可以独立开启 UniformRows，不把开关传播给后代。
- 不均分组的额外高度，不将统一值写回行的 MinSize、MaxSize、MainWeight 或布局管理器。
  行的尺寸偏好约束自然测量，最终统一分配仍由父布局决定。组高度不足时，行保留统一
  自然高度并沿用既有溢出／裁剪规则；组自身 Measure 结果仍遵守父约束。
- 每次 Measure／Arrange 都按当次宽度建立局部测量计划，不缓存共同高度或成员列表。
  内容、样式、可见性、增删及宽度变化复用已有 RequestLayout 和测量失效路径。
  Arrange 使用最终宽度重算，即使此前测量采用另一宽度，也不会沿用旧折行高度。
- 关闭后恢复普通 LinearLayout 行为：各行独立自然高度，原有 MainWeight 重新生效。
  无界宽度不被改成很大的有限值；存在有限最小宽度时按 CrossStretch 的既有规则测量。

实现集中在 `widgets/list_row_group_layout.go`：只在开启时将直接行的测量适配为共同高度、
权重 0，再交给原 LinearLayout。保留 GUI 提供的 layout.Child 适配器，因此样式通知与
测量缓存仍走现有内核，不直接调用子 Widget 绕过缓存，不增加通用布局或 GUI 特例。

### 连接几何

连接关系在**每次绘制**时由实际的可见直接子控件推导，与 `ButtonGroup` 相同：

- 相邻的两个可见 ListRow 共享一条水平边；
- 共享边归**后一行**所有，前一行省略自己的底边；
- 只有首个可见行保留上圆角，末个可见行保留下圆角，中间行四角为直角；
- 只连接直接相邻的行。出现非 ListRow 的可见子控件时链条在此截断，两侧的行
  各自保留完整轮廓，而不是省掉一条没人补画的边；
- 单行分组（或只有一行可见）不参与连接，外观与独立行完全一致。

不保存成员注册表，因此隐藏、重排、移除、`SetVisible` 之后都不需要失效或重算
索引；`MoveChildBefore` 保持 Widget 身份，绘制期扫描自然跟随新顺序。

半径与边框都来自行自己解析出的样式：分组只把不该由该行拥有的圆角置 0，并把
边框边集减去 `BorderBottom`。未声明 `BorderSides` 与显式 `BorderAll` 等价，
与样式层其他位置一致。

键盘焦点框是**独立且完整**的一圈：它重复绘制共享边（否则轮廓会开口），但保留
与行盒子相同的暴露外角，不会把相邻行占用的直角重新变成圆角。焦点框不改变布局、
不改变其他行，也不复用行表面的圆角掩码。

`ListRow.Paint` 仍是唯一样式解析入口：先按行自身状态解析 `PartDefault`，再按
同样的几何绘制可选的 `focus` part。分组不新增 Painter 能力、不改三个后端、不做
圆角裁剪，也不需要绘制后回调。

## 主题与外观

没有 `list-row-group` 的默认主题规则：连接外观完全由各行自己的
`list-row` 表面与边框组成。Basic 的 1 DIP 灰色边框、2 DIP 圆角与 Modern 的
1 DIP 中性边框、8 DIP 圆角都直接生效，分组不叠加背景，否则会画在行下面并
露出第二条轮廓。应用仍可显式给组设置 StyleName 并注册规则（例如组外框），
这与「父控件不隐式提供子样式」的既有约定一致。

Demo 设置区改为一个分组，行间距由原来的 8 DIP 变为共享接缝；区块标题与说明
仍由应用组合，不新增设置页容器。

## 输入与生命周期

分组不接管输入：悬停、按下、点击、Tab 与键盘激活仍由每行自己的
Motion/Click/Key 控制器处理，命中测试与事件传播不经过组。悬停只改变命中的
那一行，不通过组扩散到邻居；点击子控件仍由子控件赢得 gesture 竞争。

组不持有成员引用，也不在 `AddChild`/`RemoveChild` 时连接信号，因此没有需要
断开的绑定。行的销毁、Root 归属与 `Destroyed()` 判定沿用既有语义；组被销毁时
由 Window 的兜底流程销毁子树。隐藏行不参与连接，但仍是组的子控件，卸载/重挂
后顺序与身份不变。

## UI 与语义

`widgets/ui.ListRowGroup` 是普通 WidgetView：`Mount` 创建
`widgets.NewListRowGroup()`，`Update` 协调 UniformRows（省略恢复 false），并用既有
`BuildContext.UpdateChildren`
协调子声明列表，`Unmount` 不改变已挂载的行。子声明按 ID 或位置保持身份，
重排只调整顺序而不重建行；`nil` 声明是省略项，不产生空行；空列表清空托管行。

分组不新增对象作用域，也不参与 CheckButtonGroup 那样的互斥语义；行内的
Switch/DropDown 等后代保留自己的状态与动作。

Snapshot：组的 `Role=vbox`、无 `Actions`、`Focusable=false`；`Children` 是各
行的真实快照（`Role=listitem`、自身文本、前缀/尾部的角色与窗口局部边界、可
激活时的一个 click 动作）。分组不伪造 selected/checked，也不合并行的文本。
统一行高只改变实际分配：绘制、命中与快照继续读取同一个 Rect，不额外扩大交互区域。

## 参考与验证

- [libadwaita 1.6.0 `_lists.scss` 源码](https://github.com/GNOME/libadwaita/blob/1.6.0/src/stylesheet/widgets/_lists.scss)
  （查阅时库版本 1.10.0）：`list.boxed-list` 让 `> row` 共用一个卡片外观，
  只有 `:first-child` 保留上圆角、`:last-child` 保留下圆角并去掉下边框，相邻行
  之间是 `border-bottom` 单线。GOUI 采用同样的「后一项拥有共享边 + 只保留外侧
  圆角」几何，但不用 CSS 伪类、不建立列表样式体系，几何由行自身在 Paint 时
  推导，隐藏与重排无需重新选择器匹配。
- [GTK4 ListView `show-separators` 属性文档](https://docs.gtk.org/gtk4/property.ListView.show-separators.html)
  （查阅时库版本 4.24.1）：分隔线是列表视图的开关，默认关闭。GOUI 的分组是
  显式容器而非列表属性，虚拟化列表若需要分隔线应由应用选择，不在本轮引入。
- [Flutter 3.35.0 ListTile 文档](https://api.flutter.dev/flutter/material/ListTile-class.html)：
  `shape` 只描述单个 tile 的轮廓，相邻 tile 的连接由外层的
  `ListView.separated` 或 `Card` 组合完成。GOUI 同样把连接交给显式容器，
  不把分组语义塞进行本身。
- [GTK4 SizeGroup 文档](https://docs.gtk.org/gtk4/class.SizeGroup.html)（查阅时库版本 4.24.1）
  承诺协调尺寸请求，并明确说明它不控制最终分配、height-for-width 支持有限，建议在
  可能时使用容器专属机制；[Box homogeneous 属性](https://docs.gtk.org/gtk4/property.Box.homogeneous.html)
  描述容器将子控件等尺寸分配。GOUI 由分组同时负责按宽测量和最终行高，不另建跨树尺寸组。
- [Qt 6 QListView uniformItemSizes 文档](https://doc.qt.io/qt-6/qlistview.html#uniformItemSizes-prop)
  要求应用预先保证条目等尺寸，开关用于性能优化；这与本功能计算最高自然行高的契约不同，
  因此不采用首行代表全体或跳过其他行测量的假设。
- 上述分别是文档/源码参考与 GOUI 的取舍，不能代替本项目实测。

包内测试覆盖：无间距堆叠与宽度拉伸、隐藏行不占位与接缝重算、非行内容截断、
单行不连接、圆角/边集推导、焦点轮廓补齐共享边、独立行不受影响、组不扩散悬停
与点击归属；像素断言在 Basic/Modern 明暗之外另用显式探针样式（不透明白底、
2 DIP 半透明红边、8 DIP 圆角）在 1x/2x 检查共享接缝只被绘制一次（颜色不得比
外边框更深）、外侧四角透明、内侧接缝两端为直角且边框连续，预乘 sRGB RGBA8
允许 2 字节误差。`tests/window/widgets/list_row_group` 用正常桌面输入验收连接、
隐藏、重排、Tab 焦点和主题切换，不使用 DevServer。桌面缩放、原生后端与移动
触摸的验证边界见 Plan。

统一行高的包内回归另覆盖换行、隐藏最高行、内容与字号动态更新、增删重排、有限／无界宽度、
关闭恢复、非行与嵌套隔离、额外／不足高度、原始尺寸属性、加高区域命中和实际语义边界。
Linux Pango 测试不需要桌面，验证真实文字及样式失效；窗口入口加 `-uniform [-auto]`
验收声明式更新、真实排版和正常事件分发，默认连接／像素场景保留。
