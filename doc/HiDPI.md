# GOUI HiDPI 设计

> 状态：跨平台 HiDPI（高分屏 / 缩放）支持的坐标契约与落地方案
> 适用范围：`platform/`（三后端 + 三 painter）与 `gui/`
> 相关文档：`doc/PlatformDev.md`（平台层结构与约定）

---

## 1. 目标与非目标

**目标**：在 Retina / HiDPI / 分数缩放屏幕上，让 GUI 以逻辑坐标（DIP）布局、以物理像素分辨率渲染，文字与图形都清晰，输入命中与布局在同一坐标空间。

**非目标（本轮不做，留 TODO）**：
- 边框 / 细线的布局取整（仿 WPF `UseLayoutRounding`）；文字已用像素 snap 解决（见 `platform/graphics/{opengl,software}/painter.go` 的 `DrawTextLayout`）。
- x11 的 RandR 物理尺寸推导、动态 scale 变化监听；x11 `Platform.Destroy()` 收尾。

> `Window.ScaleFactor()` 已随本轮删除：scale 折进 `SizeEvent`（`PixelWidth/Width`）后成为单一事实源，公开的 per-window 查询不再需要。win32 保留内部小写 `scaleFactor()` 用于把物理坐标归一化为逻辑。

---

## 2. 一个统一规律：渲染目标的原生单位

| | 渲染目标默认单位 | 含义 |
|---|---|---|
| **win32 / x11** | **物理像素**（HDC / X drawable / GL drawable / client rect 都是像素） | 原生即物理，无需额外操作 |
| **cocoa** | **逻辑点**（NSView 的 CGContext、默认 NSOpenGL 表面按 backing 自动缩放） | 必须**显式切到物理**才能拿到 Retina 分辨率 |

因此各平台的工作量天然不同：win32/x11 主要是"坐标单位归一化"，cocoa 额外需要"把表面切到物理分辨率"。

---

## 3. 坐标契约（核心）

三个 painter 的 `Begin` 契约一致且已正确：

```
Begin(物理宽, 物理高, scale)   // 绘制坐标是逻辑的
```

- software / opengl：逐图元手动 ×scale（opengl 经 nanovg `BeginFrame(逻辑, scale)`）。
- direct2d：`Resize(物理)` + `SetDpi(96*scale)`，由 D2D 内部 ×scale。

HiDPI 的本质不是改 painter，而是**统一喂进来的坐标单位**。约定：

- **gui 世界统一用逻辑（DIP）**：布局（Measure/Arrange）、命中测试、widget 矩形全部逻辑坐标。gui 保持平台无关、基本 scale 无关。
- **平台边界负责归一化**：把原生 size / pointer 转成逻辑再上报；物理像素只在需要精确表面时提供。
- **scale 折进 SizeEvent**，不再有独立的 `ScaleEvent`：

```go
// platform/events/window_event.go
type SizeEvent struct {
    Width, Height           float32  // 逻辑 DIP —— 布局/命中测试
    PixelWidth, PixelHeight float32  // 物理像素 —— painter.Begin 表面（精确）
}
```

- gui.paint：`Begin(PixelWidth, PixelHeight, PixelWidth/Width)`。物理尺寸由平台**直接精确给出**，scale 由 `Pixel/Logical` 派生 —— 无二次取整，表面与坐标完全对齐（分数 scale 也精确）。
- Pointer / Wheel：平台内部 ÷自己的 scale 后上报**逻辑坐标**（与 widget 矩形同空间，gui 直接命中测试）。
- **不变量**：每条 scale 变化路径都必须发一个 `SizeEvent`（scale 变则物理尺寸必变），所以把 scale 折进 SizeEvent 不丢事件语义。

各平台如何填 SizeEvent 的四个字段：

