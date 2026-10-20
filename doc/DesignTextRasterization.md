# 文本区域光栅化

## 问题与职责

超长单行和窄视口中的长段落都可能产生超过原生 image surface 限制的完整位图。
先生成全文位图再裁剪不能消除该失败。通用修正在 Typography 和 Painter，TextInput、
TextView、Label 和第三方文本控件复用正常 DrawTextLayout 路径。

Typography 保留完整布局，在其坐标系中绘制指定区域；Painter 选择可见区域、处理
变换、像素对齐和图片缓存。控件继续负责自己的文本、排版约束、滚动和结构裁剪。
不切片字符串重新 shaping，不增加 SourceView 特例，不将 Direct2D 改为位图绘制。

## Rasterize 契约

直接修改既有入口，不并行维护 RasterizeRegion：

```go
Rasterize(scale float32, region image.Rectangle, buf []byte) (TextBitmap, error)
```

- scale 有限且大于零，表示每个布局逻辑单位的像素数。
- region 在布局的光栅像素平面中；布局 (0,0) 对应光栅 (0,0)，不是窗口或屏幕坐标。
  Min 包含、Max 不包含，允许负坐标。
- 返回 Width/Height 严格等于 region.Dx/Dy，位图局部 (0,0) 对应 region.Min；
  字形未覆盖的像素透明。后端不收紧、扩大或重新定位输出区域，因此不增加 Origin。
- 有效 scale 下，空或非规范化的空矩形返回空位图、nil；非空区域即使布局无字形，
  也返回完整透明像素。零矩形没有“自动输出全部”的含义。
- 区域不会改变布局、断行、字形位置或基线。布局约束用于排版，不隐式裁剪绘制；
  调用方通过 region/Painter clip 控制可见性。
- 8-bit 预乘 RGBA、紧密行 stride、调用方拥有返回像素和 buf 复用语义不变。
- 在分配前校验尺寸和乘法溢出，单次输出存储上限 64 MiB。原生 surface 还可能有
  更小的维度限制；超限及创建/绘制失败返回错误，不能发布半初始化资源。

MeasureMetrics 使用原布局坐标。Pango、DirectWrite 不再减去 occupied extents 的起点；
否则 center/end 对齐或 RTL 的度量与区域绘制不一致。MeasureSize 仍返回逻辑占用尺寸，
不是含负向 overhang 的完整墨迹边界。

## Painter、缓存与像素对齐

Software/OpenGL 按设备倍率乘以 Transform 的最大拉伸确定 rasterScale。先对整个
布局绘制原点做一次设备像素对齐，再逆变换有效 clip 的四角得到布局光栅区域。
有效 clip 是当前 target 与显式 clip 的交集；空 clip 不发起光栅化。

区域向外取整、预留采样边缘，并按 1024 像素块扩展以复用小范围滚动的图片。
目前用逻辑占用高度、布局宽度及溢出的实测宽度限制区域；它不是完整 ink-bounds API，
超出逻辑范围的极端字体 overhang 仍需独立设计。没有将全部 Cluster 度量查询加入 Paint。

原生请求每块最多 1024×1024 像素；先按统一像素网格复制到一张区域位图，再上传图片。
这样不需要分别绘制带抗锯齿边缘的相邻四边形，也不需要扩展 NanoVG 的私有接口。
整张区域输出仍受 64 MiB 及后端图片/纹理限制；不声称支持任意近奇异 Transform
或任意尺寸离屏目标。

区域放置为：

```text
regionOrigin = snappedLayoutOrigin + region.Min / rasterScale
```

不再次对 regionOrigin 取整。裁剪窗口变化不会改变原生字形的像素相位。
缓存键为布局对象身份、精确 rasterScale、实际 region。每布局最多四张图片，LRU；
单张最多 64 MiB 的 RGBA 输入，未新增全 Painter 字节预算。布局 changed 清空图片，
destroy 删除记录并断开订阅；不同 Painter 的资源独立。临时拼接像素属于 Painter，
原生 surface 仍属于 TextLayout，不共享不同视图的可变排版资源。

## 原生处理与失败恢复

- Pango/Cairo：按 region.Min 做像素平移，再按 scale 缩放，直接绘制原 PangoLayout。
  不调用 UpdateLayout，不将 logical extents 起点移至输出原点。
- CoreText：将既有 CTFrame 的向上 Y 轴和布局向下 Y 轴转换到缓冲区顶部；复制本次
  请求的行，与保留缓冲区高度无关。排版不再使用 20,000 点固定高度，无折行布局先
  求自然宽度，保留完整文本的 frame，再在区域中绘制。
- DirectWrite/WIC：固定目标 DPI=96，用矩阵表达 scale 和整数像素平移。原生
  Direct2D Painter 仍直接绘制布局，由 Painter clip 控制可见性，不额外用布局框裁剪
  溢出的无折行文本。

扩容先创建候选工作区；全部成功后才替换旧工作区。失败释放候选，保留原可用状态。
Destroy 同时清零原生句柄、位图及尺寸。绘制出现 sticky native error 时丢弃该工作区，
下一次重新创建；不能因上次记录的尺寸而跳过初始化。初始化途中取得的资源都要释放。

## 参考与验证边界

- [Qt 6.12.0 PaintContext.clip](https://doc.qt.io/qt-6/qabstracttextdocumentlayout-paintcontext.html#clip-var)
  将文档区域作为绘制提示，不能代替 Painter clip。采用区域选择与实际裁剪的职责区分，
  不照搬其文档对象体系。
- [PangoCairo.show_layout](https://docs.gtk.org/PangoCairo/func.show_layout.html)
  （页面 Library 1.58.2，Linux 实测 Pango 1.50.6）从 Cairo 当前点绘制既有布局；[Cairo translate](https://www.cairographics.org/manual/cairo-Transformations.html#cairo-translate)
  修改坐标变换。采用在完整排版结果上平移/裁剪的方式。
- [Direct2D DrawTextLayout](https://learn.microsoft.com/en-us/windows/win32/api/d2d1/nf-d2d1-id2d1rendertarget-drawtextlayout)
  （页面更新 2021-10-13，2026-10-20 查阅）以 origin 放置布局，CLIP 是显式选项；
  EndDraw 报告延迟绘制错误。

这些文档支持职责与原生用法，不保证 GOUI 的像素或资源实现正确。原生回归逐后端
验证负坐标、透明区域、caller buffer、分数 scale、区域/完整位图对应像素、对齐及
长文本尾部，并检查光栅化前后的 Line/Cluster。失败恢复另用 Cairo 实际超限请求。
GUI 离屏回归覆盖两个文本控件；桌面验证与环境记录见 PlanTextRasterization.md。
