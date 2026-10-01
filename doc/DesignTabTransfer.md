# TabView 页面转移与拖出新窗口

> 状态：主要功能已完成并通过三平台主要路径验收：命令式及 UI 跨 TabView/跨窗口
> 转移、拖出新窗口、取消/拒绝/准备失败保留源页面、转移后重建及继续编辑。
> 按用户确认的验证范围，不再以穷举边缘组合为交付门槛；真实混合 DPI 等仍是未验证
> 的后续补充项，不标记为通过。X11 迟到消息修复及本轮交付说明见第 26 节。
> 本文保存跨 TabView、跨窗口与拖出新窗口的
> 最终设计方向，替代早期需要独立页面所有者、窗口放置对象或公共桌面坐标的方案。
> 已实现的前置能力包括窗口定位/位置观察、DnD 结束位置及成功提交保护，以及既有
> DnD、RenderWidget、栏内排序等。2026-09-30 的结果契约调整见第 11 节，取代
> 第 10 节原先要求精确区分所有原生结束原因的实施门槛。
> 第 13–25 节保留阶段记录，其“尚未完成”描述以当时为准；当前进度见 [Plan](Plan.md)。
> 当前交互修订见第 28 节：推开式目标占位、隐藏源标签，以及单标签仍可转入已有窗口。
> 2026-10-01 当前 UI 身份／移交契约见第 29 节：页面是普通节点，ID 只在挂载目标内
> 匹配，不再使用独立 key、页面双向映射或任意 State 查询。第 7、15 节的旧实现描述
> 仅作阶段记录，其接口、身份规则与当前实现冲突时以第 29 节为准。

## 1. 范围与核心决定

支持三种操作：同窗口不同 TabView 之间移动、不同窗口 TabView 之间移动、
将标签拖出且没有完成投递时，按应用策略请求创建新窗口。原有同一 TabView 栏内排序保持不变。
只支持同一应用内的页面 Move，不复制 Widget，不将页面内容序列化到外部应用。

核心决定：

- 转移原 `TabPage` 及其 Widget 子树，不用相同声明重建一份页面。
- GUI 复用控件树的正常移除/挂载机制；不引入 `ChildOwner` 或独立页面宿主。
- UI 必须一起迁移子树的协调归属和 State，不能绕过协调器只搬 GUI Widget。
- 拖动期间源 TabView 始终拥有页面；目标只呈现插入预览，接受后才提交一次转移。
- TabBar 仅剩一页时仍可进入原生 DnD 并转入已有 TabBar；未被接收时恢复原位，
  不发新窗口请求。源最后一页转走后允许空 TabView，源窗口是否关闭由应用决定。
- 窗口创建、窗口内容、空源窗口是否关闭由应用决定，widgets 不代替应用管理窗口。
- 定位沿用 Window 参考系，不引入 `WindowPlacement`、显示器管理器或公共桌面坐标。
- 复用既有 DnD 会话和 UI 更新调度，不建立全局 TabView/页面注册表或平行手势系统。

不包含通用停靠布局、自动分屏、跨进程页面迁移、固定/分组标签、垂直标签栏、
标签历史恢复、多屏窗口位置持久化。Popover 不是本切片的页面转移目标。

本文前部规定行为、归属和接入边界；已落地的命令式与 UI 接口名称见第 13–16 节。
历史研究记录不代表所有路径都已完成或验收。

## 2. 当前代码基础与缺口

| 位置 | 已有行为 | 本次转移需要补充 |
| --- | --- | --- |
| `widgets/tab_view.go`、`tab_transfer.go` | TabPage 保存内容/元数据；TransferPage 校验、移交原页面、延后选择通知 | UI 协调保护与更多销毁竞态验收 |
| `widgets/tab_drag.go`、`tab_transfer_drag.go`、`tab_detach.go` | 栏内排序、原生 DnD、跨栏插入/接收、应用拖出请求、本地协商拒绝保护 | 组合验收 |
| `gui/drag_drop.go`、`gui/drag_dispatch.go` | DragSource.Begin 接管已获胜手势，Motion.Local 限时借用同应用对象，既有会话与结束事实 | 继续复用，不新增 Tab 专属路由 |
| `widgets/ui/tabs.go`、`tab_transfer.go` | 普通 TabPageView 节点、页面 ID、OnTransfer 更新两侧声明；无双向 key 映射 | 第 29 节记录当前验证范围 |
| `ui/root.go`、`ui/transfer.go` | 局部 ID／类型匹配、相对重排、Coordinator 显式移交普通子 node | 第 29 节记录当前验证范围 |
| DesktopWindow / gui.Window | WorkAreaAt、SetPosition、Position | 使用既有能力，不另造定位模型 |

当前 `platform/dragdrop.Result` 已提供 Action、Canceled、Err、Position、PositionValid、LocalTarget。
三平台传递原生结束点；macOS 根据实际完成动作和受时间限制的 Esc 观察生成结果，
不再用本方取消标记覆盖成功动作。GUI 已保护同步本地提交；栏内手势接管和命令式
页面与 UI 子树移交均已实现。结果能力边界见第 11 节。

## 3. 页面与资源归属

一个页面同一时间只能属于一个 TabView。转移保留 TabPage、业务 child 以及其中
与宿主无关的文字、编辑模型、选择、滚动、用户状态等对象，不触发页面 CloseRequest。
Closable 仍只控制关闭，不能拿来推导是否允许转移。

同一 TabView 内排序继续调用 MovePage，通过相对兄弟重排完成，不卸载。
跨 TabView 的页面移动复用正常控件树操作；跨 Root 必须经过 Unmount/Mount，
窗口相关绘制资源、输入关联和缓存按既有生命周期释放/重建。不要承诺所有 Mount
次数不变，也不要让窗口绑定资源为了“保留状态”继续引用旧窗口。

焦点、IME 会话、指针捕获和进行中的手势不是可以整体搬走的页面状态。
旧宿主按正常卸载路径清理，目标通过已有焦点机制重新建立，不迁移旧事件上下文。
自定义 Widget 也必须遵守“业务状态留在 Widget，宿主资源随 Mount 管理”的契约。

源移除当前页后沿用现有退避规则：原位置右侧优先，否则左侧；目标成功接收后选中
该页。不隐式关闭空 TabView 所在窗口，不让窗口最终兜底清理仍然拥有已转移子树。

## 4. 拖动交互与原生接管

栏内保持已实现的抓取偏移、跟手标签、相邻标签过渡、共同最小宽度与溢出滚动。
只有启用跨视图转移后，离开栏内拖动区域才允许转入原生 DnD；不改变未启用时的
栏外取消行为。启用策略与既有 Reorderable 的关系需在 API 细化时明确，不能自动
把所有已启用排序的标签升级为可移出应用布局的标签。

跨窗口 DnD 使用应用内 Local 数据携带一次会话的源页面身份，动作仅为 Move。
目标校验来源是当前有效的应用内页面，不信任外部格式或仅凭相同字符串就接受。
同一 TabView 关联的多个 TabBar 仍代表同一页面所有者，落回其中任一栏按重排处理。

预览使用 `RenderWidget` 生成的 `image.Image`，只在原生接管前准备；明确位图像素、
Scale 与 DIP hotspot。允许重新执行 Paint，不承诺复用上一帧像素。位图预览只是视觉
资源，不持有页面所有权，也不复制页面状态。

现有 DragEventController 已赢得栏内手势后，不能再叠加一个普通 DragSource 与之
重新竞争，也不能合成 PointerDown 或从任意异步任务伪造原生拖动。GUI 需要小型、
通用的接管机制，保证使用仍有效的原生按下上下文，并按顺序释放本地捕获、进入原生
会话。原生 Begin 可同步回调甚至运行嵌套循环，必须先建立会话记录再调用原生。
启动失败时清理预览/捕获并保留源页面，不能留下半接管状态或额外 Click。

目标 TabBar 根据正常 DropTarget 事件计算插入槽，支持空目标和边缘滚动；仅改变
视觉预览，不提前把页面加入目标。槽位依据最终布局而非过渡中的矩形。目标离开、
拒绝、销毁或会话结束立即清除预览；控件绘制、命中与 Snapshot 使用同一实际几何。

## 5. 接受与一次提交

顺序固定为：准备与校验 → 目标接受 → 移交原页面及协调记录 → 发布完成结果。
准备阶段源仍拥有页面；不能先 RemovePage 再等待应用创建目标、读取数据或弹出确认。

提交前检查源/目标存活、页面仍属于原源、会话未结束、目标允许接收、插入位置有效，
以及 UI 侧的 key/协调记录是否可以移交。页面数组变化后需要重算或拒绝陈旧槽位，
不能直接使用拖动开始时的索引。同源重排不重复走跨视图移除/加入。

转移原语不能只是对外散布两次裸 RemovePage/InsertPage 调用。现有页面变化信号和
Mount/Unmount 回调都可能重入，应先完成必要校验与提交保护，在可行时将页面结构/
选择通知延后到两侧归属一致之后。每次外部回调后重新核对对象和归属；提交中不允许
同页再次转移。普通失败且源仍存活时保留或恢复原归属、顺序和选择。若应用主动销毁
源或页面，则尊重销毁事实，取消余下步骤，不能以回滚为由复活对象。

DropRequest 的接受结果必须与真正完成的移交一致，不能先报 Accepted 再无限期排队
执行可能失败的移动。既有目标已经具备提交条件时完成一次提交；拖出创建新窗口是
未接受结束后的独立应用请求，不虚构一个原生接收目标。

移除源页面可能使源标签 Widget/DragSource 卸载，原生会话又可能尚未终结。必须协调
这种源卸载与 DnD 清理：源控制器及会话只做一次结束，不在成功目标提交后再次删除
页面，不把随后到达的 End/Cancel 当作撤销成功转移的指令。可以将必要的瞬时清理延至
安全结束点，但不能因此增加独立长期页面所有者。

## 6. 拖出创建窗口及坐标

只有没有完成投递、没有已识别的取消/错误、有有效结束位置、源仍有效且应用允许拖出时，
才发出创建目标窗口请求。应用通过 GUI 信号决定窗口大小、Chrome、内容和目标
TabView；UI 的 OnXXX 只是该信号的声明式绑定。没有处理或应用拒绝时源保持不变。

`Action == 0` 本身不够：还需检查 Canceled、Err、PositionValid 和本地提交记录。
平台不承诺区分外部无目标与外部拒绝；二者均为“未接受”，不是“已证明桌面空白”。
已知本应用目标明确拒绝时，Tab 策略保留源页面；外部拒绝与无目标无法区分时，
由应用的拖出请求决定是否创建窗口。没有有效位置则保留源页面。
最终命中本进程已注册拖放窗口但未完成投递时，也保留源页面；范围是整个注册窗口，
不是只按某个 Widget 的拒绝回调判断。未注册窗口不因此禁止拖出，详见第 17 节。
这是参考框架实际能力后的显式策略调整，不再把精确原因识别作为后续开发前提。
macOS Esc 采用第 11 节的兼容规则，不宣称 AppKit 提供了原生取消原因参数。

应用先准备新窗口及目标，再提交原页面；创建/定位/显示失败不得导致源页面丢失。
应用需明确必需的步骤，清理自己创建但未接收页面的空窗口。新窗口注册及转移成功前
不关闭最后一个源窗口，避免触发 QuitOnLastWindowClosed。成功后的空源窗口处理仍
属于应用策略，不由 TabView 自动销毁窗口。

已有定位接口的契约见 [窗口管理](DesignWindowManagement.md)：

```go
// 下列均已存在，不是本设计新增的桌面坐标 API。
SetPosition(relativeTo Window, position geometry.Point) error
Position(relativeTo Window) (geometry.Point, error)
WorkAreaAt(point geometry.Point) (geometry.Rectangle, error)
```

拖出结束点向 GUI 提供为源窗口客户区 DIP（允许在窗口外），并明确它对应的原生
结束时刻/坐标有效性。不能用旧的栏内最后一个 PointerMove 代替原生结束点，也不能
在源已销毁或移位后把失去参考依据的坐标当作有效数据。

新窗口外框请求位置应由结束点减去“新窗口外框到目标标签抓取点”的偏移得到。
该偏移包含原生框架到客户区、目标 TabBar 在客户区中的布局、标签槽位和标签内抓取
点。不能把源标签栏偏移当成目标偏移，或把结束点直接当外框左上角。参考与目标在
不同 DPI 时，先在平台边界明确尺度转换，再在同一参考窗口 DIP 中计算；不能直接
相减分别来自两窗口的 DIP。已有公开接口不代表所有混合 DPI 换算已实现。

位置与目标布局尚不可观察时，不通过硬编码标题栏高度或重入事件循环取得“精确值”。
实施时应明确准备/布局完成时序及无法精确定位的错误或应用回退路径，不能先移走页面
再补定位。使用 `SetPosition(sourceWindow, point)` 提交一次定位，不持续追随指针。
`Position` 是实际观察而非 setter 缓存，也不承诺请求立即完成。

`relativeTo == nil` 已定义为相对自身当前 WorkAreaAt(0,0) 的左上角；它可用于独立
窗口放置，但不用于从多个窗口各自的工作区位置猜测全局桌面坐标。边缘退避若有需要
属于应用/GUI 策略，不能顺带改写平台 SetPosition 的不裁限契约。

## 7. UI 子树协调与声明归属（历史方案，现以第 29 节为准）

### 7.1 查找路径与身份

优先复用现有路径：Window ID → `ui.app.windows` 中的 windowMount → root；
再用该 Root 范围的 Widget ID 定位 TabView/所需协调位置。不增加全局 Widget ID。
页面 key 仍只在一个 TabView 内稳定且唯一，不等同于 Widget.ID，也不以页面标题识别。

持有 GUI Widget 后，UI 核心根据实际对象身份寻找其拥有的协调记录；不能假设每个
Widget 都恰好对应独立 node。当前 `widgets/ui` 的 TabPage 是页内容的 Bin 挂载目标，
它位于所属 TabView node 的 `children` 中，具体记录是 `childTarget`。
因此转移的是该 target 及其 node 子树，不是凭页面 ID 新建一个 root。

目标已有同 key 时拒绝，不隐式覆盖或重命名。用于查找的 ID 必须无歧义；转移若使
目标 Root 内的相关 ID 冲突，应拒绝或由应用先显式解决，不改变全局 FindWidget 的
既有契约。同 Window 不同 TabView 也执行这套协调，不因 Root 相同而省略。

### 7.2 需要保留与重新关联的内容

- 原 TabPage、子 Widget 和现有 childTarget/node 对象。
- node.state、viewBaseContext 中的有效信号连接及当前声明协调状态。
- 源/目标 tabViewState 的 pages/keys 映射，并保持子树只有一个 UI 所有者。
- 更新任务的归属、过期检测及 AfterUpdate 回调的处理，不能由旧 Root 再次释放页面。

