# 应用身份与原生图标

## 范围与状态

本切片接入平台应用身份、Windows 窗口类图标，并将 App ID 通过 GUI/UI 创建入口传入。
不增加 `SetIcon`、Options、运行时图标解码、构建命令或安装器。
应用提供资源；GOUI 完成原生连接，不保证每种桌面环境的每个位置都显示图标。

## 创建接口

```go
platform.NewPlatform(name, appId string) (platform.Platform, error)
gui.NewApplication(appId string) (gui.Application, error)
ui.Run(appId string, build func(ui.App) ui.RootView) error
```

调用顺序为 `ui.Run -> gui.NewApplication -> platform.NewPlatform -> 原生后端`。
UI 不导入 platform，不在 RootView、Window 或重复执行的 build 中设置应用身份。

```go
err := ui.Run("com.example.Editor", func(app ui.App) ui.RootView {
    return ui.Root().Windows(/* 原有窗口声明 */)
})
```

约定：

- ID 是创建时确定的、区分大小写的应用身份，不是窗口 ID、标题或显示名称。
- 空字符串表示未显式指定，适用于开发运行和不需要完整桌面关联的程序。
- 显式 ID 至少包含两个点分段，每段以 ASCII 字母开头，其余字符允许 ASCII 字母、数字、连字符；总长度不超过 128 字节。建议反向域名形式，不包含版本号。
- ID 格式只在 `platform.NewPlatform` 的私有 `checkAppId` 中校验；GUI 与具体后端不重复校验，不提供公共校验函数。不自动修正大小写、空白、非法字符；无效输入返回错误。
- 平台按进程单例初始化一次，UI 的 Run 仍只允许调用一次。GUI 不保存 App ID，也不检查重复创建的身份：`App` 已存在就直接返回，首次创建才将 appId 传给平台。X11 和 Win32 保留身份冲突保护；Darwin 不围绕多次创建设计身份切换或冲突检查，获取已有单例直接返回，Bundle 检查只发生在首次初始化。
- 没有 SetApplicationID、隐式环境变量覆盖或图标 setter。
- 创建失败时返回 `nil, err`。具体后端的指针返回值在转换为 `Platform` 接口前先检查错误，避免将带类型的 nil 指针作为非 nil 接口返回。

## Linux X11

在普通顶层窗口首次 Map 前设置 `WM_CLASS`：

| 字段 | 值 |
|---|---|
| res_name | `os.Executable()` 的文件名 |
| res_class | 显式 appId；为空时回退到可执行文件名 |

窗口标题改变不改变身份。Popup、内部辅助窗口不作为独立桌面入口处理。
使用原生 XClassHint/XSetClassHint 薄绑定；临时字符串由调用方持有，Xlib 复制后不长期保留。
不为这个属性增加 XSync 等额外同步；属性设置和随后 Map 在同一连接上有序提交。

应用负责安装匹配的桌面入口，例如 `com.example.Editor.desktop`：

```ini
[Desktop Entry]
Type=Application
Name=Example Editor
Exec=example-editor
Icon=com.example.Editor
StartupWMClass=com.example.Editor
Terminal=false
```

`Exec` 的程序必须可执行并可定位；也可以使用正确转义的安装绝对路径。
桌面入口通常安装至 `$XDG_DATA_HOME/applications`（默认 `~/.local/share/applications`），
图标安装至对应数据目录的 `icons/hicolor/<尺寸>/apps/com.example.Editor.png` 等位置。
运行时不创建这些文件，不扫描 `.desktop`，不自动设置 `_NET_WM_ICON`。
WM_CLASS 是身份而非图标数据；部分窗口管理器的原生标题栏仍可能不显示应用图标。

验证：运行已编译的程序，使用 `xprop WM_CLASS` 检查两个字符串，再从已安装桌面入口启动并检查桌面关联。
`go run` 的临时可执行文件名不稳定；显式 ID 可以稳定 class，但不会替代桌面资源安装。

## Windows

### 进程身份

非空 appId 在创建任何原生窗口（包括辅助窗口）前通过
`SetCurrentProcessExplicitAppUserModelID` 设置。失败返回原生错误；空 ID 不调用该函数。
Shell API 绑定统一放在 `platform/windows/sdk/shell/api_windows.go`，复用已有 shell32 库与符号调用方式；winapi 只保留窗口类、HICON 等 User32 绑定。
不改变现有窗口类名称和 Chrome 样式。

安装器如创建快捷方式，应设置相同的 `System.AppUserModel.ID`。
进程 ID 用于任务栏关联，不用于选择 EXE 图标资源，也不替代快捷方式的配置。

### 窗口类图标

