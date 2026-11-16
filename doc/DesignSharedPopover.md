# SharedPopover 与窗口级共享宿主

本文记录 2026-11-16 确认的设计。实现和验收进度见 [Plan](Plan.md)。

## 公开对象与归属

`gui.NewSharedPopover(anchor) SharedPopover` 创建调用者独立持有、可反复使用的
轻量对象。它保存内容引用、样式、位置、输入策略和信号，不实现 Root，也不包含
rootBase、分发器、焦点、Painter 或原生 Popup。构造和 SetWidget 不挂载内容；anchor
可以尚未挂载，Show 才解析其所属 Surface。

每个 Surface 内部拥有一个私有共享 GUI 宿主。该宿主实现 Root/EventTarget，负责
唯一的内容挂载位置、布局、焦点、事件、帧和原生资源。内容的 Widget.Root() 返回它。
普通 NewPopover 保持独立宿主和多实例契约，不负责共享仲裁；两种宿主组合复用内部
基础机制，不通过包装完整普通 Popover 复制状态。

SharedPopover 提供 Widget/SetWidget、Anchor、StyleName/SetStyleName、
Placement/SetPlacement、Position/SetPosition、Modal/SetModal、
AutoDismiss/SetAutoDismiss、Visible、FocusedWidget/SetFocusedWidget、
DispatchEvent、CommitText、Show/Hide/Destroy 和 ConnectDismissRequest/ConnectClosed。
Modal 和 AutoDismiss 默认 false，保持独立。它不公开 Root 的帧连接、原生透明配置、
Painter 或共享宿主的销毁入口。隐藏对象的配置不改变别人的显示。

## 生命周期与缓存

- New 只创建轻量对象。首次 Show 按需建立所属 Surface 的宿主、Popup、Painter，
  挂载本次内容。正常 Hide/Show、内容切换不销毁重建这些资源。
- Hide 只结束自身显示，撤销输入、焦点和帧调度，保留至多一份挂载内容。
  同一对象重开复用该树；隐藏期间失效只记录，不持续绘制。
- B 替换 A 时先关闭 A，再卸载旧内容和挂载 B。旧 Widget 按既有所有权规则卸载，
  不隐式 Destroy；调用者仍可保留并再次显示。
- Destroy 只关闭自身，卸载仍属于自己的缓存内容并释放引用、连接，不销毁共享宿主。
  anchor 卸载同样结束自身显示并清除挂载缓存；轻量对象可在 anchor 再挂载后使用。
- 共享按 Surface 隔离。迁窗后使用新 Surface 的宿主，旧宿主保持隐藏供旧窗口复用。
- Surface 销毁时先使显示和回调失效，再清理内容、Painter、Popup，最后释放 owner
  原生窗口。若同步重入绘制，等 End/原生绘制调用退出后再释放资源。
- 原生目标真正终止时结束本次显示；按既有失效契约清理，并允许后续 Show 重新创建。
  正常复用和设备/原生资源失效恢复必须分开统计。

隐藏内容不接受输入、不进入可见语义快照。缓存不按每个菜单长期保留独立挂载树。

## 仲裁与重入

每个 Surface 同时只有一个 SharedPopover 显示。菜单和下拉框替换当前显示；Tooltip
优先级最低，不能抢占它们，且打开菜单/列表取消待显示提示。普通 Popover 可以独立
共存。新 anchor 若属于即将被替换、卸载的共享内容则 Show 失败；单槽不表达级联菜单。

内部同时校验对象身份、显示代次和原生代次。A 被 B 替换后，A 的 Hide、Destroy、
卸载通知和旧事件不能改变 B。同一个对象重开也不能接受上次显示遗留的内部回调。
清理输入、焦点和可见状态后再发 Closed，每次可见结束最多一次；准备失败不伪造关闭。

