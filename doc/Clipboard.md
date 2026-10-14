# GOUI Clipboard 设计

> 状态（2026-10-14 复核）：纯文本三平台后端及 TextInput／TextView 集成已实现；GUI 错误反馈迁移尚未实现。下文描述现有 v1，不把功能外的候选方向当作已排期任务。
> 适用范围：`platform/`（三后端）与 `gui/`
> 相关文档：`doc/PlatformDev.md`（平台层结构与约定）、`doc/InputTODO.md`

---

## 1. 定位与范围

- **v1**：系统剪贴板（X11 的 `CLIPBOARD` selection / Win32 clipboard / macOS general pasteboard）、**纯 UTF-8 文本**、复制 + 粘贴。
- **当前不支持**：富格式 / 图片 / 文件、X11 `PRIMARY` selection（中键粘贴）、剪贴板变更通知、Win32 延迟渲染（`WM_RENDERFORMAT`）、X11 INCR 大数据分块。错误反馈与 INCR 的后续状态见 [未来路线](Roadmap.md#future-clipboard)，其余能力按实际需求评估。
- **原则**：三后端各自使用 **OS 原生剪贴板机制**；不引入新的运行时依赖。

## 2. 架构落点

- **`Platform.NewClipboard() (common.Clipboard, error)`**：与 `NewSettings/NewWindow` 一致用 `New` 前缀表创建语义。线程亲和，须在平台线程上使用。多次调用安全：
  - **cocoa / win32**：clipboard 是无状态结构（不持有资源，直接打到全局 OS 剪贴板），每次返回一个等价实例即可，无需缓存。
  - **x11**：clipboard 持有 selection 状态且需被 `handleEvent` 路由，故为单例，存于 `platform.clipboard`。
- 上层持有单个实例。**gui 层暴露**:`gui.Clipboard` 是 gui **自有接口**(`SetText(text string)` / `RequestText(cb)`,**隐藏 platform 的 error 返回**)+ 未导出 `clipboard` 具体(包装 `platform.Clipboard`、nil 时降级为 no-op/空读),经 `App.Clipboard()` 取得——**不再是** `= platform.Clipboard` 别名。ui 层再包一层线程亲和的 `ui.Clipboard`(见 `doc/DesignApp.md`)。

## 3. 接口

```go
// platform/common/clipboard.go
type Clipboard interface {
    // SetText 用 UTF-8 文本替换剪贴板内容。同步取得所有权；X11 下进程需
    // 持续通过事件循环应答后续 selection 请求。
    SetText(text string) error

    // RequestText 请求剪贴板中的文本。callback 在平台线程上被调用恰好一次：
    // win32/cocoa 同步读到后立即回调；x11 在 SelectionNotify 到达后回调。
    // 调用方不应假设回调被延后。ok=false 表示无文本 / 转换失败。
    RequestText(callback func(text string, ok bool))
}
```

**回调时序契约**：回调只触发一次、在平台线程上；**可能在 `RequestText` 返回前（win32/cocoa）或之后（x11）触发**。这是 v1 的务实取舍——避免让 Clipboard 持有 EventLoop 引用；将来若需严格"始终延后"，可用各后端的 loop-post 机制实现。

对 gui 的粘贴用法无影响：Ctrl+V → 记住当前 caret → `RequestText(cb)` → cb 里在该 caret 处插入。

## 4. 选型：为什么用 OS 原生而非 GTK/GDK

判定用不用 GTK 看两根轴：**能否优雅降级** + **集成有多深**。Settings/主题/缩放走 GTK 合理（缺了能兜底默认值、且只是同步读属性）；**剪贴板两轴都危险**——是核心功能（没它就完全没有复制粘贴，无从兜底），且 GTK/GDK 路径必须把 GLib 主循环并进核心 EventLoop（深度耦合）。

X11 上只有两种形态：自己实现 selection 协议（Xlib/xcb/`gdk_selection` 等价的底层活），或套工具箱封装（`gtk_clipboard`，付出 GTK 硬依赖 + GLib 主循环）。GDK3 是被支配项——便利度和代码量≈原生 Xlib，却白背 GTK 栈依赖 + GLib 循环。

选 **原生 X11 selection**：零新增依赖（`libX11` 已依赖）、不碰 EventLoop、与 win32/cocoa "各用 OS 原生剪贴板" 一致、无 GTK 也能工作。

## 5. 三后端实现

统一：`SetText` 同步取得所有权；`RequestText` 数据就绪即回调。

### 5.1 x11（原生 selection）

- **owner 窗口复用 `platform.helper`**：使用进程级、长生命周期的隐藏窗口；selection 事件经平台事件循环正常路由，不能当作唤醒事件过滤。
- **atoms 进 `platform.atoms`**：`CLIPBOARD/TARGETS/GOUI_CLIPBOARD` 与既有 atoms 一样在 `newPlatform` 里 intern；`UTF8_STRING` 复用已有的。
- **SetText**：把文本存进 clipboard 缓冲；`XSetSelectionOwner(CLIPBOARD, helper, CurrentTime)`（v1 用 `CurrentTime`，未跟踪触发事件的服务器时间戳）。
- **别人粘贴（我们是 owner）**：收到 `SelectionRequest` → target 是 `TARGETS` 回 `[TARGETS, UTF8_STRING, STRING]`；是 `UTF8_STRING`/`STRING` 则 `XChangeProperty` 写文本 → 发 `SelectionNotify`。
- **RequestText（我们粘贴）**：`XConvertSelection(CLIPBOARD, UTF8_STRING, GOUI_CLIPBOARD, helper, CurrentTime)` → 等 `SelectionNotify` → `GetWindowPropertyBytes` 读文本 → 回调 → `XDeleteProperty`。
- **失去所有权**：`SelectionClear` → 丢弃缓冲。
- **事件路由**：selection 事件由现有 `handleEvent` 按类型派发给 `platform.clipboard`（留在既有事件循环，不引 GLib）。
- xlib 已有 selection 和 property 相关绑定；调用与路由位于 `platform/linux/x11/clipboard_linux.go`。

### 5.2 win32（原生 clipboard）

- **SetText**：`OpenClipboard(helperHwnd)` → `EmptyClipboard` → `GlobalAlloc(GMEM_MOVEABLE)` + `GlobalLock` 写入 **UTF-16**（`CF_UNICODETEXT`）→ `SetClipboardData` → `CloseClipboard`。
- **RequestText**：`OpenClipboard` → `GetClipboardData(CF_UNICODETEXT)` → `GlobalLock` 读 UTF-16 → 转 UTF-8 → `CloseClipboard` → 同步回调。
- winapi 已有上述绑定，适配位于 `platform/windows/win32/clipboard_windows.go`。

### 5.3 cocoa（NSPasteboard）

- **SetText**：`NSPasteboard.generalPasteboard` → `clearContents` → `setString:forType:NSPasteboardTypeString`。
- **RequestText**：`stringForType:NSPasteboardTypeString` → 同步回调（无则 ok=false）。
- appkit 已有上述绑定，适配位于 `platform/darwin/cocoa/clipboard_darwin.go`。

## 6. 需如实上报的平台事实

- **X11 剪贴板数据在拥有者进程退出后丢失**（除非有 clipboard manager；后续可加 manager 握手缓解）。win32/cocoa 由系统留存。
- X11 剪贴板 **不支持 INCR**：超过服务器单次属性传输上限的大文本不保证传输成功；上限由服务器决定，不承诺固定 256 KiB。DnD 的 INCR 实现不等于剪贴板已具备该能力。

## 7. GUI 集成

- gui `application` 暴露 `Clipboard()` 单例访问器。
- TextInput／TextView 的共享 `textEditor` 已接入 copy/cut/paste、编辑菜单和快捷键，使用 GUI 的 Primary 修饰键语义；只读、禁用和异步粘贴生命周期防护复用编辑核心。见 [文本编辑](DesignTextEditing.md)。

## 8. 验证

- 三 GOOS 交叉编译：`GOOS={windows,darwin,linux} go build ./...`。
- 运行期需真实桌面 + 另一个应用做复制/粘贴往返验证（跨进程 selection/pasteboard）。
