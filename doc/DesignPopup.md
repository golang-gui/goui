# GOUI Popup 设计

> 当前定位：原生 Popup/GUI Popover 保留为可超出 owner 客户区的独立表面。
> Menu 与 DropDown 保留原生 Popover；普通 Overlay 容器和 UI 绑定已实现，不是原生 Popup 的隐式降级。
> GUI 层已经有 Popover 与 PopoverMenu；早期代码示意以当前接口为准。
> 当前透明表面见 [DesignTransparency](DesignTransparency.md)，未实现／待评估事项见 [Roadmap](Roadmap.md)。
> 相关：`doc/DesignConventions.md`（跨层约定）、`doc/HiDPI.md`（坐标/缩放）、`doc/PlatformDev.md`。

---

## 1. Popup 是什么

统一的**瞬态浮层**：菜单、上下文菜单、下拉 / combobox 列表、tooltip。共性：无边框、依附某个窗口、按锚点定位、置于其它窗口之上、瞬态关闭、有自己的内容(widget 树 → 自己的绘制)。v1 优先**菜单 + 下拉框**，Windows 式**点击菜单**（点开、点项选中），不做 macOS 式拖选。

## 2. 分层职责

- **platform**：提供**能力/事实**——无边框、可**相对 owner 窗口定位定尺**、置顶不进任务栏的原生浮层 + show/hide + 普通输入/paint 事件。**不决定何时关闭、不做指针 grab。**
- **gui**：锚点定位、样式外壳、工作区退避、模态输入与关闭策略；原生资源由 Popover 管理，不另建全局 popup-manager。
- **ui**：菜单／DropDown 的薄声明式绑定；通用附着浮层尚未实现。

## 3. 关键决策(已定)

- **依附 owner Window**：`NewPopup(owner, handler)`。goui **没有全局/屏幕坐标概念,只有窗口内坐标**——Popup 的位置是**相对 owner 窗口内容原点**的窗口内逻辑坐标,平台内部用 owner 的屏幕位置换算成原生屏幕坐标摆放。gui 永远只碰窗口内坐标。
- **独立 `Popup` 抽象**(不复用 `Window` 接口)：popup 差异够大(无边框、相对定位、show/hide),单独抽象更清晰。
- **无 grab,gui 驱动关闭**：因为 popup 是独立原生窗口,"哪个窗口收到点击"本身就区分 inside/outside——
  - 点 popup 自身 → popup 窗口收到 → inside。
  - 点我方其它窗口 → 那个窗口收到 → outside → gui 关闭。
  - 点别的应用 / 桌面 → 失焦 `FocusEvent{false}` → gui 关闭。
  - 再加 Esc / 选择 / 程序化。
  平台只送普通事件;**外部点击是否穿透**由 gui 用 `EventController.StopPropagation` 决定。grab（拖选、指针封闭、吞掉 dismiss-click）作为后续增强,v1 不做。

## 4. 平台层抽象

```go
// platform
NewPopup(owner Window, size geometry.Size, handler EventHandler, options PopupOptions) (Popup, error)
NewPainter(surface Surface) (Painter, error) // Surface = Window | Popup

// Surface：绘制目标的公共契约。Window 与 Popup 都内嵌它,故都能建 Painter。
type Surface interface {
    Transparent() bool           // 不可变逐像素 alpha 创建配置
    RequestPaint() error
    NativeHandle() uintptr        // 绑 GPU 上下文(OpenGL / Direct2D)
    Draw(img image.Image) error   // 软件 painter 的上屏兜底路径
}

// 独立抽象;无边框、置顶、依附 owner
type Popup interface {
    Surface                         // NativeHandle + Draw:popup 是绘制目标
    Destroy()
    SetPosition(x, y float32)      // 相对 owner 窗口内容原点(窗口内逻辑坐标)
    SetSize(width, height float32)  // 逻辑尺寸
    Show() error
    Hide() error                    // 菜单/下拉反复开关,show/hide 复用
    RequestPaint() error
    // 事件经 NewPopup 的 handler：pointer / key / focus / paint / size(同 Window 事件模型)
}
```

