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
