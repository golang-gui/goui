# MenuBar 与 macOS 原生菜单栏

> 状态：2026-11-23 设计草案，尚未实现。本文保存菜单栏接入的边界与实施指导；
> 新增类型、方法、语义角色和平台能力均为拟议接口，不代表当前支持。
>
> 依赖：[子菜单](DesignMenuSubmenu.md)、[菜单模型](DesignMenu.md)、
> [MenuButtonGroup](DesignMenuButtonGroup.md)、[共享命令](DesignCommand.md)。

## 1. 定位与范围

MenuBar 是菜单模型的呈现宿主，参与普通 UI 构建，根据模式在窗口内或 macOS
系统菜单栏呈现。模型与 Command 共用，展开、选择、焦点和原生资源属于各自宿主。

首版面向桌面：macOS 支持原生菜单栏，Windows 与当前 Linux 在窗口内呈现。
Linux 全局菜单协议、Windows 原生 HMENU、Dock 菜单、托盘菜单和移动端导航菜单
不在本设计范围。移动端可以共享模型和业务命令，桌面入口的可构建不代表移动菜单
已经适配或验收。

MenuButton 是可放置自定义 Child 的按钮，不直接作为 MenuBar 的菜单声明。
MenuButtonGroup 保持自由组合控件的局部菜单联动容器；不自动升级为系统菜单栏。
原生分支接受明确的菜单数据，不从 Widget 子树提取文字，也不转换任意 Widget。

## 2. 公共接口草案

```go
type MenuBarMode uint8

const (
    MenuBarAuto MenuBarMode = iota
    MenuBarInWindow
)

func NewMenuBar() *MenuBar
func (*MenuBar) Menu() MenuModel
func (*MenuBar) SetMenu(MenuModel)
func (*MenuBar) Mode() MenuBarMode
func (*MenuBar) SetMode(MenuBarMode)
func (*MenuBar) ConnectError(func(error)) signal.Handle
```

声明式入口：

```go
func MenuBar(items ...*MenuItemView) *MenuBarView
func (*MenuBarView) Mode(gui.MenuBarMode) *MenuBarView
func (*MenuBarView) OnError(func(error)) *MenuBarView
```

```go
ui.VBox(
    ui.MenuBar(
        ui.MenuItem("File").Submenu(
            ui.MenuItem("Open", openFile),
            ui.MenuItem("Save").Command("file.save"),
            ui.MenuItem("Recent").Submenu(recentItems...),
        ),
        ui.MenuItem("Edit").Submenu(
            ui.MenuItem("Undo").Command("edit.undo"),
        ),
    ).Mode(gui.MenuBarAuto).OnError(reportError),
    content,
)
```

链式 Submenu 的互斥、空声明及递归协调见子菜单设计；该入口仍待复核确认。
MenuBar 顶层首版只接受子菜单项，标题、Enabled、Visible 来自该条目；普通叶子项和
分隔线属于配置错误，两种呈现遵守相同约束。任意按钮混排使用 MenuButtonGroup。
空模型为正常无菜单状态。隐藏与空分支的交互规则沿用子菜单，不修改 Command 状态。

MenuBar 是普通 Widget，但不公开任意 Widget 子声明入口。窗口内顶层按钮是 GUI
根据模型生成的内部子树；UI 不逐个协调这些内部按钮，也不提供第二份 MenuButton
声明作为原生投影来源。样式只控制窗口内呈现，不承诺映射 AppKit 的字体、颜色和圆角。

## 3. 模式与构建生命周期

| 模式 | macOS 桌面 | Windows／当前 Linux |
| --- | --- | --- |
| Auto，默认 | 平台能力可用时接入系统菜单栏 | 窗口内呈现 |
| InWindow | 窗口内呈现 | 窗口内呈现 |

Auto 表示采用框架已支持的接入能力，不等于查询系统中所有可能的全局菜单协议。
平台报告支持情况及失败事实，GUI 选择呈现方式。尚不增加强制 Native 模式。

原生呈现时，MenuBar 继续挂载并接受声明式更新；CommandScope 引用、模型订阅和
最新动作不会因窗口内不绘制而解除。它退出窗口内布局、绘制、命中及 Tab 导航，
父布局不为其保留 spacing，显式 MinSize／MainWeight 也不强迫它在窗口内占位。
从原生切回 InWindow 时重新按当前可用宽度和样式测量，布局属性原值不改写。

用户显式 Visible(false) 或隐藏祖先会撤销两种业务呈现；原生呈现本身不是用户隐藏。
不能仅通过 Measure 返回零大小处理，因为父布局仍可能分配间距与权重；也不能
在 ui.ComposeView 中条件删除该节点。实施前在 GUI 评审最小的呈现参与机制，统一
检查布局、绘制、焦点、事件和快照，不将 MenuBar 特例散入每种布局或 UI Root。

MenuBarView 使用普通 WidgetView 和既有菜单描述协调。模式改变只替换呈现，不
重建业务模型和命令连接。模式／模型切换先结束旧展开链，完成新投影后提交呈现，
在正常布局和首帧前确定结果；原生菜单同步不经过 Painter 或 SharedPopover。