A.Closed 回调若打开 C，原本打开 B 的调用不得覆盖 C，返回可用 errors.Is 判断的
ErrPopoverSuperseded。内置控件将其视作显示请求取消；真实原生错误仍保留原因。
所有外部回调返回后重新检查生命周期。绘制中的树变更仍遵守现有绘制回调约束。

## 几何与首帧

每次显示在隐藏阶段测量当前内容、样式和工作区，提交尺寸，再用 Popup.ClientSize
读取实际 DIP/backing 像素并 Arrange。布局细化须有限收敛；定位造成 scale 变化后重新
确认尺寸。位置始终按本次 anchor 和实际尺寸求解，不能把上次请求当系统观察。

出现时位置和大小已经确定，旧内容不得作为新内容短暂显示。可延后内容显示；若平台
必须先建立可见绘制上下文，期间仅能呈现本次干净背景或透明内容。Show 后同步 Repaint
不自动证明合成器未观察旧 backing，需要原生首帧断言和桌面验证。查询/布局失败保持
隐藏并返回错误。可见内容的尺寸更新也统一处理几何、布局与像素有效性。

## 合成器保留内容的失效

慢绘制验证在 Windows Direct2D 中明确复现：大菜单切小菜单后，新尺寸/位置已正确，
但首帧提交前仍出现旧菜单文字。隐藏阶段 Repaint 清屏本身不能保证修正，因为原生隐藏
绘制可被延后，DirectComposition 仍持有之前的 swap-chain 图像。

持有外部合成呈现的后端实现 graphics.PresentationInvalidator。Direct2D 通过现有 Visual
的 SetContent(nil)、Commit 和 WaitForCommitCompletion 使旧图像失效，保留 Device、
Visual、SwapChain、Painter；成功完成下一次 Present 后才重新挂接内容。Software 的
Windows 透明上传同样通过其原生所有者的 Direct2D Painter 失效，普通 CPU 图片不销毁。

GUI 在隐藏阶段请求合成呈现失效，并经合法原生绘制入口清除 drawable；清除不运行
Widget.Paint、布局或用户帧信号。然后固定几何显示，再正常绘制本次内容。隐藏 backing
尚不存在的平台允许延后清除，但不得重新挂接已失效的旧合成图像。失效失败保持隐藏，
返回原错误，不按透明能力降级重试。此操作不是反复绘制内容来掩盖错误。

Windows Direct2D 的同一 3 秒慢绘制放大验证中，修正前出现 Action 3-1/3-2，修正后
等待期间透明，最终只出现当前 Action 1-1。该证据不代表所有后端的桌面合成已验收。