`SetPosition` 与 `SetSize` **分开**(存在分开设置的需求)。

**Popup 可建 Painter**:`NewPainter` 形参是 `Surface`(不是 `Window`),`Window` 与 `Popup` 都内嵌 `Surface` 即都能建 painter——三个 painter 后端要的只是 `NativeHandle`(opengl/direct2d 绑上下文)或 `Draw`(software 兜底),两者皆有。**x11 特例**:GL 上下文还需窗口创建时的 `glx.FBConfig`,`opengl` 在 x11 内部用 `NativeFBConfig()` duck-type 拿(硬断言),故 `*x11.Popup` 也转发 `NativeFBConfig()`(与 `*x11.Window` 对齐);该方法是 x11 私有类型,不进跨平台 `Surface`。win32/cocoa 的 FBConfig/PixelFormat 另路获取,无此要求。

## 5. 三后端实现要点

| 后端 | 无边框 + 置顶 | 定位(owner 内 (x,y) → 屏幕) |
|---|---|---|
| **x11** | override-redirect 窗口(绕过 WM,天然置顶 + 不进任务栏) | `TranslateCoordinates(owner→root)` 得 owner 屏幕原点 + (x,y)×scale;`MoveWindow` |
| **win32** | `WS_POPUP` + `WS_EX_TOOLWINDOW｜WS_EX_NOACTIVATE`(无任务栏、不抢激活),owner hwnd | `ClientToScreen(owner)` + (x,y)×scale;`SetWindowPos(NOACTIVATE)` |
| **cocoa** | borderless `NSWindow` + `SetLevel(NSPopUpMenuWindowLevel)` 置顶 | `ContentRectForFrameRect(owner.Frame())` 得内容屏幕矩形;**y 轴翻转**(goui 左上/y下 → macOS 左下/y上);`SetFrameTopLeftPoint` |

- **坐标单位**：x11/win32 的 (x,y) 是逻辑值,乘 scale 得物理像素;**cocoa 的 point 本身即逻辑单位**(retina 缩放由 backing scale 处理),故不乘 scale。
- **HiDPI**：popup 报 `SizeEvent`(逻辑 + 物理),scale 随 owner,painter 走物理分辨率(同 `HiDPI.md`),paint 路径与 Window 一致。
- **复用 Window**：三后端把窗口创建收敛到一个内部构造器 `newNativeWindow(...)`（x11 带 `overrideRedirect` 开关;cocoa 带 `styleMask`/`rect` 参数),`newWindow` 与 `newPopup` 都走它,消除重复样板——`newWindow` 之后补顶层特有设置(x11 `SetWMProtocols` / cocoa collection-behavior),`newPopup` 之后补 popup 特有设置(cocoa `SetLevel` 置顶)。win32 差异小,`CreateWindowEx` 直接内联。
- **Popup 内嵌 `*Window`**：复用其事件路由(进窗口分发表)、`Draw`、`Destroy`、`RequestPaint`,以及 `Hide`(为此把 `Hide()` 补进 `common.Window` 接口,x11=`Unmap` / win32=`SW_HIDE` / cocoa=`OrderOut`,与 `Show` 对称)。**唯独 `Show` 不委托**:popup 要非激活显示(win32 `SW_SHOWNOACTIVATE` / cocoa `OrderFront`),不抢 owner 焦点,而 `Window.Show` 会激活。

## 6. 平台层范围

无边框 + 相对 owner 定位定尺 + 置顶不进任务栏 + 普通事件 + show/hide。**平台层不做输入 grab、不夺焦、不管 dismiss**(见 §7)。不做 拖选 / 多屏精确定位。平台 popup 接口最终定型为 `Surface + Destroy + SetPosition + SetSize + Show + Hide + RequestPaint`。

**验证**（`platform/test`）：`TestPopup`(建 → 定尺定位 → Show → PaintEvent → Draw → Destroy)、`TestPopupPainter`(为 popup 建 painter 跑一帧)。x11 本机跑通;win32/cocoa 交叉编译通过,真机由使用者验证(同 Clipboard 流程)。

---

## 7. 事件模型:owner 转发(无 grab、无焦点)

