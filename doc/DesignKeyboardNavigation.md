# 默认 Tab 焦点导航

> 2026-10-05：初始切片只实现控件之间的焦点转移。FocusVisible 已实现；
> 第 7 节保留历史验证，当前配色见第 8 节。2026-11-09 已扩充自身/子树准入与
> 指针聚焦配置，当前契约及验收见第 9 节，不扩展平台能力或控件内部键盘行为。

## 1. 初始切片的职责与范围

Tab 负责在当前 Widget 树中转移焦点，方向键、Enter、Space、编辑、选择等由
具体控件自己的 EventController 处理。不因本切片修改 Button 激活、菜单导航、
TreeView/TableView 的记录导航，也不承诺虚拟化模型中未挂载记录的 Tab 顺序。

复用 `Widget.Focusable`、`SetFocusable` 和 Window/Popover 的现有焦点入口。
不增加 FocusOnClick、TabStop、TabIndex、FocusScope 或 FocusNavigator。
以后应用确实需要“可以被程序聚焦但不参加 Tab”等区别时，再独立讨论；本轮
`Focusable=false` 同时禁止自身聚焦和参加 Tab，**不排除子控件**。

## 2. 顺序与输入契约

- 当前宿主内按实际控件树深度优先前序遍历：自身、再按 Children 兄弟顺序访问。
  Shift+Tab 为反序；窗口自绘装饰树在内容树之后，仍按自身 Focusable 判断。
- 隐藏或 gui.IsEnabled 为 false 的节点及其整棵子树跳过；销毁、未挂载对象不参加。只读取当时已有的树，
  不查询数据模型、不创建离屏记录、不维护持久焦点链。
- 仅 KeyDown 的 Tab 或 Shift+Tab 是候选；额外修饰键、KeyUp 不触发。重复
  KeyDown 可继续导航，不另外解释按键时序。
- 无焦点时选第一项，Shift+Tab 选末项；到两端在同一宿主循环。没有候选不消费；
  只有一项消费 Tab，但不反复发送同一焦点信号。
- 原有宿主控制器、三阶段控件控制器及快捷键先运行。StopPropagation 或原生
  Handled 已置位时不执行默认导航。TextView.AcceptsTab 可以先消费 Tab；自定义
  Tab 快捷键也可覆盖默认行为。平台/IME 若未交付事件，GUI 不合成替代输入。
- 调用宿主原有 SetFocusedWidget，复用 Widget 的 Focused/ContainsFocus 信号、
  Snapshot 和 Window IME 更新。回调主动更换焦点、移除控件、销毁窗口时，
  尊重最新状态，不继续向失效目标发送进入通知或滚动请求。

## 3. Popover 与滚动

模态 Popover 继续沿用 owner 转发按键，不获取原生键盘焦点。owner 在转发前
抑制自身的原生默认行为；转发副本有独立消费结果，不能把 owner 已置位的
Handled 误当作 Popover 控件已消费。Tab 只遍历 Popover 树，owner 焦点保留。

Popover.SetFocusedWidget 更新同一 EventDispatcher 的焦点 crossing，让状态、
信号与 Snapshot 一致；Hide/release 在清空 dispatcher 前清除逻辑焦点。没有
增加 UI 焦点状态机、模态焦点恢复或新的 Popover 公共方法。

默认导航后沿目标祖先链，由内向外调用已有 ScrollView.ScrollIntoView：使用
目标相对未滚动内容的 DIP 矩形。普通内容的原点包含负 scroll offset；Scrollable
内容原点固定，需补回其 offset。内层滚动后重新计算外层坐标。回调改变焦点或
移除目标时停止。已挂载虚拟行可参与，但滚动可能回收行，本轮不保证跨记录续接。

## 4. 实现路由

- `gui/focus.go`：私有默认遍历和滚入视口，不新增公共接口。
- `gui/event.go`：事件分发末尾兜底；焦点 crossing 在回调重入时遵守最新路径。
- `gui/window.go`：复用焦点入口；模态按键独立消费；拒绝销毁目标重新聚焦。
- `gui/popover.go`：补齐焦点状态通知和隐藏时清理。
- `gui/widget.go`：Children 注释明确默认导航顺序。
- 测试留在既有 event/popover/text_view 测试文件；独立桌面程序位于
  `tests/window/gui/tab_navigation`，中文操作说明见入口。
