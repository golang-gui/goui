# 外部链接与文件打开

## 接口与边界

platform.Platform、gui.Application、ui.App 均提供：

```go
OpenURL(rawURL string) error
OpenPath(path string) error
```

OpenURL 请求系统注册的协议处理程序打开地址，包括 https、mailto、自定义协议及 file URL。OpenPath 使用默认应用打开文件或目录；目录通常由文件管理器处理。两者不猜测输入类型，不提供“显示父目录并选中文件”。不引入 Launcher、Options、目标应用选择、协议注册或独立生命周期。

输入使用 string：操作无需修改 URL 的组成部分，已有 net/url.URL 的调用者使用 String()。OpenURL 必须包含合法 scheme，拒绝空值、原始控制字符及非法百分号转义；不自动补 https、不重写 URL。OpenPath 接受原生路径，相对路径按调用时工作目录转绝对路径；不展开 ~、环境变量或 shell 表达式，不提前检查文件存在性，以原生打开结果为准。两者拒绝内嵌 NUL。

## 返回结果与线程

- nil 仅表示原生接口报告打开请求成功，不保证网页加载、外部应用显示或后续处理成功。
- 同步返回请求结果，不等待目标应用退出；不承诺原生调用无阻塞。
- platform/gui 保持线程亲和；ui 使用已有 Sync 在 GUI 线程执行并返回结果。退出后或排队请求因退出未执行，返回 ui.ErrAppStopped，而不是 nil。
- GUI/UI 保留失败，不自动日志、弹框或退出应用。错误反馈原则见 DesignConventions.md。
- 错误文本保留操作上下文和可用原生诊断；不承诺统一的无关联应用、权限等精细分类。原生接口没有提供原因时只返回一般失败。
- 打开可执行文件或特殊协议可能触发执行行为，应用应判断是否允许打开不可信输入；框架不绕过系统安全确认。

## 平台实现

| 平台 | 原生路径 |
|---|---|
| Windows | ShellExecuteExW，open 动作，SW_SHOWNORMAL；不经过命令解释器 |
| macOS | NSWorkspace.openURL:；URL 使用 NSURL.URLWithString:，文件路径使用 fileURLWithPath: |
| Linux | GIO g_app_info_launch_default_for_uri；原生文件名通过 GLib g_filename_to_uri 转义 |

- Windows 复用现有 shell32 SDK 包，检查 STA 初始化结果；SEE_MASK_FLAG_NO_UI 抑制原生错误弹窗，但不绕过安全提示。不申请进程句柄，不等待外部进程。
- macOS 遵守现有 AppKit 绑定方式，在 cocoa 层进行适配；临时对象由 autorelease pool 管理，原生 false 转为一般失败。
- Linux 将 GLib/GObject/GIO 原生绑定统一放在 `platform/linux/libs/glib/api_linux.go`，在 x11 层适配，不设置独立 gio 包或 URI 绑定文件。GError 信息复制到 Go 后释放，转换得到的 URI 复制后 g_free；临时输入使用 CStringTemp。加载失败返回不可用错误，不添加命令行 fallback。
- 共享输入检查在 platform/internal 中；GUI/UI 不重复解析，也不绕过各自依赖方向。
- 首版不承诺沙箱 Portal 集成、移动端文件共享或外部应用最终完成通知。

原生参考：[ShellExecuteExW](https://learn.microsoft.com/en-us/windows/win32/api/shellapi/nf-shellapi-shellexecuteexw)、[NSWorkspace](https://developer.apple.com/documentation/appkit/nsworkspace/open(_:))、[GIO](https://docs.gtk.org/gio/type_func.AppInfo.launch_default_for_uri.html)。

## 使用与验证

运行时命令在点击回调等位置执行，不在每次声明式 Build 中执行：

```go
if err := app.OpenURL("https://example.com"); err != nil {
    // 应用决定提示、复制地址、重试或忽略。
}
```

自动测试不启动外部应用：覆盖输入、路径转义、原生结构体 ABI、GError 释放、错误透传、UI 线程转交、退出时丢弃请求。运行 go test ./...、相关 race 测试和 Windows/macOS 交叉编译。独立 example/external_open 通过用户点击验证浏览器、目录、文件及自定义协议；三平台实际打开需要真机验证。

后续单独迁移 Clipboard、FileDialog 等旧接口；不为本切片批量修改既有错误语义。

## 本切片验证记录

- 三后端与 GUI/UI 接入已实现；Windows amd64、macOS arm64 全仓交叉编译通过（不是原生运行验证）。
- 新增功能测试、相关包 race 测试通过；自动测试未启动外部应用。
- `go test ./...` 仍有既有的 `platform/test.TestApplicationIdentity` 失败：X11 创建错误时 typed-nil 装入 Platform 接口。排除该用例后全仓测试通过；此问题未纳入本切片修改。
- 三平台真正打开目标的验证留给手动例程，不将“编译通过”视为桌面行为已验收。
