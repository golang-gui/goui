# 绘制颜色与预乘 Alpha

## 唯一值契约

`graphics.Color` 始终保存 `[0,1]` 范围内的预乘 RGBA：`R/G/B <= A`；
alpha 为零时 RGB 必须也为零。不兼容同一个 Color 在不同路径中代表不同语义。

```go
graphics.Color{R: .5, A: .5}                  // 预乘半透明红
graphics.RGBA(255, 0, 0, 128)                 // 接收非预乘字节，构造时预乘
graphics.ColorOf(color.NRGBA{R:255, A:128})   // 接收标准 Go color.Color
```

- `RGB` 构造不透明颜色；`RGBA` 参数是非预乘字节，返回值是预乘浮点颜色。
- `ColorOf` 直接保留 `color.Color.RGBA()` 的预乘 16 位分量，不经 8 位中间转换。
- Color 的 `RGBA()` 返回预乘 16 位分量，`RGBA8()` 返回预乘 8 位分量，量化时四舍五入。
- 手写 Color 字面量必须自己预乘。不能把 `color.RGBA` 当作 `color.NRGBA`：
  Go 的前者也表示预乘颜色，普通 RGB+透明度请用后者或 graphics.RGBA 构造。
- GUI 的 Color、UI 的 Color 都是别名，没有第二套颜色解释；样式通过 ColorOf 接入。

## 后端转换只在边界进行

| 路径 | 处理 |
| --- | --- |
| Software 填充／描边 | 仅交换 R/B 以适配 BGRA；scanFT 接收预乘通道，不再次乘 alpha |
| Software Clear | 直接写入预乘 BGRA |
| Software 线性渐变 | 对预乘 R/G/B/A 分别线性插值，不乘除 alpha |
| OpenGL Clear | glClearColor 直接接收帧缓冲通道，不额外预乘 |
| NanoVG 画刷／渐变／阴影 | native 边界统一反预乘一次；NanoVG 会在内部预乘后插值、合成 |
| Direct2D Clear／画刷／渐变端点／阴影颜色 | native 边界统一反预乘一次；原生颜色参数始终采用 straight alpha |
| 图片／文字位图 | 继续保持预乘像素，纹理／位图标记 PREMULTIPLIED；不经过普通画刷颜色转换 |

反预乘入口在 alpha 为零时返回全零，避免除零。阴影不再拥有与普通画刷重复的
独立反预乘步骤。裁剪、变换、抗锯齿覆盖率与 HiDPI 不改变颜色值契约。

不透明原生目标可能忽略最终 alpha；窗口透明选项和平台呈现策略不由此新增归一化。
半透明窗口的最终视觉仍需要原生合成器验收，离屏像素测试不代替桌面呈现验证。

