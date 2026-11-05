# 系统安全区域、局部避让与条件布局

状态：2026-11-05 设计记录，尚未实现。本文记录后续方向，示意接口不是已存在或
已冻结的公共 API。当前仍优先完成 Android/iOS 平台的基本可用性，不因本文推进
GUI/UI 改造，也不将完整安全区、屏幕轮廓或条件布局系统列为当前切片的前置。

关联：[移动平台](DesignMobilePlatform.md)、[生命周期](DesignMobileLifecycle.md)、
[Window Chrome](DesignWindowChrome.md)、[触摸输入](DesignTouchInput.md)、
[布局](DesignLayout.md)、[测试约定](Testing.md)。

## 1. 已确认的设计定位

GOUI 不承诺同一份 UI 自动在桌面、手机和平板上获得合适的布局。应用可以按
当前窗口尺寸、运行平台和公开的屏幕特征进行条件判断，也可以分别编写不同
平台的视图，再复用业务、状态和适合共享的组件。

框架提供可信的几何事实、变化通知及常用避让工具；应用决定哪些内容需要避让、
哪些背景需要铺满，以及何时改变布局结构。不通过设备型号猜测布局，不引入
CSS 媒体查询语言或覆盖所有平台的自动布局策略。

原生系统已经为应用内容避让的部分，不在 GUI 再补一遍。判断依据是当前原生
内容与绘制区域，不是“处于竖屏所以一定安全”或“处于横屏所以必须补边”。

## 2. 需要分别表达的几类几何

| 信息 | 含义 | 处理方式 |
| --- | --- | --- |
| 系统安全 Insets | 当前内容区域受到系统栏、屏幕缺口等影响的四边范围 | 平台观察，GUI 可消费为布局边距 |
| 缺口区域 | 刘海、打孔等可公开取得的局部边界 | 独立几何，供局部布局和应用条件判断 |
| 屏幕圆角 | 平台可公开取得的角位置、中心和半径 | 独立能力，按控件位置决定避让 |
| 窗口控制按钮 | Integrated 的原生或 GUI 控制按钮占位 | 保留 WindowChrome 的局部协作 |
| 键盘与系统手势区域 | 输入遮挡或系统优先接收手势的范围 | 分别随 IME、输入设计，不合并到上述 Insets |

Insets 是常用情况的便捷表达，不能承诺描述完整屏幕安全形状。多个缺口或
局部圆角不应直接转换成整个窗口的一条空白边。背景通常可以继续绘制到边缘；
靠近下角的操作按钮可能需要避让，而居中的正文无需因此整体缩窄。

物理屏幕或系统已经限制最终可见轮廓时，应用不需要为此重画一个窗口圆角。
这与 Linux GUI Integrated 自绘装饰使用 ClearPath 是不同的职责。

## 3. 原生契约与证据

以下为官方文档/源码参考，查阅于 2026-11-04 至 2026-11-05；不是 GOUI 对这些
场景已经完成原生验收的声明。

### 3.1 系统栏与缺口

传统布局中，通常是状态栏而非应用标题栏覆盖顶部缺口区域。系统已经约束内容
时，应用可能不受影响；旋转后侧边缺口则更容易影响布局。但 edge-to-edge 会
改变这一前提：Android 15 上面向 SDK 35 的普通窗口会延伸到系统栏后的区域。
因此竖屏也必须服从实际观察，不将旋转作为唯一触发条件。

