# 长段落的增量排版与交互

> 目标设计，尚未完整实现。当前仅实现已有度量上的查询与绘制范围优化；
> 首次长段落 shaping／自动换行仍同步。原型及验证见
> [实施记录](PlanIncrementalTextLayout.md)，不能将目标当作当前性能保证。

## 1. 目标与正确性

TextView、TextInput 和第三方文本控件可以延迟显示文字，但排版工作应让出事件
循环，让窗口、滚动条及其他控件继续处理事件。不使用后台测量线程，不在排版
调用栈中递归处理窗口消息。

保留完整段落语义：段落方向、双向嵌入与 isolate、括号解析、弱／中性字符，
上下文塑形、连字、组合字符、emoji、字体 fallback、Tab 和换行。正确性以当前
后端的原生语义为依据，不要求三个后端生成相同像素。固定重叠字数、UTF-8 安全
切点或词边界，均不能单独证明可以创建互相独立的 TextLayout。

不采用“普通代码分片，复杂文本同步整段”的正式控件方案。不关闭 bidi、截断文字、
换成单字符宽度相加，也不靠关闭自动换行隐藏问题。尚不能正确完成的片段不得
作为已确认几何发布。

## 2. 参考与取舍

| 参考 | 核对内容与取舍 |
| --- | --- |
| [VS Code 1.105.0 的折行计算](https://github.com/microsoft/vscode/blob/1.105.0/src/vs/editor/common/viewModel/monospaceLineBreaksComputer.ts)和[配置](https://github.com/microsoft/vscode/blob/1.105.0/src/vs/editor/common/config/editorOptions.ts) | simple 策略使用字符列宽及旧折点；与浏览器 advanced 测量不同。借鉴结果复用，不将列宽估计当作任意原生字体的准确塑形 |
| [Scintilla 5.6.7 源码包](https://www.scintilla.org/scintilla567.tgz) | Editor.cxx 的 WrapLines / WrapOneLine、PositionCache.cxx 的 BreakFinder、Document.cxx 的 SafeSegment。借鉴可视区优先、空闲阶段推进和宽度缓存；单逻辑行与启发式分割不构成通用 Unicode 正确性／单步耗时证明 |
| [Qt QTextLayout](https://doc.qt.io/qt-6/qtextlayout.html) | createLine / beginLayout / cacheEnabled 支持行级构建和缓存；API 外形不保证内部分析能分时 |
| [Pango 1.56.3 itemize.c](https://github.com/GNOME/pango/blob/1.56.3/pango/itemize.c)及[pango-layout.c](https://github.com/GNOME/pango/blob/1.56.3/pango/pango-layout.c) | itemize 对指定范围解析 bidi；Layout 的 check_lines 与 shaping 为同步流程。范围参数不是继承前次分析状态的 continuation |
| [DirectWrite AnalyzeBidi](https://learn.microsoft.com/en-us/windows/win32/api/dwrite/nf-dwrite-idwritetextanalyzer-analyzebidi) | 官方明确不应任意切开段落，否则 bidi level 可能错误；不能独立分析每个窗口 |
| [CoreText 排版流程](https://developer.apple.com/library/archive/documentation/StringsTextFonts/Conceptual/CoreText_Programming/LayoutOperations/LayoutOperations.html) | CTTypesetter 能建议断点、创建行；仍须分别测量初始化与单行操作，不能由此推定段落分析可暂停 |
| [HarfBuzz glyph flags](https://harfbuzz.github.io/harfbuzz-hb-buffer.html#hb-glyph-flags-t)及[Pango shape_item](https://docs.gtk.org/Pango/func.shape_item.html) | shaping 边界与上下文有专门语义。字素边界未必可以拼接缓存 glyph，边界应由后端证明 |

以上版本用于源码核对，本机测量另为 Pango 1.50.6。没有将未运行的
DirectWrite／CoreText 推断写成性能结果。

## 3. 分层与接口

- `platform/typography`：准确分析、shaping、显示行和 glyph 度量、绘制资源。
  不依赖 Model、Widget 或帧调度。
- `gui/textedit`：文档、位置、纯几何查询及高度索引，不拥有原生对象和 Root。
- `gui`：所属 Root 中按帧推进、选择需求、发布结果及布局失效；通用设施供
  第三方文本控件复用。
- SourceView：语法、折叠投影与源码装饰，继续独立于 TextView。

先证明底层操作可正确暂停，再确定最小公共入口。暂不新增内部仍调用整段
NewTextLayout / MeasureMetrics 的 `Advance(deadline)` 来宣称增量排版。
已有 TextLayout 的同步、完整度量契约继续有效。

## 4. 三种范围与可恢复状态

逻辑段落由文本语义确定，不能被预算切开。测量片段是一次推进的工作范围，不改变
段落含义。显示行是当前宽度下的排版输出，可能需要多次片段工作才能确定。

每个视图的待排段落保留：

1. 文本身份、revision、字体／格式／投影版本；
2. 读取和编码映射进度、分析各 pass 的进度及未解决上下文；
3. 已确定方向 level、script、fallback、合法断点和 shaping 安全边界；
4. 已测 glyph／advance、折行候选、Tab 累积位置；
5. 已确认显示行、未确定尾部及待发布几何。

分析可能先扫描到段尾才能确定前面的中性字符或 isolate。此时等待多个事件循环
轮次，不能强制首轮得到首行。超长字素、上下文链和巨大视觉行也不能被当成
“至少完成一次”的无界工作单元。

## 5. 后端门槛

进入控件集成前，逐项证明：

- 段落分析保留上下文并可分步推进；不可拆原生调用分别测量成本。
- shaping 保留方向 level、script、语言、字体和准确上下文，不只保存 RTL bool。
- 片段边界不产生额外断行、连字中断、emoji 拆分或错误行首／行尾形式。
- 显示行 bidi 重排、行末空白处理、Tab 与准确宽度相互一致。
- 度量与绘制使用同一份确认的 glyph，绘制不重新触发整段排版。
- 输入转换、度量导出、几何建立和销毁也有工作量边界。

现有三个高层原生布局对象没有向 GOUI 提供统一的可暂停排版契约。Pango itemize
与 DirectWrite AnalyzeBidi 仍同步处理提供的完整分析范围；CoreText typesetter
初始化的工作量须实测。新增接口或换成一行一调用，不能消除这些限制。

“所有文本都在严格 2 ms 内返回”不是可承诺的原生契约。即使自建可恢复分析，
字体加载、fallback 或不可分 shaping 也需记录最长调用。若仍有随段落规模增长
的同步主步骤，本目标就尚未完成，不能以帧预算配置代替验收。

不为跨过门槛仓促引入另一套字体／Unicode 引擎。若原生公开能力不足，应先证明
补充的通用分析算法及三端 glyph 接口可维护、可校验，再接入控件。

## 6. 事件循环

使用 ConnectFrame 在正常布局／绘制前推进；未完成时 RequestPaint 请求后续帧。
这是现有帧回调，不称作原生 idle priority，不递归 pump。

同 Root 的文本工作共享约 2 ms 软预算，按视图轮转。每个小片段后检查时间、
挂载状态、revision 和需求版本，超出后返回事件循环。512～2048 byte 是实验
起点，不是可独立排版的语义边界；未完成处理保存 continuation。

优先光标／可视区，其次邻近范围，最后远处索引。新滚动目标替换旧需求；
冷跳转可能先等待段落分析，不同步追到目标。多个控件不能各自消耗一份完整预算。
SourceView 着色／折叠也需计入调度审计，但语言扫描可暂停不代表 native layout 可暂停。

Paint、Measure、命中和快照只消费已发布结果，不在查询中补齐整段。外部回调后
检查身份与生命周期。未挂载／隐藏窗口停止推进，重挂载重新登记；原生资源仍在
所属线程释放。不新增通用任务框架或 Root 的 SourceView 特例。

## 7. 缓存、失效与估计

| 数据 | 主要键与失效 |
| --- | --- |
| Unicode／字体分析 | 文本身份、revision、字体环境、范围格式；编辑重算必要上下文 |
| glyph 测量 | 分析状态、字体、script、语言及准确上下文／安全边界；纯宽度变化不默认销毁 |
| 折行／几何 | 测量结果、宽度、wrap、Tab、对齐 |
| 颜色／光栅 | glyph、配色／选择、缩放、像素区域；颜色不反向改变几何 |

换宽复用有效测量，再分步折行。编辑后只有状态与文本映射均收敛才拼接旧后缀，
不能只比较字节。可变进度和原生资源归视图所有，不跨窗口共享可变布局。

未知范围保留高度估计，确认后更新；文本锚点为模型位置加行内偏移，用户新拖动
优先于旧锚点恢复。缓存预算计入源文本、分析、glyph、几何、位图和原生资源，
逐批淘汰，避免帧尾集中销毁。未完成分析不能声称真实总高度已知。

## 8. 编辑契约门槛

当前 SetSelection 同步依据 native cluster 校正位置，删除、命中、IME、锚点也
可能取得布局。若几何未就绪，不能悄悄改成迟到 setter，也不能留下隐藏的同步
整段校正路径。

集成前明确模型合法位置、编辑边界与 shaping cluster 边界的关系；哪些输入依赖
视觉结果；未就绪命中／导航如何暂存、取消并按 revision 重验。不能将“先按 rune
改文本，再修正选区”当成现有 cluster 编辑契约的等价实现。此项与原生可恢复
能力一起验收，不留给 UI 层补偿。

快照只读取有界模型范围，不触发测量。估计的范围不伪装成精确文本矩形，不为
尚未出现的内容提供错误坐标的语义动作。

## 9. 已有度量的查询

Geometry 继续消费完整原生度量。通常显示行逻辑范围按序且仅在软折点相接：
定位先二分候选行，再保留相邻行的 affinity／最近距离比较；Y 查询二分到显示行。
不满足有序条件的数据仍使用原扫描语义，不排序或改写原度量。

Selection 保留完整选区 API。SelectionLines 只生成指定显示行区间的矩形，保留
bidi 不连续矩形与完整高度，由 Painter 最终裁剪。TextView／TextInput 和
SourceView 的选区、搜索及预编辑绘制使用可视行和至多相邻的边界候选行。

此优化不会使 Geometry 构造、原生排版或未换行的巨大行内扫描自动增量化。

已有几何的交互路径也不得无条件使父层尺寸测量失效。TextView／TextInput
移动光标与选择时复用当前有效段落，通过现有滚动机制显示目标；未测量或宽度
失效才请求正常布局。回调重入、浏览锚点与已有布局请求的处理见
[内容测量与失效](DesignTextViewSizing.md#排版与失效)。这是同步缓存复用，
仍不提供未就绪段落的分时命中或排版。

## 10. 验收

同环境／后端／字体／样本对照，分别记录创建、首次测量、度量导出、几何建立、
重排、光栅化、最长单步、事件延迟、首屏及最终完成时间。查询 benchmark 不能
替代首次打开和事件循环验收。

覆盖长词、JSON、比例字体／连字、CJK、组合字符、emoji／ZWJ、混合 bidi、
跨片段嵌入／isolate、括号、Tab、空行、极窄宽度、巨大字素，以及换宽、局部编辑、
取消、换文档、卸载和多视图。与同后端完整参考对照，另保留独立的方向、范围、
边界和 affinity 断言。行、cluster、glyph、命中、caret、选择与绘制一致后才开启
正式分块路径。窗口验收应在排版未完成时拖动滚动条、操作其他控件、输入和关闭。
