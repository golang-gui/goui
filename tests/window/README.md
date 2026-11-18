# 窗口验证程序

依赖真实桌面的验证显式启动，不由 `go test ./...` 执行，不依赖 DevServer 验收其它功能；
`gui/dev` 入口只验证 DevServer 自身的协议和公开输入路径。
分类和结果记录遵循 [测试约定](../../doc/Testing.md)。操作与预期以入口源码注释为准。

| 主题 | 入口 | 环境 |
| --- | --- | --- |
| 原生应用身份、事件循环、非法外部打开输入 | [platform/application](platform/application/main.go) | 三平台；显式运行真实事件循环，检查 Post/Quit/销毁；不打开外部应用 |
| 原生窗口创建、绘制、状态和装饰 | [platform/window](platform/window/main.go) | `-scenario=basic/states/integrated/native`；平台差异及自动断言见入口 |
| 基础 Window owner 与 Popup Painter | [platform/popup](platform/popup/main.go) | 三平台；直接 Draw 和原生 Painter 各一帧，五秒超时 |
| 原生设置查询 | [platform/settings](platform/settings/main.go) | 三平台；明暗、主题色、字体、字号；明确区分不支持与读取失败 |
| 透明 Surface 创建/两帧呈现及原生断言 | [platform/transparency](platform/transparency/main.go) | Software/OpenGL；Windows 另有 Direct2D；Linux ARGB visual/framebuffer，Windows 扩展样式 |
| 透明 Window/Popup 交互清除 | [platform/transparent_window](platform/transparent_window/main.go) | 三平台合成桌面；点击切换半透明内容和清除，需人工检查残留 |
| GLX 重复绑定与最大化绘制 | [platform/glx](platform/glx/main_linux.go) | Linux/X11；16 轮上下文绑定、12 轮最大化/恢复，每状态至少三帧 |
| X11 原生移动/缩放与按下有效期 | [platform/window_interaction](platform/window_interaction/main_linux.go) | 独立 X11 桌面、XTEST、EWMH WM；按映射/输入/原生几何推进，会移动真实指针 |
| 原生输入类型、数量和超时 | [platform/input](platform/input/main.go) | `-kind=pointer/key/wheel/focus -count=N -timeout=15s`；日志字段需对照实际输入 |
| Xlib 属性查询与缓冲区契约 | [platform/xlib_queries](platform/xlib_queries/main_linux.go) | Linux/X11；私有隐藏窗口，32位 native long、偏移/类型/删除/缺失属性 |
| 无装饰 Surface 与显式原生移动/缩放 | [platform/surface](platform/surface/main.go) | 三平台 OpenGL；按源码指定区域检查真实交互与重绘 |
| 原生按钮位置与标题栏命中 | [platform/chrome_controls](platform/chrome_controls/main.go) | Windows/macOS；Integrated/Native、1x/2x；解析/命中另有无桌面测试 |
| 声明式 Overlay、遮罩与内容保留 | [ui/overlay](ui/overlay/main.go) | 三平台；浮动/填满、左右位置、模态输入、状态和主题 |
| 菜单、正文、Tooltip 与全屏组合 | [ui/menu_editor](ui/menu_editor/main.go) | 三平台；只读选中文件，不自动启动诊断服务；侧重弹出层组合验收 |
| 外部 URL/路径打开 | [ui/external_open](ui/external_open/main.go) | 三平台；仅显式点击才启动默认外部应用；提交成功不等于外部完成 |
| macOS 普通窗口默认位置与显示保持 | [platform/window_position](platform/window_position/main_darwin.go) | macOS arm64；三种 Chrome，隐藏默认原生居中、显式定位、重复 Hide/Show、Popup 不居中与锚点；`-auto` 原生几何断言，手动检查初始可见位置与闪烁 |
| 初始 Center 创建选项、显式覆盖与重建/重现保持 | [ui/window_center](ui/window_center/main.go) | 三平台；Native/Integrated/None、1x/2x；`-auto` 核对隐藏创建、实际客户区/工作区中心、显式移动、UI 身份及 Hide/Show；`-override` / `-center=false` / `-skip-taskbar=true`；真实输入与视觉操作见中文入口 |
| 任务栏跳过偏好与普通辅助窗口输入/定位 | [gui/taskbar](gui/taskbar/main.go) | 三平台；None/Native/Integrated、显示前 SetPosition、任务栏/Alt+Tab/Dock、真实输入/移动/缩放与最小化恢复；`-auto` 仅原生初始化和定位断言 |
| BorderBox 单子边框与 None root 缩放 | [gui/border_box](gui/border_box/main.go) | 三平台；默认无外观/自定义圆角、8 方向光标、原生缩放、输入/移动、禁用/嵌套/重挂载；`-auto` 仅布局/Snapshot |
| 完整默认 Basic、Modern 一行应用与恢复兜底 | [ui/themes](ui/themes/main.go) | 三平台；未设置/显式 Basic、系统/深色 Modern；Software/OpenGL（Windows 另有 Direct2D）、1x/2x；联动/禁用/Tab、F6 身份/文本/边界断言，不使用 DevServer |
| Windows 版本对应边框、客户区尺寸与顶部两角 | [platform/window_chrome](platform/window_chrome/main_windows.go) | Windows 10/11 x64；Native/Integrated/None、隐藏几何 -auto 与实际亮暗/激活/缩放/最大化；Software/OpenGL/Direct2D、渲染 1x/2x 与真实系统 DPI 分别记录 |
| 二进制构建、原生图标及应用目录打包 | [tools/packaging](tools/packaging/main.go) | Windows EXE 属性／Explorer／任务栏、macOS plist／Finder／Dock、Linux desktop／WM_CLASS；生成文件副作用与手工安装步骤见中文入口；-auto-close 仅为启动探针 |
| Dev ref、已确认文本与模态 Popover 输入 | [gui/dev](gui/dev/main.go) | 三平台桌面；无应用 ID、Unicode/选区/撤销/只读/多行、旧引用失效及内置菜单；设置 GOUI_DEV_ADDR，可加 -verify；Software/OpenGL（Windows 另有 Direct2D）、1x/2x；不验收原生 Popover IME |
| 多语言资源、命令式 translateUI 与声明式自动协调 | [ui/mlang](ui/mlang/main.go) | 三平台桌面；八语言、SVG 主题色、编辑菜单实时切换、Widget/编辑状态保留；-auto 验证语言/资源/离屏像素，1x/2x |
| Integrated 四边布局、滚动条、标题及控制按钮居中 | [gui/integrated_layout](gui/integrated_layout/main.go) | Linux X11 合成桌面；无根 Padding，1x/2x、双向滚动、标题高度、最大化/全屏/恢复及 F6 公开边界断言；其他平台不自动内缩 |
| 滚动方向、布局、双条占位与嵌套滚轮消费 | [gui/scroll_layout](gui/scroll_layout/main.go) | 三平台；1x/2x、Shift 横滚、响应式表单、F10 定位内层列表／F4 切换长短，F6 几何及偏移日志；具体操作见入口注释 |
| 非编辑式下拉单选、浏览确认与实际内容协调 | [widgets/drop_down](widgets/drop_down/main.go) | 三平台；默认 UI、-gui 命令式、Modern/显式兜底；1x/2x、长列表、Tab、退避、重建、Snapshot 与回调销毁 |
| CheckButton 三态、互斥、按钮式外观与声明协调 | [widgets/check_button](widgets/check_button/main.go) | 三平台；Modern/显式兜底、Software/OpenGL（Windows 另有 Direct2D）；有效释放/取消、子按钮竞争、Tab/键盘、主题/重建/快照/销毁；1x/2x |
| 默认 Tab / Shift+Tab 焦点导航 | [gui/tab_navigation](gui/tab_navigation/main.go) | 三平台；树顺序、消费优先、滚动与模态 Popover；FocusVisible 2 DIP 边框、键鼠切换、F8 亮暗/F9 主题色/F10 Primary；1x/2x |
| 进度条／圆环、单向忙等待与动画生命周期 | [widgets/progress_bar](widgets/progress_bar/main.go) | 三平台；Software / OpenGL，Windows 另有 Direct2D；0/25/50/75/100%、祖先隐藏、主题切换、Snapshot、Timer 启停与销毁；1x/2x |
| 声明式进度显示与重建连续性 | [widgets/ui_progress_bar](widgets/ui_progress_bar/main.go) | 三平台；四种模式、单向忙等待、隐藏恢复、亮暗／主题色、ID 身份与重建不重启 Timer；1x/2x |
| 窗口工作区 / 相对定位与实际位置查询 | [gui/window_position](gui/window_position/main.go) | 三平台；`-auto` 无装饰位置断言，`-chrome=native/integrated` 人工核对外框、客户区与手动移动后 Position；1x/2x |
| UI 窗口 ID 查询 / 定位 / 重建与同 ID 重建窗口 | [ui/window_position](ui/window_position/main.go) | 三平台；Native 装饰、按钮/F1–F6 与 F8；定位后另行查询，重建不重置用户移动的位置；1x/2x |
| 原生按键消费与 WM_CHAR 默认处理 | [platform/keydefault](platform/keydefault/main_windows.go) | Windows；自动断言，自建不可见窗口；不替代布局/IME 手动验收 |
| 原生消息同步回调与 Go 栈地址稳定性 | [platform/message_reentry](platform/message_reentry/main_windows.go) | Windows x64；隐藏窗口自动断言 GetMessage/PeekMessage 重入、回复内容和错误/退出契约；无需 Painter |
| 左右修饰键、双侧释放及焦点恢复 | [platform/modifiers](platform/modifiers/main.go) | 三平台；逐事件断言，F8 普通场景 / S 外部按住后切入；无需 Painter |
| 快捷键、布局字母、编辑优先级与重建 | [ui/shortcuts](ui/shortcuts/main.go) | 三平台；AZERTY/QWERTZ、IME、重复键；完整原生验收待完成 |
| 样式驱动 Icon、UI 协调、矢量缩放 | [ui/icon](ui/icon/main.go) | Linux / Windows / macOS；Software / OpenGL，Windows 另有 Direct2D |
| SVG 孔洞/描边/渐变、主题/原色、缓存与共享资源 | [ui/svgicon](ui/svgicon/main.go) | Linux / Windows / macOS；Software / OpenGL，Windows 另有 Direct2D；1x / 2x |
| 图片帧内销毁、替换及已记录绘制的有效期 | [graphics/image_lifetime](graphics/image_lifetime/main.go) | 三平台；Software / OpenGL，Windows 另有 Direct2D；1x / 2x |
| 预乘颜色、渐变、文字 alpha、透明 Clear | [graphics/alpha](graphics/alpha/main.go) | Linux / Windows / macOS；Software / OpenGL / Direct2D；可选透明模式 |
| TextView 共享模型、独立视图、Cluster 选择与滚动 | [gui/textview](gui/textview/main.go) | 三平台；1/10 MiB；含 IME、闪烁、拖选自动滚动步骤，实际平台覆盖见 DesignTextEditing |
| UI 文本编辑、单行复用、重建与 modern 主题 | [ui/textediting](ui/textediting/main.go) | 三平台；TextInput/TextView、只读、Submit、主题切换、1/10 MiB |
| 原生文件拖入/拖出、Copy/取消/拒绝及一次性终态 | [platform/dragdrop](platform/dragdrop/main.go) | 三平台桌面；源/目标独立启动，明确 -result 预期，不用 GUI 控件 |
| 原生拖放结束、取消/释放/拒绝及位置 | [platform/drag_end](platform/drag_end/main.go) | 三平台公开事件断言；受控目标、准确 payload、Begin/End 次数、结束结果及最终本地注册窗口命中；`-min-duration 35s` 验证长拖动，Linux 另验无应答目标；不依赖私有 trace，位置需结合实际输入核对 |
| 五种独立 DnD 数据源/接收区、预览、跨窗口/Popover 与文件互操作 | [gui/dragdrop](gui/dragdrop/main.go) | 三平台；默认自建临时文件，可重复 -file，支持 -large-text/-preview/-actions |
| DnD 声明式重建、回调更新、禁用及移除/恢复绑定 | [ui/dragdrop](ui/dragdrop/main.go) | 三平台；窗口显示信号计数、最后接收值及 FAIL 数，退出检查失败 |
| 标签页基础交互、栏内排序与样式 | [widgets/tabs](widgets/tabs/main.go) | 三平台桌面；Integrated、Modern/兜底样式；人工或 Computer Use 观察 |
| 两态开关、水平拖动、动画及声明协调 | [widgets/switch](widgets/switch/main.go) | 三平台；Modern/显式兜底，Software/OpenGL、Windows 另可 Direct2D；1x/2x；包含取消、子按钮、重建及 Timer 清理断言 |
| 已获胜本地拖动接管原生 DnD | [gui/drag_handoff](gui/drag_handoff/main.go) | 三平台；接受、Esc、未接受、本地拖动、短点击；对象身份及一次终态断言 |
| 命令式标签跨窗口移交与取消 | [widgets/tab_transfer](widgets/tab_transfer/main.go) | 默认连续体验推开占位、源标签隐藏、单标签转移；`-expect accepted` 等严格模式自动断言；编辑页步骤见 editor.go |
| 命令式标签拖出创建新窗口 | [widgets/tab_detach](widgets/tab_detach/main.go) | 默认连续体验；单标签仅移入已有窗口，不新建；`-expect detached` 等严格模式检查准备、定位、移交及失败保留 |
| 声明式标签转移、拖出与重建 | [widgets/ui_tab_transfer](widgets/ui_tab_transfer/main.go) | 三平台桌面；Native/Integrated；原页面与 State、声明重建及源移除 |
| 通用数据树、虚拟化、按需加载、选择与键盘导航 | [widgets/tree_view](widgets/tree_view/main.go) | 三平台桌面；默认 UI，`-gui` 命令式，`-fallback-style` 兜底样式；1x/2x、万行定位、行内按钮、上下文菜单、重建和 Snapshot 断言 |
| 数据表格、固定表头、列宽调整、排序请求与双向滚动 | [widgets/table](widgets/table/main.go) | 三平台桌面；默认 UI，`-gui` 命令式，`-fallback-style` 兜底样式；`-appearance=plain/zebra/rows/grid/thick` 或 Appearance 切换背景／网格／粗线，检查自绘单元格避让；1x/2x、万行与换行、选择和键盘导航、菜单、拖宽与 Esc 回退、重建、亮暗切换和 Snapshot 断言 |

各入口注释是操作步骤与预期的权威来源；表格仅作主题索引。`go test ./...` 编译这些命令包，
不会执行 main。命令包显示 `[no test files]` 是没有 Go 测试函数，不是条件跳过或窗口验收通过。
所有截图、日志和可执行文件输出到仓库外；运行条件、失败和未执行项遵循测试约定单独记录。
