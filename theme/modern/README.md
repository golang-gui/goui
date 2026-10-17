# Modern

显式启用、跨平台一致的样式值生成器。不持有应用对象、不查询原生设置、
不注册监听；`FromSettings` 便利入口通过 GUI 的设置类型接收已解析的值。
GUI／UI 不依赖 Modern，Basic 是默认兜底。

在 `ui.Run` 的 build 中读取 Settings：

```go
return ui.Root().StyleSheet(modern.FromSettings(app.Settings())).Windows(/* windows */)
```

UI 已自动响应 Settings 变化；相同规则不会重复下发到 GUI。
固定模式、主题色或应用追加规则仍使用 `modern.Sheet(Options)`／`modern.Rules(Options)`。
GUI 命令式应用使用 `app.SetStyleSheet(modern.FromSettings(app.Settings()))`，需要
跟随系统时自行连接 Settings.ConnectChanged 并管理连接。FromSettings 只生成
当次快照，不会让返回的样式表自行变化。设回 nil 恢复完整 Basic，无需拼接 widgets/style。

Modern 采用中性、克制、边界清晰的桌面外观。独立操作控件使用细描边，内容行
主要用背景表达交互和选中，不要求所有控件都套外框。主题色用于 Primary、状态
指示和焦点，阴影主要用于浮动表面；完整规则见
[视觉原则与描边契约](../../doc/DesignModernPresets.md#modern-视觉原则与描边契约)。

`modern.Flat` 是常态收起表面的 Button／MenuButton：常态、禁用透明无框，
悬停／按下的背景、1 DIP 描边和 6 DIP 圆角与普通按钮一致，不改变测量或布局。
应用可追加 `style.Name(modern.Flat).State(style.Hovered).BorderWidth(0)` 关闭悬停
描边；按下需要单独覆盖 `style.Pressed`，不将状态规则隐式传播到其他状态。

`modern.Primary` 是主要操作的主题色实底，默认不叠加普通描边；
子 Label 和单色 Icon 显式使用 `modern.PrimaryContent`，不依赖父子样式继承：

```go
ui.Button().Content(ui.Label("保存").Style(modern.PrimaryContent)).Style(modern.Primary)
```

优先使用白色前景；很浅的主题色改用黑色前景，实底必要时微调明度，使正常文字
与三种交互背景保持至少 4.5:1 对比。悬停／按下分别改变明暗 6%／12%，黑色向亮
调整以保留反馈。禁用使用中性底色和弱化前景，同样不叠加普通描边。
圆角仍为 6 DIP，状态不改变布局；应用仍可显式设置普通描边。
应用可独立覆盖 `Primary` 表面及 `PrimaryContent` 的前景／字体。
`modern.MutedText` 是次要文字。
`ui.Icon(source)` 向 Source.Image 提供正文前景，`.Style(modern.AccentIcon)` 使用经过对比度
调整的主题色；`.Size(20)` 设置期望 DIP 边长。不依赖 SVG 解析或图标名称注册。
icon.Image 和 SVG 默认跟随前景；`original := source.WithRawColors()` 保留
资源原色，使用 `ui.Icon(original)`。Source 决定如何处理颜色，Icon 不再二次着色。
SVG 可通过独立 `icon/svg.Parse(reader)` 提前加载，不要在 UI build 内解析或反复创建配置。
样式不在父子间隐式继承，未设置 `PrimaryContent` 的普通 Label/Icon 仍保持正文色，
不会自动改成白色。Button／MenuButton 保留独立的
`focus` part 绘制入口。鼠标聚焦的 `Focused` 默认透明背景、零边框；Tab / Shift+Tab
聚焦使用 `style.FocusVisible` 的 2 DIP 内绘圆角边框。普通 Button／Flat 的颜色
仍为 75% 主题焦点色与 25% 中性边框色；仅 Primary 的键盘焦点按亮暗模式使用
固定灰色：亮色 `#414147`，暗色 `#D8D8DE`，不随主题色或内容前景改变。
不承诺这两个固定颜色与任意 Primary 实底均达到 3:1。保留既有 2 DIP
贴边绘制，不增加额外内缩或样式字段。
普通按钮、Flat、TextInput 的默认几何和配色不变。
不改变 Hover/Pressed 背景、布局或子控件样式。
应用可追加 `style.Name("button").Part("focus").State(style.FocusVisible).BorderWidth(3)`
调整键盘边框，设为 0 可关闭；颜色可用 `.BorderColor(...)` 覆盖。
FocusVisible 仍继承未覆盖的 Focused 字段，但其默认颜色和宽度优先于普通 Focused
规则；要改变鼠标聚焦外观则使用 `.State(style.Focused)`。Primary 对应 `modern.Primary`。
文本输入框保留原有焦点提示。GUI 自动管理提示模式，应用不需要另行处理输入。

`widgets.CheckButton` 的按钮式外观与普通 Button 使用相同的中性底色、边框和
6 DIP 圆角；选中时使用中性灰底（亮色加深、深色提亮），不随主题色染色。
18 DIP 方框采用 4 DIP 圆角，
选中为主题色实底与白色勾／横杠；18 DIP 单选框选中为主题色实心圆底和
8 DIP 白色圆点，不叠加描边。两者共用选中主题色，悬停／按下均加深 6%／12%，
白色标记与实底保持至少 4.5:1。未选中 Radio 保留 1 DIP 中性圆环；禁用选中
使用中性实底和灰色标记，不改变圆形尺寸。
两种指示器分别使用 indicator／radio 部件。悬停和按下轻微改变明暗，
键盘焦点仍通过独立的 `focus` part 显示。文字使用自身的 `check-button-text`
样式。可运行 `go run ./tests/window/widgets/check_button` 并排比较普通按钮和
按钮式 CheckButton，切换亮暗及主题色；例程使用系统界面字体和字号。

Switch 使用中性关闭轨道、主题色开启轨道及浅色圆形滑块，轨道为 36×20 DIP
胶囊。开启轨道仅填充，不叠加同色描边，关闭轨道保留弱轮廓。
文字独立解析 `switch-text`；键盘焦点为独立 2 DIP 边框，Hover/Pressed
不改变几何尺寸。`go run ./tests/window/widgets/switch` 可体验点击、水平拖动、
120ms 滑动过渡、禁用、亮暗切换及声明式重建。

追加规则可覆盖参数：

```go
rules := modern.Rules(modern.Options{Dark: true})
rules = append(rules, style.Name("menu").Shadow(style.Shadow{}))
sheet := style.Sheet(rules...)
```

运行 `go run ./example/modern` 可切换系统／亮／暗／GUI 兜底，以及紫、绿、黄、白、黑
主题色。建议检查鼠标悬停／按下、文本输入、滚动条和窗口装饰边界；
Menu／Long menu 可验证禁用项、分隔线、圆角阴影及长菜单滚动。
窗口边框和控制按钮仍归 GUI／平台管理，只允许样式改变窗口背景。

PopoverMenu 已接入 8 DIP 圆角、1 DIP 边框、阴影及菜单行样式。
菜单项悬停／按下使用固定灰色，不跟随主题色，仍可由应用样式覆盖。
菜单按内容撑开，主体默认最小宽度 160 DIP（含内部留白，不含阴影），不设默认
高度上限；工作区硬约束优先，空间不足时可收缩。发生滚动时，菜单项与滚动条
相隔 6 DIP。屏幕工作区约束和位置退避已接入，见 [Popup 定位](../../doc/DesignPopupPlacement.md)。
透明能力不可用时，菜单降级为无外围阴影的矩形；普通 Popover 的默认阴影为零，
显式请求透明失败仍返回错误。三平台外观仍需真机验收。

第三方控件必须在 init 中调用 `basic.Register` 提供默认规则，可同时调用
`modern.Register` 适配 Modern。应用无需额外导入适配包；正常导入控件后选择 Modern
即可。Modern 先拼接完整 Basic，因此只有 Basic 实现的控件仍有外观。各主题在首次
构造时冻结登记，注册函数返回独立、确定的普通规则。API、回调约束与覆盖顺序见
[第三方控件样式注册](../../doc/DesignThirdPartyStyles.md)。
