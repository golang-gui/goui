# MenuButton 分组与菜单栏组合

> 状态：2026-10-30 迁移为局部容器；分组职责另见 [ButtonGroup 设计](DesignButtonGroup.md)。

## 目的与边界

MenuButtonGroup 是水平线性容器，可设置间距、内边距和对齐，也允许混排普通
Button。后代 MenuButton 自动归属最近的组，嵌套组形成边界；不需要组 ID、
显式绑定或成员注册。它不自动提供连排外观，该职责由 ButtonGroup 承担。

菜单按钮使用非模态 PopoverMenu，通过通用 AutoDismiss 请求外部按下、Esc
及窗口失焦关闭。正常 owner Motion 与键盘控制器完成联动；Window 不识别
MenuButton 或组，Root／BuildContext 也不增加具体控件分支。

本轮不包含子菜单、系统原生菜单栏或助记符。相关后续设计见
[子菜单](DesignMenuSubmenu.md)，通用能力见 [Popup](DesignPopup.md)。

## 公开接口

```go
func NewMenuButtonGroup() *MenuButtonGroup
func (*MenuButton) Group() *MenuButtonGroup
func (*MenuButton) Opened() bool
func (*MenuButton) Open() error
func (*MenuButton) Close()
func (*MenuButton) ConnectOpenError(func(error)) signal.Handle
```

既有 ConnectClosed 保留。同步 Open 返回错误，不同时发 OpenError；输入触发
的失败发一次信号。空菜单、不可见／禁用按钮为无操作。至少一个 Visible 且
非 Separator 项目即可打开；全部项目禁用仍可查看。

MenuButtonGroup 不保存成员注册表或弹窗引用；Group() 实时查询祖先关系。
活动菜单从所属窗口已显示的 Popover 中查询，并限制相同 Widget Root。
键盘顺序只遍历当前组子树，跳过嵌套组；不扫描整个窗口。
移动按钮或整组后自动采用新关系；卸载沿用 Popover 锚点生命周期关闭旧面。

## 声明式接入

```go
ui.MenuButtonGroup(
    ui.MenuButton("File").Menu(ui.MenuItem("Open", openFile)).OnOpenError(reportError),
    ui.MenuButton("Edit").Menu(ui.MenuItem("Undo", undo)),
    ui.Button("Run").OnClick(run),
).Spacing(4)
```

MenuButtonGroupView 是普通 WidgetView，复用 UpdateChildren 与 Container 协调。
重建保留 Widget、组和未变的菜单模型。动作通过节点状态访问最新声明闭包；
卸载清空动作引用并断开 Closed／OpenError 连接。单项或整组移交都由实际
GUI 父子关系确定成员身份，不使用 ObjectView 或 Reparented 分组适配。

## 鼠标行为

| 情况 | 行为 |
| --- | --- |
| 没有活动菜单时悬停 | 普通 Hover，不打开菜单。 |
| 点击有效按钮 | 打开菜单并保留按钮在 owner 中的逻辑焦点。 |
| 活动时真实移动到同组按钮或其 Label 子控件 | 正常命中及传播后切换菜单。 |
| 移到间隙、普通按钮或另一组 | 保持活动菜单，不执行普通按钮动作。 |
| 隐藏、祖先禁用、遮挡或空菜单 | 不参与切换。 |
| 点击展开按钮 | 关闭一次；锚点属于 AutoDismiss 内部区域，不在按下时预先关闭。 |
| 点击普通按钮或编辑区域 | 关闭后同一次事件继续给原目标，普通按钮执行一次。 |
| 关闭后继续悬停 | 保持关闭，等待下一次显式开启。 |

联动只响应普通 Motion 事件，布局与样式产生的合成 Hover 不开启菜单；不
伪造 MotionEventController 的物理 Hover。按钮记录最近的 owner 坐标，连续
重复的位置不覆盖键盘选择；真实离开后清除，同坐标原生 Crossing 保留位置，
重新进入或真正移动仍正常切换。
这处理原生系统在弹窗改变后重发相同移动消息的行为。命中沿用 EventDispatcher 的裁剪、
子树顺序和遮挡规则，不另做按钮矩形扫描或跨原生表面的鼠标事件重放。

