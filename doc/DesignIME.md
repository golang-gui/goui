# GOUI 输入法(IME)设计

> 快捷键接入补充：IME 优先消费原生输入；未消费的键进入 GUI 传播。GUI 快捷键或编辑
> 命令通过 KeyEvent.PreventDefault 取消剩余原生默认动作，停止 GUI 传播本身不取消文字
> 提交。Windows 主循环在 TranslateMessage 前派发普通按键；macOS input context
> 拒绝且未产生命令/文字回调时只补发一次 KeyEvent。模态目标打开时暂停 owner IME，
> 防止背景编辑器接收迟到 commit。详见 [快捷键消费边界](DesignShortcuts.md#原生默认动作与文字输入)。

> 状态（2026-10-14 复核）：三平台普通窗口原生输入与 TextInput／TextView 已实现。Popover 原生 IME 尚未实现，未来规划见 §5.1／Roadmap；不同平台已执行与待验收范围见文本编辑设计。
> 定位:记"文本控件如何接输入法、平台与 gui 如何配合、命名与机制为何这样定"的设计。总哲学见 `doc/Design.md`;事件/焦点见 `doc/GuiDesign.md`;跨层约定见 `doc/DesignConventions.md`。参考 GTK/Flutter/Qt/AppKit,**吸取经验而非照搬**。

---

## 0. 现状与目标

当前文本输入由平台原生组合／提交路径提供，旧 keyEventText 的 ASCII 映射已删除。
X11 使用 XIM/XIC，Windows 使用 WM_CHAR／IMM32，macOS 使用 NSTextInputClient；
共享编辑核心接收 IMCommit，不从逻辑 KeyCode 猜字符。

目标:给**文本类控件**一个极小、好用的输入法接口,把各平台富且异步的文本输入协议全部吸收在 `platform` 层,**不往通用 `Widget`/`WidgetBase` 增加任何公开方法**(输入法只有文本控件才需要)。

## 1. 从别人身上学什么、避什么

| 框架 | 借鉴 | 规避 |
|---|---|---|
| **GTK** | `GtkIMContext` 对象模型:控件持有 context、连 commit/preedit、报 caret 位置——最贴 goui | `filter_keypress` 同步喂键 + 手动 `focus_in/out` 是控件负担,且同步返回值在 win32 IMM(异步)上兑现不了 |
| **Flutter** | 框架集中管 IME 生命周期,控件只声明自己是 `TextInputClient` | `TextInputConnection` 那层连接对象偏重 |
| **Qt** | commit + preedit 一起给,caret 用查询 | `inputMethodQuery` 大杂烩查询接口太杂 |
| **AppKit** | `NSTextInputClient` 能力强 | 富协议方法太多,绝不摊给每个控件作者 |

**一句话教训:平台的富/异步文本输入协议全部吸收在 `platform` 层,只给控件作者留一个极小的 commit/preedit/caret 面。** 这决定了下面的分层。

## 2. 命名

| 名字 | 层 | 含义 |
|---|---|---|
| `gui.IMContext` | gui | 控件持有的输入法上下文(GTK 血统)。**封印接口**(带未导出方法),实现是未导出的 `gui.imContext`;由 `NewIMContext()` 获取。见 §2.1 |
| `gui.IMClient` | gui | `interface{ IMContext() IMContext }`,文本控件实现它,window 靠它在聚焦时找到上下文 |
| `IMContext.SetCaretRect(geometry.Rectangle)` | gui | 报光标矩形(控件**本地坐标**)。用 `caret` 不用 GTK 的 `cursor`——goui 里 caret=文本插入点、cursor=鼠标 |
| `platform.InputMethod` | platform | 每窗口的输入法能力(`SetEnabled`/`SetCaretRect`/`Reset`/`Destroy`)。经 `Platform.NewInputMethod(window, handler) (InputMethod, error)` 获取,和 `NewPainter`/`NewClipboard`/`NewSettings` 一个待遇;**不支持则返 error**(不是所有窗口/平台都有 IME)。见 §2.2 |
| `platform.InputMethodHandler` | platform | 输入法**输出**回调 `func(InputMethodResult)`——与 `EventHandler func(events.Event)` 同为 func 惯例。`InputMethodResult{Kind, Text, Caret, Composed}`(Kind∈Commit/Preedit)承载输出;**不走 events 总线、也不叫 `...Event`**(否则会让人误以为能从 `Window.onEvent` 拿到) |

**为什么砍 `FilterKey`**:GTK 的同步 `filter_keypress` 在 x11(`Xutf8LookupString` 同步)、cocoa(`interpretKeyEvents:` 同步)上成立,但 **win32 IMM 是异步的**——`WM_IME_COMPOSITION` 在 `WM_KEYDOWN` 之后才到,控件在 keyDown 里同步问"IME 吃了这键吗"答不出来。三后端有一个撑不住,统一的同步 `FilterKey(e) bool` 就是假的。所以**不给控件喂键**,改由平台后端自判(见 §4)。

**为什么砍 `FocusIn/FocusOut`**:goui 焦点集中在 window,原生 IME 又是每窗口一个,所以由 window 按焦点自动接线(§5),控件不调任何 focus 方法。比 GTK 手动 focus_in/out 更省,也不会漏掉 blur 导致 preedit 卡死。

### 2.1 `IMContext` 是封印接口

`IMContext` 是**接口**,实现是未导出的 `imContext`,只由 `NewIMContext()` 产出。它和 `gui.Settings`/`gui.Clipboard` 那种"纯外部访问、无替换点"的公开接口不同:`IMContext` 由 `IMClient.IMContext()` **交回给 window**,有被替换的机会。所以接口带一组未导出方法(`emitCommit`/`emitPreedit`/`setWindow`)把它**封印**——和本仓库 `events.Event.isEvent()`、`ui.RootView.rootView()` 同一手法。由此得到三条边界:

| 场景 | 结果 |
|---|---|
| 凭空从零实现一个 `IMContext` | ❌ 编译期挡掉(写不出跨包的未导出 `emitCommit`)→ 强制走 `NewIMContext` |
| 嵌入真货装饰 `ConnectCommit`/`SetCaretRect` 等监听信号 | ✅ 允许(Go 嵌入接口会提升全部方法,含未导出的,故 wrapper 仍满足接口) |
| 篡改内部 `emitCommit`/`setWindow` 管线 | ❌ 挡掉(跨包同名未导出方法不覆盖提升来的 `gui.emitCommit`,框架管线不可劫持) |

即:用户能装饰**公开面**、动不了**框架内部管线**。公开面 4 个(`ConnectCommit`/`ConnectPreedit`/`SetCaretRect`/`Reset`);封印方法包含 `emitCommit`、`emitPreedit`、`hasPreedit`、`setWindow(*window)`（nil 解绑）。`SetCaretRect` 是纯转发(active 时给平台),不缓存——聚焦后的 caret 由控件下一帧 paint 推给平台,window 无需读回。

### 2.2 平台层:IME 是可选能力,不是 Window 方法、不是总线事件

goui 平台层**每个能力都是 `Platform.NewXxx() (Xxx, error)` 工厂**(`NewPainter`/`NewClipboard`/`NewSettings`/`NewTypography`/`NewWindow`/…),没有一个把方法糊在 Window 上。IME 归位成同一形状:

```
Platform.NewInputMethod(window Window, handler InputMethodHandler) (InputMethod, error)
```

- **可选性靠 error**:不是所有窗口/平台都支持 IME(离屏、极简后端、未来 web/android)。`NewClipboard`/`NewSettings` 本就用 `error` 表达"这平台没这能力",IME 返 `ErrUnsupported` 同理——比"强迫每个 Window 实现三个 no-op 方法、还没法表达不支持"干净。gui 拿不到就自然降级。
- **输出走 handler(一个 func)不走总线**:`commit`/`preedit` 不再是 `events` 类型,而是经 `handler func(InputMethodResult)` 上行(handler 是 func,跟 `NewWindow`/`NewPopup` 收 `EventHandler` 一致)。`InputMethodResult{Kind, Text, Caret}` 用 `Kind` 区分 commit/preedit——**特意不叫 `...Event`**,免得让人误以为能从 `Window.onEvent` 拿到 IME 输出,把总线边界搞糊。这样 events 包不耦合 IME,gui.window 也不用"拦一种永不派发给 widget 的事件";**平台层 IME = 一个可选内聚能力**,和 **gui 层 IMContext = 一个内聚抽象**上下对称。
- **和 Painter 的差异(为何窗口内部仍持有它)**:Painter 纯输出、不碰事件;IME 织进窗口的**输入路径**(x11 `XFilterEvent`/`Xutf8LookupString`、win32 `WM_IME_*`、cocoa `interpretKeyEvents:` 都在按键处理里)。所以后端 window **内部**必然持有并咨询自己的 InputMethod——但这是后端内部耦合,**公开接口照样干净**(`NewInputMethod` 里把 IM 塞回具体 window)。

## 3. 三层职责

```
platform.InputMethod  每窗口的原生输入上下文(XIC/IMM/NSTextInputContext),吸收全部平台复杂度
  · 聚焦文本控件时把每个按键先喂原生 IME(后端 window 内部持有并咨询它)
  · 被消费 → handler.Commit / handler.Preedit 上行
  · 未消费(方向/退格/回车/快捷键)→ 发正常 KeyEvent
  · SetEnabled / SetCaretRect / Reset / Destroy;经 Platform.NewInputMethod 获取,不支持返 error
        ↑ commit/preedit 经 InputMethodHandler 回调上来
gui.Window        焦点驱动的「路由器」(持有 platform.InputMethod,nil=不支持)
  · 焦点变:新控件是 IMClient 且非 nil → 绑为 activeIM、im.SetEnabled(true)、推 caret
             旧控件 → im.Reset、im.SetEnabled(false)
  · 实现 InputMethodHandler:Commit/Preedit 不走控件树,直接喂 activeIM
  · activeIM.SetCaretRect(本地) → 加聚焦控件窗口偏移 → im.SetCaretRect
        ↑ 控件持有、连回调
gui.IMContext     极薄的每控件门面:ConnectCommit / ConnectPreedit / SetCaretRect / Reset
```

`IMContext` 很薄(每控件的回调 + caret + reset);重活在 `platform.InputMethod`;`gui.Window` 是焦点路由器兼 `InputMethodHandler`。

## 4. 键盘流:平台自判"透传 vs 消费"

没有 `FilterKey`,方向键/Ctrl+A 这类怎么走?**由原生 IME 自己判定,后端按结果分流**——这比 goui 自写启发式更对,因为拼音/五笔/日文/韩文各自消费的键都不同,只有原生 IME 知道自己的键位。

x11 上具体两步(接在现有处理 `KeyPress` 的地方):

```
XFilterEvent(event) → True  : IME 消费了(组合中、选候选、编辑 preedit)→ 不发 KeyEvent
                    → False : Xutf8LookupString(xic) → 出字符 → InputMethodHandler(Commit)
                                                      → 出 keysym → KeyEvent(key+修饰)
```

- **Ctrl+A**:非组合 → 不消费 → `KeyEvent(KeyA, Ctrl)` → 控件现有 KeyEventController 处理。
- **方向键**:非组合 → KeyEvent → 控件移光标;组合中 → 被消费(选候选)→ 不下发。
- **字母/汉字**:IME 启用时所有键先给原生 IME;拉丁直接输入立即 commit,CJK 走组合。**控件只收到 IME 放行的键**,不会误动组合键,也不需要 "composing" 标志防御。

win32(IMM 组合消息 vs 普通 `WM_KEYDOWN`)、cocoa(`interpretKeyEvents:` 消费 vs `doCommandBySelector:` 透传)是同一逻辑的不同外壳。

**IME 关闭时**(聚焦非文本控件如 Button):不走 IME,键照旧发 `KeyEvent`,Space/Enter 正常激活按钮。

## 5. 焦点转移(窗口内控件间)

一个窗口**一个原生输入上下文**,常驻;focus 转移只是**改路由 + 复位组合**,不重建。`gui.Window` 持 `activeIM`,在焦点钩子(`setFocusedWidget`)里:

1. **旧控件**：调用原生 InputMethod.Reset 收尾，再解绑旧 IMContext；原生尾随输出不能误投新控件。
2. **新控件**：实现 IMClient 且返回非 nil 时绑定 activeIM，并请求启用输入法；存在 modal target 时仍保持原生 IME 禁用。caret 由随后 Paint 推送，不从不存在的缓存 getter 读回。非文本控件清空路由并禁用。

**次序**:先收尾旧、再改绑新,否则收尾 commit 会落错控件。控件作者对这套**零参与**。同控件=no-op;焦点清空/转非文本=收尾旧 + 关 IME;整窗口失焦=同样在窗口级"收尾+关闭"。

### 5.1 跨宿主：原生 Popover IME（未实现）

当前模态 Popover 经 owner 转发按键，但打开时暂停 owner 原生 IME，避免背景
编辑器接收迟到 commit。Popover 的逻辑焦点和 CommitText 已实现，后者提交
已确认文本，不等于原生 preedit、候选窗和 caret 定位支持。

原生 IME 委托是后续规划，集中记录于 [未来路线](Roadmap.md#future-popover-ime)。
旧草案按 p.position 加偏移、估计只需几十行修改的实现配方不再作为开发依据：
当前 Position 是 anchor-local 的请求值，实际工作区退避、主体／阴影偏移和嵌套锚点
必须使用实际表面几何；还须复核 owner/modal 路由、迟到提交与销毁重入。
不为这项缺口恢复 GUI 模拟字符或给所有 Widget 增加 IME 方法。

## 6. 第三方控件开发者的使用面

**`WidgetBase` 零污染。** 文本控件通过"自己 New 一个 IMContext + 实现 `IMClient`"opt-in,普通控件作者无感。接输入法就这几步:

```go
type TextInput struct {
    gui.WidgetBase
    im      gui.IMContext
    text    string
    preedit string
}

func NewTextInput() *TextInput {
    t := &TextInput{im: gui.NewIMContext()}
    t.im.ConnectCommit(t.onCommit)     // IMCommit 携带文字及原生提交来源
    t.im.ConnectPreedit(t.onPreedit)   // 内联组合串,不进 Text()
    // key controller 只处理"残余键":方向/退格/回车……
    ...
    return t
}

// 框架侧唯一约定:实现 IMClient,让 window 在聚焦时找到上下文。仅文本控件实现。
func (t *TextInput) IMContext() gui.IMContext { return t.im }

func (t *TextInput) onCommit(c gui.IMCommit)       { t.insertAtCaret(c.Text); t.clearPreedit() }
func (t *TextInput) onPreedit(s string, caret int) { t.setPreedit(s, caret); t.im.SetCaretRect(t.caretRect()) }
```

面就这么大:`NewIMContext` / `ConnectCommit` / `ConnectPreedit` / `SetCaretRect` / `Reset` + 实现 `IMClient`。**没有 FilterKey、没有 FocusIn/Out。**

`ConnectCommit(func(IMCommit))` 的值参数包含 `Text string` 和 `Composed bool`。
`Composed` 仅表示原生 API 已确认这是组合/文字服务的提交，即使此前没有 preedit；
false 表示普通字符或来源不可辨别，不承诺排除组合。Windows 的 `GCS_RESULTSTR`
和 `WM_IME_CHAR`、XIM 的 `XLookupChars` 或服务端 `keycode=0` 的提交、macOS
已有 marked text 时的 `insertText` 会携带该事实。普通文字不能根据 ASCII/CJK
内容分类。信号使用值参数，不借用可被重入覆盖的上下文状态，也不另设提交信号。
IMContext 不管理撤销；TextInput/TextView 将组合提交作为独立撤销步骤，并继续
识别 preedit-end 后才收到 commit 的原生顺序。上述最小示例未展示历史管理。

## 7. caret 单位与坐标

- **单位:字节。** 与排版层同一索引空间(x11 走 Pango,用 UTF-8 字节索引)。已提交文本 caret 与 preedit 内 caret 都是字节偏移;渲染 `display = text[:caret] + preedit + text[caret:]`,可见光标落 `caret + preeditCaret` 字节处,提交/组合/命中测试共用一套索引。
- **坐标:控件本地。** 控件把本地 caret rect 交 `IMContext.SetCaretRect`;window 转发时加聚焦控件的窗口偏移(window 已知 focusedWidget 及其 `windowRect()`),得窗口坐标再给 `platform.InputMethod.SetCaretRect`。控件不碰坐标换算。

## 8. 平台后端与落地路径

x11 先行(dev 平台);其余后端同契约后补。

1. **platform 契约**:`common.InputMethod`(`SetEnabled`/`SetCaretRect`/`Reset`/`Destroy`)+ `common.InputMethodHandler`(`Commit`/`Preedit`)+ `Platform.NewInputMethod(window, handler) (InputMethod, error)`。**Window 不带 IME 方法;events 无 IME 事件。** 不支持的后端 `NewInputMethod` 返 `ErrUnsupported`。
2. **x11(XIM/XIC)**:`platform` 层 `setlocale(LC_CTYPE,"")` + `XSetLocaleModifiers("")` + `XOpenIM`(每 display 一个);`window` 建 `XCreateIC`;事件循环先 `XFilterEvent`;`KeyPress` 用 `Xutf8LookupString` → Commit/Key 分流;`SetEnabled`→`XSetICFocus`/`XUnsetICFocus`;`Reset`→`Xutf8ResetIC`;`SetCaretRect`→`XNSpotLocation`(经 `XVaCreateNestedList`+`XSetICValues`)。非 `XIMPreeditPosition` 样式的 spot 是 Xlib/输入法扩展，不是 XIM 通用保证；绑定返回属性是否被接受，平台不补偿被拒绝的坐标，具体限制见 §9。
3. **gui**:`IMContext` + `IMClient` + `gui.Window` 路由(焦点绑定、事件转投、caret 换算)。
 4. **gui.TextInput**:接 IMContext、渲染 preedit。~~`keyEventText` 暂留作过渡回退~~——**已全局移除**(三平台 commit 路径齐备后,见 §9):文字只经 `InputMethodCommit` 进入,物理按键不再猜测字符(键盘布局、死键、AltGr、IME 均无法从键码推导)。
 5. **win32(IMM32)——已实现**(交叉编译验证,未在真机 runtime 测):`WM_IME_COMPOSITION` 的 `GCS_RESULTSTR`→Commit、`GCS_COMPSTR`+`GCS_CURSORPOS`→Composition(内联 preedit);`WM_IME_STARTCOMPOSITION` 返回 0 抑制默认组合窗;`ImmSetCompositionWindow`/`ImmSetCandidateWindow`→caret;`ImmAssociateContext`→enable/disable;`ImmNotifyIME(CPS_CANCEL)`→reset。**解耦**:组合键靠 `WM_KEYDOWN==VK_PROCESSKEY` 抑制;直接 ASCII 走 `WM_CHAR`/`WM_IME_CHAR`→Commit(UTF-16 代理对组装、重复计数、控制字符交还按键路径),`WM_IME_COMPOSITION` 已消费的 `GCS_RESULTSTR` 不再交给 `DefWindowProc` 合成第二条字符路径。
6. **cocoa(NSTextInputClient)——已实现**(交叉编译验证,待 Mac runtime 测):靠 `cgo.NewCallback`(goexlib/cgo 新增,支持 struct/float 入参返回)把协议方法写成普通 Go 函数;`class.AddMethod` + `AddProtocol` 注册到内容视图;`keyDown`→`interpretKeyEvents:`,文本→Commit、组合→内联 Preedit、命令键→`doCommandBySelector:` 重发 KeyEvent。候选定位坐标转换待现场调(§9)。

### Linux IME 传输的取舍(为何 XIM 先行、不接 GTK)

- **不接 GTK**:`GtkIMContext` 不是可复用的输入法库,离不开 GTK 的类型系统与事件循环;"用 GTK 实现"实质=嵌入 GTK,与 goui 是独立工具包冲突。
- **XIM 先行**:goui 的 xlib 绑定已是 `goexlib/cgo` dlopen 真 libX11,加 `XOpenIM`/`XCreateIC`/`Xutf8LookupString`/`XFilterEvent` 只是同套绑定再多几个函数,不引新依赖、不破 `CGO_ENABLED=0`。而"兼容多种输入法框架"XIM 天然解决:`XMODIFIERS=@im=fcitx/ibus` + `XSetLocaleModifiers("")`,XIM 自动连用户配的 IME(fcitx5/ibus 都出 XIM 桥),goui 不用知道是哪家。
- **D-Bus(ibus/fcitx 原生协议)= 后续升级,非现在**:保真度更高、无 XIM callback 别扭;代价是 ibus 和 fcitx 协议不同要各写一遍 + 引 `godbus`(纯 Go)+ 仍需 XIM 兜底其他 IME。它藏在**同一个 `platform.InputMethod` 契约**后面,以后换/加是 platform 内部的事,gui/ui 一行不动——这正是这层抽象的意义。

## 程序化已确认文本提交（2026-10-10）

Window / Popover 提供 `CommitText(text string) error`，面向当前有效逻辑焦点的
IMClient，通过现有 IMContext Commit 信号进入编辑行为；不调用编辑器 SetText，
不模拟原生按键，不假造 Composed 来源。UTF-8 校验、无输入目标、禁用/只读、
模态输入归属及已知非空 preedit 的拒绝规则见 [DevAutomation](DesignDevAutomation.md)。
IMContext 仅记录已知 preedit 状态，先更新状态再通知；不替代平台 IME 的组合算法。
此能力不等于下面仍未实现的原生 Popover IME 委托；候选窗与原生 caret 定位仍待实现。

## 9. 未做 / 首版限制(明确记账,勿当已覆盖)

- **Popover 原生 IME 未实现**：普通按键转发和 CommitText 不代替预编辑／候选窗。后续范围及实施前复核项见 [未来路线](Roadmap.md#future-popover-ime)，不沿用旧签名和工作量估算。
- **x11 已接入样式协商和内联预编辑**：查询 XIM 支持列表，优先
  `XIMPreeditCallbacks` + StatusNothing/None，再回退 root-style/none。
  start/done/draw/caret 回调按字符索引维护每个 XIC 的预编辑缓冲，输出时转换成
  UTF-8 字节 caret；不把组合写入共享文本模型。原生回调只复制数据，离开原生
  filter/lookup 调用后才发给 GUI，Reset/Destroy 清理尚未发送的数据。
  Fcitx 4 默认不公布 callback 样式，不能替用户打开输入法配置。root-style 的
  相邻普通输入/组合撤销与重做已通过真实 Fcitx 验证，依赖提交来源事实而非预编辑。
  Ubuntu libX11 1.7.5 拒绝 callback 样式的 `XNSpotLocation`，候选窗落到窗口底部；
  上游 1.8.2 才允许 OnTheSpot 的 spot。相同程序临时加载 1.8.4 后候选跟随光标，
  不据此承诺所有 XIM 服务端均支持，也不替用户升级系统库。具体环境与结果见 TextEditing。
- **win32 直接文本已切到 `WM_CHAR`→Commit、`keyEventText` 已全局删除**(x11 `Xutf8LookupString`、win32 WM_CHAR/IMM32、cocoa `interpretKeyEvents:` 三条 commit 路径齐备):CJK 走 IMM32(内联 preedit + 候选跟随 caret),直接拉丁走 `WM_CHAR`/`WM_IME_CHAR`(UTF-16 代理对组装、重复计数、控制字符交还按键路径)。**待真机验证**:非美式布局、AltGr、死键、代理对与重复提交,以及 `WM_IME_COMPOSITION` 消费 `GCS_RESULTSTR` 后 `DefWindowProc` 不再合成重复字符。
- **cocoa(NSTextInputClient)已实现**,靠 `goexlib/cgo` 新增的 `cgo.NewCallback`(支持 struct/float 入参和返回,ABI 已在 Mac 上单测验证过),不再需要手写 asm trampoline。内容视图注册 NSTextInputClient 协议 + 全部方法(`cgo.NewCallback` 造 IMP + `class.AddMethod`);`keyDown`→`interpretKeyEvents:`,文本→`insertText`→Commit、组合→`setMarkedText`→Preedit(内联),命令键→`doCommandBySelector:` 里把暂存的原始 event 重发成 KeyEvent。**待 Mac 上 runtime 验证**:组合提交、撤销分组与候选窗定位(`SetCaretRect` 的窗口逻辑→屏幕坐标转换,含视图未 flipped 的 Y 翻转)。
- preedit 分段样式、输入用途提示、reconversion／高级 substring 查询尚未实现；作为需求触发的评估方向记录于 [未来路线](Roadmap.md#future-input)。
- x11 callback 样式不暴露 XIM feedback 子句样式；多输入法、布局和候选定位仍需验证。

### XIM 服务生命周期

`XOpenIM` 成功后必须注册 `XNDestroyCallback`；如果原生拒绝注册，关闭新建 XIM，
按输入法不可用报告。通知到来时 Xlib 自己销毁 XIM/XIC，GOUI 只清零句柄、移除
回调注册表中的对象并丢弃待发预编辑，不能再次调用 `XCloseIM`/`XDestroyIC`。
窗口仍拥有原来的 Go InputMethod，随后 Reset/Destroy 安全且幂等。
平台在原生调用返回后向此前启用的上下文发送空 preedit；GUI 保留原选区和已提交
内容。原生回调期间不进入 GUI，以免通知重入触发对正在销毁的原生对象的操作。

这沿用 [Xlib Destroy Callback 契约](https://xorg.freedesktop.org/releases/current/doc/libX11/libX11/libX11.html#Input_Method_Values)
及 [GTK 3.24.43 的 XIM 生命周期处理](https://github.com/GNOME/gtk/blob/3.24.43/modules/input/gtkimcontextxim.c)。
当前不自动重连服务；失联后仍有物理按键/选择/复制能力，但不从按键伪造字符提交。
原生输入法无法打开的启动场景与此一致。自动重新建立上下文需另行验证，不以旧句柄重试。

“没有外部输入法服务”不等于“没有原生输入法”：libX11 的
[`_XimCheckIfLocalProcessing`](https://github.com/mirror/libX11/blob/libX11-1.8.4/modules/im/ximcp/imLcIm.c)
将 `@im=none` / `@im=local` 交给本地输入法实现。隔离桌面的普通键入验证可显式
使用 `XMODIFIERS=@im=none`，仍经 XIC / Xutf8LookupString 提交，不需要 GUI
合成字符。若配置 `@im=fcitx` 但该桌面没有服务，XOpenIM 可能失败；不能把此环境
误记为普通键入通过，也不在生产代码中静默替换用户配置。真实 IME/候选窗验收
仍须使用对应服务。系统 libX11 1.7.5 的对照结果见 TextEditing 验证记录。

需区分尚存的原生限制：libX11 1.8.4 的
[`_XimXRead` / `_CheckCMEvent`](https://github.com/mirror/libX11/blob/libX11-1.8.4/modules/im/ximcp/imTrX.c)
同步等候协议回复时不匹配服务窗口的 DestroyNotify；如果服务恰在 Reset/Destroy
等同步调用内部退出，销毁回调无法让这次等待返回。本地 1.7.5 已观察到 Reset 阻塞。
不能靠销毁通知宣称这个竞态已修复；不在另一 goroutine 操作线程亲和的 XIC、
不使用任意超时后的继续访问、不伪造协议回复。采用其他输入法传输或上游改进属于
后续方案，当前作为已知限制记录。
