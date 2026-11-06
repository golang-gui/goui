# GOUI 移动应用构建与打包

状态：2026-11-05，正式 CLI 已接入本地组件、移动编译、开发签名 APK 与待重签
iOS app/IPA；组件分发和完整自有证书签名尚未实现。现有基座、平台接入及
模拟器结果见 [PlanMobileBuild](PlanMobileBuild.md) 和
[PlanMobileConvergence](PlanMobileConvergence.md)。本篇描述下一阶段契约，不将
维护者的 SDK、本地 replace 或测试宿主当作应用开发者的正常依赖。

## 1. 交付目标

使用者编写普通 Go main 包，只通过 platform 创建窗口、排版与绘制，使用 Go 和
goui 工具得到可以在物理 Android/iPhone 上安装、启动、持续绘制和前后台恢复的
应用。Windows amd64、Linux amd64 为优先消费宿主，macOS arm64 为辅助构建和
原生对照宿主；目标先支持 Android arm64、iOS device arm64。

使用者不安装 C/C++ 编译器、NDK、Xcode、JDK、Gradle；维护者可以用它们生产
可复用组件。内部继续 CGO_ENABLED=1，复用原版 Go 编译器、汇编器、runtime、
cmd/link 和既有窄 cmd/go 驱动。Go 版本仍精确固定为 1.25.8，不能承诺自动兼容
任意已安装版本。普通 Go 包从源码编译，原生必需包才使用匹配归档。

本轮不实现 GUI 触摸、IME、SafeArea、ui 适配、任意第三方 cgo、原生控件嵌入、
多窗口、iPad 适配、AAB、App Store/TestFlight 上传或应用商店合规承诺。iOS
Simulator 保留为明确 variant 的维护者回归目标，不能替代 device 验收。

## 2. 命令与职责

保持 [桌面构建设计](DesignBuildPackaging.md) 的职责：build 编译、package 消费
已有产物；不因移动端必须组包就悄悄改变桌面命令的结果。

| 命令 | 移动目标行为 |
| --- | --- |
| doctor | 分项检查配置、Go/模块、匹配组件和已配置的签名材料；不启动设备、不生成“真机通过”结论 |
| generate | 导出可检查的 manifest/plist、图标和组包资源；不编译 Java/C、不写用户 main |
| build | 自动选择本地组件及 main 注册 overlay，构建 Android libgoui.so 或 iOS Mach-O |
| package | 消费二进制及其构建记录，生成并校验 APK、.app 或 IPA；不隐式编译 Go |
| build -package FORMAT | 显式组合 build 与 package，在临时目录编译并组包，最终只发布所请求的应用产物 |

最后一项显式组合编译与组包，以一个参数满足“一条命令构建 App”，避免在默认 build
中隐藏签名或安装行为。FORMAT 为 android 的 apk、ios 的 app/ipa；不增加 install
或自动选择设备。普通 build 的 -o 表示二进制，带 -package 时表示最终应用产物。
桌面已有调用保持不变，组合入口复用相同函数，不复制另一条构建路径。

```sh
# 默认使用内置组件；当前已验证的工具链为 Go 1.25.8。
# iOS 正式签名需配置自己的 PKCS#12/profile；以下显式导出待重签包。
goui doctor -target android/arm64
goui build -target android/arm64 -package apk -o dist/hello.apk
goui build -target ios/arm64 -package ipa -unsigned -o dist/hello-resign.ipa

# 编译与签名可以由不同机器/不同 CI 阶段负责。
goui build -target ios/arm64
goui package -target ios/arm64 -format ipa -unsigned
```

target 沿用 OS/ARCH，只增加 android/arm64、ios/arm64。ios 默认 device；维护者
可显式 -variant simulator，产物目录、链接输入和签名规则全部区分 variant。
Simulator 不产生“可装真机 IPA”。不把 ios 伪装为 darwin 目标，代码仍归 darwin/ios。

默认原始输出为 dist/android-arm64/libgoui.so、dist/ios-arm64/executable；Simulator
使用 dist/ios-arm64-simulator。应用输出采用独立文件名/目录，不覆盖原始构建。
build 失败不覆盖旧二进制；组包、签名、验签完成前不发布最终应用。package 保留
既有输出保护，拒绝覆盖已有目标；不递归清理用户指定的已有目录。

## 3. 最小配置

复用现有 goui.toml 的 id、name、executable、main、version、build、icon。路径仍
相对配置文件，不提供任意脚本或自由 native linker 配置，不向 gui/ui 增加接口。

```toml
id = "org.example.Hello"
name = "Hello GOUI"
executable = "hello"
main = "./cmd/hello"
version = "0.1.0"
build = 1
icon = "assets/icon.svg"

[android]
min_sdk = 29
target_sdk = 35
internet = true
debuggable = false

# 不配置时使用本机持久化的开发签名；正式分发显式提供自己的材料。
# [android.signing]
# pkcs12 = "/private/android.p12"
# password_env = "ANDROID_SIGN_PASSWORD"

[ios]
minimum_version = "15.0"

# 正式 iOS 签名：PKCS#12 包含私钥、证书及中间 CA；profile 匹配应用。
# [ios.signing]
# profile = "/private/hello.mobileprovision"
# pkcs12 = "/private/ios-development.p12"
# password_env = "IOS_SIGN_PASSWORD"
```

上述 API 29/iOS 15.0 与现有组件对齐，不意味着所有更高 target_sdk 已验证。
首个正式 Android 组件集必须同时固定、记录所支持的 target_sdk；高于该值须报错，
不能把任意整数直接写 manifest 后宣称兼容。min_sdk/最低 iOS 版本低于组件要求
时报错，高于组件要求可以提高应用声明；记录实际原生最低要求与应用声明两者。
Android ID 的每个段还须通过 Java package 命名检查，不能沿用桌面允许连字符的
规则而输出无效 manifest；错误在 native build 前报告。

