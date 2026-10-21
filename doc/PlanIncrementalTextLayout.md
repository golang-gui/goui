# 长段落排版实施记录

## 状态

2026-10-20。保持完整正确性，本轮修订为上层片段测量缓存及可视区布局；
TextLayout 不承担视口或任务策略。设计见
[片段测量与布局](DesignIncrementalTextLayout.md)。历史实验按各节原有边界理解。

- [x] 分开测量原生排版与度量导出，验证独立文本窗口原型的 bidi 缺陷。
- [x] 共享几何按行索引、可视行选区查询；TextView／TextInput 和 SourceView 接入。
- [x] 确定性回归与排版阶段 benchmark。
- [x] 已排版光标导航复用几何，避免点击触发父层测量与宽度来回重排。
- [x] Git 回退 Reflow 接口、后端实现、GUI 队列及专用绑定，保留独立 RTL 修复。
- [x] 普通值片段快照与详细几何分页；两控件使用方接入。
- [x] 有限送测实验和长词／JSON 对照；保留跨窗口方向状态的失败门槛。
- [ ] 正式片段测量的上下文衔接、范围缓存及可视区布局。
- [ ] GUI 预算内推进、可视区发布、增量布局及待就绪交互契约。
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

## Reflow 回退与设计修订（2026-10-20）

用户要求保持 TextLayout 的排版／测量／渲染职责，缓存及布局由上层管理。
本轮执行 Git revert，未 reset 或改写历史：

- `50d202b`、`47a1e93`、`7c53b38`、`ba7804d` 分别撤回实验验收记录、
  TextView 帧接入、Pango Reflow 及该版设计。
- `dab0170` 部分撤回 `cf734bc` 专用原生绑定，只保留 GlyphString.LogClusters，
  供独立的 `6b110f0` RTL cluster 起点修复使用。
- 同期 SourceView `da26b9a`、`72e240d` 撤回其验收记录和控件接入。

回退后的 platform／gui 相对 `71b25c8` 仅保留 RTL 修复及其最小绑定、回归。
区域光栅化、几何查询、可视行装饰和光标缓存复用保持有效。
本轮只修订片段值／视口布局设计，尚未实现新的片段排版。

本机 Linux amd64，GOUI `dab0170`、SourceView `72e240d`，通过：

- GOUI `GOWORK=off go test ./...`，包含原生 Pango／RTL 测量回归。
- SourceView 本地工作区 `go test ./...`。
- GOUI `GOWORK=off go test -race ./gui ./gui/textedit ./platform/typography/pango`。
- SourceView 工作区 `GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build ./...`
  和 `GOOS=darwin GOARCH=arm64 CGO_ENABLED=0 go build ./...`，覆盖 GOUI 使用路径。

SourceView `GOWORK=off go test ./...` 仍报 `updates to go.mod needed`，未修改
用户原有 replace 或执行 tidy，不能声称已完成独立发布验证。
无新长段落窗口性能结论，Windows／macOS 仅交叉构建，未原生运行；首次加载
与换宽仍可能同步阻塞，片段设计待实现。既有全仓测试可能重生成原有图片输出，
未纳入提交；未清理用户原有未跟踪文件。

## 普通值片段与几何工作集（2026-10-21）

基线 GOUI `9c39d01`／SourceView `24edd50`。本轮未修改 platform／typography，
没有恢复 Reflow，没有后台测量、递归 pump 或新的 Root 任务协议。
功能提交：GOUI `42c9b42` 为片段值与详细几何分页，`22f3e4d` 为 TextView／TextInput
接入；SourceView `5f9a81d` 为控件与原生回归接入，`cf3c544` 为有限送测实验及记录。

