# 共享操作与声明式命令作用域

> 状态：2026-11-04 命令、菜单与 UI 作用域已实现，本机独立 X11 验证通过。
> 2026-11-05 扩展：子树声明、BindEnabled、直接对象引用已实现，本机验证结果见 Plan。
> Windows/macOS 的实际运行范围及后续结果见 Plan。

## 职责

Command 是 GUI 线程上的共享操作，负责激活、Enabled、关联 Shortcut 的只读查询及通知。
Shortcut 拥有 Gesture、自身 Enabled、重复策略和 Command 关联；Controller 负责作用域与输入传播。
MenuItem 拥有标题、可见性和自身 Enabled，引用 Command；PopoverMenu 管理显示期间的观察和列布局。
Button 保持普通回调，不增加 CommandButton，不修改 WidgetBase 或平台层。

## 命令式契约

```go
func NewCommand() *Command
func (*Command) Enabled() bool
func (*Command) SetEnabled(bool)
func (*Command) ConnectEnabled(func(bool)) signal.Handle
func (*Command) Activate()
func (*Command) ConnectActivate(func()) signal.Handle
func (*Command) Shortcuts() []*Shortcut
func (*Command) ConnectShortcuts(func()) signal.Handle
func (*Shortcut) Command() *Command
func (*Shortcut) SetCommand(*Command)
func (KeyGesture) DisplayText() string
```

Command 默认启用，无原生资源，不可在使用后复制。Activate 在每个回调前检查有效性。
Enabled 变化先提交再通知；不变值不通知，禁用又恢复不能恢复旧激活过程。
SetCommand 是唯一关联修改入口；先提交双向关系，再通知旧、新 Command。
Shortcuts 返回关联顺序的切片副本，不代表各 Controller 的实时注册与焦点状态。
关联或 Gesture 变化通知 ConnectShortcuts；局部 Enabled、重复策略和注册变化不影响提示。
默认提示取首个 DisplayText 非空的关联 Shortcut，不根据临时启用或焦点筛选。

Shortcut 在输入到达时读取 Command.Enabled，不订阅、不另存命令启用状态。
禁用候选不消费按键；不允许重复的匹配项仍消费重复输入。关联 Command 是业务目标，
ConnectActivate 保留绑定激活通知，不额外合成输入。回调解除／替换关联、移除注册、
禁用或销毁宿主后，不激活过期命令。未关联命令的直接回调方式保持兼容。

MenuItem 保留 Action／SetAction 与 Command／SetCommand；后设置的来源替换前者，
清空不恢复旧来源。Enabled getter 始终是自身值。实际交互为自身、祖先与 Command 的
限制组合，局部禁用不写回 Command。执行前保存来源，Hide 后再确认目标和有效性，
保持释放模态输入后执行业务的顺序。

GUI 所有者不再使用 Shortcut 时用 SetCommand(nil) 解除反向引用。Controller.Remove
只移除当前注册，不影响其他 Controller。窗口或菜单不销毁外部命令。

## 菜单实时更新与布局

PopoverMenu 显示前按可见非分隔项收集 Command，去重观察 Enabled 和 Shortcuts；
模型变化更新集合。隐藏、原生关闭、显示失败、销毁时解除观察，模型不长期订阅。
禁用时立即清除失效的活动／按下项和未完成点击，重新启用不恢复旧点击。
只变色不重载 ListView 或清空所有测量；真实字体变化仍由既有失效路径处理。
快捷键配置变化重新获取提示，仅在文字变化时失效文字和自然尺寸。

每行采用独立 Label：menu-item-text、menu-item-shortcut，及各自的 -disabled 样式。
子 Label 独立解析样式。标题左对齐，快捷键右对齐，同一行按基线对齐；菜单内共享列宽，
分隔线不重置，离屏条目参与测量。自然宽度为最大标题、24 DIP 间距、最大快捷键及留白；
无快捷键时省略该列及间距。保留 160 DIP 最小主体、工作区与滚动条间距契约。
DisplayText 复用 GUI Key／修饰语义；Windows/Linux Primary 显示 Ctrl，macOS 显示 ⌘；
等价修饰合并，未知或当前平台不支持的组合返回空，不解析字符串或注册按键。

Snapshot 的 Text 是标题 PlainText，快捷键用 goui.menu-item Attribute；Enabled、
Selected、Pressed 与实际交互一致。Command／Shortcut 不添加 Widget 或自动化执行捷径。

## 声明式命令作用域

```go
ui.CommandScope(
    ui.VBox(
        ui.MenuButton("编辑").Menu(ui.MenuItem("复制").Command("copy")),
        editor,
    ).Shortcuts(ui.Shortcut(ui.KeyC).Modifiers(ui.ModPrimary).Command("copy")),
).Commands(ui.Command("copy").Enabled(canCopy).OnActivate(copyText))
```

