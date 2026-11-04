# 移动端构建：最小预编译基座

状态：基座、原生链接组件、移动 ARM64 FFI 修复和 Linux/Windows 消费构建
四个阶段已完成，记录见第 13–16 节。普通 Go 依赖仍从源码编译；Android/iOS
模拟器的 ABI 运行通过，iOS 真机和 Android 16 KiB 内核运行仍待验收。
正式平台启动与生命周期另见 [DesignMobileLifecycle](DesignMobileLifecycle.md)。
最新包结构、首版两端 GLES 和文字排版路线见 [DesignMobilePlatform](DesignMobilePlatform.md)。
尚未接入 GOUI 移动 UI 或改变面向用户的桌面构建命令。

开发隔离在 `/data/xuyang8/repos/goui2` 的 `feature/mobile-base` 分支；
`/data/xuyang8/repos/cgo` 与 purego 同名分支包含移动 ARM64 FFI 修复，不改主会话
的工作区。构建实现与当前限制见第 13–16 节，平台启动见上述生命周期文档。

## 1. 目标与约束

应用开发者只编写 Go 代码，在 Windows/Linux 上通过 `goui` 构建 Android/iOS
应用，不需要安装、配置或了解 C 编译器、NDK、Xcode、Gradle 等原生构建环境。
维护者可以使用这些工具制作经过验证的预编译产物，`goui` 可以携带或获取
链接器、归档工具、签名工具及目标平台必需的链接输入。

这里的 no-cgo 指应用开发体验：不要求应用源代码使用 `import "C"`，也不要求
使用者提供 C 编译链。内部允许 `CGO_ENABLED=1`，允许复用原生编译产物和
Go 原有的 `runtime/cgo`。不为了保持内部环境变量为 0 而重新实现线程、TLS、
GC 或原生回调接入。

优先复用 Go 编译器、汇编器、链接器、runtime 和成熟的 FFI 实现。确有必要时
扩展构建驱动；不预先决定维护一个新编译器、runtime 或完整链接器分支。

首批目标为 Android ARM64 与 iOS ARM64 真机；模拟器、其他架构、任意第三方
cgo 包和商店自动发布不是第一阶段承诺。

## 2. 职责边界

预编译不是一个承载所有移动端功能的框架宿主。必须将以下部分分开制作、
分开标记版本，并按依赖关系选择组合。

| 部分 | 职责 | 明确不负责 |
| --- | --- | --- |
| 运行时基座 | `runtime/cgo` 等确实需要原生编译的固定支持包；必要的最小原生 ABI 接入 | UI、Activity/Scene 策略、事件循环、窗口和 Widget |
| FFI 支持 | 复用 purego/goexlib 的符号调用与回调；仅其实际使用 cgo 的小包需要预编译 | 构建调度、平台生命周期、打包签名 |
| 链接工具与目标输入 | LLVM 链接器、归档工具、必要的 CRT/系统库 stub/链接配置 | 原生 UI 适配和 GUI 策略 |
| `goui/platform` | Android/iOS 原生事实、窗口/视图、线程归属、生命周期、输入、绘制与排版 | 构建工具、用户界面策略 |
| 平台宿主产物 | platform 所属的少量必要 C/Objective-C/Java 接入代码，独立预编译 | 通用 runtime 初始化的重新实现 |
| `cmd/goui` | 选择并校验组件，驱动 Go 构建及原生链接，生成资源、组装与签名 | 运行时 UI 差异、应用事件处理 |

运行时基座应能用一个不依赖 GOUI 的普通 Go 程序验证。它不能导入
`goui/platform`、`gui` 或 `ui`，也不包含应用 ID、图标、权限或业务代码。

平台宿主即使随工具分发，源码和行为仍归 platform。Android NativeActivity
入口、iOS 应用入口与 UIKit 适配不能因为需要预编译而移入通用基座。
平台 UI 的变化只重建对应宿主组件，不应强制重建通用运行时支持。

## 3. 最小化预编译内容

### 3.1 只预编译不能由普通 Go 编译阶段完成的部分

首个基座优先采用匹配 Go 版本的完整 `runtime/cgo` 包归档。它不仅包含 C
目标文件，还包含 Go/汇编部分、生成代码、导入导出指令及依赖元数据；不能
仅复制几份 `.o` 就视为替代了这个包。

原则上，以下内容继续从源码正常编译：

- 应用代码及普通第三方 Go 包。
- GOUI 的 `core`、`layout`、`gui`、`ui` 与 platform 的 Go/汇编实现。
- 纯 Go 标准库包与 FFI 的 Go/汇编部分。

确实包含 cgo 的 FFI 辅助包、固定 ABI 入口或平台宿主单独制作。不要把整个
GOUI、整套标准库或完整 SDK 都作为基座预编译和强制捆绑。
普通 Go 归档仍可使用正常构建缓存；缓存与必需分发的原生基座是两个概念。

基座制作需要 NDK/Xcode 并不意味着消费基座仍需要它们。消费阶段只保留
链接所需的具体输入，不运行头文件预处理、cgo 类型探测或 C/Objective-C 编译。
需要额外原生支持的第三方包必须有匹配产物；没有时明确报告包路径与缺失
组件，不静默调用用户机器上的 C 编译器。

### 3.2 不按应用内容制作基座

应用代码通过普通 Go 导入关系连接基座，原生符号表只有固定入口，不为每个
业务函数生成一份 C 导出包装。注册 Go 回调可以由 Go 包初始化完成，但初始化
只登记入口，不启动 GUI、不阻塞等待应用事件循环。

库初始化和进入应用是两个步骤。复用 Go 的运行时初始化及原生线程接入；
具体从哪个平台入口启动应用、如何满足 UIKit 主线程要求，由 platform 设计。
不从桌面版本的 `init`/`LockOSThread` 行为推断移动端主线程已经正确。

### 3.3 兼容性身份

每个归档至少记录并校验：

- 精确 Go 工具链版本/构建身份、目标 OS/ARCH、架构特性与编译模式。
- 源码及生成输入摘要、依赖包身份、Go 导出数据与 ABI 相关构建选项。
- FFI 模块版本、基座 ABI 版本，以及相关原生 API 最低版本。
- 生产时使用的 SDK/工具版本、链接输入清单和文件 SHA-256。

不使用“同为 ARM64”或“Go 小版本接近”作为兼容依据。Android Bionic 与 Linux
glibc 的归档不能混用，iOS 与 macOS 的 ARM64 归档也不能混用。
同一目标归档可以被不同宿主上的匹配 Go 工具消费，但这仍需跨宿主实测。

## 4. purego 的参考与复用

本仓库当前依赖 purego v0.10.0。应分别看待它解决的三个问题：

| 机制 | 对本方案的价值 | 不能据此推断 |
| --- | --- | --- |
| Go/汇编实现 ABI 调用与回调 | 平台绑定继续用 Go 编写，避免每个系统 API 都生成 C 包装 | 已经解决动态库链接、应用宿主或签名 |
| `CGO_ENABLED=1` 时复用 `runtime/cgo` | 可与预编译运行时支持组合，无需自行重写运行时接入 | 应用构建仍必须运行 C 编译器 |
| `CGO_ENABLED=0` 时的 `fakecgo` | 说明运行时桥接能与 UI 层独立，也提供线程/TLS 接入的实现参考 | 已完整支持 Android/iOS 的所有运行时要求 |

源码证据：

