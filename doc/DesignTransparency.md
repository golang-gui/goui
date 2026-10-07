# Window / Popup 透明绘制

> 状态：platform 三平台透明绘制已由用户验证通过；GUI 接入已实现，GUI 示例待真机体验。
> 本文替代 DesignLinuxTransparency.md 中仅 Linux、仅 Window 的透明绘制方案。
> 输入区域、Overlay 和完整 CSD 不在本切片内。

## 公共契约

```go
type WindowOptions struct {
    Transparent bool
    Chrome WindowChrome
}
type PopupOptions struct {
    Transparent bool
}
type Surface interface {
    Transparent() bool
    NativeHandle() uintptr
    Draw(image.Image) error
    RequestPaint() error
}

NewWindow(size geometry.Size, handler EventHandler, options WindowOptions)
NewPopup(owner Window, size geometry.Size, handler EventHandler, options PopupOptions)
```

- 默认不透明。透明是显式创建请求，不是透明失败后悄悄降级的偏好。
- Window 和 Popup 使用相同的底层透明表面；Popup 不继承 owner 的透明选项。
- Transparent 报告创建的 alpha 表面配置，创建后不可切换；不是窗口可见性、
  当前合成器状态、最终像素 opacity 或鼠标命中的查询。
- 创建窗口检查系统前提，NewPainter 检查实际绘制资源。失败返回错误。
  Linux 无合成器返回 ErrUnavailable，无适合的 ARGB visual 返回 ErrUnsupported。
  Windows 透明路径要求 Windows 8+ 与 DWM；普通不透明路径保持原兼容性。
- 原生边框、按钮不由此变成透明，也不自动增加圆角、阴影或背景模糊。
- Surface.Draw 同步消费图像，不保留调用方的像素引用。透明目标按 image.Image 的
  预乘 alpha 约定消费 RGBA/BGRA。每帧用 Clear(Color{}) 清空，再绘制所需形状。
- 视觉透明不承诺输入穿透：本次没有设置 ShapeInput、WS_EX_TRANSPARENT 或
  ignoresMouseEvents。系统对透明区域的原生输入行为仍可能不同。

## 呈现路径

| 平台 | OpenGL | Software | Direct2D |
| --- | --- | --- | --- |
| Linux/X11 | XRender ARGB visual 匹配 GLX FBConfig/GLXWindow | 同一 visual/depth 的 XImage，预乘 BGRA | 不适用 |
| macOS | NSWindow 非 opaque、clearColor，NSOpenGL surface opacity=0 | CGContext 清空旧内容，CGImage premultiplied alpha | 不适用 |
| Windows | 带 8 位 alpha 的 WGL framebuffer，DWM alpha redirection | CPU 光栅化后复用 Direct2D 的 composition 上传资源 | DXGI composition swap chain + DirectComposition visual |

graphics.Color 始终使用预乘颜色；后续已按 [绘制颜色契约](DesignGraphicsColor.md)
统一构造、三后端原生颜色边界及文字颜色转换，不在透明呈现路径重复预乘。
Software 内部同样使用预乘 alpha，透明目标直接提交当前帧缓冲，不执行旧的不透明
呈现路径所需的 unpremultiply，也不增加整帧输出拷贝。

### Linux

不能仅看 GLX_ALPHA_SIZE：必须由 XRender 确认 visual 的 alpha mask。当前接受
32 位、R/G/B/A 位于 16/8/0/24 的标准 BGRA 排列。GLX 不可用时，从 Xlib screen
visual 列表选择同样的格式，Software 不依赖 GLX。

窗口保存自身 visual/depth；不再错误地对非 root visual 的窗口使用 root depth
构造 XImage。透明背景像素为 0。仍使用现有 GLXWindow 与 resize 协调路径。

### macOS

窗口和 Popup 的创建入口共同配置 NSWindow 的 opaque/backgroundColor。
普通 NSView 默认是非 opaque，无需增加特殊内容视图类型。
Software 透明帧必须先清除 CGContext，否则透明像素无法擦掉上一帧。
Retina 仍按现有物理像素到 view points 的转换绘制。