CommandScope 是 ObjectView，无占位 Widget 和布局边界。命令 ID 仅在作用域内唯一，
不与 Widget ID 混用。按祖先作用域查找，最近同名定义遮蔽外层；未定义时向外查找，
不跨窗口。空定义 ID 及重复 ID 报告声明错误；同一作用域的分布式重复在本轮协调
结束时检查。未解析引用呈禁用，
FindCommand(ctx, id) 返回对象与成功标志，供自定义 View 查询和诊断，不能执行旧目标。

底层 gui.Command 以作用域与命令 ID 保持身份，顺序变化不重建，回调读取最新声明。
移除定义在本轮结束时从集合删除、禁用并解除激活连接；存活引用重新解析或禁用。
作用域通过自身集合变化信号通知引用方，不在 Root 加命令注册表。
内层作用域跟随外层集合与 ancestry；显式子树移交后重新解析，不重放业务操作。
Scope.Release 由 ObjectView 在后代清理后执行，清理全部命令与父作用域连接。

MenuItem.Command(string) 与 Shortcut.Command(string) 选择操作来源，替换直接回调；
局部 Enabled 仍属于该入口。MenuItem 函数参数可省略。构造声明没有副作用。
UI 继续协调自己创建的 Shortcut，卸载时移除注册、SetCommand(nil)，不保留旧命令引用。
菜单引用与快捷键引用解析到同一运行对象，不创建两份命令，不引入 UseShortcut、
全局 ID 或新的 BuildContext 命令接口。

## 子树声明与启用绑定

```go
ui.CommandScope(
    ui.VBox(
        ui.MenuButton("File").Menu(
            ui.MenuItem("Close").Command("file.close"),
        ),
        ui.Command("file.close").BindEnabled(&hasDocument).OnActivate(closeFile),
        editor,
    ).Shortcuts(ui.Shortcut(ui.KeyW).Modifiers(ui.ModPrimary).Command("file.close")),
)
```

Command 是返回 nil 内容的 ObjectView，使用既有 Build、Release、Reparented 协调。
它可以出现在任意后代 View 中，不生成 Widget、测量尺寸、容器间距或 Snapshot 节点。
构造 Go 值没有注册副作用；必须进入 View 树或 Commands 列表才生效。无祖先作用域的
独立声明报告错误。隐藏 Widget 不卸载声明；移除 View 才注销定义。

最近的 CommandScope 集中拥有 gui.Command。每份定义只属于最近作用域，同一作用域
内一个 ID 只能有一份定义；内层可同名遮蔽并向外回退，不按焦点自动挑选同名定义。
.Commands(...) 是直接挂在作用域上的简写，使用相同定义注册、属性绑定和发布路径。
简写以操作 ID 保留定义；独立节点按普通 View 身份协调，不将命令 ID 用作 Widget ID。

各定义只提交自己当前的 ID、回调和可用性，作用域通过自己的 AfterUpdate 一次发布
本轮最终集合，发布前完整验证唯一性，再复用同 ID 对象、移除过期对象并通知引用。
同一轮中先添加新位置再移除旧位置不会误判重复，也不会把原命令禁用再重建。
前向引用最初可为禁用占位，集合发布后在布局／绘制前解析完成。FindCommand 返回
最近一次已发布对象；在首次 Build 中可能尚未解析，不能依靠子节点声明顺序取对象。
AfterUpdate 仍按既有注册顺序执行，不承诺早于命令作用域的其他完成钩子已看到本轮集合。

BindEnabled(ReadState[bool]) 复用 BindValue、来源订阅和节点更新队列，只更新操作可用性，
不重建父节点、编辑器或菜单。Enabled(value) 替换绑定，BindEnabled(source) 替换值，
nil 绑定不改变声明；换源、取消和卸载解除旧订阅。结构增删继续通过 Build 表达。

显式移交重新确定定义的最近作用域及所有引用；本次移交全部 ancestry 安装并完成
committed 回调后，借助既有 AfterTransfer 发布两侧集合，返回调用方前生效。
移交整个 CommandScope 保留其对象，移交定义至不同作用域由目标作用域拥有新对象，
源对象禁用。输入仍只经过现有 Dispatcher／Controller，不在 Root 添加命令分支。

## 借用命令式对象

```go
ui.MenuItem("Export").CommandRef(exportCommand)
ui.Shortcut(ui.KeyE).Modifiers(ui.ModPrimary).CommandRef(exportCommand)
```

Command(string) 保留作用域内 ID 语义；CommandRef(*gui.Command) 是显式借用外部对象，
无需作用域，不使用 any 参数或运行时重载。菜单的直接 Action、ID 和外部对象来源互斥；
快捷键的 OnActivate、ID 和外部对象来源同样互斥，最后设置者生效。CommandRef(nil)
清除操作。Window.Shortcuts 可直接引用对象，不反向查找 Content 内的命令作用域。

