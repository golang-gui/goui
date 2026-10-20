# 可视区缓存与分步换宽排版

> 目标规格及当前入口，实施状态见 [长段落记录](PlanIncrementalTextLayout.md)。
> 已加载文本的换宽重排优先；首次分析的耗时另外验收。

## 目标与边界

保持完整段落语义，沿用原生字体、双向文字、cluster、Tab、换行与绘制契约。
窗口换宽时保留有效测量，分步生成新折行，优先发布可视区。允许文字延迟显示，
不把重排搬到后台线程，不在事件处理中递归 pump，也不截取独立子串冒充段落。

与 [通用增量排版](DesignIncrementalTextLayout.md) 配合：本阶段先验证已分析段落
的换宽复用，不以完成所有首次加载的可恢复分析作为开始原型的前提；也不能因此
把未解决的首次分析成本藏入换宽、绘制、命中或 Advance。

## 三类缓存

| 缓存 | 数据与键 | 换宽 |
| --- | --- | --- |
| 分析、测量 | 文本身份/版本、字体环境、影响测量的格式、完整方向 level、script、合法断点、glyph 和 advance、准确上下文 | 保留有效结果；折点附近按后端语义局部重测 |
| 折行索引 | 测量身份、有效宽度、Wrap、Tab、对齐；行文本范围、高度、累计 Y、继续状态 | 新建代次，旧坐标不能当新坐标 |
| 详细显示页 | 折行代次、显示行范围、glyph 绘制资源、cluster/命中几何；光栅另含颜色/缩放/像素区域 | 按新代次生成，发布前保留仍显示的旧页 |

当前 TextCluster 的 X/Y/LineIndex 已绑定显示行，不能直接充当宽度无关测量。
颜色不使分析和折行失效；字体、文本或测量格式变化按实际依赖失效。宽度相同而
视口高度变化只改变需求与缓存覆盖，不重新测量已有文字。

详细页以完整显示行为边界，目标覆盖当前屏与上下各一屏，加正在使用的 caret/
IME 目标。页面是缓存和调度单位，不是 bidi/shaping 上下文边界。资源暂不敏感，
优先避免反复丢弃昂贵测量；内存上限、LRU 和页面钉住仍须明确且可诊断。

## 可恢复折行

待排段落保留文本/格式身份、目标宽度/代次、已处理文本位置、当前行累计宽度与
候选断点、已确认显示行索引、未完成尾部。一次推进只处理有限测量 cluster 或
断点，不能把完整逻辑行/巨大 run 作为最小工作单元。

内部操作契约如下，原型通过前不据此发布公共类型：

1. 更新目标宽度只登记代次与需求，不同步整段重排。
2. 推进返回实际进度并保留 continuation；没有完成不等于失败。
3. 查询只读取已发布缓存，缺失返回 ready=false，不隐式补排。
4. 详细页使用已确认 glyph 资源，不以子串 NewTextLayout 重新塑形。
5. 取消旧工作后保留仍有效的测量；结果发布前核对全部身份与生命周期。

每约 128 显示行保存检查点作为原型起点：代次、行序号、文本位置、准确累计高度
和继续状态。同宽可从有效检查点继续；换宽后旧检查点的折行状态/Y 失效，旧折点
只能作为经过验证的计算提示。文本身份和状态均收敛才允许拼接旧后缀。

## 换宽、跳转与发布

换宽时保存段落身份、文本位置和行内偏移的顶部锚点；取消过期宽度，保留测量，
先完成锚点可视区及必要前置索引，再生成上下页。连续缩放只追最新宽度，不补做
所有中间尺寸。有效宽度统一扣除 padding/侧栏/滚动条，避免测量与分配之间来回
失效；真正不同的候选宽度不能被错误当成同一个缓存键。

新可视区尚未完成时保留旧宽内容并按当前窗口裁剪，新增区域可暂空。旧显示使用
旧几何；新可视区的绘制、命中、光标、IME 及高度信息成组发布，不混用版本。
显式用户滚动优先于旧锚点恢复，宿主整数 DIP 钳制与内容偏移保持一致。

