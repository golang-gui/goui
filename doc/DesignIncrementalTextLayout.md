# 文本片段测量、缓存与可视区布局

> 2026-10-21：普通值片段及详细几何分页已经接入，见第 12 节。
> 有限送测仍为实验；控件仍同步测量整段。Reflow 保持撤回，公共 TextLayout 契约不变。
> 未标为已实现的可视区测量、预算调度和增量布局仍是目标。
> 历史实验和回退验证见 [实施记录](PlanIncrementalTextLayout.md)。
> 自然尺寸查询与实际视区测量已分离：大 TextView 的 Measure 只读高度估计，
> 换宽保留旧高度，详情见文本编辑设计及实施记录；这不等于有限段落送测已实现。

## 1. 目标与分层

TextView、TextInput 与 SourceView 面对超长文本时允许延迟呈现，优先保证滚动、
缩放、其他控件输入和关闭仍可处理。利用事件循环间隔，不使用后台测量线程或
递归 pump。测量工作首先限制送入的文本量，再考虑单轮时间预算。

TextLayout 保持同步、简单：文字、格式和约束输入，排版、度量和渲染输出。
不感知视口、缓存页、Model、滚动锚点、帧预算或任务取消，不新增 Reflow／Advance。

| 层 | 职责 |
| --- | --- |
| platform/typography | 现有 TextLayout，保持三个后端的测量和绘制契约 |
| gui/textedit | 纯数据的片段、范围映射、高度组合、几何查询与结果复用，不持有窗口或原生资源 |
| gui | TextView／TextInput 的需求、原生片段资源、帧推进、生命周期和输入集成 |
| SourceView | 复用公开值设施，独立管理语法、折叠、侧栏、装饰和控件状态 |

不预建新的文本引擎、通用任务系统或一组 provider。公共 API 只提取两控件已验证
的共用值与运算，不公开 GOUI 私有 textEditor。共享 GUI 调度确有需要时也放 gui。

## 2. 单位与片段数据

区分逻辑段落、测量片段、显示行和缓存页。片段是一次有限测量的范围；页是几屏
资源的保留／淘汰范围，二者都不自动成为文本语义边界。一屏可由多个片段组成。

上层至少保存以下数据；这是内部职责，不是已发布类型：

| 数据 | 内容 |
| --- | --- |
| 测量片段 | 模型范围、实际送测范围、文本及格式身份、测量参数、只读行／cluster 副本、局部尺寸和边界确认状态 |
| 布局索引 | 宽度代次、片段顺序、准确／估计高度、累计 Y、确认的软折点和检查点 |
| 详细几何 | 当前几屏及 caret／IME 目标的 Geometry，不构造整段 Geometry |
| 绘制资源 | 可见及预取片段的 TextLayout、颜色状态及现有 Painter 缓存 |
| 待处理需求 | 请求代次、目标范围、继续位置、优先级，归 GUI 层 |

坐标以片段局部值保存，通过索引组合为控件坐标；高度变化不逐个改写后续 Y。
使用稀疏索引和累计高度，不为未知文本预分配逐字符或逐显示行对象。

## 3. 普通值缓存与有效输入

MeasureMetrics 的 TextLine／TextCluster 可以复制缓存，包括 TextLine.Clusters
切片。副本由片段拥有，原生布局销毁后继续有效。Geometry 引用这份只读数据。
淘汰原生对象不等于丢弃测量结果。

相同输入和字体环境可以复用。键必须包含文本身份／范围内容版本、影响测量的
字体范围、字体环境／Context 代次、Tab、placeholder、投影、实际上下文。
带折行坐标的结果另含有效宽度、Wrap、对齐及相关边界状态。
不能只用全局 Model.Revision，让一字编辑清空所有未变片段。

同宽同输入直接复用。换宽保留旧折点、cluster 宽度和实测密度作为新布局的提示，
但不把 TextCluster.X/Y/LineIndex 或 Width 当成无条件宽度无关的 advance。
Tab、连字、行首／行尾变化在有限范围内实际重测校验。