UI 不改变外部 Command.Enabled、不替换其业务处理者，也不因卸载而禁用外部对象。
UI 只管理自己创建的 Shortcut、关联及观察连接；解绑调用 SetCommand(nil)。
应用事件循环退出但原生窗口尚未销毁时，App.stop 也清理声明层的窗口级 Shortcut，
保留命令式注册；不靠关闭进程掩盖外部 Command 持有的关联。
直接对象替换保持未变菜单模型，GUI MenuItem.SetCommand 使打开菜单实时更新。
Menu(...)、MenuFor(...) 和控件的 Menu 声明均支持直接对象；其中只有 ID 路径需要上下文。
外部命令须遵守 GUI 线程契约，生命周期由提供者负责，不隐式注册 ID 或转移所有权。

## 转换入口与共享修饰符

内置 MenuButton 直接使用自身 BuildContext 解析命令 ID。独立的 ui.Menu(...) 没有上下文，
只转换普通回调，含命令 ID 的项保持禁用。自定义复合 View 在请求上下文菜单时使用
MenuFor(ctx, items...)；TreeView/TableView/TabView 的 UI 适配已使用此入口。
返回模型在有 ConnectItems 观察者期间跟随作用域集合变化，解除模型观察即解除对应连接。
FindCommand(ctx, id) 是供自定义 View 借用的公开查询，不转移命令所有权。

Shortcut 的 ID 引用必须位于 CommandScope 的后代声明；Window.Shortcuts 不属于
Content 内的祖先作用域，不能倒查后代。将命令快捷键声明到作用域内根 Widget 的
Shortcuts，原有窗口 OnActivate 快捷键保持不变。
共享修饰符的持久上下文仅保存通用 BuildContext；移交时刷新其 ancestry 绑定。
Root／BuildContext 没有命令表、命令接口或具体命令类型分支。

菜单保留共享列占位，但将快捷键 Label 的自然文字框贴齐列右边，而不是将 Label
铺满整列后依赖原生单行段落对齐。实际截图验证文字墨迹位置；矩形右边一致不足以
单独证明视觉对齐。不为菜单修复改变通用 Label 或 typography 的行为。

## 参考与适用范围

- GTK4 [动作](https://docs.gtk.org/gtk4/actions.html) 与
  [set_accels_for_action](https://docs.gtk.org/gtk4/method.Application.set_accels_for_action.html)：
  采用操作与入口分离、首个快捷键提示，不采用字符串动作注册与 GVariant 参数系统。
- Qt Quick Controls 6.12 [Action](https://doc.qt.io/qt-6/qml-qtquick-controls-action.html)：
  支持声明共享操作并由外部传给入口；采用声明与对象借用，仍不把文字、图标、
  按键定义或通用 Button 绑定纳入 GOUI Command。
- Flutter [Actions and Shortcuts](https://docs.flutter.dev/ui/interactivity/actions-and-shortcuts)：
  参考操作定义和按键入口可以分处子树的职责分离，不引入 Intent／Dispatcher 类型体系。
- Qt 6 [reservedShortcutWidth](https://doc.qt.io/qt-6/qstyleoptionmenuitem.html#reservedShortcutWidth-var)：
  采用菜单统一预留快捷键列，不让调用方补空格。
- Avalonia 11.3.0 [Button](https://github.com/AvaloniaUI/Avalonia/blob/11.3.0/src/Avalonia.Controls/Button.cs#L201)：
  独立控件限制与命令可执行性可以组合；本轮只用于菜单，不据此扩展 Button。

以上是官方契约／源码参考。GOUI 的 API、作用域、Enabled 与对象生命期以上文为准。

## 验证与提交切片

1. Command、Shortcut、DisplayText 与包内回归。
2. 菜单实时状态、双列布局、Basic/Modern 与回归。
3. UI CommandScope、ID 引用、增删／嵌套／局部更新／移交回归。
4. 独立 GUI/UI 窗口例程与实际验收记录。

全部修改完成后运行 go test ./...、针对性 race/vet、Windows amd64/macOS arm64 构建。
本轮用户要求本机独立 X11 运行；外平台编译不等于原生验收。窗口例程通过正常输入
检查实时禁用、重新启用、修改按键、重开菜单、局部禁用、列对齐与作用域隔离。

## 实际验证入口

- `go run ./tests/window/gui/commands`：对象关联、实时灰显、改键及独立入口启用。
- `go run ./tests/window/ui/commands`：ID 定义、局部更新、同名遮蔽／移除回退。
- UI 例程另覆盖先引用后子树声明、BindEnabled 不重建、局部分支移除回退、
  Window.Shortcuts 直接借用外部命令及运行循环退出后的关联清理。
- 两者默认人工操作；`-auto` 通过正常 DispatchEvent 验证后退出，`-basic`、`-dark`
  选择外观，`GOUI_PLAT_PAINTER` 与 `GOUI_PLAT_SCALE` 选择后端和覆盖倍率。
- 包内回归包括关联去重与解除、通知重入、回调中卸载、旧激活取消、列宽及自然文字框
  右齐、模型／作用域替换、上下文菜单观察、显式跨根移交与失效上下文查询。
- 外部原生输入验证和完整后端／倍率组合结果统一记于 Plan，不将编译或启动当成验收。
