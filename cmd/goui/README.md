# goui

可选的构建／打包命令，详细契约见 [设计文档](../../doc/DesignBuildPackaging.md)。

```sh
go install github.com/golang-gui/goui/cmd/goui@<version>
goui doctor
goui build -target windows/amd64
goui build -target darwin/arm64
goui package -target darwin/arm64 -format app
goui build -target linux/amd64
goui package -target linux/amd64 -format dir
```

在项目根目录准备 `goui.toml`（不是框架仓库的全局配置）：

```toml
id = "com.example.Notes"
name = "便笺"
executable = "notes"
main = "."
version = "0.1.0"
build = 1
icon = "assets/app.svg"

[windows]
console = false

[linux]
categories = ["Utility"]
```

应用显式使用相同 ID：`gui.NewApplication("com.example.Notes")`，或对应 `ui.Run`。
`build` 是 1..9999 的构建号，默认 1；版本号采用三个 0..65535 的数字段。
可选 `metadata = "internal/appinfo"` 生成同名包的 ID/Name/Version/Build 常量。
先 `goui generate` 再导入生成包；不要修改生成文件，修改 TOML 后再次生成。

所有相对路径以配置文件为基准，不以调用命令的目录为基准。`build -o` 指定输出二进制；
`package -binary` 可以消费外部 Go 构建的产物，`package -o` 指定尚不存在的应用目录。
`generate -o` 指定资源目录，Windows syso 始终在 main 包旁；`-tags` 参与 main 包查询。
默认输出 `dist/OS-ARCH/`。运行 `goui <command> -h` 查看命令参数。

生成资源有 `.goui` 摘要文件，用于避免覆盖用户文件。将生成的 syso/metadata
和摘要成对保存，或将它们都忽略并在 CI 中重新生成。冲突时先检查文件，不要盲目删除。
不要同时使用另一套 main 包资源 syso；console 配置控制 Windows GUI/CUI 子系统。

`build` 总是生成裸二进制；`package` 从不重新编译。Windows EXE 可以直接分发；
macOS app 提供 Bundle 身份和图标，可显式配置签名；公证仍由应用发行流程负责。Linux dir 是安装目录树，
不是自动注册的便携应用；二进制可独立运行，桌面图标需要安装 desktop/hicolor 文件。
不生成 DEB/RPM/AppImage，不自动申请权限、安装或复制系统库。

macOS 沙箱文件访问可以显式启用：

```toml
[macos]
sandbox = true
user_files = "read-write" # 或 read-only；不授予任意文件访问
persist_access = true    # 允许持久化用户选择资源的访问凭据
signing_identity = "Apple Development: ..." # 明确选择本机可用身份
```

`generate` 将 Entitlements.plist 单独生成在资源目录，不混入 Info.plist；
`package` 必须在 macOS 执行，调用系统 codesign 签入并校验，失败不发布 .app。
开发可显式用 `signing_identity = "-"`，但 ad-hoc 不保证跨构建的身份稳定；
保存凭据使用稳定签名，并在用户选择文件后取得 FileAccess.Credential。
原始二进制不因 `build` 而成为可分发的沙箱应用，必须完成 package。
不启用网络、全磁盘或临时例外权限；文件访问和取消/错误契约见
[文件访问设计](../../doc/DesignFileAccess.md)。

SVG 使用和 `cmd/svgpng` 相同的 GOUI 静态子集，按原色等比居中并保留透明。
不支持的 SVG 应先按 [SVG 设计](../../doc/DesignSVG.md) 转为支持的内容或 PNG。
可通过 `[windows].icon` / `[macos].icon` 使用已有 ICO / ICNS。

## 移动端构建（实验阶段）

Android arm64 与 iOS arm64 使用命令内置的精简基座与资源，目前支持 Go 1.25.8。
无需安装 SDK、NDK、JDK、C 编译器或独立链接器，也无需手动准备组件目录。
首次构建会使用本机 Go 自动生成并缓存构建驱动；实际支持与待验收项见
[移动构建设计](../../doc/DesignMobileBuild.md)。

```sh
goui build -target android/arm64 -package apk -o dist/app.apk
goui build -target ios/arm64 -package ipa -o dist/app.ipa
# 尚未配置 iOS 签名材料时，可显式输出供云真机重签的 IPA：
goui build -target ios/arm64 -package ipa -unsigned -o dist/app-resign.ipa
```

Android 未配置签名时使用本机持久化开发身份；正式分发配置自己的 PKCS#12。
iOS 配置 `[ios.signing]` 的 `pkcs12`、`profile`、`password_env`，PKCS#12 包含
私钥、证书及中间 CA。密码环境变量只存名称，工具不会上传私钥或自动续证。
签名和验签在 Go 内完成；设备必须属于 profile 授权范围。

`-components` 或 GOUI_MOBILE_COMPONENTS 仅供维护者覆盖。build 编译、package
组包的职责不变，build -package 显式组合两者。内部以 CGO_ENABLED=1 消费原生
必需支持包，应用只编写 Go，不执行 C/Java 编译器。iOS -unsigned 仍需重签才能
安装。公开 SDK 输入再分发审核、物理设备及 Android 16 KiB 运行仍待验收，不能
用模拟器或签名格式验证替代。