远处跳转先以高度估计响应滚动条，再分步完成目标依赖的折行索引和详细页。
准确显示行的起点依赖前面的折行；可视区优先不意味着可以从任意字节直接排一屏。
旧宽索引不能伪装成新宽准确总高度。高度修正保持文本锚点，避免逐帧来回跳动。

## 事件循环和交互

沿用 ConnectFrame，在布局/绘制前推进；同 Root 文本工作共享约 2 ms 软预算并
轮转，每个小批次后检查耗时。优先交互目标、可视区、滚动前方、另一侧、远处索引。
不新增通用任务系统，不向 Root 加入具体控件特例。

预算不能抢占原生调用。记录最长不可分调用；若换宽仍反复执行随整段规模增长的
同步分析或 shaping，就没有通过本阶段。未挂载/隐藏时停止，销毁在所属线程释放。

选择、闪烁不使测量失效。输入使用实际显示版本的几何；现有 SetSelection 的同步
cluster 校正不能静默改为迟到 setter。编辑/预编辑变化取消相关工作，原型验证
cluster 边界、软折点 affinity 和 IME 映射后才能接入正式交互路径。

## 后端与两仓职责

Pango 验证保留 item analysis、断点与 glyph，再进行分步折行和必要的上下文重塑；
DirectWrite 验证 TextAnalyzer 的 script/bidi/断点/glyph placement；CoreText 验证
保留 typesetter 后建议断点、逐行创建及初始化/单步成本。它们都是候选路径，
不能仅由 API 名称推定正确性或延迟。

GOUI platform 承担准确分析/折行/显示行资源与绘制，GUI 承担需求、帧推进及 TextView
集成，textedit 保留模型和纯几何。SourceView 通过公共契约接入，继续拥有语法、
折叠、行号和装饰，不复制 TextView 缓存。应用层不增加虚拟排版开关。

涉及绘制契约时检查 Software/OpenGL/Direct2D，不能以 Paint 内回到完整原生布局
来实现显示页。按实际原型需要确定最小公共入口，避免先增加一组空接口。

## 实施与验收

顺序：后端原型 → TextView 可视区/换宽/锚点 → SourceView → 首次分析的继续优化。
每阶段分别记录实现和缺口，按独立功能提交；不能把设计文档或编译成功当成窗口
响应目标完成。

相同环境/字体/后端/样本比较分析、shaping、折行、度量导出、几何、绘制及输入
延迟。样本包括长词、JSON、CJK、比例字体、连字、组合字、emoji、Tab、混合 bidi
和跨页嵌入/isolate。与同后端完整原生参考比较文本范围、行/cluster、命中、caret、
选区和像素，并保留独立语义断言。

窗口覆盖连续/反向缩放、视口高度变化、远跳、滚动、点击、选择、编辑、IME、
取消、卸载和关闭；文字未完成时其他控件应可操作。记录首屏等待、最长同步调用、
事件延迟、缓存峰值。Windows/macOS 未原生运行时明确保留缺口。

## 当前入口与阶段边界

第一步使用可选的 `typography.ReflowLayout`，保留 `TextLayout` 完整度量契约：

- `Reflow(width, height) bool` 只登记请求；不支持时返回 false，不修改当前布局。
- `AdvanceReflow(deadline) (done, err)` 分步推进；旧 Text/Format/Size、绘制与度量
  在完整结果准备好之前保持一致。完成时更新 box 和度量，同步发出 Changed。
- `CancelReflow()` 丢弃待发布结果，保留当前布局及仍有效的测量。
- 正常 `SetSize` 仍同步；显式修改测量格式会取消旧请求。无变化的 box 不丢弃
  已发布的测量；颜色更新只使字形绘制页失效。

