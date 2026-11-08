# 呈现宿主与窗口管理

状态：2026-11-08 确认分层方向，分阶段实施。第一阶段只迁移平台接口及其调用点，
不重写原生线程、事件循环或移动宿主。GUI/UI 的 Surface 与激活流程属于后续计划。

关联：[核心原则](Design.md#cross-platform-scope)、[移动接口复核](PlanMobileAPI.md)、
[移动生命周期](DesignMobileLifecycle.md)、[Widget Painter](DesignWidgetPainter.md)。

## 1. 目标

共享布局、控件、事件、绘制与声明式协调，不要求桌面和移动应用采用相同的顶层
生命周期。应用可以明确选择自由管理的窗口，或受系统管理的呈现宿主。
不按 Android/iOS/桌面名称推断窗口能力，也不以隐藏内容 View 模拟隐藏应用窗口。

第一阶段保留已验证原生实现，仅调整接口边界。移动原型仍有独立 GOUI 线程、
Run 前登记逻辑窗口以及内部 Show/Hide 方法；这些是现存实现，不是新设计的承诺。
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
正式的系统宿主交付及 GUI NewSurface 绑定在下一阶段完成；届时再收紧工厂职责。

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

## 3. GUI 设计（待实现）

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

## 4. 启动与 UI 声明（待实现）

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
