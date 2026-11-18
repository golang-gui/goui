# 测试用例约定

> 状态：新增测试遵循本约定；现有测试和示例仅按已确认范围逐项整理，专项记录见文末。
> 历史桌面依赖仍须单独记录，不表示全仓测试已经可以在无桌面环境运行。

## 1. 仅分两类

| 类型 | 放置位置 | 执行方式 |
| --- | --- | --- |
| 不依赖桌面的测试 | 逻辑测试留所属包；具体原生接口的跨层集成放 `tests/integration/<主题>/` | `go test`，使用断言 |
| 依赖桌面的窗口验证 | `tests/window/` 下的独立程序 | 显式启动，人工或 AI Computer Use 按注释操作 |

分类依据是运行条件，不是代码所属层级。platform、gui、ui 均遵循此规则。自动检查与人工观察是验证手段，不再划分第三类测试。

运行条件之外还要遵守依赖边界：gui、ui、widgets 等共享层包内测试不直接创建 Pango、
CoreText、DirectWrite 或调用其他具体原生接口。只验证后端事实的测试属于 platform 对应包
或 platform/test；需要同时使用原生后端和上层控件的无桌面集成测试放到 tests/integration，
使用公开接口，不使 platform 反向依赖上层，也不导出生产私有状态以便迁移。
正常的平台策略判断与跨平台 Software 离屏像素测试保留在所属包；不以 runtime.GOOS、
公共事件值类型或公共排版接口的出现作为迁移依据。

在 GOUI 正式 Release 之前，不依赖 DevServer 进行其他功能的验收，也不以完善 DevServer 为测试前置条件。DevServer 自身不依赖桌面的协议、解析和服务逻辑测试仍留在包内。正式 Release 后是否引入该自动化路径，另行评估。

## 2. 包内代码逻辑测试

适用于几何、布局、状态转换、信号、生命周期、事件分发、样式、语义快照、UI 协调、绘图数学、像素转换、缓存以及参数校验等可独立验证的逻辑。

- 不创建原生窗口，不要求桌面会话，不修改真实剪贴板、不启动外部应用。
- 不依赖桌面不等于不依赖原生库。文字排版、GLib 转换或原生 ABI 测试按上述依赖边界放置；跨平台离屏绘制可留所属包。确实需要显示连接或原生窗口的部分归入窗口验证。
- 用明确断言验证结果，不能只打印日志或输出图片。像素数据优先在内存中比较；跨平台字体栅格化差异不能无条件使用同一份像素金图判断。
- 固定资源放在对应包的 `testdata/`；临时文件使用 `t.TempDir()`，不向源码目录写图片或其他运行产物。
- 测试辅助脚本及C、Objective-C、Java、CSS等代码类测试素材保存为Go静态字符串。
  需要独立文件时通过go generate输出到仓库外目录（沿用GOUI_GENERATE_DIR），或由
  用例写入私有临时目录；物化后的代码文件不纳入提交。
- 真正共享的替身可放在本包 `test_helpers_test.go`；单个测试专用的 helper 就近放置。不建设跨层的大型 mock 框架。
- 原生库加载、线程亲和及资源释放仍须遵守相应平台要求。原生实现测试和独立集成入口可用平台文件后缀或构建约束，共享层包内测试不直接选后端。
- 优先复用已有测试主题；迁移前列出原断言与新位置的对应关系，私有缓存和状态断言可用确定性排版输入保留在原包，真实字体的跨层结果在集成测试中验证，不以删除断言或增加 skip 完成整理。

事件分发测试可以构造输入事件，但必须通过被测的正常事件路径验证行为；不能用直接修改控件状态代替对输入路径的验证。

## 3. 独立窗口验证程序

新增程序放在 `tests/window/<层或领域>/<主题>/`，每个主题使用普通 `package main`。例如：

```text
tests/window/
├── README.md
├── platform/
│   ├── window/
│   └── clipboard/
├── graphics/
│   └── rendering/
├── gui/
│   └── overlay/
└── ui/
    └── overlay/
```

