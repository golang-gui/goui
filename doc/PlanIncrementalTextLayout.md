# 长段落排版实施记录

## 状态

2026-10-20。用户选择保持完整正确性，继续完善通用方案。设计见
[增量排版](DesignIncrementalTextLayout.md)。

- [x] 分开测量原生排版与度量导出，验证独立文本窗口原型的 bidi 缺陷。
- [x] 共享几何按行索引、可视行选区查询；TextView／TextInput 和 SourceView 接入。
- [x] 确定性回归与排版阶段 benchmark。
- [x] 已排版光标导航复用几何，避免点击触发父层测量与宽度来回重排。
- [ ] 可恢复、保留上下文的段落分析与 shaping 原语。
- [ ] Root 预算内推进、测量复用、增量折行及待就绪交互契约。
- [ ] 两个控件首次超长段落不冻结窗口的桌面验收。

**本轮尚未完成首次排版修复。** 两个控件仍可能在原生整段测量时阻塞。
有 bidi 缺陷的原型未接入；没有新增后台测量或整段延后一帧的路径。

本轮按功能提交：

- `21444f9`：Geometry 查询索引与显示行选区接口。
- `25e52c1`：TextView／TextInput 的可视行选择和预编辑绘制。
- `33113b9`：Pango 长段落语义回归与分阶段诊断。

## 原型

本机 Linux amd64、Go 1.25.8、Pango 1.50.6、Cairo 1.16.0；DejaVu Sans 12 pt，
WrapWordChar，宽 520 DIP，Tab 40 DIP，无窗口。GOUI 基线 `554dbdb`。

临时原型从上次软折点截取 512／2048 byte，保留除末行外的显示行。普通代码
行级几何有改善，但跨窗口 RLO／混合 bidi 出现位置／宽度不一致。原型失败，
也尚未比较完整 glyph／像素，不能以简单样本通过证明普遍正确。

首次阶段拆分单次诊断（不是重复统计的性能结论）：

| 样本 | 字节数 | 创建 | 首次 MeasureSize | 随后 MeasureMetrics |
| --- | ---: | ---: | ---: | ---: |
| 连续 a | 8,192 | 0.24 ms | 124 ms | 24 ms |
| 连续 a | 32,768 | 0.21 ms | 1,692 ms | 91 ms |
| 重复 JSON | 37,000 | 0.28 ms | 26 ms | 95 ms |
| 混合 bidi | 42,000 | 0.29 ms | 42 ms | 80 ms |

复测入口：

```bash
go test ./platform/typography/pango -run '^TestWrappedParagraphKeepsOverrideAcrossLongPrefix$' -count=1
go test ./platform/typography/pango -run '^$' -bench BenchmarkParagraphPhases -benchmem -benchtime=1x -count=3
```

benchmark 区分 Layout、Metrics、Reflow；上下文／样本创建不计时，字体缓存热。
max-call-ms 是同步调用耗时，不是输入延迟；不依赖桌面。跨 4 KiB 前缀 RLO 测试
使用独立 RTL 预期，供后续分块实现保留段落语义。

## 几何查询对照

相同环境，每显示行 80 cluster，100／10,000 显示行。查询已取得的度量，不计
原生排版、Geometry 构建与光栅化。每项 100 ms、三次，以下为中位数：

| 10,000 显示行 | 修改前 | 修改后 |
| --- | ---: | ---: |
| Caret | 970,581 ns | 219 ns |
| Hit | 7,038 ns | 128 ns |
| 单显示行短 Selection | 4,109,663 ns | 1,306 ns |

```bash
go test ./gui/textedit -run '^$' -bench BenchmarkGeometryWrappedQueries -benchmem -benchtime=100ms -count=3
```

两次均为 80 万 cluster，查询位于段落中部。Caret／Hit 仍为零分配；
Selection 的输出分配未改变。此结果说明消除了查询随整段规模增长的扫描，
不表示首次排版或全部编辑操作达到相同耗时。

新增回归有独立 bidi／软折点预期、非单调输入的原语义、确定性随机与原穷举
查询对照、可视行完整矩形，以及两个控件的长段落全选绘制范围。
既有 SourceView occurrence 测试补充实际 viewport，原断言保留。

## 验证

环境为上述 Linux／Pango，GOUI 在无 go.work 条件下运行。通过：

