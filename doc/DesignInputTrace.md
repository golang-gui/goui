# 原生输入诊断例程

## 目标与范围

`tests/window/platform/input_trace` 用同一套 TreeView／TableView 场景收集三平台
原生输入和 GOUI 转换结果，供人工操作触摸板、鼠标、键盘及远程桌面后回传日志。
直接目标是分析“树持有焦点、指针位于表格，横向触摸板操作却改变树”的反馈；
采集前不将该现象归因于某类设备，也不增加按键转滚动、移动焦点等补偿。

观察代码与界面留在例程，框架仅补充所在原生绑定包中的必要 API。
不新增 GUI 调试入口，不修改事件循环、输入转换、命中或分发规则。

## 原生观察入口

| 平台 | 入口与记录范围 | 限制 |
| --- | --- | --- |
| Windows x64 | 当前 GUI 线程的 `WH_GETMESSAGE` 和 `WH_CALLWNDPROC`，记录全部消息编号、参数与常见输入字段；原样调用下一钩子 | 不记录安装前的创建消息；queue 与 wndproc 可重复；不是全局钩子或原始 HID 监听 |
| Linux/X11 | 首次 OpenGL 绘制时用 `glXGetCurrentDisplay` 取得实际 Xlib 连接，以 `XESetWireToEvent` 包装原转换器；记录 2..127 类事件头与已转换的常见输入字段 | 要求 OpenGL/GLX；不记录安装前已解码的事件；扩展变长载荷不解析；无 Wayland/XInput2 接入承诺 |
| macOS | `NSEvent.addLocalMonitorForEventsMatchingMask:handler:`，记录全部回调类型及键码、精确滚动、phase、momentumPhase 等适用字段，返回原事件 | 只覆盖 AppKit 交给 local monitor 的事件，不是 NSWindow 通知、全局事件或原始 HID |

Windows 的普通按键可能在 GOUI 消息队列处理中直接消费，不能只子类化窗口过程。
钩子限定当前线程，负 hook code 立即转发；仅在原生文档规定的回调期间复制 MSG／
CWPSTRUCT，不保留原生指针，也不读取任意消息参数指向的内存。输入来源查询失败、
device unavailable、origin injected 均按原值记录，不据此断言实际物理设备。

Xlib 官方不鼓励在正常应用中替换 core event decoder。本例仅将其作为临时诊断入口，
调用原转换器并返回原状态，不修改事件、选择掩码或队列，也不在回调中调用窗口和 GUI。
未注册转换器立即恢复；退出时恢复所有已包装转换器。取得连接依靠现有绘制回调和
GLX 的公开 API，避免为例程公开后端的私有 Display 或加入事件循环特例。

macOS 在 GUI 主线程安装／移除 monitor，block 在移除后释放。事件为借用对象，
仅同步读取属性，不保存 NSEvent；滚动与按键专属属性只对适用类型查询。

## 对照日志

日志使用单调经过时间、递增序号和以下类别：

- `START`／`ENV`：OS／架构、Go、设备说明、相关环境、原生窗口、请求后端和实际 PixelScale。
- `NATIVE`：各平台原生字段；未知类型保留编号，不凭名称猜测。
- `GOUI`：capture 阶段的 Wheel／Key／Pointer，含修饰键、单位、焦点与滚轮命中区域。
- `TREE`／`TABLE`／`FOCUS`：通过既有信号观察的状态变化。
- `STATE`：派发后采样的树状态、滚动偏移、范围和边界。
- `MARK`／`ERROR`／`END`：用户分段、安装错误与退出信息。

`GOUI.lastNative` 表示最近记录的原生序号，不是严格一对一因果关系。
Xlib 可批量预解码，Windows 可嵌套派发，因此同时保留原生时间与事件字段供分析。
不尝试跨系统统一原生增量、方向、单位、坐标原点或设备身份。

使用缓冲日志，每 250 ms、操作后的任务及标记时刷新。文件默认位于临时目录，
显式路径采用排他创建，防止覆盖旧记录；写入失败在退出时返回错误。
日志可能改变时序，不用于性能测量，也不包含其他应用的全局输入。

## 操作与验证

运行命令、中文操作步骤、逐项预期、平台限制、副作用、复位与退出方式以例程入口
注释为准。就绪前不能开始采集。分段标记按钮会取得焦点，每段滚动前应再次点击树。

逻辑测试验证日志顺序、原生引用、设备注释转义、旧文件保护和写入错误。
三目标交叉编译只验证构建；本机独立 X11 会话以真实事件路径验证观察器安装、
键盘／双轴滚轮、派发后状态、卸载与退出。触摸板和远程客户端需实际硬件采集，
不能以 XTEST、交叉编译或窗口启动代替。进度记录于 `Plan.md`。

## 原生契约参考

- [Microsoft SetWindowsHookExW](https://learn.microsoft.com/en-us/windows/win32/api/winuser/nf-winuser-setwindowshookexw)：线程范围、回调模块与 hook 类型。
- [Microsoft GetCurrentInputMessageSource](https://learn.microsoft.com/en-us/windows/win32/api/winuser/nf-winuser-getcurrentinputmessagesource)：当前输入来源，最低 Windows 8。
- [Xlib XESetWireToEvent](https://xorg.freedesktop.org/archive/current/doc/libX11/libX11/libX11.html#XESetWireToEvent)：wire-to-host callback、返回状态和替换 core decoder 的限制。
- [GLX glXGetCurrentDisplay](https://registry.khronos.org/OpenGL-Refpages/gl2.1/xhtml/glXGetCurrentDisplay.xml)：调用线程当前上下文对应的 Display。
- [Apple NSEvent local monitor](https://developer.apple.com/documentation/appkit/nsevent/addlocalmonitorforevents(matching:handler:))：进程内观察与返回原事件。

参考用于核对诊断入口，不扩展 GOUI 的正式触摸板或手势 API。