- `ui` 原有声明与协调过程不变；没有任何新绑定。

## 5. 参考依据与取舍

查阅日期 2026-10-05，以下为文档承诺或指定版本源码，不是 GOUI 已实测行为：

- GTK4 的 [Widget.focusable 文档](https://docs.gtk.org/gtk4/property.Widget.focusable.html)
  将自身可聚焦与整个子树的 can-focus 区别开。
  [GTK 4.18.0 gtk_widget_real_focus](https://github.com/GNOME/gtk/blob/4.18.0/gtk/gtkwidget.c#L4883)
  对自身和子控件区分正反向访问。GOUI 采用自身能力及确定的树顺序，暂不引入第二个属性。
- Qt 的 [键盘焦点文档](https://doc.qt.io/qt-6/focus.html) 支持窗口内循环及控件处理
  Tab；[QWidget.setTabOrder](https://doc.qt.io/qt-6/qwidget.html#setTabOrder) 是相对
  焦点链重排，并非数字 TabIndex。本轮没有实际顺序定制需求，不引入另一套链。
- Flutter 的 [默认焦点遍历请求](https://api.flutter.dev/flutter/widgets/FocusTraversalPolicy/defaultTraversalRequestFocusCallback.html)
  同时请求焦点及滚入可见区域。GOUI 复用自身 ScrollIntoView，不引入 traversal policy。
- [Avalonia 11.3.0 KeyboardNavigationHandler](https://github.com/AvaloniaUI/Avalonia/blob/11.3.0/src/Avalonia.Base/Input/KeyboardNavigationHandler.cs)
  为未处理的 Tab 提供导航。采用兜底时机，不照搬其模式、Attached Properties 或 scope。

## 6. 验证记录

- 新增包内回归在旧实现复现失败：首次 Tab、循环顺序、Popover 隔离、未消费
  TextView Tab、嵌套 ScrollView 均无预期焦点转移。
- 包内回归通过：输入三阶段消费与快捷键优先级、额外修饰键/KeyUp、树重排/
  移除、隐藏/不可聚焦、无目标/单目标、装饰树、焦点离开/进入/ContainsFocus
  回调中的移除/销毁/重定向、Popover 隐藏与 Snapshot、普通嵌套及 Scrollable
  子控件坐标。不会为测试导出内部状态。
- `go test ./...`：Linux amd64/Go 1.25.8，49 个有测试包、1896 个测试/子测试
  通过、零失败、14 个既有人工/opt-in 用例跳过。不把跳过算作通过；全仓仍
  沿用既有原生窗口和图像测试的运行及产物规则，没有迁移或清理这些测试。
- `go test -race ./gui ./ui ./widgets/...` 通过；清除 DISPLAY 后重新运行本轮
  Tab/Popover/TextView 回归通过，确认新增逻辑测试不需要桌面。
- `CGO_ENABLED=0 GOOS=windows GOARCH=amd64` 和
  `CGO_ENABLED=0 GOOS=darwin GOARCH=arm64` 的 GUI 测试目标 `go test -c`、
  tab_navigation 窗口程序 `go build` 通过。未在两端运行原生桌面；未执行 Linux arm64。
- 本机 Ubuntu 22.04 amd64/X11/Xfwm4 4.16.1、DISPLAY=:3.0：真实桌面按键
  经正常原生事件路径验证。Software 1x：正反向/无焦点/循环、ASCII 输入仅进入
  当前字段、隐藏与 Focusable 跳过、自定义 Tab 快捷键、TextView AcceptsTab、
  Popover 循环/关闭重开/owner 焦点保留、滚入远处字段及 F6 状态断言通过。
  OpenGL 1x 与 Software 2x：导航、滚动、Popover 与 F6 状态断言通过，并检查
  远处字段完整进入视口。OpenGL 实际使用 Mesa 23.2.1 llvmpipe，非硬件 GPU。
  2x 由环境变量请求，不等同于多屏或真实混合 DPI。例程已正常关闭。
- 测试产物在 `/tmp/goui-tab-nav.5ow24C`，含全仓 JSON、两端编译目标与桌面截图；
  真实输入焦点日志见本轮终端输出。没有 DevServer 或直接语义动作代替输入。
- 未执行：Windows/macOS 桌面、Direct2D、中文 IME 候选/组合、多屏混合 DPI、
  其他 Linux WM 与硬件 OpenGL；按窗口入口说明后续验收。控件内键盘行为、
  自定义 Tab 顺序、虚拟记录焦点续接、Dialog 模态/焦点恢复不属于本轮。

## 7. FocusVisible（2026-10-05）

逻辑焦点与键盘提示分离。`Widget.Focused()` 表示当前持有逻辑焦点；新增只读
`Widget.FocusVisible()` 表示该控件持有焦点且其宿主正在显示键盘导航提示。
Snapshot 同步报告 `focusVisible`。它不是选区，也不影响 Focusable、输入路由或
控件激活；不新增可设置的状态、FocusReason、UI 绑定或平台 API。

每个 Window/Popover 的 rootBase 独立保存提示模式。成功的默认 Tab/Shift+Tab
导航开启提示，包括只有一个目标的情况；PointerDown 关闭，哪怕焦点没有变。
PointerMove 不关闭，程序聚焦不改变模式。导航在焦点通知之前更新模式，让
回调读取一致状态；模式变化只请求重绘和语义更新，不重复发出 Focused 信号。
Popover 隐藏后清除模式；模态外部点击仍关闭 owner 提示。此阶段不把其他按键
自动归类为导航，也不按原生窗口激活状态改变既有逻辑焦点语义。

样式新增 `style.FocusVisible`，保留 `Focused` 与独立 `focus` Part。解析顺序：
同名默认 Part/Normal → 同名当前 Part/Normal → 当前 Part/Focused →
当前 Part/FocusVisible；中间的 Focused 仅对 FocusVisible 生效。
同 Part 的 Focused 字段可以被更具体的 FocusVisible 规则覆盖。
Button/MenuButton 继续先绘制 Hover/Pressed 主体，再绘制 focus Part 的背景与
可选边框，再由内核绘制子树，不隐式修改 Label 样式。普通 Focused 背景显式透明、
边框宽为 0；FocusVisible 使用 2 DIP 柔化主题色内边框，背景透明，当前规则见 7.3。
TextInput 既有 Focused 边框不变。应用可自定义任意 FocusVisible 背景/边框。

实现涉及 gui 的 root/focus/event/widget/info/style/window/popover、style 的状态与
回退规则、theme/modern；测试扩充既有 event/popover/button/style/modern 文件与
tab_navigation 窗口例程，不另建焦点管理框架。

参考（2026-10-05 查阅）：[GTK4 4.23.4 has_visible_focus](https://docs.gtk.org/gtk4/method.Widget.has_visible_focus.html)
区别逻辑焦点与可见提示；[Window.set_focus_visible](https://docs.gtk.org/gtk4/method.Window.set_focus_visible.html)
说明该模式由输入管理，通常不应由应用设置。
[Avalonia focus 文档](https://docs.avaloniaui.net/docs/input-interaction/focus)（页面更新于
2026-09-18）也区分 :focus 与 :focus-visible。仅采纳状态分离，不引入 CSS
级联、焦点 scope 或额外导航管理器。上述是文档依据，不是 GOUI 桌面验收结果。

### 7.1 实现与验证

以下保留初版底色提示的验证记录；当前边框方案及最新验收见 7.3。

- 只读状态、快照、独立宿主模式、键鼠切换、Popover 隐藏复位与透明角外部
  点击、style 回退及 Button/MenuButton 独立提示绘制已实现；没有 platform/UI
  变更。Modern 普通与 Primary 按钮均提供亮暗中性提示，主题色不影响提示底色。
- 包内回归覆盖：焦点通知读取新模式、同目标切换只重绘不布局/重复通知、
  程序聚焦继承、移除/销毁/重入、被消费或非导航按键不启用提示、Popover
  owner 隔离/隐藏复位；样式精确匹配、Focused 继承与显式零覆盖。
- Software 无桌面像素测试覆盖 Button/MenuButton、1x/2x：普通/键盘/hover/pressed、
  自定义 Focused 边框在两种模式下保留、子绘制不被提示覆盖。独立预期用预乘
  sRGB 字节：210 的底色叠加黑 alpha24 为 190，hover230 为 208；容差 1 字节
  只用于整数合成量化，直线边框与不透明子绘制用精确断言。
- `go test -count=1 -json ./...` 通过：Linux amd64、Go 1.25.8，49 个有测试包、
  1907 个测试/子测试通过，14 个既有人工/opt-in 跳过，无失败；51 个包无测试。
  既有原生窗口/图像测试仍会创建窗口及写产物，本次没有迁移/清理这些测试。
- `go test -race -count=1 ./gui ./style ./theme/modern ./ui ./widgets/...` 通过。
  清除 DISPLAY 后独立执行本轮 Tab/FocusVisible/Popover/Button 像素/TextView
  消费回归通过。Windows amd64/macOS arm64（CGO_ENABLED=0）的 GUI 测试目标
  与 tab_navigation 程序编译通过，不等于原生执行。
- 本机 Ubuntu 22.04 amd64/X11/Xfwm4 4.16.1、DISPLAY=:3.0：Software 1x、OpenGL
  1x、Software 2x，真实按键/鼠标经正常事件路径验证无焦点→Tab、同按钮移动
  保留提示/点击关闭、逆向再正向恢复、F8 亮暗切换、模态 Popover 导航及关闭。
  OpenGL 1x 与 Software 2x 的 7 个 F6 检查点另按独立预期逐项核对日志。
  Software 1x 另验证 Popover 点击不关闭 owner 提示、owner 外部点击关闭提示、
  重开复位及没有重复 Focused 通知。所有窗口正常关闭，进程退出码 0。
- Modern 1x Software/OpenGL 的按钮内部截图采样一致：亮色键盘提示 RGB231；
  Software hover+提示为 RGB223、同按钮鼠标聚焦为 RGB246；暗色提示为
  RGB(69,69,75)，由 RGB(50,50,56) 叠加白 alpha24 独立计算。2x 亮暗采样
  相同；采样避开文字和圆角边缘，窗口内容不透明。OpenGL 为 Mesa 23.2.1
  llvmpipe，非硬件 GPU；2x 是环境变量请求，不代表混合 DPI。
- 证据在 `/tmp/goui-focus-visible.FP0XI6`，含最终全仓 JSON、编译目标、截图和
  OpenGL/Software2x 输入日志；Software1x 输入日志保留在本轮终端。
  没有使用 DevServer、直接语义动作或修改系统设置。
- 未执行：Windows/macOS 原生桌面、Direct2D、Linux arm64、硬件 OpenGL、其它
  WM、中文组合/候选窗及混合 DPI。控件内部方向键等行为不在本切片范围。

### 7.2 FocusVisible 边框调整（2026-10-05）

以下为 1 DIP 版本的设计与验收记录；当前宽度及验收见 7.3。

按确认的外观方案，Button/MenuButton（包括 Modern Primary）取消键盘提示底色，
改用 1 DIP 内绘圆角边框；保持主体 Hover/Pressed 背景、圆角、布局及子样式。
Modern 边框为既有对比度调整后的主题焦点色与中性边框色混合：75% focus +
25% palette.border，量化为不透明 sRGB 8 位颜色，不用透明笔画随背景二次混色。
GUI 最小兜底使用固定淡蓝色 RGB(102,147,217)，不引入主题调色能力。

普通 Focused 仍是透明背景、零边框；TextInput 自己的 Focused 边框不变，也不
额外绘制 focus Part。不是通过检测已有边框来猜测行为，各控件负责自己的绘制。
FocusVisible 状态、Snapshot、输入路由、样式回退、平台/UI API 都不修改。

FocusVisible 仍先继承同 Part 的 Focused，但其默认背景/颜色/宽度是明确覆盖值。
只追加 Focused 边框规则可改变普通焦点，不会覆盖更具体的键盘默认边框；应用要
自定义 Tab 提示，追加 FocusVisible 规则。未明确设置的其他字段（如圆角）仍继承，
显式 BorderWidth(0) 可关闭键盘边框。原有自定义背景和边框能力不删除。

此为更温和的 1 DIP 提示，沿用 TextInput 焦点色的 3:1 校验，但不把该承诺套给
柔化后的按钮边框。针对亮暗主题、黑/白/红/蓝主题色检查量化颜色与可辨性；不改变
既有对比度断言或放宽其阈值。

涉及 gui/style.go、theme/modern/modern.go、既有 button/text_editor/modern 测试与
tab_navigation 窗口例程说明，并同步样式设计、Plan/Roadmap。验证包含无桌面
像素（1x/2x，边框精确宽度、无底色/布局变化、鼠标模式、自定义覆盖和 TextInput
不叠加）、全仓测试/相关 race、Windows/macOS 编译及本机 Software/OpenGL 操作。

实现及本轮验证完成：

- 生产改动仅在 GUI 兜底和 Modern 样式规则；回归集中在既有 button、
  text_editor 和 modern 测试文件，窗口例程增加 F9 主题色与 F10 Primary 切换。
- 临时恢复旧底色规则后，Button/MenuButton 的 1x/2x 四条像素回归均失败
  （边缘灰色 190，不是预期淡蓝），Modern 检查以 alpha24 底色失败；恢复新版
  后全部通过。原有样式扩展能力保留，测试明确区分 Focused 与 FocusVisible
  的精确状态优先级，没有删除断言或放宽对比度条件。
- 无桌面 Software 像素断言检查四边、内部、透明圆角、布局与像素尺寸、
  Hover/Pressed、键鼠切换、自定义颜色/宽度/显式零及子绘制；新增 TextInput
  1x/2x 回归，故意设置冲突的 FocusVisible 规则，仍只绘制自己的 2 DIP 边框。
  直线采样为不透明预乘 sRGB 字节，使用独立颜色预期且不设误差。
- `go test -count=1 -json ./...`：Linux amd64、Go 1.25.8，49 个有测试包、
  1910 个测试/子测试通过，14 个既有人工/opt-in 跳过，零失败；51 个包无测试。
  既有窗口与 PNG 测试的桌面/文件副作用未迁移、未清理。
- `go test -race -count=1 ./gui ./style ./theme/modern ./ui ./widgets/...` 通过；
  清除 DISPLAY 后的 Tab/FocusVisible/Popover/Button/TextInput 像素/TextView
  消费及 Modern 对比度回归通过。Windows amd64/macOS arm64（CGO_ENABLED=0）
  GUI 测试目标与 tab_navigation 编译通过，不等于原生运行。
- 本机 Ubuntu 22.04 amd64、X11/Xvnc、Xfwm4 4.16.1、DISPLAY=:3.0：Software
  与 OpenGL 均完成 1x/2x 真实 Tab、移动保留提示、同按钮点击关闭、反向/正向
  恢复、亮暗与蓝/红 Primary 切换、模态 Popover 导航/关闭。Software1x 的
  黑/白暗色另有截图采样；四组日志分别核对 11/9/9/10 个独立预期检查点。
  action 始终为窗口 DIP Rect(12,205,322,35)，点击不产生重复焦点通知；
  所有窗口关闭、进程退出码 0。没有使用 DevServer 或改变系统设置。
- 软件/GL 四组直线边缘截图均符合独立预期：亮色蓝 RGB(101,145,213)、
  暗色蓝 RGB(91,130,190)、暗色红 Primary RGB(208,80,82)、亮色红 Primary
  RGB(227,52,54)。采样避开文字/圆角；1x 第一行、2x 前两行是边框，下一行
  即原主体底色，没有额外遮罩。OpenGL2x TextInput 的前四行是原焦点色
  RGB(69,127,215)，第五行白色，确认仍为 2 DIP 单层边框。
- 日志、最终全仓 JSON、旧规则失败证据、编译目标和截图在
  `/tmp/goui-focus-border.cRqaKy`。OpenGL 为 Mesa 23.2.1 llvmpipe/LLVM15，
  不是硬件 GPU；2x 为环境变量请求，实际窗口受桌面高度限制，不代表混合 DPI。
- 未执行：Windows/macOS 原生桌面、Direct2D、Linux arm64、硬件 OpenGL、
  其它 WM、中文组合/候选窗和混合 DPI。控件内部方向键等不在本切片范围。

### 7.3 FocusVisible 边框加宽（2026-10-05）

用户体验反馈指出柔化主题色的 1 DIP 提示辨识度不足。Button/MenuButton 与
Modern Primary 的默认 FocusVisible 边框统一改为 2 DIP；仍在控件内部绘制，
不改变圆角、柔化颜色、透明背景、布局或子绘制。普通鼠标焦点不装饰，TextInput
保留自己的原有 Focused 边框；不增加 API、不修改 Painter/平台或输入机制。

只改 GUI 兜底与 Modern 两处规则。包内像素测试在 1x/2x 检查四边逐物理像素
宽度：分别前 2/4 层为边框，下一层是主体底色；同时保留内部/透明圆角/布局、
Hover/Pressed、鼠标切换、子绘制、TextInput 不叠加的覆盖。自定义 Focused
采用不同的 3 DIP 宽度以验证精确状态优先级，FocusVisible 的 0/1/3 DIP
覆盖仍有效。Modern 的亮暗、蓝/黑/白/红与普通/Primary 色值断言不变。
窗口例程及样式文档同步到 2 DIP，旧版验收留在 7.2，不代替本轮结果。

实现及本轮验证完成：

- 旧 1 DIP 规则下四个 Button/MenuButton 像素子测试在第二 DIP 的边框位置
  按预期失败，Modern 宽度检查也失败；两处规则改为 2 后通过。不放宽像素
  误差，不改颜色预期；TextInput 的原有回归通过。
- 清除 DISPLAY 后执行 Button 像素、Modern 焦点外观、TextInput 不叠加、样式
  解析和量化对比度回归通过。`go test -count=1 -json ./...`：Linux amd64、
  Go 1.25.8，49 个有测试包、1910 个测试/子测试通过，14 个既有人工/opt-in
  跳过，零失败；51 个包无测试。既有窗口/PNG 测试副作用未迁移或清理。
- `go test -race -count=1 ./gui ./style ./theme/modern ./ui ./widgets/...` 通过。
  Windows amd64/macOS arm64（CGO_ENABLED=0）GUI 测试与 tab_navigation
  窗口目标编译通过，不等于原生运行。
- 本机 Ubuntu 22.04.5 amd64、X11/Xvnc、Xfwm4 4.16.1、DISPLAY=:3.0：
  Software/OpenGL 均完成 1x/2x 真实 Tab、鼠标移动/点击切换、反向/正向恢复、
  亮暗/蓝色与红色 Primary 切换、模态 Popover 导航/关闭。四组日志分别核对
  9/9/9/10 个独立预期检查点；action 始终为窗口 DIP Rect(12,205,322,35)，
  同目标点击不重复发送焦点通知，全部正常关闭、退出码 0。没有使用 DevServer。
- 四组截图的独立颜色预期与 7.2 一致；避开文字/圆角，在不透明内容上采样
  直线顶部：1x 前两行、2x 前四行是边框，下一行即主体底色。Software1x
  Hover 内部仍为 RGB246，点击后只有原 1 DIP 中性边框；OpenGL2x TextInput
  仍为原焦点色的四行边框及白色内部，没有额外 focus Pass。
- 证据在 `/tmp/goui-focus-2dip.FGMHCs`，包含旧规则失败、全仓 JSON、race
  日志、各后端输入日志/截图与编译目标。OpenGL 为 Mesa 23.2.1 llvmpipe/
  LLVM15；2x 为环境变量请求，受桌面高度限制，不代表混合 DPI。
- 未执行：Windows/macOS 原生桌面、Direct2D、Linux arm64、硬件 OpenGL、
  其它 WM、中文 IME 和混合 DPI。不用此前 1 DIP 版验收补齐这些缺口。

## 8. 当前外观契约（2026-10-14 复核）

Basic 与 Modern 的普通 Button／MenuButton 保留独立 focus Part，鼠标 Focused
默认不新增装饰，FocusVisible 使用 2 DIP 内绘边框，不改布局或子控件样式。
Modern 普通按钮／Flat 使用柔化主题焦点色；仅 Primary 使用亮色深灰 #414147、
暗色浅灰 #D8D8DE，不随主题色改变。TextInput 继续使用自己的原有焦点边框。
具体规则见 [Modern 预设](DesignModernPresets.md) 与 theme/basic、theme/modern。

上述是当前源码契约，不把第 7 节旧 Primary 配色的截图／像素结果追认为本轮验收。

## 9. 焦点准入与指针聚焦（2026-11-09，已实现）

新增两个 Widget 配置：CanContainFocus/SetCanContainFocus 控制自身及实际 Widget
子树的焦点准入，FocusOnPointer/SetFocusOnPointer 控制自身在 PointerDown 时的
默认聚焦；二者默认 true。Focusable 保留自身能力语义，Focused/ContainsFocus
保留自身与子树的事实状态。不新增平台 API、焦点取消信号、FocusReason 或另一套
焦点管理设施；历史第1节不增加焦点属性的限制由本节扩充。

### 9.1 契约

- 接收焦点需要自身 Focusable、自己及祖先 CanContainFocus，并满足原有生命周期、
  Root、启用、可见与模态限制。getter 返回自身配置，不是有效状态查询。
- Focusable=false 只排除自身。CanContainFocus=false 拒绝整棵子树的指针、Tab、
  主动聚焦与模态初始/恢复聚焦；后代不能用 true 覆盖祖先限制，原配置保持不变。
- 关闭准入先发布限制，再通过宿主已有入口清除子树中的焦点，正常发送焦点通知、
  更新 IME。外部焦点不变；恢复准入不自动聚焦。相同值不重复通知。
- FocusOnPointer 只影响自身，不继承、不清除已有焦点，也不限制 Tab/主动聚焦。
  PointerDown 检查命中分支准入，再向上寻找最近 Focusable；该候选关闭指针聚焦
  则保留当前焦点并停止，不能绕到外部祖先。CanContainFocus 关闭的分支同样停止。
- 默认聚焦仍在宿主控制器后、普通捕获控制器前；不改传播顺序、按键或手势规则。
  保留焦点不是指定输入框；此前 Tab 到按钮后，鼠标操作不会强制把焦点移回输入框。
- 这两个设置不是 Disabled 或 Modal。限制准入不阻断鼠标/拖放、不添加禁用外观，
  不禁用 Command/窗口快捷键、不撤销模态边界。独立 Popover Root 不经锚点继承。
- 主动打开 Dialog/弹层可以按自身语义调整焦点，仍走统一准入。控件自身的指针
  handler 不应重复强制聚焦锚点，绕过 FocusOnPointer；显式 Open 与键盘操作可请求
  聚焦。菜单锚点无法聚焦时仍可鼠标操作，已有依赖锚点焦点的键盘路径不伪造焦点。

### 9.2 实现边界

WidgetBase 以反向私有 bool 保持默认 true，配置不依靠各构造器重复初始化。
共享私有判断按当前祖先链检查，不增加继承缓存或公共有效状态辅助接口。Surface
（含 Window）、Popover、Tab、Modal 的初始/恢复均使用同一准入规则。同 Root
重新挂接使用最终父链重新验证；失败要清除无效焦点，不能忽略 SetFocusedWidget
的 false。回调重入、改变配置、转移焦点或销毁时尊重最新状态。

UI ViewBase 增加同名 bool 修饰，复用 bits.Bitmap、首次挂载默认值快照与 Modify
合并；省略恢复构造默认值。先收紧准入再放宽，防止更新时临时开放焦点；setter
后的存活检查保持。WidgetInfo 增加 canContainFocus/focusOnPointer，明确报告自身
配置；现有 Focusable 和实际焦点状态不改。配置只请求语义更新，实际焦点变化
复用已有样式/绘制失效，不因配置本身新增布局或样式状态。

### 9.3 验证计划

包内覆盖嵌套命中/祖先限制、Tab 正反序、主动聚焦、动态关闭及恢复、同 Root
重新挂接、回调重入/销毁、模态初始/恢复、Popover 隔离、IME 预编辑和选区保持，
以及声明式默认值/显式覆盖/省略恢复/局部更新。检查鼠标点击期间没有失焦通知，
而不仅是点击后焦点又回来。空间/回车激活复用原有 Button/CheckButton 控制器。

独立窗口例程按 tests/window 规范提供查找栏：普通按钮与 CheckButton 关闭指针
聚焦，但可 Tab/Shift+Tab，Space/Enter 激活；另可切换整个按钮组的准入，确认
指针仍有效。通过公共输入路径验收，不使用 DevServer。执行结果见下节。

参考：GTK4 文档的 [can-focus](https://docs.gtk.org/gtk4/method.Widget.set_can_focus.html)
与 [focus-on-click](https://docs.gtk.org/gtk4/property.Widget.focus-on-click.html)，以及
[GTK4.20.0 Button 按下处理](https://github.com/GNOME/gtk/blob/4.20.0/gtk/gtkbutton.c#L325)。
采用能力/子树准入/触发方式分离，沿用 GOUI PointerDown 与统一分发，不复制 GTK
控件各自调用 grab_focus 的实现方式。

### 9.4 执行记录（2026-11-09）

实现位于 gui 的 Widget/焦点入口、MenuButton 与 widgets.DropDown 的输入路径，
以及 ui 的公共修饰；没有修改 platform、事件传播顺序或控件内部键盘行为。
`tests/window/ui/focus_policy` 提供中文操作说明，普通启动只设置初始焦点，
`-auto` 在真实布局后经公共 DispatchEvent 验证，再断言退出；不使用 DevServer。

- **逻辑测试通过**：最终已跟踪源码加本轮新增窗口例程的隔离副本，在 Linux amd64、
  Go 1.25.8、独立 Xvnc/Xfwm4 会话执行
  `CGO_ENABLED=0 go test -json -count=1 ./...`，4616 通过、11 既有跳过、0 失败。
  `CGO_ENABLED=1 go test -race -count=1 ./gui ./ui ./widgets/...` 全部通过。
- **旧行为负对照成立**：仅以 overlay 恢复原 gui/event.go，新回归的“最近候选不向外
  回退”和“保留焦点、IME 预编辑、选区”两项明确失败；最终实现下通过。预编辑的
  此项验证为包内状态断言，不冒充真实输入法桌面验收。
- **本机窗口通过**：Linux/X11、Xvnc/Xfwm4 开启合成，Software/OpenGL llvmpipe、
  1x/2x 四组自动断言通过。另在 OpenGL 1x 用 XTest 原生输入验证选中后点击四个
  查找控件，失焦次数为 0；Tab/Shift+Tab、Space/Enter、准入切换、主动聚焦及重建
  通过。原生输入 `native` 后全局重建仍保留文本与选区。
- **远端窗口通过**：Windows 10 19045 amd64、Direct2D 1x；macOS 15.7.7 arm64、
  OpenGL 1x。均在交互桌面运行最终窗口例程，十阶段自动断言和正常退出通过。
  另用原生鼠标/键盘验证：全选后点击四个控件，输入框保持焦点、选区且无失焦；
  Tab 后 Space/Enter 激活；输入 `native` 后全局重建仍保留文本和光标。
  实际后端由 Painter 创建的 Image 类型与 PixelScale 确认，不把构建或 SSH 启动
  当作桌面验收。
- **原工作区全仓命令失败**：`go test ./...` 被本轮开始前已有、未跟踪的旧例程与
  测试阻塞，包括旧 Window.Show/SetTitle 调用和已移除的 UI 私有字段。未修改、
  删除或提交这些文件；上述全仓通过指排除它们后的已跟踪源码隔离副本。
- **环境与未执行范围**：独立 X11 初始继承 `@im=fcitx`，该独立 display 没有 Fcitx
  服务，原生字符未提交；仅对测试进程改用 `XMODIFIERS=@im=none` 后原生输入通过，
  未改系统设置。真实中文输入法组合、触摸/笔、硬件 GPU/混合 DPI，以及移动端
  原生运行未执行；共享内核的合成输入结果不代替这些验收。

非模态 MenuButton 在锚点没有焦点时仅接受指针操作，不挪用输入框的按键。
DropDown 自己的模态弹层保持独立 Root 的键盘/焦点语义；显式 Open 仍可请求
锚点焦点。两者均不通过临时移走再还原焦点来伪造 FocusOnPointer=false。
