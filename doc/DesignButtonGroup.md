# ButtonGroup 与分组职责

状态：2026-10-30 已实现并完成本机独立 X11 验证；验证范围及缺口见 Plan。

## 三种职责

- gui.ButtonGroup 是零间距连排容器，默认水平、交叉轴拉伸，支持垂直；普通按钮的
  首尾外侧保留圆角，内部为直角，接缝只绘制一条边。它不改变选择行为。
- gui.MenuButtonGroup 改为局部菜单联动容器，默认水平排列，普通 Button 可混排；
  后代 MenuButton 自动属于最近的组，嵌套组是边界。菜单顺序取组内实际 Widget 树，
  不再接受显式 MenuButton.SetGroup，不扫描整个 Root。联动与连排外观独立。
- widgets.CheckButtonGroup 保留不参与布局的互斥对象，支持跨布局区域及显式对象引用。
  widgets/ui.CheckButtonGroup(child) 只建立作用域，不生成包装 Widget；后代默认使用
  最近的兼容对象祖先，不需要 ID 或重复 Group 字符串。

## 声明式接口

```go
ui.ButtonGroup(ui.Button("Back"), ui.Button("Forward"))
ui.MenuButtonGroup(
    ui.MenuButton("File").Menu(...),
    ui.MenuButton("Edit").Menu(...),
    ui.Button("Run"),
)
wui.CheckButtonGroup(ui.VBox(
    wui.CheckButton("Light"),
    wui.CheckButton("Dark"),
))
```

CheckButton.Group 接收 *widgets.CheckButtonGroup；省略表示自动查找最近作用域，
显式对象覆盖默认关联，显式 nil 保持独立。显式对象由调用方稳定持有；普通
重建保留 Widget／作用域对象，卸载解除关联，移交重新解析默认关联。

复用现有 ObjectView；FindAncestor[T](ctx) 增加按兼容对象类型查询最近祖先的形式。
既有 FindAncestor[T](ctx, id) 保留精确 ID 及遮蔽语义，不改 Root／BuildContext。
MenuButtonGroup 与 ButtonGroup 使用普通 WidgetView／ui.Container 协调，不用对象声明。
所有 API 按新契约迁移，不为组名保留字符串兼容分支。

## 布局、绘制与语义

ButtonGroup 复用 LinearLayout，接缝由可见直接子项的顺序与方向确定。子按钮仍用
自己的 StyleName／StyleScope／状态，父容器不覆写 StyleName 或隐式提供配色。
连排的圆角掩码与共享边只属于绘制几何；普通按钮路径保持原状。
Button、MenuButton 和按钮式 CheckButton 显式调用 GUI 按钮绘制辅助函数。
不扩展 Painter、不修改三后端，不添加圆角裁剪或绘制后回调。默认首尾圆角使用
各子按钮自己解析的 radius；背景与边框使用现有 Path 能力，焦点边框完整绘制。
隐藏项不占接缝序位，单项保留完整轮廓；容器无独立点击／焦点动作，子项语义保留。
MenuButtonGroup 使用可配置间距的水平线性布局；ButtonGroup 的连接间距固定为零。

### 接缝端点（2026-11-04）

水平排列由后一项绘制共享左边，前一项省略右边；垂直排列对应后一项的上边与
前一项省略的下边。省略边的内外轮廓保持同一坐标，只对实际拥有的边收缩内轮廓，
使 C 形环带垂直封口。若对四边一律收缩，内外端点会斜接，形成三角缺口。
背景在不透明边框下的收缩与环带共用这一几何规则，不改动各子项的分配矩形。
布局、共享边归属及子项自身样式在 Hover／Pressed 时保持不变。

像素回归覆盖横向／纵向、1／1.25／1.5／2x、1／2／4 DIP边宽、统一／独立圆角和
不透明／半透明预乘颜色。接缝位置对齐物理像素，断言两端直边完整覆盖，容差仅为
两级8位量化，不以放宽 alpha 条件容忍缺口。

分数物理坐标上两个独立 Path 的 AA 覆盖再按 Over 合成，仍可能产生细线；这可用
两个普通相邻矩形路径独立复现，不属于上述端点斜接。当前不承诺独立 Path 的
亚像素拼接等同于单次并集填充；通用处理另行设计，见 [Roadmap](Roadmap.md)。

### 自定义按钮焦点部件

`focus` 是按钮自己的独立绘制部件，不参与接缝归属。自定义 StyleName 应完整定义
该部件：Normal 显式使用透明背景和零边宽，FocusVisible 再定义所需边框。
否则 Part 会继承同名默认样式的背景和边宽，鼠标点击产生逻辑焦点时就会重画
完整按钮，表现为共享边或按下配色发生变化。Basic／Modern 内置名称已有相应规则。
保留 Part 的默认值合并、显式 Focused 覆盖和既有焦点状态机，不在核心特殊处理例程。

`tests/window/gui/corner_radius` 使用2 DIP白色键盘焦点边框；鼠标 Hover／Pressed
只改变背景。自动模式经 Window.DispatchEvent 验证各项按压／释放、移出取消、
Tab／Shift+Tab目标、FocusVisible和Snapshot，并对接缝及完整焦点尾边做像素断言。

## 实现入口

- `gui.NewButtonGroup()`、`SetDirection(layout.Direction)`、`AddChild`／`RemoveChild`／
  相对重排；`ui.ButtonGroup(children...).Direction(...)`。
- `gui.PaintButtonBox(p, widget, resolvedStyle)` 根据直接父 ButtonGroup 确定外露圆角，
  普通按钮直接复用既有绘制；`gui.PaintButtonFocus` 始终绘制完整焦点边。
  自定义按钮可显式调用，两者不主动改变焦点或解析子项样式。
- 菜单组复用 LinearBox 的布局字段，重写 AddChild 保证父对象是实际 MenuButtonGroup，
  并使用自己的默认样式名 `menu-button-group`。声明层仍是普通 Container 协调。
- ButtonGroup 的默认样式名为 `button-group`；两个容器默认不绘制独立装饰。
  Basic／Modern 原有按钮、expanded、focus 和按钮式 CheckButton 部件直接适用，
  无需额外主题匹配或父样式继承。自定义 StyleName 仍按子项自身解析。

## 生命周期与验证

保留已有 MenuButton 关闭／错误反馈、正常事件路径、原生 Popup 所有权、重入防护、
同 Root 限制及移动后重新查询关系。容器移除不销毁仍由调用方持有的控件。
不增加自动互斥、快捷键、动画或跨区域菜单组。

分批：先简化 CheckButton 声明；再迁移 MenuButtonGroup 为容器；最后实现连排外观
与 UI、Basic／Modern、demo 及独立窗口用例。各批运行相关包内测试后提交，最终
执行全仓测试、必要 race／交叉编译，并在独立 X11 的 Software／OpenGL 1x／2x
验证圆角、单条接缝、Hover／Pressed／FocusVisible、菜单切换与组隔离。
像素回归采样中段接缝及外侧圆角，明确预乘颜色及容差；不依赖 DevServer 验收。

参考：[libadwaita linked controls](https://gnome.pages.gitlab.gnome.org/libadwaita/doc/main/style-classes.html#linked-controls)
区分连续外观与交互；[GTK4 CheckButton grouping](https://docs.gtk.org/gtk4/method.CheckButton.set_group.html)
与 [Qt QButtonGroup](https://doc.qt.io/qt-6/qbuttongroup.html) 将互斥关系独立于布局。
采用职责划分，不引入 CSS 级联或 Flutter SegmentedButton 的独立选择模型。