internet 只生成 Android 的普通 INTERNET 声明，不引入跨平台权限系统。iOS 本轮
使用正常 HTTPS，不生成 ATS 全局例外，不申请相机、定位、通知等未实现能力。
嵌入资源继续使用 go:embed，首版不增加资源查找框架。排版 Dex、UIKit metadata、
启动 main 是框架组件，由工具自动选择，不让应用逐项列出。

签名配置只保存文件路径和密码环境变量名称。工具使用 Go 读取 PKCS#12；如底层
签名器只接受 PEM，在受限仓库外临时目录写出所需材料，结束后清理。不把密码
加入进程 argv、构建记录或日志，也不要求用户为格式转换安装 openssl。选用成熟
Go PKCS#12 实现前审核版本、许可证与测试，不自行实现密码学格式。

## 4. 组件分离与选择

### 4.1 组件类型

| 组件 | 选择依据与内容 |
| --- | --- |
| Runtime base | 精确 Go 版本、目标、variant、Go ABI、原生最低版本；runtime/cgo 及固定入口等原生必需 Go 归档 |
| 可选标准库支持 | 实际依赖是否含原生编译包；例如 Android net；不因联网需求预编译整个标准库 |
| FFI 支持 | purego/goexlib 版本、源码摘要及 ABI；其普通 Go/汇编部分仍源码编译 |
| Host link tools | 宿主 OS/ARCH、内置 golink 库版本或固定 LLD 及执行依赖；配套 goui-link 和归档消费驱动 |
| Target inputs | 目标 OS/ARCH/variant、原生最低版本、CRT、系统库 stub、可用框架和链接约束 |
| Platform auxiliary | Android 排版 Dex，iOS 最小 main 与 UIKit 协议 metadata；绑定到相关 platform 源码/组件 ABI |
| Packaging/signing tools | 宿主资源工具、独立 APK/iOS 签名器；与 Go runtime base 分开更新 |

平台 Go 源码不成为预编译宿主。base 不包含 Activity/Scene 事件循环、窗口、EGL/
EAGL、应用 manifest、ID 或权限。Go 的 .syso 不是当前必要入口，不改回临时
链接器实验；原生辅助对象仍显式传给已有 BuildOptions。

### 4.2 按应用依赖选组件

1. 确认 Go 版本、目标、variant、入口 main 与当前 GOUI/FFI 模块身份。
2. 使用目标环境的 go list -deps -json（含最终 tags）检查实际依赖，收集 CgoFiles/
   SWIGFiles；go env/go list 之前就设置 link-only CC，避免预检探测系统 gcc。
3. 只选择实际需要且已有官方匹配记录的原生包归档。重名包的源码、模块与依赖
   ABI 不一致时报错；不以任选一个或最近版本兜底，不自动运行原生编译器。
4. 将选择结果生成到本次临时归档清单，沿用当前 source/export/build-ID 校验；
   不改写发布基座。应用 main 只增加规定注册 overlay，不允许借 overlay 修改依赖。
5. 可选图形/排版辅助组件按实际包依赖与声明的组件要求选择，不分析 GUI 类型，
   不要求使用者知道每个 SDK 对象名。首版平台支持集可固定，后续再按体积优化。

任意 import C 的业务包或未知第三方包明确报告缺失归档及包路径。用户可自建
经过同样校验的组件集，但不能借用其他版本归档，也不因此承诺任意 cgo 都受支持。

### 4.3 Go 1.25.8 标准库差异

2026-11-05 在 Linux amd64 上以 GOENV=off、GOWORK=off、空 GOFLAGS、默认 tags、
GOARCH=arm64、CGO_ENABLED=1 分别执行目标 go list std，并查看实际源码：

| 目标/包 | CgoFiles / 原生依赖 | 构建处理 |
| --- | --- | --- |
| 两端 runtime/cgo | cgo.go | 继续使用精确匹配的基座归档 |
| Android net | cgo_android.go、cgo_unix_cgo.go 等 | 增加可选、版本匹配的 net 归档；不默默改用 netgo 绕过系统 DNS |
| iOS net | CgoFiles 为空；Darwin Go/汇编桥调用解析器 | 从 Go 源码正常构建，同时满足系统链接依赖 |
| iOS os/user | CgoFiles 为空；Darwin syscall 实现 | 从 Go 源码构建；不能由此承诺沙箱内获得桌面用户信息 |
| iOS crypto/x509 | CgoFiles 为空；调用 Security/CoreFoundation | 补齐所需 SDK 链接输入，验证真实系统信任链，不禁用证书校验 |
| plugin | 两端 std 清单中均有 plugin_dlopen.go | 原生构建依赖不等于移动 plugin 受支持；首版应用导入时明确拒绝 |
| 标准库内部测试辅助包 | iOS net/internal/cgotest；Android sysrand 的内部 seccomp helper | 不是普通应用运行依赖，不加入发布基座 |

目录中出现 cgo 字样、使用原生函数或 go:cgo_import_dynamic，不代表消费时要运行
C 编译器。反过来，检查 std 全集也不能替代应用实际依赖闭包。以上为固定版本的
源码/包选择核对，尚非两端 HTTPS/DNS/系统证书的运行验收。

### 4.4 内置组件与维护者覆盖

早期验证使用本地只读组件集。最终消费默认采用第11节的内置精简组件，通过
-components DIR 或 GOUI_MOBILE_COMPONENTS 仅作维护者覆盖，不把本机 SDK 路径
写进应用 goui.toml。goui-base 保留导出/封装职责，goui-link 保留独立调试入口。
不再为当前数 MiB 的资源引入组件下载器、Release 索引或独立链接器下载。

