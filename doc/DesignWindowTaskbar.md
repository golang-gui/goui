# 窗口任务栏呈现

## 范围与接口

普通 Window 可以用于完整交互的辅助窗口，不为此改造 Popover，亦不引入 Popup
窗口类别。platform/common 与 gui 的 WindowOptions 增加创建期 `SkipTaskbar bool`，
UI 使用 `ui.Window(id).SkipTaskbar(true)`；零值保留现行行为。挂载后改变创建选项
遵循既有错误契约，需要新窗口 ID。

SkipTaskbar 只是请求省略该窗口的独立任务栏条目，与 Chrome、Transparent、
输入焦点、置顶、父子关系、模态、关闭策略和定位互相独立。窗口仍登记在
Application.Windows 中，仍参与最后窗口关闭判断；不新增状态查询或运行时 setter。
Linux Integrated 的透明 None 适配和 Native 回退均保留此选项。

## 三平台边界

- Windows x64：CreateWindowEx 的扩展样式增加 WS_EX_TOOLWINDOW，不设置
  WS_EX_NOACTIVATE/WS_EX_TOPMOST/WS_EX_APPWINDOW。沿用普通 Window 的隐藏创建、
  Windows 10/11 Chrome 与 Show 路径。系统同时从 Alt+Tab 排除工具窗口，并改变
  Native 标题栏外观/高度；这是原生副作用，不承诺独立控制任务切换器。
  Native 创建尺寸与最小尺寸将扩展样式传给 AdjustWindowRectExForDpi，仍以客户区
  DIP 为准。Integrated/None 不采用 Native 标题栏计算，不修改 Painter。
- Linux/X11：Platform 初始化并持有 _NET_WM_STATE_SKIP_TASKBAR atom，首次 Map 前
  将其作为 _NET_WM_STATE 的初始 ATOM/32 属性。Xlib 数据按原生 unsigned long
  存储，而非紧密 uint32；窗口保持 WM 管理，不设置 override_redirect 或虚假
  transient parent，不增加窗口类型/skip-pager/置顶策略。WM 可能忽略提示。
- macOS/AppKit：Dock 的普通图标归属于应用，多个普通 NSWindow 不各自产生
  独立应用图标，因此此偏好无需 per-window 改动。保持普通可成为 key 的窗口类、
  collection behavior、应用 activation policy 与最小化语义。不要切换到
  accessory/LSUIElement 隐藏整个应用，亦不把窗口改成不接收输入的 Popup。
  原生最小化 Dock tile 仍可出现；本 API 不承诺阻止它。

## 定位与使用

不增加 WindowOptions.Position。创建与 Show 分离，已有 SetPosition 可在显示前
调用；仍以客户区原点和 DIP 为准。relativeTo 为窗口时相对该客户区；nil 相对
当前工作区左上角。请求不建立跟随/归属关系，也不主动退避；系统仍可限制位置。
未请求位置则保持普通窗口现行默认行为，不因为 SkipTaskbar 自动居中。
参考窗口若正处于尚未完成的移动/首次 WM 摆放，采样可能得到旧原点；不承诺
跟随其待完成的位置请求。窗口例程先显示参考窗口，再放置隐藏辅助窗口。

```go
win, err := app.NewWindow(&gui.WindowOptions{
    Size: geometry.Size{Width: 480, Height: 320},
    Chrome: gui.WindowChromeNone,
    SkipTaskbar: true,
})
if err != nil { return err }
if err := win.SetPosition(mainWindow, geometry.Point{X: 80, Y: 60}); err != nil {
    win.Destroy()
    return err
}
return win.Show()
```

BorderBox 仍是后续单子容器切片；当前不顺带实现它、扩充 Popover 或改变默认摆放。

## 依据与取舍