- `go test ./...`。
- `go test -race ./gui/textedit ./gui`。
- 新增 RLO 回归、三次 ParagraphPhases 诊断和三次几何查询对照。
- 相邻 SourceView 的 `go test ./...`、`go test -race ./...`、`go vet ./...`。
- 从 SourceView 执行 `GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build ./...`
  及 `GOOS=darwin GOARCH=arm64 CGO_ENABLED=0 go build ./...`，覆盖本轮共享 API 使用方。
- 现有 SourceView `tests/window/editor/selection`：Linux X11／XFCE、Software 1x、
  Pango，Basic／Modern Light／Modern Dark 下 JSON／XML／HTML 九组均 PASS。
  该程序通过正常 DispatchEvent 检查选区、焦点、菜单和像素；不是长段落延迟测试。

失败与限制：

- `go vet ./gui/textedit ./gui` 失败，139 条 unkeyed composite literal 诊断。
  用 HEAD 的 text_editor.go 覆盖及排除新增使用方测试后复测，规范化路径和行号后
  诊断完全一致，没有新增诊断；未顺带修改这些历史写法。
- SourceView 的 `GOWORK=off go test ./...` 要求更新现有 go.mod；保持用户文件不改。
  用仓库外 modfile 移除 replace 后，v0.2.0 在新增 LineAtY／SelectionLines 处
  编译失败，当前改动尚不具备已发布依赖的独立构建条件。
- Windows／macOS 仅交叉构建，原生排版／窗口未运行；真实 IME、物理键鼠、
  混合 DPI 和完整增量排版响应未验收。

窗口验证已自动关闭，无系统设置或剪贴板写入。全仓既有测试重生成了其原有图片
输出，未纳入交付；没有迁移、删除或放宽旧测试。新回归没有 skip 分支。

## 光标定位回归修复（2026-10-20）

基线 `b022304`。TextView 的 setSelection 原先无条件 RequestLayout；ScrollView
内容测量使用完整宽度，LayoutVisible 使用扣除滚动条后的宽度，同一段落缓存
因此在每次点击后两次失效。32 KiB 连续 ASCII 样本上，原生布局宽度反复
`733 → 741 → 733`。这是 `cb385b8` 内容尺寸测量路径与原先导航失效策略的
组合；在几何索引优化前的 `554dbdb` 上也能复现。