不能使用现有 `root.release`、`releaseChildren` 或 View.Unmount 来模拟转移：
这些路径会断开信号、清空 State 并结束 UI 生命周期。GUI 的 Unmount/Mount 与 UI
协调节点的 release 是不同概念，必须区分。正常删除页面仍走原清理路径。

原 UI 子树的 Mount/Unmount 不因移交而重新执行；目标的正常 Update 可以刷新新声明
与回调。若页面声明类型确实改变，仍按原规则重建，不能承诺保留不同类型的状态。
业务回调显式捕获旧窗口的引用不能被框架自动改写，由应用更新相应声明或业务状态。

核心新增能力只处理一般挂载目标/子树的归属移交，不认识 TabView、TabPage 或其 key。
`widgets/ui` 负责页面映射与语义，调用这一通用扩展。不要不断向 root/buildContext
加入 Tab 类型分支，不保存一次性的 BuildContext 供未来调用。

### 7.3 与更新批次协作

迁移请求、两侧页面声明归属更新和协调记录移交属于同一次受保护操作。应用需要更新
模型，使源声明不再包含页面、目标声明包含原 key/内容；不能只移动 Widget 后寄望
下一次重建自动理解，也不能只复制 View 值到另一个窗口而丢弃原协调状态。

在 GUI 同步转移请求中先移交原协调记录，再通知应用更新声明，最后使用现有 UI
更新调度重建。不得依赖“先更新源窗口”或“先更新目标窗口”的偶然遍历顺序。目标尚未存在时
先完成窗口与空目标的准备，期间源仍拥有完整页面；准备失败就取消，不保留无限期
悬空子树。不得在原生 Drop 等回调中同步启动不受控的嵌套 UI 重建。

AfterUpdate 是已有的绑定时序钩子，不是自动迁移机制。移动子树的旧回调要失效或
明确转交到正确 Root，并在目标更新后建立所需关联；不能让旧队列在源销毁后再调用。
旧 Root 的整窗更新不能触及已移出的节点。实施时验证 State 更新、旧任务和源窗口
销毁交错的情况，保证只更新目标、只释放一次。

只在声明式两端都能参与协调时承诺保留 UI 状态。不得让一个声明式页面通过裸 GUI
转移逃离协调器，或默认支持 GUI/UI 混合归属；未设计的混合情况明确拒绝。

## 8. 实施顺序与验收

1. 先补并验证原生结束事实、结束坐标与 GUI 拖动接管，保留现有 DnD 回归。
2. 实现命令式页面转移原语，测试归属、顺序、选择、回调重入与失败清理。
3. 接入跨 TabBar 插入预览和接收；原栏排序与 HeaderBar Caption 行为不退化。
4. 接入应用创建新窗口请求、布局/坐标准备与失败保留源页面。
5. 实现 UI 通用子树移交，再由 widgets/ui 接入页面声明与映射协调。
6. 完成三平台真实跨窗口、取消及拖出验收后，才标记整项完成。

测试遵循 [Testing](Testing.md)，不依赖 DevServer。纯逻辑在所属包内，真实窗口
在现有 `tests/window/widgets/tabs` 扩展或独立主题目录，入口写明步骤与预期。

包内必测：同源重排与同 Root/跨 Root 转移、空目标、目标 key/ID 冲突、源当前页
退避与目标选中、取消/失败保持原页、重复 Drop/End、回调销毁源/目标、重入移动、
多个 TabBar 关联同一 TabView；GUI 对象身份和 Mount/Unmount、UI node/State 身份
及信号有效期分别断言。转移后再重建双方、再改 State、再关闭源窗口，目标页面应
仍然可用且不重复创建/销毁。测试交换窗口协调顺序，不能只覆盖恰好成功的一种顺序。

主要窗口验收：同窗口双 TabView、两个现有窗口、空目标、拖出创建第三窗口、创建失败、
Esc 与普通未接受释放的区别、已知本应用拒绝目标不误开窗口、源最后一页、拖动中窗口关闭/布局改变、
长标题和溢出插入、抓取点/预览热点、1x/逻辑 2x、Native/Integrated 标题栏布局。
文本编辑页验证文字、撤销记录、选区与滚动保留；源窗口关闭后继续输入/更新/绘制目标。

用户确认不在边缘条件上反复验证：真实混合 DPI、更多外部目标和生命周期排列保留为
后续兼容性补充，不扩大本轮实现范围，不把未执行项伪装成通过。

Snapshot 同步表达真实页面归属、子树、顺序、Selected 和边界，预览不伪造第二份
页面语义子树。验收操作仍走正常原生输入和事件分发，不用直接修改私有字段替代。

窗口例程按 [Testing](Testing.md) 提供中文入口说明：`tests/window/widgets/tabs`
用于基础交互与样式观察，`tab_transfer` 用于命令式跨栏/跨窗口，`tab_detach`
用于命令式拖出准备，`ui_tab_transfer` 用于声明式转移/拖出及重建。主要流程与
可选回归分开说明，操作步骤和参数仅在源码维护，README 只索引。
UI 取消用例要求保留初始 `retained A`，不能先执行成功路径的编辑步骤。
取消需实际出现原生拖动再按 Esc，不以静态未变化断言或某一种回弹动画作为操作证据。

## 9. 参考及采用范围

查阅 libadwaita **1.11.alpha / API 1** 官方文档（GTK4 生态）：

