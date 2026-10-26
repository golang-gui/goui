# MenuButton 分组与菜单栏组合

> 状态：2026-10-26 已实现。CheckButtonGroup 已单独完成命名迁移；本轮按
> 非模态菜单与通用 AutoDismiss 方案实现，最终命令、结果与缺口见 Plan。

## 目的与边界

多个 MenuButton 显式组成一组。点击打开菜单后，真实鼠标移到同组另一按钮
即切换菜单。应用通过 HBox、WrapBox 等普通布局组合菜单栏，也可混排普通
Button。组只表达关系，不拥有布局，不增加 MenuBar 容器。

菜单按钮使用非模态 PopoverMenu，通过通用 AutoDismiss 请求外部按下、Esc
及窗口失焦关闭。正常 owner Motion 与键盘控制器完成联动；Window 不识别
MenuButton 或组，Root／BuildContext 也不增加具体控件分支。

本轮不包含子菜单、系统原生菜单栏或助记符。相关后续设计见
[子菜单](DesignMenuSubmenu.md)，通用能力见 [Popup](DesignPopup.md)。

## 公开接口

```go
func NewMenuButtonGroup() *MenuButtonGroup
func (*MenuButton) Group() *MenuButtonGroup
func (*MenuButton) SetGroup(*MenuButtonGroup)
func (*MenuButton) Opened() bool
func (*MenuButton) Open() error
func (*MenuButton) Close()
func (*MenuButton) ConnectOpenError(func(error)) signal.Handle
```

既有 ConnectClosed 保留。同步 Open 返回错误，不同时发 OpenError；输入触发
的失败发一次信号。空菜单、不可见／禁用按钮为无操作。至少一个 Visible 且
非 Separator 项目即可打开；全部项目禁用仍可查看。

MenuButtonGroup 不保存成员列表或弹窗引用，不管理原生资源。按钮显式保存
组身份；活动菜单从所属窗口已显示的 Popover 中查询，严格限制相同 Widget
Root。键盘顺序来自实际 Widget 树，不依赖注册、Mount 或声明构造顺序。
组具有非零大小，避免不同实例受零尺寸对象地址规则影响。

SetGroup(nil) 解除关系；更换已打开按钮的组结束本次联动，菜单继续独立存在。
下一次打开才启用新的联动关系。卸载／销毁沿用 Popover 锚点生命周期。

## 声明式接入

```go
ui.MenuButtonGroup("main-menu", ui.HBox(
    ui.MenuButton("File").Group("main-menu").Menu(
        ui.MenuItem("Open", openFile),
    ).OnOpenError(reportError),
    ui.MenuButton("Edit").Group("main-menu").Menu(
        ui.MenuItem("Undo", undo),
    ),
    ui.Button("Run").OnClick(run),
))
```

MenuButtonGroupView 复用 ComposeBase／ObjectView，在自己的 State 创建组，
Object 无副作用返回已有实例，后代解除关联后释放状态。不生成包装 Widget。
Group(string) 查询最近的同名对象祖先，缺失或类型不符报告配置错误；空名
表示独立。不同声明分支可使用同名组，Group 声明的 ID 同时是其关联名字。

重建保留 Widget、组和未变的菜单模型。动作通过节点状态访问最新声明闭包，
不因重新创建函数而重装正在展开的菜单；卸载清空动作引用、断开 Closed／
OpenError 连接并解除关联。通用 Reparented 刷新祖先关系，整组移交保留实例。

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
下的 MenuButtonInfo：HasMenu、Opened、Grouped。组对象不产生语义容器，
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
