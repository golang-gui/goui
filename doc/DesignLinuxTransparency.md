# Linux 透明窗口与输入区域（历史草案，已撤销）

> 状态：历史第一步草案。透明绘制已扩大为三平台、Window/Popup 共用并实现，
> 当前接口和实现以 [Window / Popup 透明绘制](DesignTransparency.md) 为准。
> 显式输入区域／鼠标穿透已按用户决定放弃，不是等待实施的路线。下述旧查询接口与非预乘颜色假设均不是当前契约。
>
> 当前透明绘制与普通 Overlay、GUI Linux Integrated 均已实现。Menu／DropDown 保留原生 Popover，不采用该草案的迁移顺序。当前颜色以 [预乘契约](DesignGraphicsColor.md) 为准；以下全部为历史提案，不能引用为现行 API。

## 1. 目标与边界

第一步仅推进 platform：Linux/X11 的普通 Window 可显式创建逐像素透明表面，
OpenGL、Software 均保留 alpha；调用方独立设置指针输入区域。

- 不增加 GUI/UI 透明接口，不实施 Overlay 或菜单迁移。
- 不修改 Popup/NewPopup；内部窗口创建与像素处理可复用，Popup 的透明选项以后按需增加。
- 不实现 Wayland、Windows/macOS 透明呈现、窗口模糊、统一 opacity 或颜色键透明。
- 不为平台指定圆角、阴影或透明缩放热区，不从绘制结果自动计算输入区域。
- 不改变 Chrome 定义：X11 Integrated 仍不支持；None 仍是交给 WM 管理的无装饰窗口。
  透明是完整自绘 CSD 的基础，不意味着 X11 获得保留原生边框的半自绘能力。
- 阴影占位、内容原点、最大化后的外边距和 GUI Chrome 的 CSD 组合属于后续切片。