- [Android edge-to-edge](https://developer.android.com/develop/ui/views/layout/edge-to-edge)
- [Android display cutout](https://developer.android.com/develop/ui/views/layout/display-cutout)
- [UIView.safeAreaInsets](https://developer.apple.com/documentation/uikit/uiview/safeareainsets?changes=_5)

UIKit 的 Insets 相对具体 UIView；未安装到有效视图层级或尚未可见时可能为零。
平台需要区分观察尚未就绪与有效的零边距，不能用一次早期零值替代后续通知。

### 3.2 屏幕圆角

Android 12/API 31 起提供 RoundedCorner 与 WindowInsets.getRoundedCorner，
可以取得相对应用窗口的圆角中心和半径。较早版本或其他平台不必具备同等能力。
没有公开数据时不得按设备型号填入一个看似合理的半径。

- [Android rounded corners](https://developer.android.com/develop/ui/views/layout/insets/rounded-corners)

iOS 首版使用公开 safe area；精确圆角信息仅在确认存在合适的公开契约后接入，
不使用私有屏幕圆角字段。未知圆角不等于已确认的零半径，硬件轮廓也不等于
样式系统里由应用指定的圆角值。

### 3.3 参考框架的取舍

Qt 自 6.9 提供窗口安全边距与变化信号，Qt Quick 还提供相对 Item 的安全区域。
Flutter SafeArea 用容器消费选定边沿，并减少后代接收的已消费边距。

- [Qt 6.9 QWindow](https://doc.qt.io/archives/qt-6.9/qwindow.html#safeAreaMargins)
- [Qt 6.9 SafeArea](https://doc.qt.io/archives/qt-6.9/qml-qtquick-safearea.html)
- [Flutter SafeArea](https://api.flutter.dev/flutter/widgets/SafeArea-class.html)
- [Flutter safe_area.dart](https://github.com/flutter/flutter/blob/stable/packages/flutter/lib/src/widgets/safe_area.dart)

采用事实与布局工具分离、背景可铺满、必要时防止重复消费的思路。完整的相对
几何传播、嵌套消费和类似 MediaQuery 的环境系统不是首轮必选实现；需要结合
GOUI 的 Measure/Arrange 与实际应用证明其必要性，不照搬参考框架的机制。

## 4. platform：报告当前内容几何

基础能力建议从窗口安全 Insets 的查询和事件开始，复用 geometry.Insets。
以下仅为接口候选，实施前核对最终归属和命名：

```go
// Candidate addition to common.Window.
SafeAreaInsets() (geometry.Insets, error)

// Candidate native observation event.
type SafeAreaEvent struct {
    Insets geometry.Insets
}
```

契约要求：

- 使用当前窗口/内容区域局部 DIP，与 SizeEvent 和输入事件的缩放一致。iOS
  原生 point、Android pixel 和应用 scale override 沿既有边界换算。
- 保留完整绘制目标、尺寸和坐标。查询本身不缩小 viewport，不移动窗口，也不
  改变布局、绘图、点击或手势策略。
- 有效零值表示当前没有相应侵入；暂未取得观察返回 ErrUnavailable，未提供
  能力返回 ErrUnsupported，系统查询失败按错误规则返回。
- 处理尺寸不变而 Insets 改变的情况。初始关联、旋转、系统栏变化和宿主重建
  后均需重新观察；拒绝旧 Activity/Scene 或旧目标的迟到消息。
- 同一次原生布局的尺寸、缩放和 Insets 使用一致快照，先更新可查询状态，再
  沿正常事件循环上报。不能让一次原生布局混用旧比例和新边距。
- 原生 UI 线程读取要求主线程的属性；GOUI 线程使用复制后的观察，不反向同步
  等待 UI 线程。私有宿主 ABI 随平台组件校验，不扩大 runtime/cgo 基座。

Android 要核对 WindowInsets 与 ANativeWindow/实际内容区域的关系。系统已经
缩小内容或留下空白时，不能把 decor 的整份 Insets 再套在内容上。API 29 的兼容
分支不能直接将包含 IME 的 systemWindowInsetBottom 当作永久安全边距，也不能
为使用新 API 默默提高最低版本。

iOS 从实际承载内容的 UIView 获取 safeAreaInsets，监听对应变化；不调整
CAEAGLLayer 或 Software 表面的尺寸来模拟内容避让。

macOS 原生标题栏/按钮继续由 Chrome 处理，屏幕刘海的影响需投影到实际客户区。
Windows/X11 按真实能力报告，不用任务栏高度、工作区差值或窗口外框猜测安全
边距。各桌面后端的精确映射在实施前查证，不能仅凭平台名称规定非零值。

更精确的缺口/圆角数据按独立能力扩展，保留位置和可用性，不预先冻结一个混合
尺寸、输入、屏幕、键盘和装饰策略的庞大 WindowLayoutInfo。

## 5. GUI：缓存、局部查询与可选容器

### 5.1 Root 的观察与通知

GUI 统一维护安全 Insets 的当前快照。候选接口如下：

```go
// Candidate additions to gui.Root.
SafeAreaInsets() geometry.Insets
ConnectSafeArea(func(geometry.Insets)) signal.Handle
```

查询使用缓存，不在每个 Widget 内调用原生 API。通知沿用信号风格，拟在连接时
提供一次当前值，之后只在改变时发送。Unsupported 使用已说明的零值兜底；
同一宿主的短暂失败可以保留最近观察，目标更换后重新取得有效数据。

Window 与 Popover 的 Root 数据独立，Popover 不继承 owner Window 的占位。
通知遵守线程亲和、销毁重入及 handle 生命周期；没有变化不重复触发布局。
通常只需 RequestLayout，不重建整个 UI 声明树。

是否立即扩展 Root，还是先通过更小的宿主能力接口接入，留待正式实现时复核；
无论选择哪种方式，都不为此给所有 Widget 增加一组虚拟钩子。

### 5.2 公开已有控制按钮的避让计算

现有 ChromeInfo.ControlsBounds 已通过 WindowChrome.ConnectInfo 公开。
HeaderBar.contentRect 内部的局部矩形计算尚未提供可复用查询。

建议将该算法提取到 Chrome，候选接口：

```go
// Candidate addition to gui.WindowChrome.
ControlsInsets(area geometry.Rectangle) geometry.Insets
```

输入为窗口局部 DIP，输出只针对这块区域。与按钮不相交就返回零；左侧按钮
预留左侧，右侧按钮预留右侧；Padding 由调用者在占位之后叠加。查询不修改
布局、不重新询问原生对象，变化继续使用已有 ConnectInfo。

例如按钮位于 (900, 0, 100, 40)，顶部整行应预留右侧 100 DIP，Y=40 以下正文
和不覆盖按钮的左侧分段均无需预留。不能把按钮的宽度设为整个窗口的右边距。

HeaderBar 复用同一查询，保留原有高度协作与有界布局阶段。自定义控件需要
Root 坐标时，可考虑提供通用 RootBounds(widget) 辅助函数，避免构造 Snapshot
或向 WidgetBase 塞入标题栏特例。该辅助函数的最终形式也未冻结。

Linux Integrated 的缩放边缘继续只参与输入，完整布局和 ClearPath 装饰保持
原有设计，不借安全区重新恢复四边空白。

### 5.3 SafeArea 便利容器

后续可以提供普通 Bin：SetChild、选定边沿和额外 Padding。默认只消费系统安全
Insets；窗口按钮仍由 HeaderBar 或显式 Chrome 查询处理。

容器自身占据完整分配区域、可按样式绘制完整背景，只有子控件向内布局。
额外 Padding 与安全边距相加。Measure、Arrange、baseline、Min/Max 和空内容
处理必须遵守普通布局契约；Tight 约束不足时不反向撑大窗口。

首轮先满足明确组合位置的普通使用。若实现嵌套消费，需要保证兄弟分支互不
影响，已消费边沿不重复补边，重挂载和数据变化正确失效测量缓存。不能根据
任意 Widget 上次 Arrange 的位置反复修正 Measure，从而引入居中布局反馈循环。
复杂传播不作为平台基本可用性的前置。

精确圆角避让是另一类局部计算，必须使用目标控件位置及原生可用数据；不能
简单将最大圆角半径当成全局 padding。当前不承诺任意异形轮廓的通用求解器。

## 6. UI：薄绑定与显式条件布局

SafeArea 的声明绑定可以采用下列形式，均为待实现语法：

```go
ui.SafeArea(content)
ui.SafeArea(content).Edges(gui.SafeAreaLeft | gui.SafeAreaRight | gui.SafeAreaBottom)
ui.SafeArea(content).Padding(12)
```

沿用现有 Bin/UpdateChild、属性协调及连接清理，不为此往 BuildContext 添加
平台类型特例。常规避让由 GUI 请求布局，不要求应用写 OnSafeArea 再全局更新。

应用可按当前窗口可用宽高选择单/双栏，按移动/桌面环境选择底部导航或侧栏，
按平台公开能力为屏幕角部操作安排额外留白。可以使用普通 Go 条件、独立视图
函数或按目标组织源码；不要求同一控件树自动改变为适合所有设备的设计。

窗口化和分屏时优先依据实际窗口尺寸，方向作为补充条件。条件构建所需观察
源在需求明确后接入现有 ui.Build/局部更新；当前没有一个已经实现的通用
MediaQuery API，也不在本轮创造 CSS 断点、设备类别推断或新响应式框架。

常规组合可以在 ScrollView 外使用 SafeArea，不给每个滚动项补边。Dialog 的
遮罩、Window 背景和 Overlay 仍可覆盖完整区域，内部卡片、按钮和正文按需避让。

## 7. 快照、失效与验证要求

新增能力时同步检查 Root 快照的几何信息、SafeArea 的实际子树和边界，以及
属性扩展信息；普通容器不伪造交互动作。自动化仍走原生输入事件路径。

未来逻辑测试至少覆盖：

- 有效零值、Unsupported、暂不可用、宿主更换与旧通知拒绝。
- 边沿选择、额外 Padding、嵌套/兄弟消费、baseline、无界约束和尺寸不足。
- 控制按钮无交集、左右分段、窄窗口及更新失效，正文不出现额外侧边距。
- 连接清理、重新挂载、回调销毁与 UI 属性省略后的正确复位。

未来原生验收分别覆盖系统已避让和 edge-to-edge 的内容范围、竖屏/横屏、
带/不带缺口、可取得/不可取得圆角数据、系统栏变化、缩放与后台恢复。使用
独立原生观察作为参考，不将 Insets 与缺口/圆角查询互相比作唯一预期。

像素采样先等待新尺寸的完成帧，核对截图前后几何和系统方向，再换算 DIP；
不能将早期布局通知当作已经呈现，也不能靠移动采样点或放宽误差通过。
不能只验证单一 Android API、单一方向或模拟器，再宣称全部设备可用。

## 8. 优先级与后续切片

当前先推进移动平台基本可用性：正式 Platform 工厂、绘图组件选择、包结构
收敛、生命周期、排版/绘图与无原生编译器消费链路；基础原生输入按独立切片
验证。已有 GUI 原型保留，本轮不扩展。

本文留待后续明确启用后分阶段实施：

1. 平台安全 Insets 查询/通知，建立实际内容坐标的原生验证。
2. GUI 缓存及公开 Chrome 局部查询，再按需求增加简单 SafeArea 容器。
3. UI 薄绑定及应用条件布局所需的最小观察能力。
4. 有实际需求时扩展缺口/圆角几何、局部计算和必要的嵌套消费。

以上均为待实现方向，不把文档记录、已有绘图验证或 NanoVGo 迁移算作安全区
功能已经交付，也不据此自动开始 GUI/UI 或精确屏幕轮廓实现。
