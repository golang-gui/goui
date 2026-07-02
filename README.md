# goui

[中文文档](./doc/README_CN.md)

goui is a Go-native and AI-friendly declarative UI framework.



## Declarative UI With Built-In Views

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

The `ui` package is declarative. Views are ordinary Go values with chainable
configuration methods. `ui.App` owns the command-style runtime operations, and
`State.Set` requests one coalesced UI rebuild.

## Composition Views

Simple composition can just be a function returning `ui.View`:

```go
func Login(username, password string, onLogin func()) View {
    return VBox(
        HBox(Label("Username: "), TextInput().Text(username)),
        HBox(Label("Password: "), TextInput().Text(password)),
        HBox(Button("Login").OnClick(onLogin)),
    ).Spacing(8)
}
```

For more configurable composition, define a value type with chainable methods
and `Build() View`:

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

Composition views are expanded automatically, so they can be used like built-in
views:

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

## Custom Views And Widgets

A custom `ui.WidgetView` is the bridge from declarative `ui.View` values to an
imperative `gui.Widget`. This example wraps a `gui.Label` and stores the latest
callback in the reconciler state:

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

For a fully custom widget, implement the `gui.Widget` behavior directly, usually
by embedding `gui.WidgetBase` and overriding measurement, painting, and snapshot
behavior as needed:

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

## Core Idea

goui is built around three layers:

```text
ui        declarative View layer for application code
gui       imperative Widget layer for control authors
platform  native windows, events, graphics, and typography
```

The MVP has validated the main design:

- `ui` gives application code a concise declarative API.
- `gui` gives custom control authors an imperative widget kernel with layout,
  painting, events, focus, lifecycle, signals, and semantic snapshots.
- `platform` uses native platform capabilities for windows, drawing, input, and
  text layout as much as practical.

**The result is a framework direction that is native to Go, understandable by**
**humans, and inspectable and controllable by AI.**

## Platform Direction

Current desktop backends:

| Platform | Window System | Graphics | Typography |
| --- | --- | --- | --- |
| Windows | Win32 | Direct2D | DirectWrite |
| macOS | Cocoa | OpenGL | Core Text |
| Linux | X11 | OpenGL | Pango |

goui aims to keep platform-specific work behind the `platform` layer while
preserving each system's strengths in rendering, text layout, and input.

The intended build model is still:

```bash
CGO_ENABLED=0 go build ./...
```

## AI and Automation

AI support is part of the architecture, not a later add-on.

The framework is designed so tools and AI agents can inspect semantic snapshots,
understand widget hierarchy and bounds, and drive applications through the same
event path used by real users.

## Status

The MVP is complete as an architecture validation. It includes platform events,
an imperative GUI layer, basic widgets, a minimal declarative `ui` layer, and
update scheduling through `ui.App.RequestUpdate`.

**The API is still unstable and goui is not ready for production use.**

## Roadmap

- Design a lightweight style and settings system.
- Continue platform validation for IME, clipboard, accessibility, and system
  settings.
- Expose a stable inspection and automation protocol.
- More platform support.

## License

MIT License.
