# 移动端启动、线程与生命周期

2026-11-08 设计边界更新：下文双线程、先创建逻辑窗口再 Run 等内容记录已有验证
实现，不再作为新设计必须保留的约束。后续从移动原生使用场景复核入口、宿主及
生命周期，方向是让 GUI 操作归属原生 UI 主线程；不因此强制桌面采用同一入口，
新的启动方向为 NewApplication → ConnectActivate → Run，GUI Surface 与可管理 Window
分开，具体见 [DesignPresentationHosts](DesignPresentationHosts.md)。共享范围以
[核心设计原则](Design.md#cross-platform-scope) 为准，复核与待验证事项见
[PlanMobileAPI](PlanMobileAPI.md)。本轮平台接口迁移不重写下文原型的线程实现。

状态：第一切片启动/事件线程、第二切片逻辑窗口/Software 表面已实现并完成下述阶段验收；
Android Platform/GUI 第三切片曾按旧接口接入；基础 Window 拆分后，旧 GUI Window
演示须迁移到 Surface 再验收。此处仅保留阶段记录，不表示当前移动 GUI 可用。
后续 Typography/GLES 最小后端、原生目标借用与 iOS 后台执行保护见
[DesignMobilePlatform 第 10 节](DesignMobilePlatform.md#10-两端-typography--graphics-最小实现2026-11-04)。
后续包结构、两端共用 OpenGL ES 和文字接入顺序以
[DesignMobilePlatform](DesignMobilePlatform.md) 为准；本篇保留已实现的生命周期契约。
构建基座与链接工具见 [PlanMobileBuild](PlanMobileBuild.md)。开发使用独立的
`goui2/feature/mobile-base`，不改变主会话的桌面实现。

## 目标和参考

应用仍使用普通 Go main，可先创建逻辑窗口，再进入 Run。Android 不能在
NativeActivity.onCreate 内阻塞运行 Go 事件循环或等待原生绘制表面；iOS 的
UIKit 事件循环同样由系统主线程拥有。

参考与证据（2026-11-04）：

- [Android NDK NativeActivity 回调](https://developer.android.com/ndk/reference/struct/a-native-activity-callbacks)：
  回调运行在系统主线程；窗口/输入队列销毁回调返回后不得再访问对应对象。
  redraw 回调还具有完成当前画面的时序要求。
- [Qt 6.8 Android](https://doc.qt.io/qt-6.8/android-how-it-works.html)：
  QtThread 运行应用 main，原生 UI 线程执行要求主线程的操作。
- [Go mobile Main](https://pkg.go.dev/golang.org/x/mobile/app#Main)：
  应用回调与原生主线程分开。采用线程分工，不引入其 channel 事件 API。
- [UIKit](https://developer.apple.com/documentation/uikit)：原生 UIKit 操作遵守
  系统主线程约束；不是任意 goroutine LockOSThread 后就能调用。
- [Apple 生命周期](https://developer.apple.com/documentation/uikit/managing-your-app-s-life-cycle)：
  Scene 生命周期独立于进程，断开或后台不直接等于应用退出。
- [Go 1.25.8 runtime](https://github.com/golang/go/blob/go1.25.8/src/runtime/proc.go)：
  c-shared/c-archive 完成包初始化后不自动执行 main.main。

上述为文档/源码约束。独立原生宿主验证只证明已实际覆盖的启动和回调路径，
不能代表 GUI、输入法、GPU 恢复或 iOS 真机已经完成。

## 线程归属

系统主线程拥有 Activity/UIKit 与原生事件循环。移动端启动层另起一个固定
OS 线程的 Go goroutine，调用用户 main；Application、Widget、布局、绘制调度
和 ui 协调均归该 GOUI 线程。原生回调向 GOUI 事件循环投递；必须在原生主线程
执行的操作由 platform 调度。不能把每个 Widget setter 改成跨线程调用。

桌面现有 main 线程和 Run 行为不变。移动端不依赖 platform 的桌面 init 锁定
系统主线程；正式后端接入时明确拆分 build tags。不得让 Android 选中 X11，
或让 iOS 选中 AppKit。

禁止双向同步等待：普通原生回调异步投递；原生对象销毁等必须确认完成的交接
需要专门屏障，期间 Go 侧不得反向等待系统主线程。原生输入同步查询和错误反馈
另行核对契约，不以无条件 Post、缓存请求值或隐藏错误代替实际结果。

## 入口与构建

构建工具通过受限 overlay 为 main 包添加注册文件，调用
`mobile.RegisterMain(main)`。init 只注册，不进入 UI 循环。生成文件不写进项目，
不覆盖已有文件，不改 GOROOT。不使用 go:linkname 访问 main.main。

`platform/mobile/internal/entry` 提供标准 cgo 固定入口，等待正常 Go 包初始化后，
仅把调用交给 internal/mobileentry。Android 的 ANativeActivity_onCreate 传递创建
参数，由 android Go 包安装回调及主线程投递，再调度一次应用 main，及时返回
系统循环。iOS 的 goui_run 从最小原生 main 调用一次，由 ios Go 包在真正主线程
进入 UIApplicationMain；didFinishLaunching 回调只调度 Go main，不阻塞 UIKit。
独立启动参考没有安装 UIKit Go 后端时，goui_run 仍只调度 main，并以 0/-1 报告
是否接受注册入口。不能将这一参考路径的立即返回语义套用于正式 iOS 主入口。
入口包不包含 Activity、Scene、Widget 或窗口协议；普通 Go 依赖从源码编译。

main 每个进程执行一次；重复调度不重新执行 main。正式 iOS 原生 main 入口仅调用一次。应用函数不公开为
动态符号；后续原生回调使用私有回调地址。Go runtime、注册表和回调必须比
原生宿主长寿；Android c-shared 保留 stock Go 的 NODELETE 行为。

Android 创建入口允许等待 Go 初始化的必要过程，但不能等待用户 main 或 Run。应用
init 不应执行 GUI 操作或长时间业务初始化；冷启动验证应分别记录 Go 初始化
与入口调度耗时，不把某次较快的启动作为任意 init 都不会触发 ANR 的保证。

维护者命令 `goui-base build -entry` 启用注册文件；默认仍支持无应用入口的
底层 ABI fixture。消费端无原生编译器。对应固定入口归档缺失时明确报错。
公开 cmd/goui 移动命令和安装包组装在后续接入。

受限 overlay 只允许一个尚不存在的 main 包 Go 文件，内容必须与内置模板逐字
一致；不能修改依赖、native 包或 runtime。普通任意 overlay 仍被拒绝。正常
归档源码、模块身份、依赖导出 ABI 与 build ID 校验全部保留。

## 逻辑窗口与表面

分开管理进程/Application、逻辑 Window/Widget 树、Activity/Scene、绘制表面。
NewWindow 不同步等待绘制表面；Show 记录请求，实际可用性由原生事件上报。
表面创建后提供实际 DIP/像素尺寸，布局并绘制。表面销毁时先停止并确认不再
访问旧对象，再允许原生回调返回。延迟任务携带表面 generation，拒绝旧任务。

表面丢失不发 DestroyEvent、不卸载 Widget 树，不清空选择、滚动和 ui.State。
恢复后重绘。窗口永久结束才进入 DestroyEvent 和既有资源销毁路径。

复用 rootBase 的 surfaceEpoch、绘制失效和帧调度。可绘制条件由窗口存活、
实际可见与表面可用共同决定，避免某个 resume 覆盖其他暂停原因。
WindowState 仍为窗口呈现状态；前后台生命周期不塞进该枚举，焦点单独报告。
普通 Timer 不统一暂停，动画跟随可绘制状态；系统不保证后台持续执行。

GPU 表面和 GPU 上下文不同。表面更换尽量保留上下文和 graphics.Image；真正
context loss 需要明确资源恢复契约。graphics.Image 不保留源图片引用，不能
声称重新创建 Painter 即能自动恢复所有图片。第一份真实 GUI 使用 Software。

## 尚待确认的公共契约

- 生命周期的最小查询/信号及多 Scene 的聚合规则，先由原生单窗口实测确定。
- Activity/Scene 重新关联与永久关闭的区分，不重跑 Go init/main。
- iOS 不把进入后台当作退出；现有 Quit 和最后窗口关闭退出策略须单独设计，
  不直接沿用桌面进程退出假设。
- 首版只接入系统提供的一个内容窗口；不伪造桌面多窗口、位置和工作区能力。

## 开发切片与验收

1. 入口与事件线程：注册模板、最小固定 ABI、线程固定、顺序 Post/Run/Quit；
   真正 NativeActivity/UIKit 回调调用 goui_run 后能继续响应，用户 main 阻塞
   Run 期间原生循环仍工作，重复启动不重跑，GC/线程回调后线程归属不变。
2. 原生逻辑窗口和 Software 表面：先创建后 Run；旋转、前后台、表面替换，
   销毁后零访问与无旧帧提交。
3. 先验证 Android 排版，整理平台包与构建选择，再接 iOS CoreText/Android 文字。
4. 最小 Platform、触摸和 GUI/UI：延迟 Painter、树保留、恢复布局；随后补齐
   生命周期/退出、IME 与两端 GLES，具体顺序见 DesignMobilePlatform。

第一切片不修改 gui/ui，不以测试宿主自带的原生文字/按钮声称 GOUI 控件已运行。
包内测试覆盖注册/重复启动、队列次序、重入、取消和并发 Post/Quit；测试宿主
位于 tests/window/platform/mobile_start，说明实际环境、步骤、预期与退出方法。
运行全仓测试、针对性 race/vet，并分别记录 Android/iOS 原生运行和构建宿主。

## 第一切片实现与验收（2026-11-04）

实现位于 platform/mobile、其 internal/entry、platform/mobile/base 的 entry 文件
和 goui-base build -entry。纯 Go 启动/队列源码正常编译；固定 ABI 叶包独立成
原生归档，与 runtime/cgo、FFI 的归档一起在消费清单中逐包校验。通用 runtime
不导入平台或 UI，入口归档不包含宿主/事件循环，清单聚合不改变资源职责。

消费构建使用 list -find 定位 main，避免普通 list 请求 cgo 辅助生成源码。
未知 overlay、覆盖已有文件、修改模板和依赖覆盖均拒绝。注册文件只在临时
目录存在；source/build ID/依赖 ABI 检查保持原样。

| 验证 | 环境与结果 |
| --- | --- |
| Linux 消费 | amd64 / Go 1.25.8；Android、iOS Simulator、iOS Device 注入并链接通过，PATH 无原生编译器，日志确认未生成 cgo 源码 |
| Windows 消费 | Windows 10 amd64 / Go 1.25.8；三个目标同样通过，保留原有系统安全配置；本轮未运行 Windows 产物的移动 UI |
| Android 原生 | macOS 15.7.7 ARM64 上的 Android 11 ARM64 ATD；十次 adb 原生点击、三次后台往返、结束 Activity 后同 PID 重建通过；mainCalls=1、threadFailures=0，计数保留为 10 |
| iOS 原生 | 同一 Mac 上 iOS 26.3.1 ARM64 Simulator / UIKit；独立 XCTest 十次真实按钮、三次 Home/返回、停止 Run 后重复启动通过；mainCalls=1、threadFailures=0、returned=1 |
| 包内 | 注册并发/非法注册/返回后重复启动、事件次序/重入/退出/并发投递、overlay 和消费编排回归通过；针对性 race、vet 通过 |
| 全仓 | 已跟踪源码及新增文件的隔离副本，显式 replace 到已验证 FFI 修复分支：go test -count=1 ./... 为 3491 通过、10 既有跳过；未删除/放宽断言 |

固定入口最大观测时间 Android 为 6.803 ms、iOS 为 15.223 ms，仅是本轮功能
验收记录，不代表冷启动总耗时或性能保证。Android ELF 确认 NOW/NODELETE；
Activity 重建不卸载仍在运行的 runtime。测试专用 iOS 设备已删除，Android
模拟器与专用 ADB 已停止。

原目录 go test ./... 失败：四个未跟踪旧例程使用过期接口，未修改；另外 demo
出现过一次启动段错误。本轮新增包不在 demo 的依赖路径，未修改 HEAD 的独立
副本全仓及 demo 二十次重复均通过，该次启动失败原因仍未定位，保留原日志，
不把原目录全仓计为通过。前述隔离副本使用的 FFI 替换与原始依赖不同，不能
由其通过反推原依赖不存在问题。

原生交互使用 XCTest 的真实按钮和 Home 事件，不将 simctl openurl 请求被接受
视作应用已经处理 URL。失败与最终日志均保留在仓库外。

原生桥接和事件队列不等同于 platform.Platform 移动实现。后续仍须完成逻辑窗口、
表面交接、GUI/UI、生命周期公开契约、退出、输入和渲染恢复；iOS 真机及
Android 16 KiB 内核运行仍未验收。本机及 Mac 的产物/日志目录为
/tmp/goui-mobile-start-stage1-xq3iq5zk；Windows 使用同名用户临时目录。

## 第二切片：逻辑窗口与 Software 表面（2026-11-04）

common.Surface 增加 SurfaceState 查询（Ready、Generation），替换式目标通过
SurfaceEvent 通知；VisibilityEvent 与 Focus/桌面 WindowState 分开。桌面目标
保持 generation=1，最终不可用由既有 DestroyEvent 报告；移动目标替换或绘制
配置改变时推进 generation。Ready 不等于可见，NativeHandle 查询不延长借用期。

platform/mobile.Window 实现 common.Window，先创建逻辑对象，实际 Size 来自
原生回调。Show/Hide 请求 OS 托管内容视图可见性，不负责启动 Activity 或将
应用移到前台。只支持一个不透明内容窗口；Title/Parent 等未接入能力返回
ErrUnsupported，不伪造桌面能力。尚不注册为完整 Platform，也不修改 GUI/UI。

每个原生目标有可撤销绘制许可。Native callback 可同步撤销许可并等待正在
进行的 native present 结束，再异步上报 GUI；撤销不等待 GUI 处理消息。已排队
的 attach/paint 使用目标与 generation 检查，不能复活失效目标。Software CPU
栅格化可以结束，但 End 不得访问已撤销目标；失效请求保留至恢复后重绘。

当前平台实现已迁回 Android/iOS 各自 Go 包，详见
[基座收敛计划](PlanMobileConvergence.md) 与 [平台设计第 13 节](DesignMobilePlatform.md#13-go-平台适配与最小基座收敛2026-11-05)。

- Android：Go 绑定 NativeActivity 回调、ALooper/eventfd、JNI 可见性与 NDK 软件提交；
  EGL context 由 graphics/opengl 管理。销毁前同步撤销，RedrawNeeded 等待 Go 绘制
  完成，期间 present 不等待原生 UI。
- iOS：Go 注册 UIKit 类与协议，管理 Scene、CGImage 软件呈现与 GPU 生命周期许可；
  EAGL context 由 graphics/opengl 管理。异步呈现只保留原生图片引用，不保存 Go
  图像存储。后台关闭包含 scratch 在内的 GPU 许可。
- 共享 Source 在 Go 内传递观察，绘制租约同步撤销，不再有 host.h/32 字节 C 事实包。
- 原生启动参考、UIKit SDK 协议元数据与 Java helper 内嵌 Go 生成器，输出仓库外。
  协议元数据不含平台行为，不进入 runtime 基座；普通平台 Go 代码仍从源码编译。

tests/window/platform/mobile_surface 使用正式宿主和 Software Painter；测试包装
仅提供应用私有状态路径。Run 前创建 Window/Painter/图片，周期重绘并记录
nonce、尺寸、generation、丢失与恢复、线程与呈现计数。验收旋转、前后台、
Android Activity 重建，检查窗口未 Destroy、图片未重建、状态未清零，以及
撤销后的提交被拒绝。iOS Scene disconnect/reconnect 如未触发真实系统场景，
必须标为逻辑测试覆盖、原生运行待验，不以后台返回替代断开验收。

### 第二切片实现与验收（收敛前的历史记录）

2026-11-04，Go 1.25.8，使用第三阶段 purego/cgo 修复分支。正式宿主和验收
程序分别位于 platform/mobile/native 与 tests/window/platform/mobile_surface；
最小 runtime/cgo 入口归档没有增加 UIKit、Activity 或绘制策略。Windows/Linux
消费端复用预编译宿主对象和链接组件，仍从源码编译普通 Go 平台代码。

桌面 Window/Popup 只增加 SurfaceState 查询，沿用既有销毁与绘制行为，没有
新增桌面 SurfaceEvent/VisibilityEvent 发送。公共配置读取另修正
GOUI_PLAT_SCALE=Inf 被当作有效比例的问题；测试覆盖非法值及有效的 1.5。

| 验证 | 环境与结果 |
| --- | --- |
| Linux 消费 | amd64 / Go 1.25.8；Android、iOS Simulator、iOS Device 三目标编译/链接通过；PATH 无原生编译器，日志确认复用入口归档，未执行 cgo 源码生成 |
| Windows 消费 | Windows 10 amd64 / Go 1.25.8；同三目标通过，编译器调用为零，源码注册文件未落入应用目录；未改变 Defender 或其他系统安全配置 |
| Android 原生 | macOS 15.7.7 ARM64 上 Android 11 ARM64 ATD / NativeActivity / Software 1x；十次横竖屏、三次 Home/返回、Back 结束 Activity 后同 PID 重建通过；nonce 与缓存图片保留，destroyed=0、threadFailures=0，最终 submitted=84、ready=15、lost=14 |
| iOS 原生 | 同一 Mac 上 iOS 26.3.1 ARM64 Simulator / UIKit / Software 3x；XCTest 八次旋转与三次 Home/返回通过，尺寸观测包括 393×852 和 852×393 DIP；destroyed=0、threadFailures=0、submitted=255、ready=9、lost=8。一次失效呈现被许可拒绝，随后恢复 |
| 画面 | 两端最终稳定截图的纯色内部与不透明 sRGB 预期一致；绿色缓存图为 Android 32×32、iOS 96×96 物理像素，红色标记为 10×10/30×30。跳过系统栏与圆角抗锯齿；横屏几何由原生尺寸事件验收，截图只验证稳定画面的颜色和图片物理像素大小 |
| 生命周期逻辑 | 旧排队 Paint 被新 generation 替换后继续重绘、已撤销 attach 不复活、present 与撤销互斥、GUI 尚未处理 detach 时拒绝原生访问、回调销毁不再投递后续事件，独立断言通过 |
| 包内与编译 | mobile/common/events 定向 race、Software race 和相关 vet 通过；Windows amd64 win32、macOS arm64 cocoa 测试目标交叉编译通过，不等同于桌面交互复验 |
| 全仓 | 已跟踪源码及新增文件的隔离副本、显式 replace 到 FFI 修复分支：go test -count=1 ./... 为 3505 通过、10 既有跳过、0 失败。已核对全部 Go 源码与交付源码一致，未删除或放宽断言 |

直接在原工作区执行 go test ./... 仍因四个未跟踪旧 example 目录的过期接口
编译失败；本轮未修改这些文件，未把该命令记为通过。本轮未重现第一切片记录的
demo 启动段错误，也不因此宣告旧问题已修复。现有测试产生的图片保留在原有
位置，新增验收产物均在仓库外。两端原生测试运行的是 Linux 消费产物；Windows
产物本轮只验证构建，未将其视为独立的原生运行验收。

最终全仓复验还发生过一次 xerrors 测试进程在执行测试前段错误：该轮 3496
通过、10 跳过、1 包失败。该包及其依赖链源码不包含本轮改动；用未修改的
2c2950b 与当前源码编译的独立测试程序各执行五十次均未复现。根因仍未定位，
不能宣告已修复，也未认定它与第一切片的 demo 启动失败同源。失败 JSON、
core 检查记录和对照结果保留在本轮临时目录；最终重新执行结果单独列在表中。

iOS Scene disconnect/reconnect 没有触发实际系统场景，仍为原生待验；逻辑
许可测试不替代它。iOS Device 仅链接，真机、Android 16 KiB 内核、触摸/键盘/
IME、GPU 后端、完整 Platform 服务和 GUI/UI 接入均未完成。当前一个进程只
接入一个逻辑内容窗口；Activity 结束不销毁它，显式退出与系统进程终止的契约
留在后续切片。后台被系统挂起与进程重启不承诺保留 Go 内存。

本轮创建的专用 iOS 模拟器已删除，Android 模拟器与私有 ADB 已停止，没有
修改用户已有模拟器或桌面。可复核日志、状态历史、截图、消费哈希与 receipt.json
保留在本机 /tmp/goui-mobile-surface-stage2-r_fhc5l4，Mac 同名目录；Windows
使用同名用户临时目录。SDK、组件、二进制和签名材料未加入仓库。

## 第三切片：Android 工厂与 GUI 内容窗口（2026-11-04）

目标是让普通 `gui.NewApplication/NewWindow/Run` 复用现有控件树、布局、
排版和绘制，而不是再维护应用侧 mobile 窗口 API。当前先接 Android，
iOS CoreText/工厂在下一切片；触摸、IME 和 GLES 各自独立接入。

### 设计与边界

- DefaultName 在 Android 为 android；Linux 工厂显式排除 Android，避免 Go 的
  android 同时匹配 linux 而拉入 X11/GLX/Pango。移动启动由 host 创建已锁定并
  attach JVM 的 GOUI 线程，不在包 init 再锁住 runtime 初始化线程。
- Android Platform 提供单个 EventLoop、系统内容 Window、androidtext 与
  Software。Integrated 是可选偏好，平台返回不支持后 GUI 沿用已有 Native
  降级；不冒充 DesktopWindow，不提供桌面位置、状态或窗口控制按钮。
- 剪贴板、Settings、Popup、IME、Cursor、DnD、文件服务与 URL 尚未接入，明确
  返回 ErrUnsupported。已有 GUI Settings/可选服务规则处理默认值，不在平台
  伪造设置。Window 标题修改仍不支持，因此本轮仅验证命令式 GUI，不声称 UI
  Window 的 Title 已兼容。
- rootBase 每次开始、提交帧及关键回调边界查询实时 SurfaceState。原生目标可
  先于排队通知撤销，不能仅依靠最近的 SurfaceEvent。丢失时保留 paintDirty，
  停止待提交帧与帧定时器；恢复后重新布局/绘制，保留 Painter 和控件树。
- Window 消费 SurfaceEvent，使旧帧 epoch 失效；VisibilityEvent 暂停/恢复帧。
  后台 Timer 仍按普通计时器契约工作，但不驱动画面 Frame。Surface 丢失/Activity
  重建都不发 GUI Destroy；显式 Quit、Back 与系统结束策略仍为后续工作。
- 原型验证程序仍直接使用 mobile.Window/EventLoop。本轮暂不迁移它们，避免
  工厂接入同时破坏既有 iOS 基线。两端工厂齐备后再将共享实现移入 internal，
  各平台 native 宿主归位；不保留第二套长期公开窗口接口。

参考与证据：NDK [ANativeActivityCallbacks](https://developer.android.com/ndk/reference/struct/a-native-activity-callbacks)
规定回调在应用主线程，onNativeWindowDestroyed 返回前不能再访问旧窗口。
Flutter [AndroidShellHolder](https://github.com/flutter-team-archive/engine/blob/main/shell/platform/android/android_shell_holder.cc)
也区分平台线程与引擎工作任务（参考归档源码，读取日期 2026-11-04）；本项目只
采用线程职责分离，继续使用 Go 现有 GUI 事件队列，不照搬引擎线程/渲染架构。

### 验证计划

包内测试覆盖表面未就绪、隐藏、同步撤销先于通知、目标替换和控件树保留。
`tests/window/platform/android_gui` 维护者脚本预制作宿主/Dex，消费 PATH 无原生
编译器；APK 在专用 ARM64 模拟器验证工厂、排版、布局、普通输入派发、旋转与
后台恢复。通过 JSON 明确比较 PID、OS 线程、控件地址、输入内容与帧数；真实
截图按语义色块边界采样。测试流程见该程序 README，结果在验收后填写。

### 本轮验收结果

2026-11-04，Go 1.25.8，Linux x64 维护者制作/消费，远端 macOS 15.7.7 的
Android 11/API 30 ARM64 ATD、Software 1x、480×800，CheckJNI 开启：

- 普通 gui 工厂启动、Run 前逻辑窗口与无 drawable、Integrated 偏好降级、
  GUI/系统线程分离、普通 Timer/Frame/AfterPaint、按钮正常事件派发通过。
- 多语言 Label、Button、程序设置的 TextInput 正常显示，布局/语义快照一致。
  旋转前后 480×800 → 800×480 → 480×800；同 PID 1479、GOUI 线程 1509、
  同控件对象，点击保持一次、输入文本保持不变；surface generation 为 1/3/5/7。
- Home 后两次采样 ticks 15→20，frames 保持 19；恢复 frames 增长至 21。
  回调线程断言全部通过，JNI DETECTED ERROR 与 Fatal signal 为零。
- 实际截图色块中心 (240,236) 为 sRGB RGB(32,144,96)，和独立预期完全相等
  （允许每通道误差 1），文字画面已检查，不使用跨平台字体金图。
- 首轮截图暴露独立 Label 高度为零：GUI 把 layout.Inf=1e9 直接作为 Typography
  尺寸，Android 的有限坐标验证拒绝后 Label 返回空测量。修正 GUI 的已有测量
  转换，将无界轴转成 labelMeasureExtent；新增水平/垂直/双向断言，APK 中对
  三个独立 Label 的正尺寸作断言后重跑通过。保留首轮截图和状态，未放宽原生
  范围验证，也未通过固定控件高度绕过问题。
- 隔离已跟踪源码及本轮新增文件，GOWORK=off `go test ./... -json`：3515
  通过、10 既有跳过、0 失败；相关 gui/mobile race 与 gui/mobile/platform vet
  通过。跳过为 9 个手工平台窗口用例和 macOS 原生签名测试，未计入通过。
  原工作区未跟踪旧 example 不纳入隔离副本，不改变此前记录的原目录失败。
- Windows x64、macOS ARM64 的 GUI 测试目标交叉编译通过；仅编译结果。
  本机独立 Xvnc/xfwm4、Software 1x 的既有 animation -auto 通过动画终值、
  帧空闲、离屏快照和 Popover 更新/隐藏断言，未占用用户桌面。

最终 APK SHA-256：
`dd1d1840e0690fa931863c2df6d239141ecc6c786469b4ff1371e4fa32374065`。
消费使用固定入口归档，消费 PATH 不提供 C 编译器/javac，未执行 C 编译；原生
宿主字节与前轮相同。最终 APK 对应的 14 个相关 Go/C/Java/Python 源文件与工作
区逐字节一致。材料位于本机及 Mac 的 `/tmp/goui-android-gui-x02s60ng`；专用
Android 模拟器、私有 ADB、独立 X11 会话已结束，旋转设置已恢复。

未执行：API 29/31+、Android 真机/16 KiB、iOS 本切片 GUI 和桌面远端运行。
未实现：触摸、IME、GLES、标题/系统服务、完整 UI 接入，以及原型目录迁移。
WrapChar 继续单独排期，不作为基础 GUI 接通的前置；按用户要求不为原生
uintptr 静态诊断增加无关封装或阻塞本轮进度。


## 正式工厂迁移补充（2026-11-05）

上述阶段记录中的 platform/mobile.Window/EventLoop 现已迁入
platform/internal/mobile。两端原生宿主分别归于 platform/android/native 与
platform/darwin/ios/native；固定启动支持包和 goui_run ABI 保持不变。
Android/iOS 通过 platform.NewPlatform 进入，原有 surface/text 验收例程相应
改用正式工厂，不再调用已移除的公开 mobile.Window/EventLoop。
最小 mobile_start 例程的专用宿主不实现正式窗口协议，因此仅用测试 channel
转交验证消息；正式队列的覆盖仍由包内测试和 surface/paint 例程保留。
当前入口、默认后端、前台 GPU 许可及最新验收见
[DesignMobilePlatform 第 11 节](DesignMobilePlatform.md#11-正式平台工厂与共享实现收敛2026-11-05)。
