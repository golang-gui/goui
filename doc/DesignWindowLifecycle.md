# 原生窗口生命周期

> 状态：2026-10-28 已实现；运行验证、环境和缺口见 [Plan](Plan.md)。

## 定位

原生宿主可能不经过 `gui.Window.Destroy` 就结束生命周期。X11 允许另一个 X client
销毁 Window；Win32 可以由同线程的原生调用或关联窗口的销毁触发；AppKit 的 `close`
可以绕过 `windowShouldClose`。平台层报告事实，GUI 层复用现有清理和应用退出策略。

不增加一套“异常关闭”或可否决的销毁请求，不把 Hide、最小化、遮挡当成销毁。

## 终止事件

追加 `events.Destroy` 和 `events.DestroyEvent`，表示一个平台 Window/Popup 的
**可用生命周期已经结束**，不表示所有系统存储和 Objective-C 引用已经释放。

- 在所属 GUI 线程发出，不能否决，每个原生宿主至多一次。
- 发出前 `NativeHandle()` 已为零，常规事件和后续重绘停止。
- 显式 `Destroy` 与原生终止共享幂等清理；保留释放所需的私有资源。
- 外部处理者可能重入、销毁对象。回调后不得继续无条件使用失效的原生对象。
- `CloseEvent` 始终只是关闭请求；其否决机制不变。

## 平台时序

| 平台 | 终止观察 | 清理规则 |
| --- | --- | --- |
| X11 | 真正的 `DestroyNotify`，使用其 `window` 字段 | 在 XIM/DnD 过滤前处理，移除路由、作废 XID、取消待绘制；释放自有 GC、colormap、IME、sync 资源，不再次销毁已消失的 X Window |
| Win32 | `WM_DESTROY` | 立即停止呈现；`WM_NCDESTROY` 再完成 HWND 路由关联清理，不递归 `DestroyWindow` |
| AppKit | `windowWillClose:` | 与 `windowShouldClose:` 分离；`releasedWhenClosed=false`，Window 明确拥有 allocation，最终 release 一次 |

AppKit 的委托回调中临时 retain Window、delegate、view，并在外层 autorelease pool
释放，保证 GUI 同步清理后 AppKit 的 close 调用栈仍可安全返回。不能在回调自己的内层
pool 提前 drain，也不能递归调用正在进行的 close。

X11 的 GLXWindow 与其引用的 X Window 分开管理。已知宿主丢失错误由显示连接、请求序列
范围、资源 ID、错误码共同限定；未匹配的错误转交原有处理器，不全局吞掉 BadWindow。
`platform/internal/xerrors` 仅记录标志，不在 Xlib 错误回调内部调用 GUI 或 GL。
请求范围结束后仍保留尚未处理的异步错误范围，随 Xlib 已观察的处理进度回收；查询本地
序列计数不引入服务器往返。正常帧不增加 XSync、几何探测或 server grab。

## GUI 接入

`Window.DispatchEvent` 优先处理终止，复用 Destroy 信号、控件树释放、窗口资源清理和
最后一个窗口退出策略。通知不经过关闭请求、输入事件或 Chrome 查询。

显式 Application.Quit 与桌面原生事件循环正常结束都进入同一个幂等 shutdown。
Shutdown 通知前停止定时器调度和系统设置监听，拒绝新的应用任务；通知后由窗口
所有者释放剩余控件与原生资源。ConnectShutdown 在 UI 线程执行一次，并在 Run
返回前完成；原生循环已结束时也不跳过清理，不要求通知一定发生在事件循环停止前。

Popover 的原生表面终止只释放本次表面、退出模态并发出一次 Closed，保留内容和
Popover 对象；之后 Show 可以重新创建表面。已有 `surfaceEpoch` 排除旧表面的迟到事件。
不改变 Popover 显式 Destroy 对内容的现有卸载规则，不添加 UI 层 API。

若终止同步重入活动帧，立即停止后续帧，待 End 和帧退出后再释放 Painter/控件树。
不能在 Begin/End 之间销毁 Painter；普通非绘制场景的清理保持同步。

## 绘制资源

- OpenGL 在绑定和呈现前检查宿主。缺失表面时取消提交；其他绑定错误仍保留诊断。
  销毁 Painter 前先作废图片包装，再释放文字图片缓存。控件卸载可能先释放缓存图片，
  这条路径也不能重新绑定失效的宿主。
- 只有成功绑定 Context 才执行 GL 删除；否则释放 Go 引用并销毁独占 Context，由其回收
  GPU 资源。不为失效宿主另建兜底清理 drawable，不在错误的当前 Context 中执行删除。
- GLX 的正常帧外资源操作使用 Context 所有、与实际 FBConfig 兼容的固定 pbuffer；
  窗口 drawable 只在窗口帧绑定。该资源目标不延长宿主寿命，已观察到宿主失效时不执行
  GPU 工作。绑定解除后释放 drawable/Context，见 [SharedPopover](DesignSharedPopover.md)。
