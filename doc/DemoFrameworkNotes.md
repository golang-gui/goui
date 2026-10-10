# Demo 开发发现的框架问题与使用体验

记录时间：2026-10-07。范围：`example/demo`；本机 Linux/X11、Xfwm4 合成桌面，
Software 配置 1x/2x、OpenGL 配置 1x，具体操作和后端判定边界见 DesignDemo。
只记录有代码依据或实际观察的事项，不把例程失误写成框架缺陷。
初始例程开发不顺带改框架；后续经确认的背景隔离及 Linux Integrated 布局
修复分别记录在第 3、9 节；后续滚动方向与嵌套 Stretch 修正在第 1、7 节。
其余建议仍需按具体应用需求确认范围。

## 1. 普通表单滚动缺少有限宽度的便利用法（已补齐）

分类：契约允许的行为，但常见表单场景使用成本较高，不是已确认的算法缺陷。

复现：把 `VBox(Card(...), HWrap(...))` 直接作为 `ui.ScrollView` 的 child，外层
给足宽度。最初卡片仅占约 500 DIP，自然尺寸小于约 900 DIP 的 viewport；
HWrap 在无限宽测量时也不会按 viewport 换行。

修复前依据：`gui/scroll_view.go` 的普通 child 使用双轴无限约束，并按测量出的
contentWidth/contentHeight 排列；只有 `Scrollable` 收到实际 viewport。
`CrossStretch` 只撑开 VBox 内的孩子，不能改变 VBox 自己被安排的宽度。

初始例程处理：`flowWidget` 实现已有 Scrollable，约束孩子宽度、保留纵向
自然高度。随后确认的方向切片删除适配，直接使用
`ScrollView(...).Direction(ui.ScrollVertical)`；ScrollBoth 默认仍保留自然尺寸。

实现不扩充 Scrollable/ListView 接口，不靠固定 MinWidth 假装响应式。
同时修正滚动条相互占位的独立错误，契约和回归见
[DesignScrollLayout](DesignScrollLayout.md)。

## 2. 不存在或不完整的文本 StyleName 容易静默丢失文本

分类：样式契约的可诊断性问题；例程曾误用不存在的 `button-text`，已经纠正。

实测：按钮的图标可见，Label 文本不可见；去掉错误 StyleName，使用普通 Label
样式后显示正常。自定义标题只声明字号/颜色时，字体也与 Modern 正文不同。

依据：`gui.ResolveStyle` 仅在整个样式表不存在时使用兜底表；未知 selector 不会
自动继承 `label`。`textFormatFromStyle` 的字体/字号/颜色完全来自解析结果。
这符合“不隐式级联”的原则，但拼错名称可能直接生成零字号且没有提示。

本例处理：使用真实 Label 样式；自定义文本规则明确字体、字号、颜色。

建议：补充自定义文本样式的完整示例；考虑开发期的可选缺失字段诊断，不能
通过隐式父继承来“修复”，也不建议仅为此引入 CSS selector 或资源系统。

## 3. 不同窗口的背景选择与透明示范（已修复）

历史问题：Window 原来固定解析 `window`，只有应用级样式表。例程为显示透明
None 改写共享背景，使主窗口保留给 Integrated 的边缘也透明；根 Widget 的
不透明 Bin 不能覆盖宿主拥有的区域。这是例程处理不当，不是透明后端失效。
第一轮只纠正了 Integrated 的 Transparent() 误触，显式透明 None 仍有副作用。

修复：Window 沿用 Popover 的 StyleName/SetStyleName，空名称解析 `window`；
仅选择此窗口 BackgroundColor，不向孩子传样式。背景和装饰亮暗都使用同一
选择；框架依然管理装饰形状、尺寸和命中。透明示范独立选择
`demo-transparent-window`，不再修改共享 `window`，也不需要维护额外透明记录
或在窗口关闭时恢复全局背景。Transparent() 始终报告创建配置，未改变其语义。

