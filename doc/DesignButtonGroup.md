# ButtonGroup 与分组职责

状态：2026-10-30 已确认，分批实现与独立 X11 验证；实际进展见 Plan。

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