- [cgo.go](https://github.com/ebitengine/purego/blob/v0.10.0/cgo.go)
  在 cgo 模式导入 `runtime/cgo` 和自己的 `internal/cgo`。
- [nocgo.go](https://github.com/ebitengine/purego/blob/v0.10.0/nocgo.go)
  说明 fakecgo 的运行时 hook/TLS 作用，明确该模式尚不支持 iOS。
- [dlfcn_android.go](https://github.com/ebitengine/purego/blob/v0.10.0/dlfcn_android.go)
  的动态库操作委托给自己的 cgo 辅助包。
- [go_runtime.go](https://github.com/ebitengine/purego/blob/v0.10.0/go_runtime.go)
  及 [syscall_sysv.go](https://github.com/ebitengine/purego/blob/v0.10.0/syscall_sysv.go)
  展示了调用/回调与 Go runtime 的衔接。

因此优先组合为：**标准 Go runtime + 预编译 runtime/cgo/必要 FFI 辅助包 +
源码编译的 FFI 和 platform Go 代码**。这既借用了 purego 的思路，也复用其
现有 cgo 模式，不要求先移植一整套移动端 fakecgo。

同一个进程不同时安装两套运行时 hook。若未来要改用 fakecgo，需要独立证明
TLS、外来线程、线程退出、信号/异常、GC、栈增长以及库初始化都满足目标系统，
再替换对应基座；它不是本阶段的前置条件。

`NewCallback` 的注册资源按其实际实现管理。v0.10.0 的回调注册数量有限且
不回收，平台应复用稳定的回调入口，不随每帧、每次事件不断创建新入口。
FFI 可复用不等于所有平台 ABI 类型都已验收。

后续平台 API 的结构体、浮点、回调签名仍遵守现有 goexlib/cgo 绑定规范。
不能因为固定入口只传整数，就把所有平台接口强行压成整数调用。

## 5. 编译、链接与组件交付

### 5.1 复用两阶段链接

```text
应用 / GOUI / 普通 Go 依赖源码
                   + 匹配的预编译原生支持包
                            |
               Go 依赖分析、编译、汇编、链接
                            |
               Go 链接器生成原生 go.o 等输入
                            |
             预置原生链接器 + 必要目标链接文件
                            |
                Android .so / iOS 可执行文件
                            |
              资源生成、宿主组装、签名与封装
```

Go 编译器输出的包 `.o`/`.a` 不能直接当作普通 C 目标文件全部交给 LLD。
Go 链接器仍负责 Go 符号、初始化、类型、GC 等信息，再完成原生链接。
这一边界见 [Go cgo 的外部链接说明](https://go.dev/src/cmd/cgo/doc.go)。

Android 优先沿用 `c-shared`。iOS 优先沿用 `c-archive`，将归档和独立平台宿主
链接为主可执行文件，不把“iOS 必须先支持 Go c-shared”设为前置条件。
依据是本机 Go 1.25.8 的 `internal/platform/supported.go`：iOS 支持 c-archive，
未列入 c-shared 支持范围。最终支持表要与实际冻结的 Go 版本一起验证。

如果以后改为嵌入动态库，需要另行满足 Apple 的 framework 约束，不能把
Android 的裸 `.so` 布局直接替换成任意 `.dylib` 后照搬到 iOS。
参考 [Apple TN2435](https://developer.apple.com/library/archive/technotes/tn2435/_index.html)。

### 5.2 预编译包接入是独立开发点

必须做到：普通 Go 包照常编译，指定的原生支持包由已校验归档满足，冷缓存
时也不重新运行这些包的 cgo/C 编译动作。不修改用户 GOROOT，不依赖恰好存在
的热缓存，不伪造编译器执行成功。

本轮实验确认：在 Go 1.25.8 中，仅给 `go build` 传 `-pkgdir`，不足以自动
实现上述行为，仍会尝试运行 runtime/cgo 的 cgo 工具和 C 编译器探测。

第一阶段已在匹配版本的 `cmd/go` 包构建 action 上增加窄范围适配，继续使用
原有包发现、构建约束、依赖调度、importcfg、缓存与编译参数，详见第 13 节。
它不重写整套 Go 构建系统，不改变原有 Go 编译器、链接器或 runtime。

第 9～12 节手动提供 importcfg 的早期实验只证明归档能够被 Go 工具消费；
第 13 节则另行验证了真正的冷缓存构建图，不能混用两类证据。

### 5.3 原生链接工具保持独立

首选成熟 ELF/Mach-O 链接器及归档工具，例如 LLVM 的 `ld.lld`、`ld64.lld`、
`llvm-ar`；[LLD 官方文档](https://lld.llvm.org/)说明了跨目标链接能力。
需要时提供小型驱动适配 Go 外部链接参数，不重新实现目标文件格式。

Go 外部链接参数包含 GCC/Clang 驱动形式的参数和可选能力探测，不能简单把
`-extld` 换成原始链接器路径。正式适配要校验参数、处理 response file、明确
不支持项，并保留目标平台的初始化、导出、安全选项与依赖库配置。
能力探测不等于可以临时调用 C 编译器；链接器驱动应如实拒绝不支持的源文件
编译请求，必要时在 Go 驱动侧改用与链接器相符的探测方式。

CRT、compiler support、系统库 stub、最低系统版本和页大小配置独立于 UI
产物。不能靠读取构建宿主上偶然存在的 NDK/Xcode 路径来补全缺失输入。
仅携带确实需要且可分发的内容；SDK 材料的再分发范围需单独确认。

### 5.4 按需组件与可重复构建

工具可内置必需小组件，也可按目标获取冻结版本的大组件，提供离线组件目录。
区分宿主工具与目标归档：同一 Android ARM64 基座不应因构建宿主不同重复制作，
但链接器/签名工具需要提供对应宿主的可执行文件。

组件清单至少分开列出 runtime、FFI 辅助、link support、platform host、
packaging tools，各自有来源、版本、依赖与摘要。损坏、缺失或不兼容时明确
失败，不偷偷回退到系统原生工具链。

不修改全局 Go 配置。签名证书、profile 和密码不进入公共基座、构建缓存或
日志；最终包使用应用开发者自己的身份重新签名。

## 6. platform 与构建工具的交界

本阶段不设计新的移动端 Widget、窗口或事件循环公共接口。
后续 platform 分别负责：

- Android 的 Activity/Surface 生命周期、主线程回调、JNI 归属和输入。
- iOS 的主线程、应用/Scene 生命周期、视图与原生输入。
- Go 应用入口与平台事件循环的实际协调，以及原生对象的创建/释放顺序。

基座只提供已经初始化的运行时与可信的 ABI 进出路径，不自行判断“应该创建
几个窗口”“应该在哪个线程绘图”“Activity 重建是否重启应用”等问题。
构建工具只组合 platform 声明的宿主输入，不根据 UI 类型补偿启动行为。

NativeActivity、UIKit 宿主、IME 辅助等原生源码可以作为独立平台组件预编译；
不要求所有系统功能都从一个固定宿主模板中实现，也不要求用 NativeActivity
本身覆盖 Android 的所有文字输入能力。

Android/iOS 真机启动和 SDK/商店要求独立验收。Linux ARM64 ELF 在 QEMU 运行
不能代替 Android Bionic/TLS 的验证，更不能代替 UIKit 主线程和签名验证。

## 7. cmd/goui 的未来接入

目标体验为：

```sh
goui doctor -target android/arm64
goui build -target android/arm64
goui doctor -target ios/arm64
goui build -target ios/arm64
```

移动端 `build` 计划输出可安装包，内部阶段可独立缓存。它是未来的目标专属
契约，不表示当前命令已经具备此能力；桌面裸二进制 build 与 package 的现有
行为保持不变。具体资源、签名配置在对应打包切片补齐，不借本轮扩展权限系统。

职责上可分为组件选择/校验、Go 构建、原生链接、Android/iOS 组装与签名。
不要提前把预编译包接入实现塞进 platform，或把原生生命周期塞进命令包。
跨宿主打包实现应使用 `android.go`、`ios.go` 这类普通文件名；不能误用会限制
编译宿主的 `_android.go`、`_ios.go` 后缀。

参考 [Flutter 预构建 AAR 的集成](https://docs.flutter.dev/add-to-app/android/project-setup)
来区分可复用产物与宿主集成；不因此引入 Flutter 的整个构建结构。
[Qt Android](https://doc.qt.io/qt-6/android-getting-started.html)仍要求相应开发环境，
其部署流程可以参考，但不能作为本设计已经消除用户工具链要求的证据。

## 8. 分阶段验证计划

| 阶段 | 目标 | 验收边界 |
| --- | --- | --- |
| P0 | 仓库外验证预编译支持包和纯 Go 应用重新组合 | 不依赖 GOUI，无需移动端 UI；不能声称支持移动端 |
| P1 | 冷缓存的预编译包接入 | 禁止 C 编译器调用，普通 Go 包从源码编译；不靠保存整套依赖归档绕过调度 |
| P2 | Android ARM64 基座与最小动态库 | 提供全部目标链接输入，检查导出、初始化、依赖、重定位、4K/16K 页兼容 |
| P3 | Android 无界面最小 APK | 加载、原生线程回调、GC、栈增长、进程/组件重建；至少一台真机 |
| P4 | iOS ARM64 归档与最小宿主 | 独立 Go 归档、Mach-O 链接、主线程回调、签名、安装、运行；不依赖完整 GOUI |
| P5 | 接入 cmd/goui | Windows/Linux 清洁环境构建，SDK/编译器缺失时仍可完成；组件错误可诊断 |
| 后续 | platform UI、绘制、文字、输入与系统能力 | 分别设计、实施、真机验收，不混入通用基座的完成定义 |

P1 还要验证标准库 cgo 分支与第三方 FFI 依赖的发现，不能因为最小用例只用了
runtime/cgo，就声称任意纯 Go 应用的依赖图都不会再触发原生编译。
网络、系统 DNS、HTTPS、文件与权限在后续对应切片按目标验证；不随意强制
某些 Go build tags 来掩盖缺失的系统集成。

## 9. 首轮最小验证记录（2026-11-03）

本节记录最初的 Linux ELF 机制验证；后续移动端目标的结果见第 11 节，
不以本节的环境或通过项代替移动端验证。

### 9.1 环境与隔离

- 本机 Linux amd64，Go 1.25.8，GCC 11.4.0，GNU ld 2.38。
- ARM64 对照使用已有 `aarch64-linux-gnu-gcc`/`ld` 与 QEMU 6.2.0，
  动态加载器目录 `/usr/aarch64-linux-gnu`。
- 未安装或调用 Android NDK、Android SDK、Xcode，也没有构建 APK/IPA。
- 实验程序、脚本、缓存、日志、归档和二进制全部在仓库外 `/tmp`。
- 生产阶段允许 C 编译；消费阶段的 PATH 只含拒绝执行的编译器入口，
  CC/CXX 指向拒绝并记录调用的脚本，Go 编译器/链接器与原生链接器用绝对路径。
- 消费使用独立空 GOCACHE；构建后该目录仍无缓存文件。

“PATH 不含编译器”是本轮受控命令的验证方法，不是系统级隔离证明。实际
消费脚本调用链已检查，仅运行 Go 工具、Python 链接参数适配和原生 ld。
脚本语言和本机 GNU 工具仅用于 PoC，不是未来用户依赖。

### 9.2 固定基座实验

维护者阶段编译 `runtime/cgo` 与固定 cgo 桥接包。桥接包只导出
`int64_t goui_probe(int64_t)`，通过在 Go 内注册的函数执行应用逻辑。
它不依赖 GOUI、Android 或 UIKit。

消费阶段使用同一份基座分别构建：

```text
应用一：f(x) = 3*x + 7
应用二：f(x) = 3*x + 19
```

只重新编译纯 Go main 包，复用固定原生支持；Go 链接器生成原生输入后，由
仅做链接的参数适配器调用 GNU ld。两个版本的基座 SHA-256 始终不变，
最终动态库摘要不同，运行结果分别符合各自预期。

回调内有递归栈增长和 `runtime.GC()`。amd64 通过 ctypes 在各自独立进程
加载两个版本，每个版本执行 4,097 次调用，使用 8 个工作线程；ARM64 的
预编译 C 验证宿主在 QEMU 下分别加载两个版本，每个版本执行 1,025 次调用，
使用 8 个 pthread。全部结果断言通过。

| 检查 | linux/amd64 | linux/arm64 + QEMU |
| --- | --- | --- |
| 原始 Go 编译器、链接器生成可加载动态库 | 通过 | 通过 |
| 两份纯 Go 应用复用相同基座 | 通过 | 通过 |
| 消费阶段 C 编译器调用 | 0 | 0 |
| 原生外来线程、GC、栈增长 | 通过 | 通过 |
| 移动端链接、加载、签名和 UI | 未执行 | 未执行 |

`runtime/cgo` 与固定桥接包的归档原始大小合计分别约为 700 KiB / 707 KiB。
这不是移动端成品大小、安装包大小或性能结论。

### 9.3 冷缓存 stock go build 对照

以同样的源码与包归档运行，归档按导入路径放置，例如
`<归档目录>/runtime/cgo.a`、`<归档目录>/example.com/mobileprobe/bridge.a`：

```text
CGO_ENABLED=1 CC=<拒绝脚本> GOCACHE=<新目录>
go build -buildmode=c-shared -pkgdir <归档目录> ...
```

Go 1.25.8 仍调用 C 编译器的预处理/探测入口，随后失败：

```text
runtime/cgo: .../pkg/tool/linux_amd64/cgo: exit status 2
```

该负向对照的拒绝脚本记录了 `-### -x c -c -` 和 `-E -dM ...` 请求。
它说明本轮提供的归档目录不能自动取代 stock go 的原生构建动作；P1 的
构建驱动适配仍需实现和验收，不能依赖 `-pkgdir` 名称作出可用性承诺。
本机 `cmd/go/internal/work/exec.go` 的 buildActionID 还会把 C 编译器身份纳入
cgo 包的动作摘要。因此，支持清洁机器上的预编译归档需要明确的接入契约，
不能仅搬运某台生产机器的包文件与缓存键。

### 9.4 purego 组合实验

在相同方法中加入仓库当前使用的 purego v0.10.0，仅增加其实际需要 cgo 的
`github.com/ebitengine/purego/internal/cgo` 预编译归档。桥接包仍然只负责
固定原生入口；应用中的 Go 逻辑执行：

1. `Dlopen` 打开系统 libc，注册并调用 `getpid`，断言与验证宿主进程 ID 一致。
2. `NewCallback` 生成固定 Go 回调，原生验证宿主按返回的函数地址直接调用，
   断言输入输出正确。
3. 通过 `RegisterFunc` 从 Go 调用上述原生 ABI 回调，形成
   `原生宿主 -> 固定入口 -> Go -> purego 调用 -> 原生 ABI 回调 -> Go`。
4. 最内层 Go 回调分配动态大小缓冲区，递归触发栈增长并校验递归结果，
   部分调用执行 `runtime.GC()`，返回前保持缓冲区有效。

同样构建 `3*x+7` 与 `3*x+19` 两份 main，不改变三份原生支持包。
消费阶段 C 编译器调用为 0，GOCACHE 仍为空。
amd64 每个版本通过 4,097 次循环/负值检查，ARM64/QEMU 每个版本通过 1,025 次，
均有 8 个工作线程；另有直接函数指针回调和 getpid 断言，全部通过。

三份支持包的原始归档合计约为 amd64 774 KiB、ARM64 780 KiB。
完整实验仍保存了普通 Go 依赖归档：amd64 65 个共约 30.5 MiB，
ARM64 66 个共约 31.1 MiB。这些数字只描述本次受控实验，不是产品分发
大小，也不证明生产方案必须捆绑全部 Go 依赖。

该实验验证了 purego 的现有 cgo 路径可以与预编译支持包组合；没有验证
Android/iOS 的 fakecgo，也没有验证任意 FFI 签名、原生对象或移动端系统库。
每个版本单独启动验证进程，不测试进程内多个 Go runtime 或卸载 Go 动态库。

### 9.5 当前结论的限制

本轮手动 importcfg 同时引用了保存的 42 个依赖归档，约 18.4 MiB；其中
大部分是普通 Go 标准库，并不属于计划分发的最小原生基座。消费阶段仅重新
编译 main。这验证了已有归档和新应用能够正确组合，尚未证明“普通 Go 依赖
全部从源码重建、仅替换原生支持包”的完整冷构建流程。

Go 链接器还发出了每个版本 3 次 `trivial.c` 可选参数探测。仅链接驱动真实
拒绝了这些源文件编译请求，没有伪造探测成功，没有调用 C 编译器；最终
链接成功。这项探测差异需在正式链接驱动中处理。

ARM64 GNU ld 报告 `warning: -z relro ignored`。功能调用通过不意味着 ELF
安全配置通过，本轮没有将该产物作为可分发移动端基座；正式目标链接配置
及安全属性仍是 P2/P4 的验收项。

没有执行全仓 `go test ./...`：本轮不修改库实现，验证对象是仓库外的独立
构建机制；现有工作区还包含主会话的开发改动。本记录不能替代后续正式
实现完成时的全仓、跨宿主和移动设备验证。

### 9.6 实验文件位置

以下临时路径用于本机追溯，不作为长期构建输入；清理临时目录后需按上述
步骤重新制作基座和 importcfg。

- amd64：`/tmp/goui-mobile-base-tkj2gx7v/`。
- ARM64：`/tmp/goui-mobile-base-arm64-0hin6yzu/`。
- 生产记录：`producer.log`、`environment.json`。
- 消费逻辑：`consume.py`、`link-driver`、`compile.importcfg`、`link.importcfg`。
- 证据：`base-hashes.json`、`consumer-link-args.jsonl`、各版本 `step-*.log`。
- amd64 回调检查：`check_amd64.py`。
- ARM64 回调检查：`native_host.c`、`qemu-results.json`。
- stock go 对照：amd64 目录中的 `stock-tree-build.log`、`stock-tree-cc-calls.jsonl`；
  `stock-pkgdir/` 是按导入路径组织的归档目录。
- purego 组合：`/tmp/goui-mobile-purego-j60linen/`，`run.py` 分目标制作与消费，
  `amd64/check.log` 和 `arm64/qemu-results.json` 保存运行断言结果。

## 10. 本轮进度与下一步

- [x] 收敛用户 no-cgo 的含义，允许内部预编译 cgo 支持。
- [x] 将通用基座、FFI、链接输入、平台宿主和打包工具分离。
- [x] 用仓库外普通 Go 程序验证 amd64 与 ARM64 的归档复用和原生回调。
- [x] 记录 stock go build 的接入限制与尚未覆盖的验证边界。
- [x] 直接复用 purego v0.10.0 的 cgo 路径，验证符号调用、原生回调与嵌套调用。
- [ ] 设计并验证最小预编译包接入点，不修改用户 GOROOT。
- [x] 制作仓库外 Android ARM64 运行时/FFI 支持与最小目标链接输入。
- [x] 在 Linux 消费 Android/iOS 归档并重新链接，验证预编译基座与 FFI 的职责分离。
- [ ] 修复并回归当前 purego/goexlib/cgo 的移动端 ABI 缺口。
- [x] 验证 Android 最小动态库/APK、签名和完整 ARM64 模拟器运行。
- [x] 验证 iOS c-archive、最小 UIKit 宿主、跨宿主链接与模拟器签名安装。
- [ ] 验证 Android/iOS 真机运行与正式分发签名。
- [ ] 达到上述门槛后，另行设计 platform 移动端 UI 接入和 cmd/goui 集成。

本阶段的下一开发对象是构建驱动对最小基座的接入，不是新的 GUI 生命周期
抽象，也不是将全部平台功能固化进预编译宿主。

## 11. Android/iOS 目标验证（2026-11-03）

### 11.1 范围与版本

继续使用 Go 1.25.8、purego v0.10.0、goexlib/cgo v0.1.1。测试不依赖 GOUI，
不修改 GOUI、goexlib/cgo 仓库、Go 安装目录或 Go 模块缓存源码。
只更新本设计文档；实验源码、编译产物、原生支持、补丁候选与日志仍在仓库外。

| 对象 | 环境与实际验证范围 |
| --- | --- |
| Android ARM64 | API 29 目标；NDK r27d 的最小头文件、CRT、链接 stub、builtins；Clang/LLD 14 仅用于维护者制作支持包 |
| Android 执行 | QEMU 6.2.0 用户态 + AOSP 实际 linker64/Bionic/liblog；不是完整 Android 模拟器或设备 |
| iOS ARM64 生产 | 远端 macOS 15.7.7、Xcode 26.3、iPhoneOS/iPhoneSimulator 26.2 SDK |
| iOS ARM64 消费 | 本机 Linux、Go 1.25.8、llvm-ar 14、LLD 22.1.8；分别链接 iOS 与 iOS Simulator 可执行文件 |
| iOS 执行 | 独立 iPhone 17 Pro ARM64 模拟器，iOS 26.3.1；执行无 UI 的原生验证宿主 |
| 真机 | Mac 未发现已连接设备；未执行 Android/iOS 真机验收 |

Android 维护者输入通过官方 NDK r27d ZIP 的范围读取提取，约 4.8 MB 下载，
没有安装完整 NDK 或 Android Studio。保存各文件的大小、ZIP CRC 和 SHA-256；
这不等于已经校验整个 NDK ZIP 的官方摘要，也不是正式组件交付的信任链。
Apple SDK 仅在维护者端生产，Linux 消费端仅得到 `.tbd` 链接输入和预编译宿主
对象，没有复制 Xcode、完整 SDK 或头文件；这些私有验证材料不进入仓库或发布包。

Android 执行使用实际 AOSP 运行库，没有把 NDK 的导出 stub 当作运行库。
`liblog.so` 来自 AOSP prebuilts/runtime 的固定提交
`889e7cb78ec7268b096f285f3dce8885ed06719d`；其他 Bionic 组件来自下载的 runtime APEX，
其文件摘要和来源记录在验证收据。该组合用于 ABI 验证，不代表某个完整 Android
版本的系统镜像；没有 linker namespace 配置时的警告保留在日志中。

### 11.2 编译与链接结果

Android 采用 c-shared；iOS 真机与模拟器采用各自独立的 c-archive 支持包。
从同一支持包重新构建两份纯 Go main，使用不同的版本标记检查应用代码确实
重新编译，避免误把生产阶段的种子程序当作新产物。

Linux 成功消费 Mac 制作的 Go 包归档：重新编译 main，调用 Go 链接器生成
归档，再使用本机 LLD 和预编译原生宿主对象生成 iOS/iOS Simulator Mach-O。
两种目标的 LC_BUILD_VERSION 分别断言为 iOS 和 iOS Simulator。
这验证了目标归档可跨构建宿主复用，不要求消费端再次运行 Apple 编译器。

三个目标的消费均满足：

- C/Objective-C 编译器调用记录为 0，CC/CXX 使用拒绝脚本，PATH 不提供编译器。
- 独立消费 GOCACHE 为空。
- 原始基座归档 SHA-256 不变；使用不同 main 可以得到不同程序。
- Android ELF 的 PT_LOAD 对齐为 `0x4000`，具有 GNU_RELRO 和不可执行栈。
  这里只验证文件属性；未在真实 16 KiB 页内核上执行。
- iOS 的 `__DATA_CONST` 设置 SG_READ_ONLY，模拟器产物的 ad-hoc 签名校验通过。
  未验证开发证书、profile、IPA 安装或商店接收。

普通 Go 依赖仍通过显式 importcfg 和保存的归档提供。因此上述结果仍不是
P1 所要求的完整冷构建调度，也没有实现 `goui build`。

### 11.3 独立 C 参考与 ABI 用例

goexlib/cgo 是本轮 FFI 验证的直接入口，调用与回调均使用其公开接口。
真实 C 函数由目标 C 编译器生成，提供独立的输入与期望；不只将 Go 调用桥与
Go 回调桥互相对照，以免两边相同的错误互相抵消。

| 用例 | 内容 |
| --- | --- |
| 1 | double/float/整数混合参数与浮点返回 |
| 2 | 两个 double 的 HFA 结构体参数与返回 |
| 3 | uint64 + double 的非 HFA 混合结构体参数与返回 |
| 4 | 大结构体按值参数与隐藏返回地址 |
| 5 | C 调用 Go：混合浮点参数与浮点返回 |
| 6 | C 调用 Go：HFA 结构体参数与返回 |
| 7 | C 调用 Go：非 HFA 混合结构体参数与返回 |
| 8 | C 调用 Go：大结构体参数与返回 |
| 9 | C 调用 Go：寄存器耗尽后的窄整数栈参数 |
| 10 | C 调用 Go：整数和浮点分别超出寄存器数量 |
| 11 | Go 调用 C：窄整数栈参数，经 `CallRet(...any)` |
| 12 | GC、递归栈增长和内存存活校验 |

每份程序先逐项断言，再由 8 个原生 pthread 各执行 8 轮已通过的用例。
失败项明确计入总结果，不加入并发循环并不表示跳过或通过。
每个平台分别运行两份版本标记不同的应用，结果一致。

### 11.4 当前依赖版本的实测结果

| 环境 | 原始依赖 | 仓库外平台判断修正候选 |
| --- | --- | --- |
| Android ARM64 + Bionic/QEMU | 9/12；2、3、4 失败 | 11/12；3 失败 |
| iOS ARM64 模拟器 | 7/12；2、3、4、9、11 失败 | 11/12；11 失败 |
| iOS ARM64 真机目标 | 仅编译与链接 | 仅编译与链接 |

原版失败全部保留，没有将候选补丁的结果替换为发布版本的结果。
本轮不能据此宣布当前依赖已完整适配移动端。

已定位的缺口：

1. purego `func.go` 的结构体支持判断只列出 `darwin`/`linux`，没有包含
   `ios`/`android`。Go 的相关构建标签会选择桌面同族实现，但运行时 GOOS
   仍是移动端名称，主动结构体调用因此被拒绝。
2. purego 的 ARM64 栈布局判断，以及 goexlib/cgo `callback_arm64.go` 的
   `aapcsStackPush`，仅在 `runtime.GOOS == "darwin"` 时采用 Apple 紧凑布局。
   iOS 的窄整数回调参数因此取错偏移。候选将 iOS 纳入同一 ABI 分支后，
   C 原生调用的窄整数回调通过。
3. Android 的非 HFA 混合结构体在 purego `placeRegistersArm64` 中仍按字段
   分配整数/浮点寄存器。测试的 `{uint64, double}` 应按 AAPCS64 的非 HFA
   复合类型规则传递，实测返回 `{7, 1.25}`，预期 `{17, 1.75}`。
   这不是导出表或动态库初始化问题，也不能靠结构体支持白名单修正。
4. iOS 的 `CallRet(...any)` 转发到 purego 的 `func(args ...any)` 路径时，展开
   `[]any` 后直接逐项放置参数，绕过后续 Apple 紧凑栈打包路径。
   在同一个 C 函数上的独立对照为：固定签名 `RegisterFunc` 得到 54，
   `CallRet(...any)` 得到 65815，预期均为 54。

最后一项说明：不能把 `CallRet` 换成固定签名调用后就声称通用调用接口已修好。
修复应在 FFI 包内明确处理真实原生函数的参数布局，不能要求 GUI 控件补偿。
混合结构体和紧凑栈参数也应回归 Linux/macOS ARM64，避免只补移动端白名单。

规则依据为 [AAPCS64 2025Q1](https://github.com/ARM-software/abi-aa/blob/2025Q1/aapcs64/aapcs64.rst)
及 [Apple ARM64 调用约定](https://developer.apple.com/documentation/xcode/writing-arm64-code-for-apple-platforms)。
上述具体结果来自本轮原生 C 参考实测，不能由 API 名称或构建成功推导。

### 11.5 临时修正与基座隔离

只在临时文件中扩展 purego 三个 Go 文件和 goexlib/cgo 一个 Go 文件的 OS 判断，
没有修改真实仓库或模块缓存。
stock go 拒绝对 GOMODCACHE 下文件应用 overlay；该失败保留在实验日志中。
为继续验证，将临时 Go 源文件交给原始 Go 编译器，并复用生产阶段的 symabis
和未改变的汇编对象，再打包为临时 FFI Go 归档。

两份 FFI Go 归档重建后，`runtime/cgo`、purego 的 `internal/cgo` 和固定测试
桥接包仍直接使用原始归档，文件摘要不变；消费过程没有运行 C 编译器。
候选仅验证根因，不作为正式兼容实现。剩余混合结构体和通用栈参数错误尚未
在真实依赖中修复。

这也验证了职责分离的实际价值：移动端 FFI 的 Go 层修正不要求重做通用
运行时支持、重新生产整套 SDK 或把 UI 适配并入基座。

### 11.6 链接器版本的实际约束

LLVM 14 成功生成 Mach-O，签名校验也通过，但 iOS 26 dyld 在进入 Go 前中止：

```text
__DATA_CONST segment missing SG_READ_ONLY flag
```

改用本机 Rust 工具链携带的 LLD 22.1.8 后，重新链接的文件具有正确段标志，
同一模拟器成功进入 Go 并执行 ABI 用例。没有手工修改 Mach-O、取消只读段或
绕过 dyld 校验。正式工具应选定并验证独立分发的 LLVM 版本，不要求用户安装 Rust。
这一行为与 [LLVM 的 SG_READ_ONLY 实现变更](https://github.com/llvm/llvm-project/commit/f7b752d27766ecf0241edfbdc4d7142fc40e7621)
一致；本记录不推断所有介于 14 和 22 之间的版本都已验收。

### 11.7 证据、清理与剩余门槛

本机与远端 Mac 的独立实验目录为 `/tmp/goui-mobile-targets-oc7lzp5l/`。
本机保留：

- `make_probe.py`、`src/`、`native_host.c`：测试程序与独立原生参考。
- `android-inputs-manifest.json`：维护者制作基座所用的最小 NDK 输入。
- `validation-receipt.json`：目标文件、Mach-O 检查和实际 Android 运行库摘要。
- `android/bionic-original-results.json`、`android/bionic-patched-results.json`。
- `ios-sim/lld14-results.json`、`original-results.json`、`patched-results.json`。
- `ios-sim/typed-comparison/results.log`：固定签名与通用调用的独立对照。
- 各目标的 `step-*.argv.json`、编译/链接日志、`base-hashes.json`。

本轮只创建并启动名为 `GOUI-Mobile-ABI-oc7lzp5l` 的独立模拟器，验收后关闭
并删除该临时设备；不启动、修改或删除其他模拟器，不修改系统安全设置。
日志和产物留在临时目录供追溯。没有执行全仓测试，因为没有修改库实现，
本轮的运行对象是独立的基座与 FFI 验证程序。

下一步先修复 FFI 的有限兼容缺口并加入原生 C 回归，再推进最小包接入、
Android APK/真机和 iOS 真机安装。NativeActivity/UIKit 的 UI 生命周期、
JNI/Objective-C 对象、输入、网络和系统服务尚未验证，仍由后续 platform 切片负责。

## 12. 最小应用安装与运行验证（2026-11-03）

### 12.1 验证目标与宿主边界

本轮从独立原生进程推进到操作系统实际安装、启动的应用，不再用裸可执行
文件运行替代应用验证。仍不修改 GOUI 或依赖库实现。

最小应用采用普通 Go `main`，通过固定的 cgo 桥接入口启动：

```text
系统加载应用 / 动态库
  -> 标准 Go runtime 与包初始化
  -> init 只注册 Go 回调
  -> 原生应用启动回调进入固定 ABI
  -> sync.Once 调用普通 Go main
  -> 后续原生事件调用 Go，读取并显示 Go 状态
```

测试入口仍叫 `goui_probe`，只用于验证固定 ABI；不是本轮新增的公共 API。
测试 main 初始化业务状态并返回，原生宿主拥有本轮的事件循环。
这不证明现有阻塞式 `gui.Application.Run`/`ui.Run` 可以原样移植；实际框架的
启动与事件循环契约仍须由 platform 设计。

宿主和基座保持分离：

- Android 平台宿主对象提供 `ANativeActivity_onCreate`、生命周期回调、
  NativeWindow 软件呈现和 InputQueue/Looper 接入。
- iOS 平台宿主对象提供 UIApplicationMain、AppDelegate、SceneDelegate、
  UIKit 窗口、文本与按钮。
- Go 应用负责版本标记、计数、前后台事件计数、只运行一次的 main 和点击时 GC。
- 运行时/FFI 支持包不包含这些 UI 行为。宿主源码在维护者端单独预编译，
  两个应用版本直接使用同一个宿主对象和同一套基座归档。

本轮没有把这些临时宿主代码写入 `goui/platform`，也没有使用 GOUI Widget、
Painter 或文字排版后端。界面用于验证构建与宿主路径，不能当作 GOUI 移动端
绘制、控件或输入系统已经完成。

### 12.2 构建与组包

消费者仍在 Linux amd64，Go 1.25.8：

| 目标 | 实际构建路径 |
| --- | --- |
| Android ARM64 | 纯 Go main + 基座归档 -> Go 链接器 -> LLD 14 + 预编译 NativeActivity 宿主 -> libgoui.so -> APK |
| iOS ARM64 模拟器 | 纯 Go main + 基座归档 -> Go c-archive -> LLD 22.1.8 + 预编译 UIKit 宿主 -> Mach-O -> `.app` |
| iOS ARM64 真机目标 | 与模拟器分开的归档、宿主对象和系统链接输入 -> Mach-O 与 `.app` 目录；只做构建 |

维护者端允许使用原生编译器制作宿主。消费者端重新编译版本 7 和 19 的 Go
代码时，三种目标均断言 C/Objective-C 编译器调用为 0、GOCACHE 为空、基座归档
摘要不变。版本 19 额外加入了纯 Go 主线程断言，重编译时没有重制宿主或基座。

Android 资源组包使用独立预编译的 AAPT2 35.0.1 和 android.jar 35。这些属于
资源工具与资源输入，不放进 runtime 基座；本轮未实现纯 Go 二进制 Manifest
编码。APK 使用系统的 `android.app.NativeActivity`，`hasCode=false`，没有
应用 Java 源码或 classes.dex。库名与标准入口通过 Manifest 接入。

Android APK 的 v2 签名使用临时构建的纯 Go `apksig-go`，固定提交为
`a0389a9d7f83032504713ac6052f85edfb52f64b`。两份包分别通过 Google 官方
apksigner 35.0.1 校验，并被 Android PackageManager 接受安装和覆盖更新。
Google Java 工具只用于独立校验，不用于生成这两份签名。
签名器尚未作为 GOUI 的正式依赖选型，本轮不声称验证了其全部格式与方案。
测试私钥与证书只在仓库外生成，不使用用户证书，不随基座或证据归档分发。

iOS 的 Info.plist 与 `.app` 目录在 Linux 组装；测试时在 Mac 进行 ad-hoc
签名与 strict 校验，再交给 CoreSimulator 安装。没有生成可分发的真机 IPA，
没有证明 Windows/Linux 上的最终 iOS 签名步骤已经实现。

### 12.3 实际执行环境

| 平台 | 环境 |
| --- | --- |
| Android | Android 11/API 30，AOSP_ATD ARM64，构建 RSR1.210722.013.A7/10811891；完整系统、ART、NativeActivity 与 SurfaceFlinger |
| Android 宿主 | 远端 macOS 15.7.7 ARM64；官方 Emulator 37.2.13.0，Hypervisor.Framework，独立 AVD，480×800 / 160 dpi |
| iOS | iOS 26.3.1 ARM64，独立 iPhone 17 Pro 模拟器；Xcode 26.3 / iOS SDK 26.2 |
| 两端应用 | 同一测试包标识 `org.golanggui.mobileprobe`，先安装版本 7，再覆盖为版本 19 |

Android 的模拟器、平台工具、系统镜像只放在远端隔离临时目录，下载后核对
Google 元数据中的大小与 SHA-1。AVD、用户配置与 emulator home 也使用私有
目录，ADB 服务独立使用 5039 端口，不接管用户默认 ADB 服务。
这些设备工具用于本轮验收，不是应用构建的必需运行环境。

### 12.4 结果与独立断言

| 场景 | Android | iOS |
| --- | --- | --- |
| 安装、冷启动并显示 Go 版本 | 两个版本通过 | 两个版本通过 |
| 普通 Go main 每个进程只执行一次 | 通过 | 通过 |
| Go 通过 goexlib/cgo 读取的 PID 与宿主 PID 一致 | 通过 | 通过 |
| Go 应用入口在原生 UI 主线程，版本 19 内部断言 | gettid == getpid，通过 | pthread_main_np == 1，通过 |
| 真实按钮/触摸事件进入 Go，计数及显示更新 | ADB input，经 NativeActivity InputQueue，通过 | XCTest 点击 UIButton，通过 |
| 点击回调中执行 runtime.GC 后继续工作 | 通过 | 通过 |
| Home 后返回应用，计数保留且 main 不重复 | 通过 | 通过 |
| 同一基座与宿主支持重新编译的版本 19 | 覆盖安装、重新启动与交互通过 | 覆盖安装、重新启动与交互通过 |
| 真机 / 正式证书 / 商店发布 | 未执行 | 未执行 |

iOS 两个版本各执行 1 个独立 XCTest，均为 0 failures。测试先处理系统的
“在应用中打开？”确认框，再验证 URL 回调、UIButton 点击以及 Home/返回。
测试 target 不包含被测应用，不重新编译或替换 Linux 已生成的应用程序。
URL 命令返回成功本身没有计为回调通过，最终以按钮文本与 Go 状态断言为准。

最终状态记录：

```text
Android 7 : mainCalls=1, taps=1, active=2, background=1, starts=2
Android 19: mainCalls=1, taps=1, active=2, background=1, starts=2
iOS 7     : mainCalls=1, taps=2, active=3, background=1, starts=1
iOS 19    : mainCalls=1, taps=2, active=2, background=1, starts=1
```

Android 日志实际记录了第二次 create/start 与 window-created，PID 不变；
同时 Go mainCalls 仍为 1、计数仍为 1。这验证了本用例中的 Activity 重建
不等于 Go 应用重启。不能据此推断全部 Android 进程回收或状态恢复场景已覆盖。

两端均保存系统截图并检查了界面。Android 启动/返回动画中的过渡帧未用作
稳定界面的通过依据。最终稳定截图为 RGBA8888，480×800，alpha=255：
采样 `(20,100)` 背景为 `(237,246,255)`，`(240,500)` 点击后的绿色区域为
`(54,175,126)`，`(66,160)` 字形内部为 `(32,40,48)`，精确断言通过。
iOS 使用 XCTest 的实际标签文本与截图核对，不对系统字体做跨平台金图比较。

### 12.5 证据与清理

本机及远端临时根目录为 `/tmp/goui-mobile-app-vt7cd858/`。

- `src/app/main.go`：普通 Go main、固定入口注册、状态与 UI 主线程断言。
- `src/bridge/bridge.go`：独立固定 ABI 桥接；与 UI 宿主源码分开。
- `android_host.c` / `ios_host.m`：维护者单独预编译的最小平台宿主。
- `build_android_app.py` / `build_ios_app.py`：消费者阶段的编译、链接与组包。
- `android/app-{7,19}/probe.apk`：已安装运行的 v2 签名 APK。
- `ios-sim/app-{7,19}/GoUIProbe.app`：Linux 组装的模拟器应用；安装时由 Mac 签名。
- `minimal-app-validation.json`：构建组件摘要、应用摘要和最终断言结果。
- `evidence/`：两端状态、生命周期日志与 XCTest 记录。
- `android-final-stable.png` / `ios-final.png`：实际系统截图。

iOS 覆盖安装后重新查询了 app container；测试不持久化旧安装分配的容器绝对路径。
独立 iOS 模拟器验收后关闭并删除。Android 模拟器与专用 ADB 服务验收后停止；
AVD、SDK 下载及验证材料仍保留在上述私有临时目录供复现。
不删除用户其他设备，不修改系统安全设置，不变更正式仓库依赖或源码。

### 12.6 本阶段结论与后续门槛

预编译基座、独立平台宿主和新 Go 应用可以正确组合，并走到真实系统的应用
安装、界面、交互和生命周期。Android 上原有的 Go runtime/cgo 可以在 ART
创建的 NativeActivity 主线程进入 Go；iOS 可以在 UIKit 应用回调中进入 Go。
目前没有这条路径必须先重写 Go runtime 的证据。

后续仍需要：

1. 实现正式的预编译包接入和完整冷构建；本轮仍保存了普通 Go 依赖归档。
2. 固化基座、FFI、宿主、链接输入、资源工具与签名工具的组件身份和交付方式。
3. 修复第 11 节的复杂 FFI 缺口。本轮使用标量入口和简单 PID 调用，没有消除
   结构体调用、窄整数栈打包等既有失败。
4. 设计 platform 的移动端应用入口、事件循环、窗口/视图、绘制、排版与输入。
   不把返回式测试 main 或预编译 UIKit 控件当作 GOUI 上层 API 已适配的证据。
5. 验证真机、最低系统版本、真实 16 KiB 页 Android 系统、正式证书/profile、
   iOS 跨宿主签名和实际分发安装；不以模拟器或 ad-hoc 签名代替。


## 13. 正式基座第一阶段：冷缓存归档接入（2026-11-03）

### 13.1 代码归属

- `internal/mobilebase`：维护者导出、组件清单、完整性与兼容性校验、窄范围
  `cmd/go` 构建驱动适配。只依赖标准库，不导入 platform、gui、ui 或 FFI。
- `cmd/goui-base`：基座维护者入口，提供 `driver`、`export`、`verify`。
  它用于阶段性制作与验收，不替代面向应用开发者的 `goui build`。
- `internal/mobilebase/testdata`：独立普通 Go 应用、固定 cgo 测试入口和 C 验证宿主。
  测试入口不是正式 platform 入口，不承担 Android/iOS 生命周期适配。

本阶段无须 fork Go：维护者工具读取精确匹配的 Go 1.25.8 源码，校验
`cmd/go/internal/work/exec.go` SHA-256，再用临时 `-overlay` 构建独立驱动。
不写入用户 GOROOT，不更改全局 Go 配置。驱动只在包构建 action 开始处增加
显式归档接入，继续复用原有依赖加载、并发调度、编译、汇编、importcfg、
Go 链接器和普通包缓存；不改变编译器、链接器或 runtime。

### 13.2 制作与消费

维护者先在宿主平台构建工具，再设置目标环境制作组件，不能在 Android/iOS
目标环境直接 `go run` 维护者工具：

```sh
go build -o /tmp/goui-base ./cmd/goui-base
/tmp/goui-base driver -go /path/to/go1.25.8/bin/go -o /tmp/go-mobile

GOOS=android GOARCH=arm64 CGO_ENABLED=1 CC=/path/to/ndk-clang \
  /tmp/goui-base export -go /path/to/go1.25.8/bin/go \
  -dir /path/to/probe -o /tmp/base-android \
  -buildmode c-shared -variant device -minimum 29 ./app

/tmp/goui-base verify /tmp/base-android/base.json
```

`export` 使用原有 `go list -deps -export -compiled -trimpath -pgo=off`。
`-compiled` 用于完整发现生成代码的隐含依赖，如固定入口依赖的
`runtime/cgo` 和 `syscall`。只复制实际包含 cgo/SWIG 的非 main 包；纯 Go
标准库、应用和 FFI 的 Go/汇编实现不进入组件。输出目录必须不存在，制作
失败清理临时组件；完成清单和归档校验后才发布输出目录。

消费驱动通过构建进程私有的 `GOUI_BASE=/path/to/base.json` 选择组件，应用
仍使用普通 `go build` 参数和模块依赖。示例：

```sh
GOOS=android GOARCH=arm64 CGO_ENABLED=1 \
GOUI_BASE=/tmp/base-android/base.json CC=/path/to/deny-cc \
  /tmp/go-mobile build -trimpath -pgo=off -buildmode=c-shared \
  -ldflags='-linkmode=external -extld=/path/to/link-only-driver' ./app
```

这里的 link-only driver 和目标 CRT/stub 仍是独立组件，本阶段移动端验收
复用前期仓库外实验的链接输入；**尚未实现自动下载、分发或正式 `goui build`
移动端接入**。iOS 使用 `c-archive` 与 `-extar`，最终主程序由独立宿主原生
链接形成，不把 Go main 包直接作为原生 C 目标文件处理。

### 13.3 清单和失败语义

`base.json` 固定格式、精确 Go 版本、GOOS/GOARCH、架构级别、GOEXPERIMENT、
构建模式、原生变体和最低平台版本。每个包记录模块路径/版本/Go 语言版本、
归档 SHA-256、Go build ID、选中源码及头文件摘要、直接依赖的 Go 导出数据摘要。
导出摘要排除宿主相关的 archive/build ID 头部，同时保留编译器/链接器自身的
类型与包指纹检查。文件内容、包依赖或语言版本变化不能静默复用旧基座。

归档路径限定在组件目录内，拒绝越界路径和指向目录外的符号链接。
SHA-256 只校验完整性，不代替未来下载渠道的可信版本与签名验证。

未提供组件的 cgo 包明确报出包路径，不运行原生编译动作。当前仅支持
`-trimpath` 的无 instrumentation 构建；不支持的原生包 gcflags/asmflags/PGO、
需要重新生成的 cgo header、vet/compiled auxiliary output 和源码 overlay
明确失败，不伪装成功。普通 Go 包仍可正常编译、缓存和更新。

`variant`、`minimum_version` 当前是维护者提供的原生目标元数据；本阶段
不解析全部原生对象去独立核验 SDK/API 最低版本。SDK 版本/来源、许可证、
可再分发输入、下载认证与链接工具版本的完整组件清单属于下一阶段。

### 13.4 自动化验收边界

包内测试不依赖桌面。Linux amd64 / Go 1.25.8 的 `TestColdBuild` 需要维护者
工具 cc、ld，缺失或目标不符时明确跳过，不将跳过算作通过。
C 编译仅用于制作基座与独立验证宿主；消费时 PATH 不含可用编译器，CC/CXX
指向记录后失败的脚本，最终通过仅链接适配器调用 GNU ld。第一阶段曾使用
Python 控制组驱动，第二阶段已将该测试驱动改为 Go，当前测试不要求 Python。

两版应用修改普通 Go 逻辑包中的常量，使用各自空 GOCACHE 和同一基座；
独立 C 宿主分别验证 4 个 pthread、512 次调用、GC 与递归栈增长，检查结果
符合各自版本。测试同时断言普通 Go 编译确实执行、cgo 编译未执行、组件
未改变，以及缺失原生包、源码变化、依赖 ABI 变化、非支持编译参数被拒绝。
纯逻辑测试覆盖清单、路径、源码身份与导出数据身份。

移动目标验收分别在原生维护者环境制作归档，在 Linux 的新 GOCACHE 中消费：

| 目标 | 原生支持包 | 归档总字节 | 普通 Go 冷编译与链接 |
| --- | ---: | ---: | --- |
| Android ARM64，API 29 | runtime/cgo、purego helper、测试入口，共 3 包 | 763782 | 已通过 |
| iOS ARM64 Simulator，最低 15.0 | runtime/cgo、测试入口，共 2 包 | 740428 | 已通过 |
| iOS ARM64 Device，最低 15.0 | runtime/cgo、测试入口，共 2 包 | 739884 | 已通过 |

数量差异来自 purego 在 iOS 下不引入 Android 专用的 cgo 动态加载 helper，
不是统一打包缺失依赖。iOS 归档在 macOS arm64 / Xcode 26.3 制作，随后在
Linux amd64 的 Go 1.25.8 正常构建图中消费；普通标准库与应用并非从 Mac
复制的归档。目标 `.so`/主程序最终链接使用前期冻结的 LLVM 与原生输入。

本机新包测试、race、vet 已通过。当前工作区 `go test ./...` 仅被四个已有
未跟踪旧例程的 API 编译错误阻塞（gui_test/painter_test/popup_test/ui_test），
未删除或修改它们。已跟踪源码加本轮新增文件的隔离副本全仓测试：3431 项
通过、10 项既有跳过、无失败；最终新包 race/vet 通过，维护者命令的
Windows amd64/macOS arm64/Linux arm64 交叉构建通过。

iOS 26.3.1 ARM64 模拟器中，独立 XCTest 操作 Linux 构建的 app：启动、按钮
点击、GC、前后台切换与状态保留通过；Go main 只执行一次，Go 与原生 PID
一致，Go 自身的 UI 线程断言通过。Android 11 ARM64 AOSP ATD 安装同一冷构建
链路生成的 APK 后，原生点击、GC、前后台切换和 Activity 重建通过；主入口仍
只执行一次。最终构建产物与上述送验产物 SHA-256 全部一致。

Android 安装验收还发现临时 ZIP 重组代码把 `resources.arsc` 错误压缩；签名
校验虽通过，Android 安装器正确拒绝。修正为保留 AAPT2 的存储方式并重新
对齐后通过。后续正式打包必须保证目标 API30+ 的资源表不压缩且 4 字节对齐，
不能把签名工具退出成功当作完整 APK 合法性验证。

本次专用 iOS 模拟器已关闭删除，Android 模拟器与专用 ADB server 已停止。
没有运行 iOS 真机、正式签名/商店分发、Android 16K 内核或 Windows 消费环境。
运行日志及剩余缺口见 Plan。

### 13.5 下一阶段

1. 独立链接组件和 Go 链接驱动已由第 14 节实现；工具/SDK 的公开分发仍待
   单独完成。当前消费不要求 Python，也不再另带 llvm-ar。
2. 在 cgo 分支和需要的上游 fork 中修复第 11 节确认的移动 ABI 缺陷，完成
   独立 C fixture 的结构体、浮点、溢出栈参数、回调与线程验证。
3. 制作可分发组件及 Windows/Linux 消费验收，随后集成 `cmd/goui`；不在
   此前声称已完成移动端开发支持。平台 UI 集成按职责单独推进。

## 14. 第二阶段：独立链接组件和 Go 链接驱动（2026-11-03）

### 14.1 范围与归属

本阶段继续只开发构建基座，不修改 platform、gui、ui，不把 Activity/UIKit
宿主或生命周期策略放入通用组件。实现位于 `internal/mobilebase/native_*.go`、
维护者命令 `cmd/goui-base` 和内部外部链接入口 `cmd/goui-link`。
仍未改变面向应用的 `goui build` 桌面契约。

将链接依赖分成两个独立目录：

- `inputs.json`：目标 OS/ARCH、device/simulator、最低系统版本、所用 SDK/API
  版本、原始来源/许可状态、精确的文件 SHA-256，以及 CRT/builtins/library
  目录或 iOS sysroot 的角色。Android 的 SDK/API 版本是所选 NDK stub API，
  与 APK manifest 的 targetSdkVersion 分开。
- `tools.json`：宿主 OS/ARCH、LLD 文件/版本、需要随附的工具运行库及其
  SHA-256、来源/许可状态。它不包含目标 SDK 或应用配置。

`goui-base link-inputs` / `link-tools` 从维护者准备的最小文件树和 JSON 规格
制作组件，规格中的空摘要由工具计算，已有摘要不匹配则拒绝；只有全部文件、
清单和校验成功后才发布目录。工具生产另外运行 LLD `--version` 验证版本。
消费验证全部已声明文件，也拒绝组件目录内未声明的文件，不能从未校验的
附加 stub 取符号。组件目录不应作为构建输出目录使用。

SHA-256 与元数据不代表下载认证或再分发授权。本轮组件仅用于本机/远端私有
验收，不把 SDK、工具、密钥或二进制加入仓库；公开组件的许可审核、签名、
下载渠道与宿主最低系统版本仍属于后续分发切片。

### 14.2 链接驱动

`goui-link` 只链接已经生成的原生 `.o`/`.a`。通过进程私有环境选择组件：

```text
GOUI_LINK_INPUTS=/components/android/inputs.json
GOUI_LINK_TOOLS=/components/linux-amd64/tools.json
```

驱动显式转换受支持的 GCC/Clang 驱动参数、`-Wl,`、`-Xlinker` 与 Go 的逐行
quoted response file。C/Objective-C/汇编源文件、编译模式、未知参数、变更目标
架构/SDK 搜索路径、降低页对齐/关闭 RELRO 等请求明确失败。Go 的可选链接器
能力探测仍可能请求编译 trivial.c；本驱动拒绝，不伪装编译成功，也不临时调用
C 编译器。正常链接无需这些可选探测成功。

LLD 始终由独立的已校验宿主组件调用。驱动与 LLD 之间始终使用临时 response
file，避免这段进程边界受长命令行限制；GNU flavor 显式选择 POSIX quoting，
Mach-O flavor 使用其固定 GNU tokenizer。Go→驱动一段仅实现已经发出的 Go
response 格式；Windows 上游 Go 对超长输入的可选 C 编译探测仍未特别适配，
不据此承诺任意规模的 Windows 命令行已经验收。

- Android：显式 ARM64 ELF、CRT 起止、compiler builtins、已声明系统库，
  启用 NOW/RELRO/无未解析符号、16 KiB max/common page size。不隐式链接
  platform host，宿主对象和所需库由最终调用方传入。
- iOS：显式 ARM64、device/simulator、deployment/SDK 版本、声明的 sysroot，
  `-Z` 移除标准目录，设置组件内的库和 framework 搜索路径。使用 `-all_load`
  装入 Go 链接器生成的完整原生归档，UIKit/Foundation 等由宿主自行请求。

链接先写同文件系统的临时目录，目录内保留最终文件名，校验后才替换目的文件。
LLD 22.1.8 的 Mach-O ad-hoc 签名标识取输出 basename；随机临时文件名会污染
标识和可重复构建，因此不使用随机文件名作为 `-o`。失败和取消不覆盖既有产物。ELF 校验包括 ARM64/DYN、LOAD 16 KiB 对齐、RELRO 尾端、不可执行栈、
无 TEXTREL 和 NOW；Mach-O 校验包括 ARM64 executable、精确 platform/min/SDK
及存在 DATA_CONST 时的 SG_READ_ONLY。它们不替代实际安装、签名与执行。

### 14.3 复用 Go pack，不另带 llvm-ar

Go 1.25.8 链接器生成 c-archive 时，以固定的 `-q -c -s output objects...`
调用外部归档工具。`goui-link` 在该入口复用相同 Go 版本的 `go tool pack c`，
原生对象仍由原有 Go 链接器产生；不自行编写 Mach-O/ELF 或 ar 编解码器。

```text
GOOS=ios GOARCH=arm64 CGO_ENABLED=1 \
GOUI_BASE=/components/ios-base/base.json \
GOUI_GO=/path/to/go1.25.8/bin/go \
  go-mobile build -trimpath -pgo=off -buildmode=c-archive \
  -ldflags='-linkmode=external -extar=/path/to/goui-link' ./app
```

宿主 Go 工具的调用显式指定本机 GOOS/GOARCH、基础架构选项、CGO_ENABLED=0、
GOENV=off、GOTOOLCHAIN=local，不继承用户持久化的交叉编译配置，也不修改它。
归档同样经临时文件发布。它不含原生符号索引，仅用于本驱动最终的
`-all_load` 流程，不承诺是可交给任意 C 项目按符号抽取的通用静态库。
本阶段不再要求携带独立 llvm-ar，Linux 控制组的 Python 测试驱动也改为 Go。

### 14.4 维护者命令与模板

```sh
# 规格与准备树由对应 SDK/工具版本的维护者构建步骤提供。
goui-base link-inputs -config android-inputs.json -dir prepared-target -o components/android
goui-base link-tools -config llvm-host.json -dir prepared-tools -o components/host

# Android 的 platform host 通过 extldflags 单独传入，不能预置在通用驱动中。
GOOS=android GOARCH=arm64 CGO_ENABLED=1 \
GOUI_BASE=/components/base/base.json \
GOUI_LINK_INPUTS=/components/android/inputs.json \
GOUI_LINK_TOOLS=/components/host/tools.json \
  go-mobile build -trimpath -pgo=off -buildmode=c-shared \
  -ldflags='-linkmode=external -extld=/path/to/goui-link -extldflags "/path/to/host.o -landroid"' ./app

# iOS Go 归档完成后，由独立宿主要求其 UI framework。
GOUI_LINK_INPUTS=/components/ios-simulator/inputs.json \
GOUI_LINK_TOOLS=/components/host/tools.json \
  goui-link -o Probe go.a host.o -lobjc \
  -framework CoreFoundation -framework Foundation -framework UIKit
```

LLD 的明确 target 行为及 Mach-O response tokenizer 依据：
[LLVM 22.1.8 ELF driver](https://github.com/llvm/llvm-project/blob/llvmorg-22.1.8/lld/ELF/Driver.cpp)、
[Mach-O DriverUtils](https://github.com/llvm/llvm-project/blob/llvmorg-22.1.8/lld/MachO/DriverUtils.cpp)。
16 KiB 与 RELRO 检查参考 [Android 官方说明](https://developer.android.com/guide/practices/page-sizes)。

### 14.5 阶段性验证

- 包内逻辑测试验证组件封装/损坏/缺失、参数映射与拒绝、Go response 文件、
  宿主环境隔离、归档委托及链接失败/无效输出不覆盖旧产物。合成 ELF/Mach-O
  只测试格式约束；假 LLD 只测试进程与文件发布路径，不作为真实链接证据。
- 真正链接使用独立私有组件：Android 8 个文件 865916 字节；iOS Simulator
  7 个 TBD 10189925 字节；iOS Device 7 个 TBD 10170071 字节。均不带头文件。
  iOS 集合在初次链接时发现缺少 libobjc.A 与 FileProvider 的重导出 stub，
  从匹配 Xcode SDK 补齐后由 LLD 验证，不放宽符号或搜索路径检查。
- 宿主链接工具暂用已安装 Rust 工具链提供的 LLD 22.1.8，携带其 libLLVM、
  zlib/libgcc_s 支持文件和现有 LLVM 许可文本，作为私有测试组件；其体积与
  可公开分发的独立工具包尚需后续处理，不代表公开宿主支持矩阵已经完成。
- Android ARM64、iOS ARM64 Simulator/Device 各自空缓存，普通 Go 包从源码
  编译，通过 Go 链接驱动和组件完成最终链接，消费时 C 编译器调用为零。
  iOS 归档生产已完全使用 Go pack，不再调用 LLVM ar。
- 原生运行、全仓测试和当前未覆盖项由 Plan 记录；不将真机目标编译成功写成
  真机安装/运行验收，也不把 ad-hoc 模拟器签名写成正式应用签名完成。


第二阶段最终验收补充：新包 race/vet 通过；已跟踪源码加本轮文件的隔离副本
全仓测试 3466 项通过、10 项既有跳过，无失败。当前工作区全仓测试仍仅被
四个未跟踪旧例程编译错误阻塞，原文件未动；原有测试写入的 PNG 均按运行前
备份恢复。两个维护者入口在 Windows amd64/macOS arm64/Linux arm64 交叉构建通过。

iOS 26.3.1 ARM64 模拟器与 Android 11 ARM64 ATD 运行了新的 Linux 构建产物。
启动、按钮、GC、前后台切换、状态保留通过，Go main 一次、UI 主线程与 PID
检查通过；Android Activity 重建后值保留。iOS 使用独立 XCTest 操作已安装 app，
不重建被测 app，模拟器 ad-hoc 签名通过。测试专用 iOS 设备已删除，Android
模拟器和专用 ADB server 已停止。没有进行 iOS 真机签名/安装、Android 16K
内核、Windows 消费运行或公开组件分发验收。产物及日志在
`/tmp/goui-mobile-link-xslbk_kb`，SDK 和工具二进制未加入仓库。

最终复核额外覆盖了 GNU response 引用（空格、中文、实际制表符不能按 Go 的
反斜线转义输出）和逻辑输出 basename。LLD 22.1.8 的
[CodeSignatureSection](https://github.com/llvm/llvm-project/blob/llvmorg-22.1.8/lld/MachO/SyntheticSections.cpp)
直接取输出 basename 作为临时 ad-hoc 标识；使用保留名称的临时目录后，两种
iOS 目标的重复链接字节一致。回归中的假链接器也会拒绝错误的输出 basename，
不能靠清理/重签后才能发现该差异。Android 最终产物与已执行模拟器验收的产物
SHA-256 一致。

最终输出名称修复后，iOS Simulator/Device 重复链接字节一致；Mac 上对未封装的
LLD 输出独立严格验签并确认 Identifier=Probe，随后完整 app ad-hoc 签名、安装
与启动通过，Go/UI 线程断言保持。该专用模拟器已删除；LLVM 临时签名不代替
最终 app 的资源签名。最终 Android 字节与已执行完整交互验收的产物一致。

## 15. 第三阶段：移动 ARM64 FFI ABI（2026-11-03）

### 15.1 范围与版本选择

本阶段只修复底层 FFI，不把 Activity/UIKit、窗口、事件循环或 GUI 策略加入基座。
代码分别位于独立的 `cgo` 与 `purego` 仓库，均使用 `feature/mobile-base` 分支；
GOUI 仅同步设计和进度，主工作区与已发布桌面依赖保持不变。

purego 分支基于 upstream **v0.10.2**。v0.11.2 已包含移动系统枚举与部分返回值修正，
但其 go.mod 要求 Go 1.25；本轮采用必要修正和独立 ABI 修复，保留 v0.10.2 的
Go 1.18 声明，不为 FFI 升高 GOUI 的 Go 1.24 或 goexlib/cgo 的 Go 1.23 下限。
固定 Go 1.25.8 的基座生产/消费契约与库自身的最低 Go 版本是两件事。

开发通过仓库外显式 `GOWORK` 连接三个源码模块，不提交绝对路径 replace，
不修改全局 Go 配置，也不先更新主项目到一个尚未发布的 fork 版本。
此阶段完成本地修复和验证；fork 的发布、上游提交及正式依赖版本接入另行推进。

依据使用 [AAPCS64 2025Q1](https://github.com/ARM-software/abi-aa/blob/2025Q1/aapcs64/aapcs64.rst)、
[Apple ARM64 ABI](https://developer.apple.com/documentation/xcode/writing-arm64-code-for-apple-platforms)，
以及 [purego v0.10.2](https://github.com/ebitengine/purego/tree/v0.10.2) /
[v0.11.2](https://github.com/ebitengine/purego/tree/v0.11.2) 源码。
官方规则、Clang 生成代码和原生运行分别作为契约、实现与实测证据，不互相替代。

### 15.2 参数和返回值

- Android/iOS 接入已有结构体检查，iOS 使用 Apple ARM64 ABI，不能因 GOOS
  字符串不同而退回通用的八字节标量栈槽。
- 普通小结构体按连续整数寄存器字组传递，不依据单个字段把整数和浮点分到
  两个银行。整数数组是普通聚合，不能当作原生 SIMD HVA。
- HFA 递归展开嵌套结构体和数组，最多四个同类型浮点叶子；发现混合字段后
  不能被后续嵌套结构体重置。HFA 与普通结构体都必须整体放入寄存器或栈，
  放不下时耗尽对应银行，后续参数不能复用剩余寄存器。
- 大于 16 字节的非 HFA 通过间接指针传递；即使此前浮点银行已溢出，也不能
  直接把结构体内容追加到栈。返回继续使用原有 X8 隐藏结果地址。
- ARM64 的 `...any` 先按实际类型展开，再统一分配寄存器和栈。这是固定 C
  函数签名的类型展开，不是对 C variadic 函数的支持。
- 标准 AAPCS64 的栈参数使用八字节槽；Apple 标量和 HFA 使用自然大小/对齐。
  普通小结构体仍按八字节字组对齐并占用舍入后的大小。12 字节普通结构体
  不能套用 12 字节 HFA 的规则：Xcode 26.3 自带 Clang 对 macOS/iOS Simulator 的独立函数
  均显示窄参数在 SP+0、普通结构体在 SP+8、后续窄参数在 SP+24。
- goexlib/cgo 的回调栈计划同步处理上述差异。大结构体降低为指针后，使用指针
  的栈对齐，不能直接沿用前一个窄参数后的未对齐偏移。
- 保留原有 Go runtime、原生线程接入和 Go/汇编调用入口。字符串转换、切片及
  间接参数的所有者保活至同步调用返回；不承诺 C 可以在返回后保留 Go 内存。
  不修改编译器，不增加另一套 GC 或自行替换 runtime 的 callback 机制。

### 15.3 可重复的独立 C 验收

`cgo/testdata/mobileabi` 是嵌套的维护者测试模块，操作说明放在其 README。
它包含独立的 C fixture、一个测试用 cgo 导出入口和独立 pthread 宿主，不包含
UI。根包的 `go test ./...` 不会隐式要求 Android SDK/Xcode。

28 项固定预期覆盖浮点、HFA、混合/嵌套/数组/大结构体、整组溢出、窄栈参数、
间接指针、返回值、回调期间 GC 和 Go 栈增长。首次全部通过后，再由 8 个
pthread 各重复 8 轮，最终退出码必须为零。普通 Go→Go 回环测试仅作补充，
不能替代 C 编译器生成的独立调用方和被调用方。

维护者分别生产三种基座：Android ARM64 c-shared、iOS Simulator/Device ARM64
c-archive。Android 自动收集 bridge、purego/internal/cgo、runtime/cgo 三个真正的
cgo 包；两种 iOS 各收集 bridge 与 runtime/cgo。基座消费仍由普通 Go 包从源码
冷构建，`CC/CXX` 设置为记录后失败的程序，PATH 不含原生编译器或 SDK；最终
链接只使用第 14 节的原生输入/工具组件。iOS 测试宿主只请求 CoreFoundation。

### 15.4 验证记录与边界

结果和失败命令统一记录于 Plan。验收产物位于仓库外
`/tmp/goui-mobile-ffi-stage3`，远端 Mac 使用同名目录。修复前 Linux ARM64
原有十二项中混合结构体失败；完整 macOS 回归还捕获了普通结构体栈对齐错误，
原测试断言保留。回调栈计划测试独立复现了窄参数后间接指针偏移 1 而非 8 的问题。

当前基座工具和桌面 API 不变；没有借 FFI 修复向 platform 添加窗口能力。
剩余工作包括 fork 的可分发版本与依赖接入、宿主工具/SDK 组件的再分发审核和
供应链认证、Windows 消费运行、iOS 真机与 Android 16 KiB 内核验收，最后才是
面向用户的 `cmd/goui` 编排和 platform 移动 UI 集成。

## 16. 第四阶段：无原生编译器的消费构建编排（2026-11-03）

### 16.1 本阶段边界

将前三阶段已经验证的手工命令封装到 `internal/mobilebase.Build`，并由维护者
入口 `goui-base build` 调用。它输出 Android `.so` 或与显式宿主链接后的 iOS
可执行文件；不是 APK/IPA，不提前改变 `cmd/goui build` 的桌面语义。
基座、目标链接输入、宿主链接工具仍是独立组件，不引入包含 UI 的大基座。
组件下载、认证、公开发行和正式移动 UI 宿主不在本阶段一起实现。

`BuildOptions` 显式传入 Go、适配后的 Go driver、goui-link、三个组件清单、
主包目录/包名/输出，以及独立宿主对象、库和 framework。目标、架构、变体、
最低系统版本与 buildmode 从组件读取，不要求调用方再重复声明一套。
三个组件在启动源码构建前验证；不兼容的目标、变体、最低系统版本、Go 版本
或宿主工具立即失败。iOS 必须显式提供至少一个宿主对象。

### 16.2 受控构建与发布

- 继续由原有 Go build 解析模块/工作区、依赖和缓存，正常 Go 包照常源码编译。
  子进程环境指定匹配的 GOROOT、目标、架构等级、实验选项和固定 Go 版本，
  `CGO_ENABLED=1` 用于选择真正 runtime/cgo，不能把它误称为内部全程 CGO=0。
- CC/CXX 指向只链接的 goui-link，原生编译请求明确拒绝；缺失原生包仍由已验证
  的 Go action hook 报错。消费端不回退到系统 C 编译器，也不修改用户环境。
- 保留普通 Go 模块网络配置、GOCACHE 和 GOWORK；不把工具生成阶段使用的
  GOWORK=off/GOENV=off 不加区分地套到应用构建。额外 GOFLAGS 或自定义
  CGO 编译/链接 flags 明确拒绝，避免与固定原生归档的语义产生歧义。
- 参数只通过 argv/response file 传递，不经 shell。遵循 Go 1.25.8 的
  cmd/internal/quoted 规则，不能使用 JSON 转义代替 Go flags quoting。
  Android 直接复用 cmd/go 的 setextld：从 CC/CXX 传入只链接驱动及可选的
  host response argv，避免再经 -ldflags 嵌套转义。该 Go 格式本身不能表达
  同一参数同时包含单、双引号的情况；不能静默猜测。
- Android 经 Go driver 的 c-shared/external-link 输出，iOS 经 c-archive/
  Go pack 生成归档，再复用 RunNativeLink 与调用方指定宿主完成最终链接。
  原生链接仍由独立 LLD 组件完成，Go runtime 不另行实现。
- 构建写入输出同一文件系统的临时目录，保持最终 basename，所有阶段和
  最终 ELF/Mach-O 校验成功、且未取消后才发布。失败保留旧文件；临时归档、
  response/header 等辅助产物随临时目录清理，不写应用源码树。

对应源码依据为 [Go 1.25.8 quoted](https://github.com/golang/go/blob/go1.25.8/src/cmd/internal/quoted/quoted.go)
与 [external link/archive](https://github.com/golang/go/blob/go1.25.8/src/cmd/link/internal/ld/lib.go)。
正式组件索引和自动下载应在公开分发契约确定后实现，本阶段接受已准备清单路径。

### 16.3 文件与验证

实现主要在 `internal/mobilebase/build.go`、`build_test.go`、维护者 CLI 与测试
fixture。逻辑测试覆盖组件不匹配、错误 Go 环境、argv/路径、取消、失败及
输出不被覆盖；假构建器只验证进程编排，不作为 native ABI 的证据。

本机继续使用第三阶段的真实最小 FFI 工程，对三种移动目标各自空缓存构建。
远端 Windows amd64 使用固定 Go 版本、预置的 Windows LLD 和已验证的目标
组件消费同一份源码，不安装 SDK/C 编译器；产物再交 Android/iOS 模拟器运行。
Windows 原生编译、模拟器执行、iOS Device 仅链接、尚未运行的场景分别记录，
不把交叉构建或格式检查写成真机运行。每阶段更新 Plan/Roadmap。

维护者 Go driver 的生成另外使用临时的空 GOMODCACHE。自动下载的 Go 1.25.8
位于用户模块缓存内，Go 1.25.8 work/init.go 会拒绝覆盖被视为模块依赖的文件；
工具生成只依赖 GOROOT，因此使用独立缓存让 GOROOT 与应用模块依赖分离。
这一设置只作用于该子进程，不改变真实缓存、不复制或改写 GOROOT。包内回归
把 GOMODCACHE 设为 GOROOT 父目录复现该条件，并核对原源码保持不变。

### 16.4 实现和验收结果

- `Build` 与 `goui-base build` 已实现。组件不匹配、错误 Go 版本/flags、缺少宿主、
  Go/最终链接失败、无效输出和取消均有回归，原输出保留。取消保留 context 错误
  身份；进程环境不变。测试构建器/链接器仅用于编排，不作为原生执行证据。
- Linux amd64 / Go 1.25.8 与 Windows 10 amd64 / Go 1.25.8 均从独立空 GOCACHE
  构建 Android ARM64、iOS ARM64 Simulator/Device。Go 日志确认普通包源码编译，
  使用对应原生归档，未运行 cgo 生成；只链接驱动不能调用原生编译器。
  额外宿主 response file、库与 framework 路径均在真实链接中覆盖。
- 两个宿主均覆盖空格路径及中文宿主名；Android 使用带空格/单引号的链接工具路径
  另行冷构建通过。复用 Go 的 CC→setextld 传递，避免嵌套编码误伤合法路径。
- 真实执行在远端 macOS 15.7.7 ARM64 上进行：iOS 26.3.1 ARM64 Simulator 和
  Android 11 ARM64 ATD 分别运行 Windows/Linux 产物，四组均为 28/28，8 个
  pthread 各 8 轮重复错误为零。iOS 使用 ad-hoc 签名和 simctl spawn；本阶段是
  无窗口 FFI 进程验收，不声称 APK/IPA 或完整移动 GUI 已可用。专用模拟器/ADB
  均已清理；iOS Device 仍仅验证编译/链接，没有真机或 16 KiB 内核运行。
- 隔离的已跟踪源码加本阶段文件，全仓测试 3481 项通过、10 项既有跳过；新增
  构建/driver 回归的 race 与 mobilebase/两个工具入口的 vet 通过。
  原工作区未跟踪的四个旧例程和 PNG 等文件保持原样，不移入本阶段提交。
- Windows 上传的 Linux 交叉编译版 goui-base.exe 返回 Access denied。其 ACL
  允许执行、没有 Zone stream，Defender 未查询到对应威胁记录；未确认拒绝原因，
  未更改安全策略。用远端标准 Go 从相同工具源码本机构建后可运行，预编译 Rust
  LLD 也可运行，随后完成上述消费与原生运行。公开工具附件的执行验收仍需单独
  处理，不能把本机 Go 编译成功写成上传版附件已验证。
- 维护者测试脚本首次误替换 GOROOT/使用 PowerShell 只读 Host 变量的问题均已
  修正。手写 Windows 绝对斜杠工作区路径导致 stock Go 和 driver 都报模块不在
  workspace；改用可移植相对路径后两者通过，未以框架特例补偿 Go 模块发现。
  构建失败和修正后的日志都保留于 `/tmp/goui-mobile-build-stage4`。

示例维护者命令（所有路径均为调用方准备的组件，不执行下载）：

```sh
goui-base build -go /path/to/go1.25.8/bin/go \
  -driver /path/to/go-mobile -linker /path/to/goui-link \
  -base /components/android/base.json \
  -inputs /components/android-inputs/inputs.json \
  -tools /components/host-tools/tools.json \
  -dir /path/to/application -o /output/libapp.so .

# iOS 的 SDK 输入、基座变体和独立宿主必须一致。
goui-base build -go /path/to/go1.25.8/bin/go \
  -driver /path/to/go-mobile -linker /path/to/goui-link \
  -base /components/ios-sim/base.json \
  -inputs /components/ios-sim-inputs/inputs.json \
  -tools /components/host-tools/tools.json \
  -object /platform-host/simulator.o -framework CoreFoundation \
  -dir /path/to/application -o /output/Probe .
```

组件索引/认证分发、已修复 FFI 的公开版本、APK/IPA 组装签名、正式移动平台
接入仍为后续阶段；没有将上述私有 SDK/工具二进制或个人签名材料加入仓库。
