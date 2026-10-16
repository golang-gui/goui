# GOUI Clipboard 设计

> 状态（2026-10-16）：纯文本三平台后端、结果接口及 TextInput／TextView 集成已实现；本轮验证记录见 [通用文本扩展验收](PlanSourceViewExtensions.md)，尚未发布。
> 适用范围：`platform/`（三后端）与 `gui/`
> 相关文档：`doc/PlatformDev.md`（平台层结构与约定）、`doc/InputTODO.md`

---

## 1. 定位与范围

- **v1**：系统剪贴板（X11 的 `CLIPBOARD` selection / Win32 clipboard / macOS general pasteboard）、**纯 UTF-8 文本**、复制 + 粘贴。
- **当前不支持**：富格式 / 图片 / 文件、X11 `PRIMARY` selection（中键粘贴）、剪贴板变更通知、Win32 延迟渲染（`WM_RENDERFORMAT`）、X11 INCR 大数据分块。INCR 的后续方向见 [未来路线](Roadmap.md#future-clipboard)；错误反馈的当前契约如下，其余能力按实际需求评估。
- **原则**：三后端各自使用 **OS 原生剪贴板机制**；不引入新的运行时依赖。

## 2. 架构落点

- **`Platform.NewClipboard() (common.Clipboard, error)`**：与 `NewSettings/NewWindow` 一致用 `New` 前缀表创建语义。线程亲和，须在平台线程上使用。多次调用安全：
  - **cocoa**：不持有原生资源，直接访问 general pasteboard。
  - **win32**：每个实例借用所属 Platform 的隐藏窗口作为写入所有者，不自行销毁 HWND；平台销毁后返回不可用错误。
  - **x11**：clipboard 持有 selection 状态且需被 `handleEvent` 路由，故为单例，存于 `platform.clipboard`。
- 上层持有单个实例。gui.Clipboard 是自有接口和未导出平台适配器：SetText 返回 error，RequestText 回调交付 (string, error)，不可用服务返回错误。ui.Clipboard 通过 Sync 调度并保留结果，SetText 在应用已停止时返回 ErrAppStopped。RequestText 返回受理结果：ErrAppStopped 表示未受理且不回调；nil 表示已受理，实际读取结果由回调交付。

## 3. 接口

```go
// platform/common/clipboard.go
type Clipboard interface {
    // SetText 用 UTF-8 文本替换剪贴板内容。同步取得所有权；X11 下进程需
    // 持续通过事件循环应答后续 selection 请求。
    SetText(text string) error

    // RequestText 请求剪贴板中的文本。callback 在平台线程上被调用恰好一次：
    // win32/cocoa 同步读到后立即回调；x11 在回复、超时或事件循环退出后回调。
    // 调用方不应假设回调被延后。无文本/空文本为 ("", nil)，非 nil error 只表示读取失败。
    RequestText(callback func(text string, err error))
}
```

**回调时序契约**：回调只触发一次、在平台线程上；**可能在 `RequestText` 返回前（win32/cocoa）或之后（x11）触发**。X11 的读状态和期限由所属平台的既有事件循环管理，不在后台 goroutine 调用 Xlib。退出前在平台线程清理待处理读取并交付 common.ErrUnavailable；已经进入退出状态的新读取同步报不可用。

对 gui 的粘贴用法无影响：Ctrl+V → 记住模型、revision、选区和挂载代次 → RequestText(cb) → 无错误、文本非空且身份仍有效时插入。没有可用文本格式和空文本均返回 ("", nil)，粘贴直接无操作，不删除选区，也不增加历史项。需要显式删除仍使用编辑操作。

## 4. 选型：为什么用 OS 原生而非 GTK/GDK

判定用不用 GTK 看两根轴：**能否优雅降级** + **集成有多深**。Settings/主题/缩放走 GTK 合理（缺了能兜底默认值、且只是同步读属性）；**剪贴板两轴都危险**——是核心功能（没它就完全没有复制粘贴，无从兜底），且 GTK/GDK 路径必须把 GLib 主循环并进核心 EventLoop（深度耦合）。

X11 上只有两种形态：自己实现 selection 协议（Xlib/xcb/`gdk_selection` 等价的底层活），或套工具箱封装（`gtk_clipboard`，付出 GTK 硬依赖 + GLib 主循环）。GDK3 是被支配项——便利度和代码量≈原生 Xlib，却白背 GTK 栈依赖 + GLib 循环。

选 **原生 X11 selection**：零新增依赖（`libX11` 已依赖）、复用既有 EventLoop、与 win32/cocoa "各用 OS 原生剪贴板" 一致、无 GTK 也能工作。

## 5. 三后端实现

统一：`SetText` 同步取得所有权；`RequestText` 数据就绪即回调。

### 5.1 x11（原生 selection）

- **owner 窗口复用 `platform.helper`**：使用进程级、长生命周期的隐藏窗口；selection 事件经平台事件循环正常路由，不能当作唤醒事件过滤。
- **atoms 进 `platform.atoms`**：`CLIPBOARD/TARGETS/GOUI_CLIPBOARD` 与既有 atoms 一样在 `newPlatform` 里 intern；`UTF8_STRING` 复用已有的。
- **SetText**：把文本存进 clipboard 缓冲；`XSetSelectionOwner(CLIPBOARD, helper, CurrentTime)`（v1 用 `CurrentTime`，未跟踪触发事件的服务器时间戳）。
- **别人粘贴（我们是 owner）**：收到 `SelectionRequest` → target 是 `TARGETS` 回 `[TARGETS, UTF8_STRING, STRING]`；是 `UTF8_STRING`/`STRING` 则以对应编码 `XChangeProperty` 写文本；不能表示为 Latin-1 时不声明/接受 STRING → 发 `SelectionNotify`。
- **RequestText（我们粘贴）**：必须已有未退出的事件循环。每次外部读取创建独立隐藏 requestor，复用 GOUI_CLIPBOARD 属性；先请求 TARGETS，优先 UTF8_STRING，其次 STRING。旧 owner 拒绝 TARGETS 时按这两种编码回退。STRING 使用 Latin-1，不能把 UTF-8 字节当作 STRING。
- **回复验证**：同时匹配 requestor、selection、target 和属性；读取原生 property 的 type/format/count/bytesAfter，限制为 16 MiB，检查 UTF-8，INCR、截断、非法格式和已声明格式的转换失败明确报错。超时/取消后的旧 requestor 回复不能完成新请求。不为每次读取创建永久 atom。
- **期限与退出**：整个读取（含协商）五秒；poll 使用最近读取期限，事件分发期间也检查过期；不响应时回调 ErrClipboardTimeout。清理临时窗口和 pending 后才回调，允许重入。循环退出/销毁或平台销毁时完成未结读取，回调 ErrUnavailable。期限依赖平台线程继续泵送，不声称能打断阻塞的应用回调。
- **失去所有权**：`SelectionClear` → 丢弃缓冲。
- **事件路由**：selection 事件由现有 `handleEvent` 按类型派发给 `platform.clipboard`（留在既有事件循环，不引 GLib）。
- xlib 已有 selection 和 property 相关绑定；调用与路由位于 `platform/linux/x11/clipboard_linux.go`。

### 5.2 win32（原生 clipboard）

- **SetText**：`OpenClipboard(helperHwnd)` → `EmptyClipboard` → `GlobalAlloc(GMEM_MOVEABLE)` + `GlobalLock` 写入 **UTF-16**（`CF_UNICODETEXT`）→ `SetClipboardData` → `CloseClipboard`。
- **RequestText**：`OpenClipboard` → `GetClipboardData(CF_UNICODETEXT)` → `GlobalLock` 读 UTF-16 → 转 UTF-8 → `CloseClipboard` → 同步回调。
- winapi 已有上述绑定，适配位于 `platform/windows/win32/clipboard_windows.go`。

### 5.3 cocoa（NSPasteboard）

- **SetText**：`NSPasteboard.generalPasteboard` → `clearContents` → `setString:forType:NSPasteboardTypeString`。
- **RequestText**：`stringForType:NSPasteboardTypeString` → 同步回调（无文本或空文本均为 "", nil）。
- appkit 已有上述绑定，适配位于 `platform/darwin/cocoa/clipboard_darwin.go`。

## 6. 需如实上报的平台事实

- **X11 剪贴板数据在拥有者进程退出后丢失**（除非有 clipboard manager；后续可加 manager 握手缓解）。win32/cocoa 由系统留存。
- X11 剪贴板 **不支持 INCR**：超过服务器单次属性传输上限的大文本不保证传输成功；上限由服务器决定，不承诺固定 256 KiB。DnD 的 INCR 实现不等于剪贴板已具备该能力。

## 7. GUI 集成

- gui `application` 暴露 `Clipboard()` 单例访问器。
- TextInput／TextView 的共享 `textEditor` 已接入 copy/cut/paste、编辑菜单和快捷键，使用 GUI 的 Primary 修饰键语义；只读、禁用和异步粘贴生命周期防护复用编辑核心。见 [文本编辑](DesignTextEditing.md)。

## 8. 验证与调用迁移

- 不依赖桌面的 GUI/UI 回归覆盖结果保留、空字符串、Cut 失败不删除、停止后未受理、已受理同步回调中 Quit、退出丢弃排队请求。
- X11 状态回归覆盖期限、一次完成、回调重入、迟到/无关回复及循环销毁。
- 原生显式程序见 `tests/window/platform/clipboard`；X11 外部 owner 在其 `owner/` 子目录。入口注释列明启动、预期与剪贴板副作用，不由 `go test` 自动启动。
- 三端原生 Tab/剪贴板、完整测试的通过与既有失败分别见 [验收记录](PlanSourceViewExtensions.md)。交叉编译不能代替原生执行。
- 既有 TextInput/TextView、声明式 demo、测试替身均迁移结果接口。示例只有写入成功才报告复制成功，UI 读取同时处理未受理错误与实际回调错误。
- X11 测试不能把“退出前写回原文本”当作恢复：所有者进程退出仍可能丢失内容。使用独立 X server；Windows 使用隔离 Session 0；macOS 原生写入测试需快照并恢复所有 pasteboard item/type/data。

## 9. 官方契约依据

- [OpenClipboard](https://learn.microsoft.com/en-us/windows/win32/api/winuser/nf-winuser-openclipboard)：NULL HWND 与 EmptyClipboard 的组合不能为 SetClipboardData 提供有效所有者。本实现借用平台 helper HWND。
- [X11 selection/property 协议](https://www.x.org/releases/X11R7.7/doc/xproto/x11protocol.html)：SelectionNotify 对应的 requestor/selection/target 与 property 的类型、格式及剩余数据必须验证；五秒期限、16 MiB 上限是 GOUI 策略，非 X11 保证。
- X11 INCR、PRIMARY、clipboard manager 持久化与富格式仍不在本轮范围。超时与错误反馈已实现，不再把不响应 owner 当作无限等待。
