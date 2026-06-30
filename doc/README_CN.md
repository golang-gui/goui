# goui

goui 是一个 Go 原生、AI 友好的声明式 UI 框架。

## 声明式 UI

```go
import . "github.com/golang-gui/goui/ui"

var (
    root *Root
    text string
    input string
)

root = MountWindow(win, func() View {
    return VBox(
        Label(text),
        HBox(
            Label("Input:"),
            TextInput().Text(input).OnText(func(value string) {
                input = value
                text = value
            }),
        ),
        Button(Label("Submit")).OnClick(func() {
            text = input
            root.RequestUpdate()
        }),
    )
})
```

MVP 已经证明，Go 可以用普通值和链式方法表达清晰的声明式 UI。

## 核心思想

goui 建立在三层架构之上：

```text
ui        面向应用代码的声明式 View 层
gui       面向控件开发者的命令式 Widget 层
platform  原生窗口、事件、绘图和文字排版层
```

MVP 已经验证了主要设计：

- `ui` 为应用代码提供简洁的声明式 API。
- `gui` 为复杂控件开发者提供命令式 Widget 内核，包括布局、绘制、事件、焦点、生命周期、信号和语义快照。
- `platform` 尽可能利用平台原生窗口、绘图、输入和文字排版能力。

最终目标是形成一个 Go 原生、方便人类理解、也方便 AI 检查和驱动的 UI 框架。

## 平台方向

当前桌面后端：

| 平台 | 窗口系统 | 绘图 | 文字排版 |
| --- | --- | --- | --- |
| Windows | Win32 | Direct2D | DirectWrite |
| macOS | Cocoa | OpenGL | Core Text |
| Linux | X11 | OpenGL | Pango |

goui 希望把平台差异限制在 `platform` 层，同时保留各系统在渲染、文字排版和输入上的优势。

目标构建方式仍然是：

```bash
CGO_ENABLED=0 go build ./...
```

## AI 与自动化

AI 支持是架构的一部分，不是后期附加功能。

框架设计目标是让工具和 AI agent 可以检查语义快照，理解控件层级和边界，并通过与真实用户一致的事件路径驱动应用。

## 当前状态

MVP 已完成架构验证。当前已经包含平台事件、命令式 GUI 层、基础控件、最小声明式 `ui` 层，以及通过 `ui.Root.RequestUpdate` 进行的更新调度。

**API 仍未稳定，goui 暂时不能用于生产环境。**

## 后续路线

- 设计轻量的样式和系统设置机制。
- 继续验证 IME、剪贴板、无障碍、系统设置等平台能力。
- 暴露稳定的检查与自动化协议。
- 其他平台的支持。



## 许可证

MIT License。
