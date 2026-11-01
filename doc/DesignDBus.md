# Linux D-Bus 接入设计

状态：2026-11-01 确认选型与边界，首个切片已实现并完成本机独立 X11 会话验收。
首个切片只实现
Portal 文件选择，Settings、输入法、通知、托盘及 Wayland 均不随本次修改。
文件选择的公共契约见 [DesignFileDialog](DesignFileDialog.md)，访问对象与窗口
模态见 [DesignFileAccess](DesignFileAccess.md)。

## 目标与选型

D-Bus 是 Linux 平台内部访问桌面服务的通信能力，不是新的 GUI 事件系统。
采用纯 Go 的 `github.com/godbus/dbus/v5`；初次实现以已核对的 v5.2.4 为基线。
只在 Linux 实现中引用，不向 platform/common、gui 或 ui 暴露第三方类型。

约束如下：

- 保持 no-cgo 构建，不需要 GCC，不增加 libdbus 的动态库依赖。
- 不初始化 GTK，不驱动 GLib/GMainContext，不把第二套 GUI 事件循环接入主线程。
- 后台使用 Go 的连接、goroutine 和 channel 处理消息，再经现有平台事件循环投递。
- 连接当前桌面的 session bus，不启动私有总线来伪造可用的桌面环境。
- Portal 及其桌面后端是否存在、支持哪个接口，按实际能力查询，不能按发行版猜测。
- 首个功能只提供文件选择；不为了复用新增尚无消费者的通用服务管理器或公共 D-Bus API。