> 本节是多轮讨论的**最终结论**,回到 §3 的"无 grab"方向。中途试过 grab(x11 `XGrabPointer` 全屏重定向太危险、win32 `SetCapture` 脆、cocoa 无原生 grab)和 `SetFocus`(转移焦点 → owner 标题栏变暗),都被否。

**Popup = 浮动、无边框、无焦点的原生表面**,仅用于**菜单 / 分离工具栏 / tooltip**。能拿焦点、永远压在 owner 前的是 **transient window**(Window 的变体),不是 popup,另设计。

**关键:popup 永不夺焦,owner 始终持焦(所以不变暗)。** 菜单要的键盘和"点外部关闭",靠 **owner 转发**实现:

| 事件 | 谁先收到 | 处理 |
|---|---|---|
| 按键 | owner(持焦) | 转发给活跃模态 popup 的内容树 |
| popup 上的鼠标(move/hover/click) | popup 自己(最上层独立窗口) | popup 用**自己表面的事件**处理(自身坐标),owner 不介入 |
| owner 自身 `PointerDown`(必在 popup 之外) | owner | 直接请求 dismiss(**无坐标换算、不转发鼠标**) |
| owner 失焦 `FocusEvent{false}` | owner | 请求 dismiss(焦点去了别处) |

因为 popup 遮住自己那块,owner 收到的点击**必在 popup 之外**——所以 owner **无需转发鼠标、无需坐标换算**,只在自己的 `PointerDown`/`Esc`/`FocusEvent{false}` 上请求 dismiss;popup 的 hover/点击全由它**自己的表面事件**驱动。(这也是"hover 不响应"那个 bug 的教训:根因是 gui 层重绘走了 `Window()`=nil,而非 popup 没收到事件。)

**平台层为此不加任何东西**:Window 和 Popup 本就各发各自的键/鼠/焦点事件。转发 + dismiss 全是 **gui 策略**(见 §8)。

> **各平台须保证"点击 popup 不夺 owner 焦点"**(否则 owner 收到 `WM_KILLFOCUS`/`FocusOut` → gui 误判失焦 → 误关):
> - **x11**:override-redirect 窗口不被 WM 管理,点击天然不转移输入焦点。✓
> - **win32**:`WS_EX_NOACTIVATE` 只挡"激活/前台",**不挡同线程点击时的 `SetFocus`**;需在 popup 的 windowProc 处理 **`WM_MOUSEACTIVATE` 返回 `MA_NOACTIVATE`**(不激活也不夺焦,点击照常投递)。
> - **cocoa**:popup 不设为 key window 即可(`OrderFront` 而非 `MakeKey`)。

## 8. GUI 层:`gui.Popover`(精简 v1)

平台叫 `Popup`(原生面),gui 叫 **`Popover`**(沿用 GTK 名),区分"原生面 vs 控件层抽象"。

**接口**(用 `core/geometry`):
```go
type PopoverOptions struct { Transparent bool }
func NewPopover(anchor Widget, options *PopoverOptions) Popover
type Popover interface {
    Root // Widget(), RequestPaint(), RequestLayout(), ConnectFrame(); independent host
    StyleName() string; SetStyleName(string)
    Placement() PopoverPlacement; SetPlacement(PopoverPlacement)
    DispatchEvent(events.Event) error
    CommitText(string) error
    Transparent() bool // immutable creation option, independent of owner
    Visible() bool
    SetWidget(Widget)
    Anchor() Widget
    Position() geometry.Point; SetPosition(geometry.Point)   // 相对 anchor 原点
    Modal() bool; SetModal(bool)                             // 菜单式=true;默认 false=非模态(相对 owner 窗口)
    Show() error; Hide(); Destroy()
    ConnectDismissRequest(func()) signal.Handle              // 对齐 ConnectCloseRequest
    ConnectClosed(func()) signal.Handle
}
```
- **通用浮动窗口**:Popover 本身**无 dismiss 策略**(控件决定 show/hide/destroy)。是 Menu 还是 Tooltip 由 **`Modal` 开关**区分(v1 由控件 `SetModal` 设;默认 `false`=非模态)。`Modal` 是**相对 owner 窗口**的模态(接管其输入),不阻塞别的窗口。
- **内容驱动尺寸**：无 `SetSize`，`Show`/`SetWidget` 时
  `Widget.Measure(layout.Loose(available)).Size` 的返回即尺寸。
