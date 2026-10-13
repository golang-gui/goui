# Widget 统一禁用机制

## 公共契约

`Widget.Enabled()` / `SetEnabled(bool)` 保存自身启用设置，默认 true。
父节点禁用不覆盖子节点设置；恢复后，原本显式禁用的子节点仍然禁用。
保留显示、布局、内容、选择和 Focusable 配置，程序 setter 可以继续更新。

```go
type Widget interface {
    Enabled() bool
    SetEnabled(bool)
    // 其余既有接口，包括 StyleChanged
}

// IsEnabled reports whether widget and all its ancestors are enabled.
func IsEnabled(widget Widget) bool
```

IsEnabled(nil) 为 false；不混入可见性、挂载状态、焦点资格或生命周期。
未挂载子树仍按自身与祖先设置查询。调用和修改都在 GUI 线程。
内部缓存祖先限制，但缓存字段、版本和传播算法不属于公共 API。

移除此前的 EffectiveEnabled 与 ConnectEffectiveEnabled，不增加
StateChanged、状态集合或属性系统。MenuItem、Shortcut、DragSource 等
非 Widget 的 Enabled 不改变。

## 参考与采用范围

GTK4 官方文档分别承诺 get_sensitive 返回自身设置，is_sensitive 返回
祖先限制后的结果。4.20.2 的 gtkwidget.c 将实际禁用存为状态标志，并在
状态传播时处理焦点、grab、crossing 和控制器复位。

该版本 GtkDropDown 的 Popover 仍是它的 Widget 子节点，可以接收同一棵树
的禁用传播；GtkMenuButton 则覆盖 state_flags_changed，禁用时主动隐藏
Popover。两者不能合并成“GTK 所有下拉控件都在禁用时立即关闭”的结论。
以上是文档／源码证据，本轮没有执行 GTK 的真实窗口对照。

GOUI 采用子节点设置保留、统一输入取消与样式失效；公开查询采用普通辅助
函数，不照搬 GTK 的状态标志和公共状态信号。GOUI Popover 是独立 Root，
所属控件负责自身跨 Root 的操作资格判断。

- https://docs.gtk.org/gtk4/method.Widget.get_sensitive.html
- https://docs.gtk.org/gtk4/method.Widget.is_sensitive.html
- https://github.com/GNOME/gtk/blob/4.20.2/gtk/gtkwidget.c#L6031-L6080
- https://github.com/GNOME/gtk/blob/4.20.2/gtk/gtkwidget.c#L8058-L8154
- https://github.com/GNOME/gtk/blob/4.20.2/gtk/ui/gtkdropdown.ui
- https://github.com/GNOME/gtk/blob/4.20.2/gtk/gtkmenubutton.c#L298-L309

## 内核传播与生命周期

WidgetBase 保存零值为启用的 disabled、inheritedDisabled 和私有输入版本。
自身设置或者最终父子关系变化后：

1. 提交整棵受影响子树的实际状态，同时标记样式与测量缓存失效。
2. 内核立即取消失效的输入、焦点、悬停、拖放和异步输入资格。
3. 在后续框架测量／绘制前派发合并后的 StyleChanged。

外部回调不得观察半更新状态。一轮重新挂接按最终父节点更新，内部临时
detach 不产生伪启用；Mount 回调读取正确状态。输入版本用于丢弃禁用再
启用后残留的旧操作，不扩展为公开版本或业务属性通知。

保留控制器挂接、顺序和自身设置，通过取消、Leave、Reset 清理交互。
取消子树及宿主级焦点／拖放检查集中执行，变化节点各自复位控制器，合并
布局请求。每次外部回调后检查销毁、重新挂接与输入版本，不依赖清空信号。
Controller.Reset 应清除并通知已存在的 press／drag 状态，不能提交 Click
或拖动结果；已取消的状态不重复通知。

## 事件与宿主

Pick 保持几何查询。禁用区域的点击、双击、右键和拖动起始不穿透到后面的
兄弟或激活祖先按钮。滚轮可以交给原命中路径上仍启用的外层容器。
Focusable 保存配置，实际焦点资格另要求启用和可见；禁用清除焦点，恢复
不自动抢回。

Window 与 Popover 使用共同 EventDispatcher 的指针位置和悬停路径。
启用恢复时，静止指针也会重新命中；离开表面后不能伪造 Hover。
原生光标应用仍属于宿主。Widget 层不特判具体 window。

Widget 快捷键、手势、拖放和 Chrome 查询检查实际启用与回调后的有效期。
窗口级快捷键、独立窗口装饰不受内容禁用影响。DragSource／DropTarget
的自身设置保留，实际资格还要求所属 Widget 启用；原生结束和资源清理
回调继续执行。