透明影响合成，输入区域影响指针目标；两者完全独立。无合成器时仍可能支持输入区域。
采用这一边界与 [GDK Surface 输入区域](https://docs.gtk.org/gdk4/method.Surface.set_input_region.html)
一致；不把 X Shape 的二值输入区域用作视觉裁剪。

## 2. 建议公共接口

保持 Window 的跨平台最小宿主职责。创建请求进入现有 WindowOptions；本轮查询与
输入区域进入 DesktopWindow，不扩展 Surface、Painter、Settings，不新增能力管理对象。

```go
type WindowOptions struct {
    Chrome      WindowChrome
    Transparent bool
}

type DesktopWindow interface {
    Window
    // ...existing methods...

    // Transparent reports whether this window has a per-pixel-alpha
    // presentation path and a currently available native compositor.
    // It does not inspect painted pixels or echo the creation request.
    Transparent() (bool, error)

    // SetInputRegion replaces the client-local DIP pointer input region.
    // nil restores the native default; a non-nil empty slice accepts no pointer input.
    // Rectangles are unioned. The implementation copies the slice.
    SetInputRegion(rects []geometry.Rectangle) error
}
```

### 2.1 透明创建与查询

- `Transparent=false` 是零值，保留已有不透明创建和呈现路径。
- `true` 是严格请求，不是可忽略偏好。必须选中可用的 alpha visual 且观察到合成器，
  否则创建失败；不得悄悄创建普通窗口，再把请求返回为“支持”。
- 本轮 Windows/macOS 收到 `true` 返回 `ErrUnsupported`；`false` 不改变已有行为。
- Chrome 与 Transparent 独立。X11 Native/None 均可请求客户区透明；原生装饰不属于
  客户区，不承诺其透明或输入穿透。Integrated 仍按既有错误处理。
- visual/depth 在创建时确定，不提供运行时透明 setter，也不隐藏重建原生窗口。
- 非透明目标的 `Transparent()` 返回 `false, nil`。透明目标查询当前 screen 的
  `_NET_WM_CM_Sn` selection owner：存在返回 `true, nil`，不存在返回 `false, nil`。
  查询失败返回错误；窗口已销毁返回 `ErrUnavailable`。Windows/macOS 本轮的普通
  不透明窗口也返回 `false, nil`。
- 该查询是当前环境事实，不保证合成器一定采用某种视觉效果。合成器退出时不能保证
  桌面继续透明；visual 不变，不自动重建窗口或改画背景。恢复合成后同一 ARGB 表面
  可继续参与合成。本轮不增加通知、轮询定时器或每帧同步查询；上层可按需重查。
- 无 ARGB/XRender 能力是 `ErrUnsupported`；创建时暂时没有合成器是
  `ErrUnavailable`。原生调用错误保留上下文。不得因可选扩展缺失导致整个 Platform 初始化失败。

### 2.2 输入区域

| 参数 | 语义 |
| --- | --- |
| `nil` | 恢复窗口默认输入区域，随尺寸自然变化 |
| 非 nil、长度为 0 | 设置空输入区域，整块客户区不接收指针 |
| 非空切片 | 各矩形的并集接收指针，其余客户区交给下层窗口 |

- 区域使用客户区局部 DIP，与 Painter Transform/clip 无关；保存副本，调用返回后
  用户可修改或释放切片。必须保存 nil 与空切片的区别。
- 不要求窗口透明：可见但穿透、不可见但可点击都合法。区域不包括 WM 的装饰窗口。
- 不改变键盘焦点、激活状态、原生移动/缩放和已经建立的指针 grab/capture；原生抓取
  期间的事件仍服从 X11 规则。应用要避免在正在拖动时假设更新区域会撤销抓取。
- 不在 GOUI 中重发输入，不使用 `SetHitTest` 模拟。穿透由 X server 选择下层目标。
- 支持在 Show 前设置，避免先映射出一个临时挡住鼠标的完整矩形；未设置时默认整窗接收。
- 不提供额外的整窗穿透 bool，空区域已完整表达这一需求。
- 本轮 Windows/macOS 所有输入区域请求（包括 nil）均返回 `ErrUnsupported`。

### 2.3 校验、取整与更新

- 坐标/尺寸及右下端点必须有限；负宽高返回参数错误，零面积矩形忽略。任一项非法
  拒绝整个请求，保留旧区域；不能先写入部分矩形。
- 负坐标和超出窗口的矩形允许。保存未裁剪 DIP 副本，应用时与实际客户区相交；
  窗口扩大后可重新显露原来超出部分，不把区域当百分比布局自动拉伸。
- 先将 DIP 乘 scale 并在连续物理坐标下裁剪，记每轴端点为 L/R，再采用像素中心规则：
  整数区间为 `[ceil(L-0.5), ceil(R-0.5))`。相邻矩形共享端点，避免四舍五入产生裂缝。
- 使用无符号长度/有符号起点的 XRectangle 时先检查协议范围；不强制转换截断。
  超出 INT16 起点或 CARD16 长度等协议限制返回 `ErrUnsupported`，不发布部分区域。
- 一次 `ShapeSet` 替换完整区域，矩形重叠允许，使用 `Unsorted`。超过单次请求上限
  明确返回错误，本轮不通过多次非原子写入拼接。
- ConfigureNotify 后、发送 SizeEvent 前重应用旧 DIP 区域；新的 GUI 布局以后再显式设置。
  缓存最后提交的像素区域，重复值不提交 X 请求。nil 直接恢复默认，不能固定成上次大小。
- 当前 X11 的缩放是会话级缓存；本轮不额外实现动态系统 DPI 监听。几何辅助函数接受
  显式 scale，分别用 1x/1.25x/1.5x/2x 验证，未来 scale 更新复用同一重应用入口。

## 3. Linux 原生实现

### 3.1 绑定与资源

遵循现有 `platform/linux/libs` 规范：lazy library、原生 symbol、原生结构和薄函数。

- `xrender`：libXrender，扩展探测及 `XRenderFindVisualFormat`、相关格式结构。
- `xshape`：libXext，`XShapeQueryExtension/QueryVersion`、`XShapeCombineRectangles`、
  `XShapeCombineMask`，测试使用 `XShapeGetRectangles`。输入区域要求 Shape 1.1。
- `xlib`：补充 visual 枚举、原生矩形与必要错误捕获绑定；已有 selection owner 查询复用。
- WM/compositor 判断、DIP 换算、像素打包、错误包装留在 x11/opengl，不放进 libs。

输入区域只使用 `ShapeInput`。nil 用 `XShapeCombineMask(..., None, ShapeSet)` 恢复
默认；空切片用零矩形的 `XShapeCombineRectangles(..., ShapeSet)` 表达空集。
这两种原生操作不可混用。依据 [X Shape 1.1 协议](https://www.x.org/releases/X11R7.7/doc/xextproto/shape.html)。

### 3.2 创建 ARGB 窗口

1. 校验选项和尺寸。透明请求检查 `_NET_WM_CM_Sn` 的 owner，不将 X server 支持
   Composite 扩展等同于正在运行合成器。screen 使用当前窗口所属的 X screen，
   不硬编码 0；不是指 RandR 显示器编号。依据 [EWMH 合成器约定](https://specifications.freedesktop.org/wm/latest/ar01s08.html)。
2. 优先选择可供 GLX 使用的 32-bit TrueColor visual。必须用 XRender 格式验证
   alphaMask 非零，并检查 RGB/alpha 位布局；`GLX_ALPHA_SIZE > 0` 本身不足以证明
   X visual 可透明合成。不使用 GLX 颜色键式 transparent 类型替代 ARGB。
3. 透明 GLX 候选硬过滤本轮支持的 8-bit BGRA/ARGB 布局，再按原有规则选择；保留
   `GLX_WINDOW_BIT` 兼容逻辑与 GLXWindow 绑定机制，不恢复旧 resize workaround。
4. 无匹配 GLX 配置但存在可用 ARGB visual 时允许创建 Software 可用窗口，`fb=0`；
   后续 OpenGL 创建明确失败，平台可沿用已有 Software fallback。禁止为了创建 GL
   换用另一个与窗口不匹配的 visual。Software-only 不能被 GLX 缺失阻塞。
5. Window 保存实际 visual、depth、native alpha 格式、是否启用透明和 colormap。
   显式设置背景像素为 0（CwBackPixel），使用匹配的 colormap；不可给 alpha window
   写入全窗 opaque-region 提示，也不设置全窗 opacity 属性来替代逐像素透明。
6. 所有初始化成功后才注册/发布窗口；创建失败释放 colormap 与已分配的 native 资源。
   保持默认输入区域，直到调用方设置。透明属性不改变事件、frame clock、resize sync。

本轮支持常规 32-bit、每通道 8-bit 的原生格式；其它 alpha 位宽明确不支持。
打包需核对 byte order，不能假定深度 32 就等于内存 BGRA；无转换需求时直用原始行。

### 3.3 OpenGL

- 使用窗口已经选定的 NativeFBConfig 创建 context/GLXWindow。当前 Linux context
  并不使用 NewPainter 的通用 AlphaBits 重新选格式，因此不能仅修改通用配置为 8。
- 对透明目标确认实际 GL framebuffer 有至少 8 位 alpha，失败不能当作成功透明 GL。
- NanoVG 的预乘颜色保留到交换缓冲，不在末端清除 alpha，不额外绘制窗口圆角遮罩。
- 透明目标 `Clear(Color)` 的直通 glClearColor 需改为预乘 RGB 后写入；例如半透明红
  写入 `(0.5,0,0,0.5)`，否则会产生过亮边缘。调用方 Color 仍是现有非预乘绘图值。
- 此处理只进入本轮透明目标；普通目标呈现路径不借机改写。不增加 framebuffer 读回、
  输入 alpha 扫描、每帧 XSync 或全尺寸中间 FBO。

### 3.4 Software：必须修正提交边界，而不是再预乘一次

现状：Software 内部 `p.bgra` 是预乘像素，End 却反预乘到 outputBuf 后作为 Bitmap
提交。`graphics.CopyToBitmap` 又按 image.Image 的预乘约定工作。这种旧呈现边界
不能直接用于新 ARGB 窗口，不能通过在 x11.Draw 中无条件乘 alpha 来猜测输入来源。

本轮采用最小的内部目标约定：

```go
// Structural interface used only inside platform; not embedded in common.Surface.
interface {
    NativePremultipliedAlpha() bool
}
```

- x11.Window 实现该方法，仅对成功创建的透明窗口返回 true，表示 Draw 消费预乘像素。
  该值是创建后固定的像素协议，不随合成器启停改变；不要与 Transparent() 环境查询混用。
- Software 创建时读取该事实。true 时 End 将内部预乘 BGRA Bitmap 同步交给 Draw，
  不经过 outputBuf 的反预乘/复制；false 或无该方法时保留旧提交逻辑。
- x11.Draw 对透明目标按标准 image.Image 语义获取预乘 BGRA，使用原生格式打包；
  alpha=0 的 RGB 清零。匹配格式、byte order 和 stride 时不再复制 Bitmap。
- 需要换字节序或行格式时使用 Window 自有可复用 scratch；不修改调用方像素。
- CreateImage 使用 Window 实际 visual/depth，不再使用 RootVisual/RootDepth。
  同时校验 bits-per-pixel、bytes-per-line、掩码与数据长度。
- XPutImage 同步消费调用方内存，XDestroyImage 前解除 Data 引用，不能释放 Go 内存；
  不引入 SHM 或异步保留像素引用。
- 该内部目标约定也可供 OpenGL 判定预乘 Clear 路径；普通 Windows/macOS 不实现，
  不为本切片重写其 Software 提交逻辑，不修改公共 Bitmap/Color 模型。
- 测试必须同时覆盖 Software.End、直接 Window.Draw(image.RGBA/NRGBA/Bitmap)，
  防止二次预乘、遗漏预乘以及不同调用入口颜色不一致。

### 3.5 错误、线程与生命周期

- 沿用 OS 线程亲和，SetInputRegion 不在 native hit-test 回调里运行，也不派发 GUI 信号。
- Xlib 很多请求异步报错；不能以 XID 非零或 void 调用返回作为服务端成功的证明。
  仅在窗口资源创建和改变输入区域的控制路径使用串行、限定 display/请求序号的
  error trap：先 drain 旧错误、安装 trap、提交请求、XSync 确认、恢复旧 handler。
  trap 期间不派发事件或运行用户代码；非目标错误交还原处理，不吞掉历史错误。
- 原生绑定仍只暴露 XSetErrorHandler/XSync 等原语，trap 的流程归 x11 后端。
  不新增每帧阻塞同步，不干扰已存在的 X Sync resize counter。
- 参数或原生提交失败不更新成功缓存；协议限制拒绝整次请求。Resize 重应用错误
  不能伪装成功或打开新的输入洞：保留上次服务端区域并记录错误，下次显式设置返回
  当前请求结果。构造时输入 Shape 扩展缺失不妨碍透明绘制，只让 SetInputRegion 返回不支持。
- Painter 在 Window 前销毁；Window 释放 GC、colormap、scratch、输入区域副本和
  应用缓存。无新定时器、全局持有 Widget 的表或持续存在的 error handler。

## 4. Overlay 与后续窗口装饰的衔接

窗口透明和 Overlay 是两条独立能力线：Overlay 复用不透明窗口的 Painter 即可画
圆角/阴影，不依赖 Linux 透明成功。第二步是 Window 级承载，而不是任意 Stack 布局。

- Menu、Tooltip、ComboBox 下拉默认窗口内定位、翻转/平移/限高；不自动切换原生 Popup。
- Popover 保留窗口外显示；菜单模型/内容/交互与承载分离，不做两套完整菜单。
- Overlay 进入既有布局、命中、焦点、capture 和 Snapshot；UI 仅做薄适配。
- 当前 ModalTarget 的“owner 收到的点击必在 Popup 外”前提不适用于 Overlay，迁移时
  必须改正，不能先拦截所有 PointerDown 把内部点击也当成 dismiss。
- Tooltip 默认不抢焦点、不参与指针命中；ComboBox 本体仍是正常布局中的 Widget。
- 不把绘制在窗口内的菜单阴影与桌面合成器的窗口阴影混为一谈。

Linux 完整 CSD 后续由 GUI 使用 None + Transparent + InputRegion + BeginMove/Resize
组合，保留透明 resize 热区和穿透的阴影区域。平台不自动生成区域、不模拟 HitTest；
GUI 层如何提供 Integrated 的跨平台请求语义另行设计，不直接改写 X11 Chrome()。

## 5. 实现文件与顺序（待执行）

1. common/window.go 增加已确认后的选项/接口和注释；同步 platform 别名、三端实现
   及 DesktopWindow 测试替身。Windows/macOS 只加不支持处理，不改渲染。
2. linux/libs/xrender、xshape 与必要 xlib 绑定，补 ABI 和扩展缺失测试。
3. x11 新增 window_transparency_linux.go、window_input_region_linux.go：管理选型、
   查询、输入区域和局部错误检查；window_linux.go 只保留接入点及实际 visual/depth。
4. opengl/opengl_linux.go 的透明 FBConfig 硬筛选与 Software-only 路径；OpenGL Clear
   和 Software End 的透明目标提交，保持非透明路径与资源缓存不变。
5. 平台样例及像素/原生输入验证后才进入 GUI Overlay；本轮不修改 gui/root.go 的白色
   Clear，也不宣称 GUI 创建的窗口已经透明。

## 6. 验证与退出条件

### 自动测试

- 选项默认、非法 Chrome、严格透明请求；缺 XRender/ARGB、缺合成器、无 GLX 的
  Software fallback；Native 与 None 的请求不相互暗示，Integrated 不伪造支持。
- Shape 1.1 探测、nil/空区域、矩形并集、裁剪后为空、负坐标、零面积、非法值、协议
  溢出、slice 拷贝、原生失败保留缓存；重复区域不产生请求。
- 几何转换覆盖 1x/1.25x/1.5x/2x、相邻矩形、resize 后重应用和 nil 恢复后的自动扩展。
- 预乘提交：alpha=0/128/255、非黑色半透明 Clear、DrawImage/文字边缘、同一帧多次
  source-over；Software 与 OpenGL readback 参考一致。Readback 仅测试使用。
- Bitmap 子矩形、额外 stride、BGRA/RGBA/NRGBA、byte order、像素 buffer 所有权；
  非透明目标仍走旧协议，透明 End 不额外反预乘/分配全尺寸输出。
- XShapeGetRectangles 检查服务端区域；独立扩展/原生资源反复创建销毁；Windows/macOS
  DesktopWindow 测试替身与接口交叉编译。

### Linux 真机样例

使用两个 GOUI 窗口：底层画棋盘格并显示收到的指针事件，上层为透明 None 窗口。
上层显式 Draw 半透明色块、圆角内容、可选 BoxShadow、透明但可点击的边缘热区。
区域穿透实验先在两个顶层都有交互区域时设置，不能因测试整窗穿透丢失退出手段。

- 分别强制 OpenGL、Software；确认黑/白底上的半透明颜色正确、圆角无矩形残留。
- 显式区域外真实鼠标点击到底层；区域内只到上层；恢复 nil 后恢复默认指针目标。
  同时验证“可见但穿透”和“透明但可交互”，不是只看 alpha=0 的特例。
- 实际鼠标/XTest 设备级输入验证 X server 目标选择，不能用直接调用 DispatchEvent
  冒充桌面级穿透。键盘焦点和 grab 期间行为单独记录。
- resize、反复 Show/Hide、最小化/恢复，1x/2x 会话；合成器开启、关闭、恢复及无 Shape
  环境的能力报告。原生装饰区域的穿透不在验收承诺内。
- 至少一个真实合成桌面与一个无合成器环境；Xvfb/仅单元测试不作为桌面透明验收替代。

执行 `go test ./...`；Windows amd64、macOS arm64 相关测试目标交叉编译。测试造成
输出文件时不得混入提交。无 display/扩展的测试明确 skip，不能将 skip 写成视觉通过。

退出条件：两种 Painter 透明像素正确、输入区域与 alpha 独立、失败可解释、无普通
窗口/resize 回归且资源释放可验证。完成此项只勾选 Linux 基础能力，不勾选 Overlay、
菜单迁移、Popup 透明或 Linux 完整 CSD。
