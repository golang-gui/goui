# goui

goui 是一个 Go 原生、AI 友好的声明式 UI 框架。

## 预置控件声明式 UI

```go
import . "github.com/golang-gui/goui/ui"

func main() {
    text := MakeState("Hello GOUI!")
    input := ""

    App.Run(func() RootView {
        return Window("main").
            Title("Main Window").
            Content(
                VBox(
                    Label(text.Get()),
                    HBox(
                        Label("Please input: "),
                        TextInput().
                            Text(input).
                            OnText(func(value string) {
                                input = value
                            }),
                    ),
                    HBox(Button("Submit").OnClick(func() {
                        text.Set(input)
                    })),
                ),
            )
    })
}
```

`ui` 包是声明式的。View 是普通 Go 值，通过链式方法配置；`ui.App`
负责命令式运行时操作；`State.Set` 会请求一次合并后的 UI 重建。

## 组合式 UI

简单组合可以直接写成返回 `ui.View` 的函数：

```go
func Login(username, password string, onLogin func()) View {
    return VBox(
        HBox(Label("Username: "), TextInput().Text(username)),
        HBox(Label("Password: "), TextInput().Text(password)),
        HBox(Button("Login").OnClick(onLogin)),
    ).Spacing(8)
}
```

更复杂的组合可以定义值类型，通过链式方法设置属性，并实现
`Build() View`：

```go
type LoginView struct {
    username string
    password string
    onLogin  func()
}

func Login2() LoginView {
    return LoginView{}
}

func (v LoginView) Username(value string) LoginView {
    v.username = value
    return v
}

func (v LoginView) Password(value string) LoginView {
    v.password = value
    return v
}

func (v LoginView) OnLogin(fn func()) LoginView {
    v.onLogin = fn
    return v
}

func (v LoginView) Build() View {
    return VBox(
        HBox(Label("Username: "), TextInput().Text(v.username)),
        HBox(Label("Password: "), TextInput().Text(v.password)),
        HBox(Button("Login").OnClick(v.onLogin)),
    )
}
```

组合 View 会被自动展开，因此可以像预置控件一样使用：

```go
VBox(
    Login2().
        Username("xuges").
        Password("123456").
        OnLogin(func() {
            text.Set("Login success.")
        }),
    Label(text.Get()),
)
```

## 自定义 View 与 Widget

自定义 `ui.WidgetView` 是声明式 `ui.View` 和命令式 `gui.Widget` 的桥。
下面的例子包装了一个 `gui.Label`，并把最新回调保存在 reconciler state
里：

```go
type HyperlinkView struct {
    url     string
    onClick func()
}

func Hyperlink(url string) HyperlinkView {
    return HyperlinkView{url: url}
}

func (v HyperlinkView) Url(value string) HyperlinkView {
    v.url = value
    return v
}

func (v HyperlinkView) OnClick(fn func()) HyperlinkView {
    v.onClick = fn
    return v
}

func (v HyperlinkView) Build() View {
    return v
}

type hyperlinkState struct {
    onClick func()
}

func (v HyperlinkView) Mount(ctx BuildContext) gui.Widget {
    state := &hyperlinkState{onClick: v.onClick}
    click := gui.NewClickEventController()
    click.ConnectClicked(func(gui.EventContext) {
        if state.onClick != nil {
            state.onClick()
        }
    })

    label := gui.NewLabel(v.url)
    label.AddEventController(click)
    ctx.SetState(state)
    return label
}

func (v HyperlinkView) Update(ctx BuildContext, widget gui.Widget) {
    ctx.State().(*hyperlinkState).onClick = v.onClick
    widget.(*gui.Label).SetText(v.url)
}

func (v HyperlinkView) Unmount(BuildContext, gui.Widget) {}
```

如果需要完全自定义 `gui.Widget`，通常嵌入 `gui.WidgetBase`，并按需实现
测量、绘制和语义快照等行为：

```go
type BadgeWidget struct {
    gui.WidgetBase
    text string
}

func NewBadgeWidget(text string) *BadgeWidget {
    return &BadgeWidget{text: text}
}

func (w *BadgeWidget) SetText(text string) {
    if w.text == text {
        return
    }
    w.text = text
    w.RequestLayout()
}

func (w *BadgeWidget) Measure(geometry.Size) geometry.Size {
    return geometry.Size{Width: 72, Height: 24}
}

func (w *BadgeWidget) Paint(p gui.Painter) {
    if !w.Visible() {
        return
    }
    size := w.Rect().Size
    p.FillRoundRect(
        geometry.Rect(0, 0, size.Width, size.Height),
        12,
        graphics.RGB(40, 110, 220),
    )
}

type BadgeView struct {
    text string
}

func Badge(text string) BadgeView {
    return BadgeView{text: text}
}

func (v BadgeView) Build() View {
    return v
}

func (v BadgeView) Mount(BuildContext) gui.Widget {
    return NewBadgeWidget(v.text)
}

func (v BadgeView) Update(_ BuildContext, widget gui.Widget) {
    widget.(*BadgeWidget).SetText(v.text)
}

func (v BadgeView) Unmount(BuildContext, gui.Widget) {}
```

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

MVP 已完成架构验证。当前已经包含平台事件、命令式 GUI 层、基础控件、最小声明式 `ui` 层，以及通过 `ui.App.RequestUpdate` 进行的更新调度。

**API 仍未稳定，goui 暂时不能用于生产环境。**

## 后续路线

- 设计轻量的样式和系统设置机制。
- 继续验证 IME、剪贴板、无障碍、系统设置等平台能力。
- 暴露稳定的检查与自动化协议。
- 其他平台的支持。



## 许可证

MIT License。