## 样式与控件

StyleChanged 表示当前解析样式可能变化：覆盖样式表、StyleName、框架管理的
实际启用状态、首次使用和重新挂接。先标记失效，在框架测量／绘制入口通过
真实 Widget 派发；可合并，最终样式值可能相同。未挂载时不要求注册自身。

钩子只清理／更新本地样式派生资源，不关闭弹层、取消手势、结束 IME 或修改
业务值。框架负责测量失效和调度，不要求调用 Base。Disabled 样式可能改变
字体和几何，不能只 RequestPaint。Hover／Pressed 继续由控件自己的既有
状态更新与缓存失效路径处理，本轮不建立统一状态系统。

使用 style.Disabled，优先于 hover／pressed／focus。Checked／Selected
业务状态保留。Label／Icon 使用自己的 StyleName，不继承父颜色、不统一降低
子树透明度，固定彩色图不强制灰度；兜底和 modern 使用显式中性禁用颜色。

Button／CheckButton 的 Hover 和 Pressed 由控制器取消同步。Switch 拖动通过
Cancel 恢复已提交值的外观，不增加全局动画停止规则。共享 textEditor 通过
焦点退出和输入控制器 Reset 取消 IME、拖动、自动滚动及编辑菜单，使待完成
粘贴失效；StyleChanged 更新排版资源。已提交文本、选区、历史和滚动位置保留。
ReadOnly 仍允许焦点、选择和复制；禁用拒绝这些用户操作。

## DropDown 的独立弹层

禁用后不能新展开，不能通过用户操作提交选择；SetSelected 仍允许更新。
不注册“启用变化后必须立即关闭”的监听。现有失焦、外部点击、Escape、
隐藏、卸载及模型重装规则继续有效，不保证禁用后保持展开。

已展开弹层在输入入口检查所属 DropDown 的当前资格；若不再允许交互，消费
本次普通鼠标／键盘操作并关闭，不能让自定义选项子控件先执行回调。
choose 保留最终提交检查。弹层快照同样反映所属控件限制，清除不可用动作。

这是 DropDown 私有的跨 Root 协调，不扩展通用 Popover、不伪造父子关系，
也不承诺传播两次事件之间所属控件的每一次启用变化。

## UI 与语义

ViewBase[T].Enabled(bool) 复用 fields 位图和初始设置恢复机制，保存自身
Enabled。删除 widgets/ui 重复实现。移除 Enabled 声明恢复构造初值，仍受
父节点限制。

WidgetInfo.Enabled 报告实际启用结果；保留角色、文本、选择、边界和子树，
不报告无法执行的点击、编辑和拖放动作。它是语义状态，区别于自身设置查询。

## 回归与验收

包内测试覆盖默认值、nil、祖先传播、子节点设置保留、最终重新挂接、未挂载
样式初始化、样式失效合并、字体几何、取消回调与 IME Reset 重入／销毁、按下过程中禁用再
启用、焦点／Tab／快捷键、滚轮、不穿透、异步粘贴、IME、拖放，以及 Window
与 Popover 静止指针恢复。DropDown 包内检查自身／祖先禁用后，自定义选项
鼠标和键盘输入被阻止、下一次操作关闭且不提交、语义子树动作正确。

真实窗口程序为 tests/window/ui/enabled，中文入口注释写明步骤和预期。
执行 go test ./...，相关 race、无 DISPLAY 回归和 Windows amd64／
macOS arm64 目标编译；原生桌面、IME 与 HiDPI 结果单独记录，不能用编译替代。

### 接口收敛后的验证

本轮移除公共实际状态查询／监听，保留自身 setter 和 gui.IsEnabled；启用变化
只使样式派生资源失效，行为清理走现有控制器及宿主路径。没有修改 platform、
Painter、布局算法或信号基础设施。

- `go test ./...` 通过；包含缓存结果，既有 opt-in／人工测试的跳过不能计作
  原生场景通过。既有图像测试仍按原行为写源码目录图片，本轮没有迁移它们。
- `go test -race ./gui ./ui ./widgets ./widgets/ui ./theme/modern ./widgets/style`
  通过。
- `env -u DISPLAY go test ./gui ./ui ./widgets -run 'Enabled|DisabledOwner|TextEditorDisable' -count=1`
  通过。新增回归明确断言样式失效合并、Window／Popover 静止悬停恢复、IME
  Reset 同步重新启用后停止旧清理，以及 DropDown 自身／祖先禁用时自定义选项
  鼠标／键盘回调为零、选择不提交、弹层关闭和快照子树动作移除。
