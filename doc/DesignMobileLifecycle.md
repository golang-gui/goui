# 移动端启动、线程与生命周期

状态：第一切片入口注册、固定 ABI 和事件线程已实现并完成阶段验收；尚未接入 GUI 窗口。
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

`platform/mobile/internal/entry` 提供固定 C ABI `int goui_run(void)`：完成正常
Go 初始化后，仅调度 GOUI 线程并返回。返回 0 表示启动已接受（包括重复请求），
-1 表示入口未注册。包中不包含 Activity、Scene、Widget 或业务导出。
这是独立且最小的原生支持归档，不把整个 platform/mobile 预编译。

main 每个进程执行一次；重入的 goui_run 不重新执行 main。应用函数不公开为
动态符号；后续原生回调使用私有回调地址。Go runtime、注册表和回调必须比
原生宿主长寿；Android c-shared 保留 stock Go 的 NODELETE 行为。

原生入口允许等待 Go 初始化的必要过程，但不能等待用户 main 或 Run。应用
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
3. GUI/UI：延迟 Painter、树保留、恢复布局和状态；连接不重复。
4. 生命周期和退出契约；随后接入排版、输入法、触摸、GPU 和资源恢复。

第一切片不修改 gui/ui，不以测试宿主自带的原生文字/按钮声称 GOUI 控件已运行。
包内测试覆盖注册/重复启动、队列次序、重入、取消和并发 Post/Quit；测试宿主
位于 tests/window/platform/mobile_start，说明实际环境、步骤、预期与退出方法。
运行全仓测试、针对性 race/vet，并分别记录 Android/iOS 原生运行和构建宿主。

## 第一切片实现与验收（2026-11-04）

实现位于 platform/mobile、其 internal/entry、internal/mobilebase 的 entry 文件
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
