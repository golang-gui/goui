# MenuBar 与 macOS 原生菜单栏

> 状态：2026-11-24 已实现首版桌面 MenuBar；原生操作和验证边界见 [Plan](Plan.md#menu-bar)。
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

## 2. 公共接口

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

链式 Submenu 的互斥、空声明及递归协调见子菜单设计；MenuItemView.Visible(bool) 同步条目可见性。
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
在 ui.ComposeView 中条件删除该节点。GUI 使用私有的窗口呈现参与标记，统一
检查布局、绘制、焦点、事件和快照，不将 MenuBar 特例散入每种布局或 UI Root。

MenuBarView 使用普通 WidgetView 和既有菜单描述协调。模式改变只替换呈现，不
重建业务模型和命令连接。模式／模型切换先结束旧展开链，完成新投影后提交呈现，
在正常布局和首帧前确定结果；原生菜单同步不经过 Painter 或 SharedPopover。

## 4. 原生窗口能力与资源归属

平台使用原生 Window 的可选能力，与 WindowManagement 的获取方式一致：

```go
type WindowMenuBar interface {
    Window
    SetMenu(items []MenuItem) error
}
```

- SetMenu 复制整个值描述；空切片撤销该窗口的业务菜单，基本应用菜单保留。
- 能力与 Window 是同一对象，没有单独的创建工厂、事件 handler 或 Destroy。
  Window 销毁时兜底释放本窗口投影，Platform 释放应用级原生资源。
- 失败保留此前登记的描述。GUI 决定回到窗口内呈现时显式撤销旧登记，并一次性报告
  本次更新及撤销的错误，不在重绘时重试。
- GUI 只登记各窗口的业务菜单。AppKit 主菜单选择、主窗口与附属窗口关系、原生跟踪
  和资源切换留在 Cocoa；不公开 MainWindowEvent，也不让 GUI 再维护应用级来源表。
- 原生对象不持有 gui.MenuItem、Command 或 Widget。GUI 不管理 NSMenu 的创建、安装
  或销毁，不需要公开的菜单投影、Source 和 Revision。

每个 GUI Window 最多登记一个可见 Auto MenuBar。重复登记报告一次配置错误，保留
原有效登记；原条隐藏、卸载或改为 InWindow 后，窗口内的候选可以接替。该协调只访问
所属窗口的 MenuBar，不扫描整个应用，也不在普通 RequestLayout、布局或 Painter 中同步。

MenuBar 订阅模型、命令、快捷键和逻辑上下文。祖先显隐、启用、重挂载和模态边界
通过 GUI 内部的通用上下文通知发布；观察者比较自身有效状态，只在实际变化时排队。
挂载时先确定呈现，再参与首次布局。用户可见配置和窗口内呈现参与是两个独立状态。
直接作为窗口根时同样不分配窗口内容矩形。

Cocoa 私有保存各 Window 的投影，根据 AppKit main-window 变化安装对应菜单；没有
业务菜单的普通窗口使用基本应用菜单。Popup 不成为主窗口，不抢占文档菜单。
输入焦点变化不等同于主窗口切换，应用失活不销毁窗口投影。模态限制由 GUI 计算为
菜单项可用性，激活时再校验，不由平台解释 Widget 树。

## 5. 值描述、更新与动作有效性

MenuItem 描述包含非零 ID、纯文本标题、启用、分隔线、勾选、子菜单和逻辑快捷键。
隐藏项由 GUI 省略。ID 在同一个窗口中唯一，标识动作绑定而不是数组下标。

原生选择经过窗口原有 handler，形成 `MenuEvent{Item: id}`，再进入
`Window.DispatchEvent`。平台仅报告已选条目；GUI 校验登记仍存在、宿主存活、模型成员、
祖先可见与可用、Command 和模态边界，然后调用既有激活入口。

- 标题、勾选等显示更新保留绑定 ID；同步尚未完成也不取消有效的已选动作。
- 替换 Action／Command／子模型、移除条目、禁用后重新启用，需要新的绑定 ID。
  父路径失效也使后代绑定失效。
- 隐藏、模式切换、卸载及重新登记不复用旧 ID；分配器归所属窗口，不因 MenuBar
  被销毁或替换而从头分配。失效记录及时删除，不保存无限历史。
- GUI 更新的内部序号只用于防止同步回调重入，不作为原生事件的公开参数。
- Cocoa 的跟踪与窗口切换代次属于私有实现。窗口来源切走再切回，也不能交付此前
  排队的选择。普通菜单关闭不递增这个代次，避免丢弃已经完成的选择。
- UI 的同位置动作仍读取最新声明式闭包，遵守既有协调契约。

NSMenu／NSMenuItem 全部在 AppKit 主线程管理。标题、勾选、可用性等状态原位更新；
结构变化先建立完整副本，跟踪结束后在安全边界提交。每个窗口至多保存一个待提交
快照，后续更新取代旧待提交值；不使用固定延迟推断跟踪结束。释放先断开 delegate
和 action target，再释放菜单，回调后检查生命周期。

自定义业务项明确设置 Enabled，不依赖 AppKit 根据 selector 自动启用。
叶子选择在跟踪结束后交付，可安全修改树、切换窗口或退出。系统菜单自动化使用
原生正常输入，不提供绕过 Window.DispatchEvent 的语义动作捷径。

原生菜单描述使用 common.Key／Modifiers；各平台输入实现和 GUI 继续经 events 的
同类型 alias 使用按键，不另造字符串按键和布尔修饰键。GUI 只解析当前窗口已登记的
Command Shortcut，遵守绑定顺序和实际启用；字符等价键、AppKit
修饰位和可表示范围由 Cocoa 转换。保留 Command／Option 等平台事实，不合并 Alt、
Super 或 Win。支持基础字符、导航键、删除键和 F1–F24；不能精确表达的组合不登记原生等价键，
原有 GUI 按键仍可处理。不承诺任意 Unicode、数字键盘或任意布局的原生等价键。

AppKit 的 menuHasKeyEquivalent: 回调只对已登记业务项的匹配键执行交接：按普通
keyDown 路径进入输入法或窗口 KeyEvent，再经 Window.DispatchEvent 和原有控制器
处理。该次等价键不再产生 MenuEvent；焦点控件优先级、Shortcut.ConnectActivate、
AutoRepeat 和原生文字提交保留同一路径。未处理的键也不转换成第二次菜单动作，
避免过期投影或控件保留文字后意外执行命令。平台直接使用者同样通过 KeyEvent 处理等价键。
鼠标选择和菜单内的键盘导航确认仍在跟踪结束后报告 MenuEvent。

原生按键回调可能同步更新菜单或销毁窗口；回调期间保留接收者，并将结构提交与
释放延至回调和菜单跟踪结束。标准应用菜单的快捷键继续由 AppKit 处理。

## 6. 应用退出与基本系统菜单

基本应用菜单归 Cocoa 的应用接入，独立于 gui.MenuBar 是否存在。包含 Hide、
Hide Others、Show All 和 Quit；.app 使用 bundle 显示名，裸程序使用进程名。
基本应用菜单保留 AppKit 自动可用性校验，业务项才由 GUI 提供 Enabled。
标准应用动作和菜单委托 selector 在 frameworks/appkit 初始化，Cocoa 只实现适配行为。
InWindow 只选择业务菜单的呈现位置，不删除基本应用菜单。About／Settings 等角色
及系统应用菜单本地化暂不增加，不按业务标题猜测角色。

正常应用退出直接复用既有事件循环和 GUI 收尾，不新增 QuitEvent、ConnectQuitRequest
或生命周期服务。Platform.NewEventLoop 保持无参数，其他平台和普通调用点不承载
macOS 退出通知。

Cocoa 的应用委托和基本菜单在本平台应用初始化时创建，归 Platform 所有；
NewEventLoop 只建立事件循环。标准 terminate:、Command+Q 和 Dock Quit 都进入
applicationShouldTerminate:；菜单仍在跟踪时安全结束跟踪，之后停止原生循环。
返回 NSTerminateCancel 仅防止 AppKit 直接 exit()，让 Run 正常返回。

GUI 已有的 Application.Run 返回路径执行幂等 shutdown，与显式 Quit 共用清理。
Shutdown 通知前停止定时器与设置监听，之后清理窗口、控件与服务；不提供否决，
没有窗口或业务 MenuBar 时也按相同流程退出。窗口 CloseRequest 保留独立含义。

本轮不改变 OpenURL／OpenPath 的接口与归属。

## 7. 错误、语义与实现边界

不支持 WindowMenuBar 时按 Auto 契约使用窗口内呈现，是正常能力选择。
创建、更新或登记失败由 ConnectError／OnError 报告一次，保留可用的窗口内入口。
失败后不逐帧重试，后续有效模型、模式或上下文变化可以重新请求。
错误回调可能销毁宿主、替换模型或模式，必须按当前生命周期和内部更新序号复核。

MenuBar 提供 menubar 角色及 MenuBarInfoKey／MenuBarInfo 的 Mode、Native。
Native 表示已登记为本窗口的原生菜单来源，不表示应用当前前台窗口。原生模式
窗口内 Bounds 为空，没有虚假的内部按钮、点击动作或系统菜单屏幕坐标。
原生可访问性由 AppKit 提供；窗口内按钮继续报告实际边界、可用性和展开状态。

| 归属 | 职责 |
| --- | --- |
| platform/common、platform/events、platform | WindowMenuBar、独立描述、窗口 MenuEvent |
| platform/darwin/frameworks/appkit/appkit_darwin.go | NSMenu／NSMenuItem、NSApplication 与委托绑定 |
| platform/darwin/cocoa | 原生对象、主窗口菜单选择、跟踪安全、正常应用退出接入 |
| gui/application.go | 显式退出和原生循环正常结束后的统一清理 |
| gui/MenuBar 与 Window | 窗口内协调、模型观察、值投影和动作有效性 |
| gui 通用 Widget／Root 入口 | 窗口呈现参与、有效上下文通知，无菜单类型分支 |
| ui/menu.go | 既有 WidgetView、递归模型协调和信号清理 |
| theme/basic、theme/modern | 仅窗口内外观 |

不修改绘图后端，不新增应用级 Popup 管理器，不将命令解析放入平台或 UI 协调器。

## 8. 验证

逻辑回归覆盖零占位与直接窗口根、挂载／卸载、祖先显隐／启用、模态、重复登记、
失败与回调重入、各窗口 ID 隔离、命令及快捷键更新、无关布局不触发原生同步。
动作回归同时覆盖显示变化仍执行一次，以及替换、移除再插入、禁用再启用、模式
往返和窗口销毁时拒绝旧动作；包含未完成同步的批处理窗口。

应用测试验证显式 Quit 和原生循环返回两条路径，有／无窗口时均在 Shutdown 前
停止定时器及设置监听、拒绝新任务，并在 UI 线程仅清理一次。

原生窗口入口独立验收 WindowMenuBar.SetMenu、相同 ID 在两个窗口的路由、显示
更新、禁用、非法描述不替换原菜单，以及无窗口时原生退出后正常返回。
GUI／UI 入口覆盖 macOS Auto／InWindow、两窗口、无菜单窗口、Popup、语言、
跟踪期间更新与正常退出；Linux 验证窗口内交互与亮暗主题。
全仓 go test、必要 race、跨平台编译和桌面操作分别记录，不用编译代替原生验收。
实际结果和验证缺口见 [Plan](Plan.md#menu-bar)。

## 9. 参考与取舍

2026-11-24 复核：

- [Qt 6.8 QMenuBar.nativeMenuBar](https://doc.qt.io/qt-6.8/qmenubar.html#nativeMenuBar-prop)
  保留 QWidget，原生和窗口内呈现共用声明身份。采用该职责划分，不拆分 GUI Widget。
- [Qt 6.8.3 Cocoa 菜单栏](https://github.com/qt/qtbase/blob/v6.8.3/src/plugins/platforms/cocoa/qcocoamenubar.mm)
  在 Cocoa 内部处理窗口关联和菜单切换，排除临时 Popup；其使用 Qt focusWindow
  的具体策略不直接照搬，GOUI 使用 AppKit main-window 通知。
- [Qt 6.8.3 应用委托](https://github.com/qt/qtbase/blob/v6.8.3/src/plugins/platforms/cocoa/qcocoaapplicationdelegate.mm)
  applicationShouldTerminate 即使允许退出也返回 NSTerminateCancel，使框架事件循环
  正常返回 main。仅采用正常清理的控制流，不复制其可取消会话管理体系。
- [Flutter 3.38 PlatformMenuBar](https://github.com/flutter/flutter/blob/3.38.0/packages/flutter/lib/src/widgets/platform_menu_bar.dart)
  保留构建生命周期并投影菜单模型；其 macOS 默认插件仅使用窗口 0，不作为多窗口归属证据。
- [GTK 4.18.6 Quartz 应用接入](https://github.com/GNOME/gtk/blob/4.18.6/gtk/gtkapplication-quartz.c)
  区分应用菜单及活动窗口动作。借鉴归属划分，不照搬 GAction／GMenuModel 对象系统。
- Apple [菜单归属](https://developer.apple.com/library/archive/documentation/Cocoa/Conceptual/MenuList/Articles/HowMenusWork.html)、
  [启用规则](https://developer.apple.com/library/archive/documentation/Cocoa/Conceptual/MenuList/Articles/EnablingMenuItems.html)
  说明 NSApplication 主菜单及自动验证行为。任意 Widget 转换不在 GOUI 范围内，
  并不表示 AppKit 完全禁止自定义 NSView 菜单项。