度量不含 glyph，不能直接拿 TextCluster 绘字。可见片段保留小 TextLayout；
滚回时可以用相同输入重建，并核对度量身份，避免文字与命中数据不一致。

## 4. 首次呈现

1. 读取目标附近的有限文本，取得或复用样本行高、平均宽度及换行密度。初始以
   512–2048 UTF-8 字节为工作量级，后续按耗时调整；它不是语义安全切点。
2. 估计一屏所需文本及未知范围高度。不能为了平均值先扫描／复制整个文档。
3. 用正常 TextLayout 测量有界候选范围。不能给完整长段落设置一个小 maxHeight
   就声称只测可见区域；高度约束不是分析量约束。
4. 保存结果，确认可以衔接的显示行；候选输入结束不是真正段末时，不能只因
   末行存在就发布它。未确认尾部保留为后续输入。
5. 依据实际覆盖修正范围及高度，补齐当前屏，再处理上下预取区。每个小片段后
   检查预算，不能在一次 Paint 或布局查询中追算完整段落。

状态区分未测、估计、待确认、已确认。估计可用于选择工作和滚动范围，不能冒充
caret／IME 的准确几何。缺少准确结果的查询返回未就绪，不隐式补排整段。

## 5. 片段边界正确性

可缓存普通值，不代表任意子串都与原段落排版等价。这是片段衔接的验证问题，
不扩大为后端缓存或调度的要求。

候选范围沿确认的软折点及合法文本边界组织，必要时带相邻文本测量，并保留原始
范围映射。UTF-8／字素完整是最低条件；固定重叠量、空白或两次测量相同，均不能
单独证明完整 bidi／上下文塑形等价。末行之外的行也需上下文已经确定。

必须验证段落基础方向、跨片段嵌入／isolate、中性及配对字符、上下文文字形式、
连字、组合字／emoji、Tab 原点、软折点 affinity。补充文本与确认过程分次推进，
不能同步反复扩大到段尾。已失败的跨 4 KiB RLO 样本保留为回归门槛。

当前 TextLayout 没有显式接收已解析段落方向／塑形状态的参数。因此“所有复杂
段落都能独立截取后准确拼接”仍未证明。本阶段不改 typography；先用现有能力
验证缓存及布局，并列出不能准确衔接的条件与样本。未确认结果不发布，不回退
同步整段，也不能将无限等待写成完整支持。若准确性与响应目标不能同时满足，
报告具体缺口后另行设计，不借此重新引入后端调度或宣布通用方案已完成。

## 6. 同宽滚动与远跳

同宽滚动只改变需求窗口。测量缓存命中不调用 MeasureMetrics；绘制资源也命中时
不创建布局。进入未缓存区域允许暂空，先准备目标片段，再预取滚动方向邻近内容。

远跳先用估计高度响应滚动条，再从附近有效检查点准备目标。估计位置只能作为
候选，不能把任意字节当作准确显示行起点。必要前置索引分步确认，不同步从段首
追算到目标。未知全局 Y 时可以模型位置建立局部锚点；局部几何确认后成组发布，
累计文档高度仍标为估计。

高度修正保持文本位置及屏幕内偏移，通过现有 ScrollView／ScrollIntoView 契约
恢复锚点。用户新滚动覆盖旧恢复请求；端点钳制不反复争抢偏移。相邻确认的行组
可以先发布，无须等待整段完成；绘制、命中、几何和局部高度使用同一代结果。

## 7. 换宽和局部编辑

换宽只登记最新宽度／需求代次，取消旧工作，保留有效测量及临时显示快照。
用旧折点和实测密度预测新范围，先准备锚点附近几屏，随后补远处索引。
旧内容暂显时仍用旧几何，不把旧坐标标为新宽结果。连续缩放不补做中间宽度。
高度改变仅调整工作集，不使同宽测量失效。有效宽度统一扣除 padding、侧栏和
滚动条，保留“点击不在两种宽度间反复测量”的已有回归。

