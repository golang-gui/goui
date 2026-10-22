# 已测 advance、原生查询与段落渲染验证

## 状态与范围

2026-10-22。设计见 [文本片段布局第 14 节](DesignIncrementalTextLayout.md#14-度量索引与换行分离2026-10-22)。

**本轮尚未完成“超长段落分帧、不阻塞 UI”的目标。** 已减少实际后端的度量导出和
区域渲染工作，修复普通光标边界的浮点问题，建立独立换行索引，并验证两个后端的
上下文范围塑形前置。TextView／SourceView 正式路径仍同步测量完整逻辑段落；
不能把算法基准或更多缩放中间帧解释成冷排版已被切成小任务。

GOUI 代码基线 `68cd392`，SourceView 基线 `86df709`。按功能提交：

| 提交 | 内容 |
| --- | --- |
| `c5734d6` | 同一字节位置的浮点舍入边缘合并，保留真实 affinity／bidi／软折行位置 |
| `516d2b6` | Pango 从原生 glyph run 批量导出 cluster 度量 |
| `d95978a` | DirectWrite 从原生范围框和 advance 推导 cluster 原点 |
| `f89ebea` | Pango 按请求 region 绘制原生行，复用行元数据并核对 native serial |
| `949e0df` | 普通值 WrapMeasurements／WrapIterator，尚未接入控件换行 |
| `5545262` | Pango／CoreText 保留完整段落的范围塑形验证及必要原生绑定 |

未修改公共 TextLayout 契约，未恢复 Reflow，未新增后台测量线程或嵌套事件循环。
SourceView 使用原有公开 TextLayout／Geometry 路径受益；其源码不导入 GOUI 私有设施。

## 环境、输入和测量边界

- 本机 Linux amd64，内核 3.10.0-1160.95.1.el7，X11／Xfwm4，Go 1.25.8，
  Pango 1.50.6、Cairo 1.16.0，窗口 software painter，缩放 1。
- Windows 10 Pro for Workstations 19045，amd64，Go 1.25.8，RDP 1600×900，
  DirectWrite／software painter，缩放 1。
- macOS 15.7.7，Apple M1 arm64，Go 1.25.8，VNC 1650×1050，
  CoreText／software painter，缩放 1。
- 用户只读样本 `1.json`：157,862 byte、一个逻辑段落，
  SHA-256 `7baa22a894b415fdc3425c94f8a541d0ded8f77965541c46af3c95a3364c798b`。
  不提交样本，不改变源文件；各窗口仅编辑内存模型，关闭时比较全文恢复一致。

两版分别构建同一份 SourceView interaction 窗口程序。真实输入使用 XTest 或
remote-computer-use 的 VNC／RDP 键鼠事件，不调用编辑方法代替用户操作。
Modern、WindowChromeIntegrated、自动换行；SourceView 保留行号／折叠，关闭自动补全。

GUI input→最后匹配状态的 Widget Paint，不含 native Present 或桌面传输。
IME 提交没有相应 KeyDown 时，记录 model change→Paint，不混进 input→Paint。
桌面 visible／release→visible 是外部截图观测，包含传输和采样；两类耗时分别保留。
SourceView 非 pending 标记不承诺所有稍后的语法着色已完成。10 ms Timer 的最大间隔
也不是 monitor latency。性能运行关闭 CPU profiling，构建／全包测试与最终本机操作分开。

原始记录、二进制和截图在本机 `/tmp/goui-wrap-cache-20261022/`；远端驱动记录在
`/tmp/goui-remote-long-text-20261021-resumed/`。临时资料不作为仓库运行依赖。

## 实际操作结果

Linux、Windows 各版本、各控件各两轮。下表是两轮范围，单位 ms，不是 p95。

| 平台／控件／操作 | 修改前 | 最终代码 |
| --- | ---: | ---: |
| Linux TextView：中部粘贴短词，input→Paint | 553–568 | 478–485 |
| Linux TextView：中部单字符，change→Paint | 520–529 | 437–444 |
| Linux SourceView：中部粘贴短词，input→Paint | 855–860 | 783–787 |
| Linux SourceView：中部单字符，change→Paint | 681–687 | 626–626 |
| Windows TextView：中部单字符，input→Paint | 261–270 | 207–210 |
| Windows TextView：选区替换，input→Paint | 247–250 | 192–193 |
| Windows TextView：连续输入 12 字符至最终 Paint | 2,027–2,034 | 1,334–1,366 |
| Windows SourceView：中部单字符，input→Paint | 436–447 | 375–377 |
| Windows SourceView：选区替换，input→Paint | 418–421 | 375–376 |
| Windows SourceView：连续输入 12 字符至最终 Paint | 2,262–2,273 | 1,325–1,354 |

中部点击、插入、Backspace、选择、替换、Undo／Redo、Left／Delete 及全文恢复均通过。
SourceView 的 Linux 候选版本曾出现着色重排落入后续按键的明显波动；最终两轮仍不能
据此承诺所有输入稳定低延迟。Windows Backspace／已缓存历史路径收益很小；没有把
它们的 20–30 ms 或 160 ms 级耗时宣称为本轮改进。

### 缩放响应与失败项

- Windows：最终 TextView／SourceView 各两轮，六项宽、高、角落缩放均达到至少两个
  中间 viewport 的原断言；宽／角落通常由 2 个提高到 3–4 个。最终尺寸、caret、文本正确。
  这仍是稀疏更新，不等于流畅拖动。部分 release→visible 变慢，例如 TextView 角落
  缩窄由约 445–452 ms 到约 568–569 ms；不能只报告中间帧增加。
- Linux TextView：六项最终尺寸及全文恢复正确，宽／角落四项响应断言仍失败，
  480 ms 拖动只观测到 1 个中间 viewport，原要求至少 2 个。窄宽收尾等待还会增加：
  width-narrow 约 536–538 ms → 783–788 ms。未解决，保留失败，未改验收断言。
- Linux SourceView：六项缩放断言通过，但原生完整段落仍造成长停顿；不能由通过
  “至少两个中间帧”推断已经满足无阻塞目标。
- macOS：最终 TextView／SourceView 各一轮，编辑、选择、历史、全文恢复及六项最终
  尺寸正确；六项中间帧断言均失败，VNC 期间未观测到中间 viewport。GUI 最大 Timer
  间隔分别约 373／250 ms；桌面观测约 2.3–6.2 秒，不能把传输时间当作 UI 阻塞。
  没有对未改变的 CoreText 排版路径作性能改善结论。

因此**三个平台尚未完成统一的缩放响应验收**。所有成功和失败场景均正常关闭并通过
全文验证。没有用窗口能启动、最终尺寸正确或 Timer 数据替换中间帧失败。

## 原生度量与纯值算法

本机同输入、monospace 12 pt、宽 900 DIP，先 MeasureSize 再 MeasureMetrics，
连续三次；宽度、高度、字体、代码与后端相同。以下仅为 API 阶段，不是用户操作延迟。

| 用户 JSON 的阶段 | `68cd392` | 最终代码 |
| --- | ---: | ---: |
| 不换行首次原生排版，字体热后 | 40–42 ms | 41–43 ms |
| 不换行度量导出，字体热后 | 64–65 ms | 9–10 ms |
| 自动换行原生排版，三次 | 236–251 ms | 238–241 ms |
| 自动换行度量导出，三次 | 69–82 ms | 14–16 ms |

**约 240 ms 的完整原生折行调用并未消失。** 这才是后续必须继续拆开的工作；
不能只报度量导出的倍数。新 Pango metrics 顺便保留后续 region 绘制需要的行信息，
不额外复制完整字形数据。

额外诊断（不同固定样本，不能混成上述对照）：

- `BenchmarkWrappingMeasuredLongParagraph`：约 139 KB 已测普通值重新计算全部候选，
  0.159–0.164 ms，0 alloc。排除了原生分析、索引构造、最终行确认、几何和绘制。
- `BenchmarkRasterLineTraversal`：约 166 KB 固定段落，中部 900×700 region；
  原整段绘制约 3.4 ms，已有行数据约 1.0 ms，单独冷准备行数据加绘制约 4.5 ms。
  所以正常 metrics 路径同步复用行元数据；没有把独立冷准备误报成更快。
- Pango 完整 itemization 仍是同步 O(n) 工作，约 166 KB 曾测得约 11.5–12 ms。
  已分析 context 下 512／2048 byte range 塑形远小于完整折行，但仍不是整个控件耗时；
  大段落冷分析、索引构造也必须进一步限量，不能仅给外部循环加 deadline。

## 正确性回归

- Pango 新导出与独立 native cluster extents 精确对照：空行、Tab、RTL／isolate、
  fallback、连字、组合字／emoji、对齐、不同字号、placeholder 和范围样式。
- DirectWrite 与原生位置查询比较 UTF-8 范围、方向和坐标；坐标允许最多两个原生
  float32 ULP 的不同求和顺序误差。该数值断言未放宽，两端对齐维持原生逐位置查询。
- 候选实现的真实 Windows Shift+Left 曾出现 11 键只选 9 字符，未提交该候选。
  进一步回归在旧基线的不换行长行复现：Segoe UI 中部 Offset 23200 一键仅改变 affinity，
  Consolas／Arial 也失败。共享 Geometry 现在按操作数表示精度去重，只合并同一 byte，
  不改严格排序，不把真实间隙、不同 byte／bidi 和软折行位置合并。普通值独立预期及
  Linux／Windows／macOS 原生字体、窄宽和约 46 KB 不换行长行均通过。
- Pango region 像素与原整段 native renderer 在同机逐 byte 相等：预乘 RGBA、相同灰度
  字体选项和相位；覆盖负 region、1／1.5／2 倍及范围样式变更。原 RasterRegions 的
  1.25 倍与其全部断言保留。native serial 改变时重建借用行指针的缓存。
- Pango／CoreText 的有限目标与原段落比较 glyph、advance、原始 cluster／字符索引及
  方向：长 RLO／LRO／isolate、混合文字、Arabic、ffi 和组合字。两后端的目标切点
  不完全相同；没有声称任意切点安全，没有接入新的正式范围布局 API。

## 命令与交付验证

通过：

```bash
go test ./...
GOWORK=off go test ./...                  # GOUI
# SourceView 使用不提交的本地 workspace 联调
go test ./...                           # SourceView
go test -race ./gui/textedit ./gui ./platform/typography/pango
```

Windows 原生执行完整 DirectWrite、textedit 测试二进制；macOS 原生执行完整 CoreText、
textedit 测试二进制，均通过。原生字体及上下文测试在实际调用 goroutine 锁线程。
两个目标的 SourceView interaction 从本机交叉构建并在真实桌面运行；交叉编译本身
不算桌面通过。最后代码的缩放失败如上一节所列。

失败且保留：SourceView `GOWORK=off go test ./...` 返回 `updates to go.mod needed`。
该模块已有用户的相邻 GOUI replace 和未完成的独立依赖图；本轮未修改／提交用户
`go.mod`，不执行 tidy 掩盖问题，不把 workspace 通过写成独立发布通过。

复现入口位于 SourceView `tests/window/editor/interaction`，入口注释列出构建、真实
键鼠、尺寸拖动、原状态恢复和正常关闭验证；`desktop.py`／`remote.py` 的断言未修改。
原生／算法辅助复现：

```bash
go test ./gui/textedit -run 'Test(NativeCharacterNavigation|VisualNavigation|Wrapping)' -count=1
go test ./platform/typography/pango -run 'Test(BulkMetrics|Raster|ContextualSubstring)' -count=1
go test ./platform/typography/pango -run '^$' -bench 'Benchmark(MetricsRunQueries|RasterLineTraversal|ContextualShaping)' -benchtime=10x -count=3
go test ./gui/textedit -run '^$' -bench BenchmarkWrappingMeasuredLongParagraph -benchtime=100x -count=3
```

副作用：窗口／指针／焦点位置改变；本机驱动临时使用并尽力恢复 UTF-8 剪贴板，
不承诺恢复富格式。既有文字测试仍会按其旧行为生成图片，未将产物纳入提交。
远端测试只使用任务临时目录；测试窗口全部退出，Windows 自建交互启动任务已删除，
既有 SSH／VNC／RDP 连接保留。首次错误样本路径导致的启动失败及被修正的候选失败
未计入通过场景或性能统计。用户两仓其他未跟踪工作保留。

## 下一步，尚未完成

1. 把完整上下文的有限塑形／可渲染片段落实到三后端；DirectWrite 对应事实尚未完成。
   不从独立子串或固定方向前缀推断完整 bidi，不新建后端 Reflow／视口调度接口。
2. 索引构造、冷分析和候选确认本身也逐批推进。新 WrapMeasurements 的完整复制／扫描
   不能直接放进输入回调，不能仅将整段工作延后一帧。
3. TextView 先接入真正的有限行组、视口优先及最新需求取消，再让 SourceView 复用公开
   普通值索引并映射折叠／IME／着色；TextInput 保留不换行几何回归并接入水平片段。
4. 完成局部编辑后的入口状态及后缀收敛、换宽复用、几屏资源边界；以完整正确性和同机
   真实操作验收，继续保留上述缩放失败，直到原断言通过。

后续已完成的初始高度估计、近期完整宽度复用及独立 X11 对照见
[分配验证](PlanTextLayoutAllocation.md)；没有将这些减少重复工作的改动标成有界分片已完成。