## 键盘与关闭策略

MenuButton 始终使用非模态菜单；它保有窗口逻辑焦点，显式将上下、Home／End、
Enter／Space 转交 Popover 的正常键盘路径。左右键先交给菜单，未处理时沿
同组实际树顺序循环；只有一个有效成员时保持原菜单。Tab 关闭并继续原有
焦点导航，Esc 通过 AutoDismiss 关闭，其他键仍走正常窗口快捷键。

PopoverMenu 默认仍为模态，供右键菜单使用；增加 Modal／SetModal 以便其他
使用方显式选择呈现方式。菜单呈现器与 DropDown 明确启用 AutoDismiss，
Tooltip 保持自己的关闭策略。通用 Popover 的 AutoDismiss 默认 false，与
Modal 分别配置。非模态键盘分发没有自动目标选择，原生 IME 不因此新增。

## 生命周期、重入与反馈

切换记录源、候选、组、Root、修订及窗口显示代次；关闭旧菜单后重新核对。
Closed 回调销毁／迁移对象、更换组、重新打开源或显示其他面时，原切换停止。
真实关闭仍通知一次，不为切换吞掉 Closed。原生打开失败结束该次联动并
清理面、视觉状态和输入登记；OpenError 回调更新状态或销毁后，过期通知停止。

通用 AutoDismiss 只观察事件开始前存在的面。外部点击关闭后，若回调改变根、
原命中目标或显示其他面，停止原事件；不重新投递到新界面。旧面仅撤销自己
仍持有的模态目标，Hide 的焦点回调重新显示同一面时也不隐藏新呈现。

## 外观与语义

展开状态独立于 Hover。Basic／Modern 在按钮的 expanded 部件提供持续背景，
保留既有 FocusVisible。Modern 的普通、Primary 和 Flat 按钮均支持该部件；
普通 Button 的外观不因此改变。

非模态 Bottom 弹窗裁去锚点侧的阴影留白，避免透明原生输入面挡住按钮下半部；
翻到上方时裁去底部留白。主体位置和其余阴影保持，通用规则见 Popup 设计。

MenuButton 保留 RoleButton 与正常 Click 动作，增加 gui.MenuButtonInfoKey
下的 MenuButtonInfo：HasMenu、Opened、Grouped。组容器报告 RoleHBox 与实际子树，
不公开直接切换组的动作；打开的 Popup 继续沿既有 Window 快照／枚举呈现。

## 参考与验证

2026-10-26 核对官方契约：

- [Flutter MenuBar](https://api.flutter.dev/flutter/material/MenuBar-class.html)：
  点击开启后支持悬停、方向键和 Esc。采用交互，不照搬菜单栏对象体系。
- [RawMenuAnchor](https://api.flutter.dev/flutter/widgets/RawMenuAnchor-class.html)：
  外部关闭与点击消费分别表达。非模态 MenuButton 选择继续原点击。
- [Qt QMenuBar](https://doc.qt.io/qt-6/qmenubar.html)：提供水平菜单栏及原生集成；
  GOUI 当前采用普通控件组合，原生菜单栏仍在后续范围。
- [GTK Popover.autohide](https://docs.gtk.org/gtk4/property.Popover.autohide.html)：
  涉及外部关闭与焦点获取；GOUI 保留独立的 Modal 与关闭配置及显式键盘转发。

包内用例覆盖通用关闭策略、事件继续与失效、组切换和排序、可用性、隔离、
失败和回调重入、声明复用及最新动作。tests/window/gui/menu_group 通过正常
Window.DispatchEvent 自动验证完整窗口，并供 Computer Use 验证原生鼠标。
独立 X11、Windows／macOS 原生、Basic／Modern、亮暗与缩放结果分开记录；
交叉构建、自动派发与真实桌面输入不互相替代。
