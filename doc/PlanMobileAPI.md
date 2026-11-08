# 从移动端视角复核 GOUI 接口

状态：2026-11-08。已确认共享边界与第一阶段平台接口迁移，具体方案见
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
- [ ] 按依赖推进绘制恢复、触摸、IME、几何与系统交互，逐项更新相关设计。
- [ ] 用小型真实应用验证桌面/移动分别组织视图的使用体验，再确定移动支持承诺。
