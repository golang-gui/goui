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