- Windows amd64／macOS arm64（CGO_ENABLED=0）的 GUI、widgets 测试二进制
  与 `tests/window/ui/enabled` 程序交叉编译通过；本轮没有远端原生运行。
- Linux amd64、X11／Xfwm4、Software 配置、应用 1x：普通点击和文字输入、
  整组禁用时点击／文字拒绝、已提交文本保留、单项设置及声明移除恢复、F6
  状态／身份／动作断言通过；按钮按住→禁用→恢复→松手不增加计数，随后新
  点击正常。通过 F7 定时禁用后操作 DropDown，选择通知不增加，弹层关闭，
  正常退出输出 EXIT PASS（退出码 0）。最后补充的 IME Reset 重入保护由包内
  回归覆盖，没有将普通文本输入扩大为原生 IME 验收。

窗口例程的文本检查改为比较同步 OnText 回写的当前值，避免输入后立即 F6
时拿上一轮声明文本误报 content panic；状态检查仍比较已完成构建的声明。
F7 在两秒后禁用整组，供展开期间变更的验收；不依赖模态弹层期间不可达的
窗口 F1 快捷键。该次复验没有执行 Windows/macOS 桌面、OpenGL、2x、真实
中文预编辑或混合 DPI，不把旧记录用于补足这些验证缺口。测试产物在仓库外
临时目录，未修改剪贴板或安全策略，测试窗口已退出。

### 2026-10-06 本机与远端复验（接口收敛后）

使用当前工作区重新构建程序及 Windows amd64／macOS arm64 测试目标；远端
传输后逐个核对 SHA-256。此次没有修改生产代码、测试断言或系统安全策略。

- `go test ./...` 再次通过；含缓存结果，既有跳过仍不计入桌面验收。
  `env -u DISPLAY go test ./gui ./ui ./widgets -run 'Enabled|DisabledOwner|TextEditorDisable' -count=1`
  通过。六包 race 已在上一轮对当前实现通过，本次没有重复执行。
- Linux amd64、X11／Xfwm4（XFCE）、1536×889：Software／OpenGL 配置，
  各应用 1x／2x，普通点击、整组禁用后的输入拒绝、亮暗切换、F6 状态／
  身份／动作断言和退出通过。Software 1x／2x 的已提交文本保留通过；
  Software 1x 额外覆盖单项及声明恢复、按住→禁用→恢复→松手不点击；
  Software／OpenGL 1x 覆盖定时禁用已展开 DropDown 后选择通知不增加。
  四次程序退出码均为 0。2x 时屏幕容纳不了全部内容，没有将被遮挡部分
  计作视觉验收。
- macOS 15.7.7 arm64：原生执行 GUI 的 `Enabled|TextEditorDisable`、
  widgets／UI 的 `Enabled|Disabled` 指定测试，三个测试目标均退出码 0。
  包含 IME Reset 回调重新启用时停止旧清理、静止悬停、DropDown 自身／
  祖先禁用后自定义选项鼠标／键盘回调为零等回归。
- macOS VNC 1650×1050、OpenGL 配置、应用 1x／2x：普通点击与编辑、
  整组禁用后点击／文字拒绝、已提交文本保留、亮暗切换和 F6 检查通过。
  1x 额外覆盖 Submit、单项及声明恢复、按住期间禁用再恢复后松手不点击；
  2x 观察指针不移动时单项禁用→恢复的悬停外观。两种缩放均先确认弹层
  已展开，再等待 F7 禁用，后续 Enter 没有提交选择。原生关闭均输出
  DESTROY PASS、EXIT PASS，退出码 0。
- Windows 10 Pro Workstation 19045 amd64、RDP 1600×900：传输与哈希通过，
  但 `gui.windows.test.exe` 和 `Start-Process enabled.windows.exe` 均在执行
  程序前返回拒绝访问。只读检查确认当前用户文件 ACL 允许执行，Defender
  Operational 事件 1121 对应这两个文件，规则
  `C1DB55AB-C21A-4637-BB3F-A12568109D35`。这是安全策略阻止启动，不是
  测试断言失败；widgets／UI 测试及 Direct2D 桌面验收未执行。没有关闭
  Defender、添加排除、移动文件规避拦截或修改防护设置。

OpenGL 配置可由平台工厂降级为 Software，本轮没有独立检查最终后端类型，
因此不据此承诺硬件 GPU 覆盖。1x／2x 使用 GOUI_PLAT_SCALE 覆盖应用缩放，
不等同真实 Retina／混合 DPI。真实中文预编辑、剪贴板复制与原生 DnD 的完整
操作，以及未覆盖的后端组合仍未验收；普通文字输入不能替代这些场景。

