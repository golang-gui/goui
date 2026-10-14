# Linux 原生开发规范

遵循[平台通用规范](../AGENTS.md)。本文件也适用于目录外调用 Linux 原生库的绘制、排版和测试代码。

## 绑定组织

- 动态库及符号统一放在 `libs` 的现有包中，沿用 `cgo.NewLazyLibrary`、`NewSymbol` 及所在包的调用形式；`x11` 不另建动态库绑定。
- 按原生库/类型扩展既有 `api_linux.go`、`type_linux.go` 等文件；不要为 WindowChrome、定位或拖放新建一套平行绑定。确需拆文件时保持包内组织一致。
- GLib、GObject、GIO 相关绑定集中在 `libs/glib`，不另立 GIO/URI 绑定入口。
- 原生输出、错误及释放语义留在绑定；属性含义、窗口策略与事件解释留在 `x11`。原生内存使用对应的 XFree、g_free 等释放，不与 Go/C 分配器混用。
- Cairo、Pango、GLX、Xlib、libc 等核心库是运行前提，缺失库或必需符号时允许 panic，不增加探测后的静默降级。可选扩展和 X server 能力仍按各自契约查询，不能把服务端不支持误判为缺失核心库。
- 结构体按值参数交给 `cgo.Call` / `CallRet`，不手工模拟寄存器布局。C `int` 返回值以 `int32` 接收后再转 Go `int`，C `int*` 输出使用 `int32` 存储；原生指针、long、size_t 按其真实宽度处理。不要因统一写法批量替换正确的整数/句柄调用或内核 syscall。

## X11 标识与协议

- 跨窗口共用的固定协议 atom 由 `x11.Platform.atoms` 初始化、持有，业务方法直接引用，不反复 `InternAtom`。
- 功能对象专属的固定 atom 可以随该对象初始化；运行时 MIME 等动态名称按实际需求解析，不强行塞进平台固定表。按 screen 生成的标识必须保留其 screen 归属。
- 初始化固定协议标识通常使用 `onlyIfExists=false`，避免把启动时不存在永久缓存成零。确需探测名称存在性时注明理由；不要把动态能力查询改成启动时快照。
- atom 存在不代表属性存在、WM 支持功能或 compositor 正在运行。能力、属性、selection owner 按各自契约查询，不能用 atom 非零代替。
- 读取属性检查实际 type、format、项数、剩余数据和长度上限；处理 Xlib 的 format=32 与宿主 `long` 存储宽度差别，不直接假设返回的是紧密 `uint32` 数组。
- 区分 root/client/frame 坐标、原生像素和 DIP；发送 ClientMessage 时核对目标、事件掩码、字段宽度及协议来源标记。
- 不为了“立即生效”随意添加 `XSync`、sleep、重试或强制重绘。flush、同步、WM 异步结果分别按实际协议需求处理；缺失事实应按契约返回不可用。

## 写法参考与验收

- 平台固定 atom：[platform_linux.go](x11/platform_linux.go)。
- Xlib 函数绑定：[api_linux.go](libs/xlib/api_linux.go)；原生类型：[type_linux.go](libs/xlib/type_linux.go)。
- GLib/GIO 与内存处理：[api_linux.go](libs/glib/api_linux.go)。这些是组织参考，不保证每个历史调用都是正确模板。
- 逻辑/ABI 与真实 X11 行为分开验证。桌面验收记录 X11、桌面环境/WM、实际后端、缩放及 compositor 等相关条件；无 DISPLAY 的跳过不算通过，也不能把一个 WM 的结果外推到全部 Linux 环境。
