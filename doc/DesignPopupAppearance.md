# Popup 平台圆角与原生阴影设计

> 状态：历史方案，已被新的实用化路线取代，不再作为实现依据。
>
> 当前方案：三平台 Window／Popup 透明绘制已实现，GUI PopoverMenu 消费样式绘制圆角／阴影。Menu／DropDown 保留原生 Popover；普通 Overlay 独立使用。显式输入区域已放弃，见 [透明绘制](DesignTransparency.md)、[菜单外观](DesignPopoverMenuAppearance.md) 与 [未来路线](Roadmap.md#withdrawn-proposals)。
>
> 下文保留历史设计供查阅：PopupAppearance、Settings.PopupCornerRadius、固定半径
> 上报、Linux 二值视觉 Shape、窗口失败修改全局半径缓存等均不实施。原生 Popup
> 圆角/阴影已通过透明表面与 GUI 样式实现，不恢复本文的原生半径查询方案。
>
> 以下目标、API、验证与路线均为已撤销提案的原始记录，不是现行待办。

## 1. 目标

Popup 默认仍是矩形原生表面，并可选择开启**平台圆角**。应用不向平台传入任意
圆角半径；具体半径和实现机制由平台后端决定，再通过 `platform.Settings` 报告
给上层，使 GUI 背景、边框和原生窗口边缘使用同一个实际值。

本切片同时保留原生阴影开关：阴影默认开启，但可由 Popover 或 PopoverMenu
关闭。圆角和阴影不能改变既有的 owner、无焦点、事件分发、Show/Hide、modal
和生命周期语义。

明确不做：

- 不支持调用方指定任意 Popup 圆角半径；
- 不为 Windows 引入 DirectComposition、layered window 或逐像素 alpha 重构；
- 不给 Popup 增加背景色、padding、阴影占位或通用样式对象；
- 不修改 `ui`，不实现顶层窗口自定义标题栏。

## 2. 公共 API

### 2.1 platform

```go
type PopupAppearance struct {
    Rounded bool
    Shadow  bool
}

type Settings interface {
    ColorScheme() (ColorScheme, error)
    AccentColor() (color.Color, error)
    FontFamily() (string, error)
    FontSize() (float32, error)

    // PopupCornerRadius reports the logical corner radius that this backend
    // actually uses when PopupAppearance.Rounded is true. Zero means square.
    PopupCornerRadius() (float32, error)
}

type Platform interface {
    NewPopup(
        owner Window,
        width, height float32,
        appearance PopupAppearance,
        handler EventHandler,
    ) (Popup, error)
}
```

`PopupAppearance` 是创建参数，不进入 Painter。`Popup` 接口本身不增加半径 setter
或查询方法；动态修改由 `gui.Popover` 重建其原生 Popup 和 Painter 完成。

`PopupCornerRadius` 返回逻辑坐标值。`0, nil` 是平台明确选择矩形，不是错误；
原生查询失败才返回错误。平台后端创建 Popup 时必须使用与该查询相同的内部
解析结果，不能出现“Settings 报告一个半径，原生窗口采用另一个半径”。

### 2.2 gui

```go
type PopoverAppearance struct {
    Rounded bool
    Shadow  bool
}

type Popover interface {
    // ...existing methods...
    Appearance() PopoverAppearance
    SetAppearance(PopoverAppearance)
}

type Settings interface {
    // ...existing methods...
    PopupCornerRadius() float32
}
```

`gui.Settings.PopupCornerRadius` 沿用 GUI 设置层的既有策略：平台查询成功就返回
平台值，失败或值非法时回退为 `0`。GUI 不猜测另一个半径。

默认值：

- `NewPopover`：`Rounded=false`、`Shadow=true`；
- `NewPopoverMenu`：`Rounded=true`、`Shadow=true`；
- `PopoverMenu` 提供 `Appearance()`/`SetAppearance()`，允许显式关闭圆角或阴影。

Popover 隐藏时修改 Appearance，旧原生资源立即释放并在下次 Show 惰性重建；
可见时修改则原位重建 Popup 和 Painter，保持位置、尺寸、可见与 modal 状态，
不发送 `Closed`，也不短暂注销 modal target。

## 3. 半径与样式契约

平台圆角是后端指标，不是稳定的跨平台常量。首版采用：

| 平台 | Rounded=true 时的初始逻辑半径 | 不支持时 |
| --- | ---: | --- |
| Windows 11 build 22000+ | 4 | 0，矩形 |
| macOS | 6 | 原生初始化失败则创建失败 |
| Linux/X11 | 6（X Shape 可用） | 0，矩形 |

这些数值以后可随真机视觉验证在各后端独立调整；上层必须查询 Settings，不能
硬编码 4 或 6。

PopoverMenu 开启 Rounded 时，外层背景和边框强制使用
`App.Settings().PopupCornerRadius()`；关闭时强制使用 0。平台半径覆盖 `menu`
样式中的 `Radius`，默认菜单规则移除当前硬编码的 `.Radius(4)`。其他控件的
Radius 语义不变。

菜单背景先填满整个 Popup bounds，再由原生窗口形状裁剪；边框使用同一个平台
半径绘制。这样深色菜单不会在抗锯齿边缘露出 `rootBase` 当前的白色 clear。
通用 Popover 不自动绘制背景，自定义内容可从 GUI Settings 查询半径并据此绘制。

## 4. 平台实现

### 4.1 Windows

Windows 11 build 22000 起使用
`DwmSetWindowAttribute(DWMWA_WINDOW_CORNER_PREFERENCE, DWMWCP_ROUNDSMALL)`。
该路径提供系统抗锯齿、系统 DPI 缩放，并保留普通 HWND/Direct2D/DirectWrite
呈现模型。Microsoft 也明确将 `DWMWCP_ROUNDSMALL` 用于自绘菜单。

Windows 10、DWM 属性不可用或实际设置失败时，Rounded 的解析结果为 0，Popup
与菜单都保持矩形。不使用 `SetWindowRgn`，避免 1x 下的二值锯齿；也不使用
layered window，避免破坏当前 swap chain 与 DirectWrite 路径。

平台注册两套独立 Popup window class：阴影 class 带 `CS_DROPSHADOW`，无阴影
class 不带。系统关闭窗口阴影时，`Shadow=true` 仍服从系统最终决定。

DWM 圆角路径在已知透明角区的 `WM_NCHITTEST` 返回 `HTTRANSPARENT`，使点击继续
落到同线程的 owner 窗口。resize、DPI 变化和重复 Show 后重新确认圆角状态。
若实际 Popup 创建时 DWM 设置失败，平台在首次绘制前把缓存指标降为 0，使随后
的 Settings 查询和 GUI 绘制同步退化为矩形。

### 4.2 macOS

Borderless `NSWindow` 设置为非 opaque、透明背景；content view 启用 CALayer，
使用 `cornerRadius=6` 与 `masksToBounds=true` 裁剪内容。OpenGL Popup surface
请求 alpha channel 并设置为非 opaque，Software Draw 继续复用同一 view mask。

`NSWindow.hasShadow` 实现阴影开关；尺寸或圆角状态改变后调用
`invalidateShadow`。content view hit-test 排除透明角区。若 AppKit 只能阻止
Popup 内容收到角区事件、却不能把事件继续交给 owner，则记录为平台兼容限制，
不得合成或转发 GUI 语义事件。

### 4.3 Linux/X11

增加 X Shape 扩展的动态绑定与能力探测。扩展可用时 Settings 报告 6，Popup
按当前物理尺寸生成 1-bit 圆角 mask，并同时应用到 Bounding、Clip 和 Input
shape；resize、缩放变化和 Show 后重建 mask。扩展不可用或应用 shape 失败时，
缓存指标降为 0，并使用视觉和输入都为矩形的确定性 fallback。

Popup 设置 `WM_TRANSIENT_FOR` 和 `_NET_WM_WINDOW_TYPE_POPUP_MENU`，由当前
compositor 决定是否及如何绘制系统阴影。`Shadow=false` 额外设置 picom/Compton
识别的 `_COMPTON_SHADOW=0`；其他 compositor 可以忽略，因为 X11/EWMH 没有
通用的关闭阴影协议。无 compositor 时 Rounded 仍可工作，但不会出现阴影。

## 5. 生命周期与失败处理

- Popup 尺寸保持内容尺寸，不为原生阴影扩大 surface；
- 圆角指标按平台会话缓存，实际原生应用失败时只允许从正值降为 0；
- 圆角失败不能阻止菜单显示：使用矩形 fallback；
- 阴影创建失败不影响 Popup 内容与输入；X11 阴影始终是 best-effort；
- 重建原生资源必须先销毁 Painter，再销毁 Popup，并保持 anchor/owner 的既有
  unmount 与 window-destroy 清理连接；
- Appearance 变化、resize 和 Hide/Show 不能产生额外 `Closed` 或 modal 切换。

## 6. 测试与验收

### 自动化测试

- Settings：有效半径、0 值矩形、查询错误及 GUI 的 0 fallback；
- GUI：Popover 默认矩形、PopoverMenu 默认圆角、阴影默认开启、Appearance
  更新、菜单外层覆盖 style radius，以及重建不触发 `Closed`；
- Windows：DWM 成功、旧系统/失败退化、圆角 hit-test、DPI/resize、两套阴影
  class 和资源生命周期；
- macOS：透明窗口、CALayer 半径、OpenGL alpha surface、阴影刷新和释放；
- X11：Shape mask 位序、Bounding/Clip/Input shape、resize、无扩展 fallback、
  `_NET_WM_WINDOW_TYPE_POPUP_MENU` 与 `_COMPTON_SHADOW` 属性；
- 更新全部 `platform.Platform`、`platform.Settings` 测试替身与调用点。

### 平台样例

Popup 平台样例增加：矩形/平台圆角、阴影开关、深浅背景、动态 resize、重复
Show/Hide，以及 1x/2x HiDPI。菜单样例同时验证 GUI 背景和原生边缘一致。

### 验收标准

- 支持圆角的平台没有矩形底色残留，GUI 背景与原生边缘使用同一半径；
- 不支持圆角的平台完整显示为矩形，不出现半圆角或样式缺口；
- resize、DPI 变化和资源重建后没有旧 shape；
- owner、非激活、modal、键鼠输入、dismiss 和生命周期行为不回归；
- `go test ./...` 通过，并完成 Windows/macOS 交叉编译与三平台真机验证。

## 7. 路线衔接

本设计验收后，P2 可标记完成。下一独立切片为 Integrated Window Decoration：
Windows 使用 DWM 与自绘标题栏，macOS 使用 full-size content 并保留原生交通灯，
Linux 提供 System/Integrated CSD 选择。该切片另行设计，不与 Popup 实现混合。
