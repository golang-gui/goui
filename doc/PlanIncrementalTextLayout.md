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

### Pango 度量导出减少重复原生查询

`platform/typography/pango` 按已确定的字体/script run 遍历 cluster，避免每个
cluster 都重新查询所属行和 run。保留 GetClusterExtents 原生坐标，不自行累加
字形 advance 推导 RTL 坐标。绑定的 GetExtents/GetLineExtents/GetClusterExtents
对应原生可空输出，普通逻辑度量不再要求未使用的 ink extents。抽象 TextLayout
契约、排版文本和线程不变，没有 Reflow、后台测量或 GUI 策略进入后端。

依据为 [Pango 1.50.6 pango-layout.c](https://github.com/GNOME/pango/blob/1.50.6/pango/pango-layout.c)
的 next_cluster/get_cluster_extents 与 optional ink 参数；官方文档亦说明
[cluster extents](https://docs.gtk.org/Pango/method.LayoutIter.get_cluster_extents.html)
和 [layout extents](https://docs.gtk.org/Pango/method.Layout.get_extents.html) 两种输出
均可 NULL。本机 Pango 1.50.6 实测，官方网页当前显示 1.58.2，未把网页版本当成本机。

保留原逐 cluster 查询为测试内原生参考，对照全部行/cluster 的确切值，覆盖空行、
Tab、换行、字体/颜色 run、CJK、组合字、emoji/ZWJ、Arabic/Hebrew、RLO/isolate，
另有独立 UTF-8 边界预期。原生 pango 包全套通过。已预排的 139264 byte 换行样本，
三次 BenchmarkMetricsRunQueries 的中位数：108.044 ms → 60.911 ms，
每次分配约 17.94 MB/829149 次 → 13.60 MB/424935 次。这仅是导出度量阶段，
不是一次编辑或完整排版的延迟；初始 wrap/shaping 仍须单独计量。

用户单行 JSON 为 157862 byte，校验 SHA256
`7baa22a894b415fdc3425c94f8a541d0ded8f77965541c46af3c95a3364c798b`。
使用未改动输入的内存编辑、实际滚动条远跳和点击至约 77601 byte 处，各三轮
关闭 profile 的真实输入（与上述环境一致）：中部输入中位数 / 最大值
994/1030 → 562/563 ms，Backspace/Delete 1007/1060 → 559/581 ms，
选词替换 993/1018 → 549/559 ms。六轮均原文逐字节恢复、脚本/窗口退出 0；
仅运行 middle-edit 范围，不包含缩放。原始记录为仓库外 user-json-middle-text-*。
此前一轮开启 profile 的完整场景被桌面焦点变化中断，没有完成退出核验，不计通过。

上述减少了重复工作，**仍未解决新文本的单次整段测量**：同文件一次 native size
约 290 ms，加上度量、编辑几何和绘制仍约 550 ms。不能把较旧基线的一秒降到
半秒称为流畅。下一阶段的送测范围限制仍在控件层推进，必须保留段落双向上下文，
不能以独立截断复杂文本、关闭自动换行或改用后台测量来替代。

### TextView 近期完整输入复用

继续针对同一个 157862 byte JSON，TextView 保留最多 4 个被局部编辑替换的已测
长段落，合计额外 1 MiB 源字节。只在完整文本、格式和宽度相同的情况下移回当前
缓存，不复用新文本的旧 Cluster。资源移出索引后才销毁，模型变更在销毁重入后
核验模型/修订/挂载；测试覆盖恢复、预算、唯一所有权、颜色失效、卸载与模型替换。
新增恢复测试在禁用该复用路径时失败，启用后通过。没有针对 ASCII/RTL 分类降级。

用户文件中部真实按键三轮，未启用 profile，全文恢复、脚本/窗口退出均为 0：
Backspace 中位数 / 最大值 60/77 ms，Delete 61/74 ms，撤销替换 60/73 ms，
重做替换 58/63 ms。初次输入新字符仍 548/592 ms、初次替换仍 558/561 ms。
与此前约一秒的基线相比，缓存命中的编辑显著改善，新输入仍不流畅；不以查询
复用掩盖“有限送测和段内增量排版”尚未实现。记录为仓库外
user-json-middle-text-history-*；针对性核验后又执行两仓完整测试与 GOUI race。

### 远端 macOS：CoreText 导出临时存储

2026-10-21，macOS 15.7.7、Apple M1 arm64、Go 1.25.8。在远端独立临时目录中
验证 GOUI `b50da4f` 与 SourceView `33ac118`，未覆盖远端已有仓库及未跟踪文件。
用户 157862 byte JSON 已核对 SHA256，与 Linux 的输入相同。CoreText 初始打开
的分析器记录 native size 约 30 ms、metrics 约 18 ms；开启了分析器，不能作为
真实点击、输入或滚动的端到端延迟。

CoreText 的 MeasureMetrics 改为在一次调用中复用 positions/advances/indices 数组
和逐行清空的索引表；保留完整原生上下文和输出值。新回归与此前逐 run 分配的
原生参考比较全部行/cluster，涵盖宽窄换行、RTL/override/isolate、Tab、组合字、
emoji、颜色 run 和占位符；后续不同宽度测量不能更改已返回的普通坐标。

`go test ./platform/typography/coretext -count=1` 通过；
`go test -race ./gui ./gui/textedit ./platform/typography/coretext` 通过。
同进程预先完成 shaping，对 188416 byte 样本导出 metrics，
`go test ./platform/typography/coretext -run '^$' -bench '^BenchmarkMetricsScratch$' -benchtime=10x -count=5`：

| 导出实现 | 五组中位数 | 每次分配字节 | 每次分配次数 |
| --- | ---: | ---: | ---: |
| 逐 run 分配 | 14.064 ms | 约 29837023 | 约 54356 |
| 调用内复用 | 8.399 ms | 约 9916916 | 36250 |

这只覆盖原生度量导出阶段，不改变 TextLayout 契约，不是通用分时排版完成。
远端 GOUI `go test ./...` 存在基线失败：`platform/test` 的
`TestWindowInteractionCapabilities` 在没有输入上下文时，resize 返回
`ErrUnavailable`，测试期待 `ErrUnsupported`。失败发生在这项
CoreText 修改之前，未删除断言、扩大 skip 或顺带修改窗口能力契约。

桌面验证尚未完成：mac 的 SSH 可用但配置的 VNC 返回 `CONNECT_FAILED`，当前进程
未获桌面事件发送授权；win 的 RDP 可用，但 SSH 端口返回 `Connection refused`。
未更改权限、远程服务或连接安全设置。不能把窗口创建、CoreText 回归或目标编译
算作实际远端编辑、滚动及拖动宽高验收，须恢复连接后继续。

### 远端连接恢复后的修正与实测

同日 SSH、VNC、RDP 恢复后，继续在两端仓库外隔离快照验证，未覆盖已有工作区。
macOS 15.7.7 / M1 / CoreText / VNC 1650×1050；Windows 10 19045 / Xeon Gold
5318Y / DirectWrite / RDP 1600×900；Go 1.25.8、Software、1x。窗口为 Modern、
WindowChromeIntegrated、1100×760，正常平台输入，无 profile 的轮次用于对照。

按功能分三项修正，具体契约与回归见 DesignWindowManagement、DesignTypography：

- Cocoa 在 drawRect 内请求下一帧会丢失 SetNeedsDisplay；改用主 run loop
  common modes 的合并请求并复查窗口有效性。独立原生重绘回归旧实现仅 1 帧
  后超时，新实现准确 4 帧、销毁后无回调；Linux 同一窗口回归通过。
- DirectWrite SDK 的 HitTestTextPosition 保留原生坐标与 HRESULT，以一次
  pinned 输出工作区精简调用封送；原生精确对照和 inline callback 栈增长通过。
- DirectWrite 自有光栅目标只在颜色变化或目标失效时重新应用范围效果，避免
  相邻 Rasterize 区域重复设置同样效果。不保留外部 target，目标交替、重建、
  1x/2x 重着色及逐像素原生参考回归通过。

同一个 157862 byte 用户单行 JSON，通过真实滚动条远跳及正文点击到中部，执行
插入、退格、短词输入、选词替换、撤销重做与 Delete，最后正常关闭全文恢复并
取得进程退出 0。以下是同机 GUI capture 到末尾 Widget.Paint 的实测范围，
排除此前平台排队、native Present、桌面合成和网络；不称为完整用户感知延迟。

| 对照 | 操作 | 修改前 ms | 修改后 ms |
| --- | --- | ---: | ---: |
| Windows TextView，SDK 旧 2 / 新 3 轮 | 首次 x | 337.6–338.0 | 254.1–272.4 |
| 同上 | 首次替换 | 317.8–331.2 | 245.6–257.3 |
| Windows SourceView，颜色复用各 2 轮 | 首次 x | 613.8–629.7 | 437.9–439.2 |
| 同上 | Backspace | 337.7–343.2 | 161.3–164.4 |
| 同上 | Undo 替换 | 327.5–331.7 | 154.9–156.2 |

Mac TextView 旧 `4018c7a` 与本轮新实现各 1 轮，两边共用 Cocoa 修正以正常呈现。
GUI capture 到 Paint 的 Backspace 184.7 → 23.6 ms、Delete 145.8 → 36.9 ms；
打印字符可能走 IME 而无 KeyDown，只另报模型 change 到 Paint：首次 x
171.6 → 148.4 ms。该区间不包含控件先前同步 change 处理，不能混入上表口径。
单轮不据此推断稳定百分比。VNC 桌面观察仍可达数秒，原始远程延迟另行保留。

两端 TextView 的 3.3 MB TXT，以及 SourceView 的用户 JSON、54 MB 多行 JSON、
2.8 MB XML、4.0 MB HTML、0.9 MB Markdown，均完成一轮中部真实输入、完整恢复
和正常退出。XML/HTML 使用 End/Cmd+Right 实际定位行尾后测普通插入，保留标签
联动；最初点击标签名导致双处合法编辑、脚本单处字节预期不成立的轮次记失败，
撤销并全文核验后重跑。没有以编辑 API 或仅加载窗口替代操作。

Windows TextView、SourceView 单行 JSON，以及颜色复用后的 54 MB SourceView
JSON 六次真实改宽/高/角部拖动均观察到至少两次中间尺寸，最终尺寸与全文断言
通过。Mac 六次最终尺寸、文本、光标及关闭校验通过，但 VNC 未采到中间尺寸，
六项响应断言仍失败；GUI 日志记录 11 次尺寸变化、缩放区间 Timer 投递间隔最大
约 180 ms，不能替代未通过的桌面观测。Windows TextView 同口径间隔仍约
347 ms，单行 SourceView 颜色优化后独立缩放轮仍约 367 ms。

本机两仓 `go test ./...`、GOUI `GOWORK=off go test ./...` 通过；Windows SDK /
DirectWrite 及 Mac Cocoa / CoreText 相关普通、race 回归通过。Mac 全仓仍有
既有 TestWindowInteractionCapabilities 及 SourceView 两种分段 probe 失败。
Windows GOUI 服务会话全仓仍有 D2D 预乘渐变、popup/window/paint 超时和
TestWindowStyle 失败；SourceView 的 longRLI 分段 probe 在旧、新 SDK 均同样失败。
服务会话测试不能替代交互桌面验收，保留失败断言。SourceView 独立依赖验证仍
报 go.mod 需要更新，未改用户的本地 replace。详细样本、轮次与失败记录在
SourceView 的 doc/PlanRemoteEditingLatency.md。

**新内容和新宽度的超长单段完整原生同步排版仍未解决**。上述减少重复工作并
修复绘制请求丢失，没有 Reflow、后台测量或上层调度进入 typography，也不以
近期完整输入复用或光栅化基准宣称已经实现通用分时排版。


## 2026-10-22：批量度量、原生区域绘制与独立折点索引

本轮代码、普通边界精度修复、三个后端验证、真实操作对照及未通过的缩放场景见
[测量与段落渲染验证](PlanMeasuredTextLayout.md)。这是减负及前置验证，正式上下文
片段／事件间隔内布局仍未完成；保持本文件顶部未完成项，不将本批提交标为整体达标。

## 超长单行有界原生测量（2026-10-24）

- 样本 `/data/xuyang8/temp/test.json` 为 184,500,131 bytes、无 LF/CR 的单逻辑行；
  大部分为 ASCII，包含少量中文、组合字符和数 MB 的 JSON 字符串。不复制样本进仓库。
- GOUI 基线 `5f30e52`，Linux amd64 / Go 1.25.8 / Pango / Software，
  独立 Xvnc `:89`、Xfwm4 合成、100% 应用缩放，Integrated 1000×700 窗口。
  通过相同 TextView 驱动对照：基线原生输入 184,500,131 bytes；真实按钮点击
  未得到处理，超过 5 秒仍未首屏就绪。看门狗向本次基线进程发 SIGQUIT 取栈，
  主线程停在 Pango GetExtents；这是人工终止的诊断，不记录为正常退出或通过。
- 新版 TextView 在相同样本和环境中完成首屏、中部／末尾定位、连续键入 LATENCY、
  退格、撤销、重做、末尾输入、滚轮、滚动条拖动和改变宽高。输入经 XTest 正常
  事件路径；模型 byte 数和 caret 均按实际键入变化，截图观察文字及滚动位置。
  原生输入最大 4,149 bytes；中部请求处理 4.16 ms，目标原生窗口实际绘制就绪
  1.07 秒；末尾 4.13 ms／0.57 秒。这是本次观察值，不是延迟上限或性能 SLA。
- 同步加载 423.5 ms；加载后 HEARTBEAT 的最大观察间隔 162.5 ms。
  不能将原生输入变小等同于已消除整个 GUI 的停顿。未定位剩余间隔的全部来源。
- 原生对照覆盖 ASCII、中文、e+combining、emoji/ZWJ、tab、RTL 基方向、跨窗口
  RLO 与 isolate。与完整小规模原生参考比较行起止、宽高和 cluster 的 X、宽度、
  方向。接缝连续 20 次编辑、Destroy 重入不发布过期对象也有回归。
  测试中的原生送测 guard 为 8192 bytes，未通过放宽原有断言来取得结果。
- 值层覆盖 2 MiB 单行 IME Projection 的范围读取、长词的 UAX #29 对照及 UTF-8
  边界。真实 IME 候选窗口／平台提交未运行，不把 Projection 测试冒充真实 IME。
- 冻结已跟踪源码和本批新增文件到仓库外副本，避免纳入用户既有未跟踪测试／示例。
  副本中 `GOWORK=off go test ./... -count=1` 通过；工作仓库
  `go test ./gui ./gui/textedit`、`go test -race ./gui ./gui/textedit -count=1`、
  `GOWORK=off go vet ./gui ./gui/textedit` 通过。
- Windows amd64、macOS arm64：`CGO_ENABLED=0 GOWORK=off go test ./gui ./gui/textedit
  -exec=true` 交叉编译通过；新增对应 DirectWrite／CoreText 原生入口，未原生运行。
  不能据此声称跨平台排版或 UI 性能通过。
- 保留限制：未确认全局滚动范围仍为估计；任意复杂 UAX #9、单个超大 grapheme、
  非均匀 NoWrap 的完整坐标等价性未穷尽验证；初始同步建模、超长 Unicode 词边界
  和剩余帧间隔仍需后续优化。不再整段送测解决的是当前样本的主要阻塞路径。
