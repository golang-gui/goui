# 移动端维护者验收构建

此工具只组织移动平台例程验证，不是已交付的应用开发者打包 CLI。使用固定 Go
1.25.8，组件制作需要对应 NDK/Xcode；消费构建只使用 Go 和预制链接工具，不安装
原生编译器。Android ARM64、iOS ARM64 device/simulator 分别制作组件，不可混用。

在仓库根执行，配置和所有输出放在仓库外：

```sh
go run ./tests/window/platform/mobile_build base /absolute/base-config.json
go run ./tests/window/platform/mobile_build native /absolute/app-config.json
go run ./tests/window/platform/mobile_build build /absolute/app-config.json
go run ./tests/window/platform/mobile_build package /absolute/app-config.json
```

- `base`：创建仓库外最小 main，只依赖固定入口和 FFI；调用 `goui-base export`。
  输出 `output/base/base.json` 及原生必需包归档，仍使用原版 Go runtime/cgo。
- `native`：运行 `go generate`，将 Go 中内嵌的原生测试参考、最小 iOS main、Java
  Typography helper 生成到 `output/generated`；仅编译测试 probe、iOS main、UIKit SDK 协议元数据和 Dex。
  Android/iOS 窗口、生命周期、软件呈现、EGL/EAGL 均由平台 Go 包实现。
- `build`：消费匹配归档与链接输入，将普通 Go 源码链接为 libgoui.so 或 Probe。
  不执行 `go generate`、cgo 或 SDK 编译器。可把上一步组件传到其他构建主机运行。
- `package`：Android 使用测试资源工具和私有测试密钥组装/签名/验签 APK；iOS
  输出未签名 `.app`，在目标 Mac 签名后安装。正式 IPA、分发、证书申请不在本例范围。

配置沿用 mobile_paint 的字段。`fixture` 可选 mobile_paint（默认）、mobile_surface、
mobile_start、android_gui、android_text。`target` 为 android 或 ios；`variant`
为 device 或 simulator（Android 仅 device）。`painter` 为 software 或 opengl。

```json
{
  "fixture": "mobile_paint",
  "target": "ios",
  "variant": "simulator",
  "painter": "software",
  "output": "/absolute/artifacts/ios-software",
  "go": "/absolute/go1.25.8/bin/go",
  "gouiBase": "/absolute/tools/goui-base",
  "cc": "/absolute/maintainer/ios-simulator-cc",
  "driver": "/absolute/tools/go-mobile",
  "linker": "/absolute/tools/goui-link",
  "base": "/absolute/components/base-ios/base.json",
  "inputs": "/absolute/components/ios-inputs/inputs.json",
  "tools": "/absolute/components/host-tools/tools.json",
  "consumerPath": "/absolute/consumer-bin-without-native-compilers",
  "modules": ["/absolute/cgo", "/absolute/purego", "/absolute/nanovgo"]
}
```

`modules` 显式接入本地开发分支，不修改项目 go.mod。`cc` 可为 argv 数组；base
阶段须为单个编译器 wrapper 路径。维护者执行工具时使用本机 GOOS，不在外层
设置移动 GOOS，以免把 `go run` 的生成器编译成不能在本机运行的移动二进制。

Android 另配置 java、androidJar、d8、aapt2、signer、key、cert、apksigner；含义
分别为 Java 可执行文件、平台 jar、D8 jar、资源工具、接受 -key/-cert/-in/-out 的
测试 APK 签名工具、测试私钥、证书、SDK 验签 jar。android_text 另传 `font`，
维护阶段通过系统 HarfBuzz 制作独立 shaping 参考；不加载 Android 私有排版库。
密钥与 SDK 不入库。需要额外打包资源时使用 `assets` 指定已有资源目录。

Android 测试通过 `android.app.func_name=probe_onCreate` 包装真正的
ANativeActivity_onCreate，只采集独立参考数据。mobile_start 是有意独立的原生
启动参考；其他例程使用正式平台实现。生产代码没有测试 getter 或 goui_host_* 协议。
iOS 的 Scene 类为 GoUIScene；独立启动参考使用 ProbeScene。UIKit 生成器仅保留
系统协议元数据，以使 Go 注册的类可以正式声明协议。元数据对象没有窗口或回调
实现，也不进入 runtime 基座；iOS 消费链接需同时传 main.o、uikit.o、probe.o。

每阶段记录日志、receipt 和产物 SHA-256。消费阶段检查已复用固定入口、未执行
cgo、未改写应用源目录；另保留拒绝编译器 PATH 的调用记录作为独立证据。
构建使用本次产物目录中独立的冷 Go 缓存，以保证日志实际经过消费钩子；热缓存可能
完全跳过钩子，不能用它证明本轮原生支持的消费。format 1 校验固定入口归档，
format 2 校验 runtime/cgo 与入口的对象 recipe；两种格式都拒绝调用 cgo 编译器。

本工具不会启动设备。安装、旋转、后台恢复、像素和操作预期见各例程 README。
模拟器通过不能替代真机验收；设备组件链接通过也不能替代安装与实际操作。
验证后关闭并删除专用模拟器、ADB 服务和临时应用，不影响用户已有设备或签名配置。