修复在有效段落上直接计算光标矩形并按需请求滚动，不改变文档尺寸缓存；
目标未测量、宽度不符或尚无视口时保留布局请求。TextInput 共用该逻辑并保留
横向滚动。原有的文本／Padding／IME 布局失效不被消费；滚动通知重入转交后续
布局，回调替换模型、修改选择、卸载或销毁后不再通知过期选择。
设计见 [内容测量与失效](DesignTextViewSizing.md#排版与失效)。

新增包内回归覆盖点击、Shift 点击、拖选、方向键、文首／文尾、TextInput、
未测量目标、Padding／IME／模型失效及回调重入。正常输入均走 DispatchEvent。
以基线 text_editor.go 作 Go overlay 时，
`TestTextViewCaretMovesReuseWrappedLayout` 在点击等子项明确失败。

新增窗口程序：

```bash
XMODIFIERS=@im=none GOUI_PLAT_PAINTER=software GOUI_PLAT_SCALE=1 go run ./tests/window/gui/text_caret
XMODIFIERS=@im=none GOUI_PLAT_PAINTER=software GOUI_PLAT_SCALE=2 go run ./tests/window/gui/text_caret
```

本机 Linux amd64／X11／XFCE（xfwm4）／Pango 1.50.6／Software，Modern，
760×360 DIP 集成窗口，32,768 字节单段。两种缩放均通过五项操作与原生调用计数
断言；不是物理键鼠或 IME 验收。窗口自动关闭，无剪贴板、设置或用户文件写入。

同环境诊断的三次点击，基线每次两次完整排版，约 2.8 秒；修复后三次均无
NewTextLayout／SetSize／MeasureMetrics。正式窗口回归使用基线 overlay 时，
第一次点击在 2.868 秒后失败，resize 计数由 1 增为 3、metrics 由 2 增为 4；
修复后全程保持初始计数（created=1、resized=1、metrics=2）。

修复后的窗口采样为“事件派发开始 → 第一帧完成后的定时器回调”，定时器间隔
20 ms，不是显示合成延迟或性能上限：1× 三次点击约 20 ms，文尾跳转约 52 ms；
2× 点击约 32～49 ms，文尾跳转约 127 ms。跳转绘制仍有成本，首次整段排版
也仍同步；不能将此次交互缓存复用写成原生增量排版完成。

用户提供的 157,862 字节单行 JSON 只读诊断：TextView 三次点击事件处理约
0.59～0.95 ms，随后没有新增完整度量；SourceView 启用 JSON 高亮后同样未在
三次点击中重建布局。此临时诊断只记录调用与耗时，不替代语义／像素回归。
另观察到 SourceView 初次滚动期间一次约 293 ms 的 MeasureSize，发生在着色
更新后；应继续检查颜色失效是否引起原生重新排版，尚未作为本次修复的结论。

本次验证：

- GOUI `go test ./...`、`go test -race ./gui ./gui/textedit`；
  补充未测量目标用例后再次全量测试和目标 race。
- SourceView `go test ./...`、`go test -race ./...`。
- SourceView `tests/window/editor/selection`：上述 Linux／Software 1× 下，
  JSON／XML／HTML × Basic／Modern Light／Modern Dark 九组事件与像素检查通过。
- GOUI `./gui` 与新窗口程序、SourceView `./...` 的 Windows amd64／macOS arm64
  `CGO_ENABLED=0 go build`；仅交叉构建，未运行两端原生窗口。
- SourceView 关闭工作区测试仍报 `updates to go.mod needed`，保留用户依赖配置。

原生首次排版、颜色失效、2× 绘制延迟以及实际 IME／物理输入仍须继续推进；
本节不改变上面的通用分时排版未完成状态。

## 已测段落分帧换宽（2026-10-20）

实现规格见 [可视区缓存与分步换宽](DesignViewportTextLayout.md)。基线 GOUI
`71b25c8`，SourceView `e0b1a6c`。原生绑定 `cf734bc`、RTL 修正 `6b110f0`、
Pango 重排 `6676636`、GUI/TextView 调度 `57163fd`；SourceView 接入 `cdf100c`。这一步优化已存在段落的换宽，未完成全文通用
增量排版：首次加载仍同步；第一次保留完整分析、巨大单行／cluster、Geometry
构造和实际绘制仍有不可抢占工作，新宽度等整段折行索引完成才统一发布。

GOUI 增加可选 `typography.ReflowLayout` 与 `gui.DeferTextLayout`。Pango 保留
完整 Unicode/logical attrs、font/item、glyph advance，使用原段落上下文对折点
局部塑形，不独立分析子串。换宽取消旧 continuation，单次推进按显示行检查
软期限；同 Root 队列约 2 ms、按约 250 μs 小批次轮转。TextView 已测 8 KiB
以上段落接入；TextInput 不换行，保留原路径。隐藏暂停，卸载／原生销毁／缓存
淘汰取消；编辑移动未修改段落时使用当前索引；换宽取消屏幕外的过期任务。
SourceView 独立缓存和语法状态，经公共 helper 参与同一预算。

原生 glyph 页只保留光栅请求区域与上下各一屏范围的显示行，滚回从测量重建；
折行/cluster 元数据仍完整保留。颜色批量更新只登记绘制失效，首次光栅前复制
一次属性，不在每个 token setter 中复制整份属性，也不重新折行。测量格式变化
清除快照；不同布局独立持有 item/font/glyph，不能共享可变页。

两个正确性修复随本轮交付：

- Pango RTL 迭代器的 index 是 cluster 最后一个字符；改从 glyph log 取得
  逻辑起点，矩形继续取原生迭代器。RLO+组合字独立语义回归在旧行为 overlay
  下失败（得到 `[0,4)`，应为 `[3,6)` RTL），修正后通过。
- Pango 1.50.6 的 show_glyph_string 共享 renderer 清除颜色但残留 alpha。
  保留字形改由公开 Cairo scaled-font/glyph 绘制；缺字框使用 Pango 原生轮廓。
  隔离回归显式先画半透明普通布局，再销毁它、推进并绘制另一布局，1×/2×
  严格比较参考像素，不放宽容差。重复五次及组合运行通过。

本机仍为 Linux amd64／Go 1.25.8／Pango 1.50.6／Cairo 1.16.0／X11／XFCE
(xfwm4)。原生头文件的 C 编译器核对：GlyphItem=32、GlyphInfo=20、LogAttr=4、
CairoGlyph=24、CairoGlyph.x offset=8；包内增加公开前缀布局断言。新路径需要
Pango 1.50+；DirectWrite/CoreText 尚无对应实现，不能将交叉构建记为三端优化完成。

### 对照与窗口范围

相同 Linux/Pango/DejaVu Sans 12 pt，32,768 个 a，520 → 480 DIP，三次
`BenchmarkDeferredResize -benchtime=1x`。两条路径都包含完整 line/cluster 导出，
不含初始分析、绘制和 GUI Geometry：

| 路径 | 累计执行耗时 | 最长单次调用 |
| --- | --- | --- |
| 原生 SetSize + 完整度量 | 1.848～1.881 s | 1.845～1.877 s |
| 保留测量、分步折行 | 78.3～79.1 ms | 2.56～2.76 ms |

分步 benchmark 没有帧等待；不等于新文本呈现延迟，也不证明冷分析或任意输入
的上限。期限不是原生调用的抢占机制。固定矩阵核对 WrapChar/WrapWordChar ×
1/17/80/480/520/700 DIP 的行范围、X/Y/宽高/baseline、glyph ID/位置/level、
cluster 和 1×/2× 预乘 RGBA 像素；样本覆盖连字、字体范围、CJK、emoji/组合字、
Tab、SHY/ZWSP/NBSP、RTL/isolate、跨 4 KiB 的 override、空段及段落分隔符。
混合段落的总宽度按行矩形并集计算，不能仅取最大行宽。

两仓窗口联验入口在 SourceView `tests/window/editor/reflow`，分别运行 TextView
和 SourceView 模式。真实 X11 窗口由 xdotool 连续换宽，按钮/文本输入通过正常
DispatchEvent。Modern 集成窗口、Software 1×/2×、OpenGL 1× 验证：
待排版期间按钮可点击、无同步 SetSize、过期宽度取消、最终完整原生几何一致、
有可见墨迹、点击定位正确、编辑取消资源。测试关闭行号/折叠来显式固定文本原点。

32 KiB 样本在待排版期间观察到几十次 20 ms 定时器回调；1× Software 的最长
定时器间隔约 60～70 ms，2× 约 130～150 ms，OpenGL 1× 约 30～40 ms。
这是进程内观察，包含绘制、布局及调度影响，不是合成延迟或硬上限。高 DPI
绘制仍需优化。用户 `1.json`（157,862 字节，单段）也通过两控件 Software 1×
换宽/几何/点击/编辑验证，未写入文件；具体采样保留在 SourceView 实施记录。

### 验证与缺口

- 两仓全量 `go test ./...`，GOUI 另在 GOWORK=off 条件通过。
- GOUI `go test -race ./gui ./gui/textedit ./platform/typography/pango`、
  SourceView `go test -race ./...` 通过。
- GOUI `./gui` 与 SourceView `./...` 的 Windows amd64／macOS arm64
  `CGO_ENABLED=0 go build` 通过；两端原生排版与窗口均未运行。
- SourceView `GOWORK=off go test ./...` 仍报 `updates to go.mod needed`，
  未改用户现有 go.mod/replace；已发布 v0.2.0 不含所需新 API，不能独立发布。
- 单元回归还覆盖过期 deadline 不做分析、换宽保留测量、取消不发布、字体失效、
  颜色不重测、页淘汰再访问、两布局交错/销毁、完成回调重入、隐藏恢复、编辑
  行号移动与缓存淘汰。窗口测试不等于真实 IME／物理键鼠验收。
- 原有全仓测试仍会重生成其历史图片输出，未纳入提交。新增窗口不写剪贴板、
  系统设置或用户文件，测试窗口均关闭；一次初始化诊断用 SIGQUIT 终止测试
  进程以定位重复任务，未作为场景通过依据。
- 新首屏提前发布、详细几何分页、检查点/宽度后缀收敛、可恢复冷分析、巨大单行
  continuation，以及 DirectWrite/CoreText 分帧换宽仍未实现。
