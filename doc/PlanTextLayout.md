# TextLayout 能力补全实施与验收记录

状态：字体、删除线和占位代码已实现，Typography 已提交为 `d67e89d`；原生绑定
已由用户另行提交。三端原生离屏回归完成，具体范围与缺口见第 5 节，不代表所有
桌面后端／DPI 均已验收。本文与 DesignTypography 配套维护实施记录和长期设计。

## 1. 文档定位与切片范围

当前公共契约、资源归属及后端设计统一见 [Typography 设计](DesignTypography.md)；
绘制侧图片缓存见 [DesignPaint](DesignPaint.md)，上层文本编辑见
[DesignTextEditing](DesignTextEditing.md)。本文只保存本次能力补全的实施依据与记录，
不再维护第二份公共 API 定义。

该切片修复 CF 绑定、字体注册与字体样式，补齐 run 级删除线和纯度量行内占位；
没有实现 RichTextView／Markdown 文档模型，也没有将 GUI 对象传入 Typography。
具体完成情况、历史失败及未覆盖环境按日期保留；历史中的“未提交”等描述仅指当时状态。

## 2. 前置诊断与证据

此前已通过本机和远端无头程序直接创建 Typography Context，使用 Rasterize
输出普通／删除线位图及度量信息，没有创建原生窗口。

### 2.1 验证条件

| 环境 | 排版后端 | 字体与验证范围 |
| --- | --- | --- |
| Linux Ubuntu 22.04、amd64，Pango 1.50.6／Cairo 1.16.0 | Pango | DejaVu Sans，拉丁、中文、混合字号，栅格化倍率 1x／2x |
| Windows 10、build 19045、amd64 | DirectWrite | Segoe UI，相同类别的文本与栅格化倍率 |
| macOS 15.7.7、arm64 | CoreText | Arial，相同类别的文本与栅格化倍率；另测试字体 traits |

这些结果是现有实现的前置诊断，不是待实现功能的验收。栅格化倍率不是实际桌面
DPI；不同平台的默认字体也不是同字体对照。Windows 的 MiSans 注册测试没有
证明注册字体实际参与排版，不能将其当作成功的同字体比较。

### 2.2 已确认问题

| 项目 | 诊断结果 | 本轮处理 |
| --- | --- | --- |
| macOS 字体 traits | 设置权重时触发原生崩溃；CF 集合 callback 结构常量被错误读取，集合没有正确保留成员 | 修正结构常量地址绑定与生命周期 |
| Windows AddFont | GDI 注册返回成功，但当前 DirectWrite 字体集合找不到该字体，实际仍使用 fallback | 使用 DirectWrite 自定义字体集合 |
| 字体样式 | Linux／Windows 未应用 Weight、Width；macOS 支持不完整；公共接口缺少斜体 | 明确公共值模型并补齐三端实现 |
| 删除线 | 单字号位置基本合理；Pango 将混合字号的连续删除线合并为平均位置与厚度；Windows／macOS 按字体 run 表现 | 明确 run 级语义，修正 Linux 绘制 |
| 行内控件 | TextLayout 没有可参与文字排版的占位对象 | 增加纯度量接口 |

