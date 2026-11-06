# 移动平台基座收敛

状态：2026-11-05，本切片代码与模拟器验证完成，按构建工具、Android 绑定、平台接入分批归档；文档独立提交。工作区为 goui2/feature/mobile-base，不修改主会话工作区。

## 固定原则

- 使用原版 Go 编译器、汇编器、runtime、cmd/link；继续复用已验证的窄 cmd/go
  归档接入驱动和 LLVM 链接适配器，不采用临时 Go 链接器实验。
- 仅固定入口由最小 cgo 导出归档提供；不新增手写 callback/runtime 或 .syso 注册协议。
- platform/mobile 负责 main 注册和 Go 启动衔接；Android/iOS 的窗口、原生回调、
  软件呈现回到各自 Go 包；GLES context 实现归 graphics/opengl。移除统一 goui_host_* C ABI 窗口协议。
- 仅维护 Go 源文件；必要的 C/Objective-C/Java 源码内嵌 Go 生成器，通过 go generate
  写到仓库外维护者目录。使用方消费预编译组件，不安装 SDK/原生编译器。
- 原生绑定保留 SDK 契约，适配留在平台包。共享层只保留 Go 队列、逻辑窗口状态和
  可撤销绘制目标等机制。原生测试参考独立于框架，不以测试钩子扩展生产 API。

## 实施顺序与检查点

1. [x] Go 生成器收拢测试参考与 Java helper；替换 Python 构建脚本。
2. [x] 固定导出归档传递 Android 创建参数与 iOS 主线程入口，Go 注册与线程启动分离。
3. [x] Android NDK/JNI/Looper/软件呈现及 EGL 迁 Go；保留源目标同步撤销。
4. [x] iOS UIKit/Scene/主线程投递/软件呈现及 EAGL 迁 Go；保留后台 GPU 屏障。
5. [x] 两端有自己的 Window/EventLoop，复用共享 Go 状态机制；graphics/opengl 定义
   最小窗口接口并创建 GL context，不反向依赖具体窗口，不查询统一宿主 C 导出。
6. [x] go generate 制作新基座，Linux 无原生编译器消费；模拟器四条路径、桌面回归。
7. [x] 更新设计、路线、验证记录；不把链接、模拟器或原型通过描述成真机/正式发布通过。

## 原生证据与约束

- Android NativeActivity 的回调运行在系统主线程，env 仅该线程可用；VM 可用于附加
  GOUI 线程。目标销毁返回前必须停止访问，不能等待 GUI 通知才停止。
- 测试使用 android.app.func_name 指定 probe_onCreate，再调用真实
  ANativeActivity_onCreate；框架不增加探针或环境私有状态 getter。
- iOS 最小原生 main 仅调用 goui_run；UIKit 初始化、类/IMP 和 UIApplicationMain
  由 Go 在真正主线程执行。应用 main 在 GOUI 工作线程运行，不能阻塞 NativeActivity。
- EGL/EAGL context 与 Window 表面分离；重建 drawable 保留 context/图片，iOS 后台
  返回前关闭包括 scratch 在内的全部 GL 执行许可。

