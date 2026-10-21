# 文本模型 LF 索引验证

2026-10-21。设计见 [TextEditing](DesignTextEditing.md)。本轮仅修改 `gui/textedit`
的私有 Piece Tree；TextInput、TextView、SourceView 共用模型查询收益，不改变
TextLayout、线程、换行和 cluster 语义，不代表三个控件的所有长段落问题已经解决。

## 变更与回归

- 每个至多 64 KiB 的文本块记录 uint16 LF 偏移，piece 的范围查询与子树累计配合。
- 只索引新增尾部，保留 undo/redo 中旧 piece 的范围；块随最后一个引用释放。
- 查询下一个 LF 时不线性遍历任意长的无 LF piece 链。
- 覆盖 byte 65535 的 LF、65536 的排他边界、全 LF 块、UTF-8 分块、切分、随机编辑、
  undo/redo、撤销后分支和跨多个无 LF 块。测试用独立字符串 LF 扫描核对行范围，
  原树不变量的 LF 预期也改用独立 bytes.Count。

## 查询对照

Linux amd64，Go 1.25.8，Xeon Gold 5318Y。旧版 `a618df5` 的临时 worktree 仅复制
相同 benchmark 文件；每组固定 10,000 次，重复三次。计时只包括随机顺序的
`LineRange` 加 `LineAt` 及正确性断言，不含下载、读取、建模、GUI、原生排版或绘制。
两个版本均为 0 B/op、0 allocs/op。真实文件来源、SHA-256 和下载脚本随 SourceView
的 `tests/window/editor/corpus` 保存，输入文件留在仓库外，不复制第三方文档进本库。

| 输入 | 原版中位数 ns/op | LF 索引中位数 ns/op |
| --- | ---: | ---: |
| Gutenberg War and Peace TXT，3,359,405 bytes | 52,004 | 291.5 |
| GitHub REST API JSON，13,502,446 bytes | 55,035 | 356.2 |
| OpenGL Registry XML，2,774,722 bytes | 51,155 | 289.7 |
| Gutenberg War and Peace HTML，4,039,247 bytes | 50,055 | 296.0 |
| Rust releases Markdown，931,116 bytes | 50,257 | 260.4 |

```bash
GOUI_CORPUS_DIR=/path/to/corpus go test ./gui/textedit -run '^$' -bench '^BenchmarkCorpusLineQueries$' -benchtime=10000x -count=3
go test ./gui/textedit -run '^$' -bench '^BenchmarkModelLineQueries$' -benchtime=10000x -count=3
```

未指定语料目录时，新增语料 benchmark 显式 skip；常规模型回归不依赖下载或联网。
合成 400,000 行查询对照约 45–47 µs 降至 0.35 µs，仅为该查询路径，不是编辑器帧率。

## 验证

- `go test ./...`：通过，包含本机已有的未跟踪测试；没有将其纳入提交。
- `go test -race ./gui/textedit ./gui`：通过。
- Windows amd64、macOS arm64 的 `CGO_ENABLED=0 go build ./gui/...`：通过。
- 原生 Windows/macOS 运行：未执行。此次没有变更平台实现。

额外 LF 索引按实际 LF 数量占用内存，uint16 元素为两字节，另有切片头与容量余量；
不将输入字节预算或 Go 分配量冒充完整进程资源。LF 密集文件会相应增加模型内存。
