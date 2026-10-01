# TabBar 与 TabView

状态：命令式控件、声明式适配和包内测试已接入；真实窗口交互按下述入口验收。

跨 TabView、跨窗口页面转移与拖出新窗口见 [Tab 转移设计](DesignTabTransfer.md)。
2026-10-01 声明式页面改为普通 WidgetView，并复用通用 ID 协调和显式节点移交；
不再另存页面 key 映射。历史栏外取消说明以转移设计的当前契约为准。

## 定位和使用

复杂控件放在同仓库 `widgets`，声明式适配放在 `widgets/ui`。现有 `gui`
控件保持原位。`TabView` 持有页面、顺序和当前页；`TabBar` 关联它，可以放在
`HeaderBar`，页面放在窗口主体。一个 TabView 可有多个 TabBar。当前栏内功能不要求
平台层新增能力；跨窗口拖出的原生前置工作另见转移设计。

```go
pages := widgets.NewTabView()
pages.AppendPage(widgets.NewTabPage("Home", home))
bar := widgets.NewTabBar()
bar.SetView(pages)
bar.SetTabWidthRange(112, 240) // equal widths: shrink first, then scroll
```

Tab 专属的 `RoleTabView`、`RoleTabBar`、`RoleTab`、`RoleTabPanel` 常量归属
`widgets`，类型仍为开放的 `gui.Role`，字符串保持 `tabview/tabbar/tab/tabpanel`。
通用选择状态 `WidgetInfo.Selected` 留在 GUI。GUI 不需要认识 Tab 类型或注册这些角色。

基础外观由 `widgets/style.Rules()` 提供，不再加入 `gui.DefaultStyleRules()`。
不使用 Modern 时，应用显式组合（`widgetstyle` 为 `widgets/style` 的导入别名）：

```go
rules := append(gui.DefaultStyleRules(), widgetstyle.Rules()...)
// Append application overrides here, then install the sheet on the application/root.
sheet := style.Sheet(rules...)
```

规则包只显式读取 GUI 固定默认 Label 字体，保留原有平台字体默认值；不读取当前主题、
不监听设置、不向父子控件传递样式、不在 init 中注册。Modern 已提供完整 Tab 外观，
继续直接使用 `modern.Sheet`，无需叠加基础规则。`-fallback-style` 窗口验收参数可验证
显式组合后的基础外观；此模式不响应例程的 Dark/Text size 外观按钮。

声明式调用使用 `wui.TabView(wui.TabPage("home", home).Title("Home"))`，
`wui.TabBar("documents")` 根据 TabView 的 Widget.ID 关联。
`TabPage("home", home)` 的第一个参数就是该页面的 Widget/UI ID，不是另一个 key。
页面嵌入 ViewBase，是对应 widgets.TabPage 的普通 WidgetView；非空 ID 在 Root 内唯一。
重复的页面 ID 在修改列表前拒绝，不同 Root 可复用 ID，转移时目标冲突拒绝。

## 页面与选择

TabPage 是单子控件容器。标题、图标来源和 Closable 是自身属性。TabView
只允许直接拥有一个页面；页面不能被多个视图共享。空视图 Current 为 nil，
第一项加入时自动选中。当前页移除时选择原位置右侧页面，若没有则左侧。
重复选择和无效操作不产生变化。CloseRequest 是可异步响应的请求，应用
确认后才调用 RemovePage。RemovePage 不受 Closable 限制。

非当前页面保持挂载，但页面容器不可见；业务 child 的 Visible 不被改写。
因此文字、选择、滚动等 Widget 状态可跨切页与重排保留。隐藏后焦点按照
现有 Widget 可见性规则清除，本版不恢复历史焦点，也不暂停业务计时器。

