# GOUI 跨层设计约定

> 状态：platform → gui → ui 贯通时反复出现的约定、思路与"配方"。
> 定位：`doc/Design.md` 记总哲学,本文记**具体、可操作**的跨层约定,指导后续平台能力(**IME / DragDrop / Popup** 等)的开发,防止理解偏差。
> 只写思路/哲学/约定,不写具体实现。

---

## 1. 三层的对外契约

| 层 | 职责 | 形态 |
|---|---|---|
| **platform** | 系统**事实**;线程亲和;拿不到就返回 `error`;**不发明默认、不承载策略** | `(value, error)` |
| **gui** | **抹平平台差异**，拥有明确的 fallback / 策略；无法消化的操作失败交给调用方；线程亲和 | 按操作契约返回值或错误 |
| **ui** | 声明式适配及运行时命令的线程转交；保留 GUI 的操作结果，不隐藏业务失败 | 声明式值；运行时命令可返回错误 |

**设计每层 API 时问"对外自然、够用吗",不为某个特定上层用途(如 style)让路。**

## 2. 事实 vs 策略

- **事实**(系统值 + "拿不到"用 `error`)归 platform。
- **策略**(默认值 / fallback / 解析 / 偏好)归 **gui-或以上**。平台层不能擅自决定默认;GUI 抹平差异 → 返回可用值,消费者不必逐项判"这平台有没有这项、有没有效"——否则等于把平台差异又暴露上去了。

### 错误反馈规则

抹平平台差异不等于隐藏操作失败。是否返回错误由操作语义决定，不由所在层决定：

- 有明确默认值的查询（主题、字体等）：GUI 使用默认值，上层不必处理原生查询错误。
- 明确约定为尽力而为的提示（最小尺寸、现有 RequestState）：可以不返回错误；需要观察的状态以实际通知为准，不伪造成功状态。
- 应用主动发起、失败影响业务的命令（OpenURL、OpenPath、写入剪贴板、文件对话框）：没有合理兜底时将失败交给调用方。
- 用户取消是正常结果，必须与系统故障区分；可用空结果加 nil error 表示取消。
- 无法启动或继续运行的错误由创建入口或 ui.Run 返回。局部命令失败不应自动退出应用。
- 同步命令返回 error；异步命令通过该次操作的完成结果交付错误。不为同一个失败同时返回错误、发送全局错误信号和打印日志；日志不能替代操作结果。
- 不默认弹错误框。是否提示、重试、替代或忽略由应用决定。错误信息用于诊断，不作为可本地化的用户提示文案。
- 对外只承诺文档列出的错误语义；不要求应用解析原生错误字符串。底层错误的可解包性也是 API 契约，不因包装而无意承诺所有原生错误分类。

当前进度：OpenURL/OpenPath 与 FileDialog 已遵循本规则；文件对话框交付 `[]FileAccess, error`，取消为 `nil, nil`，见 [文件访问](DesignFileAccess.md)。Clipboard 已迁移为 SetText 返回 error、RequestText 回调交付 (string, error)，无文本/空文本为 ("", nil)，仅实际读写失败使用 error；ui.RequestText 另返回调度受理结果，未受理不回调；实现及平台验证边界见 [Clipboard](Clipboard.md)。Settings 的有效默认值策略保持不变。

## 3. 线程模型(最容易踩坑)

- 平台对象**线程亲和**,只在其 OS 线程(主 / UI 线程)上用;唯一跨线程操作是 `EventLoop.Post` / `Quit`。
- **原生线程亲和 API(GTK / AppKit / Win32 UI)绝不在后台 goroutine 上调**——会崩溃。
- 平台回调可能在**后台线程**触发 → **碰 widget / 发信号前必须 `Post` 到 UI 线程**。
- 需要**轮询 / 检测原生状态**(如系统设置变化)→ 放主线程(经事件循环 `Post`),**不要**开后台 goroutine 直接读原生。
- ui 层用 `Post`(异步)/ `Sync`(在 UI 线程直跑、否则投递并等待)把用户调用搬到 UI 线程;回调也保证落在 UI 线程。

### 原生同步回调与 Go 地址稳定性

同步调用也可能形成 `Go → 原生函数 → Go 回调 → 原生函数返回`。原生函数持有的
Go 内存地址必须在整个调用期间稳定，不能假设锁定 OS 线程就同时锁定 goroutine 栈。
回调中的布局、分配或 GC 都可能改变 Go 栈；`runtime.KeepAlive` 只保证存活，不固定地址。

Windows SDK 根据具体调用路径选择防护，不因参数含有指针就批量改写绑定。
GetMessage、PeekMessage、DispatchMessage 等消息入口，以及已确认会同步调用
窗口过程的 CreateWindowEx、BeginPaint、同进程窗口文字读写，使用既有
`syscall.LazyProc.Call`，让 `uintptrescapes` 在地址转换处生效。消息循环在循环外
分配一个 MSG 并复用，每次取消息前清零；`new` 本身不保证堆分配，由调用边界保障
逃逸，并核对编译器结果。

