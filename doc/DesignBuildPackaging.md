# GOUI 构建与打包工具

## 定位与边界

`cmd/goui` 是可选命令，不是运行时服务。复用 Go 构建工具、GOUI 的静态 SVG
渲染和原生应用资源格式，不改变 platform/gui/ui 的接口或应用生命周期。
所有配置路径相对 `goui.toml` 所在目录；不执行配置中的脚本，不自动安装工具。

- `build` 生成构建所需资源，再执行 `go build`；三平台均输出原始二进制。
- `generate` 单独生成资源，Windows 的 syso 写入 main 包目录以参与普通 Go 构建。
- `package` 消费已构建的二进制，不隐式调用 build。
- `doctor` 检查配置、图标、Go 工具及 main 包；不宣称目标机器的运行库已验证。
- Windows EXE 自含图标与版本，可以直接分发，不提供 `package -format exe`。
- macOS `.app` 属于 package；裸 Mach-O 用于调试，不保证 Finder/Dock 应用身份。
- Linux `dir` 是安装目录树，提供 ELF、desktop、hicolor 图标。不把 ELF 文件图标、
  未安装的桌面集成或原生窗口图标当作跨桌面保证。

首个切片实现 Windows EXE、macOS app、Linux dir；DEB/RPM 后续按实际需求
接入 nFPM。暂不实现归档格式、AppImage、公证、上传、自动安装。
文件访问切片增补明确配置的 macOS 沙箱 Entitlements 与原生签名，详见
[文件访问设计](DesignFileAccess.md)；不扩展为通用权限系统。
不随包分发 GOUI 动态加载的系统库；用户仍须提供支持的桌面系统环境。

## 配置

```toml
# 必须和 gui.NewApplication / ui.Run 的参数一致。
id = "com.example.Notes"
name = "便笺"
executable = "notes"
main = "./cmd/notes" # 默认 "."；必须是单个 main 包，不接受源文件列表
version = "0.1.0"   # 首版严格采用三个数字段
build = 1           # 1..9999，作为 Windows 版本第四段 / CFBundleVersion
icon = "assets/app.svg" # SVG 静态子集或 PNG，保留原色、透明和比例
# 可选，生成普通 Go 常量；应用显式导入，不修改 main.go。
# metadata = "./internal/appinfo"

[windows]
console = false
# icon = "assets/app.ico" # 可选原生资源覆盖

[macos]
# icon = "assets/app.icns"
# sandbox = true
# user_files = "read-write" # 或 read-only
# persist_access = true
# signing_identity = "Apple Development: ..." # 开发可显式用 "-"

[linux]
categories = ["Utility"]
```

拒绝未知字段、无效 ID、路径型 executable、控制字符、无效版本与目标。
name 是显示名称，可以是中文；executable 是可移植的 ASCII 文件名。
不支持环境变量插值、隐式继承或自由 plist/manifest 注入。
ID 校验与 GOUI 平台运行时一致（最长 128 字节、各段以字母开头、无下划线）。
build 采用 Apple CFBundleVersion 的正数、最多四位限制，而不是放任生成无效 plist。
metadata 生成 `appinfo.ID/Name/Version/Build`；包名固定 appinfo。
ID 一致性仍由调用方保证，工具不扫描或重写应用的 Go 调用。

## 使用

```sh
go install github.com/golang-gui/goui/cmd/goui@<version>
goui doctor -target windows/amd64
goui generate -target windows/amd64
goui build -target windows/amd64
goui build -target darwin/arm64
goui package -target darwin/arm64 -format app
goui build -target linux/amd64
goui package -target linux/amd64 -format dir
```

共同参数 `-config goui.toml`、`-target OS/ARCH`。默认目标是宿主；支持
windows/amd64、darwin/arm64、linux/amd64、linux/arm64。`build -o` 指定二进制；
`package -binary` 指定输入，`-o` 指定新应用目录；`generate -o` 指定资源目录。
默认二进制 `dist/OS-ARCH/executable[.exe]`。`build -tags`、`-ldflags` 原样作为
独立 Go 参数传入，不通过 shell；Windows GUI 子系统由 console 配置决定。

