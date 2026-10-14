# GOUI 鼠标指针(Cursor)设计

> 状态：基础光标已实现，已补齐缩放方向及 Linux GUI Integrated 自动反馈。下述 MVP 设计保留背景，新增规则见下一节。
> 定位:记"指针形状如何跨三层流动、平台与 gui 如何配合、命名与机制为何这样定"。总哲学见 `doc/Design.md`;事件/hover/crossing 见 `doc/GuiDesign.md` 与 `gui/event.go`;可选平台能力的形状范本见 `doc/DesignIME.md`。参考 GTK/Qt/Flutter/Web,**吸取经验而非照搬**。

---

## 缩放光标增量（已实现）

保留 `CursorResizeHorizontal`、`CursorResizeVertical`、`CursorResizeNWSE`、
`CursorResizeNESW` 四种通用轴向语义，供分隔条或图形编辑等场景使用。
新增 `CursorResizeLeft/Right/Top/Bottom/TopLeft/TopRight/BottomLeft/BottomRight`
八方向边缘语义，供窗口边框使用。通过 platform、gui 和 ui 的既有光标值模型流转，不新增接口。
八方向不要求八张不同图像：后端或主题可以合并图像，但公共层不能提前丢失边缘方向。

- X11 采用 GTK4/Chromium 的语义名称和传统别名，例如 `w-resize → left_side`、
  `se-resize → bottom_right_corner`。libXcursor 负责主题资源、继承和尺寸选择；
  库不可用或名称加载失败才回退对应的 cursor font。禁止按发行版或桌面环境写映射分支。
- Windows 将八个边缘方向映射到四种现有 `IDC_SIZE*` 光标。
- macOS 水平/垂直使用公开 `resizeLeftRightCursor`、`resizeUpDownCursor`；
  对角线先检查公开 `frameResizeCursorFromPosition:inDirections:` 是否可用，
  通用对角线传入 TopLeft/TopRight 和 All，边缘语义传入具体位置和 All。
  旧 AppKit 的边缘光标回退水平/垂直；对角线回退箭头，不使用私有选择器。
  SDK 绑定保持在 frameworks/appkit，能力检查和回退留在 cocoa 后端。

Linux GUI 自绘 Integrated 对最终 `ChromeRegion` 结果进行光标映射，不另写边框命中算法。
八个区域分别使用八方向光标，不再两两合并为轴向光标。
它优先于底层内容 Widget 的光标；Client 覆盖、模态和捕获时恢复已有 Widget 光标解析。
窗口保存最后的客户区指针位置及是否在客户区内；离开/失焦清除有效位置，状态和布局变化
重新解析，仅普通窗口状态启用自动缩放反馈。原生窗口仍由系统管理边框光标，None 仍由用户管理。
不从透明像素推导光标，也不新增鼠标穿透功能。相同形状继续去重。

验证：八方向映射、最终信号覆盖、离开/失焦、最大化/全屏/最小化、捕获/模态、None 边界、
资源销毁，以及三端编译；X11 可用真实原生输入配合 XFixes 光标图像查询检查切换。

### X11 主题设置和生命周期

- 光标配置属于平台事实，内部读取 XSETTINGS 的 `Gtk/CursorThemeName`、
  `Gtk/CursorThemeSize`，缺失时读取 X Resources 的 `Xcursor.theme/size`。
  显式 `XCURSOR_THEME/SIZE` 环境变量优先；仍缺失时使用默认主题与 libXcursor 初始尺寸。
- 使用 GOUI 自身 X11 事件循环监听设置属性、根窗口 RESOURCE_MANAGER 和 MANAGER 通知，
  支持设置服务退出、重启；不驱动额外 GTK/GLib 主循环，不轮询设置。
- 选择区 owner 查询、订阅及属性读取在短暂 XGrabServer 内完成，防止 owner 在查询后销毁。
  解析、磁盘资源加载与光标更新在释放 server grab 后执行；此机制不参与窗口缩放同步。
- 配置由同一 display 的 Cursor 共用，资源仍由各 Cursor 缓存和释放。配置改变后清缓存、
  重新应用当前形状，鼠标静止和 GUI 同形状去重都不会阻止更新；隐藏光标保持隐藏。
  最后一个 Cursor 销毁后取消当前设置窗口和根窗口订阅。