- Software 只对 `ErrUnavailable` 的呈现失败丢弃本帧，意外错误仍报告。
- Direct2D 平衡已经开始的 EndDraw，但不再向已终止的 HWND Present 或安排重绘。

## 初始居中

`Center` 是创建偏好：工作区或定位返回 `ErrUnsupported`/`ErrUnavailable` 时保留系统默认
位置，继续创建。其他错误继续上报，并释放隐藏窗口。显式 WorkAreaAt/SetPosition 的
错误契约不变，不拿屏幕尺寸冒充工作区，不增加显示器或 owner 抽象。

## 验证

包内覆盖居中降级/意外错误、关闭否决、终止去重、应用退出、绘制重入清理顺序、Popover
内容保留和重新打开、旧表面事件隔离、OpenGL 图片卸载及 X 错误精确匹配/异步回收。

[独立窗口用例](../tests/window/gui/window_lifecycle/main.go) 可先以 `-platform` 验证平台
Window/Popup 和 Painter，再验证 GUI。Linux 使用另一 X 连接执行 XDestroyWindow，
不是 XKillClient；Windows 使用同线程 DestroyWindow；macOS 直接 close。
包含实际绘制、排队请求、重复 Destroy、失效后的提交、多个宿主和最后窗口退出。

Windows 的 RequestClose 异步投递 WM_CLOSE，用例在下一阶段检查计数；操作失败不会被
后续成功覆盖。外部执行器同时核对退出码和 PASS 日志，不把启动或空日志算成验收通过。

## 原生契约与参考

以下是 2026-10-28 核对的 API 契约和 GTK main 源码，不代替运行验收：

- [GTK4 X11 foreign destruction](https://github.com/GNOME/gtk/blob/main/gdk/x11/gdksurface-x11.c)：借鉴宿主终止时撤销调度/路由、区分自身销毁和外部销毁，不引入 GDK 对象体系。
- [Win32 WM_DESTROY](https://learn.microsoft.com/en-us/windows/win32/winmsg/wm-destroy)、[WM_NCDESTROY](https://learn.microsoft.com/en-us/windows/win32/winmsg/wm-ncdestroy)：分别用于停止使用和完成原生关联清理。
- [AppKit close](https://developer.apple.com/documentation/appkit/nswindow/close())：与关闭请求不同，明确引用归属和回调期间的有效期。
- [GLX 1.4，3.3.4/3.3.7/3.3.10](https://registry.khronos.org/OpenGL/specs/gl/glx1.4.pdf)：区分 GLX drawable 和底层 X Window，限定宿主丢失错误。

## Popover 的 Surface 资源登记（2026-11-17）

锚点卸载只结束显示；普通 Popover 的隐藏原生资源仍登记到 Surface，直到显式 Destroy、
原生终止、跨 Surface 迁移或 owner 结束。Surface 清理包括隐藏实例，并等待其活动帧、
原生绘制及内容变更回调安全退出。先卸载绘制资源再释放 Painter，最后释放原生 Popup。
普通 Go 对象可在新 Surface 重新显示；共享槽的专用普通 Popover 则随 Surface 最终 Destroy。

## macOS 正常应用退出

macOS 的菜单 Quit、Command+Q 和 Dock Quit 都表示正常退出，不增加可取消的应用
退出请求，也不为菜单接入增加通用 ApplicationLifecycle 服务或平台应用事件流。
Platform.NewEventLoop 保持无参数；WindowMenuBar.SetMenu 继续只负责窗口业务菜单。

Cocoa 的应用委托与基本应用菜单随本平台应用初始化建立，由 Platform 释放。
标准 terminate: 进入 applicationShouldTerminate: 后，先安全结束仍在跟踪的菜单，
再通过既有 EventLoop.Quit 停止原生循环。委托返回 NSTerminateCancel 仅为了避免
AppKit 直接结束进程、跳过 Go 的收尾；不是交给 GUI 或应用否决退出。

原生循环正常返回后，gui.Application.Run 调用既有幂等 shutdown；与显式 Quit
共用定时器、设置监听、Shutdown 信号和窗口／服务清理。在 Run 返回前完成清理，
不依赖存在窗口、MenuBar 或应用级事件 handler。窗口自身的 CloseRequest 仍是
单独的可取消操作，不阻止 Application 直接销毁窗口完成退出。

该接入仅处理 AppKit 的正常应用退出，不改变 Android/iOS 的系统宿主与进程寿命。
参考 Qt 6.8.3 Cocoa 应用委托同样避免直接 exit 并正常返回 main；GOUI 不接入其
可取消会话管理。菜单边界和验证见 [DesignMenuBar](DesignMenuBar.md)。