Windows syso 名为 `goui_windows_amd64.syso`，只对该目标生效。工具记录内容摘要；
不覆盖不属于工具或被用户修改的文件，发现其他资源 syso 时要求用户自行协调。
生成的 metadata 同样保护。构建失败不覆盖旧二进制；package 拒绝覆盖既有目录，
使用临时目录完成后一次发布，不递归清理用户目录。

## 平台产物

Windows 使用 goversioninfo v1.7.0 生成版本、图标与基本 manifest；不请求提权，
DPI awareness 沿用运行时的初始化。ICO 的各尺寸直接由 SVG 渲染，PNG 居中等比缩放。
编码参考 xuges/goico；其当前版本拒绝 256×256，命令内部按 ICO 规范以宽高字节 0
表达 256，并用独立解析测试验证。未修改外部 goico 仓库。

macOS 使用 icns/v3 v3.0.1（与项目 Go 版本兼容），通过公开 IconSet 写入独立渲染的
32..1024 像素 PNG 项。生成 Contents/Info.plist、MacOS/executable、Resources/AppIcon.icns。
plist 提供 Bundle ID、名称、版本、APPL 类型及高分辨率声明。显式配置 sandbox
时另外生成 Entitlements，package 在 macOS 使用配置的 signing_identity 签名
并校验；沙箱/用户选择文件/持久访问为独立权限项，不写进 Info.plist。不自动
解锁钥匙串、选择证书、公证或申请分发资格。

Linux 安装目录树包括 usr/bin/executable、usr/share/applications/ID.desktop、
usr/share/icons/hicolor/<size>x<size>/apps/ID.png，SVG 另存 scalable。
desktop 的 Icon、StartupWMClass 使用 ID，Exec 指向 /usr/bin/executable。
目录本身不注册到桌面；安装由调用方负责。

## 验证与进度

- [x] 无桌面测试：严格配置、路径/版本校验、图标尺寸/透明/颜色、ICO 256、ICNS、
  plist/XML 和 desktop 转义、已有文件保护、二进制架构、package 不重建、失败不覆盖。
- [x] 使用真实 Go 编译临时 main 包，检查 Windows PE 图标/版本/manifest 和子系统；
  macOS/Windows 目标交叉构建，不把编译当作桌面运行。
- [x] 全仓 `go test ./...`、命令包 race；Linux amd64 / Windows amd64 / macOS arm64 工具与窗口例程编译。
- [x] Linux X11/XFCE（DISPLAY :3，Software 1x）：命令实际构建、打包，desktop-file-validate，
  安装目录树中的二进制启动／自动关闭、WM_CLASS 和 UTF-8 标题核查。没有自动安装桌面文件。
- [ ] Linux arm64 工具／GUI 窗口编译：被既有 `platform/linux/libs/libc/syscall_linux.go`
  的 `syscall.SYS_PIPE`（29 行）/ `syscall.SYS_POLL`（61 行）未定义阻塞；本轮文件与 HEAD 一致。
  临时标准库 main 包的 arm64 build/package 测试通过，不代表 GOUI arm64 全仓可构建。
- [x] 2026-10-11 Windows 10 22H2 amd64（Go 1.25.8，RDP 桌面 1x）：
  `go test ./cmd/goui -count=1 -v` 全部通过；命令实际构建窗口 EXE，
  版本资源 0.1.0.1、中文名称、Explorer 大／小图标、标题栏／任务栏图标，
  无控制台启动、缩放和关闭通过。窗口使用默认绘制配置，未单独验收各后端。
