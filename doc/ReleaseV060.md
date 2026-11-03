# GOUI v0.6.0 发布说明

[v0.6.0 已正式发布](https://github.com/golang-gui/goui/releases/tag/v0.6.0)，三平台 Demo 为0.6.0 / build 1。
标签及正式附件冻结于提交 `691493afe4a390de52a9036b84990a35ed827ed4`；标签不随发布后的文档收尾移动。
macOS 最终 ZIP 已通过 Developer ID 签名、公证、票据及 Gatekeeper 检查。
三平台附件、校验摘要和完整验收范围见 Release 的
[BUILD-INFO.md](https://github.com/golang-gui/goui/releases/download/v0.6.0/BUILD-INFO.md) 与
[SHA256SUMS](https://github.com/golang-gui/goui/releases/download/v0.6.0/SHA256SUMS)。

本版本以 v0.5.0 为基线，新增文本输入策略、可配置遮蔽与末字素显露，以及可选绘制诊断；同时修正文本缓存、测量和弹窗交互。三平台 Demo 同步更新为 **0.6.0 / build 1**。

## 主要变化

- **文本输入策略**：TextInput 和 TextView 支持 InputFilter，在用户编辑提交模型前接受、拒绝或转换输入。`gui/textedit` 提供策略组合、字素限长、数字、ASCII 用户名和密码策略。
- **可配置遮蔽**：TextInput 支持 `Obscured`、`ObscureChar` 和 `ObscureDelay`，可自定义掩码、切换正文显示，或短暂显露刚输入的末字素。失焦、光标移动等操作恢复遮蔽，粘贴不触发显露；遮蔽时限制复制/剪切并脱敏语义正文。
- **可选诊断**：`gui/diagnostics` 提供显式开始、读取和停止的 CPU 帧及资源操作采集，可选逐控件计数；`gui/dev` 增加对应诊断端点。默认不采集，不为读数启动绘制或轮询。
- **文字资源缓存**：Label 的等价样式通知保留排版和绘制缓存，独立测量不再反复失效显示资源。
- **测量与弹窗修正**：TextView 测量保留已分配文本几何；TableView 测量保留伸展列宽；修正重新打开 Popover 的遗留 Hover 和 DropDown 重测后的滚动条间隔。
- **Demo 与文档**：文本页展示输入策略、自定义遮蔽及延迟反馈；窗口检查页支持跨页诊断、手动读取和停止冻结。新增文案覆盖中、英、日、韩、德、法、俄、阿拉伯八种语言；中英文 README 全面更新当前能力清单。

## 使用与升级

```sh
go get github.com/golang-gui/goui@v0.6.0
```

```go
ui.TextInput().BindText(&password).
    InputFilter(textedit.Password(128)).
    Obscured(true).ObscureChar('•').
    ObscureDelay(600 * time.Millisecond)
```

这里 `textedit` 是 `github.com/golang-gui/goui/gui/textedit`，`password` 是稳定持有的 `ui.State[string]`。
现有控件默认不遮蔽、不附加输入策略，延迟显露默认关闭。策略约束用户编辑；程序赋值及历史回放保留原有语义，业务数据校验仍由应用负责。限长按扩展字素计算，不按 UTF-8 字节计数。遮蔽改变呈现，不改变绑定模型的正文。

诊断入口必须在 GUI 线程调用。FPS 是近期 CPU 完帧间隔估算，不代表 GPU 执行或显示刷新率；资源计数是操作量，不代表实时内存或显存。详见仓库的 DesignTextInput、DesignDiagnostics 和 Demo 说明。

## Demo 下载与验证

下载本页的 Windows amd64 EXE、Linux amd64/X11 可执行文件或 macOS arm64 ZIP。
Linux 运行前执行 `chmod +x goui-demo-v0.6.0-linux-amd64`；macOS 解压后打开 `GOUI Demo.app`。
Mac App 已使用 Developer ID 签名、通过 Apple 公证并装订票据；Windows EXE 未执行 Authenticode 签名。
`SHA256SUMS` 和 `BUILD-INFO.md` 提供来源、摘要、签名结果和逐平台验收范围。

本轮干净候选源码全仓测试：**3415 项通过、10 项跳过、零失败**。三平台全仓交叉构建、相关 race 及最终 Demo 桌面操作通过，具体环境与覆盖以 BUILD-INFO 为准。
Linux 正式目标仍为 amd64/X11；原生 Wayland、Linux arm64、触摸、子菜单和原生 Popover IME 未在本版本新增。真实中文输入法、混合 DPI、多屏、全量拖放和每种后端组合没有在本轮重新验收。

## English summary

- Add composable input policies to TextInput/TextView, including grapheme limits, numeric, username and password policies.
- Add configurable TextInput obscuring, custom masking characters and optional brief last-grapheme reveal. Programmatic assignments and history replay retain their existing semantics.
- Add opt-in CPU-frame and resource diagnostics, optional widget counters, and the development-protocol adapter. Counts are operations, not live memory usage; FPS is not a presentation-rate measurement.
- Preserve Label text caches across equivalent styles and independent measurements. Fix allocated editor/table geometry and popup hover/scrollbar spacing.
- Refresh the eight-language Demo and the complete feature inventory in both READMEs.
- Demo **0.6.0 build 1** is provided for Windows amd64, Linux amd64/X11 and macOS arm64. The macOS bundle is Developer ID signed, notarized and stapled. See BUILD-INFO.md for test environments and limits.

Full changes: https://github.com/golang-gui/goui/compare/v0.5.0...v0.6.0
