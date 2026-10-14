# GOUI 声明式 UI 层设计

> 状态：View、挂载目标、局部 ID 协调、显式移交及共享修饰器已实现。附着浮层尚未实现且须重新设计，见 §4；不把旧 Key／UpdateOverlays 草案当作已确认 API。
> 2026-09-24：§3.6 的 ID/Name 分离与 ID 查询已实现；§3.7 的拖放源数据声明已接入。
> 2026-10-01：§2.1 增加局部 ID 匹配与相对重排；§2.2 改为普通节点的显式移交，移除任意 State 查询。
> 2026-10-03：BuildContext 改为可保留的节点借用上下文，延迟行构建动态解析当前 Root；详见 §2.1 和 DesignTreeView §6.3。
> 定位:记 ui 层 **View/reconciler 模型**的具体设计/约定。应用**入口、`app` 句柄、生命周期、StyleSheet 落点、DevPort** 见 `doc/DesignApp.md`;总哲学见 `doc/Design.md`「Declarative UI Principles」/「Semantic and AI Integration Principles」;popup 平台/事件模型见 `doc/DesignPopup.md`;跨层约定见 `doc/DesignConventions.md`。

---

## 1. 定位:gui 之上的薄声明式适配

ui 是 gui 的**声明式简便面**,不是第二套 widget 系统。View 值描述 UI 结构,`Root` 把它协调成一棵 gui widget 树；同一挂载目标内按 **ID + 具体 View 类型**复用，无 ID 时按位置和类型复用。语法要 Go-friendly——链式方法在比 struct 字面量更清楚时才用。

## 2. View 模型(现状)