- 测试覆盖两种字节序、截断/非法 XSETTINGS、名称回退、真实属性通知、服务重启、
  资源重载与销毁。测试使用私有选择区和窗口，不修改用户桌面设置。

参考：
[GTK4 边框命中](https://github.com/GNOME/gtk/blob/4.18.6/gtk/gtkwindow.c)、
[GTK4 X11 光标](https://github.com/GNOME/gtk/blob/4.18.6/gdk/x11/gdkcursor-x11.c)、
[Chromium 边框光标](https://github.com/chromium/chromium/blob/main/ui/wm/core/compound_event_filter.cc)、
[Chromium 名称映射](https://github.com/chromium/chromium/blob/main/ui/base/cursor/cursor_factory.cc)。


## 0. 现状与目标

基础光标、轴向／八方向缩放光标和三平台映射已实现。窗口按当前 hover／已接受手势的捕获目标解析 Widget 光标；上方增量补充 Linux Integrated 的边缘反馈。

目标:让**控件**声明"指针悬停在我身上时是什么形状",框架据此在指针进入/离开时切换原生光标。约束:

- 平台层只提供"把某个窗口的当前光标设成 X"这一**事实能力**,不含"哪个控件用哪种光标"的 GUI 策略。
- gui 层复用**已有的 hover/crossing 机制**决定当前指针下是哪个控件、该用什么光标,不新造一套命中/追踪。
- 不往通用 `Widget` 接口加一堆方法——光标是**可选**的控件属性,像 IME 一样只有需要的控件才关心。
- ui 层薄暴露:声明式 `Cursor(...)`,内部落到 gui setter。

## 1. 从别人身上学什么、避什么

| 框架 | 借鉴 | 规避 |
|---|---|---|
| **Web/CSS** | `cursor` 是元素的一个属性,命中最内层元素即取其 cursor(就近覆盖) | 把 cursor 塞进庞大的 CSS 级联;goui 的 style 系统刻意不做隐式继承 |
| **GTK** | `gdk_window_set_cursor` 是窗口级事实;`GtkWidget` 有 cursor 属性,按 motion 更新 | `GdkCursor` 富对象(带主题/动画);MVP 只需一组具名标准光标 |
| **Qt** | `QWidget::setCursor` + 悬停时栈式解析;标准 `Qt::CursorShape` 枚举 | cursor 栈 + `unsetCursor` 的隐式继承链偏复杂 |
| **Flutter** | `MouseRegion.cursor` + `MouseTracker` 命中最上层区域取其 cursor;`SystemMouseCursors` 具名枚举 | `MouseCursorSession`/延迟解析那层机制偏重 |

**一句话教训:光标形状用一组具名标准光标(枚举),不做富光标对象;命中就近(最内层控件的光标胜出);平台只负责"设当前窗口光标",追踪与决策留在框架层。**

## 2. 命名与类型

| 名字 | 层 | 含义 |
|---|---|---|
| `common.CursorShape` | platform | 具名标准光标枚举(`CursorDefault`/`CursorText`/`CursorPointing`/…)。平台层类型,因为最终要映射到原生光标 ID |
| `platform.Cursor` | platform | 每窗口的光标能力对象——**与 `Window`/`Popup`/`InputMethod`/`Painter` 同为"能力名词"**,不是"一个光标"。经 `Platform.NewCursor(window) (Cursor, error)` 获取,和 `NewInputMethod`/`NewClipboard` 一个待遇;不支持返 `ErrUnsupported`。MVP 只有 `SetShape(CursorShape)` + `Destroy()`;方法用 `SetShape` 而非 `Set`,给后续 `SetImage`/`SetAnimate` 留同族命名空间(见 §8) |
| `gui.Cursor` 接口 | gui | 光标值的统一抽象(封印接口,只有 gui 提供的类型可实现)。MVP 唯一实现是 `CursorShape`,将来加自定义图像光标时用 `imageCursor` 结构扩展,无需改 `SetCursor` 签名——迁移对调用方近乎无破坏(§8.1)。**不与** `platform.Cursor`(能力对象)冲突,前者是"光标的值"(应用面),后者是"设光标的能力"(实现面) |
| `gui.CursorShape` | gui | 新类型包装 `platform.CursorShape`(不是别名,因为需要加方法),实现 `Cursor` 接口。常量 `CursorDefault`/`CursorText`/… 类型为 `gui.CursorShape`,可直接传给 `SetCursor(Cursor)`,让 gui/ui 消费者不直接 import platform |
| `WidgetBase.Cursor()/SetCursor(Cursor)` | gui | 控件的光标属性,getter 默认为 nil（未设置）；解析时 nil／CursorDefault 均不覆盖。`SetCursor` 仅设字段,不发请求——光标在指针/按键事件派发后自动重解析(§4.3) |
| `ui.Cursor` / `ui.CursorShape` + 常量 | ui | re-export `gui.Cursor`/`gui.CursorShape` 及 `ui.CursorPointing` 等常量(与 `ui.ColorScheme` 同法),让应用只写 `ui.`、不 import gui/platform |
| `ui.View.Cursor(ui.Cursor)` | ui | 声明式设置,协调时落到 gui `SetCursor`;不调用=不覆盖控件自带光标 |

### 2.1 CursorShape 枚举(MVP 集合:三平台共同支持)

MVP 只取 win32/x11/cocoa **都有原生对应**的标准光标:

命名采用**形状/外观导向**(对齐类型名 `CursorShape`,与 Qt `Qt::CursorShape`、macOS `NSCursor` 一路),全为单名词、单一维度,避免混入语义词/状态词:

| CursorShape | 外观 | win32 IDC | x11 XC | cocoa NSCursor |
|---|---|---|---|---|
| `CursorDefault` | 箭头(也表示"不覆盖,用继承") | `IDC_ARROW` | `XC_left_ptr` | `arrowCursor` |
| `CursorText` | 文本 I-beam | `IDC_IBEAM` | `XC_xterm` | `IBeamCursor` |
| `CursorPointing` | 指向手(食指,可点击处) | `IDC_HAND` | `XC_hand2` | `pointingHandCursor` |
| `CursorCrosshair` | 十字 | `IDC_CROSS` | `XC_crosshair` | `crosshairCursor` |
| `CursorForbidden` | 禁止圈 | `IDC_NO` | `XC_X_cursor` | `operationNotAllowedCursor` |
| `CursorNone` | 无(隐藏,指针仍被追踪) | `SetCursor(NULL)` | 透明 XCreatePixmapCursor | 透明 NSImage 光标 |

命名取舍:
- **`CursorPointing`** 而非 `CursorPointer`——`Pointer` 与 cursor 同义(冗余,且看不出是手型);`Hand` 又易被读成拖拽用的张开/握合手掌。`Pointing` 描述"食指指向"的手势,零歧义,也为将来 `OpenHand`/`ClosedHand`(拖拽)留出空间。
- **`CursorForbidden`** 而非 `CursorNotAllowed`——统一为单名词,对齐 Qt `Forbidden`。
- **`CursorNone`** 而非 `CursorHidden`——"无形状",与形状维度一致,对齐 Web `cursor:none` / Flutter `none` / Qt `BlankCursor`。

Cursor() 返回显式设置值，未设为 nil；CursorDefault 是显式的通用默认值，解析时同样让更外层提供默认。标准缩放光标已补齐，见上方增量；不因零值 getter 伪造用户配置。

## 3. 三层职责

```
platform.Cursor     每窗口的原生光标能力对象(吸收平台复杂度)
  · SetShape(shape)  把该窗口"当前光标"设为 shape 对应的原生光标(win32: SetCursor+缓存以应对 WM_SETCURSOR;x11: XDefineCursor;cocoa: NSCursor.set / invalidateCursorRects)
  · 光标句柄由平台缓存(标准光标是共享资源,懒加载一次)
  · 经 Platform.NewCursor(window) 获取,不支持返 error
        ↑ 无回调:光标是纯输出能力(像 Painter,不像 IME 需要上行)
gui.Window           指针驱动的「光标路由器」(持有 platform.Cursor,nil=不支持则整体兜底为 no-op)
  · hover 路径变化时,解析出"当前指针下的有效光标",调 cursor.SetShape
  · 有效光标 = 已接受手势捕获目标路径优先，否则 hover 路径；取最内层非 nil／非 Default 值
  · 复用 EventDispatcher 已算好的 hoverPath,不重复命中测试
gui.WidgetBase       控件的光标属性:Cursor()/SetCursor(Cursor);显式 getter 默认 nil
  · SetCursor 仅设字段,不发请求——光标在指针/按键事件派发后自动重解析(§4.3)
```

### 3.1 为什么是"每窗口一个 platform.Cursor",而不是 Painter 那样每 Surface

光标是**窗口级**状态(原生 API 都作用于窗口:`XDefineCursor(window)`、win32 光标随 `WM_SETCURSOR` 在窗口内解析、`NSCursor` 按窗口的 cursor rects)。popup 是独立原生窗口,若将来 popup 内也要改光标,各自 `NewCursor(popupWindow)` 即可,契约不变。所以能力挂在 `platform.Window` 维度,和 IME 完全对称。

### 3.2 为什么无 handler / 无总线事件

IME 要把 commit/preedit **上行**给控件,所以有 `InputMethodHandler`。光标是**纯下行输出**:框架决定形状 → 平台设置,没有需要回调的平台输出。因此 `platform.Cursor` 只有 `SetShape`/`Destroy`,形状同 `Painter`(纯输出、不碰事件总线)。

## 4. 解析规则:就近覆盖(nearest-wins)

已接受手势存在 captureTarget 时，使用有效捕获目标到 root 的路径；否则使用实时 hoverPath。取最内层已启用节点的非 nil／非 CursorDefault 光标，无覆盖时用 CursorDefault。捕获期间 hover 仍正常更新，结束或取消后恢复实时悬停光标，避免 SplitView／表格列宽快拖时闪回箭头。

- 与 Web/Flutter/Qt 一致:最具体(最内)的控件说了算。
- 一个 `Label` 放在 `Button` 里:Label 不设光标(Default),Button 设 `CursorPointing` → 悬停 Label 仍显示手型(向上找到 Button)。这正是期望行为。
- `TextInput` 设 `CursorText` → 悬停其上显示 I-beam。
- 空白容器区域(路径里都没设)→ `CursorDefault`(箭头)。

**为什么就近而不是最外优先**:符合直觉——你悬停在哪个具体控件上,就用那个控件的语义;外层容器只提供兜底。

## 4. 触发时机(事件驱动,无需请求)

光标只在**指针下的控件集合变化**或**控件自身光标值变化**时需要重解析。方案:

- **主路径:指针/按键事件后自动重解析**。`window.DispatchEvent` 在派发 `PointerEvent` 或 `KeyEvent` 后,调用 `applyCursor()` 重新从 `dispatcher.hoverPath` 解析光标并应用。触发场景:
  1. **指针移动**:hoverPath 变化 → hover 的控件换了 → 取新控件的光标。
  2. **按键(如打字)**:hoverPath 不变,但被 hover 控件在按键处理里调了 `SetCursor`(如 TextInput 打字时设 `CursorNone`)→ 解析拿到新值。
  3. **PointerLeave**:hoverPath 清空 → 解析为 `CursorDefault`。
- **无需 `RequestCursorUpdate`**:控件 `SetCursor` 仅设字段,不发请求。下次输入事件(指针或按键)派发后自动重解析,开销可忽略(已有的 `applyCursor` 内部 dedup 防止冗余原生调用)。这和 IME caret 一致——caret 变化搭 paint 的便车,光标变化搭事件派发的便车,两者都不需要独立的 per-widget request。
- **未覆盖场景**:纯定时器/网络回调改光标(无输入事件)MVP 不管,应用需要时自行 `RequestPaint` 或等下次指针动。

**与 IME 的对称性**:IME caret 由**控件内部**(插入点移动)触发,搭重绘便车;光标由**控件外部**(指针跨控件)触发,集中在 dispatcher 的指针事件里处理。两者本就都不该要 per-widget request,删掉 `RequestCursorUpdate` 恢复了这个对称。

## 6. 平台后端落地

契约:`common.CursorShape` 枚举 + `common.Cursor{ SetShape(CursorShape); Destroy() }` + `Platform.NewCursor(window) (Cursor, error)`。不支持的后端返 `ErrUnsupported`,gui 拿不到就整体降级(光标恒为窗口类默认)。

- **win32(已落地)**:标准光标 `LoadCursor(0, IDC_*)`(已有绑定,见 `winapi/api_windows.go`)缓存进 map。`SetShape(shape)` 记录当前 shape 并调 `SetCursor(hcursor)`;**关键**:Windows 每次指针在客户区移动会发 `WM_SETCURSOR`,DefWindowProc 会把光标重置回窗口类光标,所以必须在 `WndProc` 处理 `WM_SETCURSOR`(命中客户区时用缓存的当前光标 `SetCursor` 并返回 TRUE),否则光标会闪回箭头。这块是 win32 的必经工作(窗口类光标应改为不设/NULL,交由 WM_SETCURSOR 决定)。
- **x11(已落地)**:标准光标优先按名称从 libXcursor 当前主题加载，失败才使用 `XCreateFontCursor(display, XC_*)`，缓存在 map 中；主题变化更新缓存（详见上方增量）。`SetShape`→`XDefineCursor`；`CursorNone` 用 `XCreatePixmapCursor` 创建 1x1 透明光标；`Destroy`→`XFreeCursor` 释放资源。
- **cocoa(已落地)**:`NSCursor` 具名工厂(`arrowCursor`/`IBeamCursor`/`pointingHandCursor`/`crosshairCursor`/`operationNotAllowedCursor`)。`SetShape` 解析出 `NSCursor` 存到 window 上并调 `set`;**关键**:AppKit 会在指针移动时把光标重置回默认,且 gui 层对同 shape 做了 dedup(不重复 `SetShape`),所以 cocoa 后端在 `mouseMoved`/`mouseEntered`(即 `emitPointer` 之后)主动 `reapply()` 重设当前光标,与 win32/x11 的"命中即设"对齐。`CursorNone` 用全局 `[NSCursor hide]`/`[unhide]` 配对(自动跨移动持续,无需 reapply);window 上记一个 `hidden` 标志保证 hide/unhide 严格配平,`Destroy` 时若仍隐藏则补一次 `unhide`,避免光标卡死不可见。appkit 绑定已补 `NSCursor` 类 + 上述工厂/`set`/`hide`/`unhide` 选择子。

按 `doc/Design.md`"平台先验证再上层"的顺序:先 win32 落 `platform.Cursor` 并真机验证光标能切换且不闪回,再接 gui 路由,最后 ui 暴露。

## 7. gui / ui API 面

gui(控件作者/自定义控件):

```go
// gui.Cursor 接口(封印,只有 gui 提供的类型可实现)
type Cursor interface { isCursor() }  // MVP 唯一实现:CursorShape

// WidgetBase
func (w *WidgetBase) Cursor() gui.Cursor
func (w *WidgetBase) SetCursor(cursor gui.Cursor) // 仅设字段,不发请求;下次输入事件后自动重解析

// 内建控件默认值(在各自构造里 SetCursor):
//   TextInput → CursorText;Button → CursorPointing;Label/Box → 不设(Default)
```

ui(应用作者,声明式;全程只用 `ui.`,不 import gui/platform):

```go
TextInput().Cursor(ui.CursorText)    // 覆盖默认
Button("OK")                          // 默认已是 CursorPointing,无需写
Box(...).Cursor(ui.CursorDefault)     // 显式回退箭头
Label("...").Cursor(ui.CursorNone)    // 隐藏
```

`ui.View.Cursor` 是纯声明语法,协调时调 gui `SetCursor`。修饰符字段类型为 `ui.Cursor`(接口),**nil = 未调用 → 不覆盖控件自带光标**;非 nil(含 `CursorDefault`)= 显式指定。这天然实现了 Flutter `MouseCursor.defer`(交给下层解析)的语义,无需专门哨兵。参照 SwiftUI `pointerStyle`/Flutter `MouseCursor`:声明层只认框架自有词汇,不暴露平台原生类型——goui 里 `gui` 层已提供语义抽象,ui 经 re-export 复用(对齐 `ui.ColorScheme`),不另造平行枚举。与 DesignUI 的"OnXXX 只是声明式语法、内部管理 gui 状态"一致。

## 8. 边界与非目标(MVP)

### 8.1 自定义光标的候选扩展（未实现）

当前只支持具名标准光标枚举。自定义图片光标须按需求重新评估，见 [未来路线](Roadmap.md#future-input)；下面工厂及 SetImage 只是候选代码，不是公共 API。**已占坑**:`gui.Cursor` 封印接口 + `CursorShape` 实现它,将来加自定义光标时:

```go
// 在 gui/cursor.go 加新类型(已有 Cursor 接口无需改)
type imageCursor struct {
    img      image.Image
    hotspot  geometry.Point
    fallback CursorShape // 平台不支持时降级
}
func (imageCursor) isCursor() {}  // 实现 Cursor

// 加工厂函数
func NewImageCursor(img image.Image, hotspot geometry.Point, fallback CursorShape) Cursor

// platform.Cursor 加新方法
func (c *platformCursor) SetImage(img image.Image, hotspot geometry.Point) error
```

关键:**`SetCursor(Cursor)` 签名无需改**,因为 `CursorShape` 早已实现 `Cursor`,旧代码 `SetCursor(CursorPointing)` 无需修改继续工作。对调用方近乎无破坏,只需 window 的 `applyCursor()` 加一个类型分支(检测到 `imageCursor` 就调 `cursor.SetImage`,平台拒绝就用 `fallback`)。

`platform.Cursor` 的方法族按 `SetXxx` 命名也是预留——后续平滑加 `SetImage`/`SetAnimate` 不破坏现有 API:`Set*` 前缀让 shape/image/animate 三种源共处一个自洽的命名空间。

### 8.2 其他非目标

- **隐藏光标与手势捕获已支持，指针锁定尚未实现**:隐藏(指针不可见但仍在原位被追踪)作为 `CursorNone` 形状值,复用全部现有机制;"打字时隐藏、鼠标一动恢复"由控件自行在 keydown/mousemove 切换 `None`/`Text` 即可,无需新机制。已接受手势的输入捕获由 GUI 协调器管理；原生相对位移／锁定属于另一项待评估能力，见 [未来路线](Roadmap.md#future-input)。
- **不做标准光标热点**:标准光标由系统提供,goui 不管热点;仅自定义光标(上面预留)才需要传 hotspot。
- **不进 style 系统**:光标是交互语义、不影响布局与绘制,不属于 `style.Style`(样式"影响布局和绘制,不替代控件行为/语义",见 AGENTS.md)。它是控件的行为属性,和 `Focusable` 同类,放 `WidgetBase`。
- **HiDPI**:标准光标由系统按 DPI 渲染,goui 不缩放,无需处理。
## 9. 原落地路径（历史，不是当前待办）

1. **platform 契约 + win32**:`common.CursorShape`/`common.Cursor` + `Platform.NewCursor` + win32 实现(含 `WM_SETCURSOR` 处理、窗口类光标改 NULL)。其余后端 `NewCursor` 返 `ErrUnsupported`。**可独立编译 + win32 真机验证光标切换**(临时在窗口创建后手动 `SetShape(CursorText)` 验证)。
2. **gui 路由**:`WidgetBase.Cursor/SetCursor` + `gui.CursorShape` re-export + window 持有 `platform.Cursor` + hover 变化时解析并 `SetShape` + `RequestCursorUpdate`。内建控件设默认光标(TextInput/Button)。window 对 `platform.Cursor == nil`(平台不支持)整体兜底为 no-op,与 `gui.clipboard`/`gui.settings` 判 nil 兜底同法(见 §3 与 `gui/clipboard.go`)。
3. **ui 暴露**:`View.Cursor`。
4. **x11 / cocoa 后端**:补齐 `xlib` 光标函数 / cocoa `NSCursor`,去掉对应 `ErrUnsupported`。

### 9.1 gui 层兜底(对齐 Clipboard/Settings)

平台不支持 IME 时 `NewInputMethod` 返 error、gui 拿到 nil 就降级;光标同理。gui.window 持有的 `platform.Cursor` 可能为 nil(平台无此能力或 `NewCursor` 失败),所有经 window 的光标调用都先判 nil——nil 即 no-op(光标恒为系统/窗口类默认),应用与控件代码无需感知。这与 `gui/clipboard.go`(nil 时 `SetText` 静默、`RequestText` 回调 `"",false`)、`gui/settings.go`(nil 时 getter 返默认值)是同一套"可选平台能力 + gui 兜底"惯例。