参考 GTK 4.24.1 的 [Window 独立 CSS 节点](https://docs.gtk.org/gtk4/class.Window.html#css-nodes)
及 [实例 CSS class](https://docs.gtk.org/gtk4/method.Widget.add_css_class.html)，采用
按对象选择样式的职责，不采用其 CSS、选择器级联或 Widget 继承体系。GOUI
仍由一个应用样式表解析小值模型，Window 仍不是 Widget。这里只补 GUI 接口，
透明示范本来就通过 GUI 创建，不为此扩展 UI Root 或添加单独的颜色属性。

无桌面回归直接渲染两个独立 Software surface，覆盖 1x/2x、边缘预乘 RGBA、
主题变更、透明窗销毁、内容/语义/布局保持及装饰亮暗选择；不再测试已删除的
“修改全局背景并恢复”记录算法。窗口输入与桌面合成验证范围见 DesignDemo。

## 4. UI 对部分既有 GUI 能力仍需手工桥接

分类：声明式便利性缺口，不是协调器错误。

- ScrollBar 尚无 UI View；例程用约 30 行标准 WidgetView 管理 signal handle。
- 通用 Popover 尚无 UI 声明；例程从 ID 找锚点，使用 GUI 接口创建，离页销毁。
- 透明窗口创建未提供 UI modifier；例程使用 GUI.NewWindow，而非改 Root 特例。

建议：优先根据实际应用确认最有价值的薄绑定，不引入第二套状态机。需要 GUI
逃生口时，清楚说明 UI 与 GUI 的所有权，不以通用协调器中特判作为捷径。

## 5. 文本工具栏操作需应用协调历史选区

分类：编辑能力的便利性限制，不是当前已复现的撤销错误。

TextView 的内置快捷键/上下文菜单在编辑器内部管理历史选区；外部工具栏没有
公开 Undo/Redo 命令，当前需要调用 TextModel.UndoWithSelection/RedoWithSelection，
再把返回的非 nil Selection 设置给视图。本例按这个公开契约实现，不调用私有
textEditor，也不把直接改状态当成键盘回归测试。

建议：实用编辑工具栏需求增多时，统一设计公开的编辑命令能力；不能暴露内部
状态或把 DevServer 语义执行当作正常命令接口。

## 6. X11 指向不可用 XIM 服务时，普通文本输入没有提交

分类：特定输入环境下已复现的健壮性问题；尚未扩展验证至普通桌面会话，未修复。

复现环境：额外 Xvnc `:83` + Xfwm4，继承 `XMODIFIERS=@im=fcitx`，该 display
没有相应 XIM 服务。TextInput/TextView 能获得焦点，Ctrl+A / Ctrl+End / Enter
等按键正常，XTest 输入普通 ASCII 字母后模型没有修改。保持程序和桌面不变，
仅该进程使用 `XMODIFIERS=@im=none` 重启后，同一操作产生正确文本提交。

代码依据：X11 Platform 初始化用 SetLocaleModifiers("") / OpenIM；`newInputMethod`
在 p.im 为零时返回 unavailable；普通文本提交依赖 IC 的 Utf8LookupString。
这与外部 XIM 服务不可用相符，但本轮未采集 p.im 或底层诊断，不把服务缺失
判定扩大为已经证明的唯一原因，更不把它写成所有 Linux 的文本输入问题。

建议：另开输入专项，明确“配置的 XIM 服务不可用”时本地 XIM 降级与错误反馈
契约，覆盖未安装、服务退出/重启和独立 display。示例不全局改 XMODIFIERS，
本地验证只使用进程级 local XIM；中文预编辑和候选窗本轮未验证。

## 7. 纵向滚动表单内横排 Stretch 的测量/排列差异（已修复）

分类：独立无窗口回归确认的 LinearLayout 自然基线约束问题。

复现：本例纵向 Scrollable 的 VBox 中加入一个带 MinHeight(110) 的背景 Bin，
内部是 `HBox(Label(...).MainWeight(1)).Padding(16).CrossAlign(CrossStretch)`。
表单在无限高测量后沿自然高度排列；接收区实际高度被撑大，后面的说明超出
已报告的 ContentSize，滚轮无法把末尾完整移入 viewport。去掉该 HBox 的
CrossStretch，保留水平权重、Padding 和接收区最小高度后，末尾可正常滚到。

修复前代码依据：layout/linear.go 的 compute/childConstraint 给非加权孩子传父级
主轴最大值；stretchCrossExtent 对无限最大值采用自然测量，对有限最大值
消费全部交叉空间。外层 VBox 从无限高测量到有限高 Arrange 时，内层 HBox 的
交叉约束随之改变，重新计算出的高度可能大于第一次测量的自然高度。

独立回归中 Measurement=300×140，但末项底部曾到 170 DIP；行列转置同样
复现。现统一自然主轴无界基线，仅加权孩子在最终分配后紧约束重测；末项
底部与测量一致。不扩大 ContentSize，不加隐藏补偿。Button/MenuButton 单内容
居中从 LinearLayout 解耦，保留最大尺寸换行与 baseline。兼容影响见
[DesignScrollLayout](DesignScrollLayout.md)。

## 8. 例程错误与预期行为，不记为框架缺陷

- 分类根节点最初没有不同 ID，同类型同位置的 ScrollView 被协调器正常复用，
  把上一分类的滚动偏移带到下一页。本例为各分类赋稳定的不同 ID；共享业务
  模型继续保留。不是协调器应主动猜测“已经换页”，也不需要新增 Key 概念。

- 单色 SVG 会把白色前景路径也染色。最初的带白色字母/勾选图案变成轮廓实心；
  本例改为透明底的描边图标，并另列 WithRawColors。不是 SVG 解析错误。
- 动态 Size 的 Icon 最初被自己的 MinHeight 和父级 CrossStretch 撑大，虽然
  动画读值变化，绘制尺寸不变。固定占位高度改放到外层 HBox，内层 Icon 保留
  自然尺寸；这是例程的约束设置错误，不是动画系统未更新。
- SplitView 的显式 StartSize/Ratio 是受控值，不是只应用一次的初值。最初例程
  没有 OnResize 保存调整结果，F5 重建恢复初值；布局/数据页现已保存尺寸和比例。
  无尺寸声明才是由控件自行保留的非受控用法。契约见 widgets/ui/split.go 顶部。
- 截图紧跟 mouseup 时可能先拍到移交中的源占位；等待 GUI 更新后，源/目标
  页面与新窗口正确提交。不能据此认定标签回弹或脱离失败。
- Xfwm4 可把较宽的脱离窗口退回工作区内；这是窗口管理器行为，不承诺拖动
  释放点在屏幕边缘时精确维持窗口原点。
- 额外标签窗口可能被激活的主窗口遮住。拖入前把目标移到可见位置，不把被
  遮挡位置当作另一窗口的输入目标。
- 通用 Popover 默认 Modeless，不承诺 Esc 或外部点击自动关闭。本例打开页内
  遮罩时先隐藏手工 Popover，避免原生浮窗盖住普通 Overlay；这属于应用协调，
  不要求 Overlay 反向管理窗口外 Popup。

后续新发现继续追加具体操作、源码位置、实际结果和建议；未验证的可能性
保持“待验证”，不升级成三平台事实。

## 9. Linux Integrated 的布局区域与绘制安全边界不一致（已修复）

分类：既定实现的契约/使用成本问题；2026-10-07 经确认修正 Linux GUI 自绘路径。

代码事实：gui/window.go 的 floatingFrame/frameInset 为 Linux GUI 自绘 Integrated
普通状态保留约 5 DIP 边缘；gui/root.go 在该安全矩形裁剪 Widget 绘制，但
旧 layoutFrame 向根 Widget 分配完整窗口矩形。ScrollView 将 8 DIP 滚动条安排
在自身最右侧，因此满宽内容会把其中 5 DIP 放进不可绘制区，也与缩放命中重叠。
用户反馈和本机截图相符，不是 Software/OpenGL 的滚动条宽度或透明合成错误。

首轮例程修复：主体加 8 DIP 外侧留白，HeaderBar 保持现状。该补偿曾满足演示，
但不是要求应用了解私有装饰尺寸的长期解法。Window 自动布局避让完成后，
按用户确认移除主体的这段重复留白，仅保留导航/页面等自身的内部间距。

参考依据（源码行为，不推断为四框架统一承诺）：

- GTK 4.24.1 的 [GtkWindow 原生布局](https://github.com/GNOME/gtk/blob/4.24.1/gtk/gtkwindow.c#L2130)
  在分配前排除 CSD shadow extent；[GtkWidget.allocate](https://github.com/GNOME/gtk/blob/4.24.1/gtk/gtkwidget.c#L4056)
  再扣除 CSS border/padding，把剩余空间交给 size_allocate/layout manager。
  [GtkWindow 官方说明](https://docs.gtk.org/gtk4/class.Window.html#css-nodes)
  区分 .solid-csd 的 CSS border 与 .csd 的窗口外 shadow resize area。
  GTK 的区域数值/圆角裁剪实现不等于 GOUI 的安全矩形，不照搬其 CSS/影子体系。
- Avalonia 11.3.0 的 [WindowDecorationMargin](https://github.com/AvaloniaUI/Avalonia/blob/11.3.0/src/Avalonia.Controls/Window.cs#L298)
  来自平台 ExtendedMargins，属于窗口装饰事实；[Fluent Window 模板](https://github.com/AvaloniaUI/Avalonia/blob/11.3.0/src/Avalonia.Themes.Fluent/Controls/Window.xaml#L10)
  区分 chrome overlay 与 content presenter。仅借鉴窗口统一协调装饰/内容的职责，
  不引入模板、公共 FrameMetrics 或让每个控件读装饰数据。

已采用：由 gui.Window 将根 Widget 的可布局区域与实际安全绘制区域统一，窗口
背景/边框仍覆盖完整 surface；不要只在 Paint 时截断，也不要修改 Painter 或
为 ScrollView、TabBar 等逐个添加 Chrome 特判。条件是存在窗口占用且不允许
Widget 绘制的边缘，不是只检查 Chrome==Integrated 就三平台固定缩进。当前
只有 Linux GUI 自绘 Integrated 普通状态需要这段安全边缘；最大化/全屏恢复完整区域，
Native 和特殊用途 None 不凭空加留白，原生已经排除的边框不能重复扣除。

实施范围：根 Rect 的偏移进入现有局部/窗口坐标转换；HeaderBar 查询以根布局
顶部为准，不再要求窗口 Y==0。窗口 Chrome 计入内容起点，按钮自身不变，
原有距完整窗口右侧的 6 DIP 不重复叠加避让。由内容推导的最小尺寸加入占用
边缘，显式窗口 MinSize 保持既有语义，状态/缩放变化重新布局及更新派生 hint。
内缩后边缘不再命中 Widget，EventDispatcher 补齐无 Widget 命中时的窗口
host controller 分发；仍走既有输入与手势竞争，保证边缘缩放，不修改平台。
Position、WorkArea 和平台坐标不重新定义；Snapshot、命中、滚动到焦点及
RenderWidget 跟随现有 Rect 转换。补齐无额外 Padding 的贴边滚动条、
不同 HeaderBar 高度/分段、控制按钮居中、最大化/恢复和 1x/2x 回归，再按
涉及的平台实际实现验收。DesignGUIWindowChrome 已同步从“仅绘制裁剪、全窗口
布局”调整为 Linux 内侧布局；不增加 safe-area 公共接口。样式动画、常见滚动
及高频 UI 绑定另行设计，不包含在本次修复中。

## 10. 高频 UI 绑定优化（记录，暂缓实施）

2026-10-07 用户确认：当前优先保证功能正确，以下只记录优化方向，不修改实现。
尚未进行同环境前后性能对照，不将源码上的重复工作写成已测得的性能瓶颈。

源码观察：

- UI RequestUpdate 合并尚未执行的更新任务，不承诺按绘制帧合并；更新仍协调
  所有声明窗口。ui/app.go 每轮应用标题，gui.Window.SetTitle 尚无同值防护。
- ui/root.go 的 UpdateChildren 每轮规范化、匹配孩子并请求相对重排；
  WidgetBase 的重排查找使用 slices.Index，稳定子树也可能重复做顺序查找。
- demo 每次构建 Modern 规则与样式表，样式比较能避免重复安装，但不能避免
  构造与比较本身。ListView/TreeView/TableView 的刷新则有真实语义：构建回调
  可以读取已经变化的外部状态，不能仅凭模型地址或函数地址跳过。

后续可评估的最小范围：标题同值防护、稳定孩子的线性快速路径、demo 按实际
亮暗及主题色缓存样式表。快速路径必须检查真实 Widget 顺序，不能只相信上一轮
声明；不能改变 State.Set 的通知、View.Update 的可变值语义或列表刷新契约。
不引入隐式依赖追踪、额外 Key、通用对象猜测比较或强制同帧协调。

验证要求：100/1000 个孩子的稳定更新与实际增删/重排对照，记录分配及耗时；
保留原生标题调用次数、最新回调、外部模型变化和外部重排的正确性断言。
只有具体应用测量证明仍有必要，才设计局部重建等更大的机制。

## 11. 深度体验反馈：功能优先的待修复清单

记录时间：2026-10-07。以下现象来自用户对 demo 的体验，不是本轮 Agent
完成的桌面复现。用户明确指出第 4 项为 Linux、第 5 项为 macOS；其他项的
平台、绘制后端、缩放及桌面版本尚未完整记录。源码核对与因果推断分开列出，
全部尚未修复、尚未验收；不扩大为三平台共同缺陷。

### 11.1 拖动过程中鼠标指针跳变

状态：待明确是光标形状、屏幕位置还是拖动预览变化，以及具体拖动操作与平台。
gui/window.go 的 applyCursor 按 hoverPath 选择光标，而拖动事件可以按手势
捕获路径派发；两者在拖过其他控件时可能不同。这只是形状变化的候选原因，
不能据此认定原生指针位置跳变，也不能统一冻结所有 hover。

涉及：gui/window.go、gui/event.go、gui/gesture.go；如仅原生拖放发生，再检查
对应平台的反馈光标和从 GUI 手势交接原生会话的时机。
验收：分别拖标签、SplitView、列宽与滚动条，跨不同光标控件和窗口边缘，
区分实际指针与预览；结束/取消后光标正常恢复。

### 11.2 TreeView 与 Modern 风格不和谐

分类：外观反馈，不是已确认的渲染算法错误。Modern 已采用中性选中底色，但
仍需一起检查行高、留白、展开箭头及当前行边框。demo 的自定义行在
example/demo/data.go 中创建普通 Label/Icon，没有选择 tree-item-text /
tree-item-icon；不能通过父控件隐式传样式来修正。

涉及：theme/modern/modern.go、widgets/tree_row.go、widgets/tree_expander.go、
example/demo/data.go。先用普通正文、菜单、表格作相邻对照，再确定样式调整，
保留选择与键盘定位的可辨性，不简单换成大面积主题色。
验收：亮暗/主题色、选中/悬停/当前行/禁用、不同层级及 1x/2x；不改变行命中、
虚拟化和 Snapshot。

### 11.3 TableView 调整列宽后排序还原

已确认源码原因：example/demo/data.go 每轮声明固定 Width(180/100/110/210)，
没有 OnResize 保存用户调整；排序请求触发 RequestUpdate，声明的受控值重新
写回列。widgets/ui/table_view.go 的 Width 是受控属性，不是初值，现有非受控
用法已支持保留用户宽度。不能为修 demo 改成“setter 只生效一次”。

建议：demo 为各列保留宽度，OnResize 更新对应值；保留排序与模型独立。
涉及：example/demo 的状态及 data.go，相关例程测试；无需改变 TableView API。
验收：拖动各列后排序、选择行、F5、切换主题均保留宽度，明确重置才还原。

### 11.4 Linux 标签拖至外部应用：脱离失败或接收提示残留

源码事实：widgets/tab_transfer_drag.go 的标签会话仅 SetLocal；无可移植内容时
X11 只提供 application/x-goui-local 标记，不声明文本或文件。该标记不把 Go
对象发送给外部进程；不能为了让外部接受而伪装文本、文件或 URI。

已确认的协议清理缺口：platform/linux/x11/dragdrop_linux.go 的 dropOrEnd 在
目标未接受时直接 end，未经过已有 leaveTarget；普通 end 也不发送 XdndLeave。
取消路径会发送 Leave，已经发送 Drop 的路径等待 Finished，两者不能混淆。
这与 gedit 提示残留相符，但关联尚须实测。Qt 6.8.3 XCB 的
[send_leave/cancel 源码](https://github.com/qt/qtbase/blob/v6.8.3/src/plugins/platforms/xcb/qxcbdrag.cpp)
也明确区分撤销目标交互与实际 Drop；参考其协议职责，不照搬事件循环结构。

建议：未提交 Drop 的会话结束前清理已进入的目标；保留正常 Drop/Finished、
取消和故障结果的区别。GoLand 上未创建窗口尚不能定因，需要核对 XdndAware/
Proxy、Status 与结束结果。widgets/tab_detach.go 不在取消、错误或应用内目标
上自动创建新窗，不能把协议超时改成成功，也不能写 GoLand 特判。

涉及：X11 拖放实现与窗口例程；若发现通用 GUI 结束结果契约问题再调整上层。
验收：拖至桌面、GoLand、gedit、另一 GOUI TabBar，分别松手、离开、Esc；
外部拒绝的提示必须清除，正常脱离只创建一窗，取消不创建；正常文件/文本
拖放仍保留真实类型和接收结果。必要时用最小外部目标覆盖拒绝、代理及不响应。

### 11.5 macOS 脱离窗口的标签不在鼠标下

待验证的主要假设：example/demo/tabs.go 用源 TabBar 的局部位置计算目标窗口
位置，但主窗口的标签 HeaderBar 嵌在内容页，新窗口的 HeaderBar 在顶部；
macOS 后者还会避让左侧原生控制按钮，目标 TabBar 起点不一定等于源起点。
Linux/Windows 的右侧控制区通常不改变标签起点，不能由此断言 AppKit 坐标错误。

建议：先记录释放点、Hotspot、源/目标 TabBar 实际起点，区分目标布局偏移与
平台窗口位置换算。Qt 6.8.3 Cocoa 的
[draggingSession:endedAtPoint:operation:](https://github.com/qt/qtbase/blob/v6.8.3/src/plugins/platforms/cocoa/qnsview_dragging.mm)
按原生屏幕、窗口及 View 的坐标边界换算，可用于检查 AppKit 调用；当前只是
参考其坐标分层，并未证明 GOUI 平台实现有误。

涉及：先检查 example/demo/tabs.go 与 HeaderBar 控制区避让；如坐标契约仍有
偏差再改 platform/darwin/cocoa。不添加经验偏移、固定红绿灯宽度或通用隐藏
窗口预布局 API。验收：主窗口与独立标签窗口各自脱离，原生按钮位置及缩放
变化后，释放点仍在新标签的对应 Hotspot；Linux/Windows 复验不回归。

### 11.6 移交后关闭确认仍出现在原窗口

已确认源码原因：closingTab 是 demo 全局状态，确认 Overlay 只放在 tabsPage；
requestTabClose 收到 tabWindow 却未使用，反而强制主窗口跳到标签页。
独立 tabWindowView 没有对应 Overlay。这是例程的窗口归属问题，不是 Widget
移交应自动迁移所有外部业务弹层。

建议：每个标签工作区使用同一套内容和确认 Overlay，关闭请求记录实际所属
窗口/标签；确认期间发生移交或窗口销毁时明确取消或重新定位所有者。
涉及：example/demo/tabs.go、状态与测试，无需扩展 Overlay 或 UI Root。
验收：移交/脱离后从标签关闭按钮和菜单发起确认，只在当前所属窗口出现；
取消不丢文档，确认只删除对应文档，销毁窗口不留下悬挂确认。

### 11.7 新窗口 TabBar 空白区不能拖动窗口

已确认源码原因：Transferable 在整个 TabBar 上注册 gui.DropTarget；HeaderBar
的 defaultDragRegion 把它归为未知交互控制器，所以空白命中路径也被排除。
DropTarget.HandleEvent 对普通输入为空，实际处理的是独立的拖放 offer。

建议：在 HeaderBar 的保守分类中识别已知、不处理普通指针输入的 DropTarget；
真实标签/关闭按钮的 Focusable、Click、Drag 仍阻止 Caption，未知自定义控制器
仍保守。不要把整个 TabBar 或它的所有后代无条件声明成拖动区。
涉及：gui/header_bar.go 及回归、标签窗口例程；不增加控件类型特判。
验收：空白可移动/双击，标签仍选择/排序/移交，关闭与溢出按钮正常；应用内
拖入目标仍显示占位并接收。

### 11.8 Tab 聚焦按钮后 Space / Enter 无响应

已确认源码原因：gui.Button / MenuButton 仅安装点击及悬停控制器，没有键盘
激活处理；Tab 已完成焦点转移，不能由 EventDispatcher 冒充按钮语义。
GTK 4.24.1 的 [Button.activate 文档](https://docs.gtk.org/gtk4/signal.Button.activate.html)
把 Space / Enter 绑定到按钮激活，应用监听 clicked；采用控件负责激活的职责，
不为此引入另一套全局动作分发。

建议：控件复用 KeyEventController，键盘与鼠标走同一点击/打开信号路径；明确
按下/释放、Repeat、修饰键和取消规则，不借此增加所有 Widget 的自动激活。
涉及：gui/button.go、gui/menu.go、行为测试及键盘窗口例程；UI OnClick 无新 API。
验收：普通/Primary/MenuButton 经 Tab 聚焦后激活一次，重复键不重复提交；
禁用、失焦、销毁和取消安全，鼠标点击及 FocusVisible 外观不回归。

### 11.9 右侧滚动条仍显得靠边

分类：页面留白的视觉反馈，与第 9 节已修复的边缘裁剪不同。demo 的 page
把 Padding(16) 放在滚动内容内部，不会移动属于 ScrollView 的滚动条；Window
的自动安全边缘只负责缩放/裁剪边界，不代表页面设计上的视觉间隔。

建议：在普通页面 ScrollView 的外部提供对称的页面留白，先以 8 DIP 作视觉
候选，再实际对照；不恢复全窗口的装饰补偿，不改变 HeaderBar/WindowControls，
也不新增 ScrollView 或 Painter 的窗口边缘策略。
涉及：example/demo 的 page 布局及相关断言。验收：亮暗、窄窗、最大化/恢复、
1x/2x 下完整滑块与对称留白，滚动内容测量、命中及末尾可见性保持正确。

### 修复顺序与实施边界

1. 优先处理已确认的功能问题：列宽受控绑定、关闭确认归属、按钮键盘激活、
   Transferable TabBar 的空白 Caption；补独立回归，不改无关公共 API。
2. 处理原生拖放清理及 macOS 定位；光标问题在明确具体操作后并入对应切片。
   先本机验证，再在受影响平台验收，不把 GoLand/gedit 现象当成同一根因。
3. 对照调整 TreeView 和页面留白；样式与行为分开，保留已修复的窗口安全边界。
4. 上述功能稳定前不实施第 10 节优化，不随修复扩大动画、资源或协调器设计。

本轮仅源码核对及文档记录，未修改实现、未执行上述桌面验收。

记录核对：git diff --check 通过。`env -u DISPLAY go test ./...` 未全通过：
platform/typography/test 的 TestTypography 在创建平台时返回 `can not open display`；
该既有测试需要显示连接，不因文档修改而改变。其余包返回成功或无测试文件；
桌面相关测试的跳过不算窗口验收通过。本轮不迁移测试、不修改跳过条件，
也不为了文档记录在用户桌面启动测试窗口。

## 12. 深度体验反馈修复与验证记录

2026-10-07，用户授权逐项修复、验证、提交。以下是本次实现及实际验证，
与第 11 节只做源码分析的历史记录分开；高频 UI 绑定优化仍暂缓。

### 修改及边界

| 反馈 | 确认原因与修正 | 自动回归 / 指定实测 |
| --- | --- | --- |
| 快速拖动缩放箭头跳变 | Window 原先只读实时 hover；现有已接受手势的捕获目标在捕获期间拥有光标，结束后恢复实时 hover，不冻结 hover 本身。 | 捕获、释放、取消、隐藏、禁用、卸载、失焦和原生接管；Linux XFixes、Windows GetCursorInfo 检查 SplitView / 列宽拖动。 |
| TreeView 外观不和谐 | Modern 树行选中色改为窗口底色与前景的浅中性色；鼠标焦点不叠加 current 边框，FocusVisible 保留细边框。demo Label/Icon 分别声明自己的样式。 | 亮暗色独立色值/对比度断言，自定义 current 样式原有覆盖保留；Linux 外观复验。未改变 TableView 配色。 |
| 排序恢复列宽 | demo 重建一直声明固定 Width；现在各列 OnResize 回写自己的宽度状态。 | Linux / Windows 拖宽后排序及 F5 重建保留宽度；TableView 的受控 Width 契约不变。 |
| Linux 外部标签拖放失败或提示残留 | 本地标记没有外部可读取的表示，却仍进入外部客户端等待 Status；只含本地对象的 offer 现不进入外部目标。另在未发 Drop 的结束路径补 Leave。 | 本地/混合/可移植格式及目标身份断言；gedit 上标签脱离且无接收提示；独立原生拒绝目标记录 Enter、Motion、Leave，没有 Drop，源结果为非取消、无错误的未接收。GoLand 本身未运行。 |
| macOS 新窗口标签偏离鼠标 | 源 HeaderBar 嵌在主页面，目标 HeaderBar 顶置并避让原生按钮；复制源偏移不适用。按源已知尺寸和目标 Chrome 实际占位计算目标起点，GUI 保留原生水平锚点而只垂直居中。 | 控制区/根起点独立断言，原生水平锚点与定位去重回归；macOS 非工作区边缘释放点对应新标签 Hotspot。Linux / Windows 脱离复验。 |
| 关闭确认回到原窗口 | 原来只有主页面拥有全局 closingTab；现由每个 tabWindow 持有确认并共用工作区 Overlay。 | 窗口归属、错误窗口请求、移交撤销旧确认回归；Linux / macOS 关闭脱离标签时只遮罩目标窗口。 |
| 新窗口空白标签栏不拖窗 | HeaderBar 将整个 TabBar 注册的 DropTarget 当成普通鼠标交互。现在识别该已知控制器，不把未知控制器或 Click/Drag/Wheel 放行。 | 命中路径控制器策略断言；三平台脱离后的空白区域实际拖窗。 |
| Tab 后 Space / Enter 不激活 | Button / MenuButton 只有鼠标控制器；增加私有 KeyEventController，共用点击或打开菜单路径。 | 仅自身聚焦、无额外修饰时在非 Repeat KeyDown 激活一次；子控件输入、禁用、释放、回调销毁回归。三平台真实 Tab、Space、Enter 计数均为 2，F6 公开焦点断言通过。 |
| 滚动条视觉靠边 | 原留白在滚动内容内部，不移动 ScrollView 自己的滑块；普通滚动页面在视口外增加对称 8 DIP 留白。 | 独立分配/快照断言及 Linux 1x 观察；不改变 Window 安全边界、HeaderBar、Controls 或全页标签布局。 |

没有新增公共 GUI 或布局接口；没有伪装文本/文件类型、按外部应用名称分支、
把协议超时改成成功，或改动 macOS 原生窗口位置换算。可移植文本/文件 offer
仍走原生 XDND；已发送 Drop 的会话保留 Finished 协议，不错误发送 Leave。
HeaderBar 高度只决定原生按钮的 Y，不能在首次显示时改变已报告的水平占位。

### 命令与结果

通过：

- 隔离显示下 `DISPLAY=:88 XMODIFIERS=@im=none go test -count=1 ./...`。
- `go test -race` 的 gui、widgets、ui、widgets/ui、example/demo、theme/modern、
  platform/linux/x11 七包（同一隔离显示）。
- Windows amd64 / macOS arm64 的 `CGO_ENABLED=0` demo、GUI、Modern、demo 测试
  目标和 tab_navigation 窗口程序编译；远端执行 GUI、Modern、demo 包内测试
  均 PASS，最终 GUI 修改再次原生运行通过。不是远端全仓测试。
- 两个目标的 `GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build ./...` 和
  `GOOS=darwin GOARCH=arm64 CGO_ENABLED=0 go build ./...` 均通过。
- `git diff --check`、`go vet ./theme/modern ./example/demo`，以及
  `go vet -composites=false ./gui ./widgets ./theme/modern ./example/demo`；
  完整 vet 的既有报警仍保留下述失败记录，不算完整 vet 通过。

失败 / 环境限制：

- 第 11 节的无 DISPLAY 全仓命令仍受既有 TestTypography 的显示连接依赖限制；
  本轮使用自己的隔离显示完成全仓，不扩大 skip 或删除测试。
- `go vet ./gui ./widgets ./theme/modern ./example/demo` 的 composites 检查报告
  既有 TextSelection/TextRange 和 Popover 测试里的非键名结构体字面值。
  报警所在 text_editor、text_input、text_view、popover 测试等文件未在本轮改动；
  不把该命令算作通过，不在缺陷修复中顺带重写这些文件。

原生桌面环境与实际范围：

- Linux amd64：任务专用 Xvnc 1.12.0 + Xfwm4 合成器，不操作用户原有显示。
  实际 OpenGL、应用 1x；另有本机 Software 1x 初轮观察。完成上述指定输入，
  原生光标名称在捕获期间保持 sb_h_double_arrow，释放后恢复 left_ptr。
  gedit 为本轮空白进程，没有打开、写入或保存用户文件。独立拒绝目标使用
  platform/drag_end 窗口例程，Leave 在松手后收到，而非把取消当拒绝。
- Windows 10 19045 amd64：RDP 1600×900、1x；Tab 验证的 NewImage 实际类型为
  direct2d.imageResource。实际列宽排序/F5、SplitView/列宽光标、标签脱离及空白
  拖窗通过。原生光标检查在交互会话内用 IDC_SIZEWE 句柄作独立预期，不以
  GUI 自己的 Cursor 枚举自证正确；SSH Session 0 不能代替该桌面检查。
- macOS 15.7.7 arm64：VNC 1650×1050、实际 OpenGL、应用 1x。实际 Hotspot 定位、
  空白拖窗、目标关闭遮罩与按钮键盘输入通过。屏幕边缘处系统可能限制窗口
  位置，位置验收选用不触及工作区边缘的释放点，不把系统退避算坐标错误。

未执行：GoLand 指定应用视觉复验、其他发行版/WM、无合成器、全部后端组合、
本轮完整 2x/混合 DPI、跨屏定位、菜单弹出按钮的三平台原生键盘操作。
MenuButton 的打开路径已由包内测试验证，但不冒充 Popup 桌面验收。
既有全仓测试会创建窗口及源码目录 PNG；不迁移、不删除、不把跳过算通过。

日志和截图保存在仓库外 `/tmp/goui-demo-fixes.ZmXskm`，主要证据为
delivery-tests.log、final-race.log、三平台 tab-nav 日志、win-column-cursor.log、
win-split-cursor.log、reject-source.log / reject-target.log，以及对应桌面截图。
不依赖 DevServer；验收结束只关闭本轮程序和临时桌面/任务，保留既有远端会话。

### 工具调用问题记录

- remote-shell 的 Windows status 报 default_shell=powershell，但 SSH 默认实际
  按 cmd 解析；显式调用 powershell.exe -EncodedCommand 后正常。
- Windows SSH 的 Console.In.ReadToEnd / ReadLine 上传路径曾等待不结束，改用
  已有 SSH ControlMaster 上的 SCP；没有重启 SSH 或修改安全设置。
- remote-computer-use 的动作成功仅代表输入投递。窗口首次激活、绘制和截图
  可能尚未完成，需等待短暂的处理时间并重新观察，不能立即截图判为应用失败。
  本轮用显式按下/移动及 finally release-input 验证持续捕获，不修改技能文件。
- SSH Session 0 的 GetCursorInfo 失败是会话限制；临时交互任务中的原生探针
  可以检查真实桌面光标。该差异不是 GOUI 的渲染或捕获缺陷。

## 13. TableView Modern 外观收敛

2026-10-09，按用户确认的方案调整 TableView，不改平台、GUI 公共 API、Cell
填满分配、文本样式隔离或编辑器焦点行为。

参考 [GTK4 ColumnView 官方文档](https://docs.gtk.org/gtk4/class.ColumnView.html)
（查阅时 Library 4.24.1）的 header/listview 结构及独立行列分隔线设置，采用
结构与内容可分别定制的思路；具体颜色是 GOUI Modern 的设计，不声称来自 GTK。

- 表格 selected 与 TreeView 共用浅中性色阶，亮暗色随窗口底色／前景变化，
  不随主题色变化；保留圆角以及 hover／pressed 层级，TreeView 原有颜色不变。
- 表头略区别于正文；外框和表头底线保持 1 DIP，颜色比行线稍清晰。默认仍为
  淡水平线，不启用垂直网格或斑马色，应用的显式外观设置继续生效。
- 行绘制向 current Part 转交 TableView 的 FocusVisible；Modern 默认鼠标
  current 无轮廓，键盘焦点显示 1 DIP 中性轮廓。这里只传状态，不硬编码禁止
  鼠标轮廓，应用可以继续覆盖 current 样式；兜底样式保持原有策略。
- 验收例程补中文操作说明和实际 backend/pixel-scale 日志，避免将 OpenGL
  初始化失败后的 Software 回退算作 OpenGL 验收。

自动回归先在旧实现复现：原选中色／边框层级与独立预期不符，current 未传递
FocusVisible，1x/2x 边缘样本为选中绿色而非键盘边框蓝色。修正后测试通过。
颜色测试使用独立 sRGB 字节预期并检查文字对比度；像素测试使用正常根输入分发，
覆盖鼠标选中、Tab、hover 保留、重复点击清除、自定义鼠标轮廓与失焦。
采样直边和内部的不透明预乘颜色，整数 DIP 的 1x/2x 满覆盖样本无需容差。
既有网格、填满与编辑器测试保留，不删除断言或放宽误差。

验证通过：

- 无 DISPLAY 的 widgets、theme/modern、tests/window/widgets/table 包测试。
- 隔离显示下 `DISPLAY=:89 XMODIFIERS=@im=none go test -count=1 ./...`。
- 同显示下 `go test -race -count=1 ./theme/modern ./widgets/... ./tests/window/widgets/table ./example/demo`。
- `go vet ./theme/modern ./widgets ./tests/window/widgets/table ./example/demo` 及 `git diff --check`。
- Windows amd64／macOS arm64 的 `CGO_ENABLED=0 go build ./...`；两个目标的
  widgets、theme/modern 测试程序交叉编译，不计作原生运行。
- 本机 Linux amd64，任务专用 Xvnc 1.12.0 + Xfwm4 4.16.1 合成器。表格例程
  实际 Software/OpenGL 各应用 1x/2x；亮暗切换、选中、键盘提示及 F6 语义断言
  通过，OpenGL 1x 的排序／Rebuild 保留选择，Software 2x 的 thick 模式编辑器
  边界与青色内容检查通过。桌面直边采样另确认 OpenGL 的细框为 1/2 个物理
  像素、鼠标不叠轮廓；不使用跨平台字体金图。
- `example/demo` 本机 OpenGL 配置 1x 的数据浏览页，TreeView/TableView 亮暗
  并排观察及选中通过；该项不代替四种配置都有实际后端日志的表格例程检查。

未执行：Windows/macOS 原生桌面及 Direct2D、其他 WM、混合 DPI、全部菜单／IME
交互。本轮不作性能结论；既有全仓测试会创建窗口及源码目录 PNG，未迁移或删除，
条件跳过不算窗口验收通过。不操作用户原有桌面、不修改系统设置或剪贴板。
日志和截图存于仓库外 `/tmp/goui-table-appearance.46cBKZ`。

## 14. 嵌套滚轮与原生拖出差异

2026-10-10 用户反馈及后续确认：本轮仅修复滚轮消费，另外两项明确保留原生行为。

- 内外层同时滚动是已确认的 GUI 缺陷。ScrollView 的滚轮回调处理偏移后没有
  StopPropagation；TabBar 的实际标签/箭头也有同类遗漏。按请求轴范围消费整次事件，
  包括边界和小数舍入未移动的输入；无对应范围交给祖先。复用既有事件分发，不增加
  滚动链配置或特殊 dispatcher 分支。窗口例程的长短内层列表、正常输入回归和验证
  结果见 [DesignScrollLayout](DesignScrollLayout.md#嵌套滚轮消费2026-10-10)。
- 标签使用应用内对象格式，不是文本；用户对照其他 Windows 原生拖放标签应用后
  确认可接受外部目标消费原生拖放，从而不触发新窗口的差异。用户观察不是本轮远端
  实测，也不证明某个编辑器实际读取了哪种格式。本轮不重解释已执行的 native action、
  不伪造接受、不新增框架模拟拖放；拖出即建窗并跟随指针属于应用策略。
- GOUI 的 SetPosition 提交客户区原点请求，没有主动工作区退避。Linux 窗口管理器、
  AppKit 和 Windows 对屏幕边缘位置的客观约束可以不同。用户确认保持当前契约：
  正常位置尽量保留 Hotspot，边缘系统限制可使抓取点不再精确对应，不用额外的显示后
  搬窗、绕过系统约束或三平台统一主动钳制来补偿。

平台 DnD/定位代码本轮未修改；现有 Linux 本地对象 offer 的外部限制保持不变。
框架保证数据契约、事件与结果的清晰语义，不承诺所有原生目标和窗口管理器行为相同。
