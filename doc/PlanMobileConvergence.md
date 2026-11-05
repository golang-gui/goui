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