参考：[NativeActivity](https://developer.android.com/reference/android/app/NativeActivity#META_DATA_FUNC_NAME)、
[ANativeActivity](https://developer.android.com/ndk/reference/struct/a-native-activity)、
既有 Go 绑定规范和 PlanMobileBuild 第 17 节入口实测。

验证结果随阶段记录；原有 C/Objective-C 实现仅作为仓库外行为对照，不作为未来宿主模型。

## 验证记录（持续更新）

- Go 层：internal/mobile、internal/mobileentry、internal/nativegen、platform/mobile/base
  的测试与 race 已通过；覆盖延迟目标不复活、租约撤销、关闭后拒绝观察及生成路径限制。
- Android：新基座已由标准 cgo export 制作，Linux 在拒绝原生编译器的 PATH 下消费链接，
  APK 资源、Dex、签名/验签通过。首轮 Software 模拟器通过 273 项及旋转、后台恢复。
- iOS：Simulator/device 固定入口基座制作通过，Linux 消费链接 Software/GLES；首轮
  模拟器发现 Scene 协议声明遗漏，已修正并复测。链接通过不计为原生运行通过。
- 后续复验已完成：修正后的两端 GPU/生命周期、iOS Software、桌面回归与清理，见下表。
- 未验：Android/iOS 真机、iOS Scene 由系统实际断开重连、触摸/IME、正式组件分发。

### iOS 协议元数据补充

直接 Go 注册的 delegate 没有编译期 @protocol 引用，iOS 26.3 实测
objc_getProtocol(UIApplicationDelegate) 为空；未声明 Scene 协议会被 UIKit 拒绝。
UIKit Go 生成器现保留两个 SDK 协议引用，维护者编译成约 24 KiB 的元数据对象。
全部类、IMP 和生命周期仍由 Go 实现；没有恢复 C/Objective-C 宿主协议。

## 最终验收

工作区 goui2/feature/mobile-base，未修改主会话 goui，也未改动 cgo/purego/nanovgo。
归档仍使用 Go 1.25.8；产物、日志和配置位于本轮仓库外临时目录。

| 项目 | 结果与范围 |
| --- | --- |
| 基座 | Android、iOS Simulator/device 的标准 cgo 导出通过；原生归档只有 runtime/cgo、purego 必需支持包、固定入口，普通 Go 包继续源码编译 |
| 原生源 | Go 生成器输出仓库外；原目录没有独立维护的 .c/.m/.h/.java/.inc/.py 文件；生产代码无 goui_host_* 引用 |
| 维护工具 | Go base/native/build/package 阶段通过；Android 原生/Dex/签名及 iOS 原生、最小 main、UIKit 协议 metadata 制作通过 |
| 消费审计 | Linux amd64，Android 和 iOS Simulator 使用独立空缓存与全新拒绝编译器目录；两端链接通过，编译器调用为零，日志确认复用入口而未执行 cgo |
| Android | macOS 15.7.7 ARM64 上 Android 11/API 30 ARM64 ATD、SwiftShader、1x；最终 Software 277 项、GLES 278 项断言通过；旋转及 Home/返回保持 PID、nonce 和缓存图像，后台帧数停止；最终 GLES 另验证 Back 结束 Activity 后同 PID、nonce 恢复 |
| iOS | 同一 Mac 上 iOS 26.3 ARM64 Simulator、3x；Software 274 项、GLES 276 项通过；XCTest 八次旋转、三次 Home/返回，后台停止绘制，恢复保持 nonce；最终冻结源码两后端再次启动和断言通过 |
| iOS device | Linux 消费链接 Software/GLES 通过；没有签名安装或真机运行，不视为真机通过 |
| 像素 | Android 两后端与 iOS 两后端的横竖屏稳定截图，在绿色背景、蓝色缓存图、clip 内橙色和 clip 外背景各取内部点；不透明 sRGB、预乘等于原值、误差为零。避开系统栏、刘海与抗锯齿边缘；iOS 固定物理方向的横屏截图按测试 LandscapeLeft 方向旋转 90° 后采样。独立预期累计 24 点通过，最终源码 iOS 另 8 点通过 |
| Go 回归 | 独立 Xvnc :78 / X11 / xfwm4 / Linux amd64；受控源码副本 go test ./...，3523 通过、10 既有跳过、0 失败；876 个 Go 文件与交付源码逐字一致 |
| 竞态与编译 | internal/mobile、internal/mobileentry、internal/nativegen、platform/mobile/base 及维护工具定向 race 通过；Windows amd64/macOS arm64 全仓交叉编译通过，不冒充原生桌面运行 |
| 既有例程 | Android mobile_start/mobile_surface/android_text/android_gui 的新生成、消费编译、APK 组装/签名/验签均通过；没有把这些构建结果当作各自操作场景的重新验收 |

第一次桌面全仓验证因继承 GOWORK 导致 cmd/goui 的临时独立模块不在 workspace
而失败；改为只在受控副本 go.mod 中显式 replace，并使用 GOWORK=off 后全量通过。
没有为此修改生产代码、测试断言或扩大 skip。原工作区的未跟踪旧 example 与现有
测试图片保持不动；全仓命令在隔离副本执行，不能描述成原工作区直接执行通过。

初轮运行发现并修正的两个问题保留证据：GLES 参考误查旧宿主导出；iOS 缺少
正式协议及其 SDK metadata。没有以临时原生宿主、空协议或放宽断言掩盖失败。

仍未验/未做：Android/iOS 真机、Android 16 KiB 内核、系统实际 Scene disconnect/
reconnect、全套 GUI/UI 操作、触摸/IME、SafeArea、组件分发、公开 cmd/goui 移动打包
与商店验收。当前没有宣布移动端完成或合并主线；保持单 Activity/Scene 内容窗口边界。

## EGL/EAGL 包归属修正（2026-11-05）

已实现并复验：上下文实现迁至 graphics/opengl/opengl_android.go、opengl_ios.go，分别为
私有 eglContext、eaglContext。取消窗口 NewGLContext 委托；参考 GLX 的绘制包定义
小接口、窗口提供所需原生数据的方式，绘制包不导入 android 或 darwin/ios 包。

Android 只需 NativeWindow 与 AcquireTarget。iOS 保留窗口拥有的绘制层和系统
生命周期锁，以小接口供渲染线程借用；所有 EGL/EAGL/FBO 操作由绘制包拥有。
公共 common.Window、原生绑定、基座归档及 NativeTarget 契约保持不变。

验证：两端 GLES 消费编译、Android APK、iOS Simulator/device 链接及原生回归分开记录。

本轮验收（Go 1.25.8）：

- Linux amd64 消费 Android ARM64、iOS ARM64 Simulator/device 的 GLES 产物通过；
  复用同一批原生归档，拒绝编译器目录没有调用记录。
- Android 11/API 30 ARM64 ATD / SwiftShader / 1x：278 项通过，旋转、Home/返回与
  Back 结束 Activity 后同 PID/nonce 重建通过；缓存图像保留，后台帧数停止。
- iOS 26.3 ARM64 Simulator / GLES / 3x：276 项通过，XCTest 八次旋转、三次 Home/
  返回通过；后台帧数停止，恢复保持 nonce。device 仅验证链接，未运行真机。
- Android 稳定截图与 iOS 横竖屏截图各四个内部点，以前述独立 sRGB 预期、零容差
  检查绿色背景、蓝色缓存图、clip 内外；合计 12 点通过。
- 独立 Xvnc :78 / X11 / xfwm4 的受控源码副本执行 go test ./...，3523 通过、10
  既有跳过、0 失败；876 个 Go 文件与交付源码一致。定向 race 通过，Windows amd64/
  macOS arm64 全仓交叉编译通过。旧 example、图片、主会话及依赖仓库未修改。
- 本轮未重复 Software 操作场景；其呈现实现未改动，包内及全仓回归通过。真机、
  系统实际 Scene disconnect/reconnect 的验证缺口沿用前文，不扩展通过范围。

原生验收使用 remote-shell 的 mac 连接，macOS 15.7.7 ARM64 上的专用模拟器。
构建与运行证据保留在本轮仓库外临时目录；验证后关闭并删除专用模拟器及本机
X11 会话。代码与文档按职责分别归档，验证范围不因提交而扩大。

## 构建基座包归属修正（2026-11-05）

已完成并验证：移动构建工具包整体迁至 platform/mobile/base，最终包名为 base，
cmd/goui-base 与 cmd/goui-link 显式使用 mobilebase 导入别名，工具入口名称不变。
实现、测试及 testdata 整体移动，嵌入文件保持相对关系；驱动中的源码 package
替换同步为 base → work。platform/mobile 仍只负责应用启动，不导入构建工具子包。

不扩大为平台层运行时能力，也不改动原生基座身份、入口或归档格式。验证覆盖新包、
工具入口、冷缓存驱动/归档接入、全仓回归和两端消费构建；原生 UI 行为未改动。

本轮迁移验收：

- 初次迁移时 24 个实现/测试资源文件移动前后 SHA-256 一致；随后按最终约定修正
  包名和嵌入源码 package 替换，见下节。旧目录及旧路径引用已移除。
- 新路径 go test ./platform/mobile/base ./cmd/goui-base ./cmd/goui-link 与定向 race
  通过，含 Linux amd64 冷缓存构建、Go driver 生成、独立 C 宿主回调/GC/栈增长验收。
- 重新构建 goui-base、goui-link 和 Go driver，使用原有组件消费链接 Android ARM64、
  iOS ARM64 Simulator/device 均通过；全新拒绝编译器目录仍无调用记录。
- 新包及两个工具的 Windows amd64、macOS arm64 交叉编译通过。没有改动原生行为，
  本轮不重复原生窗口操作验收，也不将链接或交叉编译描述成原生运行通过。
- 独立 Xvnc :78 / X11 / xfwm4 受控副本 go test ./...：3523 通过、10 既有跳过、
  0 失败；876 个 Go 源文件与交付源码一致。platform/mobile 的依赖列表确认不包含
  platform/mobile/base，不向应用启动路径引入构建工具。临时桌面验证后关闭。

### 包名与导入别名补充（2026-11-05）

按最终约定，platform/mobile/base 的包声明统一为 base，调用方显式导入：

```go
import mobilebase "github.com/golang-gui/goui/platform/mobile/base"
```

前一迁移记录中的 mobilebase 包声明是当时状态；本轮修正包名，并同步更新
嵌入源码的 package base → package work 替换。调用处 mobilebase.Build 等名称
不变。新包、工具入口及其 race 通过，包含 Linux amd64 的冷缓存驱动/归档接入
与独立 C 宿主验证；受控副本 go test ./... 为 3523 通过、10 既有跳过、0 失败，
876 个 Go 文件与交付源码一致。Windows amd64/macOS arm64 的新包及工具交叉编译
通过。没有改动原生代码或接口，不重复原生窗口验收。

## 下一阶段边界

先完善正式移动构建、组包与签名流程，以仅依赖 platform 的例程完成 Android/iOS
真机安装、启动、绘制及前后台恢复验收。当前纯 Go 构建与运行说明见
[PlanMobileBuild 第 19 节](PlanMobileBuild.md#19-当前纯-go-应用的构建与运行路径2026-11-05)。
GUI 触摸、IME、SafeArea 和声明式适配继续后置，不将其作为构建验证的前置条件。

## 正式构建与官方标准库 App（2026-11-05）

### 本次实现

- cmd/goui 增加 android/arm64、ios/arm64、明确的 simulator variant 和本地
  mobile.json profile；校验文件摘要、原生目标、Go 版本及实际 cgo 依赖闭包。
  消费编译复用 mobilebase.Build，支持普通 tags 与受限 -s/-w/-X。
- build 仍输出原生二进制及摘要记录；package 不重新编译；显式 build -package
  组合编译/组包。生产 APK 使用系统 NativeActivity 默认入口，包含原色图标和
  正式排版 Dex，没有 probe_onCreate、probe_* 或统一 goui_host_* 窗口宿主。
- Android 以 Go 生成并稳定复用本机开发证书，独立 Go 签名器/验签器组包；最终
  APK 另经官方 SDK apksigner 验证 v3，并在显式 24–27 验签范围验证 v2。后者
  是签名算法验证，不代表应用支持低于声明 min_sdk=29 的系统。
- iOS 使用既有 Go c-archive 与最小 main/SDK metadata 输出 Mach-O、原色 PNG/
  plist、app/IPA。用户确认云真机先重签，故显式 -unsigned 支持待重签 IPA；
  没有签名时默认仍报错，不把 LLD ad-hoc 记录或模拟器签名当作设备授权。
- Android 首次 onCreate 从真实 Context.getCacheDir 取得应用缓存路径；仅当
  TMPDIR 未设置时在应用 main 前初始化，保证 os.TempDir/testing.T.TempDir
  使用可写目录。不猜测 /data 路径，不把 HOME 指向临时测试目录。
- 新增 mobile_std 维护者生成器，复制 Go 1.25.8 选定的完整外部测试源，保留
  字节、断言、原 skip、版权及摘要。只用公开 platform 的 Go App 自动执行
  testing.MainStart/M.Run，并保存完整日志、顶层结果和窗口状态，不导入 GUI/UI。
  原生辅助/Java 仍由维护者预制；本次不改触摸、IME 或 Widget。

### 源码与消费验证

本机 Linux amd64 / Go 1.25.8，仓库外独立工作区与首次目标空缓存；PATH 只有 Go
和会记录调用后失败的 cc/gcc/clang/java/javac/d8 等拒绝工具。三个正式 CLI 目标
均通过，拒绝调用记录为零。维护者此前 NDK/Xcode 制作和本轮补充 net 归档、
Security/resolv stub 不混入消费阶段。普通应用/平台 Go 代码仍从源码编译。

现阶段使用本地移动版 cgo/purego/nanovgo 分支和私有组件；不宣称正式 tag、下载
缓存、组件自动组合或 SDK 再分发许可已经完成。本地 profile 可包含 net 原生
归档，未知 cgo 明确失败；Android os.UserCacheDir/UserConfigDir 的 HOME 语义
尚未因此扩展，不将本例临时目录可用性概括为全部标准库能力已覆盖。

| 验证 | 最终结果与边界 |
| --- | --- |
| 全仓 | 受控源码副本、独立 Xvnc :78/X11/Xfwm4；go test -json ./...：3531 通过、10 既有跳过、0 失败；883 个 Go 文件与交付源码一致 |
| 竞态/静态 | cmd/goui、platform/mobile/base、mobile_std 的 race 和 vet 通过；生成器用临时错误用例证明失败 code 不被吞掉 |
| 桌面目标 | Windows amd64/macOS arm64 的上述测试目标交叉编译通过，-exec /usr/bin/true 不算原生运行 |
| Android | 远端 macOS15.7.7 ARM64 的专用 Android11/API30 ARM64 ATD、Software、1x；最终 134 项官方用例+4 项额外检查，PASS137/FAIL0/SKIP1、code0；CheckJNI 未报告错误；Home/返回后 nonce 不变，后台 frames 9→9，恢复增加到13 |
| iOS | 同一 Mac 的专用 iOS26.3 ARM64 Simulator、Software、3x；同一 134+4 集，PASS137/FAIL0/SKIP1、code0；完成后持续呈现，截图确认状态及内容可读；本例本轮未重新验收 iOS 旋转/前后台组合 |
| 原 skip | 两端均仅 path/TestCleanMallocs，由原始测试的 short 模式条件触发；报告/日志保留原因 |
| APK 静态/签名 | 正式 NativeActivity 入口、没有测试宿主符号，四个 PT_LOAD 均 16384 对齐；Go 与官方 SDK 验签通过；16 KiB 内核运行仍未验收 |
| 真机 IPA | 本机生成 arm64 device Mach-O，LC_BUILD_VERSION platform=2，Payload/app 布局及可执行权限正确；待云平台重签与真实 iPhone 安装/运行，未当作通过 |

最初 Android 验收脚本继承旧 ANDROID_ADB_SERVER_PORT=5039，与新私有 ADB5043
不一致，导致启动等待超时；日志保留，修正环境并清理本轮残留进程后完成最终
验收，没有为此修改生产代码或跳过断言。首次 iOS 截图被系统状态栏覆盖，修正
例程保守顶部留白；没有将其冒充 SafeArea 实现。

构建/原始日志/源码清单/截图在 Linux/Mac 的 /tmp/goui-mobile-build-psdcikpx。
交付 APK、待重签 IPA、Simulator 包、源清单和结果另保存到
/data/xuyang8/artifacts/goui-mobile-stdlib-20261105；最终路径及 SHA-256 由该目录
SHA256SUMS 记录。验证结束关闭专用 X11、ADB、Android 模拟器，
删除专用 iOS Simulator；未修改主会话仓库、安全策略或用户未跟踪文件。

仍待完成：物理 Android/iPhone 的同源产物验收、16 KiB 运行、更多标准库测试文件、
正式自有证书签名、组件分发/许可、公开依赖版本。134 项顶层选择集不是 go test std
全集；本地 HTTPS/localhost 不等于公网 DNS、代理与完整证书链环境已验证。

## 同步主线后的回归（2026-11-05）

feature/mobile-base 以 origin/main `af55926` 为新基线，保留原有职责提交。
Plan/Roadmap 的双方新增段落均保留；rootBase 冲突按原语义合并：主线的帧追踪
继续覆盖阶段，移动端在调用和提交前仍检查实时绘制许可。没有引入新公开接口。
mobile_paint 的 DrawBoxShadow 参数同步为 geometry.UniformCornerRadius(12)，
使其与主线四角圆角 API 一致；其余移动平台实现和预编译组件不变。

验证使用 Linux amd64、Go 1.25.8，受控副本仅包含当前已跟踪源码，未复制或
删除工作区原有未跟踪旧例程。独立 Xvnc :136、1280×960、xfwm4 合成开启，
`GOWORK=off go test -json -count=1 ./...`：3891 项测试/子测试通过、10 项按
原条件跳过、零失败。GUI、platform/internal/mobile、mobileentry 的 race，
GUI/移动工具与支撑包的 vet 通过；定向构建工具、基座、生成器、圆角与阴影
测试通过。既有测试生成的图片留在受控副本，不改用户工作区资源。

消费者以显式仓库外 go.work 接入原移动版 cgo/purego/nanovgo，复用匹配组件；
PATH 中原生编译器/JDK 为记录后拒绝执行的工具，消费调用记录为零。
OpenGL 版 mobile_std 的 Android ARM64 APK、iOS ARM64 Simulator App 与
device 待重签 IPA 构建通过；mobile_paint 的 Android 和 iOS Simulator 原生
二进制链接通过。这里只是编译/组包结果，没有重新执行 App 或官方测试集，
不覆盖此前模拟器实测，也不消除真机、签名与组件公开发布的验收缺口。

桌面交叉编译：Windows amd64、macOS arm64 的全部适用非测试源码目标在
CGO_ENABLED=0 下通过。初次直接 go build ./... 因 mobile_start/mobile_surface
在桌面只有生成器、没有桌面 main 而失败；这两个目录在变基前已相同，按各自
移动端使用边界排除，不修改旧入口。后续显式包清单初轮又包含仅有 _test.go
的目录而失败，改为按 GoFiles/CgoFiles 筛选适用构建包后通过；不将前两次命令
算作通过，也不将交叉编译描述为原生运行。未改测试断言或新增 skip。

备份分支、range-diff、全仓 JSON/跳过清单、交叉编译日志和移动制品保存于
`/tmp/goui-mobile-rebase-cxwm4jhr`；变基前的 25 个未跟踪文件摘要保持一致。
独立 X11 会话已停止；没有修改主会话仓库或依赖仓库，也没有推送分支。

## 移动 FFI 依赖正式化（2026-11-05）

### 实现与发布

移动 ABI 支持收敛到 goexlib/cgo，保留官方 purego v0.10.2：purego 提供动态加载、
原生栈切换和 runtime 回调桥，cgo 负责 ARM64 类型化参数、返回、结构体、HFA、
栈布局与同步指针有效期。Call、CallRet、RegisterFunc、RegisterLibFunc 共享新路径；
CallRaw 保留整数/地址契约，其他架构不改 ABI。平台绑定仍由 GOUI platform 负责。

- cgo：7d333e0 共享 ARM64 分类；2855af7 类型化调用与回归；42c56df 设计及
  验证记录。保留先前 Apple 回调修正与独立 C ABI 例程，main 发布 v0.1.2。
- nanovgo：移动 GLES 提交仅修正作者/提交者为 xuges <xuges@qq.com>，得到
  fce1712，main 发布 v0.0.12；未改写已有公开标签。
- GOUI：在 /data/xuyang8/repos/goui 的 pre-release/v0.7 更新 go.mod/go.sum，
  使用 cgo v0.1.2、purego v0.10.2、nanovgo v0.0.12。依赖模块均通过正常下载获取，
  不再使用实验 purego 分支、未发布依赖源码或本地 replace。
- CheckButtonGroup 仅记录确定生命周期设计，未修改 Widget、weak 或分组行为。

### 验证结果与边界

统一使用 Go 1.25.8，详细日志与制品位于构建机及专用 Mac 的
/tmp/goui-ffi-stable-1dj6hiys。依赖发布后，额外核对模块缓存中的47份 cgo 已跟踪
文件与验证提交一致；GOUI 全仓关闭 go.work、本地 replace 后在源码副本复跑。
副本包含既有未跟踪测试，既有测试的窗口和图片副作用留在副本中，不修改用户资源。

| 范围 | 环境与结果 |
| --- | --- |
| GOUI 全仓 | Linux amd64 / Xvnc :136 / 1280×960 / xfwm4 合成开启，GOWORK=off go test -json -count=1 ./...：4102通过、15项原条件跳过、零失败 |
| cgo Linux | amd64 包测试与 race 通过；ARM64/QEMU6.2 类型化调用、回调、参数/返回、原生线程与 GC/栈增长检查通过 |
| 独立 ABI | Linux ARM64/QEMU、macOS15.7.7 ARM64、Android API30 ARM64、iOS26.3 ARM64 Simulator 均28/28；8 pthread × 8轮，零重复失败 |
| cgo macOS | 56通过、1项原始 CallRaw 紧凑 int32 栈参数限制跳过，race通过；原来两项类型化栈参数 skip 已恢复执行并通过 |
| cgo Windows | Windows10 19045 amd64 原生无头测试通过，使用远端编译的独立 C fixture；不是桌面交互验收 |
| 正式 GLES App | Android API30 ARM64/1x 与 iOS26.3 ARM64 Simulator/3x，确认实际 OpenGL Painter，1x/2x FBO 像素检查通过；134项官方测试加4项额外检查为137通过、1项原 short 条件跳过、零失败 |
| 生命周期 | Android App 前后台往返保留 nonce、后台停止帧更新、恢复后继续；本轮未重跑 iOS 旋转/前后台组合 |
| 消费构建 | Linux 上 Android APK、iOS Simulator App、device待重签IPA通过；PATH中的 C/C++/Java/JDK工具为拒绝执行工具，消费阶段未调用原生编译器 |

cgo 完整 vet 仍报告 cast.go、extern.go、utils.go 的3项既有 unsafe.Pointer 诊断，
不计为通过；Linux amd64/ARM64 的其他分析器在 -unsafeptr=false 下通过。
没有增加忽略条件掩盖新增失败，也没有用 Go 往返测试替代独立 C ABI 参考。

正式依赖下载后，另用只包含应用与 GOUI 的开发 workspace（不包含 cgo/purego/
nanovgo 源码）重新构建上述三个目标，全部通过；模块解析确认三项依赖均为正式
版本且无 Replace。先前模拟器验收使用与已发布模块一致的生产源码，本轮未为了
依赖标签切换重复模拟器交互。

预编译组件仍是维护者提供的本地只读组件，Android net 基座以官方 purego 模块身份
重新导出后消费；这与 Go 模块依赖正式化是两件事。组件自动下载/许可审核、真实
Android/iPhone、16 KiB内核运行、正式签名及云真机重签仍待验收。unsigned IPA
不能宣称已可直接装机。独立模拟器、ADB和X11进程均只操作本轮实例并在结束后清理，
未修改系统安全策略。

## LLD 静态组件体积验证（2026-11-06）

### 范围与构建

本轮只验证宿主链接工具的体积及构建兼容性，不修改 GOUI/Cgo/NanoVGo 生产代码，
不替换既有共享组件。独立纯 Go 最小原生链接器保留为后续独立项目，参考/选择性
复用 Go 链接器代码，消费官方 Go 工具链产生的原生目标文件；不维护 Go 工具链
补丁。本轮未实现该链接器，边界见 DesignMobileBuild 第4.5节。

验证环境：Linux amd64 / Ubuntu22.04 / GCC11.4 / CMake3.22.1 / Ninja1.10.1。
官方 LLVM22.1.8 release 源码包的 SHA-256 与 GitHub release 元数据一致：
922f1817a0df7b1489272d18134ee0087a8b068828f87ac63b9861b1a9965888。
源码未打补丁，构建/制品/日志均在 /tmp/goui-lld-static-lt1uq41s。

主要配置：MinSizeRel；BUILD_SHARED_LIBS=OFF、LLVM_BUILD_LLVM_DYLIB=OFF、
LLVM_LINK_LLVM_DYLIB=OFF、LLVM_STATIC_LINK_CXX_STDLIB=ON；静态libgcc/zlib，
启用函数/数据独立段和链接回收。完整命令保存为 cmake-command.json。
LLVM_TARGETS_TO_BUILD=AArch64 只限定 LLVM 代码生成后端，不代表裁掉其他架构的
普通原生链接能力；LLD上游 ELF/Mach-O/COFF/Wasm 驱动均保留，四种version调用
通过。LLVM_ENABLE_LTO=OFF 指工具自身没有用LTO构建，不是删除工具的LTO输入
处理；日志确认LLVMLTO被构建。未执行完整LLD测试集，不宣称其他格式已验收。

首次编译在Mach-O阶段因精简解压漏掉libunwind/include而失败，保留初轮日志；
从同一已校验源码包补齐头文件后增量构建通过，没有修改LLVM源码或弱化检查。

### 体积与运行依赖

| 项目 | 字节数 | MiB |
| --- | ---: | ---: |
| 新LLD，尚未strip | 65,691,896 | 62.649 |
| 新LLD，strip --strip-unneeded | 48,394,872 | 46.153 |
| 新工具组件，含许可证和清单 | 48,484,470 | 46.238 |
| 新组件tar.gz，gzip6 | 18,827,789 | 17.956 |
| 原Rust LLD+共享LLVM工具组件 | 211,437,566 | 201.643 |
| 原组件tar.gz，同为gzip6 | 67,533,037 | 64.405 |

组件解压体积减少77.07%，压缩体积减少72.12%。这不是严格单变量实验：上游来源、
编译器/优化方式、代码生成后端与原Rust分发包不同，因此不能把全部差值归因于
静态链接，也不据此推断链接性能。新工具仍保留插件支持，未继续裁剪其导出。

readelf/ldd确认只需libm.so.6、libc.so.6和ld-linux-x86-64.so.2，不需要libLLVM、
libstdc++、libgcc_s或libz共享库。最高引用GLIBC_2.34，尚未在其他宿主基线执行；
正式Linux发布需在约定的最低宿主环境重建并验证。这里不是完全静态Linux程序。
LLVM、GCC运行库和zlib声明已随测试包保留，正式再分发审核仍未完成。

stripped LLD SHA-256：
24c9bd9575bff25257b7182133f005327292c1ae44e41e1c96c1d39a2ef34155。
工具包为 lld-static-linux-amd64.tar.gz；精确数据见 measurements.json。

### 兼容验证与缺口

只在仓库外以硬链接复制既有组件，替换副本tools并重新封装清单/摘要；原组件
的6份profile/tools清单摘要前后保持一致。使用原有goui-link参数适配和校验，
不绕过Go版本、归档身份、源码或原生格式检查。消费PATH仍为只提供Go及拒绝
C/C++/Java编译的工具集，使用正式cgo/purego/nanovgo依赖。

- 正式OpenGL ES标准库例程的Android ARM64 APK、iOS ARM64 Simulator App、
  device待重签IPA均构建成功。输出及命令在mobile-build-results.json和各目标日志。
- 独立读取APK内ELF，四个PT_LOAD均至少16KiB对齐；直接读取两份Mach-O，
  LC_BUILD_VERSION平台分别为7/2。Simulator含ad-hoc签名记录，device保持显式
  unsigned路径；不把16KiB静态检查当作16KiB内核运行，不把组包当作真机验收。
- 已跟踪源码0cd2dc0的冻结副本在独立Xvnc :136 / 1280×960 / xfwm4合成开启、
  Go1.25.8、CGO_ENABLED=0、GOWORK=off下，go test -p 4 -json -count=1 ./...：
  3891通过、10项既有条件跳过、零失败。当前主会话并行修改和既有未跟踪测试
  未纳入，既有测试的窗口/图片副作用留在副本，结束后停止本轮独立X11进程。

本轮未在Android/iOS模拟器或物理设备运行新链接产物，未构建Windows/macOS宿主
版本，未做性能比较或公开分发，也未替换正式组件。后续先补齐这些验证，再决定
发行包切换；独立Go链接器继续作为后续计划，不作为本轮交付前置。

## 独立Go链接器与原生编译边界验证（2026-11-06）

用户明确目标：独立Go程序完成受控原生链接，参考/复用Go链接器代码，不维护Go
工具链补丁；同时尽可能减少经过原生工具链编译的代码，而非只压缩整包Go归档。
本轮原型均位于 /tmp/goui-minild-m8sb7mnp，未改GOUI生产实现或现有共享组件。

### 预编译范围

逐成员审计现有归档，区分Go导出数据、Go对象、Go汇编对象和原生ELF/Mach-O：

| 目标/包 | 真正原生对象字节数 |
| --- | ---: |
| Android runtime/cgo | 82120 |
| Android net | 23928 |
| Android purego/internal/cgo | 10568 |
| Android GOUI固定入口 | 5456 |
| Android合计 | 122072 |
| iOS device合计 | 56816 |
| iOS Simulator合计 | 57352 |

Android net归档约4MiB中的绝大部分是Go代码；runtime/cgo归档也主要是Go对象。
这些内容不应因所在包包含少量C代码，就全部作为原生预编译产物按Go版本分发。
直接拼接原生对象的gzip6大小分别为28409/17804/17838字节，仅用于内容量级统计，
不是带完整元数据的可发行组件包。

现场生成验证：以当前Go1.25.8 GOROOT为源，临时overlay保留runtime/cgo/cgo.go的
普通Go定义、去除import C触发；CGO_ENABLED=0，以-shared的Go/汇编标志编译
runtime/cgo，再用Go pack组合14个既有原生ELF对象。未编辑GOROOT，也没有运行
C编译器。Go源码与Go汇编由当前Go编译，编译结果不是复用旧Go对象。最终应用
构建仍使用原有CGO_ENABLED=1的外部链接流程，不宣称全链路CGO_ENABLED=0。

首次链接因省略Cgo原本生成的-llog要求而缺__android_log_vprint；将原生库要求显式
传给既有构建器后通过，保留两轮日志，没有弱化符号检查。这说明原生输入元数据
必须独立保存链接要求，而不能依赖预编译Go对象碰巧带有Cgo指令。

组合库由纯Go生成的小型AArch64原生宿主加载，Go init和runtime.GC完成，写入
独立预期文本。该宿主只调用dlopen/sleep后退出，ELF对象及最终程序均由Go生成，
不是用原生编译器临时做的消费者入口。原生支持对象仍来自此前维护者的NDK构建。

这只证明当前Go版本的runtime/cgo拆分可行。完整net、FFI和固定入口的Go生成
源码、依赖指纹、缓存与原生ABI版本契约仍需设计/实现；不能混用旧Go对象或删除
Go版本校验冒充跨版本兼容。进一步移除线程/TLS等C代码要适配fakecgo的库初始化、
Bionic pthread/TLS、外部线程销毁与GC。purego v0.10.2的原样无Cgo实验：Linux
ARM64普通exe可编译；Android绑定缺少内部Cgo动态加载函数，c-shared以及iOS
exe/c-archive被Go工具链的外部链接要求拒绝；保留fakecgo-probes.json。

### 独立链接器

只依赖Go标准库，读取官方Go链接器产生的原生对象及NDK链接输入；本轮未复制
cmd/link内部包或修改Go工具链。输出支持范围为实际Android ARM64样本涉及的
ELF共享库、57个动态导入/57个导出、GNU hash、GOT/PLT、RELRO、立即绑定、
构造/析构数组、12种实际ARM64重定位及已知CFI编码的EH查询表。未知类型拒绝。

正式GLES标准库例程的输入为27个原生对象、192699条加载段重定位，无TLS段；
扫描既有builtins归档但实际无需装载成员。仅生成加载所需段，未保留非加载DWARF；
所以较小APK不能作为与LLD等价的体积优化结论。通用TLS、分支跳板、符号版本、
完整归档语义、C++/LTO、增量链接和Mach-O均不属于已完成承诺。

测试接入没有改生产NativeTools：仓库外捕获包装器在Go模式返回明确标注的测试
LLD版本兼容信息，真正链接交给Go原型，MINILD_REFERENCE_LLD指向不存在路径。
未来正式接入必须定义真实后端身份和版本契约，不能沿用该测试适配方式。

Linux/Windows/macOS的CGO_ENABLED=0工具原生重放同一输入，输出字节相同：
5885b1d9a01affffa0d312f8b35e9c1e271c0b60e29bd423936f49999ecf77f3。
Linux版本ldd确认不是动态可执行文件，不依赖glibc。Go1.25.8，-s -w大小如下：

| 宿主 | 可执行文件字节数 | gzip6字节数 |
| --- | ---: | ---: |
| Linux amd64 | 2498744 | 1086930 |
| Windows10 19045 amd64 | 2600960 | 1117735 |
| macOS15.7.7 ARM64 | 2468722 | 1014797 |

这只是受控ELF原型，不预测完整ELF+Mach-O链接器最终大小或性能。单元/race/vet
通过：GNU hash直接对照NDK生成的表，检查输出段权限/入口和已知拒绝路径。
重定位范围及源代码见原型README，正式化前仍需审查错误处理、输入限制与测试。

### Android运行

Mac上专用Android API30 ARM64 ATD、SwiftShader、1x、CheckJNI，私有ADB5059、
模拟器端口5630。运行两条独立证明：现场Go部分组合库加载/GC，以及纯Go链接
的生产OpenGL标准库APK。后者134项官方选择用例加4项检查，137通过、1项原始
short条件跳过、零失败；GLES 1x/2x FBO像素检查、后台暂停帧及恢复后nonce保持
通过。Windows/macOS这里只运行构建工具，不是移动App运行；未做iOS或物理设备
验收。模拟器和私有ADB在finally中清理，没有修改用户安全设置或工作桌面。

原型源码、可移植输入、三宿主工具、运行证据和SHA256清单已另存
/data/xuyang8/artifacts/goui-minild-20261106-m8sb7mnp，source.tar.gz不包含LLVM或SDK
工具，replay-inputs.tar.gz包含本轮私有验证的原生输入，未公开分发。

最终已跟踪HEAD816a1b7的冻结副本在独立Xvnc :136 / xfwm4下执行
CGO_ENABLED=0、GOWORK=off的go test -p 4 -json -count=1 ./...：3893通过、
10项原条件跳过、零失败。未纳入用户未跟踪测试，窗口/图片副作用留在副本；
停止本轮显示进程。此次仅更新设计/计划，未将原型集成到框架或提交生产代码。


## 独立 Go 链接器正式接入（2026-11-06）

本节先记录独立进程的验证阶段；最终接入已按后续确认改为直接引用 Go 功能包，
不要求下载独立 golink 二进制，见文末“直接引用功能包并提交”。

### 决策与职责

本轮确认：如果每个 Go 版本的基座够小，就保留预编译产物。普通 Go 包仍现场
编译；runtime/cgo、入口与必要的 net/FFI 归档继续严格匹配 Go1.25.8、目标、
模块/源码身份及导出 ABI。前轮 runtime/cgo overlay 是可行性实验，不作为此次
生产构建前置，也没有删除版本校验或尝试跨 Go 版本混用。

独立模块位于 /data/xuyang8/repos/golink，只有标准库依赖、Go 源码和文档。
它实现 native ELF 链接、版本和 request format 1 JSON；没有 GOUI、组件清单、
SDK 探测、签名或 UI 策略依赖，不调用外部进程。不维护 Go 链接器补丁。
GOUI 仅在 platform/mobile/base 接入组件 format 2 的 golink 选择和 JSON 转换，
保留 format 1 LLD；未知目标与选项明确失败，无静默回退、无临时 LLD 版本模拟。
消费者的 goui build 命令不增加后端特例开关，由已校验的组件决定。

链接器仍是受控 Android ARM64 实现：没有 DWARF 输出、通用 TLS、分支跳板、
符号版本、COMDAT 或完整 GNU 归档顺序语义。支持格式外的输入应报错。
原生输入解析增加段/重定位界限、归档成员/尾部、CIE/FDE 和大小/对齐检查；
库只将预期输入错误转为 error，不吞掉 runtime panic。最终结果先在临时文件
完成，再发布；GOUI 另外保留格式/段约束检查和原产物保护。

### 基座测量

量取已验证的完整 base 目录，包含 base.json 和其全部归档，不只量 C 对象；
以 tar+gzip6 压缩。原始归档、来源和严格兼容约束均未改动：

| 目标 | 文件合计字节 | tar+gzip6字节 |
| --- | ---: | ---: |
| Android ARM64 | 4840219 | 1051041 |
| iOS device ARM64 | 750792 | 136416 |
| iOS Simulator ARM64 | 751331 | 135605 |

不包括 host Go driver、golink/LLD、SDK stubs、资源、打包和签名工具，不能称为
整个工具包下载大小。runtime 基座、Go driver、目标 SDK 输入与链接工具仍可按
各自身份独立缓存/发布。现阶段只有 Go1.25.8，其他 Go 版本须各自制作与验证。

### 验证过程与结果

验证目录 /tmp/goui-golink-w9cbv3xn。冻结当前已跟踪 HEAD 加本轮文件 overlay，
不纳入工作区原有未跟踪旧测试，未修改主会话业务代码。

- 独立模块单元、race、vet 通过。测试用 Go 写 ELF/归档，不需要 NDK；覆盖
  ABS64 动态重定位、CALL26 的固定机器码、COMMON、强弱符号、内存权限、
  RELRO、确定性和失败保留。GNU hash 预期摘录自真实 NDK libc.so 表并记录摘要。
- GOUI 新增正式协议与后端歧义/版本、失败退出、无效产物、未支持选项的回归。
  第一轮测试 fixture 与工具清单共用目录触发 undeclared file，修正测试目录
  隔离后通过；没有放宽组件完整性校验。构建脚本最初误找不存在的 cmd/go-mobile，
  改为复用本次未改变的已验证版本 Go driver，不增加同名生产命令。
- 正式 goui build + 精确匹配预编译基座 + golink 组件完成 Android APK；消费者
  PATH 仅 Go 与拒绝 C/C++/JDK/LLD 的桩，独立空缓存构建和二次审计构建产生
  字节一致 APK。拒绝桩主动调用自检能够记录/失败；实际构建调用记录为空。
  无 LLVM 文件被放入新组件，所有组件文件与工具摘要继续校验。
- APK v2/v3 验签通过；原签名器对没有 v1 MANIFEST 的提示保留。目标 minSdk29
  不把缺少 v1 当作三平台运行失败，也不据签名结果宣称应用运行成功。
- Android API30 ARM64 ATD、SwiftShader、1x、CheckJNI，安装并实际运行生成
  APK：134项官方选择用例加4项额外检查，137通过、1原 short 条件跳过、零失败。
  GLES Painter 的1x/2x FBO像素、后台停帧、恢复帧推进及 nonce 保持通过。
  专用模拟器5630和ADB5059已清理，未操作用户桌面或修改安全设置。
- 正式 Go CLI 在 Linux amd64、Windows10 19045 amd64、macOS15.7.7 ARM64
  原生重放同一27对象/192699重定位输入；输出 SHA256 均为
  5885b1d9a01affffa0d312f8b35e9c1e271c0b60e29bd423936f49999ecf77f3。
  Windows/macOS 此处运行的是构建工具，不是 iOS 或物理移动 App。
- GOUI 冻结源码在独立 Xvnc :136 / xfwm4 合成会话，CGO_ENABLED=0、GOWORK=off，
  go test -p4 -json -count=1 ./...：3897通过、10项原条件跳过、零失败。
  既有测试窗口和图片副作用留在副本，会话已停止；测试范围见 source-record.json。

当前 APK SHA256：a5763604c4b40822342c15adaed48528a4a4350a6f1b46c8472d3cdefb0a5b04；
原生 payload SHA256：3d13a065caf522575fc2322b78e42e735b99980dbdb22ca06ee2fd9782f24bef。
捕获输入带显式 SONAME 的重放样本与正式构建无 SONAME 的 payload 不是同一
输入，不要求它们互相等于；各自的重复输出/远端运行证据独立保留。

### 剩余边界

本轮只正式接入 Android。iOS Mach-O、更多 Go 版本、物理设备/16 KiB 内核、
公开组件分发与稳定 golink 发布尚未完成。用户不需要原生编译器，但内部最终
构建依然使用 Go external-link 和 CGO_ENABLED=1 来消费预编译支持；没有宣称
所有移动构建模式已经支持 CGO_ENABLED=0。没有将 SDK、私有输入、产物或密钥
提交到框架；本轮不打 tag、不推送独立仓库。下一阶段先补 Mach-O 后端和 iOS
Simulator 运行，再制作可分发组件，不为缩小基座优先重写线程/TLS。


独立工具使用 CGO_ENABLED=0、-trimpath、-s -w 构建：Linux amd64 2519224字节
（gzip6 1100111），Windows amd64 2618880字节（1129447），macOS ARM64
2469234字节（1023485）。Linux ldd 显示不是动态可执行文件，不依赖 glibc。
最终补充归档尾部残缺检查后，三宿主再次重放保持同一 ELF，第三次空缓存 APK
与已在模拟器运行的附件逐字一致；没有仅凭旧工具的运行结果推定新产物。

源码归档、GOUI 切片补丁、三宿主工具、APK、运行/构建日志、冻结源码记录、
尺寸和 SHA256 清单持久保存在
/data/xuyang8/artifacts/goui-golink-20261106-w9cbv3xn。未公开发布或推送，仓库内
只有构建接入、测试和文档；独立模块源码保存在 /data/xuyang8/repos/golink。


### 直接引用功能包并提交（2026-11-06）

最终确认：GOUI 不要求用户另行下载 golink 可执行程序。platform/mobile/base
直接 import github.com/golang-gui/golink，调用 Link(Request)，随已有构建工具
编译。删除此前 GOUI 侧重复的请求结构、JSON 临时文件及 golink 子进程调用。
独立项目可选 CLI 仍可单独使用，不进入 GOUI 消费组件。

NativeTools format 2 的 golink 是版本字符串 "0.1.0"，核对库 Version；没有
file 字段，也不再执行 --version。只包含清单时 files 可为空，仍遍历拒绝未声明
文件。format 1 的 LLD 可执行文件、摘要与版本校验不变；目标输入仍必须有文件。
重复声明后端、错误版本、未支持目标、非法输入和取消都不发布结果。

新增回归直接调用真实 Go 链接库：Go 创建 ARM64 对象及 stub，验证链接成功、
请求数据完整、失败保留原结果、取消与版本匹配，不再使用假 golink 进程。
先前加到假 LLD fixture 中的 JSON 分支已撤回。

独立库 /data/xuyang8/repos/golink 的本地提交535077c，作者/提交者
xuges <xuges@qq.com>，本地标签v0.1.0。GOUI go.mod/go.sum 固定该模块版本；
没有提交本机绝对路径 replace。用户明确选择只提交本地、不发布，所以没有创建
公开仓库、推送提交或标签；其他机器尚不能从公共模块源直接获取该新依赖。
本地可以用临时 go.work 关联两个源码目录。为检查无 replace 的消费者，验证
还从该提交制作本地模块代理，按正式 Go 模块规则生成并核对 go.sum；代理与
GONOSUMDB 仅在测试进程环境使用，不修改用户或系统配置。

直接引用后的验证目录 /tmp/goui-golink-library-0_aj1cue：

- golink 的 race、vet 和 GOUI base 针对性 race/vet 通过；base、goui、goui-base
  包测试通过。全部测试使用本地源码/提交，未声称依赖已公开发布。
- 冻结HEAD816a1b7及本轮明确文件，在独立Xvnc :136/Xfwm4，CGO_ENABLED=0，
  GOWORK=off，go test -p4 -json -count=1 ./...：3897通过、10项原条件跳过、
  零失败。未纳入主会话并行修改或原有未跟踪文件，显示会话已清理。
- 新组件 tools 目录只有 tools.json，没有 golink 二进制。重新编译 goui 和
  goui-link，沿用版本匹配 Go driver，正式 goui build 在空缓存/拒绝编译器 PATH
  完成 APK；C/C++/JDK/LLVM 调用零，APK v2/v3验签通过。
- 新APK与上一阶段已经在Android API30 ARM64模拟器运行并通过137项/1原skip、
  GLES像素及生命周期检查的APK逐字一致，SHA256仍为
  a5763604c4b40822342c15adaed48528a4a4350a6f1b46c8472d3cdefb0a5b04。
  因附件一致，没有重复启动模拟器。当前库接入不等于iOS后端已完成。


## iOS Mach-O 独立 Go 链接后端（2026-11-06）

### 实现与边界

独立 golink 增加 iOS ARM64 设备/模拟器后端，GOUI 仍直接 import 功能包并调用
Link(Request)，不下载/执行独立 golink，也不在消费者调用 Apple/LLVM 编译器。
标准 Go c-archive、Go pack、精确 Go1.25.8 基座和 UIKit 协议/入口预编译对象保持
原有归属；本轮不修改 runtime、iOS 窗口/排版/图形实现或任何 GUI/UI 行为。

Request 使用 IOSOptions 区分 variant/minimum/sdk；Libraries 为显式 TBD 根库，
Reexports 为额外已封装 stub，只解析重导出闭包，不产生额外根加载或宿主搜索。
TBD v4 的目标、导出、Objective-C 名称、库 current/compatibility 版本均保留。
Simulator SDK 的三个宿主系统库标签按 LLD22.1.8 InputFiles.cpp 中确切列表适配；
只放宽 arm64-ios-simulator 到这三个嵌入的 arm64-macos 文档，不放宽其他输入。

Mach-O 合并同名原生节，处理当前输入的 ARM64 UNSIGNED、SUBTRACTOR、BRANCH26、
PAGE21/PAGEOFF12、GOT_LOAD 与 ADDEND 组合，保留 dyld rebasing/eager binding、
导出 trie、Go/UIKit 初始化数组、Objective-C 协议元数据、只读常量和展开信息。
未知 CPU 子类型、平台/部署版本、越界、TLS、指针认证、LSDA/personality、LTO、
branch 溢出等明确失败。私有 SDK 输入、签名材料和构建产物均不进入源码仓库。

ad-hoc 代码页签名复用 Go1.25.8 cmd/internal/codesign 的 BSD 源码，去掉内部
hash 常量依赖，保留版权、LICENSE 和 THIRD_PARTY.md。没有 Go 工具链补丁。
裸 Mach-O 签名与 App 的 Info/资源封印是两个阶段；前者不替代设备证书/profile。
当前生成 unsigned、可由用户或云真机重签的 IPA，不宣称已通过物理设备安装。

### 真实输入与对照

本轮捕获现有正式 Go/GLES 标准库例程的 LLD 输入，只在私有组件副本中保存归档
和参数，未改变已有共享组件。Simulator 样本16个原生对象，原始重定位类型为
0/1/2/3/4/5/6/10；DWARF 非加载段省略。设备和模拟器分别使用其已验证的基座、
SDK stub、入口对象，禁止交叉混用。参考 SDK 为 Xcode26.3 / iOS SDK26.2。

对同一输入核对 LLD 与 Go 输出：平台/最低版本/SDK、两个模块初始化指针、
_main/goui_run/crosscall2 等关键导出、W^X，以及共同根 dylib 的版本一致。
不同工具的 LC_BUILD_VERSION ntools 可以不同（LLD=1，独立 Go 不伪造工具 ID）；
初轮比较误把这个调试元数据也要求相同，确认格式定义后只比较相同契约的字段。
不把不同段布局、符号表或签名的字节差异当作链接失败，也不要求与 LLD 整文件相等。

三宿主原生工具对同一捕获输入的最终输出字节一致：

| 目标 | Linux amd64 / Windows10 19045 amd64 / macOS15.7.7 ARM64 共同 SHA256 |
| --- | --- |
| iOS Simulator | e267ae2adc659c97f7f6cc01da73a2c067a3672bb221b80f48d3da713e63c77e |
| iOS device | 995fe0734cfb55ba74df2cfce445b8d4ccbd68754ba2534afc81f1abfb06f8c9 |

macOS 原生 codesign --verify --strict 通过两份裸 Mach-O。Android 真实27对象样本
重放仍为5885b1d9a01affffa0d312f8b35e9c1e271c0b60e29bd423936f49999ecf77f3，
与上轮运行过的原型逐字一致；共用归档读取调整没有改变既有 Android 输出。

### 最终构建与验证

最终功能包 v0.2.1，构建验证位于 /tmp/goui-golink-ios-3d9t9uel：

- 独立库单元/race/vet 通过。纯 Go 构造 Mach-O、TBD 与归档，不依赖 Xcode；
  覆盖导入 stub、非惰性 GOT、指针 rebase/bind、入口、PIE/权限/只读常量、代码页
  哈希、设备/模拟器、SDK版本、TBD目标/引号/重导出及拒绝越界/错误 CPU 等。
  GOUI base 的实际库调用、两个 variant、声明输入完整性、取消与失败保留亦通过。
- 维护者环境已有 SDK 输入；消费者 PATH 仅 Go 与拒绝 C/C++/JDK/LLD 的桩，
  独立空 Go 缓存生成 Verified-Simulator.app 和 Verified-Resign.ipa；拒绝调用零。
  仅复用版本匹配的 Go driver 和源代码包，不需要消费者 LLVM/Xcode 工具链。
- 最终冻结源码为HEAD f4315d6及本轮明确文件，独立Xvnc :136 / Xfwm4 / Linux
  amd64 / Go1.25.8，CGO_ENABLED=0、GOWORK=off，go test -p4 -json -count=1 ./...：
  4483通过、10原条件跳过、零失败。该基线包含主会话已提交的边框更新，不与早前
  3897项的范围混为同一次统计。未纳入主会话未提交内容和原有未跟踪测试。
- 一轮全仓出现新私有模块缺少 go.sum 的 setup failure；补齐 v0.2.1 的选择后
  校验项重新运行通过，没有放宽 skip/断言。使用从本地提交生成的私有模块代理
  和进程范围 GONOSUMDB；不修改用户全局配置，也不宣称模块已公开发布。

第一次真实样本暴露 Foundation TBD 大型 flow list 的重复扫描，改为增量扫描，
不再每读一行扫描此前整个列表。第一轮签名测试在 .app 内直接验证未封印资源的
可执行文件，命令按 bundle 解释而失败；改为裸文件验证代码页，再单独完成测试
bundle 的 ad-hoc 资源签名，维持打包与链接职责边界。初始与最终版本分别验收，
不以仅打开窗口或一次链接成功代替运行测试。

设备 IPA 的 Mach-O header/metadata/签名已检查，真机与自有证书/profile安装未执行。
Simulator 的包签名为测试机 ad-hoc，无开发者凭据和商店流程。SDK输入没有公开分发；
后续仍需处理组件发行/许可证、更多 Go/SDK 版本与物理设备。

本地提交：golink 2e9979d（输入/签名工具）、96b758b（iOS链接）、e466372（库版本
元数据），最终本地标签v0.2.1；0.2.0是本轮保留的初始实现检查点，均未推送。
GOUI 只更新功能包依赖与构建接入/测试/文档，不包含 SDK、私有捕获输入或 App。


最终 v0.2.1 的模拟器复验：macOS15.7.7 ARM64 上专用 iPhone16 / iOS26.3
Simulator，实际屏幕3x、OpenGL ES Painter；134项官方用例加4项检查，137通过、
1项原 short 条件跳过、零失败。包含显式 runtime.GC 与 GLES 1x/2x FBO 像素检查。
先在 bundle 外核对原始 Go ad-hoc 代码页签名，再用测试机 ad-hoc 封印 App 资源并
核对严格签名。模拟器运行的是最终附件，不只重放初始样本；测试结束已关闭、
删除本轮 UDID，没有操作用户桌面、钥匙串或已有模拟器。

最终消费者 Simulator Mach-O：
d76ca2355194aa1a206dc1b17c463282d0556130f7bcceeb0f27613991d27d47；
设备 Mach-O：10524396fa41e2cd21bdb444b9c190db5866239eafe45358062bc11b2aff7cb9；
Verified-Resign.ipa：54f8027b5ccdada83e67b53a5b67f7c98b1a2fb9448b02b7dac42a77a6ac3962。
它们使用正式构建驱动，Go build ID 与早先捕获样本不同；跨宿主相等检查使用同一
捕获输入，运行验收使用完整最终 App，不将不同输入的哈希混作不一致。

可复现输入、独立库源码归档、最终 App/IPA、三宿主工具、构建/运行/失败日志及
SHA256清单位于 /data/xuyang8/artifacts/goui-golink-ios-20261106-3d9t9uel。
其中 replay-ios.tar.gz 含私有 SDK stub，仅本地验证保存，没有公开分发。
GOUI 接入与设计记录继续分开本地提交；不推送源码、标签或附件。