原生契约：[IDCompositionDevice::WaitForCommitCompletion](https://learn.microsoft.com/en-us/windows/win32/api/dcomp/nf-dcomp-idcompositiondevice-waitforcommitcompletion)
等待前一次 Commit 被合成引擎处理，不承诺扫描显示完成；GOUI 只用于显示前的内容失效。

## GLX 帧外资源目标

Linux X11/OpenGL 软件光栅化、1x 的原生实验中，窗口 Map 前
几何已正确；旧内容卸载释放纹理时提前绑定窗口 drawable，随后扩大窗口的首帧仍可能
沿用旧缓冲范围。固定离屏资源目标原型通过 60/60 次完整坐标断言；这不是所有驱动或
三平台已验收的结论。

GLX 使用同一 Context 和实际兼容的 FBConfig 创建固定 pbuffer，供帧外图片创建、更新、
销毁、Painter 初始化及 RenderImage 使用。窗口 drawable 只用于真正窗口帧。资源目标
归 Context 所有，不随菜单切换重建，不绕过宿主有效性检查；创建失败保留真实原因。
销毁先在有效 Context 中释放 GPU 对象，解除绑定，再释放 drawable/Context。
WGL/AppKit/GLES 保留其原生有效期及线程约束，不因 GLX 问题机械增加离屏资源。

这将 [窗口生命周期](DesignWindowLifecycle.md) 中不建立隐藏清理 drawable 的约束
细化为：不为失效宿主另建兜底清理目标；正常帧外资源目标仍受所属宿主有效性约束。

## 输入、快照和声明式接入

原生事件进入实际宿主，经现有 EventDispatcher/Controller 处理。SharedPopover 的输入、
焦点方法检查当前身份及内容边界后转交，不另建事件路径。隐藏/失效目标拒绝输入。

Surface.Popovers 改为返回 []PopoverTarget，普通和共享弹层均返回绑定当次显示的受限
目标，提供枚举、快照、正常事件投递及关闭通知，不提供配置、Show/Hide/Destroy。
一次显示结束后目标失效，同一对象重开也产生新身份，避免旧 DevServer 引用操作新树。
该协议服务现有枚举，不增加供控件管理的 Presentation 类型。

PopoverMenu、MenuButton/文本右键菜单、DropDown、Tooltip 改用 NewSharedPopover；
完成迁移后移除 NewTransientPopover。ui 继续协调既有控件和信号，不增加独立宿主体系。
归属以 Surface 为边界；移动端呈现和原生输入效果仍须按移动复核流程单独验收。

## 参考与适用条件

- Qt 6.8.3 [QTipLabel::reuseTip](https://github.com/qt/qtbase/blob/v6.8.3/src/widgets/kernel/qtooltip.cpp#L178)
  和 [showText](https://github.com/qt/qtbase/blob/v6.8.3/src/widgets/kernel/qtooltip.cpp#L439)：
  参考 Tooltip 的内容与既有宿主复用，不声称 Qt 全部浮层共用或 Hide 后永久保留。
- Flutter [OverlayEntry 契约](https://api.flutter.dev/flutter/widgets/OverlayEntry-class.html)
  及 [3.24.5 源码](https://github.com/flutter/flutter/blob/3.24.5/packages/flutter/lib/src/widgets/overlay.dart#L205)：
  参考宿主与独立操作对象分工，不引入多层 Overlay 模型。
- Qt 6.8.3 [QOffscreenSurface](https://github.com/qt/qtbase/blob/v6.8.3/src/gui/kernel/qoffscreensurface.cpp#L20)
  说明纹理/FBO 资源目标及实际格式兼容要求。
- GTK 4.16.13 [GLX make-current](https://github.com/GNOME/gtk/blob/4.16.13/gdk/x11/gdkglcontext-glx.c#L228)
  的帧外 leader surface 是资源与窗口帧目标分离的源码参考，不照搬 GTK 事件循环。

## 验收

包内回归覆盖宿主身份、挂载次数、隐藏调度、配置/焦点隔离、重入、失败、窗口隔离和
当次显示目标失效。桌面验证在 tests/window 下独立运行，检查原生首帧完整坐标、资源
身份/创建次数、大小内容交替、慢绘制、主窗移动/屏幕边缘及缩放。

Linux Software/OpenGL、Windows Software/OpenGL/Direct2D、macOS Software/OpenGL
分别记录原生结果；编译不代替运行。ApiBox HTTP 方法下拉框与 URL 输入框右键菜单
交替作为应用回归。交付执行 go test ./...，通过、失败、跳过和未执行分别记录。


## 现有调用方迁移

`NewTransientPopover` 已移除。保存该返回值的字段也应从 `gui.Popover` 改为
`gui.SharedPopover`，不能只替换构造函数名。内容需要帧/布局宿主时通过挂载后的
Widget.Root() 访问；Surface.Popovers 的元素改为 PopoverTarget，不再是可配置宿主。

ApiBox 当前使用的 goui-sourceview v0.6.0 在 sourceview/completion.go 保存了
`popup gui.Popover` 并调用旧构造函数，需要同时迁移这两处。仓库内没有修改外部依赖
或模块缓存；应用集成验证通过仓库外临时 workspace 使用依赖副本中的这份适配，
数据目录另由 Go overlay 隔离。外部库应独立更新依赖
后消费新的 GOUI API，不能把该验证记录当作旧模块版本直接兼容的承诺。