截图和本机二进制保留在 `/tmp/goui-enabled-validation.SeEd2Q`，macOS 日志与
退出记录位于 `/tmp/goui-enabled-validation.ke7wIu`；Windows 待允许运行的
文件位于 `C:\Users\xuyang8\AppData\Local\Temp\goui-enabled-validation-SeEd2Q`。
已退出本轮测试窗口、停止临时 HTTP 传输服务、释放按键并断开本轮开启的
VNC／RDP；原有 SSH 连接与用户程序保留。

下面属于此前带公共实际状态信号的实现，其结果仅保留为历史记录。

### 2026-10-06 首次执行记录（接口收敛前）

本轮只修改 GUI、UI、widgets、显式兜底/Modern 样式、相关断言及窗口例程；
没有增加平台接口，也没有改变 Painter 或布局算法。旧 CheckButton/Switch
的 Child 独立交互约定被本公共契约取代，对应文档已同步。

- `go test ./...`：49 个有测试包，2021 项测试/子测试通过，14 项既有
  人工/opt-in 测试跳过，无失败。既有图像测试仍按原行为写源码目录图片，
  既有平台测试有短暂原生窗口副作用；本轮新增测试产物全部在仓库外临时目录。
- `go test -race ./gui ./ui ./widgets ./widgets/ui ./theme/modern ./widgets/style`
  通过；移除 DISPLAY 后的 GUI/UI Enabled 与 TextEditorDisable 指定回归通过。
- 默认 `go vet` 仍报告既有 GUI/UI 文本选择别名的非具名结构体字面量警告；
  新增字面量使用具名字段。`go vet -composites=false` 对上述六包通过，不把
  此结果记作默认 vet 全部通过，也不在本切片重写旧测试。
- Windows amd64 / macOS arm64 的 GUI/UI 测试目标和 Enabled 窗口程序交叉
  编译通过（CGO_ENABLED=0）。macOS 15.7.7 arm64 原生执行最新 GUI 测试
  二进制的 `Enabled|TextEditorDisable` 回归通过，本地/远端 SHA-256 一致。
- Linux amd64、X11/Xfwm4（XFCE）、应用 1x，Software/OpenGL 配置：亮暗
  显示、父节点限制、单项禁用与声明移除、输入拒绝、已提交文本保留、F6
  状态/身份断言和退出通过。最终 Software 例程额外验证按住按钮→禁用→
  恢复→松手不点击，之后新点击正常。OpenGL 配置未独立探测工厂 fallback
  后的最终类型，不据此承诺硬件 GPU 覆盖。
- macOS 15.7.7、VNC 1650×1050、OpenGL 配置、应用 1x：正常点击与编辑/
  Submit、整组禁用后点击/文字输入无效、单项设置恢复、子图标弱化、移除
  声明、亮暗切换、DropDown 打开与关闭、重复 F6 断言通过；退出码 0。
  最后增加的回调重入防护由最新原生包内回归覆盖，没有把已验证的普通输入
  扩大为完整 IME 或所有控制器组合验收。
- Windows 10 19045 amd64、RDP 1600×900：测试二进制传输哈希一致，但在
  当前安全策略下启动被拒绝访问（包装退出码 5，桌面安全通知），没有完成
  Direct2D 桌面验收。没有关闭 Defender、增加排除或修改安全策略。

回归测试还覆盖两个重入场景：悬停回调禁用后不再向后续控制器发送旧 Enter
（补齐路径检查前已复现失败），以及编辑选区归一化回调禁用/重新启用后
本次 Commit 停止。后者使用显式组合 cluster 的确定性排版替身，不假定
逐 rune 的测试替身会自动产生真实字体 shaping 的 cluster。

例程检查针对最新已构建声明的预期，排队读取当前检查闭包，避免快速连续
按键拿待协调状态与上一轮 Widget 树比较；子图标同时检查按钮与组的限制。
退出正常返回输出 EXIT PASS；App.Quit 不承诺再发声明式 OnDestroy。

工具/清理：Windows SSH 的实际入口需显式调用 PowerShell，连接元数据的
shell 值不能直接当作可执行语法；大文件 stdin 文本读取未可靠完成，任务
限定的读取进程已停止，改用临时 HTTP 传输并校验哈希。HTTP 服务与本轮
Windows 交互任务已移除，VNC/RDP 已断开，原有 SSH 与用户程序保留。

待验收：Windows 原生桌面；三端真实 IME、实际 Retina/混合 DPI、2x 和
未覆盖后端组合、原生 DnD 的完整操作场景。包内模拟和编译不替代这些验收。
