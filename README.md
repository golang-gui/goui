# goui

[中文文档](./doc/README_CN.md)

goui is a Go-native and AI-friendly declarative UI framework.



## Declarative UI

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

**The MVP shows that Go can express a clear declarative UI with ordinary values and chainable methods.**



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
update scheduling through `ui.Root.RequestUpdate`.

**The API is still unstable and goui is not ready for production use.**

## Roadmap

- Design a lightweight style and settings system.
- Continue platform validation for IME, clipboard, accessibility, and system
  settings.
- Expose a stable inspection and automation protocol.
- More platform support.

## License

MIT License.