编辑按实际 Edit 顺序重映射范围，从最近有效边界开始更新。未改且上下文不受
影响的片段保留身份。后续文本、格式、宽度和入口状态收敛才复用旧后缀，不能
只比较字节或碰巧相同的行宽。高度变化更新索引路径。最坏可传播到段尾，但传播
跨帧进行；LF 插删、字体和投影修改各自明确影响范围。

文档编辑、撤销与信号维持同步契约，排版可以稍后显示。点击已显示区域直接使用
片段 Geometry，不构造整个逻辑段落几何。待就绪的命中／视觉导航记录修订及需求
代次，新输入、滚动、换文档使旧请求失效。

SetSelection 的 cluster 校正不能偷偷改成迟到 setter。原型需要分别明确模型
合法位置、字素、塑形 cluster 和视觉停靠点，验收删除、左右／上下导航、IME。
不能先按 rune 修改再修正选择来冒充现有语义。

## 8. 缓存量、生命周期和事件循环

详细几何及绘制资源初始覆盖当前一屏、上下各一屏及 caret／IME 目标；测量值
保留更久。测量值软预算初定 16 MiB，按原型调整，可见集超出时记录峰值。
原生资源按覆盖和对象数管理，不用输入字节冒充原生内存。逐批淘汰不可见原生
对象，再淘汰详细几何，最后按 LRU 淘汰测量值；不每帧扫描整个缓存。

原生对象归当前控件及 Context，在所属线程创建／销毁。卸载、Context 改变和
关闭先取消任务及信号，再释放资源。不同视图只共享模型，不共享可变布局和进度。

使用 ConnectFrame／RequestPaint。约 2 ms 为同 Root 全部文本布局合计的初始
软预算，多个控件公平轮转；每个小片段后检查时间及生命周期。GUI 层如需共享
helper，只处理文本工作的预算与轮转，不扩展 Root 为具体控件的任务中心。
优先交互目标、当前屏、滚动方向预取、另一侧、远处索引。隐藏停止主动推进，
恢复根据最新需求继续。外部回调返回后重验修订、格式、宽度、挂载和销毁状态。

预算不能抢占原生调用，需要记录最大单步并按实测缩小后续送测量。一个逻辑段落
不算有界工作；字体冷加载和巨大 cluster 单独记录，不能承诺所有输入严格 2 ms。
Paint、Measure、快照和命中只读缓存或估计，不隐藏整段同步工作。

## 9. 控件集成和实施顺序

TextView 接入纵向片段及可变行高；SourceView 复用共同值设施，映射折叠／IME
投影后应用语法与装饰，不继承 TextView；TextInput 与不换行 TextView 按水平
范围管理片段，额外验证累计 advance、基线和 Tab 原点。未知最大宽度标为估计
或下界，不为内容尺寸精确值强制测全文。区域 Rasterize 继续复用。

实施次序：纯数据片段／索引原型 → 有限 TextLayout 测量与边界校验 → TextView
视口／预算／输入 → 确定最小共享 API 并接入 SourceView → 单行水平路径 →
增量后缀、淘汰和着色失效收口。当前阶段与验证边界见第 12 节。

## 10. 验收

普通值在布局销毁后仍可用；同宽滚回复用；高度变化不重测；换宽／远跳只追最新
目标；一字编辑保留无关片段；取消、重入、多视图隔离和资源释放有独立断言。
颜色不使上层度量失效，可见重绘不重新送测整段。

长词、JSON、CJK、比例字体、连字、Tab、RTL／isolate、组合字、emoji／ZWJ、
窄宽及巨大字素与完整同后端参考比较范围、行、cluster、caret、命中、选区和
像素，另有独立边界预期。不得放宽旧断言，不只比较两条可能同时错误的路径。

同环境对照 32 KiB、用户只读 JSON、MB 级单段和多段；扩大前先验证模型与读取
成本。分别记录每片段输入量／测量、复制、Geometry、绘制、首屏、最长事件间隔
和内存峰值。正常窗口事件在排版未完成时滚动、点击其他控件、拖宽和关闭；
定时器间隔不是显示合成延迟。三个后端分别验证，交叉编译不等于原生验收。

## 11. 依据与取舍

