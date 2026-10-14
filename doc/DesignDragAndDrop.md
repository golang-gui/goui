# Drag and Drop 详细设计

状态：三平台代码已接入；Linux/X11 的核心桌面场景已验收，Windows/macOS 仅完成无头测试与交叉编译，真实桌面互操作待验收。
用户确认 RenderWidget 已验证。位图导出契约见 [RenderWidget](DesignRenderWidget.md)。
源端发起已接入 [GUI 手势竞争](DesignGesture.md)；目标端 Offer 协商保持独立。
功能进度和验收结果分别记录在 Plan / Roadmap。
2026-10-04：补充独立 DragMotionEventController 及 ScrollView 边缘自动滚动，
Window/Popover 复用 rootBase；不读取数据或替应用决定放下策略。此切片已实现，
三平台核心窗口验证及未覆盖组合见 [交互扩展设计](DesignInteractionExtensions.md)。
下述“首版不含自动滚动”保留为原始范围，不再代表当前实现缺失。
2026-09-24：§10 确定单条 UI 声明路径：DragSource.Data 预置数据，OnPrepare
按需补充；DropEvent 更名为 DropRequest。ID 查询预览路径已加入窗口用例，
真实桌面验收另行记录。

## 1. 目标与取舍

### 1.1 首版范围

同一套源/目标支持控件间、跨 Window/Popover、拖出应用及从外部应用拖入。
支持文本、真实本地文件/目录路径、URL、自定义 MIME 字节和应用内 Go 对象，
支持 Copy / Move / Link 动作协商及可选 image.Image 预览。

不包含虚拟文件/文件承诺、流式业务数据、触摸长按、右键拖拽菜单、自动滚动、
悬停展开目录、列表重排动画、文件复制/删除、剪贴板 API 重构、Wayland 后端。
文件示例只启用 Copy；框架不因为收到 Move 就删除源文件。
不为应用内拖拽另造一条绕开正常事件分发的快捷通道。

### 1.2 参考与证据

核对时间为 2026-09：GTK 在线文档当时标注 4.23.4，源码固定 4.20.0；Qt 文档为 6.11.2。
下面是参考框架契约/源码观察，不是 GOUI 的原生验收结果。