- [Microsoft 扩展窗口样式](https://learn.microsoft.com/en-us/windows/win32/winmsg/extended-window-styles)：
  TOOLWINDOW、NOACTIVATE、APPWINDOW 的契约互相独立。
- [EWMH 1.5 窗口属性](https://specifications.freedesktop.org/wm/1.5/ar01s05.html)：
  withdrawn 窗口直接提供初始状态；SKIP_TASKBAR 是 WM 提示。
- [AppKit activation policy](https://developer.apple.com/documentation/appkit/nsapplication/activationpolicy-swift.enum)：
  应用级呈现不能用作单个窗口的隐藏策略。
- Avalonia 11.3.9 [Windows 源码](https://github.com/AvaloniaUI/Avalonia/blob/11.3.9/src/Windows/Avalonia.Win32/WindowImpl.cs)
  使用隐藏 owner 抑制任务栏条目；[X11 源码](https://github.com/AvaloniaUI/Avalonia/blob/11.3.9/src/Avalonia.X11/X11Window.cs)
  使用 SKIP_TASKBAR。GOUI 采用已确认的 Windows TOOLWINDOW 路径，不为了保留
  普通标题栏而增加隐式 owner 和额外资源生命周期；X11 采用同类原生提示。

以上是文档/源码依据，不替代各桌面的运行验收。

## 验证

包内测试覆盖 GUI 默认值与 Chrome 回退透传、普通窗口登记、UI 创建/重复重建/
拒绝运行时变更，以及 Windows 扩展样式和 Native 尺寸转换。

独立程序 `tests/window/gui/taskbar` 用中文说明任务栏/Alt+Tab、显示前定位、真实
文本输入、点击、移动/缩放、最小化/恢复和关闭。Linux/Windows 通过原生绑定
观察初始状态/样式；macOS 不用缓存选项伪造任务栏观察，需真实 Dock 验收。
`-auto` 仅断言原生初始化与位置，不覆盖输入和系统呈现。

### 2026-10-13 执行记录

- Go 1.25.8 / Linux amd64：`go test -count=1 -json ./...` 最终 2466 通过、
  15 个既有跳过、0 失败；`go test -race ./gui ./ui ./platform/linux/x11` 通过。
- Windows amd64 / macOS arm64：`CGO_ENABLED=0 go build ./...`、有关包测试目标
  及独立窗口程序编译通过。Windows 原生扩展样式/尺寸两个测试，以及两平台
  GUI/UI 新增 SkipTaskbar 测试原生执行通过；传输产物 SHA-256 已核对。
- 本机 Ubuntu 22.04.5 / X11 / Xfwm4 4.16.1：Software 与 Mesa 23.2.1 llvmpipe
  OpenGL、应用 1x/2x 的指定组合通过原生提示/WM 管理/定位断言。Software 1x
  None 和 OpenGL 1x Integrated 的真实输入与点击通过；移动/缩放、重新定位和
  提示保留按实际操作观察。任务栏只列普通窗口，不列辅助窗口。
- 远端 Windows 10 19045 x64 / RDP Session 1 / Direct2D / 应用 1x：None 的
  实际键盘输入/点击、标题拖动、边缘缩放、重新定位、最小化/恢复与退出通过。
  任务栏与 Alt+Tab 均只列普通窗口，不列辅助窗口。None/Native/Integrated 三组
  `-auto` 均退出 0，原生样式、显示前客户区创建尺寸（包括工具标题栏）与显示后
  客户区定位断言通过；不是仅检查进程启动。
- 远端 macOS 15.7.7 arm64 / OpenGL / 应用 1x：None 的真实输入/点击、移动、
  边缘缩放、重新定位以及经 Dock/主窗口重新显示后保留编辑内容通过；应用 Dock
  图标没有因为创建辅助窗口而隐藏。Native/Integrated 的 `-auto` 定位通过、
  退出 0。未承诺最小化窗口 tile 的具体呈现或精确拖动像素。
- 普通 `go vet ./gui ./ui` 仍有既有非 keyed struct literal 告警，Windows win32
  普通 vet 仍有既有 unsafe.Pointer 告警；本轮未修改相关代码。
  平台 common/X11/新例程的普通 vet、GUI/UI 关闭 composites 项后的 vet、
  Win32 关闭 unsafeptr 项后的 vet 通过，不能写成完整 vet 通过。
- Windows 11 真机、其他 X11 WM、Wayland、混合系统 DPI、远端其他绘制后端未验收。
  应用缩放覆盖不等于真实系统 HiDPI 覆盖。

首轮例程同时移动隐藏参考窗再立即定位辅助窗，在 X11 得到旧参考原点，
`taskbar -auto -chrome=none/native/integrated` 报 `{440,-1}` 而非 `{480,80}`。
这与“采样实际原点一次”契约相符：例程改为先显示参考窗，再请求隐藏辅助窗位置，
不改 SetPosition、不在后端增加同步/重试或按旧请求补偿；最终断言仍为明确坐标。

远端工具问题另作边界记录：Windows 的 remote-shell 直接二进制 stdin 传输阻塞，
含固定长度读取的重试仍未完成；改用临时局域网 HTTP 传输并核对 hash。
Windows `-c` 实测经 cmd 解析，与连接信息的 PowerShell 标记不一致，显式使用
PowerShell 脚本/原生命令避免嵌套解析。RDP 向 Run 长命令键入出现前缀丢失，
返回 ok 不等于命令执行；改用本轮临时 Interactive/Limited 任务启动验证程序。
以上不是 GOUI 兼容处理；临时任务和 HTTP 服务均已删除/停止，桌面连接随后释放。

全仓旧测试仍可能写剪贴板、创建原生资源或更新已有输出图片；本轮未重写这些
测试或把桌面跳过算通过。窗口例程资源已关闭，另清理了本轮误用 XDestroyWindow
后剩余的 Software 验证进程；没有关闭用户原有应用或修改系统设置。

日志、截图、二进制和完整测试 JSON 保留在 `/tmp/goui-taskbar.lKvqVK`；本轮提交
包含代码、回归用例与本设计文档，用户原有 README 修改及其他未跟踪内容保持不动。