以上仅为目录示意，按新增场景建立，不预先创建空目录。入口索引见 `tests/window/README.md`，用于列出主题、入口与适用环境；具体操作步骤以源码注释为准，不维护两份流程。

独立启动示例：

```bash
go run ./tests/window/ui/overlay
```

该入口提供声明式 Overlay、遮罩、布局和状态保留的实际操作步骤。

- `go test ./...` 可以编译这些命令包，但不会执行 `main`。窗口、平台和事件循环的初始化应由运行入口显式发起，不在 `init` 中自动启动。
- 通过公共 `platform`／`gui`／`ui` 启动的程序已由 `platform.go` 的 `init()` 保留启动线程，直接在 `main` 中创建应用并运行，不必重复锁定。仅使用底层原生绑定且需要真正主线程的程序自行在 `init()` 中保留线程；不能把 `main` 内才调用 LockOSThread 当作主线程保证，也不能把 AppKit 工作搬到后台 goroutine。
- 需要真实事件循环、桌面剪贴板或 GLX drawable 的场景，即使不显示窗口，也属于此类。
- 一个程序聚焦一个验证主题，允许包含紧密相关的多个场景。不引入统一 runner、场景注册框架或新的自动化协议。
- GUI/UI 验证保持既有层间依赖。人工和 AI Computer Use 使用真实桌面输入，不为验证提供绕过 `Window.DispatchEvent` 的语义动作捷径。
- 不为独立验证程序导出生产代码内部状态。私有算法和内部状态断言留在包内，真实窗口集成通过公开契约和可观察结果验证。

## 4. 注释与可观察结果

每个程序的入口注释必须说明：验证目的、环境要求、启动方式、初始状态、编号操作及逐项预期、平台差异、副作用、复位和退出方式。多个场景可在相应构建函数旁分别说明，入口提供索引。

示例模板：

```go
// Overlay 窗口验证
//
// 环境：记录操作系统、桌面环境、绘制后端及缩放比例。
// 启动：go run ./tests/window/ui/overlay
// 初始：背景和面板计数均为 0，浮动面板可见，遮罩未显示。
//
// 操作与预期：
//   1. 点击浮动面板外的背景计数按钮。
//      预期：背景计数增加 1，面板计数不变。
//   2. 点击浮动面板中的计数按钮。
//      预期：仅面板计数增加，背景计数不变。
//   3. 显示遮罩，再点击原背景按钮所在位置。
//      预期：背景计数不变。
//   4. 关闭遮罩，再次点击背景按钮。
//      预期：背景计数增加 1。
//
// 副作用：无系统设置或剪贴板修改。
// 复位：重新启动程序。退出：关闭窗口。
```

验证程序应提供清晰标题、控件文本及必要的可见计数、状态或诊断日志。可计算的结果尽量直接检查，失败时说明预期和实际值；外观与真实输入体验仍需实际观察。

不能仅写“显示正常”“交互正确”。涉及布局、命中、输入时说明具体位置和状态变化；涉及圆角、阴影或文字时说明需要观察的边缘、接缝、裁剪和清晰度。涉及性能时说明测量范围、后端、资源规模与比较条件，避免只记录没有基准的一次耗时。

剪贴板写入、打开链接等影响桌面的操作必须在注释中说明，并由用户主动触发。程序退出时释放自己创建的窗口及资源。

## 5. 验证与结果记录

完成修改前仍运行仓库要求的：

```bash
go test ./...
```

现有桌面依赖、跳过条件和已知失败尚未迁移，执行时应如实记录；不能把这次文档约定当作“全仓默认测试无桌面副作用”的保证。也不能通过删除断言、扩大跳过范围来掩盖真实缺陷。

窗口验证记录至少包括：

- 代码版本、启动命令、操作系统；Linux 补充桌面环境或窗口管理器。
- 绘制后端、缩放比例及相关前置条件。
- 按操作编号记录通过、失败、未执行或不适用；失败写明实际结果，必要时附截图或日志。