普通查询和绘图沿用 Syscall/CallRaw；结构体按值及浮点等 ABI 转换继续采用原有
`cgo.Call/CallRet`。Call/CallRet 有反射与参数构造成本，不能为统一写法搬入绘图热路径。
检查原生回调之外，也须留意地址转换后、中间 Go 包装调用的栈增长；有具体风险时
在该入口保证逃逸或固定内存，不把参数变成 uintptr 后才补 KeepAlive 当作地址防护。
已经按生命周期固定的 OLE 对象继续由原所有者管理。原生函数若在返回后保留数据，
仍须另外满足其所有权和长期有效期契约。

依据：Go 1.25.8 的 [Proc.Call](https://github.com/golang/go/blob/go1.25.8/src/syscall/dll_windows.go)
逃逸声明及 [原生回调实现](https://github.com/golang/go/blob/go1.25.8/src/runtime/syscall_windows.go)；
微软 [GetMessageW](https://learn.microsoft.com/en-us/windows/win32/api/winuser/nf-winuser-getmessagew)
明确调用期间会派发 sent messages。`tests/window/platform/message_reentry` 使用实际
同步消息和强制栈增长验证 MSG 地址及回复内容，分别隔离 GetMessage、PeekMessage
及 DispatchMessage 的缓冲区；各次构建的执行结果见 DesignSplitView 的验证记录。
其它保留 Proc.Call 的入口依据微软的 [CreateWindowExW](https://learn.microsoft.com/en-us/windows/win32/api/winuser/nf-winuser-createwindowexw)、
[BeginPaint](https://learn.microsoft.com/en-us/windows/win32/api/winuser/nf-winuser-beginpaint)、
[GetWindowTextW](https://learn.microsoft.com/en-us/windows/win32/api/winuser/nf-winuser-getwindowtextw) 和
[SetWindowTextW](https://learn.microsoft.com/en-us/windows/win32/api/winuser/nf-winuser-setwindowtextw) 的同步消息契约。

## 4. 类型不外泄下层

- gui / ui 的**公开签名只出现本层类型**。要暴露下层类型 → **别名 re-export**(`gui.X = platform.X`、`ui.X = gui.X`,常量一并),这样用 gui API 不必 import platform、用 ui API 不必 import gui。
- **纯值/枚举**(`ColorScheme`)用**别名** re-export;**行为需适配**的(`Clipboard`/`Settings`)每层出**本层自有类型**——gui 出接口，包装 platform、执行明确的兜底并保留无法消化的失败；ui 出线程安全包装。别名只给零适配的透传类型。
- **例外**:有意暴露的低层逃生口(`App.Platform()` / `App.Typography()`)。

## 5. 命名约定

- 配置 setter → `SetXxx`(链式返回本身);服务 getter → 裸名词(`Clipboard()` / `Settings()`);动作 → 动词(`Run` / `Post` / `Sync` / `Quit` / `RequestUpdate`)。
- **getter 返回 explicit,不返回默认**:与 setter 配对的裸 getter 返回"**你设了什么**"(round-trip:`SetX(v); X()==v`,未设 = 零值/nil),**不**在 getter 里偷偷返回默认;需要"**实际生效什么**"(含 fallback)时,用命名明确的 resolver(`resolvedXxx()` / 使用点 fallback)。理由:可预测、保留"是否设过"、fallback 集中一处。
  - 范例:`Application.StyleSheet()` 返回 nil(未设 custom,explicit)+ `ResolveStyle` 内部兜底 `DefaultStyleSheet`;`Widget.StyleName()` 返回 raw override(未设 `""`)+ 控件在调用点内联兜底自己的默认名(`if name==""{name=styleNameButton}`;**不设私有 helper——第三方控件也要能解析,故 `ResolveStyle` 公开**)。**别让一个裸 getter 两头兼职。**
- **gui 精确、ui 简洁**:能力层 getter/setter 求精确消歧义(`StyleName`/`SetStyleName`——区别于 `style.Style` 值类型),声明式 ui 层用短修饰器(`.Style(name)` / `.Padding()`);两层命名不同是分工,不是不一致。

## 6. 依赖哲学

- **只依赖系统自带的动态库 / API**;目标不是零依赖,而是**不深度依赖**。
- 用不用 GTK 看两根轴:**能否优雅降级?集成有多深?** 两者都危险就别用。
  - **可用**:可降级 + 浅集成的系统查询(Settings——拿不到就 fallback、只同步读属性)。
  - **不可用**:核心的 + 要挂事件循环的能力(Clipboard / DnD——用 OS 原生机制:X11 selection / XDND、Win32、Cocoa)。

## 7. 暴露一个平台能力的"配方"(IME / DnD / Popup 照此)

1. **platform**:OS 原生实现,报事实,线程亲和;先验证平台往返 / 语义。新增测试按 [测试用例约定](Testing.md) 放入所属包或 `tests/window/platform/`，现有 `platform/test` 暂不迁移。
2. **gui**:薄包装 / 别名 → 抹平差异 + fallback + 本层类型 +(必要时)变化信号(在 UI 线程发);服务在 `NewApplication` **急切创建**;**非必需**服务创建失败**非致命**(存 nil / 降级,不拖垮 app 启动)。
3. **ui**:线程安全包装(`Sync` / `Post`)，保留操作结果，无原生线程负担。错误反馈规则见第 2 节。
- 每步单独验证;三 GOOS(`windows` / `darwin` / `linux`)交叉编译。

## 8. 坐标与单位

- gui / ui 世界统一**逻辑(DIP)**;pointer / size 在平台边界归一化为逻辑;**物理像素只用于 painter 表面**。详见 `doc/HiDPI.md`。
- **IME 的 caret rect、Popup 的锚点定位、DnD 的坐标**都按此:对外逻辑,必要时内部换算物理。

## 9. 反过度设计

- **不提前造机器(YAGNI)**。不为推测的消费者拆分信号;单信号够用就单信号。
- **变化的"后果"因场景而异,框架代替不了 app 判断** → 不做粗粒度的全局 auto-update(如"设置一变就全量重建":主题该重绘、双击间隔不该、语言/时区取决于用法)。响应式等需求明确、且能 per-consumer 精确时再设计。
- 渲染归渲染、布局归布局(如文字**像素网格对齐是渲染层**的事,不是布局层)。
- 拿不准时,选"让**所有权 / 事件流 / 线程 / 生命周期**更好解释"的设计。

## 10. Widget 组合模式(不用继承 / 不用 Self)

**背景**:Go 没有继承,只有接口与组合。`WidgetBase` 是内嵌的匿名基座,
`Widget` 接口由每个具体控件(内嵌 `WidgetBase`)实现。曾考虑给
`WidgetBase` 加 `Self` 字段 / `Init()` 让基座方法知道"我是谁",**已否决**:

- 用户可见、多级继承都会出错(忘记设 Self → 静默坏);
- 引入对子类行为的依赖,一不小心就是**死递归**(基座方法回调子类方法,
  子类方法又回到基座);
- 本质是在用组合模拟面向对象的继承——Go 里这是反模式。
- ui 层 `ViewBase` 是另一处妥协,但 ui 层使用不涉及自定义控件;gui 层
  `WidgetBase.Self` 会**直接影响每个自定义控件实现**,代价不可接受。

**落地约定**:

- **挂载原语公开但受语义防御**:`WidgetBase.AddChild(parent, child Widget)`
  双参数——parent 由调用者显式传入("基座不知道我是谁"的惯用解法,与
  GTK `gtk_container_add(container, child)` 同构)。这是 `Container.AddChild`
  和 `Bin.SetChild` 实现者的底层原语(GTK/Qt 同样公开 `gtk_widget_set_parent`
  等原语)。**防御**:`AddChild` 对 `Bin` 拒绝未经 `SetChild` 登记的 child
  (SetChild 先更新 child 槽再挂载),因此直接对 Button 调 AddChild 是 no-op,
  不会造成 child 槽与 children 不一致;`ListView` 在 `LayoutVisible` 时清理
  未登记进 item 表的残留 child。外部代码应走 `Container.AddChild`(多子)和
  `Bin.SetChild`(单内容)语义入口。
- **多子容器** 各自写一行单参数包装满足 `Container` 接口:
  ```go
  func (b *LinearBox) AddChild(child Widget)    { b.WidgetBase.AddChild(b, child) }
  func (b *LinearBox) RemoveChild(child Widget) { b.WidgetBase.RemoveChild(child) }
  ```
- **单内容容器**(Button/ScrollView)**不是 Container**,而是满足
  `Bin` 接口(`Child`/`SetChild`):内容 API 是"替换整个 child 槽",
  没有位置语义。内部挂载仍走公开原语 `WidgetBase.AddChild(self, child)`。
  `Bin` 是 GTK `GtkBin` 的对应(与 `Container`/`GtkContainer` 成对),
  由 gui 层正式定义;ui 层 reconciler 依赖它做整体替换(而非位置 diff),
  不再需要具体类型断言。
- `RemoveChild(child)` 单参数是对的——child 自带 `parentWidget` 反链,
  不需要显式传 parent,接口只收 child 即可;`removeChild` 私有原语同理。
- **`Children() []Widget` 属于 `Widget` 接口而非 Container**(见
  `DesignScroll.md`):所有 widget 物理上都有 `children` 字段(挂载/绘制/
  快照/生命周期递归都需要),叶子返回空切片;hitTest 直接遍历
  `widget.Children()`,无需 Container 断言。

---

## 相关文档

`Design.md`(总哲学) · `PlatformDev.md`(平台层结构) · `HiDPI.md` · `Clipboard.md` · `Settings.md` · `DesignPlatformSettings.md` · `InputTODO.md`(输入推进顺序)。