页面顺序与 TabView 的子节点顺序保持一致。`MovePage(page, index)` 的 index 是
最终页面位置：向前移动调用 `MoveChildBefore(page, target)`，向后移动调用
`MoveChildAfter(page, target)`，不移除再挂载。WidgetBase 仅提供相对兄弟重排，
完整契约见 [Widget Painter 的兄弟顺序说明](DesignWidgetPainter.md#兄弟顺序与相对重排)。
TabBar 的标签布局依据页面顺序；拖动期间把被拖标签移至视口子节点末尾只用于置顶，
不改变页面顺序。释放提交后才更新页面列表，取消/归位后恢复普通兄弟顺序。

TabView 的私有容器适配器把 AddChild／RemoveChild／MoveChildBefore 映射到页面 API；
UpdateChildren 按页面 ID／类型管理页面节点，页面自己 UpdateChild 管理内容。
无私有双向 pages/key 注册表，也不查询另一 View 的 State。Current、OnCurrent、
OnCloseRequest、OnMoved 使用页面 ID。协调期间 GUI 信号不转发给 UI 用户回调。
TabView 最后应用显式 Current 和选择态可见性；页面容器的 Visible 由选择管理，
内容的 Visible 仍属于自身。若应用不更新声明顺序，下一轮重建会恢复原顺序。
成功转移报告 TabTransfer.PageID／TabDetachRequest.PageID，应用更新两侧集合并
RequestUpdate；OnTransfer 在选择通知之前执行。参见转移设计的最新切片。

## 栏、输入和布局

### 标签关闭请求与上下文菜单（2026-10-01）

关闭请求统一由 `TabView.RequestClose(page)` / `ConnectCloseRequest` 承载；
关闭按钮和菜单动作使用同一路径，请求本身不选择、不移除页面。
页面不可关闭、已销毁、已移交或正在移交时不发出请求；前一个监听者移除页面后，
后续监听者不得继续操作失效目标。异步确认后须重新确认页面归属，再调用 RemovePage；
RemovePage 不受 Closable 限制。

上下文菜单采用“信号查询模型、标签栏管理显示”，而不是要求应用手动管理 Popover：

```go
bar.ConnectContextMenu(func(page *widgets.TabPage, result *gui.MenuModel) {
    menu := gui.NewMenu()
    menu.Append("关闭", func() { view.RequestClose(page) }).SetEnabled(page.Closable())
    *result = menu
})
```

- 查询和弹出管理均归属 TabBar。同一 TabView 的多个 TabBar 各自提供菜单策略、管理
  一个 PopoverMenu，互不覆盖；需要相同菜单时显式复用构建函数，不隐式继承 TabView
  的菜单。关闭请求仍归属 TabView。没有 SetContextMenu、SetOnContextMenu 或内置菜单命令集合。
- 查询在 GUI 线程同步执行，结果初始 nil，按信号连接顺序传递；后连接者可以替换或
  清空模型，结果指针不得保留。nil 或零条目的模型不弹出；可每次新建或复用模型。
- 菜单目标是命中的页面，不是 Current。右键包含标签关闭按钮区域，不选择、关闭或
  启动拖动；macOS Control+单击同义，并在 capture 阶段阻止主键点击／拖动。
  使用与现有文本编辑上下文菜单一致的 PointerDown 时机。空白和间隙无菜单控制器，
  保持 HeaderBar Caption 行为；排序或原生拖放进行中不打开菜单。
- Shift+F10 针对获得焦点的标签（含关闭按钮），不是当前页；菜单位置取标签下沿。
  物理菜单键尚未进入现有平台按键枚举，本切片不扩展平台层来支持它。
- 输入坐标由标签局部转换为 TabBar 局部 DIP，包含视口位置和滚动后的实际标签位置；
  原生窗口位置／屏幕退避仍由现有 PopoverMenu 处理，用户不用处理坐标。
- 回调中卸载、解绑、移除或转移目标后，不显示旧菜单；Bar 的查询代次防止旧查询
  覆盖嵌套的新查询。页面移除／移交、Bar 解绑／卸载和开始拖动时关闭菜单；正常
  dismiss 后也清空模型引用，不长期保留捕获页面的动作。原生资源沿已有 anchor 生命周期释放。
- `TabBar.ConnectContextMenuError(func(error))` 单独报告输入触发的弹出失败；取消或
  未提供菜单不是错误，不重复打印日志，也不混入 TransferError。
- 已打开的菜单是这次查询得到的模型；UI 重建不会重新执行菜单构造器或自动重开菜单。
  复用模型时可通过已有模型通知更新菜单。应用自定义动作应自行检查业务对象的有效性。

UI 使用普通链式声明，builder 在每次请求时使用最新声明，不在协调期间执行：

```go
wui.TabView(pages...).ID("documents").
    OnCloseRequest(confirmClose)
wui.TabBar("documents").
    ContextMenu(func(pageID string) []*ui.MenuItemView {
        return []*ui.MenuItemView{ui.MenuItem("关闭", func() { requestClose(pageID) })}
    }).
    OnContextMenuError(reportError)
```

`ui.Menu(items...)` 将已有 MenuItemView 描述转换为新的 MenuModel；MenuButton 与
widgets/ui 复用它，不读取描述符私有字段、不增加 Root/BuildContext 特例。
MenuModel 以 ui 别名公开，构造过程不执行菜单动作。协调仅更新 builder／错误回调，
省略属性恢复无菜单，Unmount 断开声明式连接。Snapshot 保留原标签角色、完整标题、
Selected 和关闭按钮动作；不为仅查询菜单而伪造新的点击或关闭动作。

不采用 `ContextMenu(MenuItem(...), ...)` 的静态列表：现有 MenuItem 动作是 `func()`，
无法直接获得本次右键目标；将动作绑定 Current 会误操作其它页面，保存共享 contextPage
又引入隐含状态。按请求页面 ID 构建可以直接捕获目标，也支持动态 Enabled 状态；
查询信号只在所属 TabBar 上连接，不在 TabView 增加默认菜单或覆盖优先级。

参考：GTK4 生态 libadwaita 的
[menu-model](https://gnome.pages.gitlab.gnome.org/libadwaita/doc/main/property.TabView.menu-model.html)
与 [setup-menu](https://gnome.pages.gitlab.gnome.org/libadwaita/doc/main/signal.TabView.setup-menu.html)
（文档 1.11.alpha），及
[1.8.0 TabBox 源码 do_popup](https://github.com/GNOME/libadwaita/blob/1.8.0/src/adw-tab-box.c)；
其“固定模型＋准备动作”适合 GAction。GOUI 的 MenuItem 直接持有 Go 闭包，按目标
查询模型可直接捕获页面，不需要共享的可变 contextPage，仍允许复用固定模型。
Qt [customContextMenuRequested](https://doc.qt.io/qt-6/qwidget.html#customContextMenuRequested)
（本轮文档 Qt 6.12）允许应用自行显示；GOUI 不采用该方式作为标签菜单的默认契约。
Avalonia [11.3.0 ContextMenu 源码](https://github.com/AvaloniaUI/Avalonia/blob/11.3.0/src/Avalonia.Controls/ContextMenu.cs)
展示控件绑定菜单及 Opening／卸载关闭机制；仅借鉴弹窗管理归属，不引入属性或路由事件体系。
libadwaita 将菜单模型放在 TabView 并非 GOUI 必须遵循的归属：GOUI 的独立 TabBar 是
交互入口，允许同一页面集合的不同标签栏声明不同菜单。SwiftUI 的
[集合上下文菜单](https://developer.apple.com/documentation/swiftui/view/contextmenu(forselectiontype:menu:primaryaction:))
（2026-10-01 官方文档）也向菜单构建闭包提供目标项标识；仅借鉴目标上下文显式传入，
不引入 SwiftUI 的选择集合或 ViewBuilder 系统。

验证入口仍为 `tests/window/widgets/tabs`，增加中文操作步骤；包内覆盖信号顺序／取消／
模型复用、多个 TabBar 共享 TabView 的菜单隔离、未选中标签和关闭按钮右键、空白、滚动坐标、焦点菜单、移交／卸载重入、
UI builder 更新／省略／断开及错误反馈。运行结果与平台缺口记录在 Plan/Roadmap。

TabBar 是一行水平标签。标准标签包含可选 16 DIP 图标、标题、可选
24 DIP 关闭按钮；最小高度 32 DIP。标题先按实际字体测量，水平方向留白
10 DIP、垂直方向 4 DIP；关闭按钮右侧另留 4 DIP。32 DIP 是下限，不作为文字的紧约束。字号变大时
标签栏和 HeaderBar 随行高增长。极长标题可以水平裁剪，关闭命中区保留。
关闭和滚动箭头使用固定几何符号，不随标题字号放大或裁剪。
相邻标签之间保留 4 DIP 真实间距，首尾不增加间距。测量、溢出、当前页揭示、
拖动目标槽位与换位阈值均计算该间距；间隙命中 viewport，不命中相邻标签。
HeaderBar 示例使用 6 DIP 内部 Padding，避开 Linux Integrated 的安全绘制边缘；
不移动 HeaderBar 本身，不让 TabBar 感知窗口装饰。
内容宽于可用空间时启用横向滚动和两侧按钮。当前页变化时揭示其标签，
用户主动滚动后不在每次布局重置滚动位置。

### 统一宽度与溢出阈值

`TabBar.TabWidthRange() (minWidth, maxWidth float32)` 查询配置；
`SetTabWidthRange(minWidth, maxWidth float32)` 设置 DIP 宽度范围，默认 112..240。
`widgets/ui.TabBar(...).TabWidthRange(112, 240)` 是薄绑定；省略声明恢复默认值。
不提供 ShrinkTabs 开关；相同上下限（如 240..240）表示固定宽度并直接滚动。
非正数、NaN 或 Inf 恢复对应默认值，max 小于 min 时提升到 min；
全栏有效 min 还需容纳每个标签的图标/关闭按钮/留白，max 至少为有效 min。

所有标签采用相同宽度，不再按标题长度或选择态分配。令 N 为数量、A 为栏宽、
G 为总间距 `(N-1)*4`：能容纳 `N*min+G` 时，宽度为 `min(max, (A-G)/N)`；
否则所有标签保持 min，显示左右滚动按钮，viewport 扣除按钮区域。
按钮宽度不反过来参与溢出阈值，防止临界值反复显隐。N=0 不参与除法。
极窄窗口的箭头各不超过栏宽一半，viewport 可为零；标签仍保持最小宽度，由
viewport 裁剪。当前标签比 viewport 宽时优先展示其左侧，不尝试同时满足两端可见。

空间充分时不超过 max，剩余区域留空。改变尺寸、范围或数量后重新揭示当前页，
压缩阶段 scroll 为零；用户滚动后的普通布局不强行重新揭示。
标题只在实际分配区域中裁剪，不省略、不改写字符串、不缩小字体；高度仍来自实际行高，
Snapshot 保留完整标题与实际边界。隐藏关闭按钮仍预留空间，不触发宽度变化。
范围或分配几何变化取消拖动/归位过渡；普通排序不改变数量，也不改变统一宽度。

Qt 6.11 的 [expanding](https://doc.qt.io/qt-6/qtabbar.html#expanding-prop)
和 libadwaita 1.11.alpha 的 [expand-tabs](https://gnome.pages.gitlab.gnome.org/libadwaita/doc/main/property.TabBar.expand-tabs.html)
文档描述填满剩余空间；GOUI 不直接照搬该开关，只采用已确认的统一宽度范围和滚动阈值。

标签主体使用 Click 与阈值 4 DIP 的 DragEventController 参与正常手势
竞争。关闭按钮是兄弟控件，点击关闭不选择也不开始拖动。拖动中的标签
保持按下时的抓取偏移，直接跟随指针水平移动，原 Widget 保持原父节点并
临时移到绘制顺序顶层，不重新挂载，也不创建图片代理。

标签的前缘越过相邻目标槽位中点时改变预览插入位置；相邻标签以 140ms
cubic ease-out 过渡让位。判定依据目标槽位，不能使用过渡中的实际位置，
以免静止指针导致来回换位。反向拖动从当前视觉位置开始新的过渡。
拖动过程不改变 TabView 的页面顺序；释放时提交一次 MovePage，并滑入
目标槽位。Esc 或在栏外释放时滑回原位。页面增删、外部重排、布局尺寸变化、
解绑或卸载立即清理拖动状态；信号回调可移除页面或解绑，不得继续使用旧关联。

边缘滚动与位置过渡复用栏内单个 GUI Timer，仅在需要时运行，按实际经过
时间推进；指针离开标签栏的垂直范围后停止边缘滚动。拖动结束后过渡完成
即停表，卸载立即停止并断开连接。该私有实现不引入公共动画系统或平台 DnD。

键盘在标签主体聚焦时支持左右、Home、End、Enter、Space，事件向 TabBar
冒泡处理；不隐式注册窗口级关闭或切页快捷键。焦点位于标签主体和按钮，
TabBar 与 viewport 本身不接受焦点；滚轮控制器只属于实际标签和按钮。
因此空白区的命中路径没有指针交互控制器，HeaderBar 的既有保守规则自然
将其识别为 Caption；标签、关闭和滚动按钮保持客户输入区。不添加特定控件
类型判断或覆盖 HeaderBar 的通用规则。

TabView、TabBar 与条目使用私有 LayoutManager，经 WidgetBase 的标准
测量与布局入口工作。`WidgetBase.MoveChildBefore/After` 只改变同父子节点顺序，不触发
Mount/Unmount。Snapshot 增加 TabView、TabBar、Tab、TabPanel 角色和
Selected 状态；真实输入仍经过 Window.DispatchEvent。过渡使用实际 Widget
矩形，绘制、命中与快照坐标一致。TabBar 快照保持实际从后到前的绘制顺序，
被拖动的标签暂时位于最后；TabView 的页面子树仍表达已提交的页面顺序。

## 样式与资源

标签使用 `tab-bar`、`tab-item`（`selected` / `dragging` part）、`tab-item-text`、
`tab-item-icon`、`tab-close-button`、`tab-scroll-button` 名称；选择态的标题、
图标分别使用 `-selected` 名称。子控件自己解析 StyleName，不继承父样式。
兜底规则由 `widgets/style` 显式提供，Modern 规则保留在主题入口。IconSource 可在页面间共享，
每个 Icon Widget 自己管理平台图片缓存。TabBar 卸载时断开页面通知，停止
边缘滚动计时器；页面由其根宿主最终销毁。

Modern 采用低对比度浮动圆角标签：6 DIP 圆角，普通态透明底面和正常文字/图标色；
悬停、按下使用淡中性色反馈。选中态使用正常文字/图标色、不透明中性底面和淡色
1 DIP 边框；暗色底面略亮于窗口。拖动态仍用不透明底面，仅增强中性边框，
不改变原选择态的文字颜色，不添加受 viewport 裁剪的外扩阴影。
选中不使用主题色描边、不加粗文字，不与键盘焦点语义混同。
未选中不是禁用态，不降低标题和图标的对比度。关闭按钮默认只有次要色叉号，
选中、整个标签悬停或标签子树包含键盘焦点时显示；其余时候隐藏。悬停/按下按钮
显示中性底色，滚动箭头使用相同反馈。所有颜色/圆角/边框由样式决定，不硬编码进控件绘制。

`tab-bar` 的 `separator` part 使用 ForegroundColor 绘制中性分隔竖线；透明色可关闭。
线宽 1 DIP、高 16 DIP（不超过栏高），位于真实 4 DIP 间隙中心并垂直居中。
仅相邻两个未选中且均未悬停的标签之间显示，无首尾分隔线；拖动和归位过渡期间
整栏隐藏。由已有 viewport 按实际标签矩形绘制，并受其结构裁剪，不增加 Widget、
命中区域或 Snapshot 节点。尺寸为私有几何常量，不为其扩展通用样式字段。

可关闭标签的主体固定预留 24+4 DIP 尾部空间，关闭按钮作为兄弟覆盖于主体上。
按钮隐藏不改变标题、标签和滚动范围；不可关闭标签则不预留该空间。
按钮在页面选中、标签悬停或关闭按钮自身子树持有焦点时显示。标签主体持有焦点
不强制显示：拖动未激活标签后仍保留主体焦点，但移开指针即隐藏关闭按钮，提交、
Esc 取消和栏外释放均如此。不通过清除焦点修饰外观。监听关闭按钮自身的焦点变化，
使焦点从关闭按钮回到同一标签主体时也能重新布局；标签整体 ContainsFocus 不变
不应阻止显隐更新。
隐藏按钮使用正常 Widget 可见性语义，不参与绘制或输入，Snapshot 的 Visible 为 false；
原关闭区域落到标签主体，点击选择、拖动排序。显隐在 Arrange 应用，不在 hover crossing
中立即改变命中，避免 PointerDown 先更新 hover 后重新命中时误关。主体按下期间
不显示原本隐藏的按钮，防止两次输入之间的布局改变正在进行的点击目标；焦点或悬停
变更只请求布局，复用标准事件/布局/绘制链路，不扩展公共 Widget 或 EventDispatcher。

参考 GTK4/libadwaita [AdwTabBar 的 inline 样式说明](https://gnome.pages.gitlab.gnome.org/libadwaita/doc/main/class.TabBar.html#style-classes)
（查阅版本 1.11.alpha）：去掉栏背景可适用于不同宿主。GOUI 采用栏背景透明的方向，
不引入 GTK CSS、平台主题依赖或新的标签显示模式；以上尺寸与配色是 GOUI 自身设计。

跨兄弟声明式关联使用通用 `BuildContext.AfterUpdate`：一轮 View 树协调、
应用 ID 修饰并完成 Window.SetWidget 后，按注册顺序运行一次；已释放或被
新一轮节点更新替代的回调跳过。回调只能完成正常绑定和 setter，不能保存
BuildContext 或同步启动嵌套协调。没有给 Root 增加 Tab 专用分支。

## 验证

包内测试按职责集中组织，不与实现文件一一对应：

- `widgets/tab_view_test.go`：页面管理、选择、顺序和关闭请求。
- `widgets/tab_bar_test.go`：布局、溢出、输入、排序、关闭按钮、分隔线、菜单与快照。
- `widgets/tab_transfer_test.go`：页面移交、拖放占位及拖出请求的生命周期。
- `widgets/tab_bar_linux_test.go`：不依赖桌面的 Pango 字体测量，保留平台限定和线程亲和。
- `widgets/test_helpers_test.go`：多文件共用的固定布局和正常输入派发替身；单文件专用 helper 就近保留。
- `widgets/ui/tabs_test.go`：声明协调、身份、移交、菜单构建与信号清理。
- `widgets/style/style_test.go`：兜底样式的显式组合。

合并只改变文件归属及分组，不改变测试名称、输入、断言、容差或跳过条件。
真实窗口验证仍放在 `tests/window/widgets/`，不合入包内测试。

包内测试覆盖页面选择、删除退避、关闭请求、真实子节点重排、溢出布局、
输入分发下的拖动提交/取消、跟手位置、让位与反向过渡、空白命中路径、
Snapshot 和 AfterUpdate 时序。Linux 下使用不依赖桌面的真实 Pango 度量
验证 14pt/24pt 标题的完整行高与窄栏关闭区域，不能用缺少 Typography 时
返回零尺寸的 Label 代替文字布局验收。依赖桌面的点击、关闭、拖动取消、窄窗口、
HeaderBar 和缩放验收见 `tests/window/widgets/tabs` 的入口注释。执行
`go test ./...`；本机窗口操作和后端、缩放结果分别记录。

当前实现不涵盖跨窗口转移和拖出新窗口，后续按 [转移设计](DesignTabTransfer.md)
实施，不再将其视为未规划方向。固定/分组标签、页面懒加载、自定义标签头和垂直
标签栏仍不在本次范围。

实现参考：Qt 6.8.0 的 [QTabBar](https://github.com/qt/qtbase/blob/v6.8.0/src/widgets/widgets/qtabbar.cpp#L1951)
包含拖动跟随、相邻标签滑动和释放归位处理。GOUI 采用这一交互方式，但使用
原 Widget 的位置与绘制顺序，并保留释放时一次提交的页面模型契约。
