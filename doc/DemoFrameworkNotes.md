# Demo 开发发现的框架问题与使用体验

记录时间：2026-10-07。范围：`example/demo`；本机 Linux/X11、Xfwm4 合成桌面，
Software 配置 1x/2x、OpenGL 配置 1x，具体操作和后端判定边界见 DesignDemo。
只记录有代码依据或实际观察的事项，不把例程失误写成框架缺陷。
初始例程开发不顺带改框架；后续经确认的背景隔离及 Linux Integrated 布局
修复分别记录在第 3、9 节，其余建议仍需按具体应用需求确认范围。

## 1. 普通表单滚动缺少有限宽度的便利用法

分类：契约允许的行为，但常见表单场景使用成本较高，不是已确认的算法缺陷。

复现：把 `VBox(Card(...), HWrap(...))` 直接作为 `ui.ScrollView` 的 child，外层
给足宽度。最初卡片仅占约 500 DIP，自然尺寸小于约 900 DIP 的 viewport；
HWrap 在无限宽测量时也不会按 viewport 换行。

依据：`gui/scroll_view.go` 的普通 child 使用双轴无限约束，并按测量出的
contentWidth/contentHeight 排列；只有 `Scrollable` 收到实际 viewport。
`CrossStretch` 只撑开 VBox 内的孩子，不能改变 VBox 自己被安排的宽度。

本例处理：例程内 `flowWidget` 实现已有 Scrollable，约束孩子宽度、保留纵向
自然高度。包内测试检查有限宽/无限高、滚动偏移和语义子树。

建议：先明确 ScrollView 自然尺寸与 viewport 填充的常见用法并提供文档；后续
表单需求反复出现时，再设计简单的纵向滚动/横向填充策略。不要为 Demo 改通用
布局算法，也不靠固定 MinWidth 假装响应式。

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

## 7. 纵向滚动表单内横排 Stretch 的测量/排列差异

分类：已观察到的布局组合一致性问题，责任边界待独立回归确认，本轮未改框架。

复现：本例纵向 Scrollable 的 VBox 中加入一个带 MinHeight(110) 的背景 Bin，
内部是 `HBox(Label(...).MainWeight(1)).Padding(16).CrossAlign(CrossStretch)`。
表单在无限高测量后沿自然高度排列；接收区实际高度被撑大，后面的说明超出
已报告的 ContentSize，滚轮无法把末尾完整移入 viewport。去掉该 HBox 的
CrossStretch，保留水平权重、Padding 和接收区最小高度后，末尾可正常滚到。

代码依据：layout/linear.go 的 compute/childConstraint 给非加权孩子传父级
主轴最大值；stretchCrossExtent 对无限最大值采用自然测量，对有限最大值
消费全部交叉空间。外层 VBox 从无限高测量到有限高 Arrange 时，内层 HBox 的
交叉约束随之改变，重新计算出的高度可能大于第一次测量的自然高度。

本例不需要把文字在纵向铺满，使用默认 CrossCenter 即可。建议后续用嵌套
VBox/HBox 的纯布局用例比较 Measurement、最终子矩形和可滚动 extent，先判断
父级分配和 Stretch 契约，不能仅靠扩大 ContentSize 或新增隐藏重绘补偿修复。

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
