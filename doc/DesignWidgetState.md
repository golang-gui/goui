# Widget 状态与显式样式绘制

## 范围与依据

Widget 的状态由 GUI 内核和控件实现维护，统一从 Widget.State 读取；绘制不读取
Snapshot，不扫描控制器猜测状态。WidgetBase.Paint 保持为空，框架不增加绘制前的
公共装饰步骤。普通 LinearBox/WrapBox 通过显式辅助函数支持自身背景和边框。

参考 GTK4：公开文档 4.24.1 的 StateFlags/get_state_flags/set_state_flags；源码固定
4.20.0。GtkWidget 持有可组合标志，gtkmain 维护指针状态，CheckButton setter 更新
CHECKED，状态变化统一更新样式节点；Button 仍有点击完成判定所需的内部数据。
GOUI 采用统一状态与明确维护者，不照搬 GTK 的 CSS、父状态传播或 ACTIVE 计数。

- https://docs.gtk.org/gtk4/flags.StateFlags.html
- https://docs.gtk.org/gtk4/method.Widget.get_state_flags.html
- https://docs.gtk.org/gtk4/method.Widget.set_state_flags.html
- https://github.com/GNOME/gtk/blob/4.20.0/gtk/gtkmain.c
- https://github.com/GNOME/gtk/blob/4.20.0/gtk/gtkwidget.c
- https://github.com/GNOME/gtk/blob/4.20.0/gtk/gtkcheckbutton.c

## 值模型与状态归属

style.State 改为 uint32 位标志；Normal=0，Has 判断包含全部指定标志。保留原有名称，
增加 ContainsFocus、Selected、Checked、Mixed、DropActive。同一 Widget 可以同时
Hovered/Pressed/Focused/FocusVisible/Checked，不再由控件重复压成互斥枚举。
数值不兼容旧枚举，按名称使用的规则保持有效；不承诺数值持久化格式。

Widget 增加 State() style.State。WidgetBase 存储控件状态与内核维护的 Hovered、
Focused、ContainsFocus；Disabled 从已有有效 Enabled 事实派生，FocusVisible 从
宿主的键盘焦点提示派生。既有 Focused/ContainsFocus/FocusVisible 查询保留，
不另存一套同义 bool。Hovered 表示指针命中自身或子树；精确命中仍由 Motion 控制器
提供。Snapshot 从同一状态读取公共字段，增加 Hovered/Pressed，不拿快照反向控制行为。

WidgetBase.SetState(flags, enabled) 供控件实现设置 Pressed/Selected/Checked/Mixed/
DropActive。Normal 无操作；传入内核保留位或未知位是编程错误，panic，不静默伪造
焦点或启用状态。此方法不加入 Widget 接口，不作为声明层的通用状态 setter。
控件业务值仍通过 SetChecked/SetSelection 等明确 API 变化，状态位只是其展示投影。
每个标志必须有明确维护者；多个输入来源由控件合并，不让不同控制器互相清除状态。

Pressed 由识别点击、拖动或键盘行为的控件维护，释放、取消、禁用、卸载时清除，
不等同于一次 PointerDown 或点击成功。控制器保留手势状态机，控件不再重复保存纯
展示用 hovered/pressed 字段。控件中的滑块命中等 Part 局部状态可以继续独立存在。
语义 Selected/Checked/Mixed 在卸载时保留，重用虚拟行时由 Bind/Unbind 明确同步。

只有实际变化才失效样式、绘制和语义；StyleChanged 复用既有延迟机制，不在
事件更新中直接调用。Root 合并待处理 Widget，在帧回调之后、布局之前通知；若此前
测量有效且宿主尚未请求布局，按原约束比较新测量结果，尺寸/基线变化才请求布局。
这样普通悬停不会在输入分发中让窗口装饰几何失效，也不会让光标移动触发全树布局。
卸载/销毁移除待处理引用；Tab 关闭按钮显隐等行为仍显式请求布局。
TextInput/TextView 保留自身 Focused 边框，绘制时忽略额外 FocusVisible 条件；
菜单和下拉选项的记录禁用仍保留既有指针观察/浏览策略，State 投影 Disabled；
下拉项从所属模型投影 Selected，展开的 DropDown 合并点击/展开来源的 Pressed，
不另存重复的悬停/按下布尔字段。外部焦点/控制器回调可能销毁、卸载或改变状态，继续操作前按
既有生命周期和版本检查。禁用使交互状态失效，启用不恢复旧 Pressed/Hovered。