- 窗口级命令通过 `App.FindWindow(id)` 借用已挂载的 `gui.Window`，复用定位/工作区等 GUI 能力，不新增位置声明或包装对象。查询可调度到 UI 线程，但返回对象仍线程亲和；完整生命周期与首次显示边界见 [窗口定位的 UI 接入](DesignWindowManagement.md#ui-access-through-mounted-window-identity)。
- `View`:要么 `Build() View`(组合),要么是 `WidgetView`(`Mount(ctx)→gui.Widget` / `Update` / `Unmount`,映射到一个 gui widget)。
- `Root` 协调;`BuildContext` 提供 `State`/`SetState`/`UpdateChild`/`UpdateChildren`。
- 信号句柄对应用代码隐藏:用户写 `OnXxx` 回调 + 请求更新,ui runtime 内部管信号连接生命周期。
- 更新显式:state 是普通 Go 变量,变了就请求更新;不引入自动依赖追踪/属性绑定(MVP 不需要)。

`AfterUpdate` 是一次性协调完成钩子，不是绘制或计时回调。独立 Root 在整棵树
更新并挂到 Window 后执行；App 的多窗口重建先按声明顺序更新全部窗口树、清理
已移除的窗口，再执行各 Root 的完成钩子，最后显示仍存活的新窗口。
新窗口挂载前已登记，完成钩子可通过 `App.FindWindow` 获取隐藏宿主，定位及移交内容。
这样显式跨树移交不会被本轮尚未处理的旧声明再次覆盖；从树更新到完成钩子及 Show
均禁止同步嵌套整个窗口批次，完成钩子本身不承诺布局已执行或原生尺寸已到达。
回调内业务变更通过 `RequestUpdate` 请求下一轮，失效、已释放或被替代的 owner 跳过。
此边界适用于所有 WidgetView，不增加 TabView 专用 Root 分支。

虚拟化行在普通 Update 之外通过 BuildContext 构建时，子协调内部进入本次协调范围，
完成回调通过 GUI 主循环 Post 延后到同步布局／行挂载结束后；仍核对 owner、Root 和
版本，不在 Bind 内 flush。普通整批窗口更新的上述完成顺序不变，AfterUpdate 仍不
承诺后续所有布局或绘制都已结束。

### ListView／TreeView 模型安装边界（2026-10-04）

声明重建不等于重新安装模型。ui.ListView 与 widgets/ui.TreeView 通过底层 Widget
的 Model() 读取当前模型，仅在引用变化时调用 SetModel。同一非空、同类型的模型
指针跨重建复用；不同指针即使数据相同也替换。nil 接口清空模型。非指针值每次
Update 都重新安装，不用可比较字段或深比较推断身份；typed-nil 不作为有效模型。

命令式 ListView／TreeView.SetModel 则显式断开旧订阅、安装并重新绑定，即使实例
没变。TreeView 同时重置展开、选择、Current 和待定位状态。SetDelegate 也显式
重新安装和清理旧行池；TreeView 的展开、选择及 Current 不因替换 Delegate 重置。
模型 Items 通知用于数据变化，Refresh 用于保留安装关系的表现刷新。

绑定在 Mount 创建并持有 Delegate，Update 更新其模型数据、行构建器和回调，并
按需安装模型，最后 Refresh。Mount 后紧接的首次 Update 不重复安装。TreeView
在模型安装后按展开、选择、Current 顺序应用受控值，并保留既有信号反馈抑制。
同模型更换构建器仍刷新可见内容，不比较闭包，也不增加 appliedModel 或 Root 状态。

两个适配器复用 internal/identity.SamePointer：仅检查 nil 接口或同类型的非空指针
相等；不递归比较对象、不保存 uintptr 地址，不扩展成公共属性／身份协议。
此规则只针对本轮 ListView／TreeView 安装契约；图片缓存、样式内容比较、菜单和
TextView setter 维持各自契约，不据此全仓统一 Set 的副作用。

### 2.1 子节点挂载目标（已实现）

节点协调与具体 Widget 的挂载方式分离。公开接口使用项目已有术语：

```go
type Bin interface {
    SetChild(gui.Widget)
}

type Container interface {
    AddChild(gui.Widget)
    RemoveChild(gui.Widget)
    MoveChildBefore(child, sibling gui.Widget) // nil sibling 移到末尾，不解除挂载
}

type BuildContext interface {
    State() any
    SetState(any)
    Coordinator() *Coordinator
    AfterUpdate(func())
    UpdateChild(target Bin, child View) gui.Widget
    UpdateChildren(target Container, children []View) []gui.Widget
}
```

`gui.Bin`／`gui.Container` 是 Widget 能力；`ui.Bin`／`ui.Container` 只是挂载目标，
不要求实现 Widget，不创建额外布局或语义树节点。普通 GUI 控件自动满足对应接口：

```go
ctx.UpdateChild(button, v.child)
ctx.UpdateChildren(box, v.children)
```

自定义绑定需要不同槽位时，创建普通 Go 适配器实现上述方法即可。比如 Overlay
直接作为主 Child 的 Bin；覆盖列表用私有适配器把 AddChild／RemoveChild 转为
AddOverlay／RemoveOverlay。Root 不检查具体控件类型，绑定不访问私有 node 或 Root。

身份和更新约定：

- 目标是由绑定持有、跨更新复用的非 nil 指针。多个目标在同一 View 中独立协调；
  目标指针不是 Widget ID，也不是列表项的 Key。不要在每轮 Update 重建适配器。
- 同一目标不能由多个 View 共同管理，也不能同时作为 Bin 和 Container 使用。
  使用者不能在协调器之外替换或重新挂载其管理的子控件。
- UpdateChild 管理零或一个子节点；ID 或具体类型改变重建。
  UpdateChildren 在目标内按非空 ID／类型匹配，无 ID 按实际位置／类型匹配；
  不匹配的旧节点单独移除，不再因一个类型改变而重建整个尾部。
  nil View／Build 不生成节点，Mount 返回 nil 也不占用协调位置。
- 列表先展开组合 View 并检查重复非空 ID，再更新子节点；匹配节点的相对重排
  不触发 Mount/Unmount，不重建信号和 State。不同目标之间没有 ID 自动认领。
- UpdateChildren 返回结果与传入切片逐项对应，未产生 Widget 的位置为 nil；实际
  Widget 树和复用位置跳过空项。绑定可以按声明索引更新 Fill 等逐节点配置。
- nil／空内容显式清空该目标；某轮没有调用目标不隐式删除内容。普通空目标记录释放，
  避免动态列表累计历史行；直接属于当前 Widget 的容器，以及通过 TransferContainer
  标识该 Widget 的容器适配器，保留空列表注册以接收明确移交。
  nil 接口目标是 no-op；值目标或 typed-nil 指针报错。
- 此接口只用于 GUI 线程。目标指针在绑定 Mount 中创建，在状态中保存；不是需要
  手动 Release 的公开协调对象。

生命周期：

- Root 记录每个目标的 UI-owned 子节点，统一递归清理。普通移除／卸载调用目标的
  SetChild(nil) 或 RemoveChild；不枚举、清空 GUI 容器内不归 UI 协调的其它内部控件。
- Window 销毁时释放 View 状态和信号，但不调用解除挂载操作，保留 GUI 树供窗口销毁。
- 已释放节点的协调上下文不再接受更新，避免保留上下文的委托在卸载阶段重新挂载。
- 信号仍由具体 View 绑定管理；适配器可以在 RemoveChild 中清理对应连接，View.Unmount
  兜底断开窗口销毁路径中仍保留的连接，不借此拆除 GUI 树。
- ListView 的每个行 shell 作为独立 Container，Bind 调用 UpdateChildren，Unbind 传空列表；
  所有行由同一 Root 的通用生命周期管理，不再断言私有 buildContext 或另存 node。
- BuildContext 可由需要延迟构建的适配器保留，但只借用所属 node，不保存创建时的
  Root、不延长 View 生命周期。每次子协调获取 node 当前的 Root，移交后跟随新 Root；
  释放后 UpdateChild／UpdateChildren 是 no-op。普通 Update 复用当前协调范围，
  延迟构建内部进入范围，禁止嵌套整树重建或在移交中构建。完成时序见上文。

### 2.2 显式普通节点移交（2026-10-01）

复合控件适配器可在 Mount 中保存 `ctx.Coordinator()` 用于移交；延迟行构建保存
BuildContext，仍使用 UpdateChild／UpdateChildren，不增加 Coordinator 的构建方法。
Coordinator 仅借用当前 node，释放 View 后失效；移动子树内的句柄随 node 指向新 Root。

```go
type TransferContainer interface {
    Container
    Widget() gui.Widget
}
func (*Coordinator) TransferChild(child, to gui.Widget, move func() error, committed func()) error
func (*Coordinator) AfterTransfer(func())
```

没有 Lookup(State) 或任意 State 访问接口。协调器只在明确移交时，以真实 Widget
查找本应用现有协调节点。API 面向绑定实现者，普通应用仍使用 App.FindWidget / FindWindow。
目标必须有唯一、已注册的所属容器。普通 Widget 容器直接识别自身；非 Widget 的
适配器用 TransferContainer.Widget 表明所属 Widget，不创建第二棵树或全局注册表。
已有自定义 Container 绑定须补 MoveChildBefore；页面绑定改用普通 WidgetView，
不再把 GUI 内部 Bin 当作不存在的节点来移交。

TransferChild 同步移交一个现有普通子 node、State 和信号连接，不移动整个挂载槽位，
不调用 View 的 Unmount/Mount。`move` 执行已有 GUI 事务；普通失败须保留源 GUI 树。
实际 Parent、Root 和生命周期符合目标时才认定成功；返回 nil 但未移动不会被接受。
成功采用目标实际兄弟位置接入，失败按观察到的 GUI 归属恢复或释放，不复活已销毁对象。
源与目标 Root 不允许嵌套重建，正在 Update 的树也不能发起移交；正常 RequestUpdate
仍延迟执行。旧 AfterUpdate 作废，目标下一次 Update 再建立新绑定。
空内容的 TabPage 仍有普通节点；不为 GUI 内部 Bin 伪造无记录移交。
遇到重复 ID、外部所有者、子树环或重入时，在执行 move 前拒绝。

move 期间源销毁不会释放临时持有的子树；目标最终不存在则释放协调状态。
回调 panic 不转换为普通成功/错误：做基本记录清理并继续传播，不承诺恢复任意
半途修改的 GUI 或业务模型。不会另建长期页面所有者，也不会自动修正应用声明。
`committed` 在节点采用成功后更新两侧业务声明；AfterTransfer 使绑定的选择通知在它
之后发布，过期／销毁 owner 的通知跳过。异常继续传播，不发布延后的业务通知。
不会替应用自动维护声明集合。具体接入见 [Tab 转移设计](DesignTabTransfer.md)。

## 3. 共享修饰器基类:`viewBase` + `ViewBase[T]`(已实现)

**动机**:`Name`/`Hidden`/`Visible`/`Style` 这类横切修饰器现在每个控件抄一份;将来 `Tooltip`/`ContextMenu`/`Popover`(Popup 阶段)只会更多。要一处定义、链式仍返回**具体类型**。

### 3.1 结构 + `base()` 封印

链式 builder 用**指针接收者 + 泛型 self**。指针返回**不违反**声明式不可变——不可变指"每帧重建整棵树",与返回值是不是指针无关。

全部在 `ui/view.go`。**封印 `base()` 与应用 `apply()` 都放在非泛型 `viewBase` 上**——好处:不需链式的 View(如测试辅助)可直接嵌 `viewBase` 满足封印;真实控件嵌 `ViewBase[T]`(经内嵌的 viewBase 得到 `base()`/`apply()`)。

以下为结构示意，省略显式设置位、初始值恢复等协调细节。

```go
// 非泛型:共享修饰器状态 + 封印 + 应用(职责相当于 gui 的 WidgetBase)
type viewBase struct {
    id        string
    name      string
    hidden    bool
    styleName string  // 语义样式名(= SetStyleName);"" 回退控件默认
    // + 布局标量字段(minWidth/maxWidth/mainWeight…);padding 在容器视图上
}
func (b *viewBase) base() *viewBase   { return b }   // 封印:只有嵌了才有
func (b *viewBase) apply(w gui.Widget) {             // 框架在 Update 后统一调
    w.SetID(b.id)
    w.SetName(b.name)
    w.SetVisible(!b.hidden)
    w.SetStyleName(b.styleName)  // + 下发布局修饰器(SetMinWidth/SetMainWeight…)
}

// 泛型可嵌基类:Self 给链式,内嵌 viewBase 给状态 + 封印
type ViewBase[T any] struct {
    Self *T
    viewBase
}
func (b *ViewBase[T]) self() *T {                    // 护栏:Self 忘设则响亮 panic
    if b.Self != nil { return b.Self }
    panic("ui: view not initialized via its constructor (ViewBase.Self is nil)")
}
func (b *ViewBase[T]) ID(s string) *T           { b.id = s; return b.self() }
func (b *ViewBase[T]) Name(s string) *T         { b.name = s; return b.self() }
func (b *ViewBase[T]) Hidden(v bool) *T         { b.hidden = v; return b.self() }
func (b *ViewBase[T]) Visible(v bool) *T        { b.hidden = !v; return b.self() }
func (b *ViewBase[T]) Style(name string) *T     { b.styleName = name; return b.self() }

type View interface {
    base() *viewBase   // 封印首位:不嵌 ViewBase 就实现不了 View,编译不过
    Build() View
}

type ButtonView struct {
    ViewBase[ButtonView]
    child   View
    onClick func()
}
func Button(text ...string) *ButtonView {
    v := &ButtonView{}
    v.Self = v                    // 唯一需纪律的一行
    if len(text) == 1 { v.child = Label(text[0]) }
    return v
}
func (v *ButtonView) OnClick(fn func()) *ButtonView { v.onClick = fn; return v }
```

> **当前实现的共享修饰器 = `Name`/`Hidden`/`Visible`/`Style`(语义名)+ `Cursor`/`Focusable`/`OnFocus` + 布局标量(`Padding`/`MinWidth`/`MainWeight`…)**(`.Style(name)` 设 `styleName`,布局标量设字段)。`Tooltip`/`ContextMenu`/`Popover` 装饰器 + `viewBase` 的 overlay 字段属 **Popup 阶段(§4),尚未实现**。`Cursor`/`Focusable` 遵循"仅显式调用才应用"约定:控件构造函数里已设的默认(gui.Button/`MenuButton`/`TextInput` 的焦点)在未调用时保留,可显式 `Focusable(false)` 剥离(如菜单栏式行)。`OnFocus(func(focused bool))` 是回调不是 setter,由 `viewBase` 的**私有生命周期**处理:reconciler 只在 node 首次 mount 时构造一个持久 `viewBaseContext`(跨 rebuild 存活,存有 `handles []signal.Handle` + 当前生效 `onFocus`),mount 时连接 `gui.Widget.ConnectFocused`、每次 update 把新 view 的回调盖进 context、unmount 时断开——闭包触发时读 context 里的最新回调,因此 rebuild 永不重连。`viewBaseContext` 是私有类型不暴露 root 内部结构,也不提升到自定义控件的公开 API;`root.go` 只做通用代理,不含任何焦点语义——任何可聚焦 View 都可用。回调**带 boolean 状态**(gain=true / loss=false):因为焦点可从当前控件移开到无物可享(如移到不可聚焦区域),若只在 gain 时触发则无法清除跟踪状态;传 boolean 让调用方在 loss 时清理(如 `if f { focused=&x } else { focused=nil }`)。注意调度顺序:焦点 A→B 时,旧 A 先得 `false`、新 B 后得 `true`,期间跟踪指针瞬时为 nil(菜单期间焦点稳定,无影响)。这是"焦点控件发布能力"的声明式入口(配合 `ui.State` 实现如"当前焦点 text 状态"的跟踪)。

两层对齐:

| | gui(命令式) | ui(声明式) |
|---|---|---|
| 状态 + 封印 | `WidgetBase` + `Widget.base()` | `viewBase` + `View.base()` |
| 链式壳 | 无 | `ViewBase[T]{Self}` 泛型层 |

### 3.2 强制嵌入 + reconciler 统一应用

- **"必须嵌入"编译期强制**:`base()` 是未导出封印方法,只有 `ViewBase` 提供 → 不嵌就实现不了 `View`。这是**刻意的封印模式**(同 `Widget.base()`),**不是**"拿接口假装解耦平级组件"那种反模式。
- **"必须应用"框架代劳**:reconciler 在 `Update` 后**统一调 `view.base().apply(w)`** 把 `id`/`visible`/`style` 刷上去,控件无从遗漏,连方法都不用调:
  ```go
  view.Update(ctx, w)
  view.base().apply(w)   // SetID / SetName / SetVisible / SetStyleName
  ```
  控件的 `Update` 只写控件专属(onClick、children…);共享横切全归框架。(附着浮层 tooltip/menu/popover 的 apply 属 Popup 阶段 §4,当前 `apply` 只做 id/visible/style。)

### 3.3 与「避免自引用技巧」原则的张力(诚实标注)

`Design.md` 的 Declarative UI Principles 有一条:**避免不安全的自引用技巧、内嵌 Props 结构、以及只为模仿别的语言而存在的抽象**。`ViewBase[T]{Self *T}` 正是一种自引用(CRTP,源自 C++)。

**为何仍采用(有意识的例外)**:
- 它解决的是**真实的 Go 问题**——值语义链式 builder 下,共享修饰器无法返回具体类型(内嵌基类只能返回基类型,断链);不是"为模仿而模仿"。
- 收益是**结构性的**:所有横切修饰器(Name/Hidden/Tooltip/ContextMenu/Popover,及未来 Padding/Align…)一处定义、零重复,控件越多越赚。
- "不安全"的部分被**围栏**:见 §3.4。

> 结论:这是一处**受控的、有明确收益 + 护栏**的自引用,而非该原则要防的"炫技/模仿"。`Design.md` 的 Declarative UI Principles 已相应放宽——明确"受控 + 有护栏(panic-on-nil + 测试)的 self-type 允许,纯自引用/模仿不允许"。两份文档一致。

### 3.4 `Self` footgun + 兜底

`base()` 封的是"必须嵌入",封不了"必须设 `Self`"——`Self` 忘设(构造器漏 `v.Self = v`)→ 链式修饰器返回 nil → 后续链 nil。围栏:
- 所有链式修饰器经 `self()`:`if b.Self == nil { panic("view 未经构造器初始化") }`,把静默 nil 变成开发期响亮报错。
- 冒烟测试 `TestViewConstructorsWireSelf`:遍历所有控件构造器 + 各调一个共享修饰器,漏设 `Self` 即在 `self()` panic。

### 3.5 局部接口定稿:语义样式 + 布局标量,无具体值

局部声明式接口的**全貌**(刻意极小,细节见 DesignStyle):

| 类别 | 方法 | 归属 |
|---|---|---|
| 标识/名称 | `.ID(string)` · `.Name(string)`（见 §3.6） | viewBase |
| 显隐 | `.Visible(bool)` / `.Hidden(bool)` | viewBase |
| **语义样式** | `.Style(name string)` | **唯一视觉入口**——命名意图(= `SetStyleName`),主题给外观 |
| 布局标量 | `.Padding(n)` · `.MinWidth/.MaxWidth/.MinHeight/.MaxHeight(n)` · `.MinSize/.MaxSize` · `.MainWeight(n)` | 布局字段,**非样式** |
| 结构(容器) | `.Children(...)` · `.Spacing(n)` · `.MainAlign(a)` · `.CrossAlign(a)` | 容器视图字段 |

**灵魂:局部只能「选语义 + 摆布局」,不能「设具体样式」。**

- `.Style(name string)` 只收单个 string(语义名),= `widget.SetStyleName(name)`,**收不到颜色/字体值**——堵死逃生口在类型上就可见;`name` 常量归应用/主题自己的包(`mystyle.Danger = "danger"`),不在 `ui`。普通控件 `Button()` 用默认样式名(`"button"`)。变体(outlined/filled)也是一个语义名,不用额外参数。
- **移除旧 `.Style(rules ...style.Rule)`**(那是"局部设值"的口子)+ **移除 `gui.Widget` 的 `StyleRules()/SetStyleRules()`**(不再有局部 rule 覆盖);没有 `.Background()/.Foreground()`、没有 part 构造器——part/state/具体值全在主题。
- 所有修饰器**覆盖语义**(最后一次赢):`.Style()` 与 `.Padding()/.MinWidth()` 一致,无累积、无二义。
- padding/align/尺寸/weight 是**布局**不是样式(DesignLayout §9/§10),和 `.Style()` 分属两个系统。

### 3.6 ID、Name 与 Widget 查询（已实现）

完整契约与迁移见 [Widget 标识设计](DesignWidgetIdentity.md)。ID 是范围内唯一的程序标识，
Name 是可重复的自然名称；StyleName 和 Text 独立，不由 ID/Name 推导。
已新增 `.ID()`，`.Name()` 设置 `SetName`，不保留同时写入 ID 的兼容路径。
共享协调分别保存两项的显式设置位与初始值；未声明时恢复原值，显式空值清空。

Window ID 在应用内唯一，非空 Widget ID 在所属 Root 的 Widget 树内唯一；
Window/Popover 分别成域，Overlay 不成域。查询按明确的范围和 ID 定位当前挂载对象，
重复 ID 返回首个匹配，未找到返回 nil，Name 允许多匹配；不增加 WidgetRef 或 View.Widget()。
UI 查询通过 Sync 调度并委托 GUI，不维护第二棵对象树。仅在 GUI 线程使用结果，不夺取所有权，
不认为查到对象就已完成布局，不将旧引用视为跨重建/销毁永久有效。

ID 参与 §2 的挂载目标内匹配，不引入独立 Key 或全局唯一身份。Snapshot 同时记录 ID/Name/Text，
HeaderBar.Captions 按实际命中 Widget 的 ID 标记 Caption 区域。旧例程及测试按用途迁移，
不机械替换所有 Name。

### 3.7 DragDrop：数据声明与 GUI 事件接入

`DragSource().Data(data)` 直接声明固定数据，按需增加 OnPrepare、OnEnd；
`DropTarget(formats...).OnDrop(...)` 同步决定 `DropRequest.Accepted`。
复杂场景直接用 GUI 的 Prepare、Motion、DropRequest、Result 等类型及同步结果参数，
UI 管理连接和生命周期，不再定义一套 DragContext/DropInfo/DropProposal。

Preview 保留 image.Image/Scale/Hotspot；需要 RenderWidget 时通过 ID 查询现有对象。
回调中的业务操作不破坏声明式定位：Build 只声明能力，GUI 解释输入，业务仍显式请求更新。
接口、校验阶段、SwiftUI 参考及验收要求见 [DnD UI 设计](DesignDragAndDrop.md#10-ui-与语义快照)。

## 4. 附着浮层声明（待重新设计，未实现）

当前已有普通 ui.Overlay、MenuButton 菜单声明和通用 OnContextMenu 输入绑定，
但没有通用 Tooltip／Popover 附着修饰符。它们不是同一个缺口，也不要求 Menu
或 DropDown 迁移到窗口内 Overlay。

先前 OverlaySpec.Key／BuildContext.UpdateOverlays／WidgetBase.AttachPopover
只是未采用的草案，不是待添加接口。未来需求、状态真相、独立宿主子树协调、
dismiss 回写及生命周期边界集中记录在 [未来路线](Roadmap.md#future-ui-popover)；
重新设计须复用已实现的挂载目标和 ID 协调，不给 Root 塞入控件特例。

## 5. 当前使用入口

窗口内查找栏／遮罩 Dialog／Toast 使用 [普通 Overlay](DesignOverlay.md) 组合。
菜单使用 ui.MenuButton／ui.MenuItem；应用也可通过 OnContextMenu 处理 GUI 的
同步菜单请求。通用原生 Popover 目前仍是 GUI 命令式底座，不展示尚不存在的链式 API。

## 6. 影响面

**§3 已落地**:
- 所有 View 由**值改指针**:内嵌 `ViewBase[Self]`、构造器设 `Self`、`View` 由 `*T` 实现、`Build/Mount/Update/Unmount` 改指针接收者。
- 控件 `Update` 去掉 `SetID`/`SetVisible`/`SetStyleName`;reconciler `Update` 后统一 `view.base().apply(w)`。
- 冒烟测试 `TestViewConstructorsWireSelf` 兜 `Self` 接线。

**§4 待评估**：先明确实际使用场景，再设计附着浮层协调，不预先添加 UpdateOverlays 或第二套 Key。

## 7. 后续评估边界

- **糖是否要**:`Tooltip`/`ContextMenu` 单列,还是只留通用 `Popover`。倾向先只做通用 `Popover` 附着,糖等 `Menu` 基元落地、真觉啰嗦再加;且糖加在 gui(`SetTooltip`)还是 ui(`.Tooltip()`)一起定——倾向 **gui 出能力、ui 出糖**。
- Menu／MenuItem 基元和分隔线已实现；子菜单与菜单键盘导航的后续设计见 [子菜单](DesignMenuSubmenu.md)。
- 组合型 View（只 Build）的共享修饰器传播仅是评估方向，当前 normalizeView 展开
  后只应用最终 WidgetView 的修饰器，不自动传播外壳 ID／Style，也不为外壳保留
  独立协调状态。现行使用方式与扩展限制见 [自定义 Widget／View](CustomWidgetView.md)。
- 若将来同一 anchor 上组合状态驱动、hover 与上下文菜单，须明确协调所有权和优先级；不预先承诺 OverlaySpec 或 Key 接口。