- **widget 宿主**:`popover` 有自己的 root + `EventDispatcher` + painter,与 `gui.window` 同构。
- **各自持有原生面**:每个 popover 惰性建**自己的** `platform.Popup`(v1 不池化);三个下拉框 = 三个面(仅开过的才建),成本正比"开过的数量"。

- **透明与背景分离**：nil options 默认不透明；透明创建失败不退回不透明。
  `"popover"` 样式控制主体背景，默认白色；透明选项不自动移除背景。
  清空、样式更新与资源重建契约见 [透明绘制 GUI 接入](DesignTransparency.md#gui-接入)。

**事件模型(§7 转发,per-window)—— `ModalTarget` 公开接口**:

window 是**被动的输入路由器**,只持有一个抽象模态目标,**不知道"popover"这个类型**:
```go
type ModalTarget interface {
    DispatchEvent(events.Event) error  // 转发的键盘 → 目标内容
    RequestDismiss()                   // 请求关闭
}
// Window 接口新增:
SetModalTarget(ModalTarget)            // 装 / 卸(nil)当前模态目标
```
- **window 拥有 dismiss 策略**(反正 Esc/外部点击/失焦本来就是 window 收到的):自身输入在有模态目标时——`Esc`→`RequestDismiss()`;其他键→`DispatchEvent()`;`PointerDown`→`RequestDismiss()`(owner 只会收到 popup 外的点击);`FocusEvent{false}`→`RequestDismiss()`(且不消费,window 照常处理自身焦点)。
- **popover 实现 `ModalTarget`**(`DispatchEvent`→自己的 dispatcher;`RequestDismiss`→emit `dismissRequest`);`Show` 时若 `Modal` 就经 **`anchor.Window().SetModalTarget(p)`**(纯公开 `Window` API)登记,`Hide`/`SetModal(false)`/释放时 `SetModalTarget(nil)`。
- **只有 `Modal==true` 的 popover 登记为模态目标**;非模态 tooltip 不登记、不影响 window 自身事件(hover 照常),任意多可并存。
- v1 **单槽**:同一 window 同时只一个模态目标,**无自动 supersede**(控件保证)。
- popover 的 hover/点击由**它自己的表面事件**(经它自己的 `EventDispatcher`)驱动,window 只转发键盘 + 触发 dismiss,**不换算坐标**。
- **`EventDispatcher` 独立化**:吃 `EventTarget{Widget();FocusedWidget();SetFocusedWidget()}`,window 和 popover 都实现,一份分发两处用。

**分层解耦原则(本轮重构确立)**:
- 公开协议按实际替换需求设计；Widget／IMContext 等有明确约束的封印接口不被一概否定。共享 rootBase 是宿主的内部状态 helper，不是另一套公开控件体系。
- 控件只依赖 **`Root`**(`Widget()`/`RequestPaint()`/`RequestLayout()`),绝不碰具体 `*window`/`*popover`;焦点相关经 `EventTarget`。
- **依赖方向**:`Window→Popover` 经 `ModalTarget`(输入,宿主→内容的自然向);`Popover→Window` 经公开 `Window`(`SetModalTarget` 登记 + `PlatformWindow()` 逃生口建面 + `ConnectDestroy` 生命周期)。归属选**模型 A**(popover 自持原生面,经 anchor 登记),window 只做输入路由,不当浮窗管理者。
- `Window` 增逃生口 **`PlatformWindow() platform.Window`**(类比 `NativeHandle`);platform/typography 从全局 `App` 取。
- 宿主的 root 被销毁后经 **`liveRoot`**(查 widget 的 `destroyed` 终态)在 `Widget()`/`paint()` 惰性丢弃——不需要宿主回调接口。

**生命周期(两条,信号桥接)**:
- **句柄**(`Popover` 对象):控件所有,`NewPopover`..`Destroy`(可选);跨 unmount 复用。
- **原生资源**(`platform.Popup`+painter):**归 Window**,首次 `Show` 建(绑当前 owner)、`Hide` 保留复用、**anchor 卸载**(`anchor.ConnectUnmount`)/ **owner 窗口销毁**(`window.ConnectDestroy`)/ `Destroy` 时释放,**永不悬空**(x11 override-redirect 的 X 父是 root,不随 owner 级联销毁,必须显式收)。`Show` 若发现 anchor 换了窗口则释放旧的重建。
- **Widget 不加 Destroy 信号**(谁调用语义模糊);popover 靠 unmount 自动收原生面,句柄由控件按需 `Destroy` 或随 GC。

Menu／MenuItem 和声明绑定、Point／Bottom 定位及工作区退避已实现。子菜单与原生 Popover IME 的缺口见 [未来路线](Roadmap.md)；不因此承诺模态栈、自动 Modal 推导、Popup 池化或新位置 getter。

> Window／Popover 已共享内部 rootBase，统一帧、分发、手势和拖放状态，但保留各自原生宿主与模态策略。普通鼠标仍由各自表面处理，不全量转换成 owner 事件。

## 9. Overlay 与原生 Popover 的当前边界

[Overlay](DesignOverlay.md) 已实现为普通单主内容、多覆盖内容容器，UI 薄适配已完成。
窗口无需知道覆盖层细节；布局、裁剪、穿透和遮罩事件复用现有 Widget 机制。
Menu 与 DropDown 仍使用原生 Popover，不迁移到 Overlay，也不动态切换承载方式。

## 10. UI 接入与未来附着浮层

MenuButton／MenuItem、DropDown、Overlay 和 OnContextMenu 已有声明式入口。
通用 Tooltip／Popover 附着声明尚未实现，须重新设计，见
[未来路线](Roadmap.md#future-ui-popover)；旧 UpdateOverlays、OverlaySpec.Key、
WidgetBase.SetTooltip／AttachPopover 等配方不作为待添加 API。

## 11. 语义与宿主枚举

Popover 是独立 Root，不伪装成 anchor 的子控件。Window.Popovers 枚举当前可见
浮层，dev 自己为宿主及 Widget 分配 ref；Snapshot.Attributes 支持具名值扩展，
不靠为每种浮层修改 gui.Info。输入仍走 DispatchEvent／CommitText，没有动作旁路。
详见 [自动化](DesignDevAutomation.md) 与 [Widget Snapshot](DesignWidgetSnapshot.md)。

## 12. 已实现：透明表面上的圆角外壳

Popup 可显式请求透明 surface。GUI 先清空为透明，再绘制样式圆角背景、边框与阴影，
角区保持 alpha；不需要 platform 提供菜单半径。无透明能力时 PopoverMenu 按明确策略降级。

目标是让 Popup 的**原生可见区域**形成真实圆角，而不只是让内容 Widget 使用
`FillRoundRect`。

### 12.1 分层原则

- platform 负责透明呈现与合成器事实；指定输入区域已放弃，不恢复视觉 Shape 草案。
- gui 根据 Popover/Menu 样式决定圆角半径，调用通用 Painter 绘制；
  `ui` 只声明样式或内容，不接触原生窗口能力。
- 圆角不能改变既有的 owner、无焦点、事件分发、Show/Hide 和生命周期语义。
- 平台不支持完整透明合成时必须采用明确 fallback，不能静默留下未定义边角。

### 12.2 平台验证点

- Windows：比较透明合成与 window region 方案，验证 Direct2D 表面、DPI 和点击区域。
- macOS：验证 borderless NSWindow 的透明背景、content view/OpenGL surface 和圆角裁剪。
- Linux/X11：验证 ARGB visual/compositor 与 Shape region 的可用性，并覆盖无 compositor/llvmpipe 场景。

三平台共享逻辑半径与 HiDPI 语义，具体呈现机制见 [透明绘制](DesignTransparency.md)。
透明像素不会自动令鼠标穿透；本契约不保证原生输入区域跟随每个 alpha 像素。
Popup resize／重复显示／阴影占位／清空及降级验证见 [菜单外观](DesignPopoverMenuAppearance.md)。