- [AdwTabView.transfer_page](https://gnome.pages.gitlab.gnome.org/libadwaita/doc/main/method.TabView.transfer_page.html)
  明确复用原 page 对象。GOUI 采用保留页面身份的方向，不引入其固定标签等额外能力。
- [AdwTabView::create-window](https://gnome.pages.gitlab.gnome.org/libadwaita/doc/main/signal.TabView.create-window.html)
  将创建、定位新窗口和提供目标 TabView 交给信号处理方。GOUI 采用应用决定窗口的
  职责划分，信号形式仍遵循 Go/GOUI 约定，不照搬 C 返回值回调。

这些是公共 API 文档承诺，不是对 GOUI 的原生接管或取消识别已经可用的证明。
既有栏内交互继续参考 [Tabs 设计](DesignTabs.md) 的 Qt 实现证据；本设计不假设
Qt 的 movable 属性就等于跨窗口转移能力。原生接管/结束检测的具体 SDK 用法与源码
证据需在第一个实施切片补齐，特别是 macOS，不能用本章参考替代三平台验证。

## 10. 第一切片：原生结束证据探针（2026-09-29）

本节保留当时的设计与实测记录。原先的严格原因门槛已由第 11 节修订，历史失败
不改写为通过；本节 trace 操作是历史记录，当前用例与清理范围见第 12 节。

当前只实现验证设施，不冻结新的 DragResult 原因字段或手势接管 API，也不移动
TabPage/UI 子树。三平台区分取消、释放、无目标、拒绝的证据齐备之前，本切片不算完成。
尤其不能以 macOS 返回 Unknown 后忽略拖出，宣称满足了三平台拖出设计。

入口为 `tests/window/platform/drag_end`，使用现有 platform.DragDrop 的真实原生路径，
而不是复制一套只供测试的 OLE/XDND/AppKit 实现。原生回调在结果归一化之前写入
Go execution trace 的 `goui.dnd.x11/ole/cocoa` 类别；未启用 trace 时不查询诊断坐标。
没有新增获取内部状态的公共接口，也没有为探针模拟 PointerDown、安装全局键盘监听、
申请辅助功能权限或改写拖放策略。Windows 增补只观察的 IDropSourceNotify：与源对象
共享 IUnknown 身份和引用计数，不单独持有会话，不影响原有 QueryContinueDrag 决策。
AppKit 的 CurrentEvent、NSEvent.Type/Timestamp 绑定位于已有 appkit 文件。

记录内容：

- X11：ButtonRelease 的 root 物理坐标、源客户区物理坐标及 DIP、该点的 XDND target、
  缓存移动点/target、Esc 时间、XdndStatus/Finished 原始字段、结束时是否已释放/投递。
  诊断查询不改变现有 release 逻辑。现有实现仍以最后一次 Motion 更新投递位置，后续
  需要单独修正并覆盖释放点与缓存点不同的情况；不能把采到正确坐标算作接口已修好。
- Windows：QueryContinueDrag 的 escape/keyState/programCancel、当时 GetCursorPos
  与 GetMessagePos（分别是即时观察和消息点，不先认定任一个就是准确释放点）、
  IDropSourceNotify 进入/离开潜在目标、DoDragDrop HRESULT/effect。目标在最终清理时
  Leave，不能清掉释放时的目标证据。仍需真机核实通知顺序及快速移走指针的情况。
- macOS：开始事件类型/时间、会话身份、endedAtPoint 原始屏幕点及源客户区 DIP、
  operation、程序取消标志，以及结束时 CurrentEvent 的类型/时间/仅 KeyDown 的 keyCode。
  这些是原始观察，不将 CurrentEvent 直接认定为结束事件，不从 None 猜测无目标。

用例提供接受、未注册目标、格式拒绝、动作拒绝、最终 Drop 拒绝、独立源/目标进程、
程序取消/销毁、非输入回调启动失败；预览具有独立标记热点。入口注释列出 Esc 后再
释放、立即移动指针、回弹和 1x/2x 操作。接收方断言准确文本，一次源操作断言 Begin/End
数量；解码后的证据另由 `-check/-expect` 检查，缺失、矛盾、混合会话和未知结果不能 PASS。
校验器只是本用例的证据检查，不是新的平台状态机。AppKit 零动作目前明确不能通过
无目标/取消/拒绝判定；即便 CurrentEvent 是 Esc，也只是待验证线索。

Go 1.25 的使用方式（较旧工具可能使用 `-d=1`，按本机 `go tool trace -help`）：

```sh
go run ./tests/window/platform/drag_end -target none
# 用程序打印的 TRACE 路径，输出应放临时目录：
go tool trace -d=parsed /tmp/goui-drag-end-xxx.trace > /tmp/goui-drag-end-decoded.txt
go run ./tests/window/platform/drag_end -check /tmp/goui-drag-end-decoded.txt -expect no-target
```

### 10.1 原生依据及不采用的推断

- [Microsoft IDropSource.QueryContinueDrag](https://learn.microsoft.com/en-us/windows/win32/api/oleidl/nf-oleidl-idropsource-querycontinuedrag)
  给出 Esc 与按钮状态；[IDropSourceNotify](https://learn.microsoft.com/en-us/windows/win32/api/oleidl/nn-oleidl-idropsourcenotify)
  自 Vista 起报告潜在目标，不等同于目标接受。探针保留两种事实，不合并为 effect=0。
- [AppKit endedAtPoint 契约](https://developer.apple.com/documentation/appkit/nsdraggingsource/draggingsession(_:endedat:operation:))
  提供会话、结束点与操作，没有独立的 no-target/cancel 原因参数。
- [GTK 4.20.0 macOS 源码](https://github.com/GNOME/gtk/blob/4.20.0/gdk/macos/GdkMacosWindow.c)
  的结束回调把零动作映射为 NO_TARGET。该框架选择不证明原生已经区分 Esc 与拒绝，
  不满足这里的严格拖出门槛，不能直接照用。
- [Qt 6.8.3 Cocoa 源码](https://github.com/qt/qtbase/blob/v6.8.3/src/plugins/platforms/cocoa/qnsview_dragging.mm)
  使用结束点转换并补发鼠标释放，主要处理自身拖放循环/输入状态；它不是独立取消
  原因的证明，GOUI 不为这个探针补发 GUI 输入。
- Firefox 的 [nsCocoaWindow.mm](https://searchfox.org/firefox-main/source/widget/cocoa/nsCocoaWindow.mm)
  中 CurrentEvent/Esc 做法只作为验证候选，不作为 SDK 保证；需要核对对应源码版本和
  真实会话中的时间/回弹，不能把当前键盘状态、旧事件或某框架的命名当成系统事实。

### 10.2 已验证与缺口

Linux 本机 X11/Xfwm4、Software、Native 装饰：1x/2x 各六组（接受、未注册目标、
三种拒绝、目标上按住鼠标时 Esc）经 XTest 正常输入完成，12 份 trace 通过独立
`-check` 判定。接受场景同时断言目标收到准确文本。额外两组在释放后立即移动指针，
以 XTest 请求点和 xwininfo 客户区原点断言释放物理坐标与 DIP 换算，而非两条后端
公式互相比较。仅操作自建窗口，未修改系统设置/剪贴板；trace 存于临时目录。
1x 另验非输入回调 Begin 失败且无 Begin/End；按住按钮时由 GUI 线程取消、销毁源窗口，
各产生一次取消终态、没有释放/Drop。尚未覆盖目标已提交后的销毁交错。trace 增加诊断
查询和记录开销，只用于事实验证，不作帧率/延迟的性能基准。

`go test ./...`、探针包 race 检查通过；Windows amd64/macOS arm64 的探针和受影响
包目标编译通过。Windows 10 19045 amd64 在 Session 0 原生运行 Notify 的 ABI 布局、
IUnknown 身份、共享引用计数和实际 vtable 调用测试通过（不创建 HWND/不启动 OLE 拖放）。
macOS 15.7.7 arm64 已原生运行证据校验器的无头测试；这不执行 AppKit
窗口操作。后续远端桌面验证见下一节；完整三平台门槛仍未达到，不能由 Linux 结果替代。

### 10.3 Windows / macOS 远端桌面验证（2026-09-30）

使用 remote-computer-use 的截图、鼠标和键盘，在真实桌面终端启动探针；SSH 仅用于
传输、哈希核对和读取日志。Windows 10 19045 amd64、RDP 1600×900、Direct2D；
macOS 15.7.7 arm64、VNC 1650×1050、OpenGL；均为 Native 装饰、默认 1x。
另用 `GOUI_PLAT_SCALE=2` 检查逻辑缩放，不等同于系统 Retina/混合 DPI 验收。
SSH Session 0 或 `/dev/console` 的用户值不能单独作为桌面不可操作的判断依据：
本次两端均通过桌面终端启动并实际完成拖放。

| 场景 | Windows | macOS |
| --- | --- | --- |
| 同进程接受、独立源/目标进程接受 | 均收到准确文本；Begin/End 各一次；原生 Copy；检查器通过 | 同左；会话身份一致 |
| 格式、动作、最终 Drop 拒绝 | 三组 Action=0，拒绝检查通过 | 三组 operation=0，未传输文本；严格原因检查不通过 |
| 未注册目标（1x/2x） | 两组均被检查器误判为 rejected，不能验收 no-target | operation=0；仍不能证明 no-target |
| 程序取消、拖动中销毁源 | 按钮仍按下时返回取消 HRESULT，End 一次，无 Drop | 两组目标仍收到文本，原生 operation=Copy，但源端报告 Canceled=true；不能验收取消语义 |
| 无原生按下上下文时 Begin | 拒绝启动，无 Begin/End，通过 | 同左 |
| 按住鼠标时 Esc | 未完成有效操作，见下文 | 未执行有效操作 |

Windows 的无目标失败是探针结束原因判定的反例，不能通过调整期望为 rejected 消除：
`-target none` 没有创建目标 DragDrop、没有目标事件，但释放前仍有非零
`DragEnterTarget(hwnd)`。1x 与 2x 均复现。现有检查器用最后一个潜在目标 HWND
判定拒绝的证据不足；需要进一步核实该 HWND 与原生 OLE 行为，不能直接升级为公共
结束原因契约。[IDropSourceNotify 文档](https://learn.microsoft.com/en-us/windows/win32/api/oleidl/nn-oleidl-idropsourcenotify)
只承诺潜在目标通知，不承诺目标接受。

macOS 的普通释放记录中，CurrentEvent 在无目标/格式拒绝时为 type=6，在动作/最终
Drop 拒绝时为 type=2；同样的零动作没有唯一事件类型。程序取消/销毁源的两组中，
目标发生 Drop 并读到准确文本，ended 回调仍为 operation=1、program_cancel=true，
而 `dragSourceEnded` 因本地 canceled 标志覆盖为 Action=0/Canceled=true。现有代码
只拒绝后续 source mask 查询，实测不能保证撤销已运行的原生会话；成功投递事实和
本地取消请求需要另行设计，不能用这个结果驱动 Tab 转移回滚。本轮仅记录，不修改行为。

释放点以远程输入请求与截图客户区原点为独立预期，四组精确匹配：

| 平台/逻辑缩放 | 屏幕输入点（左上原点） | 原生结束点 | 源客户区原点 | 源 DIP |
| --- | --- | --- | --- | --- |
| Windows 1x | (750,260) | (750,260) | (68,131) | (682,129) |
| Windows 2x | (1300,450) | (1300,450) | (128,231) | (586,109.5) |
| macOS 1x | (735,280) | (735,770)，AppKit 左下原点 | (60,153) | (675,127) |
| macOS 2x | (1300,470) | (1300,580)，AppKit 左下原点 | (120,253) | (590,108.5) |

松手后发送远处 move，记录没有变成该 move 点；但工具串行输入，不能证明零间隔竞态。
Windows 尝试长 drag 期间并发发送 Esc，工具仍在 drag 松手后才执行按键，trace 没有
escape=1，不能把该次运行算作 Esc 验收。两端按住时 Esc、精确回弹过程/热点、外部
非 GOUI 应用、目标提交后的销毁竞态、系统混合 DPI 和栏内手势接管仍待验证。

本次仅恢复 macOS dragdrop 文件缺失的三个 framework 导入，未修改其他行为代码。
两端探针交叉编译、传输哈希核对及本机 `go test ./...` 通过（多数包缓存）；既有
图片/窗口测试按原规则运行。日志、原始 trace 与 Go 1.25 `-d=parsed` 解码位于本机
`/tmp/goui-drag-verify.vrOBv3`，文件前缀为 `win-` / `mac-`；截图在 `/tmp/goui-*.png`。

## 11. 采用框架实践的结果契约（2026-09-30）

不再追求原生 API 没有承诺的完整结束原因分类，也不增加 Unknown/NoTarget/Rejected
公共枚举。`Action` 表示完成的动作；`Canceled` 表示已识别的取消；其余零动作表示
未接受，可能是无目标或拒绝。取消是请求，不能撤销已经投递的数据。

本轮核对的源码及取舍：

- GTK **4.20.0** 的 [GdkMacosWindow.c](https://github.com/GNOME/gtk/blob/4.20.0/gdk/macos/GdkMacosWindow.c)
  在 endedAtPoint 中将非零操作视为完成、零操作归为 NO_TARGET。采用其以实际动作
  为准的方向，但不把其枚举名称解释成系统证明没有目标。
- Qt **6.8.3** 的 [qnsview_dragging.mm](https://github.com/qt/qtbase/blob/v6.8.3/src/plugins/platforms/cocoa/qnsview_dragging.mm)
  保留应用内部已经接受的动作，不用稍后的原生反馈覆盖。GOUI 的 GUI 会话同样在
  同步 Drop 返回 Accepted 后保留提交结果；Drop/Leave/Finish 中重入 End 延至回调
  返回后处理，只发一次 End。目标尚未接受时仍保持原有取消结果。
- libadwaita **1.7.0** 的 [adw-tab-box.c](https://github.com/GNOME/libadwaita/blob/1.7.0/src/adw-tab-box.c)
  通过 drag_cancel(NO_TARGET)、drop_performed 和 create-window 协调拖出。
  采用应用决定新窗口和基于本地投递事实作策略的方向；不照搬其提前 detach 页面流程，
  GOUI 继续保留源所有权直到提交。
- Firefox Cocoa 的 [nsCocoaWindow.mm](https://searchfox.org/firefox-main/source/widget/cocoa/nsCocoaWindow.mm)
  （2026-09-30 查阅 gecko-dev master，非固定发布版）检查 NSApp.currentEvent 的
  KeyDown/Escape。GOUI 采用这一兼容规则，额外要求事件时间不早于源按下时间；
  非零完成动作仍优先。这不是 SDK 保证，真实按住鼠标时 Esc 和回弹时序仍需桌面验证。

`dragdrop.Result` / `gui.DragResult` 增加 `Position` 和 `PositionValid`：源客户区 DIP，
允许在窗口外，(0,0) 也是有效点；取消、错误、源销毁或无法取得位置时无效。
Windows 在 QueryContinueDrag 判定释放时读取消息点；X11 使用 ButtonRelease 的
root 点，并重新协商最终位置，不能用旧 XdndStatus 直接提交新位置；macOS 使用
endedAtPoint 的结束点转换，不查询拖放结束后指针位置，不加预览热点补偿。

当时探针新增加 `-expect unaccepted`；Windows 潜在 HWND 不再用于分类拒绝。
严格 no-target/rejected 仍只用于有相应证据的 X11 诊断，不能将 Windows/macOS
的未接受记录当成严格分类通过。旧“原生 Copy 被取消标志覆盖”的 trace 仍必须失败。

后续切片是栏内手势合法接管、TabPage 一次移交、UI 子树协调与新窗口策略，
不是继续等待不可能统一的外部目标原因。上述修改不代表 TabView 转移功能已完成。

### 11.1 本轮验证

- 本机 `go test ./...`、`go test -race ./gui ./platform/dragdrop ./tests/window/platform/drag_end`
  通过。新增位置有效性、Drop/Leave/Finish 重入 End、重复 End、已提交动作优先的回归。
- Windows amd64 / macOS arm64 的平台测试和探针编译通过。远端 Windows 10 19045
  原生运行 Notify 身份/vtable 测试通过；macOS 15.7.7 arm64 原生运行 TestMacDrag 系列
  通过，包括成功晚于取消、旧 Esc 时间、其他按键、原生协议与 pasteboard 无头测试。
- 独立 Xvnc :97 / Xfwm4 / Software / Native / 1x：接受、无目标、格式拒绝、动作拒绝、
  最终 Drop 拒绝、按住鼠标时 Esc 六组通过。接受收到准确文本；快速释放 trace 显示
  释放后等待旧 Status，再发最终 Position、收到新 Status 后才 Drop。四组未接受的
  Result.Position 与 xwininfo 客户区原点及 XTest 输入点独立核对，随后移动指针不改变结果。
- 最初在现有桌面操作时误投到 GoLand，插入测试文本；已仅撤销该次插入，并由截图与
  文件内容确认恢复。该次不计受控目标验收；后续切换隔离显示。隔离桌面和测试进程已清理。
- 本轮未重跑 Windows/macOS 桌面或 2x/混合 DPI；旧桌面证据不能替代修改后的验收。
  既有全仓图像测试仍按原规则运行。产物、日志、trace 在 `/tmp/goui-dnd-fix.o1LEEk`。
测试未修改系统设置或剪贴板，只传输探针文本，已关闭自建窗口和测试终端，保留验证
产物；Windows 本轮 RDP 连接已断开，原有 macOS VNC/SSH 会话保留。

## 12. 临时诊断清理（2026-09-30）

三平台原生拖放的 runtime/trace 调用、辅助函数和仅为日志读取的坐标查询已删除。
Windows 不再实现诊断专用 IDropSourceNotify，删除其绑定、GetCursorPos 绑定和
对应 tear-off 测试；保留 IDropSource 的 ABI、IUnknown 身份与引用计数测试。
GetMessagePos 与 AppKit CurrentEvent/Type/Timestamp 仍用于正式行为，不能删除。
三平台结束位置、AppKit Esc 识别、成功动作优先以及 GUI 提交重入保护保持不变。

窗口用例保留受控接受/拒绝、程序取消/源销毁、无效启动与跨进程场景，
移除 -trace/-check 和私有 trace 解析器。现在直接检查公开 Begin/End 次数、
结果契约、目标准确 payload、取消时无 Drop、释放位置有效性，以及源销毁后的
位置失效。包内表驱动测试验证这些断言能拒绝重复结束、虚假取消、丢失位置等结果；
AppKit 的旧 Esc 时间和完成动作优先仍由平台包内测试覆盖。
坐标精度需用真实输入点和独立客户区原点核对，程序的 PASS 不代替这一步。

当前启动方式：

```sh
go run ./tests/window/platform/drag_end -expect accepted
go run ./tests/window/platform/drag_end -target none -expect unaccepted
go run ./tests/window/platform/drag_end -expect canceled
```

最后一项需保持鼠标左键按下，按 Esc 后再松开；具体步骤见入口中文注释。
旧 trace、证据分类及其历史失败记录只用于解释设计演进，不再作为运行依赖。

清理后验证：

- 本机 Linux 的 `go test ./...` 与
  `go test -race ./gui ./platform/dragdrop ./tests/window/platform/drag_end` 通过。
  既有图像测试仍按原规则运行；本轮不额外操作本机桌面。
- Windows amd64、macOS arm64 的探针与平台包测试交叉编译通过。
  远端 Windows 10 19045 的 TestDragSourceIdentity、macOS 15.7.7 arm64 的
  TestMacDrag 系列原生无头运行通过。
- Windows 10 / Direct2D / Native / 1x，macOS 15.7.7 / OpenGL / Native / 1x
  真实桌面各验证正常接受与按住时 Esc。接受准确传输文本并报告 Copy；
  Esc 均只有一次取消 End、无 Drop、位置无效。
- 独立坐标核对：Windows 源客户区原点 (68,131)，释放 (750,260)，
  返回 (682,129) DIP；macOS 原点 (60,153)，释放 (735,280)，
  返回 (675,127) DIP。截图与输入坐标均为本轮 1x。
- 产物与日志：`/tmp/goui-dnd-clean.HsxFaY`。本轮未复验 Linux 桌面、
  2x/混合 DPI、外部非 GOUI 目标和提交后销毁竞态，不扩大验收结论。

远程工具注意事项：RDP 的 WIN+R 返回成功只保证输入已发送，不表示运行框已经
取得焦点。本轮连续 type 导致部分命令输入旧地址栏；未打开链接或执行该文本，
后续改为截图确认运行框后再输入。显式 pointer-down/move/ESC/pointer-up 与
release-input 在两端正常，无需并发调用 drag。测试窗口与自建终端均关闭，
不修改系统设置或剪贴板；保留原有 macOS 桌面会话，断开本轮 Windows RDP。

## 13. 手势接管与命令式页面移交（2026-09-30）

### 13.1 实现接口与边界

```go
// GUI 通用手势接管，不是 Tab 专属入口。
func (*gui.DragSource) Begin(*gui.DragEventController, *gui.DragData, gui.DragPreview) error
func (*gui.DragMotion) Local(name string) (any, bool)

func (*TabView) TransferPage(page *TabPage, target *TabView, index int) error
func (*TabBar) SetTransferable(bool)
func (*TabBar) ConnectTransferError(func(error)) signal.Handle
```

- Begin 只能从已获胜左键拖动的 Update 同步调用，源不必注册为第二个事件控制器。
  先验证数据与宿主，再结束原指针路由、安装会话，最后进入原生 Begin。
  不再次发送 Prepare；接管后不产生本地 End/Cancel 或额外 Click。原生同步完成、
  启动失败、重复 End、取消和源卸载沿用现有会话保护。
- DragEventController 在 Begin 后继续处理导致阈值跨越的同一 Move。
  原实现漏掉此位置，macOS 连续移动合并时实际复现未接管；单次远移回归与修复后
  远端快速移动复验通过，不通过添加计时器或伪造按下来补偿。
- Motion.Local 只在 Enter/Motion 信号派发期间访问当前选择格式的本地对象。
  必须匹配活动源会话，外部 offer 不可获得 Go 对象；回调结束后请求失效。
- TransferPage 跨视图保留原 TabPage/child，正常 Unmount/Mount，目标选中该页，
  源按右邻、左邻退避。同视图沿用 MovePage 的最终索引和保持选择语义。
  生命周期期间冻结两侧页面 API，完成结构/选择后才发送通知；普通失败尝试恢复
  活着的源，明确销毁的对象不复活。不发送 CloseRequest，不自动关闭空源窗口。
- WidgetBase.Destroyed 是最终销毁事实，不等同未挂载。树操作增加重入保护：
  在 Unmount 销毁页面/源/目标时不重复卸载，不向已销毁对象重新挂载。
  这属于通用生命周期修复，不是 TabView 在 GUI 内核中的类型特例。
- Transferable 默认关闭；源同时需要 Reorderable，目标只需 Transferable。
  未开启的栏仍保持原来的栏外取消行为。会话局部对象携带源 view/page，无全局注册表。
  RenderWidget(scale=0) 与 Preview.Scale=0 使用同一宿主比例；热点为标签局部 DIP。
  有效 allocation 可在布局待处理时导出，不执行隐藏布局，见 DesignRenderWidget。
- 目标用普通 DropTarget，按槽位而非动画矩形判断位置，放下时重新计算。
  插入标记是最后绘制的装饰子控件，不添加语义动作、不修改 Painter 遍历顺序。
  空目标和溢出边缘滚动已接入；其桌面组合仍须补验。
- 本阶段未暴露 widgets/ui 的 Transferable，也未实现 UI 所有权协调保护。
  不能通过 FindWidget 后手动开启转移来绕过声明式协调；裸 TransferPage 目前只用于
  命令式页面。下一切片必须补齐协调保护、key/ID 校验与原 node/childTarget 迁移。

### 13.2 参考与取舍

GTK 4.20.0 的 [gtkdragsource.c](https://github.com/GNOME/gtk/blob/4.20.0/gtk/gtkdragsource.c)
与 libadwaita 1.7.0 的 [adw-tab-box.c](https://github.com/GNOME/libadwaita/blob/1.7.0/src/adw-tab-box.c)
提供了从已识别拖动进入原生 DnD 的参考。GOUI 采用合法原输入上下文内接管，
不增加第二轮手势竞争；不采用 libadwaita 开始拖放就移出页面的策略，保留源至提交。
[Adw.TabView.transfer_page 文档](https://gnome.pages.gitlab.gnome.org/libadwaita/doc/1-latest/method.TabView.transfer_page.html)
（本次页面标注 1.9.3）明确复用原 page 对象；GOUI 同样保留对象，但失败恢复、通知
时序及 UI 记录迁移遵循本项目自己的生命周期和声明式约定。

### 13.3 阶段验证

- `go test ./...`、`go test -race ./gui ./widgets ./widgets/ui` 通过。
  现有全仓图片测试仍按原规则生成图片，未迁移或扩大 skip。
  Windows amd64/macOS arm64 的 GUI/UI/widgets 测试目标与新用例编译通过。
- 包内覆盖：接管同步/异步/启动错误与数据冻结、失效调用保留本地手势、首次跨阈值
  Move、Local 访问范围、原对象/文字保留、同视图重排、跨视图通知/选择一致、重入
  拒绝、循环/索引校验、销毁重入，以及不受动画位置影响的空目标/插入槽。
- `tests/window/gui/drag_handoff`：Linux/Xvnc :97/Xfwm4/Software/Native/1x
  本地拖动、原生接受、Esc 通过；Windows 10 19045/Direct2D 与 macOS 15.7.7
  arm64/OpenGL/Native/1x 的接受、Esc 通过，准确同一个 Go 对象及一次终态。
  接受释放点独立核对：Windows (730,280)-(68,131)=(662,149)，
  macOS (730,280)-(60,153)=(670,127) DIP。
- `tests/window/widgets/tab_transfer`：上述三平台 Native/1x 真实输入将 A 从源 A/B
  移至目标 T 前，目标 A/T、源 B；保留原输入对象和文本、Mount=2/Unmount=1，
  销毁源窗口后目标页面仍存活。另各验证 Esc 后不移交，Mount=1/Unmount=0。
  本机确认插入标记不被标签背景遮挡；远端预览精确热点/跟随仍不以单张截图判通过。
- 首次 Tab 窗口验证因 RenderWidget 的全宿主 layoutDirty 检查失败；按照 13.1
  明确当前 allocation 契约后复验通过，保留原失败日志。macOS 接管探针一次
  ESC 后立即 pointer-up 仍成功投递，保持按下观察回弹后再释放则取消通过；记录为
  工具输入/系统处理时序观察，不能将该次取消失败改写为通过，尚未证明具体归因。
- 日志、截图、编译产物在 `/tmp/goui-handoff.tGYSjD`。remote-shell 用于传输与日志，
  remote-computer-use 用于已截图确认的测试窗口实际操作；不依赖 SSH Session 0
  启动 GUI。输入命令返回成功不是桌面动作完成；显式按住期间始终配套 release-input。

仍未完成：UI 子树迁移与混合所有权拒绝、应用创建新窗口及正确定位、已知本地拒绝
与拖出请求策略、同窗口多栏/空目标/长栏溢出的桌面组合、Integrated、2x/混合 DPI、
更多通知销毁交错。上述阶段通过不代表完整 TabView 拖出功能已完成。

本轮结束已关闭自建测试窗口、Windows 命令窗口和 macOS 测试终端标签；断开本轮
Windows RDP，保留原有 macOS VNC/SSH。停止本机隔离 Xvnc/Xfwm4，不影响用户桌面。

## 14. 命令式拖出请求与新窗口准备（2026-09-30）

新增 `TabBar.ConnectDetachRequest(func(*TabDetachRequest))`。请求给出原 Page、
源 Window、结束 Position（源客户区 DIP）、标签内 Hotspot。应用可以保存请求，
等待自己创建的目标正常布局，然后调用 `TransferTo(target, index)`；不创建独立
页面所有者，不先卸载源页面。`Cancel()` 幂等；新拖动、源卸载、SetView、关闭
Transferable 会使旧请求失效。TransferTo 是一次性尝试，失败也消费请求，错误
直接返回，不重复经过 TransferError。应用负责清理未接收页面的窗口。

参考 [Adw.TabView::create-window](https://gnome.pages.gitlab.gnome.org/libadwaita/doc/1-latest/signal.TabView.create-window.html)
（本轮官方页面标注 libadwaita 1.9.3 / API 1）：应用创建、定位窗口并提供目标
TabView。GOUI 沿用该职责划分，但不照搬同步返回值；允许应用在正常布局之后完成
一次移交，不通过重入事件循环强行取得布局，也不把拖出后创建的窗口伪装为原生 Drop。

### 14.1 结果事实与未完成边界

GUI 的 DragResult 不再是平台 Result 的 alias，增加 `LocalDrop`：只有本应用
实际收到匹配源会话的 DragDrop 才置位，在协商/读取/回调之前记录，拒绝与同步
End 不抹掉这一事实。平台 Result 不增加 GUI 策略字段，UI 仍复用 GUI 结果。

请求要求 Action=0、无 Canceled/Err、有合法 Position、没有 LocalDrop/页面提交，
且源栏/页面仍属于原窗口。LocalDrop=false 不证明桌面空白，也不把曾经 hover
本应用等同最终本地投递。外部拒绝/无目标仍无法严格区分。

**本阶段原有缺口，现由第 17 节补齐：** 原生协商阶段返回零动作后，有的后端不再发送 Drop。这时单靠
LocalDrop 不能完整识别“最终停留于本应用明确拒绝区域”，不能把本切片标记为
已完成全部本地拒绝策略。后续需结合可验证的最终目标事实处理，不以“曾经进入过
本应用”永久禁止拖出，也不删除普通未接受后创建窗口的能力。

本阶段尚未实现的 UI 迁移保护已在第 15–16 节接入；声明式页面不可绕过协调器裸移交。

### 14.2 窗口准备示例与坐标

`tests/window/widgets/tab_detach` 用应用自有的 frameProbe 在正常 Paint 后 Post
准备工作，不在 Paint 中搬页面。目标暂放仅用于测量的标签元数据占位页（没有复制
编辑子树）；布局/显示/定位成功后移除占位、移交原页面。取消或注入准备失败时
关闭空目标，原页面从未卸载。

定位使用观察到的目标标签客户区位置、`target.Position(target)` 外框偏移和两侧
Paint 的 PixelScale：

```text
目标外框（源 DIP）= 释放点 - (目标标签位置 + 抓取点 - 目标外框自身偏移) × 目标比例/源比例
```

不硬编码标题栏高度；新窗口最终定位由 `Position(source)` 再观察核对。准备期间
源窗口移动或比例改变，用例明确失败；不会把旧释放点重新解释为新的客户区点。
这个示例不承诺复杂自适应布局或跨屏 DPI 变化后的通用自动定位。

### 14.3 阶段验证

- 包内断言覆盖结果事实（hover 不算 Drop、本地拒绝、外部隔离、同步 End）、
  请求资格、延迟准备保留源、原对象一次移交、取消/禁用/换 View/新拖动/卸载失效、
  提交失败保留源。`go test ./...`、`go test -race ./gui ./widgets ./widgets/ui` 通过。
- Windows amd64/macOS arm64 的 GUI/UI/widgets 测试目标及窗口程序交叉编译通过；
  编译不算原生逻辑运行。既有全仓图像测试仍按原规则运行，未迁移或扩大 skip。
- Linux 隔离 Xvnc :97 / Xfwm4 / Software / Native 1x：真实拖出新窗口、Esc、
  注入目标创建后准备失败通过。定位释放 (850,550)，源客户区 (55,129)，返回
  (795,421) DIP；新窗口原抓取点再次观察为相同坐标。原页面/输入对象、文本、
  Mount=2/Unmount=1 及成功后关闭源窗口的目标存活断言通过。
- Linux 逻辑 2x 拖出/定位通过，释放 (400,150)，源客户区 (105,229)，返回
  (147.5,-39.5) DIP；请求 Integrated，但隔离环境未开启合成器，截图显示原生
  装饰回退，因此**不计为 Linux 自绘 Integrated 验收**。
- Windows 10 19045 amd64、macOS 15.7.7 arm64 / Native / 1x：通过桌面终端启动
  已核对 SHA256 的程序，真实拖出新窗口、定位、Esc、注入准备失败均通过。
  使用默认后端选择（Windows 优先 Direct2D，macOS 优先 OpenGL，均允许回退），
  本轮未独立观测最终 Painter 类型，不据此宣称某一特定后端验收。
  Windows 释放 (850,540)，源客户区 (58,131)，结果 (792,409)；macOS 释放
  (850,550)，源客户区 (50,153)，结果 (800,397)。新目标抓取点均与释放点相等。
- 产物/截图/日志：`/tmp/goui-detach.UBtGMo`。未提交代码，未修改系统设置或
  剪贴板。真实混合 DPI、Integrated、最后一页及更多销毁交错仍待验收。
  已关闭测试窗口和本轮新建终端/标签，断开本轮 Windows RDP，保留原有 macOS
  VNC/SSH；已停止自建 Xvnc/Xfwm4。远端工具本轮无新的调用错误，截图仍可能
  早于实际画面更新，均通过后续截图和终态日志确认，没有因初帧陈旧重复拖动。
未提交代码、未修改系统设置或剪贴板；临时验证产物保留供复查。

## 15. UI 协调节点移交（2026-09-30 历史切片，现以第 29 节为准）

### 15.1 实施接口与归属

`ui.Coordinator` 借用现有 node，通过真实 Widget 身份查找 State，同步移交 Bin
的 childTarget/node；不是缓存 BuildContext，也不创建全局页面注册表。通用接口
旧接口现已由 [UI 普通节点移交](DesignUI.md#22-显式普通节点移交2026-10-01) 替代。

`widgets.TabView.ConnectTransferRequest` 在普通 TransferPage 提交前同步询问源，
源未处理才询问目标。请求输入只读，`Handled`/`Err` 可明确拒绝；`Commit()` 只在
通知内有效且只能一次调用。已处理但未提交不能返回虚假的成功。无处理器仍使用
原命令式事务；UI 任一侧的处理器会拒绝 GUI/UI 混合归属，避免裸 reparent 逃离协调。

`widgets/ui` 的链式接入：

```go
wui.TabBar("documents").Reorderable(true).Transferable(true).
    OnTransferError(func(err error) { /* 由应用处理失败 */ })

wui.TabView(pages...).ID("documents").OnTransfer(func(change wui.TabTransfer) {
    // change: Key、SourceWindow/SourceView、TargetWindow/TargetView、Index。
    // 同步更新两侧页面声明集合，再调用 app.RequestUpdate()。
})
```

跨视图时两侧 Window/View 必须有 ID，源必须提供 OnTransfer，目标不能有同 key。
key 仍是 TabView 内部页面身份，不成为全局 Widget ID。协调器检查移动子树与目标
Root 的 ID 冲突。源和目标映射在 GUI 生命周期回调前一起切换，提交失败恢复；
UI 选择回调在声明模型更新后发出，避免把移动后的页面解析成空 key。
同视图重排仍使用原 Moved/OnMoved，不重复发跨视图通知。

UI 的 `OnDetachRequest(func(*wui.TabDetachRequest))` 传出带 Key 的命令式准备请求。
应用可以先声明空目标窗口，通过 App.FindWidget 找到目标后 TransferTo；此路径
也经过同一个协调请求。本节阶段先验既有窗口转移；声明式新建窗口、正常布局
与热点定位的后续整合验收见第 16 节。

### 15.2 参考与取舍

libadwaita 1.10.0 的 [transfer_page 官方契约](https://gnome.pages.gitlab.gnome.org/libadwaita/doc/1-latest/method.TabView.transfer_page.html)
明确复用原 page，本实现也保留原页面对象。[Flutter Element.inflateWidget 的官方
实现](https://api.flutter.dev/flutter/widgets/Element/inflateWidget.html) 展示了复用并重新
挂载既有 Element 的方式。采用的是保留协调状态的思路；不引入 Flutter 的 GlobalKey
注册表和隐式取回机制，GOUI 在现有薄绑定中显式移交挂载目标。Flutter 页面没有可靠
发行版本标识，此处只记录 2026-09-30 查阅的官方实现，不推断某个版本承诺。

### 15.3 验证与范围

- 包内断言覆盖同 Root/跨 Root、两种重建顺序、原 State/信号上下文与 Widget
  身份、过期 AfterUpdate、空页面、重复 ID/外部目标/子树环、失败恢复、panic
  传播、源释放、两侧释放、嵌套重建拒绝，以及 GUI 转移请求的单次提交/失效。
- 新窗口用例 `tests/window/widgets/ui_tab_transfer` 入口为中文操作说明，不依赖
  DevServer。自定义 View 在 Mount 创建输入框并记录 State，重复 Mount 直接失败；
  通过真实输入验证移交前后文本和连接，源声明删除后再次断言原对象仍属于目标。
- Linux 隔离 Xvnc :97 / Xfwm4 / 显式 Software / Native 1x：移交、重建、继续
  编辑、源窗口删除及 Esc 通过。初次继承 `XMODIFIERS=@im=fcitx` 时文字输入失败，
  完整断言报错；测试进程改用 `@im=none` 后通过，未修改用户会话设置。
- Windows 10 19045 amd64、macOS 15.7.7 arm64 / Native 1x：已核对 SHA256 的
  程序经真实桌面执行，同样移交、重建、编辑、源窗口删除及按住时 Esc 通过。
  使用默认后端选择，未独立观察最终 Painter，不声称特定绘制后端验收。
- 产物 `/tmp/goui-ui-transfer.95Hvso`，包括初次失败、成功日志与截图。远端截图
  可能早于已发送输入的画面，需要后续截图与终态日志核对；本轮没有新的工具错误，
  没有修改系统设置或剪贴板。测试后关闭自建窗口/终端并清理隔离显示。
- `go test ./...`、`go test -race ./gui ./ui ./widgets/...` 通过；Windows amd64 /
  macOS arm64 的 GUI/UI/widgets 目标编译通过。既有全仓图像测试仍按原规则运行。
  桌面操作后另增加了“正在重建时不得转移”的拒绝检查及回归，最后的包内/race/目标
  编译覆盖该增量；这一拒绝增量尚未重新运行远端桌面，后续新窗口切片一起复验。

本节阶段结束时仍待完成 UI 新窗口准备/失败路径及更多组合；后续进展见第 16 节。

## 16. UI 新窗口准备与失败清理（2026-09-30）

`tests/window/widgets/ui_tab_transfer` 增加 `detached`、`prepare-failure`、
`prepare-cancel` 模式和 `-chrome native|integrated`。本切片复用现有生产接口，
不增加 Window/Root 帧回调，不把应用的窗口创建策略下沉到 widgets。

### 16.1 准备流程与断言

- OnDetachRequest 保存原请求、源位置/缩放，声明目标窗口及仅含元数据的占位页。
  原输入框此时仍由源页面持有，不先创建第二个编辑器。
- 用例自己的 Bin 在普通 Paint 观察 PixelScale，随后 App.Post 进行准备。
  此时从 Snapshot 取得目标标签真实布局、Position(self) 取得客户区与外框偏移，
  按第 14 节公式将目标定位到释放点；不强制布局或重入事件循环。
- TransferTo 通过同一个 UI Coordinator 移交原页面及 node/State；OnTransfer
  同步修改两侧声明集合并移除占位页，RequestUpdate 进行正常协调。
- Verify 检查原 TabPage/TextInput/State 身份，View 只 Mount 一次且未 Unmount，
  GUI 正常跨窗口 Mount=2/Unmount=1；重建后继续输入，再移除源窗口声明验证目标存活。
- `prepare-failure` 声明同 key 的独立空目标页，实际调用 TransferTo，由 UI key
  冲突检查拒绝。`prepare-cancel` 则主动 Cancel。两者都尝试再次提交并断言失败，
  删除准备窗口后检查源原对象/文本不变、GUI 与 View 均未发生 Unmount。
  这不是原生窗口创建失败注入，也不是原生 DnD 协商拒绝；不混用三者的验收结果。
- Esc 模式必须先观察到原生预览再取消；仅点击 Verify 得到状态未变化，不能证明
  原生取消路径正确。操作说明已明确该前提。

### 16.2 阶段验收

产物、截图及日志保存在 `/tmp/goui-ui-detach.sR1Vb5`。Windows amd64 / macOS arm64
测试产物传输前后 SHA256 一致：Windows `0daf6c422ca300e29f6cedcd8ae4fafd3b19ebbc49e7f0febc463fb4c8d12160`，
macOS `7c10ac87ca80def0c320a6f1eb2713bd47c0618ec62c235c3d49b43243990058`。
远端验证后仅调整中文操作注释，没有未复验的生产行为增量。

- Linux/X11、隔离 Xvnc :97、Xfwm4、显式 Software：Native 1x 的拖出新窗口、
  同 key 拒绝、主动取消准备通过；Integrated 1x/逻辑 2x 的拖出、重建、继续编辑、
  源声明删除通过，Integrated 1x Esc 通过。Integrated 截图确认是 GUI 装饰而非
  Native 回退。隔离会话使用 `XMODIFIERS=@im=none`，未修改用户会话输入法。
- Windows 10 19045 amd64、macOS 15.7.7 arm64：通过真实桌面输入完成 Native 1x
  和 Integrated 1x 拖出/定位/重建/继续编辑/源窗口销毁；Native 1x 同 key 拒绝、
  主动取消准备，以及 Integrated 1x 原生预览出现后的 Esc 通过。使用默认后端选择，
  未独立查询最终 Painter，不声称特定后端覆盖。
- 独立坐标核对：Windows Native 源客户区原点 (58,111)，释放 (820,500)，
  结果 (762,389) DIP；macOS Native 原点 (50,133)，释放 (820,510)，
  结果 (770,377) DIP。Integrated 对应原点 Windows (57,81)、macOS (50,105)，
  结果分别 (763,419)、(770,405) DIP。目标截图中原抓取点均与释放点重合，
  Verify 使用实际 Position 查询再次检查不超过 1 物理像素。
- Linux Integrated 2x 原点 (100,160)、释放 (400,100)，结果 (150,-30) DIP，
  目标外框 (160,32) 物理像素；原标签抓取点最终仍为 (400,100)。这是同屏逻辑
  缩放，不代表跨显示器或混合 DPI。
- `go test ./...`、`go test -race ./gui ./ui ./widgets/...` 通过，Windows/macOS
  窗口程序构建通过；本轮远端程序包含第 15 节最后加入的重建期转移保护。
  既有全仓图像测试仍按原规则运行。

远程工具没有新的调用错误。两端 action 成功后立刻截图仍可能取得输入前画面，
因此等待后重取截图并核对日志，不将 action 返回成功当作窗口操作已完成。
测试只编辑测试输入框和操作自建窗口，未改系统设置或剪贴板；结束后清理测试进程
及本轮终端，断开本轮 Windows RDP，保留预先存在的 macOS VNC/SSH 会话。

剩余：第 14.1 节的完整本地协商拒绝策略、更多混合所有权/销毁交错、同窗口多栏、
空目标/溢出边缘等组合，以及真实混合 DPI；本阶段通过不代表整个转移功能已完成。

## 17. 原生协商拒绝与最终目标事实（2026-09-30）

`LocalDrop` 只记录真实投递，无法覆盖原生协商时即拒绝、系统不再发送 Drop 的路径。
新增平台事实 `Result.LocalTarget`：结束点命中本进程仍注册拖放的原生窗口。不暴露
窗口句柄、不判断 Widget，也不代表接受或投递；false 仍不能区分外部拒绝和无目标。
只有正常结束且 PositionValid 时可以为 true；取消/错误/位置不可用时为 false。

- X11：ButtonRelease 的最终 motion 已解析实际 XDND 目标，release 时与本进程窗口
  及注册格式核对，并保存到会话结束；不在异步 Status/Finished 后重新使用游标位置。
- Windows：QueryContinueDrag 松手时，用 GetMessagePos 的同一物理位置调用
  [WindowFromPoint](https://learn.microsoft.com/en-us/windows/win32/api/winuser/nf-winuser-windowfrompoint)，
  核对 HWND 对应的本进程活跃注册。POINT 经 `cgo.CallRet` 按值传递，
  不传指针，也不在绑定中自行判断架构或打包坐标。
- macOS：在 draggingSession endedAtPoint 回调中，用提供的屏幕位置调用
  [NSWindow.windowNumber(at:belowWindowWithWindowNumber:)](https://developer.apple.com/documentation/appkit/nswindow/windownumber(at:belowwindowwithwindownumber:))，
  与本进程注册接收类型的 NSWindow 比对。该 API 使用鼠标命中规则，会排除忽略鼠标
  的窗口；不改用后来的 mouseLocation 或历史 entered/exited。

GUI 透传该事实，与实际 `LocalDrop` 分开。TabBar 未接受结束时，只要任一为 true
就不发 DetachRequest。这是 widgets 策略，不将“应否新建窗口”下沉平台。
范围刻意是注册窗口：同一窗口非接收 Widget 区域的零动作也不会新建窗口；未注册
本应用窗口和外部窗口不凭几何猜测拒绝。曾经进入注册窗口后又离开不会永久禁止拖出。
成功投递仍以 Action/页面提交为准，目标提交期间销毁不会撤销成功。

参考 [libadwaita 1.7.0 adw-tab-box.c](https://raw.githubusercontent.com/GNOME/libadwaita/1.7.0/src/adw-tab-box.c)
的 tab_drop_performed_cb/tab_drag_cancel_cb：它依赖 GDK 通知及 NO_TARGET，并允许
在自家非标签区域创建窗口。GOUI 不照搬这项策略或 GDK 的原因分类；采用既定的本地
拒绝保留源页策略，并仅补充可验证的原生窗口事实。

测试入口：`platform/drag_end -local-target true|false` 显式断言终点事实；
`widgets/tab_detach -expect rejected` 断言协商拒绝后零新窗口请求、原对象及挂载次数
不变；`widgets/tab_detach -reject-target` 要求先经过拒绝目标再拖出，断言仍能创建
新窗口并移交原页面。两项都要求实际访问拒绝目标，且目标未收到 Drop。

### 17.1 验证记录

产物、原始日志和截图：`/tmp/goui-local-target.5ZaGbr`。传输前后 SHA256 相同：
Windows 原生探针 `a5d1d8f34b0d45be7cc5da04646ceec6a7d036bb8d45a68d4f6347830da6343e`，
Tab 用例 `d1cf502bcbe521f57055e011023f18977cc18a7ca2dc5c3198fda9951fd34606`；
macOS 原生探针 `ee90c49dd6c1fcb4168adff22ee2eadaca399520abe17b089f654de1db801bfe`，
Tab 用例 `640dc7f580053c4458b2a8defcb3d909274dc029ee7df95a9783c2b5bebfa3e7`。

- Linux/X11、隔离 Xvnc :97、Xfwm4 compositor on、显式 Software、Native 1x：
  原生 accept/action/format/drop 命中为 true，未注册 none 和 Esc 为 false；
  Tab 协商拒绝 requests=0、Mount/Unmount=1/0，先经过拒绝窗口再拖出
  requests=1、Mount/Unmount=2/1，定位独立核对相等。Integrated 逻辑 2x 拒绝通过。
- Windows 10 19045 amd64 / RDP、macOS 15.7.7 arm64 / VNC，Native 1x：
  原生 accept/action 为 true、none/Esc 为 false；实际后端分别为 Direct2D/OpenGL。
  macOS 额外验证 format 拒绝：原生未发送 Enter/Drop，仍正确报告 LocalTarget=true。
  两端 Tab 用例均通过拒绝保留原页面、经过拒绝目标后继续拖出，以及定位/源销毁隔离。
  两端另验 Integrated 1x 协商拒绝：requests=0、Mount/Unmount=1/0、Drop=0。
  Tab 用例采用默认后端选择，未单独观测最终 Painter，不能据此扩展后端覆盖结论。
- `go test ./...` 通过（最终复跑主要使用缓存），`go test -race ./platform/dragdrop
  ./gui ./ui ./widgets/... ./tests/window/platform/drag_end` 通过；两端原生包、GUI/UI、
  widgets 和窗口用例的 `CGO_ENABLED=0 go test -exec=/bin/true` 仅作为编译检查通过。
  既有图像测试仍按原有规则生成文件，没有删除断言或扩大 skip。

这次只操作自建窗口，不修改系统设置或剪贴板。没有新的 remote-computer-use CLI
错误；macOS 立即截图仍可能返回输入前画面，需结合后续截图和原生 Begin/End 日志。
Windows Run 中一条过长的连续启动命令只启动了首个用例；检查进程/日志确认后改为
逐个短命令启动，没有把输入调用成功当作后续用例通过。不是 GOUI 失败，也未据此
认定为工具实现缺陷。

本轮不代表注册窗口在原生结束查询前销毁、外部应用、多屏/真实混合 DPI、所有透明
命中与窗口遮挡组合均已验收；这些与现有更多生命周期/组合项继续分别跟踪。

## 18. 命令式转移的销毁交错验收（2026-09-30）

沿用第 3、5 节的事务边界，不新增公开接口或平台补偿。现有
`tests/window/widgets/tab_transfer` 增加两个真实输入模式：

- `-expect target-destroy`：原输入框 Unmount 回调中立即 Destroy 目标窗口。
  此时页面尚未提交，原页面恢复到源的原序号，恢复选中和可见状态；输入框身份和
  文本不变，累计 Mount/Unmount=2/1。目标确实已销毁，报告一次转移失败，且没有
  DetachRequest。不能把这次真实 Drop 的失败再解释成拖出空白处。
- `-expect source-destroy`：页面已经移交、两侧选中状态一致后，在源的选中通知中
  立即 Destroy 源窗口。成功提交不因此撤销；目标仍持有同一页面和输入框，能继续
  编辑，Mount/Unmount=2/1，零转移错误、零 DetachRequest。

这些回归检查命令式 `TransferPage`、GUI 生命周期和原生 DnD 嵌套回调的组合。
本轮未发现需要调整生产实现的缺陷。它们不代替 UI node/key 映射的销毁交错测试，
也不代表销毁目标在所有不同原生回调时刻都已覆盖。

### 18.1 验证与远程操作记录

产物、日志和截图在 `/tmp/goui-transfer-life.xmXIyr`。远端二进制 SHA256 上传前后一致：
Windows amd64 `dba7a7f1f9d35177e4e9ab0e8e44d241bb6406efbdae5ba0518749b529cd1b07`，
macOS arm64 `6dbc988c2a1df9604d8dbada658d248f49462f82263f2a810fafb7fa41c28309`。

- Linux/X11、隔离 Xvnc :97、Xfwm4 compositor on、显式 Software、Native 1x：
  两种模式均 PASS；源销毁后通过真实键盘将原输入框改成 `after source destruction`。
- Windows 10 19045 amd64 / RDP、macOS 15.7.7 arm64 / VNC，Native 1x：
  同样两项 PASS，包括源销毁后继续编辑，Verify 检查页面和输入框对象身份。
  使用默认 Painter，未独立观测后端，不能把结果扩展为所有绘制后端已验收。
- `go test ./...` 通过；Windows/macOS 窗口程序交叉构建通过并实际运行。
  本轮只修改窗口测试及文档，既有包内断言保留；未新跑 race，不将之前结果算作本轮。

remote-computer-use 调用均返回成功，但 Windows 一次 WIN+R 后立即 type，运行框尚未
取得焦点，部分文本落入 Edge 地址栏，随后出现截断路径的启动错误。未将此记为 GOUI
失败；取消错误对话框，把地址栏恢复为操作前的 `cmd`（不提交导航），再截图确认
运行框就绪后输入，同一二进制正常启动。这是输入投递成功不等于目标窗口就绪的
具体实例，技能可补充“切换焦点/打开运行框后，先观察目标再输入”的示例。

测试结束后自建应用均退出；关闭本轮 macOS 终端标签并保留原有标签，释放显式输入，
断开本轮 Windows RDP，停止本轮隔离 Xvnc/Xfwm4；不关闭原有 SSH/macOS VNC。
无系统设置或剪贴板修改。

### 18.2 UI 协调归属的销毁交错

`tests/window/widgets/ui_tab_transfer` 进一步加入 `target-destroy` / `source-destroy`：
前者在输入框 GUI Unmount 回调销毁目标并撤掉目标声明，后者在输入框已进入目标的
GUI Mount 回调销毁源，随后由 OnTransfer 更新模型并撤掉源声明。两项均执行重建和
继续编辑，独立检查原 TabPage、输入框、State 身份、文本通知连接及挂载次数。

- 目标销毁：转移失败，源仍是原 A/B 顺序，A 仍选中；零 OnTransfer、零拖出请求，
  原 GUI 输入框重新挂载（累计 2/1），View 不卸载，目标不会因重建再次出现。
- 源销毁：一次 OnTransfer，原协调记录移到目标，GUI 累计 2/1，View 不卸载。
  源的正常 UI 清理不能清理正在移交、暂由同步事务持有的记录。
- 已销毁目标的声明式信号连接会正常断开，因此不要求销毁后仍向其 OnTransferError
  发送通知；回滚正确性通过仍存活的源窗口和真实对象检查，不为测试保活失效连接。

Linux/X11、Xvnc :97、Xfwm4 compositor on、Software、Native 1x，以及 Windows 10
19045 amd64 / RDP、macOS 15.7.7 arm64 / VNC 的 Native 1x，两项均通过实际拖放、
Rebuild、键盘编辑和 Verify。远端使用默认 Painter，未独立确认后端。该增量只有
窗口测试与文档，未修改生产行为；`go test ./...`、`go test -race ./ui ./widgets/...`
通过，Windows/macOS 窗口程序构建并实际运行。非混合 DPI 验收。

产物和日志 `/tmp/goui-ui-life.ttJqnp`；远端传输前后 SHA256 一致：
Windows `e8401e0eff423728c9aed6362881a963b186f840381b8ebeca040bdefc947a3f`，
macOS `0591e568bdc1b971ad3373f87f0984105f4e1a9390dac576a9e59b17b8063475`。
读取 macOS 日志时遇到 SSH Timeout，查询确认 connected=false 后只重连 mac，
读取既有 PASS，没有因此重跑桌面操作。VNC 始终可用。运行框/终端先观察就绪再输入，
这次没有误输入；Computer Use 没有返回调用错误。清理范围与第 18.1 节相同，修复后
mac SSH 保持连接，保留原有会话可用性。

## 19. 空目标与原生拖放中的溢出滚动（2026-09-30）

扩展原有 `tests/window/widgets/tab_transfer`，不新增生产 API 或平台补偿：

- `-same-window -target empty`：同一 Window 内两个 TabView，从左侧 A 拖入右侧
  没有页面的标签栏。空栏沿用 TabBar 自身的正常测量高度，不靠额外透明命中层。
- `-target overflow`：目标 T1..T8，鼠标停在右箭头左侧的视口边缘，保持静止约
  5 秒，让计时器滚至 T8 后再松开。Verify 要求完整顺序为 T1..T8/A，不能只以
  页面进入目标判断通过；同时检查目标选中 A、源选中 B。
- `-target overflow -expect canceled`：同样先滚到末尾，保持左键时按 Esc，再松开。
  要求两侧页面顺序和选中项不变、零卸载/重新挂载、零拖出请求。

最初溢出用例没有设置窗口最小尺寸，GUI 从整排标签的期望宽度推导 WM 最小尺寸，
导致目标窗口被撑宽而没有溢出。该次只证明测试环境不满足前提，不记为滚动通过。
用例现显式 `SetMinSize(400×280 DIP)`，保持创建尺寸 480×320（同窗口 1000×320），
让真实视口约束生效；没有修改生产窗口/布局规则。Verify 保留所有既有身份和生命周期
检查，并补充源选中项检查。测试说明使用中文。

### 19.1 实测范围与结果

三个模式均通过真实桌面输入及 Verify：

- Linux/X11、隔离 Xvnc :97 1600×900、Xfwm4 compositor on、显式 Software、Native 1x。
- Windows 10 19045 amd64、RDP 1600×900、Native 1x。
- macOS 15.7.7 arm64、VNC 1650×1050、Native 1x。

远端使用默认 Painter，未独立确认具体后端，不声称覆盖所有绘图后端。同窗口空栏场景
先将输入文本改为 `edited A`，转移后文本不变，Mount/Unmount 累计 2/1；溢出接受也是
2/1，取消为 1/0。三平台的悬停期间均未补发移动事件，截图观察到 T8 从不可见变为
可见，放下按最终槽位插入；不是仅验证移动驱动的滚动。所有模式 errors/detach 均为 0。

`go test ./...` 通过（在隔离 DISPLAY=:97 下运行，既有测试仍按原规则执行，包括
图片产物）；`go test -race ./widgets/...` 通过，使用缓存。Windows amd64/macOS arm64
窗口目标交叉构建后已实际运行，不把编译作为桌面验收替代。未新增或修改生产行为。
本轮未验 Integrated、2x/物理混合 DPI、同一 TabView 关联多个 TabBar 或 UI 同 Root
协调；这些仍是后续组合，不由本节的两个独立 View 验收替代。

产物、截图和日志位于 `/tmp/goui-tab-combinations.tanue3`，远端传输前后 SHA256 一致：
Windows `b64107c55d3c696c1bcd9592c2e6d500035e280703c737e9a156468551c4d4e1`，
macOS `c02aa3593ebd6cbdc8d900bbbe65645f0eb97b79e78d9c5aa1599bb14c097ed7`。

### 19.2 工具和清理

通过 remote-shell 传输、校验和读取日志，remote-computer-use 执行按下/移动/静止悬停/
Esc/松开，finally 释放输入。本轮没有工具调用错误或连接故障；即时截图偶尔仍为
动作前画面，重新截图观察到运行框或终端提示符后才输入，没有误输入或重复启动。
这再次说明输入 ACK 不能当作界面已经完成切换的证据。

自建测试应用均退出，关闭本轮 macOS 终端标签并保留原四个标签，释放显式输入，
断开本轮 Windows RDP；隔离 Xvnc/Xfwm4 会话已终止。保留原 SSH/macOS VNC 会话。
未改系统设置或剪贴板，未提交代码。

## 20. 同一 View 多栏的原生插入位置（2026-09-30）

原生接管后源页面仍属于原 View，源栏和其他关联栏仍显示该标签。因此计算鼠标命中时
不能跳过它的宽度；只有计算最终 MovePage 索引时才应排除它。旧实现同时跳过两者，
使 A/B/C 中将 A 放到 B 后面变成放到 C 后面。固定 100 DIP 宽、4 DIP 间距的回归
先复现了 A-before-B、A-after-B、B-before-C 三个失败，再验证七组独立预期均通过。

`TabBar.incomingSlot` 统一返回最终索引和内容坐标边界；遍历全部可见槽位，逻辑索引
不重复计入源页面。命中和插入标记共享该结果，滚动时从当前指针重新计算；删除冗余
`dropIndex`，最终 Drop 仍按释放位置重新命中。动画中的 item 坐标不参与槽位计算。

参考 libadwaita 1.7.0 的 [adw-tab-box.c](https://github.com/GNOME/libadwaita/blob/1.7.0/src/adw-tab-box.c#L1989)
`calculate_placeholder_index` / `insert_placeholder`：其索引与实际标签宽度、偏移及
占位项关联。GOUI 采用几何与显示一致的原则，但保留“成功放下才移交”的设计，
不引入其提前脱离源 View 的所有权流程，也不把插入线替换成整页占位所有者。

窗口用例 `tab_transfer -same-window -target mirror` 中两栏共享一个 View，页面只挂载
一次。真实拖动 A 到右栏 B 的右半侧，两个栏应同时成为 B/A/C，Moved(0,1) 恰好一次，
不触发 CloseRequest，输入内容保留且 Mount/Unmount=1/0。另启 `-expect canceled`，
同路径按住时 Esc，要求 A/B/C 不变、零 Moved、生命周期仍为 1/0。

实测通过：Linux/X11 隔离 Xvnc/Xfwm4、Software、Native 1x；Windows 10 19045 amd64
RDP、macOS 15.7.7 arm64 VNC 的 Native 1x。远端使用平台默认 Painter，本轮未独立查询
实际后端，不能把默认推断当作后端观测。Linux 另复验外来 View 的溢出栏静止悬停滚动
后尾插，结果和生命周期 2/1 正确。该轮未验这些组合的 Integrated 或 2x/混合 DPI。

`go test ./widgets -run 'TestIncoming|TestTransferPageValidationAndSameView' -count=1`、
全仓测试、widgets race 通过；测试覆盖滚动、视口夹取及动画坐标偏离。旧失败、修复后
日志与截图保存在 `/tmp/goui-tab-slots.GctVmV`。远端核对 SHA256：
Windows `18a09eeb1c03700bd790f5f7e0aa39c701dc5cdacc22a4007f360a67d4c5f9c2`，
macOS `979f02c1e0fd6d14e82fb321924abc20d9cdbf8324a2de1ba6ef4d438f579347`。

## 21. UI 同 Root 的协调与源容器卸载（2026-09-30）

`ui_tab_transfer` 增加 `-same-window`：同一窗口 main 内用 source/target 两个 View ID
区分声明集合；OnTransfer 必须报告相同窗口 ID、不同 View ID。`-target-first` 交换
两个容器的声明顺序，同时验证源先更新、目标先更新。转移仍经过原生输入/GUI 拖放及
现有 Coordinator，不新增 UI 内核分支。该模式仅支持 transferred/canceled，其余
销毁交错与新窗口准备继续由原跨窗口用例承担。

验收固定把 A 放到 C 后面，检查源 B、目标 C/A、目标选中原 A；重建后继续编辑为
after move，再移除源容器。要求原 TabPage、TextInput、State 与 ID 查找保持一致，
一次 OnTransfer、GUI Mount/Unmount=2/1、View Unmount=0；源 B 的 View 释放一次，
源控件脱离 Root/父节点，目标不受影响。保留外层布局槽位，避免把普通位置协调时
移动整个容器的语义混入页面转移。Esc 则要求零转移及 GUI 生命周期 1/0。

### 21.1 实测与纠正过程

- Linux/X11、隔离 Xvnc/Xfwm4、显式 Software、Native 1x：两种顺序均完成真实拖动、
  重建、继续编辑与源容器卸载；Esc 通过。
- Windows 10 19045 amd64 / RDP 1600×900、macOS 15.7.7 arm64 / VNC 1650×1050，
  Native 1x、平台默认 Painter：同样三组通过。远端未独立查询 Painter 类型。
- 全仓 `go test ./...`（DISPLAY=:97）与 `go test -race ./ui ./widgets/...` 通过，
  多数包使用缓存；Windows amd64 / macOS arm64 的 CGO_ENABLED=0 构建通过，随后
  实际运行校验哈希的窗口程序，不以编译代替桌面验证。

新增用例最初的 Fill 容器未限制宽度，导致左栏占满，修正为两个 520 DIP 的固定测试
槽位后重跑。初次源卸载断言错误地要求 Destroyed，后又要求已断开观察器记录 GUI
Unmount；日志证明转移状态全部保留。依据 `WidgetBase.Destroyed` 与 `ui.root.release`
的契约改为检查 Root/Parent、B 的 View 释放及实际脱离，未修改内核或删除目标状态断言。
早期失败/中止记录保留，不记作通过。测试文件还修正过新增代码的类型和参数编译错误。

本轮没有修改 GUI/UI 的生产代码。Native 同 Root 的通过不代替 Integrated、2x/真实
混合 DPI 或其他混合所有权场景。证据保存在 `/tmp/goui-tab-slots.GctVmV`，UI 程序哈希：
Windows `906e6ccde739dabdc3e0dd168cf31582351caa0fc096be4c165190f180891c6c`，
macOS `799ce77df6cb10d19efa8537e60990ebe2a4c11dbbc5ce2c4d801e71729ee8d7`。
哈希对应远端实测程序；之后仅补充测试说明注释和本文。

remote-shell 用于传输、哈希与日志；remote-computer-use 用于实际输入及截图，显式
按住后 Esc 和 finally 释放输入均正常。本轮无调用错误；Windows Run 弹窗和 macOS
拖放后的即时截图有时仍是旧帧，重新观察后再操作。技能仍应强调输入 ACK 不代表绘制
已完成，不能据旧帧重复启动或重复拖放。既有全仓图像测试仍按其原规则生成文件。

交付前另一次全仓测试遇到 `TestTypography: can not open display`；检查确认自建
Xvnc :97、Xfwm4 进程和显示 socket 已结束，原会话句柄也返回终态。恢复同配置隔离
会话后重跑 `go test ./...` 通过。保留 `go-test-delivery.log` 的环境失败与
`go-test-delivery-restored.log` 的通过记录，不修改测试或添加跳过。

自建远端测试程序已退出；释放所有显式输入，关闭自建 macOS 第五个终端标签，保留
原四个标签和 SSH/macOS VNC 会话；断开本次 Windows RDP。未修改系统设置或剪贴板，
未提交代码。交付测试结束后，按已核对的 PID 终止恢复的隔离 Xvnc/Xfwm4，确认句柄
返回终态且进程已退出；不影响用户原有桌面。

## 22. Integrated 多栏与同 Root 验收

本切片只补充窗口用例，不改生产接口：命令式 `tab_transfer` 增加 `-chrome integrated`，
将每个 TabBar 放进 HeaderBar；UI 同 Root 用例同样将两侧 TabBar 放进各自的 HeaderBar，
并通过 CrossStretch 保证两栏位于顶部。保留页面身份、完整顺序、生命周期和通知断言。

实测矩阵：

| 环境 | 命令式同 View 多栏 B/A/C | 按住鼠标 Esc | UI 同 Root、target-first 转移/重建/编辑/移除源 | UI Esc |
| --- | --- | --- | --- | --- |
| Linux/X11、Xvnc/Xfwm4 合成器、Software、Integrated 1x | 通过 | 本切片未测 | 本切片未测 | 本切片未测 |
| 同环境、逻辑 2x、2560×1200 桌面 | 通过 | 通过 | 通过 | 通过 |
| Windows 10 19045 amd64、RDP、Integrated 1x | 通过 | 通过 | 通过 | 通过 |
| macOS 15.7.7 arm64、VNC、Integrated 1x | 通过 | 通过 | 通过 | 通过 |

远端使用平台默认 Painter，未独立查询其实际类型。命令式用例断言 Moved 正好一次、
Mount/Unmount=1/0，取消为零次 Moved；UI 断言原页面/输入框/State 保留、GUI 生命周期
2/1、View Unmount=0，取消为 1/0。两端拖动前后截图中的窗口位置未变化。
此次未验收空白 Caption 拖窗，不以标签没有移动窗口反推 Caption 正常。
逻辑 2x 不是物理跨屏混合 DPI；Integrated 的同 Root source-first 顺序亦未在本切片验收。

`go test ./...` 初次失败于 `TestTypography: can not open display`，确认隔离 Xvnc 和
Xfwm4 已退出后恢复会话，重跑通过。`go test -race ./ui ./widgets/...` 通过；多数包
使用缓存，既有全仓图片测试仍按原规则生成文件。Windows amd64/macOS arm64 的两个
窗口程序均 CGO_ENABLED=0 编译通过，远端实际运行的程序已校验 SHA256：

- Windows GUI：`359932039bcf3886b5c18cdefab9f30972069ae13529bba714e70be90d437414`
- Windows UI：`0f6e71a1776bdacfa5a441283abf6021f9d4d37a8dbce532bc10566e919d6eda`
- macOS GUI：`06dc980dd5049631ea863722432cd839113151416df77ef2eba1c7d5cc51748f`
- macOS UI：`a1f9c5a9b11af7997c5a4ecfd92a39ef4b8b5e5af0ca502a4f623b574e620032`

日志、截图和本地二进制保存于 `/tmp/goui-tab-integrated.TD3BUv`。remote-shell 用于
核对进程、程序哈希及断言日志；remote-computer-use 用于真实输入，按住鼠标插入 Esc
及 finally 释放输入均正常，没有新增工具调用问题。此次仅操作测试窗口，不改系统设置
或剪贴板，不使用 DevServer，不提交代码。此矩阵不是全部转移需求的完成声明。

所有本轮测试进程已退出。已关闭自建 macOS 第五个终端标签、断开本次 Windows RDP，
保留原四个终端标签及既有 SSH/macOS VNC；按核对的 PID 终止隔离 Xvnc/Xfwm4，
确认进程退出。不影响用户原有 Linux 桌面。

## 23. 最后一个编辑页与编辑状态保留

`widgets/tab_transfer_test.go` 增加成功移交与显式拒绝的包内回归：保留原 TabPage、
ScrollView、TextView、TextModel、反向选区、滚动和撤销/重做记录；最后一页离开后，
源 Pages、Current、Snapshot 子树均为空。包内使用无原生窗口的宿主，不据此宣称
真实窗口销毁或原生输入已验收；销毁交错仍由第 18 节承担。

`tests/window/widgets/tab_transfer -editor` 使用公开 API 准备文档，然后由真实拖放移交。
Check 在点击编辑区之前检查选区和滚动，随后显式销毁源窗口；用户在目标通过键盘
撤销、重做、输入 `!`，Finish 检查原编辑器的 Change 通知及每一步文本。测试不通过
直接调用 Model.Undo/Redo 冒充键盘验收。具体操作见该目录 `editor.go` 的中文注释。

实测结果（Integrated）：

| 环境 | 最后一页移交、源关闭、撤销/重做/继续输入 | 按住鼠标 Esc，状态不变 |
| --- | --- | --- |
| Linux/X11、隔离 Xvnc/Xfwm4 合成器、Software、1x | 通过 | 早期第 13 行选区版本通过，最终版本本轮未重跑 |
| 同环境、逻辑 2x | 通过 | 通过 |
| Windows 10 19045 amd64、RDP、1x | 通过 | 通过 |
| macOS 15.7.7 arm64、VNC、1x | 通过 | 通过 |

远端使用默认 Painter，未独立查询具体后端。成功时 GUI Mount/Unmount=2/1，取消为
1/0；最终版本选区为 Anchor=480/Caret=468，滚动为 240 DIP。快照检查目标选中页和
原编辑器子树，源为空；源窗口由 Check 关闭，不是 TabView 自动关闭。

用例纠正过程：最初把选区放在第一行，待处理的光标可见请求令滚动回到 0；移动到
第 13 行后 Linux/Windows 通过，但 macOS 字体行高下滚动为 224。最终选择第 15 行，
并在 Prepare 中立即断言滚动为 240，分离测试准备失败与转移失败；未放宽状态断言，
未修改生产代码。macOS 另一次首击仅激活窗口、未完成 Prepare 的操作失败也保留日志。

全仓 `go test ./...`（DISPLAY=:97）、`go test -race ./widgets/...` 通过；Windows
amd64/macOS arm64 的 CGO_ENABLED=0 构建通过。远端运行的最终程序 SHA256：

- Windows：`744f67a4ded35292a40d1430ae9b54e0baa779ac24d594ff61969e7a92f2dcaa`
- macOS：`8399623451062fd29548d95ee4aa01cb7a677698aef911b4bf2ca8a606ad10cd`

日志、截图和程序保存在 `/tmp/goui-tab-editor.jhikur`，包括失败与修正后的记录。
remote-shell 用于传输、哈希和日志，remote-computer-use 用于真实拖放与键盘。
调用时序问题：WIN+R 返回成功不代表运行框已经取得焦点，立即输入曾部分误入浏览器
地址栏；重新截图确认焦点后启动成功，地址栏已恢复原文本，未执行页面导航。应当在
启动器输入前确认新截图，而不是把输入 ACK 当作窗口状态确认。没有修改系统设置或
剪贴板，不使用 DevServer，不提交代码；既有全仓图像测试仍按原规则生成文件。

本切片不代表 UI 多行编辑页重建、拖动中布局变化或真实跨屏混合 DPI 已验收。

## 24. 拖动期间目标重新布局

新增包内回归 `TestIncomingDropRevalidatesAfterLayoutOrSourceChange`：不再发送 Motion，
将目标槽位从 100 DIP 改为 140 DIP，检查正常 Arrange 更新插入线、Drop 重新计算
位置；另一分支移除源页，检查不接受失效页面、不改变目标。测试只直接调用被测的
内部算法，不据此宣称覆盖原生事件分发。

窗口用例 `tab_transfer -target relayout -chrome integrated` 从已有 DropTarget 的
Enter 信号请求宽度变化，正常布局后保持指针位置再释放。Verify 独立断言目标顺序
为 T1/A/T2/T3、原页面身份、选中项和 Mount/Unmount=2/1。`-expect canceled`
要求宽度变化确实发生，之后按住鼠标 Esc；顺序不变、生命周期 1/0、零拖出请求。
三平台 Integrated 1x 的正常投递及 Esc 均通过：Linux/X11、隔离 Xvnc/Xfwm4、Software；
Windows 10 19045 amd64、RDP；macOS 15.7.7 arm64、VNC。远端默认 Painter 未独立查询。
本次没有修改生产代码，未验证此组合的 Native、2x、混合 DPI 或拖动中关闭整个窗口。

首次 Linux 验收在按住鼠标时停留过久，返回 `x11 dragdrop: native session timed out`；
日志保留为 `linux.log`。短暂等待布局后重跑通过（`linux-retry.log`），不能将其解释为
长时间拖放正常。代码检查发现现有源会话将 10 秒无进展、30 秒总时长用于整段拖动，
而非仅用于协议应答等待。此限制需另行核对 GTK/Qt 的 Xdnd 做法并修正、验证，不能
通过要求用户加快操作规避；本轮尚未修复，也没有放宽测试断言。

`go test ./...`（DISPLAY=:97）、`go test -race ./widgets/...` 通过；Windows amd64、
macOS arm64 的 CGO_ENABLED=0 窗口程序构建通过，运行前核对远端 SHA256：

- Windows：`8142580180f966b1e4386b73433421381d6b66b25cfa25db322e1487c1219162`
- macOS：`45ae34774b55811ddb4daa7b2fb8fbad00d86a47df59294c937b532262589975`

证据保存在 `/tmp/goui-tab-relayout.ypAYWl`。remote-shell 用于程序、哈希和日志；
remote-computer-use 完成远端真实输入，显式按住、Esc 和 finally 释放均正常。
即时截图有旧帧，重新截图确认启动器/终端再输入；本轮没有误输入或新工具错误。
未使用 DevServer、修改系统设置或剪贴板、提交代码。全仓已有图片测试按原规则运行。

交付前再次运行全仓测试通过（多数包使用缓存）。远端测试进程已退出，自建 macOS
第五个终端标签已关闭，原四个标签保留；Windows 临时 RDP 已断开，既有 SSH/macOS
VNC 保留。核对 PID 后终止第 23 节沿用的隔离 Xvnc :97，确认其及 Xfwm4 都已退出；
未影响用户原有 Linux 桌面。

## 25. 长时间拖动与释放后协议超时

第 24 节发现的按住超时已修复。`xdndSource` 不再从 Begin 开始限制整个操作：
按住时，即使暂时没有 Status 应答也不创建过期计时器，用户可以继续移动或 Esc。
ButtonRelease 记录独立起点；仅等待最终 Status 或 Finished 时启用现有的 10 秒
无进展/释放后 30 秒上限。迟到的计时任务在 GUI 线程重查当前源和应答期限，不能
把已清除或延长的等待当成超时；结束时停止并清空计时器。不改变公共结果契约。

参考证据与取舍：GTK 4.20.0 的
[X11 motion/drop 实现](https://github.com/GNOME/gtk/blob/4.20.0/gdk/x11/gdkdrag-x11.c#L1347)
把拖动协商与释放提交分开；Qt 6.8.3 的
[drop](https://github.com/qt/qtbase/blob/v6.8.3/src/plugins/platforms/xcb/qxcbdrag.cpp#L435)
在投递后建立 transaction 并为外部目标启动清理计时，
[timerEvent](https://github.com/qt/qtbase/blob/v6.8.3/src/plugins/platforms/xcb/qxcbdrag.cpp#L997)
清理这些未结束事务。GOUI 采用区分用户操作时长与协议等待的原则，不照搬事务模型，
也不宣称 10/30 秒是协议规定或上述框架的公共保证。

包内新增回归验证按住不创建计时器、Status/Finished 期限、进展延长但不超过完成
上限，以及等待结束/源失效后的计时器清理。旧实现在 idle/awaiting-status 两分支
均失败（`regression-before.log`），修正后通过。窗口探针新增 `-min-duration`，
按 Begin 到 End 的单调时钟断言时长，保留既有 payload、结果和事件次数检查。

本轮真实输入结果（均 1x，未使用 DevServer）：

- Linux/X11、隔离 Xvnc :97、Xfwm4、Software、Native：接受 36.14 秒、
  未注册目标 36.00 秒、Esc 36.00 秒均通过；等待期间没有指针移动。
- Windows 10 19045 amd64、RDP、Direct2D、Native：接受 36.03 秒、
  Esc 36.04 秒通过。
- macOS 15.7.7 arm64、VNC、OpenGL、Native：接受 36.01 秒、
  Esc 36.58 秒通过。
- Linux Integrated 的 `tab_transfer -target relayout`：目标变宽后等待 36 秒再
  放下，Verify 检查 T1/A/T2/T3、原页面/输入框及 Mount/Unmount=2/1，通过；
  同样长悬停后 Esc 也通过，Mount/Unmount=1/0、零转移错误/拖出请求。

remote-shell 用于文件哈希和日志；remote-computer-use 用于远端实际输入。
技能明确按住输入有 30 秒空闲租约，因此远端每 15 秒发送同位置 move 续租；这不能
等同于完全没有原生 Motion，完全静止由本地 XTest 场景覆盖。两端接受/取消都在
finally 释放输入，未出现新工具调用错误；启动后先等新截图确认窗口再操作。

### 25.1 无应答目标的部分通过与新增失败

分别启动 source/target 进程；进入目标后只 SIGSTOP 测试目标，移动后等待 12 秒，
断言源尚未 End。随后释放，约 10 秒后源报 `native reply timed out`，单次 End、
无取消/成功/有效位置，源端公开结果断言通过。恢复目标后却收到
`BadWindow (X_SendEvent, opcode 25)` 并退出：目标向已销毁源发送迟到 Status。
因此该双端场景尚未通过，不把目标崩溃解释为预期，也不以源通过隐藏目标失败。
目标已经恢复并退出，没有遗留暂停的进程。原日志为 `linux-silent-source.log` 与
`linux-silent-target.log`。

下一步需补对端消失的请求级错误处理。GTK 4.20.0 的
[`_gdk_x11_send_client_message_async`](https://github.com/GNOME/gtk/blob/4.20.0/gdk/x11/gdkasync.c#L128)
按具体发送请求识别 BadWindow；GOUI 尚无对应保护。实现须遵循既有 libs/xlib
绑定规范，保留其他错误处理，避免全局吞错、存活预查竞态或为每次 Motion 强制
同步。还要重新验证目标恢复后仍可继续处理窗口事件；本轮未修改这部分原生绑定。
Finished 无应答只覆盖期限逻辑，尚未单独进行桌面故障注入。

全仓 `DISPLAY=:97 go test ./...`、X11/dragdrop/gui/widgets 的 race 测试通过，
Windows amd64/macOS arm64 探针构建通过。两端执行文件 SHA256 已再次核对：

- Windows：`909b83604ca918d646aab011c99d6a6d3d113b99390145a7cc50eae20aadf2e5`
- macOS：`49b568899a24ab601b2e6172bfccb5983b0dd4f936a5b2e2fc0d425baa8fc8f9`

产物在 `/tmp/goui-tab-timeout.ouLwpR`。Linux 首次取消测试因隔离 Xvnc 退出而中断，
保留 `linux-cancel.log`，确认进程终止后重建隔离显示，`linux-cancel-retry.log`
才是通过证据；退出原因未确定，不能归为 GOUI 取消失败或成功。既有全仓图片测试
仍按原规则运行；未修改系统设置/剪贴板，未提交代码。远端测试程序已退出，自建
macOS 第五个终端标签已关闭、原四个标签保留；临时 Windows RDP 已断开，既有
SSH/macOS VNC 保留。本切片不代表真实混合 DPI 或所有外部目标生命周期已验收。

交付前再次运行全仓测试与针对性无显示 X11 逻辑回归通过。所有本轮窗口程序已退出，
核对 PID 后关闭隔离 Xvnc :97 与 Xfwm4，确认进程消失；用户原有 Linux 桌面未受影响。

## 26. X11 迟到消息修复与主要功能交付

第 25.1 节的已知崩溃已修复。沿用 GTK 4.20.0
[`_gdk_x11_send_client_message_async`](https://github.com/GNOME/gtk/blob/4.20.0/gdk/x11/gdkasync.c#L128)
按具体请求处理对端消失的原则，不复制其 Xlib 私有异步结构。现有 `libs/xlib`
补充错误回调与请求序号绑定；Platform 在发送 Xdnd ClientMessage 前记录序号和目标，
仅消费 display、serial、resource、BadWindow、SendEvent opcode 全部吻合的错误。
其他错误仍交给原 handler，不全局忽略 BadWindow，不用存活预查替代竞态保护。

使用 Xlib 已知服务器处理序号回收记录，没有增加逐次 XSync 或服务器往返。
回调只更新请求记录，不发出 X 请求或调用 GUI；归属与现有进程期 Platform/display
一致。依据为 [Xlib 的错误处理及 Display 宏契约](https://xorg.freedesktop.org/archive/current/doc/libX11/libX11/libX11.html)。
此保护仅限上述消息，不宣称已处理所有属性读取、Selection/INCR 对端消失场景。

验证结果：

- 包内覆盖 XErrorEvent LP64 ABI、逐字段错误过滤、原 handler 转交和记录回收；
  X11/xlib/dragdrop/gui/widgets 针对性 race 通过。
- Linux/X11、隔离 Xvnc :97、Xfwm4、Software、1x：再次暂停独立目标，源释放后
  超时并退出，再恢复目标。目标没有像旧版那样因 BadWindow 退出；新源向同一目标
  投递成功，双方 payload/结果断言通过，最后正常关闭目标。原失败记录保留在第 25 节。
- Linux 同环境、Windows 10 19045 amd64、macOS 15.7.7 arm64 的 Integrated 1x
  跨窗口接受与按住时 Esc 回归均通过：接受 Mount/Unmount=2/1，取消=1/0，原输入
  对象/文本保留。远端使用默认 Painter 选择，本轮未独立查询最终后端类型。
- `DISPLAY=:97 go test ./...` 通过（交付复跑多数包使用缓存）；Windows amd64、
  macOS arm64 窗口程序交叉构建通过，远端传输哈希一致后才进行桌面操作。

证据目录 `/tmp/goui-tab-peer.vcmbqk`。远端使用 remote-shell 取结果、
remote-computer-use 进行真实拖动和 Esc；未使用 DevServer。动作后即时截图有旧帧，
以新截图和程序断言确认结果；没有新增工具缺陷。一次调用误写 CLI 路径返回 127，
属于调用方拼写错误，实际清理使用正确路径完成。macOS 自建第五个终端标签关闭，
原四个标签及 VNC/SSH 保留；临时 Windows RDP 断开。既有全仓图片测试按原规则
运行；未提交代码、未改系统设置或剪贴板。

主要功能的依据为第 13–24 节的实现与实际验收，包括 GTK4/libadwaita 参考、
原手势接管、原页面与 UI 子树转移、应用新窗口准备及失败保留源页面。
本轮再次核对命令式与 UI 拖出日志，三平台正常拖出、取消/准备失败、UI 重建及源关闭
均有通过记录。按用户最新要求，到此交付主要功能，不继续为边缘组合追加开发切片。
混合 DPI 和未执行的外部互操作仍如实保留为后续项，不构成已覆盖的承诺。

收尾已核对并结束自建 Xvnc :97 / Xfwm4，测试应用均已退出；用户原有 Linux
桌面不受影响。`git diff --check` 通过，工作区保留未提交改动。

## 27. 单标签交互与 X11 预览修正（2026-09-30）

> 阶段记录：本节“单标签仅本地拖动”的限制已由第 28 节修正；X11 预览与光标分离保持不变。

用户实测指出拖出时鼠标消失、只剩一页仍可拖出，以及例程连续操作退出。
X11 原实现用整张预览作为 cursor，已改为独立无输入预览窗口和主题动作光标，详见
[DnD 设计](DesignDragAndDrop.md#83-linuxx11)。不改变 Windows/macOS 原生预览机制。

TabBar 在接管手势前检查页面数量；只有一页时沿用本地跟手/回位动画，不创建原生会话。
待处理的新窗口请求提交前也重新检查数量，防止准备期间源已变成单页仍移走它。
包内真实派发路径回归先确认旧实现越界后报 RenderWidget 错误并结束本地拖动，再验证
修正后跟手、无转移/请求及释放归位。显式最后一页 TransferPage 的状态保留测试继续保留。

`tab_detach` 原来的第二次请求触发 duplicate detach request，属于严格一次性用例的
退出策略，而非正常窗口生命周期。现在 `tab_detach` 与 `tab_transfer` 默认连续体验，
提供 Add tab，可以反复移动/创建窗口；关闭全部窗口才退出。严格断言保留，分别显式使用
`-expect detached` / `-expect accepted`，取消/拒绝等原有模式不变。
编辑页原生拖放用例增加一个留在源的 Remaining 页，以符合单标签交互约束；仍检查原编辑器、
选区、历史、滚动及源关闭后的输入，不通过删除断言放宽状态保留契约。

本轮验证：

- `DISPLAY=:97 go test ./...`、`go test -race ./widgets ./platform/linux/x11` 通过。
- Windows amd64、macOS arm64 两个窗口程序交叉编译通过；本轮未执行远端桌面操作。
- 本机隔离 Xvnc / Xfwm4 合成桌面、Native、1x、默认 Painter 选择：独立 240×32
  预览窗口与 XFixes 查询到的 24×24 主题光标同时存在；未独立探测最终 Painter 类型。
- 连续拖出、剩余单标签释放不新建窗口/不退出、添加后再次拖出、Esc 取消已操作；
  跨窗口转移及往返通过。严格 accepted 输出 PASS，Mount/Unmount=2/1，errors=0，
  原文本 retained text 保留。证据位于 `/tmp/goui-tab-fix.KpMXE8`。
- 无合成器的二值 shape 仅完成像素/覆盖范围单测，未完成窗口外观验收；2x 和混合 DPI
  未重跑，不外推本轮 1x 结果。既有全仓图片测试仍产生原有测试产物。

验证过程另有一次旧连续体验进程返回 143（SIGTERM），随后向其旧 XID 执行的
`xdotool windowactivate` 报 BadWindow；未取得终止来源证据，不把它归因于本次修复。
重新启动最终构建后，跨窗口往返、单标签拖动及逐个关闭全部窗口正常，进程返回 0。
严格 accepted 亦返回 0。自建 Xvnc :97 / Xfwm4 已清理，用户原桌面未操作。

## 28. 推开式占位、源标签隐藏与单标签转移（2026-09-30）

本次只改 widgets 的展示/交互状态及窗口例程，不修改平台 DnD、Painter、公共 Widget
或 UI 协调接口。参考 [libadwaita 1.7.0 的 AdwTabBox](https://github.com/GNOME/libadwaita/blob/1.7.0/src/adw-tab-box.c)
中 `reorder_placeholder`、宽度分配和重排动画的做法，采用占位让位的视觉处理，不采用
其提前脱离源页面的归属流程。GOUI 仍在成功 Drop 时才提交原页面转移。

### 28.1 目标占位

- 删除装饰性的插入竖线和 `tabInsertion`。空槽是 TabBar 的私有布局状态，不创建
  TabPage、不挂载内容，也不向 Snapshot 添加假标签或交互动作。
- 外来页面按目标栏的等宽、最小宽度和溢出规则增加一个槽；宽度与位置复用既有
  140ms 缓出动画。退出目标后动画收回。占位计入滚动范围和边缘自动滚动，不增加
  宿主的自然期望尺寸，避免悬停时要求窗口变大。
- 命中使用最终宽度、稳定的规范槽位中点和当前滚动量，不读取动画中间的 Widget
  矩形；布局改变或 Drop 时重新计算。指针不动时不会因让位动画自行切换槽位。
- 拖回源栏、或进入展示同一个 TabView 的另一栏时，将原槽移至目标位置，而不是
  同时留下原槽与一个新槽。页面数量和真实顺序在成功放下前不变。
- Snapshot 中真实标签的边界与动画中的绘制、Pick 共用实际 Arrange 矩形；占位不
  获得页面身份。拖入预览及收回动画期间不绘制标签间分隔线。

### 28.2 源标签与生命周期

- 先 RenderWidget 生成图片，再请求原生接管；直到 DragSource 的 Begin 通知才隐藏
  源 tabItem 子树，避免 SetVisible 取消尚未移交的原手势。
- 隐藏范围包括标签背景、文字、图标和关闭按钮；不改变 TabPage 的可见性、归属或
  Current，正文不卸载。源仍保留一个完整槽位；隐藏前保存标签测量高度，避免仅剩一页
  或较高标签隐藏后使 HeaderBar 收缩。隐藏标签快照不再暴露可点击的子内容。
- 取消/未接受/失败恢复标签可见性，成功移交才删除源槽。会话可以同步结束，因此恢复
  同时覆盖 Begin 返回错误和 End 通知，不以异步结束作为假设。

### 28.3 单标签策略与测试修订

取消手势接管处的页数限制。单标签可拖到同应用已有目标栏；仅 `TabDetachRequest`
的创建和提交保留“源页数大于一”检查，因此在桌面释放不会另建窗口。
默认连续例程更新中文操作说明，编辑器用例恢复“唯一 Editor 页转到空目标”的场景，
保留选区、滚动、历史、对象身份、空源快照和源关闭后的继续编辑断言。

新增/调整逻辑回归覆盖：目标空槽推开和收回、宽度动画、等宽压缩与溢出、同 View
无重复槽、布局改变后的新位置、源标签隐藏/恢复/命中/快照、单标签失败恢复、单标签
不发新窗口请求。保留旧竖线测试的槽位和重算断言，只替换竖线几何为占位几何。
新增高度测试曾发现隐藏 Widget.Measure 返回零而导致 48 DIP 高度降到 32，已改为
隐藏前保存高度并复测通过，没有放宽断言。

### 28.4 验证结果

- `DISPLAY=:97 go test ./...`、`go test -race ./widgets/...` 通过。
- Windows amd64 / macOS arm64 的 tab_transfer、tab_detach、ui_tab_transfer 交叉
  编译通过；没有把编译当作原生桌面验收，本轮未操作远端。
- 本机 Linux/X11、隔离 Xvnc + Xfwm4 合成桌面、Software、Native、1x：观察到源标签
  留空槽且正文仍显示，目标推开占位随位置切换，Esc 后两边恢复；正常转移后剩余一页
  在桌面释放恢复，再拖入另一窗口成功，源空栏及窗口保持存活。
- `tab_transfer -expect accepted` 输出 PASS，Mount/Unmount=2/1、errors=0，原文本
  retained text 保留。截图和测试日志在 `/tmp/goui-tab-gap.i4eBy8`。
- 最终新增的高度保留与自然尺寸不扩张修正经单测验证；2x/混合 DPI、OpenGL/Direct2D
  桌面、本轮恢复的 Editor 唯一页完整撤销/重做操作未重跑，不标记为本轮已验收。
- 自建窗口、Xvnc :97 与 Xfwm4 在验证后清理；用户桌面未操作。全仓既有图片测试仍按
  原行为运行，未迁移其产物规则。

## 29. 普通页面节点与局部 ID（2026-10-01）

### 29.1 最终身份模型

不把 ID 扩展成应用全局身份，不引入独立 Key。非空 Widget ID 在 Root 内唯一，
Window ID 在应用内唯一，App.FindWidget 仍显式传 windowID/widgetID。
同一挂载目标的子列表按非空 ID＋具体 WidgetView 类型匹配；无 ID 时按位置＋类型匹配。
改变声明 ID 或类型会重新创建节点；Name/Style/Text 不改变身份，GUI SetID 只是改名。
同 ID 的声明换父级、Root 不自动认领旧节点，只有明确移交保留原节点。
这是采用 Flutter 父级局部身份匹配而不采用 GlobalKey 的取舍，参考与迁移说明见
[Widget 身份设计](DesignWidgetIdentity.md#ui-局部身份与状态)。

TabPageView 嵌入 ViewBase，直接对应 widgets.TabPage；空内容仍有页面节点。
构造参数 `TabPage("document-32", content)` 就是页面 ID，与 Widget.ID 一致。
同 Root 多个 TabView 的页面 ID 不得重复，不同 Root 可以重复，但向已有同 ID 的
Root 移交会被拒绝，不覆盖、不重命名。应用自身的业务文档标识可独立保存，
并不要求所有业务文档实例都用相同 Widget ID。

### 29.2 协调与通知

TabView 通过普通 UpdateChildren 和私有 tabPages 容器适配器维护页面顺序，
TabPage 自己 UpdateChild 管理内容。适配器仅转接既有页面 API；没有 page/key 双向映射。
Container 增加 MoveChildBefore，nil sibling 表示末尾，重排不解除挂载。
用于移交的非 Widget 容器可实现 TransferContainer.Widget 报告所属 Widget；
保留其空注册以接收原节点，普通动态行空目标仍释放。

Coordinator 不再提供 Lookup(any State)。TransferChild 的对象是已协调的普通子 node，
目标也须是同一 UI 应用内的真实节点与已注册的所属容器。它先验证归属、ID、环和重入，
暂时从源列表取出 node，再执行 GUI 事务，按实际 Parent/Root/销毁状态采用或恢复节点。
返回 nil 却没有建立目标归属视为失败；孤儿或已销毁节点只释放，不复活。
恢复／采用会使旧 AfterUpdate 作废，异常做归属清理后继续传播。

成功移交保留页面、子树、State、信号与共享绑定快照，不调用 View Mount/Unmount。
GUI 的 Unmount/Mount 正常发生，焦点和窗口绑定资源仍按 GUI 生命周期释放、重建。
`committed` 在采用成功后调用源 OnTransfer，应用更新两侧声明集合并 RequestUpdate；
AfterTransfer 使两侧的选择回调在此之后执行，已销毁或过期 owner 跳过。
不允许同步嵌套协调，不建立页面所有者、跨窗口 ID 注册表或 UI 平行状态机。

### 29.3 API 迁移和验证

`TabTransfer.Key`／`TabDetachRequest.Key` 改为 `PageID`；Current/OnCurrent/OnMoved/
OnCloseRequest 的字符串含义也统一为页面 ID。普通应用只维护自己的声明集合，
不使用 Coordinator。TabBar 通过页面 ID 传递准备请求；真正的 UI 归属验证在提交时执行。

包内回归覆盖局部 ID 重排／插入／删除、类型与 ID 改变、GUI 改名、重复 ID、跨父级
不自动认领、Overlay 叠放顺序与位置连接、普通空页面移交、两种重建顺序、失败／panic/
销毁、实际 GUI 归属、旧 AfterUpdate 失效及业务更新先于通知。
旧“改 UI ID 仍复用”及“类型变化重建整个尾部”测试按新契约修改，仍保留属性更新、
类型替换、默认值恢复和其它位置独立的有效断言，没有降低错误检查条件。

本轮 Linux/X11、隔离 Xvnc＋Xfwm4、Software、1x，XMODIFIERS=@im=none：

- Native 跨窗口：编辑为 edited A，真实拖放，目标 Rebuild，再输入 after move，关闭源。
  用例输出 PASS；原页面/输入框/State、文本连接保留，updates=4、notifications=18。
- Integrated 同 Root、target-first：编辑、移交到 C 右侧、重建、继续编辑、移除源。
  用例输出 PASS；GUI Mount/Unmount=2/1，View Unmount=0。
- `DISPLAY=:97 XMODIFIERS=@im=none go test ./...`、`go test -race ./gui ./ui ./widgets/...`
  通过；Windows amd64／macOS arm64 的 ui_tab_transfer 与 UI／widgets/ui 测试目标
  交叉编译通过（测试二进制未在远端运行）。最终 JSON 中 14 个既有 platform/test
  手动／显式启用场景跳过，含输入、剪贴板、GLX、透明窗口、Chrome/状态/交互；
  不把这些跳过计作通过，无新增失败。无测试的命令包只编译，不执行 main。
- 首次隔离桌面继承 Fcitx 设置，拖放完成但文本未提交，关闭退出未完成验收；改为本地
  XIM 后按完整操作重跑通过，不把初次尝试算作通过，也未修改平台 IME 实现。
- 加入页面自身 ID 查询断言后，同 Root 连续自动输入曾未完成 Rebuild，updates=2
  被例程拒绝。保留失败日志；改为确认移交结束后分步点击，重跑原断言通过
  （same-window-retry.log），没有降低所需更新次数。
- Windows/macOS 原生桌面、其它后端、2x/混合 DPI，本轮未执行；此前三平台结果
  不等于本轮身份重构已经完成三平台桌面验收。日志及截图在仓库外
  `/tmp/goui-ui-identity.2gAGKg`，全仓既有图片测试仍按原规则运行。
- 验证后已停止自建 Xvnc :97、Xfwm4，测试窗口正常退出；用户桌面未操作。
  未提交代码，仓库外的测试产物保留供复查。