## 4. 应用菜单与窗口菜单来源

AppKit 的主菜单属于 NSApplication，GUI 中声明的 MenuBar 仍属于所在 Window。
GUI 选择当前窗口的有效业务菜单来源，平台只安装指定的原生菜单，不识别
MenuButton、CommandScope 或 Widget 祖先。

- 每个 Window 最多登记一个可见的 Auto 来源；其他业务条显式使用 InWindow。
  重复来源报告配置错误并保留原有效登记，不由树遍历顺序反复抢占。
- 普通窗口切换时选择新窗口来源；新窗口无来源或明确 InWindow 时，切回应用默认
  菜单，不能继续激活另一个文档窗口的业务操作。
- Popover、Tooltip、下拉菜单及临时附属表面不成为新来源。辅助面、原生面板和模态
  对话框按所属窗口关系处理；模态限制参与操作可用性，不让附属面抢走文档菜单。
- 应用失活不销毁各窗口投影；重新激活时根据实际窗口事实重选。所有来源失效后只
  保留应用默认菜单，不保留已销毁窗口的动作。
- 菜单挂载、隐藏、卸载、跨窗口移交和窗口销毁均更新登记。旧清理仅撤销自己仍
  持有的来源代次，不能清除随后安装的新来源。

窗口 active／main／key 及附属关系以 AppKit 事实为依据；GUI 不把任意 FocusOut
都当作切换文档来源。公开窗口输入焦点契约不因此改义。需要新增事实时先在
platform 独立验证，再用于 GUI 来源选择。

macOS 的应用菜单与 File／Edit 等窗口业务菜单分开。首版必须具备可用的基本应用
菜单，隐藏、显示和退出走对应原生／GUI 生命周期入口；不能直接 terminate 绕过
框架的退出处理。About、Settings、Window、Help 等角色映射的公共 API 单独复核，
不根据标题字符串、语言或 MenuButton 的内容猜测搬移条目。InWindow 只控制该组
业务菜单，不表示删除应用的基本系统菜单。

## 5. 原生投影、状态与动作

GUI 将模型转换为有限、明确的菜单项描述：稳定标识、纯文本标题、是否可见、
实际启用、分隔线、勾选状态、子菜单及可原生表达的快捷键。平台描述不持有
gui.MenuItem、Widget 或 ui.View。模型中的复用子树允许分别生成原生对象；AppKit
对象不因模型共享而同时被多个父菜单拥有。

原生 NSMenu／NSMenuItem 由平台接入对象拥有，所有操作在 AppKit 主线程。
已发布项在状态改变时同步；不每帧重建菜单，不为每项永久注册新的 UI 监听器。
原生自动启用可能覆盖手动 Enabled；自定义业务项采用明确的手动可用性或统一
验证桥，结果来自 GUI 实际状态。不能因安装了同一个 action selector 就全部启用。

原生选择发出包含来源、项标识及呈现代次的平台事件。窗口业务事件进入所属
Window.DispatchEvent，再由 GUI 校验当前来源、条目、祖先、模态限制及 Command，
执行既有激活入口。系统菜单操作不模拟鼠标／按键，自动化也不能直接调用回调。
无窗口时的系统级操作归应用事件入口，不借用已销毁窗口的分发器。

声明式回调始终读取最新节点状态；模型替换后的旧事件不能按索引激活新项。
叶子选择先结束菜单跟踪／展开，再执行业务，关闭或命令回调可重建 UI、切换窗口
或退出应用。执行前后均检查生命周期和代次，清理不依赖清空所有信号。

原生快捷键与 GUI Shortcut 必须保证一次输入只激活一次。先核对 AppKit key
equivalent 在现有事件循环中的顺序；能表达的键可进入原生等价键处理，已处理后
不得再次投递为 GUI 激活。无法表达的键保留 GUI 行为并如实显示能力限制，不能
只显示一个系统并不执行的快捷键。具体组合边界在平台阶段验证后固定。

菜单正在跟踪时，不直接重排 AppKit 禁止修改的结构。关闭／卸载立即使过期动作
失效，结构修改在安全边界提交；操作可用性仍以激活时的 GUI 状态复核。GUI 更新
调度须能在菜单跟踪期间处理必要的状态变化，不能靠延迟若干毫秒推断跟踪结束。

## 6. 错误与恢复

不支持原生接入时按 Auto 契约在窗口内呈现，是正常能力选择。
创建、状态同步或安装失败是实际错误：通过 ConnectError／OnError 报告一次。
新呈现完整可用后才撤下旧呈现；首次原生安装失败保留窗口内菜单与基本应用菜单。
实际呈现状态用于快照和诊断，不把“请求 Auto”冒充“原生安装成功”。

失败后保留可解释的有效资源状态，不在每次重绘中循环重试；后续有效模型／模式
变化可以重新尝试。回调重入导致请求被新操作取代属于失效请求，不重复上报旧错误。
同步平台调用返回的错误由 GUI 消化为这一次自动呈现操作的结果，平台不再另打日志。

## 7. 语义、平台差异与实现边界