Pango 的混合字号删除线表现有原生源码依据，不是 GOUI 额外加入的偏移。
源码的合并与绘制逻辑见 [Pango 1.50.6 pango-renderer.c](https://github.com/GNOME/pango/blob/1.50.6/pango/pango-renderer.c#L284)。
本轮选择字体 run 度量作为 GOUI 删除线的契约，不要求三端使用同一份像素金图。

## 3. CF 绑定修复记录

该阶段先修正 CF 绑定，没有在字体创建代码中额外 retain 来掩盖集合 API 的错误。

涉及文件：

- `platform/darwin/frameworks/utils/load_darwin.go`
- `platform/darwin/frameworks/core_foundation/core_foundation_darwin.go`
- 对应绑定测试与 `platform/typography/coretext` 回归测试。

修改点：

1. 统一使用 `Const[T]`，保留 `PVar *T` 值输出，增加可选的 `PAddr *uintptr`
   符号地址输出；两者可单独或同时指定，至少提供一个。符号只查找一次，
   仅在 `PVar` 非空时读取内存，查找失败不改写输出。
2. CFDictionary、CFArray 的 callbacks 使用对应原生结构体的地址，而非读取结构首字段。
3. 核对原生结构大小、字段顺序、参数类型和生命周期。
4. 原生常量地址为 framework 生命周期内的借用资源，不执行 CFRelease。
5. 仅审视同类结构常量绑定，不扩大为整个 frameworks 重构。

CFStringRef／CFBooleanRef 等指针变量使用 `PVar` 获取其保存的引用；callbacks
结构体使用 `Const[uintptr]{PAddr: ...}` 直接获取原生结构地址，不读取首字段。
不再保留独立的 `ConstAddress` 类型，也不增加加载模式开关。`PAddr` 不转移
原生资源所有权；`PVar` 复制值本身也不隐式 retain 所指向的对象。

CF 集合是否保留成员取决于传入的 callbacks，见
[Apple CF 集合创建说明](https://developer.apple.com/library/archive/documentation/CoreFoundation/Conceptual/CFCollections/Articles/creating.html)。

## 4. 验证组织

长期回归要求见 [Typography 测试与差异](DesignTypography.md#11-测试要求与当前差异)，
全仓组织规则见 [Testing](Testing.md)。本切片按以下步骤实施：

1. 本机 Linux 原生包内测试与必要的旧实现缺陷复现。
2. Windows／macOS 无头运行同源测试，校验产物 SHA256。
3. 在隔离桌面及仓库外同源快照运行 `go test ./...`，保留既有测试的窗口／PNG 副作用。
4. 受影响目标交叉编译，以及按风险执行 race／ABI 回归。
5. 真实桌面、系统 DPI、硬件 GPU 及未覆盖字体外伸边界独立记录，不用离屏结果代替。

测试只证明实际运行的输入、版本与环境。交叉编译不等于原生运行，栅格化倍率不等于
桌面 DPI；既有跳过与未执行都不记为通过。原生调用的线程亲和与夹具生命周期也属于
验收前提，不能因无需窗口而忽略。

## 5. 阶段、提交与验证记录

| 阶段 | 内容 | 完成标准 |
| --- | --- | --- |
| 0 | 固化本文中的契约、修改边界及证据 | 设计文档完成 |
| 1 | macOS CF 结构常量绑定与生命周期 | 原生无头回归通过，独立修复提交 |
| 2 | 三端字体注册，重点修正 DirectWrite | 固定字体实际匹配成功 |
| 3 | FontInfo 样式值模型、三端实现及调用点迁移 | Regular／Bold／Italic 等一致生效 |
| 4 | 删除线契约及 Linux run 级绘制 | 混合字号不再采用平均位置，度量不变 |
| 5 | 行内占位接口及三端实现 | 排版正确，对象不由 Typography 绘制 |
| 6 | 全量验证、文档与进度同步 | 记录验证范围及剩余缺口 |

上表保留当时的阶段规划，不要求后续重新执行。实际绑定由用户按平台拆分提交，
Typography 接口、三端实现与必要测试按依赖整合为 `d67e89d`
（`Update typography font and placeholder support.`）；不能把原规划写成实际提交顺序。

继续在本文记录新验收及范围，在 Plan／Roadmap 维护进度和未验收项；不把前置
诊断结果追认为新实现验收。后续提交、推送或 Release 更新仍需用户授权。

当前进度：

- [x] 实施计划保留于本文；当前长期契约已拆分至 DesignTypography。
- [x] CF 绑定与生命周期修复：macOS 15.7.7 arm64 无头回归通过。
- [x] 字体注册完善：Linux 实际匹配、Windows 10／macOS 原生注册回归通过。
- [x] 字体样式接口与实现：公共枚举、三端映射、完整范围替换及实际 face 验证。
- [x] 删除线语义与 Linux run 级绘制；保留 Windows／macOS 原生绘制。
- [x] 行内占位对象：三端原生排版接入，复制纯度量，不绘制替代字形。
- [x] 三端原生离屏验收与进度同步；桌面／硬件 GPU 验收不在此项中冒充完成。

### 阶段性验证记录（2026-10-14）

- 阶段 1：同源旧实现在 macOS 15.7.7 arm64 中三个 callbacks 地址均为零，
  集合没有 retain 成员；字体 traits 子进程触发 SIGTRAP。修正地址加载后，
  CF 集合测试连续 10 轮、CoreText 现有离屏测试与 traits 回归通过。
- 新增常量地址入口不改变 CFStringRef／CFBooleanRef 的值加载；callbacks
  地址借用 framework 静态资源。没有向 GUI／Painter 添加补偿 retain。
- 阶段 2 发现 Linux 历史 PangoFc 符号错误绑定到 libpango，查找失败后返回零；
  改从 libpangoft2 加载，在 Context 独立 Fontconfig 配置中注册并通知映射更新。
  Go 字体实际匹配（先创建 fallback，再注册）、重复注册及失败后可用性本机通过。
  不采用额外重设配置或延时刷新 workaround。
- 阶段 2／3：经 SHA256 校验的同源测试在 Windows 10 19045 amd64 和
  macOS 15.7.7 arm64 中通过注册与 Regular／Bold／Italic 基础／范围栅格化、
  恢复普通字体、无效字体请求、默认字体回归；不创建桌面窗口。
  Windows 初次新增调用发现 untyped nil 不符合 cgo.Call 参数要求，已改为
  显式空指针值并重新原生执行通过；未改用 GDI 注册或高版本 Factory5。

### 最终实现与验证范围

- 字体夹具来自既有 `golang.org/x/image/font/gofont`（Go 项目 BSD 许可）；
  在仓库外生成 Regular TTF 和含 Regular／Bold／Italic 三个 face 的 TTC。
  Linux 检查实际 Pango face，macOS 检查 PostScript 名，Windows 检查自定义集合
  的匹配 face 与 simulations=0，不仅比较位图或 AddFont 返回值。
- 注册覆盖无效内容、缺失文件、重复文件、失败后集合保持可用；Windows 使用
  Factory3 + FontFile.Analyze 逐 face 加入，先完整创建新集合再替换。
  macOS 注册为进程级，Context 不注销共享字体；测试文件保留到整套测试结束，
  避免注册仍有效而单个 t.TempDir 已被删除的夹具错误。
- 字体范围覆盖基础／范围一致、恢复 Regular、布局隔离、默认请求、无效请求；
  typed Weight／Width／Style 仅在原生边界映射，不增加绘制模拟。
- 删除线用空格中部采样、预乘 RGBA、原始字体 OS/2 度量独立核对 Pango 与
  DirectWrite。DirectWrite 的 offset 是上沿，Pango 是中心；CoreText 未公开
  删除线位置查询，使用单字体独立原生 frame 作参考，不猜测内部算法。
  1x／1.5x／2x 允许最多 1 物理像素的位置误差（原生 grid fitting），不放宽 alpha。
  临时快照恢复旧 Pango 删除线后，混合字号用例明确失败；修正后通过。
- 行内对象覆盖单个／相邻对象、换行、Hebrew RTL、emoji 后的 UTF-8／UTF-16
  偏移、更新／清除、零宽对象、度量指针复制、布局隔离及连续 50 次替换／栅格化。
  对象位图保持透明。DirectWrite 使用 C 堆上的 numeric-only COM 对象，由原生
  引用计数释放；CoreText delegate 的 numeric-only refCon 由原生 deallocate
  回调释放；Pango 复制 shape 矩形，不把 Widget／Go 栈指针交给原生保存。
- 扩展回归修正 DirectWrite HitTestMetrics.bidiLevel 的 UINT32 绑定与行内方向读取；
  在 GTK/Pango 与 DirectWrite 的原生段落基方向可能不同的条件下，不强制尾部中性
  对象具有同一方向。FontSetBuilder／Factory3／InlineObject 的 vtable 及原生
  FLOAT／BOOL 布局、CTRunDelegateCallbacks、Pango 属性布局均有 ABI 断言。
- Windows 加入 native inline 回调后，旧 GetMetrics／GetLineMetrics／GetClusterMetrics
  的栈输出地址在 Go→C→Go 栈增长期间失效；强制增长栈的旧快照连续 3 次返回空
  度量。保留 CallRaw，只对这些查询及同类 DetermineMinWidth 使用 Pinner 固定
  输出存储；计数改为原生 UINT32，typed HitTest 输出经逃逸诊断确认已在堆上。
  新实现的强制栈增长和全套 DirectWrite 回归连续 3 轮通过，不机械替换全部绑定。
- 零宽占位补充发现 CoreText 的建议宽度可为零，退化 frame path 会丢失行与
  Cluster。使用请求宽度创建非退化路径，保持原生测量的实际 advance；纯空白旧
  实现返回空度量的回归已复现并修正，不使用 GUI 偏移补偿。
- 本机 Go 1.25.8／Linux amd64／Pango 1.50.6／Cairo 1.16.0：保存当前工作区
  同源快照，在隔离 Xvnc／Xfwm4 会话执行 `CGO_ENABLED=0 go test -json ./... -count=1`，
  2501 项通过、15 项既有跳过、零失败。旧测试创建窗口及 PNG 的副作用只发生在
  隔离桌面／快照中，未覆盖工作区内用户图片。
- 相关 Typography／textbitmap／Software race 通过。普通 vet 的底层 unsafe.Pointer
  与既有非键名字面量告警保留；关闭 unsafeptr／composites 后的针对性 vet 通过，
  不将原命令失败记为通过。
- 经 SHA256 校验的 Windows 10 19045 amd64／macOS 15.7.7 arm64 测试产物进行
  无窗口原生执行；CoreText、DirectWrite 全套包内回归连续 3 轮，CF 集合连续
  10 轮，DirectWrite SDK 回归／ABI 连续 10 轮。离屏倍率不是桌面系统 DPI。
- `CGO_ENABLED=0` 的 Windows amd64／macOS arm64 全仓构建通过；Linux arm64
  受影响 Typography／Pango／Cairo／Fontconfig 包构建通过。全仓 Linux arm64
  被 HEAD 中既有 libc `syscall.SYS_PIPE`／`syscall.SYS_POLL` 引用阻断，未擅自
  扩大为 libc syscall 修复；原失败命令保留，不计作通过。
- 尚未执行新的真实桌面、硬件 GPU、不同系统 DPI 和字形外伸专项金图验收；
  不因原生离屏通过宣称全部 Painter／桌面行为通过。不自动提交、推送或更新 Release。

### 常量加载入口收敛（2026-10-15）

- `Const[T]` 统一提供可选 `PVar`／`PAddr` 输出，移除独立 `ConstAddress`。
  现有取值声明保持不变，三个 CF callbacks 声明只指定 `PAddr`；不增加
  callbacks 结构体副本，也不改变 CF 集合及字体资源的所有权。
- 新增加载器原生测试覆盖仅取值、仅取地址、同时获取、结构体符号地址、
  缺失符号输出保持不变和无输出配置错误。CFBoolean 值通过原生函数核对。
- 经 SHA256 校验的产物在 macOS 15.7.7 arm64 无窗口运行：加载器和 CF 集合
  生命周期回归各连续 10 轮、CoreText 全套回归连续 3 轮通过。
- 本机 Linux amd64／Go 1.25.8 同源快照在隔离 Xvnc :87／Xfwm4 中执行
  `CGO_ENABLED=0 go test -json ./... -count=1`：2501 通过、15 项既有跳过、
  零失败；既有窗口／PNG 输出仅发生在隔离会话／快照。macOS arm64
  `CGO_ENABLED=0 go build ./...` 通过。
- 本轮只收敛 macOS 绑定加载方式，没有重跑 Windows、桌面外观、GPU 或 race；
  不把前一阶段的这些记录作为本轮新增验收。不提交、推送或修改 Release。

### 占位接口命名与提交版本验证（2026-10-15）

- GOUI 公共接口统一为 `SetPlaceholder`／`PlaceholderMetrics`，三端度量状态、
  测试及替身同步命名；设置不插入或替换文本，不接管实际内容或绘制。
  DirectWrite 原生 COM 类型和方法保持 InlineObject 命名，用户合并后的
  `context_windows.go` 文件组织保留；不改排版机制、资源归属或原生 ABI。
- 验证对象为从 Git 暂存树导出的干净源码快照，只包含已有跟踪文件和本次拟提交
  的实现／测试，不把其他既有未跟踪测试或图片隐式纳入代码提交。
- Linux amd64／Go 1.25.8，在隔离 Xvnc :87／Xfwm4 中执行
  `CGO_ENABLED=0 go test -json ./... -count=1`：2162 通过、10 项既有跳过、
  零失败；Pango／Typography utils／textbitmap 的 race 测试通过。
- Windows amd64／macOS arm64 的 `CGO_ENABLED=0 go build ./...` 通过；
  同一暂存快照构建并经 SHA256 校验的 CoreText／DirectWrite 包内测试在
  macOS 15.7.7 arm64／Windows 10 19045 amd64 无窗口各连续 3 轮通过。
  不把离屏倍率当作桌面 DPI，不重复桌面或 GPU 验收。
- 本次代码提交只包含 Typography 与接口必需的三个测试替身；设计文档、截图、
  绑定层未跟踪测试及其他既有文件保持工作区状态，不推送或修改 Release。

### 设计文档拆分与核对（2026-10-15）

- 长期契约迁至 DesignTypography，补齐当前单位、Cluster、栅格化、缓存和错误边界；
  本文保留原始诊断与带日期的验证记录。修正代码已提交状态，不改历史失败或容差。
- 5 份相关文档的 186 个本地链接／锚点、代码围栏和选定公共 API 名称检查通过，
  `git diff --check` 通过；RTL 起止边映射与其他范围 setter 错误分支只标记待复核。
- 当前工作区同源快照在 Linux amd64／Go 1.25.8、隔离 Xvnc :87／Xfwm4 中执行
  `CGO_ENABLED=0 go test -json ./... -count=1`：2501 通过、15 项既有跳过、零失败。
  既有窗口及 PNG 输出只发生在隔离会话／快照；本次不修改源码、测试或用户截图。
- 本轮没有新增远端、真实桌面交互、GPU 或系统 DPI 验收，不以文档核对扩张原测试
  结论；文档整理按用户授权提交，不推送或更新 Release。

## 6. 后续方向

未实现能力与待复核项见 [Typography 后续边界](DesignTypography.md#12-后续能力边界)
和 [Roadmap](Roadmap.md#textlayout-富文本基础本轮已实现应用层按需求推进)；
不由本实施记录授权继续扩展接口。新增验收仍附具体环境、命令和结果，不覆盖原始证据。