- [VS Code 1.105.0 换行计算](https://github.com/microsoft/vscode/blob/1.105.0/src/vs/editor/common/viewModel/monospaceLineBreaksComputer.ts)：复用旧折点，不照搬等宽列模型作为通用度量。
- [同版可见行渲染](https://github.com/microsoft/vscode/blob/1.105.0/src/vs/editor/browser/viewParts/viewLines/viewLines.ts#L566)：详细呈现以视口为中心，不声称全文索引也仅计算一屏。
- [Scintilla 布局缓存](https://www.scintilla.org/ScintillaDoc.html#SCI_SETLAYOUTCACHE)：借鉴页／caret 工作集，逻辑行缓存不直接解决超长单段。
- [Pango shape_full](https://docs.gtk.org/Pango/func.shape_full.html)及[DirectWrite AnalyzeBidi](https://learn.microsoft.com/en-us/windows/win32/api/dwrite/nf-dwrite-idwritetextanalyzer-analyzebidi)：上下文用于制定片段校验门槛，不作为本轮新增后端接口的要求。

以上为文档／源码依据；片段算法、预算和缓存量是本项目待验证的选择，实测另记。

## 12. 普通值片段和详细几何分页（已实现）

`gui/textedit.NewFragment(range, occupiedSize, lines, clusters)` 验证并复制普通
度量，返回不持有原生对象的 Fragment。输入的度量坐标和 byte offset 相对于
该测量输入，range 则记录它在父文本中的位置。空的度量不能冒充非空文本已就绪。
嵌套 Clusters 和独立 cluster 切片都不借用调用方的存储；两者同时给出时采用独立
cluster 切片作为规范输入。

- Range／Origin 把片段本地位置映射回原输入；不插入换行或改变模型。
- LineRange 返回指定完整显示行的只读视图，与父快照共享不可变存储。
- Metrics 导出独立的本地坐标副本，工作量只涉及该视图。
- Geometry 创建可缓存的纯 Go 编辑索引；一个快照创建的两个 Geometry 不共享
  可变的详细页缓存。原生布局修改／销毁后快照仍有效。

显示行及实际停靠范围严格有序时，超过 32 行的 Geometry 按需建立每页 32 行的
停靠点、排序及边界索引，每个 Geometry 保留最多 4 页。行查询先使用普通元数据，
点击／caret 准备相关页，选择先限制逻辑及显示行范围，不遍历远处详细页。
软折点两侧 affinity、跨页视觉导航及逻辑删除均保留原语义。
重复空行／停靠范围不严格有序等一般度量保留旧 Geometry 行为，不能为了分页
改变最近停靠点的平局选择。未换行的巨大单行仍可能在一页中含大量 cluster。

TextView／TextInput 在首次需要编辑几何时复制快照，普通显示不提前支付复制成本；
重新测量后使旧快照和详细几何失效。SourceView 原本创建段落时建立完整 Geometry，
现改为复制快照后创建分页几何。两控件仍使用原来的原生布局进行绘制，未独立截取
子串渲染，所以此阶段不改变文字 shaping、bidi 或像素。

快照创建仍需复制和检查传入的完整度量；缓存原生资源的现有逻辑仍按段落管理。
尚未完成：有界读取／送测的正式路径、按帧测量预算、范围测量 LRU、换宽行组提前
发布、增量后缀及水平测量片段。不能将几何分页解释为首次测量已经不阻塞。

SourceView 的测试内实验使用 512／2048 字节候选，保留末行继续测量，确认了长词
和 JSON 的行／cluster 可由普通值组合。跨 4 KiB RLO 的独立窗口会丢失 override，
有明确防误用断言；该实验不在正式控件路径启用。耗时和未完成项见实施记录。

## 14. 度量索引与换行分离（2026-10-22）

Scintilla 5.6.7 的 PositionCache／EditView 先保留已塑形 advance，再由 WrapLine
计算折点；宽度改变不必丢弃字符位置。采用这一分离，但不把其等宽 ASCII 优化作为
复杂文字的替代。Monaco 的 simple 列数模型不适合作为 GOUI 通用文本的准确几何；
advanced DOM 路径的二分查询复用同一个已排版 DOM，不等于重复原生排版不同前缀。

### 14.1 普通值索引

`gui/textedit.WrapMeasurements` 拷贝完整段落的逻辑 cluster advance，保留 UTF-8
边界、前缀和、字素及 UAX #14 机会。它不持有原生对象，不读取视口。多个
`WrapIterator` 独立前进，单个 `Next` 返回一个候选行，可在任一候选后让出事件循环。
测量输入允许视觉顺序，构造时按逻辑范围规范化；字体 fallback 拆出的 cluster
还须与字素边界求交，不能拆开 emoji／ZWJ。Tab 在每个新显示行的原点重新展开。

宽度变化仅新建 iterator，二分查询前缀 advance，不重新逐字测量。宽度超过一行
的单个完整 cluster 仍整体保留，确保进度；零宽前导控制符不形成一个空白候选行。
候选不是最终坐标：行端重塑形、软连字符、双向行重排、对齐及绘制仍须使用同一份
已确认的最终行。不能把逻辑 advance 直接当作 visual X，更不能用候选结果定位 caret。

此索引已经有独立预期的测试和基准，但**尚未接入控件的正式换行路径**。构造索引
仍需扫描／复制完整传入度量；它也不解决首次原生分析。本节不承诺超长段落已经分帧。
上层缓存的键、预算、销毁和上下文失效继续遵守第 3、7、8 节。

### 14.2 原生度量导出

Pango 从原生已塑形 GlyphString 的视觉 glyph 顺序累计整数 advance，以原生 run
原点、对齐和 bidi 分析为基准，替换逐 cluster 的迭代／extents 调用。不是逐字符
独立测量；字体、Tab、连字及行尾处理均保留原生结果。输出仍是独立普通值。

DirectWrite 一次取得范围命中框和 cluster 度量。仅在框边界完整、累计 advance
与框宽在原生 float32 精度内一致时，推导内部 cluster 原点；不能覆盖的框继续
使用原生位置查询。原点由 float64 前缀累计后转为 float32，保留原生 cluster
advance，避免在超长不换行文本上累计 float32 游标而漂移。普通边界的舍入处理
属于 textedit.Geometry 的停靠点索引，不能改写原生测量值来掩盖键盘导航问题。
两端对齐可能单独调整内部位置，保持原生逐位置查询。完整 bidi、inline placeholder、
换行及 UTF-8／UTF-16 转换仍按原生事实处理。

两项已经接入实际 TextLayout；TextView、TextInput、SourceView 自动受益。
CoreText 已有按 run 批量取得 positions／advances／indices 的路径，本轮不重复改写。
公共 TextLayout 契约不变，原生后端不感知模型、视口、任务预算或交互需求。

### 14.3 区域栅格化

Pango 依据原生行 ink 范围选择能触及 Rasterize region 的行，保留完整段落分析和
原生行对象，只缩小绘制遍历。按实际像素缩放保留一个物理像素的抗锯齿边缘；
字形负 bearing、斜体及超出逻辑行框的墨迹不能仅用显示行高裁掉。
借用原生行指针的缓存属于 TextLayout，另用 Pango layout serial 识别原生 context
变化；比较相等性而不比较大小，避免序号回绕。任何布局或样式变化先失效缓存，再通知监听者，
销毁先丢弃借用指针再释放原生布局。不与其他 TextLayout 共享。

批量度量已经访问行范围及基线时，同步保留原生行及 ink 数据，避免绘制再次遍历；
未查询度量的调用仍可在首次栅格化准备数据。缓存随所属布局释放。

逐行绘制以矩阵平移保留原始分数基线，并清除 Cairo current point，避免 MoveTo
在设备像素平面量化后改变分块间的 glyph phase。与原整段渲染在相同后端和字体下
逐像素比较，覆盖 1／1.5／2 倍、负 region、混合文字、对齐、字体及范围样式变更。
这是同步区域渲染优化，不把原生整段冷排版变成有界任务。

### 14.4 上下文测量前置

新增的 Linux 绑定仅表达 `pango_itemize`、`pango_shape_with_flags` 和对应资源所有权。
先对完整段落分析，保留字体／脚本／bidi analysis，再用同一原生文本缓冲塑形有限范围，
可以避免方向控制符前缀回放丢失非局部上下文的问题。必须使用实际 context 的 round
positions 标志，不能把分数宽度与原来的取整宽度混用。

CoreText 已通过保留完整 attributed string 的 CTTypesetter 创建有限 CTLine 的
原生对照：长 RLO／LRO／isolate、混合文字、完整 Arabic 词、ffi 和组合字的目标
glyph、advance、原始索引与方向同完整 CTFrame 一致。调用者拥有 typesetter／line
引用，先释放这些引用，再销毁文字及 Context；没有截取后重新创建小 attributed string。

目前仅为平台实验：它不证明任意字节切点安全，也未形成三后端公共范围布局 API。
完整 itemization 仍是一次不可抢占调用；MB 级输入的冷分析、复杂 cluster 边界、
行端确认、DirectWrite 对应事实及可渲染片段仍需验证。**控件当前仍会同步排完整长段落**。
后续通用扩展只提供必要的分析／塑形事实，缓存、折行、视口和事件间隔推进留在上层；
不恢复 Reflow，不以后台线程或延后一帧的完整测量替代有界工作。

### 14.5 参考与版本

- [Scintilla 5.6.7 官方源码](https://www.scintilla.org/scintilla567.tgz)：
  `src/PositionCache.cxx`、`src/EditView.cxx`、`src/Editor.cxx` 的宽度缓存、
  WrapLine 及 visible／idle wrapping。未采用旧 GitHub mirror 的版本号替代 5.6.7。
- [VS Code 换行计算](https://github.com/microsoft/vscode/blob/05b953b62cc925ade75c4347ebe11e3bbca65025/src/vs/editor/common/viewModel/monospaceLineBreaksComputer.ts)、
  [DOM 换行计算](https://github.com/microsoft/vscode/blob/05b953b62cc925ade75c4347ebe11e3bbca65025/src/vs/editor/browser/view/domLineBreaksComputer.ts)：
  采用复用度量／折点原则，不复制其 DOM、简单列模型及超长行呈现限制。
- [Pango shape_with_flags](https://docs.gtk.org/Pango/func.shape_with_flags.html)
  明确接受 item 子串与段落上下文，API 自 1.44 可用；本机实测 1.50.6，在线文档 1.58.2。
- [CTTypesetterCreateLine](https://developer.apple.com/documentation/coretext/cttypesettercreateline(_:_:))
  表达原 attributed string 内的范围；本轮只记录上述已测条件，不推广为任意切点安全。
- [DirectWrite HitTestTextRange](https://learn.microsoft.com/en-us/windows/win32/api/dwrite/nf-dwrite-idwritetextlayout-hittesttextrange)
  提供范围命中框；内部 advance 推导是本项目经原生位置对照验证的选择，不是文档保证。
- [PangoCairo show_layout_line](https://docs.gtk.org/PangoCairo/func.show_layout_line.html)、
  [LayoutIter 行范围](https://docs.gtk.org/Pango/method.LayoutIter.get_line_extents.html)
  确定绘制原点和 layout-local ink 坐标；
  [Layout serial](https://docs.gtk.org/Pango/method.Layout.get_serial.html)
  用于识别原生 context 改变，不假定借用行指针永久有效。

[Qt block 布局](https://github.com/qt/qtbase/blob/4053f2e82f776bb58ace844d7793f6f8304e2534/src/widgets/widgets/qplaintextedit.cpp)、
[Avalonia shaped-run／line-break 缓存](https://github.com/AvaloniaUI/Avalonia/blob/c4dfde67c35e6fb4732d8b4770cae9fbe56957fd/src/Avalonia.Base/Media/TextFormatting/TextFormatterImpl.cs)
及 [GTK4 可见区／idle 验证](https://github.com/GNOME/gtk/blob/3b438f0576193689bf39835174a51aa747333281/gtk/gtktextview.c)
支持分离工作的方向，但不能证明一个超长原生段落已被切成有界调用。本轮实测
与失败项记录在实施文档，不由参考框架的总体流畅度推断 GOUI 的冷调用上限。

## 15. 避免重复分配宽度与保留近期结果（2026-10-22）

这一阶段减少完整段落布局的重复调用，不改变 TextLayout，不将估计坐标用于编辑。
第 14 节的有限上下文塑形、普通值折点索引及真正有界的行组准备仍需继续集成。

### 15.1 首次滚动范围的有限样本

单逻辑段落超过 4096 byte、自动换行且尚无已知高度时，TextView 的自然尺寸查询
只截取至多 512 byte、位于 UTF-8 rune 边界的前缀，以当前字体、换行模式和候选宽度
作一次原生测量。用样本的宽度或换行后高度与长度比估计总高度；零 advance 的样本
至少保留正常一行高。样本按本次输入／格式的高度索引重置失效，用后立即销毁。

目的在于让 ScrollView 首次分配时就有合理的滚动范围：否则先按“一行高”分配完整
宽度，昂贵的完整测量让滚动条出现，再按减少后的宽度重复折行。自然尺寸查询不改变
实际 layoutWidth；常规 LayoutVisible 仍在最终分配宽度创建完整上下文的准确布局。

样本只估计滚动范围，既不发布文本片段，也不用于断行、caret、选择、命中或绘制。
其截点无需被误认为合法 shaping 边界。字体分布、长组合序列及后缀都可能使估计
不准确；确切测量必须无条件替换它，也必须能移除已不需要的滚动条。不额外扫描
全文件、多段落或完整 grapheme。TextInput、不换行文本和 IME 投影不走这一估计。

### 15.2 长段落的近期宽度与历史共用缓存

长度大于 4096 byte 且不超过现有 1 MiB 保留预算的已测段落，换宽时从活动缓存移到
既有近期输入缓存。保留原生布局、普通值行／cluster、已构建的编辑 Geometry 和确切
高度；它们共同描述同一份输入和宽度。恢复相同完整文本及原生盒宽时直接转移回活动
缓存，恢复 HeightIndex 和 observedWidth，不再 SetSize、MeasureSize 或 MeasureMetrics。

保留区和编辑历史共用最多四个输入、合计 1 MiB 源文本的预算，不为每个拖动宽度建
独立无界缓存；活动区仍遵守原预算，当前工作集可被固定。该 byte 上限不是原生内存
或所有几何数组的上限。小段落继续复用单个布局并 SetSize，TextInput 不新增换宽缓存。

同一原生对象只能属于活动区或保留区之一，不同编辑器不共享可变资源。完整文本
匹配防止换宽恢复旧编辑结果；字体、换行、模型重置、上下文释放和卸载继续清理全部
资源，前景颜色变化也释放保留区，避免恢复旧颜色。逐出前先解除所有权，销毁回调
返回后检查模型、修订、挂载与当前宽度，不在重入后发布旧状态。

采用第 14.5 节 Scintilla 的有界可见布局缓存、Monaco 的测量复用原则；当前以少量
完整上下文快照换取近期宽度的直接复用，避免在尚未确认的子串边界上重新 shaping。
这不是跨宽度复用 advance 的最终方案，新宽度和新文本仍可能阻塞。真实拖动会经过
许多未命中宽度，应与反复最大化／还原两个确切宽度分别验收，不能混报收益。

具体前后对照和剩余失败见 [分配与宽度复用验证](PlanTextLayoutAllocation.md)。


## 16. 收敛到现有 TextLayout 契约（2026-10-22）

撤回本日引入的 GlyphContext、GlyphLayout、GlyphMetrics 和专用字形绘制通道。
完整段落字形复用曾降低实测开销，但这是一条具体实现路线，不是分块、缓存和可视区
优化的必要前提，不能由这条路线倒推一个并行的底层公共协议。

### 16.1 正式路径与保留能力

Context.NewTextLayout 继续是唯一创建入口。TextView、TextInput 和 SourceView
使用现有 TextLayout 的测量、范围样式、尺寸变更及区域 Rasterize；Painter 只通过
原有 DrawTextLayout 绘制。没有另设可选工厂、原生字形句柄或专用图像缓存。

上层保留文本范围、实际宽度／格式身份、原生布局及普通值测量结果。Fragment 复制
TextLine／TextCluster，原生对象变化或销毁不影响快照；Geometry 按原有页面缓存
详细命中和停靠数据。近期宽度、编辑历史、初始高度估计、滚动锚点及生命周期规则
继续采用第 12–15 节的实现。不同宽度与视图不共享可变 TextLayout。

SourceView 的语法及选区前景继续经 SetTextColor 设置，缓存有效颜色范围以避免
重复属性更新；颜色的逻辑缓存不能被表述为后端保证永不重新塑形。
Pango GetRuns 不再按段落字节数预分配 run 容量的修正保留；它是独立的分配修正，
不引入排版策略或公共接口。

### 16.2 后续优化的约束与当前限制

测量结果可以作为普通值复用，也可以用来预测候选折点和送测范围。但需要重新绘制
时仍使用描述同一输入、格式与宽度的 TextLayout，不能仅凭坐标推定字形可随意搬移。
有限文本送测、换宽后的可视区准备及编辑后缀收敛遵循第 3–8 节，尚未完成的能力
继续记为目标。任意子串不自动继承完整 bidi、连字或组合上下文。

本次是接口边界修正，不是新的性能提升。新文本和未命中宽度仍可能同步完成长段落
排版；恢复近期准确宽度和历史输入可直接复用。完整塑形复用分支此前取得的耗时
不再代表当前正式路径。事件循环间隔不能抢占一次完整原生调用，也不能通过将它
推迟一帧就宣称已消除阻塞。

专门验证被撤回协议的测试随协议移除；普通 TextLayout 的区域光栅化、像素、
Fragment、复杂文本及缓存回归保留。补充超过原 32 KiB 分支阈值的完整上下文、
资源隔离、宽度恢复与快照生命周期回归，不削弱已有准确性断言。
具体撤回与实测记录见 [实施记录](PlanShapedWrapping.md)。


### 16.3 TextView 连续分配的上层合并

移除字形分支后的真实拖宽检查发现，TextView 对每个中间宽度同步重新排版，会失去
足够的中间视区帧。为避免重复工作，在现有 gui/textEditor 增加分配合并，不改变
任何 typography 或 Painter 接口。

仅当已挂载、有现成准确布局、可见段落至少 32 KiB、自动换行且无 IME 投影时生效。
陌生宽度先保留当前布局、测量值和 Geometry，用新视区裁剪旧呈现；既有 GUI Timer
在最新宽度稳定 150 ms 后只请求正常布局。Timer 不调用原生排版，新的分配仍能覆盖
已就绪但尚未执行的旧宽度。高度变化不重启等待；返回当前或已保留的准确宽度立即
恢复，不等待 Timer。无可用 Timer 时维持同步路径，不丢失布局请求。

延迟期间仍以旧几何命中，不发布旧坐标为新宽度结果；快照的 goui.text 属性明确
报告 layoutPending=true 和实际 layoutWidth。待完成后撤去该属性。现有交互观察
程序读取这个状态，拖动期间可以观察旧呈现的帧，但最终耗时必须等待新宽度完成。
旧宽度不临时制造横向滚动条。编辑、换模型、测量格式重置、卸载和销毁停止请求，
已经投递的旧 Timer 也不得发布结果；隐藏时停止等待，恢复分配后按最新需求处理。

这只合并连续约束，不是有界原生测量：宽度稳定后那次完整调用仍可能阻塞，冷编辑
开销也未消除。响应性与最终文本就绪分别记录，不能将旧快照的快速绘制当作新布局
已经完成。确定性 Timer 测试覆盖中间宽度不测量、只执行最新宽度、准确缓存立即
恢复、快照状态及已投递回调在编辑／换模型／卸载后失效。