已实现 NewFragment 的度量深拷贝、文本／坐标映射、完整显示行视图，以及每页
32 行、每个 Geometry 最多 4 页的详细几何。查询只使用普通值，不调用原生布局。
Native 异常／非分区度量保持已有 Geometry 语义，重复空行导致不严格有序的
停靠范围也保留旧平局规则。边界随机回归发现并修正过跨页最近停靠点差异，
没有放宽原断言。详见 [当前实现](DesignIncrementalTextLayout.md#12-普通值片段和详细几何分页已实现)。

TextView／TextInput 首次需要几何才创建快照，换宽重测后丢弃旧快照；普通显示
不提前复制完整度量。SourceView 的原有完整几何构造改为快照及按需详细页。
首次原生测量、度量复制、换宽及单条巨大显示行仍有同步成本。没有实现正式的
有限送测／事件循环测量预算，不能宣布超长文本 UI 冻结目标已达成。

### 几何阶段对照

Linux amd64／Go 1.25.8／Xeon Gold 5318Y。所有前后台项目验证完成后单独运行：

```bash
go test ./gui/textedit -run '^$' -bench '^BenchmarkFragmentFirstScreen' -benchtime=1x -count=3
```

相同现成度量，每行 80 cluster，测量“建立索引＋文档中部 20 行选择及一次命中”；
Fragment 路径包含度量复制，旧路径直接借用输入。三次中位数：

| 显示行数 | 原完整 Geometry | Fragment＋按需详细页 | 分配字节，旧 → 新 |
| --- | ---: | ---: | ---: |
| 1,000 | 13.197 ms | 3.806 ms | 31.4 MB → 7.0 MB |
| 10,000 | 126.635 ms | 30.179 ms | 312.6 MB → 41.1 MB |

这是人工度量的上层阶段对照，不包含 native shaping、绘制、事件循环等待或模型
加载。快照复制仍随输入规模增长，不能由此承诺首次显示／严格帧延迟。

### 验证与限制

Linux amd64／Pango 1.50.6／Cairo 1.16.0／X11／XFCE（xfwm4），通过：

- 最终 GOUI `GOWORK=off go test ./...`；另一次 `-count=1` 全仓通过。
- `go test -race ./gui ./gui/textedit`、`go vet ./gui/textedit`。
- SourceView 工作区全仓测试及 race；其原生测试覆盖 JSON、CJK／组合字／emoji、
  Arabic／Hebrew、RLO／isolate，销毁原生布局后比较全部行的命中、caret、删除
  边界及跨页选区，保留独立方向预期。
- SourceView Windows amd64／macOS arm64 无 CGO 的 `go build ./...`。
- `XMODIFIERS=@im=none GOUI_PLAT_PAINTER=software GOUI_PLAT_SCALE=1 go run ./tests/window/gui/text_caret`
  及 scale=2。32 KiB、Modern 集成窗口，三次点击与文首／尾导航通过；
  created=1、resized=1、metrics=2 均未增加。计时仅是事件至帧后定时器，
  不作为本轮性能对照或合成延迟结论。
- SourceView `tests/window/editor/selection`，上述 Software 1× 环境，JSON／XML／HTML
  × Basic／Modern Light／Modern Dark 九组事件和像素检查通过。

失败不计为通过：首次 GOUI 全仓在 `cmd/svgpng` 出现 `signal: segmentation fault
(core dumped)`；该包随后 `go test -v ./cmd/svgpng -count=3`、全仓 `-count=1`
及最终全仓均通过。尚未复现或确认根因，未把它归因为既有问题或宣称已修复。
SourceView `go vet ./...` 在未修改的用户 `example/temp/main.go:58` 报 unreachable
code；正式包、示例及窗口程序的定向 vet 通过，未修改用户例程。
其 `GOWORK=off go test ./...` 仍要求更新 go.mod；依赖尚未发布，未执行 tidy。

窗口自动关闭，无剪贴板、系统设置或用户文件修改。既有全仓测试可能重生成
原有图片输出，未提交；用户未跟踪文件保留。Windows／macOS 仅交叉构建，
没有原生窗口／IME／物理键鼠或混合 DPI 验收。

## 大 TextView 的自然尺寸与已分配几何（2026-10-21）

修复前 `measure` 在 LinearBox 的无约束高度查询中遍历所有逻辑段落；外层宽度
还包含 ScrollView 的滚动条，导致同一分配中的正文换宽、重复折行。修复将大文档
Measure 改为读取 HeightIndex，保留小文档精确 hug；LayoutVisible 负责实际宽度
测量并通知高度收敛。换宽沿用旧高度作估值，避免滚动条反馈改变第二次宽度。
没有新增公共接口或修改 typography，也没有启用独立截取的 shaping 原型。

三个新增回归在修复前明确失败：自然尺寸为 100001 行创建 100001 个布局、尺寸
查询使现有段落失效、只改高度重复导出度量。修复后均通过，另有大预编辑投影回归。
`go test ./...`、`GOWORK=off go test ./...` 和
`go test -race ./gui ./gui/textedit` 通过；既有全仓测试可能重生成未跟踪图片，
没有纳入提交。Windows amd64 / macOS arm64 的 gui 测试二进制交叉编译通过，
未在这两端原生运行。

真实桌面使用 SourceView 仓库的 `tests/window/editor/interaction`，其 `-kind text`
直接承载本仓 TextView，正常 XTest 按键、CLIPBOARD 及窗口边框拖动，不调用编辑
接口测延迟。Linux amd64 / Go 1.25.8 / Pango 1.50.6 / Cairo 1.16.0 / X11 / XFCE
(xfwm4) / Software 1×，Modern 集成窗口 1100×760，显式最小 640×360。
相同语料和观察口径，前后各三轮：

- 128 KiB 单段落，只改高度松手到就绪的中位数 / 最大值：579 / 1052 ms →
  62 / 84 ms；操作期间最大 GUI 事件间隔 550 → 41 ms。六次高度拖动均有中间帧。
- 同一段落的 64 KiB 粘贴：1136 / 1183 ms → 301 / 445 ms。字符输入仍约
  595 ms，改变宽度/右下角拖动仍没有足够中间帧，三轮脚本明确退出 1；窗口退出 0，
  字符、删除、选区、撤销/重做和最终原文字节核验均完成，不把缩放失败计为通过。
- 54,009,790 byte、1,428,166 行派生 JSON：旧代码在 120 秒内无法就绪；新代码
  三轮均完成 57 条记录和所有六项拖动，脚本/窗口退出 0、原文逐字节恢复。字符
  输入中位数 / 最大值 56 / 84 ms，删除 64 KiB 选区 63 / 69 ms。没有有效的旧版
  编辑操作基准，不能声称其编辑速度提升某个倍数。

计时从真实输入发送到 X11 像素观察；采样目标 5 ms，不是显示器扫描延迟。原始
记录位于仓库外 `goui-sourceview-corpus-20261021` 下的
`desktop-final-text-paragraph-128k.txt-*`、`optimized-text-paragraph-128k.txt-*`、
`optimized-text-github-api-4x.json-*`，前后均保持完整文本和默认字体。逐操作数据
及语料来源见 SourceView [编辑记录](../../goui-sourceview/doc/PlanEditingLatency.md)。
测试临时占用剪贴板，只能尝试恢复原 UTF-8 文本，不能恢复其他格式；改变焦点、
指针及窗口几何，测试窗口均已关闭，输入语料未修改。

尚未解决：单个长段落首次/编辑后/新宽度的不可抢占 native 测量，TextInput 的
横向长文本分片，及严格帧预算。这次不把自然尺寸优化写成通用分时排版已完成。

### 长单行局部编辑不重置高度估计

真实输入跟踪继续发现：131072 byte 单段落中输入一个字符，TextView 每次调用两次
整段 MeasureSize 和 MeasureMetrics。原因是 spliceParagraphs 将新段落高度重置成
一行，ScrollView 先去掉再恢复纵向滚动条，宽度随之变化。现在一段替换一段时沿用
旧高度作估值，文本布局仍立即失效；一次编辑只在实际正文宽度测量一轮。
回归验证旧实现失败、新实现通过，另检查清空全文后高度精确收敛至空段落。

Linux Software 1×、同一个桌面程序的诊断记录中，头部输入由两组
131/89 ms、123/88 ms（size/metrics）变为一组 134/92 ms；这一记录开启 CPU
profile，只说明调用次数与位置，性能对照另用关闭 profile 的真实输入。
本次曾有一次定向 gui 测试在 0.002 秒以 segmentation fault 退出，未进入断言；
随后同一命令、旧实现失败/新实现通过对照、全仓及 race 均完成。根因未确认，
没有把这次原生启动失败写成已修复或忽略。
