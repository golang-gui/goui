# 通用文本扩展实施与验证

日期：2026-10-16。基线为 GOUI `4167bb79823efffbea43e00558260448959536b0`。
本轮只提交通用文本能力及调用迁移；GOUI 不依赖 SourceView，不引入语言或高亮资源。
原有 Plan/Roadmap 修改、其他未跟踪示例和测试不纳入提交。

## 实施范围

- `textedit.Model.ApplyBatch`：原始字节坐标、完整预检、一次修订/通知/历史项，保存前后选区。
- `TextFormat.TabWidth` / `TextLayout.SetTabWidth`：DIP，0 恢复原生默认；三后端原生排版，文本和 cluster 保留。
- 剪贴板：GUI 写入返回 error，平台/GUI 读取回调为 `(string, error)`。
  **没有文本格式或文本为空均返回 `("", nil)`**，不另设无文本错误。
  UI 读取另返回调度受理错误；拒绝受理不回调，受理后回调恰好一次。
- 修正 Pango 绑定边界：绑定直接对应 TabArray 与 Layout.SetTabs，重复站点策略和临时数组所有权在排版适配层。
- 修正 Windows 写入所有者：借用平台隐藏 HWND；写入失败释放未转移的 HGLOBAL，读取检查分配范围内的 UTF-16 终止符。
- X11 外部读取使用每请求临时窗口，校验回复身份与属性，协商 UTF-8 / Latin-1；
  五秒期限覆盖协商和读取，循环退出/销毁完成待处理请求，清理在回调之前。
  单次读取最多 16 MiB，INCR 和非法/截断数据明确报错。期限依赖事件循环继续运行。
- 迁移 TextInput/TextView、声明式 demo 和测试替身。空文本粘贴无操作，保留选区和历史；
  写入失败时 Cut 不删除，示例只有成功后才报告复制成功。
- SourceView 同步迁移空文本粘贴、错误信号与专题设计；其代码不在本仓提交。

契约细节分别见 [文本编辑](DesignTextEditing.md)、[排版](DesignTypography.md) 和 [剪贴板](Clipboard.md)。

## 验证方法与环境

拟提交文件覆盖到 `git archive` 基线产生的干净副本后验证，避免依赖工作副本中未跟踪代码。
两台远端收到相同源码归档，SHA-256 为
`a89f9e3fa422ea5664f73dc8627a507c2bca5f60991b6b2b53b0bb5e8bbf14a3`；
这是最终 Go 代码的验证输入，之后仅补充本记录等文档。

| 环境 | 实际执行与结果 |
| --- | --- |
| Linux amd64，Ubuntu 22.04.5，Go 1.25.8，Pango 1.50.6 | GOUI 工作副本与干净副本 `go test ./...` 均通过；干净副本使用 `GOWORK=off` |
| Linux 独立 Xtigervnc X11 / Xfwm4，Software，1x / 2x | SourceView `go run ./tests/window/editor -auto` 均通过，日志确认实际 PixelScale 与软件资源类型；默认不写剪贴板 |
| Linux 相关 race | `go test -race ./gui/textedit ./gui ./ui ./platform/linux/x11 ./platform/typography/pango` 通过 |
| Windows 10 19045 amd64，Go 1.24.0，SSH Session 0 | `go test -count=1 ./gui/textedit ./gui ./ui ./platform/typography/directwrite ./example/demo` 通过；`GOWORK=off go build ./...` 通过；原生剪贴板往返与无文本格式读取通过 |
| macOS 15.7.7 arm64，Go 1.24.9 | `go test -count=1 ./gui/textedit ./gui ./ui ./platform/typography/coretext ./platform/darwin/cocoa ./example/demo` 通过；`GOWORK=off go build ./...` 通过；原生剪贴板往返与无文本读取通过 |
| SourceView 联调 | 本机和两台远端 `go test ./...` 通过，本机 `go test -race ./...` 通过；均使用两模块本地 workspace |