godbus 自行实现 D-Bus 协议。已核对 v5.2.4 的 go.mod 要求 Go 1.20，依赖
golang.org/x/sys；GOUI 当前已依赖后者。
参考：[godbus 文档](https://pkg.go.dev/github.com/godbus/dbus/v5)、
[v5.2.4 模块依赖](https://github.com/godbus/dbus/blob/v5.2.4/go.mod)。

## 其他方案与选择依据

| 方案 | 能力与代价 | 本次选择 |
| --- | --- | --- |
| GIO/GDBus | 可复用现有 libgio，GDBus 从 GLib 2.26 即可用；常规信号订阅在 thread-default main context 交付 | 不接入 GMainContext，暂不选择 |
| GIO message filter | 能从 GDBus 消息线程取得消息，但增加原生线程回调、GVariant 所有权和注销并发处理 | 技术可行，不作为首个接入方式 |
| godbus | Go 协议实现、channel 信号与并发连接，无需原生回调；增加二进制代码和连接资源 | 采用 |
| libdbus 动态绑定 | 可以自行处理消息或接入 poll，但需要维护 ABI、消息构造与解析、分发和资源管理 | 暂不选择 |

Portal 的方法返回请求句柄，最终选择通过 Request.Response 信号交付。因此仅把
GIO 同步调用移到 goroutine，不能替代完整的响应接收设计。
GDBus filter 是可行的替代，不把“未选择”描述为“不支持”。
参考：[GDBus 信号上下文](https://docs.gtk.org/gio/method.DBusConnection.signal_subscribe.html)、
[filter 的消息线程](https://docs.gtk.org/gio/method.DBusConnection.add_filter.html)、
[libdbus 分发](https://dbus.freedesktop.org/doc/api/html/group__DBusConnection.html)、
[Portal Request](https://flatpak.github.io/xdg-desktop-portal/docs/doc-org.freedesktop.portal.Request.html)。

## 分层、线程与生命周期

协议实现放在 `platform/linux/internal/portal`，直接使用 godbus；它属于桌面服务
适配，不伪装成 libs 中的原生 C 绑定，也不再包装一套通用 D-Bus Go API。
该包接受普通请求参数和 parent 标识，不依赖 Xlib、Window、Widget 或 GUI 策略。
X11 负责提取真实 XID、校验 owner 和协调窗口及事件循环的生命周期。

连接按需建立，由 Linux 平台内部明确持有；同一服务使用独立拥有的连接，不能关闭
godbus 的全局共享 SessionBus 连接。多个请求复用连接，不为每个窗口常驻一条总线。
将来新增服务可以复用已实现的连接能力，但不预设所有服务都能共用同一条连接。

网络连接、方法调用和信号接收不阻塞 GUI 线程。后台只使用已复制的普通 Go 数据，
不读取或操作窗口句柄。完成结果经 EventLoop.Post 返回所属平台线程，检查请求和
owner 有效性后交付一次。既有 GUI ModalScope 输入限制继续生效。

事件循环退出时先结束请求并关闭远端选择器，再释放连接与信号订阅。关闭过程不能
等待用户完成选择；连接断开时结束受影响请求，不自动重放已受理操作。
本地取消和迟到结果必须有明确的归属，不能依赖最终收到远端 Response 才能清理。

## 首个切片：Portal FileChooser

- 保留平台／GUI／UI 的现有文件选择 API 和 FileAccess 所有权。
- 支持 OpenFile、OpenDirectory、SaveFile，以及既有过滤器、初始目录与默认文件名。
- 有 owner 时传递 `x11:<十六进制 XID>` 和 modal=true；无 owner 时传空标识和
  modal=false。不通过猜测 helper 窗口或反复抢焦点补原生关系。
- 查询 FileChooser 能力；目录选择要求接口支持 directory 选项，不能忽略后选中普通文件。
- Portal 不可用或明确缺少所需能力时保留现有 helper 降级；不把用户取消、拒绝或
  不确定是否已经显示的请求超时当作重新弹出选择器的理由。
- 收到成功结果后校验并解析文件 URI，再在平台线程转换为既有 FileAccess。
  不把路径凭据升级解释为跨会话永久沙箱授权，不顺带承诺完整 Flatpak 支持。
- 真实父子关联、激活、模态与桌面行为须在实际 Portal 后端和窗口管理器上验证；
  参数受理不等于行为验收通过。

方法参数、响应订阅顺序、取消竞态、能力降级和测试安排见
[文件选择改造设计](DesignFileDialog.md#linux-portal-改造设计2026-11-01)。
参考：[FileChooser](https://flatpak.github.io/xdg-desktop-portal/docs/doc-org.freedesktop.portal.FileChooser.html)、
[父窗口标识](https://flatpak.github.io/xdg-desktop-portal/docs/window-identifiers.html)。

## 后续复用：需求触发，当前不实现

| 方向 | 可复用的协议能力 | 约束 |
| --- | --- | --- |
| 系统外观设置 | Portal Settings 的亮暗偏好、主题色等查询与 SettingChanged | 键和后端支持须查询；不能替代全部字体／DPI 设置，本轮不改既有 GTK 查询或轮询 |
| 系统通知 | org.freedesktop.Notifications 或适用的 Portal 通知接口 | 先明确发送、动作与生命周期契约，不扩大到推送服务 |
| 托盘 | StatusNotifierItem 与 watcher/host 协作 | 桌面没有托盘宿主时仍可能不支持；菜单等能力另行设计 |
| 打开链接／文件 | Portal OpenURI/OpenFile 等请求与结果 | 本轮保留已有 GIO 接入；不能把 file URI 直接传给仅支持非文件 URI 的方法 |
| 输入法 | IBus、Fcitx 的输入上下文、按键、预编辑与提交协议 | 当前 XIM 不动；两套协议、按键顺序和服务重启需专项设计 |
| 全局快捷键、阻止休眠、屏幕共享等 | 对应桌面服务或 Portal 请求／会话 | 按应用需求实现，不能把接口存在等同于所有桌面均支持 |

参考：[Settings](https://flatpak.github.io/xdg-desktop-portal/docs/doc-org.freedesktop.portal.Settings.html)、
[通知规范](https://specifications.freedesktop.org/notification/latest/)、
[StatusNotifierItem](https://github.com/KDE/kstatusnotifieritem/blob/master/src/org.kde.StatusNotifierItem.xml)、
[OpenURI](https://flatpak.github.io/xdg-desktop-portal/docs/doc-org.freedesktop.portal.OpenURI.html)、
[IBus](https://ibus.github.io/docs/ibus-1.5/IBusInputContext.html)、
[Fcitx5](https://github.com/fcitx/fcitx5/blob/master/src/frontend/dbusfrontend/dbusfrontend.cpp)。

## 与 Wayland 的关系

D-Bus/Portal 可以为未来的 Wayland 后端复用文件选择、系统设置及其他桌面服务。
Portal 的 parent 从 X11 的 XID 改为导出的 Wayland 窗口句柄，公共 GUI 契约不需要
因此改变。

Wayland 窗口创建、绘制提交、普通鼠标键盘、剪贴板与拖放仍使用 Wayland 协议，
不由 D-Bus 替代。Wayland 输入法需考虑 text-input 协议与合成器支持，不能直接
假定复用 X11 的 XIM 或某一套输入法 D-Bus 协议即可完成适配。
参考：[Wayland 协议](https://wayland.freedesktop.org/docs/book/Protocol.html)、
[text-input](https://github.com/wayland-mirror/wayland-protocols/blob/main/unstable/text-input/text-input-unstable-v3.xml)。

## 体积与资源评估

2026-11-01 在 Linux amd64、Go 1.25.8、godbus v5.2.4 下的临时接入探针，使用
CGO_ENABLED=0、-trimpath、-ldflags="-s -w"，结果如下：

| 项目 | 观测增量 |
| --- | --- |
| 独立小程序二进制 | 1.20 MiB |
| 当前 Demo 加共同测量入口的二进制 | 0.77 MiB，约 7.8% |
| Demo 已链接但未连接的 Go 活跃堆 | 约 4 KiB |
| Demo 连接、调用方法并订阅响应后的 Go 活跃堆／PSS | 约 75 KiB／1.23 MiB |

内存对照在私有测试总线上运行 9 次，采样前执行 GC，取中位数；Demo 在创建窗口前
进入测量入口，用于隔离客户端增量。未打开真实选择器，不含 Portal 服务和选择器
后端进程内存。未连接 PSS 增量约 0.1 MiB，接近运行波动，不作为固定成本承诺。
关闭连接后 goroutine 恢复，但代码页和运行时保留内存不立即回到基线。

这些是选型证据，不是正式实现的性能验收；实际接入后重新检查二进制、空闲及运行中
资源和请求清理。新增服务可复用协议代码，但仍有各自代码、订阅和对象开销，不是免费。

## 验证边界

协议与生命周期逻辑使用无桌面测试和受控 D-Bus 服务，覆盖快速响应、错误、断线、
取消和迟到结果；无需真实选择器时不启动桌面。真实 FileChooser、所属窗口激活与
模态验收放在现有 tests/window/gui/file_dialog，记录 Portal 后端、WM 和缩放。
功能不可用、helper 降级和 GUI 输入隔离分别断言，不把降级验证写成 Portal 验收。