| 平台 | 物理（Pixel*）来源 | 逻辑（Width/Height）来源 |
|---|---|---|
| cocoa | `view.ConvertRectToBacking(frame)` | `view.Frame()`（点，现成） |
| win32 | client rect（`WM_SIZE` lParam / `GetClientRect`） | 物理 × 96 / `GetDpiForWindow` |
| x11 | `ConfigureNotify` 的 width/height | 物理 / 缓存的全局 scale（见 §5） |

物理值总是取原生精确值；逻辑值在 win32/x11 上由物理派生（可为分数，用 float32 保精度）。

---

## 4. 已落地阶段与当前限制

### Phase 0 —— 共享原子改动（三 GOOS 必须一起改到能编译）

改 `SizeEvent` 结构 + 删 `ScaleEvent` 会同时打断 events / gui / 三后端，因此这是不可分割的一步。

- `platform/events/event.go`：删 `Scale` 枚举常量（iota 重排；dispatch 按具体类型 switch，不依赖枚举值，安全）。
- `platform/events/window_event.go`：`SizeEvent` 改 4×`float32`；删 `ScaleEvent` 类型与 `_ Event = ScaleEvent{}` 断言。
- `gui/window.go`：
  - `width/height` 由 `uint` 改 `float32`，新增 `pixelWidth/pixelHeight float32`；删 `scale` 字段（改 paint 时派生）。
  - `DispatchEvent` 的 `SizeEvent` 分支存逻辑+物理；删 `ScaleEvent` 分支。
  - `paint`：`Begin(pixelWidth, pixelHeight, pixelWidth/width)`（`width==0` 时 scale 取 1 兜底）。
- 三后端 SizeEvent 发射点适配（详见 §3 表）：cocoa 合并 Scale/Size 为一个 SizeEvent 并删两处 ScaleEvent；win32 `WM_SIZE` 补逻辑、`WM_DPICHANGED` 删 ScaleEvent 保留 resize；x11 `ConfigureNotify` 按当前全局 scale 换算逻辑尺寸（见 Phase 3）。

### Phase 1 —— win32 正确性

- `input_windows.go`：pointer / wheel 坐标 ÷scale 归一化为逻辑。
- 核对创建时首个 `WM_SIZE` 已带正确 dpi（自然覆盖初始 scale）。
- direct2d / GL drawable / `PER_MONITOR_AWARE_V2`：已就绪，仅核对。

### Phase 2 —— cocoa 正确性

- appkit 绑定 `setWantsBestResolutionOpenGLSurface:`，建 GL context 时对 NSView 调用 → GL drawable 变 backing 物理像素。
- `window_darwin.go` `drawImage`：dest rect 改逻辑点尺寸（`view.Frame()`），物理位图经 CGContext ×backing 落到 1:1。

### Phase 3 —— x11 缩放（当前实现）

X11 使用进程内缓存的全局 scale：首次查询后固定，窗口创建、ConfigureNotify、
pointer／wheel 和 painter 共享该值，不再提供已删除的 Window.ScaleFactor。

## 5. x11 scale 检测策略

当前优先级以 [scale_linux.go](../platform/linux/x11/scale_linux.go) 为准：

1. 有效 GOUI_PLAT_SCALE 显式覆盖；
2. 有效 GDK_SCALE；
3. GTK Settings 的 gtk-xft-dpi（DPI × 1024），换算为 DPI／96；
4. 缺失时使用 X11 96 DPI 对应的 1x 基线。

当前没有 gdk_monitor_get_scale_factor 查询或动态缩放监听；Cursor 的主题订阅
也不能当作窗口 scale 已动态更新。动态缩放／跨显示器与更细的像素布局策略
属于 [未来路线](Roadmap.md#future-settings-hidpi)，实施时必须同步尺寸和输入事实。

## 6. 验证方式

- 三 GOOS 交叉编译：`GOOS={windows,darwin,linux} go build ./...`。
- 运行期用临时 `[goui-dbg]` 日志核对 `scale / 物理尺寸 / drawable` 三者对齐（定位后清除）。
- 重点在 2× 屏上验证文字清晰度（1× 屏因像素密度低本就偏软，不足以判断）。
