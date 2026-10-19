# GOUI 实用化与未来路线

## 当前基线（2026-10-14）

本轮按 pre-release/v0.1 的实现复核设计文档，不修改代码、不重新扩大首版范围。
未来事项分为“待实现（已有确认设计）”“待重新设计／评估”和“需求或测量触发”。
这不是发布承诺或自动实施授权；具体切片仍按设计、确认、实现、验证推进。

当前已具备：三层内核、Basic／Modern、文本编辑与快捷键／Tab、普通 Overlay、
WrapBox、拖放和标签移交、TreeView／TableView／SplitView、CheckButton／Switch／
ProgressBar／DropDown、帧更新与纯 animation、SVG Icon、多语言、透明 Window／
Popup、WindowChrome／BorderBox／SkipTaskbar／初始 Center、文件访问凭据、dev 文本
与 Popover 引用、cmd/goui 构建打包。**实现不等于每种平台／后端／DPI 都已验收**；
各设计中的带日期记录与 [Plan](Plan.md) 分开维护验证范围。

旧 P0～P4 排序和阶段日志保留在文末历史区，不再据其中“下一步”或未勾选项判断
当前缺失能力。上一次审查与本轮分类见 [文档复核](DesignDocumentAudit.md)。

## TextLayout 富文本基础：本轮已实现，应用层按需求推进

字体样式与有效注册、run 级删除线、U+FFFC 纯度量行内占位已按
[Typography 设计](DesignTypography.md) 接入三端；具体实施／验证记录见
[PlanTextLayout](PlanTextLayout.md)，原生离屏验证不等于所有桌面组合验收。
排版占位公开入口为 `TextLayout.SetPlaceholder`／`PlaceholderMetrics`，不创建或
绘制行内内容；实际图片、Widget 及事件仍由上层负责。
后续 RichTextView／Markdown 文档模型、Widget 排布、链接交互和编辑行为留在
上层单独设计。字形外伸边界、不同桌面 DPI／GPU 继续专项验证；不预先扩展
InkBounds、文本布局图片或 Widget 回调接口。Linux arm64 全仓的既有 libc syscall
编译缺口需要独立切片，不能写成排版后端已经原生运行通过。

待复核／评估：Begin／End 与原生段落基方向、自动方向处理的组合，确认 RTL 对齐
是否满足公共语义；仅发现左右枚举映射差异不等于已复现缺陷。其他范围 setter 的
非法输入防护尚未统一，需另行收敛，不能从合法输入回归推定全部错误分支一致。
行距、上／下标、OpenType features、竖排和复杂文字背景仍为需求触发的候选，
不因本轮基础能力完成而自动进入实施；设计与已知差异见 DesignTypography。

## 使用方开发支持

- 应用开发 [Skill](../skills/goui-app/SKILL.md) 与统一 [INSTALL.md](../skills/INSTALL.md)
  随仓库提供；AI 按自身工具的原生机制安装完整技能，GOUI 不维护各 Agent 适配器。
- Skill 不替代应用依赖版本的源码，不自动升级模块；中英文 README 提供安装提示。
  后续根据真实使用反馈改进内容，不因为 Agent 品种增加多套应用开发规则。
- [自定义 Widget／View 指南](CustomWidgetView.md) 覆盖两层的组合、扩展和实现，
  Skill 随附独立参考；纯 Build 修饰器自动传播不属于已实现能力，不借教程扩展框架。
- 技能格式、引用与示例验证记录见 Plan；发布后的远端文件可用性、不同工具的
  实际安装与发现仍需分别验证，不承诺工具通用兼容或本会话自动加载。

## 有确认设计的待实现功能

<a id="future-linux-arm64"></a>

### Linux arm64：基础适配与 QEMU 桌面验收待实施

- 设计：[PlanLinuxArm64](PlanLinuxArm64.md)。沿用 Linux X11／Pango／Cairo／
  Software／OpenGL，保持 no-cgo 和现有上层 API，不另建平台后端。
- 已确认全仓编译的首个阻塞是 libc `SYS_PIPE`／`SYS_POLL`；采用已有 x/sys 的
  Pipe2／Poll，随后验证原生 ABI、回调、排版和绘图，不以局部交叉编译代替运行。
- 本轮主要环境为本机 QEMU TCG 完整系统模拟的 Ubuntu 24.04 arm64／X11／VNC。
  实体 ARM 设备不是前置；实际 Painter／GL renderer、缩放及模拟环境结果单独记录。
- GLX／桌面 GLSL 与 CPU 架构支持分开确认，不将 Software 回落算成 OpenGL 通过。
  EGL／GLES、Wayland、musl 和实体 GPU／真实性能留在范围之外或后续专项验证。
- 当前只完成设计；环境、代码、窗口验收、CI 和新 Demo 产物均未据此完成或发布。

<a id="future-submenu"></a>

### 子菜单与跨层键盘导航：待实现

- 基线：MenuModel／PopoverMenu、原生独立表面、工作区退避和中性菜单样式已实现。
- 缺口：递归 Submenu 模型、父菜单拥有子宿主、根菜单唯一模态登记、多级展开／
  收起、跨层活动键盘协调、左右／Enter／Esc、跨表面移动保护及递归 UI 声明。
- 设计：[DesignMenuSubmenu](DesignMenuSubmenu.md)。实施前复核 Attributes 快照、
  现有定位算法所在文件、rootBase 手势／输入消费和 Window.Popovers 枚举。
- 不借此增加模态栈、全局菜单管理器、菜单栏、富控件菜单或平台公共接口。
  先模型／生命周期，再定位／输入，最后 UI／外观和三平台验收。

<a id="future-popover-ime"></a>

### 原生 Popover IME：待实现，实施方案需复核

- 基线：普通窗口三平台 IME、TextInput／TextView、模态按键转发和 CommitText 已实现。
  原生模态打开时暂停 owner IME；CommitText 只提交已确认文本，不模拟预编辑。