窗口打开成功不等于场景验收通过。交叉编译只证明相应目标可编译，不代表该平台运行验证通过。平台不支持的能力应明确记录，不伪装成功；符合契约的降级行为则按其明确预期进行验证。

`example/` 继续用于展示 API 用法，不强制迁移为测试。后续新增的专门回归验证程序遵循本文；现有测试的整理若要推进，另行确认范围。

## 原生排版跨层集成

`go test ./tests/integration/typography` 不创建原生窗口、不运行桌面事件循环，需要当前平台的
原生字体库。Pango／CoreText／DirectWrite 工厂集中在该目录的三个平台入口，所有原生操作
都在各叶子测试实际执行的锁定线程内完成，排版释放先于 Context 释放与线程解锁。
gui、widgets 和 textedit 的断言通过公开控件树、事件分发、几何、绘制与生命周期接口执行。

迁移覆盖对应关系：

| 原位置／场景 | 当前逻辑或私有状态检查 | 真实后端集成检查 |
| --- | --- | --- |
| widgets 的三个 Linux 排版文件 | 原有 ListRow／ListRowGroup／TabBar 逻辑测试 | 集成目录的 tab_bar、list_row、list_row_group 测试，保留字号、插槽、换行、隐藏、关闭按钮与裁剪断言 |
| textedit 的三个原生字符导航入口与 helper | 原有几何、索引与精度测试 | geometry_test.go 保留字体、宽度、单字符导航断言 |
| GUI 原生编辑导航 | text_context_test.go 使用确定的 RTL／组合 cluster 输入，保留光标方向与编辑状态断言 | TextInput／TextView 通过真实 cluster 验证扩选、删除和撤销 |
| GUI 原生首尾光栅化 | text_context_test.go 保留 RenderWidget、滚动状态和 1024 像素请求上限检查 | input／nowrap／wrapped 首尾在 1x／1.25x／2x 重复真实栅格化，保留非空正文、滚动位置和请求上限 |
| GUI 原生滚动与宽度往返 | text_context_test.go 保留布局收敛、pending 状态及固定字符锚点 | 95000 行末尾定位；890～917 DIP 起点和九次宽度往返，基于实际 Paint 原点与原生行度量核对同一字符，保留 1 DIP 容差 |
| GUI 段落原生参考与巨大坐标 | paragraph_layout_test.go 保留分页内部状态、销毁重入及 64 MiB 虚拟前缀的局部坐标断言 | paragraph_test.go 通过公开几何对照完整原生段落，保留复杂文本、软折点、接缝命中／选区／绘制和 8192 字节上限 |
| GUI 完整上下文宽度缓存 | text_context_test.go 保留缓存身份、度量、资源计数和释放后几何 | text_test.go 通过原生 Context 装饰器保留完整参考、RLO、宽度复用、控件隔离与配对释放 |

集成测试的无事件循环应用替身提供不可启动的 Timer，使编辑器按既有契约采用同步排版回退。
事件循环调度、重排合并与 pending 状态仍由 gui 包中的可推进计时队列和原有状态测试覆盖；
这里的通过不能记为桌面事件循环、IME 或窗口验收通过。

## X11 包内测试组织

2026-10-30 按已确认范围将 `platform/linux/x11` 的 15 个测试文件集中为 8 个主题文件：

| 文件 | 覆盖主题 |
| --- | --- |
| `window_linux_test.go` | 窗口身份、定位、显示／激活消息、Popup 尺寸 |
| `input_linux_test.go` | 按键、修饰键、重复事件、焦点恢复快照 |
| `ime_linux_test.go` | 创建失败、预编辑、上下文隔离与生命周期 |
| `cursor_linux_test.go` | 光标映射、主题解析／加载、原生主题生命周期 |
| `clipboard_linux_test.go` | 剪贴板状态、超时、重入与编码 |
| `dragdrop_linux_test.go` | XDND、错误分流、超时、数据解码与预览 |
| `dialog_linux_test.go` | 文件对话框 helper 的输出、取消与错误 |
| `settings_linux_test.go` | locale 解析与无桌面 GLib 语言查询 |

