# 长段落排版实施记录

## 状态

2026-10-20。用户选择保持完整正确性，继续完善通用方案。设计见
[增量排版](DesignIncrementalTextLayout.md)。

- [x] 分开测量原生排版与度量导出，验证独立文本窗口原型的 bidi 缺陷。
- [x] 共享几何按行索引、可视行选区查询；TextView／TextInput 和 SourceView 接入。
- [x] 确定性回归与排版阶段 benchmark。
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
