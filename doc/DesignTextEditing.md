# 文本编辑：TextModel、TextView 与 TextInput

> 状态：首版代码及开发验证已完成；其他平台窗口用例交用户验收，不阻塞后续开发。
> 首版是通用纯文本编辑器，不是代码编辑器。本文记录目标契约、实际验证和已知限制。
> 参考定位：Scintilla 为主，Monaco／VSCode 为辅；保留现有 Piece Tree，不引入原生编辑器依赖。

## 1. 目标与参考

先完成多行 TextView，再将验证过的编辑逻辑复用到 TextInput。支持选择、光标导航、
Clipboard、IME、滚动、undo/redo；规模目标是 1～10 MiB、数万行的常规多段落文档。
超长单行或单段落不承诺同等性能，本轮不引入局部文字栅格化。语法高亮、多光标、
富文本、文件编码、语言服务和代码补全不属于首版。

以 Scintilla 的原生编辑控件架构、低延迟交互和缓存失效策略为主要参考，以
Monaco／VSCode 的增量文本存储和编辑历史为辅助参考。参考用于选择具体机制，
不改变 GOUI 的分层、Cluster、线程亲和及资源隔离契约，也不扩展首版功能边界。

采用的参考（Scintilla 源码核对版本为 5.6.6）：

- [Scintilla Component Design](https://www.scintilla.org/Design.html)：
  主要参考 Document/Editor 分离、多视图共享文档和变更通知，以及平台无关编辑核心
  与原生事件接入的分工。该页面含旧实现说明，只作架构背景；具体机制以版本源码为准。
- [Scintilla 5.6.6 官方源码包](https://www.scintilla.org/scintilla566.tgz)：
  `src/PositionCache.h` 的 `LineLayout::ValidLevel` 及 `src/PositionCache.cxx`
  的 `LineLayout::Invalidate`、`LineLayoutCache::Retrieve`、`PositionCache::MeasureWidths`
  是分层失效、行布局复用和短文本位置缓存的实现参考；不将这些机制直接等同于
  GOUI 的段落 LRU，也不将其缓存键移植到不同字体、后端或视图之间。
  `win32/ScintillaWin.cxx` 的组合输入处理提供 Windows 原生 IME 接入参考，不能
  外推为 macOS/Linux 的行为；GOUI 仍通过 platform → Window → IMContext 接入。
- [Scintilla 布局缓存契约](https://www.scintilla.org/ScintillaDoc.html#SCI_SETLAYOUTCACHE)：
  对比光标行、可见区域和全文缓存的内存/重复计算取舍；GOUI 不照搬缓存模式 API。
  [idle styling](https://www.scintilla.org/ScintillaDoc.html#SCI_SETIDLESTYLING)
  是语法样式工作的调度机制，不代表 GOUI 已有后台排版；
  [布局线程](https://www.scintilla.org/ScintillaDoc.html#SCI_SETLAYOUTTHREADS)
  依赖线程安全测量能力，不作为放宽 GOUI 原生 TextLayout 线程亲和的依据。
- [Monaco v0.52.2](https://github.com/microsoft/monaco-editor/blob/v0.52.2/README.md)
  作为增量模型与编辑体验的辅助参考；Model/Editor 分离与 Scintilla 的文档/视图
  分离共同支持现有共享模型设计，不引入 URI 注册、浏览器渲染体系和语言服务。
- [VSCode 1.21 的 piece tree 说明](https://code.visualstudio.com/blogs/2018/03/23/text-buffer-reimplementation)
  与 [1.104.3 编辑历史源码](https://github.com/microsoft/vscode/blob/1.104.3/src/vs/editor/common/model/editStack.ts)：
  采用增量文本存储、差量历史和撤销组，不照搬 JavaScript UTF-16 索引。
- [GTK4 TextView 文档](https://docs.gtk.org/gtk4/class.TextView.html)
  （设计时文档版本 4.23.4）：采用独立模型和可滚动内容组合，不复制 GTK 的完整类型体系。
- [uniseg v0.4.7 的词边界实现](https://github.com/rivo/uniseg/blob/v0.4.7/word.go)：
  仅用于按词导航和选择；不以 Unicode 字素分段替代 GOUI 的 Cluster 契约。
- [GTK 4.18.6 GtkText 的光标移动实现](https://github.com/GNOME/gtk/blob/4.18.6/gtk/gtktext.c#L4008)：
  普通左右键收起选择时比较端点的视觉位置，而不是字节大小。GOUI 在共享编辑核心
  中采用该区分，并按显示行及段落顺序处理跨行选区；不搬入 GTK 的强/弱光标系统。

低延迟与可控资源占用是设计目标，而不是框架间已验证的性能排名。Piece Tree 在
VSCode 的特定工作负载中改善了存储和编辑成本，不意味着所有大文件操作都更快；
Scintilla 的缓存和原生接入经验也不证明 GOUI 已达到相同响应时间。最终以相同
环境、文档和操作下的 GOUI 测量为准。以上参考不代表对应运行场景已经验收。

## 2. 归属与分层

| 对象 | 拥有 | 不负责 |
| --- | --- | --- |
| textedit.Model（gui.TextModel 类型别名） | 文本、逻辑行索引、版本、历史及编辑选区元数据 | 字体、Cluster、活动视图光标、原生 IME |
| 每个视图的私有编辑状态 | Anchor/Caret、导航状态、预编辑 | 共享文本存储 |
| 每个视图的排版缓存 | TextLayout、Cluster 几何、段落尺寸 | 其他视图的资源 |
| ScrollView | 偏移、滚轮、滚动条 | 文本编辑策略 |

模型不依赖原生排版。平台只报告文字、排版和输入事实；编辑行为属于 GUI。
UI 只是声明式适配，不另建编辑状态机。真实和模拟按键/鼠标经过既有事件分发路径。
原生文本提交继续通过现有 InputMethod/IMContext 路径，不从物理按键推导字符。

### 独立文本编辑包

与 Widget 无关的实现位于 `gui/textedit`，依赖方向为
`gui -> gui/textedit -> core / platform/typography`。目录位于 gui 下不代表反向
依赖 gui；与作为上层适配的 gui/dev 不同，textedit 是被控件使用的算法包。
不导入 gui、ui、原生窗口或具体排版后端，不要求 Application 或桌面会话。

- `Model / Range / Selection / Edit / Change`：共享文本模型、UTF-8 字节位置、
  增量通知和历史；piece tree、文本块及历史栈保持私有。
- `Geometry / Position`：消费调用方传入的 typography 行与 Cluster 度量，
  提供命中、边界吸附、视觉导航和选区矩形，不创建或销毁 TextLayout。
- `HeightIndex`：段落高度估计、测量更新、索引和累计高度。
- `Projection`：视图独立的预编辑文本投影、段落查询及显示位置到模型位置映射；
  不连接原生 IME，不修改模型或历史，模型变化后由 GUI 丢弃。
- 文本规范化、单词边界和编辑位置映射也位于该包；WordAt 不替代 Cluster，
  GUI 使用分词结果时仍按当前排版吸附到 Cluster 边界。

GUI 的私有 textEditor 保留事件解释、焦点、Clipboard、IMContext、Timer、
样式、原生排版缓存、绘制、视口锚定与滚动协调，以及 Widget 生命周期和 Snapshot。
不为拆包引入模拟 Widget 的宿主接口。不同视图只共享 Model，不共享 Projection、
Geometry、HeightIndex 或原生排版资源；所有可变对象仍要求顺序访问。

编辑入口 `Model.Apply(Edit, EditOptions)` 接收独立 `EditSource` 身份标记、
原子历史策略和编辑前选区；模型复制选区并记录编辑后的插入末尾位置。
切换来源中断历史合并，模型不持有 Widget 或引用 Widget 的闭包。
`UndoWithSelection / RedoWithSelection` 返回操作是否发生、该操作的 Revision
及可选选区副本，GUI 不再访问历史栈。信号可重入修改模型或卸载控件，因此 GUI
恢复选区前仍检查生命周期、模型身份与结果版本；返回值不冒充回调后的最新状态。

`gui.TextModel / TextRange / TextSelection / TextEdit / TextChange` 保留为类型
别名，`gui.NewTextModel` 仅转发构造；现有 UI 和应用不必迁移。需要独立使用时可
直接调用 `textedit.NewModel`，不建立第二套模型。

纯模型、存储、几何和高度索引测试随实现迁入新包；混合测试文件中的纯模型通知
与大选区投影用例一并迁移。GUI 集成测试保留，用公共查询或实际撤销结果代替
跨包读取私有字段，并补充来源隔离、选区副本、重入版本与投影映射的公开 API 测试。

拆包验证（Linux amd64）：

- `env -u DISPLAY -u WAYLAND_DISPLAY go test -count=1 ./gui/textedit` 通过；
  `go list -deps ./gui/textedit` 确认不依赖 gui、ui 或具体平台后端。
- `go test ./...`、`go test -race ./gui/textedit ./gui ./ui` 通过。
- Windows amd64 的 gui、ui 测试目标交叉编译通过；macOS arm64 的 textedit
  测试目标交叉编译通过。
- macOS 完整 GUI 交叉编译未通过：`GOOS=darwin GOARCH=arm64 go test -c ./gui`
  在未改动的 `platform/darwin/cocoa/platform_darwin.go:56` 起报告
  `undefined: AutoReleasePool`、`NSApplicationClassId` 等名称。
  单独执行 `GOOS=darwin GOARCH=arm64 go build ./platform/darwin/cocoa`
  可复现同样错误，不涉及本轮 gui/textedit 拆分；未顺带修改平台绑定。
- 本轮不新增窗口功能，未执行桌面操作验收；既有 GUI/UI 窗口用例保留供回归。

Linux XIM 在服务端公布支持时选择 `XIMPreeditCallbacks`，将按字符索引的增量
预编辑转换成已有 UTF-8 `InputMethodPreedit`。原生回调只复制数据，离开
`XFilterEvent`/`Xutf8LookupString` 后才通知 GUI，允许信号回调重置或销毁上下文。
不支持回调时保留 root-style/none；不能从字符内容猜测组合输入。平台通过
`InputMethodResult.Composed` 报告已识别的原生组合提交，Window 原样转入
`IMContext.ConnectCommit(func(IMCommit))`；编辑器据此决定独立撤销步骤。
false 只表示普通输入或来源未知，仍保留 preedit-end→commit 的状态识别。
具体平台来源和候选定位的原生版本限制见 [IME 设计](DesignIME.md)。

## 3. TextModel

公开具体类型 textedit.Model；gui.TextModel 是兼容别名，不提前增加可替换存储接口。
以下保留 GUI 层兼容名称：

```go
type TextRange struct { Start, End int }
type TextEdit struct {
    Range TextRange
    Text string
}
type TextChange struct {
    Revision uint64
    Edits []TextEdit
    Reset bool
}

func NewTextModel(text string) *TextModel
// Text、Len、Slice、LineCount、LineRange、Revision
// SetText、Replace、ReplaceAtomic、BreakUndoGroup
// Undo、Redo、CanUndo、CanRedo、ClearHistory
// ConnectChange(func(TextChange)) signal.Handle
```

- Replace 是可合并的普通编辑路径：末尾连续输入与同侧连续删除并入一个撤销步骤；
  ReplaceAtomic 是独立撤销步骤，对应“一次粘贴或 IME 提交”；BreakUndoGroup 由
  视图在光标/选区移动或切换编辑视图时调用，落实本节的历史分组规则。

- 位置是 UTF-8 字节偏移，范围为半开区间，行号从零开始。空文档有一行，末尾 LF
  后有一个空行；LineRange 不含行分隔符。
- 输入转换为有效 UTF-8（非法序列替换为 U+FFFD），CRLF/CR 归一为 LF；不保留
  文件编码和原始换行方式。Normalize 后的位置用于后续操作。
- Replace 校验范围和 rune 边界，非法范围返回错误，不静默钳制。模型不判断
  Cluster 边界；GUI 用户操作以视图的 Cluster 为最小单元。
- SetText 是重新装载并清空历史；相同内容无操作，不改变版本/历史/光标/预编辑。
  Replace 是可撤销操作；Undo/Redo 无可用历史时返回 false。
- 增量信号不自动提取全文；Edits 按应用顺序解释，每项范围对应上一项执行后的文本。
  Revision 单调递增，Reset 标记重新装载。
- 模型顺序访问，绑定 GUI 后只在 GUI 线程操作。外部线程通过 Application 投递。
  变更和通知按事务顺序处理；外部回调可能再次编辑或卸载视图，不能继续访问过期状态。
- 无原生资源，不需要 Destroy；视图拥有并清理自己的 signal handle。

存储采用分块 piece tree，私有 AVL 节点汇总子树字节数和换行数。修改只操作受影响
片段，连续输入追加到块，相邻片段可合并；不复制全文或移动整份行数组。历史持有
差量/片段引用，不保存文档快照；块失去文档和历史引用后可回收。

参考方向调整不替换此存储：继续使用最多 64 KiB 的独立块、AVL 子树汇总和
LF 顺序查询游标，不改成 Scintilla 的底层存储，也不新增可替换存储接口。
模型操作只是输入响应链路的一部分，其吞吐基准不能替代原生排版、绘制和呈现的
交互延迟测量。全文读取/绑定有复制成本，大文本继续优先 Model/OnChange 增量路径。

历史保留到 SetText 或 ClearHistory，不承诺固定内存上限。连续输入和同方向删除
可合并；光标/选区移动、切换编辑视图、粘贴、IME 提交打断组。一次粘贴或 IME 提交
是一个撤销步骤。历史可保存选区值，但不持有 Widget。视图发起撤销时恢复该步骤
的选区到发起视图，其他视图仅映射位置，不同步选区或焦点。

## 4. Cluster 与排版

复用现有 TextLine/TextCluster，不新增 TextLayout.HitTest 或选区绘制接口。
Cluster 是本项目用户编辑操作的最小不可分割单元；模型则保存独立于字体的字节位置。

先校正三后端已有度量契约：末尾 Cluster 长度、多 glyph 的同一 Cluster、RTL 实际
位置和方向、零宽内容范围。不得凭逻辑宽度累加推断双向文字的视觉坐标。
换行符没有可见 glyph，GUI 以段落和行边界补足，不把分隔符吞进上一行最后一个
可见 Cluster。平台原生空行事实与 GUI 为编辑提供的空行光标区分开处理。

- 点命中先定位显示行，再按 Cluster 的几何和方向选择前/后边界。
- 左右箭头按视觉顺序导航，Backspace/Delete 删除逻辑顺序前/后 Cluster。
- 已有选区且不按 Shift 时，普通左右键收起到视觉左/右端；同一段落比较端点的
  显示行与 X，不把 RTL 的字节起止位置当作视觉顺序。端点亲和性朝向选中文字，
  跨段落则按上下顺序选择两端，无须排版中间的选中内容。按词命令保持逻辑顺序。
- 上下导航保留期望 X；软换行和双向边界保存内部光标亲和性，字节偏移不能单独
  决定光标绘制位置。
- 选区保存 Anchor/Caret 及方向；按选中的 Cluster 合并视觉相邻矩形，RTL 混排
  允许多个矩形。词边界最后校正到 Cluster 边界。
- 模型变更映射各视图位置，再按各自排版校正；不把 Cluster 写回共享模型。

按 LF 逻辑段落创建布局，绝不为全文创建一个 TextLayout 或每行创建一个 Widget。
每个视图独立维护累计高度索引，未测段落使用估值，测量后修正并保持视口顶部文本
锚点。WrapNone 的水平范围随测得段落宽度修正，不因换行宽度变化重排正文。

当前 `gui/text_editor.go` 已具备以下机制，不再使用“只保留上下各完整一视口”的描述：

- 段落 TextLayout 按编辑器隔离，以 LRU 保留近期资源；保留预算为 512 段和
  1 MiB 源文本字节，超出任一预算时淘汰非活动范围的旧资源。字节预算并非原生
  内存计量，当前活动布局范围被保护，可能超过预算；这不是绝对内存上限。
- 活动排版范围优先覆盖视口，上方测量范围参与锚定；下方额外预排版最多四段，
  同时受下方范围边界限制，避免每次跳转都同步排版另一整屏。近期非活动段落仍可
  留在 LRU 中，活动测量范围和资源保留范围不是同一概念。
- `MeasureSize` 与编辑几何分离：命中、导航、选择或锚定需要时才通过
  `editGeometry` 获取行/Cluster 度量并构建 Geometry，同一测量结果按需复用。
- 换行宽度变化使尺寸测量失效，但保留原生 TextLayout；访问段落时再 SetSize
  和测量，重建对应几何。字体/换行模式变化重置段落缓存，颜色变化更新已有布局，
  光标闪烁和选区背景变化不重建文字布局。
- 普通局部编辑只失效受影响段落，其余缓存保留资源、最近使用顺序并调整逻辑索引；
  全文 Reset 或不能安全解释的版本/批量变更仍采用既有完整失效路径。

已有对应测试：`TestTextParagraphCacheRecencyAndBudgets`、
`TestTextViewMeasuresEditingGeometryOnlyOnDemand`、
`TestTextViewWidthChangeReusesLayoutAndRefreshesHeight`、
`TestTextViewTrailingOverscanIsBounded`，以及局部编辑、颜色/字体和视口锚定回归。
测试存在和源码具备机制不等于真实后端的延迟收益已经测量；运行结果按验证记录解释。

视口锚点以正文实际裁剪区顶部取样，不以控件外边界取样。正文平移
`padding-offset`，裁剪从 `padding` 开始，因此可见内容的起点为 `offset`；
被 padding 完全遮住的上一行不能成为锚点。恢复偏移使用与 ScrollView 相同的
整数 DIP 四舍五入及向下取整的最大范围，不按上次偏移方向分别 floor/ceil，
否则带小数的原生行高可能使布局在相邻整数间反复请求滚动。

宽度连续变化期间保留原来的模型字符，不在每次重排后改为新显示行的行首。
只要该字符所在行及行内偏移经整数 DIP 归一后仍解释当前视口，就保持锚点；
主动滚动或内容范围钳制导致位置不再匹配时重新取样，模型编辑仍映射原字符位置。
栅格化倍率只影响输出像素，不应改变已测量的换行、Cluster 位置和选择几何。

保持 TextLayout 身份稳定，复用 Painter 的文字资源缓存。光标闪烁/选择背景变化
不重建文字布局；字体变化与颜色变化分别失效。卸载释放布局、计时器、连接，
保留模型内容；重新挂载恢复视图所需状态。

### 低延迟与资源预算：后续优化顺序

后续以 Scintilla 的局部工作和缓存分层思路审视热路径，但不照搬它的消息 API、
类型体系或完整代码编辑功能。以下为待评估工作，不表示已经实现：

1. **先测量输入响应链路。** 分开记录原生输入提交、模型修改、布局、绘制和呈现；
   明确测量起止点。只有具备呈现观测手段时才报告输入到可见反馈的端到端延迟，
   不把模型耗时、Paint 返回或提交绘制命令当成屏幕呈现时间。
2. **验证现有缓存收益。** 记录布局创建/复用/销毁、几何构建、缓存命中/淘汰、
   全文读取和 Go 分配；区分源文本预算、Go 可达存储与原生资源占用。避免只测热缓存，
   忽略冷启动、远距离滚动和尺寸变化时的停顿。
3. **按证据缩小同步工作。** 优先检查重复测量、过宽失效和非必要预排版，再决定
   是否调整策略。若引入延后执行，单独设计调度、版本失效、取消及资源生命周期，
   不默认新增线程、公共缓存配置或独立调度框架。
4. **最后评估边界扩展。** 超长段落、超过当前目标的文件、缓存参数公开化与并行
   排版按真实需求另行设计；不以“参考 Scintilla”为由提前承诺或放松线程亲和。

验证覆盖连续输入/删除、跨段粘贴、撤销/重做、滚动跳转、resize、IME 组合、主题
变化和多视图。记录 1/10 MiB 文档的行长分布、冷/热缓存、字体、OS、Linux WM、
实际绘制/排版后端及 1x/2x；将超长单段落单列，不能混入常规文档得出平均结论。
模型基准与真实窗口测量分别报告，耗时分布（如中位数与 P95）及内存对照来自同环境
重复运行；不预设未经验证的毫秒目标或性能倍数，不把新增性能观察代替正确性断言。

## 5. TextView 交互与 IME

NewTextView 默认创建空模型，SetModel 可以共享外部模型。公开 Model、Selection、
ReadOnly、WrapMode、Padding 的 getter/setter，内容变化和选择变化使用信号。
默认可编辑、WrapWordChar、4 DIP padding。

选择值使用 `TextSelection{Anchor, Caret int}`（UTF-8 字节），`Range()` 返回有序
半开区间。`SetSelection` 钳制文档范围并按视图 Cluster 校正，`ConnectSelection`
报告方向保留的选区；`ConnectChange` 报告模型增量。命令式视图不拥有模型的生命周期。

首版包括单击定位、拖选、Shift 扩选、双击选词、拖出视口自动滚动；方向键、
Home/End、PageUp/PageDown、全选、剪切复制粘贴、undo/redo。Enter 插入 LF。
Tab 默认不消费，AcceptsTab 显式开启插入 tab；不顺带实现全局 Tab 焦点遍历。
只读仍可导航、选择、复制，但不启用 IME。光标闪烁和拖选自动滚动复用 GUI Timer。

预编辑归属视图：开始时记录待替换选区，显示时临时替换，提交才修改模型，取消
不改变文档。其他来源修改模型时取消已有组合，避免以过期范围提交。
异步粘贴保存模型、revision、选区和挂载状态；返回时条件变化则放弃，不插入到
用户后来选中的新位置。Clipboard 回调也可能同步执行。

预编辑投影只复制替换范围两端仍需显示的段落文本，不复制被选中的大段内容；
修改组合文字只失效投影涉及的段落。原生 Reset 期间的同步回调不能再次提交已取消
的组合；继续执行的调用方检查模型与挂载代次。粘贴还检查视图输入版本，避免组合
开始或光标亲和性变化后把旧剪贴板回复插入新位置。Snapshot 的组合信息独立于已
提交文本，带替换范围、完整长度及最多 4096 字节的文本前缀。

光标使用 500 ms GUI Timer，仅请求重绘，不重建排版；编辑/导航后先显示光标，
组合期间保持显示。窗口失焦、控件失焦或卸载停止计时。拖出视口时使用 40 ms
GUI Timer 按越界距离滚动并扩选，释放捕获或卸载停止；不增加 Widget goroutine。

普通输入使用原生字符提交；Windows 补 WM_CHAR 的 UTF-16 组装并处理 IMM32
重复提交路径，核对 Linux/macOS 提交路径后移除 GUI 美式物理按键转换。
[WM_CHAR](https://learn.microsoft.com/en-us/windows/win32/inputdev/wm-char) 不与
按键一一对应；导航/编辑命令和文本输入不能混成一条路径。

### 编辑上下文菜单

TextView、TextInput 在私有 textEditor 中共用 PopoverMenu，不在 UI 层重复实现，
也不引入公开的应用命令系统、ContextMenu Widget 或平台菜单适配。
参考 [GTK 4.20.3 gtk_text_view_do_popup / gtk_text_view_get_menu_model](https://github.com/GNOME/gtk/blob/4.20.3/gtk/gtktextview.c#L8720)：
文本控件驱动 PopoverMenu，将菜单操作关联到既有编辑动作；GOUI 采用这种归属，
不照搬 GTK 的 action registry。菜单仍使用既有样式、透明能力降级与工作区退避。

- 次键按下打开；macOS 另接受 Control+左键。命中现有选区时保留方向及边界，
  否则将光标移到点击位置。命中依据当前段落的 Cluster 选区矩形（包含 LF 高亮），
  不只比较最近插入点的字节偏移，避免选中末字符的右半边误清空选区。
- 右键与 Shift+F10 共用 `ContextMenuEventController`；键盘请求在光标下方打开，位置为
  控件局部 DIP，扣除滚动偏移；不改变选区，不将窗口坐标再次应用到 ShowAt。
  本轮复用现有 Menu 输入能力，不扩展通用菜单方向键导航或快捷键文字显示。
- 命令为 Undo、Redo；Cut、Copy、Paste、Delete；Select All。
  只读禁用所有写操作；无选区禁用 Cut/Copy/Delete；无相应历史禁用 Undo/Redo；
  空文档禁用 Select All。Paste 在可编辑且存在 Clipboard 时启用，不为展示菜单读取
  剪贴板内容；无文本/读取失败沿既有契约无操作。Copy/Cut 同样要求 Clipboard。
- 菜单与编辑快捷键共用私有操作入口，菜单 Delete 只删除选区，不模拟 Delete 按键。
  修改模型/选区/只读状态时刷新菜单启用状态；执行前再次检查条件。模型的
  ClearHistory 不发送 Change，已打开菜单未必即时刷新，但执行时仍检查历史可用性。
- 打开前取消预编辑并停止拖选，未提交文本不进入正文或撤销历史；一次剪切、粘贴
  或菜单删除是一条原子编辑，沿用既有选区恢复与异步粘贴过期保护。
  单独按复制快捷键仍保留组合：复制已提交选区，不能用临时投影排版校正模型偏移。
- PopoverMenu 先 Hide、解除模态再执行操作；编辑焦点保留，Escape、外部点击及
  失焦沿既有窗口模态路由关闭。替换模型或卸载主动 Hide，原生资源仍由 Popover 的
  anchor-unmount / window-destroy 连接释放。菜单动作校验模型、挂载代次及菜单模型
  身份，避免关闭回调中的卸载、替换或重开使旧动作编辑新文档。
- Snapshot 复用文本编辑状态和菜单 RoleMenu / RoleMenuItem；禁用项无 ActionClick。
  菜单是独立 popup 树，不伪装成编辑器子控件，不添加绕过输入的语义动作。

包内验证集中在 text_editor_test.go：真实 Window.DispatchEvent → PopoverMenu →
菜单行事件控制器，替换原生表面/排版/剪贴板，不打开桌面。窗口验收复用
tests/window/ui/textediting 的第 10–11 项。

本切片验证（2026-09-22）：

- Linux amd64 `go test ./...`、
  `go test -race ./gui ./ui -run 'Test(TextEditor|TextView|TextInput|Shortcut|PopoverMenu)' -count=1`
  通过。全仓有 31 个按原生/手动开关跳过的测试，34 个包无测试；跳过不计为窗口验收。
- 共享命令提取期间，`TestTextEditorCopyCommandDuringPreedit` 曾复现短投影将
  `{1,6}` 选区错误钳制为 `{1,2}`，仅复制 `b` 而非 `bcdef`；修正 Copy 的组合期
  选区处理后通过，未删除断言或放宽输入保护。
- Windows 10 19045 amd64、macOS 15.7.7 arm64：上传并核对 SHA-256 的 GUI 测试
  二进制原生运行 `Test(TextEditor|TextViewKeys|Shortcut)` 全部通过，包含七项菜单
  命令、生命周期、Control+click 平台分支及原生 DirectWrite/CoreText 导航回归。
  本轮远端只跑无头测试，不代表 Windows/macOS 桌面菜单已经验收。
- 本机隔离 X11/Xfwm4、Software 1x、XMODIFIERS=@im=none：通过 XTest 输入运行
  UI textediting。验证右键选择保留、菜单复制/剪切/粘贴/撤销/重做、关闭后继续
  键入、Shift+F10/Escape、只读项不可点击、带绑定 Notes 的多行粘贴与回写。
  Clipboard 精确核对 `abcdef`、单行 `one two`、多行 `one\ntwo`；关闭带菜单的
  主窗口后没有残留 popup。未使用 DevServer 或直接调用动作。
- 首轮连续输入脚本观察到一次 Notes 粘贴后未变化；该脚本紧接着派发全选/复制，
  未等待异步读取完成。临时日志和分步复测中动作与回复条件通过，未稳定复现该
  现象，也未据此修改异步过期规则；保留“粘贴完成前不改变选区”的验收前提。
- 隔离 WM 未开启合成器，因此本次观察的是菜单不透明降级，不能据此宣称圆角/
  阴影合成、2x、OpenGL 或真实中文候选窗已重验。剪贴板仅属于隔离显示，未改用户
  桌面的布局/剪贴板；测试窗口和隔离显示已退出。

## 6. ScrollView、TextInput 与 UI

```go
func (sv *ScrollView) ScrollIntoView(rect geometry.Rectangle)
type ScrollIntoViewRequester interface {
    ConnectScrollIntoView(func(geometry.Rectangle)) signal.Handle
}
```

矩形使用未扣滚动偏移的内容 DIP 坐标。ScrollView 自动连接可选请求接口，替换
内容时断开旧连接，不检查 TextView 类型。布局中的请求合并到布局及范围刷新之后，
避免递归 Arrange。编辑/导航请求光标可见；滚轮浏览不强制回到光标。

当前自动连接仅针对 ScrollView 的直接内容。后续扩展滚动能力时，优先评估
GTK4 式独立共享滚动模型，区分目标可见性、锚点修正和主动滚动；本轮不替换
请求接口。参考证据与取舍见 [滚动模型扩展规划](DesignScroll.md#后续扩展优先评估独立滚动模型)。

显式光标导航优先于保留旧视口顶部的锚定请求；导航尚未完成时，不恢复旧的
浏览锚点。完成条件同时要求内容使用的偏移与宿主偏移一致，并按 ScrollView
的整数 DIP 最大范围判断可达位置。测试只执行实际请求的布局帧，不以额外的
无条件 Arrange 掩盖调度丢失，也不能只检查宿主偏移而忽略正文实际绘制偏移。

```go
model := gui.NewTextModel("第一行\n第二行")
editor := gui.NewTextView()
editor.SetModel(model)
scroll := gui.NewScrollView()
scroll.SetChild(editor)
```

TextView 验证后提取私有编辑状态/操作给 TextInput，不让单行控件嵌套 TextView。
TextInput 内部独立模型，首版不与多行共享；保留 Text/SetText/ConnectText、Padding、
样式。单行不换行，内部水平偏移显示光标，无滚动条；Enter 发 ConnectSubmit，
SetText/粘贴的每个归一化 LF 转为空格，保持稳定行高/baseline。相同 SetText 无操作。

实现采用 `textEditor` 私有普通结构，TextView/TextInput 分别持有一份；它不是
Widget，不进入树，不拥有第二套焦点/事件路径。公开控件仍直接持有 WidgetBase，
共享代码通过明确的 owner 请求布局、绘制和生命周期检查。模型、选择、预编辑、
段落布局、计时器和缓存都按控件隔离。TextInput 不实现 Scrollable、不公开 SetModel。

TextInput 增加 Selection/SetSelection/ConnectSelection、ReadOnly/SetReadOnly、
ConnectSubmit。改变 SetText 清历史并将光标置于末尾；相同归一化文本不动选区或
预编辑。布局使用缓存的混合文字样本计算稳定行高/baseline，实际文字按该 baseline
放置，不根据每个字符串的高度单独居中。单行布局完整排版、按内部矩形裁剪，
水平偏移由控件管理。颜色变化原位更新 TextLayout，字体变化才重排。

模型/字体变化可能改变 Cluster 边界；输入命令执行前按当前排版校正端点，并检查
选择通知期间的卸载/模型变化。样式失效不强制排版不可见的远处光标段落。

- 样式新增 text-view；selection part 控制背景，caret part 控制颜色，选中文字
  保持普通前景色，避免每次选择修改 TextLayout 颜色。
- Snapshot 增加角色和文本编辑状态：只读、选区、总字节数、有范围标识的可见文本。
  多行不自动复制全文、不生成逐行 Widget，不新增绕过输入分发的语义动作。
- UI 增加 TextView().Model(model)，与 ui.ScrollView 组合；提供 GUI 模型的类型
  别名和构造入口。模型变化直接使 GUI 失效，无须每键重建 UI。
- 多行提供 Text/OnText/BindText，普通编辑采用文本值绑定；保留 Model 和增量
  OnChange 作为共享文档、大文本入口。单行和多行均提供 BindSelection，重建不能
  破坏输入状态。GUI TextView 无需增加 Text/SetText，UI 直接协调到已有 Model。
- 复用现有协调和连接生命周期，不向 Root/BuildContext 添加文本类型特例。

UI 的具体约定：`TextView().Model(model)` 使用外部模型；nil/省略恢复挂载时保留
的初始私有模型，不在每次重建时创建新模型。Selection 是可选声明；省略时保留
GUI 当前选择。单行省略 Text 时保留编辑器内容，显式 Text("") 才清空；BindText
仍是 Text+OnText 的适配。协调更新时阻塞自身的信号连接，卸载时断开，不拦截其他
命令式监听者，不另外实现 IME/撤销。modern 同时提供两种编辑器的正文、selection
和 caret 样式；选中文字保持正文颜色，主题极端色也检查文字对比度。

### TextInput / TextView 的 Placeholder

两个控件均提供 `Placeholder() string` / `SetPlaceholder(string)`，UI 对应
`.Placeholder(string)`；默认空字符串关闭提示。UI 更新省略该属性时恢复为空，
与省略 Text 时保留用户编辑内容的契约独立。

提示在文档字节长度为零且没有 IME 预编辑时显示，聚焦后继续显示。空格或 LF
均属于真实内容；只读和禁用不关闭提示。取消预编辑恢复空文档提示，提交后显示正文。
设置提示不改变模型版本、选区、历史、光标、预编辑或编辑信号，只请求绘制和语义更新。
TextInput 沿用 NormalizeSingleLine；TextView 沿用 Normalize，保留显式换行。

实现位于每控件独立的私有 textEditor，提示及原生缓存属于视图，不进入 TextModel /
textedit。共享模型的视图可以配置不同提示，一侧的预编辑不改变另一侧的提示状态；
SetModel 保留提示。Snapshot 的 TextEditingInfo.Placeholder 单独报告配置，文本值、
Length、VisibleText、选区、复制及 IME surrounding text 仍仅描述正文。

提示沿用正文的字体、Padding、起始对齐和裁剪区，绘制在光标之前。单行不换行，
按既有稳定基线垂直放置；多行从正文首行开始，按 WrapMode 和可用宽度换行。
提示不参与 Measure、ContentSize、段落高度、滚动条、命中测试或选区几何；超出视口裁剪。
text-input / text-view 的 placeholder part 只读取前景色，Basic 提供中性灰和禁用灰，
Modern 使用现有 muted / disabled；字体指标由正文样式确定。

每视图至多保留一个原生提示布局，首次可见绘制时创建。提示文本、字体、换行方式、
影响换行的宽度变化时释放重建；颜色变化原位更新；卸载时释放，重挂载后按需创建。
提示只在 GUI 线程访问，释放前清空所属引用，颜色通知回调后检查生命周期和缓存身份。
现有平台 TextLayout 足够，不新增平台接口；其 SetPlaceholder 的行内 U+FFFC 度量
与这里的空内容提示是不同概念。

参考与取舍（2026-10-19）：Qt 6.12 在线文档中的
[QLineEdit](https://doc.qt.io/qt-6/qlineedit.html#placeholderText-prop)、
[QPlainTextEdit](https://doc.qt.io/qt-6/qplaintextedit.html#placeholderText-prop) 和
[QTextEdit](https://doc.qt.io/qt-6/qtextedit.html#placeholderText-prop) 均提供独立提示属性；
QLineEdit 通常聚焦后仍显示提示，居中对齐有例外。
GTK 4.24.1 在线 [Entry 文档](https://docs.gtk.org/gtk4/property.Entry.placeholder-text.html)
描述为空且未聚焦时显示。GOUI 采用聚焦后保留提示，适合自动聚焦的查找框；
多行、IME、视图隔离和滚动范围规则是本项目明确约定，不从参考文档推断原生实现。

### 文本与选区值绑定

TextInputView / TextViewView 的可选声明使用普通值字段和各自独立的
`fields bits.Bitmap[uint64]`，与 viewBase 的实现风格一致。位仅表示是否声明
Text、Selection、Padding（以及多行 WrapMode），不与 viewBase 共享位编号；
ReadOnly / AcceptsTab 仍是普通 bool 属性值。省略与显式零值的既有语义不变。
本次存储重构在 Linux amd64 执行 `go test ./...` 和
`go test -race ./ui -run 'TestText' -count=1` 均通过；新增显式零值、默认恢复及
viewBase 标记隔离测试。未重新执行跨平台编译或窗口验收，不改变上述待验收项。

```go
// 状态保存在持续存在的应用/组件中，不在每次 Build 时重新初始化。
text := ui.MakeState("初始内容")
selection := ui.MakeState(ui.TextSelection{})
// Build 内：
ui.ScrollView(ui.TextView().BindText(&text).BindSelection(&selection))
```

- `BindText(s)` 等价于 `Text(s.Get()).OnText(s.Set)`；`BindSelection(s)` 等价于
  `Selection(s.Get()).OnSelection(s.Set)`。nil 不改变声明。回调槽不叠加，后设置
  的 OnText/OnSelection 会替换写回；需要附加业务处理时显式使用值+回调。
- ui.State.Set 复用 RequestUpdate 合并调度；自定义 BindState 自行安排更新。
  不引入依赖追踪、Controller、公共事务或 Root/BuildContext 文本特例。
- 非 nil Model 与 Text/BindText 同时声明属于编程错误，在修改 Widget 前 panic；
  Model+OnText 可以观察全文，但大文本优先 OnChange。没有全文监听时不读取全文。
- 私有模型在同一 Widget 生命周期内保持。省略 Text 保留内容，Text("") 清空；
  省略/移除 Model 恢复原始私有模型，不把外部模型复制为私有内容。
- Update 阻塞自己的连接，先选模型、设置文本，再设置选区。文本直接调用
  Model.SetText：归一化后相同内容不改变版本、历史、光标或 IME；不同内容清历史，
  通过已有 GUI 模型通知取消组合、重置多行光标到开头和滚动位置。单行沿用光标
  到末尾的契约。有 Selection 声明时再以新文本校正、应用它。
- 外部选区是归一化文本的 UTF-8 字节位置，GUI 按文档范围/可用 Cluster 校正。
  协调不反向修改应用状态，也不发送业务回调；未归一化字符串、越界选区留在应用
  状态中作为请求值，控件使用有效值。其他 GUI 监听者不受本适配器的阻塞影响。
- 输入经正常事件分发在 GUI 完成后才写回绑定；不等待 UI 重建完成编辑。
  IME 预编辑不进入正文绑定，提交才更新。相同文本回传保持撤销分组和预编辑。
  文本与选区通知独立，不保证回调顺序或同步观察到原子双字段状态；正常重建由
  既有事件循环合并调度，使用当前输入处理完成后的状态。
- OnText 查询通知时的当前全文，重入修改时可能已超过该通知版本；需要按版本
  处理变化应使用 OnChange。独立连接由 UI 卸载时释放，回调后不继续访问 Widget。
- 两个控件绑定同一字符串不共享历史；另一侧收到新内容属于全文重新装载。
  共享文档和历史使用同一 Model，各自选区/滚动仍独立。不声称全文绑定优化大文本。

参考：SwiftUI [Binding](https://developer.apple.com/documentation/swiftui/binding)
的读写入口契约，不采用其状态运行时；Qt Quick 6
[TextEdit](https://doc.qt.io/qt-6/qml-qtquick-textedit.html) 区分全文属性与增量操作的
成本；Flutter [TextEditingController](https://api.flutter.dev/flutter/widgets/TextEditingController-class.html)
提示正文/选区更新及 composing 的关联，GOUI 保留现有 GUI 编辑机制，不新增 Controller。
以上为公开接口参考，不推断 SwiftUI 内部编辑器实现。

2026-09-21 实现与验证：

- 仅修改 UI 适配、测试、窗口例程和文档，GUI/platform 生产代码未改。
- Linux amd64：`go test ./...` 通过；
  `go test -race ./gui ./ui -run 'Test(Text|Scroll|Window.*IME|IMContext)' -count=1`
  通过，补充测试后 `go test -race ./ui -run 'TestText' -count=1` 再次通过。
  新增包内测试使用替身窗口和真实事件分发验证选择删除、撤销/重做及回传重建；
  覆盖来源冲突、归一化/选区顺序、连接清理及共享字符串不共享历史。
- Windows amd64：`GOOS=windows GOARCH=amd64 go test -c ./ui -o <tmp>/ui.test.exe`
  与 `go build` 同目标的 `./tests/window/ui/textediting` 均通过，仅为交叉编译。
- macOS arm64：UI 测试目标及窗口例程交叉编译均失败，Cocoa 包已有未解析符号
  `AutoReleasePool`、`NSApplicationClassId`、`NSURLClassId` 等；独立执行
  `GOOS=darwin GOARCH=arm64 go test -c ./platform/darwin/cocoa -o <tmp>/cocoa.test`
  同样失败，未顺带更改平台绑定。
- 本轮未启动桌面窗口，也未使用 DevServer/远端桌面。新增 Notes、外部替换/选区
  按钮及 Auto rebuild 计时器的真实 IME、Clipboard、主题/HiDPI 验收仍待执行；
  入口列出操作步骤，保留原共享模型 1/10 MiB 场景。
- 全仓既有像素/排版测试仍会更新源码目录下的 PNG 测试产物，本轮没有新加此类
  输出；交叉编译产物放仓库外 `/tmp/goui-text-bindings.MNn9KX/`。

## 7. 切片、验证与进度

### 测试文件组织

包内测试按被测对象集中，不再为每个缺陷或子功能创建独立文件：

| 文件 | 归属 |
| --- | --- |
| `gui/text_editor_test.go` | 共用替身与事件入口、共享编辑行为、双向导航、剪贴板与历史、预编辑提交分组、光标计时器、原生导航验证 helper |
| `gui/text_input_test.go` | 单行模型和编辑约束、Submit、水平滚动、baseline、绘制、样式与 Snapshot |
| `gui/text_view_test.go` | 多行视图、共享模型、预编辑投影、视口缓存和锚定、自动滚动、原生排版定位 helper 与性能基准 |

Pango、CoreText、DirectWrite 的三个平台后缀文件仅保留原生排版测试入口，
继续在执行原生调用的 goroutine 中锁定 OS 线程。共享 helper 不直接导入具体
平台排版后端。`im_context_test.go` 保留 IMContext 和 Window 输入路由测试。

`gui/textedit/model_test.go` 集中模型、存储块生命周期、通知、行定位与模型
基准；几何、高度索引、投影仍各自对应实现文件。外部包 `textedit_test` 的
`api_test.go` 保留跨包契约验证，不与包内私有状态测试混合。UI、主题与窗口
用例仍留在各自所属位置。

2026-09-21 整理仅移动声明、整理 imports 和增加分组注释：9 个零散文件内容
归入上述主文件。迁移前后 141 个声明（含 80 个 Test、3 个 Benchmark）的
Go token 哈希一致，保留名称、断言、基准参数、线程锁定与正常事件分发路径；
平台入口文件逐字节不变。

验证限制：整理前 `go test ./gui ./gui/textedit ./ui` 已因
`gui/text_editor.go:820/821: undefined: time` 失败；整理后原样
`go test ./...` 仍遇到同一问题。本轮不修改生产文件，使用仓库外 Go overlay
仅补 `time` 导入后，全仓测试、`go test -race ./gui ./gui/textedit ./ui`
及 Windows amd64 GUI 测试交叉编译通过（上述命令均附加 `-overlay` 参数）。
macOS arm64 GUI 交叉编译仍被 Cocoa 中未解析的 `AutoReleasePool`、
`NSApplicationClassId` 等既有错误阻挡。临时覆盖的通过不等于工作区原样通过；
窗口手动验收本轮未执行。

### 开发验证与用户窗口验收

按用户最新确认的分工：窗口验证以本机为主；Windows/macOS 平台能力通过
remote-shell 离屏验证；其他平台真实窗口由用户运行既有独立用例验收。
不再以远端桌面可访问作为开发前置条件，不要求安装桌面代理或修改系统权限。
切片完成表示代码与开发验证完成，不表示未执行的三平台窗口场景已经通过。

| 验证领域 | 开发验证证据 | 用户窗口验收／后续矩阵补验 |
| --- | --- | --- |
| 原生文字提交与 IME | Linux XIM 实际组合/取消/提交；三平台文字组装及编辑逻辑测试 | Windows/macOS 原生窗口提交、真实组合及候选位置；键盘布局输入 |
| Clipboard 与编辑状态 | Linux 实际大文本复制/粘贴、过期保护、撤销与共享模型；GUI/UI 包内回归 | Windows/macOS 窗口中的复制/剪切/异步粘贴、撤销及受控/非受控 UI 重建 |
| 排版与交互一致性 | 三平台原生 Cluster/RTL 和重排离屏回归；Linux Software/OpenGL 部分窗口场景 | Windows/macOS 选择/导航/拖选、窗口重排和 1x/2x 后端矩阵；Linux 尚未覆盖的矩阵组合 |
| 生命周期 | 包内回调重入、卸载、计时器、预编辑回归；Linux 组合期间关闭窗口 | Windows/macOS 真实组合、拖选和闪烁期间关闭/失焦 |

窗口入口及逐项预期：`tests/window/gui/textview`、`tests/window/ui/textediting`。
入口注释包含 PowerShell/POSIX 启动命令、后端与 1x/2x、快捷键差异、编号操作、
预期、副作用、复位和退出；GUI 用例另有 `-numbered` 与 `-accept-tab`。
运行时记录实际后端和 PixelScale，不将环境变量偏好或成功启动当作通过。
Windows 需在用户交互会话中运行，而不是 SSH 的 Session 0；macOS 需登录到
可访问的 Aqua 会话，并让实际操作进程具备必要的屏幕录制/辅助功能权限。
不创建 SYSTEM 任务、不修改 TCC、不自动解锁或切换用户会话以绕过访问边界。
远端验证仅使用 win/mac；Linux 在本地验证。

2026-09-20 先前自动桌面验收的环境记录：Windows 命令 SessionId=0，RDP 用户 SessionId=1；
macOS `/dev/console` 用户为 root，`AXIsProcessTrusted=false`。因此本轮未截图、
未投递输入、未启动远端窗口；已询问恢复可访问桌面或由用户按用例操作反馈。
这些历史环境缺口不计作功能测试通过，也不再阻塞开发；无需重复探测未改变的桌面。

### 各切片状态

- [x] A：三平台 Cluster 契约修正、原生字符输入、回归测试。
  - 实现：三后端 MeasureMetrics 按原生几何重建 Cluster（多 glyph 同 Cluster、
    RTL 视觉位置、行尾分隔符剥离、空行/末行度量）；Windows WM_CHAR/WM_IME_CHAR
    组装 UTF-16 代理对并抑制 GCS_RESULTSTR 双路径；gui 层删除美式物理按键
    转字符（keyEventText），文字仅经 InputMethod Commit 进入。
  - 验证：Linux/Pango 度量与 stringpos、gui 输入路径测试通过；
    macOS 15.7.7 arm64 的 CoreText 与 Windows 10 22H2 amd64 的 DirectWrite
    度量及接入编辑器的离屏测试均已实际运行通过（见末尾原生验证记录）。
    缺口：WM_CHAR/interpretKeyEvents 真实窗口提交路径及桌面输入场景仍待验收。
  - Linux 真机验证通过（2026-09-18，Ubuntu 22.04，Pango 1.50.6，
    TigerVNC :3 + xfwm4，经 remote-shell）：`go test ./gui/
    ./platform/typography/...` 全过（含 TextModel 与 Pango Cluster
    度量）；900x500 真窗渲染 `ab中😀` 混合 Cluster 正常；XTest 合成按键
    经 XIM/Xutf8LookupString 逐键 commit（`Hi-7` 四次提交），BackSpace
    删除、Left 导航后中间插入位置精确，Ctrl+修饰键无提交；
    `platform/test -run TestClipboard` 在真 X 上通过
    （ASCII/UTF8/Multiline/Overwrite，PasteFromApp 跳过）。
    真实 IME（中文输入法组合）与多键盘布局仍待后续覆盖。
- [x] B：TextModel、piece tree、逻辑行索引、增量信号、历史。
  - 实现：`gui/textedit/model.go` + `gui/textedit/piece_tree.go`；初始内容及插入均按最多
    64 KiB 的独立块存储，连续输入复用前一片段的块尾；片段直接持有块，没有
    长期保留全部内容的缓冲区注册表。行查询带 LF 游标缓存，
    顺序遍历线性；历史以差量片段引用记录，不存快照；连续输入与同侧删除
    合并、反向删除不合并。
  - 验证：随机编辑对照独立字符串参考（含撤销栈镜像、合并规则、信号协议、
    树不变量）、空文档、归一化、非法范围、重入、1 MB 文档与 undo/redo
    回归通过；race 通过。1/10 MiB 编辑周期、局部排版、滚动/重排基准与视图集成
    已补齐，具体环境和结果见下文；不将替身排版基准等同于真实绘制帧率。
- [x] C：TextView、编辑/选择/IME/Clipboard、滚动请求、视口排版（开发完成，用户窗口验收单列）。
  - 已实现：`ScrollIntoView` 可选信号接口、延迟合并滚动请求；`TextView` 共享模型/
    独立选区、视口优先排版与有预算的 LRU 段落布局缓存、压缩段落高度索引、Cluster
    点命中/视觉导航/不连续选区、选择与原生文字提交、剪切复制粘贴、只读、
    按词移动/双击选词、撤销重做的发起视图选区恢复、有限可见文本 Snapshot。
  - 回调顺序：TextModel 重入编辑立即提交，但通知按 revision 顺序排队；较晚的
    监听者查询模型时可能已经看到更新的版本。视图在版本不匹配时丢弃旧排版，
    按增量顺序映射选区，不能拿未来内容解释旧变更。历史只存选区值，编辑者使用
    不引用 Widget 的标识，切换编辑者会打断合并组。
  - 滚动：跨越未测段落的光标导航保留可见性请求，直到新排版的实际目标可见；
    不能在第一次按估值滚动后就清除请求。滚轮浏览取消待完成的导航请求。
  - 缓存：本段编辑只失效受影响段落，其他可见段落保留 TextLayout；远距离行查找
    走 piece tree 汇总索引，只有相邻行查找使用 LF 顺序游标。字体变化重排，
    颜色变化更新已有 TextLayout，Paint 不为视口外光标创建排版。当前还具备按需
    编辑几何、宽度变化复用布局、512 段/1 MiB 源文本保留预算（活动范围例外）及
    最多四段下方预排版；具体边界与现有测试见第 4 节，不据此新增性能验收结论。
  - 已接入：视图私有 IME 预编辑投影、取消与原子提交；GUI Timer 光标闪烁和
    拖选自动滚动；模型修改、字体/宽度变化后的视口文字锚定。锚点保留到宿主应用
    滚动请求，不能在 ScrollView 第一次内容布局后提前消费；末页按范围钳制收敛。
  - 本机窗口及三平台原生离屏证据见后文；Windows/macOS 的真实输入、组合、
    Clipboard 和 HiDPI 窗口场景由用户按用例验收，不把离屏结果外推为窗口通过。
- [x] D：TextInput 复用与兼容，UI 重建状态保持（代码与包内测试，平台验收见 E）。
  - TextView/TextInput 共享私有 textEditor；单行独立模型、选择/Cluster 操作、
    IME/Clipboard/历史、只读、Submit、水平滚动和稳定 baseline 已接入。
  - 原有测试保留有效覆盖：按 rune 删除改为供应 Cluster 的替身；Ctrl+A 改为
    已实现的选择命令；单行完整排版后裁剪；颜色变化由销毁布局改为原位更新。
    没有通过跳过测试保留旧的单行编辑路径。
- [x] E：UI 绑定、modern/兜底样式、Snapshot、窗口用例与开发验证。
  - 已实现 UI TextView、TextModel 类型别名/构造、TextInput 选择/只读/提交绑定，
    受控与非受控内容重建保持、声明式连接生命周期，以及两类编辑器 modern parts。
  - `tests/window/ui/textediting` 提供共享模型、两种单行内容模式、主题切换及
    1/10 MiB 验收步骤。其他平台 IME/Clipboard、双向导航及 1x/2x 后端矩阵
    已交付操作说明，待用户窗口验收，不阻塞开发完成。

历史选区已由 TextView 的私有模型编辑入口记录；不增加要求应用传选区的模型公共 API。
块生命周期已改为直接引用：文档及历史都不再引用的块交给 Go GC 回收；同块内仍
被引用的少量内容可能保留整个块，最大 64 KiB，不做搬移式压缩。历史仍保留至
ClearHistory/SetText，没有未经声明的自动截断。连续输入与删除同时合并历史片段，
历史引用数按块而非按键增长。没有用 GC 运行时机作为测试通过条件。
DevServer 的键入命令在移除物理
按键转字符后需要经真实 commit 路径的注入入口，随 gui/dev 适配另行设计。

逻辑测试放所属包：随机编辑与独立字符串参考比较，行索引、undo/redo、非法范围、
空文档；Cluster 的中文、组合符、emoji、连字、混合方向、零宽内容、软换行和空行；
共享模型隔离、异步粘贴过期、重入/卸载、重建、只读切换和资源生命周期。

性能测试对照同环境 1 MB/10 MB 文档的局部输入、滚动、resize，记录耗时、分配和
布局创建数，断言局部编辑不提取或排版全文；不凭一次耗时承诺帧率。

桌面程序放 tests/window，入口写操作与逐项预期。三平台验证真实键盘布局、IME、
Clipboard；Software/OpenGL/Direct2D 验证选择、裁剪、光标和 1x/2x。不得以编译通过
代替真机验收，不依赖 DevServer。交付运行 go test ./...、相关 race、交叉编译，
分别记录通过/失败/跳过/未执行。

### 2026-09-20 C 切片阶段验证

- `go test ./...` 通过；保留既有桌面测试及其运行开关，没有把跳过的手动场景算作通过。
- `go test -race ./gui -run 'Test(Text|Scroll)' -count=1` 通过。包内新增覆盖
  Cluster 几何、随机高度索引、共享视图资源隔离、局部排版、过期粘贴、回调重入/
  卸载、只读、按键/鼠标分发、历史选区及估算高度修正后的光标可见性。
- Windows amd64 / macOS arm64 的 `go test -c ./gui` 交叉编译通过；不是原生运行验收。
- `BenchmarkTextViewLocalEdit`：Linux amd64，Xeon Gold 5318Y，替身排版，
  500×400 DIP，1 MiB/10 MiB 常规多段落，`-benchtime=100x -count=3`。
  每次局部替换并刷新视口仅创建一个段落布局（有断言）；1 MiB 为 15.9～16.8 µs/op，
  10 MiB 为 14.9～15.0 µs/op，均 43 allocs/op、约 7.3 KiB/op。
  这是规模敏感性检查，不是新旧实现性能对比，也不代表真实字体排版或绘制帧率。
- 窗口入口：`tests/window/gui/textview`，含 1/10 MiB 双视图和逐项操作预期。
  当前实际验证：Ubuntu 22.04.5、X.Org 1.21.1.4 / VNC :3 / Xfwm4、Pango 1.50.6、
  Software、1x、1 MiB。XTest 输入通过正常 XIM/KeyEvent 路径验证 ASCII/Enter、
  Shift 选择及两视图共享显示；Ctrl+End 显示最后空行光标，Ctrl+Home 恢复顶部，
  另一视图不随之滚动。原末尾导航缺陷已有先失败后通过的回归测试；窗口已正常关闭。
  此轮没有实际验收 IME、真实剪贴板、10 MiB 操作、2x、OpenGL 或 Windows/macOS 窗口。

### 2026-09-20 后续实现与包内验证

- 新增预编辑、计时器与视口锚定回归：跨段替换、仅组合光标变化、空组合取消、
  共享模型隔离、Reset 同步回调/卸载、过期粘贴、焦点生命周期、拖选释放、
  自动滚动回调卸载；宽度重排先复现锚点丢失，修复后通过，末页收缩也能收敛。
- 新增块归属测试：加载/大粘贴分块，删除后 undo/redo，ClearHistory 释放中间块，
  放弃 redo 分支、连续 65,664 次输入和反向删除合并、1,000 次替换/清历史。
  检查模型、缓存及历史底层容量中的可达块，不把尚未执行 GC 误判为泄漏。
- `go test ./...`、`go test -race ./gui -run 'Test(Text|Scroll)' -count=1` 通过。
  Windows amd64 / macOS arm64 的 GUI 测试目标与 TextView 窗口程序交叉编译通过；
  未在对应平台运行。
- Linux amd64 / Xeon Gold 5318Y，`-benchtime=1000x -count=3`：
  `BenchmarkTextModelEditCycles` 在 1/10 MiB 文档中局部替换，每 64 次清历史，
  分别 0.608～0.734 / 0.675～0.739 µs/op，均 454 B/op、7 allocs/op；结束时
  可达数据块容量分别 1,049,888 / 10,487,072 字节（断言有界，不是进程 RSS）。
  `BenchmarkTextViewLocalEdit` 使用替身排版，1/10 MiB 分别为
  11.1～11.4 / 9.74～11.75 µs/op，约 7.38 kB/op、45 allocs/op；每次仅新建
  一个段落布局。它们是规模与分配检查，不是相同测量条件的新旧实现性能比较。
- 同一 Linux 桌面环境、Software、1x、10 MiB：XTest 真实点击激活后，连续截图
  比较只在光标的 `1×28` 物理像素区域发生闪烁变化；拖出视口的两次截图显示
  左侧选区继续扩展、右侧内容像素不变，释放后的状态栏选择范围保持不变。
  首次用 XSetInputFocus 激活未保持窗口焦点，不能作为计时器失败证据；改用真实
  点击激活后重测通过。测试窗口已正常关闭；没有修改剪贴板或系统设置。此轮仍
  未实际验收原生 IME、Clipboard、2x 及其他后端/系统。

### 2026-09-20 D/E 实现与阶段验证

- `go test ./...` 通过；相关 GUI/UI 的 Text、Scroll、IME 路由、StyleChanged
  race 检查通过。新增单行 Cluster（组合符/字体更换）、替换与撤销、单行归一化、
  水平滚动、baseline、Submit、只读、Snapshot、过期粘贴、重建和信号生命周期测试。
- Windows amd64 / macOS arm64 GUI 测试目标及 UI 文本编辑窗口程序交叉编译通过。
  这不是对应平台的原生运行结果。
- Ubuntu 22.04.5 / X.Org 1.21.1.4 / VNC :3 / Xfwm4 / Pango 1.50.6、Software、
  1x、1 MiB：XTest 经真实点击、按键和正常 GUI 分发验证单行输入 Hello、Enter
  仅一次 Submit；重建前后文字区域像素不变。非受控框输入 draft 后重复 Rebuild
  仍保留；亮暗切换后两框保留内容；多行选择前三个字符、替换、撤销，两侧共享
  文本同步，发起视图恢复原选区。测试窗口正常关闭，未改剪贴板、文件或系统设置。
- `BenchmarkTextViewScrollAndReflow`：Linux amd64 / Xeon Gold 5318Y，替身字体
  10×20 DIP 单元，`-benchtime=100x -count=3`。1/10 MiB 的滚动均为 2.57 layouts/op，
  约 92.4 kB 分配；窗口宽度在 400/600 DIP 间切换均为 37 layouts/op、约 1.46 MB
  分配。耗时分别为滚动 137～140 / 111～138 µs、重排 1.130～1.137 / 0.914～0.939 ms。
  每次创建数有上限断言；这是规模敏感性检查，不代表真实文字后端或帧率。

### 2026-09-20 双向选区补充回归

- 先以独立 RTL 度量复现：选区收起使用 UTF-8 起止位置，导致两类编辑器在 RTL
  段落或嵌入的 RTL 文本中，Left 落到视觉右端。共享核心现改为比较端点的行和 X，
  保留选中一侧的光标亲和性；没有修改平台接口或复制整个选区文本。
- `go test ./gui -run TestTextEditor -count=1` 通过：纯 RTL、反向选择、嵌入 RTL、
  不连续选择、LTR、跨段落及软换行；真实 Pango 的 RTL 导航/扩选/删除/撤销和
  `e + combining acute` 整簇移动/删除。Pango 用例无需显示连接，在执行原生调用
  的测试 goroutine 内锁定线程，并在线程解锁前释放所有 TextLayout 和 Context。
  这些结果不代表原生窗口输入法或其他平台已验收。
- 修复后 `go test ./...`、
  `go test -race ./gui ./ui -run 'Test(Text|Scroll|Window.*IME|StyleChangedRefreshes)' -count=1`
  通过；Windows amd64 / macOS arm64 的 GUI 测试目标交叉编译通过。
  全仓测试仍包含既有写出图片的测试；本次新增用例不写测试产物、不连接桌面。

### 2026-09-20 原生平台验证与环境缺口

- 使用 remote-shell 的 mac/win 已有连接，将交叉编译的测试程序传到独立临时目录；
  未覆盖远端仓库、修改系统设置或启动桌面测试窗口。
- macOS 15.7.7（24G720）、arm64：CoreText 测试程序实际运行
  `-test.run TestTextMetrics -test.v` 通过。新增 emoji/ZWJ、空文档、连续空行、
  末尾 LF、CRLF 测试检查 UTF-8 内容覆盖和分隔符剥离；不要求原生后端凭空补出
  空行 glyph 或编辑光标。GUI 测试程序实际运行
  `-test.run 'Test(Text|Scroll|Window.*IME|StyleChangedRefreshes)' -test.count=1` 通过；
  GUI 这组用例使用替身窗口/排版，不能记为 CoreText 编辑器窗口或输入法验收。
- Windows 10 22H2（19045）、amd64：程序传输 SHA-256 与本地相同，但
  `directwrite-http.test.exe -test.run TestTextMetrics -test.v` 被系统拒绝执行
  （Access denied）；不是测试断言失败，也不是通过。没有调整安全策略、文件权限
  或尝试绕过；拒绝原因尚未确认。新增 DirectWrite 度量用例目前仅交叉编译通过。
- 桌面交互未执行：Windows SSH 进程位于 Session 0，用户 RDP 桌面位于 Session 1；
  macOS 的 CGSession 报告 `CGSSessionScreenIsLocked=1`，执行上下文的
  `AXIsProcessTrusted()` 为 false，System Events 查询也以 AppleEvent 超时结束。
  能取得桌面截图不等于具有交互权限。继续原生 IME/Clipboard/HiDPI 验收需要
  可操作且已授权的桌面会话；不以临时 SYSTEM 任务、放宽安全设置等方式绕过。

### 2026-09-20 Linux 隔离桌面、剪贴板与 XIM 回归

- 环境：Ubuntu 22.04.5 / 独立 Xtigervnc :91（2560×1600，96 DPI）/ Xfwm4，
  Pango 1.50.6，Fcitx 4；OpenGL 图像资源类型与 PixelScale=2 由窗口用例首次
  Paint 日志确认。GL 实际由 Mesa 23.2.1 llvmpipe/LLVM 15 软件驱动，不是 GPU
  性能测试。输入走 XTest 和正常事件分发，不使用 DevServer。独立 XDG 配置、
  D-Bus 会话与剪贴板，不改用户正在使用的桌面配置或剪贴板。
- 修复新桌面启动崩溃：没有 RESOURCE_MANAGER 时，`CStringTemp("")` 传 NULL
  给 `XrmGetStringDatabase`；现绑定传真实空 C 字符串。无桌面回归在修复前
  发生 SIGSEGV，修复后通过。三平台 `NewInputMethod` 同时消除错误路径的
  typed-nil interface；输入法不可用时不再对 nil 原生对象调用 SetEnabled/Reset。
  Linux 不存在的 XIM 服务场景实际启动、导航及关闭通过，不伪造字符输入回退。
- 1 MiB/12,059 行共享模型：粘贴含 LF、中文、组合符及 emoji 的文本替换 `Edi`，
  undo 恢复原选区、redo 后全选复制；1,048,686 字节与独立字符串参考逐字节一致。
  Fcitx root-style 的选区替换、Esc 取消、提交后一次撤销也通过；这不证明相邻
  普通输入与 IME 提交的历史隔离——单独测试 `a`→拼音“你”确实复现一次全撤销。
- 补齐 XIM 样式查询和四个预编辑回调；原生 ABI、增量插入/替换/删除、仅更新
  feedback、宽字符/locale 多字节转换、字符到 UTF-8 caret 偏移、Reset 丢弃
  待发更新、销毁后回调忽略，均有无桌面测试。不同 XIC 的数据分别持有。
- Fcitx 4 的 `UseOnTheSpotStyle` 默认 false；仅在隔离会话中开启后重启验证。
  内联组合时只有左视图临时显示“你”，右视图及模型字节数保持不变；Esc 后复制
  得到原选区 `Edi`。提交替换后的全文核对通过；输入 `a` 后组合提交“你”，
  第一次撤销保留 `a`、第二次撤销恢复初始全文，均通过 1 MiB 精确剪贴板核对。
  组合期间关闭窗口正常退出；测试结束后已关闭本次独立输入法、窗口管理器和
  X server。GUI 包内另覆盖单行/多行的 preedit-end→commit 顺序，确保组合前后
  的普通连续输入仍各自合并成组，而组合提交独立。
- 尚未通过：root-style 的相邻输入分组；Fcitx 内联模式候选窗本次落在窗口底部，
  候选定位需继续核对原生调用，不能以经验坐标补偿。尚未覆盖其他输入法、全部
  键盘布局、Software 2x、完整后端/三平台矩阵。本轮不把 C/E 切片标记完成。
- `go test ./...` 通过；
  `go test -race ./platform/linux/x11 ./platform/linux/libs/xlib -run 'Test(InputStyle|Preedit|XIM|ResourceDatabase|InputMethodFactory)' -count=1`
  通过。全仓仍存在既有写出 PNG 的测试，本轮不迁移或删除它们。
- Windows amd64 win32、macOS arm64 cocoa 的 factory 回归测试交叉编译通过，
  不是原生运行。额外检查 Linux arm64 时，`go test -c ./platform/linux/x11`
  因既有 `libc/syscall_linux.go` 使用 arm64 不存在的 `SYS_PIPE`/`SYS_POLL` 常量
  失败；该文件本轮未修改，不以 amd64 通过替代 arm64 验证，也不顺带扩大修改。

原生实现依据：[Xlib 预编辑回调契约](https://xorg.freedesktop.org/releases/current/doc/libX11/libX11/libX11.html#Input_Method_Callback_Semantics)、
[GTK 3.24.43 的 XIM 适配](https://github.com/GNOME/gtk/blob/3.24.43/modules/input/gtkimcontextxim.c)
（此处特意参考仍含 XIM 实现的 GTK3，不将其冒充 GTK4 行为）、
[Fcitx 4.2.9.9 的样式公布与预编辑实现](https://github.com/fcitx/fcitx/blob/4.2.9.9/src/frontend/xim/xim.c)。
采用样式协商及增量回调，不引入 GTK 依赖或搬入输入法特定的配置策略。

### 2026-09-20 提交来源、候选定位与 Software 裁剪

- 补齐 `InputMethodResult.Composed` → `IMCommit` 的原生事实传递；
  `ConnectCommit` 现在接收值类型 `IMCommit`，第三方文字控件应从 `.Text` 取文字。
  TextInput/TextView 在同一私有编辑核心内执行分组，不改变 TextModel 的平台独立性。
  包内测试覆盖无 preedit 的连续普通字符、ASCII 组合提交、中文组合提交、两侧普通
  输入的 undo/redo，以及信号重入时来源标记不被覆盖。Windows UTF-16 组装新增
  `WM_CHAR` 与 `WM_IME_CHAR` 来源测试；本轮只交叉编译，未宣称原生 Windows 通过。
- Linux 独立 :91 / Xfwm4 / Fcitx 4.2.9.8 / libX11 1.7.5，1 MiB：root-style
  （`UseOnTheSpotStyle=False`）下 `a`→“你”后 undo 得 `a`、再 undo 得原文；
  OpenGL（llvmpipe）2x 的两次 redo 同样通过，每步全选复制按独立参考逐字节核对。
  Software 2x 的提交和两次 undo 也通过。此项不依赖 callback preedit 来区分提交。
- 候选定位已定位到原生版本限制，而非 DIP/物理像素换算错误：系统 libX11 1.7.5
  的 `XSetICValues` 拒绝 callback 样式的 spot；上游 1.8.2 明确加入支持。
  在仓库外构建 libX11 1.8.4，仅通过进程 `LD_LIBRARY_PATH` 加载，同一 OpenGL
  2x 测试程序得到属性接受且候选位于内联 caret 下方。没有安装或升级系统库，
  没有坐标补偿、版本硬编码分支或替用户更改输入法配置。
- Software 2x 全选末页额外复现选区溢出到状态栏：形状与结构 clip 完全不相交时，
  `image.Rectangle.Intersect` 的零矩形被 scanFT 当作无裁剪。现在 `setShapeClip`
  返回是否有交集，无交集跳过本次扫描绘制且清理路径，保留后续绘制状态。
  OpenGL/Direct2D 的持久 scissor/axis clip 路径不计算这种逐形状空矩形，未加入
  GUI 特例或改动公共 Painter API；两者的原生全场景矩阵仍待验收。
- 新增 `TestDisjointShapeClip`：12 类绘制操作 × 1x/2x × 无变换/旋转缩放平移，
  对每个物理像素断言裁剪外保持原色，同时验证无 clip 时确实绘制、跳过后同帧
  绘制不受污染。修复前 FillRect 1x 已污染 64 像素，旋转图片及其他扫描路径也失败；
  修复后通过。Software 2x 原生末页全选截图复验，状态栏不再被选区背景覆盖。
- 验证：`go test ./...` 通过；
  `go test -race ./platform/graphics/software ./gui ./ui ./platform/linux/x11 ./platform/linux/libs/xlib -run 'Test(DisjointShapeClip|PainterCompositesTransparentBrushAndPreservesClip|Text|Scroll|Window.*IME|IMContext|StyleChangedRefreshes|InputStyle|Preedit|ComposedLookup|XIM|ResourceDatabase|InputMethodFactory)' -count=1`
  通过。Windows amd64 的 win32/gui/software 和 macOS arm64 的 cocoa/gui/software
  测试目标交叉编译通过；不替代原生窗口运行，前述权限/桌面缺口仍保留。
  全仓既有测试仍会写 PNG，本轮新增测试不产生仓库内文件。
- Software 的两次首次立即读取剪贴板曾返回 `target STRING not available`；没有
  重发复制操作，稍后从同一活跃窗口读取即与独立参考全文一致。自动输入送达不等于
  GUI 已处理完毕，记录该观察，不用延迟读取结果宣称同步复制延迟或性能已验收。
- 清理时在 Alt+F4 后约 100 ms 立即停止隔离 Fcitx，触发窗口退出期间阻塞在
  `XIC.ResetIC` → `Xutf8ResetIC`；SIGQUIT 调用栈确认，测试进程随后被结束。
  这次退出不是通过。重启同一隔离环境，保持输入法存活，Software 2x 全选复制
  后关闭窗口正常退出（exit 0），确认退出后才停止 Fcitx。XIM 服务中途退出的
  生命周期处理尚待补齐，不因正常关闭通过而抹去该场景。

原生依据：
[libX11 1.8.2 发布记录](https://github.com/mirror/libX11/blob/libX11-1.8.2/README.md#release-182)、
[libX11 1.8.4 XIM_COMMIT 接收与 lookup](https://github.com/mirror/libX11/blob/libX11-1.8.4/modules/im/ximcp/imDefLkup.c)、
[WM_IME_CHAR](https://learn.microsoft.com/en-us/windows/win32/intl/wm-ime-char)。
这里只使用原生可识别事实；无来源信息的 API 不根据文字内容伪造组合标记。

### 2026-09-20 XIM 失联后的句柄安全

- 补齐原生 XNDestroyCallback 绑定及平台处理，遵守 Xlib 自行销毁 XIM/XIC 的
  所有权。先失效句柄，再于原生回调退出后通知 GUI 取消预编辑，不调用失效对象的
  Reset/Destroy。无自动重连，具体契约和原生同步等待限制见 [IME](DesignIME.md)。
- Linux Ubuntu 22.04.5 / 独立 Xtigervnc :91 / Xfwm4 / Fcitx 4.2.9.8、callback
  模式 / libX11 1.7.5 / Software 2x：旧程序内联组合“你”，停止输入法后预编辑
  残留；随后关闭窗口触发 `textEditor.cancelPreedit` → `XIC.ResetIC` 的 SIGSEGV。
  本轮真实复现的是已失效句柄崩溃，不与上一轮同步调用中途断线的阻塞混为一谈。
- 修复版在同环境选择 `Edi`、开始组合再停止服务：预编辑取消，选区和两视图内容
  保留；复制选区精确得到 `Edi`，方向键/全选/复制正常，全文 1,048,655 字节与
  独立参考一致；之后关闭窗口 exit 0。未使用 DevServer、未修改用户桌面/剪贴板。
- 无桌面测试覆盖无关/重复通知、多个上下文、只取消此前启用的上下文、通知前的
  Reset/Destroy、通知回调销毁另一个接收者，确认无过期通知或重复释放。
  `go test ./...`、
  `go test -race ./platform/linux/x11 ./platform/linux/libs/xlib -run 'Test(InputMethodLoss|Preedit|XIM|InputMethodFactory)' -count=1`
  通过。此次生产代码仅在 Linux 文件内，无 Windows/macOS 新行为；没有重跑其
  原生矩阵，也没有把其此前交叉编译结果当作此次运行验证。

### 2026-09-20 UI 10 MiB / Software 2x 验证

- 环境：Ubuntu 22.04.5 / 独立 Xtigervnc :91 / Xfwm4 / Pango 1.50.6 /
  Fcitx 4.2.9.8 callback 模式 / libX11 1.7.5。运行
  `GOUI_PLAT_PAINTER=software GOUI_PLAT_SCALE=2 go run ./tests/window/ui/textediting -mb 10`；
  本次执行的是该入口构建的二进制。首帧通过公开 Painter API 创建临时图像，
  日志确认 `*software.imageResource`、PixelScale=2；没有新增生产测试接口。
  XTest 输入经正常事件路径，复制后与独立构造的 UTF-8 内容逐字节比较。
- 单行：CRLF/LF 粘贴归一为空格，中文/组合符/ZWJ emoji 保留；Enter 仅一次
  Submit，undo/redo 恢复准确。两次 Rebuild、亮暗与主题色切换后，受控框保留
  已编辑文本，非受控框保留 `draft`。Readonly 阻止插入/删除但允许选择/复制。
- 共享多行：初始 10,485,833 字节；含 CRLF/LF、中文及 emoji 的替换粘贴得到
  10,485,864 字节，undo 恢复初始全文及 `Edi` 选区，redo 恢复粘贴后全文。
  左侧只读时修改右侧仍生效；在开头插入 `z` 得到 10,485,834 字节，undo 回到
  初始全文。每一步检查独立预期，不只检查两侧显示相同。
  Fcitx 日志出现 `Selection is too long` 警告；GOUI → xclip 的上述全文核对通过，
  但没有据此宣称输入法自身的剪贴板功能不受长度限制。
- 原生 IME：选择 `Edi` 后组合“你”，只有当前视图显示预编辑，模型字节数不变；
  Esc 取消后复制仍为 `Edi`。提交后全文等于 `你 + initial[3:]`，一次 undo 恢复
  选区；结束前再次复制核对初始全文。保持输入法运行，内联组合期间关闭窗口
  exit 0，然后才停止本次隔离桌面。系统 libX11 1.7.5 的 callback 候选窗仍位于
  底部，这是前述已确认的原生版本限制，不能记作候选定位通过。
- 原生 Cluster/RTL：单行 Shift+方向键复制得到完整 `👩‍💻`（11 字节）与
  `e + combining acute`（3 字节）。粘贴 `אבג`，全选后 Left 收起到视觉左端，
  Shift+Right 复制为 `ג`；删除后全文为 `אב`，undo 恢复原文及 `ג` 选区。
  可重复的操作与预期已补充到窗口程序入口注释。
- 首次快速脚本在异步粘贴完成前改变选区，回复按既定契约被丢弃，得到原始全文；
  确认粘贴完成后再全选核对通过。没有修改过期保护以迎合脚本，没有以固定延时
  或单次耗时宣称响应性能通过。此次未验证 resize 锚点精度或完整混合方向导航。
- `go test ./...` 通过；GUI/UI、Software clip 和 Linux XIM 相关 race 用例通过，
  `go test -race ./theme/modern -count=1` 通过。UI 窗口程序 Windows amd64 /
  macOS arm64 交叉编译通过，未执行其原生窗口验收。全仓既有 PNG 输出仍保留。
  本轮只补充窗口用例和文档，没有新增生产行为；C/E 的跨平台验收仍未完成。

### 2026-09-20 原生排版接入编辑器的三平台测试入口

- 原 `TestTextEditorPangoNavigation` 的全部断言提取到包内
  `testTextEditorNativeNavigation`，保留原测试名称与覆盖；新增 CoreText 与
  DirectWrite 平台入口。不新建测试框架，不导出生产状态，也不依赖桌面或 DevServer。
  每个单行/多行子测试在执行原生调用的 goroutine 内锁定 OS 线程，先卸载控件、
  释放 TextLayout/Context，再解锁。
- 覆盖真实排版 Cluster 到 GUI 事件分发的接入：纯 RTL 选区向视觉左侧收起、
  Shift+Right 反向扩选及实际光标 X 增加、删除/撤销恢复内容和方向选区；
  `e + combining acute` 的整簇导航与删除。预期是明确 UTF-8 偏移和视觉顺序，
  不固定不同平台的字形宽度或抗锯齿像素。
- Linux Pango：`go test ./gui -run TestTextEditorPangoNavigation -count=1 -v`
  与相同用例的 race 检查通过。
- macOS 15.7.7（24G720）arm64：交叉编译 GUI 测试程序，经 remote-shell 传输到
  独立临时目录并核对 SHA-256。原生执行
  `-test.run TestTextEditorCoreTextNavigation -test.count=1 -test.v`，单行和多行均通过；
  `-test.run 'Test(TextEditor|TextInput|TextView)' -test.count=1` 也通过。只有新增
  CoreText 入口明确使用真实排版，其余替身测试不能算作真实 CoreText 场景覆盖。
- Windows amd64：包含 `TestTextEditorDirectWriteNavigation` 的 GUI 测试目标
  交叉编译通过，本轮未执行原生测试，也未改动用户已有的 DirectWrite 实现修改。
- 重查桌面条件：macOS 仍报告锁屏、`AXIsProcessTrusted=false`；Windows SSH
  仍在 Session 0，用户 RDP 在 Session 1。没有修改权限、解锁、切换会话或启动
  桌面窗口。以上离屏运行不替代真实 IME/Clipboard、窗口绘制及 HiDPI 验收。
- 本轮不改生产行为；`go test ./...` 与
  `go test -race ./gui ./ui -run 'Test(Text|Scroll|Window.*IME|IMContext|StyleChangedRefreshes)' -count=1`
  通过，既有 PNG 测试副作用仍保留。

### 2026-09-20 Windows 原生离屏验证补齐

- 环境：Windows 10 22H2（19045）amd64，SSH Session 0。通过已有 remote-shell
  连接及其 SFTP 通道传输到独立临时目录，GUI、DirectWrite、win32 三个测试程序
  的 SHA-256 均与本地交叉编译产物一致。本轮运行未出现执行权限拒绝，没有调整
  ACL、安全策略或用户会话；此前 `Access denied` 的具体原因仍未知，保留历史记录。
- `TestTextEditorDirectWriteNavigation` 单行、多行均通过：真实 DirectWrite
  排版，RTL 视觉移动、扩选、删除/撤销和组合字符整簇编辑。GUI 的
  `-test.run 'Test(TextEditor|TextInput|TextView)' -test.count=1` 通过；其他使用
  替身的用例仍只证明 GUI 逻辑，不扩大为真实后端覆盖。
- DirectWrite `-test.run TestTextMetrics -test.count=1 -test.v` 通过，包含
  UTF-8 Cluster 覆盖、中文/组合符/emoji、RTL 几何、零宽文本、空行及 CRLF/LF。
  `TestTextPremultipliedAlpha` 通过，覆盖 1x/2x 离屏栅格化、半透明前景和区间
  颜色反复修改；未执行会向源码目录写 PNG 的旧演示测试。
- win32 `-test.run TestInputMethod -test.count=1 -test.v` 通过：UTF-16 代理对、
  不完整序列、控制字符、重复计数、Reset/禁用/销毁、普通/组合提交来源及工厂
  错误返回。这些用例不创建 HWND、IMM 上下文或消息循环，不代表真实 IME 已通过。
- 本轮第一次标准输入传输停滞，主动结束该专用进程后改用 SFTP；第一次测试启动
  的未加引号 `-test.*` 参数被 PowerShell 拆开，程序以 usage/exit 2 结束。
  改为逐个字符串实参后上述测试 exit 0；没有将失败的启动计作测试通过。
- 本轮只更新验证记录，没有修改生产代码或已有用户 DirectWrite 改动。
  Windows/macOS 真实窗口、输入法候选、键盘布局、Clipboard 与绘制后端矩阵
  仍需可操作的交互桌面；Session 0 可运行离屏测试，不等于能验收用户桌面行为。

### 2026-09-20 正文裁剪边界锚定及小数行高收敛

- `tests/window/gui/textview` 增加可选 `-numbered`，在段落首/中/尾显示编号，
  默认数据不变。重复的无编号段落不足以证明 resize 后仍保留同一文字位置。
- 真实 Pango 连续宽度重排回归先失败：期望 scroll=950，实际 922，偏差整行
  28 DIP。原锚点从 `offset-padding` 取样，可能选择完全被正文 padding clip
  遮住的上一行。现在保存、恢复和 overscan 高度修正统一以正文可见起点取样。
  确定性 10 DIP 字宽/20 DIP 行高用例固定预期：旧视口 20 字/行、offset=21，
  扩宽到 30 字/行后保留字节 20 所在行及 1 DIP 裁剪，ScrollY 必须等于 1。
- Windows DirectWrite 新回归继而失败：scroll=1313/contentOffset=1314，
  anchor/reveal 请求始终未清除。使用 20.5 DIP 行高的替身也复现 offset 在 79/80
  之间变化。修复恢复位置的方向性 floor/ceil，改为与 ScrollView 一致的 round
  和整数最大范围钳制；不是增加布局次数上限或放宽断言。
- 新的共享原生回归从原生 TextLine 推导正文可见位置，不以编辑器自己的 anchor
  作为预期；跨 28 个初始偏移、9 次连续宽度变化检查顶部位置及请求收敛。1 DIP
  容差来自 ScrollView 的整数 DIP 契约，不是为各平台字形差异设宽松金图容差。
  Pango 1.50.6 / Linux amd64、CoreText / macOS 15.7.7 arm64、DirectWrite /
  Windows 10 22H2 amd64 都已实际运行通过。后两者经 remote-shell 传输并核对
  SHA-256，原生命令为 `-test.run 'TestTextView.*(Resize|Anchor)' -test.count=1 -test.v`。
- 既有宽度锚定断言同步改为正文 clip 坐标，保留视口上方增删、末页范围收敛及
  初始换行不滚走等覆盖；新增隐藏行锚定和小数行高回归。未改平台、Painter、
  TextLayout 接口或用户已有的 DirectWrite 修改。
- `go test ./...` 与 GUI/UI Text、Scroll、IME 路由及样式相关 race 检查通过；
  Windows/macOS GUI 目标交叉编译通过。既有测试的 PNG 输出副作用仍保留。
- 窗口复验仍未通过完整 resize 验收：Ubuntu 22.04.5 / 独立 Xtigervnc :91 /
  Xfwm4 / Software 2x，`-mb 10 -numbered`，10,485,857 字节、94,473 行。
  XTest 拖动原生右边框，客户区宽度从 2100 到 1836 再到 2443 物理像素；
  稳定画面中左侧顶部从 `000005:middle` 变为 `000005:start`，右侧保持
  `000009:middle` 附近。须继续区分连续重排中的锚点重采样、显示行变化及绘制
  几何因素；不能因上述离屏断言通过就把该窗口场景标为完成。初次即时截图还曾
  捕捉到未完成重绘，后续使用稳定画面核对，不以该瞬态判断最终布局。

### 2026-09-20 连续重排锚点及 Pango 栅格化换行

- 已确认连续重排会重新采样显示行首，从而丢失原字符。确定性回归在
  20 字/行、scroll=21 时锚定字节 20；变为 30 字/行再恢复原宽度，旧实现
  scroll=1，预期 21。现在保留仍匹配实际视口的逻辑锚点，不新增 resize 手势
  状态或平台补偿。主动滚动 1 DIP、重排后的模型插入及末页收敛均有回归。
- 三平台原生 resize 回归改为整段宽度序列使用最初的原生 TextLine 字符位置，
  而不是每一步重采样预期。Pango 本地、CoreText/macOS 15.7.7 arm64、
  DirectWrite/Windows 10 22H2 amd64 实际执行通过。远端只使用 win/mac，
  经已有 SSH/SFTP 传输、SHA-256 核对；未修改权限或用户桌面会话。
- 窗口往返位置正确后，较宽窗口的实际文字仍与度量不一致。Pango 按行着色
  像素回归复现：sans 18、宽 615 DIP、1.5x/2x，属于第二显示行的字符画到
  下一行。旧实现绘制前 `UpdateLayout` 将缩放传入排版，改变 hint 后的换行；
  绘制后恢复上下文不能修复已生成的位图。
- 现在 Pango 保留已测量的布局，以 Cairo CTM 缩放后直接 `ShowLayout`；灰度
  抗锯齿选项在独立的 PangoContext 创建时配置，原生复制后立即释放临时选项。
  `textPainter` 不再持有无效的重复 font options。CoreText 绘制既有 CTFrame，
  DirectWrite 绘制既有 layout，未发现同样的绘制时重排步骤；这只是代码审视，
  不替代其像素矩阵验收，未修改用户已有的 DirectWrite 实现。
- 参考依据：Pango 1.50.6 的
  [update_layout 实现](https://github.com/GNOME/pango/blob/1.50.6/pango/pangocairo-context.c#L463)
  会更新布局上下文；GTK 4.12.5 的
  [Cairo 文本节点](https://github.com/GNOME/gtk/blob/4.12.5/gsk/gskrendernodeimpl.c#L4727)
  绘制已定位 glyph，采用其“测量与绘制分离”的原则，不引入 GSK 抽象。
- 新回归同时比较 Rasterize 前后的全部 Line/Cluster，进一步暴露 Pango 1.50.6
  的 `pango_layout_iter_copy` 漏拷贝 `end_x_offset`：跨字体 run 的 X 会读取
  未初始化值，race 构建下稳定复现。例如同一中文字的 X 从 298 变为 329.95508。
  [对应源码](https://github.com/GNOME/pango/blob/1.50.6/pango/pango-layout.c#L6641)。
  MeasureMetrics 改为两个从头创建并同步前进的原生游标，一个保留空行、一个
  遍历 Cluster；不复制游标、不按系统版本分支，也不调整错误坐标。
- 像素回归覆盖 430～620 DIP 的 39 个宽度、1x/1.5x/2x，独立着色每一原生
  显示行，检查其非透明像素不越过度量行框；仅允许边界 1 物理像素的取整误差。
  各行使用独立布局，避免把重叠颜色区间的覆盖行为混入换行验证。初稿曾复用
  布局并反复清除范围，观察到旧区间残留；该范围覆盖问题仍待独立核查，未把
  相关失败宣称为通过，也未调整现有颜色测试。
- 本地窗口：Ubuntu 22.04.5、独立 Xtigervnc :91 / Xfwm4、Software 2x，
  `-mb 10 -numbered`。XTest 右边框宽度 2100→1836→2443→2100 物理像素，
  高度 1400。Pango 修正后，2443 宽度下 `000005:middle` 与 `start` 确实处在
  同一绘制行，不再是度量与像素的不同换行；恢复 2100 后，左右顶部正文采样
  区域分别与本次初始截图逐像素相同。末尾的双游标修正另由包内度量/race 验证。
  应用正常关闭；此项不覆盖 Windows/macOS 实际窗口或所有渲染后端。
- 最终 `go test ./...` 通过；
  `go test -race ./gui ./ui ./platform/typography/pango -run 'Test(Text|Scroll|Window.*IME|IMContext|StyleChangedRefreshes)' -count=1`
  通过。新增度量不变断言最初失败，保留断言并修正双游标后再通过，没有放宽
  容差或跳过失败。全仓原有 PNG 输出副作用仍存在。Windows 首次启动的引号
  被远端 CMD 解释为管道而失败，改用 PowerShell 编码命令后实际测试 exit 0；
  本地两次误用不存在的 `python` 未执行脚本，改用 `python3` 后成功。

### 2026-09-20 Pango 范围样式覆盖修正

- 独立回归确认不只是颜色问题：先设置内部区间，再覆盖整个文本时，颜色、
  字体、下划线、删除线都会残留旧属性。全透明覆盖后的像素仍存在非零通道，
  字体恢复后仍保留原内部区间的不同度量；这些测试在修复前明确失败。
- 根因是 setter 使用 `pango_attr_list_insert`，它按范围插入但不删除旧属性。
  按 [Pango AttrList.change 契约](https://docs.gtk.org/Pango/method.AttrList.change.html)，
  改为原生 `Change`，覆盖同类型的重叠范围并合并相邻相同属性。绑定增补在原
  `libs/pango` 下；仍由原生列表接管属性，未引入 Go 区间算法或新增缓存状态。
- 新增包内 `TestTextAttributesReplaceNestedRanges`、
  `TestTextColorClearOverlappingRanges` 和 `TestTextColorOverlapPreservesOutsideRange`。
  1x/2x 原生像素分别与独立构造的最终样式比较，字体同时比较 Line/Cluster；
  透明清除要求全部 RGBA 通道为零，交叉覆盖检查范围外的旧值仍被保留。
  保留现有预乘、半透明、颜色反复修改及栅格化换行回归，不放宽断言。
- CoreText 使用 CFAttributedStringSetAttribute 覆盖原范围；DirectWrite 按调用顺序
  应用 DrawingEffect。代码审视未发现相同 Insert 叠加问题，未改动这两平台的
  实现或用户已有修改。本次验证在本地 Linux/Pango 1.50.6，无窗口、无远端调用；
  不据此追加 Windows/macOS 原生像素通过记录。
- `go test ./...` 通过；
  `go test -race ./platform/typography/pango ./gui ./ui -run 'Test(Text|StyleChanged)' -count=1`
  通过；新增范围覆盖用例 `-count=5` 重复运行通过。全仓既有 PNG 测试产物仍保留，
  本次未运行窗口程序；跨平台窗口/IME/Clipboard 验收仍未完成。

### 2026-09-20 OpenGL 窗口复验与远距离导航锚点优先级

- 本地 Ubuntu 22.04.5 / 独立 Xtigervnc :91 / Xfwm4，无桌面合成；OpenGL
  实际为 Mesa 23.2.1 llvmpipe，首次绘制确认 imageResource 和 PixelScale=2。
  10 MiB 编号文档（10,485,857 字节、94,473 行），左右分别滚到不同段落，
  XTest 拖动右边框，使客户区宽度 2100→1836→2443→2100、高度 1400。
  Pango 最终双游标/范围属性版本的窗口换行正确；恢复后顶部正文采样区
  `(36,154)-(1010,450)`、`(1072,154)-(2035,450)` 与初始像素完全一致。
- 随后 Ctrl+Home、Ctrl+Shift+End 暴露新问题：选区终点已到文档末尾，正文
  仍停在更早的段落。包内最初只检查 ScrollView 偏移也会误通过；加上正文实际
  offset 及 pending 状态断言后，确定性替身和 Pango 原生回归均明确失败。
  原因是延迟测量保留的浏览锚点覆盖宿主已经应用的光标滚动位置，两者偏移
  分离，宿主又没有新的位置可滚。修复在光标导航期间放弃恢复旧浏览锚点，
  不增加布局循环、强制重绘或平台补偿。
- Windows/macOS 原生回归进一步发现小数行高下导航请求不结束：宿主已达整数
  最大偏移，完成条件仍比较小数上限。独立 101 行 × 20.5 DIP 回归先失败，
  固定预期 `floor(101*20.5+8-108)=1970`；现在按同一整数 DIP 范围判断完成。
- Pango/Linux amd64、DirectWrite/Windows 10 22H2 amd64、CoreText/macOS
  15.7.7 arm64 的 `TestTextView.*(RevealEnd|ResizeAnchor)` 原生离屏测试通过。
  远端仅 win/mac，经 SSH/SFTP 传输并核对 SHA-256；不需要交互桌面。
  三平台入口复用相同的 95,000 段落/真实排版回归，只消费实际调度的布局帧，
  检查光标可见、正文/宿主偏移一致及锚点/导航/宿主请求结束。
- 导航修复后，Linux OpenGL 和 Software 2x 窗口中最后编号 `094465` 及末尾
  空行光标实际可见。工具栏 `(29,45)-(2050,145)`、右侧正文
  `(1072,154)-(2035,450)` 与各自初始截图像素完全一致。Software 的首次即时
  截图尚未显示输入结果，随后核对稳定画面，未重复发送按键。最后的小数上限
  修正在 Software 窗口及三平台离屏运行；未再次跑 OpenGL 全矩阵。
- 临时截图保留在仓库外 `/tmp/goui-text-final-gl.cu5Cot/`。诊断打印已移除，
  未改 Painter、平台实现、公共 API 或用户 DirectWrite 修改。此轮不含 IME/
  Clipboard/Windows/macOS 真实窗口操作，也不将 llvmpipe 当作硬件 GPU 验收。
- 最终 `go test ./...`、
  `go test -race ./gui ./ui -run 'Test(Text|Scroll|Window.*IME|IMContext)' -count=1`
  和 `git diff --check` 通过。既有 PNG 输出副作用保留。所有测试窗口正常退出；
  专用 Xfwm4/Xtigervnc 会话已停止，Xfwm4 在会话关闭时报告连接断开并 exit 1，
  不作为应用运行失败，也不将其记为测试通过。未更改用户桌面或远端系统权限。

### 2026-09-20 UI OpenGL 2x 主题与编辑状态复验

- 本地 Ubuntu 22.04.5 / 独立 Xtigervnc :91 / Xfwm4，Mesa 23.2.1 llvmpipe，
  无 WM 合成。重新构建 `tests/window/ui/textediting -mb 10`；实际资源类型为
  OpenGL，PixelScale=2，客户区 2160×1440 物理像素。未连接远端 Linux。
- 经 XTest 正常按键分发，初始单行末尾 Shift+Left 复制完整 ZWJ emoji（11 字节），
  再导航复制 e+combining acute（3 字节）；均与独立 UTF-8 预期逐字节相等。
  Enter 只产生一次 Submit，日志和状态栏均为 1。
- 未声明 Text 的输入框粘贴 `draft`；Rebuild 后切到暗色、Accent 切到 Black，
  内容仍为 `draft`，重新复制精确等于 5 字节。声明式单行中的组合字符选区与
  多行 `Edi` 选区保持，状态栏左侧仍为 `{Anchor:0 Caret:3}`、rebuilds=1。
  右侧全文复制与独立构造的初始 10,485,833 字节文档完全相等。
- 非受控输入框粘贴 `אבג`，全选、Left、Shift+Right 后复制 `ג`（2 字节）；
  Backspace 后复制全文为 `אב`（4 字节），撤销后选择恢复为 `ג`，全文恢复为
  `אבג`（6 字节）。切回亮色后文本及左右独立选区保持，选区/正文可见。
- 本会话未启动 XIM：尝试物理字母 d/r/a/f/t 时没有文本提交，不能计作普通
  输入通过，也不能把后续粘贴成功当作键入成功。无 XIM 时输入行为仍需独立
  核查（后续已定位为服务与环境变量不匹配，见下一节）；本轮未验证 IME、
  候选位置、只读、拖选或完整后端/缩放矩阵。
- 本轮仅补验收记录，未修改生产代码或测试逻辑。剪贴板仅在独立 :91 会话中
  改写；截图保留于 `/tmp/goui-ui-final-gl.oGzxeO/`，窗口正常退出。没有调用
  DevServer，没有更改用户桌面/输入法或远端权限；Windows/macOS 窗口门槛仍未满足。
- `go test ./...` 与 `git diff --check` 通过；此轮未修改代码，未重复 race 或
  远端编译/运行。既有 PNG 测试副作用保留，专用 Xfwm4/Xtigervnc 均已退出。

### 2026-09-20 隔离桌面本地输入法对照

- 同一 Ubuntu 22.04.5 / Xtigervnc :91 / Xfwm4 无合成桌面，系统 libX11
  1.7.5-1ubuntu0.3，Mesa 23.2.1 llvmpipe；重新构建 UI 用例，10 MiB，
  实际 OpenGL imageResource、PixelScale=2。根窗口没有 _XIM_SERVERS 属性。
- `XMODIFIERS=@im=none`：第二个空框通过 XTest 逐键输入 d/r/a/f/t，显示
  draft；Enter 后状态栏及日志 Submit=1；全选复制独立断言精确等于 5 字节
  draft。本次没有预先粘贴，证据确实覆盖文字键入。
- 关闭后用同一二进制、同一桌面、同一后端及缩放，仅改为 `@im=fcitx`：
  相同字母按键不产生文字，输入框保持空；Enter 仍触发一次 Submit。
  直接调用系统 libX11 对照断言：fcitx 的 XOpenIM 返回空，none 返回非空。
  因此此前问题是独立桌面继承了不存在服务的配置，不是编辑器遗漏普通字符。
- 保留当前原生输入契约，不新增 GUI 字符映射、自动切换输入法或重连策略。
  DesignIME 和两处 GUI/UI 窗口用例入口补充本地输入法与真实 IME 的启动区别。
  本项不代替中文组合、候选定位、其他键盘布局或三平台整体验收。
- 截图与本轮二进制保留于 `/tmp/goui-local-im.dzPyth/`。两个窗口均正常退出，
  已停止专用 WM/Xserver；仅在独立桌面复制文本，没有更改用户输入法、系统库
  或远端桌面权限。本轮无生产代码改动。
- `go test ./...` 通过（多数包使用缓存，既有输出 PNG 的测试仍有原有副作用），
  `git diff --check` 通过。本轮未重复 race、跨平台编译或远端运行；
  Windows/macOS 交互桌面及其原生窗口验收仍待完成。

### 2026-09-20 Linux 键盘布局与本地 Compose 验证

- Ubuntu 22.04.5 / 独立 Xtigervnc :91（1400×1000）/ Xfwm4 无合成；
  系统 libX11 1.7.5，XMODIFIERS=@im=none，实际 Software imageResource、
  PixelScale=1，UI 用例 1 MiB。未连接远端 Linux，未修改用户桌面的键盘布局。
- 独立桌面设置 fr / pc105 / evdev，确认原生 keycode 24/25/11 对应 a/z/é，
  34 对应 dead_circumflex。XTest 逐键输入 a/z/é、^ 后 e、AltGr+E；单行
  复制独立断言精确为 azéê€（9 UTF-8 字节）。没有经 Clipboard 注入这些文字。
- 左侧多行选中初始 Edi，输入相同文字后 Enter。从右侧共享视图全选复制，
  与独立构造的 `"azéê€\n" + initial[3:]` 逐字节相等，共 1,048,665 字节。
  右侧仍保持自己的视口锚点，不因左侧在前面插入行而强制跟随左侧光标。
- 不重启窗口，独立桌面改为 us / intl；第二个单行全选后键入 q/w、死键
  单引号后 e，复制独立断言精确为 qwé（4 字节），截图确认显示一致。
- 本轮补窗口用例入口的操作、预期及隔离环境要求，无生产代码改动。
  截图位于 `/tmp/goui-keyboard-layout.OMbaEb/`；窗口正常退出后清理专用会话。
  不据此承诺所有键盘布局、真实中文 IME 或 Windows/macOS 输入已验收。
- 远端仅作只读复查：Windows SSH SessionId=0、用户 RDP SessionId=1；macOS
  控制台用户仍为 root，Apple 的 gui/501 Aqua 域存在但不等于桌面可操作。
  本次未做远端截图或输入，未更改登录状态、计划任务或权限。
- `go test ./...`、`git diff --check` 通过；多数包使用缓存，既有 PNG 输出测试
  保留原有副作用。未修改生产代码，本轮未重复 race 或跨平台编译。

### 2026-09-20 开发交付与验收分工调整

- 按用户确认，取消远端交互桌面作为开发完成门槛。A～E 的代码、包内测试、
  本机窗口验证已完成；Windows/macOS 窗口用例交用户执行，未执行项仍如实保留。
  不新增桌面代理、计划任务或系统权限配置，也不以其他平台窗口等待阻塞路线。
- GUI 用例新增 `-accept-tab`，仅左侧启用；两个入口补全 PowerShell/POSIX 命令、
  适用后端、1x/2x、macOS 快捷键差异及环境恢复说明。只修改用例与文档，未改
  生产编辑策略。首版范围、分层和 Cluster 契约不变。
- 本机 Ubuntu 22.04.5 / 独立 Xtigervnc :91 / Xfwm4、Software 1x、本地 XIM、
  1 MiB：在 Edi 后按 Tab，复制前四个 Cluster 精确为 `Edi\t`，模型增加一字节，
  右侧同步；撤销后在右侧默认模式按 Tab，全文复制精确恢复初始 1,048,655 字节；
  左侧切只读再按 Tab，全文保持相同。截图位于 `/tmp/goui-text-handoff.ILAy7Z/`。
  窗口正常退出，专用 WM/Xserver 已停止，用户桌面的配置与剪贴板未改。
- 从当前源码交叉构建 GUI、原生 typography、原生窗口包的测试二进制，传输后
  SHA256 与本地一致。Windows 10 22H2 amd64 / DirectWrite、macOS 15.7.7 arm64 /
  CoreText 均实际离屏执行以下选择并通过，无窗口、无真实 Clipboard/IME 操作：
  - GUI：`-test.run='^Test(Text|Scroll|IMContext|Window.*IME)' -test.count=1`。
  - typography：`-test.run='^TestText' -test.count=1`（含 Cluster、RTL、分隔符、
    预乘 alpha 与布局资源隔离；不运行旧的写图 `Test_TextLayout`）。
  - win32/cocoa：`-test.run='^TestInputMethod' -test.count=1`（factory；Windows
    另含字符组装、组合来源与生命周期）。
  Windows 首次命令因未给 PowerShell 的 `-test.count=1` 加引号而被拆成 `-test`，
  exit 2，未进入测试；修正参数引号后以上三组均 PASS，不是生产测试失败。
- `go test -race ./gui ./ui ./platform/linux/x11 ./platform/typography/pango
  -run 'Test(Text|Scroll|IMContext|Window.*IME|InputMethod|Preedit|ComposedLookup)'
  -count=1` 通过；两处窗口入口的 linux/amd64、windows/amd64、darwin/arm64
  编译通过。`go test ./...` 通过（多数包使用缓存，既有写图副作用保留）。
- 已知限制不因开发交付而消失：libX11 旧版本 callback spot 限制、同步调用内
  服务退出时的原生等待、超长单段落非优化目标，仍见 DesignIME 与本文范围说明。
  其他平台窗口真实 IME/Clipboard、候选位置、输入布局及后端/HiDPI 场景由用户
  按入口编号验收，不将本次离屏 PASS 或目标编译通过解释为这些窗口项已通过。

## 批量编辑公开契约补充（2026-10-16）

textedit.Model.ApplyBatch(edits, BatchOptions) 提供可复用的原子批量编辑：
输入是原始文档坐标中已排序、非重叠且起点无歧义的 Edit；所有范围、UTF-8 边界、
规范化结果和前后选区在变更前验证。同起点插入需由调用方先合并。
空操作不增加 revision；有变更时从右到左执行，只增加一次 revision、Change 和 undo 项。
Change.Edits 按实际执行顺序报告，订阅方继续逐项映射，不另引一种通知坐标体系。

BatchOptions 包含 Source、BeforeSelection、AfterSelection；历史保存独立选区值，
UndoWithSelection / RedoWithSelection 一次恢复整个批次并返回对应选区和操作 revision。
批次不并入相邻输入组。操作完成回调后的模型 revision 可能已经被重入推进，使用方应核对 HistoryResult.Revision。

原子性、无效输入不变性、规范化 UTF-8、前后选区、通知顺序、undo/redo 和重入回归已通过。
该契约属于通用 textedit，不包含语言、高亮或具体代码编辑器依赖。
