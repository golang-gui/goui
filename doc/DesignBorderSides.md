# 可选择边的矩形边框

## 定位

应用需要只绘底边、两条平行边、L形或三边框。保留统一宽度、统一颜色的简单
样式模型，仅增加物理方位的边集合；不增加四边分色、分宽、CSS BorderSide、
PathShadow、软件位图降级或新的 Painter 原语。

## 样式接口

```go
type BorderSides uint8

const (
    BorderTop BorderSides = 1 << iota
    BorderRight
    BorderBottom
    BorderLeft
    BorderAll = BorderTop | BorderRight | BorderBottom | BorderLeft
)

func (r Rule) BorderSides(sides BorderSides) Rule
func (s Style) BorderSides() (BorderSides, bool)
```

```go
style.Name("underline").BorderWidth(1).BorderColor(color.Gray{Y: 140}).
    BorderSides(style.BorderBottom)
style.Name("rail").BorderSides(style.BorderLeft | style.BorderRight)
style.Name("open-frame").Radius(8).BorderSides(style.BorderAll &^ style.BorderBottom)
```

未设置沿用四边，显式0关闭边框。集合是一个可选原子值：状态、Part和后规则
整体替换，不按位继承；未知位忽略。宽度／颜色／圆角继续独立解析。

此字段控制矩形盒形装饰，不改变 Measure、Arrange、Padding、命中或子树裁剪。
各控件既有内容避让策略保持；不把缺边变成隐式布局设置。圆形 Radio／Switch
指示器、TableView的网格线等有自己的几何语义，不把四边开关应用于圆或网格策略。
Native／Integrated的系统边框仍由平台／GUI私有样式管理。

## 圆角与绘制

外轮廓仍为完整 CornerRadius 圆角矩形。启用的边以 width 内缩，缺失的边内缩0。
边框为外轮廓减去内轮廓，一次 FillPath 合成；不用四次描线，避免转角重复透明度。

内角X半径减去左／右内缩，Y半径减去上／下内缩，分别钳到0。
任一轴归零则该内角为直角；相邻内半径超出内矩形时整体归一。
两条启用边之间保留圆角；启用边与缺边相邻时，沿圆角自然收窄到缺边。
这不同于在圆角切点硬截断的独立线段。粗边框仅在实际选边的内缩使内部消失时
填满外轮廓；单上边不能因为width达到半高就提前填满整个矩形。

参考 Flutter **3.32.0** 的
[BoxBorder.paintNonUniformBorder／_deflateRRect源码](https://github.com/flutter/flutter/blob/3.32.0/packages/flutter/lib/src/painting/box_border.dart#L291-L374)：
分别内缩每边，构造椭圆内角，以外／内轮廓绘制。GOUI仅采用该几何方法；
样式仍只允许0或同一宽度，不引入Flutter的完整BorderSide抽象。
椭圆只是内部计算，公共CornerRadius仍是四个圆形半径。

轮廓算法位于graphics/utils：RoundRectInsetPath、RoundRectBorderInsetsPath
接收geometry.Insets，不依赖style。非零内缩必须相等，不借工具接口另行承诺四边不同宽度。
四边都有内缩时使用外闭合轮廓加反向内轮廓／Hole；
缺边时将每段连续启用边的外、内边界连接成闭合区域，两条平行边是两个区域。
避免内外轮廓在缺边处重合，导致stencil渲染的AA条带歧义或接缝漏画。
所有区域仍在一次FillPath中合成，复用Software、NanoVG、Direct2D现有能力。
四边完整时保留既有原生快捷路径。

### OpenGL收尖覆盖率（2026-11-06）

自然收尖形成的亚像素窄区域会暴露NanoVG传统“半像素内缩＋边缘羽化”的限制。
单上边框、16DIP圆角、4DIP边宽、1x时，尖端某像素的独立椭圆面积积分预期
alpha约2/255，Software为2，旧OpenGL为242；相邻轮廓外像素旧OpenGL为239。
这不是正常的1x阶梯或颜色对比问题，降低边框Alpha只会减弱错误的可见程度。
上游作者也说明了这一[内缩覆盖率限制](https://github.com/memononen/nanovg/issues/415)。

修正在nanovgo，不改变BorderSides的形状或增加GUI后端补偿。对存在尖锐凸转角、
边数不超过128的简单／互不相交实心轮廓，在原有填充与AA片元中按每条线段
与物理像素方块的有向面积积分计算覆盖率。命令复制原始边坐标，避免后续路径
缓存复用污染排队命令；普通绘制前恢复覆盖uniform，Cancel／Flush重用批次存储。
继续使用原画刷、剪裁和SourceOver／DestinationOut，不新增绘制pass或离屏资源。

128边表适配GL3.3／ES3.0保证的fragment uniform预算。自交、相接、嵌套、孔洞、
更大路径及旧WebGL shader保留原算法，不扩大为任意Path的精确覆盖承诺；积分
基于细分后的多边形，曲线逼近误差仍由既有细分精度决定。

GUI边框例程单独测尖端及其外侧，平台圆角例程用独立椭圆积分验证1x／1.25x／
1.5x／2x、旋转、clip、透明／实色／渐变和ClearPath，并检查后续绘制的状态恢复。
旧依赖必须在相同断言下失败。验收与耗时对照见Plan。

超粗选边侵入另一端的圆角时，单纯连接两条边界可能跨出外轮廓。该退化场景
将闭合区域与凸的外轮廓相交；曲线以1/64 DIP平坦度细分后作凸多边形裁切，
仍是一次FillPath，没有离屏位图、GPU读回或额外Painter裁剪状态。
常用边宽保留原三次曲线，不承担多边形相交的计算。

不透明边框的组合背景仅沿启用边退入半个边宽，以遮住背景外缘AA；缺边坐标保持。
半透明边框下保留完整背景；显式无边框不收缩背景。独立PaintStyledBackground仍
绘完整背景。ButtonGroup将样式边集合与本段拥有的边求交，焦点使用自己的集合，
共享接缝角保持直角；不另保留一套三边轮廓。Popover复用相同规则。

## 验证

- style包：未设置／显式0、原子覆盖、状态／Part、规则比较和未知位。
- graphics/utils：统一内缩兼容、非对称内角控制点、无效和消失轮廓。
- GUI离屏Software：16种组合、圆角／直角、不透明／半透明、背景、粗边框、
  clip、旋转、1x／1.25x／2x；用独立椭圆方程及标准库Over计算内部像素预期。
  超粗选边另检查外部透明像素，确保选边和背景均不穿出对面圆角。
- tests/window/gui/border_sides：真实后端离屏像素断言与可操作的控件／Popover；
  同时回归corner_radius例程的共享接缝、焦点与按压。
- 本机独立X11和远端Windows／macOS的结果、平台条件与验证缺口记录在Plan。