辅助函数与使用它的测试就近放置；保留原有测试名称、断言、Fuzz 种子和运行条件。
此次只合并文件、整理说明，并修正测试中已经更名的私有错误处理入口，未修改生产代码。

`TestWindowApplicationClass`、`TestNativeCursorThemeLifecycle` 是历史桌面依赖测试，
本次保留其 `DISPLAY` 检查、线程锁定和原生资源释放；不是无桌面测试。
前者创建不映射的原生窗口读取 `WM_CLASS`；后者使用私有窗口和 selection，
验证原生属性通知、光标缓存与主题重载，不修改真实桌面 XSETTINGS owner。
无 `DISPLAY` 时这两项仍跳过，不能算作通过；后续迁至独立窗口程序时需保留有效覆盖，
不能为从包外调用私有实现而导出生产接口。本次整理不新增桌面测试例外规则。

无桌面验证可用 `env -u DISPLAY go test ./platform/linux/x11`；完整原生包验证应在
独立 X11 会话中执行，并单独记录上述两项及整体结果。保留原生库依赖，
不把“无桌面”误写成“没有系统依赖”。实际验证记录见 [Plan](Plan.md)。

## Win32 与 Cocoa 包内测试组织

2026-10-30 沿用功能主题划分，保留测试名称、输入、断言、线程锁定和资源释放顺序，
只整理文件、中文说明和 import 分组。不为文件数量一致而合并独立职责。

Win32 的 10 个文件集中为 7 个：

| 文件 | 覆盖主题 |
| --- | --- |
| `application_windows_test.go` | 应用图标提取、窗口类图标与资源生命周期 |
| `window_windows_test.go` | 窗口样式、客户区几何、系统版本与命中区域 |
| `cursor_windows_test.go` | 缩放光标的原生资源 ID |
| `dragdrop_windows_test.go` | OLE 对象身份／引用计数、回调 ABI、数据、预览与反馈 |
| `ime_windows_test.go` | 创建失败、UTF-16 拼接、重置与提交来源 |
| `input_windows_test.go` | 已消费按键、逻辑按键与左右修饰键快照 |
| `settings_windows_test.go` | 原生系统语言查询 |

Cocoa 保留已有 7 个主题文件：application、window、ime、input、dragdrop、
file_access、settings，均使用 `_darwin_test.go` 后缀。
`ime_factory_darwin_test.go`、`popup_darwin_test.go` 分别归入
`ime_darwin_test.go`、`window_darwin_test.go`；Bundle 身份、文件访问凭据及语言查询
仍各自独立。Win32 同样把 factory／text 合并至 ime，source／target 合并至 dragdrop。

`TestApplicationClassIcons` 是历史窗口系统测试，会创建隐藏 HWND，不能称为纯逻辑
测试；本次保留既有入口与线程锁定，后续迁移遵循相同的独立窗口程序规范。
OLE／Objective-C／Foundation 的对象与 ABI 测试保留相应原生库和服务依赖；
不能仅凭测试名中的 Headless 就推断任意远端会话都能运行。
AppKit 的真正主线程要求也不能以测试 goroutine 的 LockOSThread 代替。
原生运行须分别记录实际会话和适用条件，交叉编译不等于这些测试已经执行。
本次不新增 skip 或迁移历史窗口测试，不重复验收交互拖放场景。

整理前后的函数 AST 及测试清单用于检查覆盖保留；原生运行结果与当前平台缺口见
[Plan](Plan.md)。

## Tooltip 验证入口

