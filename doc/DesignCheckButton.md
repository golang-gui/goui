# CheckButton

## 目标与边界

在 `widgets` 提供一个 CheckButton，通过指示器／按钮外观和显式互斥组支持
checkbox、radio 和 toggle button。Switch 已作为独立控件实现，见 [Switch](DesignSwitch.md)。
不改 gui.Button、公共 Painter、平台输入接口或全局 style.State。

参考 GTK 4.20.3 [CheckButton 源码](https://github.com/GNOME/gtk/blob/4.20.3/gtk/gtkcheckbutton.c)、
[分组接口](https://docs.gtk.org/gtk4/method.CheckButton.set_group.html) 和
[Qt 6 QButtonGroup](https://doc.qt.io/qt-6/qbuttongroup.html)：组不参与布局，选中
成员再次点击不会取消；初始组可以为空。GTK 的 inconsistent 只控制外观；GOUI
采用显式 CheckState 表达部分选中，不照搬其 active/inconsistent 双属性。

## API 与行为

```go
b := widgets.NewCheckButton()
b.SetChild(gui.NewLabel("记住登录"))
b.SetCheckState(widgets.CheckMixed)
b.ConnectChange(func(state widgets.CheckState) { /* 用户操作 */ })

group := widgets.NewCheckGroup()
a.SetGroup(group)
b.SetGroup(group)
b.SetAppearance(widgets.CheckAppearanceButton)
```

- CheckUnchecked、CheckChecked、CheckMixed；默认 Unchecked。独立控件点击在
  Unchecked/Checked 之间切换，Mixed 下一次激活变 Checked，不自动循环三态。
- 有 Group 的指示器是圆形，无 Group 为圆角方形。按钮外观只改变绘制与布局，
  不改变选中语义。组最多一个选中，SetCheckState(Unchecked) 可主动清空。
- SetGroup、SetCheckState、SetChecked 都静默。加入组的 Checked 成员替换原
  选中；Mixed 在组中归一为 Unchecked；非法枚举分别归一为 Unchecked/Indicator。
- 用户选择先同时提交旧成员 Unchecked、新成员 Checked，然后按旧、新顺序发
  Change。回调可以销毁或重新设置状态，后续过期通知通过修订号和存活检查跳过。
- 组仅弱引用当前选中者，不维护成员列表，不拥有 Widget；普通卸载／移交不
  改变分组。销毁或回收选中者后 Checked() 返回 nil，不自动替选。
- 复用 ClickEventController 的正常手势竞争；有效抬起才切换，越界／取消不
  切换，交互 Child 获胜时不切换父控件。Space/Enter 激活，Repeat 消费但不切换。
  Tab 沿用 GUI 树顺序；本轮不增加组内方向键、组的特殊 Tab 策略。
- Enabled 复用 Widget 公共机制：自身设置与 gui.IsEnabled 查询分开，禁用限制
  整棵 Child 子树，不改写 Child 的自身设置。Focusable 保留原配置，焦点资格
  另检查实际启用状态；恢复不自动取得焦点。见 [统一禁用契约](DesignWidgetEnabled.md)。

## 布局、样式与快照

单 Child；默认 padding 6 DIP（与 gui.Button 一致）、indicator 18 DIP、间隔 8 DIP。Child 水平靠左、
垂直居中；按钮外观双轴居中。测量和安排采用相同内容约束及 baseline 偏移。
WidgetBase 保留 MinSize/MaxSize、父硬约束及隐藏行为。子控件独立解析样式。

`check-button` Parts：`indicator`、`indicator-checked`、`indicator-mixed`、
`radio`、`radio-checked`、`button`、`button-checked`、`button-mixed`、`focus`。Hovered/Pressed/Disabled
仍为瞬态样式；FocusVisible 用独立 2 DIP 边框，不替换选中填充。Modern 的
按钮式未选中使用普通 Button 的底色、中性边框和圆角；选中采用中性灰阶底色，
普通文字保持可读。Modern 勾选框使用主题色实底与白色标记，单选框选中采用
主题色实心圆底和白色圆点、未选中为中性空心圆；两种指示器分别解析
indicator／radio 部件，允许独立自定义。
完整兜底由 theme/basic 提供，GUI 自动使用；Modern 提供完整自身规则。

### 当前外观（2026-10-06，按钮式选中改为中性灰阶）

参考 GTK 4.20.3 [默认主题按钮及指示器规则](https://github.com/GNOME/gtk/blob/4.20.3/gtk/theme/Default/_common.scss)
和 Avalonia 11.3.0 Fluent [ToggleButton](https://github.com/AvaloniaUI/Avalonia/blob/11.3.0/src/Avalonia.Themes.Fluent/Controls/ToggleButton.xaml)
／[Button](https://github.com/AvaloniaUI/Avalonia/blob/11.3.0/src/Avalonia.Themes.Fluent/Controls/Button.xaml)：
采用普通／切换按钮共用尺寸、圆角及反馈语言的思路，保留独立选择状态。
不照搬样式继承、模板系统或动画，配色由 GOUI Modern 的现有调色板决定。

- Modern 按钮外观使用 6 DIP 圆角、1 DIP 中性边框；未选中 Normal／Hover／
  Press／Disabled 的底色与普通 Button 相同。Checked／Mixed 使用普通按钮
  底色向正文色混合 12%／16%／20% 的中性灰阶；亮色主题加深、深色主题提亮，
  不随主题色染色。Normal 选中底分别为 #E4E4E5／#48484E。最小尺寸
  采用自身样式字号换算的一行高度，避免固定 20 DIP 下限使小字号按钮偏高。
- 指示器由 16 增至 18 DIP。方框使用 4 DIP 圆角、1 DIP 边框；未选中轮廓
  相对普通按钮适当增强。Checked／Mixed 使用实色主题底和白色勾／横杠；
  将过亮底色向黑色调整至白色标记至少 4.5:1 对比度。Hover／Press 分别
  轻微加深 6%／12%，轮廓与填充同色，避免轮廓叠加。
- radio 使用独立样式部件：2026-10-13 按最终确认方案，Modern 选中改为
  18 DIP 主题色实心圆底和 8 DIP 白色圆点，零描边以避免同色描边重复覆盖边缘；
  未选中保留 1 DIP 中性轮廓。实底复用 CheckBox 的选中配色及 6%／12%
  悬停／按下色阶，白色圆点与实底至少 4.5:1。组中没有 Mixed，因此不提供
  radio-mixed 部件。控件本身仍按各部件样式绘制，不强制此外观。
- Hover／Press 不改变轮廓宽度；按钮的中性轮廓保持稳定。Disabled 使用中性
  配色；Modern Radio 禁用选中为中性实底与灰色圆点，填充与 CheckBox 相同。
  选中状态与 FocusVisible 独立，只有键盘提示显示既有 2 DIP 柔和
  边框，鼠标焦点没有额外装饰。
- Basic 兜底采用中性按钮、独立指示器与焦点部件；当前规则位于
  `theme/basic/check_button.go`，不动态查询父样式。Checked／Mixed 方框与
  选中 Radio 为零描边实底，避免覆盖率重复合成；未选中保留独立轮廓。
- Padding 仍是控件自身属性，默认统一为 6 DIP；调用方可显式覆盖。Child
  独立解析 check-button-text 等自身样式，不继承父控件颜色或字体；实际禁用
  状态受祖先限制，用自己的 Disabled 样式绘制。
- 窗口例程将 app.Settings 的 FontFamily／FontSize 显式传入 Modern，以系统
  界面字体观察真实比例；显式兜底仍保留其固定字体。Settings 更新沿用 UI
  已有协调机制，不增加控件内监听。

当前代码归属为 widgets、widgets/ui、theme/basic、theme/modern
及窗口例程和本文档；不修改普通 Button、选中状态机、公共样式 API 或绘制后端。
回归覆盖未选中与普通按钮一致、按钮中性选中底色、两种指示器的实底与白色标记、
18 DIP 布局／baseline 及小字号按钮测量；白色勾／横杠／圆点的 4.5:1
对比度断言覆盖 Normal／Hover／Press，Radio 另验证未选中中性背景与零选中描边。
旧自定义像素用例仍在紧约束下使用 16 DIP 绘制区域，保留预乘合成、约束和
自定义 Radio 空心圆环的覆盖，不把 Modern 的新预设强加给自定义样式。

角色在 widgets 定义为 checkbox/radiobutton/togglebutton；Attributes["goui.check"]
包含 State、Appearance、Grouped 的值快照；保留 Child 子树，启用时提供 Click，
焦点能力沿用 WidgetBase，不在 gui.WidgetInfo 添加控件专用字段。

## 声明层

```go
wui.CheckButton("记住登录").Checked(remember).OnChange(func(s wui.CheckState) {
    remember = s == wui.CheckChecked
    app.RequestUpdate()
})
```

通过 Child/Content/Text、Group、Appearance、CheckState、Checked、Enabled、
Padding 与 OnChange 链式配置。Group 在重建函数外创建；声明重建静默同步，
保留 Widget 身份与单个 Change 连接，回调替换为最新闭包，卸载时断开连接。
内容使用已有 ui.Bin 协调。Text 的 Label 显式使用 check-button-text，不继承父样式。

## 实施与验证

代码涉及 widgets/check_button.go、widgets/ui、theme/basic、theme/modern、
widgets/info.go，以及包内测试和 tests/window/widgets/check_button 独立窗口例程。
不修改 GUI 内核或平台。验证参数／三态、组原子性与重入、正常输入派发／取消／
子控件竞争、布局 baseline／硬约束、快照、亮暗主题及像素、声明协调与连接清理。
完成后执行全仓测试、相关 race/vet、Windows amd64/macOS arm64 编译，本机
Software/OpenGL 1x/2x 桌面操作；编译不替代两端原生桌面验收。

### 2026-10-06 验证记录

- 通过：不依赖 DISPLAY 的新逻辑／像素／UI／样式测试；实际 EventDispatcher
  路径覆盖有效抬起、越界、失焦与中途禁用取消、子控件手势竞争、键盘 Repeat；
  互斥原子性、回调重入过期通知、卸载／重新挂载保持组、baseline、硬约束、
  JSON 快照与 1x/2x Software 预乘像素。Modern 覆盖亮暗、黑／白／红／黄主题。
- 通过：`go test ./...`，49 个有测试包、1936 个测试／子测试通过、14 个既有
  人工／opt-in 跳过；`go test -race ./widgets ./widgets/ui ./widgets/style ./theme/modern`，
  对以上包及窗口程序的 `go vet`。既有人工／opt-in 测试仍按原有条件跳过，
  本轮不删除或扩大 skip；无新增失败。
- 通过：Windows amd64、macOS arm64，CGO_ENABLED=0 的 `go build ./...`，
  widgets、widgets/ui、theme/modern 的 `go test -c`。仅编译，未执行两端二进制。
- 通过：隔离 Xvnc/X11 + Xfwm4，Software 和 OpenGL（Mesa 23.2.1 llvmpipe）
  的 1x/2x，另外 Software 1x 兜底；每组 26 次明确状态断言及正常销毁。
  检查指示器／文字激活、抬起取消、Mixed、互斥与重复选中、按钮式切换、
  子按钮竞争、禁用、四次重建、亮暗／红色主题、清空组、Tab/Shift+Tab、
  长按 Space 与 Enter；通过正常原生输入，不使用 DevServer。
- 通过：Software 1x、OpenGL 2x 的 `-destroy-on-change`。旧选中成员的 UI
  回调直接销毁窗口；额外原始 GUI 连接故意不由 UI 卸载断开，仍不得被调用。
  断言只发生一个 Change、两个组无选中者、进程正常退出，验证不是依靠
  DisconnectAll 或 UI teardown 偶然兜底。
- 环境限制：隔离桌面无合成器，Integrated 请求回退 Native；本轮不是窗口
  装饰验收。尚未执行 Windows Direct2D/macOS OpenGL 原生桌面、硬件 OpenGL、
  混合 DPI 或其它 WM；组内方向键／Switch 不在本轮范围。

测试日志、独立输入脚本与截图在仓库外 `/tmp/goui-check-button.wBd29b`，未把
截图／可执行文件作为源码提交；入口操作说明见
[窗口例程](../tests/window/widgets/check_button/main.go)。本轮未提交 git。

### 外观统一复验（2026-10-06）

- 通过：`go test ./...`，49 个有测试包、1950 个测试／子测试通过，14 个既有
  人工／opt-in 跳过；相关 widgets、widgets/ui、widgets/style、theme/modern
  的 race、vet，以及无 DISPLAY 的 CheckButton 回归通过。
- 通过：亮暗及蓝／红／黑／白／黄主题的样式断言；默认蓝色底的独立 sRGB
  预期；1x/2x Software 实际绘制的柔和填充、彩色标记、圆形几何、透明 Padding
  和按钮中性边框。保留并扩展标记对比度覆盖。
- 通过：隔离 Xvnc/X11 + Xfwm4，Software／OpenGL（Mesa 23.2.1 llvmpipe）
  各 1x/2x，另有 Software 1x 显式兜底。每组 28 次状态断言及正常销毁；
  新增普通 Button 与 Toggle 对照，核对实际分配大小、Normal／Hover／Press
  填充、稳定轮廓、选中浅底、彩色指示器和深色红主题。截图按内部不透明
  sRGB 像素采样，填充仅允许 1 级量化误差；键盘焦点边框为 2/4 物理像素。
  兜底 Button 的边框为零，采样明确按填充验证。OpenGL 2x 回调销毁复验通过。
- 通过：Windows amd64／macOS arm64 的 `CGO_ENABLED=0 go build ./...`，
  widgets、widgets/ui、widgets/style、theme/modern 测试目标交叉编译。
- 额外检查失败：`GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go build ./...`，
  `platform/linux/libs/libc/syscall_linux.go:29,61` 引用该架构不存在的
  `syscall.SYS_PIPE`／`syscall.SYS_POLL`。对应绑定与 HEAD 一致，本轮没有修改；
  该平台绑定问题另行处理，不将此目标计为编译通过。
- 未执行：Windows／macOS 原生桌面、硬件 GPU、混合 DPI；隔离桌面无合成器，
  Integrated 请求回退 Native，仍不视为窗口装饰验收。

本次复验日志、脚本和截图位于仓库外 `/tmp/goui-check-style.dapu8N`。
已同步窗口例程操作说明；未提交 git。

### 第二轮视觉结构复验（2026-10-06）

- 通过：`go test ./...`，49 个有测试包、1951 个测试／子测试通过，14 个既有
  人工／opt-in 跳过；相关 widgets、widgets/style、theme/modern 和窗口程序 vet。
  旧样式断言按本轮明确改变的视觉契约更新，保留选中标记对比度、独立文字、
  默认值、预乘像素及真实事件路径覆盖；增加 10／14／22 pt 与普通按钮的
  测量对照，覆盖系统字号下固定最小高度导致的差异。
- 通过：亮暗、默认蓝／红／黑／白／黄主题的选中状态与对比度；1x/2x
  Software 像素确认白色横杠、实色勾选背景、radio 中性间隙和彩色圆点、
  按钮更明确的选中浅底。白色主题与白色按钮底重合时仍能区分选中状态。
- 通过：隔离 Xvnc/X11 + Xfwm4，Software／OpenGL（Mesa llvmpipe）各 1x/2x，
  另有 Software 1x 显式兜底；每组 26 次交互状态断言及正常销毁。检查亮暗
  蓝／红主题、点击与取消、三态、互斥、子控件竞争、禁用、重建、Tab 和键盘。
  实际 Settings 为 Sans 10 pt；普通 Button 与 Toggle 的分配大小一致。
- 通过：Windows amd64／macOS arm64，`CGO_ENABLED=0 go build ./...`。
  本轮不涉及并发与资源管理变更，未重复执行第一轮 race 或回调销毁专项。
- 未执行：Windows／macOS 原生桌面、硬件 GPU、混合 DPI。隔离桌面仍为
  无合成器，Integrated 回退 Native；上一轮记录的 Linux arm64 绑定阻塞未改动。

日志与亮暗截图位于 `/tmp/goui-check-solid.2XJ4YU`。窗口例程已更新中文说明，
测试桌面已关闭，不影响用户桌面；本轮未提交 git。

### 按钮式中性配色复验（2026-10-06）

- 仅调整 Modern 与显式兜底的按钮 Checked／Mixed 底色，勾选框、单选框、
  边框／圆角、布局、状态机与键盘焦点均保持不变。更新已有样式和像素断言，
  覆盖亮暗主题下 Normal／Hover／Press 的确定颜色及主题色无关性。
- 通过：`go test ./...`，49 个有测试包、1951 个测试／子测试通过，14 个既有
  人工／opt-in 跳过；CheckButton 针对性回归通过。
- 通过：本机隔离 Xvnc/X11 + Xfwm4，Software 1x、OpenGL 2x（Mesa llvmpipe），
  各 26 次交互状态断言。截图检查亮暗外观；不透明填充内部采样确认选中、
  悬停／按下的六种 sRGB 颜色，亮色下蓝／红主题切换不改变选中底色，
  每通道只允许 1 级量化误差。正常关闭测试程序。
- 未执行：本次不涉及平台或资源契约，未重复远端原生桌面、交叉编译、race。
  无合成器测试桌面中 Integrated 回退 Native，不作为窗口装饰或硬件 GPU 验收。

日志、脚本和亮暗截图位于 `/tmp/goui-check-neutral.8HAIU3`；未提交 git。

### Modern 单选与勾选配色统一（2026-10-13）

本节记录仅统一 RGB 的中间阶段；用户体验后确认改为下节的实底圆形指示器。

暗色 Radio 明显浅于 CheckBox 的原因是不同的配色约束：原规则将 Radio 朝正文色
调整，直到与普通按钮三个中性背景均达到 4.5:1；CheckBox 实底只为白色勾选
标记作必要的加深。以系统蓝 #0078D4 为例，暗色 Radio 原色为 #78B3E1，
CheckBox 为 #0078D4。这是样式生成行为，不是后端透明度或抗锯齿错误。

按用户确认，仅在 Modern 共用 CheckBox 的三态选中颜色：圆环／圆点分别对应
CheckBox Normal／Hover／Press 的实底。系统蓝为 #0078D4／#0071C7／#006ABB，
亮暗模式一致；不改圆形几何、中性底色、禁用、文字、焦点或按钮式外观。
不修改 widgets/style 的显式兜底、GUI／平台／Painter，也不新增公共 API。

此选择撤回 Radio 圆点的文字式 4.5:1 目标，不承诺与任意中性背景达到固定对比度，
尤其是黑色等极端主题色。CheckBox 白色勾／横杠的 4.5:1 及独立内容文字的
原有对比测试保留。用户需要不同指示器配色时，可覆盖现有 radio-checked 部件。

新增回归使用独立的不透明 sRGB 字节预期，覆盖亮暗、默认／系统蓝／紫蓝／红／
白／黄／黑主题及三态圆环／圆点、CheckBox Checked／Mixed 实底；同时验证
中性背景、2 DIP 轮廓、9 DIP 圆角参数和禁用配色未变。新断言在旧实现复现
暗色系统蓝的 `{120 179 225 255}`，预期 `{0 120 212 255}`；修正后通过。
旧测试只将已撤回的 Radio 4.5:1 约束替换为配色一致性，不减少 CheckBox 覆盖。

本轮验证（本机 Linux amd64；样式逻辑不涉及绘制后端／缩放）：

- `go test -count=1 ./theme/modern ./widgets ./widgets/ui ./example/demo` 通过。
- `go test -json -count=1 ./...`：2373 个测试／子测试通过，15 个既有条件跳过，
  零失败。跳过仍为 14 个平台手动／opt-in 场景及非 macOS 的 TestMacOSSigning，
  未扩大 skip；完整日志在 `/tmp/goui-radio-accent.ufeFLE/all-tests.json`。
  全仓含既有原生窗口及图片输出副作用，本轮没有新增此类副作用。
- `go test -race -count=1 ./theme/modern ./widgets ./widgets/ui ./example/demo`、
  `go vet ./theme/modern ./tests/window/widgets/check_button` 通过。
- Windows amd64／macOS arm64 的 `CGO_ENABLED=0 go build ./...` 及
  Linux arm64 的 `CGO_ENABLED=0 go build ./theme/modern` 通过，仅为编译验证。
  已记录的 Linux arm64 示例 libc 问题未修改，不重复验证该入口。
- `git diff --exit-code -- style gui widgets platform` 和本轮文件的 diff 检查通过；
  只有 Modern 配色、相关测试／文档及窗口程序操作说明改动。
- 原生桌面及远端外观未执行；窗口例程提供亮暗及蓝／红主题对照，留给用户
  体验。不将编译或颜色数值断言等同于真实外观验收，本轮不提交。

### Modern 实底单选指示器（2026-10-13）

用户截图中，CheckBox 实底与 Radio 圆点采样均为不透明 #3078C4；但圆环／圆点
之间的大面积中性间隙让单选视觉仍弱于勾选。这说明同色不等于选中状态视觉一致，
上一阶段的数值断言不等同于外观验收。按确认方案，仅改变 Modern radio-checked：
主题色实心圆底、白色圆点、零描边；未选中为原有中性空心圆。

源码参考：GTK 4.20.3 Default 主题的
[_common.scss:2454–2519](https://github.com/GNOME/gtk/blob/4.20.3/gtk/theme/Default/_common.scss#L2454)
让 check／radio 共用选中背景与前景，形状和标记分别区分；
[_drawing.scss:415–447](https://github.com/GNOME/gtk/blob/4.20.3/gtk/theme/Default/_drawing.scss#L415)
实际设置选中背景与标记颜色。这是该主题的源码行为，不是所有平台的系统承诺。
GOUI 只采用选中实底／对比标记这一视觉分工，不复制渐变、阴影、SVG 标记资源或 CSS。

复用现有 BackgroundColor／ForegroundColor／BorderWidth，圆底 18 DIP、圆点
8 DIP 的几何由现有控件 Paint 完成，不改 Widget、Painter 或公共 API。填充沿用
CheckBox 的三态色阶，禁用选中为两种模式的 #D2D2D3／#57575C 中性底与禁用
文字色圆点；未选中、普通按钮、焦点及显式兜底不变。

回归按明确的新外观契约更新：独立 RGB 验证圆底、白点、零描边及禁用实底，
保留七种主题色、亮暗、三态、CheckBox Checked／Mixed 的既有字节预期；新增
未选中三态的 1 DIP 中性轮廓检查。由于圆点现为白色标记，恢复两种指示器
Normal／Hover／Press 的 4.5:1 对比断言。同步 widgets 包中已有 Modern Software
像素测试的选中预期：1x／2x 精确断言白色圆心、实色圆底和透明角／Padding；
不修改控件实现或自定义样式像素回归。采样远离抗锯齿边缘，不放宽颜色容差。
新断言先在旧规则复现：暗色系统蓝背景为 #323238、圆点为 #0078D4，预期为
#0078D4 背景与白色圆点；修改后通过。

本轮验证（本机 Linux amd64）：

- Modern 样式回归通过，七种主题色／亮暗／三态的实底、白点、未选中轮廓及
  禁用配色有独立字节断言；CheckBox 与独立内容的对比回归保持。
- 已有 TestCheckButtonModernSoftwarePixels 初次仍断言旧的彩色圆点／中性间隙而
  失败；按新契约同步预期后，
  `go test -count=1 -run '^TestCheckButton(SoftwarePixels|ModernSoftwarePixels)$' ./widgets`
  通过。覆盖 Software 离屏 1x／2x、亮暗、白点／蓝底／圆角外透明／Padding，
  独立精确 sRGB 字节断言，无新增容差；自定义半透明绘制回归保持通过。
- `go test -race -count=1 ./theme/modern ./widgets ./widgets/ui ./example/demo` 和
  `go vet ./theme/modern ./widgets ./tests/window/widgets/check_button` 通过。
- `go test -json -count=1 ./...` 最终复跑有 2373 测试／子测试通过、15 既有条件
  跳过，但整体失败：工作期间新增的未跟踪 `tests/temp/main.go:11` 无参数调用
  `NewPopup`，缺少 Window／Size／EventHandler／PopupOptions，无法编译。
  Windows amd64／macOS arm64 的 `CGO_ENABLED=0 go build ./...` 同样被该例程
  阻塞。未修改无关临时例程，不将全仓验证记为通过；前后日志在
  `/tmp/goui-radio-solid.cgQgUN/all-tests.json`／`all-tests-final.json`。
- Windows amd64／macOS arm64 的 `CGO_ENABLED=0 go build` 针对
  `./theme/modern ./widgets/... ./example/demo ./tests/window/widgets/check_button`
  通过；Linux arm64 的 Modern 包编译通过。仅代表编译，不替代原生运行。
- style／gui／platform、widgets 控件实现／显式兜底／UI 无差异；widgets 唯一
  变动是上述已有像素测试的明确预期更新。相关文件 diff 检查通过。
- 未执行桌面或远端外观验收；最终观感由用户运行 demo／窗口例程体验，本轮不提交。