PaintStyledBox 在背景与完整边框均不透明时，用边框色填满外形，再在真实内轮廓
填入背景色，使形内过渡保持不透明；同色只填一次。透明/半透明组合及独立背景/
边框绘制沿用原有 SourceOver，不改变预乘颜色或图元契约。轮廓、AA 对照和适用
条件见 [不透明异色细边框](DesignCornerRadius.md#不透明异色细边框2026-11-12)。

Direct2D 原生参数依据：[画刷颜色](https://learn.microsoft.com/en-us/windows/win32/direct2d/direct2d-brushes-overview)、
[Clear](https://learn.microsoft.com/en-us/windows/win32/api/d2d1/nf-d2d1-id2d1rendertarget-clear%28constd2d1_color_f%29)、
[渐变端点](https://learn.microsoft.com/en-us/windows/win32/api/d2d1_1/nf-d2d1_1-id2d1devicecontext-creategradientstopcollection)。
目标位图采用预乘格式，不代表颜色参数也采用预乘格式。

### Direct2D 渐变插值

Direct2D 后端已有 ID2D1DeviceContext，使用其 CreateGradientStopCollection1
绑定显式选择输入/输出 sRGB、8BPC_UNORM、CLAMP 和 PREMULTIPLIED 插值。
端点仍先反预乘为原生 straight alpha；插值前预乘由 Direct2D 完成，不把预乘
端点直接传给原生，也不在 Widget 或渐变值中补偿。

旧 RenderTarget 版本的 CreateGradientStopCollection 在 Windows 10 / WARP
实测对不同 alpha 的端点产生 straight 插值结果：红 alpha=64、蓝 alpha=192
的中点为预乘 `{64,0,64,128}`，不符合既定 `{32,0,96,128}` 契约。
采用 DeviceContext 版本的原因是其显式提供插值模式；不增加新的 Painter API、
颜色空间选项或软件渐变替代路径，不改变既有画刷缓存的所有权和失效条件。

依据：[Microsoft 的 DeviceContext 渐变契约](https://learn.microsoft.com/en-us/windows/win32/api/d2d1_1/nf-d2d1_1-id2d1devicecontext-creategradientstopcollection)
明确区分端点参数与插值模式；sRGB → sRGB 不额外做 gamma 转换。
参考 [Qt 6.8.0 颜色表生成](https://github.com/qt/qtbase/blob/v6.8.0/src/gui/painting/qpaintengine_raster.cpp#L3949)
在 ColorInterpolation 模式下先预乘端点再插值的做法，沿用 GOUI 已有契约，
不引入颜色表缓存。
回归保留原填充/描边绝对值断言，增加 1x/1.5x/2x 的四分位采样、透明端点、
同色端点换方向和颜色缓存切换；量化容差保持为 ±2，不扩大容差掩盖失败。

## Software 栅格器依赖

Software Painter 使用自维护 `github.com/golang-gui/rasterx v0.1.0` 和
`github.com/golang-gui/scanFT v0.1.0`，与 icon/svg 统一依赖，不使用 replace 或本地路径。
修改仅涉及 Painter 及直接扫描器测试的 import，不增加公共 API 或另一套绘制适配。

rasterx 负责路径填充/描边轮廓，scanFT 负责覆盖率与颜色合成。几何或扫描错误在所属库
修复，GOUI 不按图形做坐标/透明度补偿。每个 Painter 继续独立持有可变 Scanner、Filler、
Stroker 和 BGRA 缓冲区；SVG Source 与 Painter 不共享这些状态。

负坐标的 26.6 定点分解必须向下取整：`[-0.25,0.25] × [1,2]` 在 `(0,1)`
只有 0.25 的覆盖面积，不能把画布外面积算进来。回归通过公开 FillRect/FillPath 和
RenderImage，以像素交集面积独立计算预乘 RGBA；FillRect 使用显式用户缩放保留小数
边界，普通矩形另按像素对齐契约验证。覆盖正反绕向、恒色 ColorFunc、
画布外/包围画布、1/1.25/1.5/2x。容差为既有 12 位覆盖率至 8 位通道的 ±1 量化误差。
新测试在旧依赖上失败、迁移后通过，不以两条扫描路径彼此相同作为正确性证据。

ClearPath 的覆盖度掩码及 destination-out 实现保持不变。现有零 clip 哨兵的处理也保留；
本轮没有把这些既定语义改成新的裁剪或清除模型。原有预乘、透明合成、渐变、
变换、HiDPI、文字图片缓存及资源释放断言继续全部执行。结果见 [Plan](Plan.md)。

### UI 平直边缘的像素对齐

Software 与 OpenGL、Direct2D 复用 `graphics/internal/pixelsnap`：FillRect、
FillRoundRect 对齐外框；DrawRect、DrawRoundRect 对齐描边外轮廓后恢复中心线；
DrawLine 只对齐水平/垂直线的法向位置。计算包含当前平移和 HiDPI，而非只取整局部坐标。
奇数物理像素宽度采用半像素中心，偶数宽度采用整数中心；保留非整数描边宽度。
背景与边框使用一致的外边界，不通过样式加深颜色或调整窗口按钮坐标来补偿。

只在用户变换为纯平移时启用；旋转、缩放、错切、斜线、任意 Path、文字和图片不加入
整数像素对齐，既有 clip 与 ClearPath 语义不变。浮点到 26.6 定点统一取最近值，
避免对齐后的坐标因浮点尾差向零截断而退回 1/64 像素；这一步仍保留亚像素坐标。
扫描器将负定点拆为整数像素和小数部分时仍向下取整，与最近值量化属于不同阶段。

回归用独立物理边界公式检查 1/2 像素直边完整覆盖及相邻像素透明，不放宽容差；
圆角必须仍有抗锯齿。既有 CheckButton/Switch 的独立圆面积预期改为使用对齐后的矩形
边界，Radio 的椭圆原语保持原始坐标，不改变覆盖率容差或颜色预期。
复合图形需要从最终外框计算内部几何时，可用 Painter.SnapRect 查询同一对齐结果。
Switch 据此计算滑块圆心和端点，避免矩形轨道与未取整椭圆使用不同中心；
椭圆半径和中间动画位置仍保留亚像素。查询不改变上述任何图元的默认行为。
GUI 样式盒形的三个 PaintStyled helper 显式对齐共同外框，包括生成 Path 的混合
圆角和选边组合；该样式策略不改变任意用户 Path 的行为，详见
[样式外框对齐](DesignCornerRadius.md#样式外框对齐2026-11-22)。
窗口验证入口为 [ui/pixel_snap](../tests/window/ui/pixel_snap/main.go)，包含普通/Primary、
35.25 DIP 高度及真实标题栏符号；Windows 1x 的修复前后对照与其他结果见 Plan。

### Direct2D 原生曲线

Direct2D 后端保留原生图元和曲线定义：椭圆／圆角矩形直接调用对应的 Fill／Draw
方法，Path 将 ArcTo／BezierTo 写入原生 GeometrySink，再用 FillGeometry／DrawGeometry
绘制。轮廓细分与逐图元抗锯齿由 Direct2D 负责。不预先强制展平曲线，不额外生成
高分辨率覆盖遮罩，也不以 Supersampling、离屏缩小或质量缓存替代默认原生输出。
ClearPath 继续使用原生透明画刷与 COPY 混合，随后恢复原混合状态。

边界适配仍遵守既有契约：矩形原语和样式盒形的像素对齐、窗口局部 clip 的物理
像素向外包络、矩阵换算及预乘颜色转换保留；任意曲线不额外吸附到像素网格。
正常的后端抗锯齿差异不属于接入错误，不承诺 Software、OpenGL、Direct2D 的边缘
逐像素相同。使用 Direct2D 不承诺与某个 Windows 控件工具包的绘制结果相同。

依据：Microsoft 的 [PER_PRIMITIVE 契约](https://learn.microsoft.com/en-us/windows/win32/api/d2d1/ne-d2d1-d2d1_antialias_mode)
将非文字边缘抗锯齿交由 Direct2D；[Qt 6.8.3 的原生椭圆路径](https://github.com/qt/qtbase/blob/v6.8.3/src/plugins/platforms/direct2d/qwindowsdirect2dpaintengine.cpp#L1346)、
[wxWidgets 3.2.6 的圆角矩形](https://github.com/wxWidgets/wxWidgets/blob/v3.2.6/src/msw/graphicsd2d.cpp#L4851)
及 [JUCE 8.0.6 的圆角／椭圆](https://github.com/juce-framework/JUCE/blob/8.0.6/modules/juce_graphics/native/juce_Direct2DGraphicsContext_windows.cpp#L1590)
均在相应原生绘制分支直接提交图元。此为已核对版本的源码行为，不推断其所有回退
路径，也不作为 Direct2D 在所有设备上逐像素一致的承诺。

回归以独立原生 Direct2D 调用为参考：WARP、相同 BGRA8 预乘目标、DPI、变换、clip
和颜色下，直接手写原生图元及 GeometrySink，不复用生产路径转换或像素对齐 helper。
覆盖填充／描边、Arc／Bezier、孔洞、擦除、半透明、亮暗底色、小数像素相位及
1/1.25/1.5/2x；可精确表达的颜色要求逐像素一致。另保留已有预乘颜色、渐变、裁剪、
直边和资源生命周期断言。此前理想圆面积／Bézier 积分的严格预算属于额外质量目标，
已由原生一致性回归替代；遮罩／细分缓存随实现移除，其专用生命周期测试相应删除。
可变 Path 的更新、旧图片隔离、曲线渐变坐标和擦除状态覆盖继续保留。

原生对照若复现相同边缘特征，记录为该环境的原生表现；若 GOUI 明显劣于原生，
先检查绑定 ABI、DPI／Transform、clip、颜色及合成，不默认引入更高采样率补偿。
性能对照沿用相同 WARP、256×128 RGBA8 和 32 图元完整离屏创建／绘制／读回，
分别记录固定几何与每轮改变全部几何／相位的结果，不推算桌面 FPS。结果见 [Plan](Plan.md)。

## 文字颜色

TextFormat.TextColor 和 SetTextColor 使用标准 Go color.Color，而不是 graphics.Color
专用参数；同样必须按其 RGBA 返回值已预乘的事实处理。

- PangoCairo 的 source RGBA、Pango 的 foreground RGB/alpha 属性接收非预乘分量。
- DirectWrite 的原生绘制和离屏文字位图都由 Direct2D brush 接收非预乘分量。
- CoreText 的 CGColor 接收非预乘分量，输出位图保持预乘格式。
- Pango 局部文字颜色必须同时设置 foreground 和 foreground-alpha，不能丢弃 alpha。
- PangoCairo 将 renderer alpha=0 解释为默认值；显式全透明范围使用最小非零
  16 位 alpha，量化到当前 8 位文字位图为零。此转换仅在原生边界，不修改调用者的颜色。
  该行为依据 [PangoCairo 实现](https://github.com/GNOME/pango/blob/1.50.12/pango/pangocairo-render.c)。

文字缓存、生命周期、样式通知不改变。颜色或范围更新继续触发现有 revision 失效。

## 回归验证

- graphics：构造函数、0/低 alpha/半透明/不透明、16 位精度及颜色量化。
- Software：填充、描边、Path、圆角、椭圆、Clear、渐变和退化渐变；对照标准库
  image/draw；透明／白底、1x/2x；图片和阴影作为不重复转换的对照。
- OpenGL：包内原生颜色往返测试；Linux GLX+RGBA8 FBO 实际读回检查 Clear、
  填充、描边、渐变、阴影和图片，1x/2x；不读取遮挡不确定的桌面帧缓冲。
- Direct2D：WARP 离屏预乘位图像素测试及 native 颜色边界测试，不依赖 HWND。
- 三平台文字：离屏检查纯色文字的预乘比例、范围 alpha 覆盖、透明后恢复、1x/2x；
  不使用跨平台字体像素金图。
- GUI：Icon 的半透明像素增加绝对值断言，不仅比较两条可能同时出错的绘制路径。
- 桌面验证：[tests/window/graphics/alpha](../tests/window/graphics/alpha/main.go)，
  注释提供逐项操作与预期。原有测试不迁移，仅修正与既定颜色契约冲突的输入和断言。

本次在 Linux 验证 Software、Pango 和真实 GLX 像素；Windows/macOS 的编译与运行
结果分别记录，不能将交叉编译通过当作其原生渲染验收通过。

### 2026-09-17 验证结果

- Linux Software 全部新增离屏颜色断言通过；Pango 默认／局部颜色及连续 alpha 切换通过。
- Linux GLX 的 52 组 RGBA8 FBO 读回检查通过（1x/2x，Clear 透明开关两种分支），
  原有彩色阴影像素回归通过。新 FBO 诊断通过临时测试覆盖执行，未向包内新增桌面依赖测试。
- graphics、Software、OpenGL 颜色转换、Pango 和 GUI Icon 的针对性 race 测试通过。
- Windows/amd64 的 Direct2D、DirectWrite，macOS/arm64 的 CoreText，以及两平台
  graphics/Software/OpenGL/GUI 测试目标、独立窗口程序交叉编译通过；原生运行尚未执行。
- `go test ./...` 已运行；唯一失败仍为既有 `platform/test.TestApplicationIdentity`
  的冲突 AppID 判空测试，本次未修改该无关路径。

## 图片缩放采样

Software 的整数对齐缩放和通用仿射路径均按设备像素中心映射到源像素中心，
对预乘的 8 位 sRGB 通道做双线性插值，源图边缘钳制；镜像遵守相同映射。
对齐 1:1 绘制保留精确拷贝，不因位置是否整数切换缩放采样模式。
已有资源快照测试的中间样本相应改为独立计算的插值及背景合成值，保留快照／裁剪断言。

后端源码核对：Direct2D 的 DrawBitmap 使用 D2D1_BITMAP_INTERPOLATION_MODE_LINEAR；
OpenGL 创建图片只指定 ImagePreMultiplied，nanovgo v0.0.13 的 gl_backend.go
在未指定 ImageNearest 时使用 GL_LINEAR。此修复不添加采样选项或改变这两条路径。
离屏与窗口用例覆盖整数／小数定位、预乘透明、裁剪和缩放；离屏像素通过不代替桌面呈现验收。
