# 长段落分配与近期宽度复用验证

2026-10-22。基线为 GOUI `9b4cbeb`、SourceView `d4d606f`；优化后的生产代码为
GOUI `6e4fc88`（有限样本估计）／`1eabb0b`（近期宽度缓存）、SourceView `8aa754e`
（初始估计及局部编辑高度保留）。设计见 [第 15 节](DesignIncrementalTextLayout.md#15-避免重复分配宽度与保留近期结果2026-10-22)。

**本批减少重复调用，完整原生段落折行仍可能阻塞 UI，尚未完成有界分片排版。**

## 实现及正确性

- TextView：首次长单段文本从至多 512 byte 的原生样本估计滚动范围，避免完整测量
  后滚动条出现再重复折行。样本立即释放，精确几何始终来自完整原生段落。
- TextView：长段落换宽时与历史共用四项／1 MiB 源文本的保留区；恢复相同完整输入、
  原生盒宽时转移所有权并复用原生布局、行高与 Geometry。连续新宽度仍会未命中。
- SourceView：采用同样的初始估计，单段局部编辑保留原高度作为暂定值；新测量替换
  估计。原有每文本两个宽度的缓存不扩大。本批不修改 typography 或公共 API。

新增回归验证样本上限、重复 Measure 不重测样本、滚动条首次分配仅创建一个完整
布局、已测宽度不调用原生测量、缓存预算、编辑／Undo、释放回调重入、卸载释放。
SourceView 原生对照包含长 RLO、Arabic／中文／组合字／emoji／Tab、零 advance 文本，
独立完整原生布局的所有行与 cluster 坐标一致。全文删除后正常移除滚动条。

将新增用例复制到两仓基线的独立归档后执行，原版本分别在初始估计、宽度复用及
局部编辑高度保留断言失败；同一用例在当前代码通过，没有放宽已有断言。

## 独立 X11 的真实操作

Ubuntu 22.04／linux amd64、Go 1.25.8、Pango 1.50.6／Cairo 1.16.0、TigerVNC 1.12.0
X server、Xfwm4 compositor 开启、software painter、缩放 1。单独创建带 MIT cookie
的 `:188`，1500×1100、24 bit；禁用 X11 TCP，RFB port 为 -1。WM 使用独立 DBus、
配置和缓存目录。输入及观察直接经 XTest／XGetImage，没有 VNC 图像传输。
没有在用户正在使用的 `:3.0` 或其他既有 X11 会话执行键鼠、剪贴板或窗口操作。

使用 SourceView 的 `tests/window/editor/interaction`，Modern、WindowChromeIntegrated，
自动换行；SourceView 保留行号、折叠，关闭自动补全。测量期间不并行运行本机编译、
Go 测试或其他自建 GUI 场景。输入文件只读，编辑内存，正常关闭比较全文原始内容。

用户 `1.json`：157,862 byte、一个逻辑段落，SHA-256 为
`7baa22a894b415fdc3425c94f8a541d0ded8f77965541c46af3c95a3364c798b`，验证后未变。
前后各两轮完整编辑／六项拖动，另各两轮中部编辑及三次真实 Alt+F10 最大化／还原。
后者单独检查确切近期宽度，不替代连续新宽度拖动。

| 测量边界，ms | 基线两轮 | 本批两轮 |
| --- | ---: | ---: |
| TextView 首次完整 Widget 帧，布局到 Paint | 708–711 | 408–419 |
| TextView 三次最大化后的还原，桌面可见等待 | 428–1,043 | 79–84 |
| TextView 中部短词粘贴，input→Paint | 471–488 | 476–477 |
| TextView 中部新字符，change→Paint | 444–445 | 452–456 |
| SourceView 首次非 pending 的 Widget 帧 | 349–356 | 357–363 |
| SourceView 中部短词粘贴，input→Paint | 776–789 | 792–844 |

首帧指标不包含文件读取、模型创建或此前的应用启动；SourceView 首个 pending 帧
约 20 ms，不能拿它当完整文本已准备的帧。输入计时到最后的 Widget Paint，不含
native Present；无 KeyDown 的 IME 提交用 change→Paint 单独表达。最大化／还原
由 WM 消费按键，只报告外部键鼠发送到非 pending 像素可见的时间，含 WM 与采样，
不能标成编辑器 input→Paint。采样间隔 5 ms，实际间隔随每次操作日志记录。

TextView 后续最大化有约 79–83 ms 的命中，也仍有约 410–411 ms 的未命中，首个
新宽度约 469–472 ms。没有把某一次命中推广成任意换宽都可即时复用。SourceView
既有缓存通常约 96–100 ms，仍观测到约 713／410 ms 的重新测量等待，未见稳定新收益。
新字符和新宽度仍是完整段落工作，本批没有改善它们的原生折行耗时。

### 通过和未通过

通过：所有完成场景的真实中部点击、插入、Backspace、选择、替换、Undo／Redo、
Left／Delete；完整场景的首尾 64 KiB 多行粘贴、删除、历史恢复；最终六项窗口尺寸、
selection／caret／长度及正常关闭后的全文一致性。最大化／还原新增场景全部通过。

未通过：TextView 前后两轮的四项宽／角落拖动均只有一个中间 viewport，原断言要求
至少两个。SourceView 基线两轮通过，本批一轮通过，另一轮 corner-narrow 只有一个
中间 viewport；第二个帧约在 507 ms 才可见，实际拖动约 501 ms。保留失败，尚未归因，
不能用最终尺寸正确或另一次通过替代。完整场景 GUI 最大 Timer 间隔 TextView 约
709–712 → 718–727 ms，SourceView 约 578–620 → 574–594 ms；阻塞目标未达成。

额外对既有公开语料各执行 TextView／SourceView 一轮中部真实编辑及两次最大化／
还原，共十个场景：128 KiB 长段 txt、2.7 MB OpenGL XML、3.9 MB HTML、0.93 MB Rust
Markdown、3.3 MB《战争与和平》txt，全部通过并全文恢复。来源／许可继续见
[语料清单](../../goui-sourceview/tests/window/editor/corpus/testdata/corpus.json)。这些只有
优化后的验收，没有用来推断前后性能提升；也没有代替用户单段 JSON 的完整拖动失败。

## 命令与产物

通过：两仓 `go test ./...`；GOUI `GOWORK=off go test ./...`；GOUI
`go test -race ./gui ./gui/textedit`；SourceView `go test -race .`。首个功能的精确暂存树
另归档执行 `GOWORK=off go test ./gui`，通过。Windows amd64／macOS arm64 的 gui 与
SourceView 原生测试包均交叉编译通过，本批未在这两种系统原生运行。

仍失败：SourceView `GOWORK=off go test ./...` 提示 `updates to go.mod needed`。
保留用户已有 replace 及未完成的独立依赖图，不执行 tidy、不提交其 go.mod，不能称作
独立发布验证通过。两仓均保留其他已有未跟踪工作，没有推送远端。

窗口场景复现入口（先建立独立、带认证且启用合成器的 X11 会话，再使用它的环境）：

```bash
go build -o /tmp/source-interaction ./tests/window/editor/interaction
# 在该独立会话中启动以上程序：
XMODIFIERS=@im=none GOUI_PLAT_PAINTER=software GOUI_PLAT_SCALE=1 \
  /tmp/source-interaction -kind text -file /outside/document.json
# 在同一独立会话的另一进程执行，完整编辑／拖动：
python3 tests/window/editor/interaction/desktop.py \
  --file /outside/document.json --kind text --output /outside/full-case
# 单独启动新的原始窗口后，中部编辑及近期宽度循环：
python3 tests/window/editor/interaction/desktop.py \
  --file /outside/document.json --kind text --output /outside/width-case \
  --middle-edit --width-cycles 3
```

以上命令在 SourceView 仓库执行，产物目录不能位于仓库内，还须核对应用 exit 0 与
`verified` 记录。驱动始终经过原生键鼠事件；最大化时 GUI resize inset 从 5 DIP
变为 0，像素观察现在同时识别这两个真实位置，原 magic／header 与拖动断言保留。
首次未开启 compositor 的会话启动失败、旧观察器不识别最大化 inset 的诊断失败均
单独保留，修正测试环境／观察器后重新执行，未混入通过场景及前后数据。

当次日志、操作记录、截图、原始二进制和基线归档在仓库外
`/tmp/goui-layout-next-20261022/`。基线二进制 SHA-256 为
`970bf4e4b08545277fcab94b0701123e9d02d4881d6f0b4597325c3a00841cd9`，本批为
`c927410feeddb6ca90026cdfde52fe89544e11f541948e75754bebfd552e7a29`。
测试改变的窗口、焦点、指针和 UTF-8 剪贴板均属于独立会话；结束后关闭测试窗口及
自建 X server／WM，既有会话保留。部分旧 Go 测试仍按其既有行为生成图片，未纳入提交。