`gui/tooltip_test.go` 使用可推进计时器及正常事件分发验证目标、延迟、取消、重入和资源清理；
`ui/view_test.go` 验证共享修饰器及默认值恢复。独立窗口程序为
`go run ./tests/window/gui/tooltip [-inspect]`，操作及逐项预期见入口注释。
声明式手工程序为 `go run ./tests/window/ui/tooltip`，覆盖共享修饰器在真实窗口中的
文字更新、空串清除、省略恢复构造默认值、隐藏／卸载与重建，以及 Basic / Modern 切换。
程序通过真实按钮操作或 F6 检查文字、控件身份、输入和选区保留；启动及 ASSERT PASS
不代表视觉检查通过。悬停、Tab、Escape、定位与缩放按入口注释另行检查。
每次触发均等待 700 ms；Modern 亮暗提示无阴影。特别检查按钮底部和其他留白，
静止三秒时提示与 Hover 均稳定，不只在文字中心验证。
真实桌面检查 Basic / Modern 亮暗、鼠标悬停、禁用说明、Tab、点击与滚轮、屏幕边缘及缩放，
实际结果和既有失败见 [Plan](Plan.md)。

## UI Timer 原生循环验收

2026-11-03 按已确认范围，将旧 `ui/timer_native_test.go` 的原生集成场景迁至
`tests/window/ui/timer`，不在包内 init 或 `go test` 中启动这两个场景。参数、失效句柄、
同步投递及 State 消费逻辑继续由所属包测试覆盖；其他历史桌面测试本轮不迁移。

```sh
go run ./tests/window/ui/timer -scenario=runtime
go run ./tests/window/ui/timer -scenario=initial-failure
```

两次使用独立进程，均需要真实原生事件循环；Linux 需要 DISPLAY，即使没有可见窗口。
保留主线程、单次／周期、回调内重用／Stop、非法重启及退出检查；将过时的“未消费
State 写入触发全应用重建”预期改为“不重建”，另测显式 RequestUpdate 并等待构建通知。
初次声明失败时，用公共 GUI Timer 检查同一调度器已关闭，不让 UI Active 的保护掩盖
底层任务泄漏，不为此暴露生产私有状态。操作、超时及退出码见程序入口中文注释。

## 2026-11-18 全仓测试整理

本轮在原工作区整理已跟踪和未跟踪的测试，不使用仅含已跟踪源码的副本替代全仓结果。
原 23 条编译诊断中，21 条是旧窗口验证调用已从 Window 拆出的管理方法，改为
WindowManagement 能力调用；2 条是 FindWindow 测试仍构造旧 windowMount，已并入
ui/app_test.go 并按现有 hostMount 生命周期构造。未恢复旧生产接口。

原 11 项条件跳过的归属：

| 原测试 | 当前入口／执行条件 |
| --- | --- |
| TestClipboardPasteFromApp | tests/window/platform/clipboard，使用 -expect 明确核对外部文本及恰好一次回调 |
| TestFocusEventsManual、TestKeyEventsManual、TestPointerEventsManual、TestWheelEventsManual | tests/window/platform/input，指定类型、数量、期限；焦点重复通知仍记录但不重复计为状态转换 |
| TestWindowChromeIntegrated | tests/window/platform/window -scenario=integrated，Windows/macOS 原生装饰验收 |
| 三项 TestWindowState*Transitions | 同入口 -scenario=states，保留原状态序列、原生观察与超时断言 |
| TestMacOSSigning | cmd/goui/signing_darwin_test.go，macOS 上调用真实 codesign |
| TestNativeIOSSDKSelection | platform/mobile/base/sdk_darwin_test.go，macOS + iOS Simulator SDK；缺 SDK 时仍明确跳过 |

其余窗口断言同步从 platform/test 移至独立入口，不再由默认测试启动：