拟为 MenuBar 增加 menubar 语义角色及请求／实际呈现属性。窗口内顶层项报告菜单
入口、可用性和实际展开状态；其 Popup 仍是独立宿主。原生模式下内部按钮没有窗口
内可点击边界，不伪造屏幕坐标或把系统菜单当作窗口子控件点击。

原生菜单自身的可访问性由 AppKit 提供；GOUI 快照说明当前来源和原生呈现状态。
需要覆盖系统菜单的自动化须经过原生正常输入，不能新加执行 MenuItem 的语义捷径。

| 归属 | 计划修改 |
| --- | --- |
| platform/common、platform/events、platform 导出 | 原生菜单能力事实、值描述和激活事件；接口先独立评审 |
| platform/darwin/frameworks/appkit | NSMenu／NSMenuItem、NSApplication 菜单、delegate 与 selector 绑定 |
| platform/darwin/cocoa | 原生资源、应用默认菜单、安装与事件回传；主线程及跟踪安全 |
| gui/MenuBar 与窗口边界 | 模型观察、来源选择、呈现参与、动作校验及资源清理 |
| ui/menu.go 或对应主类型文件 | MenuBarView、递归菜单协调和信号生命周期 |
| theme/basic、theme/modern | 窗口内菜单条样式，保持布局参数和样式分离 |

文件名与拆分按职责和实际体量决定。无须修改 Painter，不新增应用级 Popup 管理器，
也不把菜单命令解析放入 platform 或 UI 协调器。

## 8. 实施与验证

1. 先完成递归菜单模型、PopoverMenu 多级呈现与父子协调，覆盖右键菜单、独立
   MenuButton、MenuButtonGroup；完整子菜单能力是窗口内 MenuBar 的前置。
2. 在 platform 验证原生菜单创建、修改、选择、窗口事实、快捷键、跟踪期间调度和
   退出路径，确认应用级与窗口级事件的边界，不以 FFI 可调用代替应用可用。
3. 实现 GUI／UI MenuBar 的窗口内呈现、模式与来源生命周期，再集成 macOS 原生
   投影。按独立能力拆分提交，相关回归随实现提交。

逻辑测试覆盖递归描述、最新闭包、Command 绑定、隐藏／卸载、模式切换、无布局
留白、重复来源、来源切换、失败及旧事件代次。语义检查包含原生模式无窗口命中
区域，状态不能随 UI 重建遗留。

窗口验收覆盖 Linux／Windows 的窗口内菜单、macOS 的 Auto／InWindow、两个窗口
不同菜单、附属弹窗／模态限制、关闭来源窗口、亮暗与语言变化、禁用／勾选、动态
子菜单及一次性快捷键。macOS 同时注明裸可执行文件或 .app，验证应用菜单、正常
退出、激活恢复和实际菜单跟踪。全仓 go test、必要 race 与交叉编译分别记录；
构建、原生对象创建和窗口启动均不替代真实菜单操作验收。

实施前还需复核：应用菜单角色 API、通用呈现参与机制、原生快捷键可表达集合、
多窗口与模态面板来源优先级，以及平台能力／事件的最终命名。本文不宣称这些
细节已完成原生验证，也不将其混入子菜单基础模型的实现批次。

## 9. 参考与取舍

2026-11-23 核对：

- [Qt 6.8 QMenuBar.nativeMenuBar](https://doc.qt.io/qt-6.8/qmenubar.html#nativeMenuBar-prop)
  文档承诺支持的平台可在原生与窗口内菜单间选择；GOUI 首版只接 macOS 原生，
  不承诺 Qt 的 Linux 全局协议或按标题推断角色。
- [Qt 6.8.3 Cocoa 菜单栏源码](https://github.com/qt/qtbase/blob/v6.8.3/src/plugins/platforms/cocoa/qcocoamenubar.mm#L241)
  区分窗口来源与应用菜单，并排除临时 Popup 对正常来源的抢占。采用职责划分，
  不照搬其全局对象表、文本匹配或特定系统兼容处理。
- [Flutter PlatformMenuBar](https://api.flutter.dev/flutter/widgets/PlatformMenuBar-class.html)
  当前官方文档将其保留在构建树中以更新菜单数据，原生显示没有 Flutter 视觉实体。
  GOUI 结合窗口内模式，保留普通 Widget 生命周期，不另建 UI 原生菜单树。
- Apple [菜单归属](https://developer.apple.com/library/archive/documentation/Cocoa/Conceptual/MenuList/Articles/HowMenusWork.html)
  说明 NSApplication 主菜单与 NSMenu 层级；[启用规则](https://developer.apple.com/library/archive/documentation/Cocoa/Conceptual/MenuList/Articles/EnablingMenuItems.html)
  说明自动与手动验证的区别。这些是 AppKit 契约，窗口来源策略由 GUI 决定。
- Apple [Views in Menu Items](https://developer.apple.com/library/archive/documentation/Cocoa/Conceptual/MenuList/Articles/ViewsInMenuItems.html)
  实际允许部分自定义 NSView，且有输入和跟踪限制。GOUI 不支持任意 Widget 转换
  是本设计的范围选择，不能写成 AppKit 完全不支持复杂内容。
