# Plan

本篇维护实现与验收记录；未来功能统一在 [Roadmap](Roadmap.md) 分类，不从历史
未勾选项推定当前缺失能力。原生验收缺口、已知限制与未实现功能是三种不同状态。

## Linux arm64 适配规划（2026-10-15）

- [x] 编写 [PlanLinuxArm64](PlanLinuxArm64.md)，明确沿用 Linux X11 后端、
  no-cgo、glibc 和 Software／OpenGL 边界，不扩展 GUI／UI API。
- [x] 前置检查复现全仓 arm64 构建被 libc `SYS_PIPE`／`SYS_POLL` 阻塞；
  Xlib／GLX／Cairo／Pango／Fontconfig／GLib／GTK 绑定及两绘图包局部交叉编译通过。
  这些不是 ARM64 运行验收，也不表示修复两处调用即完成全部适配。
- [x] 本轮文档链接／围栏／空白检查通过；Linux amd64 隔离工作区快照／桌面中
  全仓非缓存测试 2501 通过、15 项既有跳过、0 失败，不计为 ARM64 验收。
- [ ] 搭建本机 QEMU 完整系统模拟的 Ubuntu 24.04 arm64／X11／VNC 环境。
- [ ] 完成 Pipe2／Poll 最小适配、ABI／离屏回归和客体 Software／OpenGL 桌面验收。
- [ ] 本机最终回归通过后同步支持说明；实体 GPU／真实显示硬件留作后续补充。
- 本轮只写设计与计划，不搭建环境、不修改代码、不提交或更新 Release。

## TextLayout 富文本基础能力补全（2026-10-14）

- [x] 按 [Typography 设计](DesignTypography.md) 完成 CF callbacks 地址与成员生命周期、
  三端字体注册、FontInfo 标准枚举／完整范围替换、Pango run 删除线和三端行内占位。
  诊断、阶段及具体环境的验证证据保留在 [PlanTextLayout](PlanTextLayout.md)。
- [x] 旧 macOS traits 原生崩溃、旧 Pango 混合字号删除线和 macOS 纯空白丢失度量
  均有复现与回归；补 DirectWrite bidiLevel UINT32、COM／delegate／Pango ABI 验证。
- [x] 本机同源隔离桌面全仓非缓存：2501 通过／15 既有跳过／0 失败；相关 race
  通过。Windows amd64／macOS arm64 全仓构建与原生离屏回归通过，具体范围见实施记录。
- [x] Linux arm64 的受影响排版／绑定包构建通过；全仓被既有 libc SYS_PIPE／SYS_POLL
  引用阻断，不计为通过，不扩大修改范围。
- [x] 2026-10-15 macOS 常量加载收敛为 `Const[T]` 的可选 `PVar`／`PAddr`，移除
  `ConstAddress`；加载器／CF 集合原生回归各 10 轮、CoreText 3 轮通过。
  本机隔离全仓 2501 通过／15 既有跳过／0 失败，macOS 全仓构建通过，详见实施记录。
- [x] 占位接口命名收敛为 `SetPlaceholder`／`PlaceholderMetrics`；原生 DirectWrite
  命名及用户调整的文件组织不变。暂存树干净快照全仓 2162 通过／10 既有跳过／
  0 失败，相关 race、Windows／macOS 构建及两端原生回归各 3 轮通过；详见实施记录。
- [x] Typography 代码按用户授权提交为 `d67e89d`；绑定由用户另行提交。本轮将长期
  设计拆分至 DesignTypography，PlanTextLayout 只保留实施／验收记录，不改代码。
- [x] 文档拆分后的链接／锚点、围栏、接口名称及空白检查通过；隔离工作区快照全仓
  2501 通过／15 既有跳过／0 失败。本轮不重跑远端或扩大桌面验收，文档按用户授权提交。
- [ ] 新增能力的真实桌面／系统 DPI／硬件 GPU 验收；本轮不推送或修改 Release。

## 使用方 AI 应用开发 Skill

- [x] 补充 [自定义 Widget／View](CustomWidgetView.md) 的六种方式、外部包示例，
  同步技能独立参考、安装清单和 README；只改文档，不改 gui／ui 生产实现。
- [x] 按 normalizeView 修正 DesignUI 中纯 Build 修饰器传播的旧评估表述，
  明确外层／内层身份、Self、同节点 State 槽、子树协调与销毁职责。
- [x] 本轮从教程直接提取 7 个 Go 代码块到临时外部模块；CGO_ENABLED=0 的
  Linux amd64、Windows amd64、macOS arm64 编译与接口断言通过。
- [x] 外部模块 3 项非缓存逻辑验证通过：组合／扩展 Widget，Surface 父链、
  min／max／父约束、局部 Arrange 和 Snapshot，以及 View Self／委托／子树请求。
- [x] 本机 X11、Software、指定 1x：短暂窗口通过 Window.DispatchEvent 验证
  Counter 最新回调、只连接一次和 Unmount 断开；实际 ui.Run 协调另验证同 ID 复用、
  换 ID 替换、纯 Build 外壳 ID 不传播、Surface 父链与 UnitLabel／Counter 更新。
- 初次临时回调替身验证 `go test -count=1 -v ./custom` 失败（old=0/new=0）：
  未挂载 Widget 的 liveRoot 校验拒绝输入，不能当作示例故障。没有修改框架或
  弱化回调断言，原诊断保留在临时目录；原断言改由真实宿主独立程序验证并通过。
- [x] 既有 UI 构造／Root 相关用例非缓存执行通过；go test ./... 通过（包含缓存），
  技能格式、91 份 Markdown 的本地链接／锚点及围栏检查通过。
- 本轮没有框架源码／测试修改，临时样例与验证程序只放仓库外；已退出两次短暂
  本机窗口。旧 ui_test 例程为原有未跟踪文件，仅作当前工作区参考；本文示例不依赖
  它发布。未执行其他绘图后端、远端桌面或真实多工具安装，不计为三端原生验收。
- [x] 已确认：skills/goui-app 为可独立安装的通用技能，skills/INSTALL.md 为 AI
  执行的统一安装入口；不增加 Agent 适配器、不改使用方 AGENTS、不自动安装依赖。
- [x] 编写入口与应用／构建调试参考，覆盖版本核对、公开 API、更新与生命周期、
  样式／DIP、打包和验证边界；Star 克制提醒、Issue／PR 证据与授权规则。
- [x] 中英文 README 增加一句话安装入口，安装来源、范围、同名保护、检查和
  加载状态明确区分；设计契约即 [安装说明](../skills/INSTALL.md) 和
  [Skill](../skills/goui-app/SKILL.md)，不另建重复设计文档。
- [x] 本机 Linux amd64／Go 1.25.8：quick_validate.py 技能格式检查通过；89 份
  Markdown 的 450 个本地链接／锚点、围栏及新增文件空白检查通过。
- [x] 临时独立模块直接提取两个完整 Go 示例，replace 指向本轮源码；CGO_ENABLED=0
  的 Linux amd64、Windows amd64、macOS arm64 六组构建通过，不计为原生运行验收。
- [x] go test ./... 通过（包含缓存），git diff --check 通过；保留既有 opt-in
  条件和 PNG 原目录输出测试，不把全仓结果称为纯无头或无桌面副作用。
- 不在本机 AI 工具目录安装、不执行真实多工具安装或远端桌面；修改阶段不自动提交。
  后续按用户明确指令提交相关文档，不推送；发布后再验证 main 上的安装入口，
  不将工作区链接存在当作线上已可安装。

## 2026-10-14 文档复核与同步

- [x] 修正登记差异中的旧 API、线程／资源归属、布局与颜色契约；只修改文档。
- [x] 现行专题文档同步 Basic、帧驱动、局部 ID 协调、文件访问、Popover 定位、
  Linux Integrated 安全布局和 Windows 10/11 版本边界。
