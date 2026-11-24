# 级联子菜单

> 2026-11-23：窗口内级联已实现。验证结果及未验收组合见 [Plan](Plan.md#级联子菜单2026-11-23)。
> [MenuBar／macOS 原生菜单栏](DesignMenuBar.md) 保留为独立草案，本轮不接入系统菜单。

## 1. 范围与分层

菜单模型描述树形数据，PopoverMenu 管理一条呈现路径。每级使用 SharedPopover，
实际普通 Popover 继续承担 Root、原生 Popup、Painter 和内容布局的职责。

```text
Window W
  共享槽 → 普通 Popover P0：菜单 A
             共享槽 → 普通 Popover P1：子菜单 B
                        共享槽 → 普通 Popover P2：孙菜单 C
```

窗口和普通 Popover 各自延迟持有一个子槽；兄弟分支抢占同层槽，父子层可以同时显示。
槽归实际呈现宿主，不归 MenuModel、MenuItem 或某个虚拟行。SharedPopover 是轻量客户端，
不实现 Root。逻辑父子控制器属于 PopoverMenu，不属于原生资源缓存。

最近呈现宿主与原生所有者分别解析：前者决定借哪个共享槽，后者仍由 anchorSurface
沿锚点链找到窗口。所有层的原生 Popup 都可以归同一窗口所有；不让 Popover 实现
完整公共 Surface，不改变 WorkAreaAt 的坐标约定，不新增平台接口或 Painter 策略。

| 对象 | 职责 |
| --- | --- |
| platform.Popup | 原生表面、尺寸及输入事实 |
| Window／surfaceBase | 正常输入入口、唯一模态登记、通用外部关闭策略 |
| 普通 Popover | Root、绘制资源、生命周期及通用定位 |
| 共享槽／SharedPopover | 每宿主缓存、客户端抢占及隔离 |
| PopoverMenu | 父子关系、展开项、键盘活动层、延迟任务及整链关闭 |
| menuContent／menuItemRow | 测量、虚拟化、正常事件控制器及样式投影 |
| Menu／MenuItem | 数据和通知，不保存任何呈现状态 |
| ui | 递归协调模型、命令及最新回调 |

不增加应用级菜单池、模态栈或另一套事件分发器。本轮不包含菜单栏、助记键、
搜索、富控件菜单项、菜单项 Tooltip 或移动端导航菜单。

## 2. 模型与公共接口

```go
func (*MenuItem) Submenu() MenuModel
func (*MenuItem) SetSubmenu(MenuModel)
func (*Menu) AppendSubmenu(label string, submenu MenuModel) *MenuItem
func (*PopoverMenu) Destroy()
func (*PopoverMenu) ConnectError(func(error)) signal.Handle
```

Action、Command、非 nil Submenu 互斥，最后一次来源 setter 生效。SetSubmenu(nil)
清除来源，留下无动作叶项，不恢复旧 Action 或 Command。分隔项始终不可交互。
Menu 和 MenuItem 的修改在 GUI 线程进行，同步通知呈现者。

空、全部隐藏或只有分隔项的子模型不能展开，也不退回执行旧 Action；全部禁用的
非空子模型允许展开供查看，但没有可选项。Enabled 保留调用方设置，命令与祖先的
禁用状态只影响实际可用性，不改写局部属性。

Menu 维护条目成员订阅，覆盖 Append、Insert、Set、Remove、SetItems 和 Modify。
同一 MenuItem 可属于多个 Menu，移出后解除对应通知。纯 Action 闭包刷新不重建行；
改变来源、标签、启用、可见性等展示数据仍通知模型。

模型可以共享为 DAG。呈现者仅沿当前祖先条目路径检测循环，不对任意 MenuModel
接口做无条件比较，因此切片等非可比较实现也可使用。循环展开作为一次失败报告，
不损坏已有父菜单；不存在全局“某模型已经展开”的状态。

## 3. 资源生命周期

- Hide 先结束后代显示、定时任务和订阅，再关闭本层。各层普通 Popover、Painter、
  原生 Popup 保留供复用，不在根关闭时销毁重建。
- 同层切换最多保留一份内容树。旧客户端被逐出时解除内容、锚点和输入协作，
  不保留旧模型的活动订阅。隐藏菜单也解除列表、命令和直接子模型订阅，重开时重连。
- 客户端 Destroy 释放逻辑控制器和内容关联，不销毁宿主缓存。实际宿主 Destroy
  递归释放缓存子树；窗口对仍归属它的普通弹层及其子缓存最终兜底。
- 跨原生所有者重新呈现时释放旧所有者资源。短暂绘制目标失效不等于逻辑宿主销毁。
- 缓存规模取决于曾经使用的最大级联深度，与访问过多少兄弟条目无关。

关闭、内容替换、模型通知和用户回调都可能重入。槽 revision、显示 epoch、源条目
revision 及宿主存活检查共同防止旧清理关闭新显示、旧展开提交到新行、旧动作执行。
父内容替换必须保留关闭回调中已经提交的新内容；子客户端配置期间销毁窗口须等待
相关宿主退出资源转换，再按既有帧／原生释放顺序处理。

## 4. 交互状态与模型更新

每个 PopoverMenu 保存父指针、一个可复用的逻辑子呈现者、expanded 条目、键盘是否
位于子层、显示 epoch 和延迟任务。活动项、展开项及键盘活动层分别负责即时选择、
呈现路径和按键路由；延迟任务不得把活动项改回展开项。指针在某层真正移动或按下时，
该层按实际命中选择，其他层清除活动项，展开路径保留较弱提示。

结构或展示模型变更取消待执行任务，关闭依赖旧行的后代并重新测量。有效当前条目
尽量保留，失效条目清除。呈现者只观察本层及直接子模型的可展开性，不提前构建未
显示的所有后代。行回收、解绑、移出有效视口或部分不可见时关闭该分支。

样式状态刷新只使测量／绘制失效，不冒充模型变化。正常 hover 绘制不能取消待展开
任务。父层完成布局后检查锚点有效性，再请求存活子层更新定位。

## 5. 按键路由与活动层

模态根（独立／上下文菜单默认）是窗口唯一 ModalTarget。MenuButton 的根仍为
非模态，沿用获得焦点的按钮显式转发。子级均为非模态，不登记新的模态目标，
不获取原生焦点。

根菜单验证所属窗口、当前模态授权、显示代次及可用祖先后，选择键盘活动层，
通过该层正常 EventDispatcher 和 EventController 处理。公共 Popover.DispatchEvent
继续拒绝不拥有窗口模态输入的子弹层，不为子菜单放宽公共所有权检查。

| 按键 | 行为 |
| --- | --- |
| Up／Down | 当前层循环选择可用项，并滚动显示 |
| Home／End | 当前层首个／末个可用项 |
| Right | 有分支时展开、进入子层并选首个可用项 |
| Left | 关闭当前子层和后代，返回父层；根未处理时交给 MenuButtonGroup |
| Enter／Space | 分支展开并进入，叶项激活 |
| Escape | 关闭当前键盘活动层和后代；根结束整链 |
| Tab／Shift+Tab | 结束整链；模态根消费，非模态按钮继续原焦点导航 |

导航允许重复；确认、展开和关闭不因按键自动重复连发。物理弹出方向不改变 Right
进入、Left 返回的逻辑。通用 Popover 的 Escape 先交内容，未处理且原显示仍有效时
才执行 AutoDismiss 默认行为；普通 Popover／DropDown 仍保留默认 Esc 关闭。

## 6. 指针与跨层命中

点击分支立即展开；重复点击已展开的分支保持打开。顶层 MenuButton 仍按原来的
切换规则关闭自己。首次悬停展开延迟 250 ms，在同一行中继续移动不重启计时。
移到其他可用项立即改变活动项；禁用项、分隔线和留白立即清空活动项。
移到另一个分支时保留旧子层，等待 250 ms 后替换；移到普通项、禁用项或本层留白，
等待 250 ms 后关闭旧子层。候选行变化重新确认意图，同一候选中的移动不延长期限。
按下、点击及键盘导航继续即时处理，不等待悬停任务。
进入后代取消祖先待切换任务，静止坐标及重复 crossing 不用来触发展开。

仅在指针从当前展开的源行朝已显示子层移动时，按子层最终主体矩形和轨迹判断保护三角形。
跨行斜移最多保护 500 ms，期间活动项仍跟随实际命中；改变候选行不得延长同一次保护的截止时间。
进入子层或回到源行结束该次保护。明确远离子层后恢复普通宽限，但不超过原保护截止时间。
保护与 GUI Timer 使用同一时钟；不能用键盘展开前停留在其他位置的旧坐标启动保护。
任务使用 GUI Timer 和 epoch 取消；关闭、替换、解绑后，已经排队的旧任务也不能展开。

整链以主体（含圆角）命中，不把阴影当菜单内容。前层阴影覆盖祖先时，事件转给
实际命中的祖先主体；已经接受的捕获手势优先处理 move／up。转发只换算窗口局部 DIP，
随后仍进入目标的普通分发／控制器路径，不直接调用叶项动作。

Enter 与 Move 使用相同的主体命中。原生接收者与逻辑活动层可以不同：指针进入子层
原生表面的阴影而仍命中父项时，父项继续 Hover；离开父行或进入子层主体时，父项
只保留 expanded 提示。切换逻辑活动层时清除其他层的悬停路径，快照中的 Hover 与选择保持一致。
祖先即使已在逻辑悬停路径中，新的原生 Enter 也须按当前位置更新活动项。

根呈现者保存最近提供指针事件的原生接收者；转发不改写这个来源。另一接收者的
Enter／Move 已到达后，旧表面的 Leave 不得清除当前逻辑活动项。原生 Leave 的坐标
可能只是最后一次已知位置，不能据此推断全局去向或重新选择条目。
当前原生接收者离开时清除整链活动项、悬停路径和待执行任务，保留已展开路径。
来源随该层结束显示而失效；跨层 crossing 回调若重开或替换菜单，旧交接停止处理。
根菜单通过普通 Popover 的私有观察入口接收宿主新发生的 Move／Enter，确认移到链外时
清除整链活动项、指针基线及悬停任务，保留所有已展开层，允许指针移回继续选择。
到期任务即使已进入投递队列也不得改变离开后保留的路径。外部按下、Esc、激活、失活或
所属内容失效仍按既有规则关闭；离开不是新的关闭条件。
该入口只观察窗口局部 DIP，不重放输入、不知道菜单类型、
不新增全局监听。静止坐标的重复 Move 不改变键盘展开状态，已接受的捕获优先。
Left／Esc 返回父层时恢复源项选择，避免鼠标跨表面离开后丢失键盘导航位置。

外部按下：模态根关闭并消费；非模态根关闭后允许原目标处理。关闭回调销毁窗口、
重开菜单或替换输入上下文时，旧事件不得操作新界面。无需全局鼠标监听器。

## 7. 侧向定位与视口

普通 Popover 的内部定位求解器支持侧向策略；本轮不新增公共屏幕坐标或菜单专用
平台查询。子层主体默认覆盖父主体边缘 4 DIP，向左、向右对称；极窄主体限制重叠量
不超过父主体宽度的一半。该值属于菜单的内部布局策略，通过菜单锚点区域实现，
不修改通用 Popover 的侧向策略，也不把阴影当作重叠量。
垂直仍让子层首行对齐源行，阴影和内容 padding 各计算一次。子层自然尺寸不参与父菜单测量。

首层优先向右，后代优先延续父层实际方向。空间不足时翻转，再选择较少溢出的方向并
滑入工作区；尺寸受工作区约束，长菜单继续滚动。所有计算使用原生所有者局部 DIP、
父层已求解位置和实际采用尺寸。Position 是请求，requestedPosition 是最后提交的值，
不声称它是原生观测位置。

锚点使用本次绑定的虚拟行。布局和滚动导致源行部分／全部离开 viewport 后关闭分支，
不能把同一行外壳回收后误认为原条目仍有效。

## 8. 激活、错误与重入

激活先捕获条目、Action／Command、来源 revision、祖先条目及所属窗口；整链关闭后
重新验证，再执行一次。正常关闭会卸载源行，因此不以源行仍挂载为执行条件。
关闭回调删除、替换、禁用条目，重开菜单或销毁窗口时，旧激活终止。Command.Checked
仍由业务显式提交，不由菜单暗中切换。

2026-11-24：关闭与动作在同一输入调用栈内同步完成，关闭先释放模态输入并执行关闭
回调，然后校验并调用动作。TreeView／TabBar／TableView 的关闭回调会用 SetMenu(nil)
释放临时模型；这些清理及重复 Hide 不取消已经选中的动作。

布局、模型观察及悬停任务继续使用既有 epoch。动作另由根呈现者的 activationRevision
识别新来源：安装非 nil 模型（包括同一模型重装）、请求重新显示、销毁任一级呈现者，
都使旧动作失效；清空模型及普通隐藏不改变该代次。不能要求关闭后 epoch 恰好增加一次，
也不比较任意 MenuModel 接口值。

关闭前短暂保存叶项及祖先的原始模型，校验成员关系时使用这些模型；呈现者已清空模型
不等于原始模型移除了条目。原模型中真正删除条目、条目／命令 revision 改变、祖先
不可用、根仍可见、宿主销毁，均阻止旧动作。所属窗口的已有显示计数用于识别关闭回调
中新显示的其他弹层，即使该弹层随后隐藏也不执行旧动作。所有捕获只持续到本次同步
处理结束，不新增公共 API，不要求上层控件延长临时模型或闭包的生命周期。

参考 [Qt 6.8.3 QMenuPrivate::activateAction](https://github.com/qt/qtbase/blob/v6.8.3/src/widgets/widgets/qmenu.cpp#L1443)：
源码在隐藏前保存激活路径，隐藏后分发动作。GOUI 沿用先结束呈现再执行的顺序，
使用自身模型及资源生命周期校验，不照搬 Qt 对象／信号体系；此依据不是原生平台实测。

子层 Show 失败保持父层可用，不提交 expanded 状态，不无限重试。
同步 ShowAt／MenuButton.Open 只返回错误；输入中的失败仅在根报告：独立菜单使用
ConnectError，MenuButton 复用 ConnectOpenError／ui.OnOpenError，不并行新增第二套错误信号。
透明创建只沿用 Unsupported／Unavailable 的既有不透明降级，不重试布局或查询失败。

<a id="submenu-ui"></a>

## 9. 声明式入口

```go
ui.MenuButton("File").Menu(
    ui.MenuItem("Open", openFile),
    ui.MenuItem("Recent").Submenu(
        ui.MenuItem("Project A", openProjectA),
        ui.MenuItem("More").Submenu(
            ui.MenuItem("Browse", browseProjects),
        ),
    ),
    ui.MenuSeparator(),
    ui.MenuItem("Save").Command("file.save"),
)
```

MenuItemView 是普通描述值，不是 Widget。Submenu()／Submenu(nil) 显式表示空分支，
清除旧动作／命令；不额外提供 ui.Submenu 构造函数，也不在每个分支创建 CommandScope。
循环声明是编程错误，在转换／协调时拒绝；共享声明分支允许组成 DAG。

MenuButton、Menu 与 MenuFor 递归生成模型。结构签名记录层次、分支／叶项身份、
标签、Enabled、分隔及命令来源，不比较函数。结构不变时保留各级模型，动作闭包按
索引路径读取最新声明；CommandRef 对象替换独立于展示签名，重新关联即可。
DefineCommand 递归收集到宿主现有范围，遵循同名冲突及生命周期规则；临时模型仍须
使用 Command／CommandRef。MenuFor 的各级模型在被观察时连接已有命令范围。
结构变化允许重建受影响模型、关闭无效分支，不为菜单向 Root／BuildContext 加特例。

## 10. 外观与语义

Basic 和 Modern 使用既有 menu／menu-item 外观，新增 menu-item 的 submenu Part
绘制矢量方向箭头；尾列独立预留，与勾选、标题及快捷键列共存。Enabled 与 expanded
反馈不依赖字体符号。主要高亮只表示活动项，沿用 Selected 到 Hovered 的样式映射；
展开项不伪造 Hovered／Selected。非活动的展开父项使用 menu-item 的 `expanded` Part，
Basic 使用较浅的同色背景，Modern 使用较弱的中性背景；活动项优先使用普通高亮。
应用可独立覆盖该 Part 的颜色、边框和圆角。Part 只改变行背景，不改变文字、箭头、
测量、命中或语义；一次只绘制一种背景，不叠加两次透明覆盖。

MenuItemInfo 增加 HasSubmenu、Expanded；实际 Enabled 包含祖先状态。各级 Popup
仍是独立语义宿主，不伪装成窗口 Widget 子树，不新增直接执行菜单项的语义捷径。

## 11. 验证与参考

回归集中在既有 gui/menu_test.go、Popover／共享 Popover／定位主题测试，以及
ui/menu_test.go。覆盖来源互斥、非可比较模型、循环、递归成员通知、整链激活、
关闭回调重入、悬停计时、模型更新、失败保留父层、语义、缓存复用与释放、命令和最新声明。
窗口场景扩展 tests/window/gui/menu_group，保留既有用例并补三级键盘与鼠标操作。
各平台／后端的实际结果及缺口单列于 Plan，不以编译或正常派发替代原生验收。

2026-11-23 参考的边界：

- [GTK4 PopoverMenuFlags](https://docs.gtk.org/gtk4/flags.PopoverMenuFlags.html)，
  页面版本 4.24.1、Nested 自 4.0：官方承诺可选择传统嵌套 popover 呈现。
  GOUI 采用独立呈现层，不引入 GTK 的 focus／action 对象体系。
- [Flutter SubmenuButton](https://api.flutter.dev/flutter/material/SubmenuButton-class.html)：
  官方 API 描述级联菜单的展开和键盘交互。GOUI 保留现有模型／控制器分层。
- [Qt 6.8.3 qmenu.cpp](https://github.com/qt/qtbase/blob/v6.8.3/src/widgets/widgets/qmenu.cpp#L3229)：
  源码含延迟及 sloppy 移动保护，支持“跨行斜移应有有限保护”的取舍。
  GOUI 使用自己明确的计时和坐标契约，不声称与 Qt 原生行为逐项相同。

- [Qt 6 QStyle.PM_SubMenuOverlap](https://doc.qt.io/qt-6/qstyle.html#PixelMetric-enum)：
  官方 API 明确父子菜单的水平重叠；Qt 6.8.3 的上述 qmenu.cpp 使用该度量。
  GOUI 采用主体轻微重叠的原则，4 DIP 是本框架的内部布局取值，不照搬 Qt 样式体系。

2026-11-24 补充核对：Qt 6.8 系列官方 [QStyle 文档](https://doc.qt.io/qt-6.8/qstyle.html#StyleHint-enum)分别定义
SH_Menu_SubMenuSloppySelectOtherActions、SH_Menu_SubMenuSloppyCloseTimeout 和
SH_Menu_SubMenuDontStartSloppyOnLeave；上述 Qt 6.8.3 源码的 setCurrentAction 将活动项
更新与子菜单关闭计时分开。Flutter 3.38.0 的 Material
[SubmenuButton 源码](https://github.com/flutter/flutter/blob/3.38.0/packages/flutter/lib/src/material/menu_anchor.dart#L1812)
在 PointerExit 中清除悬停记录，不直接关闭子菜单。这些分别是官方可配置契约与固定版本
自绘菜单源码行为，不代表各平台原生菜单一致。GOUI 采用反馈即时、分支变化容错、链外
保留路径的组合；250／500 ms 是本框架的内部策略，不增加平台查询或公共行为选项。

每宿主一级共享缓存是 GOUI 的资源归属设计，不归因于这些框架的公共承诺。
