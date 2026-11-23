# 四角圆角与盒形阴影

## 范围与 API

本切片实现独立四角、统一宽度／颜色的边框和匹配轮廓的 BoxShadow。不引入
PathShadow、四边分色、圆角子树裁剪、CSS selector 或 CPU 位图降级。

`core/geometry.CornerRadius` 包含 TopLeft、TopRight、BottomRight、BottomLeft，
均为物理方位的圆形半径。UniformCornerRadius 为四角相同的简便构造。
Normalize 按相邻半径和与边长的最小比例整体缩放；零／无效尺寸返回零，
负／非有限圆角按零处理。Inset 收缩四个半径，Contains 用有效轮廓命中。

参考 GTK4 4.24.1 的 [RoundedRect](https://docs.gtk.org/gsk4/struct.RoundedRect.html)
及 [CSS 圆角重叠归一化](https://www.w3.org/TR/css-backgrounds-3/#corner-overlap)。
GOUI 采用四个圆形半径，不引入椭圆、百分比或逻辑方位。

Style.CornerRadius 返回完整值及存在标记；Rule.Radius(float32) 和
Rule.CornerRadius(geometry.CornerRadius) 写入同一可选字段，后设置整体覆盖。
显式全零关闭圆角，不逐角继承。Style.Radius 单值 getter 迁移为 CornerRadius；
自定义控件应读取完整值，不能取最大值绘制。Max 仅用于既有保守内容避让。

graphics.Painter／gui.Painter.DrawBoxShadow 的 radius 参数改为 CornerRadius。
统一调用显式使用 UniformCornerRadius。BoxShadow 和 style.Shadow 的颜色、偏移、
模糊、Spread 四个字段不变；FillRoundRect／DrawRoundRect 单半径快捷接口不变。

## 路径和边框

可选择边的扩展见[矩形边框设计](DesignBorderSides.md)。边宽／颜色仍统一，
内轮廓可按选边非对称内缩；内部椭圆计算不改变CornerRadius的公共圆形半径模型。

基础几何属于 geometry；graphics 保留绘制类型、接口和 Path 的指令记录／遍历能力。
图形轮廓生成与曲线换算算法位于 graphics/utils，不由 graphics 包依赖这些工具。
utils.RoundRectPath 是有效的闭合轮廓。utils.RoundRectBorderPath 是外轮廓减去
内缩轮廓的环带；内半径为 max(外半径-width,0)，
再按内矩形归一；内部消失时填满外轮廓。半透明边框一次合成，不覆盖中心的目标像素。

原型验证确认：单一路径往返连接在 NanoVG AA 条带中产生重复合成，不能使用。
补齐现有遍历对多条闭合轮廓的处理；RoundRectBorderPath 生成外轮廓及反向内轮廓，
用 PathHole 标记后者供需要显式孔洞方向的后端消费。其他后端按反向轮廓／奇偶规则处理。
Path.MoveTo 追加轮廓，Path.Hole 记录孔洞标记，不自动反转几何；这两项是通用指令
记录能力。Path.Grow 仅预留额外 float32 记录槽位，工具保留原有48／96槽位分配，
不暴露底层切片，不增加通用路径编辑／填充规则配置接口。
CurrentPos 按记录查询实际当前位置：Close 回到当前轮廓起点，Hole 不改变位置。
未闭合的尾轮廓仍按既有入口规则处理。

二次贝塞尔到三次贝塞尔的换算迁为 utils.QuadBezierTo(path, cx, cy, x, y)，
返回追加后的 Path；Path.BezierTo 仍只记录原生三次曲线指令。调用原
path.QuadBezierTo 的代码改用工具函数，不保留会形成反向依赖的包装入口。

NanoVG 非凸填充改为先绘内部，再用 stencil 消费每个 AA 像素一次，最后清理标记；
半透明曲线凸轮廓也使用该路径，避免小圆角短线段的 AA 三角形重复合成。
不透明凸形及矩形保留快捷路径；没有 FBO 或 CPU 读回。新增像素回归覆盖这项前置。

普通统一圆角保留原生快捷路径；小圆角配粗边框以及四角不同时使用精确环带，
不能机械地把背景路径描边。控件子树仍按矩形裁剪；圆角背景不改变通用命中。
PaintStyledBox 的完整不透明盒形组合采用完整边框底色加内部背景；当前规则见
下方“不透明异色细边框”一节，独立边框和透明组合仍使用上述图元。

### 不透明边框下的背景外缘（2026-11-04）

背景和边框各自对相同外轮廓抗锯齿时，边框部分覆盖像素会透出下层背景，
使浅色边框外侧出现偏蓝或偏暗毛边。这是 GUI 组合两次绘制的策略问题，
不是路径几何或单次 SourceOver 的颜色契约错误。

参考 Flutter 3.38.0 的
[BoxDecoration 背景调整](https://github.com/flutter/flutter/blob/3.38.0/packages/flutter/lib/src/painting/box_decoration.dart#L444-L478)：
仅在实色不透明边框覆盖背景时，将背景退入边框宽度的一半。GOUI 同时从原矩形
归一后的四个半径中减去该内缩量，再按背景矩形归一，保留圆角中心的几何关系。
不使用物理像素经验偏移、不改变边框外形、布局、命中、内容裁剪或样式值。

PaintStyledBox 采用此组合；单独的 PaintStyledBackground 仍绘完整背景，
PaintStyledBorder 仍只绘边框。ButtonGroup 只内缩实际拥有的边，省略的共享尾边
保持背景连续；宿主单独绘背景／边框时复用该规则，已有最终擦除模式保持原有处理。
不增加 Painter 能力、离屏层、CPU 位图缓存或 shader 分支。

背景本身可半透明，其中心 alpha 保持不变。半透明／透明／未设置的边框不缩背景，
保留背景位于边框下方的 SourceOver 语义，不将两个半透明层预混成互斥色块。
此规则不是通用的多层覆盖率合并或子树抗锯齿裁剪。

包内回归以两张独立遮罩筛选4 DIP边框的外轮廓部分覆盖像素，组合结果与边框
单独绘制的预乘 RGBA 比较；同时检查半透明层在直边满覆盖处和中心的 Over。
窗口例程对同一蓝底／浅灰边框加1x／2x颜色断言，实际覆盖 OpenGL 的已报告场景。

### 同色不透明填充的圆角接缝（2026-11-10）

Modern选中／三态CheckButton的背景与1 DIP全边框同色。此前不透明边框的背景
内缩与矩形像素对齐组合后，内背景与边框中心线可能分别取整，造成角内透明洞及
右下轮廓不均。旧实现的Software 1x回归在形内像素得到RGBA(29,74,121,158)，
独立几何预期为完全覆盖的(48,120,196,255)；修改控件颜色或半径不能修复该接缝。

PaintStyledBox在全边框宽度有效、边框不透明、背景与边框完整RGBA值完全相同时，
只填充一次原始外轮廓。比较复用core/colors.Equal的16位分量，支持不同color.Color
实现，不能先量化8位或比较接口身份。保留原半径归一、像素对齐、Transform及clip，
不改变样式参数、Painter接口或后端。异色、选边、透明、半透明组合及独立背景／
边框helper仍走原有规则；本次不是任意两个AA覆盖层的合并方案。

验证范围：

- 包内指令回归覆盖等价颜色类型、16位微小差异、半透明、选边与零边宽；Software
  独立圆方程验证1x/1.25x/1.5x/2x、小数平移、旋转、不同四角及粗细边框。
  整颗形内像素须alpha=255，颜色仅容许1级量化差异；保留外缘AA。
- Modern实际CheckButton覆盖亮暗、选中／三态、正常／悬停／按下／禁用；边缘使用
  独立圆覆盖率，沿用Basic既有10级路径近似容差，角内不放宽alpha断言。既有异色、
  半透明Over、选边及Basic像素回归保持通过。
- 已跟踪源码及本次文件的隔离副本，独立Xvnc/Xfwm4全仓4805通过、11既有条件跳过、
  0失败；定向gui/widgets/theme race与相关vet通过。原工作区`go test ./...`因已有
  未跟踪旧例程的platform.Window接口和ui/window_lookup_test.go旧字段编译失败，
  未删除或修改这些文件。受影响包、Demo及新窗口程序Windows amd64/macOS arm64
  的CGO_ENABLED=0交叉编译通过。
- 新增tests/window/widgets/check_button_corners，真实宿主RenderWidget检查Basic／
  Modern亮暗、状态输入和Snapshot。Linux独立X11、Xfwm4合成、Software/OpenGL
  宿主1x/1.25x/1.5x/2x，Windows 10 19045交互Session 1的Direct2D
  同四档宿主缩放，以及macOS 15.7.7 arm64 OpenGL宿主1x/2x的自动断言通过。
  每次还检查离屏1x/1.25x/1.5x/2x；应用缩放不等同于系统混合DPI。
- Windows SSH Session 0的初次运行没有实际绘制记录，被测试拒绝，未计通过；
  改用交互桌面执行后，已确认实际Direct2D，没有把Software回退计入。
  Windows另在1x实际点击切换亮暗，截图复核选中／三态方框及单选轮廓，并正常关闭。
  证据位于临时目录`/tmp/goui-check-corners-7nkpn2bm`；窗口和任务仅用于本次验证。
- 未执行移动设备、Windows OpenGL、真实硬件GPU和系统混合DPI验收；本次不修复
  异色细边框在任意分数坐标下的背景／描边联合覆盖问题，需另行设计与回归。

### 不透明异色细边框（2026-11-12）

Switch 关闭轨道为不透明灰色背景加 1 DIP 异色边框。旧组合把背景退到描边
中线，再分别对齐矩形并合成；Software 1x 中整颗位于胶囊内的像素 alpha 最低
仅 168/255，叠到浅色页面后形成浅色晕边。只让内缩背景改走未取整 Path 仍有
242/255 的内部像素，说明仅统一坐标不足以消除两次 AA 合成的覆盖缺口。

PaintStyledBox 对完整四边、背景和边框均不透明且异色的组合，先用边框色填充
完整外轮廓，再用背景色填充按完整边宽内缩的真实内轮廓。外框只对齐一次，
内轮廓由同一外框生成 Path，保留分数边宽、四角半径、变换和 clip；内部消失时
只保留外形填充。内部过渡叠在不透明底色上，形内保持 alpha=255。同色规则仍
合并为一次填充；不添加颜色、Painter API、离屏层或逐控件补偿。

Flutter **3.38.0** 的 BoxDecoration 半边宽背景调整仍是半透明背景/选边组合的
参考；本轮实测表明它不能保证 GOUI 异色不透明细边框的形内覆盖，因此完整
不透明盒形采用上述底色策略。透明/半透明颜色、独立选边、单独 Background/
Border helper 及宿主分步绘制保留原有 SourceOver 语义，不将半透明背景垫实。
原生填充和中心线描边可以有不同 AA 曲线；不透明盒形的外缘参考改为同几何的
单次边框色填充，半透明背景仍与独立边框对照。内外过渡仍使用原生 AA/SourceOver，
不承诺任意多轮廓的精确联合面积合成。

新回归用独立物理边界与圆方程检查整颗形内像素，alpha 不设宽松容差；颜色
取合法背景/边框区间，允许1级字节量化。实际 Switch 覆盖 Basic/Modern 明暗、
开启/关闭及正常/悬停/按下/禁用。外缘继续检查颜色、透明度和 AA，保留既有
半透明、选边、变换、裁剪、文字与光标断言；相关指令预期同步为底色/内轮廓。
真实后端、桌面截图、旧版失败与跨平台验证边界见 [Plan](Plan.md)。

## 阴影与后端

阴影先归一源轮廓，按 Offset 平移、Spread 扩缩后重新归一四角；无效原始阴影参数
仍跳过，收缩为空不画。sigma=BlurRadius/2，有限支持为 3 sigma；正模糊最小0.5 DIP。
颜色保持预乘，仅在原生要求非预乘的入口转换。

Software/OpenGL 延续矩形解析积分减四个角缺口的高斯近似，角半径独立。
大半径／小模糊的积分范围限制到像素附近的有效高斯支持，再固定采样，避免跨整个
巨大圆角均匀采样导致漏积分。清晰遮罩用实际四角 Contains，不依赖中心象限选角。

nanovgo 增加四角 BoxShadowVarying；原有 BoxShadow 为统一半径包装。
专用 shader 单次计算，复用阴影分支不使用的 uniform 空间，不创建 FBO／模糊纹理。

Direct2D 的 [Shadow effect](https://learn.microsoft.com/en-us/windows/win32/direct2d/drop-shadow)
消费输入 alpha。录制四角路径，缓存键为宽、高和完整圆角；位置、颜色、模糊不进入
轮廓缓存键。保留16项缓存、失败时完整清晰降级、DPI／设备失效和最终释放规则。

## GUI 消费与边界

公共 PaintStyledBackground／Border／Box 统一使用完整值。ButtonGroup 清零连接处的
角，保留外露角和共享边规则。Popover 的背景、边框、阴影、命中用相同有效轮廓；
BorderBox、ScrollView、Menu、DropDown 更新既有安全内缩。Dialog／Toast 复用 surface。
矩形 ProgressBar 使用四角，圆形进度与语义圆形控件保持原语义。

Native／Integrated 装饰仍归 GUI／平台所有，应用只改变已有允许的窗口背景。
ui 不增加逐 View 圆角属性，使用 StyleName；沿用 StyleChanged 和样式比较失效。
Snapshot 不改变角色／动作，圆角不伪装成新的通用输入裁剪。

### 样式外框对齐（2026-11-22）

PaintStyledBackground／Border／Box 在任何圆角或选边组合下，先用 Painter.SnapRect
得到共同的外框。此前只有统一圆角的矩形原语自动对齐，混合圆角和选边路径仍使用
原布局小数位置，1x 下可使相同的 2 DIP 边宽分布到三行像素，形成不一致的浅色边。

对齐仍只发生在纯平移及 HiDPI 的物理边界；旋转、缩放、错切保留几何。
边框内轮廓从该外框按原边宽生成，不再次量化，1.5 物理像素边宽的内缘仍有半覆盖。
样式策略集中在 GUI，不让各 Painter 的任意 Path 自动取整；Direct2D 的细分与抗锯齿
交由原生绘制处理，见 [绘制契约](DesignGraphicsColor.md#direct2d-原生曲线)。

此修改只影响样式绘制的设备覆盖，不改写 Widget.Rect、布局、命中、Snapshot 或
结构 clip；仍使用原有的保守裁剪包络。共享层 Software 回归按独立边界公式检查
三个入口、统一／混合圆角、完整／选边、半透明／不透明和 1/1.25/1.5/2x；
整数宽度的直边不可分摊，分数宽度不可改成整数。实际 Demo、原生窗口及既有
OpenGL 外缘差异的对照结果见 [Plan](Plan.md)。

## 阶段、验证与迁移

1. 几何、背景／环带路径及退化像素验证。
2. nanovgo 本地测试、提交、新 tag 推送；GOUI 使用正式 tag，无本机 replace。
3. 三阴影后端、接口及调用者迁移。
4. 样式与控件、状态／测量缓存迁移；统一值旧场景回归。
5. 独立 X11 Software/OpenGL、远端 Windows Direct2D/OpenGL、macOS OpenGL。

包内测试覆盖规则覆盖／存在性、几何、独立面积采样和高精度卷积；窗口测试覆盖
预乘半透明、clip、Transform、1/1.25/1.5/2x、连接段 AA、动态切换、Popover 和资源复用。
全仓 go test、相关 race/vet、目标构建；性能对照区分统一值快捷路径和独立四角。
实际执行结果及缺口写入 Plan，按依赖拆分提交；未执行或环境不支持的项目不记为通过。

### 原生像素参考说明

专用shader与Software按独立高斯中心采样比较。初版对Direct2D也强制3/255逐点相等，
远端Windows10在90度旋转下观测到3.255/255差异；精确整数旋转矩阵仍复现，
不归因于浮点旋转，也不因此修改原生效果位置。原生核按DesignShadow原有一物理像素
范围契约，以独立积分的像素足迹包络验收，固定色值余量仍为3/255，并打印实际最大
中心采样差异。具体运行结果、程序版本与是否通过见Plan。