- GTK4 的 [DragSource](https://docs.gtk.org/gtk4/class.DragSource.html)、
  [DropTarget](https://docs.gtk.org/gtk4/class.DropTarget.html) 是附加到 Widget 的控制器，
  采用 prepare / enter / motion / leave / drop 信号。GOUI 采用组合方式，
  不引入 GObject/GValue、GType 自动序列化和完整 Gesture 继承体系。
- [gtkdragsource.c，4.20.0](https://github.com/GNOME/gtk/blob/4.20.0/gtk/gtkdragsource.c)
  默认 Copy，识别手势后启动，该版本还有时间门槛。GOUI 首版只采用明确的位移阈值。
- [GTK DropTarget::drop](https://docs.gtk.org/gtk4/signal.DropTarget.drop.html) 提供最终接收确认；
  [Qt DnD](https://doc.qt.io/qt-6/dnd.html) 同样区分格式和动作。
  GOUI 使用同步结果指针和普通 Go 数据，不让业务管理原生 Drop 会话。
- [GTK Win32 实现](https://github.com/GNOME/gtk/blob/4.20.0/gdk/win32/gdkdrop-win32.c)
  说明异步包装与原生同步回调仍需衔接，不能把异步理解为没有重入。
  GOUI 不向业务暴露任意延迟 Finish，不额外运行嵌套循环等待业务任务。
- [GTK X11 源](https://github.com/GNOME/gtk/blob/4.20.0/gdk/x11/gdkdrag-x11.c)、
  [macOS 目标](https://github.com/GNOME/gtk/blob/4.20.0/gdk/macos/gdkmacosdrop.c)
  用于核对原生会话、协议和资源生命周期；最终绑定仍遵循 GOUI 原有目录及规范。

## 2. 架构与所有权

| 对象 | 负责 | 不负责 |
| --- | --- | --- |
| platform DnD 服务 | 原生注册、传输、格式映射、动作事实、预览、坐标换算 | Widget 命中、文件操作 |
| gui.Application 私有协调器 | 当前本地源会话、Local 数据、代次、跨宿主关联 | 保存所有控件的业务内容 |
| GUI rootBase / 手势协调器 | 源端竞争、活动目标、路由、销毁兜底 | 解读业务内容、执行导入 |
| DragSource / DropTarget | 配置、输入语义、统一信号 | 独立事件循环、窗口绘制 |
| UI 描述符 | 声明式配置和控制器/信号协调 | 第二套拖拽状态机 |

每个 Window/Popup 至多一个平台 DnD 服务，与 Painter 无关。Window 和 Popover
共用宿主内部适配；Popover 原生句柄存在时才创建相应服务。
同一 Application 同时至多一个本地源会话，第二次启动报告忙，不抢占已有会话。
外部 Offer 不需要伪造 GUI DragSource。

所有 GUI 信号、原生对象操作均在 GUI 线程。原生回调可能重入并销毁窗口，
对象必须活到原生真正不再使用；不能把 GUI 对象解除挂载视为原生资源已释放。

## 3. 数据与格式

### 3.1 动作和格式类型

```go
type DragAction uint8

const (
    DragCopy DragAction = 1 << iota
    DragMove
    DragLink
)

type DragFormat string

const (
    DragFormatText  DragFormat = "text"
    DragFormatFiles DragFormat = "files"
    DragFormatURLs  DragFormat = "urls"
)

func MIMEFormat(mime string) DragFormat
func LocalFormat(name string) DragFormat
```

Actions 配置是位集合，结果只允许一个已知位或 0；0 表示拒绝/没有完成动作。
源/目标默认 Copy，SetActions(0) 不回退默认值；未知位作为无效配置报告。

MIMEFormat 使用 `mime:<规范化 media type>`，LocalFormat 使用 `local:<name>`，
与三种内置格式分离，不直接把原生 atom/UTI/CLIPFORMAT 当 GUI 格式。
Local 名称大小写敏感且非空；MIME 按标准 media type 解析。格式工厂不返回 error，
非法输入得到无效格式，使用时通过源结果/目标错误反馈，不变成任意格式匹配。
格式字符串本身不提供信任保证；外来数据不能凭 `local:` 前缀成为 Go 对象。

目标格式按声明顺序选择首个兼容表示，重复项去重，空列表表示不接收。
选定后的读取/解码失败不自动改选后一个格式，避免内容语义悄悄变化。

### 3.2 DragData

```go
type DragData struct { /* 私有字段 */ }

func (*DragData) SetText(string)
func (*DragData) SetFiles([]string)
func (*DragData) SetURLs([]string)
func (*DragData) SetBytes(mime string, data []byte)
func (*DragData) SetLocal(name string, value any)

func (*DragData) Text() (string, bool)
func (*DragData) Files() ([]string, bool)
func (*DragData) URLs() ([]string, bool)
func (*DragData) Bytes(mime string) ([]byte, bool)
func (*DragData) Local(name string) (any, bool)
```

- 零值可用，无需 NewDragData；setter 替换相应表示，不追加。
- Text 为有效 UTF-8；空文本、空 MIME 字节是有效值，和不存在由 bool 区分。
- Files 是非空的绝对本地路径列表，不传文件内容；目录同样支持。不自动 Stat、解析符号链接、
  改大小写。文件在拖拽期间可能消失，最终访问错误由应用处理。相对路径、NUL 和不能合法
  编码为原生路径的内容在源启动校验时报错。
- URLs 为非空的绝对 URI 列表，不限 HTTP，但框架不负责打开/执行。业务需自行校验 scheme。
- Files 与 URLs 映射到同一原生 URI 列表时，必须代表同一批 file URI、顺序一致；
  不一致在启动时报错，不由平台静默覆盖。SetBytes 不接受与内置 text/plain、text/uri-list
  等表示冲突的 MIME；文本/文件与自定义 MIME 可以同时提供。
- Local 只在本 Application 中传引用，不深拷贝、不序列化；nil 与不存在由 bool 区分。
- setter 复制切片/字节，getter 返回独立切片/字节；Prepare 返回后冻结本次数据。
  后续修改原 DragData 不影响活动会话。Local 对象自身的可变状态由业务同步。
- Drop 收到独立 DragData，仅含选定格式的数据；不暗示其他表示也已读取。
  可以在回调后保留并用于业务任务，不依赖原生对象继续存活。

setter 不执行 I/O 或发信号。校验在 Prepare 返回后集中执行，失败走源 End.Err；
目标读取/解码失败不调用 Drop，发送一次目标 Error，再以拒绝结束原生传输。

## 4. 拖拽源与预览

```go
func NewDragSource() *DragSource
func (*DragSource) Enabled() bool
func (*DragSource) SetEnabled(bool)
func (*DragSource) Actions() DragAction
func (*DragSource) SetActions(DragAction)
func (*DragSource) SetFeedback(DragFeedback)
func (*DragSource) Dragging() bool
func (*DragSource) Cancel()
func (*DragSource) ConnectPrepare(func(*DragPrepare)) signal.Handle
func (*DragSource) ConnectBegin(func()) signal.Handle
func (*DragSource) ConnectEnd(func(DragResult)) signal.Handle

type DragPrepare struct {
    Position geometry.Point
    Data     *DragData
    Preview  DragPreview
}

type DragPreview struct {
    Image   image.Image
    Scale   float32
    Hotspot geometry.Point
}

type DragResult struct {
    Action   DragAction
    Canceled bool
    Err      error
    Position geometry.Point // 源客户区 DIP，允许在窗口外
    PositionValid bool
    LocalTarget bool // 原生结束点命中本进程已注册拖放窗口；不表示投递/接受
    LocalDrop bool // GUI 实际收到本应用 Drop；不是 hover 或桌面空白判断
}
```

DragSource 实现 EventController，固定 Capture 收集候选，不提供 SetPhase。
默认 Enabled=true、Actions=Copy；一个控制器不能跨 Widget 共享。
Position 是原始按下点换算到源当前局部的 DIP，不是越过阈值的位置。
候选期间源被移除/换宿主则取消，不改为从其他控件启动。

Prepare.Data 初始 nil。信号按连接顺序执行，后连接者可覆盖结果；最终 nil 表示拒绝。
参数指针仅本轮同步调用有效，不允许保存后异步补写。一次按压内，同一个源最多 Prepare 一次；
拒绝后仍可尝试原按下路径上的其他候选。

预览契约：

- Image=nil 使用原生默认反馈，不要求调用方提供图片。
- Scale=0 使用源宿主当前比例；其余必须有限且为正。显示 DIP 尺寸=像素尺寸/Scale。
- 图片 Bounds 非零原点在复制时归零；Hotspot 是相对预览左上角的 DIP，允许在图外，不静默钳制。
- 启动时复制像素并按 graphics 预乘契约转换；不依赖源/目标 Painter，不随 Motion 重画。
- 不自动调用 RenderWidget。其导出失败由 Prepare 处理者决定是否省略预览，不自动取消拖拽。
- 图片参数无效属于启动错误。图片有效而原生仅自定义预览不可用时允许退回默认反馈，
  但不能把整个 DnD 初始化失败也作为“预览问题”吞掉。

```go
source := gui.NewDragSource()
source.ConnectPrepare(func(e *gui.DragPrepare) {
    data := new(gui.DragData)
    data.SetFiles(selectedPaths)
    e.Data = data
    if img, err := gui.RenderWidget(item, 0); err == nil {
        e.Preview = gui.DragPreview{Image: img, Hotspot: e.Position}
    }
})
item.AddEventController(source)
```

只有预览来自 item 时才能直接以 Position 作热点。重新 Paint 可能包含当前 pressed 外观，
不承诺与之前显示帧完全相同；布局未完成时导出可失败而数据拖拽仍可继续。

### 源端后续操作的视觉反馈（2026-10-02）

`DragSource.SetFeedback(DragFeedback)` 的零值保留原生反馈；启动时复制到原生
会话，之后改 setter 不影响进行中的会话。`NeutralOutsideTargets` 只控制外部无接受
目标时的中性光标，`DisableReturnAnimation` 独立控制失败／取消回弹，不再合成一个 bool。
只有源提供未接受放下后的另一项操作模式（例如启用 Transferable 的多页标签栏）
才启用；普通文件拖放与单页标签保持原生默认反馈。标签栏不查询信号连接数量；
始终发出 DetachRequest 查询，以最终 handled=false 作为恢复源标签的默认防护。
视觉配置不保证应用一定创建新窗口，也不是原生目标接受证据。

这不是接受动作：Action=0、已识别 Canceled、Err、LocalTarget/LocalDrop 的契约不变。
在本应用已注册原生目标拒绝时，仍保留拒绝反馈；不承诺识别任意外部应用的拒绝原因。

- Windows：GiveFeedback 在 effect=0、未请求取消、指针不在本进程注册的目标时
  提供普通箭头并返回 S_OK。成功/拒绝的原生动作仍由 OLE 报告，不返回伪造 DROPEFFECT。
- Linux/X11：同样只在未接受且位于本进程已注册目标之外时使用主题普通指针。
  目标命中来自 XDND 的原生窗口记录，不从 Widget hover 推测。
- macOS：启动 NSDraggingSession 后立即将
  animatesToStartingPositionsOnCancelOrFail 设为 false，避免未接受后的系统回弹先于
  源端新窗口显示。已识别取消仍由 GUI 恢复源标签，不触发 DetachRequest。

依据：[IDropSource::GiveFeedback](https://learn.microsoft.com/en-us/windows/win32/api/oleidl/nf-oleidl-idropsource-givefeedback)、
[AppKit 回弹开关](https://developer.apple.com/documentation/appkit/nsdraggingsession/animatestostartingpositionsoncancelorfail)。
视觉配置由 GUI 明确请求，平台层只应用原生设置，不知道 TabBar 或新窗口策略。

## 5. 放置目标与动作协商

```go
func NewDropTarget(formats ...DragFormat) *DropTarget
func (*DropTarget) Enabled() bool
func (*DropTarget) SetEnabled(bool)
func (*DropTarget) Actions() DragAction
func (*DropTarget) SetActions(DragAction)
func (*DropTarget) Formats() []DragFormat
func (*DropTarget) SetFormats(...DragFormat)
func (*DropTarget) ConnectEnter(func(*DragMotion)) signal.Handle
func (*DropTarget) ConnectMotion(func(*DragMotion)) signal.Handle
func (*DropTarget) ConnectLeave(func()) signal.Handle
func (*DropTarget) ConnectDrop(func(*DropRequest)) signal.Handle
func (*DropTarget) ConnectError(func(error)) signal.Handle

type DragMotion struct {
    Position geometry.Point
    Format   DragFormat
    Allowed  DragAction
    Action   DragAction
}

type DropRequest struct {
    Position geometry.Point
    Format   DragFormat
    Action   DragAction
    Data     *DragData
    Accepted bool
}
```

目标固定 Bubble 语义，默认 Enabled=true、Actions=Copy。Formats 返回拷贝。
Enter/Motion 只允许修改 Action，Drop 只允许修改 Accepted；其他字段的外部改写
不改变框架内部锁定的信息。指针只在同步通知期间有效，最终结果在 Emit 后校验。

### 5.1 命中与跟踪

正常命中得到唯一可达分支，遵循现有可见/禁用、结构裁剪、装饰遮挡和模态规则。
从最深节点向祖先检查目标，同节点按附加顺序，不穿透到背后兄弟控件。
匹配格式和非空动作交集形成候选，查询 Enter；Action=0 则配对 Leave 后尝试祖先。
Motion 拒绝、活动目标/格式变化时先 Leave，再按新配置重新 Enter。

一个 Offer 同时只有一个活动目标。Enter 表示候选进入，可立即拒绝；Leave 表示跟踪结束，
可以是移出、完成、拒绝或失效，不能把它一律解释为放置失败。
Drop 业务回调只投递一次；读取失败、业务拒绝或业务错误之后不向祖先重新投递 Drop。

### 5.2 动作

Allowed 取源能力与目标配置交集，并遵守平台可观察的明确动作限制；为空则拒绝。
优先采用交集内的原生建议，否则按 Copy、Move、Link 选一项。原生建议不是能力位集合，
不能错误地用它进一步强制相交。平台知道用户明确请求某动作而不能满足时拒绝，不悄悄改为其他动作；
协议不能区分建议与强制请求时不能伪造该事实。

监听者可将 Action 改成 Allowed 中单个位或 0；多位、未知位或越界按拒绝处理。
修饰键变化即使鼠标不动也应重新协商。不在 GUI 统一硬编码 Ctrl=Copy，具体规则由原生适配处理。

### 5.3 数据读取与接收

释放时重新检查目标/动作，锁定目标、格式、宿主 DIP 落点和代次，进入 Reading。
读取完成再检查目标仍挂载于原宿主且允许接收，将锁定落点换算为目标此时的局部 DIP，
派发 Drop；不因布局变化换一个业务目标。Accepted 默认 false，最终 true 才表示接收。

如果 Drop 回调接收后把自己移除/销毁，不撤销已给出的 Accepted：业务可能已经发生。
停止访问销毁对象，平台完成使用独立会话信息。其他阶段销毁对象则使候选/读取失效。

```go
target := gui.NewDropTarget(gui.DragFormatFiles)
target.ConnectDrop(func(e *gui.DropRequest) {
    paths, ok := e.Data.Files()
    if !ok { return }
    enqueueImport(paths)
    e.Accepted = true
})
folder.AddEventController(target)
```

回调只快速接收、更新模型或排队；不要等待文件复制或模态确认。
原生成功表示接管操作，不等于业务完成；文件操作、事务回滚和后续任务错误由业务负责。
禁止保存事件指针稍后修改 Accepted。Move 不自动删除源，Link 不自动创建快捷方式。

## 6. 输入协调与状态机

### 6.1 源时序

```text
Idle → Candidate → Preparing → Starting → Dragging → Ending → Idle
```

| 退出情况 | Begin | End |
| --- | --- | --- |
| 未越阈值 / Prepare 最终 Data=nil | 无 | 无 |
| Data 非 nil，但校验或启动失败 | 无 | 一次，Err 非 nil |
| 正常启动 | 一次 | 最终一次 |
| 已识别取消且未完成投递 | 启动后有 | Canceled=true、Action=0、Err=nil |
| 取消请求晚于目标成功投递 | 启动后有 | 保留完成动作，不报告取消 |
| 松手无目标 / 目标拒绝 | 启动后有 | Action=0、Canceled=false、Err=nil |
| 原生故障 | 可能有 | Err 非 nil、Action=0 |

平台无法区分拒绝与用户取消时，保留可观察事实，不根据 Action=0 一律猜取消。
零动作也不证明没有外部目标；不增加精确 NoTarget/Rejected 公共承诺。
Position 取原生结束点，而不是结束后查询鼠标；取消、错误或源已销毁时无效。
macOS 采用 Firefox CurrentEvent/Escape 兼容规则；GUI 保留本地同步 Drop 已接受的
动作，重入 End 等待 Drop/Leave/Finish 完成后再通知一次。详见
[Tab 转移结果契约](DesignTabTransfer.md#11-采用框架实践的结果契约2026-09-30)。
先安装 Starting 会话，再调用可能同步回调的原生 API；Begin 表示会话提交，不保证最终放置成功。

### 6.2 候选和阈值

1. 只处理原生左键；保存原按下路径、宿主 DIP 起点及序列号。阈值为 dx²+dy² > 4²，
   不使用可能随布局/滚动改变的控件局部坐标计算距离。
2. Capture 阶段源加入当前手势序列，不 StopPropagation、不立即调用 Prepare。
   普通事件继续执行，保留现有焦点和点击处理。
3. 手势协调器在普通派发完成后、当前原生事件回调返回前裁决。记录传播停止边界：
   Capture 提前阻断则不启动未到达分支；Target/Bubble 阻断时保留同节点及以下候选，
   不让祖先越过子控件输入边界。Button 同节点源不能因 Click 先 StopPropagation 而失效。
4. 文本选择和滚动条在 Down 申请即时手势；若其候选赢得竞争，同次按下不启动 DnD。
   不通过抢占实现尚未设计的“选中文字拖出”。
5. 超阈值只检查原路径的有效候选，最深节点优先，同节点按附加顺序；拒绝者本按压不反复查询。
   没有启动则普通输入继续，不在鼠标经过另一个控件时更换源。
6. Begin 不能 App.Post 延迟执行；macOS 等平台需要的当前原生事件不能由合成坐标代替。

### 6.3 原生接管前取消普通按压

DragSource 越过阈值后至多调用一次 Prepare，冻结并校验数据，再申请竞争。
Prepare 返回 nil 表示该候选放弃；同一序列中的多个候选可能 Prepare，只有胜者
启动原生拖放。Prepare 不应提前执行删除源项目等不可回滚操作。

裁决先取消落选者，Click 的按压反馈通过 GestureCanceled 撤销且不发送 Clicked。
检查胜者仍有效后，源控制器结束 GUI 序列、安装 Application 源会话并同步调用原生 Begin。
启动失败不重放旧按压、不合成 Click，等待下一次真实按下。
接管后残余 PointerUp 仅结束本序列，不 Click；之后 hover 经正常输入恢复。
不能把拖拽期间所有失焦当取消，拖向外部应用本来就可能改变焦点。

## 7. 平台接口与事件通道

### 7.1 类型依赖

新增小的 platform/dragdrop 契约包，放 Action、Format、便携 Data、Preview、Result、Offer；
另外用 Feedback 描述单次会话的原生视觉反馈配置，不包含 GUI 拖出策略。
只依赖底层几何/标准库，不依赖 events/common/gui，不含 Go Local 对象。
events 导入它定义 DnD 事件，common 导入它声明服务，避免 events ↔ common 循环。
GUI DragData 组合便携数据和 Local 表；UI 只经 gui 导出类型。

```go
// platform.Platform 工厂；服务接口放 common，platform 按既有方式导出。
NewDragDrop(surface Surface) (DragDrop, error)

type DragDrop interface {
    SetFormats([]dragdrop.Format) error
    Begin(id uint64, data *dragdrop.Data, actions dragdrop.Action, preview dragdrop.Preview, feedback dragdrop.Feedback) error
    Cancel()
    Destroy()
}

// 平台事件接收者使用，不向 GUI 业务暴露。
type Offer interface {
    ID() uint64
    SourceID() uint64
    Formats() []Format
    Read(Format) error
    Finish(Action) error
}
```

Surface 必须是同平台的活 Window/Popup；只用于定位原生宿主，不向 Surface 塞 DnD 方法。
每个宿主一个服务，SetFormats 接收全部启用目标的便携格式并集，不逐 Widget 注册。
空集合不接收外来便携数据；Local 目标另使用内部会话标记，识别本地源后由 GUI 补充格式。

Begin 的 id 是 GUI 分配的非零序号；便携 Data 可以为空（仅 Local），平台仍能发起原生会话。
会话标记不包含 Go 指针或 Local 名称。SourceID=0 表示非本地/不能确认，不能仅用外部字节查本地表。
Begin 要求当前有效原生按压；无上下文、忙、销毁返回错误。Windows 可到嵌套原生循环结束才返回，
macOS/X11 可较早返回，上层不依赖返回时机。非 nil 表示尚未提交且无源 Begin/End；
已提交则经事件反馈终态，Begin 返回 nil，避免重复报告。

### 7.2 原生事件

在 platform/events 增加封闭 Event 实现：源 Begin/End，目标 Enter/Motion/Leave/Drop，
读取 Data。Enter/Motion 包含窗口局部 DIP、Offer、源动作、原生建议/可观察限制、
原生修饰键和同步 Action 应答槽。事实只读，应答仅在回调有效；更新 Event 注释，
像现有 Key 默认行为响应一样明确例外，不将平台事实偷偷改为 GUI 策略。

```text
native callback → Window/Popover.DispatchEvent → 手势协调器（源）或 rootBase 目标协调（目标）→ 控制器 → 信号
Offer.Read → native read → Data event → 同一 DispatchEvent 路径 → Drop → Offer.Finish
```

Read 返回 error 表示未启动、不会有 Data；返回 nil 后恰好一个 Data 结果，可以在 Read 返回前
同步到达，所以必须先设置 Reading 状态。首版每个 Offer 只读一个选定格式一次。
Data 携带 Offer ID、格式及解码值/错误。Local 快路径也经过宿主的目标协调处理，
不直接调用业务回调。格式不匹配不启动读取。

Enter/Motion 同步回复动作，Drop 最终 Finish 一次，0 表示拒绝；失效/重复 Finish 返回错误，
不操作已释放对象，GUI 内部应避免重复调用。读取前 Leave 结束跟踪；Drop 已开始后原生出现
Leave 不应丢掉传输，平台区分交互退出与最终完成，GUI 保留独立 Reading/Delivering 状态。

### 7.3 同步与异步边界

Windows OLE/AppKit Drop 通常需要同步返回接收结果。首版有限文本、路径和 MIME 在该回调内
读取、派发 Data、执行 GUI Drop 并 Finish，不额外嵌套循环等待业务。
X11 selection/INCR 异步完成后派发 Data，Finish 对应协议完成消息。
不能把 COM/AppKit 对象移到后台 goroutine 来假装消除原生阻塞。

保护上限：单格式便携数据 64 MiB，源便携数据总量 64 MiB，文件/URI 至多 100000 项；
目标按实际解码总量限制，不信任声称长度。预览另沿用 RenderImage 的像素上限，这不是总内存保证。
X11 异步读取/结束等待总截止 30 秒、无进展截止 10 秒，只有真实进展更新后者。
外部同步 OLE/NSPasteboard 调用不能保证被 GOUI timer 强制中断，不声称有统一硬超时。

## 8. 三平台适配

### 8.1 Windows

- GUI 线程正确建立 OLE STA，配对成功的 OleInitialize/OleUninitialize（含 S_FALSE）。
  核对已有 COM 初始化计数及退出顺序，不多 Uninitialize。仅 CoInitializeEx 不足以注册 OLE DnD，
  见 [RegisterDragDrop](https://learn.microsoft.com/en-us/windows/win32/api/ole2/nf-ole2-registerdragdrop)。
- 每 HWND 一个 IDropTarget，销毁前 RevokeDragDrop；IUnknown 计数、vtable、Go 回调锚点覆盖
  原生持有期。回调执行中请求销毁时，延后最终释放，不依赖 GC 恰好没有回收。
- 源实现 IDataObject、IEnumFORMATETC、IDropSource，调用 DoDragDrop；常规有限数据使用
  TYMED_HGLOBAL。GetData 交付独立介质，接收遵守 ReleaseStgMedium。
- Files 使用 CF_HDROP UTF-16 路径列表，Text 使用 CF_UNICODETEXT；单 URL 提供原生 URL
  表示，URI 列表/自定义 MIME 使用注册格式。不以 WM_DROPFILES 代替双向 OLE。
- 预览使用 [IDragSourceHelper](https://learn.microsoft.com/en-us/windows/win32/api/shobjidl_core/nn-shobjidl_core-idragsourcehelper)
  和目标侧 IDropTargetHelper；SHDRAGIMAGE 输入为非预乘 BGRA，因为 Helper 会自行预乘。
  创建成功后由 Helper 接管 HBITMAP；缺省用原生反馈光标。
- QueryContinueDrag 处理 Esc、源按键释放、取消请求；GiveFeedback 默认用原生光标，
  显式 NeutralOutsideTargets 的中性反馈规则见 §4 的源端视觉反馈。
  POINTL 屏幕坐标只在平台换成客户 DIP；本地识别核对真实 IDataObject 身份与本地登记。
  POINTL 与 `winapi.POINT` 同为两个有符号 32 位坐标；回调用 `cgo.NewCallback`
  按值接收该结构体，后续适配也传递结构体，不传播手工打包的 `uintptr`。
- Shell 可能请求辅助格式和 SetData；正确处理支持范围，不以 Move 结果直接推导删除源。
  [Shell 数据传输场景](https://learn.microsoft.com/en-us/windows/win32/shell/datascenarios)
  区分优化移动等情况。首版不宣称支持回收站语义、虚拟文件或异步 Shell 文件操作。

### 8.2 macOS

- 在 Surface 对应 NSView 注册类型并接入 NSDraggingDestination；以保留的原始 mouse-down NSEvent
  创建 NSDraggingSession，同一个 NSView 也实现 NSDraggingSource。原生会话结束前保留源和预览资源。
- Text、file URL、URL 映射原生 pasteboard 类型；文件逐项表达，保留批次和顺序。
  MIME 经原生转换/反向映射表处理，不把任意 MIME 名称当已注册 UTI。
  无系统映射者使用明确命名的应用私有 type，保证两个 GOUI 进程的同 MIME 往返。
- draggingEntered/Updated 返回动作；performDragOperation 读取有限数据并同步确认；
  源和目标结束回调都走正常事件通道。参考
  [GTK 原生 View 对接](https://github.com/GNOME/gtk/blob/4.20.0/gdk/macos/GdkMacosBaseView.c)，
  但绑定遵循 frameworks 类/方法规范，应用适配留 cocoa。
- NSImage 像素和逻辑尺寸分开设置；热点、Y 翻转和 backing scale 在平台边界换算。
  SourceID 通过原生源对象/会话序号与本地登记联合确认，不借用通用剪贴板。
- Cancel 使 GOUI 会话不再交付新业务结果，并尽力终止原生跟踪。不承诺所有 AppKit 版本
  都有立即关闭拖拽图的公开 API，不合成 Escape、不用私有 API；原生若继续跟踪，保留最小
  delegate/数据至真实结束后释放，期间不启动第二个会话。

### 8.3 Linux/X11

- 实现 XDND v3–v5，发布 XdndAware，按对端版本解释字段；校验 XdndProxy，处理 TypeList /
  ActionList。旧版本无完成动作字段时不猜 Move，更不能据此清理源数据。
- 源处理 Enter/Position/Status/Leave/Drop/Finished，目标回复 Status/Finished。
  XdndSelection 与 CLIPBOARD 分开；当前 clipboard 的单 pending 文本模型不能直接复用。
- selection 按 source/requestor/property/selection/时间戳关联；处理 SelectionRequest /
  Notify / Clear、PropertyNotify、销毁和超时，支持直接属性及 INCR 收发。
  INCR 按删除应答推进、空块结束；并发请求不得共享单 pending 回调。
- Text 优先 UTF-8；Files 与 URLs 使用正确编码的 text/uri-list。Files 只解码本地绝对
  file URI，不静默丢弃混合网络 URI 或把远端 hostname 当本地路径；一般 URI 应由 URLs 接收。
- 协议 root 坐标换成客户 DIP，覆盖负屏幕坐标及 HiDPI，不向 GUI 新增桌面坐标接口。
  原生按压、pointer grab、动作光标属于后端协议适配，不由 GUI 伪造鼠标事件模拟桌面拖拽。
- 预览与鼠标光标分离：会话拥有 override-redirect 预览窗口，Shape 1.1 空输入区域
  确保它不遮挡目标命中。合成器可用时上传预乘 BGRA；无合成器时使用二值 bounding
  shape，覆盖阈值为 alpha≥128，覆盖像素在此边界转为不透明色。Shape 1.1 不可用时
  省略预览，只保留动作光标。图片大小不再受鼠标光标大小限制；仍保留像素总量限制。
- libXcursor 只加载主题动作光标（dnd-copy/move/link/none，标准名称次选，最终箭头
  兜底）；XdndStatus 更新 active grab 的光标。预览以 root 像素位置减 DIP 热点的
  物理换算定位，热点可以在图外，不需要扩展图像。释放时隐藏预览，终态/启动失败
  销毁预览与会话光标；不新增公共 Window/Popup API。
- 参考 [GTK 4.20.0 X11 DnD](https://github.com/GNOME/gtk/blob/4.20.0/gdk/x11/gdkdrag-x11.c)
  的独立 drag surface 与动作光标。Xorg `ProcTranslateCoords` 同时检查输入 shape，
  空输入区域不会成为 `XTranslateCoordinates` 返回的 child；因此沿用现有目标查询，
  不临时隐藏窗口、不增补 GUI 命中补偿。源码见
  [X server dispatch.c](https://github.com/mirror/xserver/blob/xorg-server-21.1.4/dix/dispatch.c)。
- 本地识别核对活源 helper XID、selection owner 与会话登记，不能只信任 atom 内容。
  X11 本身不隔离同 X server 的恶意客户端，不将该识别夸大为安全认证。

## 9. 生命周期、重入与错误反馈

### 9.1 挂载和配置

宿主存在 DnD 控制器时，在 mount/附加流程建立私有适配并创建平台服务，不在 Paint 初始化。
之后按目标格式并集更新；一轮 UI 协调中合并重复更新。最后一个控制器移除后可保留空服务至
宿主销毁，避免频繁原生注册。移动子树需更新原/新宿主。

通过内部可选附加/移除通知衔接内置控制器，不向 Widget/EventController 必实现接口添加 DnD 字段。
WidgetBase 缺少 self 时使用结构身份与 mount 时实际 Widget，不伪造 WidgetBase 作为业务控件。
源配置在 Starting 冻结，活动时改 Actions 只影响下一次；目标配置变化重新协商，
Reading 期间格式/动作失效则拒绝当前读取，不改读另一个格式。

原生注册失败不让无关窗口创建失败，也不静默宣称可接收。保留错误事实，目标 Error 在安全
GUI 通知点报告一次；源启动通过 End.Err。未挂载时先配置/连接是推荐用法。
同一配置/注册代次不在每次 Motion 重复报错；重新挂载/配置变化可重试，不设无限重试计时器。

### 9.2 失效处理

| 事件 | GUI 处理 | 原生资源 |
| --- | --- | --- |
| 候选源移除/禁用/隐藏/换宿主 | 丢弃候选，无 Begin/End | 尚无会话 |
| 活动源移除/禁用/销毁 | 标记取消，停止新业务投递 | 请求 Cancel，保留到真正结束 |
| 目标离开/禁用/移除 | Leave，拒绝失效读取 | 完成拒绝/释放跟踪 |
| 新模态根阻断目标 | 重新验证可达性，拒绝失效目标 | 不转交另一个业务目标 |
| 宿主关闭 | 取消源、拒绝未交付 Offer、取消注册 | 最后回调退出再最终释放 |
| 应用退出 | UI 先断连接，会话先于宿主/Platform 清理 | 不先销毁 OLE/AppKit/X11 环境 |

每个外部回调后检查生命周期，不能靠 DisconnectAll。用代次和结构身份阻挡旧读取结果，
不能拿可重用的 Widget ID 作为会话身份。Drop 已确认接收后自毁的规则见 5.3。
显式持有已移除 DragSource 的业务仍可收到最终 End；UI 卸载先断开连接，不再调用旧组件。
内部终态恰好一次，数据/预览/Local 引用释放必须晚于原生使用期。
Cancel 幂等，不回滚已经交付的业务副作用；完成与取消竞争时，已确认完成优先。
同一本地会话正在 Delivering 时，回调引起的源移除/Cancel 先记为待取消，不在 Drop 信号
尚未返回时立即向原生提交取消；最终 Accepted=true 先完成接收，否则执行待取消。
此规则不改变外部原生故障的事实，也不保证能撤销外部目标已经执行的操作。

目标读取错误只发目标 Error，本进程源得到 Action=0 的普通结果，不复制同一错误重复报告；
源启动/传输故障走 End.Err。无目标、不兼容、正常拒绝不是错误，不默认打印日志或弹窗。

## 10. UI 与语义快照

### 10.1 单条声明路径

固定数据源使用 `DragSource().Data(data)`；动态数据、预览和错误反馈在同一描述符上
按需增加回调。目标使用 `DropTarget(formats...).OnDrop(...)`。View 的修饰方法接收
一个描述符，nil 清除；每个 View 各有一个 UI 管理的源和目标，GUI 自身仍可附加多个控制器。

```go
data := new(ui.DragData)
data.SetText(model.Title)

ui.Label(model.Title).DragSource(
    ui.DragSource().Data(data).OnEnd(handleResult),
)
ui.Label("Drop here").DropTarget(
    ui.DropTarget(ui.DragFormatText).OnDrop(func(r *ui.DropRequest) {
        value, ok := r.Data.Text()
        if !ok { return }
        model.Title = value
        r.Accepted = true
        app.RequestUpdate()
    }),
)
```

`DragSourceView.Data(*DragData)` 替换预置数据；nil 清除预置数据，不移除源。
Enabled 默认 true，Actions 默认 Copy。没有 Data 时仍可由 OnPrepare 提供；最终没有
数据则放弃候选。非 nil 的空数据由 GUI 校验，失败经 OnEnd.Err 报告。
OnDrop 不自动接收；Accepted 初始 false。格式顺序是目标匹配优先级。
描述符只是声明值，其指针不作为协调身份；Build/挂载/普通重建不启动拖放。

GUI 与 UI 的 `DropEvent` 统一更名为 `DropRequest`，表示数据已读取、业务需要同步确认。
Position、Format、Action、Data 是本次事实；只有 Accepted 决定接收结果。
请求指针仅在同步回调期间有效。ConnectDrop、OnDrop 保持原名；UI 使用 GUI 类型别名，
不另建 DropInfo 或 DropProposal。

### 10.2 Prepare、数据与重建

每次 Prepare 先以描述符当前的 Data 初始化 `DragPrepare.Data`，再调用 OnPrepare。
回调可以提供预览、替换数据，或写 nil 拒绝本次；回调返回后不得再次回退预置 Data。
`Data(...).OnPrepare(...)` 与反向链式调用顺序等价。一次通知使用同一份已协调描述快照。

UI 保存数据指针，不在每次重建复制载荷；GUI 在 Prepare 返回后校验并冻结本次数据。
重建改为 B 后，下一次 Prepare 使用 B，已经启动的会话继续使用冻结的 A。
应用应将声明数据视作只读，业务变动时创建新数据并请求更新；Local 对象仍是业务引用。
RequestUpdate 尚未完成协调时，Prepare 使用上一次已应用的声明。活动会话的数据和
动作不随声明更新，但之后的通知使用最近一次已协调的回调。

初次绑定应先保存回调、连接信号并设置格式/动作/启用状态，最后附加控制器。
后续更新先替换回调槽，再调用 GUI setter；setter 导致的 Leave/取消通知由最新回调接收。
重建不会人为调用 Prepare/Drop，配置变化则可能正常产生 Leave 或 End。
卸载先断 UI 信号，再移除自己拥有的控制器，释放闭包与数据引用；GUI 负责结束原生会话。

### 10.3 预览、校验与反馈

预览通过 OnPrepare 同步设置 `gui.DragPreview`，保持 Image、Scale、Hotspot 契约。
需要控件图像时，用 [Widget ID 查询](DesignWidgetIdentity.md) 定位已挂载对象，再调用
`gui.RenderWidget(widget, 0)`；重新 Paint 不等于捕获上一帧。查询失败可以省略预览继续，
或将 Data 设为 nil 放弃本次候选。不能 Post 后补写 Prepare，见 §6.2。

Enter/Motion 依据 GUI 选定的格式和动作协商；DropRequest 在读取后校验内容，
同步设置 Accepted。业务拒绝不是传输错误。源错误经 OnEnd.Err，目标注册/读取错误经
OnError；不默认弹窗、记录日志或重复上报。Move 的源数据清理由业务负责。
高亮由 Enter/Leave 回调驱动模型并显式 RequestUpdate，不自动施加样式。

### 10.4 Snapshot、参考与验证

现有 DragDropInfo 继续报告能力、格式、动作和活动状态，不暴露 payload、路径或
Local 对象。源数据 nil 不自动改变 Enabled，也不影响原 Role、Bounds 或子树。

参考 [SwiftUI draggable](https://developer.apple.com/documentation/swiftui/view/draggable(_:))
的数据声明和 [GTK4 DragSource](https://docs.gtk.org/gtk4/class.DragSource.html)
的预置内容/Prepare；GOUI 使用普通 Go 数据与现有 GUI 信号，不引入 Transferable。
参考的是公开 API 契约，不代表 GOUI 原生桌面已验收。

包内回归覆盖 Data/OnPrepare 顺序、覆盖和清空、重建与卸载、无重复控制器、
当前会话冻结数据、默认拒绝、配置变化和 Snapshot。输入经正常分发路径验证。
窗口用例同时使用 Data 与 OnPrepare 生成预览，检查重建后的文本、1x/2x 热点和
拒绝路径；各平台原生桌面验收单独记录。

## 11. 测试与验收

遵循 [测试约定](Testing.md)。包内可用假原生服务，但通过正常 DispatchEvent 驱动，
不能直接 Emit 业务信号代替输入测试。真实原生互操作必须有真实桌面输入，
不能把缺少原生 NSEvent/X 时间戳的直接构造事件当跨进程验收。

### 11.1 无桌面逻辑回归

- 数据：空值、复制隔离、Local/MIME 区分、路径/URI 编解码、UTF-8/NUL、尺寸上限、
  Files/URLs 冲突、空格/中文/百分号/UNC、URI CRLF/注释行。
- 源：阈值两侧、布局移动、Button 子树竞争、子控件阻挡祖先、即时手势优先、Prepare 拒绝只一次、
  同节点顺序、启动失败、同步原生 Begin/End 重入。
- 接管：Pressed(false) 被通知，无误 Click，残余 Up 不 Click，后续普通点击正常。
- 目标：裁剪/遮挡/可见/禁用/模态、祖先回退、Enter/Leave 配对、静止修饰键、格式优先级、
  非法 Action、数据完成同步/异步、Drop 不二次投递。
- 生命周期：每种信号中销毁/移除控件或窗口、Reading 中关闭、旧结果、已接收后自毁、
  Cancel 与结束竞争、一次性终态、原生引用释放、Local 引用不泄漏。
- UI：重建不启动、不重复连接、回调替换、nil 清除、卸载、会话身份稳定。
- 快照：能力/活动状态、合并去重、无 payload 泄露、无输入捷径。
- 原生纯逻辑：FORMATETC/UTF-16/URI 映射、XDND 版本/INCR 状态机、ABI、引用计数。
  不在包内测试创建窗口、连接 X display 或修改真实剪贴板。

### 11.2 窗口互操作矩阵

逐步增加 tests/window/platform/dragdrop、tests/window/gui/dragdrop、tests/window/ui/dragdrop。
入口注明环境、操作、预期、退出、复位与文件副作用。只在独立临时目录生成测试文件，
源和目标目录不同，记录原文件哈希；不使用用户真实文件验证 Move/Delete。

| 场景 | 必须核对 |
| --- | --- |
| Explorer / Finder / GTK 文件管理器 → GOUI | 多文件/目录、中文空格路径、Copy、拒绝区域 |
| GOUI → 文件管理器 | 实际复制成功、哈希一致、源仍存在、源一次 End |
| 文本/URL ↔ 原生应用 | UTF-8、链接不被执行、动作正确 |
| 两个 GOUI 进程传 MIME | 字节一致，Local 不跨进程 |
| 应用内同树/跨窗口/Popover | 原生事件路径、Local 身份、目标与坐标正确 |
| Button / ScrollView | 小位移点击、大位移不点击、滚动条/选区不被抢占 |
| Esc / 拒绝 / 销毁 | 无误 Click、无悬挂高亮、可再次启动、结果不重复 |
| 1x / 2x / 跨屏 | 预览 DIP 尺寸、热点、客户区落点；不要求跨屏重画预览 |
| UI 重建 | 无重复会话/旧回调，不把旧 Drop 投给复用 ID 的新控件 |
| 大于 X 单属性容量 | INCR 收发，中断、对端退出与超时无遗留状态 |

本机人工或 AI Computer Use；按用户约定远端只做无头测试/编译，不默认操作远端桌面。
记录 OS、Linux WM/文件管理器、绘制后端及比例。交叉编译不等于互操作通过，启动不等于拖放验收。
不依赖 DevServer。每切片执行 go test ./...，按风险补 race、三平台构建，分别记通过/失败/跳过/未执行。

验收入口分工：platform/dragdrop 检查原生文件传输和一次 Begin/End，源通过
`-result copy/cancel/reject` 指定操作预期；gui/dragdrop 提供 Local、Text、Files、
URLs、MIME 独立源及同名接收区，避免 Local 优先级遮蔽便携格式传输。Files 和 URLs
在 X11 同用 text/uri-list，验收时必须按目标语义分区，不能把 HTTPS 放进 Files
目标后将解码拒绝算作实现故障。默认文件由程序在临时目录生成，正常退出清理。
ui/dragdrop 用 revision 文本和可见计数核对重建后的回调更新、禁用和 nil 绑定移除。
操作步骤、退出和副作用以各入口注释为准；RECEIVED 与 VERIFIED 明确区分。

## 12. 切片、文件归属与交付门槛

| 切片 | 内容 | 退出条件 |
| --- | --- | --- |
| P0 平台文件 Copy | 契约/事件、三个原生服务、注册/注销、路径、取消 | 三平台与文件管理器双向 Copy |
| P1 平台完整数据 | 文本/URL/MIME、INCR、预览、动作、源身份、故障清理 | 跨进程往返、1x/2x、取消/故障 |
| P2 GUI | 数据、控制器、候选/取消、Local、宿主生命周期、快照 | 逻辑回归及 GUI 原生窗口用例 |
| P3 UI | 链式声明、通用协调、信号、声明式用例 | 重建/卸载回归及应用内外拖拽 |

P0/P1 先通过独立平台程序验证，不用上层补偿遮蔽原生错误；三平台分别记录进度，
不能只完成 Linux 就标记公共能力完成。

预计修改归属：

- platform/dragdrop 放平台契约/有限数据模型，platform/events 放 DnD 事件，common/Platform 接入工厂。
- windows/win32、darwin/cocoa、linux/x11 放适配/状态机；SDK/frameworks/libs 只表达原生绑定。
- gui/drag_drop.go 放数据与控制器公共 API，gui/drag_dispatch.go 放宿主注册、应用会话与目标事件分发；
  event.go/input_controller.go/widget.go 接入通用手势裁决、按压取消和生命周期衔接。
- ui/drag_drop.go 做薄协调，info.go 做语义元数据；测试按所属包与 tests/window 分开。

不顺带实施文本拖出、自动滚动、文件任务系统、剪贴板重构或窗口输入区域 API。
新增原生约束如果迫使公共契约改变，应先提供证据更新设计并确认，不能静默平台特判。

### 实施记录（2026-09-23）

已实现 `platform/dragdrop` 的数据/格式契约、三平台原生源和目标、GUI 控制器与
会话协调、UI 描述符和语义快照。Linux/X11 本机 Xfwm4/Thunar、1x：同窗口 Local、
GOUI → Thunar 文件 Copy（独立 SHA-256 一致）、Thunar → GOUI 文件、256 KiB INCR、
同窗口/跨窗口/Popover Local、UI 重建后重复拖放以及 Esc 取消均已验证。
自定义预览在 1x 为 48×48 物理像素/热点 (12,12)，在 2x 为 48×48/热点
(24,24)；2x 的 Popover 放置和 Esc 取消也已验证。Popover 随 owner 移动的定位
属于另一个问题，本轮未作为 DnD 验收条件。
platform、gui、ui 的窗口用例分别构造同样规格的 48 DIP、带透明边缘和热点标记的
自定义预览；`-preview=false` 保留无图时的原生回退验收。此用例调整不代表新增
Windows/macOS 桌面验收结果，三平台仍须分别核对逻辑尺寸、热点与透明合成。
Windows 10 amd64 远端仅做 OLE 数据对象、文件/URL、拖拽图像和 COM 辅助对象的
无头测试；macOS 15 arm64 远端仅做 AppKit 协议/绑定、PasteboardItem/NSData/URL
无头测试。两者的真实 Explorer/Finder 互操作、窗口销毁竞态和 HiDPI 尚未验收，
不能因编译和无头测试通过就标记 P0/P1 全部完成。
因此 P0 尚未完成，不能以新契约或编译通过代替桌面互操作验收。
