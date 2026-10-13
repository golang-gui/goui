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

Direct2D 原生参数依据：[画刷颜色](https://learn.microsoft.com/en-us/windows/win32/direct2d/direct2d-brushes-overview)、
[Clear](https://learn.microsoft.com/en-us/windows/win32/api/d2d1/nf-d2d1-id2d1rendertarget-clear%28constd2d1_color_f%29)、
[渐变端点](https://learn.microsoft.com/en-us/windows/win32/api/d2d1_1/nf-d2d1_1-id2d1devicecontext-creategradientstopcollection)。
目标位图采用预乘格式，不代表颜色参数也采用预乘格式。

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