- [x] 2026-10-11 macOS 15.7.7 arm64（Go 1.24.9，VNC 桌面 1x）：
  命令实际构建并打包 app，plutil 校验、1024 像素 ICNS、包内外二进制哈希一致，
  Finder／Dock 图标、菜单栏名称、双击启动、缩放和关闭通过。
  窗口使用默认绘制配置，未验收签名、公证或 Gatekeeper 分发。
- [x] 2026-10-11 修正 `TestConfigOutsideModule` 的路径别名误判：Go 报告
  `/private/var/...` 而 t.TempDir 返回 `/var/...`，两者设备号／inode 一致。
  断言通过 `os.Stat` / `os.SameFile` 检查实际目录，仍拒绝解析到其它模块，
  不修改生产路径处理。macOS 15.7.7 arm64 / Go 1.24.9 先复现旧测试失败，
  修正后目标回归、完整命令包非缓存测试、race 和 vet 全部通过；本机 Linux
  全仓测试、命令包 race / vet 通过。本次仅无桌面测试修正，不重复窗口验收或
  Windows 原生测试；此前两端窗口验收记录保持有效。
- [ ] Linux desktop 安装集成；不自动安装，不以启动代替图标验收。
- [x] 2026-10-11 文件访问切片：显式 macOS sandbox/user_files/persist_access/
  signing_identity、独立 Entitlements、原生签名/校验与失败不发布通过。
  macOS 15.7.7 arm64 实际 ad-hoc 沙箱 app 的容器目录、外部选择/保存、
  read-write/read-only 凭据和跨启动恢复通过；开发证书 SSH 签名失败和
  稳定证书跨更新验证缺口见 [文件访问验证记录](DesignFileAccess.md#实施与验证记录2026-10-11)。

验证命令：`go test ./...`、`go test -race ./cmd/goui`、
`GOOS=<os> GOARCH=<arch> CGO_ENABLED=0 go build -o <临时目录> ./cmd/goui` 及
`./tests/window/tools/packaging`；测试产物均位于 t.TempDir 或仓库外临时目录。
全仓包含已有缓存结果、桌面 opt-in 跳过及既有离屏图片输出测试，本轮未迁移这些用例。
上述 Windows/macOS 原生验收已完成；未进行 Linux 安装操作。远端源码和产物位于
独立临时目录，传输后校验 SHA-256；未覆盖远端工作区或修改系统安全设置。

## 参考与取舍

- [Fyne 元数据](https://docs.fyne.io/started/metadata/)：采用集中 TOML；不引入其运行时对象。
- [Wails 手工构建](https://wails.io/docs/guides/manual-builds/)：syso 是构建输入，不是事后修改 EXE。
- [Qt 部署](https://doc.qt.io/qt-6/qt-generate-deploy-app-script.html)：区分构建和部署；GOUI 不复制系统 SO。
- [Flutter macOS 分发](https://docs.flutter.dev/deployment/macos)、[Avalonia macOS](https://docs.avaloniaui.net/docs/deployment/macos)：应用结构与签名、公证分开。
- [goversioninfo](https://github.com/josephspurrier/goversioninfo)、[goico](https://github.com/xuges/goico)、[icns](https://github.com/jackmordaunt/icns/tree/v3.0.1)。
- [Apple Bundle](https://developer.apple.com/library/archive/documentation/CoreFoundation/Conceptual/CFBundles/BundleTypes/BundleTypes.html)、[Desktop Entry](https://specifications.freedesktop.org/desktop-entry/latest-single/)、[Icon Theme](https://specifications.freedesktop.org/icon-theme/latest/)。
- [Apple CFBundleVersion](https://developer.apple.com/library/archive/documentation/General/Reference/InfoPlistKeyReference/Articles/CoreFoundationKeys.html)、[ICO 的 256 尺寸表示](https://learn.microsoft.com/en-us/windows/win32/menurc/iconresdir)。

以上是格式/API 参考；本工具是否满足具体桌面显示由原生验收确认，不推断所有桌面一致。
