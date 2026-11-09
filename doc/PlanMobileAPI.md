# 从移动端视角复核 GOUI 接口

状态：2026-11-09。共享边界、平台接口与 GUI/UI Surface 已完成此前切片；
当前优先稳固平台实现，再接原生触摸，验收后继续上层。具体宿主方案见
[呈现宿主与窗口管理](DesignPresentationHosts.md)。本文保留其余主题的复核计划，
不是移动 GUI 已可交付的声明；原生主线程、GUI/UI 启动和 GUI Surface 共享内核
及 ui.Surface 已接入，原生输入仍待实施。

关联：[核心原则](Design.md#cross-platform-scope)、[跨层约定](DesignConventions.md)、
[移动生命周期](DesignMobileLifecycle.md)、[移动平台](DesignMobilePlatform.md)、
[触摸](DesignTouchInput.md)、[安全区域](DesignSafeArea.md)。

## 1. 已确认的边界

- 共享 GUI 内核、可组合组件和合适的界面代码，不承诺一个应用模型或 View 树
  自动适用于桌面、手机和平板。应用可以分别组织不同视图与交互流程。
- 从移动原生任务与生命周期出发重新审视接口，同时检查桌面需求。允许修改甚至
  替换不合适的公共 API，不把已经完成的原型或旧调用方式当作不可改变的前提。
- 应用入口和呈现宿主可以分别适配，Widget、布局、绘制、信号、事件分发及
  声明式协调仍尽量共用。不为统一入口复制一套控件内核，也不让桌面无端迁就移动端。
- GUI 只集中适配具有共同语义的差异。平台事实有选择地公开，不建立万能能力表，
  不让普通控件反复查询 OS，不以空实现、伪造成功或隐藏失败维持形式一致。
- 原生 UI 主线程承载 GUI；平台调度与系统宿主交付已完成模拟器阶段验收。
  GUI/UI 激活与 GUI Surface 已完成后续切片；原生触摸/IME 和完整应用交互仍待接入，
  不能将构建、平台或合成输入通过当作完整 GUI 验收。
- 桌面保持当前产品重点；移动端保持实验性接入，通过真实应用逐步确定支持范围。
  已有构建、FFI 和链接工作独立保留，不因已经投入而扩大 GUI 功能承诺。

## 2. 参考依据及其适用范围

以下资料核对于 2026-11-07。文档承诺、源码行为、GOUI 实测和设计推断分开记录。

- [Flutter 自适应原则](https://docs.flutter.dev/ui/adaptive-responsive/platform-adaptations)：
  区分环境要求的交互行为与需要应用选择的界面设计，不自动替应用决定所有平台外观。
- [Flutter 自适应方法](https://docs.flutter.dev/ui/adaptive-responsive/general)：
  提取共同内容、测量窗口空间，再选择界面分支。可用尺寸与输入设备分别考虑，
  不能把窗口宽度直接当作操作系统或触摸能力。
- [SwiftUI 多平台视图层级](https://developer.apple.com/tutorials/swiftui-concepts/specifying-the-view-hierarchy-of-an-app-using-a-scene)：
  官方示例允许不同平台使用不同视图树和场景。共享声明能力不要求统一完整界面。
- [Flutter 架构](https://docs.flutter.dev/resources/architectural-overview)：
  Android/iOS 从 3.29 起默认合并 UI 与平台线程。共享线程并不消除原生宿主、
  生命周期、输入和绘制目标的差异，也不表示 Flutter 的光栅线程与 UI 线程相同。
- [Flutter 3.29.0 Android 源码](https://github.com/flutter/flutter/blob/3.29.0/engine/src/flutter/shell/platform/android/android_shell_holder.cc)：
  核对合并模式的 UI/platform task runner 与独立 raster/IO runner。
- [Qt 6.8 Android](https://doc.qt.io/qt-6.8/android-how-it-works.html)：
  QtThread 与 Android UI 线程分工。理解其原生接入职责，不因其保留传统 main/exec
  路径而直接要求 GOUI 采用同样的线程模型。
- [Android NativeActivity 回调](https://developer.android.com/ndk/reference/struct/a-native-activity-callbacks)、
  [Apple 应用生命周期](https://developer.apple.com/documentation/uikit/managing-your-app-s-life-cycle)：
  具体线程、原生对象有效期与前后台行为以系统契约为准；API 变更前还要查看相应框架源码。

SwiftUI、Flutter 和 Qt 用于提出问题与验证取舍，不以接口名称相似作为采用理由，
也不从某个框架或产品的商业结果推导 GOUI 的技术路线。

## 3. 复核方法：先理解原生流程，再审视 GOUI

每个主题先完成一份小型记录，按以下顺序推进：

1. **用户任务**：描述一个完整操作，而不是先决定需要增加什么接口。
2. **原生事实**：谁创建对象、哪个线程调用、事件何时到达、对象何时可能失效。
3. **框架参考**：选 Flutter/Qt 中相关公开契约与对应版本源码，说明采用和不采用的部分。
4. **现状复核**：映射到 GOUI 的真实调用链，指出桌面假设、共同机制和未知项。
5. **方案与影响**：比较保持、修改、拆分或仅在后端处理；给出调用示例、桌面影响、
   生命周期、错误、资源和声明式协调变化。允许接口重设计，但不先铺通用抽象。
6. **最小验证**：先用 platform-only 程序验证原生调用与事件顺序，再验证 GUI/UI。
   已验证、失败、跳过和未执行分别记录，不拿构建结果代替交互结果。

每一轮给出有证据的建议与尚待确认的问题，由用户确认具体公共契约后实施。
不把未经确认的命名、回调形态或包结构预先当作结论。启动方向已确定为
NewApplication → ConnectActivate → Run，GUI 使用 NewSurface/Window 区分宿主；
原生交付、Quit 和恢复细节仍按此方法验证，不引入另一个 gui.Run 包级入口。

## 4. 接口复核清单与顺序

以下为检查入口，不表示已经认定这些接口必须修改。

| 顺序 | 主题与当前入口 | 从移动端提出的问题 | 应形成的决策 |
| --- | --- | --- | --- |
| 1 | 启动与线程：platform.NewPlatform、EventLoop、gui.NewApplication/Run、ui.Run、mobileentry | 系统何时允许构建界面？Run 由谁驱动？原生回调能否直接执行 GUI？服务在哪个阶段创建？ | 区分启动 goroutine 与原生 UI 主线程；判断移动与桌面入口是否需要不同契约，不预设统一 Run 或激活信号 |
| 2 | 宿主、窗口与退出：Window、WindowManagement、WindowSource、gui.Surface/Window、Root | Activity/Scene、逻辑窗口、Widget 树是否同寿命？重建、暂时断开与永久结束如何识别？Show/Hide/Close/Quit 在各端到底请求什么？ | 明确所有者、可恢复状态与最终清理；决定哪些能力共享、哪些分开，避免仅隐藏内容 View 来模拟窗口呈现 |
| 3 | 绘制与调度：Surface、SurfaceState、NativeTarget、Painter、Image、Root.ConnectFrame | Surface 丢失时保留什么？上下文丢失后谁重建图像？后台能否执行 GPU？帧需求如何接原生刷新回调？ | 区分绘制目标与上下文有效期，审视租约/锁/资源失效，保持 Painter 的通用性 |
| 4 | 输入：PointerEvent、EventDispatcher、EventController、手势竞争 | 多指、取消、长按、滚动竞争和设备切换如何发生？鼠标和触摸能否同时存在？ | 平台保留输入事实；GUI 解释点击与拖动，不在平台伪造鼠标左键或仅按 OS 推断输入能力 |
| 5 | 文本与键盘：InputMethod、Typography、TextInput、TextView | 原生需要查询和修改哪些文本范围？组合与选区怎样协作？系统键盘、硬件按键与输入动作如何区分？ | 建立编辑会话及范围转换契约，保留 Cluster 交互原则，避免复制整个文档或重复提交字符 |
| 6 | 布局：SizeEvent、Measure/Arrange、安全区域、键盘遮挡 | 原生实际内容区域是什么？旋转、分屏、字体缩放及键盘如何改变它？哪些区域已经被系统避让？ | 提供必要的几何与变化，不自动改写背景或布局；分开尺寸、安全区域、输入遮挡与输入设备 |
| 7 | 呈现与系统能力：Popup/Popover、Overlay、Modal、Back、FileDialog、Clipboard、Settings | 菜单、返回与文件选择在移动端如何完成用户任务？哪些调用异步？宿主失效后结果给谁？ | 在交互边界集中适配；按需要提供移动呈现与原生服务，不让普通控件查询一大批能力 |
| 8 | UI 与应用结构：RootView、ui.Window、状态、局部更新与挂载 | 移动内容是否必须声明一个桌面式 Window？宿主恢复时哪些状态保留？桌面/移动视图如何复用组件？ | 允许不同顶层组织；复用同一协调内核，不复制状态机或为单个控件增加 Root/BuildContext 特例 |

先完成第 1、2 项的联合设计，防止仅修改 Application 后又被 Window 生命周期推翻。
后续项只记录影响点，等具体主题复核时再决定完整签名；不一次性重写全部接口。

## 5. 用真实应用验证共享边界

选择一个带列表、详情和文本编辑的小型应用流程，覆盖：浏览、打开内容、编辑、
返回、键盘显示、旋转/调整大小、后台/恢复及持久化。应用业务与存储独立于 GUI；
可以分别编写桌面分栏和移动页面，复用状态、编辑内容及合适组件。

验证重点不是把桌面 Demo 缩成手机尺寸，也不是追求代码复用比例。应检查：

- 普通控件能否不感知 Activity/Scene，完成正确的点击、滚动、焦点和文本编辑。
- 平台差异是否集中在入口、宿主和交互边界，而非散落在每个控件内。
- 原生线程、对象所有者、异步结果和旧宿主回调是否可解释并可验证。
- 业务和合理的视图状态在恢复时保留；最终关闭不会泄漏旧控件或旧原生资源。
- 桌面仍保持清晰的多窗口、鼠标/键盘及窗口管理语义，没有为了通用形式受损。

测试沿用 [Testing](Testing.md)：不依赖原生宿主的逻辑在包内验证；需要真实事件循环
或窗口的程序放 tests/window，注明 Android/iOS 环境、构建安装、操作、预期、
副作用、复位与退出。模拟器、真机、交叉编译和桌面回归分别记录。

## 6. 本轮记录与后续检查点

- [x] 核心原则明确共享内核与不同应用模型的边界，并同步 AGENTS 和跨层约定。
- [x] 旧移动生命周期设计标明原型约束可复核，未将历史双线程验证改写为主线程验收。
- [x] 建立原生流程优先的复核方法和跨层清单。
- [x] 联合复核宿主与窗口管理，确认 [DesignPresentationHosts](DesignPresentationHosts.md)：
  同对象的可选 WindowManagement、基础 Window owner 的原生 Popup、GUI Surface/Window
  分工与 GTK 风格激活顺序。平台接口和原生 UI 线程切片已验收，系统宿主交付已完成模拟器验收。
- [x] 原生主线程调度完成模拟器验收，详见 DesignPresentationHosts 第 8 节。
- [x] 系统宿主交付完成 Android/iOS Software/GLES 模拟器验收，详见第 9 节。
- [x] GUI/UI ConnectActivate/Shutdown、服务初始化线程和 Run 错误传播完成阶段验收。
- [x] GUI Surface/Window、Widget/IME/Popover 宿主依赖及 Snapshot/dev/diagnostics
  已迁移；桌面八组及 Android/iOS Software/GLES 控件树验证见 DesignPresentationHosts 第 11 节。
- [x] ui.Surface 复用既有协调内核，声明/局部状态/退出清理完成 Android/iOS 两后端
  模拟器验证；桌面回归与首轮断言修正见 DesignPresentationHosts 第 12 节。
- [ ] 先完成第 7 节平台稳固及原生输入验收，再推进 GUI 触摸；IME、几何与系统
  交互继续独立设计，不把所有平台服务都设为本轮前置。
- [ ] 用小型真实应用验证桌面/移动分别组织视图的使用体验，再确定移动支持承诺。

## 7. 平台实现稳固优先（2026-11-09）

### 决策与范围

现有模拟器已验证基座、原生主线程、系统宿主和 GUI/UI Surface，但这些阶段
成果不等于移动平台实现已稳定，也没有覆盖原生触摸。当前暂停扩展 GUI 手势、
控件触摸行为和 UI 声明，先核对平台实现质量、已有契约和原生验证路径。
不回退现有 GUI/UI 代码；它们保留为回归消费者，不能代替 platform-only 验收。

稳固针对已经提供的能力，先纠正有证据的问题，再做职责清晰、可独立验证的整理。
不一次性补齐所有系统服务，不按设计想象重写全部移动实现，也不把任意 GPU
上下文丢失自动恢复、所有设备或商店发布作为本轮前置。不能保证的范围如实记录，
已失效资源则必须拒绝使用，不能以兼容名称或静默成功隐藏。

### 代码复核顺序

| 顺序 | 范围 | 必须核实的事项 |
| --- | --- | --- |
| 1 | Android/iOS 原生绑定与 FFI | 已有库/类/方法的复用、ABI/浮点/结构体、栈内存与回调有效期、JNI/ObjC 所有权；绑定不夹带宿主策略 |
| 2 | 启动与原生线程 | 准备/Run 等待与 UI 执行分开；Post、观察、绘制、服务创建/释放在正确线程；无重复队列或双向同步等待 |
| 3 | 系统宿主与输入源 | WindowSource 单次交付、旧/新 Activity/Scene 区分、适配器释放/重新领取、回调重入和最终清理；不伪装管理窗口 |
| 4 | 绘制目标与资源 | 尺寸/DIP/像素一致，Software/GLES 的目标借用与撤销、前后台许可、图片/排版隔离、失败释放和显式后端选择 |
| 5 | 原生输入准备 | 当前丢弃/未绑定的输入入口、不支持来源的处理、事件寿命、确认/取消与坐标基准；先确认事实再设计 GUI 行为 |

Android/iOS 适配留在各自平台包；platform/internal/mobile 仅复用有共同契约的
Go 调度、宿主和目标状态，不扩张为统一原生协议。OpenGL 上下文归绘制后端，
Typography 归排版后端。基座只负责 Go/native 接入，不承担窗口、输入或 UI 策略。

公共接口的问题检查所有适用目标；只涉及某端实现时不顺带改写另一端或桌面绑定。
原生资源清理必须在有效 UI 阶段完成，不能因为 Run 在启动线程返回，就把原生
Destroy 放进启动 goroutine 的 defer。JNIEnv 按线程获取，原生引用和借用句柄的
有效期分别核对，不能把线程锁定本身当作已在系统主线程的证据。

### 当前输入差距（源码复核，不是运行结果）

- platform/android/window_android.go 的 inputCallback 当前取出队列事件，处理
  preDispatch 后直接 finishEvent(..., 0)，尚未解码/交付触摸。
- platform/darwin/ios/window_ios.go 当前创建内容 UIView，尚未注册触摸阶段回调。
- 现有 activation 例程用 DispatchEvent/CommitText 验证 GUI/UI 和状态保留，
  不覆盖上面两项原生输入缺口。

原生触摸契约与 GTK4 风格手势的既有方案统一维护在
[DesignTouchInput](DesignTouchInput.md)。本轮文档复核不把以上缺口标成已修复。

### 平台验收与进入上层的条件

1. 对源码实际调用链建立所有者、线程、有效期和失败路径记录；确定的问题先有
   回归用例，再以小范围修改解决。结构整理和行为修复分开提交，避免难以复核。
2. 用 platform-only 程序覆盖冷启动、延迟宿主/目标、旋转/尺寸变化、前后台、
   适配器释放和重新绑定、最终退出。Android/iOS 分别使用 Software/GLES，
   核对真实 UI 线程、目标代次和资源释放；不能只看 App 能启动。
3. 绘制验证包括文字/图片、预乘颜色与透明合成、clip/Transform、物理像素和 DIP
   的关系。保留既有 Image 生命周期约束和不同 TextLayout 状态隔离；目标重建
   与真正 context loss 分别记录，未实测者不算通过。
4. 按 DesignTouchInput 的 P1 先接 PointerEvent 原生事实，以 platform-only
   例程观察阶段、ID、时间、位置、取消和宿主有效期。没有 Widget 时仍能独立
   判断原生入口是否正确；不让平台实现点击或滚动识别。
5. 验证记录分清 Android/iOS、模拟器/真机、Software/GLES、系统缩放与应用缩放。
   支持多指注入的环境验证多指；没有该能力就标明缺口，不能拿单指脚本或合成
   GUI 事件替代。保留原生事件到转换事件的对应证据。
6. 必要逻辑测试、race、go test ./...、受影响桌面目标编译和桌面鼠标回归通过。
   既有失败、环境缺失和未执行单列；不删除断言、扩大 skip 或降低阈值换取通过。

以上平台能力稳固、两端原生输入基本链路通过后，再进入 GUI 的按触点路由、
单控制器单序列、点击/滚动竞争与触摸聚焦。若还存在会影响接口/生命周期的基础
问题，先解决并记录，不靠上层补偿继续推进。IME 仍作为独立专题先设计和验证
平台事实，再接 GUI 编辑设施；不提前添加 UI 专属移动输入系统。

### 当前进度

- [x] 更新既有 PointerEvent/GTK4 Gesture 设计和平台优先顺序；本轮仅修改文档。
- [ ] 完成现有平台实现复核，按证据修复并记录未覆盖项。
- [ ] 完成 Android/iOS 原生输入事实与 platform-only 验收。
- [ ] 达到上述条件后继续 GUI 触摸与控件适配；UI 仅按具体需要增加绑定。


## 8. Android 绑定稳固首批（2026-11-09）

本批仅整理 Android 原生绑定与必要调用点，不接触摸、不改变 GUI/UI，也不改
基座/链接器或 iOS 原生实现。已确认的首批问题：EGL 空属性切片被直接取首元素，
会在进入原生前越界；部分 EGL/NativeWindow 方法省略原生参数；JNI 使用裸槽号且
把 void/int 方法统一当 uintptr 返回；libc 包混入 eventfd 唤醒策略。

- NDK/EGL/Bitmap 按现有平台习惯分开 API、类型和常量，保留原生参数/返回语义。
  EGL 的 nil 属性列表显式传 NULL，配置缓冲区/共享 context/版本输出由调用方选择。
- JNI 区分实例/静态方法查找，A-method 名称与真实返回类型对应；函数表槽用具名
  常量标注。Go Unicode/数组转换与异常格式化属于明确的辅助函数，不混在裸调用里。
- 当前 goexlib/cgo v0.1.2 的 ARM64 typed bridge 会保活/固定指针和切片；同步输出
  直接传 Go 指针，不先丢成 uintptr 再逐处手工 Pinner。异步保留仍由资源所有者负责。
- eventfd 属于 Android 主 Looper 唤醒适配，使用 Go syscall；保留进程级归属，
  不随 Activity 销毁关闭。EINTR 重试，非阻塞 EAGAIN 表示已有唤醒；创建失败释放 fd。
- 只机械更新 Android 排版/绘图消费者，不改变其算法。测试覆盖固定 ABI 布局、
  eventfd 的累积/耗尽、EGL 空属性/显式参数及 JNI Unicode、数组、负 int/void、
  异常后恢复；platform-only 原生验收另验证真实 ART/EGL 与 Software/GLES 生命周期。

本批参考原生 API 的公开契约与头文件：

- [Android NDK NativeActivity](https://developer.android.com/ndk/reference/group/native-activity)、
  [NativeWindow](https://developer.android.com/ndk/reference/group/a-native-window)、
  [Looper](https://developer.android.com/ndk/reference/group/looper)。
- [JNI 函数表](https://docs.oracle.com/en/java/javase/25/docs/specs/jni/functions.html)
  与 [Invocation](https://docs.oracle.com/en/java/javase/25/docs/specs/jni/invocation.html)：
  参考其稳定槽位和所有权；代码仅使用 Android 已有 JNI 1.6 部分，不引入新版 JDK API。
- [EGL 1.5 §3.4](https://registry.khronos.org/EGL/specs/eglspec.1.5.pdf)：允许
  NULL 属性列表和 NULL configs 数量查询。原生参数合法与某台驱动实际可用分开记录。

### 验证结果

- **通过：** Go1.25.8，已跟踪源码与本轮新增文件的隔离副本，在独立 Xvnc/Xfwm4
  执行 `CGO_ENABLED=0 go test -json -count=1 ./...`：4550 项通过、11 项原条件
  跳过、0 失败。工作区原有未跟踪测试/图片/程序不混入副本；不迁移或删除它们。
  `CGO_ENABLED=1 go test -race -count=1 ./platform/android/...` 五个包通过，
  相同范围及新增验收生成器的 `go vet` 通过；包括 ABI、真实 eventfd、CLOEXEC、
  并发累积、EAGAIN、耗尽与错误传播。
- **通过：** 正式 `cmd/goui build` 构建 Android ARM64 APK。远端 macOS15.7.7
  ARM64 上运行私有 API30 AOSP ATD、480×800、160dpi（1x）、SwiftShader ES3。
  [android_bindings](../tests/window/platform/android_bindings/README.md) 的独立
  JNI/EGL 检查通过；global 引用跨帧与 GC、local 引用提升、UTF-16/NUL/非 BMP、
  jint/jfloat/void、异常消费后恢复、EGL 版本输出/共享 context/pbuffer/分别为 NULL
  的参数均有实际断言。四个应用启动均启用 CheckJNI，最终日志 JNI error/warning 均为 0。
- **通过：** 既有 platform-only mobile_dispatch 的 Software/GLES 两组，每组四次
  旋转、两次后台停绘/恢复、GC、最终 UI 释放和 Run 返回边界均通过。宿主只交付一次，
  两后端目标均到第 13 代，启动线程与 UI 线程不同；截图检查多语言文字、绿色块和
  蓝色图片，不将其视为全面像素或透明合成验收。
- **未通过：** 同一模拟器上 `eglChooseConfig(display, nil, nil)` 的全 NULL 组合
  会挂起。首轮等待结果超时；再测阶段文件停在 egl-null-query，原生栈为
  eglChooseConfig → rcChooseConfig_enc → qemu_pipe_read。分别传 NULL 可通过，
  不能推出组合可通过。保留默认 all 与单独 egl-null-query 的断言，不改写生产参数，
  也不把选跑 JNI/EGL 的成功记录成全部通过。故障位于进入原生后的调用链；尚未用
  其他 Android 版本或真机确认原因和适用范围。
- **编译通过：** Windows amd64/macOS arm64 的 platform、gui、ui、widgets、demo
  及新验收生成器；没有重新执行两端桌面操作。初次直接 `go build ./...` 因既有
  mobile_start/mobile_surface 在桌面目标无 main 而失败，随后按适用包编译，不改其断言。
- **构建路径限制：** 裸 `CGO_ENABLED=0 GOOS=android GOARCH=arm64 go build
  ./platform/android/... ./platform/internal/androidruntime ./platform/typography/androidtext
  ./platform/graphics/opengl` 在 purego 的 runtime/cgo.Dlopen/Dlsym/Dlclose 处失败。
  该命令绕开预置基座，不是本项目的 Android 正式构建路径；正式 goui build 已通过，
  没有为此改动 purego、goexlib/cgo、基座或依赖版本。
- **清理：** 已停止并卸载本次两个测试应用、关闭本次只读模拟器和专用 adb 服务，
  保留 SSH 和原有 SDK/AVD。结果、首轮失败日志、截图及清理记录保留在两端临时目录
  `/tmp/goui-android-bindings-pz55zesc`，不提交二进制和日志到仓库。
- **未执行：** 真机、其他 Android 版本、iOS 新一轮原生运行、原生触摸/IME、
  真正 GPU context loss、透明合成/clip/Transform 全组合。本批不标记整个 P0 平台
  稳固完成；剩余所有权/失败路径与输入事实仍按第 7 节继续核实。


## 9. iOS UIKit 绑定与实现整理（2026-11-09）

本批按 AppKit 既有 `ImplementNSView` / `ImplementNSWindowDelegate` 的模式整理
UIKit，不引入新宿主管理框架，不扩展触摸、IME、多 Scene 或公共 Window 能力。

### 职责与归属

- `frameworks/uikit/uikit_ios.go` 按原生类型组织初始化、ClassId、selector、实例
  方法与 Override；`ImplementUIApplicationDelegate`、`ImplementUIWindowSceneDelegate`
  和 `ImplementUIViewController` 仅负责类/协议注册、ABI 及类型化参数转发。
  nil override 不安装可选方法，不隐式补策略、不切换线程、不隐式调用 super。
  UIApplicationMain 保持原生 C 函数形式；Foundation 的 NSDictionary 仅包装借用引用。
- GOUI 类名与生命周期逻辑留在 `ios`。`application_ios.go` 拥有进程级入口、
  UIApplication delegate、dispatch 回调、启动标记和 Go 事件循环引用；
  `window_ios.go` 拥有 Scene、UIWindow/controller/view/EAGL layer、逻辑 Window
  绑定、几何及目标代次；`present_ios.go` 拥有 Software 图像转换和最新图像提交。
  Platform 工厂与服务继续在 `platform_ios.go`。
- 当前原生宿主仍为单 Scene 模型，逻辑 Window 可跨 Scene 暂时断开继续存在。
  connect 保存拥有引用的 Scene；disconnect 释放。controller class 保存注册返回值，
  不按字符串重新查找。UIKit SDK 协议元数据仍由既有生成器保留，不移除或重编基座。
- Scene 回调保留并核对来源；controller layout 只更新当前 controller 对应的目标。
  onSystemThread 仅固定进入线程并断言系统主线程，不把 LockOSThread 当作线程切换。
- GPU gate 先拒绝并排空工作，再撤销目标或释放原生对象；Software 提交保留
  generation/handle 校验及 CGImage 释放。断开先移除当前身份，再发送可重入通知，
  并用局部拥有引用释放旧对象，避免旧回调释放新 Scene 的资源。
- Revoke 先更新代次、清理旧图像，再通知消费者；目标重建在回调后核对 Scene、View
  与代次。bind 的目标/可见性通知后重新检查 Window 有效性；交付回调后检查 Platform
  与 Window 身份。GUI/UI 与通用 mobile 机制不随本次结构调整重写。

参考：[Apple UIWindowSceneDelegate](https://developer.apple.com/documentation/uikit/uiwindowscenedelegate)
将 Scene delegate 定义为管理一个界面实例，继承 UISceneDelegate；
[viewDidLayoutSubviews](https://developer.apple.com/documentation/uikit/uiviewcontroller/viewdidlayoutsubviews())
报告 controller 根 View 的布局完成。对照 Apple 公开文档与当前源码，不把结构整理
解释为 UIKit 的额外能力承诺。

### 验收与边界

- [x] 正式 cmd/goui 构建 iOS ARM64 Simulator Software/GLES 和未签名 device IPA。
- [x] 新增 ios_bindings：协议/可选方法、参数/BOOL、super/重复注册、无关来源、
  交付中销毁、撤销中销毁与重领、Platform 最终释放，两后端均实际运行通过。
- [x] 既有 mobile_dispatch 两后端：各两次真实前后台、GC、停止与主线程释放。
- [x] 独立 X11 的全仓测试、共享 mobile 逻辑 race 与受影响桌面目标编译。
- [ ] Software 已观察一次竖屏到横屏；四次连续旋转与 GLES 旋转本轮未完成。
- [ ] 真机、系统实际 Scene disconnect/reconnect、真正 context loss 未覆盖，分别记录。

### 本轮验证结果

- 本机 Go1.25.8 / Linux amd64：以 ff53223 的已跟踪源码叠加本轮 iOS 文件及新例程，
  独立 Xvnc/Xfwm4 执行 `CGO_ENABLED=0 go test -json -count=1 ./...`，4551 项通过、
  11 项原条件跳过、0 失败。此前包含当时 Android 待提交代码的副本为 4550 通过，
  两次输入不同，不作性能或通过数量提升的对照。原有未跟踪文件未删除或迁移。
- `CGO_ENABLED=1 go test -race -count=1 ./platform/internal/mobile ./platform/internal/mobileentry`
  通过。Windows amd64/macOS arm64 的 `CGO_ENABLED=0 GOOS=... GOARCH=... go build
  ./platform/... ./gui/... ./ui/... ./widgets/... ./example/demo` 通过；这是编译结果，
  不表示重做了两端桌面操作。纯组织调整的中间版本也经正式 CLI 构建 Simulator App。
- 正式构建命令以 `tests/window/platform/ios_bindings/goui.toml` 和既有
  `mobile_dispatch/goui.toml` 为配置，使用 `go run ./cmd/goui build -target ios/arm64
  -variant simulator -package app -unsigned`；GLES 加 `-ldflags '-X main.painterName=opengl'`。
  device 使用 `-variant device -package ipa -unsigned`，只验证构建，不使用正式签名凭据。
- 远端 macOS15.7.7 ARM64 / iOS26.3 ARM64 Simulator / 3x，专用 iPhone17 Pro：
  ios_bindings 两后端各九项检查、三次交付、两次销毁及 Run 返回后 completed 均通过。
  使用旧 window/platform 实现、仅适配类型化 InitWithWindowScene 后，同一例程明确
  panic `无关 Scene 回调修改了当前目标`；修正后的两后端均通过，不是只检查截图。
- mobile_dispatch Software/GLES 各两次启动系统 Settings 进入后台，再启动本例恢复。
  后台报告 visible/Ready 均 false，间隔 1.3 秒 frames 不增长；恢复后代次与 frames
  增长、nonce 与单次 delivery 不变，GC=true；stop 后 stopped 与 callbackReturned 均 true。
  六个内部点检查绿色块、蓝色图像、背景：不透明 sRGB、预乘等同原值，3x 物理采样，
  原生固定竖屏截图按已观察的横屏坐标换算，RGB 零误差。避开文字、圆角和系统区域。
- Software 实际观察到一次竖屏转横屏和相应尺寸/代次变化。共享远端桌面后来出现
  其他操作，停止鼠标键盘输入，未继续四次旋转或 GLES 旋转；不把命令返回成功或
  未达预期的早期采样算通过。前后台验收使用专用模拟器系统命令，不占用共享桌面输入。
- **原工作区全仓命令失败：** `go test ./...` 被本轮开始前已存在的未跟踪旧文件阻断。
  例如 `example/plat_test`、`example/transparency`、`example/window_chrome`、
  `platform/test`、`tests/window/graphics/image_lifetime`、旧 drag_end/dragdrop 仍调用
  已迁移的 Window.Show/SetTitle；`ui/window_lookup_test.go` 仍使用 windowMount 的
  旧 runtime/id 字段。错误清单与初始未跟踪状态对应；不通过修改这些文件掩盖结果。
- 裸 `GOOS=ios GOARCH=arm64 CGO_ENABLED=0 go build ./platform/darwin/ios` 会遇到
  purego 的 `_PUREGO_REQUIRES_CGO_ON_IOS` 构建限制，不能代替正式移动构建链；本轮
  通过 cmd/goui 的预置基座与移动 Go 驱动完成构建和运行，未因此新增 cgo 要求。
- 本轮专用 Simulator 验收后关闭并删除；保留已有远端 SSH、模拟器及其他应用。


## 10. Android 组织与生命周期整理（2026-11-09）

以现有平台的绑定/适配边界和文件职责收敛实现，采用 Android 自身的生命周期，
保留已验证的系统 UI 主线程、一份 Go 队列及逻辑宿主/绘制目标分离。规范见
[Android AGENTS](../platform/android/AGENTS.md)。

1. 固定规范；将 eventfd 变为基于 x/sys/unix 的独立唤醒对象，保留 Signal/Drain，
   随 native Looper 连接现有 Go 队列，迁移既有 NDK/JNI/EGL 绑定及回归消费者。
2. 按 platform/application/event/input/window/present 组织 Android；SDK 集中类型化
   Java 方法和错误边界，JNIEnv 显式传入；androidruntime 保留进程 VM/类加载器及线程附着。
   明确 Activity、输入队列、绘制目标和逻辑 Window 的寿命，保护实例替换与事件重入。
3. Typography 固定方法改具名字段、保持每个 Layout 可变状态独立；EGL 分步初始化、
   保存原始错误再回滚，核对解绑失败与绘制目标借用的关系。
4. 每批完成逻辑/race、正式 Android 构建和适用原生验收后拆分提交；结构整理与行为
   修复分清范围。最终全仓隔离 X11 回归，桌面受影响包编译；不覆盖其他会话的 iOS 改动。

验收使用既有 Android binding/mobile_dispatch 程序，增加 SDK 异常恢复和回调中
销毁/重新领取宿主用例。全 NULL EGL 查询仍单列失败，不删除断言或添加生产 workaround。
本批已实现并完成以下验收；触摸/IME/UI 仍不在本轮范围。

### 实现与所有权

- 组织遵循 Android AGENTS：platform/application/event/input/window/present 各有
  明确职责；SDK 按 app/view/java 组织内置 Java 类，JNIEnv 显式传入，初始化后共享
  只读 method ID。SDK 的类 global ref 和 androidruntime 的应用 ClassLoader 由进程
  持有；初始化失败回滚已创建的类引用。ClassLoader 初始化成功才转移调用方的 global ref。
- nativeApplication 保留进程调度设施、原生回调和逻辑 Window 绑定；activityState
  借用当前 ANativeActivity 与输入队列。回调核对来源，切换实例先撤销旧目标和输入，
  代次持续递增；向消费者发事件或进入 Java 后重新核对当前 Activity/Window。
  这仍是单内容宿主模型，不承诺多个独立 Activity 窗口同时可交互。
- EventLoop 持有现有 mobile.EventLoop 队列，nativeDispatcher 只接 Looper 与
  eventfd.Signal/Drain；Run 返回后的 Destroy 解除队列引用。唤醒 fd 属于进程，不随
  Activity 或逻辑 Window 暂时断开关闭；没有额外 GUI goroutine 或第二任务队列。
- Typography Context/TextLayout 按职责分文件，固定方法表替换字符串 map。
  method ID 表为独立的只读对象，由同一 Context 创建的 Layout 共享；每个 Layout
  的 TextPaint、SpannableString、Bitmap 和 Go 测量缓存仍独立。Context 释放自身的
  表引用不会清空其他 Layout 正在持有的只读表；不按 Layout 复制整份方法表。
- GLES 核心符号和库查询统一到 libs/gles，OpenGL 后端不再另建系统库绑定。
- EGL context/pbuffer/窗口 surface 分步创建；先保存失败阶段和 EGL 错误，再释放
  本次资源、目标借用与 NativeWindow 引用。默认 EGLDisplay 共享，不随单个失败
  Painter 被 Terminate。资源回滚不破坏已有 Painter。

### 命令、环境与结果

- **通过：** Go1.25.8，独立 Xvnc/Xfwm4 源码副本执行
  `CGO_ENABLED=0 go test -json -count=1 ./...`：4551 通过、11 原条件跳过、0 失败。
  副本只合入本轮 Android 文件和此前已提交源码，不纳入工作区既有未跟踪测试/产物；
  并行 iOS 工作保持其自身提交。未为本轮重写、删除或弱化既有测试。
- **通过：** `CGO_ENABLED=1 go test -race -count=1` 覆盖 internal/eventfd、
  internal/mobile、internal/mobileentry、Android ABI 包和 androidtext 纯 Go 字体逻辑。
  对 eventfd、ABI 包与三个新验收程序运行 go vet 通过。race 结果不是 Android
  原生代码已经通过 race 的证明；JNI/Activity 的线程有效期另由设备断言核实。
- **通过：** 正式 cmd/goui 构建 ARM64 APK。远端 macOS15.7.7 ARM64 的私有
  Android API30 AOSP ATD、480×800、160dpi（1x）、SwiftShader ES3，启用 CheckJNI。
  整理前后逐批执行 mobile_dispatch；最终 Software/GLES 各四次旋转、两次后台
  停绘/恢复、GC、一次宿主交付、最终 UI 释放和 Run 返回边界通过。
- **通过：** android_bindings 的 JNI/EGL 分项；新增 SDK ClassLoader 不存在类的
  异常消费与恢复、SDK File 中文路径往返。没有把选跑分项算成全 NULL 组合通过。
- **通过：** [android_lifecycle](../tests/window/platform/android_lifecycle/README.md)：
  第一次真实 SizeEvent 中释放并重领 Window，第二次事件中销毁平台；交付/尺寸/
  销毁各 2 次，原生 UI 线程和回调返回边界断言通过。它不伪造旧 Activity 回调，
  不能替代未覆盖的多实例交错验收。
- **通过：** [android_rendering](../tests/window/platform/android_rendering/README.md)：
  两个 Layout 的字号、测量与 RGBA 预乘颜色有独立预期；修改第二个 Layout 后，
  第一个测量及位图不变。重复创建同一 NativeWindow 的 EGLSurface 触发 0x3003，
  新错误保留 create EGL window surface 阶段，原 Painter 随后离屏读回
  (32,144,96,255) 像素并完成两帧。最终 native 回归日志无 JNI warning/error。
- **旧实现对照：** 隔离副本仅将 opengl_android.go 换回 387be30 的版本，并在同一
  例程的阶段断言前写出错误作诊断，保留全部断言。旧版返回 bind EGL context: 0x3003，
  阶段断言未通过，未生成成功报告；am start -W 等待超过 40 秒。保留该失败记录，
  随后终止旧应用；最终重新安装新版本，全部断言通过。
- **编译通过：** CGO_ENABLED=0 的 Windows amd64/macOS arm64，范围为 platform、
  gui、ui、widgets、demo 及本轮三个独立验收程序；不把它当作桌面操作验收。
- **首轮修正：** 拆分 Context/TextLayout 时遗留未使用 runtime 导入，Android 构建
  失败，已移除并重新构建。一次复用输出路径被 cmd/goui 正常拒绝，随后使用新的
  仓库外产物路径；不将这些失败记为通过。
- **证据与清理：** 命令日志、APK SHA、JSON、截图、旧实现失败和清理记录位于
  两端 `/tmp/goui-android-organize-oziqbx8f`。仅清理本次四个测试应用、只读模拟器
  和专用 adb 服务，保留共享 SDK/AVD 与 SSH。不提交生成产物或系统日志。

### 保留的缺口

- 第 8 节的全 NULL EGL 查询在该模拟器原生挂起仍未解决；保留默认 all 和独立
  egl-null-query 验收，不加入生产参数改写，也不把此轮分项成功当作它已通过。
- EGL 解绑失败尚未注入/实测。源码还显示共享 OpenGL Painter.End 以 defer 调用
  ClearCurrent 并忽略错误；仅在 Android 修改绑定标志或保持 lease 可能改变共同
  错误契约或阻塞目标撤销。后续单独设计失败传播和资源终止；本次不引入推测性重试。
- 真机、其他 Android 版本、多 Activity 的实际交错、真正 context loss、context/
  pbuffer 内存耗尽未验收。以上不足不由普通旋转、重复 EGLSurface 或 Go race 代替。
- 触摸/IME、Popup、SafeArea 按独立计划推进；整个 P0 平台稳固仍未标记全部完成。


## 11. Android Typography 三批整理（2026-11-09，已完成）

详见 [DesignAndroidTypography](DesignAndroidTypography.md#2026-11-09三批职责整理)。
按 SDK/JNI、字体与批量布局、栅格化与 Dex 三批推进；保留 Java 批量循环，原生
标准 API 归 SDK，GOUI 私有 Java 桥接归 androidtext。不同 Layout 可变状态隔离，
Context.Destroy 不关闭已有 Layout 的字体设施。只更新所需 Android Dex，不重制
运行时基座或改动 GUI/UI；正式消费仍无需 Java/NDK。

- [x] SDK/JNI 与 ART/CheckJNI 原生绑定验收，1495c7a。
- [x] 字体/Layout 与生命周期回归，d508908；中间版本单独构建与实跑。
- [x] Bitmap/Canvas 由 Go Layout 独立拥有，Java 保留批量绘制；Dex/Go ABI 3 同步，
  正式默认组件与故意混用旧 ABI 2 组件的错误恢复通过，d05480a。
- [x] API30 ARM64/SwiftShader/1x、CheckJNI：text 三轮、170项后端、18组/3814项
  交互参考通过；Software/GLES 四次旋转、两次后台恢复及清理通过，EGL回滚通过。
- [x] 独立 X11 已跟踪源码全仓4558项通过/11项既有跳过，相关race及Windows/macOS
  编译通过；两版同环境各三轮耗时对照，无可宣称的显著提速或数量级退化。

原工作区 go test ./... 仍因开始时未跟踪旧接口例程而失败，未删除或迁移；原生
参考的过期VM测试符号、消费工具的format2日志断言已修正并保留覆盖。具体命令、
取证、耗时范围与首轮失败见 DesignAndroidTypography。API29/31+、真机、16KiB、
长期内存、WrapChar及既有全NULL EGL查询缺口仍保留；本三批不扩为完整移动支持。