缓存按输入摘要区分 Go 版本、目标、variant 和 ABI；宿主 Go 驱动另行缓存。
拒绝越界路径、软链接及摘要不符；用临时目录和原子发布避免中断/并发破坏缓存。
普通 Go 编译及模块下载继续交给 Go；内置组件无需联网，Go 模块的离线策略沿用
标准 Go 工具，不增加一套框架依赖解析器。

维护者可用 NDK/Xcode 制作；使用者消费阶段不包含 SDK 编译步骤。Apple SDK stub
和其他输入目前仅做过私有验证，provenance 中的 review status 不授予再分发权。
公开分发前逐项审核来源、许可和附带声明；未完成时只宣布本地组件验证，不能
宣称已解决所有平台的公开无 SDK 构建。组件的真实尺寸以打包实测记录，不预估承诺。

### 4.5 链接工具体积与后续替代（2026-11-06）

现有验证包复用 Rust 工具链的 LLD 22.1.8，动态依赖完整 libLLVM；其体积不是
移动基座或链接流程的固有要求。本轮已在仓库外用官方同版本源码构建静态链接 LLVM
组件的独立 LLD，记录构建参数、运行依赖、原始/strip/压缩尺寸，并通过既有 Android
和 iOS 构建路径。首轮保留上游默认驱动，不为缩小体积维护源码裁剪补丁。
这里的静态链接是工具自身的 LLVM/C++ 库组合，不改变 App 链接方式，也不自动
承诺工具完全不依赖宿主 libc。Linux amd64 的工具组件由201.6 MiB降至46.2 MiB，
gzip包由64.4 MiB降至18.0 MiB；其他宿主与新产物的原生运行待验收。配置、比较
边界和原始证据见 [收敛计划](PlanMobileConvergence.md#lld-静态组件体积验证2026-11-06)。

后续保留独立的纯 Go 最小原生链接器计划：消费官方 Go 工具链生成的原生目标
文件、已知预编译基座和平台链接输入，输出 Android ARM64 ELF 或 iOS ARM64
Mach-O。参考或选择性复用 Go 链接器源码，不维护 Go 工具链补丁，也不改变 Go
编译器、runtime 或 GC 元数据生成。该项目先限定受控输入，未知原生段/重定位
明确拒绝；Android 先行、iOS 后续。它是现有最终原生链接步骤的可选替代，不是
当前移动平台交付的前置。Android ELF 已从原型接入正式构建驱动，见下节；
iOS ARM64 Mach-O 后端随后完成；通用原生链接器能力仍不属于当前承诺。

### 4.6 最小原生编译范围与独立Go链接器验证（2026-11-06）

目标是缩小必须经过NDK/Xcode/JDK等原生工具链编译的代码范围，而不是仅压缩
预编译产物。普通 Go 包继续由用户工具链编译；原生必需包可以保留按精确 Go
版本匹配的预编译归档，不为消除少量发行文件强制引入现场生成的复杂度。
当前 Go 1.25.8 完整基座（包含归档及清单，不含宿主工具/SDK输入）的 gzip6
包：Android 1051041 字节，iOS device 136416 字节、Simulator 135605 字节。
这是本地组件测量，不代表包含完整构建工具的最终下载大小。

已有包归档中的真正原生对象合计：Android122072字节，iOS device56816字节、
Simulator57352字节；其余大部分为Go对象与导出数据。Android runtime/cgo的Go
和Go汇编已通过临时源码overlay现场编译，再组合14个既有原生对象（82120字节）；
整个消费者流程没有调用原生编译器。组合库在API30 ARM64模拟器中加载、执行
Go init及GC通过。这是复用标准runtime/cgo原生桥的验证，不是完整fakecgo。

现场生成保留为后续可选优化；固定 ABI 入口和 FFI 辅助可逐项评估 Go 汇编。
现阶段保留标准 runtime/cgo 已验证的原生线程/TLS/栈支持，原生代码仍由维护者
预编译，消费者不需要 NDK/Xcode。兼容 Go 版本须检查和验证，不能仅删除版本
约束。当前只验证 Go1.25.8；完整的 net、FFI 与入口 Go 支持现场生成，以及跨
Go 版本原生对象复用尚未实现。

purego v0.10.2的Android动态加载绑定仍依赖cgo，iOS显式拒绝无cgo模式；其
桌面fakecgo不能直接替代移动共享库初始化。优先保持已验证的线程、栈和TLS
行为，进一步消除原生桥须单独验证库加载、外部线程回调和GC，不以零C为硬前置。

独立纯Go ELF原型已消费官方Go链接器产生的27个原生对象、192699条加载段
重定位，共12种类型。Linux/Windows/macOS宿主生成相同字节的Android ELF；
Go链接的正式GLES标准库例程在Android模拟器通过137项/1原skip、像素及前后台
检查。链接器只使用Go标准库，无Go工具链补丁；Android 已接入正式构建驱动，DWARF、
通用TLS、分支跳板、符号版本、完整归档语义和Mach-O等仍非已支持承诺。详细
源码、场景与边界见 [收敛计划](PlanMobileConvergence.md#独立go链接器与原生编译边界验证2026-11-06)。

### 4.7 独立 Go 链接器接入（2026-11-06）

独立 `golink` 模块不依赖 GOUI，不解析 GUI、APK、IPA 或组件目录。它只消费
显式原生输入并生成目标二进制；Go 编译、ABI/版本匹配、SDK 输入和签名仍由
已有各层负责。当前实现支持 Android ARM64 ELF 与 iOS ARM64 Mach-O（设备/模拟器）；LLD 保留为显式可选组件。

`NativeTools` format 1 保持现有 `lld` 可执行文件声明；format 2 使用 `golink`
库版本字符串，不能同时配置非空 LLD，也不提供失败后静默回退。组件仍校验宿主
和已声明文件摘要；内置库不需要可执行文件，所以 format 2 的 files 可以为空。
不再下载、执行独立 golink，也不模拟 LLD 版本：

```json
{
  "format": 2,
  "goos": "linux",
  "goarch": "amd64",
  "provenance": {
    "name": "golink",
    "version": "0.2.1",
    "source": "github.com/golang-gui/golink@v0.2.1",
    "license": "MIT"
  },
  "golink": "0.2.1",
  "files": {}
}
```

GOUI 工具直接 import `github.com/golang-gui/golink`，随工具一起编译，并调用
`golink.Link(golink.Request{...})`。组件版本必须匹配编译进工具的 golink.Version。
现有 goui-link 仍为 Go external-link 驱动，但它直接调用库，不再生成 JSON 临时
文件或启动 golink 子进程。独立项目的可选 CLI/JSON 接口仅供单独使用和验证，
不属于 GOUI 使用者必需下载的构建组件。

驱动沿用已有参数校验，将对象、归档、目标库和 symbolic/nodelete 等要求转为
类型化 Request。不搜索宿主库，不传递 LLD 命令行。Android 后端总是 eager binding、
GNU hash、RELRO、16 KiB LOAD 对齐并导出可见符号，省略非加载 DWARF；不支持
的 build-id、压缩调试段等要求返回错误。链接库负责输入/重定位检查，GOUI 再
检查输出 ELF，校验和取消检查通过后才替换已有产物。

本轮正式驱动的空缓存 APK 构建、跨宿主链接与 Android 模拟器运行通过；公开
组件分发、更多 Go 版本和真机/16 KiB 内核尚未完成；iOS 运行验收见下节。完整证据见
[接入记录](PlanMobileConvergence.md#独立-go-链接器正式接入2026-11-06)。

### 4.8 iOS Mach-O 功能包后端（2026-11-06）

GOUI 直接调用 golink v0.2.1 的 Link，传入目标 ios/arm64、IOSOptions
（variant/minimum/sdk）、完整 Go c-archive、平台入口/协议对象和显式 TBD。
原生 Go/FFI 支撑继续使用精确版本基座，不修改 runtime、UIKit、文字或 GUI。
消费者不需要下载独立 golink、LLVM 或 Apple 编译器；标准 Go external-archive
仍用 Go pack 写出，最终链接由 Go 功能包完成。

TBD v4 根库通过 Libraries 明确传入，Reexports 只提供已封装 SDK 中的补充
TBD，不形成额外根加载命令，不搜索本机 SDK/库。保留导出目标过滤、重导出
闭包、Objective-C 名称和 current/compatibility 版本。模拟器 SDK 的三个宿主
库平台标记采用 [LLD 22.1.8 的窄兼容规则](https://github.com/llvm/llvm-project/blob/llvmorg-22.1.8/lld/MachO/InputFiles.cpp)：
仅 libsystem_kernel/platform/pthread 的 ARM64 macOS 标签可作为 iOS simulator
的系统重导出；其他平台不混用。不是任意 YAML/TBD 版本的解析器。

后端验证 ARM64 CPU 子类型、对象的平台/部署版本、段与重定位边界，支持当前
Go/SDK 的 UNSIGNED、SUBTRACTOR、BRANCH26、PAGE21/PAGEOFF12、GOT 和 ADDEND
组合。合并原生同名节，保留 Go 模块及 UIKit 初始化数组、Objective-C 元数据、
只读重定位段、非惰性导入/桩、导出 trie、compact/DWARF 展开信息及确定的 UUID。
未知 TLS、arm64e 指针认证、personality/LSDA、LTO 和分支溢出明确失败。

输出是 PIE 可执行文件，LC_BUILD_VERSION 区分 device=2/simulator=7。
复用 Go1.25.8 的 BSD ad-hoc 签名工具，许可证保存在独立模块，不维护工具链补丁。
裸 Mach-O 的代码页签名不等于 App bundle 资源封印，也不能代替设备证书/profile；
最终 App/IPA 签名仍由打包层或云真机重签负责。本轮生成待重签 IPA，不承诺已能
在物理设备安装，也不将模拟器通过当作商店审核/真机验收。

golink v0.2.1 已公开发布，并通过全新模块缓存下载核验。SDK 输入再分发仍须审核。
运行与跨宿主
验证见 [iOS 接入记录](PlanMobileConvergence.md#ios-mach-o-独立-go-链接后端2026-11-06)。

## 5. Android 构建、组包与签名

### 5.1 原生输出与资源

调用既有 mobilebase.Build，输出 Android arm64 c-shared ELF。固定导出为
ANativeActivity_onCreate，生产 manifest 使用系统 NativeActivity 默认入口；不带
测试 probe_onCreate、探针符号或测试 Activity。基础生产包结构为：

```text
AndroidManifest.xml          # Go 编码的二进制 manifest
resources.arsc / res/        # 名称、图标、必要系统主题资源
classes.dex                 # 实际使用 Android 排版时带 helper
lib/arm64-v8a/libgoui.so
META-INF/                   # 如签名方案需要
```

Manifest 及五种 density 图标资源表由 Go 编码，只支持框架明确生成的资源集合，
不实现通用 Android 资源编译器。消费阶段不需要 aapt2 或 android.jar。图标复用
现有 SVG/PNG 渲染；adaptive icon 与商店素材后续单独设计。
所有 XML 属性正确转义，Android exported、native lib name、包 ID、版本、最低/
目标 SDK 与选用组件保持一致。活动与 GOUI app ID 一致性仍由应用参数保证。

首版使用压缩 native library + extractNativeLibs=true，以系统安装时提取库的
方式简化 APK 内原生库对齐；这只减少 ZIP 直接 mmap 约束，不能绕过 ELF 16 KiB
要求。组装、所需对齐必须在签名之前，签名后禁止再次修改 ZIP 内容。

### 5.2 签名

开发包允许工具用 Go 生成专用开发 key/cert，保存于用户配置目录并稳定复用，
明确输出“开发签名”；不放仓库、不随不同组件版本重新生成，不冒充正式发布身份。
应用 debuggable 由配置独立决定，不以证书种类推导。正式签名显式使用开发者材料，
支持同证书升级覆盖；换证书不伪装成兼容升级。

当前直接集成 apksig-go v1.1.0 的 Go 库，生成 v2/v3 签名并验签；PKCS#12 由
go-pkcs12 解码，密码只从指定环境变量读取。正式身份与开发身份分开报告，不启动
外部签名/验签程序。维护者以官方 apksigner 和系统安装结果交叉验证，使用者不
需要 Java；原生库压缩及资源表的四字节对齐保持明确。

正式集成前将 min_sdk 对应的签名方案明确到包格式；不能仅凭某个 API 30 模拟器
成功就承诺所有设备。签名、独立验签及最终结构检查通过后才发布 APK。

### 5.3 16 KiB 前置验收

Android 15 起存在 16 KiB 内核设备。组件与应用的 ELF PT_LOAD 对齐、各 load 段
的文件/虚拟地址关系及 runtime/FFI 的页大小假设均需检查；对齐通过不等于运行通过。
Android 链接约束参考官方要求，LLD 对应 max-page-size/common-page-size=16384；同时检查 Go
最终链接产物而非只检查几份 NDK .o。所有新产物仍须在 4 KiB 设备回归。

先在独立 16 KiB 模拟器启动/后台/GC/栈增长验收，随后在可用的物理 16 KiB 设备
验收；没有设备时保留硬件缺口，不将兼容模式启动当作原生 16 KiB 支持。
目标 API 与上架政策另行记录，本轮不以商店规则代替底层兼容性证明。

## 6. iOS 构建、组包与签名

### 6.1 产物模型

复用 Go c-archive + golink 功能包（可显式选择 LLD），链接为单个 Mach-O 主可执行文件。最小 native main 仅
调用 goui_run；SDK 协议 metadata 与 Go 实现的 UIKit 类/IMP 配合。没有独立 Go
framework，也不需要对不存在的 framework 先签名。生产 App 不包含测试 probe
或 XCTest bundle。

```text
Hello.app/
  hello
  Info.plist
  AppIcon*.png
  embedded.mobileprovision  # device 签名时
  _CodeSignature/           # 签名工具生成
```

Info.plist 提供 bundle ID、版本、executable、最低系统、设备类型、方向、正式
Scene 声明和最小启动屏配置；首批设备类型为 iPhone。优先验证无需 ibtool 的
UILaunchScreen 字典和 PNG 图标，消费端不编译 storyboard/asset catalog。若目标
系统实际要求预编译资源，则由维护者预制固定资源，不把 Xcode 交还给用户。

iOS 所需框架从实际 Go 原生导入和组件要求闭包确定；联网增加 Security 等依赖
时扩展 Target inputs，不通过忽略未解析符号或跳过证书校验来“构建成功”。

### 6.2 签名是设备运行前置条件

Simulator 可用显式 ad-hoc 签名，device 必须有有效的 Apple iOS 开发/相应分发
证书及私钥、匹配 profile，并满足设备资格。macOS Developer ID、公证凭据不能
替代 iOS 材料。工具不申请账户、自动续证、猜选证书或向苹果上传用户私钥。

签名前检查 profile 的 CMS 内容/签名、有效期、平台、App ID 匹配、授权证书、
Team/App ID Prefix 和 entitlement；安装目标必须处于 profile 的设备授权范围。
不把 TeamIdentifier 与 AppIdentifierPrefix 无条件等同，不把 profile 中所有
entitlements 直接复制给 App。只生成本阶段必要授权并受 profile 约束，匹配错误
在签名前报告；最终以设备安装/启动验证系统是否接受。

device 的 package 默认要求签名材料，缺失时明确报错。2026-11-05 用户
确认先提供云真机重签的 IPA，因此显式 -unsigned 可导出 app 或 IPA，命令输出
明确说明仍须签名后安装；这是待重签产物，不冒充已签名真机包。IPA 使用
Payload/Hello.app 标准布局，不把 ZIP 后缀本身当作签名证明。

### 6.3 跨宿主 Go 签名

直接使用 Go 库，不预置 zsign、OpenSSL 或原生 codesign 程序：go-macho 负责
Mach-O 签名布局，go-codesign 的资源封印、profile/entitlement 格式辅助与
go.mozilla.org/pkcs7、go-pkcs12 配合生成 SHA-256 CodeDirectory/CMS 签名。
维护者使用 macOS codesign 作独立参考；用户构建不调用它。

内置 Apple 官方公开根证书，校验 PKCS#12 代码签名证书链和 profile 的 CMS
信任链；不接受任意自签 CMS 作为 Apple 授权。中间 CA 随用户的 PKCS#12 提供，
签名过程不自动联网补证书。授权只包含本阶段必要的应用/团队标识及受 profile
约束的调试设置，wildcard App ID 按实际应用展开。用户私钥不上传、不导入系统
钥匙串，密码不进入命令参数。打包不猜测具体设备，profile 的设备覆盖由使用者
选择的安装目标决定；物理设备的安装、更新、启动仍是后续验收项。

profile 签名者还须符合 Apple 的独立签发角色，不能把一张普通开发者证书的
Apple 信任链当作签发 profile 的权限。依据 Apple Security 源码中的
[SecPolicyCreateiPhoneProvisioningProfileSigning](https://github.com/apple-oss-distributions/Security/blob/main/OSX/sec/Security/SecPolicy.c)
核对 subject/issuer common name，并使用生产 Apple 根证书校验 CMS。离线构建不
进行网络吊销查询，最终授权仍由设备系统验证。

签名不得修改 load commands/签名区之外的原程序数据；签后检查代码页、CMS、
特殊槽和实际资源。Windows 生成的 IPA 显式保存入口的 Unix 0755 权限，不依赖
宿主文件系统能否表示该权限。合成材料的单元测试及 macOS 严格验签通过，不能
替代有效 Apple 身份在真实设备上的安装验收。

## 7. 实现落点与构建记录

- cmd/goui：扩展现有 target/config/doctor/generate/build/package，按目标增加
  mobile、android、ios 辅助文件；复用图标、路径、输出保护与命令执行，不另建
  一套 CLI、平台运行时或动态插件。配置校验按目标执行，避免 iOS 必填材料阻碍
  同一工程构建 Linux/Android。
- platform/mobile/base：补充需要的 tags/有限 Go ldflags、组件组合及构建校验。
  保留原版 cmd/link、归档驱动的源码校验和 CC 预检防护；不负责 manifest、UI
  或签名。普通 tags 可用，但最终原生包选择/源码必须仍与组件匹配。移动首版
  ldflags 明确支持 -s、-w、-X，拒绝覆盖 extld、buildmode、ABI 和工具选择；
  桌面现有透传行为不变。
- platform/mobile、android、darwin/ios：保持已收敛的 Go 生命周期实现。只有
  真机证明存在平台错误时才改对应后端，不为组包向平台添加构建命令或测试 getter。
- tests/window/platform/mobile_app：新增生产式验收 main 与 goui.toml，只用公开
  platform/graphics/typography/Go 标准库；不导入 gui/ui，不依赖 probe 符号。
- tests/window/platform/mobile_build：继续作为维护者组件/独立参考工具。可复用
  已验证逻辑，不能将测试 fixture 枚举或原生探针复制进公开 CLI。

原始移动二进制附带带 SHA-256 的构建记录，保存目标/variant、Go 版本、所选
组件摘要、主模块和必要辅助组件身份，供跨机器 package 检查；不把绝对本机路径
或密钥写入。仅 hash 不能认证任意第三方二进制，记录只约束构建与组包一致性。
二进制与记录逐一发布，不声称两个文件是一次原子操作；package 拒绝缺失/摘要
不匹配的组合。一步 build -package 则在同一临时目录消费记录，成功后发布完整
安装产物，任何编译、签名或校验失败都保留原有最终包。

## 8. 分阶段实施与验收

当前已实现内置 profile、依赖检查、编译记录、Go 资源组包与 APK/iOS 签名代码。
Android net 归档包含在内置基座中。新 Go 版本的基座、自有 Apple 身份的真机验收、
Android 16 KiB 运行及 SDK 输入公开再分发审核仍须分别完成。

| 阶段 | 实现内容 | 完成标准 |
| --- | --- | --- |
| B1 本地组件/编译 | 组件选择、配置、main overlay、移动 target、原始输出/记录；补充按需标准库支持 | Windows/Linux 无原生编译器、空缓存构建 platform-only 与联网 main；未知 cgo 明确失败 |
| B2 Android 生产 APK | 正式 manifest/图标/Dex、签名与独立验签；16 KiB 构建和运行 | 不含测试宿主的 APK 在物理 Android 启动、绘制、旋转、前后台与覆盖更新；4/16 KiB 分别记录 |
| B3 iOS 生产 bundle/IPA | 最小主程序资源、profile/entitlements、Mac 参考签名、跨宿主签名 | Linux/Windows 构建签名产物在物理 iPhone 安装/启动/前后台/更新；device 不混用 simulator |
| B4 使用体验/分发 | build -package、内置精简组件与缓存、依赖正式版本、操作文档 | 干净消费环境只需匹配 Go + goui + 自有签名材料，无 SDK/JDK/编译器；公开组件许可审核通过 |

每阶段代码、逻辑测试及设计记录同步提交。不以本地硬编码 /tmp、未发布依赖或
手动组件准备作为正式交付终点；尚未发布的 GOUI 改动可在维护者测试中使用源码替换。
B2/B3 可以按可用设备并行推进验证准备，但不提前修改 GUI 输入体系。

### 8.1 无设备测试

配置/target/variant/路径转义、组件身份与篡改、包依赖选择、未知 cgo、Go 版本/
源码不符、native flags 拒绝、取消/失败保留输出、构建记录/二进制错配、APK
manifest/Dex/签名与 ELF 对齐、iOS plist/profile/entitlements/Mach-O/IPA 结构。
逻辑测试放所属包；需要真实 SDK 制作的部分作为维护者显式测试，不在普通测试
里下载或安装 SDK。敏感材料用临时测试身份；不提交私钥或真实 profile。

### 8.2 真机例程

画面显示平台、后端、缩放、逻辑尺寸、启动 nonce、帧计数和各项检查结果；绘制
多语言文本、透明合成、clip、Transform、缓存图像。使用自身 Go 定时任务向
EventLoop 投递，不需要触摸控件即可观察变化。分别构建 Software/GLES；复用
正常后端选择机制，不为构建工具增加控件或主题设置。

操作说明写中文，记录安装、冷启动、持续绘制、旋转、系统 Home/返回、锁屏/
恢复、后台停绘、GC/栈增长、覆盖安装后再次启动和卸载步骤；逐项给出可观察
预期。记录具体设备、OS、页大小、后端、物理缩放和来源组件，不虚构已连设备。
触摸只操作系统导航，框架触摸/IME 保持未验，不因能操作系统就称输入适配完成。

联网用例验证 DNS + HTTPS（真实证书校验）和错误传播，文件用例验证系统分配
的应用私有目录读写。联网失败区分网络环境与库错误，不把私有证书全局放行或
外部网络暂不可达包装成通过；这些测试用于证明常用标准库可用，不新增 GUI 服务。

真机可用性以实际安装/运行结论为准。模拟器、LLD 成功、签名器 exit 0、包解析
通过分别记录；无设备或签名材料时明确保留缺口，不重复宣告“整体已经可用”。

## 9. 参考与证据范围

查阅日期 2026-11-05；以下是设计依据，除另列实测之外不构成本项目已验证承诺。

- [Flutter iOS deployment](https://docs.flutter.dev/deployment/ios)：区分构建产物、
  应用身份、签名和发布流程；采用这项职责分离。该官方流程需要 Mac/Xcode，
  不把它作为本项目“无需原生工具链”的实现证据。
- [gomobile build 源码](https://github.com/golang/mobile/blob/master/cmd/gomobile/build.go)：
  参考 Go main、平台产物和组包编排。复用 Go 产物模型，不照搬其 SDK 编译前提。
- Go 1.25.8 本机 GOROOT/src/net/cgo_android.go、cgo_unix_cgo.go、
  cgo_unix_syscall.go、os/user/cgo_lookup_syscall.go、crypto/x509/internal/macos：
  本篇按目标实际 go list 与源码核对标准库，不根据文件名猜测。在线对照见
  [Darwin net 源码](https://raw.githubusercontent.com/golang/go/go1.25.8/src/net/cgo_unix_syscall.go)。
- [AAPT2](https://developer.android.com/tools/aapt2)：资源编译/链接及独立宿主工具；
  [apksigner](https://developer.android.com/tools/apksigner)：签名、验证及先对齐后签名。
- [Android 16 KiB](https://developer.android.com/guide/practices/page-sizes)：ELF/ZIP
  对齐、运行时页大小与运行验收；本项目目前 API30 模拟器结果不能替代。
- [apksig-go](https://github.com/agusibrahim/apksig-go)：当前固定 v1.1.0，复用 APK
  对齐、v2/v3 签名及验证；另用官方 apksigner 作独立检查。项目采用 Apache-2.0。
- [Apple development profile](https://developer.apple.com/help/account/provisioning-profiles/create-a-development-provisioning-profile/)、
  [TN2415](https://developer.apple.com/library/archive/technotes/tn2415/_index.html)：
  设备、App ID、证书、授权与签名约束；工具不替用户获得苹果开发资格。
- [UILaunchScreen](https://developer.apple.com/documentation/bundleresources/information-property-list/uilaunchscreen)：
  优先评估字典式启动屏，仍须以目标真机验证。
- [go-macho](https://github.com/blacktop/go-macho)、
  [go-codesign](https://github.com/aluedeke/go-codesign)：当前固定 v1.1.258/v0.0.6，
  分别复用 Mach-O 签名和 profile/资源格式辅助；不用后者的完整原生签名入口。
  两者均采用 MIT。Mac 原生 codesign 是本轮独立验签参考，真机授权仍待实测。

## 10. 维护者本地 profile 覆盖

默认构建使用内置组件。维护者可准备以下只读目录，通过 -components 或
GOUI_MOBILE_COMPONENTS 覆盖；不要求普通应用维护这些原生细节：

```text
components/
  android-arm64/mobile.json
  ios-arm64/mobile.json
  ios-arm64-simulator/mobile.json
```

每个 profile 内可分别含 base、inputs、tools、host、platform 目录。mobile.json
的必要字段为 format=1、host_os、host_arch、target、variant、base、inputs、tools、
driver、linker、files；objects/libraries/frameworks 为所选平台的显式链接要求。
files 是每项实际输入相对 profile 目录的路径与 SHA-256。清单、子清单及其归档、
工具、执行依赖和辅助资源均列入；越界、软链接、摘要/host/target/variant 不符时报错。

旧 format 1 Android profile 可保留 aapt2、android_jar、signer、verifier 字段，
当前组包不读取或执行它们；只消费 dex 和 target_sdk 能力上限。iOS 使用
main.o/uikit.o、objc/resolv 及 UIKit/Foundation/CoreGraphics/
QuartzCore/OpenGLES/Security。SDK stub 只参与链接，不打入 App，不把完整 SDK
目录传给消费者。工具选择不是自由配置脚本，消费阶段不会调用生成器或 C/Java 编译器。

移动 FFI/渲染依赖现已使用正式版本：goexlib/cgo v0.1.2、官方 purego v0.10.2、
nanovgo v0.0.13。正常模块下载即可获取，不再需要移动版 purego 分支或依赖仓库的
本地 replace/go.work。开发尚未发布的 GOUI 本身时，仍可显式用 go.work 连接应用与
GOUI 源码；这不等于依赖私有 FFI 补丁。

Android net 的支持归档由维护者在 producer main 导入 net/http 后使用现有 export
制作，仍只导出实际需要原生编译的包。内置资源由这些经过校验的 profile 生成；
再分发许可审核和真机验收不能与 Go 依赖发布混为一谈。

App 构建/运行的可复现入口见
[mobile_std](../tests/window/platform/mobile_std/README.md)。产物与最终原生证据见
[收敛计划](PlanMobileConvergence.md#正式构建与官方标准库-app2026-11-05)。

## 11. 内置最小组件与纯 Go 组包（2026-11-06）

最终消费入口是一个 `goui` 命令。使用者安装 Go、配置应用及自己的签名材料后，
执行 `goui build -target android/arm64 -package apk` 或对应的 iOS 命令；无需
准备 SDK、NDK、JDK、LLD、原生编译器或 `-components`。本地组件参数保留给维护者
和自定义组件使用，不能继续成为默认构建的前置条件。

实现分三步，逐步记录验收结果：

1. Android 固定宿主的资源结构由 Go 按二进制格式编码，包含 Manifest、资源表、
   名称、版本、SDK 声明和图标；不需要预编译 APK 模板或二进制字符串替换。消费阶段不再
   调用 aapt2 或读取 android.jar。APK 签名、对齐与验签采用已验证的 Go 实现。
2. 原生必需 Go 归档、Dex、iOS 最小 main/协议对象及目标链接输入按职责拆分，
   压缩嵌入 Go 源码。共同目标数据不随 Go 版本重复存储，不包含宿主工具或完整
   SDK。固定输入的生成、来源、许可证及摘要随源码保留。Go 构建驱动以本机 Go
   现场生成并缓存，原生链接由同一 goui 程序调用内置 golink 完成。
3. iOS 的待重签 IPA 继续支持 `-unsigned`；正式签名已接入 Go 库，证书/profile
   由使用者提供，完整核对资源封印、entitlements 和签名结果。未完成真机验收
   前，不把 Mach-O ad-hoc 签名或模拟器通过当作已签名真机 IPA 的通过证据。

后续维护目标是最近两个 Go 大版本各自最新且通过验证的补丁版本。组件选择按精确
版本进行；当前已有验证基座仅为 Go 1.25.8，其他版本在补齐匹配归档、驱动源码
校验和运行验收前明确报错，不删除版本防护以假装兼容。

原生预编译范围沿用已验证边界，不把生命周期、窗口、图形或输入逻辑搬入预编译
壳。SDK 输入优先投影为目标 ABI 所需的数据；保留系统库依赖、重导出及库版本
信息，不能只因样例没有引用某个符号就删除其他受支持标准库所需的输入。
SDK 派生输入的来源与再分发条款仍须逐项记录，内嵌不改变这一要求。

基座的原生必需包按完整源码和依赖导出 ABI 校验。非空 module_version 表示发布
归档，继续精确锁定依赖版本；从工作源码导出的空版本归档不绑定应用引用它时的
发布标签，但模块路径和语言版本仍须相同。源码或依赖 ABI 不同仍拒绝消费，不能
为了发布版本复用而跳过这些校验。这样相同的固定平台入口不必随 GOUI 标签复制。

源码归档所在模块采用工作区形式的调试/导出路径，不附加模块版本后缀；实际模块
版本、BuildInfo、源码与依赖 ABI 校验保持独立。构建驱动精确校验 cmd/go 的
exec.go/gc.go 后应用临时 overlay，并为这种路径归一加入编译缓存标识；不修改
用户 GOROOT、Go 编译器或 runtime，不接受自定义版本的 cmd/go 源码。

资源模板参考 Go 官方 x/mobile 的二进制资源生成路径，格式依据 Android
`ResourceTypes.h`；维护者以 aapt2 dump、官方 apksigner 和模拟器安装作为独立
参考。消费验收使用空缓存和仅包含 Go/goui 的 PATH，禁止调用 C/Java 编译器、
aapt2 或外部链接器，并分别记录 APK、iOS device/simulator 的构建和运行结果。

### 11.1 预置数据体积与边界

下表为本轮生成数据的实际 gzip 字节数，不包含可执行构建工具本身。共享目标输入
不随 Go 版本复制；TBD 是目标系统库链接描述，不会放入 App。

| 数据 | 压缩字节数 |
| --- | ---: |
| Go 1.25.8 Android ARM64 基座 | 1,063,742 |
| Go 1.25.8 iOS ARM64 device 基座 | 135,385 |
| Go 1.25.8 iOS ARM64 simulator 基座 | 136,896 |
| Android 共享目标输入及 Dex | 234,595 |
| iOS device 共享目标输入及入口对象 | 752,733 |
| iOS simulator 共享目标输入及入口对象 | 753,082 |
| 合计 | 3,076,433（约 2.93 MiB） |

Android Dex 原始 18,848 字节，iOS 每个 variant 的两个入口/协议对象合计
24,496 字节。原生平台生命周期、窗口、排版、绘图仍由 platform 中的 Go 实现。
compiler-rt 保留经校验的归档，链接器按需抽取成员，不把整个归档装入 App；此轮
不为进一步缩减归档增加另一套原生对象裁剪机制。

命令的图标处理直接复用 oksvg，不导入 GUI、桌面窗口或 FFI 后端。Linux amd64
CGO_ENABLED=0 构建的 goui 为静态可执行文件。目标输入、Go 基座、宿主生成工具
各自缓存，校验失败就报错，不从已经修改的缓存重新生成可信摘要。

### 11.2 验收范围

Linux amd64 的冻结已跟踪源码加本轮改动，在独立 Xvnc/Xfwm4 会话运行
`CGO_ENABLED=0 GOWORK=off go test -p 4 -json -count=1 ./...`：4492 通过，
10 项原有条件跳过，零失败。未混入工作区的历史未跟踪测试和产物；源码清单及
摘要保存在本轮仓库外验收记录。cmd/goui、androidres、mobile/base 的 race 和
相关工具包 vet 通过。遗留包内窗口测试只在该独立显示会话创建窗口。

Windows10 build19045 amd64 和 macOS15.7.7 ARM64 均使用 Go1.25.8，限定构建
PATH 只含 Go，不提供 -components 或本地 FFI 分支，默认 APK/待重签 IPA 构建
及组件缓存、APK/iOS 签名逻辑测试通过。签名测试不导出私钥；Mac 上同时运行
系统 codesign 严格验签。未发布 GOUI 候选源码通过单一模块 replace 接入，不把
这种源码替换说成已经发布的 Go 模块。

Android API30 ARM64 / SwiftShader / GLES / 1x 和 iOS26.3 ARM64 Simulator /
GLES / 3x 均为 137 通过、1 项官方 short 跳过、零失败；两端 GLES 1x/2x
FBO 像素检查通过。APK 的资源与签名另经官方 aapt2/apksigner 检查。Go iOS
签名使用合成测试身份，经 macOS codesign 严格验签；这不是 Apple 设备授权。

物理 iPhone、真实开发证书/profile 的安装、Android 物理及 16 KiB 设备、更多
Go 版本、SDK 派生输入的再分发审核仍待完成。此轮保持本地开发，不发布组件。
