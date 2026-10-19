<p align="center">
  <img src="logo/goui-logo.svg" width="400" alt="GOUI">
</p>

<h1 align="center">无需 GCC，用 Go 构建轻量、现代的桌面应用</h1>

<p align="center">
  <a href="../README.md">English</a>
</p>

GOUI 是纯Go实现的原生 GUI 框架。用 Go 编写界面与业务逻辑，非套壳浏览器引擎，无需 C/C++ 构建工具链。

当前版本已具备开发小型桌面应用所需的基础机制和控件，公共 API 仍在完善。

## 相关项目

| 项目 | 用途 |
| --- | --- |
| [goui-sourceview](https://github.com/golang-gui/goui-sourceview) | 代码编辑器扩展，支持语法高亮、补全、括号匹配和查找替换。 |
| [applesdk](https://github.com/golang-gui/applesdk) | 纯 Go 的 macOS SDK 绑定，提供 StoreKit、iCloud 和 CloudKit 等服务。 |

## 体验 Demo

[Demo](../example/demo) 集中展示基础控件、文本编辑、布局、数据视图、可拖出的
标签页、覆盖层、图形、动画和系统集成，支持切换亮暗模式、主题色和八种界面语言。

| 平台 | Demo 下载 |
| --- | --- |
| Windows x64 | [下载 EXE](https://github.com/golang-gui/goui/releases/download/v0.1.0/goui-demo-v0.1.0-windows-amd64.exe) |
| macOS Apple Silicon | [下载 .app ZIP](https://github.com/golang-gui/goui/releases/download/v0.1.0/goui-demo-v0.1.0-darwin-arm64.zip) |
| Linux x64 / X11 | [下载可执行文件](https://github.com/golang-gui/goui/releases/download/v0.1.0/goui-demo-v0.1.0-linux-amd64) |

详见 [v0.1.0 发布说明](https://github.com/golang-gui/goui/releases/tag/v0.1.0)与
[SHA-256 校验和](https://github.com/golang-gui/goui/releases/download/v0.1.0/SHA256SUMS)。
Linux 下载后先执行 `chmod +x goui-demo-v0.1.0-linux-amd64`，再启动程序。
macOS 解压后打开 `GOUI Demo.app`；该应用包已使用 Developer ID 签名并通过 Apple 公证，已装订公证票据。

Windows

<img src="images/demo-windows.png" alt="Windows 上的 GOUI Demo">

macOS

<img src="images/demo-macOS.png" alt="macOS 上的 GOUI Demo">

Linux

<img src="images/demo-linux.png" alt="Linux 上的 GOUI Demo">

也可以在仓库根目录直接运行当前源码：

```sh
CGO_ENABLED=0 go run ./example/demo
```

## 为什么选择 GOUI？

- **No cgo，无需 GCC。** 使用 `CGO_ENABLED=0` 和 Go 工具链构建，支持已适配目标的交叉编译。原生 API 在运行时绑定，构建应用不需要 C/C++ 编译器。
- **单二进制交付。** 图标等资源可通过 Go 的 `embed` 一同嵌入。Windows EXE、Linux 二进制可以直接运行；macOS 可将二进制包装为 `.app`，补齐应用身份、图标和签名。
- **现代外观，原生集成。** Modern 样式提供多平台一致的亮暗外观和主题色，窗口装饰、字体、文字排版、输入法、剪贴板和文件对话框利用系统能力。
- **AI友好。** 语义快照描述控件树；`gui/dev` 提供窗口和Popover 检查、输入事件派发与文本提交，用于调试及 AI 自动化。

单二进制交付**不等于没有系统依赖**：GOUI 在运行时加载系统自带原生库。具体要求见[平台与依赖](#平台与依赖)。

### 站在巨人肩膀上

GOUI 吸收成熟框架的设计经验，而不是重新发明所有机制：

参考[GTK4](https://docs.gtk.org/gtk4/) 的控件与事件控制器、[Flutter](https://docs.flutter.dev/ui/layout/constraints) 的约束布局、[Qt](https://doc.qt.io/) 和 [Avalonia](https://docs.avaloniaui.net/) 的桌面集成，以及 [SwiftUI](https://developer.apple.com/documentation/swiftui) 的声明式组合。最终服从 Go 风格和实际需求，不照搬任何一个框架。

底层利用 Direct2D/DirectWrite、Core Text、Pango/Cairo 等原生能力，以及项目维护的[NanoVG Go 分支](https://github.com/golang-gui/nanovgo)。SVG 使用项目维护的[oksvg 分支](https://github.com/golang-gui/oksvg)。这些成熟基础被收敛在清晰的平台、控件和声明式边界内。

## 程序大小与内存占用

GOUI 是原生GUI框架，资源占用较小，但具体可执行文件大小与运行内存取决于应用内容和绘制后端。

### 可执行文件大小

使用 Go 1.25.8、`CGO_ENABLED=0`、`-trimpath`、`-ldflags="-s -w"` 构建，Windows 另加 `-H=windowsgui`。

| 目标 | 最小应用 | Demo |
| --- | ---: | ---: |
| Windows amd64 | 4.58 MiB | 8.56 MiB |
| macOS arm64 | 4.75 MiB | 8.26 MiB |
| Linux amd64 | 4.46 MiB | 8.43 MiB |

完整 Demo 包含 Logo、SVG 图标、八语言资源和九个功能页面；Windows 包含生成的图标及版本资源。数据来自开发构建，并非已经发布的下载产物；不包含 `.app`元数据、安装包／压缩包开销或系统库。

### 进程内存占用

| 应用 | RSS | PSS |
| --- | ---: | ---: |
| 最小应用 | 58.8 MiB | 28.6 MiB |
| Demo | 78.5 MiB | 46.2 MiB |

RSS 包含驻留的共享库页面，PSS 按进程分摊共享页面；由Linux `/proc/<pid>/smaps`汇总，仅供参考，其他平台可能会有出入。

## 当前可以使用的能力

- **基础与交互控件：** 按钮、标签、图片／图标、单行输入、多行文本编辑、菜单、下拉列表、复选／单选／切换按钮、Switch、长条和圆形进度指示。
- **布局与数据：** 横纵线性盒、换行布局、基线对齐、滚动、Overlay、SplitView、虚拟化列表、TreeView，以及支持自定义单元格、排序和调整列宽的 TableView。
- **多标签工作区：** 关闭请求、右键菜单、拖动排序、跨窗口移交与拖出创建窗口。
- **输入：** Unicode 编辑、CJK 输入法、选区、撤销／重做、剪贴板、快捷键、Tab 焦点导航，以及应用内和跨应用拖放。
- **外观与绘图：** Modern 亮暗样式、主题色和语义色、SVG 与跟随样式的图标、HiDPI、透明窗口、阴影和自定义控件绘制。
- **应用设施：** GUI 线程定时器、帧驱动数值动画、系统设置、八语言框架资源、文件对话框和访问凭据，以及通过系统默认应用打开链接／文件。

核心控件位于 `gui`，声明式绑定位于 `ui`，配套控件位于 `widgets`，对应声明式绑定位于 `widgets/ui`。

## 使用 AI 编写应用

[GOUI 应用开发 Skill](../skills/goui-app/SKILL.md) 帮助你的 AI 正确使用 GOUI 的接口、
状态、布局、样式和资源，并完成构建与调试。只需告诉 AI：

> 请按照 https://github.com/golang-gui/goui/blob/main/skills/INSTALL.md 的说明，安装 GOUI 应用开发 Skill。

AI 会按[安装说明](../skills/INSTALL.md)使用当前工具的原生技能机制完成安装，
不需要 GOUI 为不同 Agent 编写适配器，也不需要你手动复制文件。
不支持 Skill 的工具可以直接阅读开发指南，但不等同于原生安装。
安装 Skill 不会安装或升级应用的 GOUI 依赖。

需要接入或定制代码编辑器时，可使用由 SourceView 仓库独立维护的
[SourceView 应用开发 Skill](https://github.com/golang-gui/goui-sourceview/blob/main/skills/goui-sourceview-app/SKILL.md)，
安装方式见其[安装说明](https://github.com/golang-gui/goui-sourceview/blob/main/skills/INSTALL.md)。

需要复用业务界面或自定义控件时，参阅[自定义 Widget 与 View](CustomWidgetView.md)，
包含两层的组合、扩展、实现方式及生命周期示例。

## 快速上手

需要 **Go 1.24 或更高版本**，以及支持的桌面环境。创建模块并添加 GOUI：

```sh
mkdir hello-goui
cd hello-goui
go mod init example.com/hello-goui
go get github.com/golang-gui/goui@latest
```

保存为 `main.go`：

```go
package main

import (
    "fmt"
    "log"

    "github.com/golang-gui/goui/theme/modern"
    "github.com/golang-gui/goui/ui"
)

func main() {
    count := ui.MakeState(0)
    err := ui.Run("com.example.Hello", func(app ui.App) ui.RootView {
        return ui.Root().
            StyleSheet(modern.FromSettings(app.Settings())). // 使用“现代”主题
            Windows(
                ui.Window("main").
                    Title("Hello GOUI").
                    Size(480, 240).
                    Content(
                        ui.VBox(
                            ui.Label(fmt.Sprintf("Clicked %d times", count.Get())),
                            ui.Button("Click me").OnClick(func() {
                                // count.Set(count.Get() + 1) 的更优用法：
                                count.Update(func(old int) int {
                                    return old + 1
                                })
                            }),
                        ).Spacing(12).Padding(20),
                    ),
            )
    })
    if err != nil {
        log.Fatal(err)
    }
}
```

在当前平台构建并运行：

```sh
CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o hello-goui .
./hello-goui
```

Windows PowerShell 使用 `$env:CGO_ENABLED='0'`，然后执行：

`go build -trimpath -ldflags="-s -w -H=windowsgui" -o hello-goui.exe .`。

View 是普通 Go 值；`State.Set` 会请求合并后的重建。系统设置变化也会自动重建此例并更新 Modern 样式。复杂应用可以使用函数或自定义 View 类型组合界面，也可以通过 `gui.WidgetBase` 实现控件。

## 平台与依赖

| 平台／当前发布目标 | 窗口系统 | 绘图后端 | 文字排版 |
| --- | --- | --- | --- |
| Windows / amd64 | Win32 | Direct2D、OpenGL、Software | DirectWrite |
| macOS / arm64 | AppKit | OpenGL、Software | Core Text |
| Linux / amd64 | X11 | OpenGL、Software | Pango / Cairo |

Windows、macOS 使用操作系统提供的原生框架。Linux 当前后端需要 X11 显示服务；Wayland 会话需要 XWayland，具体窗口行为仍需在目标桌面验证。运行库包括X11/Xext/Xrender/Xrandr/Xcursor、GL/GLX、GLib/GIO、Fontconfig 和 Pango/Cairo；
系统设置与原生文件对话框使用 GTK，不需要开发头文件或 C 编译器。
Linux Integrated 窗口装饰还需要合成器，否则可回退原生装饰。

可通过 `GOUI_PLAT_PAINTER=software` 或 `opengl` 选择绘图后端，Windows 还支持`direct2d`。平台详细说明见 Demo 使用说明。

### 构建与打包

普通 `go build` 即可构建应用。可选的 [`cmd/goui`](../cmd/goui/README.md) 从`goui.toml` 生成平台资源并协助打包：Windows 图标与版本资源，macOSInfo.plist／ICNS／`.app` 与显式签名，以及 Linux desktop／图标资源。
这些步骤不需要 GCC；macOS 签名使用系统 `codesign` 工具。

例如，构建 Demo 并生成平台资源：

```sh
CGO_ENABLED=0 go run ./cmd/goui build -config example/demo/goui.toml -ldflags="-s -w"
```

## 环境变量

通常无需设置。以下变量用于启动配置、调试或兼容性验证，请在启动前设置，修改后重新启动应用；它们不是运行时切换接口。

| 变量 | 用途与取值 | 默认行为 |
| --- | --- | --- |
| `GOUI_PLAT_SCALE` | 覆盖界面的逻辑单位到物理像素的缩放比例，支持分数，如 `1`、`1.5`、`2` | 按平台获取系统缩放；未设置、解析失败或非正数时不覆盖 |
| `GOUI_PLAT_PAINTER` | 选择绘图后端：`opengl`、`software`；Windows 还支持 `direct2d` | 使用平台默认后端及其初始化降级策略 |

`GOUI_PLAT_SCALE=2` 表示 1 DIP 对应 2 个物理像素，它不修改操作系统的显示设置，也不能代替真实设备上的 HiDPI 验证。

绘图后端名称使用小写。未设置或名称不识别时使用默认选择：

- **Windows：** 默认 Direct2D，初始化失败时依次尝试 OpenGL、Software。

  指定 `direct2d` 也遵循这个顺序；显式指定 `opengl` 时不自动降级，初始化失败会返回错误。

- **Linux / macOS：** 默认 OpenGL，初始化失败时降级到 Software；指定 `opengl` 也遵循此策略。

- 指定 `software` 时直接使用软件绘制。这些降级发生在后端初始化时，不是运行期间自动切换。

Linux / X11 还识别以下兼容变量：

| 变量 | 用途 |
| --- | --- |
| `GDK_SCALE` | 指定缩放比例，如 `1`、`2`；优先级为 `GOUI_PLAT_SCALE` > `GDK_SCALE` > 桌面 DPI 设置 |
| `XCURSOR_THEME` | 指定已安装的鼠标光标主题名称，覆盖桌面光标主题设置 |
| `XCURSOR_SIZE` | 指定鼠标光标大小，使用正整数像素值，如 `24`、`32`，覆盖桌面光标大小设置 |

例如，在仓库根目录用 Bash 启动 2x 缩放的软件绘制 Demo：

```sh
CGO_ENABLED=0 GOUI_PLAT_PAINTER=software GOUI_PLAT_SCALE=2 go run ./example/demo
```

Windows PowerShell 示例：

```powershell
$env:CGO_ENABLED = '0'
$env:GOUI_PLAT_PAINTER = 'direct2d'
$env:GOUI_PLAT_SCALE = '1.5'
go run ./example/demo
```

Bash 示例中的变量仅对该命令生效；PowerShell 示例会保留在当前终端会话中。
恢复自动选择时，删除相应变量并重启应用，例如在 PowerShell 中执行：
`Remove-Item Env:GOUI_PLAT_PAINTER, Env:GOUI_PLAT_SCALE -ErrorAction SilentlyContinue`。

### 可选开发工具

`GOUI_DEV_ADDR` 是 [`gui/dev`](../gui/dev) 开发适配层的监听地址，不属于平台绘制配置。
应用需先添加 `import _ "github.com/golang-gui/goui/gui/dev"`，再设置此变量才会启用；
未导入该包或未设置变量时不会启动服务。例如运行已导入该包的应用：

```sh
GOUI_DEV_ADDR=127.0.0.1:8888 ./your-app
```

服务提供语义快照、输入事件派发和文本提交。请仅用于本地开发，不建议在正式交付中启用。

## 小而清晰的分层

```text
ui        声明式 View 与显式更新协调
gui       控件、布局、绘制、事件、焦点、信号与语义快照
platform  原生窗口、事件循环、输入、绘图与文字排版
```

按需选择使用的层次。`animation`、`layout`、`style`、`icon`、`mlang` 提供专注的能力，`gui/dev` 是可选的调试适配层。GOUI 希望成为实用的 GUI 工具包，而不是浏览器或无所不包的平台 SDK。

## 后续路线

- 增加触摸输入与手势，同时保持鼠标、键盘体验。
- 适配更多平台、架构与窗口系统。
- 增加更多用途控件与可复用应用组件。
- 持续优化性能和内存占用，以可复现测量支持性能结论。

## 许可证与致谢

GOUI 使用 [MIT 许可证](../LICENSE)，依赖保留各自许可证。

[Logo](logo/goui-mark.svg) 保留 Go gopher / Renee French / CC BY 4.0 的归属说明。
