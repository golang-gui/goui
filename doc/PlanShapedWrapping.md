# 完整字形来源的折行验证

2026-10-22。设计见 [DesignShapedWrapping](DesignShapedWrapping.md)。本批已接入
Linux/Pango 的 TextView 及独立 SourceView；不是仅运行 advance 原型。其他原生
排版后端保留原路径，没有宣称 Windows/macOS 已获得同样收益。

## 环境与测量边界

- Ubuntu 22.04.5，Go 1.25.8，Pango 1.50.6／Cairo 1.16.0。
- 独立 Xvnc `:189`、1500×1100×24、xfwm4 compositor on、Software、1×。
  私有 Xauthority／DBus／XDG 目录；未在用户桌面注入输入。输入文件只读。
- 修改前：GOUI `b7f76ff`，SourceView `4b0dbc5`；修改后为本提交序列。
  用户已有的本地依赖配置未改动。
- 同一个 157862 byte／157830 rune 的单逻辑段落 JSON，SHA256
  `7baa22a894b415fdc3425c94f8a541d0ded8f77965541c46af3c95a3364c798b`。
- 两种控件各做修改前两轮、最终修改后两轮完整真实输入流程：头／中／尾插入、
  选中替换、撤销重做、删除、64 KiB 粘贴和六次 480 ms 边缘／四角拖动。
  事件由 XTest／剪贴板经正常桌面路径进入窗口，未调用编辑接口替代输入。
- `input→Paint` 到匹配状态的非 pending Widget.Paint；IME 单字符使用
  `change→Paint`，不含提交事件到达之前的时间。桌面等待另含原生呈现、窗口管理器
  与 5 ms 采样；两者不能混用。Timer 最大间隔为整次流程观察，不等于单一接口耗时。

二进制 SHA256：before `2525f608f6a7e7c29ae374abafae0bed8f4f11711eb4aab2a99cefca82b5e7c4`；
after `0584c4a0fdab0063dbd72515e7781f36ac79ecab9cb9555ff50ea487b7ec0226`。
原始日志、操作状态及截图在会话临时目录 `/tmp/goui-shaped-20261022/`，不是发行资源。

## 真实操作结果

表中为两轮范围，单位 ms。完整首次帧按第一个非 pending Widget.Paint 的 frame_ms
记录，不宣称是从磁盘打开到物理显示器呈现的总时间。

| 范围 | TextView 修改前 → 修改后 | SourceView 修改前 → 修改后 |
|---|---:|---:|
| 首次完整帧 | 403–406 → 195 | 357 → 185 |
| 中部插入字符，change→Paint | 442–443 → 179–181 | 614–752 → 248–250 |
| 中部粘贴短词，input→Paint | 462–472 → 217 | 422–782 → 280 |
| 中部点击，input→Paint | 27 → 26 | 33 → 31–32 |
| 陌生宽度缩窄，释放后桌面等待 | 814–824 → 58–63 | 647–669 → 84–89 |
| 宽度／四角拖动中的中间尺寸帧数 | 1 → 6–9 | 2 → 8–10 |
| 全流程 Timer 最大间隔 | 706–724 → 216–237 | 571–581 → 188 |

最终四轮完整流程全部 driver=0、app=0，并由退出时整个模型与原文比较通过。
原有拖动断言仍要求至少两个中间尺寸帧，未改条件。修改前 TextView 的两轮四项宽度／
四角拖动均失败，SourceView 两轮通过但等待仍很长。高度拖动两种控件前后均通过。
JSON 两种控件另各通过一次中部编辑及两轮最大化／还原；不与陌生宽度拖动混为一项。

五份既有外部语料 `paragraph-128k.txt`、`opengl.xml`、`war-and-peace.html`、
`rust-releases.md`、`war-and-peace.txt`，两种控件共 10 次中部编辑和最大化／还原
均通过、全文恢复通过。来源与散列沿用 SourceView 的
`tests/window/editor/corpus/testdata/corpus.json`；本批未重复下载或将内容提交。
这些是回归覆盖，没有对应修改前重测，不据此声称多行文件变快。