| 原覆盖 | 当前覆盖 |
| --- | --- |
| 应用身份、Post/Quit/销毁、无效外部打开参数 | platform/application；主线程直接执行原生操作，跨线程投递有五秒上限 |
| Window 标题/首帧/关闭否决、主动重绘、装饰/生命周期、无输入移动拒绝、工作区 | platform/window -scenario=basic；保留各项断言 |
| Windows 客户区/最小尺寸/NCCALCSIZE、macOS 按钮组平移/隔离/恢复与原生 zoom | platform/window -scenario=native；过时的“macOS 全屏不支持”预期由公共 states 场景替代 |
| Popup 基础 owner、直接 Draw、NewPainter(Popup) | platform/popup；保留句柄、首帧、五秒期限与资源释放顺序 |
| Settings 枚举/颜色/字体/字号 | platform/settings；保留单项不支持与全部不支持的区别 |
| 剪贴板同进程 ASCII/Unicode/多行/覆盖 | 既有 platform/clipboard -write，保留同步且恰好一次回调与确值检查 |
| GLX 绑定/最大化、透明呈现与原生 framebuffer/扩展样式、X11 真实移动/缩放 | platform/glx、transparency、window_interaction；保留量化容差与一次性输入会话断言 |
| Xlib 原生属性查询 | platform/xlib_queries；ABI 断言仍留原生绑定包，移除 t.Run 导致的跨线程原生调用 |
| Key/Pointer/Wheel/Focus 事件值 | platform/events/input_event_test.go，继续由默认 go test 执行 |

未跟踪测试按价值纳入已有主题：应用 ID/Sync/FindWindow 并入 gui、ui 的 application/app
测试；图标上传复用保留独立外部包测试；Modern 图标/选区对比度并入主题测试；
SVG 重复 arc 的着色覆盖并入现有解析圆回归，保留原 1.25 物理像素边界，不放宽容差。
原生 ABI、资源所有权与排版 cluster/分隔符/alpha/属性替换测试归入各后端。

platform/typography/test 的叶子测试各自锁线程并直接创建排版 Context，不再为离屏排版
创建原生 Platform 或显示连接；字体和 Painter 都按明确生命周期释放。
PNG 编码检查改写 t.TempDir；bitmap 复制/通道交换改为内存中精确像素断言。
未被引用的 SavePng 临时辅助函数不进入生产包。

旧例程中的 Overlay、原生装饰/Surface/透明窗口、外部打开及菜单编辑器移至对应窗口
主题并保留有效布局/状态断言；image_lifetime、drag_end、dragdrop、ui/window_position
纳入版本管理。重复的 gui_test/ui_test、旧自绘进度安装器及最小 hello 原型已清除，
相关能力分别由 Demo、tree_view/composited/textediting、progress_bar 与 README 示例覆盖。
生成的 PNG、可执行文件和旧运行产物已清除；静态 testdata 保留，原件备份在仓库外。

当前仍有 X11 包内的两个历史原生私有状态测试：TestWindowApplicationClass 与
TestNativeCursorThemeLifecycle。它们继续使用原有 DISPLAY 条件和有效断言；
无显示会话时跳过，不能声称默认测试已在所有无桌面环境完整执行。
本轮 Linux/X11 全仓结果为零失败、零条件跳过；取消 DISPLAY 后全仓命令仍退出 0，
6106 项通过，只有上述两项按原条件跳过，未计为通过。macOS 的 codesign/SDK 与两个非本机
排版后端仅完成目标编译，本轮未执行原生运行。详细验证结果见 Plan。

## 2026-11-18 测试文件归并

在既有包内继续归并小型测试文件，不改变测试分类或运行条件：

| 所属包 | 归并位置 |
| --- | --- |
| Linux GLib、Pango、Xlib 绑定 | pointer、type、strings 测试并入各包 api_linux_test.go |
| Windows DirectWrite SDK | hit_position、typography 测试并入 api_windows_test.go |
| Windows Shell、WinAPI SDK | dialog、window_placement 测试并入各包 abi_windows_test.go |
| CoreText、DirectWrite、Pango 排版后端 | shaping、metrics、region、tabs、raster 等测试并入各后端 context 测试文件 |
| Win32 | drag_preview、fullscreen 测试分别并入 dragdrop、window 测试文件 |

涉及的 10 个包由 45 个测试文件归并为 16 个。归一化导入别名后，150 个函数、类型与
变量声明的 AST 全部保留且一致，其中 63 个声明仅改变所在文件；不改变断言、输入、
线程锁定、资源释放、skip 条件或基准循环。platform/common/env_test.go 是其所属包的
唯一测试文件，继续独立保留缩放环境变量的无效输入和正数覆盖，不属于冗余文件。
验证结果见 Plan 的同日归并复核记录。