三端 `TestTabWidth` 实际执行：连续/重复 Tab 的独立坐标预期、setter 通知、文本/格式保留、恢复默认、颜色修改后位置及非法创建参数。
这些是原生排版验证，不是所有绘图后端、RTL、换行、跨屏或真实 IME 验收；原生剪贴板程序没有绘图后端或缩放参数。
全仓命令保留既有需要环境开关的交互用例跳过条件，未把跳过计作场景验收，也未扩大 skip；本轮未执行的专项列在下节。

## 剪贴板专项

显式程序为 `go run ./tests/window/platform/clipboard -write`，入口注释列明副作用。
X11 外部 owner 在 `tests/window/platform/clipboard/owner`，通过真实跨进程 selection 协议验证：

- UTF-8、Latin-1、旧 owner 拒绝 TARGETS 时的回退、无 owner、无文本格式。
- 属性格式错误、非法 UTF-8、INCR、已声明转换被拒绝、超出 16 MiB 的数据均为读取失败。
- 不响应 owner 在五秒期限内返回超时；100 ms 后退出循环完成待处理读取，不等待外部看门狗。
- 逻辑回归验证只完成一次、回调重入、旧 requestor/其他 selection/旧 target 不污染当前读取、销毁后拒绝新请求。
- GUI/UI 回归通过正常键盘分发验证 Cut 失败与空粘贴保留内容/历史；另验证停止后的调度拒绝及队列退出。

Linux 所有写入均在独立 X server；退出时销毁该会话，不触碰日常桌面剪贴板。
Windows 写入仅发生在隔离 Session 0，不代表交互用户会话的剪贴板/输入已验收。
macOS 使用临时 Swift 包装程序快照并恢复 pasteboard item/type/data；本次原始状态为 0 items，恢复后核对仍为空。
远端全仓既有窗口测试曾短暂创建原生窗口；本轮没有发送消息、发布、推送或修改远端既有检出。
既有测试的图片输出保留在隔离副本中；工作副本全仓测试仍有既有写图副作用，新用例不写源码目录产物。

## 失败、环境缺口与保留项

- Linux 初次不设置 DISPLAY 的全仓测试失败：既有 `platform/typography/test.TestTypography` 报 `can not open display`。
  已在独立 X11 重跑并通过。SourceView 窗口首次仅有 X server、没有 WM，布局边界为零而失败；补齐 Xfwm4 后 1x/2x 原断言通过，未弱化检查。
- GOUI `go vet ./...` 返回非零；基线与当前代码排除行号后逐项对照均为 193 条，未增加或减少。
  包括原生绑定的 unsafe.Pointer 和既有未命名字段字面量提示，本轮不扩大为全仓清理。
- macOS `GOWORK=off go test ./...` 仍失败于既有 `platform/test.TestWindowInteractionCapabilities`：
  resize 返回 ErrUnavailable，而测试预期 ErrUnsupported。在纯基线执行相同测试复现。
- Windows `GOWORK=off go test ./...` 仍失败于三个包，已在纯基线执行相同用例逐项复现：
  `graphics/direct2d.TestPremultipliedNativeColors` 的 gradient/stroke-gradient 1x/2x 像素为 `{64 0 64 128}`，预期 `{32 0 96 128}`；
  `win32.TestWindowStyle` 的 build 22000 样式为 `0x50000`，预期 `0xcf0000`；
  `platform/test` 的 TestPopup/TestPopupPainter/TestWindow/TestWindowRequestPaint 在 Session 0 等不到 paint，分别超时。
  不把这些完整测试失败写成通过，不修改断言或扩大 skip。
- 本轮不验证交互桌面真实 IME、所有 GPU 后端/显示 DPI、跨屏、长时间资源压力或 X11 INCR。
- SourceView 的 `GOWORK=off go test ./...` 仍不能通过：所声明的已发布 GOUI v0.1.1 不含 ApplyBatch/BatchOptions。
  本地 GOUI 提交不等于依赖已发布；SourceView 保留可获取的依赖版本，发布前仍须更新并验证独立依赖。