`gui.DeferTextLayout(owner, layout, width, height, ready)` 是第三方控件也可使用
的薄调度入口，返回取消函数与是否受理。按 Root 共享约 2 ms 软预算，每个布局
约 250 μs 后轮转；卸载／原生布局销毁立即取消并解除连接，隐藏时暂停。完成
回调重入、替换任务或销毁控件后，不继续发布旧回调。未受理时调用方使用普通
SetSize；首次加载不隐式切换为这种优化。

Pango 保留完整段落的 logical attrs、原生 item/font 与 glyph/advance；只在
折点处带原始段落上下文塑形。绘制颜色先从测量属性中去除，再用
`pango_glyph_item_apply_attrs` 按 cluster 重新施加。原生 item 与 glyph 必须
独立持有；不构造缺少 Pango 私有字段的 item 来执行 copy/split。原生布局与
快照均归单一 TextLayout 所有，不跨视图共享可变状态。此入口需要 Pango 1.50+。

当前仅 Pango 实现该能力，Begin 对齐、WrapChar／WrapWordChar、默认或自定义
Tab、Unicode／bidi 和字体范围参与验证。placeholder、下划线、删除线及其他对齐
暂不受理，沿用完整原生路径；不是按 ASCII／复杂 Unicode 划分准确性。
DirectWrite／CoreText 仍沿用原实现，尚未获得换宽分帧优化。

绘制使用 `pango_cairo_font_get_scaled_font` 与 Cairo positioned glyph，颜色和
alpha 只来自当前绘制上下文；缺字框通过 Pango glyph-string path 保留原生轮廓。
不使用 Pango 1.50.6 的 show_glyph_string 便捷 renderer：其清除 color 后仍保留
上一次布局的 alpha，详见
[pangocairo-render.c](https://github.com/GNOME/pango/blob/1.50.6/pango/pangocairo-render.c#L840)。
用透明普通布局与另一个 prepared 布局交错绘制验证对象隔离，不以全局重置或
额外空文本绘制修补原生共享状态。

本阶段先完成整段折行及 cluster 索引后发布，原生 glyph 资源只缓存请求区域及
上下各一屏内的显示行，离开范围时释放，滚回时从保留分析再生成。
折行与 cluster 元数据仍完整保留；TextView 按需构造 Geometry，SourceView 在
完成回调中构造 Geometry。这不是目标中的详细几何分页与新首屏提前发布。

首次请求中构建完整原生分析、单条显示行／巨大 cluster 的处理、Geometry 构造
与实际绘制仍不可抢占。2 ms 是软预算，不能承诺任意输入均不超时；这些工作
继续按上文目标拆分。该入口避免把未完成度量伪装成完整 TextLayout，也使当前
绘制后端无须引入假的只读 TextLayout 或偷偷重建整段。

## 参考

- [Scintilla 缓存](https://www.scintilla.org/ScintillaDoc.html#SCI_SETLAYOUTCACHE)：
  借鉴可见页/caret 缓存，不将逻辑行缓存当作超长段落分时证明。
- [VS Code 1.105.0 折行复用](https://github.com/microsoft/vscode/blob/1.105.0/src/vs/editor/common/viewModel/monospaceLineBreaksComputer.ts)：
  借鉴旧折点提示，不照搬等宽列模型作为任意原生字体的测量。
- [Pango shape_full](https://docs.gtk.org/Pango/func.shape_full.html)、
  [shape_item](https://docs.gtk.org/Pango/func.shape_item.html)：保留段落上下文。
- [DirectWrite AnalyzeBidi](https://learn.microsoft.com/en-us/windows/win32/api/dwrite/nf-dwrite-idwritetextanalyzer-analyzebidi)：
  不任意截断段落进行方向分析。
- [CoreText 排版](https://developer.apple.com/library/archive/documentation/StringsTextFonts/Conceptual/CoreText_Programming/LayoutOperations/LayoutOperations.html)：
  验证 typesetter 行级操作，不据此承诺可抢占的原生分析。
