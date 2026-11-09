# 呈现宿主与窗口管理

状态：2026-11-08。平台接口迁移、移动原生 UI 主线程调度已验收；系统宿主交付
已完成阶段验收。GUI/UI 激活与退出、GUI Surface 共享内核已接入并完成模拟器
阶段验收；ui.Surface 也已完成后续切片，原生触摸与 IME 仍待后续。

关联：[核心原则](Design.md#cross-platform-scope)、[移动接口复核](PlanMobileAPI.md)、
[移动生命周期](DesignMobileLifecycle.md)、[Widget Painter](DesignWidgetPainter.md)。

## 1. 目标

共享布局、控件、事件、绘制与声明式协调，不要求桌面和移动应用采用相同的顶层
生命周期。应用可以明确选择自由管理的窗口，或受系统管理的呈现宿主。
不按 Android/iOS/桌面名称推断窗口能力，也不以隐藏内容 View 模拟隐藏应用窗口。

第一阶段仅调整接口边界，保留当时的独立 GOUI 线程和 Run 前登记窗口原型。
后续原生线程与宿主交付进展分别见第 8、9 节；内部 Show/Hide 只保留为旧验证路径，
不是系统宿主接口的承诺。
不能把接口迁移或交叉编译通过当作原生 UI 主线程、移动 GUI 或 Popup 已验收。

## 2. 平台接口

```text
Surface                         通用绘制目标；Window 与 Popup 均可绘制
  ├─ Window                     原生呈现宿主、事件、资源释放
  │    └─ WindowManagement      同一个对象提供的可选窗口管理能力
  └─ Popup                      有 owner 的独立原生浮动表面
```

### Surface 与 Window

保留现有 `platform.Surface` 的低层含义：NativeHandle、SurfaceState、Transparent、
Draw、RequestPaint。不把 Widget 树或应用激活引入 Painter 的绘制目标接口。

`Window` 嵌入 Surface，并提供 Destroy。尺寸、焦点、可见性、绘制目标替换与最终
销毁继续通过工厂的 EventHandler 报告。此阶段不新增一组重复缓存这些事件的 getter。
Destroy 表示释放所拥有的宿主资源；与用户请求关闭的 RequestClose 分开，也不等于
结束应用进程。系统宿主的所有权和释放方式由平台适配负责。

### WindowManagement

`WindowManagement` 嵌入 Window，承接以下方法：

- Parent/SetParent、Title/SetTitle、Show/Hide、RequestClose；
- WorkAreaAt、Position/SetPosition、SetMinSize；
- Chrome、ControlsRect、SetControlsPosition；
- State/RequestState、SetHitTest、BeginMove/BeginResize。

它与原来的 DesktopWindow 一样，通过同对象的接口断言获取，**不是** Cursor 那样
另行创建、持有和销毁的服务。不新增工厂或管理对象，也不复制事件流。
暂保留弃用的 DesktopWindow 类型别名供迁移；仓库正式调用点使用新名称。

```go
window, err := backend.NewWindow(size, handler, options)
if err != nil {
    return err
}
defer window.Destroy()
management, ok := window.(platform.WindowManagement)
if !ok {
    return platform.ErrUnsupported
}
return management.Show()
```

完整能力表示该对象可管理，不保证每个操作都可用。既有 Windows 原生命中测试与
Linux/macOS BeginMove/BeginResize 的差异保持不变，单项失败继续返回原有错误。
不通过为系统宿主补齐一组无效方法，让它满足管理接口。

第一阶段保留 `Platform.NewWindow(...)(Window, error)` 签名，避免把类型迁移与
系统宿主交付流程绑在一次实现中。桌面工厂继续返回实现 WindowManagement 的对象；
移动原型返回基础 Window。GUI 的主动管理操作必须检查能力，不能重新扩大基础接口。
系统宿主交付通过第 9 节的可选 WindowSource 完成；GUI NewSurface 尚待接入。
移动原型工厂暂保留给旧验证路径，不能与 WindowSource 模式混用。

### Popup 独立于窗口管理

`Platform.NewPopup(owner Window, ...)` 保留基础 Window owner；不改为
WindowManagement。系统管理的宿主也可以具有原生 Popup 能力。

Android PopupWindow 和 UIKit 的呈现机制证明移动端存在浮动内容的原生设施，
但不是现有 GOUI Popup 契约的自动等价实现。其坐标、焦点、输入路由、外部关闭、
旋转及宿主脱离等行为需要分别适配、验证；本轮 Android/iOS 仍明确返回不支持。

不因为移动端就强制把 Popover、DropDown 或上下文菜单替换为 Overlay。
Overlay 仍是普通窗口内布局容器；Popover 保留独立原生承载的定位。原生 Popup
可以有多个，不意味着必须创建多个主 Activity 或 Scene。

WorkAreaAt 暂随原有接口迁移，避免扩大改动。未来系统宿主的 Popup 定位如果需要
额外的只读几何，应在具体平台接入时设计，不能据此要求 owner 实现管理接口，
也不能凭桌面 WorkArea 推断移动键盘遮挡、安全区域或可用布局尺寸。

## 3. GUI 设计（已接入，见第 11 节）

```text
gui.Root                        控件使用的最小宿主协议，维持共享内核
  ├─ gui.Surface                控件树、布局、绘制、输入、焦点、观察与生命周期
  │    └─ gui.Window             主动管理窗口、标题、状态、位置和装饰
  └─ gui.Popover                由 Surface 拥有的独立浮动根
```

Surface 是逻辑对象，可以比绘制目标存在更久。暂时失去 drawable、进入后台或
重新绑定原生宿主，不自动销毁 Widget 树、业务状态、信号和声明式协调状态。
最终释放与系统临时脱离必须区分；不得把 Activity/Scene 的任意一次回调解释为进程退出。

Surface 不提供 Show、Hide、位置、大小请求和桌面 WindowState。
Window 继续提供这些明确的管理操作。NewSurface 表示构造逻辑宿主并关联系统提供的
原生呈现机会，不表示由应用创建、任意显示或关闭一个 Activity/Scene。

Application、Surface、Window 及其资源由明确的所有者管理；原生主线程执行 GUI。
普通 Widget 面向 Root；HeaderBar、WindowControls、BorderBox 与标签拖出等管理窗口
的功能明确要求 Window。基本控件不以运行时 OS 分支弥合不同宿主的权限。

Popover 的 owner 扩大为 Surface，公共输入、模态、焦点和事件路由复用 rootBase。
先审视现有具体 *window 依赖，再修改 DropDown、菜单、tooltip、诊断和自动化调用点。
Snapshot 需统一基础宿主信息，并可携带窗口管理信息；不能通过同名方法返回不同类型
假装实现 Go 的接口协变。dev 继续自行管理 ref，不向 GUI 添加自动化专属身份。

第一阶段 GUI 只迁移平台调用点，不新增公共 Surface，不改变现有桌面 API。
移动 `gui.Window` 原型不是可管理窗口；需要窗口管理的操作返回不支持，后续使用
NewSurface 接入，不能给移动对象填充空方法来保留旧演示的成功表象。

## 4. 启动与 UI 声明（已接入）

使用 GTK 风格的生命周期顺序：

```text
NewApplication → ConnectActivate → Run
                       ↓
                NewSurface / NewWindow
```

NewApplication 建立逻辑应用与启动所需设施；Run 接入原生循环，原生已经允许构建
GUI 后触发 Activate。所有原生 GUI 对象在这个阶段之后创建；也可以在后续点击、
计时器或其他 GUI 线程回调中创建，不限于 Activate 回调的调用栈。

Activate 可以重复，处理者复用已有内容。普通焦点变化、drawable 重建不是新的
Activate。首先支持一个主移动宿主，不提前暴露多 Scene 会话路由；自绘方式本身
不限制今后的多宿主支持。

GUI 用户仍通过 NewSurface 主动构造逻辑对象，不被迫直接接收原生 Window 回调。
平台负责交付系统宿主，GUI 负责逻辑对象与原生对象关联，不能在平台实现 GUI 策略。
不新增另一个 `gui.Run(appID, callback)` 来代替 Application。

UI 对应提供 ui.Surface 和 ui.Window；WindowOptions、Center、Chrome 等管理选项
只属于后者。共享 View、状态与局部更新协调，允许应用针对任务、尺寸和原生约束
分别组织不同视图树，不承诺桌面 Demo 原样自动适配手机。

## 5. 阶段与验收

1. **平台接口迁移**：收缩 Window，增加 WindowManagement，保留 Popup owner，
   调整调用点并验证桌面回归、无桌面逻辑与受影响目标编译。原生实现不优化。
2. **原生主线程和系统宿主**：验证 Android/iOS 的激活、暂停、恢复、绘制目标重建、
   最终释放及退出；随后再落实平台交付接口，不以轮询或隐藏 View 模拟窗口。
3. **GUI/UI 接入**：NewSurface、Window 分工、ConnectActivate、Popover owner，
   保留桌面多窗口行为与共享控件内核。
4. **原生 Popup**：逐平台验证实际浮动设施如何满足 GOUI 契约，再接 DropDown 和菜单。
5. **触摸、IME、安全区域**：按各自设计推进，不夹带在接口迁移里。

## 6. 参考与采用理由

核对日期：2026-11-08。以下是外部依据，不是 GOUI 已通过的运行结果。

- [Qt 6.8.3 iOS QIOSIntegration](https://github.com/qt/qtbase/blob/v6.8.3/src/plugins/platforms/ios/qiosintegration.mm)：
  其能力判断区分 MultipleWindows 与 WindowManagement。采用“多个呈现对象不等于
  可自由管理窗口”的区分，不复制 QWindow 的完整跨平台接口。
- [GTK 4.24.1 GdkPopup](https://docs.gtk.org/gdk4/iface.Popup.html)：
  Popup 附属于 Surface，而不是只附属于可管理的 Toplevel。用于确认 owner 与
  窗口管理是独立关系，不将 GDK 的全部 Surface 方法照搬进 Painter。
- [GTK 4.24.1 GtkApplication 初始化](https://docs.gtk.org/gtk4/class.Application.html#initialization)：
  activate/open 等处理者在 GUI 就绪后创建窗口，并需要处理重复激活。采用这个
  启动语义，不引入 GTK 应用服务、对象系统或自动资源装配。
- [Android PopupWindow](https://developer.android.com/reference/android/widget/PopupWindow)、
  [UIKit UIPopoverPresentationController](https://developer.apple.com/documentation/uikit/uipopoverpresentationcontroller)：
  原生确有浮动呈现设施；API 名称不能证明其满足 GOUI 独立绘制与输入契约。
  对应后端和机型适配属于后续原生验证，不能据此声明移动 Popup 已支持。

## 7. 第一阶段验证记录

2026-11-08，Go1.25.8：独立 Linux amd64 Xvnc/Xfwm4 会话的全仓测试 4508 通过、
11 原条件跳过、0 失败；GUI/共享移动宿主 race 通过。使用已跟踪源码临时副本，
不纳入工作区既有未跟踪测试，不改写它们的内容。平台测试会在独立桌面创建、
激活并关闭自身窗口，最终已停止临时桌面。

Windows amd64、macOS arm64 测试目标编译/链接通过，未执行本轮远端窗口验收。
内置构建基座生成 Android ARM64 APK 和 iOS ARM64 未签名 IPA，并编译 GUI、UI、
widgets 依赖；不代表移动 GUI 或新生命周期已运行。旧 android_gui 验收需要迁移。

Linux ARM64 的 SYS_PIPE/SYS_POLL 既有编译问题已用 HEAD 原文件复现，另行跟踪。
完整命令及产物位置见 [Plan](Plan.md#呈现宿主与窗口管理平台接口切片2026-11-08)。

## 8. 原生 UI 主线程调度切片（2026-11-08）

参考 [Flutter 3.29 AndroidShellHolder](https://github.com/flutter/flutter/blob/3.29.0/engine/src/flutter/shell/platform/android/android_shell_holder.cc#L123)：
合并模式让 UI 与 platform 共用原生线程的 task runner。GOUI 采用同一 UI 执行线程，
不复制 Flutter 的 isolate、光栅线程或完整 task-runner 架构。

Go 入口保留一个仅用于启动和等待的 goroutine，不能阻塞原生 onCreate/启动回调。
移动 EventLoop.Run 在该 goroutine 等待，Post 的执行、原生事件、窗口观察及绘制
归系统 UI 主线程。应用可在 Run 前登记 Post，首次任务在原生主循环中执行；
后续 GUI Activate 将复用此机制，不在这里提前引入 GUI 信号或 Surface。

具体实现：

- 共享移动 EventLoop 只维护一份任务队列，通过后端 wake 函数唤醒原生主循环。
  Android 由主 Looper 的 eventfd 回调取任务，iOS 由主 dispatch queue 回调取任务。
  Run 不再消费任务或拥有另一个 GUI 线程。
- 原生观察在所属 UI 线程直接交付；Run 前的观察仍排队。Android redraw 在当前
  原生回调内完成绘制，禁止投递后等待同一线程，以免死锁。
- Quit 丢弃待执行任务，经原生主循环确认后再让 Run 返回；已进入的 UI 回调必须
  先返回。Quit 不表示退出 Activity/Scene，也不承诺系统会结束进程。
- Android 唤醒描述符与主 Looper 绑定，生命周期属于进程启动设施，不随 Activity
  重建撤销；否则旋转间隙 Post 会丢失。Activity 的窗口、输入队列仍逐次解绑。
- iOS 不再保留另一份函数队列或同步等待主线程的桥。原生 UI 对象只在主线程
  调用；已有后台 GPU 许可和目标租约保护先保留，不在本轮顺带改资源策略。

平台 NewPlatform/NewEventLoop 允许启动阶段准备；窗口、Painter 等运行资源应在
主线程 Post 回调中创建与释放。旧移动原型的 Run 前逻辑对象准备仅作为过渡路径，
不作为未来 GUI Surface 契约。新验收程序显式在 UI 回调完成整个资源生命周期。

验证先覆盖队列顺序、跨 goroutine Post、原生线程身份、redraw 同步完成、Quit 的
回调完成边界，以及 Android/iOS Software/GLES 的启动、后台恢复和目标重建。
新回调模型实测稳定后，再实施系统宿主交付及 GUI/UI 激活与 Surface 接入。

### 本切片验证

Go1.25.8，独立 Linux amd64 Xvnc/Xfwm4 的已跟踪源码与本轮新增例程副本：
`go test -json -count=1 ./...` 为 4514 通过、11 原条件跳过、0 失败。
共享移动宿主 `go test -race -count=10` 通过，包含原生观察重入 RunLoop 时的
退出确认测试。Windows amd64、macOS arm64 受影响包及测试目标编译通过，
未执行本轮 Windows/macOS 桌面窗口操作。

远端 macOS15.7.7 ARM64 的独立环境：

- Android API30 ARM64 AOSP_ATD / Emulator37.2.13 / SwiftShader / 1x：
  Software、GLES 各通过原生主线程身份、显式 GC、四次旋转、两次后台停绘和恢复、
  目标代次递增，以及在 UI 回调释放资源后 Quit 的完成确认。
- iOS26.3 ARM64 Simulator / iPhone14 / 3x：Software、GLES 各通过原生主线程、
  显式 GC、两次后台停绘和恢复、目标代次更换与同样的释放/退出确认。
  本轮未执行 iOS 旋转、多 Scene、物理设备或正式签名安装。
- 通过正式 cmd/goui 构建并核对最终包摘要；另构建 iOS device 未签名 IPA。
  四组截图辅助检查文字与图像显示，不将其当作跨后端像素精确验收。

Android 测试配置必须 debuggable 才能 run-as 读取自身沙箱；发布默认值未改变。
当前模拟器配置的 GLESDynamicVersion 默认关闭，首次 GLES 创建正确报告不满足
ES3 配置。使用启动参数 `-feature GLESDynamicVersion` 后重新完成两后端验收，
没有修改 GOUI 的 ES3 要求或增加 ES2 fallback。该选项的作用见
[Android Emulator 源码](https://android.googlesource.com/platform/external/qemu/+/595172be4b752e226bbc05f9e217bcdf7d8b3015/android/data/advancedFeatures.ini)。

用例与操作流程见 [mobile_dispatch](../tests/window/platform/mobile_dispatch/README.md)。
本地/远端证据目录 `/tmp/goui-native-ui-qy_b8s2l`：包摘要、四组生命周期 JSON、
截图、构建及测试日志。本轮专用 iOS Simulator 已关闭删除，Android Emulator
和私有 ADB server 已停止；未接管用户已有模拟器或修改系统配置。

当前仅证明平台线程与调度路径。保留的启动线程 JNI 附着及旧预启动逻辑对象
准备是过渡兼容，不再承担 GUI 任务；后续 GUI 原生服务创建移到 Activate。
NewSurface、窗口来源交付、触摸、IME 和移动 Popup 不在已完成范围。

## 9. 系统宿主交付切片（2026-11-08）

保持 Platform 的桌面窗口工厂，通过同一个 Platform 对象的可选接口发现系统宿主来源：

```go
type WindowSource interface {
    SetWindowHandler(func(Window) EventHandler) error
}
```

- NewEventLoop 后、Run 前可以登记，也可以在原生 UI 回调中更换处理函数；其他线程
  通过 EventLoop.Post 投递。交付只发生在 Run 后的原生 UI 线程，回调返回该 Window
  后续的事件处理函数。绘制目标和尺寸可能尚未就绪，等待正常的 Surface/Size 事件。
- 首个回调登记选择系统宿主模式；不能与移动原型的 NewWindow 混用。桌面 NewWindow
  保持原行为，移动 NewWindow 暂作旧验证路径，不让它充当系统宿主的创建契约。
- 当前只交付一个主逻辑宿主。绘制目标代次变化、Activity/Scene 重建或前后台切换
  不重复交付；已有事件处理函数继续接收观察。更换/清空交付函数只影响未来宿主，
  不回放已有 Window，不转移其资源所有权。返回 nil 允许不监听后续事件。
- Window.Destroy 释放 GOUI 适配与资源，不表示关闭系统应用。交付回调中可以销毁它
  或 Platform；尚未执行的绑定不得复活已销毁对象。后续的宿主交付机会才可能重新交付。
- 系统宿主没有 Show/Hide 等管理 API，内容初始绑定由适配完成；实际可见性仍由
  原生生命周期决定。不通过伪造请求尺寸或 WindowOptions 来构造核心 Window。

Android 以 NativeActivity 可用为交付机会，iOS 以 Scene 创建内容 View 后为交付机会。
不把 ANativeWindow/CAEAGLLayer 替换误认为新逻辑宿主。系统负责 Activity/Scene，
Go 适配负责事件处理函数及自己的资源。原生事实依据 Android NDK
[Activity callbacks](https://developer.android.com/ndk/reference/struct/a-native-activity-callbacks)
和 UIKit [Scene connect](https://developer.apple.com/documentation/uikit/uiscenedelegate/scene(_:willconnectto:options:))；
“跨目标保留同一逻辑宿主”是 GOUI 的生命周期策略，不是两套原生 API 的自动等价。
消费方返回处理函数后才安排绑定，随后报告实际尺寸、可见性和焦点；原生回调若
已经观察到焦点，不能因注册晚于该回调而丢失初始状态。

验证覆盖：正式交付路径、单次交付计数、不可管理能力、事件处理函数安装、回调销毁
防复活，以及 Software/GLES 的 GC、目标重建和后台恢复。GUI/UI Surface 与 Activate
属于下一切片；触摸、IME、Popup 和多 Scene 不在本轮承诺内。

### 本切片验证

- Go1.25.8 / Linux amd64 / CGO_ENABLED=0：仅已跟踪工作源码的仓库外独立
  Xvnc/Xfwm4 副本执行 `go test -json -count=1 ./...`，4516 通过、11 按原条件
  跳过、0 失败；现有未跟踪文件未纳入副本。共享 mobile 包 race 连续 10 轮通过。
- Windows amd64 / macOS arm64：受影响 platform、共享 mobile、gui、ui 和验收目标
  `go test -exec=/bin/true` 编译通过；本轮未执行这两个桌面平台的窗口交互。
- 本机 cmd/goui 使用既有 prebuilt 模式输出 Android Software/GLES 开发签名 APK、
  iOS 两后端 unsigned Simulator App 和 device 未签名 IPA。宿主 CLI 和桌面测试
  使用 CGO_ENABLED=0；移动链接内部使用既有 CGO_ENABLED=1 语义及预编译支持，
  未调用 Android/iOS 原生编译器，不能将其表述为移动链接禁用 cgo 语义。
- 远端 Mac15.7.7 ARM64 的 Android API30 AOSP_ATD ARM64 / 480×800 / 1x /
  SwiftShader（开启 GLESDynamicVersion）：两后端分别四次旋转、两次后台停绘和
  恢复。iOS26.3 ARM64 Simulator / iPhone14 / 3x：两后端分别两次后台恢复。
  四组均验证主线程与启动线程不同、初始焦点、显式 GC、单次宿主交付、替换交付
  函数不重放、不具备 WindowManagement、禁用混合 NewWindow、资源释放及 Quit
  回调完成边界。Android 旋转目标代次递增而逻辑交付仍为 1。
- 包内测试覆盖事件处理函数安装、销毁后不能重新安装或复活，以及核心构造不伪造
  几何、绘制目标和可见性。回调销毁后的延迟绑定由两后端显式生命周期检查防护。
- 原生验证使用系统测试工具，不依赖 DevServer；仅操作专用模拟器及应用自身报告
  文件。截图辅助检查文字、图形和图像，JSON 独立断言线程、状态和代次。
  产物摘要、报告和截图在 `/tmp/goui-window-source-v0t9wwnd`，专用 iOS Simulator
  已删除，Android Emulator 与私有 ADB server 已停止，原 SSH 连接保留。
- iOS 旋转、多 Scene、物理设备、正式签名安装、触摸、IME、Popup、安全区域及
  GUI/UI Surface/Activate 未验收。构建命令首次漏传 `-package apk` 导致库文件
  被当作 APK 安装失败；修正命令后正式 APK 通过，原失败记录保留在证据目录。

## 10. GUI 启动与退出切片（2026-11-08）

Surface 提取前先稳定启动边界。Application 新增 ConnectActivate、ConnectShutdown，
Run 返回 error。Activate 在原生 UI 线程初始化 Typography、Settings、Clipboard、
FileDialog 后触发；初始化失败不触发 Activate，完成 UI 线程退出后由 Run 返回错误。
这里只接入首次激活，不把焦点、目标重建或恢复当作应用激活，不新增手动 Activate。

桌面暂兼容 Run 前创建资源：NewApplication 初始化服务，Run 后才发首次 Activate。
系统宿主平台通过 WindowSource 能力识别，NewApplication 仅准备平台、事件循环与
GUI 默认服务外观；原生服务延迟到 UI 回调。所有新代码在 Activate 后使用原生资源。
这不是最终桌面迁移完成，也不表示 gui.Surface 已实现；不为此引入假的管理窗口。

Quit 可以跨线程调用：停止计时调度、拒绝新 GUI Post，再投递 UI 线程的 Shutdown，
通知使用方和 UI 层清理连接/资源，最后退出原生循环。回调中 Quit 不继续运行后续
Activate 监听者。Shutdown 只触发一次，已开始的回调完成后 Run 才返回。
桌面原生循环直接退出时，在同一 UI 线程完成 Shutdown；移动端循环由 GUI 的 Quit
路径结束，异常缺失 UI 线程退出通知必须作为错误报告，不从启动线程伪装清理。
本轮不改变应用/窗口既有最终资源所有权，也不在退出中提前释放仍有 Widget 借用的
Typography。对象的原生资源仍在 UI 生命周期回调释放。

ui.Run 把首次构建和设置/语言订阅移入 Activate；Shutdown 在 UI 线程解除订阅与
声明式协调连接，Run 返回后仅完成纯 Go 的等待者通知。Sync 不能因 Quit 请求或
Shutdown 提前返回到仍在执行的回调之外。UI 不导入 platform，不查询移动操作系统。

参考 GTK 的 [activate 示例](https://www.gtk.org/docs/getting-started/hello-world) 和
[shutdown 信号](https://docs.gtk.org/gio/signal.Application.shutdown.html)：采用原生就绪后
构建、退出前通知的顺序，不引入其远程应用实例、D-Bus 或命令行注册系统。
GUI Surface/Window、IME/Popover owner 与快照迁移放在下一切片。

### 实现和验证

- Activate 是应用构造时排入的首个任务，早于用户 Run 前 Post，确保服务就绪顺序。
  系统宿主模式的 Typography 在激活前为 nil；Settings 保持同一 GUI 对象并先提供
  GUI 默认值，再在 UI 线程绑定原生来源。NewWindow 未就绪返回 ErrAppNotReady。
- Quit 以原子标记只提交一次 UI 线程退出，尚未开始的普通 Post 不再执行；每个
  Activate 监听者再次检查退出状态，不通过 DisconnectAll 掩盖回调销毁/退出。
  Shutdown 与计时停止、未交付文件访问资源清理均在 UI 线程完成。
- ui.Run 的初始构建、设置/语言订阅、View Mount/Unmount 属于激活线程。协调释放
  与纯 Go 的最终完成通知分开，Shutdown 不提前释放仍在执行回调的 Sync 等待者。
  初始构建失败仍返回原错误并在 UI 线程清理；GUI 原生初始化错误由 Run 传递。
- 包内独立固定线程模拟验证：服务只创建一次、Post 排序、启动失败、Run 前 Quit、
  激活回调 Quit 后跳过后续监听者、一次 Shutdown、重复 Run，以及 UI 构建/挂载/
  卸载归属激活线程。缺失移动 UI 退出确认返回错误，不能在启动线程伪装清理。
- Go1.25.8 / Linux amd64 / CGO_ENABLED=0，已跟踪工作源码与本轮新例程的独立
  Xvnc/Xfwm4 副本运行全仓测试，4525 通过、11 按原条件跳过、0 失败；
  现有未跟踪测试不纳入该副本。
  GUI/UI 全包 race 通过，新增跨线程生命周期另连续 10 轮 race 验证。
- 独立 X11 的 GUI/UI × Software/OpenGL × 应用 scale 1x/2x 八组自动程序通过
  激活线程、首次布局/Snapshot、UI 绑定更新及一次退出检查。首次驱动脚本清理
  变量冲突已修正，八组重跑退出码均为 0，专用 X11 进程已清理。
- Windows amd64/macOS arm64 的 GUI/UI/Widgets、demo 与验收目标编译通过；
  本轮未做这两个桌面系统的窗口交互，也未将应用 2x 当作真实系统 HiDPI。
- 远端 Mac15.7.7 ARM64：Android API30 AOSP_ATD ARM64 1x / SwiftShader
  （GLESDynamicVersion）两后端各四次旋转、两次后台恢复；iOS26.3 ARM64
  Simulator / iPhone14 / 3x 两后端各两次后台恢复。四组均检查初始服务延迟、
  Activate=1、Shutdown=1、稳定的 UI 线程、GUI Timer、显式 GC 和 UI 线程资源
  释放，恢复不重复激活。移动图形仍由平台 WindowSource 承载，不声称 GUI Surface
  或移动控件树已完成。两后端 APK/Simulator App 及 device 未签名 IPA 已生成。
- 例程和中文流程见 [gui/activation](../tests/window/gui/activation/README.md)。
  产物摘要、JSON 报告和截图在 `/tmp/goui-activation-34l4ak0y`；专用模拟器和
  私有 ADB 已停止，iOS 临时设备已删除，保留原 SSH 连接。物理设备、iOS 旋转、
  多 Scene、正式签名、触摸、IME、Popup、安全区域及完整移动 GUI/UI 未验收。

后续先提取 GUI Surface 的共享状态和行为，再迁移 Widget 宿主查询、IME/Popover、
快照和开发适配，最后加入 ui.Surface；避免用包装假窗口或为移动补齐管理空方法。

## 11. GUI Surface 共享内核（2026-11-08，已实现）

Window 嵌入 Surface 公共协议，分别由 window 与 surface 实现；两者组合共享的
surfaceBase，继续复用 rootBase 的帧、事件、手势、焦点和资源内核，不保留两套
绘制/输入实现。Window 只增加标题、状态、位置、装饰、关闭请求等主动管理。
Surface 不实现 Window；不能靠隐藏管理方法的代理或伪造管理空方法冒充系统宿主。

Application.NewSurface 在 Activate 后领取当前系统主宿主，当前仅支持一个；可先
建立逻辑树，稍后绑定平台 WindowSource 交付的 Window。丢失目标只暂停帧并取消
受影响输入，保留 Widget/模型/排版与可复用 Painter。首个目标就绪时创建 Painter，
异步创建失败经 Application.Run 返回。显式 Destroy 结束逻辑树；不解释为关闭应用。

应用准备一个未领取的系统宿主适配以保存初始尺寸、可见性、焦点；未领取不绘制
或对外枚举。领取时使用实际状态启动绘制，不能补造请求尺寸。Shutdown 在 UI
线程最终释放系统 Surface；桌面 Window 的既有生命周期暂不改。再次 NewSurface
可以在旧 Surface 最终释放后重新领取，不引入多 Scene 选择或导航策略。

Widget.Surface 获取所在的呈现宿主（含 Popover 的 owner）；Root 继续表示实际
输入/绘图根。Widget.Window 只对直接桌面 Window 根返回管理窗口，装饰类控件
继续使用它。文本、菜单、DropDown、焦点和 Modal 改为依赖公共 Surface 或最小
内部宿主能力；HeaderBar、BorderBox 和标签拖出明确依赖 Window。

Popover owner 扩大为 Surface，实际创建仍用基础 platform.Window，工作区查询
继续按可选管理能力处理。移动 Popup 未实现就返回原错误，不转成 Overlay。
FileDialog 的 GUI owner 同样接受 Surface，原生支持与输入隔离维持既有规则。

统一 SurfaceInfo（暂保留 WindowInfo 别名）供所有宿主 Snapshot 使用；Title 和
Controls 只作为 Window 的可选语义元数据，不扩大 Surface 管理接口。Application
统一枚举 Surfaces，Windows 仍提供管理窗口子集。快照及 dev 引用遍历公共宿主，
dev 自行维护 ref，执行仍通过 DispatchEvent/CommitText，不向 GUI 加自动化捷径。

验证同时覆盖公共接口不具备管理能力、Root 身份、焦点/文字提交、目标暂失保持
Widget 与状态、销毁重入、Popover/Modal、语义快照与引用；窗口例程增加真正的
移动 GUI 控件树。桌面回归检查原生窗口、装饰和既有控件，不以编译替代交互验收。
内部字段移动需同步包内构造器，保留已有测试的有效断言，不借机改变测试含义。

### 使用方式与事实边界

```go
app, err := gui.NewApplication("org.example.mobile")
if err != nil {
    return err
}
app.ConnectActivate(func() {
    host, err := app.NewSurface()
    if err != nil {
        // Handle the startup error and request Quit as appropriate.
        app.Quit()
        return
    }
    host.SetID("main")
    host.SetWidget(gui.NewLabel("Hello"))
})
return app.Run()
```

NewSurface 不接受 WindowOptions，不调用 Show，也不在没有系统宿主的桌面平台
合成窗口；重复领取同一个存活宿主返回不支持。领取可以早于原生绘制目标就绪，
不能依赖此时存在有效尺寸。Visible 是最近一次原生可见性事实，Focused 是宿主
焦点事实；两者与控件逻辑焦点分开。目标暂失保留逻辑焦点，但中断进行中的输入。
最终 Destroy 不复活同一个逻辑宿主；再次领取建立新的逻辑根，不复用旧 Widget。

普通 Application.Snapshot 的顶层字段改为 Surfaces / JSON surfaces。WindowInfo
保留为 SurfaceInfo 别名，便于窗口 Snapshot 使用方迁移；不同时输出两份宿主列表。
dev 同步使用 snapshot.surfaces，旧的 snapshot.windows 客户端需更新，ref 的分配、
失效和正常输入路径不变。diagnostics 元数据、HUD 与文件对话框 owner 也支持公共
Surface；普通控件不必猜测操作系统。HUD/诊断清理在 Shutdown 的 UI 回调中完成，
不能等 Run 返回后在移动启动线程调用可能释放原生排版的回调。

### 参考与取舍

2026-11-08 核对 [Qt Quick 6.12.0 QQuickWindow](https://doc.qt.io/qt-6/qquickwindow.html)
的图形资源释放与 sceneGraphInvalidated 契约：图形资源可用性有独立生命周期，
不应据此销毁应用逻辑内容。[Flutter Android FlutterView](https://api.flutter.dev/javadoc/io/flutter/embedding/android/FlutterView.html)
公开 attach/detach 引擎，并要求宿主接好
[Activity 生命周期](https://docs.flutter.dev/add-to-app/android/add-flutter-view)。这些是
参考框架的公开契约；GOUI 由此采用逻辑内容与原生目标分离，不引入 Qt 渲染线程、
Flutter 引擎/隔离域或任意 View 重新附着能力，也不推断引擎必然跨 Activity 存活。
下面是 GOUI 的实测范围，不是从参考资料推导的支持承诺。

### 本切片验证

- Go1.25.8 / CGO_ENABLED=0，Linux amd64 独立 Xvnc/Xfwm4：已跟踪源码及本轮
  新增文件的副本执行 go test -json -count=1 ./...；最终计数见 Plan 本轮记录。
  未混入工作区既有未跟踪测试、图像或实验目录。GUI/dev/diagnostics/UI/Widgets
  全包 race 通过；Surface 与诊断 UI 清理定向 race 连续 10 轮通过。
  race 使用宿主标准 CGO_ENABLED=1；初次 CGO_ENABLED=0 go test -race
  -run '^TestApplicationStopsDiagnosticsOnUIThread$' -count=10 ./gui 被 Go 的
  “-race requires cgo” 拒绝，未执行测试，调整后在独立暂存副本通过。普通全仓、
  目标交叉编译与用户移动构建的无原生编译器契约没有改变。
- 包内覆盖公共宿主不具备窗口管理能力、Widget.Root/Surface 身份、焦点/文字提交、
  目标替换保留内容、显式销毁重入、Popover owner、枚举、语义快照和 dev 引用失效。
  Painter 同步创建失败交调用方，延后创建失败交 Run，均验证资源释放。既有测试
  仅迁移内部构造字段，保留原有布局、事件、样式和资源断言。
- 独立 X11 的 GUI/UI × Software/OpenGL × 应用缩放 1x/2x 八组窗口验收通过。
  Windows amd64 / macOS arm64 的 GUI/dev/diagnostics/UI/Widgets、demo 与
  受影响例程编译通过；本轮没有 Windows/macOS 桌面交互验收。
- 远端 macOS15.7.7 ARM64：Android API30 ARM64 AOSP_ATD / Emulator37.2.13 /
  SwiftShader / 1x，Software/GLES 各通过四次旋转、两次后台恢复；iOS26.3
  ARM64 Simulator / iPhone14 / 3x，两后端各通过两次后台恢复。
  四组均实际建立 Label/TextInput/Button/LinearBox 控件树，显式 GC、按钮点击、
  文本提交与正常退出通过；每组挂载/卸载/销毁各一次，目标更换不重挂树。
  公开诊断核对实际 Backend，未静默降级；隐藏停绘/恢复、线程与原生适配身份稳定。
- 输入通过 Surface.DispatchEvent/CommitText 正常路径注入，不绕过控件状态机，
  但不等于原生触摸/IME 验收。截图辅助复核文字和布局，不作为跨后端像素金图。
  顶部留白是例程明确设置，不代表 SafeArea 已接入。
- 本机使用正式 cmd/goui 构建两组 APK、两组 Simulator App，另生成 device 未签名
  IPA，仅验证构建；消费机未调用原生编译器。产物与摘要、包内测试、四组报告、截图
  及清理结果在 /tmp/goui-surface-w1pyo_cn（本机/远端）。专用模拟器、私有 ADB 已
  停止，临时 iOS 设备已删除，未接管用户设备或修改系统设置。

ui.Surface 已完成第 12 节后续切片；原生触摸/IME、移动 Popup、SafeArea、
多 Scene、iOS 旋转、真正 GPU 上下文丢失、物理设备及正式签名安装仍待后续。


## 12. UI Surface 声明（2026-11-08，已实现）

ui.Surface(id).Content(view) 是系统主宿主的 RootView，支持 Commands、Shortcuts、
OnAfterPaint 和 OnDestroy；没有 Size、Show、Chrome、Center、Transparent 或状态请求。
ui.Root().Surface(surface).StyleSheet(sheet) 可以声明应用样式，也可以同时声明 Windows；
能否创建实际对象由 GUI/平台决定，不在 UI 层按操作系统推断或自动替换成其他宿主。
RootNode 仅保留一个 Surface 声明槽，不提前引入 Scene 列表。

ID 不能为空，Surface 与 Window 共用应用内的宿主 ID 命名空间。同 ID 的 Surface 更新
复用原对象与协调节点；移除或换 ID 结束旧声明的生命周期，再领取新逻辑宿主。原生目标
暂失或后台切换不改变声明，不重建树或丢失局部 State。领取失败正常交给 ui.Run，
不能在不支持 WindowSource 的平台创建假窗口或静默忽略。

内容继续使用现有 root/node 协调与批次调度；把 root 的挂载对象收敛为 gui.Surface，
窗口与 Surface 仅保留各自创建/管理/信号胶水。默认 CommandGroup 和快捷键绑定共享，
所有根更新完成后再执行 AfterUpdate，之后仅对新管理窗口 Show。ComposeView 的自动
状态依赖与属性绑定仍沿既有路径，不因增加宿主建立另一份更新循环。

App.FindSurface(id) 查询任意已挂载宿主；FindWindow 继续只返回管理窗口。
FindWidget/FindCommand 的第一个参数改称 hostID，允许两类宿主，ID 规则不变。
文件对话框同样用宿主 ID 解析 owner，不添加 Surface 专用的第二套文件接口。
跨根协调搜索纳入系统 Surface；窗口管理类控件依旧依赖 gui.Window。

UI 协调释放连接与订阅，GUI 宿主负责最终销毁 Widget。暂失绘制目标不触发 UI Release；
真正 Destroy 后断开声明连接并从查询表移除。退出清理仍在原生 UI 线程；停止后排队的
State 更新不得重建控件或重新领取宿主。

实施涉及 ui/app.go、root.go、window.go、surface.go、transfer.go、dialog.go 及相关
包内回归；窗口例程增加 UI Surface、绑定及局部构建状态保留验收。先执行无桌面逻辑、
race/全仓和桌面回归，再在 Android/iOS Software/GLES 验证声明、正常输入路径、
后台恢复、Android 旋转与 UI 线程退出，不把合成输入视为原生触摸/IME 支持。

### 最小声明与共同协调

```go
count := ui.MakeState(0)
err := ui.Run("org.example.mobile", func(app ui.App) ui.RootView {
    return ui.Surface("main").Content(ui.VBox(
        ui.Build(func() ui.View {
            return ui.Label(fmt.Sprintf("Count: %d", count.Get()))
        }),
        ui.Button("Increment").OnClick(func() { count.Set(count.Get() + 1) }),
    ))
})
```

count 的所有者在 Run 外创建一次；事件写状态，Build 追踪读取并局部更新。不要在
每次视图构建中重建 Model、启动计时器或创建原生对象。Surface 与 Window 共用
hostMount 的默认命令组、快捷键、内容 Root 和连接管理；Window 仅附加创建选项、
标题/最小尺寸/Show/关闭请求。没有第二个状态调度器，也没有向 BuildContext 添加
Surface 特例。Root 的内部挂载方法由 Window 改称 Host，既有测试仅同步私有名称和
公共状态的构造位置，保留原断言。

### 本切片验证

- Go1.25.8，Linux amd64 独立 Xvnc/Xfwm4，已跟踪源码与本轮新增文件的隔离副本：
  CGO_ENABLED=0 go test -json -count=1 ./... 为 4544 通过、11 原条件跳过、0 失败。
  工作区既有未跟踪实验/测试/产物未混入。UI 与 Widgets 全包 race 通过，新增 Surface
  回归 race 连续 10 轮通过；race 使用宿主标准 CGO_ENABLED=1。
- 新增断言覆盖局部 State/BindText、全量重建保留宿主/输入、移除/换 ID 的释放和
  重领、销毁后订阅与查询失效、错误声明/不支持平台不回退、Run 传播领取失败、默认
  命令组/快捷键、绘制回调不重复、跨根 AfterUpdate、文件对话框 Surface owner。
- 本机独立 X11 的 GUI/UI × Software/OpenGL × 应用缩放 1x/2x 八组均通过，
  桌面管理窗口仍通过普通 Show 显示。Windows amd64/macOS arm64 受影响 UI/Widgets、
  例程与 demo 编译通过；本轮不宣称两端桌面原生交互已验收。
- 正式 cmd/goui 在本机生成两组 UI APK、两组 Simulator App 及 device 未签名 IPA。
  继续复用最小移动基座，消费构建未安装/调用移动原生编译器。IPA 仅构建，不视为
  物理设备、签名安装或商店发布通过。
- 远端 macOS15.7.7 ARM64 的专用模拟器：Android API30 ARM64 AOSP_ATD /
  Emulator37.2.13 / SwiftShader / 1x，Software/GLES 各四次旋转、两次后台恢复；
  iOS26.3 ARM64 Simulator / iPhone14 / 3x，两后端各两次后台恢复，四组通过。
  普通事件路径完成文本提交和按钮点击，ui.State 的文本/数值与最终 Label 一致；
  应用构建一次、计数区域局部构建两次。目标替换不重挂树，Widget 挂载/卸载/销毁各
  一次，OnDestroy 一次；UI/GUI 在原生 UI 线程清理，Run 等待回调完成。
- 初次移动例程在写 State 的同一 Timer 回调里断言协调后的 Label，触发断言导致
  两端首帧后退出，驱动报等待超时。保留失败报告和 Android panic；改为后续回调
  检查正常异步批次完成后的结果，所有原预期不变，重新构建并核对摘要后四组通过。
  没有修改框架调度来强迫同步，也没有削弱失败条件或增加平台跳过。
- 证据目录 /tmp/goui-ui-surface-zl4fu2jp（本机/远端）包含正式包摘要、全仓 JSON、
  桌面八组、四组原生生命周期报告/截图和首轮失败。截图辅助复核布局与文字，不作为
  抗锯齿金图或原生触摸/IME 验收。专用 iOS Simulator 已关闭删除、只读 Android
  Emulator 和私有 ADB5059 已停止；未修改用户已有设备、应用或系统设置。

完整命令与操作见 [activation 验收例程](../tests/window/gui/activation/README.md)。
2026-11-09 调整后续顺序：先按 [平台稳固计划](PlanMobileAPI.md#7-平台实现稳固优先2026-11-09)
复核现有实现并验证原生触摸事实，通过后再继续 GUI 手势和控件。IME 独立推进；
移动 Popup、安全区域、多 Scene、iOS 旋转、GPU 上下文丢失与物理设备继续分别验证。