58519 byte 的持久 RLO 混合中英文／Arabic／Hebrew／组合符／emoji／Tab 长段落，
两种控件的中部编辑及两轮宽度切换通过。该场景显式使用 `--rtl-insertion`：
Right 跨回插入的 ASCII 字符，仍断言精确 byte/caret、Delete 及全文恢复。初次探索
误用默认 LTR driver，Left 实际按 RTL 向后文移动，因不满足 byte 递减谓词超时；
未计为通过，随后经真实 Undo／关闭完成全文验证。默认 LTR 判据未改变。

另有一次脚本诊断误选 before 二进制作为 after，已排除；最终表格只取
`linux-before-*-1/2` 与 `linux-after-*-7/8`。没有混用诊断和性能样本。

## 原生／逻辑与绘图验证

- 完整 Pango 与范围 glyph 绘制在 1／1.25／1.5／2× 下逐像素相等，涵盖透明度、
  Arabic、Hebrew、RLO、isolate、fallback、组合符及 ligature；拒绝拆 cluster。
- 300000 个 M 的未换行来源超过原生 int32 Pango 坐标，末尾有限范围仍有正确墨迹。
- 用完整原生段落的实际折点独立核对数百行 bidi 行内顺序；保留空格与 Pango 收掉
  breaking space 的差别另以明确 L1 几何预期验证，不删除差异来冒充一致。
- Software 的分 cluster／完整布局 RGBA 逐像素相等；覆盖 37 pt 斜体外伸、半透明、
  四档缩放。该用例发现并修正了 1.5× 下 float32 原点相减导致的 hinting 相位错误。
- 独立 X11 的 `tests/window/graphics/glyph_ranges` 显式要求实际 OpenGL，四档缩放
  最大 byte 误差均为 0（预设上限 1/255，依据 GPU 采样／8-bit 转换）。未静默回退。
- 同视图不同宽度只创建一份原生字形；不同视图独立。着色不改字形资源；缓存键含
  颜色／range／region／scale／相位；销毁及改变会释放图像。Fragment 在来源销毁后
  仍能查询几何。默认 Tab 在新显示行重新展开，含第一字体改变后的独立原生对照。原有测试断言保留。

通过：GOUI `GOWORK=off go test ./...`，SourceView 工作区 `go test ./...`，
GOUI `go test -race ./gui/textedit ./gui`，SourceView `go test -race .`。
Windows/amd64、macOS/arm64 的 gui、textedit、graphics、typography 及 SourceView／ui／
interaction 使用 `go test -exec /bin/true` 编译通过，**未运行原生桌面场景**。
一次完整 GOUI 测试中的 `icon/internal/pixels` 进程以 `signal: segmentation fault`
退出；没有 Go 栈。该包单独 `go test ./icon/internal/pixels -count=3` 及随后的完整
`GOWORK=off go test ./...` 均通过。根因未定位，不归类为已证实的既有失败，也没有
通过 skip／改断言处理。原始失败保留于 `goui-verified.log`。
SourceView `GOWORK=off go test ./...` 仍失败：`updates to go.mod needed`；本次未动用户
已有 go.mod，本地工作区不能当成独立发布验证。需要包含新公开契约的 GOUI 版本。

## 剩余限制

冷塑形、完整索引扫描、完整折点列表，以及特大单字素／零 advance 显示行仍可能
产生较长同步工作。中部新文本仍可超过 100 ms；本批没有完成事件循环硬预算、增量
后缀收敛、CoreText／DirectWrite 的字形来源接入。含强制分隔符的长逻辑段落保留旧路径。
下一批应首先拆分冷索引与折点发布，并减少编辑后必须重建的事实；继续保留完整
上下文、字素和 bidi 正确性，不能把这批收益表述为超大文本已完全不卡 UI。