- 缺口：Popover 编辑控件的原生预编辑、候选定位、提交与取消、隐藏／销毁后安全恢复。
- [DesignIME §5.1](DesignIME.md#51-跨宿主原生-popover-ime未实现) 记录边界。
  旧“p.position 加偏移”配方不沿用：Position 是锚点局部请求，实际退避位置、阴影
  占位、内容原点、DIP／原生尺度须分别核对。先确定原生上下文是否借用／如何路由，
  不提前修改公开 IMContext 签名或承诺固定工作量。
- 保持普通窗口布局／编辑隔离；先逻辑回归，再真实三平台 IME 与候选窗验证。

<a id="future-clipboard"></a>

### Clipboard 错误反馈：待实现；INCR：需求触发

- 基线：三平台纯文本读写、编辑快捷键及菜单已经接入；FileDialog 的访问资源与
  错误结果迁移也已完成，不能继续与 Clipboard 合并写成“两项都未迁移”。
- 缺口：GUI／UI 写入失败和读取失败的明确操作结果。按
  [错误反馈规则](DesignConventions.md#错误反馈规则) 单独设计兼容范围，不假定底层
  bool 结果已能区分“无文本”和系统失败，不以日志／空字符串伪装成功。
- X11 Clipboard 当前不支持 INCR；DnD 的 INCR 不是 Clipboard 实现。
  大数据分块、PRIMARY、富格式与变更通知按具体需求评估；不承诺任意文本长度。
  现行 v1 见 [Clipboard](Clipboard.md)。

## 需要重新设计或评估的方向

<a id="future-ui-popover"></a>

### 通用附着浮层与 Tooltip：待重新设计

已有 MenuButton、OnContextMenu、DropDown 和普通 Overlay，不再建设一套窗口内
popup-manager。若应用确有通用附着 Popover／Tooltip 声明需要，应确定状态驱动显隐、
anchor 归属、独立子树协调、dismiss 回写、原生资源释放及 Snapshot 枚举。
复用 ui.Bin／Container、局部 ID 与 Root 生命周期，不引入第二套 Key、
BuildContext.UpdateOverlays 或 WidgetBase 具体浮层特例。旧草案不算已确认 API；
相关边界见 [DesignUI](DesignUI.md#4-附着浮层声明待重新设计未实现)。

<a id="future-scroll"></a>

### 共享滚动模型与虚拟化扩展：待评估

当前 ScrollView 拥有偏移，Scrollable 提供范围／可见布局，ScrollIntoViewRequester
请求目标可见；变高行、Refresh／Reveal、拖放自动滚动和单轴有限交叉约束已实现。
若增加同步滚动、更复杂视口锚定或位置控制，优先评估类似 GTK4 Adjustment 的独立
共享模型；不是已经确定要替换现有接口。须区分目标可见性、主动位置控制、范围
更新与锚点修正，说明所有权、双向通知／重入、布局时序及嵌套边界。
固定行高选项、增量模型通知／diff 与 delegate 简化也按测量／真实需求决定。
参考和取舍见 [DesignScroll](DesignScroll.md#后续扩展优先评估独立滚动模型)。

<a id="future-clipping"></a>

### 通用抗锯齿裁剪或局部擦除：待评估，择一按需求推进

Linux GUI Integrated 已使用圆角背景、安全内容布局及 ClipRect，当前无需窗口
专用 Painter 遮罩。只有任意内容铺满圆角容器／明确局部擦除等需求出现时，再从
[裁剪讨论](DesignGraphicsClipping.md) 选择通用裁剪或 ClearPath；两者语义不同，
不同时预建，不承诺已确定的方法名。须共同验证 Software／OpenGL／Direct2D 的
预乘合成、覆盖率、Transform／clip／HiDPI、状态恢复与资源成本。

<a id="future-layout"></a>

### 内容最小尺寸、视觉度量与二维布局：待评估

Constraint、baseline、Linear 默认值、测量缓存、WrapBox 与单轴 ScrollView 均已实现。
当前自动窗口最小尺寸取根自然测量，WrapBox 的无界单行宽度不等于可换行最小宽度；
暂由应用显式 Window.MinSize 控制，后续需分别设计“期望尺寸”和“可用最小尺寸”。
TextInput 的建议宽度仍为 160 DIP；若按文字容量／字体度量调整，保留 min/max 和父
硬约束优先，不把 Padding 擅自搬入样式系统。Grid／FormLayout 按跨行列对齐需求评估，
不把 LinearBox 重新定义成 FlexBox。见 [DesignLayout](DesignLayout.md)／[WrapBox](DesignWrapBox.md)。

<a id="future-style"></a>

### 样式动画与进一步主题扩展：需求触发

Basic 与 Modern、明确 Part／State、FocusVisible、Settings 自动更新及常用预设已实现。
现有 animation 是纯数值计算，样式表仍为普通值；隐式样式过渡未实现。
若需要状态间颜色／尺寸动画，先明确目标值、失效、快照与生命周期，不每帧替换全局
StyleSheet、不破坏独立 Widget 样式和原生装饰隔离。
多 StyleName、Class／ID selector 仅保留评估方向，不恢复 CSS／隐式级联或局部样式树。
见 [DesignStyle](DesignStyle.md#deferred-evaluation-not-a-committed-api)／[Modern](DesignModernStyle.md)。

<a id="future-performance"></a>

### 高频更新与渲染资源优化：测量触发，暂缓实现

Root.ConnectFrame、Switch／ProgressBar／Tab 帧驱动和 TextLayout→Image 缓存已实现。
原生 vsync／帧时钟尚未接入，不把 compositor 或 Present 的同步等同于动画时间戳同步。

后续先建立同环境、同后端、同规模的延迟／资源预算，再评估：

- 文本编辑的失效范围、排版预算与缓存淘汰；
- layout 私有 textPainter 工作区的按需创建／收缩／释放，不重新共享可变 Pango 状态；
- 高频 UI 回调更新／数据绑定，明确稳定资源与声明闭包的身份；
- 局部重绘、子树缓存、合批或后端 glyph atlas，仅在测量证明收益后进入切片。

不引用没有基准的“GPU 非瓶颈／微秒级”等估计作为结论。见
[DesignPaint](DesignPaint.md)、[文本编辑预算](DesignTextEditing.md)、
[DemoFrameworkNotes](DemoFrameworkNotes.md)、[DesignAnimation](DesignAnimation.md)。

<a id="future-settings-hidpi"></a>

### 设置通知与动态 HiDPI：待评估

当前 GUI Settings 在 Run 期间用 Application.NewTimer 每两秒轮询；UI 已自动请求更新。
X11 scale 按 GOUI_PLAT_SCALE／GDK_SCALE／gtk-xft-dpi 查询一次并缓存；没有动态
monitor-scale 监听。Cursor 的 XSETTINGS 订阅只服务光标主题，不能当作窗口 DPI 能力。

原生设置变化通知、X11 动态 scale／混合 DPI 和通用像素布局取整分别评估；若实现，
必须一起更新 SizeEvent、surface、输入和坐标转换。不能先引入显示器管理模型再寻找
消费者。当前契约见 [Settings](Settings.md)／[HiDPI](HiDPI.md)。

<a id="future-input"></a>

### 输入、可访问性与辅助工具：需求触发

鼠标、侧别修饰键／平台事实转换、快捷键、Tab、手势竞争和 dev 语义 ref 已实现。
尚未实现或未承诺的方向包括 Touch／Pen 原生映射、多指手势、相对位移／指针锁定、
自定义图片光标、输入用途／preedit 子句／reconversion、原生无障碍桥接、Inspector、
dev 截图及虚拟光标。dev 的独立性与正常输入路径必须保留，不为自动化侵入 GUI 内核。

Touch／Pen 须一起设计标识、捕获／取消、手势与多平台输入事实，不提前塞入占位字段。
原生输入限制（如 XIM 同步调用时服务退出）与未实现的新功能分别记录，不以任意超时
或后台访问线程亲和句柄兜底。见 [InputTODO](InputTODO.md)／[DesignIME](DesignIME.md)。

<a id="future-system-integration"></a>

### 平台、系统集成、更多控件与打包：需求触发

- 托盘图标／运行期本地通知尚未实现。保持小接口、Application 生命周期和原生结果；
  Windows 不强制引入额外 SDK runtime，也不以旧托盘气泡冒充现代通知。
- Wayland、移动端及更多目标待具体平台设计。Linux arm64 基础适配按
  [专项计划](PlanLinuxArm64.md) 推进，不与新增窗口系统／图形后端捆绑。
- cmd/goui 已支持原生资源、二进制／macOS .app／Linux 安装目录树；DEB／RPM、
  更完整分发签名或安装集成按需求补齐，不做 AppImage 和系统库复制。
- Linux FileDialog portal 适配待评估，不把现有 helper 当作沙箱 FileChooser。
- GUI owner／transient 窗口、更丰富浮窗焦点策略、Docking／高级控件按具体应用需求
  扩展；platform.Window 已有 Parent／SetParent，不等于 GUI 已提供业务 owner 策略。
  不为一般对话框绕开已有 Overlay，也不把 SkipTaskbar 改成 Popup 类型。
- 通用提权、PTY／helper 控制不在 GOUI 当前范围；由应用或独立库处理，不重新排期。

## 验证与已知限制

功能存在、逻辑通过、跨平台编译、原生运行与桌面场景验收分别记录。
未来实施项必须提供回归与 tests/window 入口说明；本轮只同步文档，不复验历史硬件。
当前验收缺口与原生限制见 [Plan](Plan.md)，不要把它们当成未实现功能。

<a id="withdrawn-proposals"></a>

## 已撤销／不恢复的提案

ClientSize／PrepareLayout、独立 ClientPosition／SetClientPosition、HasListeners 作为
输入查询门槛、WindowBehavior／通用 Restore／FrameMetrics、自由布局 WindowControls、
菜单／DropDown 默认迁移 Overlay、指定窗口输入区域／鼠标穿透、PopupCornerRadius
系统查询、Window 专用 Painter 遮罩和旧 PointerCaptureController 都不进入未来路线。
未采用的附着浮层 Key／UpdateOverlays 也不是已确认 API。

相关历史文件保留显式说明；未来若需求变化须重新讨论，不因旧代码草案尚在文档中
而直接实施。

## 历史路线与验证记录

<details>
<summary>展开原 P0～P4 路线快照与阶段日志（不作为当前任务清单）</summary>

以下原文保留当时的切片顺序、版本与验收证据，部分未带日期的“当前／下一步”
只对应早期阶段。已撤销接口、旧主题归属和阶段限制以本篇前述基线及专题当前契约
为准；这里的未勾选／待实施项不自动进入未来路线。

2026-10-13 macOS 普通窗口默认位置切片：在隐藏的原生尺寸初始化后调用已有
NSWindow.Center 一次，补齐固定 (0,0) 的默认摆放遗漏；不扩公共接口，不改
Show 或 Popup。旧代码真机复现，macOS 三种 Chrome/应用 1x/2x 几何及指定
Software/OpenGL 1x 实际交互、既有 GUI 定位验收通过；本机最终全仓 2481 通过、
15 既有跳过，目标编译通过。范围见 DesignWindowManagement；代码与回归用例
已提交 90bdb83，未推送，原有未跟踪设计/路线文档及测试索引保留工作区。

2026-10-13 Demo 增加 SkipTaskbar 与 BorderBox 辅助窗口示例，沿用 GUI 公共能力与
现有语言/主题更新；只改例程，不扩展平台或框架接口。本机逻辑/race/目标编译与
X11 Software/OpenGL 1x 指定窗口交互通过，全仓 2481 通过、15 既有跳过；
任务栏仅核验 X11 请求属性，范围见 DesignDemo。按用户要求未执行远端桌面，
demo 代码、测试及设计说明已提交 33e8a84，未推送。

2026-10-13 BorderBox 切片按 [DesignBorderBox](DesignBorderBox.md) 实现：
默认无外观，单子安全布局与 None root 自动缩放；不改 Painter/原生能力，不扩展 Popover。
验收修正 macOS None 既有 zoom 状态误报，本机及 Windows/macOS 指定桌面场景、
相关逻辑/race/目标编译通过，最终全仓 2474 通过 / 15 既有跳过 / 0 失败；
中途 Pango 偶发失败及未覆盖项如实记录；提交已拆分为 bc181aa（macOS 状态修复）
与 971ecb9（BorderBox），最终文件树保持不变，go test ./... 再次通过，未推送。

2026-10-13 普通窗口任务栏呈现切片：创建期 SkipTaskbar 与 Chrome/透明/定位独立，
UI 薄协调；保持输入焦点与普通窗口生命周期，不引入 Popup 类别或位置创建字段。
Windows TOOLWINDOW、X11 WM 初始提示、macOS 应用 Dock 边界与差异写入
[DesignWindowTaskbar](DesignWindowTaskbar.md)。本机全仓/race/目标编译与指定桌面场景、
远端 Windows 10 Direct2D/macOS OpenGL 1x 的任务呈现、输入与定位及原生测试通过；
完整 vet 的既有告警、Windows 11/真实混合 DPI/其他 WM 等未验收项明确保留；
代码/用例/对应设计已提交为 1bac615，未推送。
BorderBox 与默认摆放调整不包含在本轮。

2026-10-13 完整兜底迁入 theme/basic，GUI 缓存接入且保留旧入口兼容；未设／恢复
nil 样式表自动覆盖基本及 widgets 控件，自定义样式表仍整体替换。Modern 增加
FromSettings 快照便利入口，GUI/UI 不依赖具体 Modern，不隐藏监听或安装。
迁移保留现行颜色与几何；另修正完整表头暴露的 TableView 无界哨兵扣减错误，
不扩接口或放宽虚拟化约束。本机全仓 2387 通过／15 既有跳过／0 失败、相关 race、
Windows/macOS 编译及 Software/OpenGL 各应用 1x/2x 主题与输入场景通过；普通 vet
的既有 composites 告警与未运行远端桌面明确保留，见 [DesignThemeFallback](DesignThemeFallback.md)。
Basic 边缘优化按确认方案完成：选中方框／开启开关取消同色重复描边，Radio
采用同状态实色底与白点，Switch 固定蓝色柔化；只改 Basic 两个生产文件，
不改 Modern／Widget／Painter。旧样式回归失败已复现；本机全仓 2464 通过／
15 既有跳过、相关 race／限定 vet、Windows/macOS 编译及 Software/OpenGL
各应用 1x/2x 的外观与指定交互通过；远端与 Direct2D 未复验，未提交。

2026-10-12 桌面启动线程统一由公共 platform.go 的 init 保留；GUI/UI 共用，
不重复加后端入口或生产文件。用户从 main 创建并运行应用，后台用 Post／Sync；
Run 返回后保留线程用于清理，init 不创建原生对象。回归在真实 main goroutine
中验证，旧实现已复现迁移；本机全仓／race、Windows/macOS 构建和两平台各
20 次原生无窗口线程回归通过。macOS demo OpenGL 桌面启动、页面切换、
正常关闭通过，用户另行完成验证并确认提交；具体范围见 Plan／DesignApp。

2026-10-12 Windows Integrated 兼容切片：Windows 10 采用 ModernWindow 的
Popup/ThickFrame/MaximizeBox 与顶部 1 物理像素；Windows 11 保留 OverlappedWindow，
顶部不保留非客户区像素，两条路径均安装 DWM 扩展及自定义非客户区计算。
创建与最小尺寸统一客户区 DIP，原生边框换算不泄漏到 GUI/UI，不修改 Painter。
实现及回归用例已完成；本机全仓/race、Windows/macOS 全仓编译、Windows 10
三模式 1x/1.25x/2x 原生客户区/最小尺寸/最大化恢复、Software/Direct2D 1x
绘制通过。Windows 11 圆角像素、真实系统 HiDPI 和可用 OpenGL 桌面仍待验收。
RDP 的 OpenGL 像素格式初始化失败与 Session 0 无 DWM 如实记录，不计通过；
编译、原生运行与圆角像素验收分别记录于 Plan，不能互相替代。

2026-10-12 README 更新为首版发布介绍：使用新 Logo，突出 Go 开发体验、no-cgo、
无需 GCC 与单二进制交付，区分原生系统集成与统一 Modern 控件外观。同步中英文、
替换旧 API 示例和已完成路线项，预留版本化下载及真实截图／动图位置。占用数据
必须注明构建、后端与场景，架构对比不代替等价应用实测。后续聚焦功能完善、
触摸、更多平台、配套工具、用途控件与性能／资源优化；验证结果记录在 Plan。
本轮 no-cgo 且无可用 C 编译器的六项构建、README 示例／链接／数值校验及本机
非缓存全仓通过（2343 通过／15 既有跳过）。体积约 4.5～4.7 MiB / 8.3～8.6 MiB，
Software 1x 的本机空闲 RSS/PSS 经三轮采样并明确环境后写入首页；不将交叉编译
计为 Windows/macOS 原生运行，不伪造等价应用对比，下载和展示仍待发布时补齐。

2026-10-12 Demo 窗口框架紧凑化：标题使用系统正文字号，Logo 24 DIP / 留白
6 DIP；侧栏不再常驻详细快捷键说明；状态栏最小 28 DIP / 留白 6 DIP，长消息
自然换行增高。不重排页面或修改 Modern/框架 API。旧声明回归失败，调整后
全仓非缓存 2343 通过/15 既有跳过，例程 race/vet、三目标构建和本机指定亮暗、
窄窗、大字号、长消息及标题栏交互通过；远端原生桌面未执行，见 DesignDemo。

2026-10-12 Demo 接入用户设计的 `doc/logo`：方形标志用于标题栏及构建图标，
横版用于图形页；嵌入资源且保留原色，删除旧手绘标志。新增例程 `goui.toml`，
使用现有 `cmd/goui` 构建 Linux amd64、Windows amd64、macOS arm64 二进制与
平台资源，不扩框架接口。全仓非缓存 2341 通过/15 跳过、相关 race/vet 与本机
Software 1x/OpenGL 配置 2x 指定外观操作通过；远端原生窗口和桌面图标部署
未执行，见 [DesignDemo](DesignDemo.md)。提权/PTY 不在本轮实现范围内。
随后修正动画区域误用单色 logo 的绑定；新回归在旧声明下失败，修正后全仓
2342 通过/15 既有跳过，三目标重构建及本机初态/中间帧/终态、亮暗/主题色
操作通过。不修改动画、SVG、样式或绘制后端的公共契约。

2026-10-11 文件访问进入完整切片：FileDialog 返回具有明确归属的 FileAccess，
Credential/RestoreAccess 管理可持久化访问资料，GUI/UI 上报错误；macOS 配套
安全作用域、签名沙箱与构建配置。目录/读写复用 Go，不扩充 StorageProvider
或隐式提权。基础切片已实现，本机/远端 macOS 非缓存全仓、相关 race 和 Windows
受影响包通过；三平台实际选择/保存/关闭和跨启动恢复、macOS 显式 ad-hoc
read-write/read-only 沙箱通过。既有 Windows 全仓失败、Linux arm64/vet 阻挡
及稳定证书验证缺口不隐去，实施与验收见 [DesignFileAccess](DesignFileAccess.md) / Plan。

2026-10-10 嵌套滚轮消费修正：ScrollView 按请求轴范围消费，首尾边界不传给外层，
无对应范围则继续冒泡；TabBar 实际条目/箭头在有溢出时消费，空白保持被动。
不扩展公共 API、修改分发器或平台。旧实现复现后，包内回归、全仓 2245 通过/
14 既有跳过、相关 race、Windows/macOS 编译及本机 Software/OpenGL 各应用 1x/2x
的嵌套滚轮输入通过。普通 vet 既有警告和未验收项见 [DesignScrollLayout](DesignScrollLayout.md)。
原生拖放被外部目标消费以及窗口位置的系统约束按用户确认保留，不新增模拟拖放或主动退避。

2026-10-10 Demo 本地化补齐：默认编辑样例、拖放说明、计时和树/列表/表格展示
进入八语言 resource，编辑后用显式归属/模型 Revision 保留用户内容与历史，
不扩大框架 API。旧版复现五处遗漏；全仓非缓存 2218 通过/14 既有跳过、相关
race/vet、Windows/macOS demo 编译、本机 Software 1x 八语言指定场景与 OpenGL
配置 2x 编辑场景通过。远端原生桌面及母语校对未执行；demo 按用户确认独立提交，见
[DesignDemo](DesignDemo.md) 和 [DesignLocalization](DesignLocalization.md)。

2026-10-10 短菜单布局收敛：PopoverMenu 主体最小宽度 160 DIP，包含内部留白、
不含阴影；长文案自然撑宽，窄工作区仍为硬约束。复用 Widget 的既有尺寸机制，
不扩大样式/平台/UI API，不影响通用 Popover 或 DropDown。确定性回归复现旧问题
后通过；非缓存全仓最后复跑 2202 通过/14 既有跳过、相关 race、Windows/macOS
目标编译及本机 Software/OpenGL 配置 1x/2x 指定菜单场景通过。普通 vet 既有
警告及额外隔离复跑中一次 Pango 偶发失败（独立五次和全仓复测通过）已保留记录；
原生远端/Direct2D/真实 HiDPI 未执行，见 Plan / [菜单外观设计](DesignPopoverMenuAppearance.md)。

2026-10-09 Modern 通用预设推进：Card、Sidebar、Flat、Warning、Error、Success；
保持独立样式与消费者绘制职责，不扩展 Widget 或平台接口。demo 采用既有 surface
展示卡片与侧栏，基础页展示 Flat 和语义前景。全仓 2138 通过/14 既有跳过、
相关 race/vet、Windows/macOS 示例编译、本机 Software 1x/OpenGL 请求配置 2x
指定窗口场景通过；原生远端及真实 HiDPI 未执行，见
[DesignModernPresets](DesignModernPresets.md)。

Sidebar 随体验反馈单独调整为更有层次的亮暗中性背景；Flat、普通按钮和当前
分类的 Primary 不变。此前保留工作区供用户体验，本轮已获明确提交指令。
全仓、相关 race/vet、Modern 目标编译及本机 Software 1x 指定交互/截图对照
已通过。另补齐 demo 八处 Card 样式引用遗漏，恢复面板/Dialog/Toast 及其他表面
背景；保留同名拖放数据格式，不修改核心样式解析或绘制契约。
全仓 2140 通过/14 既有跳过、相关 race/vet 与 Windows/macOS demo 编译通过；
本机 Software 1x 亮暗窗口交互及内部背景/透明外围的独立采样通过。

明确 Modern 的中性桌面视觉理念与类别描边契约：Flat 仅在常态收起表面，
悬停/按下恢复普通按钮的背景和 1 DIP 描边；保留独立焦点、应用覆盖和布局。
全仓 2141 通过/14 既有跳过、相关 race/vet、受影响目标编译及本机 Software 1x
亮暗交互/像素验证通过，完整范围见 DesignModernPresets；未验收其他后端或远端。

2026-10-09 多语言资源首版完成：独立 mlang 与应用 resource 集中管理，命令层
显式 translateUI、声明层自动协调；首批框架文案提供简中、英、日、韩、德、法、
俄、阿拉伯语。全仓 2129 通过/14 既有跳过、相关 race、Windows/macOS 编译及
本机 Software/OpenGL 配置 1x/2x 八语言切换通过；真实打字、实时菜单和翻译后
复制已在本机指定配置验证。远端原生桌面/母语校对未执行，完整 vet 的既有
警告、后端回退和缩放覆盖边界见 [DesignLocalization](DesignLocalization.md)。
后续补齐八语言便利入口与 Settings.Language，Windows/macOS 原生无头查询通过；
demo 八语言菜单及一次性使用系统语言保留模型、编辑状态和固定 ID，四组本机
配置验收通过。全仓 2135 通过/14 既有跳过；额外 Linux arm64 全仓编译因既有
libc SYS_PIPE/SYS_POLL 不可用而失败，本轮未修改，详见
[DesignSystemLanguage](DesignSystemLanguage.md)。不扩大为完整格式器或 RTL 镜像。

2026-10-07 综合演示：`example/demo` 以九类交互场景展示已有控件、Modern 主题、
编辑、虚拟化、标签移交、覆盖层、绘图/动画、系统操作与拖放、窗口/语义检查。
保持资源和业务模型在重建之外，默认空闲，初始实现采用已有公开扩展。
设计和验证范围见 [DesignDemo](DesignDemo.md) / Plan；开发发现单独归入
[DemoFrameworkNotes](DemoFrameworkNotes.md)，不把未验证现象扩成平台缺陷。
体验反馈的滚动条边缘裁剪和 Integrated 辅助窗误触透明背景已在例程修复；
后续显式透明 None 的背景串改通过最小的 Window.StyleName / SetStyleName 修复，
沿用 Popover 的独立背景选择，不修改平台、Painter 或布局。全仓及本机指定场景
通过，Windows/macOS 仅编译。随后确认的 Linux GUI 自绘 Integrated 自动布局
避让已实现：根内容与安全 clip 一致，HeaderBar/按钮居中使用内容起点，四边
仍用于窗口缩放。不影响其他窗口模式或平台，不引入公共 SafeArea；验证记录
见 [DesignGUIWindowChrome](DesignGUIWindowChrome.md)。demo 随后移除主体重复的
8 DIP 外侧留白，页面内部间距和透明 None 外围设计保留；包内回归、全仓及本机
Software 1x/OpenGL 配置 2x 指定场景通过，Windows/macOS 仅交叉编译，验证见 DesignDemo。
常见滚动随后推进为独立切片：ScrollView 方向配置、双条占位与嵌套 Stretch
自然尺寸一致性；demo 移除局部适配。见 [DesignScrollLayout](DesignScrollLayout.md)，
全仓非缓存 2087 通过/14 既有跳过、相关 race、本机实际 Software/OpenGL 各
应用 1x/2x 指定输入场景及 Windows/macOS 编译通过；完整 vet 既有报警、
Linux arm64 既有 libc 编译问题与原生验证缺口见设计执行记录。

2026-10-07 用户深度体验后，当前优先修功能正确性：demo 的列宽受控值与关闭
确认归属、按钮键盘激活、Transferable TabBar 的空白 Caption；随后处理 Linux
跨应用拖放清理、macOS 脱离定位及明确后的光标跳变，再调整 TreeView 与页面
留白。九项修改已实施，全仓/race/目标编译、远端相关包内测试及三平台指定
窗口场景通过；GoLand 指定应用视觉复验及完整 HiDPI/后端矩阵仍未执行。
源码依据见 [DemoFrameworkNotes §11](DemoFrameworkNotes.md#11-深度体验反馈功能优先的待修复清单)，
修正、实测与边界见 [§12](DemoFrameworkNotes.md#12-深度体验反馈修复与验证记录)。
2026-10-09 追加 TableView Modern 外观收敛：浅中性选中、低对比表头／分隔线与
FocusVisible current 细框，不改 Cell 分配或公共 API。本机 Software/OpenGL
1x/2x 指定场景及自动回归通过；远端桌面未复验，见
[§13](DemoFrameworkNotes.md#13-tableview-modern-外观收敛)。
高频 UI 优化按用户要求暂缓，仅保留标题、稳定子树与样式构建的候选及验证
边界；尚无性能对照，不先扩充协调器或局部响应式更新。样式动画也不随本轮
修复扩大范围。候选记录见 [DemoFrameworkNotes §10](DemoFrameworkNotes.md#10-高频-ui-绑定优化记录暂缓实施)，实施清单见 Plan。

2026-10-07 帧更新与数值动画首版完成：Root.ConnectFrame 是布局/绘制前的被动
信号，唯一唤醒入口仍为 RequestPaint；主动请求合并、60Hz 单次唤醒节流与隐藏/
释放/销毁清理共享现有 Application 到期堆。独立 animation 包只依赖 core，
支持显式缓存值过渡与自定义插值/曲线；Switch / ProgressBar / TabBar 迁移共享帧，
UI 复用 State 和既有异步协调，不引入隐式属性动画。全仓、相关 race、目标编译、
远端包内与本机/Windows/macOS 指定窗口验收通过；Windows 同步 Popup.Show
时序回归已修复。原生 vsync、严格同帧协调、混合 DPI 及额外后端待后续设计，
验证配置及缺口见 [DesignAnimation](DesignAnimation.md) 与 Plan。

2026-10-06 Widget 统一禁用切片：Enabled 保存自身设置，gui.IsEnabled 查询祖先
限制后的结果；移除 EffectiveEnabled 与实际状态信号，保持显示、
内容和 Focusable 配置；输入、焦点、手势、拖放及文本编辑清理共用内核机制。
CheckButton / Switch / DropDown 迁移公共实现，UI 通过 ViewBase.Enabled 协调。
Label / Icon 解析自身 Disabled 样式，不引入父颜色继承、属性系统或平台接口。
实际资格变化复用延迟 StyleChanged，输入立即取消；Window/Popover 共用静止
指针恢复，DropDown 弹层在输入入口阻止失效操作与自定义选项回调。
实现、针对性回归及中文窗口例程完成；验证范围和待验收项见
[DesignWidgetEnabled](DesignWidgetEnabled.md) 与 Plan，不以编译替代原生运行。
接口收敛后全仓、相关 race、无 DISPLAY 指定回归与 Windows/macOS 目标编译
通过；本机 Linux/Xfwm4 Software／OpenGL 配置各应用 1x／2x 指定场景复验
通过；macOS 15.7.7 arm64 原生 GUI/widgets/UI 指定回归及 OpenGL 配置应用
1x／2x 桌面交互也已复验。Windows 最新程序仍被 Defender Exploit Guard
阻止启动（事件 1121），未修改防护设置，原生验收待完成。应用缩放覆盖值与
后端 fallback 限制单独记录，不扩大为真实 IME、Retina 或混合 DPI 验收。

2026-10-06 DropDown 留白调整：主体外围至少 6 DIP；垂直滚动条独立占列，
选项与滚动条的间隔等于左侧留白，选项背景、命中和快照共用实际边界。
宽内容和长短切换回归、全仓、相关 race/vet 与 Windows/macOS 目标编译通过；
本机 Xfwm4 Software 1x/OpenGL 2x 长短列表外观及间隔输入通过；Windows 10
Direct2D 配置/macOS 15.7.7 OpenGL 配置的应用 1x/2x 远端长短列表、间隔输入、
滚动及尺寸切换也已复验，缩放覆盖与后端 fallback 的验证边界单独记录。
验证结果见 [DesignDropDown](DesignDropDown.md)。

2026-10-06 Popover 定位接口整理：公开 Point / Bottom 策略，SetPosition 与
SetPlacement 独立，统一由 Show 显示；MenuButton、PopoverMenu 和 DropDown
共用公开能力，可见期间通过既有布局请求更新。全仓、相关 race、Windows/macOS
目标编译及 Linux X11/Xfwm4 Software 1x 定位场景通过；远端 DropDown Bottom
已随留白调整复验，其余菜单定位与其他后端/缩放未复验，结果见
[DesignPopupPlacement](DesignPopupPlacement.md#54-2026-10-06-定位接口整理验证记录)。

2026-10-06 DropDown 切片完成：widgets 非编辑式单选，复用 ListView / ScrollView /
Popover，UI 使用 Selected / OnSelected 和实际内容 View；不加入编辑、搜索、
多选或新的列表接口。显式兜底 / Modern、快照、逻辑 / 像素 / UI 测试及中文窗口
例程完成；全仓、相关 race / vet、Windows/macOS 编译与本机 Software / OpenGL
1x/2x 指定场景通过。Windows Direct2D / macOS OpenGL 两端原生桌面 1x/2x 已验收；
macOS 短列表 Popup 像素对齐回归已修复并复验，硬件 OpenGL 专项、真实 Retina
与混合 DPI 待验收，GUI 默认
vet 的既有警告单独记录，详见 [DesignDropDown](DesignDropDown.md)。

2026-10-06 Switch 切片完成：独立两态开关、可选内容、点击／水平拖动竞争、
120ms 滑动过渡、键盘焦点、widgets/ui 薄绑定、显式兜底／Modern 与语义快照。
GUI／平台／Painter 无新增接口。全仓、相关 race／vet、Windows/macOS 编译，
本机 Software/OpenGL 1x/2x、兜底、动画采样及回调销毁清理验证通过；
Windows Direct2D／macOS OpenGL 远端桌面各 1x/2x 已完成 108 项状态断言、
外观及关闭清理验收；硬件 OpenGL 专项、Windows OpenGL 与混合 DPI 待单独验收，
详细范围见 [DesignSwitch](DesignSwitch.md)。
外观精简已同步 Modern／兜底：无描边白色滑块、弱轮廓关闭轨道、开启边框与填充
同步及弱化禁用配色；保持尺寸与交互，全仓和本机 Software/OpenGL 1x/2x 复验通过。

2026-10-09 随体验反馈仅修正 Modern 开启轨道：取消同色描边的重复抗锯齿合成，
保留关闭态弱轮廓、独立焦点、尺寸与配色，显式兜底和 Painter 不变。48 组独立
圆弧覆盖率回归在旧实现失败、修正后通过；全仓 2190 通过/14 既有跳过、相关
race/vet、受影响目标编译及本机 Software/OpenGL 1x/2x 指定场景与 1x 前后
像素对照通过。Direct2D/原生远端等未复验项单列于 DesignSwitch 与 Plan。

2026-10-06 CheckButton 切片完成：widgets 三态、显式非拥有互斥组及指示器／
按钮式外观，widgets/ui 薄绑定、独立子样式、兜底／Modern 和具名语义快照。
GUI 核心／平台／Painter 无新增接口；Switch 留作独立控件。全仓／相关 race／
vet、无 DISPLAY 回归、Windows/macOS 编译通过。本机 Software/OpenGL 1x/2x
正常输入、分组与重建、主题、键盘及回调销毁验证通过；隔离桌面装饰回退 Native，
未验收两端原生桌面或硬件 GPU，详见 [DesignCheckButton](DesignCheckButton.md)。
外观统一调整：未选中按钮式与普通 Button 对齐，选中改为柔和填充；checkbox／
radio 使用彩色标记，默认 Padding 6 DIP。全仓／race／vet、Windows/macOS 编译、
本机 Software/OpenGL 1x/2x 像素与交互复验通过；额外发现的 Linux arm64 既有
libc 编译阻塞及两端原生验收缺口见 Plan 和设计文档。
第二轮按体验反馈调整视觉结构：18 DIP 勾选框采用实色底／白色标记，单选框
采用独立圆环／圆点样式；加强按钮选中底色，例程接入系统字体与字号。
第二轮全仓、相关 vet、Windows/macOS 编译、本机 Software/OpenGL 1x/2x 与
显式兜底复验通过；实机平台范围仍以设计文档的验证记录为准。
按钮式选中底色进一步改为中性灰阶，亮色加深、暗色提亮，不随主题色染色；
勾选／单选及键盘焦点保持原样。全仓、本机 Software 1x／OpenGL 2x 的
选中／悬停／按下与主题切换复验通过；本次未重复远端原生桌面验收。
2026-10-13 Modern 单选外观按最终反馈与勾选框统一：选中采用主题色实心圆底与
白色圆点，零描边；实底共用 CheckBox 三态选中配色，禁用为中性实底。
未选中仍为中性空心圆，控件几何及 GUI／Painter 接口不变。恢复圆点／实底的
4.5:1 对比回归；验证记录见 [DesignCheckButton](DesignCheckButton.md#modern-实底单选指示器2026-10-13)。

ProgressBar 切片：widgets 控件支持长条／圆形、确定／忙等待模式，圆形活动
可替代 Spinner，复用现有 GUI Timer／Painter／样式／UI 协调及 Snapshot 扩展。
长条改为单段从左向右循环，不往返；声明绑定和兜底样式分别属于 widgets/ui
与 widgets/style。迁移后包内及全仓／race／相关 vet／本机窗口／交叉编译通过。
圆形忙等待改为固定 90° 弧段、1.4 秒一圈的顺时针匀速旋转；包内回归及本机
Software／OpenGL 1x/2x 连续帧、生命周期复验通过，全仓／race／相关 vet 通过。
远端原生桌面、硬件 OpenGL 与混合 DPI 待验收。不引入公共动画或平台进度 API。
详细契约与验证缺口见 [DesignProgressBar](DesignProgressBar.md) 及 Plan。

> 定位：记录项目从可行性验证进入真实桌面应用开发的主线。扁平工作清单见
> `doc/Plan.md`，具体技术契约见对应 `Design*.md`。若旧笔记与本文或
> `AGENTS.md` 冲突，以后两者为准。

## 1. 当前阶段

2026-10-05 FocusVisible 完成 GUI 提示状态与样式接入：与逻辑 Focused 分离，
默认 Tab 开启、鼠标按下关闭、程序聚焦保留模式；Window/Popover 隔离。
按后续确认方案，Modern 普通/Primary 按钮与 GUI 兜底使用透明背景及 2 DIP
柔化主题色边框。FocusVisible 明确覆盖键盘样式，TextInput 既有边框保留；
UI/平台无新增机制。2 DIP 版全仓 1910 个测试/子测试、相关 race、无桌面回归与
Windows amd64/macOS arm64 目标编译通过。本机 X11/Xfwm4 Software/OpenGL
的 1x/2x 指定真实键鼠、亮暗/主题色/Primary 与 Popover 场景通过；独立像素
预期确认边框 2/4 物理像素、内部底色及布局不变，TextInput 不叠加。
Windows/macOS 桌面、Direct2D 等未验收；当前证据见 DesignKeyboardNavigation
§7.3，§7.1/7.2 分别保留旧底色与 1 DIP 版历史，不代替本轮验收。

2026-10-05 默认 Tab 导航采用最小 GUI 切片：复用 Focusable、当前树顺序和
宿主焦点入口，在控件／快捷键消费后兜底，支持反向循环、Popover 隔离及滚入
视口；不引入 TabIndex/TabStop 或修改控件内部键盘行为。UI 无新增机制。
全仓／相关 race／无桌面新回归、Windows/macOS GUI 测试目标及窗口程序编译通过；
本机 Software 1x、OpenGL 1x、Software 2x 指定真实输入场景通过。两端原生桌面、
Direct2D、中文 IME 与混合 DPI 未验收，详见 [DesignKeyboardNavigation](DesignKeyboardNavigation.md)。

2026-10-05 TableView 正文填满切片：取消固定 Cell Padding，内容填满扣除网格后的
区域；UI 内部容器不再收缩内容。例程增加按 RowID 保存的 Notes 编辑器、独立
直角主题色焦点边框及 F6 检查。未增加样式 Padding、编辑事务或 GUI 核心接口。
两条旧行为回归已复现并修正；全仓／相关 race／无桌面逻辑、Windows/macOS 编译及
本机 Software 1x GUI/UI、OpenGL 1x UI、Software 2x UI 指定操作通过。
Windows/macOS 原生桌面、Direct2D 及其它缺口记录在 DesignTableView §15／Plan。

2026-10-05 快照扩展边界调整：公共 WidgetInfo 保持结构体，Attributes／SetAttribute
允许控件包提供具名语义数据。TableInfo／HierarchyInfo 归 widgets，不再随复杂
控件增加核心专用字段；快照数据隔离与 JSON 失败响应契约见
[DesignWidgetSnapshot](DesignWidgetSnapshot.md)。全仓／相关 race／无桌面快照与两端
编译通过；本机 Software 1x Table UI／Tree GUI 的 Snapshot 集成验证通过。没有
重跑两端原生桌面或其它绘制后端，具体范围见其第 9 节及 Plan。

2026-10-04 完成通用交互扩展：菜单触发集中到 GUI 控制器；公开 TreeExpander
并允许整行组合，RowAt 提供已布局行命中；ScrollView 通过共享 rootBase 的拖放
观察与 Timer 完成静止指针自动滚动及目标更新。UI 仅做绑定，ListView 不扩展接口。
全仓/race、两端原生逻辑测试及三平台 Integrated 1x 核心窗口流程通过。
外部文件拖入、Popover 根、2x/混合 DPI 等仍待补验，见
[DesignInteractionExtensions](DesignInteractionExtensions.md)。

2026-10-02 拖出窗口按实际内容区收敛：TabDetachRequest 携带 ContentSize（DIP），
应用加上自己的装饰开销，在显示前定位、移交原页面。撤销 ClientSize/PrepareLayout、
隐藏目标占位页和像素比例探测；保留客户区定位、pending 占位和整批 AfterUpdate，
DragFeedback 分离光标与回弹动画。Linux Software 1x 的命令式拖出（含源窗口放大）、
Integrated UI 重建/编辑/源关闭及取消/准备失败通过；本轮远端桌面未重验。
当前契约见 DesignTabTransfer 第 31 节，下述隐藏布局/精确热点方案为历史记录。

2026-10-02 远端复验发现并修复 UI 完成钩子早于其它窗口树更新、macOS 隐藏窗口
未报告原生按钮占位、Windows OLE 滚动提示位阻断中性光标的问题。
Windows 10/Direct2D 与 macOS 15.7.7/OpenGL、Integrated 1x 的声明式拖出、重建、
继续编辑和关闭源通过，原 Widget/State 保留；Windows 原生箭头采样、Esc 和
OLE 无头测试通过。定位例程修正小于内容最小高度的初始尺寸后，Windows Integrated
逻辑 2x 全部断言通过，不增加生产位置补偿。此前 ASR 阻止的记录仍保留，但最终
构建已实际运行，未修改安全策略。其它后端及混合 DPI 仍未验收，见 DesignTabTransfer 30.5。

2026-10-02 Tab 拖出反馈/初始定位修复：目标在隐藏状态完成真实布局、客户区定位和
原页面移交后再 Show，源标签在 pending 请求期间保持隐藏占位；单次原生反馈请求
关闭 AppKit 未接受回弹、在允许源端后续操作的区域使用中性光标，原生结果契约不变。
Position/SetPosition 统一为客户区原点（含 Native）；移除单独客户区 setter，
拖出信号用默认 false 的 handled 查询，不扩展信号连接计数。上一版
Linux/Xfwm4 和 macOS 的 Native/Integrated 1x 拖出定位已通过，Linux UI 重建/编辑和
macOS 分步 Esc 取消通过；Windows EXE 被 Defender ASR 阻止，本轮桌面验收有缺口。
全仓/race/跨平台编译与剩余验证见 Plan、DesignTabTransfer 第 30 节。
客户区/handled 调整后已重跑全仓、相关 race、两端目标编译；Linux/Xfwm4、Software
三种 Chrome 的 1x/逻辑 2x 定位（含隐藏查询）、Native/Integrated 1x 拖出和
Integrated UI 重建/编辑/关闭源通过。Windows/macOS 原生桌面本次未重跑，
其它后端/WM、多屏与混合 DPI 不作已验收承诺，详见 DesignTabTransfer 30.4。

2026-10-01 标签上下文菜单由 TabBar 按页面同步查询 MenuModel，并在内部复用 PopoverMenu，
UI 通过 TabBar.ContextMenu builder 提供菜单项；共享 TabView 的多个标签栏菜单独立，
不保留 TabView 菜单入口；关闭继续统一走 TabView 的请求信号。
不扩大平台能力或增加全局菜单／命令体系。包内／全仓／相关 race 与 Windows/macOS
目标编译通过；Linux 隔离 X11／Xfwm4、Integrated、Software 1x/逻辑 2x 已验证
右键目标、菜单关闭和 Shift+F10，1x 另验空白拖窗及移动后定位。Windows/macOS
真实桌面和其它后端本轮未验收；详见 Plan，契约见 DesignTabs。
菜单查询从 TabView 收敛到 TabBar 后，已重跑全仓／相关 race／跨平台编译，并在
Linux 隔离 X11／Xfwm4、Integrated、Software 1x 验证目标页菜单、关闭、Esc 和
Shift+F10；2x 和其它平台桌面未在本次 API 归属调整后重验。

2026-10-01 UI 身份模型收敛：ID 继续按 Root 分域，同一挂载目标内参与类型／身份匹配；
无 ID 仍按位置匹配，不引入 Key、全局 ID 或隐式跨树认领。TabPage 是普通节点，
通用协调负责重排与显式节点移交，不再维护页面 key 映射或公开任意 State 查询。
Linux Software 1x 两条真实编辑／移交／重建／源移除流程及全仓/race/跨平台编译通过；
本轮 Windows/macOS 桌面及其它后端、缩放组合未重跑。见 DesignTabTransfer 第 29 节。

TabView 拖出主要功能已完成：跨容器/跨窗口保留原页面及 UI 状态、拖出新窗口、
取消和准备失败保留源页面，三平台主要路径已验证。按用户确认，参考成熟框架实现，
不再反复穷举边缘条件；混合 DPI 等保留为后续补充，不作为本轮交付门槛，也不声称
已经通过。最新收尾见 DesignTabTransfer 第 26 节，以下保留各实施阶段依据。

Tab 转移的原生结果切片已按框架实践调整：完成动作优先于取消请求；零动作表示
未接受，不承诺区分所有外部拒绝与无目标。已补齐三平台结束位置、修复 AppKit
成功被取消覆盖和 GUI 提交重入问题。历史探针证据保留，不把原先失败改写为通过。
不再以完美原因分类阻塞后续手势接管、页面/UI 子树移交。临时平台 trace 与诊断专用
OLE Notify 已移除，用例改为公开事件断言；Windows/macOS 1x 的正常放下和按住时
Esc 已实测。精确回弹、外部目标和混合 DPI 等缺口继续跟踪，见 DesignTabTransfer
第 10–12 节及 Plan。

TabView 跨窗口拖出的前置能力已接入：DesktopWindow / gui.Window 的一次性
SetPosition，nil 表示相对调用时自身工作区左上角，有参考时使用参考客户区
DIP，目标为客户区左上角；原生装饰换算留在后端，不引入显示器或公共桌面坐标体系。详见
[窗口定位契约](DesignWindowManagement.md#one-shot-relative-positioning)。
窗口定位与标签转移分别记录实施和验收，不以定位通过代替页面转移验收；详见 Plan。
跨窗口标签转移的最终方向已整理到 [DesignTabTransfer](DesignTabTransfer.md)：
保留原 TabPage/Widget 子树，以普通 GUI 生命周期更换宿主；UI 同步移交挂载目标
及协调状态，应用负责新窗口与空源窗口策略。手势接管、命令式 TransferPage、
TabBar 显式跨栏转移与插入标记已实现；三平台 Native 1x 跨窗口移交、Esc、
成功后源窗口销毁通过。命令式拖出请求及应用准备新窗口/定位流程已接入（第 14 节）；
UI 通用 Bin 协调记录与页面 key 映射移交也已接入，三平台 Native 1x 的既有窗口
转移、重建、继续编辑和源窗口删除通过（第 15 节）。UI 新窗口准备、定位与失败清理
也已整合，三平台 Native/Integrated 1x 拖出、Native 同 key 拒绝/准备取消与
Integrated Esc 通过，Linux Integrated 另验逻辑 2x（第 16 节）。本地协商拒绝已补充
最终注册窗口命中事实，三平台拒绝及离开后仍可拖出通过（第 17 节）。命令式卸载中
目标销毁后的恢复、提交通知中源销毁后继续编辑，三平台 Native 1x 通过（第 18 节）。
UI 协调的卸载中目标销毁回滚、挂载中源销毁提交也已完成三平台 Native 1x 重建/编辑
验收（第 18.2 节）。同窗口双 View 向空栏转移、跨窗口静止悬停滚动后尾插与滚动后
Esc 已完成三平台 Native 1x 验收（第 19 节）。同一 View 多栏的原生插入错位已修复，
三平台中间插入与 Esc 通过（第 20 节）；UI 同 Root 的两种协调顺序、重建/编辑/
源容器卸载与 Esc 也已通过（第 21 节）。Integrated 多栏重排、同 Root target-first
转移/重建/编辑/源移除与取消已通过 Windows/macOS 1x、Linux Software 逻辑 2x
验收（第 22 节）。其他组合与真实混合 DPI 继续跟踪；不以零动作推测无目标，
不引入全局页面所有者。
最后一个多行编辑页的命令式移交已通过三平台 Integrated 1x、Linux Software 逻辑 2x
验收：原文档、选区、滚动及撤销记录保留，关闭源窗口后真实键盘仍可撤销、重做和输入
（第 23 节）。不将此结果扩展为 UI 多行编辑页重建或真实混合 DPI 已完成。
拖动中目标重新布局后的投递和 Esc 已通过三平台 Integrated 1x（第 24 节）。同时
发现 X11 整段源拖放会话的超时限制会终止长时间按住的操作；已改为仅限制释放后的
协议等待，三平台 Native 1x 长时间投递/Esc、Linux 未注册目标及 Integrated 长悬停
投递通过（第 25 节）。无应答目标恢复时向已销毁源发送迟到应答的 BadWindow 已按
请求范围处理；目标恢复后存活并接受下一次拖放，通过回归（第 26 节）。不扩大为
所有外部对象生命周期均已覆盖。
已补充同参考系的 `Position(relativeTo)` 原生位置观察；不缓存设置值或承诺
请求后立即完成，不增加位置通知、UI 绑定或显示器身份。X11 观察缺失时显式返回错误。

当前主线进入 Drag and Drop：RenderWidget 已实现并由用户确认验证，
[DnD 详细设计](DesignDragAndDrop.md) 已落到三平台原生服务、GUI 控制器、UI 描述符与快照。
GUI 输入现已接入 [手势竞争机制](DesignGesture.md)：`rootBase` 保留原名并集中宿主公共状态，
Click、Drag、文本选择、源拖放与非原生 Chrome 共享同事件裁决和取消规则。
跨平台原生接管及组合场景仍须按 Plan 中的待验收项复测。
Linux/X11 1x 的 Thunar 文件双向 Copy、INCR、应用内同窗口/跨窗口/Popover 拖放、
预览和 UI 重建已验收；2x 的 Popover、预览及 Esc 取消也已验收。Windows/macOS
远端按约定只做无头验证；真实 Explorer/Finder 互操作、更多跨进程场景及销毁竞态
仍是发布前验收项。首版不扩展虚拟文件、自动滚动或文件任务系统。
验收入口已按平台、GUI、UI 分工整理，提供独立格式接收区和可见失败计数；
本轮 Linux 两个进程间 URL/MIME/双向大文本及 Local 隔离已验证，记录见 Plan。

2026-09-24 [Widget ID/Name 分离与查询](DesignWidgetIdentity.md) 已完成代码与包内回归，
Window / Popover 公开接口统一嵌入 Root，查询不依赖内部类型断言。
按 ID 获取现有 Widget 生成 Preview 的窗口用例待真实桌面验收。
[UI 拖放声明](DesignDragAndDrop.md#10-ui-与语义快照) 已在现有描述符上增加 Data；
按需使用 GUI 事件契约，不引入 WidgetRef、平行拖放状态机或协调 Key。

前一切片 WrapBox：纯算法、GUI/UI 容器、语义快照和测试例程已实现；
Linux 全仓/race 检查、Windows amd64/macOS arm64 交叉编译通过。
示例显式设置窗口最小尺寸后，Linux/Xfwm4、1x（含显式 Software）的缩窗换行、
点击及滚动已验证；其他平台和 2x 桌面验收待进行。窗口自动最小尺寸的通用
推导策略仍需单独设计。详见 [WrapBox 设计](DesignWrapBox.md) 与 Plan。
保留现有 gui 包结构，不继续迁移既有控件；新增的复杂 TabBar/TabView
放在同仓库 `widgets`，声明式适配位于 `widgets/ui`。见 [Tabs 设计](DesignTabs.md)。
标签跟手排序、相邻让位过渡、字体自适应高度与 HeaderBar 空白 Caption 已接入；
Modern 中性选中底面、紧凑留白及真实标签间距已接入，不改变窗口装饰或 Painter。
未选中标签保持正常文字/图标色，关闭按钮按选中、悬停或关闭按钮自身焦点显示，
标签主体焦点不强制显示关闭按钮，显隐不改变布局。
TabWidthRange 已接入：默认 112..240 DIP 全部标签等宽压缩，到最小宽度后滚动；
相等上下限可保持固定宽度。标题只做视觉裁剪，相邻未选中标签提供中性装饰分隔线，
悬停、选中及拖动期间按状态隐藏；不扩展 Painter 或 Snapshot 节点。
WidgetBase 的同父节点重排现明确为 MoveChildBefore/After 相对接口，保持绘制、命中
和默认快照兄弟顺序一致；Tab Role 常量归属 widgets，兜底外观归属 widgets/style，
由应用显式组合，GUI 不再收录复杂控件的专属样式规则。
Linux/X11 的本机 1x/2x 窗口验证与包内回归见 Plan，其他平台桌面验收仍单列。

GOUI 已经验证三层架构、三平台窗口与输入、基础绘制、文字排版、约束布局、
常用基础控件、声明式协调和语义自动化链路。现阶段目标不再是证明框架可行，
而是使它能够稳定、高效地构建具有现代外观的常用桌面应用。

现有基础包括：

- `ui -> gui -> platform` 分层，以及独立的 `core` / `layout`。
- Windows Direct2D、Windows/macOS/Linux OpenGL 和 Software 绘制路径。
- Path、Transform、clip、线性渐变、BoxShadow、持久 Image、TextLayout 图片缓存
  和 HiDPI。
- 原生 Window、Popup、事件循环、IME、Clipboard、Settings 和 FileDialog。
- Button、Label、Image、TextInput、ScrollView、ScrollBar、ListView 和 Menu。
- `ui` 声明式协调、语义快照和 `gui/dev` 输入自动化。

仍需持续处理三平台真机差异和性能回归；可见缺陷始终高于路线中的新增功能。

## 2. 排期原则

1. **先消除基础歧义**：绘制、布局、输入和资源归属不清时，先重构基础设施。
2. **平台事实自下而上**：平台能力先在 `platform` 验证，再进入 `gui`，最后按需
   提供 `ui` 薄适配。
3. **外观能力先于成套控件**：主题、图标、Popup 边缘等基础表现力先稳定，避免
   每个高级控件自带一套临时视觉方案。
4. **命令式内核先稳定**：控件行为、信号、焦点、生命周期和语义先在 `gui`
   完成，`ui` 不实现第二套状态机。
5. **不静默降级**：平台不支持时返回明确事实；默认与 fallback 由上层决定。
6. **每个切片独立验收**：单元测试、平台测试和可重复真机场景随功能一起完成。

## 3. P0 · Widget Painter 重构

状态：**已完成（2026-09）**。

目标是把 Widget 树遍历和绘图上下文解耦，并消除嵌套 `SubPainter` 包装链。

- `gui.Painter` 只提供局部绘图、`Save/Restore`、clip、transform 和资源创建。
- `Widget.Paint` 只画自身；GUI 内核在返回后自动按树顺序绘制子节点。
- 每个 Widget 获得独立的局部原点、结构 clip 和 Identity 局部 transform。
- ScrollView 用真实内部 viewport 表达内容裁剪，绘制、命中测试和结构一致。
- `platform/graphics.Painter` 的 Surface 帧接口和三个后端均不改变。

详细契约见 `doc/DesignWidgetPainter.md`。

退出条件：不存在 `SubPainter`、`PaintChildren` 或 GUI 无参数 `Begin/End`；状态
失衡与 panic 可恢复；Window/Popup 共用遍历入口；完整测试通过。

## 4. P1 · 现代外观基础

### P1.1 三平台透明表面与 Linux GUI Integrated

常规窗口推荐 Native/Integrated；None 仅面向特殊自绘需求。None 的 Chrome
区域协作和 macOS 显式 BeginResize 适配已实现，光标由调用方负责；macOS
真机缩放验收仍待执行。Linux GUI 自绘 Integrated 和 Caption 双击交互已实现：
透明圆角主体、实线边框、GTK4 风格圆形控制按钮，不绘制 GOUI 阴影。

透明绘制已扩大为三平台 Window/Popup 共用并实现，见 [透明绘制](DesignTransparency.md)；
三平台底层呈现已由用户验证，GUI Window/Popover 透明选项与背景样式已接入，GUI 示例待体验。
指定输入区域/鼠标穿透已按用户决定取消，旧草案仅作历史记录。透明角落仍属于矩形输入表面。
GUI Integrated 使用 None + Transparent；平台仍报告实际能力和 Unknown 装饰观察，不模拟 HitTest。
实现、能力失败策略及两种 Painter 的验证见 [GUI Window Chrome](DesignGUIWindowChrome.md)。

### P1.2 当前主线：Modern 样式与 PopoverMenu

按 [Modern 样式设计](DesignModernStyle.md) 顺序独立交付：

1. BoxShadow 样式值及三后端阴影颜色约定。
2. Widget.StyleChanged 与派生资源、测量和列表缓存失效。
3. 窗口装饰隔离：GUI／平台管理装饰，应用样式只改变窗口背景。
4. UI 内部响应 Settings 变化并合并 RequestUpdate。
5. 独立、显式启用的 `theme/modern`：亮暗、主题色、系统字体及完整菜单阴影规则。
6. [PopoverMenu 外观](DesignPopoverMenuAppearance.md)：消费样式，接入透明外壳与统一菜单内容。

不增加另一套 Theme 系统；nil custom StyleSheet 保留最小 GUI 兜底，不默认启用
Modern。文本编辑重设计不在本轮范围。六步之后再推进 Overlay。

第 1 步代码已完成：Software／GLX 彩色阴影像素测试及全仓库测试通过，Windows
Direct2D／OpenGL 与 macOS 平台测试目标编译通过；Windows／macOS 运行验收待执行。
第 2 步代码已完成：Widget.StyleChanged 延迟合并派发，Label／TextInput、ListView
和菜单自然尺寸缓存已接入；包括 Scrollable 绕过 Measure 的入口、Popover 资源重建
及局部样式名变更。稳定帧继续复用 TextLayout 和 Painter 图像缓存。
第 3 步代码已完成：自绘边框和窗口按钮不再读取应用装饰规则或 Button 样式，
私有亮暗配色跟随窗口背景；多倍缩放 Software 像素／几何测试、全仓库测试及
Windows／macOS GUI 交叉编译通过，三平台装饰和窗口交互真机验收待执行。
第 4 步代码已完成：GUI Settings 初始化快照，轮询随 Run 开始／结束；UI 内部唯一
监听自动合并重建，退出／失败断开并阻止过期更新，相同样式表继续去重。
全仓库测试、Settings 专项竞态检查及 Windows／macOS GUI／UI 交叉编译通过，
系统设置切换真机验收待执行。
第 5 步代码已完成：独立 theme/modern 提供亮暗、主题色、字体及完整菜单阴影规则，
Primary 已按后续反馈改为主题色实底与显式 PrimaryContent，默认无普通描边；
仅 Primary 的键盘焦点按最新请求使用亮色深灰 #414147／暗色浅灰 #D8D8DE，
不按主题实底或内容前景选择，鼠标焦点不变，沿用既有 2 DIP 贴边绘制；
回归验证模式对应的固定颜色，不承诺与任意 Primary 实底的对比目标；
不扩展样式字段或公共绘制，其他按钮不变，demo 导航选中保持中性；
配色、焦点及内容归属契约见 [Modern 预设](DesignModernPresets.md)，本轮桌面验收独立记录。
Button／MenuButton 按体验反馈默认关闭焦点边框，保留样式启用能力；HeaderBar 最小高度收紧至 40 DIP，
Linux 控制图标缩至 8 DIP、点击区域不变。全仓库测试、250 组配色对比度回归、
Windows／macOS GUI 和 Modern 交叉编译通过；`example/modern` 供三平台外观验收。
第 6 步代码已完成：PopoverMenu 消费菜单样式；透明圆角外壳、阴影占位、稳定锚点、
菜单行 TextLayout、输入关闭和透明创建失败降级均已接入。模型／文字资源生命周期、
Software 多倍缩放像素和 Pango 状态换色测试通过；全仓库测试与 Windows／macOS
GUI、macOS 平台测试目标编译通过。`example/modern` 已增加普通与长菜单入口。
三平台菜单外观验收独立推进；P1.3 已按后续确认的普通容器方案实现 GUI Overlay。

### P1.3 普通 Overlay 容器与窗口内覆盖内容

从原 P3 提前，后续设计收敛为普通 Widget 容器，详见 [Overlay 设计](DesignOverlay.md)。
Window 不管理特殊图层；Overlay 可放在窗口根部、局部内容区，也可以嵌套。

- GUI 基础已实现：SetChild、AddOverlay／RemoveOverlay、逐节点 Fill 开关和位置查询信号。
- 主 Child 决定期望尺寸；覆盖节点默认按内容测量，Fill 使用父级 Tight 约束；对齐由使用者计算。
- 复用普通控件树的绘制、裁剪、命中、焦点及语义快照，不增加通用穿透属性。
- 查找栏按内容尺寸浮动；确认层铺满，内部面板居中。Dialog 行为和 Toast 管理由独立组件负责。
- Menu 和 ComboBox 下拉继续使用原生 Popover，不自动切换承载方式。
- UI 薄适配已实现：OverlayItem 链式 Fill／OnPosition，主节点与覆盖列表独立协调；复用 GUI 信号与语义树，不添加包装 Widget。
- UI 协调扩展统一为 Bin／Container 挂载目标；Overlay 使用普通适配器，ListView 行复用同一生命周期机制，Root 不按具体控件类型分支。
- 不包含窗口边缘退避或焦点模态，后续独立推进。不依赖窗口透明或 Linux 完整 CSD。
- 已补齐布局、事件分发、嵌套确认层、信号、生命周期和绘制裁剪测试；应用场景真机体验待完成。

原生 Popup 圆角、阴影集成仍为按需后续能力；三端透明表面已单独实现。旧
[Popup 外观方案](DesignPopupAppearance.md) 已归档，不再作为当前实施要求。

### P1.4 样式驱动 Icon 与后续静态 SVG

首个绘制型 Icon 切片已实现，详见 [Icon 设计](DesignIcon.md)：

- `gui.Icon` 接受公开 IconSource.Image 接口，传入所需物理像素尺寸与自身样式前景，等比居中显示最终图片；默认 16 DIP。固定多尺寸图片、SVG、自定义 Software 离屏绘制统一接入。
- `ui.Icon` 为薄适配，支持 Size、Source 和普通 Style；不改 Image，不增加资源注册系统。
- UI 每次协调直接设置 Source，不推断相等性；GUI 重新取图后复用相同不可变图片的原生资源，普通重绘保留请求缓存。
- Tint 已移除 RGBA/NRGBA 逐像素分配；真实 SVG 示例与基准已补齐。连续弧线解析已在自有 oksvg fork 修复并提交上游 PR，GOUI 固定直接依赖 fork；Brain/People 离屏独立对照通过，三平台窗口验收仍单独跟踪，见 [SVG 设计](DesignSVG.md)。
- 创建时验证：icon.NewImage 固定图片集合，svg.Parse 完成格式检查；Image 只提供最终图片，不返回 error。缺失结果显示占位符，不发送渲染错误信号。
- 具体资源属于 icon.Image、icon/svg、icon/draw；资源着色共用 icon/internal/pixels，gui/ui 不依赖具体实现，graphics 不承载资源工具。
- GUI 兜底与 Modern 正文色、AccentIcon 主题色规则；不向子控件继承父级颜色。
- 包内布局、样式、UI 协调、语义与 Software 像素测试；窗口验证入口为
  `tests/window/ui/icon`，三平台视觉验收待执行。
- 普通画刷重复预乘已按 [绘制颜色契约](DesignGraphicsColor.md) 统一修复三后端及文字颜色入口，不由 Icon 补偿；Windows/macOS 运行验收待执行。

受限 SVG 切片已实现，见 [SVG 设计](DesignSVG.md)：可选 icon/svg 包，默认前景着色、
WithOriginalColors 保留原色；SVG Source 缓存原始像素与着色结果，Icon 持有最终图片/原生缓存，替换资源在
End 后释放。全仓/race/像素测试和 Windows/macOS 交叉编译通过；窗口验收待执行
（`tests/window/ui/svgicon`）。

以下原始目标是后续按需求扩展的范围，不是首版已经支持的完整列表：

- 支持 path、基本图形、group、transform、viewBox、fill/stroke、dash、线性与
  径向渐变、opacity、clip、mask、use 和可替换颜色槽。
- 排除 text、script、animation、filter、外部资源和浏览器 CSS。
- 解析留在可选资源适配包，`gui` 负责主题颜色解析和资源生命周期，`ui` 只提供声明式值适配。

SVG 后续退出条件：内置图标能随亮暗与主题色切换，三个绘制后端在规定 profile 内结果
一致，unsupported 内容返回明确错误。

## 5. P2 · Window 与 Popup 现代样式

### 浮层样式

- 窗口内查找栏、确认面板、Toast 等普通 Widget 使用 GUI 样式；Overlay 容器本身不定义背景或遮罩。
- Menu 和 ComboBox 下拉保留原生 Popover 承载及其样式驱动外观，不迁移到 Overlay。
- 确认组件自行处理遮罩点击和结果信号；完整键盘模态、焦点恢复等另行设计。
  不把原生 Popup 的“owner 收到的鼠标必在浮层外”机制用于窗口内组件。

### Window

GUI 接入已按 [主动 Chrome 信号服务](DesignGUIWindowChrome.md) 实现：Window.Chrome()
返回非 nil 服务，所有标题栏协作使用通知/查询信号。Window 唯一拥有 Controls，内容保持完整客户区；
HeaderBar 为单 child 容器，可有多个，自动避让 Chrome 报告的按钮区域。
ChromeControls 只查询已布局顶部区域的 Height，ChromeInfo 的几何信息只保留 ControlsBounds；没有窗口级 TitleBarHeight 配置。自绘 Controls 是窗口独立的内部控件树，
不插入应用根节点，仍使用统一 EventDispatcher。Linux Integrated 在原生不支持时采用 GUI 自绘，
所需透明能力缺失时才尝试 Native；显式透明请求不降级。默认仍为 Native。
GUI 不再公开 HitTest/BeginMove/BeginResize 转发。以下平台能力说明不代表 GUI 公共接口；
HeaderBar 已提供 UI 单 child 绑定及 Captions 按 ID 扩展；GUI 根据实际命中路径的焦点能力和控制器类型作保守自动拖动判定，特殊区域通过 ConnectDragRegion 覆盖，不增加 Widget/EventController 元信息。
上一版 WindowChrome 已通过用户三平台验证；本轮 Controls 归属/避让及 HeaderBar 自动拖动变更需重新真机回归。

- 系统窗口装饰继续作为默认，保证兼容性和平台一致性。
- 一体化标题栏是 opt-in，而不是强制伪造统一窗口按钮。
- `WindowChrome` 协调窗口拥有的 Controls 并报告原生/自绘占用区域；HeaderBar 自动留位，不建立通用障碍避让系统或隐藏根控件。
- 原生按钮归平台窗口，GUI 仅通过专用适配器协调客户区内的局部占用；普通 Widget 不读取 FrameMetrics。
- `Window` 保留跨平台宿主共同特征，`DesktopWindow` 增加最小尺寸、互斥 `State/RequestState` 与 `SetHitTest`；不把桌面能力强加给未来移动端，不提供通用 Restore 或按钮权限掩码。
- `ControlsRect()` 查询客户区 DIP 下的原生按钮占位，不增加专用通知；GUI 在布局及已有尺寸/状态变化时重查。
- platform 已增加 `SetControlsPosition(*geometry.Point)`：macOS Integrated 按客户区 DIP 放置原生按钮组，nil 恢复默认，不改变按钮所有权、大小或间距；Windows/X11 明确不支持。macOS 内容视图关闭隐式拖窗，客户区拖动仅由 `BeginMove` 发起；新输入/定位行为的真机验证以及 GUI 原生按钮布局适配仍待完成。
- 新合同迁移与 Windows/macOS Integrated 平台实现已完成，详见 [Window Chrome](DesignWindowChrome.md)、[Desktop Window](DesignWindowManagement.md) 和 [Hit Test](DesignWindowHitTest.md)。HeaderBar/WindowControls 的 GUI 信号接入已实现并由用户确认三平台验证。Linux 平台仍不报告 Integrated；GUI 现已通过透明 None 表面实现无阴影的圆角窗口装饰。
- 原生交互按能力拆分：Windows 保留 `SetHitTest`，`BeginMove/BeginResize` 返回不支持；Linux/macOS 的 `SetHitTest` 返回不支持，Linux 提供 EWMH 移动/缩放，macOS 提供原生移动、不模拟客户区缩放。GUI Chrome 内部先尝试 `SetHitTest`，仅在不支持时从正常输入控制器调用交互请求，应用只使用 Chrome 信号。
- Windows/macOS `Integrated` 半自绘保留原生平台机制，Linux 不伪装支持 Integrated。
  Linux 透明表面及无阴影的 GUI Chrome 已完成；圆角背景、内部安全 ClipRect（只裁绘制）、
  实线边框、圆形按钮和 resize 热区
  由 GUI 协作；八方向边缘光标已接入最终区域查询，X11 按系统主题加载并监听配置变化。
  GUI 自绘按钮仍走正常输入、控制器与信号链路，
  原生按钮保留系统所有权；Snap 和各平台交互需真机验收。
- Windows 实现已按 ModernWindow 修订为 `WS_POPUP | WS_THICKFRAME | WS_MAXIMIZEBOX` 与 DWM 顶部 1 物理像素扩展；普通态保留 1px 顶边，最大化直接使用当前显示器工作区。`None/Integrated` 不再补偿创建/最小尺寸，实际客户区以 `SizeEvent` 为准；Windows 10/11 的激活边框、任务栏与多屏最大化仍待 GOUI 真机验收。

退出条件：默认窗口行为不回归；支持一体化的平台保留原生交互，不支持的平台明确报告能力；窗口与
Popup 在亮暗、主题色和缩放下形成一致视觉体系。

## 6. P3 · 高级 GUI 能力

**TableView 首版代码及本机主要场景已实现／验证（2026-10-05）**，见 [设计与验证](DesignTableView.md)。
面向多列记录展示，组合现有 ScrollView／ListView，支持固定表头、稳定行身份、
行虚拟化、列宽拖动、选择、排序请求、菜单及 UI 声明。先以独立回归稳定 ListView
的 RequestLayout／Refresh 与历史尺寸估计契约，不新增 InvalidateMeasurements，
不向核心加入表格特例；widgets／widgets/ui、独立样式和 TableInfo 快照已完成。
全仓、相关 race、无 DISPLAY 表格测试通过；Linux/Xfwm4 Software 1x UI、
OpenGL 1x GUI 与 Software 2x UI 的已操作场景通过。Windows amd64／macOS arm64
交叉编译通过，原生桌面和完整 2x 验收待完成。额外 Linux arm64 编译被既有 libc
SYS_PIPE／SYS_POLL 绑定阻塞，未扩展本切片的平台修改范围。
首版不扩展成电子表格，不引入完整二维模型、编辑事务或横向列虚拟化。
边框／背景补充已完成：内外水平／垂直线、斑马色、自绘内容安全区域均由样式与
表格内部布局处理，不增加 Style 公共字段、Painter 或 UI 协调接口。1x/2x 像素
回归及本机指定 Software/OpenGL 场景通过，目标系统原生验收仍待完成，见设计 §14。

按依赖和实现风险排序：

Overlay 已在 P1.3 实现为普通容器；确认框、Toast 等组合组件及 UI 薄适配后续单独交付。

**TreeView 首版已实现**，详细方案和验收记录见 [设计](DesignTreeView.md)。
复杂控件位于 widgets／widgets/ui，采用通用树模型、稳定节点 ID、展开平铺
及虚拟化，首版包括单／多选、键盘导航、按需加载、自定义行、上下文菜单、样式和 Snapshot。
ListView 补 Refresh／Reveal，实际滚动复用既有 ScrollView；延迟行构建仍以 BuildContext
为唯一入口，借用节点并动态获取当前 Root，完成回调延后到行挂载后，Coordinator 仅负责
移交。本机 Linux/X11/Xfwm4 Software 1x UI、OpenGL 2x GUI 核心交互已验证；
Windows amd64／macOS arm64 仅交叉编译，原生桌面验收待完成。
后续质量调整已区分状态、内容和结构刷新，复用行壳、保留离屏宽度估计；修正信号
重入与子控件右键传播，补齐选中行／展开器交互样式，统一 Reveal 完成通知。
Modern 树选中背景及当前项边框已改为随亮暗模式切换的中性色，不受主题色影响。
ListView／TreeView 命令式模型／Delegate 设置现为显式重新安装；重复声明的模型
引用由 UI 适配器判断并跳过，不在 GUI 用通用相等函数推断。需要跨重建保留树状态
时复用稳定模型指针；非指针模型每次声明按替换处理。
不以树拖放重排、行内重命名或多列树扩大本轮边界；后续继续既定高级控件路线。

应用验证已补 `example/filemanager`：不新增控件 API，使用现有树／列表、公开 Pick、
自定义行及 DragDrop，实现内存同级排序、子树移动和多选校验。本机 Software 1x
主要交互及 OpenGL 1x 排序／亮暗通过；Windows／macOS 仅交叉编译。内容区域方案
不能覆盖私有缩进／展开器，后续可组合行外壳及拖动滚动能力仍待设计，见 TreeView
设计第 10 节；不以此例程宣称完整树拖放或文件管理器已经完成。
例程两侧右键菜单已补齐：复用树菜单接口，列表适配留在例程内；提供信息、目录展开和
选择集合移动，不新增核心 API。菜单逻辑、全仓／race 和本机 Software 1x 交互通过，
Windows／macOS 仍仅交叉编译，见 TreeView 10.5。
例程分割宽度已按 SplitView 受控契约绑定应用状态，选择及重建等操作保留拖动值；
回归、race、全仓及本机 Software／OpenGL 1x 验证通过，见 TreeView 10.6。
例程标题栏已移出正文留白，按既有高度协作恢复 Linux 控制按钮居中；正文保留留白
并充满剩余空间，本机 Software／OpenGL 1x 及回归验证通过，见 TreeView 10.7。

1. **SplitView**：基础实现已完成，见 [设计](DesignSplitView.md)。代码位于 `widgets`／
   `widgets/ui`：两区嵌套、默认按内容建立比例、固定一侧／显式比例、约束及尺寸恢复、
   9 DIP 命中区／1 DIP 中性色线、独立可覆盖的悬停／拖动样式、既有拖动竞争、
   焦点后的键盘调整与 Snapshot 范围；焦点不保持指针高亮。
   本机 Software 1x UI 和 OpenGL 2x GUI 核心交互通过；Windows/macOS 仅交叉编译，
   桌面验收待完成；不扩展平台能力、Docking 或全局键盘导航。
2. **TextView**：纯文本多行编辑，包含选择、Clipboard、IME、滚动、undo/redo；
   按 [文本编辑设计](DesignTextEditing.md) 先实现 TextModel 和 TextView，再将经过验证的
   编辑逻辑复用到 TextInput。共享模型、独立视图，以 Cluster 为交互最小单元，
   ScrollView 外包；目标为 1～10 MB 常规多段落文档，首版不做富文本和超长单段落优化。
   首版开发已完成：排版/输入契约、模型、TextView 视口排版、Cluster
   选择/导航、共享模型、剪贴板、撤销、IME 预编辑投影与计时器交互已接入；
   TextInput 已复用私有编辑核心，UI 与 modern 样式已接入；窗口验证以本机为主，
   Windows/macOS 原生离屏由 remote-shell 验证，其他平台窗口用例由用户验收，
   不再因远端桌面访问条件阻塞后续开发。RTL 选区视觉收起和 Pango 离屏编辑
   回归已补齐，CoreText 度量及接入单行/多行编辑器的 RTL/组合字符离屏回归已在
   macOS 15.7.7 实际运行通过；Windows 10 22H2 的 DirectWrite 度量、编辑回归、
   1x/2x 文字预乘透明度及 win32 字符组装/提交来源测试也已实际通过。
   resize 锚点已统一到正文 clip 顶部，消除小数行高偏移振荡；三平台真实排版的
   连续重排回归通过，窗口用例增加可辨识的编号段落。
   连续宽度往返现保留原逻辑字符；已修正 Pango 栅格化时意外重新换行，以及
   1.50.6 复制游标导致的跨字体 run 度量漂移；包内像素和度量回归覆盖，
   本地 Software 2x 编号窗口已复验宽度往返。Pango 范围样式覆盖已改用原生
   AttrList.Change，补齐嵌套/交叉覆盖、透明清除和字体/装饰线恢复回归。
   OpenGL（llvmpipe）2x 编号窗口宽度往返已复验；修正远距离导航与浏览锚点
   的优先级及小数范围请求收敛，三平台原生离屏、Linux 两后端末尾导航通过。
   UI OpenGL 2x 已补重建/主题状态保持、10 MiB 全文 Clipboard 及 Cluster/RTL
   选择删除撤销验证；普通文字未提交已定位为隔离桌面继承了缺失服务的
   @im=fcitx，@im=none 本地输入法的真实键入/Submit 对照通过，生产逻辑未变。
   UI Software 1x 已补法语 AZERTY、死键、AltGr 及运行中切换 US international
   的字符提交核对；单行与 1 MiB 共享多行均经原生输入验证。
   两平台窗口提交/组合/Clipboard/交互及后端矩阵仍待用户验收。验证细节见设计文档，不能把
   Linux Software 1x 阶段验证视为三平台整体验收。Linux OpenGL（llvmpipe）2x
   已补真实大文本剪贴板及 Fcitx 内联组合/撤销验证；root-style 撤销/重做与
   Software 2x 提交/撤销已验证。callback 候选定位要求原生库支持该扩展：
   libX11 1.7.5 拒绝，临时加载 1.8.4 验证跟随，不以 GUI 补偿掩盖限制。
   Software 末页全选空交集裁剪、XIM 失联后失效句柄及预编辑残留已修复；
   UI Software 2x / 10 MiB 的 Clipboard 精确核对、重建/主题/只读、IME 以及
   原生组合字符/emoji/RTL 选择删除撤销已验证；组合期间关闭窗口正常退出。
   libX11 同步调用期间服务退出导致的 Reset 阻塞仍是已知限制，
   后端和三平台完整矩阵仍待用户验收，不计作已通过。

每个控件必须同时完成 Measure/Arrange、输入、焦点、信号、生命周期、样式和语义
快照，再增加对应 `ui` View。

## 7. P4 · 桌面系统集成

窗口初始居中采用 GUI `WindowOptions.Center` 与 UI `Center(bool)`；仅按初始 Size
在隐藏创建阶段定位一次，不增加 owner、显示器模型或预布局机制。实现与包内、独立
窗口用例已补齐，本机及远端 Win10/macOS 的三种 Chrome、指定应用 1x/2x 与定位保持
验收通过；Xfwm4 未指定初始位置的 Hide/Show 既有重新定位已记录，不扩大修复范围。
真实系统 DPI／多屏等缺口见 [窗口契约](DesignWindowManagement.md#explicit-initial-centering)。

构建打包辅助工具独立于运行时，首个切片为 `cmd/goui` 的 TOML、原生资源生成、
二进制构建和 macOS app / Linux 安装目录树，见 [构建打包](DesignBuildPackaging.md)。
DEB/RPM、签名和权限不阻塞本切片，不包含 AppImage 或系统库复制。
命令代码、资源／构建／打包逻辑测试及本机 Linux 启动切片已完成；Windows/macOS
原生构建、图标、窗口操作已验收。macOS 路径别名的测试误判改为实际目录身份判断，
原生旧失败复现与修正后命令包测试／race／vet 通过，不修改生产实现。
Linux 桌面安装集成仍待验收，Linux arm64 的既有 libc 编译问题待单独修复。

### 托盘图标

- 每个 Application 最多一个托盘项。
- 首版只有图标、可访问名称和点击信号，不包含原生上下文菜单。
- 图标生命周期属于 Application；退出时确定性释放。

### 通知

- 只支持应用进程运行期间发出的本地通知。
- 不包含冷启动激活、计划通知、推送或跨进程持久队列。
- 平台层只报告能力和发送结果；GUI 决定 unsupported fallback。
- Windows 仅在系统自带 API 能够按项目现有风格动态探测、动态加载时实现；否则
  明确不支持，不强制依赖 Windows App SDK，也不降级为旧托盘气泡。

系统集成与核心 Widget 不互相阻塞，但在 SVG 图标和 Application 生命周期稳定后
实施可减少重复资源方案。

## 8. 依赖与推荐顺序

DevServer 的独立补全切片见 [DevAutomation](DesignDevAutomation.md)：dev 管理短期 ref，
通过公开 Window / Popover 输入入口提交事件及已确认文本；不重定义 Snapshot，
不增加内核自动化注册表、截图或无障碍系统。实现、全仓及 race、本机 Software/OpenGL
1x/2x 的协议窗口验收已完成；Windows/macOS 仅编译通过，原生运行及真实 IME 组合验收待执行。

```text
BoxShadow 样式 → 样式钩子 → 窗口装饰隔离 → UI Settings → Modern → PopoverMenu
Widget Painter + 上述样式基础 ─── 普通 Overlay ─── 查找栏 / Dialog / Toast 组合
Linux 透明 + 圆角背景 / 内部 ClipRect ─── Linux GUI Integrated（无阴影，已实现）
Modern StyleSheet + 静态 SVG ──────── 现代控件
DragEventController ──────────── SplitView
TextModel ───── TextView ──────── TextInput 复用
SVG + Application 生命周期 ───── Tray / Notification
```

当前 Linux GUI Integrated 无阴影切片已实现；下一主线按 P1.2 的六步顺序实施。
GUI Overlay 基础已实现；随后按需增加 UI 薄适配和窗口内组合组件。
Menu 与 ComboBox 下拉继续使用原生 Popover。
Overlay 不以透明呈现作为运行前提；指定窗口输入区域不再列入当前路线。
SVG 按控件需要安排；SplitView、TextView 分别独立交付，P4 按真实应用
需求安排。原生 Popup 圆角/阴影集成不阻塞当前主线。

局部重绘、场景图、完整响应式系统、富文本和通用手势仍是候选方向，
只有真实需求或性能数据证明收益后再进入正式切片。

</details>