参考 GTK4 的 RegisterGdkClass：从当前 EXE 调用 `ExtractIconExW(path, 0, ...)`
提取第一个图标组的大小图标，填入 `WNDCLASSEX.Icon/IconSm`。
索引 0 不是资源 ID 0，也不要求用户的资源 ID 必须是 1。
同一窗口类只加载一次，所有实例共用。没有图标时保持系统默认行为，不阻止创建窗口。

提取的 HICON 归平台窗口类资源所有：

- 不由某个 Window 销毁，不在注册后立即释放。
- 注册失败清理已加载图标和创建的辅助资源。
- Platform.Destroy 在窗口全部销毁后注销类，再释放不同的 HICON 各一次。
- 若类仍被窗口使用、无法注销，保留图标，允许窗口销毁后再次清理。
- 同一 HICON 同时用于两个尺寸时不重复释放。
- 平台销毁后不可重新使用；不扩展成可重复启动 GUI 的生命周期。

### 手工嵌入资源

已有 `.syso` 可直接使用。没有时可使用资源编译器，例如：

```rc
1 ICON "app.ico"
```

这里的 1 只是示例编号，不是 GOUI 要求。ICO 应提供多个尺寸。
将目标架构的资源对象放在 main 包目录，命名例如 `app_windows_amd64.syso`：

```bash
x86_64-w64-mingw32-windres app.rc -O coff -o app_windows_amd64.syso
CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -o editor.exe .
```

按包构建（`.` 或 `./cmd/editor`），不要使用 `go build main.go`；后者不会按普通包方式收集 .syso 和其他生成文件。
已有资源对象时不要重复生成另一份冲突资源。EXE 图标、窗口图标和固定快捷方式图标分别验证。
本轮不处理安装器、应用快捷方式创建或签名。

## macOS

真实应用身份和图标来自应用 Bundle，不通过 NSWindow 类名或运行时 Dock 图标 setter 模拟。
显式 appId 与 MainBundle 的标识不一致时，在 NSApplication 启动前返回错误。
标记为 APPL 的 Bundle 缺少标识，同样不能满足显式 ID。
空 ID 使用原生事实；裸可执行文件没有 Bundle 身份时可以运行，但不具备打包应用的图标保证。

手工打包目录：

```text
Editor.app/
  Contents/
    Info.plist
    MacOS/
      editor
    Resources/
      AppIcon.icns
```

最小 Info.plist 示例（与代码的 `com.example.Editor` 保持一致）：

```xml
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>CFBundleIdentifier</key><string>com.example.Editor</string>
    <key>CFBundleName</key><string>Editor</string>
    <key>CFBundleDisplayName</key><string>Example Editor</string>
    <key>CFBundleExecutable</key><string>editor</string>
    <key>CFBundlePackageType</key><string>APPL</string>
    <key>CFBundleShortVersionString</key><string>1.0.0</string>
    <key>CFBundleVersion</key><string>1</string>
    <key>CFBundleIconFile</key><string>AppIcon.icns</string>
</dict>
</plist>
```

可在 macOS 上用 iconutil 将标准 .iconset 转成 .icns，或直接提供现成 .icns。
将可执行文件放入 Contents/MacOS 并保留执行权限；用 `plutil -lint` 检查 plist，
再用 `open Editor.app` 验证 Finder/Dock 显示。生成目录不等于完成签名、公证和分发。
当前 `example/plat_test` 的 ID 是 `org.golang-gui.PlatformDemo`，打包它时相应替换示例 ID。

## 验证与后续

自动化覆盖 ID 语法、GUI 直接返回已有单例、首次创建向平台传参、X11/Win32 单例冲突、UI 转发顺序、X11 属性与标题独立性、空 ID 回退、
Windows 原生图标提取/类注册/资源释放、macOS Bundle 一致性判断。
运行 `go test ./...`；交叉编译只验证构建，不代替 Windows/macOS 真机执行。

原生身份切片不让运行时库负责生成 .syso、安装 .desktop、写 Info.plist 或转换图标。
这些能力由后续独立的 [构建打包工具](DesignBuildPackaging.md) 承载；运行时契约不变。

## 参考

- [GTK4 GtkApplication](https://docs.gtk.org/gtk4/class.Application.html)
- [GTK4 Windows 窗口类图标](https://github.com/GNOME/gtk/blob/main/gdk/win32/gdksurface-win32.c)
- [Desktop Entry 规范](https://specifications.freedesktop.org/desktop-entry/latest-single/)
- [ExtractIconExW](https://learn.microsoft.com/en-us/windows/win32/api/shellapi/nf-shellapi-extracticonexw)
- [AppUserModelID](https://learn.microsoft.com/en-us/windows/win32/shell/appids)
- [Apple Bundle 结构](https://developer.apple.com/library/archive/documentation/CoreFoundation/Conceptual/CFBundles/BundleTypes/BundleTypes.html)