### Windows

Direct2D 保持已有 D3D11/DXGI/D2D 1.1 资源链，增加 DirectComposition Device、
Target 和 Visual。透明目标使用 WS_EX_NOREDIRECTIONBITMAP、PREMULTIPLIED swap
chain 和 D2D bitmap；资源失败清理和 device-lost 重建包含 composition 对象。

CreateSwapChainForComposition 要求 FLIP_SEQUENTIAL、SCALING_STRETCH 和非零初始
尺寸。这里的 STRETCH 是 API 要求，并不把 visual 拉到窗口大小：visual 始终 1:1，
buffer 仍按实际物理尺寸 ResizeBuffers，保留原有同步 resize 重绘机制。

Window/Popup 只保存透明配置，创建时不设置绘制后端专用的扩展样式。
Direct2D Painter 和 wglContext 各自通过 NativeHandle 取得 HWND，在初始化时
完成呈现配置；不通过 Window 的 NativePrepare 接口反向调用，也不在 Window 中
保存当前绘制后端状态。成功绑定后不解除或切换；初始化失败清理资源并回滚扩展样式，
以便创建流程尝试其他后端。

WGL 需要 DWM redirection，创建 OpenGL context 时配置对应 HWND；
启用空 blur region 以保留 framebuffer alpha，采用 GLFW 的集成路径，不做 GPU
读回，也不使用 WS_EX_LAYERED。DWM alpha 在 context 创建成功后最后启用，
避免此前失败的步骤改变 DWM 配置。同一表面不允许同时使用多个呈现后端或绑定后更换后端。

Software 仅借用 composition 做像素上传与呈现，光栅化仍由 Software 完成。
上传 Painter/Image 由原生 Window 持有，尺寸不变时 Update，销毁窗口前释放。
因此 Windows 的透明 Software 呈现也依赖 D3D/DXGI（硬件设备或 WARP），不是
纯 GDI 的透明兜底；不透明 Software 仍使用原有 GDI 路径。

透明 Direct2D 目标默认灰度文字抗锯齿。不透明目标保留原来的文字效果。
这不是 DirectWrite 不支持透明：ClearType 需要文字背景已知且不透明，不能对
整张含透明内容的表面无条件启用。本次不引入不透明区域追踪或修改文字公共 API。

## GUI 接入

```go
// WindowOptions 保留原来的 Size / Chrome 字段。
type WindowOptions struct {
    Size        geometry.Size
    Chrome      WindowChromeMode
    Transparent bool
}
type PopoverOptions struct { Transparent bool }

func NewPopover(anchor Widget, options *PopoverOptions) Popover
// Window / Popover 均提供 Transparent() bool，无 setter。
```

创建时复制选项，nil/零值默认不透明。Popover 不继承 owner 的透明配置，
原生资源惰性创建前即可查询；Hide/Show 复用及 unmount 后重建都保留配置。
透明创建失败返回错误，不改成不透明重试；Window 的 Chrome 降级保留透明请求。
Popover.Show 失败不进入可见或模态状态，不发送伪造的关闭通知。

### 背景属于样式，清空属于帧初始化

Window 默认使用 `"window"`、Popover 默认使用 `"popover"` 的 Default part / Normal
样式。两个宿主均可用 StyleName()/SetStyleName(string) 独立选择名称，空名称恢复
各自默认；默认 BackgroundColor 为白色，透明表面本身不会自动移除背景。
绘制顺序统一为：清空帧缓冲 → 样式主体背景 → Widget 内容 → 窗口装饰控件。

- 透明表面清空为零 RGBA；不透明表面清空为不透明黑色，确定未覆盖区域的值。
- 样式背景目前填充整个客户区，独立于 Clear；本轮仅消费 BackgroundColor。
- 未设置或 nil 背景不填充；显式透明背景也不会盖住透明帧缓冲。
- 自定义样式表沿用原有替换语义，需要默认规则时扩展 `DefaultStyleRules()`，不增加隐式级联。
- 内容为空仍提交清空后的帧，避免上一帧残留。应用样式更新同步失效窗口和 Popover。
- Window.SetStyleName 仅请求自身重绘，不向 Widget 继承样式、不改变布局或
  Transparent 配置；相同名称或销毁后的 setter 是 no-op。只消费 BackgroundColor，
  原生/GUI 装饰几何不由该样式决定。GUI 装饰的亮暗选择跟随此窗口的实际背景。
