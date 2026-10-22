# 完整上下文字形与上层折行（已撤回）

> 2026-10-22：本文件以下内容是已撤回的实验设计，不是当前公共 API 或正式控件路径。
> GlyphContext／GlyphLayout／GlyphMetrics、专用绘制通道及 ShapedText／WrappedText
> 已移除。正式设计回到 [现有 TextLayout 契约](DesignIncrementalTextLayout.md#16-收敛到现有-textlayout-契约2026-10-22)。
> 保留实验动机与证据用于解释取舍，不以本文件的接口或性能数字指导新代码接入。

## 目的与边界

2026-10-22。既有完整原生自动折行是长逻辑段落编辑及陌生宽度的主要阻塞点。
本阶段将完整上下文的字形、advance 和视觉顺序保留下来，换宽只重算普通值折点，
详细几何和字形绘制限于需要的显示行。策略在 gui/textedit 及控件，typography
只提供已塑形事实与指定字形范围的光栅化，不接收视口、预算或重排任务。

先落实 Pango；CoreText／DirectWrite 没有对应能力时保持既有准确路径，记录编译与
原生运行缺口。没有通过独立截取子串、方向控制符重放或 ASCII 判断放弃复杂文本。
首次完整塑形、索引构建仍需后续拆分，不能将本阶段称作所有冷调用均已限时。

## 原生事实

新增可选 GlyphContext／GlyphLayout 契约，不改变 TextLayout 的基本职责。
GlyphContext 创建不自动换行的完整输入；GlyphLayout 导出不可变 GlyphMetrics：
逻辑字形簇、advance、段落基方向和原生视觉顺序。TabWidth 记录原生实际的重复 tab 间隔，包括零请求值对应的默认值；上层不能
直接沿用整段原有 Tab advance 作为新行内的 Tab 宽度。不同宽度可以共享同一不可变字形
来源，不同视图独立拥有来源。颜色作为绘制参数，不反复改动原生布局的属性。

RasterizeGlyphs 选择完整 shaping cluster 范围，保留原字形及其内部偏移，将选中字形
按原视觉顺序排列到范围局部坐标原点。region 是这个局部字形平面的物理像素区域；
保持既有预乘 RGBA、缩放、精确 region 大小与缓冲区契约。额外 offset 是物理栅格
像素平移：绘图层保留小数像素相位，避免每个颜色范围独立取整；先在 float64
计算物理像素余量，再转换到局部坐标，不减去两个已舍入的大 float32 坐标。
GlyphMetrics.InkPadding 来自原生字体的 glyph extents，保守覆盖各范围的
斜体、组合符与 fallback 墨迹外伸。该操作画字形，不画下划线
等上层装饰，不独立塑形子串。指定前景覆盖或使用来源颜色，彩色字形仍经原生 renderer。

Pango 从原生 glyph runs 读取字形、log cluster 与整数 advance，不使用可能溢出的
超长单行整体 int32 宽度作为唯一依据。指定范围只复制相应 glyphs 到自有临时
GlyphString，经 PangoCairo show_glyph_string 绘制；所有借用随 native serial 失效。
glyph string、字体、来源布局及 painter 缓存的生命周期和线程保持明确归属。

## 上层折行与几何

ShapedText 拥有完整来源和普通值 WrapMeasurements，引用计数仅在 GUI 线程访问。
WrappedText 是给定宽度的不可变投影：保留逻辑 byte 范围与显示行，不改变模型。
近期宽度共享 ShapedText，几何及原生图像缓存属于各自宽度／painter 的工作集。

折点采用现有 UAX #14 机会及 shaping／grapheme 共同边界，Tab 按新行原点展开。
不插入自动连字符，保留代码空白；不要求复刻原生排版器所有正文排版选择。
连续显示行使用完整来源的 ascent／descent，高度不依赖当前可见子串。

显示行的可见字形顺序取完整原生视觉顺序在本行的限制；对新行尾的空白／格式符
执行 UAX #9 L1，移动到段落基方向的尾部。字形簇和字素内部的原生顺序不拆散，
保留 Arabic joining、连字和组合字符。视觉连续、逻辑覆盖连续且颜色相同的范围
合并绘制；控制符和 Tab 不伪造墨迹。几何和字形使用同一投影。

Fragment 的惰性提供者只保留普通 facts／折行元数据，不通过闭包捕获原生来源或
WrappedText 所有者；来源销毁后仍可查询。Fragment 支持从不可变投影按行取得 cluster 值，Geometry 继续仅缓存四个 32 行页面。
初始化／换宽不建立全段停靠点。绘制遍历 clip 相交显示行，不遍历全部字形。

TextView／SourceView 对至少 32 KiB 且自动换行的段落，在可选原生能力可用时接入。
含 CR、VT、FF、NEL、LS、PS 等强制分隔符的逻辑段落保留原路径；不是按 ASCII
或脚本筛选。空白保留、原生完整上下文字形与逐行方向是明确的新路径契约，不要求
与 Pango 自动折行末尾被收掉的空格逐项相同。格式、IME 投影、文档变更
和销毁按原规则失效。SourceView 的语法颜色保留普通值，不为着色重新塑形整段。
详细几何四个 32 行页面仍可能因一行含大量零 advance cluster 或超大单字素而
产生较多工作，不能将行数限制表述为字符数／时间的硬上限。
本阶段仍需完整输入的冷分析和折点索引；后续将扫描和状态构建分批推进，并实现编辑
后缀收敛。不能仅把不可分的完整调用延后一帧作为该后续目标的完成。

## 参考与验证

沿用 [Scintilla 5.6.7](https://www.scintilla.org/scintilla567.tgz) 的 PositionCache／
WrapLine 分离及 [Avalonia shaped runs](https://github.com/AvaloniaUI/Avalonia/blob/c4dfde67c35e6fb4732d8b4770cae9fbe56957fd/src/Avalonia.Base/Media/TextFormatting/TextFormatterImpl.cs)
的测量复用原则，不复制控件体系。
[Monaco 的 DOM 折点计算](https://github.com/microsoft/vscode/blob/05b953b62cc925ade75c4347ebe11e3bbca65025/src/vs/editor/browser/view/domLineBreaksComputer.ts)
用于核对复用同一份已排版输入的思路；不采用其简单列估计替代复杂文本几何。
GTK4 的可见区／idle 分工及 Qt block 布局的取舍沿用
[既有证据与版本](DesignIncrementalTextLayout.md#145-参考与版本)，不把它们的总体体验
当作 GOUI 已完成有界冷调用的证据。
[CSS Text 3（2026-08-14 草案）](https://www.w3.org/TR/2026/CRD-css-text-3-20260814/#line-break-details)
要求紧急单词内折行保留 joining 上下文、字素完整；此处用于说明选择保留上下文，
不是宣称实现全部 CSS。[UAX #9](https://www.unicode.org/reports/tr9/tr9-52.html#L1)
明确行尾级别重置及逐行重排，不能仅根据方向奇偶猜测原段落状态。
[PangoCairo 字形绘制](https://docs.gtk.org/PangoCairo/func.show_glyph_string.html)
作为原生渲染入口；本机 Pango 1.50.6、Cairo 1.16.0。

回归覆盖字形范围像素、范围边界、透明度、缩放、字体／样式失效、RTL／isolate／
行尾空白、连字、fallback、emoji、Tab、缓存所有权和释放。保留所有原断言。
桌面前后对照使用独立 X11、同样本及真实编辑／陌生宽度拖动，比较 input→Paint、
外部可见等待和 GUI Timer 最大间隔；新文本、新宽度与缓存命中分别报告。