## 组合状态匹配

不增加 selector 语法、父样式继承或 CSS 系统。规则仍是 Name/Part/State。

1. 先合并同名默认 Part 的 Normal，再合并所请求 Part 的 Normal。
2. 只匹配所请求 Part 的非零状态规则；规则的全部标志都存在时匹配。
3. 状态条件较少的规则先合并；同等条件数按固定优先级合并，后者覆盖字段。
   从低到高为 ContainsFocus、Selected、Checked、Mixed、DropActive、Focused、FocusVisible、
   Hovered、Pressed；含 Disabled 的规则最后，同类再比较条件数/优先级。
4. 同一个状态条件的重复规则保留声明顺序。FocusVisible 查询隐含 Focused。
5. Disabled 查询忽略 Hovered/Pressed/Focused/FocusVisible/ContainsFocus，保留
   Selected/Checked/Mixed/DropActive 等语义信息；Disabled 规则覆盖启用态规则。
6. 指定 Part 不隐式继承默认 Part 的非 Normal 规则，保持现有 Part 隔离契约。

规则在 Sheet 创建时稳定排序，Resolve 不在每次 Paint 动态排序/分配组合子集。
焦点装饰仍用独立 focus Part，避免 Hovered/Pressed 遮蔽键盘提示。原有 -checked、
selected 等 Part 保留兼容，控件同步对应状态位，使应用也能定义组合状态规则。

## 显式绘制辅助

在 gui 提供 PaintStyledBackground、PaintStyledBorder、PaintStyledBox：参数为
Painter、局部 Rectangle 和已解析 Style。分别绘制背景/边框/二者，不保留资源，
不改布局、裁剪或 Transform；背景支持 Radius，描边内缩保持在矩形内。未设置字段、
透明颜色、非正尺寸/边框宽度不发送对应绘制指令。首版不把 Shadow/字体解释为盒子
绘制，阴影继续显式 DrawBoxShadow，并受结构裁剪约束。

提供 WidgetStyle(widget, defaultName, part) 使用自身 StyleName（为空用 defaultName）、
State 和最近 StyleScope。PaintWidgetStyle(p, widget, defaultName) 绘制整个局部边界
的默认 Part，仅由调用者在 Paint 内显式调用。复杂控件可使用 WidgetStyle + 指定
矩形/Part + 分离背景/边框绘制，不增加 Painter 接口或要求调用 WidgetBase.Paint。

LinearBox/WrapBox 默认名称 linear-box/wrap-box，透明无边框；Padding 继续由布局
属性控制，不根据边框/圆角改变布局。圆角背景不等于子树圆角裁剪，子 Widget 继续
矩形结构裁剪。普通背景覆盖包含 Padding 和间距的整个分配区域，不影响命中。
GUI、widgets 和 demo 复用同一辅助函数；自定义控件文档与技能同步说明。
第三方控件通过 basic.Register 提供兜底外观，可选 modern.Register；不新增注册体系。

## 实施与验证

按样式值/匹配 → Widget 状态与回归 → 内置控件/辅助绘制/Box → 窗口例程推进。
包内测试覆盖组合覆盖顺序、Disabled/FocusVisible、Part 隔离、未知位保护、状态归属、
真实事件分发、取消/禁用/卸载/重挂、语义投影及无指令透明绘制。独立 X11 + xfwm4
合成会话验证 Software/OpenGL 1x/2x 的 Box、外部自定义控件、按钮/选择/拖动/Tab、
主题与 StyleScope；真实输入仍经 Window.DispatchEvent，不使用 DevServer。
执行全仓 go test、针对性 race/vet、Windows amd64/macOS arm64 交叉编译。本轮不做
远端桌面验证，交叉编译不算原生验收。结果与剩余缺口记入 Plan，不弱化既有断言。