- [x] 已确认未实现方案与待重新设计／评估方向集中到 Roadmap；已撤销提案只作历史。
- [x] 历史测试、后端／缩放、失败和未覆盖证据保留，不追认成本轮验收。
- [x] 本轮 84 份 Markdown 的本地链接／锚点与代码围栏检查、README SHA-256
  范围保护、git diff --check、go test ./... 通过（包含缓存）；结果见 [文档审查记录](DesignDocumentAudit.md#本轮验证)。
- 本轮不修改源码、测试、README、AGENTS，不执行远端桌面；修改阶段不自动提交，
  随后按用户明确指令将本轮 52 份文档归为 7 个主题提交，不推送。

## 验收缺口与限制的阅读方式

专题设计中最新的带日期结果覆盖对应的旧阶段任务，不覆盖未操作的平台／场景。
例如 Switch／ProgressBar 的 Timer 时代测试保留，但当前驱动是 ConnectFrame；
早期 Position 外框观察与隐藏布局记录已被客户区定位及 TabTransfer §31 替代。
Windows 11 圆角、真实系统／混合 DPI、其他 WM、文本编辑／IME 等遗漏组合仍按
具体设计逐项核对，不能把编译、旧截图或用户“基本通过”扩大为完整矩阵通过。

X11 Clipboard 无 INCR、Popover 无原生 IME、XIM 同步等待的服务退出限制分别见
Clipboard／DesignIME。已有 Xfwm4 未设置初始位置时 Hide/Show 重新定位与 Linux
arm64 构建限制保留在原记录中；本轮未修改实现或复验这些原生问题。

## 原切片记录

<details>
<summary>展开既有实现／验收日志（旧状态不自动成为当前待办）</summary>

以下记录按当时切片保留。以最新专题契约和日期明确的验证范围为准；无日期的
早期计划表已完成项作同步，明确撤销的提案标为历史，不删除有效失败证据。

## 窗口初始居中选项

- [x] Demo 主窗口启用 Center(true)，保留辅助窗与标签拖出定位；现有声明回归及 Demo 设计说明同步。Demo/resource 非缓存测试和全仓测试（部分缓存）通过；按用户要求不重复窗口居中验收。已提交 de44eda，未推送。

- [x] 已确认设计：GUI WindowOptions.Center bool / UI Center(bool)，归一化初始 Size + 既有工作区/定位；隐藏创建时一次执行，显式 SetPosition 可覆盖，不改 Show/平台接口，不增加 owner、启动枚举或预布局。
- [x] GUI/UI 实现、包内尺寸/缺失事件/回退/错误清理/声明式创建约束回归，以及中文独立窗口例程，见 [DesignWindowManagement](DesignWindowManagement.md#explicit-initial-centering)。SkipTaskbar 保留 bool 参数。
- [x] 最终本机全仓 2492 通过/15 既有跳过/0 失败，无 DISPLAY GUI/UI、race、限定 vet、Windows/macOS 全仓编译；Linux 两后端偏好 × 三 Chrome × 应用 1x/2x 与远端 Win10 Session 1/macOS 的对应窗口定位、显式覆盖、重建/重现及输入验证通过。实际后端 Name 未独立读取，应用缩放不代替真实系统 DPI；完整范围见设计验证记录。代码、包内测试和窗口例程已提交 650f293，未推送；原有未跟踪设计/路线文档及测试索引保留工作区。
- [ ] 本机 Xfwm4 Center=false 的 Hide/Show 重新定位已在旧 HEAD 90bdb83 复现，保留失败断言、记录为既有差异，不在本次扩展平台修改。Windows11、多屏/混合 DPI、其他 WM 和超大窗口未执行。

## macOS 普通窗口默认位置

- [x] 已确认设计：普通窗口原生尺寸初始化后、隐藏状态调用已有 Center 一次；不改 Show、Popup、显式定位、公共 API 或其他平台。
- [x] 原生回归程序在旧代码复现左下角；macOS 15.7.7 arm64 六组 Chrome/应用 1x/2x 几何、OpenGL Native/Integrated 与 Software None 1x 实际输入/拖动、既有 GUI 三模式定位验收通过。
- [x] 本机最终全仓 2481 通过/15 既有跳过/0 失败，限定 race/vet、Windows/macOS 全仓与 Cocoa 测试目标编译通过；验证范围及未覆盖项见 DesignWindowManagement。代码与回归用例已提交 90bdb83，未推送；原有未跟踪设计/路线文档及测试索引保留工作区。

## Demo 辅助窗口示例

- [x] 增加 Native + SkipTaskbar 与 None + Transparent + root BorderBox 两个入口；复用定位、关闭、主题和语言生命周期，保持其他窗口背景隔离。
- [x] BorderBox 显式使用 modern.Card，不改默认无外观；正文可滚动、标题保留，新增说明覆盖八语言，窗口输入不随语言/主题重建。
- [x] 示例配置/控件树、八语言、全仓/race/目标编译及本机 X11 Software/OpenGL 1x 窗口验证；全仓 2481 通过、15 既有跳过、0 失败，详情及验证缺口见 DesignDemo。按用户要求不执行远端桌面；已提交 33e8a84，未推送。

## BorderBox 单子边框容器

- [x] [DesignBorderBox](DesignBorderBox.md)：默认无外观，6 DIP 安全留白，None root 自动命中，其他场景仅装饰布局。
- [x] GUI/UI 薄绑定、Basic/Modern 透明默认规则、包内回归与中文窗口验收程序。
- [x] 本机相关回归/race/vet、两目标全仓编译、Software/OpenGL 指定 1x/2x 桌面与远端 Windows Direct2D/macOS OpenGL 1x 输入/缩放及三模式 -auto 验收；详见 DesignBorderBox 的范围和验证缺口。
- [x] macOS None 既有 isZoomed 状态误报复现并修正，Normal/缩放真实回归通过；不在 GUI 绕过错误状态，不新增原生能力。
- [x] 最终全仓 2474 通过 / 15 既有跳过 / 0 失败；相关 race、限定 vet 与跨平台编译通过。中途未改 Pango 删除线位图偶发失败及单独 count=10 通过的记录保留于 DesignBorderBox，未放宽断言。提交已拆分为 bc181aa（macOS 状态修复）与 971ecb9（BorderBox、相关测试与独立设计），最终文件树与原 0a9e82a 完全一致，go test ./... 再次通过；未推送。

## 普通窗口 SkipTaskbar 创建偏好

- [x] 已确认设计写入 [DesignWindowTaskbar](DesignWindowTaskbar.md)：不引入 Popup 类别、不增加 Options.Position、不改造 Popover，继续 Show 前 SetPosition。
- [x] platform/gui/UI 创建选项贯通与 Chrome 回退透传；Windows TOOLWINDOW 保留激活、Native 客户区大小；X11 初始 EWMH 提示保留 WM 管理；macOS 不改应用 Dock policy。
- [x] 包内回归与独立中文 tests/window/gui/taskbar 验收例程已编写。
- [x] 本机最终全仓 2466 通过/15 既有跳过/0 失败，相关 race、受影响目标及 Windows/macOS 全仓编译通过；完整 vet 的既有告警与限定检查结果记录于 DesignWindowTaskbar。
- [x] Xfwm4、Software/OpenGL 的指定 1x/2x 原生属性与定位通过；本机真实输入/移动/缩放、Windows 10 Direct2D 1x 任务栏/Alt+Tab/输入/定位、macOS 15.7.7 OpenGL 1x Dock/输入/定位及远端新增 GUI/UI 原生测试通过。远端三种 Chrome 的指定自动断言通过；资源已清理，代码/用例/对应设计已提交为 1bac615，未推送。
- [ ] Windows 11、其他 WM/Wayland、混合系统 DPI 及远端其他后端未验收；不把应用缩放或交叉编译当作这些场景通过。

## Basic 完整兜底与 Modern 一行应用

- [x] 已确认设计写入 [DesignThemeFallback](DesignThemeFallback.md)：UI／GUI 不依赖具体 Modern；Basic 作为默认完整外观，自定义样式表仍整体替换。
- [x] GUI／widgets 兜底值迁入 theme/basic，保留 DefaultStyleRules/Sheet 与 widgets/style.Rules 兼容包装；禁用／焦点／独立文本规则按现行值迁移，不采用旧 Basic 副本的缺项。
- [x] modern.FromSettings 使用消费方设置接口，兼容 gui.Settings／ui.Settings；不保存源对象、访问全局 App、连接信号或自动安装主题。
- [x] 默认样式／Software 1x/2x 回归在旧实现复现透明指示器和缺失字体，迁移后通过；补 Basic 独立颜色/几何、恢复 nil、Modern 设置复制/隔离与兼容测试。
- [x] 新增 tests/window/ui/themes 中文窗口验收程序；原 tests/temp 不修改，example/modern 系统默认模式使用新入口。
- [x] 完整表头默认值暴露 TableView 的 Inf 扣减缺陷，仅有限轴扣减边框／表头；高表头四种约束回归在旧实现失败、修正后通过，保留虚拟化上限及单元格完整内容断言。
- [x] Go 1.25.8 Linux amd64：非缓存全仓 2387 通过／15 既有跳过／0 失败，相关 race 通过；Windows amd64／macOS arm64 全仓与相关测试目标编译通过。普通 vet 的既有 composites 警告保留，限定新主题／widgets／例程及关闭该单项后的相关 vet 通过。
- [x] Ubuntu 22.04.5／X11／Xfwm4、Software 与 Mesa llvmpipe OpenGL、应用 1x／2x 四组主题切换、恢复 nil、联动／禁用／Tab／Space／F6 检查通过；原 tests/temp 在 Software 1x／OpenGL 2x 显示正常，未改例程。完整范围见 DesignThemeFallback；未提交，未执行远端桌面。
- [x] Basic 外观方案确认并实现：选中方框／开启开关取消同色描边，Radio 使用同状态实色底与白点，开关采用较柔和固定蓝色；仅 theme/basic 两个生产文件，不改 Modern／Widget／Painter。
- [x] 旧样式复现边缘覆盖超出单次几何预期；新增 Basic 方框／单选／开关边缘回归并沿用既有容差，保留未选中轮廓及 2 DIP 焦点；相关包内测试通过。
- [x] Basic 外观优化：非缓存全仓 2464 通过／15 既有跳过／0 失败、相关 race／限定范围 vet、Windows amd64／macOS arm64 全仓编译通过；本机 Software／OpenGL 各应用 1x／2x 的 Basic 外观、切换／禁用、单选鼠标／键盘、身份与关闭清理通过。范围与自动化重试见 DesignThemeFallback §6.1；未提交。
- [ ] Basic 外观优化的 Windows／macOS 桌面、Direct2D、真实系统 HiDPI 和硬件 GPU 未验收，不以编译替代。

## 桌面启动线程保留

- [x] 在现有 platform.go 的 init 中统一保留启动线程，移除 ui.Run 的重复锁定；不新增生产文件、不创建原生资源、不改变后台投递机制。
- [x] 同步 Platform／GUI／UI 入口注释和 DesignApp／Testing：从 main 创建并运行，Run 返回后不解锁，导入副作用与直接后端调用边界明确。
- [x] 既有 platform/application_test.go 加入真实 main goroutine 的子进程回归；旧实现复现 main 从启动线程迁移。
- [x] Go 1.25.8 Linux amd64：非缓存全仓测试、platform／gui／ui race 通过；本机启动线程回归连续 30 次通过。CGO_ENABLED=0 的 Windows amd64／macOS arm64 全仓构建及 platform 测试目标编译通过。
- [x] 经哈希校验的同源测试目标：Windows 10 19045 x64／macOS 15.7.7 arm64 无窗口线程回归各连续 20 次通过；未启动远端 GUI，不计作真实 AppKit 窗口验收。
- [x] macOS 15.7.7 arm64：OpenGL、应用缩放 1x 的真实 demo 启动、页面切换和原生关闭通过，退出码 0，无主线程异常；用户另行完成验证并确认提交。批量重复启动未执行，不计为独立通过。

## Windows Integrated 版本与客户区尺寸

- [x] 按 ModernWindow 的注册表 Build 查询区分 Windows 10/11，Platform 缓存成功观察；Windows 10 保留 Popup 组合与顶部 1 物理像素，Windows 11 保留 OverlappedWindow 并将顶部非客户区设为 0。
- [x] DWM 扩展、非客户区安装与 FRAMECHANGED 独立于 Style 是否变化；Chrome 查询接受对应实际样式，ControlsRect 仍为空。
- [x] 所有 Chrome 的创建尺寸及最小尺寸均为客户区 DIP；Native 保留标准 AdjustWindowRectExForDpi，Integrated 仅增加自身实际边框；同步公共注释和 DesignWindowChrome。
- [x] Windows 10 桌面发现 Popup 的原生边框为 7 物理像素，而标准 metric 为 8；改为按最终 Style 用 DefWindowProc 计算，随 FRAMECHANGED 设置隐藏窗口最终尺寸，不增加事后搬动或猜测偏移。
- [x] 本机非缓存全仓 2357 通过/15 既有跳过；GUI/UI race；Windows amd64/macOS arm64 全仓构建及测试目标编译通过。既有普通 Windows vet 的 unsafe.Pointer 告警保留，受影响包关闭该单项后的 vet 通过。
- [x] Windows 10 19045 x64 六组无桌面逻辑测试通过；Session 1、系统 DPI 96，Native/None/Integrated 的客户区、最小尺寸、普通/最大化/恢复在渲染 1x/1.25x/2x 通过；Software/Direct2D 1x 的亮暗背景和状态绘制、Software 边缘缩放通过。Session 0 的 Integrated 明确跳过，不计作通过。
- [ ] Windows 10 RDP 下 OpenGL 报 `can not choose useful pixel format`，未完成该后端桌面验收；真实系统 HiDPI、多显示器跨 DPI 未执行。
- [ ] Windows 11 暗色顶部两角、亮暗切换、激活/失活、最大化/恢复、三个绘制后端及真实 DPI 桌面验收。

## README 与发布介绍

- [x] 同步英文首页与中文说明，采用新 Logo 和“用 Go 构建轻量、现代的桌面应用”；突出 no-cgo、无需 GCC、单二进制交付，同时明确运行库和 macOS 包装边界。
- [x] 展示已实现能力、现代样式与原生集成、Go 风格和成熟框架参考，替换过时示例及路线；下载、截图、动图预留明确位置，不伪造发布产物。
- [x] CGO_ENABLED=0 且 CC/CXX 指向不存在路径，示例及 cmd/goui 完整 Demo 的 Windows amd64/macOS arm64/Linux amd64 六项构建通过；裁剪后示例 4.46～4.75 MiB、Demo 8.26～8.56 MiB，注明 Go 1.25.8/参数/工作区和未计入资源。
- [x] Linux Ubuntu 22.04.5 用户空间、Xvnc 1.12/Xfwm4、Mesa 23.2.1 llvmpipe、Software 1x，空目录运行单文件；每应用三轮、每轮预热 5 秒／采样五次，报告 RSS/PSS 中位数与明确范围，不冒充 Go 堆或跨平台数值。宿主 3.10 内核无 smaps_rollup，诊断脚本改为正确汇总 smaps 后获得有效样本，不修改框架。
- [x] 中英文唯一完整示例与已编译源码一致，所有可见本地链接、锚点、三个展示占位及体积／内存数据校验通过；git diff --check 通过。本机隔离桌面 CGO_ENABLED=0 的非缓存 go test -json ./... -count=1：2343 通过、15 既有跳过、0 失败；保留既有测试的原生窗口和图片输出副作用。
- [x] 保留作者的中文 README 修改，仅替换环境变量 TODO：核对缩放覆盖、各平台后端初始化降级、Linux 光标／缩放兼容变量和可选 gui/dev 开关，补 Bash／PowerShell 示例。章节外文本与英文首页未改；Markdown、本地链接和 Bash 语法检查通过，PowerShell 未原生执行。本机隔离 Xvnc/Xfwm4、CGO_ENABLED=0 全仓非缓存复跑 2343 通过／15 既有跳过／0 失败。
- [x] 中文 README 单张截图说明改为 Windows、macOS、Linux / X11 的三个 img 占位，纵向排列、统一宽度 900，图片路径为 doc/images/demo-{windows,macos,linux}.png；保留作者描述、动图占位和英文首页。标签／相对路径及变更范围检查通过；本机隔离 Xvnc/Xfwm4、CGO_ENABLED=0 全仓非缓存复跑 2343 通过／15 既有跳过／0 失败。真实截图尚未提供，不生成替代图片。
- [ ] 发布时补齐 Demo 下载、校验和与真实截图／动图；远端内存与等价应用对比没有实测前不作数值承诺。

## 文件访问凭据与 macOS 沙箱

- [x] FileAccess、Credential/RestoreAccess、三平台选择结果与明确生命周期，见 [DesignFileAccess](DesignFileAccess.md)。
- [x] GUI/UI 保存创建与操作错误，取消保持空结果/nil error；更新例程和替身。
- [x] macOS 原生书签、实际沙箱检测、cmd/goui Entitlements 和签名。
- [x] 本机/远端 macOS 非缓存全仓、相关 race、Windows 受影响包/目标编译及三平台真实桌面通过；macOS 显式 ad-hoc read-write/read-only 沙箱、容器标准目录和跨启动恢复通过。Windows 全仓既有 Direct2D 渐变断言及 Session 0 窗口超时在修改前 HEAD 复现；Linux arm64 既有 libc 编译阻挡、普通 vet 既有 composites、开发证书 SSH 签名失败如实记录，不当作通过。
- [ ] 稳定证书跨更新、真实 HiDPI 与沙箱资源移动/删除的完整桌面验收；不影响已验收基础切片提交。
- [ ] 提权执行单独后续设计，不混入文件凭据。

## 嵌套滚轮消费

- [x] ScrollView 按请求轴范围消费完整滚轮事件，首尾边界不传给外层；无对应范围继续冒泡。TabBar 实际标签/箭头溢出时消费，空白区保持被动；不修改平台输入、通用控制器或事件分发器。见 [DesignScrollLayout](DesignScrollLayout.md#嵌套滚轮消费2026-10-10)。
- [x] 正常派发路径回归先在旧实现复现，再覆盖横/竖/Shift、双轴、边界、小数/零增量、短/空内容和禁用方向；独立窗口例程补长短内层列表与操作说明。
- [x] 全仓非缓存 2245 测试/子测试通过、14 既有跳过；无 DISPLAY 两包测试和 GUI/widgets/UI race、Windows/macOS 测试及例程编译、本机 Xvnc/Xfwm4 实际 Software/OpenGL 各应用 1x/2x 共 36 次真实滚轮偏移检查通过。普通 vet 既有 composites 警告，其他检查通过，见设计验证记录。
- [ ] Windows/macOS 原生桌面、Direct2D、真实 HiDPI/触控板和其他 WM 未执行。
- [x] 本轮确认保留原生 DnD 的外部目标消费行为及系统窗口位置约束；不新增模拟拖放，不给 SetPosition 或 demo 添加主动工作区退避。平台客观差异不以光标或事后搬窗补偿。

## 短菜单最小宽度

- [x] PopoverMenu 主体默认最小 160 DIP，包含内部留白、排除阴影；长内容自然撑宽，工作区硬约束优先。复用 Widget 最小尺寸，不增加样式、UI 或平台接口，见 [DesignPopoverMenuAppearance §5.1](DesignPopoverMenuAppearance.md#51-布局)。
- [x] 包内回归已在旧实现复现过窄问题，修正后覆盖中英文、长文字、约束、阴影、伸缩、无多余滚动条和菜单输入/语义边界。
- [x] 全仓非缓存最后复跑 2202 通过/14 既有跳过，相关 race、Windows/macOS demo/GUI 测试目标编译及本机 X11/Xfwm4 Software/OpenGL 配置 1x/2x 中文短菜单亮暗/点击、2x 长菜单滚动、指定英/德语言变化通过。普通 vet 既有警告、一次 Pango 偶发失败及复测结果见设计验证记录，不将其隐去或改断言。
- [ ] Windows/macOS 原生桌面、Direct2D、真实 HiDPI 未执行。

## Modern 通用样式预设

- [x] Primary 改为主题色实底与显式 PrimaryContent 对比内容；默认无普通描边，保留 6 DIP 圆角／2 DIP 键盘焦点。键盘焦点按最新请求只随模式选灰色：亮色 #414147、暗色 #D8D8DE；鼠标焦点及普通按钮／Flat 不变，不额外内缩，不扩展样式或公共绘制。demo 当前分类改用私有中性选中表面，示例内容同步。
- [x] 最终范围确认后全仓非缓存 2358 通过／15 条件跳过，Modern/demo race 与 Windows amd64／macOS arm64 全仓编译通过；公共焦点绘制及普通按钮／Flat 的原有规则已恢复，结果见 DesignModernPresets。
- [x] Primary 深浅主题色焦点新版全仓非缓存 2358 通过／15 既有条件跳过，Modern/demo race／vet、Windows amd64／macOS arm64 全仓编译通过；style/gui/widgets 的扩展已撤回，具体记录见 DesignModernPresets。
- [x] 用户认可暗色 Primary 焦点；亮色按确认方案柔化为 25% 混合起点／2:1 三态对比，暗色原有 50%／3:1 不变。独立 RGB 断言在旧代码复现过黑，修正后通过；仅调整 Modern 配色、测试与文档。
- [x] 亮色柔化版全仓非缓存 2358 通过／15 既有条件跳过，Modern/demo race／vet 与 Windows amd64／macOS arm64 全仓编译通过；Modern Linux arm64 包编译通过，arm64 示例被既有 libc SYS_PIPE／SYS_POLL 编译问题阻塞，未扩范围，见 DesignModernPresets。
- [x] 亮色柔化方案仍被用户反馈过深，按确认方案改为固定浅灰／深灰焦点。两种模式复用 Primary 已有前景明暗选择，颜色断言在旧代码失败、修正后通过；250 组组合恢复统一 3:1 焦点目标，不修改普通按钮、Flat、TextInput 或绘制实现。
- [x] Primary 中性色焦点全仓非缓存 2358 通过／15 既有条件跳过，Modern/demo race／vet、Windows amd64／macOS arm64 全仓编译及 Modern Linux arm64 包编译通过；style/gui/widgets 无变动，详见 DesignModernPresets。
- [x] 用户未采用中性色效果，按请求恢复普通 Button 默认焦点规则；250 组组合改为验证默认配色一致，保留独立 RGB、整数混合、2 DIP、应用覆盖、实底／文字和内容隔离回归。不改 GUI、普通按钮、Flat 或 TextInput。
- [x] Primary 默认焦点恢复版全仓非缓存 2358 通过／15 既有条件跳过，Modern/demo race／vet、Windows amd64／macOS arm64 全仓编译及 Modern Linux arm64 包编译通过；style/gui/widgets 无变动，详见 DesignModernPresets。
- [x] 按确认方案仅修改 Primary FocusVisible 为模式对应的深灰／浅灰，独立 RGB 回归先在旧实现失败、修改后通过；250 组组合不依赖主题实底或前景，普通按钮、Flat、TextInput 和绘制实现不变。
- [x] Primary 按模式选择焦点灰色版全仓非缓存 2358 通过／15 既有条件跳过，Modern/demo race／vet、Windows amd64／macOS arm64 全仓编译及 Modern Linux arm64 包编译通过；style/gui/widgets 无变动，见 DesignModernPresets。
- [ ] Primary 按模式选择焦点灰色版由用户体验原生外观；本轮不提交。
- [x] 六个具名预设与 demo 集成：Card/Sidebar 表面、完整 Flat 按钮、Warning/Error/Success 前景，见 [DesignModernPresets](DesignModernPresets.md)。
- [x] 包内回归、全仓 2138 通过/14 既有跳过、相关 race/vet、Windows/macOS demo/modern 编译；本机隔离 X11/Xfwm4 Software 1x/OpenGL 请求配置 2x 的亮暗、语义色、Flat 交互/焦点及 demo 展示通过，具体范围见设计验收记录。
- [ ] Windows/macOS 原生桌面、真实 HiDPI 与额外后端/缩放组合未执行；不将编译或应用缩放覆盖算作原生验收。
- [x] Sidebar 独立亮暗背景调整：不修改 Flat/普通按钮/Primary；背景色、表面层次与正文/次要/语义文字对比度回归已编写。
- [x] 本轮全仓、Modern/demo 非缓存测试、相关 race/vet、Windows/macOS Modern 测试目标编译；本机 X11/Xfwm4 Software 1x 亮暗/切页/悬停/按下和截图对照通过。
- [x] 用户已要求提交 Sidebar 调整，按独立功能提交；未重跑远端/其他后端/真实 HiDPI。
- [x] 补齐 demo 五个文件中八处 Card 样式迁移遗漏，保留同名拖放数据格式；回归在旧实现报告八处错误，修正后通过。
- [x] 全仓 2140 通过/14 既有跳过、相关非缓存/race/vet、Windows/macOS demo 编译；本机 X11/Xfwm4 Software 1x 亮暗面板/Dialog/Toast/标签关闭确认/权重/接收区/透明辅助窗口背景与桌面合成采样通过，独立提交。其他后端/原生远端/真实 HiDPI 未执行，见 DesignModernPresets。
- [x] 明确 Modern 整体视觉原则和类别描边契约；Flat 常态/禁用透明无框，悬停/按下统一普通按钮的背景及 1 DIP 描边，独立键盘焦点与应用覆盖保持不变。
- [x] 全仓 2141 通过/14 既有跳过、相关非缓存/race/vet、Windows/macOS 示例与 Modern 测试目标和 Linux arm64 Modern 测试目标编译；本机 X11/Xfwm4 Software 1x 亮暗按钮/菜单按钮/禁用/Tab 焦点及独立背景、1 像素描边、文字位置采样通过。其他后端/原生远端/分数缩放/真实 HiDPI 未执行，见 DesignModernPresets。

## 多语言资源

- [x] 补齐 demo 默认编辑样例、拖放提示/结果、计时及树/列表/表格展示的八语言资源；编辑后按显式归属/模型 Revision 保留数据与历史，表格重置备注恢复跟随，不改框架 API。
- [x] 旧版英文模式复现五处默认文案未翻译；新增八语言、同值赋值、撤销回原文/重做、备注归属/重置、原始拖放数据和文档关闭记录回归通过，操作步骤见 demo README。
- [x] 本轮全仓非缓存 2218 通过/14 既有跳过、例程无 DISPLAY/race/vet、Windows/macOS demo 编译；本机独立 X11/Xfwm4 Software 1x 八语言指定页面/实际输入/历史/备注/列宽/文本拖放及 OpenGL 配置 2x 指定编辑场景通过，范围见 [DesignDemo](DesignDemo.md)。demo 代码、回归和对应设计说明按用户确认独立提交；本清单保留为本地文档。
- [ ] 本轮 Windows/macOS 原生桌面、Direct2D、真实 IME/混合 DPI、跨应用文件拖放和母语校对未执行。
- [x] 独立 mlang、具名 Getter、任意 BCP 47 标签与变化信号；应用 resource 统一文本/图标入口，见 [DesignLocalization](DesignLocalization.md)。
- [x] UI 自动更新与退出清理；编辑菜单实时翻译、窗口控制/标签语义文本；框架首批简中、英、日、韩、德、法、俄、阿拉伯八语言。
- [x] 包内断言、相关 race、全仓 2129 通过/14 既有跳过、Windows amd64/macOS arm64 全仓和相关测试目标编译、本机 Software/OpenGL 配置 1x/2x 八语言窗口自动检查及指定真实输入通过；完整 vet 既有 composites 警告、后端回退与缩放覆盖边界见设计执行记录。
- [x] En/ZhCN/Ja/Ko/De/Fr/Ru/Ar 便利入口及标签一致性测试。
- [x] Settings.Language 三平台首选界面语言、GUI 默认值及 UI 自动协调；Windows/macOS 原生无头查询和系统偏好对照通过，见 [DesignSystemLanguage](DesignSystemLanguage.md)。
- [x] demo 八语言菜单、一次性使用系统语言、234 条具名资源、稳定页面 ID 与原模型更新；全仓 2135 通过/14 既有跳过、相关 race/vet、Windows/macOS 编译、本机四组配置的状态保留与窗口翻译通过。
- [ ] Windows/macOS demo 原生桌面、真实 IME/混合 DPI 与母语人工校对待后续；Linux arm64 全仓编译被既有 libc SYS_PIPE/SYS_POLL 阻断，未扩展本轮范围。格式器和 RTL 布局镜像仍不在承诺内。

## 综合演示例程

- [x] Demo Logo 永远保留原色：独立解析并只暴露原色 d.logo，移除普通图标集合中的 Logo 单色副本；带图标按钮和标签页统一原色，单色示范改用普通文档 SVG。既有标题栏、动画、横版和平台应用图标保持原色，不改 Icon 或 Painter。
- [x] 新增集合隔离／按钮与标签页绑定回归在旧实现失败；修正后展示尺寸与前景色像素回归、例程无桌面/race/vet、全仓非缓存 2357 通过／15 既有跳过、三目标 CGO_ENABLED=0 构建通过。本机 Software 1x／OpenGL 配置 2x 亮暗、主题色、禁用、标签和图形样例检查通过；2x 动画初态／中间／终态通过。未执行 Windows/macOS 原生桌面及 Direct2D，详见 DesignDemo。
- [x] demo 窗口框架紧凑化：24 DIP Logo / 6 DIP 标题栏留白，标题跟随系统正文字号；侧栏移除详细快捷键说明；状态栏最小 28 DIP / 留白 6 DIP，长消息仍换行并自然增高。不改 Modern 或框架接口，设计见 DesignDemo。
- [x] 紧凑化声明回归在旧声明下失败；调整后全仓非缓存 2343 通过/15 既有跳过，例程无 DISPLAY/race/vet、三目标 cmd/goui 构建通过；本机 Software 1x 亮暗/窄窗/德文/18pt/长结果及标题拖动/双击、OpenGL 配置 2x 亮暗/窄窗/短结果通过。未执行远端原生桌面，范围见 DesignDemo。
- [x] demo 采用 `doc/logo` 的用户设计：方形标志用于标题栏，横版用于图形页；原色展示与单色样例分离，资源嵌入且校验与设计源一致，新增 `goui.toml` 复用构建工具。
- [x] logo 副本一致性/原色像素回归、例程无桌面/race/vet、全仓非缓存 2341 通过/15 跳过；`cmd/goui` 构建 Linux amd64、Windows amd64、macOS arm64 demo，本机 Software 1x/OpenGL 配置 2x 亮暗/主题色及仓库外启动观察通过。未安装桌面资源，Windows/macOS 仅交叉构建，完整范围见 DesignDemo。
- [x] 修正动画 logo 错用单色样例：改用 `d.logo`，保留上方单色主题演示；新增动画声明绑定回归，旧声明失败。
- [x] 动画绑定修正后全仓非缓存 2342 通过/15 既有跳过，例程无桌面/race/vet、三目标 `cmd/goui` 重构建；本机 Software 1x/OpenGL 配置 2x 初态/中间帧/终态及亮暗/主题色切换通过，Software 反向回到 32 DIP 通过。未执行远端原生桌面，范围见 DesignDemo。
- [x] `example/demo` 九类场景、Modern 亮暗/主题色、持久模型、操作反馈及中文操作说明；采用既有公开 API，不改框架核心。见 [DesignDemo](DesignDemo.md)。
- [x] 分类离开停止动画/计时器并销毁手工 Popover；菜单遮罩不冒充完整键盘模态，系统操作由点击触发，不自动读写文件。
- [x] 四项包内例程测试、无 DISPLAY、race/vet、全仓（2050 通过/14 个既有跳过）、Windows amd64/macOS arm64 编译及本机指定 Software 1x/2x、OpenGL 配置 1x 场景通过；具体范围见 DesignDemo。
- [ ] Windows/macOS 原生桌面、真实 IME、跨应用拖放、系统对话框/外部打开结果及混合 DPI，本轮未执行，不把编译/应用缩放覆盖当作原生验收。
- [x] 开发遇到的框架使用成本、样式/窗口背景粒度等记录于 [DemoFrameworkNotes](DemoFrameworkNotes.md)，区分事实、契约限制与例程错误，不顺带扩接口。
- [x] 第一轮反馈修复：主体 8 DIP 外侧留白避免滚动条裁剪；排除 Integrated 误触共享透明背景，显式 None 的全局副作用由下一项修复。该轮全仓 2052 通过/14 既有跳过、例程 race/vet、Windows/macOS 编译与本机指定场景通过，历史范围见 DesignDemo。
- [x] 透明 None 背景隔离：补充 Window.StyleName / SetStyleName，透明辅助窗选择独立规则，删除例程全局背景覆盖和关闭恢复逻辑；不改平台/Painter或边缘布局。Software 1x/2x 像素回归、全仓 2055 通过/14 既有跳过、相关 race/vet、Windows/macOS 全仓和测试目标编译、本机 Software 1x/OpenGL 配置 2x 指定桌面场景通过；原生远端未复验，范围见 DesignDemo。
- [x] Linux GUI 自绘 Integrated 布局/绘制安全边界统一：根内容整体进入四边缩放区内侧，HeaderBar 以根布局顶部查询高度、Chrome 计入内容起点；按钮控件和完整窗口右侧距离不变。派生最小尺寸计入边缘，显式 MinSize 保持语义；Native/None、Windows/macOS 原生 Integrated 和 Popover 不自动内缩。包内、像素与独立窗口验收见 DesignGUIWindowChrome。
- [x] 内缩后的窗口边缘输入：无 Widget 命中时继续派发既有 host controller，普通/禁用/空内容的八方向及回调销毁回归通过。全仓 2072 通过/14 既有跳过、相关 race/vet、Windows/macOS 全仓及 GUI 测试目标编译；本机 Xfwm4 Software 1x/OpenGL 2x 指定场景通过，未执行项见 DesignGUIWindowChrome。
- [x] demo 边缘收敛：删除主体重复的 8 DIP 外侧留白，滚动条贴合 Window 分配的内容边界；保留页面内部和透明 None 外围间距。宽/窄、零/非零起点、完整宽度/命中/Snapshot 回归、例程 race/vet、全仓 2076 通过/14 既有跳过、Windows/macOS 例程编译、本机 Software 1x/OpenGL 配置 2x 滑块/窄窗/最大化恢复通过；原生远端未复验，见 DesignDemo。

## demo 深度体验反馈（功能优先，已实施）

- [x] 九项用户反馈及源码依据记入 [DemoFrameworkNotes §11](DemoFrameworkNotes.md#11-深度体验反馈功能优先的待修复清单)，区分已确认原因、待验证假设与视觉反馈；本轮未改实现、未验收。
- [x] demo 表格各列保存 OnResize 的受控宽度，排序/重建不恢复固定初值；Linux/Windows 实际拖列、排序和 F5 通过。
- [x] 标签关闭确认归各工作区所有，移交/脱离后只在实际所属窗口显示，窗口销毁清理确认；包内及 Linux/macOS 指定窗口验收通过。
- [x] Button/MenuButton 补键盘激活，Tab 仍只负责焦点；禁用、重复键、子控件输入及回调销毁回归通过，三平台原生 Tab + Space/Enter 计数验证通过。
- [x] HeaderBar 识别 DropTarget 的普通指针输入职责，Transferable TabBar 空白可拖窗；Linux/Windows/macOS 实际拖动通过，其他交互控制器仍保守。
- [x] Linux 仅本地对象的 offer 不进入外部客户端，未发 Drop 的结束补 Leave；gedit 上实际脱离和独立原生拒绝目标的 Enter/Motion/Leave 清理通过，正常可移植 offer 不屏蔽。
- [ ] GoLand 指定应用的视觉复验；本轮没有运行 GoLand，不把 gedit 或协议断言算作该应用验收。
- [x] macOS 脱离 Hotspot 按目标 Chrome 控制区计算，GUI 保留原生水平锚点；macOS 非工作区边缘释放定位通过，Linux/Windows 脱离复验。
- [x] 按用户确认的 SplitView/列宽拖动形状跳变修复捕获期间的光标归属；Linux XFixes 和 Windows GetCursorInfo 原生检查通过，结束/取消/隐藏/禁用/移交回归通过。
- [x] TreeView 的 Modern 行选中色与键盘焦点提示调整，子控件显式选择自己的样式；亮暗色独立断言、既有自定义 current 样式及 Linux 外观检查通过。
- [x] TableView Modern 外观与 TreeView 中性选中色阶统一；表头／边框分层，current 转交 FocusVisible，保留应用覆盖及 Cell 填满契约。全仓／相关 race、Windows/macOS 编译、本机 Software/OpenGL 1x/2x 指定场景通过；原生远端未复验，见 DemoFrameworkNotes §13。
- [x] demo 页面滚动视口提供 8 DIP 对称视觉留白；包内分配/快照及 Linux 1x 观察通过，不改变自动安全布局或 HeaderBar/Controls。
- [x] 全仓非缓存 go test ./...、相关七包 race、Windows amd64/macOS arm64 编译及远端 gui/theme/例程包内测试通过；完整配置、边界与工具问题见 DemoFrameworkNotes §12。

## 高频 UI 绑定优化（暂缓）

- [x] 用户确认功能正确优先，候选优化和契约边界记入 [DemoFrameworkNotes §10](DemoFrameworkNotes.md#10-高频-ui-绑定优化记录暂缓实施)，尚无前后性能数据。
- [ ] 功能稳定后再评估标题同值防护、稳定子树线性快速路径及 demo 样式表缓存；不跳过读取外部状态的列表刷新，不改变 State.Set 通知或引入隐式响应式机制。
- [ ] 实施前准备相同环境下的 100/1000 子控件耗时/分配对照及外部重排、模型变化、最新回调的独立正确性断言。

## 普通内容滚动布局

- [x] LinearLayout 自然主轴无界基线、嵌套 Stretch 行列回归；Button/MenuButton 私有单内容居中解耦，保留换行/baseline。
- [x] ScrollBoth/Vertical/Horizontal、UI Direction 薄绑定和默认恢复；双条占位、统一范围/输入/可见请求/嵌套拖放、极小尺寸和虚拟内容回归。
- [x] demo 删除 flowWidget/640 回退，中文窗口例程 `tests/window/gui/scroll_layout`。见 [DesignScrollLayout](DesignScrollLayout.md)。
- [x] 全仓非缓存 2087 通过/14 既有跳过、六包 race、Windows amd64/macOS arm64 全仓编译；本机实际 Software/OpenGL 各应用 1x/2x 四组共 48 次输入/几何检查和正常退出，demo 指定页面复验。完整 vet 的既有 composites 报警、Linux arm64 既有 SYS_PIPE/SYS_POLL 编译问题如实记录；其余 vet 通过。远端桌面/混合 DPI/性能未执行，详见设计执行记录。

## 帧更新与数值动画

- [x] Root.ConnectFrame 被动信号、RequestPaint 合并与单次唤醒；隐藏/释放/销毁取消任务，帧回调先于布局绘制，离屏不推进帧。见 [DesignAnimation](DesignAnimation.md)。
- [x] 独立 animation 泛型数值过渡与自定义插值/曲线；缓存读值、重定向、Stop / Finish、错误与回调重入保护，不依赖 gui。
- [x] Switch / ProgressBar / TabBar 迁移 Root 帧驱动；UI 复用 State 与既有异步更新，新增中文 GUI / UI 窗口例程。
- [x] 全仓、相关 race、Windows amd64 / macOS arm64 编译及远端包内测试；本机 Software / OpenGL 与远端 Windows / macOS 指定 1x/2x 窗口验收通过。Windows 同步 Popup.Show 初轮失败已修复并复验。
- [ ] 原生 vsync、严格同帧 UI 协调、高刷新率、混合 DPI 与额外后端组合不在首版承诺内，验收范围详见 DesignAnimation。

## Widget 统一禁用

- [x] Widget 自身 Enabled / SetEnabled，实际资格通过 gui.IsEnabled 查询；删除 EffectiveEnabled 与实际状态监听，不增加 StateChanged。先提交子树，再清理输入；重新挂接与回调重入保护。见 [DesignWidgetEnabled](DesignWidgetEnabled.md)。
- [x] 输入不穿透、滚轮可交给可用祖先、焦点与 Tab、Widget 快捷键、手势和拖放清理；保留 Focusable 和控制器注册，窗口快捷键与装饰独立。
- [x] Button / MenuButton / 文本编辑禁用、IME 和异步粘贴失效；CheckButton / Switch / DropDown 迁移公共机制，删除重复字段与控制器移除策略。
- [x] Label / Icon 按自身样式解析 Disabled，GUI 兜底、widgets/style 和 Modern 显式规则；公共 UI Enabled modifier、声明移除恢复自身初值、Snapshot 有效状态和动作检查。
- [x] 实际资格变化复用延迟、可合并的 StyleChanged；子树取消与布局调度集中处理，控制器 Reset 通知已有 press/drag 取消。Window/Popover 共用指针状态恢复静止悬停；DropDown 独立弹层先检查所属控件，拒绝失效输入及自定义选项回调。
- [x] 包内回归与中文窗口例程 `tests/window/ui/enabled`；旧控件禁用断言改为保留 Focusable，UI 公共属性覆盖移到真实协调器测试，不以直接调用控件 Update 模拟通用协调。
- [x] 接口收敛后的全仓、六包 race、无 DISPLAY 指定回归及 Windows amd64/macOS arm64 GUI/widgets 测试目标与窗口程序交叉编译通过。
- [x] 接口收敛后本机 Linux/Xfwm4 Software 配置 1x：点击／文字输入、整组拒绝、焦点清理、单项与声明恢复、按住期间禁用再恢复、定时禁用已展开 DropDown、F6 与退出断言通过；最后 IME Reset 重入保护由包内回归覆盖。
- [x] 接口收敛后复验：Linux/Xfwm4 Software／OpenGL 配置各应用 1x／2x 指定场景通过；macOS 15.7.7 arm64 原生 GUI/widgets/UI 指定回归与 OpenGL 配置应用 1x／2x 桌面交互、静止悬停恢复及弹层禁用检查通过，正常退出。具体范围、缩放覆盖值及后端 fallback 限制见 DesignWidgetEnabled。
- [x] 接口收敛前，本机 Linux 的 Software/OpenGL 配置 1x 与 macOS 15.7.7 OpenGL 配置 1x 指定场景通过；只作为历史验证，不替代本轮验收。具体范围见 DesignWidgetEnabled。
- [ ] Windows 原生回归与桌面复验：最新程序仍被 Defender Exploit Guard 拦截（事件 1121），尚未执行，未改防护设置。真实 IME、Retina／混合 DPI、原生 DnD 完整操作及其他后端组合仍需单独验收。

## DropDown

- [x] 弹出选项两侧至少 6 DIP 留白，垂直滚动条独立占列并保留等宽间隔；选项绘制/命中/快照边界一致，宽内容与行回收保持正确。
- [x] 本轮留白修改：全仓 2000 项通过（14 个既有跳过），相关 race/vet、Windows/macOS 目标编译及本机 Linux Xfwm4 Software 1x/OpenGL 2x 长短列表外观、间隔输入、F6 与退出验证通过，结果见 DesignDropDown。
- [x] 留白修改远端复验：Windows 10 19045 Direct2D 配置/macOS 15.7.7 OpenGL 配置，各应用 1x/2x 的长列表留白、勾选、间隔输入、滚动确认和短列表 4→1→4 行无多余滚动条通过；四次 F6/销毁断言通过。缩放覆盖及后端 fallback 的验证边界、退出码记录和工具问题见 DesignDropDown。
- [x] Popover 定位公开为 Placement / SetPlacement（Point / Bottom），删除 ShowBelow；配置与显示分离，菜单 / DropDown 使用公开接口，可见时通过既有布局调度更新。
- [x] 本轮定位接口整理：全仓 1997 个测试/子测试通过（14 个既有跳过）、相关 race、Windows/macOS 目标编译；Linux X11/Xfwm4 Software 1x 下方对齐、主窗口移动后重开、下缘翻转及右缘退避、F6 与正常退出通过，详见 DesignPopupPlacement §5.4。
- [ ] 本轮定位接口整理的 Windows/macOS 菜单及其余定位场景、其他后端/缩放验证；DropDown Bottom 已随留白调整复验，菜单透明降级与模式切换已有包内覆盖，不以此替代未执行的桌面场景。
- [x] widgets 非编辑式单选、索引模型、Selected 信号、浏览 / 确认分离、自定义内容与虚拟化。见 [DesignDropDown](DesignDropDown.md)。
- [x] Popover 通用锚点弹出、正常 Tab 导航；widgets/ui Selected / OnSelected、Item / SelectedItem 薄绑定。
- [x] 显式兜底 / Modern、快照、逻辑 / 样式 / 像素 / UI 测试及中文窗口例程；全仓 1987 个测试 / 子测试通过（14 个既有跳过），相关 race / vet、Windows amd64 / macOS arm64 编译、本机 Software / OpenGL 1x/2x 指定场景通过。
- [x] Windows 10 Direct2D / macOS 15.7.7 OpenGL 1x/2x 指定原生桌面场景验收；macOS 小数高度 Popup 向上对齐物理像素，短列表创建与 Shrink/Reset 尺寸更新、长列表滚动及关闭回归通过，见 DesignDropDown 执行记录。
- [ ] 混合 DPI、真实 Retina 及未覆盖的绘制后端组合后续验收，不以编译或缩放覆盖值替代。

## Switch

- [x] 独立 widgets.Switch 两态、可选 Child、静默 setter／用户 Change；复用点击与水平拖动竞争，未提交位置和状态分离，支持取消及生命周期保护。见 [DesignSwitch](DesignSwitch.md)。
- [x] 120ms 过渡、GUI Timer、快速反向续接、Animated 开关；widgets/ui 薄绑定、显式兜底／Modern、widgets 角色与具名值快照。
- [x] 逻辑／预乘像素／1x/2x／clip／平移与旋转／UI 协调测试，中文窗口例程。
- [x] 外观精简：Modern／兜底改为无描边白色滑块、弱轮廓关闭轨道、开启边框跟随填充及可辨禁用态；尺寸／行为不变。全仓、相关 vet、Windows/macOS 编译及本机 Software/OpenGL 1x/2x 复验通过。
- [x] 全仓 1973 个测试／子测试通过（14 个既有跳过）、相关 race／vet、Windows amd64/macOS arm64 编译；本机 Software/OpenGL 1x/2x、显式兜底、真实动画帧和回调销毁清理验证通过，详见设计文档。
- [x] Windows 10 Direct2D／macOS 15.7.7 OpenGL 远端桌面各 1x/2x 验收：共 108 项状态断言、亮暗／蓝红／悬停按下／禁用／键盘焦点截图及关闭清理通过，详见 DesignSwitch。
- [ ] 硬件 OpenGL 专项、Windows OpenGL 与混合 DPI 待单独验收。
- [x] Modern 开启轨道移除重复的同色描边，保留尺寸、主题色、关闭轮廓和键盘焦点；新增实际控件的独立圆弧覆盖率回归，旧实现明确失败。
- [x] 本轮全仓 2190 通过/14 既有跳过、相关 race/vet、Windows/macOS 窗口与 Modern/widgets 目标和 Linux arm64 Modern 目标编译；本机 X11/Xfwm4 OpenGL/Software 1x/2x 亮暗、按下、禁用、Tab 焦点与 1x 前后独立圆弧采样通过，见 DesignSwitch。
- [ ] 本轮 Direct2D/Windows/macOS 原生桌面、硬件 OpenGL、分数缩放窗口与真实 HiDPI 未复验；不以此前远端通过或本轮编译代替。

## CheckButton

- [x] widgets 三态、显式非拥有互斥组、指示器／按钮外观；不扩展 GUI／平台／Painter。见 [DesignCheckButton](DesignCheckButton.md)。
- [x] widgets/ui 链式声明、显式兜底与 Modern、包内事件／重入／布局／快照／像素／协调测试。
- [x] 中文窗口例程、本机 Software/OpenGL 1x/2x、全仓／race／vet 及 Windows/macOS 编译；回调销毁窗口的原始 GUI 后续连接保护已单独验证。
- [x] 外观统一：按钮式复用普通 Button／Primary 配色，指示器柔和底色与彩色标记，稳定中性轮廓、3 DIP 方框、默认 Padding 6 DIP；例程加入普通按钮对照及布局断言。
- [x] 外观调整后的全仓／race／vet、Windows/macOS 编译和本机 Software/OpenGL 1x/2x 复验；普通按钮对照、填充／边框／2 DIP 键盘提示像素断言及交互通过，详见设计文档。
- [x] 第二轮外观：18 DIP／4 DIP 圆角实色勾选框与白色标记；radio 独立部件、圆环与圆点留白；按钮选中底色加强、最小尺寸跟随字号；例程使用系统字体。
- [x] 第二轮外观：全仓 1951 个测试／子测试通过（14 个既有跳过），相关 vet、Windows/macOS 编译，本机 Software/OpenGL 1x/2x 及显式兜底交互／截图检查通过；系统 Sans 10 字号下普通按钮与 Toggle 尺寸一致。
- [x] 按钮式选中底色改为独立于主题色的中性灰阶，保留勾选／单选外观及 2 DIP 键盘焦点；Modern、显式兜底和相关断言同步。全仓通过，本机 Software 1x／OpenGL 2x 的亮暗、悬停／按下与切换主题色像素检查通过。
- [x] Modern Radio 圆环／圆点复用 CheckBox 三态选中主题色，撤回单独向正文色提亮的 4.5:1 约束；CheckBox 白色标记原约束保留，不改几何、中性底、禁用、控件或绘制接口。独立 RGB 回归先复现、后通过，见 DesignCheckButton。
- [x] 配色统一版全仓非缓存 2373 通过／15 既有条件跳过，相关 race／vet、Windows amd64／macOS arm64 全仓编译及 Modern Linux arm64 包编译通过；style/gui/widgets/platform 无变动，见 DesignCheckButton。
- [x] 用户截图确认两者实际 RGB 相同但视觉仍不协调，已按确认方案将 Modern Radio 改为主题色实心圆底／白色圆点／零描边，禁用选中使用中性实底；未选中、尺寸、焦点、控件与 Painter 不变。独立颜色回归先复现、后通过，白色标记 4.5:1 断言恢复覆盖两种指示器。
- [x] Radio 实底版样式、Software 离屏 1x／2x 像素及相关 race／vet 通过；Windows amd64／macOS arm64 受影响包和例程编译通过，Modern Linux arm64 包编译通过。不改控件实现，仅同步已有 Modern 像素测试的新预期，见 DesignCheckButton。
- [ ] 全仓测试／Windows 与 macOS 全仓编译被工作期间新增的 tests/temp 无参数 NewPopup 调用阻塞：最终 2373 测试／子测试通过、15 既有条件跳过，但临时例程编译失败；未修改该无关文件。原生外观由用户体验，本轮不提交。
- [ ] 额外发现 Linux arm64 编译受既有 libc SYS_PIPE／SYS_POLL 引用阻塞，平台绑定待单独修复。
- [ ] Windows/macOS 原生桌面、Direct2D 与混合 DPI 待单独验收；Switch 不在此切片。

## ProgressBar

- [x] widgets 长条／圆形、确定比例／忙等待；长条单段单向循环、圆形固定 90° 弧段匀速顺时针旋转，复用 gui.App 的逻辑 Timer；不扩展 GUI/Painter／平台接口。见 [DesignProgressBar](DesignProgressBar.md)。
- [x] widgets/style 显式兜底／Modern 中性轨道与主题色、widgets/ui 链式声明、widgets 角色及具名只读 Attributes；忙等待不报告假进度。
- [x] 包内参数、布局、像素、clip／Transform／1x/2x、动画生命周期及 UI 协调针对性测试通过。
- [x] 迁移后 widgets GUI／UI 中文窗口例程、本机 Software／OpenGL 1x/2x 指定场景、全仓／race／相关 vet／Windows/macOS 目标编译通过；环境及验证缺口见设计文档。
- [x] 圆形固定弧段旋转回归及本机 Software／OpenGL 1x/2x 复验通过；全仓／race／相关 vet 及 Windows/macOS 目标编译通过。
- [ ] Windows Direct2D、macOS OpenGL 原生桌面及混合 DPI 待单独验证，不以交叉编译替代。

## 默认 Tab 焦点导航

- [x] 最小设计及实现：复用 Focusable，树前序／反序、宿主内循环，控制器／快捷键消费后兜底；不扩展控件内部键盘行为或 UI API。见 [DesignKeyboardNavigation](DesignKeyboardNavigation.md)。
- [x] Popover 焦点 crossing、模态转发独立消费、隐藏清理；焦点回调移除／重定向／销毁保护及嵌套 ScrollView 滚入回归。
- [x] 旧实现回归失败已复现；针对性及 GUI/UI/widgets 逻辑测试通过，中文窗口例程已编写。
- [x] 本轮全仓：49 个有测试包、1896 个测试/子测试通过，14 个既有人工/opt-in 跳过；GUI/UI/widgets race、无 DISPLAY 新回归、Windows amd64/macOS arm64 GUI 测试目标及窗口程序编译通过。
- [x] 本机 X11/Xfwm4：Software 1x 的导航／输入／跳过／快捷键覆盖／AcceptsTab／Popover／滚动；OpenGL 1x、Software 2x 的导航／Popover／滚动及 F6 状态检查通过。完整范围及环境见 DesignKeyboardNavigation §6，不扩大为全部平台／后端验收。
- [ ] Windows/macOS 原生桌面、Direct2D、IME 与混合 DPI 等组合单独验收，不以编译替代。

## FocusVisible

- [x] Widget 只读提示与 Snapshot、Window/Popover 独立模式；默认 Tab 开启、PointerDown 关闭、程序焦点保留模式；不重复发出 Focused，不增加 UI/平台接口。
- [x] style.FocusVisible 同 Part 继承 Focused；Button/MenuButton 提示独立于 Hover/Pressed 和子样式；初版底色及 1 DIP 边框已验证，当前按可辨性反馈调整为 2 DIP 柔化主题色边框，TextInput 原样。
- [x] 状态/生命周期/消费优先/Popover 隔离、精确样式回退与可扩展边框、1x/2x Software 像素回归；中文 Tab 例程增加 F6 提示检查与 F8 亮暗切换。
- [x] 初版全仓 49 个有测试包、1907 个测试/子测试通过，14 个既有跳过；相关 race、无 DISPLAY 新回归、Windows amd64/macOS arm64 GUI 测试与窗口目标编译通过。
- [x] 初版本机 X11/Xfwm4 Software 1x/OpenGL 1x/Software 2x：键鼠切换、亮暗提示、Popover 指定输入场景与日志/截图采样通过。底色版历史证据、范围及副作用见 DesignKeyboardNavigation §7.1。
- [ ] Windows/macOS 原生桌面、Direct2D、混合 DPI 等未验收；其它键盘导航不在本切片范围。
- [x] 1 DIP 边框新版：GUI 兜底/Modern 规则、四边/内部/圆角像素回归、自定义 FocusVisible 明确覆盖、TextInput 不叠加完成；恢复旧规则的回归失败已复现，不以旧底色版结果代替验收。
- [x] 新版全仓 49 个有测试包、1910 个测试/子测试通过，14 个既有跳过，零失败；相关 race、无 DISPLAY 回归、Windows amd64/macOS arm64 GUI 测试与窗口目标编译通过。
- [x] 新版本机 X11/Xfwm4、Software/OpenGL 的 1x/2x：键鼠切换、亮暗/主题色/Primary、Popover 与布局不变通过；独立截图断言确认边框 1/2 物理像素、内部原底色，OpenGL2x TextInput 保留原 2 DIP 边框。环境、证据、副作用和未验收项见 DesignKeyboardNavigation §7.2。
- [x] 2 DIP 边框调整：两处默认样式规则已改，旧 1 DIP 规则回归失败已复现；四边逐物理像素、0/1/3 DIP 自定义宽度与 TextInput 不叠加回归通过。
- [x] 2 DIP 版全仓 49 个有测试包、1910 个测试/子测试通过、14 个既有跳过；相关 race、无 DISPLAY 回归与 Windows amd64/macOS arm64 GUI 测试及窗口目标编译通过。
- [x] 2 DIP 版本机 X11/Xfwm4、Software/OpenGL 1x/2x 的指定键鼠、亮暗/主题色/Primary、Popover 及布局检查通过；截图独立断言确认边框 2/4 物理像素、内部原底色、TextInput 不叠加。当前范围、证据、副作用和未验收项见 DesignKeyboardNavigation §7.3，不以 1 DIP 历史结果替代。

## 开发规范

- [x] 2026-10-03 收敛 Windows 崩溃修复：恢复普通绘图及 SDK 查询的原调用方式，
  仅保留 WinAPI 8 个窗口/消息入口的 Proc.Call，以及 HitTestPoint 的 BOOL* 签名修正。
  两个消息循环在循环外分配并复用 MSG，每次取消息前清零，Windows 逃逸分析确认在堆上。GetMessage /
  PeekMessage 回归各自隔离缓冲区，增加真实 DispatchMessage 重入；撤回本轮自行新增、
  模拟非真实 GetMetrics 回调的用例，不改既有排版/绘图测试。契约见 DesignConventions。
- [x] 收敛后 Linux amd64 隔离 Xvnc :98 / Xfwm4 / Software 的 `go test ./...` 通过：
  46 个有测试包、1694 个测试/子测试、14 个既有人工/opt-in 跳过，无失败。
  Windows amd64 全仓测试目标编译通过；既有图像测试仍按原规则写图片。
- [x] MSG 每轮清零后重跑同一隔离环境的全仓测试及 Windows amd64 全仓编译通过，
  两个缓冲区仍确认逃逸到堆上；本次未重复受 ASR 阻止的远端桌面验收。
- [x] 收敛并补齐 MSG 清零后的 Windows 10 x64 原生消息重入复测：按用户要求临时
  关闭 Defender 实时防护，等待状态确认关闭后执行哈希一致的新/旧对照。旧绑定以
  `MSG moved during native callback`、退出码 2 失败；修正版 GetMessage、PeekMessage、
  DispatchMessage 及错误/退出契约全部通过、退出码 0。结束恢复原配置，并确认实时
  防护重新开启；不改变 ASR 规则。此为 SSH Session 0 隐藏窗口原生验证，不等于桌面拖动。
- [ ] 收敛后 HitTestPoint 重新验收：dwrite 测试 EXE 已传输、哈希校验一致，此前启动
  被 Defender ASR 1121 拦截；本次只复测消息入口，没有重跑排版测试。产物仍在
  `/tmp/goui-win-msg.hQYVQV`，初轮原生结果保留为历史证据。
- [ ] 收敛后 Windows 桌面快速 Split 拖动复验：RDP Session 1 的最终 split-view.exe
  在 Direct2D / Software 两种启动配置下都返回拒绝访问（启动器退出码 5），未进入窗口
  场景。与前述原生测试同属安全策略阻止启动，未更改策略，也不把已有旧窗口当作验收。

- [x] 初轮修复（调用方式已由以上收敛设计替代）：Windows 快速 Split 拖动崩溃，原生消息回调导致栈上 MSG 地址迁移。
  修正 WinAPI 指针调用边界及同类 SDK 输出/缓冲区传参；保留原生指针、标量和已固定
  OLE 对象的现有路径。消息重入旧绑定对照失败、修正后通过；DirectWrite 回调输出及
  HitTestPoint 的两个 BOOL* 原生测试通过。契约见 DesignConventions 的原生同步回调节。
- [x] 初轮 Linux amd64 隔离 Xvnc :98 / Xfwm4 / Software 全仓测试：46 个有测试包、
  1694 个测试/子测试通过，14 个既有人工/opt-in 跳过；Windows amd64 全仓测试目标编译
  通过，Windows 10 19045 x64 的 6 个 SDK 包共 8 个原生测试通过。消息重入程序在
  SSH Session 0 显式创建隐藏 HWND 验证，不等同于交互桌面验收；既有图像测试仍写文件。
- [x] 初轮 Windows 10 x64、RDP Session 1、Native 1x：Direct2D / Software 持续快速 Split
  拖动及松手后几何断言通过，正常关闭退出码 0。本轮仅修改 Windows SDK，不改 Split
  或 GUI 行为；macOS/Linux 窗口行为不在本轮专项验收范围。
- [ ] 初轮 OpenGL 在当前 RDP 环境无法选择可用像素格式，旧/新程序均启动失败，尚未验收；
  额外 Direct2D / DirectWrite 整包原生测试被 Defender ASR（1121）阻止启动，未通过。
  没有更改安全策略；这不影响前述已执行的 6 个 SDK 包测试结果。详见 DesignSplitView。

- [x] 固化 platform 通用及 Linux/Windows/macOS 目录级 AGENTS.md，根指南增加阅读路由；PlatformDev 改为代码地图与设计/测试入口，移除过期接口副本、能力表和 TODO。本次仅更新文档，不迁移历史实现。
- [x] 规范文档本地链接与空白检查通过；Linux 本机 `go test ./...` 通过（多数包使用缓存，既有图像测试仍按原规则生成文件）。文档变更未重跑三平台编译或桌面场景，不代表新增平台能力验收。

## Widgets · TabBar / TabView

- [x] 2026-10-02 按 TabView 场景收敛拖出：请求携带源内容区 DIP 尺寸，应用添加自身装饰开销；真正的空目标在 Show 前定位、移交，不做隐藏布局或首帧后补定位。撤销 ClientSize/PrepareLayout，拆分 DragFeedback 的中性光标与禁用回弹选项。保留客户区 Position、最终 handled 查询、源隐藏占位和整批 AfterUpdate；补齐嵌套批次及 Show 失败归属回归。见 DesignTabTransfer 第 31 节，覆盖下列第 30 节的旧隐藏布局方案。
- [x] 新方案 Linux 隔离 Xvnc/Xfwm4、Software 1x：Native 拖出及放大源窗口后拖出、Integrated UI 移交/重建/继续编辑/关闭源、Native Esc/模拟准备失败通过。其它后端/2x/混合 DPI、本轮 Windows/macOS 桌面未重验；不沿用旧的精确热点验收。
- [x] 新方案最终全仓测试 46 个有测试包、1672 个测试/子测试通过，零失败，14 个既有人工/opt-in 跳过；gui/widgets/widgets/ui/ui race 及 Windows amd64/macOS arm64 全仓目标编译通过。证据在 /tmp/goui-tab-content.Npzahm；无新远端桌面验收承诺。

- [x] 2026-10-02 远端失败修复：App 在全部窗口树更新后执行 AfterUpdate、显示新窗口，避免旧源声明重建已移交节点；macOS 隐藏 ControlsRect 保留真实按钮占位；Windows OLE 反馈屏蔽 DROPEFFECT_SCROLL 提示位。新增源/目标两种顺序及 SCROLL 回归，旧时序 overlay 确定性复现失败，见 DesignTabTransfer 30.5。
- [x] Windows 10 19045.6466/Direct2D 与 macOS 15.7.7/OpenGL、Integrated 1x 的 UI 拖出/重建/编辑/关闭源通过，原页面/输入框/State 保留。Windows 原生箭头采样、Esc 和 6 个无头 OLE 测试通过；Integrated 逻辑 2x 的隐藏查询及 7 步定位通过。定位例程高度改为 400 DIP 以满足实际内容最小高度，原断言/容差不变；无生产定位补偿。其它后端、多屏/混合 DPI 未执行。
- [x] 本轮最终全仓测试：46 个有测试包、1671 个测试/子测试通过、零失败，14 个既有人工/opt-in 跳过；ui/widgets 相关 race 及 Windows amd64/macOS arm64 全仓测试目标编译通过。远端最终产物传输哈希一致，日志和旧行为回归失败证据在 `/tmp/goui-detach-fix.37sgsr`，见 DesignTabTransfer 30.5。
- [x] **历史、已由第 31 节取代**：2026-10-02 第 30 节尝试 ClientSize／PrepareLayout 隐藏布局；这些接口现已撤销。客户区 Position、最终 handled 查询、源隐藏占位与整批 AfterUpdate 保留；中性光标与取消回弹拆为独立 DragFeedback 配置，不修改原生 Action 或取消结果。
- [x] 客户区/handled 调整后验证：全仓 46 个有测试包、1668 个测试/子测试通过，无失败，14 个既有人工/opt-in 跳过；相关 race 和 Windows amd64/macOS arm64 全仓交叉编译通过。Linux 隔离 Xvnc :98/Xfwm4、显式 Software：None/Native/Integrated 的 1x/逻辑 2x 六组定位（含 Show 前查询）通过；Native/Integrated 1x 标签拖出定位及 Integrated UI 移交/重建/编辑/关闭源通过。Windows/macOS 原生桌面本次未重跑，其它后端、WM、多屏/混合 DPI 仍是缺口。日志与中文步骤见 DesignTabTransfer 30.4。
- [x] 上一版 Linux 隔离 Xvnc :98/Xfwm4、默认 Painter 1x 的 Native/Integrated 拖出与 UI 移交/重建/编辑/关闭源通过；macOS 15.7.7 arm64、默认 Painter 1x Native/Integrated 拖出定位与分步 Esc 取消通过，原生无头拖放测试通过。Windows amd64/macOS arm64 全仓交叉编译通过；全仓测试和相关 race 通过，14 个既有 opt-in/人工测试仍跳过。此为接口统一之前的结果，不替代当前验收。既有图片测试按原规则产生文件，新增产物在仓库外。
- [x] 前一版 Windows 测试 EXE 被 Defender ASR（1121）阻止的记录保留，未改安全策略；本轮最终构建已完成上述原生无头、光标、定位及 UI 交互验证，原缺口按实际范围关闭。工具与中间失败详情见 DesignTabTransfer 第 30 节。

- [x] 2026-10-01 包内测试按职责整理：11 个文件合并为 7 个，集中到 TabView、TabBar、Transfer、Linux 字体度量、共享 helper、UI 协调及 style；58 个测试名称和全部测试／helper 声明内容核对不变，不改生产代码、断言、输入、容差或跳过条件。无 DISPLAY 的 `go test -count=1 ./widgets/...`、race 和随机顺序重复运行通过；隔离 Xvnc :98／Xfwm4 下 `go test -count=1 -json ./...` 通过，14 个既有人工／opt-in 测试仍跳过。Windows amd64／macOS arm64 的 widgets、widgets/ui、widgets/style 测试目标交叉编译通过；本轮未重新进行真实窗口交互验收。日志及临时工具在仓库外 /tmp/goui-tests-organize.fdy0CS。
- [x] 2026-10-01 标签上下文菜单接入并收敛归属：TabBar 信号同步提供 MenuModel，同时管理定位／显示／关闭；共享 TabView 的多个 TabBar 独立提供菜单。nil 或空模型不显示，不内置业务菜单。关闭按钮和菜单统一走 TabView.RequestClose，补齐失效页面保护。
- [x] UI TabBar.ContextMenu 按页面 ID 生成已有 MenuItem 声明，复用通用 ui.Menu 转换；不保留 TabView 菜单入口或覆盖优先级。ContextMenuError 独立反馈，更新／省略／卸载复用既有连接协调。设计与中文窗口操作说明见 DesignTabs 和 tests/window/widgets/tabs。
- [x] 菜单归属调整后重新验证：无 DISPLAY 的 widgets/UI 测试及 race 通过，补齐命令式／声明式多 TabBar 菜单隔离和查询后续回调的卸载／解绑保护；`DISPLAY=:98 XMODIFIERS=@im=none go test -json ./...` 通过，14 个既有人工／opt-in 测试仍跳过。Windows amd64／macOS arm64 的 widgets、widgets/ui 测试目标和 tabs 例程交叉编译通过。Linux 隔离 Xvnc／Xfwm4、Integrated、Software 1x 实际输入验证未选中标签右键、目标页关闭、Esc 和 Shift+F10 通过；日志／截图／二进制在仓库外 /tmp/goui-tabbar-menu.PEtNso。本次归属调整未重新验收 2x、其它后端或 Windows/macOS 桌面。
- [x] 无 DISPLAY 的 widgets/UI 针对性测试及 race 通过；Linux 隔离 Xvnc :97／Xfwm4 下全仓测试通过，无失败；platform/test 中 14 个既有人工／显式 opt-in 场景保持跳过，不计为验收。既有图像测试仍按原规则生成测试输出，本轮新增日志／截图／二进制均放在仓库外。Windows amd64／macOS arm64 的 widgets、widgets/ui 测试目标和 tabs 窗口例程交叉编译通过，不代表原生运行。
- [x] Linux/X11 隔离 Xvnc／Xfwm4、Integrated、显式 Software 1x/逻辑 2x：右键未选中标签不切页，菜单关闭走同一 CloseRequest，Esc 与 Shift+F10 通过；1x 另验关闭按钮区域右键不关闭、菜单切页、空白 Caption 拖动及移动窗口后重开菜单定位。日志／截图位于仓库外 /tmp/goui-tab-menu.g5TIg0；初次快速合成拖动未移动，加入按下／移动间隔后空白 Caption 拖动完成，不改实现或弱化断言。
- [ ] Windows/macOS 原生桌面菜单交互（含 macOS Control-click）及其它后端本轮未验收。物理菜单键未进入现有按键枚举，本轮仅接入 Shift+F10，不扩展平台能力。

- [x] 2026-10-01 UI 身份收敛：ID 在挂载目标内参与匹配，无 ID 按位置；不引入 Key/全局 ID。TabPageView 成为普通节点，去掉页面双向 key 映射和 Coordinator 任意 State 查询；显式移交验证真实 GUI 归属并保留 State/连接，业务更新先于选择通知。见 DesignWidgetIdentity、DesignUI 和 DesignTabTransfer 第 29 节。
- [x] 本轮针对性测试、全仓测试、GUI/UI/widgets race、Windows amd64/macOS arm64 窗口目标编译通过。Linux/X11 隔离 Xvnc/Xfwm4、Software、1x：Native 跨窗移交/重建/编辑/源关闭与 Integrated 同 Root target-first 移交/重建/编辑/源移除通过。初次 Fcitx 环境未完成，改本地 XIM 后完整重跑；快速连续输入缺少 Rebuild 被拒绝，确认转移结束后分步重跑通过，未放宽断言。既有图片测试仍产生文件。
- [ ] 本轮身份重构的 Windows/macOS 原生桌面与其它后端、2x/混合 DPI 未重跑，按需补充；不以交叉编译或历史验收充作本轮通过。

- [x] 推开式拖入占位，复用位置/宽度动画并计入等宽收缩和溢出；源标签在原生 Begin 后隐藏但保留槽和正文；单标签允许移入已有栏，仅禁止未接受后新建窗口。测试和中文窗口例程同步，详见 DesignTabTransfer 第 28 节。
- [x] 本轮全仓测试、widgets race、Windows amd64/macOS arm64 三个标签窗口程序交叉编译通过；本机 X11/Xfwm4、Software、Native、1x 让位/隐藏/取消/单标签移交操作与严格 accepted 通过。远端桌面、其他后端和混合 DPI 本轮未执行。

主要功能已完成并交付；按用户确认，不再穷举边缘条件作为本轮门槛。下列未勾选的
混合 DPI、更多外部目标及生命周期排列为后续补充验证，不表示主要拖出功能尚未实现。

- [x] 按 Testing 整理四个 widgets 窗口例程的中文入口说明：分离主要流程和可选回归，明确初始文本、操作与逐项预期、观察/自动断言边界、平台差异及退出规则；精简 README 索引。不改变参数、运行逻辑或有效断言。
- [x] 例程说明整理验证：四个入口的 `package main` 及后续代码与修改前逐字节一致；Linux 隔离 Xvnc :97 下 `go test ./...`、格式与空白检查通过。未重跑窗口交互或远端验收；既有图片测试仍按原规则产生文件。

- [x] 原生结束证据探针 `tests/window/platform/drag_end`：曾通过 Go trace 与 Windows IDropSourceNotify 核对原生事实；临时诊断现已移除，保留五类目标与取消/销毁/无效启动场景，改用公开事件和结果断言。AppKit CurrentEvent/事件类型/时间绑定为取消判断所需，继续保留。
- [x] Linux/X11、Xfwm4、Software、Native 1x/2x 的接受/无目标/三种拒绝/Esc 共 12 组通过；两组释放后立即移动的物理坐标和 DIP 独立断言通过。全仓测试、探针 race 和 Windows amd64/macOS arm64 目标编译通过；macOS 15.7.7 arm64 无头证据校验通过。既有全仓图片测试仍按原规则运行。
- [x] 2026-09-30 远端真实桌面验证：Windows 10 19045 / Direct2D、macOS 15.7.7 / OpenGL 的同进程与跨进程接受通过；无按下上下文启动失败通过；1x/逻辑 2x 释放点与截图原点独立核对通过。本机 `go test ./...` 通过；仅恢复 macOS 三个 framework 导入，未修改行为。详见 [远端证据](DesignTabTransfer.md#103-windows--macos-远端桌面验证2026-09-30)。
- [x] 参考 GTK 4.20.0、Qt 6.8.3、libadwaita 1.7.0 与 Firefox Cocoa 实现，调整结束契约为完成动作/已识别取消/未接受，不再要求精确区分所有外部拒绝与无目标。补齐三平台 Position/PositionValid；修复 macOS 取消请求覆盖成功、GUI 同步提交期间 End 重入、X11 最终释放点协商。见 DesignTabTransfer 第 11 节。
- [x] 本轮全仓测试、GUI/结果/探针 race、Windows/macOS 目标编译通过；两端原生无头测试通过。本机隔离 Xvnc/Xfwm4、Software、Native 1x 六组正常输入通过，结束点独立核对通过；初次桌面误插测试文本已撤销，后续改为隔离显示。详见 DesignTabTransfer 第 11.1 节。
- [x] 临时诊断清理：删除三平台 trace、Windows 诊断专用 Notify/GetCursorPos 绑定及 trace 解析器；保留释放点、Esc 与成功提交保护。公开结果回归测试替代解析器测试，Windows 保留 IDropSource 的身份/引用计数/ABI 测试。
- [x] 清理后全仓测试、针对性 race、Windows/macOS 编译和原生无头测试通过；两端 Native 1x（Windows Direct2D、macOS OpenGL）真实桌面正常放下与按住时 Esc 均通过，释放点与截图独立核对通过。Linux 桌面与 2x/混合 DPI 本轮未重跑，详见 DesignTabTransfer 第 12 节。
- [ ] 新结果契约的精确回弹/热点、外部非 GOUI 目标、提交后销毁竞态及混合 DPI 桌面验收；无头测试和编译不代替这些场景。
- [x] Windows 10 19045 amd64 / SSH Session 0 无头运行 Notify ABI、IUnknown 身份、共享引用计数和实际 vtable 测试通过；未启动 HWND/OLE 拖放，不代表 Windows 结束事实验收。
- [x] Linux 1x 另验非输入回调启动失败、按住按钮时程序取消/销毁源窗口；后两者均只有一次取消终态、未观察到释放/Drop。目标已提交后的销毁交错仍未验证。

- [x] UI 定位接入：`App.FindWindow(id)` 借用已挂载 GUI 窗口，复用 Sync；不增加位置声明/窗口包装/缓存，不改原生定位和创建顺序。补充窗口身份、移除/关闭、同 ID 重建、卸载及跨线程/退出测试，新增 `tests/window/ui/window_position` 按钮/快捷键用例；首次显示前定位仍为后续协调切片。
- [x] UI 定位验证：`go test ./...`、`go test -race ./ui` 通过；Windows amd64/macOS arm64 的 UI 包和新窗口用例 `CGO_ENABLED=0 go test -exec=/bin/true` 仅编译通过。Linux/X11、Xfwm4、显式 Software、Native 装饰 1x/2x 经 XTest 键盘/标题栏拖动验证相对定位、工作区重复定位、实际位置查询、手动移动后重建保留位置、移除/同 ID 重建；1x 另验证移动参考窗口不跟随与原生关闭后重建。xwininfo 原生客户区与外框偏移独立核对 200×100 DIP 对应 1x 的 200×100 和 2x 的 400×200 物理像素。仅移动/关闭自建窗口，无系统设置修改；既有全仓图像测试仍按原规则生成文件。Windows/macOS 桌面、多屏/混合 DPI 未验收。
- [x] 跨 TabView/跨窗口与拖出新窗口的最终设计整理至 [DesignTabTransfer](DesignTabTransfer.md)：原页面与 UI childTarget/node 一起移交、应用决定新窗口、复用相对定位，不新增页面所有者/显示器模型。本项仅为设计完成
- [x] 已获胜 DragEventController 通过 DragSource.Begin 同步接管原生 DnD，修复首次跨阈值 Move 丢失；Motion.Local 限时借用应用内对象。Linux/Windows/macOS Native 1x 接受与 Esc 通过，详见 DesignTabTransfer 第 13 节（包含失败记录与时序限制）
- [x] 命令式 TabView.TransferPage、TabBar 显式 Transferable、插入标记与目标接收；保留原页面/输入框与正常 Mount 生命周期。三平台 Native 1x 跨窗口接受、取消及成功后源窗口销毁通过；全仓/race/Windows/macOS 目标编译通过
- [x] 命令式 ConnectDetachRequest + 一次性 TransferTo/Cancel：检查未接受、取消/错误、位置有效性、实际本地 Drop 与原页面归属；应用准备新窗口/正常布局/定位后才提交，失败保留源页面。三平台 Native 1x 拖出、Esc 和准备失败通过，Linux 另验逻辑 2x；全仓/race/目标编译通过，见 DesignTabTransfer 第 14 节
- [x] 本地协商拒绝保护：三平台 Result.LocalTarget 记录最终命中的本进程注册拖放窗口，与 GUI LocalDrop 分开；TabBar 不因该窗口拒绝且未发送 Drop 而误发拖出请求。三平台 Native 1x 拒绝保留源页、经过拒绝目标后离开再拖出均通过，详见 DesignTabTransfer 第 17 节
- [ ] 更多转移生命周期/通知销毁交错与失败恢复回归；Integrated 同 Root 的其他协调顺序、远端 2x 与真实混合 DPI 桌面组合验收
- [x] Integrated 多栏组合：命令式同 View 多栏中间插入与 Esc，Windows/macOS 1x、Linux Software 逻辑 2x 通过，Linux 另验 1x 插入；UI 同 Root target-first 的转移/重建/编辑/源移除及 Esc 同样通过。测试用例将 TabBar 嵌入 HeaderBar，无生产修改；全仓/race/目标编译通过，环境失败及验证范围见 DesignTabTransfer 第 22 节
- [x] 最后一个编辑页：包内成功/拒绝检查原 TextModel、选区、滚动及撤销记录；窗口 -editor 三平台 Integrated 1x、Linux 逻辑 2x 的移交/源关闭/真实键盘撤销重做及继续输入通过，取消范围、测试准备失败与修正见 DesignTabTransfer 第 23 节。全仓/widgets race/目标编译通过，不代表 UI 多行编辑页或混合 DPI 验收
- [x] 拖动期间布局变化：包内验证无新 Motion 的插入线更新/Drop 位置重算及源页移除后的拒绝；窗口 -target relayout 的真实投递和 Esc 三平台 Integrated 1x 通过，全仓/widgets race/目标编译通过，见 DesignTabTransfer 第 24 节
- [x] X11 源拖动不再按用户按住时长过期：仅在释放后的 Status/Finished 等待应用 10 秒无进展/30 秒完成上限。包内回归先复现旧错；三平台 Native 1x 的 36 秒投递/Esc、Linux 未注册目标与 Integrated 重新布局后长悬停投递通过。无响应目标的源超时结果通过，但恢复目标暴露下项缺陷；见 DesignTabTransfer 第 25 节
- [x] X11 对端销毁后的迟到协议发送：按 display/请求序号/目标/opcode 精确处理已跟踪 Xdnd SendEvent 的 BadWindow，其他错误转交原 handler。独立目标恢复后存活、新源再次投递及正常关闭通过；ABI/过滤/回收回归、race、全仓和三平台 Integrated 1x 接受/Esc 通过，见 DesignTabTransfer 第 26 节
- [x] 同一 View 多个 TabBar：修复原生接管后插入计算跳过仍可见源标签宽度、导致多移一格的问题；共享命中/标记槽位计算并移除冗余 dropIndex。七组独立坐标断言先复现旧错再通过；三平台 Native 1x 中间插入 B/A/C、一次 Moved、零卸载及 Esc 零修改通过，见 DesignTabTransfer 第 20 节
- [x] 命令式组合验收：同窗口双 TabView 向空栏移交、跨窗口溢出栏静止悬停滚动后尾插、滚动后按住鼠标 Esc 取消，三平台 Native 1x 实测通过。用例显式设置窗口最小尺寸以真正产生溢出，并检查源/目标选中项、原页面/输入框身份、完整顺序和 Mount 次数；全仓与 widgets race 通过，见 DesignTabTransfer 第 19 节
- [x] 命令式销毁交错窗口回归：页面 Unmount 中销毁目标，原页面按原顺序/选中状态恢复；提交后的源选中通知中销毁源，目标保留原输入框且继续编辑。三平台 Native 1x 真实拖放与 Verify 通过；详见 DesignTabTransfer 第 18 节，不代替 UI 协调层销毁交错验收
- [x] UI Coordinator 借用 node，同步移交 Bin 子树；widgets/ui 两侧 key 映射与 OnTransfer 声明归属协调。三平台 Native 1x 既有窗口移交、重建、继续编辑、源窗口删除及 Esc 通过，见 DesignTabTransfer 第 15 节
- [x] UI 协调全仓测试、GUI/UI/widgets race、Windows/macOS 目标编译通过；第 15 节桌面后补充的重建期拒绝检查已随第 16 节程序在远端复验
- [x] UI 拖出新窗口准备/定位/失败清理：正常布局后定位、移交原 node/State、OnTransfer 更新声明；三平台 Native/Integrated 1x 拖出/重建/编辑/源关闭通过，Native 同 key 拒绝与主动取消准备、Integrated Esc 通过；Linux Integrated 另验逻辑 2x。见 DesignTabTransfer 第 16 节
- [ ] 更多 UI 混合所有权及回调销毁集成回归；同 key 拒绝与第 17 节的原生协商拒绝分别验收
- [x] UI 生命周期交错：GUI Unmount 中目标销毁后回滚、目标 GUI Mount 中源销毁后提交，三平台 Native 1x 均经过真实拖放/声明重建/继续编辑检查，保留原 TabPage/输入框/State 和连接；全仓与 UI/widgets race 通过，见 DesignTabTransfer 第 18.2 节
- [x] UI 同 Root 双 View：窗口用例新增 -same-window / -target-first；三平台 Native 1x 的两种声明协调顺序、真实移交/重建/继续编辑/源容器卸载和 Esc 通过。未修改 UI 内核；源卸载检查 Root/父节点及 View 释放，不误用最终销毁标志；全仓与 UI/widgets race 通过，见 DesignTabTransfer 第 21 节
- [x] 三平台主要标签转移窗口验收：跨窗口/拖出新窗口、取消/拒绝/准备失败、Native/Integrated、转移后重建/继续编辑与源关闭，见 DesignTabTransfer 第 13–24 节。真实混合 DPI 仍为后续未执行项，未据此声称覆盖

- [x] 增加 DesktopWindow / gui.Window.Position(relativeTo)：实际外框位置查询，与 SetPosition 使用相同参考系/DIP；nil 当前工作区、自身参考为外框偏移。无缓存请求值、无移动等待、无位置变化信号/UI 绑定。X11 无有效外框观察或 managed unmapped 时返回 Unavailable
- [x] 仅将 Position/SetPosition 的 `_NET_FRAME_EXTENTS`、`_NET_MOVERESIZE_WINDOW` 移至 X11 Platform.atoms 初始化并复用；保留实时属性/能力查询，不动 WorkAreaAt、透明窗口或其他 atom。`go test ./...` 及 Linux/Xfwm4、Software 1x/2x 的 `window_position -auto` 通过；本轮未复验 Native 装饰、其他平台或重跑跨平台编译（仅 Linux 私有初始化/引用变化），未作性能结论。窗口用例仅移动/关闭自身窗口，既有全仓桌面测试按原规则执行
- [x] Position 验证：`go test ./...`、`go test -race ./gui ./ui ./platform/linux/x11` 通过；无 DISPLAY 下查询委派/生命周期/EWMH extents 逻辑测试通过。Windows amd64/macOS arm64 `CGO_ENABLED=0 go test -exec=/bin/true ./...` 仅编译通过，未执行原生测试。Linux/Xfwm4、Software 1x/2x 的 `window_position -auto` 对 nil/自身/双向参考位置与 WorkAreaAt 独立观察断言通过
- [x] Linux/Xfwm4、Software 1x Native：实际鼠标点击查询 B.Position(A)=(200,100)、Position(self)=(-5,-29)。原生标题栏拖动 B 后，查询变为 (300,160)，Position(nil)=(345,229)；xwininfo 客户区 (350,299)、EWMH 边距 (5,5,29,5)、工作区原点 (0,41) 独立核对一致。仅操作本次测试窗口，未改变系统设置；全仓已有桌面依赖测试按原规则运行
- [ ] Position Windows/macOS 桌面验收、混合 DPI / 多屏与最小化等状态下的原生观察验收

- [x] 窗口定位前置能力：三平台 DesktopWindow.SetPosition + gui.Window.SetPosition/WorkAreaAt；nil 相对调用时自身 WorkAreaAt(0,0)，有参考时相对参考客户区 DIP。2026-10-02 统一目标为客户区左上角，不引入显示器或桌面坐标模型。见 [窗口契约](DesignWindowManagement.md#one-shot-relative-positioning)
- [x] 定位验证：`go test ./...`、`go test -race ./gui ./ui ./platform/internal/workarea ./platform/linux/x11` 通过；新增定位逻辑在无 DISPLAY 下通过。Windows amd64 / macOS arm64 的 `go build ./...` 和 `go test -exec=/bin/true ./...` 仅交叉编译通过，不代表原生执行。编译期间发现并修正 AppKit 字段拼写错误，修正后通过
- [x] 本机 Linux/X11、XFCE/Xfwm4、显式 Software 1x/2x：`tests/window/gui/window_position -auto` 验证显示前定位、相对参考/自身、nil 重复请求不累积、参考移动不跟随、负偏移、非法点和销毁引用。1x Native 另经真实 XTest 点击按钮及 xwininfo/_NET_FRAME_EXTENTS 独立观察：A 客户区 (45,110)，B 外框 (245,210)，正确相差 (200,100)；初始外框相对工作区 (0,41) 分别为 (40,40)/(120,120)。仅创建、移动、关闭本次用例窗口；已有全仓桌面依赖测试仍按原规则执行
- [ ] Windows/macOS 的定位桌面验收；多屏、混合 DPI、原生装饰外框与系统位置约束按窗口用例分别验证
- [x] TabView 跨容器/跨窗口转移、拖出新窗口与 UI 子树协调（实施分项见本节顶部及 [转移设计](DesignTabTransfer.md)）；主要路径完成，边缘补充验证不阻塞本轮交付

- [x] 修复拖动未激活标签后关闭按钮保持显示：主体焦点不再强制显示按钮，仅选中/悬停/关闭按钮自身焦点显示；监听关闭按钮的焦点变化，不清除标签焦点。先用定向回归复现旧版提交/Esc/栏外释放均失败，再验证修正及隐藏区域命中、关闭按钮焦点保持。`go test ./...`、`go test -race ./widgets/...` 通过；本机 Linux/X11、XFCE、Software 1x 三种结束路径和 2x 提交后移出鼠标复验通过，Windows/macOS 桌面未复验
- [x] 默认宽度调整为 112..240 DIP，显式 TabWidthRange 不变；分隔线增高至 16 DIP，仍为 1 DIP 宽且垂直居中，不超过栏高。补验默认重置、344 DIP 三标签溢出临界点和短栏分隔线。`go test ./...` 通过；本机 Linux/X11、XFCE、Software 1x/2x 验证宽度、分隔线及缩放/固定宽度溢出，本轮未重复跨平台编译或桌面验收
- [x] 同仓库 `widgets` 命令式 TabPage/TabView/TabBar、`widgets/ui` 声明式绑定
- [x] GUI 同父节点重排、UI 更新后关联、标签角色与 Selected 快照、兜底与 Modern 样式
- [x] 页面归属/选择、重排/溢出、边缘滚动计算及关联时序的包内回归；独立窗口例程
- [x] Linux/X11、XFCE 本机：默认 Painter 1x 验证切页、拖动重排、关闭、溢出按钮、重建后文本保留与亮暗切换；窗口位置未变化。默认 Painter 可能在 OpenGL 创建失败时回退 Software，未单独认定为 OpenGL 验收
- [x] Linux/X11、XFCE 本机：显式 Software 2x 验证绘制、点击切页和拖动重排；窗口保持位置与 1600×960 物理像素尺寸
- [x] Linux/X11、XFCE 本机 1x：真实键盘输入验证标签主体聚焦后 Right、End、Home 顺序切换页面
- [x] Linux/X11、XFCE 本机 1x：Esc 与纵向拖出后释放均取消重排；栏内释放仍只提交一次重排。纵向释放缺陷增加包内回归
- [x] 修正体验：标签跟手、相邻标签/释放归位过渡；按实际字体行高布局；TabBar 空白路径恢复 HeaderBar Caption
- [x] 新增事件分发回归：拖动期间模型不变、一次提交、Esc 取消、反向过渡、关闭命中与快照绘制顺序；无 DISPLAY 的 Pango 14pt/24pt 行高测试通过
- [x] Linux/X11、XFCE 本机显式 Software 1x：跟手/让位、键盘导航、空白 Caption 移动窗口；9 个标签左右两端自动滚动、释放后一次重排，窗口位置保持不变
- [x] Linux/X11、XFCE 本机显式 Software 2x：跟手排序、Esc/纵向拖出取消、空白 Caption、关闭按钮、亮暗切换与 14pt/24pt 完整文字；窗口为 1600×960 物理像素
- [x] 修正后 `go test ./...`、`go test -race ./gui ./ui ./widgets/...` 通过
- [x] 标签视觉优化：Modern 中性色选中/拖动底面、弱化图标与关闭按钮、6 DIP 圆角；真实 4 DIP 标签间距、10 DIP 标题留白与关闭按钮右侧 4 DIP；示例 HeaderBar Padding 6
- [x] 回归覆盖间隙命中、精确溢出/首尾范围、当前页揭示、不等宽拖动阈值与快照；亮暗/多主题色下文字对比度及按钮状态。`go test ./...`、`go test -race ./gui ./ui ./widgets/... ./theme/modern`、无 DISPLAY 的 widgets/modern 测试通过
- [x] 本机 Linux/X11、XFCE、显式 Software 1x/2x 验证中性色外观、完整顶部边框、14pt/24pt 行高、关闭按钮反馈、间隙 Caption、拖动/取消与溢出滚动；Windows amd64/macOS arm64 窗口目标仅交叉编译通过，不代表桌面验收
- [x] 未选中标题/图标恢复正常前景色；关闭按钮仅选中/悬停/焦点在标签内时显示，固定预留空间，隐藏区域可选中或拖动，不误关。包内覆盖跨帧按下/抬起、显隐布局稳定、焦点迁移、Snapshot 和 Closable 变更；本机 Software 1x/2x 亮暗主题、悬停与关闭未选中页、键盘切页、拖动已验证
- [ ] Windows/macOS 真实窗口验收（本轮按要求仅本机验证）
- [x] TabBar / UI `TabWidthRange`：默认 96..200 DIP，全部标签等宽收缩，到共同最小宽度后才滚动；相等上下限保持固定宽度。保留图标/关闭按钮安全空间，标题仅视觉裁剪，文本与快照不截断
- [x] `tab-bar/separator` 装饰分隔线：仅相邻未选中、未悬停标签之间显示，拖动及归位期间隐藏；兜底和 Modern 亮暗中性色样式，不增加 Widget 或快照节点
- [x] 回归覆盖等宽分配、临界宽度反复布局、滚动保持、选中页揭示、布局变化取消拖动、UI 参数重置、分隔线状态/裁剪/透明样式，以及 14pt/24pt 文字行高。全仓、相关 race、无 DISPLAY 的 widgets/modern 测试通过
- [x] 本机 Linux/X11、XFCE、显式 Software 1x/2x 验证等宽压缩、最小宽度溢出/放大恢复、固定宽度切换、分隔线和拖动排序；2x 补验亮暗与大字体裁剪。例程显式设置 400×320 DIP 最小尺寸供缩窗验收；Windows amd64/macOS arm64 窗口目标仅交叉编译通过
- 设计契约见 [Tabs 设计](DesignTabs.md)。既有 gui 控件保持原位。
- [x] WidgetBase 改为同父节点 `MoveChildBefore/After` 相对重排，公开兄弟顺序、nil 首尾和生命周期契约；TabView 保留页面索引 API，TabBar 置顶不修改页面顺序。Tab Role 常量移至 widgets，Selected 留在 GUI；兜底规则移至 widgets/style，显式组合，Modern 无须追加
- [x] 相对重排的正反向/首尾/无效参数、焦点与 mount、无操作不重绘、绘制/命中/Snapshot/布局一致性及页面索引转换回归通过；样式组合、覆盖及无全局注册回归通过。`go test ./...`、`go test -race ./gui ./ui ./widgets/... ./theme/modern`、无 DISPLAY 的 gui/widgets/modern 测试通过
- [x] 本机 Linux/X11、XFCE、Software：1x 显式 fallback-style 的前后排序、Esc 取消、选择和关闭；2x Modern 的排序、Esc 取消与重建保持已复验。仅操作本轮启动的验收窗口。Windows amd64/macOS arm64 窗口目标交叉编译通过，未做远端或其他平台桌面复验

## UI 拖放简化与 Widget ID / Name

- [x] 2026-09-24：确认并写入设计
- [x] GUI Name/SetName、UI ID/Name 独立协调及 WidgetInfo.Name
- [x] Root 范围的 ID 查询返回 Widget（未找到返回 nil，重复返回首个匹配）；UI 通过 Sync 获取当前挂载 Widget，不引入 WidgetRef
- [x] Popover 公开接口嵌入 Root，查询与布局/绘制请求直接通过公开接口使用；移除测试中的内部类型断言
- [x] HeaderBar.Captions 按 ID 精确匹配 Caption 区域；按用途迁移旧 Name、内置固定标记、例程和测试
- [x] UI DragSource.Data 预置数据并由 OnPrepare 补充，DropEvent 更名为 DropRequest；复用单源/单目标描述符与信号协调
- [x] Data/Prepare 覆盖、重建及绑定移除包内回归；Linux `go test ./...`、`go test -race ./gui ./ui` 通过，Windows amd64/macOS arm64 的 GUI/UI 与拖放窗口目标交叉编译通过（不代表真实桌面验收）
- [ ] Data 声明与预览、重建、拒绝、错误反馈的窗口操作验收
- [x] ID/Name、Snapshot、HeaderBar.Captions 与查询生命周期包内回归；按 ID 查询生成 Preview 的窗口用例
- [x] 测试适配 FindWidget 单返回值与首个匹配语义，UI 查询测试锁定线程；Linux 全仓测试通过，Windows amd64/macOS arm64 的 GUI/UI 与拖放窗口用例交叉编译通过（未运行桌面验收）
- [ ] 按 ID 查询生成 Preview 的 Linux/Windows/macOS 真实窗口操作验收
- 设计见 [Widget 标识](DesignWidgetIdentity.md)、[DnD UI 设计](DesignDragAndDrop.md#10-ui-与语义快照)
  和 [UI 共享修饰器](DesignUI.md#36-idname-与-widget-查询已实现)。既有 DnD 验收记录不代表新入口已验收。

## GUI 手势竞争与宿主状态

- [x] 保留 `rootBase` 名称，集中持有分发器、原生 surface、DnD 服务和目标状态；Popover 旧 surface 回调按代次隔离
- [x] 可选 GestureController / JoinGesture 参与协议、同事件同步裁决、深度和注册顺序优先、取消及重入保护
- [x] Click、双击计数、立即/阈值 Drag、文本选择、DragSource 和非原生 Chrome 接入；移除 Dispatcher 中源候选及旧捕获扫描
- [x] GUI 包内父子竞争、同事件裁决、取消回调销毁获胜者和第二次点击转拖动回归
- [ ] Windows/macOS 原生 DnD 与 BeginMove/BeginResize 真机接管验收；Linux 桌面组合交互复测
- [x] 移除 Chrome 旧直接处理分支，双击/拖动测试使用可控时钟
- [x] Linux `go test ./...`、GUI 手势/目标取消定向 race 测试通过；Windows amd64/macOS arm64 的 GUI 测试目标与 GUI DnD 窗口例程交叉编译通过
- 语义见 [手势竞争设计](DesignGesture.md) 与 [DnD 设计](DesignDragAndDrop.md)。

## Drag and Drop / RenderWidget

- [x] `RenderWidget` 按需重画子树，三个 Painter 的离屏目标/读回实现与回归用例
- [x] 用户确认 RenderWidget 已验证（2026-09-23；未额外推断用户验证的平台范围）
- [x] DnD 详细开发设计：数据、控制器、原生事件、生命周期、UI、快照与互操作矩阵
- [ ] P0：三平台原生真实文件 Copy 双向互操作及注册/取消/销毁
  - [x] 基础契约：便携 Data、动作/格式/预览、URI 编解码、Offer/服务与原生事件类型
  - [x] GUI Click 取消按压协议、源/目标协调与无误 Click 回归
  - [x] Windows OLE、macOS AppKit、Linux XDND 的原生源/目标代码
  - [ ] Windows Explorer、macOS Finder 的双向真实桌面互操作及销毁竞态验收
- [ ] P1：文本、URL、MIME、INCR、预览、动作协商与会话身份
  - [x] 三平台代码；Linux 大文本 INCR 和预览光标、Windows OLE 预览辅助对象无头测试
  - [ ] Windows/macOS 真实桌面文本、URL、MIME、预览与动作验收；Linux 外部大文本反向 INCR
- [x] P2：GUI 源/目标、候选与指针取消、Local 数据、生命周期及快照；Linux 1x 同窗口/跨窗口/Popover 路径通过
- [x] P3：UI 链式绑定、协调与独立窗口例程；Linux 1x 点击/拖放/重建回归通过
- [x] 三平台验收用例整理：原生 Copy/取消/拒绝预期、GUI 五种独立格式源与接收区、可见计数、UI revision/禁用/移除恢复绑定；具体操作见 tests/window 入口注释
  - 本轮 Linux/X11、Xfwm4：原生用例（实报 OpenGL、1x）Copy/取消/拒绝退出检查通过；GUI/UI 使用默认 Painter（未单独断言实际后端），1x 五种格式、拒绝、UI 重建/禁用/移除恢复通过；2x 大文本、文件路径和 Popover 通过。
  - 两个 GUI 进程间 URL、二进制 MIME、双向 256 KiB 文本内容校验通过，Local 跨进程拒绝；全部正常退出且 FAIL=0。三组入口均完成 Windows amd64/macOS arm64 可执行文件构建，本轮未运行两平台的新窗口用例。
- [ ] Windows/macOS Explorer/Finder 真实桌面互操作；Linux 外部反向 INCR、销毁竞态及更多跨进程场景
- 设计见 [Drag and Drop](DesignDragAndDrop.md)、[RenderWidget](DesignRenderWidget.md)。
  Linux/X11、Xfwm4/Thunar：1x 文件双向 Copy、256 KiB INCR、Local 同窗口/跨窗口/Popover、
  UI 重建及预览已验收；2x Popover/预览/Esc 取消已验收。Windows/macOS 远端按约定只做
  无头测试，不计桌面互操作。
- 2026-09-23：P0 基础契约及 GUI 指针取消协议进入代码；平台/事件/GUI 包内测试、
  Linux 全仓测试、Windows amd64/macOS arm64 平台与 GUI 测试目标交叉编译通过。
  该记录对应早期基础契约阶段；后续实现状态见上方清单。
- 2026-09-23：Linux `go test ./...`、DnD 相关 race 测试通过；Windows amd64/macOS arm64
  全仓交叉编译通过。Windows 10 OLE 和 macOS 15 AppKit 的专项无头测试通过。

## WrapBox 自动换行容器

- [x] 纯布局 WrapLayout、GUI WrapBox、UI HWrap/VWrap；不拆分 gui 包
- [x] 行内/行间距、对齐/基线/拉伸、快照及动态更新回归测试
- [x] 独立窗口例程 `tests/window/ui/wrap`，含操作与预期
- [x] Linux amd64 `go test ./...` 与 `go test -race ./layout ./gui ./ui`
- [x] Windows amd64 / macOS arm64：窗口例程构建、gui/ui 测试目标交叉编译
- [ ] 三平台窗口缩放、点击、显隐、基线及滚动真机验收
- 设计与限制见 [WrapBox](DesignWrapBox.md)；不隐式改变 ScrollView 双轴无界语义。
- 2026-09-22：上述命令通过；现有 OpenGL 像素/手动窗口测试按环境开关跳过，
  不计为桌面验收。
- [x] 示例显式最小尺寸 400×600，避免无界单行期望宽度阻止缩窗；补充
  GUI 显式窗口最小尺寸优先及往返换行回归；缩短竖向示例和说明文字，400 DIP 可见。
- [x] Linux/X11/Xfwm4、1x 真实桌面：窗口原生 hint 为 400×600，XTest 拖动边缘
  从 780 缩到 400 后换行，放宽恢复；点击计数、末项显隐、行内居中、滚动到 Item 30。
  默认后端选择路径验证后，显式 Software/1x 再次验证缩窄及上述交互；默认路径
  未单独确认是否发生 OpenGL→Software fallback，不计为 OpenGL 专项验收。
- [ ] Windows/macOS 与 2x 桌面复测；窗口“最小尺寸与期望尺寸”的通用契约另行设计，
  本次仅修正示例，不改变窗口自动最小尺寸策略。

## P0 · Widget Painter

- [x] `gui.Painter` 改为 `Save/Restore` 局部绘图接口
- [x] 删除 `SubPainter`、`PaintChildren` 和 GUI 无参数 `Begin/End`
- [x] GUI 内核自动遍历可见 Widget 子树并隔离绘图状态
- [x] ScrollView 使用内部结构 viewport
- [x] 补齐状态、顺序、裁剪、panic 和 ScrollView 测试
- [x] 同步 Widget Painter、Transform、Paint 和 Scroll 设计文档

## P1 · 现代外观基础

- [x] 特殊 None 窗口通过 Chrome 提供拖动/八方向缩放区域，不自动生成按钮或光标
- [x] macOS BeginResize 后端有限模拟；几何单元测试与跨平台编译
- [ ] macOS 手动缩放真机验证：八方向、最小尺寸、窗口外释放、跨屏 HiDPI、透明重绘及取消
- [x] Linux GUI 自绘 Integrated：透明圆角、实线边框、GTK4 风格圆形按钮，无 GOUI 阴影；复用 Caption 双击及原生移动/缩放，不伪造平台 Integrated
- [x] Software 1x/1.5x/2x 像素测试、真实 GLX 遮罩对照及两后端 X11 外观检查
- [x] Integrated 最终区域查询驱动缩放光标；圆形控制按钮改为 24 DIP 可见背景、28 DIP 点击区域
- [x] Linux GUI Integrated 移除 graphics 专用窗口遮罩。早期仅 ClipRect 的方案已被安全内容区域布局取代：根 Measure／Arrange 与结构 clip 一起避让缩放边缘，见 DesignGUIWindowChrome。
- [x] 边框保留八方向光标语义；X11 接入 libXcursor 主题/别名加载及 XSETTINGS、X Resources 动态更新，不按发行版分支

- [x] 三平台 Window/Popup 显式透明创建与 Surface.Transparent 配置查询
- [x] Linux ARGB visual/FBConfig、macOS 非 opaque 表面与 OpenGL/Software 预乘 alpha 呈现
- [x] Windows DirectComposition + Direct2D、WGL/DWM alpha、Software composition 上传
- 指定窗口输入区域/鼠标穿透已按用户决定取消；不作为 Linux Integrated 的依赖
- [x] 三平台底层透明呈现用户验证
- [x] GUI Window/Popover 透明选项、宿主背景样式、空帧清理及生命周期测试
- [ ] GUI 透明示例真机体验；设备丢失恢复专项验证
- 当前实现：[Window / Popup 透明绘制](DesignTransparency.md)、[GUI Window Chrome](DesignGUIWindowChrome.md)。旧输入区域草案保留为历史记录，不继续推进。
- 下列六步已实现，保留原实施顺序；分平台待验收不等于代码未完成，详见 [Modern 样式](DesignModernStyle.md)：
- [x] 1. style.Shadow 值、覆盖、显式关闭和比较；三后端阴影颜色修正，Software／GLX 像素回归及 Windows／macOS 交叉编译
- [ ] 第 1 步 Windows 原生阴影像素测试及 Windows／macOS 样例运行验收
- [x] 2. Widget.StyleChanged、文字资源及布局／列表缓存失效；涵盖重新挂载、隐藏／销毁、装饰树及 Popover，稳定帧资源复用与 Software 像素测试
- [x] 3. 窗口装饰隔离，应用样式仅控制窗口背景；私有亮暗配色、装饰像素／几何回归及 Windows／macOS GUI 交叉编译
- [ ] 第 3 步三平台装饰与窗口交互真机验收
- [x] 4. GUI Settings 首份快照及 Run 轮询生命周期；UI 自动 RequestUpdate、合并、相同样式去重、监听清理与过期任务失效
- [ ] 第 4 步系统设置变化自动更新真机验收
- [x] 5. 显式启用 theme/modern：亮暗、主题色、字体与完整菜单阴影规则；按钮默认焦点边框为 0，保留样式扩展入口；nil sheet 保留 GUI 兜底；Modern 示例及对比度／规则回归
- [ ] 第 5 步 Modern 示例三平台外观验收
- [x] 6. PopoverMenu 样式驱动外壳、菜单行文字、定位、输入、透明降级及生命周期
- [x] GUI 普通 Overlay 容器：单主节点、多覆盖节点、内容测量、逐节点 Fill／位置查询；复用现有命中、绘制、生命周期和快照，见 [Overlay 设计](DesignOverlay.md)
- [x] Overlay 的 UI 薄适配：链式 OverlayItem、独立主槽位与覆盖列表协调、信号生命周期及输入／快照测试
- [x] UI 通用挂载目标：ui.Bin／ui.Container、UpdateChild／UpdateChildren；Overlay 与 ListView 不再访问私有协调节点，Root 无具体控件清理特例
- [ ] Overlay 三平台应用场景体验；查找栏、确认层等按普通 Widget 组合
- [ ] Dialog 键盘模态／焦点恢复、Toast／应用内通知组件单独设计；不由 Overlay 隐式接管
- Menu 和 ComboBox 下拉继续使用原生 Popover，不作为本轮 Overlay 的迁移任务
- [x] 绘制型 gui.Icon / ui.Icon、默认及 Modern 样式、布局／样式／协调／离屏测试，见 [Icon 设计](DesignIcon.md)
- [ ] Icon 三平台／三后端窗口外观验收：`tests/window/ui/icon`
- [x] 统一预乘颜色：构造、三后端画刷／Clear／渐变、三平台文字颜色及 Pango 局部 alpha，见 [绘制颜色](DesignGraphicsColor.md)
- [ ] Windows/macOS 颜色运行回归与三平台透明呈现体验：`tests/window/graphics/alpha`
- [x] 公开 IconSource.Image(物理宽高, 前景色)：移除 IconDraw 双路径；Source 负责最终像素，Icon 自持原生缓存；位图集合、SVG、Software 离屏适配与 UI 安全复用
- [x] Icon 资源创建/取图契约：icon.NewImage 构造时验证并固定图片集合；Image 不返回 error；移除渲染错误信号，缺失占位符及上传重试由控件处理
- [x] Icon UI 协调不比较 Source：每次设置使请求失效；GUI 重新取图后复用相同图片的上传资源，nil/卸载立即释放
- [x] Tint RGBA/NRGBA 快速路径、像素/分配/基准回归；svgicon 同时展示 Brain/People/Dev tool 与原有辅助场景
- [x] SVG 连续弧线解析：自有 oksvg fork 修复并提交上游 PR；直接固定 fork 依赖，Brain/People 独立几何对照与公开 Image 回归通过，见 [SVG 验证记录](DesignSVG.md#连续弧线参数解析修复2026-09-18)
- [ ] 跟踪 fyne-io/oksvg#6 及稳定版本；复杂 SVG 渐变的既有差异按具体资源需求另行处理
- [x] 资源归属收敛：固定图片移至 icon.Image，移除 gui/ui.IconImage；共享着色移至 icon/internal/pixels，不扩展 graphics 公共工具
- [x] 受限静态 SVG 适配及像素/缓存/race 回归，见 [SVG 设计](DesignSVG.md)；全仓测试和 Windows/macOS 交叉编译通过
- [ ] SVG 三平台/三后端窗口验收：`tests/window/ui/svgicon`；包含位图构造、缺失占位和空 Source；完整 SVG 能力仍按资源需求扩展

## P2 · Window 与 Popup 样式

- [x] 迁移 `Window` / `DesktopWindow`：桌面最小尺寸、互斥 `State/RequestState`、同步 `SetHitTest`；移除独立管理包装与 FrameRegions
- [x] `ControlsRect()` 查询原生按钮占位（客户区 DIP），无专用通知；macOS/Windows 实现，X11 明确不支持
- [x] macOS 内容视图 `mouseDownCanMoveWindow=false`，客户区仅显式 `BeginMove` 发起拖窗；新增 platform `SetControlsPosition(*geometry.Point)`：Integrated 红绿灯组客户区 DIP 定位、nil 恢复默认，保留原生按钮；Windows/X11 返回不支持。补充几何/生命周期测试和样例，GUI/UI 尚未接入
- [x] 拆分原生交互能力：Windows SetHitTest；Linux/macOS 显式 BeginMove，Linux BeginResize；GUI 转发与 native-first 样例，能力/输入生命周期测试及 Xfce 1x/2x 真实输入验证（Windows/macOS 交互待真机验证）
- [x] 原生 X11 状态流程与 2x 测试；Windows/macOS 测试目标交叉编译（交互和按钮占位待真机验证）
- [x] GUI WindowChrome 主动信号服务：信息通知、顺序覆盖的区域查询；移除 GUI HitTest/BeginMove 等转发，Integrated 不支持时回退 Native
- [x] Window 唯一拥有 Controls：Chrome 在内容布局后只查询顶部区域高度并报告 ControlsBounds；HeaderBar 单 child、多实例、自动避让；自定义拖动区域信号与 gui.Pick；自绘按钮复用事件分发并单独进入窗口语义快照
- [x] HeaderBar 保守自动拖动判定：命中路径上的焦点能力及控制器分类，GUI 信号覆盖；UI HeaderBar 单 child 绑定、默认值恢复及 Captions 按 ID 精确命中扩展，不修改公共 Widget/EventController 接口
- [x] GUI RequestState 改为无返回值的尽力请求，观察状态与状态信号仍来自平台；新增分段标题栏 GUI 样例及服务/控件回归测试
- [x] 上一版 WindowChrome 三平台验证通过（用户确认）
- [ ] 本轮窗口拥有 Controls 的 GUI 真机回归：Windows Snap/按钮输入；macOS 原生定位；多 HeaderBar 自动避让及 HiDPI
- 设计：[GUI Chrome 信号服务](DesignGUIWindowChrome.md)、[Window Chrome](DesignWindowChrome.md)、[Desktop Window](DesignWindowManagement.md)、[Hit Test](DesignWindowHitTest.md)；平台实现与 GUI 组合、真机验收分开推进。
- [ ] 查找栏、确认层、Toast 等窗口内覆盖内容的现代样式，不查询/猜测系统圆角半径
- [x] Popover 外观按 P1 六步实现；旧 PopupAppearance 方案已撤销，验收范围见对应设计
- [x] Windows/macOS opt-in Integrated 平台实现：Windows 按 ModernWindow 使用 POPUP/THICKFRAME/MAXIMIZEBOX、DWM 顶部 1px 扩展，最大化客户区使用显示器工作区；None/Integrated 初始与最小尺寸不做标准边框补偿。macOS FullSizeContentView 保留原生按钮；Linux 明确不支持。独立样例与几何/生命周期测试已补齐
- [ ] Windows/macOS Integrated 真机验收：全部 Painter、1x/2x、原生边缘/阴影、最大化、按钮占位、Snap、系统命令和 AppKit 交互
- [ ] macOS 输入/按钮定位真机回归：未调用 BeginMove 不隐式拖窗，标题栏按钮点击不误拖；自定义位置、nil 恢复、缩放/resize、全屏往返与原生按钮动作/可访问性
- [ ] Linux 完整 None/CSD GUI 组合：复用 P1 透明和输入区域，补阴影占位、缩放热区、光标及窗口状态适配；不伪造平台 Integrated
- [ ] 保留平台原生拖动、最大化、Snap、系统菜单和窗口按钮行为

## P3 · 高级 GUI 控件

- [x] WidgetInfo 具名语义属性（2026-10-05）：Attributes／SetAttribute、TableInfo／HierarchyInfo 归 widgets；既有公共字段和输入路径保持。迁移包内及窗口断言，DevServer 在完整编码成功后发送响应，失败返回明确 HTTP 500；契约见 [Widget Snapshot](DesignWidgetSnapshot.md)。
- [x] 本轮属性扩展的全仓／race／无桌面快照测试、Windows amd64／macOS arm64 编译和本机 Software 1x Table UI／Tree GUI 的 Snapshot 集成验证通过；结果统一记录在 DesignWidgetSnapshot §9，不扩大为原生交互全面复验。
- [x] 2026-10-04 通用交互扩展：ContextMenuEventController、UI OnContextMenu／Label.WrapMode；公开 TreeExpander、整行委托及 RowAt；DragMotionEventController、ScrollView 拖放自动滚动、嵌套每轴仲裁及静止指针再次命中。未改 ListView 公共契约或文件管理业务。全仓、相关 race、Windows/macOS 编译与原生相关逻辑测试通过；Linux Software/OpenGL、Windows Direct2D、macOS OpenGL 的 Integrated 1x 核心菜单／滚动／投递／取消通过。新窗口例程和具体缺口见 [交互扩展设计](DesignInteractionExtensions.md#7-进度与验证记录)。
- [x] TableView 详细设计（2026-10-04）：见 [TableView](DesignTableView.md)。稳定行 ID、列委托、固定表头、行虚拟化、选择／排序请求／菜单、UI 与样式；明确不新增 InvalidateMeasurements，复用 RequestLayout／Refresh。
- [x] TableView 前置（2026-10-05）：独立回归验证局部布局失效、Refresh 与离屏估计；复现并修正 ListView 宽度失效后的定位前缀，Reveal 完成要求有效实测行。未新增衍生控件特例或公开清缓存接口。
- [x] TableView 首版代码与本机主要验收（2026-10-05）：widgets／widgets/ui、独立样式、TableInfo 快照、包内回归及 tests/window/widgets/table 已实现；全仓、相关 race、无 DISPLAY 表格测试、Windows amd64／macOS arm64 编译通过；Linux/Xfwm4 Software 1x UI、OpenGL 1x GUI、Software 2x UI 的已操作场景通过。具体范围见 [验证记录](DesignTableView.md#13-首版实现与验证记录2026-10-05)。
- [ ] TableView Windows／macOS 原生桌面及完整 2x 菜单／拖宽验收；Linux arm64 额外编译被既有 libc SYS_PIPE／SYS_POLL 绑定阻塞，未在本切片修改平台。
- [x] TableView 边框／背景补充（2026-10-05）：独立内外水平／垂直线、斑马色、自定义单元格内容安全区域；样式、布局、包内及 1x/2x 像素回归、窗口例程已完成。全仓、相关 race、无桌面表格测试、Windows/macOS 编译与本机 Software/OpenGL 指定窗口场景通过；具体范围及原生待验收见 DesignTableView §14。
- [x] TableView 正文填满（2026-10-05）：取消固定 Padding，保持独立网格空间和表头布局；UI 私有承载容器采用 FillLayout。Notes 编辑列按 RowID 保存文字，独立直角焦点样式与 F6 几何／四角检查已加入，旧问题回归已复现并通过；契约见 DesignTableView §15。
- [x] 本轮 Cell 填满与 Notes 例程的全仓／race／无桌面逻辑、Windows amd64／macOS arm64 编译通过；本机 Software 1x GUI／UI、OpenGL 1x UI、Software 2x UI 的已操作场景通过，范围见 DesignTableView §15.3。
- [ ] 本轮 Notes 编辑列 Windows／macOS 原生桌面及 Direct2D／OpenGL 2x、IME／编辑菜单验收；不以交叉编译或旧例程结果替代。
- [x] TreeView 详细设计已写入 [TreeView](DesignTreeView.md)，2026-10-03 按实际实现同步借用上下文、生命周期与验证记录。
- [x] TreeView 通用前置能力：ListView.Refresh／Reveal 复用 ScrollIntoView／ScrollView，修正旧 delegate 解绑、回收及语义顺序；BuildContext 借用所属节点、动态获取当前 Root，延迟完成经 GUI Post 执行。ui.ListView 同步修正，Coordinator 不增加构建接口，独立回归已补齐。
- [x] TreeView 模型及命令式控件：TreeModel／TreeStore、状态归一化、行复用、输入、按需加载、菜单和生命周期；复杂控件留在 widgets，不向 GUI 内核加入树特例。
- [x] TreeView UI、样式和语义：widgets/ui 链式绑定、widgets/style 兜底／Modern 外观、HierarchyInfo；包内测试与 tests/window/widgets/tree_view GUI／UI 例程已实现。本机 Linux/X11/Xfwm4 Software 1x UI、OpenGL 2x GUI 核心交互通过；Windows amd64／macOS arm64 例程交叉编译通过。
- [ ] TreeView Windows／macOS 原生桌面验收：选择／导航、平台菜单触发、窗口 resize、主题／缩放和行内控件；不以交叉编译代替真实操作。
- [x] TreeView 质量调整：区分状态／内容／结构刷新，保留同 delegate 行池和离屏宽度估计；修正状态通知重入、右键子控件优先级；补齐 selected／expander 交互样式，current 仅画边框，Reveal 完成判断统一到 ListView。验证记录见 DesignTreeView。
- [x] TreeView Modern 中性高亮（2026-10-04）：selected 三态背景及 current 边框不再跟随主题色，仅跟随亮暗模式；保留样式覆盖、圆角和交互状态。对应调色板及回归说明见 DesignTreeView。
- [x] ListView／TreeView 模型安装边界（2026-10-04）：命令式 SetModel／SetDelegate 显式重新设置，UI 按稳定模型指针避免重复安装并复用 Delegate；移除 sameListObject／treeSameObject，补齐订阅计数、重置／保留状态及不可比较值回归。验证见 DesignTreeView。
- [x] 内存文件管理例程（2026-10-04）：example/filemanager 用现有公开 API 组合树／列表拖放、同级排序、整棵子树移动、多选去重和自定义反馈；模型及 Snapshot 回归、全仓测试、本机 Software 1x 主要操作与 OpenGL 1x 排序／亮暗通过。Windows／macOS 仅交叉编译；整行外壳、悬停展开和拖动自动滚动限制见 DesignTreeView 第 10 节，未扩展控件 API。
- [x] 文件管理例程右键菜单：树复用 ContextMenu，右侧稳定 ListView 使用例程专属 View／控制器；打开、信息、展开／折叠及选择集合移动，边界／只读禁用和过期命令再校验。包内测试、race、全仓及本机 Software 1x 操作通过，Windows／macOS 仅交叉编译；详细验证见 DesignTreeView 10.5。
- [x] 文件管理例程宽度绑定：StartSize 使用应用状态，OnResize 保存用户调整；选择／菜单／重建／亮暗及数据重置不重置左侧宽度，不改 SplitView 契约。回归、race、全仓及本机 Software／OpenGL 1x 验证通过，Windows／macOS 仅交叉编译；见 DesignTreeView 10.6。
- [x] 文件管理例程标题栏组合：HeaderBar 位于无留白的窗口顶部，正文独立保留 Padding 并占满剩余高度，恢复 Linux 控制按钮居中；未改框架定位契约。布局声明回归、全仓／race、本机 Software／OpenGL 1x 与交叉编译通过，见 DesignTreeView 10.7。
- [x] 快捷键基础按键语义：Command、Win、Super、Option、Alt 明确区分，原生映射与文本编辑/gui-dev/日志同步；Windows/macOS 真机输入待验收，见 [Shortcuts](DesignShortcuts.md)
- [x] GUI 独立 KeyModifiers 与 Resolve 转换：Primary/Control/Alt/Shift 语义及平台专属修饰键，文本编辑复用转换。
- [x] 基础逻辑字母布局适配与 GUI Key 转换；GUI Shortcut/ShortcutController 与窗口级作用域逻辑实现，见 DesignShortcuts.md；不代表原生窗口验收完成。
- [x] 原生文字默认动作响应与快捷键消费衔接：Windows pre-Translate、macOS input context 未消费命令兜底、模态 owner IME 隔离；逻辑回归已覆盖，原生交互待验收。
- [x] 快捷键本机窗口验收：Linux X11/Xfwm4、Software 1x；优先级、重复/启停/重建、AZERTY/QWERTZ、CapsLock、AltGr/死键、中文 XIM 提交与取消通过，见 DesignShortcuts.md。
- [x] 修正上述首次验收未覆盖的正常按住时序：XIM 过滤前更新重复状态导致 local-F5 不激活；改为只跟踪过滤后的有效按键，补齐逻辑回归，并在 Fcitx 中/英文模式验证 150ms F5、长按抑制、F6 重复及局部/窗口焦点切换。
- [x] 快捷键远端无头验收收尾：macOS 按键/GUI/UI 测试通过；Windows 此前 EXE 启动失败，本轮按键/GUI/UI 逻辑复验已执行通过。按 2026-09-22 约定不要求远端桌面，Windows/macOS 窗口输入仍明确保留为未覆盖项。
- [x] 左右修饰键正确性：Windows 消息状态查询、macOS 事件内侧别标志、X11 modifier map + KeymapNotify 快照；双侧释放与焦点恢复逻辑、Linux 原生 Ctrl/Shift/Alt、XIM F5 回归通过，Windows/macOS 无头逻辑通过。另修复 X11 AltGraph 跨布局组误匹配；不扩展 KeyGesture 侧别 API，真机覆盖见 DesignShortcuts.md。
- [x] UI 通用 ViewBase/WindowView 快捷键列表协调与 tests/window/ui/shortcuts 用例；逻辑重建/移除验证，原生操作待验收。
- [x] UI Shortcut 声明统一为 `Shortcut(key).Modifiers(...).OnActivate(...)` 链式指针描述对象，移除 UI KeyGesture 别名；默认值、修饰键替换、回调更新、位置协调与移除回归及 race 通过，Linux 全仓测试通过，Windows amd64/macOS arm64 窗口用例交叉编译通过。本轮未重跑真实桌面输入验收。
- [x] TextView/TextInput 共用 PopoverMenu 编辑上下文菜单：右键/Shift+F10、命令可用状态、焦点/IME/历史与生命周期保护；全仓与相关 race 通过，Linux X11/Xfwm4 Software 1x 窗口及 Windows/macOS 无头验证通过，详情见 DesignTextEditing.md。
- [ ] 编辑上下文菜单 Windows/macOS 桌面验收，以及 Linux 合成器开启、OpenGL/2x、真实中文候选窗组合验收；操作见 tests/window/ui/textediting 第 10–11 项。
- [x] X11 Level3/AltGraph 实际修饰位与 GUI 显式声明；编辑命令精确匹配，避免额外修饰位误触发。

- Overlay 已在 P1 实现为普通容器，不作为原生 Popup 的隐式 fallback。
- [x] SplitView 基础：`widgets` 两区可嵌套容器、按 Child 测量建立初始比例、StartSize／EndSize／Ratio、约束退化与首选尺寸恢复；`widgets/ui` 双 Bin 协调、独立样式、真实拖动／键盘调整和 Snapshot Range，见 [SplitView](DesignSplitView.md)。全仓及相关 race、本机 X11/Xfwm4 Software 1x UI、OpenGL 2x GUI 与兜底样式验证通过。
- [x] SplitView 外观修正：各状态 1 DIP 中性色默认线；应用可覆盖 Hovered／Pressed 颜色，焦点不参与线色选择。保持 9 DIP 命中和键盘焦点，包内回归及本机 X11/Xfwm4 Software 1x 拖动后外观／键盘调整通过。
- [ ] SplitView Windows/macOS 桌面交互验收：两平台窗口程序与测试目标交叉编译通过，操作见 `tests/window/widgets/split_view`；不将语义快照等同于 OS 无障碍桥接或完整 Tab 导航。额外 Linux arm64 编译被既有 libc SYS_PIPE／SYS_POLL 绑定错误阻塞，本切片未修改平台代码。
- [x] 文本编辑设计确认并写入 [TextEditing](DesignTextEditing.md)：先多行、后单行复用；共享 TextModel、独立视图、现有 Cluster、外包 ScrollView
- [x] 文本编辑参考调整为 Scintilla 为主、Monaco/VSCode 为辅；保留 Piece Tree，同步现有 LRU、按需几何和原生布局复用的设计描述
- [ ] 按 TextEditing 第 4 节建立低延迟/资源预算对照，再评估失效范围和同步排版优化；不扩大首版文件规模及超长段落承诺
- [x] 三平台 Cluster 度量与原生字符输入契约修正：Linux/Pango 测试通过，Windows/macOS 交叉编译通过
- [x] macOS 15.7.7 arm64 CoreText 原生离屏度量验证；包含 emoji、连续空行、末尾换行及 CRLF
- [x] Windows 10 22H2 amd64 DirectWrite 原生离屏度量及 1x/2x 预乘透明度；win32 字符组装/提交来源/生命周期测试实际通过
- [ ] 用户验收：Windows/macOS 窗口提交、IME/Clipboard、选择及 HiDPI；按 tests/window/gui/textview、tests/window/ui/textediting 入口步骤执行，不阻塞开发。平台原生离屏由 remote-shell 验证
- [x] TextModel 基础 piece tree、逻辑行索引和共享编辑历史：随机编辑参考、合并/反向删除、重入与 1 MB 文档测试通过；TextView 私有入口记录撤销组选区，通知按 revision 排队
- [x] TextModel 独立有界块与直接引用回收；ClearHistory/放弃 redo、连续输入/删除历史合并、重复替换的可达存储测试及 1/10 MiB 编辑周期基准
- [x] DevServer ref、Popover 与文本提交：设计见 [DevAutomation](DesignDevAutomation.md)。编号和查询留在 dev；GUI 仅提供 Popovers / CommitText。全仓及相关 race 通过，本机 X11/Xfwm4 Software/OpenGL × 1x/2x 协议窗口验收通过；Windows/macOS 编译通过，原生运行和真实 IME 组合验收未执行
- [x] TextInput/TextView 共享私有 textEditor；单行独立模型、Cluster 选择、水平滚动、稳定 baseline、只读与 Submit
- [x] 与 Widget 无关的模型、存储、编辑几何、段落高度、预编辑投影及位置算法拆入 gui/textedit；GUI 保留类型别名，通过明确编辑/历史结果接口使用独立包，纯算法测试随包迁移
- [x] 文本编辑测试按 editor/input/view 集中，模型存储与通知测试并入 model；保留平台入口及原有断言，声明 token 一致性已核对。原样验证受既有缺失 time 导入阻挡，临时 overlay 与跨平台验证结果见 TextEditing 设计
- [x] TextView 多行布局、选择、Clipboard、IME、滚动和 undo/redo（开发实现及本机验证；其他平台窗口验收单列）
  - [x] 视口段落缓存、Cluster 光标/选区、增量模型绑定、只读、基础编辑/剪贴板、撤销选区恢复、范围标记 Snapshot；逻辑与 race 测试通过
  - [x] ScrollIntoView 通用信号与布局后合并；远距离导航在实际高度修正后继续保证光标可见
  - [ ] 后续扩展滚动能力时优先评估独立共享滚动模型：参考 GTK4 Adjustment，区分目标可见性、锚点修正与主动滚动；当前接口保持不变，见 [滚动设计](DesignScroll.md#后续扩展优先评估独立滚动模型)
  - [x] tests/window/gui/textview 独立用例；Linux Software 1x 的 ASCII/Enter、Shift 选区及首尾导航验证
  - [x] IME 预编辑投影/取消/原子提交、Timer 闪烁/自动滚动、模型编辑及重排锚定；包内回归与 race 通过
  - [x] TextInput/TextView modern 正文、selection/caret 样式；亮暗和极端主题色对比度测试
  - [x] 共享编辑核心按视觉端点收起 RTL 选区；独立度量及真实 Pango 离屏导航、扩选、删除/撤销、组合字符回归
  - [x] 同一原生排版编辑回归增加 CoreText/DirectWrite 入口；macOS 15.7.7 arm64、Windows 10 22H2 amd64 的单行/多行实际通过
  - [x] 正文 clip 顶部锚定及小数行高收敛修复；独立预期回归、Pango/CoreText/DirectWrite 连续 resize 原生离屏验证通过，窗口用例可选编号段落
  - [x] 连续宽度往返保留原逻辑字符；三平台原生回归通过。修正 Pango 栅格化重排与复制游标的跨 run 度量错误，补充按行着色像素及度量不变回归
  - [x] Pango 颜色/字体/下划线/删除线范围覆盖改用原生 AttrList.Change；嵌套覆盖、透明清除和范围外保留的 1x/2x 像素及字体度量回归通过
  - [x] Linux OpenGL（llvmpipe）2x / 10 MiB 编号窗口宽度往返复验；显式光标导航优先于浏览锚点，整数 DIP 边界请求收敛，三平台原生离屏回归及 Linux OpenGL/Software 2x 末尾导航通过
  - [x] Linux XIM 支持样式协商及内联预编辑回调；增量字符/UTF-8 caret、Reset/销毁回归与 race 通过
  - [x] Linux OpenGL（llvmpipe）2x：1 MiB Clipboard 精确核对，Fcitx 内联选区替换、取消、普通输入与组合提交分组验证
  - [x] 原生组合提交来源经 IMCommit 传递；XIM root-style 相邻输入 undo/redo 验证；Linux Software 2x 提交与 undo 验证
  - [x] Fcitx callback 候选定位明确 libX11 版本限制：1.7.5 拒绝，临时加载 1.8.4 验证跟随；不修改系统库或补偿坐标
  - [x] Software 形状/clip 无交集时跳过绘制；填充/描边/阴影/旋转图片 1x/2x 像素回归，原生 TextView 末页全选不污染状态栏
  - [x] XIM 服务销毁通知失效 XIM/XIC 并取消预编辑；Software 2x 原生失联后选择/复制/关闭与包内生命周期回归通过
  - [ ] XIM 原生同步调用期间服务退出：libX11 等待回复不处理销毁事件，Reset 阻塞仍为已知限制；不与失效句柄修复混为一谈
  - [ ] 用户验收：IME/Clipboard/双向导航、计时器交互及 HiDPI 的跨平台窗口场景；不作为后续开发前置条件
- [x] UI TextView/Model 与 TextInput 选择/只读/Submit 薄适配；重建状态保持、连接清理和 Snapshot 覆盖
- [x] UI TextView 文本值绑定与两类编辑器 BindSelection；直接协调已有 TextModel，保留 Model/OnChange 高级路径，不新增 GUI API
- [x] TextInputView/TextViewView 可选声明统一采用独立 bits.Bitmap；保留属性 bool，覆盖显式零值、默认值恢复和基础修饰符隔离
- [ ] 文本/选区值绑定的窗口验收：Notes 输入、IME 组合、回传重建、撤销/重做、外部替换与范围选择；保留共享模型 1/10 MiB 场景
- [x] tests/window/ui/textediting；Linux Software 1x 单行 Submit、重建保持、主题切换与共享编辑/撤销阶段验证
- [x] UI Linux Software 2x / 10 MiB：全文剪贴板参考核对、单行归一化/Submit、重建/主题/只读、IME 取消/提交/撤销、原生 Cluster/RTL 与组合期间关闭验证
- [x] UI Linux OpenGL（llvmpipe）2x / 10 MiB：重建、亮暗/主题色切换后的内容与选区保持，全文 Clipboard、组合字符/emoji、RTL 删除撤销核对
- [x] Linux 普通键入环境核查：缺失服务的 @im=fcitx 导致 XOpenIM 失败；同一独立桌面改用 @im=none 后 UI OpenGL 2x 逐键 draft/Submit/复制精确核对通过，补充 GUI/UI 用例启动条件
- [x] Linux UI Software 1x：fr AZERTY 原生字符、死键与 AltGr 输入，单行和 1 MiB 共享多行全文独立核对；运行中切换 us intl 后 Compose 字符提交通过，补充窗口用例操作步骤
- [x] 文本编辑开发交付：补齐三平台窗口运行说明及 GUI -accept-tab；本机插入/撤销/默认不消费/只读回归，Windows/macOS GUI+原生排版+输入工厂离屏复验、相关 race 和三平台窗口目标编译

## P4 · 系统集成

- [x] 可选 `cmd/goui`：TOML、doctor/generate/build/package，Windows EXE / macOS app / Linux dir；见 [构建打包](DesignBuildPackaging.md)
- [x] 命令逻辑／资源像素／文件保护／真实 Go 编译与打包测试，全仓与命令包 race；Windows amd64、macOS arm64、Linux amd64 工具／窗口目标编译；Linux X11/XFCE Software 1x 安装树二进制启动／自动关闭、WM_CLASS 与 desktop 文件校验
- [x] Windows 10 22H2 amd64 / macOS 15.7.7 arm64 原生构建、图标及启动／缩放／关闭验收；不自动安装或修改安全设置，具体范围见构建打包设计
- [x] macOS `/var` 与 `/private/var` 别名导致的命令测试误判修正：采用实际目录身份比较，旧测试原生复现失败，新测试、完整命令包、race / vet 原生通过；本机全仓及命令包 race / vet 通过，不修改生产代码
- [ ] Linux 启动器安装集成；Linux arm64 工具／窗口编译被已知 libc SYS_PIPE/SYS_POLL 阻塞。不含权限、签名、DEB/RPM 或自动安装

- [x] OpenURL/OpenPath 三平台原生实现、GUI/UI 运行时命令及错误反馈；设计见 [外部打开](DesignExternalOpen.md)
- [x] 输入、URI 转换、原生错误释放、GUI/UI 错误透传与退出测试；相关 race 测试及 Windows/macOS 交叉编译
- [ ] example/external_open 三平台真机打开验证（浏览器、文件、目录、自定义协议）
- [x] FileDialog 已迁移访问资源与操作错误结果，见 DesignFileAccess
- [ ] Clipboard 错误反馈尚未迁移，见 [未来路线](Roadmap.md#future-clipboard)；不改 Settings 的有效默认值策略
- [ ] Application 单托盘图标、可访问名称和点击信号
- [ ] 三平台运行期本地通知能力探测与发送
- [ ] Windows 只使用可动态加载的系统能力，不增加强制 SDK runtime

- [x] X11 拖放预览与动作光标分离；tab_detach/tab_transfer 默认连续体验，显式参数保留严格验收。第 27 节原单标签限制已按第 28 节改为允许移入已有窗口。无合成器窗口外观仍未验收。

## 持续要求

- [ ] 每项平台能力按 `platform -> gui -> ui` 推进
- [ ] Direct2D、OpenGL、Software 的坐标、clip、Transform 和 HiDPI 语义一致
- [ ] 新控件覆盖布局、输入、焦点、信号、生命周期和语义快照
- [ ] 每个切片提供自动测试和可重复的三平台真机验收步骤
- [ ] 完成修改前运行 `go test ./...`

## 已验证基础

- [x] Linux 绑定保持分包：XSync 按值结构体改用 cgo，Cairo/Pango/GLX/Xlib 的 C int 返回值显式按 int32 接收，GLX 四组 int* 输出修正为 int32；核心库缺失允许 panic，可选扩展支持判断保留。
- [x] 本轮 Linux x64 无桌面 ABI 验证：`TestFormatStrideForWidth`、`TestSwapIntervalSignedResult`、`TestValueABI` 及三个包 race 通过；Pango 离屏测试通过。用 Git HEAD 旧实现的 Go overlay 重跑 Cairo/GLX 回归，两者均因把 -1 解释为 4294967295 失败，现实现通过。
- [x] 本机 `go test ./...` 通过；涉及的 Cairo/Pango/GLX/Xlib/XSync 包及 Cairo/GLX/Xlib/XSync 测试目标 Linux arm64 交叉编译通过。未进行 arm64 原生运行或新增桌面交互验收；全仓运行保留既有桌面依赖和图片输出测试。
- [ ] Linux arm64 全仓编译仍被既有 `platform/linux/libs/libc/syscall_linux.go` 阻塞：29 行 `syscall.SYS_PIPE`、61 行 `syscall.SYS_POLL` 未定义。`GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go build ./...` 及单独构建 libc 均失败；该文件与 HEAD 一致，本轮未改动内核 syscall 路径。

- [x] Windows POINT 按值调用和 OLE POINT 回调统一使用 goexlib/cgo，绑定层移除手工坐标打包及架构分支。
- [x] Windows x64 真机无窗口 ABI 回归（Windows 10 19045，2026-10-01）：`TestPointValueABI`、`TestDropTargetPointABI`、`TestDragSourceIdentity` 通过。独立整数调用端核对 POINT 布局/按值调用、负坐标和边界值、64-bit 句柄、OLE 回调寄存器/栈参数及 HRESULT；本轮不重复拖放交互验收，后端/缩放不适用。
- [x] 本轮本机 `go test ./...`、Windows amd64 两包测试目标交叉编译通过；远端执行前校验测试归档 SHA-256。Linux/macOS 无实现变更，未重复远端验收。

- [x] 三层架构与跨平台 Window/Popup/EventLoop
- [x] Path、Transform、clip、线性渐变、BoxShadow 和 HiDPI
- [x] 持久 Image 与 TextLayout/Painter 文字图片缓存
- [x] 约束布局、LinearBox、ScrollView、ScrollBar、ListView
- [x] Button、Label、Image、TextInput 和 Menu
- [x] IME、Clipboard、Settings、FileDialog
- [x] 声明式 `ui`、语义快照和开发自动化协议

</details>

## 通用文本扩展与 SourceView 接入复查（2026-10-16）

- [x] 保留并验证批量编辑、三端 Tab 间隔和剪贴板结果接口；迁移已有调用方。
- [x] 修正 Pango 绑定边界、Windows 写入所有者、X11 期限/退出/迟到回复及 UI 受理结果。
- [x] 无文本/空文本统一为 ("", nil)，空粘贴不删除选区，不增加历史或错误信号。
- [x] 本机全仓、相关 race、两台远端原生 Tab/剪贴板及 SourceView 联调已验证。
- [x] 分别记录 Windows/macOS 纯基线可复现的完整测试失败、既有 vet 诊断和独立发布限制；
  详见 [实施与验收记录](PlanSourceViewExtensions.md)。未将这些失败算作通过。
- [ ] GOUI 发布包含这些接口的版本后，更新 SourceView 依赖并完成关闭 workspace 的独立验证。


## DirectWrite 全范围前景覆盖（2026-10-16）

- [x] 复现完整重着色累积旧颜色范围；保持公开接口及部分覆盖语义，完整覆盖时回收旧记录。
- [x] Windows 原生回归修复前失败、修复后 1000 轮记录数有界；DirectWrite 包测试通过。
- [x] Linux GOUI 全仓测试及 SourceView 三端原生度量/位图联调通过，细节见 DesignTypography 第 14 节。
- 修正提交 `227220a`；未发布，不修改已有发行版或将尚未发布的修正写成可获取依赖。

## 单层菜单键盘导航（2026-10-16）

- [x] 通用 PopoverMenu 支持上下/Home/End、Enter/Space、可用项跳过和虚拟行活动快照。
- [x] 本机全仓、远端 Windows/macOS 菜单逻辑和 SourceView 原生排版测试通过。
- [x] SourceView 在独立 X11 桌面以 Software 1x/2x、OpenGL 1x 验证菜单键盘/指针操作。
- [ ] Windows/macOS 菜单桌面验收及跨层子菜单；细节见 [PlanMenuKeyboard](PlanMenuKeyboard.md)。

## TextInput / TextView 占位提示（2026-10-19）

- [x] `1ada17e` 增加 Placeholder / SetPlaceholder、每视图缓存与资源释放、IME 显隐、
  独立语义字段及 Basic / Modern 的 placeholder 前景；模型、选区、历史及滚动范围不变。
- [x] `c6827e9` 增加 UI 两种编辑器的 Placeholder 属性；省略时恢复为空，保留编辑状态。
  契约与 Qt / GTK 文档取舍见 DesignTextEditing 的 Placeholder 小节。
- [x] 新增 `tests/window/gui/textplaceholder`：检查提示像素及裁剪、测量/基线、真实值/历史、
  焦点、共享模型、换行、字体/主题、只读/禁用、卸载/重挂载；支持截图与保留窗口检查。
- [x] 本机 Linux amd64 / Go 1.25.8 / X11 / Xfwm4 / Pango：全仓非缓存 2545 通过、
  15 既有跳过、0 失败；GOWORK=off 全仓通过；gui/ui/两主题 race 通过。
  新窗口 Software 1x/2x、OpenGL 1x 通过，桌面键盘输入、清空及共享编辑/撤销已观察。
- [x] macOS 15.7.7 arm64 / Go 1.24.9 / CoreText：受影响包通过；新窗口 Software 1x、
  OpenGL 2x 通过。Windows 10 19045 amd64 / Go 1.24.0 / DirectWrite：受影响包通过；
  新窗口从 RDP 交互会话启动，Direct2D 1x 全部断言通过并生成截图，实际 Painter 日志已观察。
- GOUI 全仓远端仍有既有失败：macOS TestWindowInteractionCapabilities 的错误值断言；
  Windows Direct2D 渐变/描边渐变像素及 Win32 TestWindowStyle 断言，均在原 903daf8 复现。
  Windows SSH 会话的原平台窗口用例和新程序初始布局超时，按会话限制记录，不计为通过。
- GOUI vet 仍报告既有测试的跨包未具名结构字面量；新增 placeholder 文件无该诊断。
  没有弱化断言或扩大 skip，也没有顺带修改平台实现。
- SourceView FindBar 已移除模拟 Label，迁移到正式接口，八语言、浮层、键盘和声明式专项
  在 Linux 通过；其 macOS 原有计数/展开高度/菜单语言场景失败在修改前两仓复现。
- [ ] 真实中文 IME、混合显示器 DPI 与三端完整后端矩阵；包内已覆盖预编辑显隐和状态隔离。
- 新程序仅创建窗口及可选 PNG。全仓沿用既有桌面测试：Windows/macOS 的 TestClipboard
  会覆盖真实剪贴板，本次运行没有保存原内容；原有窗口与图片写入副作用保持不变。

## SourceView 项目与技能入口（2026-10-19）

- [x] 双语 README 增加可选 SourceView 项目入口及其应用开发 Skill 的安装指引。
- [x] goui-app 按需指向 SourceView 仓库维护的技能；不在 GOUI 保存副本、增加模块依赖或自动安装技能。
- [x] 技能格式、Markdown 本地链接/图片路径及差异检查通过；Linux amd64 / Go 1.25.8 / 独立 X11/Xvnc/Xfwm4 测试桌面上 `GOWORK=off go test ./...` 通过（基线 `136e775`）。首次测试桌面进程退出导致显示连接失败，重启并保持服务后重跑通过。
- 本次仅修改文档和技能指引，未做跨平台桌面或实际技能安装验收；SourceView 接入/定制样例在其仓库工作区编译及测试通过，独立依赖限制由其技能入口和实施记录说明。

## Demo 文本占位提示（2026-10-19）

- [x] 文本页的现有 TextInput / TextView 分别使用单行／两行 Placeholder；新增提示和清空操作说明覆盖八语言，不替换初始样本或持久模型。
- [x] 回归覆盖用户清空后切换八语言及重建页面：内容保持为空，模型、修订及撤销／重做历史保留。同步 DesignDemo 与示例操作说明。
- [x] 基线 `6ad3254` 加本次源码的干净快照，不含其他未跟踪源码；Linux amd64 / Go 1.25.8 / 独立 Xvnc/Xfwm4 中执行 `GOWORK=off CGO_ENABLED=0 go test -json ./... -count=1`：2207 项通过、10 项既有跳过、无失败。既有测试的窗口／图片副作用限定在快照及测试桌面。
- [x] 同源交叉构建的产物经 SHA256 校验，在 macOS 15.7.7 arm64 / CoreText 与 Windows 10 19045 amd64 / DirectWrite 原生运行：Demo 57 项、resource 4 项均通过，无跳过或失败；包含新回归及八语言资源检查。
- [x] Linux X11/Xfwm4 / Software 1x 的实际 Demo 操作：清空显示提示、输入隐藏、撤销恢复原文、重做恢复空内容、只读阻止输入、跨分类保留内容，以及中／英／德和亮暗切换通过。
- [x] Windows RDP 交互会话 / 请求 Direct2D 1x 的实际 Demo 操作：单行／多行清空、输入、撤销／重做、只读、中／英及亮暗切换通过；保留正常窗口输入路径。
- [x] 复用 `tests/window/gui/textplaceholder` 在 macOS Software 1x、Windows RDP Direct2D 1x 原生执行，实际 Painter 与排版后端日志已核对，退出码 0。三种样式的提示像素／裁剪、正文／历史、焦点、共享模型、换行、字体／主题、只读／禁用及重挂载断言通过。
- macOS Demo 已启动并观察初始画面；本轮 VNC 点击和 Tab 未引发可观察反馈，Demo 的手工输入／语言／主题操作不计为通过，以上专项窗口结果不能替代它。未修改框架规避会话问题。
- 验证工具曾遇 Windows 标准输入传输停滞、PowerShell 未加引号的 `-test.v` 参数拆分，以及 zsh 的只读 `status` 变量；分别改用校验 SHA256 的临时 HTTP 传输、字面参数和 `/bin/sh` 后完成验证，未更改控件或弱化断言。
- 未执行真实中文 IME、混合 DPI、2x 及完整后端矩阵，本次也未重跑远端全仓测试或改写既有失败。测试产物和截图在仓库外；本次操作未读写剪贴板、用户文件或系统主题设置。
