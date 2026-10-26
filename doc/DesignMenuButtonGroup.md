# MenuButton 分组与菜单栏组合

> 状态：设计，尚未实现。CheckButton 显式对象声明已单独实现；本文不将
> MenuButton.Group、键盘切换或菜单扩展状态写成已具备的能力。

## 目的与边界

多个 MenuButton 显式组成一组。点击其中一个打开菜单后，鼠标移到同组另一
按钮即切换菜单。应用使用 HBox、WrapBox 等普通布局组合菜单栏，也可以插入
运行、保存等普通 Button；组不拥有布局，不新增 MenuBar 容器。

命令式控件管理弹出会话、输入和状态；声明式显式声明 MenuButtonGroup 对象，
成员通过名字引用最近祖先，复用 ComposeView／ObjectView 与 FindAncestor。
不修改 platform，不增加中央弹窗管理器或全局名字解析。子菜单是独立设计，
本能力可在单层菜单基础上实施，不以子菜单实现为前置。

参考官方公开契约（2026-10-25 查阅）：

- [Flutter MenuBar](https://api.flutter.dev/flutter/material/MenuBar-class.html)
  规定点击开启菜单，打开后可以悬停切换，外部点击／Esc 关闭菜单系统。
  采用其显式开启后悬停切换的交互，不照搬 MenuBar 容器和对象层级。
- [Qt QMenuBar](https://doc.qt.io/qt-6/qmenubar.html) 提供水平菜单栏和平台原生
  菜单栏集成；GOUI 当前目标是普通控件组合，系统原生菜单栏不在范围内。
- 声明式名字与运行时组对象的区别沿用
  [CheckButton](DesignCheckButton.md#声明层)，不同 Root 相互隔离。

这些是公开契约参考，不代表 GOUI 已做对应平台实测。本文的 modal 路由判断
来自当前 gui/window.go、gui/popover.go 和 gui/menu.go 的源码检查。

## 公共接口与用法

拟议命令式接口：

```go
func NewMenuButtonGroup() *MenuButtonGroup
func (*MenuButton) Group() *MenuButtonGroup
func (*MenuButton) SetGroup(*MenuButtonGroup)
```

Group 只表示关联身份，不维护成员列表，不拥有控件、原生弹窗、任务或信号
连接；不提供另一套组级 Open／Close 或动作注册。命中和键盘导航都以实际
Widget 树过滤 Group 相同者，无需另记注册顺序。构造的组保证具有独立身份，
不依赖 Go 零尺寸对象的指针唯一性。

SetGroup(nil) 解除关联，原有独立 MenuButton 保持可用；不根据父容器隐式
建组。移除／更换打开按钮的组时静默结束联动会话，当前菜单继续作为独立菜单
存在，不把修改关系本身视为一次关闭／重开。后续真实关闭仍按普通 Popover
处理；按钮卸载、销毁和跨宿主迁移则沿用正常的锚点失效清理。具体弹窗和
异步资源始终由按钮／PopoverMenu 持有。

拟议声明式接口与用法：

```go
ui.MenuButtonGroup("main-menu", ui.HBox(
    ui.MenuButton("File").Group("main-menu").Menu(
        ui.MenuItem("Open", openFile),
    ),
    ui.MenuButton("Edit").Group("main-menu").Menu(
        ui.MenuItem("Undo", undo),
    ),
    ui.Button("Run").OnClick(run),
))
```

拟议 MenuButtonGroupView 是非视觉 ObjectView，首次 Build 创建实际组对象，
Object 提供借用，Release 在成员解除关联后清理自身状态。它只产生内容的
Widget 输出，不添加 GUI 包装节点。整个声明移交时保留组实例。

`.Group(string)` 引用最近的同名祖先 MenuButtonGroup，不自动创建对象。
缺失或类型不符报告配置错误；空名表示独立。应用无需在 Build 外保存组，
不同组件分支可用同名声明隔离，不存在 Root 级组注册表。

组名不限定直接布局父级；共同对象祖先范围内的不同容器可显式共享组。命令式即便把同一组
对象用于不同窗口，悬停／键盘切换也只在活动菜单的 owner Root 内寻找成员，
不关闭或切换另一窗口的菜单。活动会话依附 owner 的当前 modal 菜单，不在
组中维护强引用窗口表。

## 输入行为

| 情况 | 行为 |
| --- | --- |
| 组内菜单均关闭，鼠标经过按钮 | 只显示普通 Hover，不打开菜单。 |
| 点击／Enter／Space 激活有效菜单按钮 | 成功显示后建立活动会话。 |
| 活动时移到同组、同 Root 的另一有效按钮 | 立即关闭旧菜单并打开新菜单，不套用子菜单延迟。 |
| 移到标签／图标子控件 | 沿命中路径识别所属 MenuButton，效果与按钮本体相同。 |
| 移到当前按钮、间隙、普通按钮或另一组 | 保持当前菜单，普通控件不因悬停触发动作。 |
| 移到隐藏、禁用、被遮挡或无菜单内容的成员 | 不切换；遵守整个祖先路径的可见／启用条件。 |
| 点击当前展开按钮 | 关闭会话，不立刻重新打开。 |
| 点击同组其他有效按钮 | 切换到目标；同一次 Down／Up 不再把新菜单关掉。 |
| 激活叶项、Esc、外部点击或窗口失焦 | 结束会话，后续单纯悬停不再打开。 |

可打开的菜单至少包含一个 Visible 且非 Separator 的项目；全部项目禁用时
仍可以显示菜单。空菜单、仅分隔线菜单不参与切换，不修改调用方模型的 Enabled。

混排普通 Button 的点击沿用现有 modal 外部点击约定：第一次点击关闭菜单并
消费，下一次点击执行普通按钮动作。不自动重放事件，也不因为同一 HBox 而
给普通按钮特权。这样混排布局无需改变任意底层控件的点击语义。

## owner 指针路由与安全切换

当前 Window.routeToModalTarget 消费 owner 上的全部指针事件，Down 触发 dismiss；
仅增加 MenuButton 的 ContainsHover 回调无法实现分组切换。需要在 gui 内增加
面向菜单的私有协作：

1. owner 收到真实 PointerMove 时，活动 modal 菜单先获得观察机会。继续用
   EventDispatcher 的正常命中规则查找 owner 中实际命中的控件及祖先路径，
   过滤遮挡、裁剪、祖先禁用和不属于当前 Root 的对象。
2. 只把已验证的同组 MenuButton 作为切换候选，消费当前事件；不向底层普通
   控件派发 Click，不把原生 Popup 局部坐标直接当作 owner DIP。
3. PointerDown 同样识别当前锚点及同组目标，完成关闭或切换；其余位置沿用
   modal dismiss。配套处理 Up，避免一次手势先切换后误关闭。
4. 布局／样式变化造成的合成 Hover 更新不能触发切换，以免菜单无鼠标移动
   自行跳转。不改变 MotionEventController 所报告的物理 Hover。

复用现有 Window／Popover 事件入口，外部不增加 DispatchEvent 捷径。私有协作
不扩大 ModalTarget 公共契约，其他类型的模态浮层保持原有规则。

切换通过会话序号保护：记录预期源和候选，关闭旧菜单，再核对窗口、对象、
组成员关系与序号后显示新菜单。Closed／错误回调可能销毁窗口、按钮或打开
另一浮层，届时停止旧切换。旧 Popover 的延迟关闭只能撤销它实际拥有的 modal
目标，不得清除新菜单。源 Closed 仍按真实关闭发出一次，不为切换合并或吞掉。

成功显示新菜单后才提交活动身份及展开状态。显示失败则结束该次会话，清理
modal 和视觉状态，通过菜单统一错误信号反馈；同步 Show 方法返回值不再重复
发错误。现有 log-only 打开失败需要在实施时按此契约补齐，不把失败保留成
逻辑上仍展开的菜单。与 [子菜单设计](DesignMenuSubmenu.md) 的错误规则保持一致。

## 键盘、样式与语义

根菜单的 Left／Right 切换同组、同 Root 内前／后有效按钮，按实际 Widget 树
顺序循环，跳过普通按钮；不是按组注册或 Mount 顺序。子菜单实施后，进入子菜单
和返回父菜单优先，只有根层剩余的水平导航交给组。Esc 先按菜单层级处理，
根层关闭才结束会话。Tab 沿用现有 modal／焦点契约，本轮不新增 Alt 助记符。

菜单展开与物理 Hover 分开保存。展开按钮使用 Basic／Modern 的持续选中背景
及原有 FocusVisible；移动到 Popup 内时仍能识别当前顶层按钮。可复用按钮部件，
但不伪造 Hover 状态，不影响普通 Button。

MenuButton Snapshot 继续作为按钮，补充菜单可用性及展开状态的值属性；按钮
动作仍是正常 Click，不暴露直接切换组的自动化动作。组是非视觉关系，不产生
虚构的语义容器；真实打开的 Popup 继续按现有规则出现在窗口快照中。

## 实施文件与验证

预期涉及 gui/menu.go 的组与按钮状态、gui/window.go／gui/popover.go 的私有
modal 协作和身份校验、ui/menu.go 的 Group(string) 薄绑定，以及 Basic／Modern
样式、语义快照、对应逻辑测试和独立窗口程序。先单独实现顶层切换，不顺带
实现子菜单、系统菜单栏或编辑器功能。

回归必须覆盖：空闲悬停、真实 Move 切换、跨容器与混排、子控件命中、遮挡／
隐藏／禁用、空菜单与全禁用菜单、同 Root 顺序导航、不同 Root／组的隔离，
以及 Closed 重入、打开失败、快速 A→B→C、旧关闭回调和窗口销毁。声明式覆盖
重建复用、改名、卸载和跨 Root 移交；组的 UI 生命周期复用 ObjectView，无需再建一套实现。

桌面按正常 Window.DispatchEvent 输入验收，包含 Software／OpenGL 的 1x、
1.5x、2x 与 Windows／macOS 原生弹窗。检查 native popup grab 下 owner 是否实际
收到鼠标事件；若事件被 grab 重定向，须在既有 GUI 路由处用明确坐标转换处理，
不能凭桌面未运行的单元测试声称已具备跨平台悬停切换。