- 样式表仍属于应用；独立透明窗选择自己的透明背景规则，不能改写共享 `window`
  规则来影响所有窗口。不增公开颜色 setter 或每窗口样式表。

背景的含义是“窗口主体底色”，不是“整张原生表面的清空色”。Linux GUI Integrated
后续切片已复用同一背景色，绘制圆角主体、在安全矩形内绘制内容/按钮，再绘制
边框；安全裁剪阻止子控件盖住圆角。不使用整帧圆角遮罩，不预留或绘制阴影。
Linux GUI 自绘 Integrated 的根布局与安全裁剪使用同一内侧矩形，四边保留给
窗口缩放；HeaderBar 随根布局避让，不重复添加 Padding。最大化/全屏恢复完整
区域，其他平台原生 Integrated、Native 和 None 不增加避让。细节见
DesignGUIWindowChrome.md；不扩展为公共 SafeArea，也不改变窗口坐标系。
该模式自动请求 alpha 表面，`Transparent()` 因而报告 true。Native/None/Popover 的
透明选项与样式仍彼此独立，不会自动生成圆角或阴影。指定输入区域已取消。

本轮不添加 UI 透明选项或迁移 Menu/Tooltip 到 Overlay。

## 验证

- Software 单元测试：1x/2x、半透明预乘值、连续帧透明清空。
- Linux 原生 GL framebuffer 读回：ARGB visual 与半透明 Clear 的实际 RGBA 值。
- 三端原生创建测试：默认不透明、Window/Popup 独立透明、失败不伪造成功。
- 原生呈现测试：Software/OpenGL，Windows 额外 Direct2D，两种宿主、多帧清空。
- DirectComposition vtable ABI 测试；Windows/macOS 交叉编译。
- 用户已确认三平台底层透明绘制通过；设备丢失恢复仍需专项验证。
- GUI 单元测试覆盖 1x/2x、清空与背景顺序、空内容、默认/动态样式、Chrome 降级、
  Popover 选项隔离、复用/重建和失败清理；GUI 真机外观仍待体验。

```sh
go test ./...
GOUI_TEST_TRANSPARENCY=1 go test ./platform/test -run Transparent -count=1 -v
GOUI_PLAT_PAINTER=software go run ./example/transparency
GOUI_PLAT_PAINTER=opengl go run ./example/transparency
# Windows: GOUI_PLAT_PAINTER=direct2d
```

示例点击任意表面后清空/恢复内容，Popup 部分超出 owner；用于肉眼检查边缘和残留。
只有 Window 保留原生标题栏便于移动和关闭，不把自绘标题栏策略混入示例。

GUI 示例：`go run ./example/gui_transparency`，可切换 Window/Popover 背景、显隐 Popover、
移除/恢复其内容；同样可通过 `GOUI_PLAT_PAINTER` 选择绘制后端。

## 原生依据

- [Direct2D alpha 与文本抗锯齿](https://learn.microsoft.com/en-us/windows/win32/direct2d/supported-pixel-formats-and-alpha-modes)
- [CreateSwapChainForComposition](https://learn.microsoft.com/en-us/windows/win32/api/dxgi1_2/nf-dxgi1_2-idxgifactory2-createswapchainforcomposition)
- [DirectComposition 初始化](https://learn.microsoft.com/en-us/windows/win32/directcomp/initialize-directcomposition)
- [GLFW Win32 framebuffer transparency](https://github.com/glfw/glfw/blob/master/src/win32_window.c)
- [macOS OpenGL 窗口表面](https://developer.apple.com/library/archive/documentation/GraphicsImaging/Conceptual/OpenGL-MacProgGuide/opengl_contexts/opengl_contexts.html)
- [X11 compositing manager selection](https://specifications.freedesktop.org/wm/latest/ar01s08.html)
