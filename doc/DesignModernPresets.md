# Modern 通用样式预设

## 范围与参考

2026-10-09：在现有 `theme/modern` 中增加六个具名预设，不增加 Widget、
Style 字段、CSS class 组合、隐式父子继承或平台适配。

参考 [libadwaita 1.9.4 的样式约定](https://gnome.pages.gitlab.gnome.org/libadwaita/doc/1.9/style-classes.html)：
Flat 区分静止与交互表面，Card 表达内容分组，Warning/Error/Success 表达状态。
这里只采用视觉语义，不复制 GTK 的 class 组合、继承或自动修改子控件机制。
GOUI 的 Sidebar 表示背景表面，不等同于 libadwaita 的 navigation-sidebar 列表策略。

## Modern 视觉原则与描边契约

2026-10-09 确认：本节定义整个 Modern 的视觉原则，不仅适用于下面的六个预设。
后续控件和预设应先明确自己所属的视觉类别，再选择颜色、描边和状态表现。

Modern 是中性、克制、边界清晰的现代桌面风格：用表面表达层次，用细描边明确
独立操作区域，用颜色表达交互和重要状态。跨平台使用同一套 Widget 视觉规则，
不模仿某个系统的整套原生外观；原生窗口装饰继续由 GUI／平台负责。

- 色彩：大面积表面、普通交互和内容行选中使用中性色。主题色用于 Primary、
  明确的状态指示及焦点，不把所有选中项都染成主题色；语义前景保持独立。
- 状态：悬停轻微变化，按下更明确，选中是持续状态。按钮鼠标聚焦不额外装饰，
  键盘焦点独立绘制；编辑框仍需要可见的编辑焦点。禁用保留值、几何与类别特征，
  不显示悬停／按下反馈。状态解释和输入行为仍归控件，不由样式实现。
- 几何：同类控件共享圆角、描边尺度；交互不改变尺寸、Padding 或文字位置。
  普通按钮描边 1 DIP，圆角 6 DIP；键盘焦点 2 DIP，不改变布局。
- 层次：普通按钮和 Card 不靠阴影制造立体感。阴影主要服务浮动表面，并由具体
  样式显式选择；Card 与普通 Popover 默认无阴影。亮暗模式只改变配色，不改变规则。
- 归属：样式名是完整预设，父控件不隐式改孩子的样式；内容表面和独立分隔线
  由各自消费者绘制。不增加 CSS、主题运行时或自动判定上下文的控件策略。

描边服务于边界辨识，不要求所有控件都套同一个外框：

| 类别 | 描边规则 |
| --- | --- |
| 普通 Button／MenuButton、DropDown、按钮式 CheckButton | 常态及交互状态保持 1 DIP 中性描边，明确独立操作区域 |
| Primary Button／MenuButton | 主题色实底，默认不叠加普通描边；键盘焦点亮色为深灰、暗色为浅灰，沿用 2 DIP 贴边框 |
| Flat Button／MenuButton | 常态、禁用透明且无描边；悬停／按下恢复普通按钮对应状态的背景及 1 DIP 描边 |
| TextInput | 常态 1 DIP；编辑焦点使用 2 DIP 主题色描边 |
| 菜单项、列表行、树节点 | 不逐项描边；用背景表示交互和选中；键盘当前项指示与普通表面独立 |
| Tab | 未选中无描边；选中／抬起表面使用细描边区分页面层次，不套普通按钮语义 |
| Card、Popover、菜单外壳 | 柔和 1 DIP 描边，区分内容表面与周围背景 |
| Sidebar、HeaderBar、普通布局容器 | 默认无描边；需要区域分隔时使用独立分隔线 |
| CheckBox、Radio、Switch | 描边服务于指示器形状与状态，不给整个控件外壳套边框 |

Switch 的 Modern 开启轨道仅填充主题色胶囊，不再绘制与填充同色的 1 DIP 描边。
重复的同色填充／描边会在抗锯齿边缘重复合成覆盖率，使亮色背景上的圆弧过渡
变硬；它并没有提供额外的边界区分。关闭轨道保留弱化的 1 DIP 描边，键盘焦点
仍是独立的 2 DIP 轮廓。轨道 36×20 DIP、圆角 10 DIP、滑块 16 DIP 及状态配色
不变；应用仍可显式覆盖轨道描边。不修改通用 Painter 或 Switch 行为。

Flat 明确定义为“常态收起表面的普通按钮”，而不是“所有状态都无框的按钮”。
悬停／按下的背景、描边颜色、宽度和圆角与普通按钮一致：亮色背景为
`#F6F6F6`／`#EDEDED`，暗色为 `#39393F`／`#414147`；描边分别为
`#C6C6CE`／`#55555F`。沿用普通按钮 4%／8% 的中性混合，不再从窗口背景计算。
常态／禁用无框与键盘焦点独立，不能把焦点边框当成悬停边框。
普通按钮规则不变；Primary 按下节采用实底。应用可追加 Flat 的 Hovered／Pressed 规则
关闭或改变描边，普通按钮仍从自身常态继承应用覆盖的描边。

参考边界：[libadwaita 1.9.4](https://gnome.pages.gitlab.gnome.org/libadwaita/doc/1.9/style-classes.html#flat)
将 Flat 描述为悬停前像文字或图标；[Flutter Material TextButton 当前官方 API](https://api.flutter.dev/flutter/material/TextButton-class.html)
明确采用无可见边框、依赖位置和上下文识别。这是文档承诺，不据此推断其每个
后端的具体绘制实现。GOUI 采用显式视觉分工，不复制自动 toolbar 样式或无框按钮
约定；Flat 在交互时恢复描边是本项目已确认的选择，不宣称是框架共同标准。

## API 与规则

### Primary 实底与显式内容（2026-10-12）

参考 [libadwaita 1.9 的 suggested-action 文档](https://gnome.pages.gitlab.gnome.org/libadwaita/doc/1.9/style-classes.html#suggested-action)
及固定 [1.9.0 源码 `_buttons.scss`](https://github.com/GNOME/libadwaita/blob/1.9.0/src/stylesheet/widgets/_buttons.scss)：
suggested-action 采用不透明主题背景与独立 accent 前景，交互通过明暗叠层表达。
这是 GTK Widget 样式行为，不是平台按钮承诺。GOUI 采用同样的实底视觉语义，
不复制 CSS、父子颜色继承或其交互混合比例。

- `modern.Primary`：Button/MenuButton 的完整主要操作样式；替换原 12%/18%/24% 浅底。
  6 DIP 圆角，常态、悬停、按下默认均无普通描边，以实底本身表达操作边界，
  避免描边与实底叠加造成生硬外观。应用仍可显式设置描边宽度和颜色。
- `modern.PrimaryContent`：Label/单色 Icon 的独立前景与主题字体，不提供背景、
  描边或布局。使用 `ui.Button().Content(ui.Label("保存").Style(modern.PrimaryContent)).Style(modern.Primary)`；
  原色图标不受前景替换影响。普通 Label/Icon 规则不变。
- 对主题色先按既有契约转为不透明 sRGB。相对亮度大于 0.5 的浅色使用黑字，
  其余优先白字；必要时朝黑／白微调实底到文字对比度至少 4.5:1。此阈值是
  GOUI 配色策略，不是系统事实。悬停／按下向黑混合 6%／12%；近黑实底（亮度
  小于 0.02）反向提亮，保证黑色仍有反馈。两种模式使用相同主题实底与对比前景。
- 禁用使用既有中性按钮底色及禁用前景，同样不叠加普通描边，不改变几何或行为。
- 鼠标 Focused 仍为零边框且保留原有备用颜色。FocusVisible 仍为 2 DIP 内绘边框；
  2026-10-13 按确认方案，仅 Primary 的键盘边框按模式选择固定灰色：
  亮色 `#414147`、暗色 `#D8D8DE`。不按主题实底或内容前景选择，不读取子控件
  实际样式，不把 Primary 实底加入共用焦点色的约束集合。普通 Button／Flat
  仍使用 75% 既有主题焦点色／25% 中性边框色，TextInput 不变。
  固定灰色不承诺与任意 Primary 实底达到专项 3:1；文字仍至少 4.5:1。
  既有几何、透明背景、应用覆盖和父子样式隔离均保留，不扩大到 GUI 绘制实现。
- demo 侧栏选中不是主要操作，改用私有 `demo-nav-current` 中性色表面，复制
  普通按钮规则并显式覆盖三态底色；子 Label 仍使用正文前景。确认操作和基础页
  Primary 采用显式内容样式，未选中分类继续使用 Flat。

验证：独立 RGB 字节参考、250 组亮暗／主题色组合的文字与焦点对比度、极端黑白
交互反馈、独立字体／应用覆盖、禁用与声明检查；桌面例程检查真实绘制、Tab 焦点
和侧栏。具体命令及结果记录在本文末尾，不将编译等同于原生桌面验收。

2026-10-12 上一轮配色阶段的验证（不代替当前默认焦点版本验收）：

- 已撤回公共焦点绘制内缩及普通按钮／Flat 的额外焦点色调整，`gui/style.go`、
  `gui/button_test.go` 与提交版本无差异；既有 1x／2x Software 边框像素回归通过。
- `go test -json -count=1 ./...`：2358 测试／子测试通过、15 条件跳过、零失败。
  跳过为既有平台手动／opt-in 场景及非 macOS 下的签名测试，未扩大 skip。
- `go test -race -count=1 ./theme/modern ./example/demo` 通过；Primary 三态实底、
  明暗／极端主题色、2 DIP 焦点、独立内容与应用覆盖回归通过。
- `GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build ./...` 及
  `GOOS=darwin GOARCH=arm64 CGO_ENABLED=0 go build ./...` 通过，仅代表编译验证。
- 最终 Primary 焦点配色的原生桌面外观留待用户通过 `example/modern`／`example/demo`
  验收；本轮没有执行 Windows/macOS 远端或新一轮桌面外观检查，未提交。

### Primary 焦点颜色收敛（2026-10-12）

按用户最新确认，撤回内缩方向及相关样式字段、GUI／widgets／Popover 实现、
测试和文档扩展。保留已完成的 Primary 实底、显式 PrimaryContent 及 demo 内容，
只调整 Modern 的 Primary 焦点颜色，继续使用现有 2 DIP 贴边框、6 DIP 外缘圆角。
亮色优先深色主题变体，暗色优先浅色变体，极端颜色选择能与三种实底保持至少
3:1 对比的方向。本阶段亮暗均采用 3:1，后续亮色柔化见下一节。
普通按钮、Flat 和 TextInput 的规则与绘制实现不变。

验证使用独立 RGB 字节参考、250 组主题色／亮暗组合、既有 1x／2x 边框像素测试
及全仓回归。原生外观留给用户验收，本轮不提交；执行结果在下文记录。

亮色柔化前的收敛版本验证（本机 Linux amd64；不代替下一节新版验收）：

- `git diff --exit-code -- style gui widgets` 通过；这三个目录的受跟踪代码已恢复
  至提交版本，取消的内缩字段、实现和专项测试无残留，不删除原有边框像素覆盖。
- RGB 独立参考包含 `#0078D4` 主题色：亮色焦点为 `#001F37`，暗色为 `#B3D7F2`；
  原色实底、悬停／按下色及黑白主题的反向对比仍有明确断言。
- `go test -json -count=1 ./...`：2358 测试／子测试通过、15 既有条件跳过、零失败。
  `go test -race -count=1 ./theme/modern ./example/demo` 及相同范围的 `go vet` 通过。
- Windows amd64／macOS arm64 的 `CGO_ENABLED=0 go build ./...` 通过，仅为编译检查。
- 本轮未操作原生桌面或远端；最终外观由用户运行 Modern／demo 用 Tab 验收。

### Primary 亮色焦点柔化（2026-10-12）

本节记录已被后续中性色方案替代的尝试，不作为当前配色契约或验收结果。

用户反馈亮色加深过黑、暗色外观合适。仅修改 `theme/modern` 的亮色 Primary
焦点颜色：向端点混合的起点从 50% 减为 25%，三态实底的目标从 3:1 调整为
2:1。不能只减少混合起点而保留 3:1，否则颜色仍会继续加深到原有近黑结果。
暗色保留 50%／3:1 的算法及全部颜色，普通按钮、Flat、TextInput 与绘制实现不变。
系统蓝 `#0078D4` 对应亮色焦点从 `#001F37` 改为 `#003D6C`，暗色仍为 `#B3D7F2`。

验证保留独立不透明 sRGB 字节预期，覆盖紫蓝／系统蓝／黑白主题、两种模式及
三态实底。色立方体的焦点断言仅对亮色 Primary 改为 2:1，暗色 Primary 和其他
控件原有 3:1 断言、文字 4.5:1 断言均保留。2 DIP 宽度、圆角、应用覆盖、
独立内容和禁用回归不变；不增加 skip 或颜色容差。

本轮验证记录：

- 新 RGB 断言先在旧代码失败：亮色紫蓝焦点实际 `{6 12 25 255}`，预期
  `{23 47 94 255}`；修正后 Modern 包内回归通过。
- 本机 Linux amd64：`go test -json -count=1 ./...` 2358 测试／子测试通过，
  15 既有条件跳过、零失败。跳过为 14 个平台手动／opt-in 场景和非 macOS 的
  `TestMacOSSigning`，未扩大 skip。完整结果保留在仓库外
  `/tmp/goui-primary-light.jhCc1l/all-tests.json`。
- `go test -race -count=1 ./theme/modern ./example/demo` 与
  `go vet ./theme/modern ./example/demo` 通过；独立 RGB、250 组亮暗／主题色组合、
  2 DIP 焦点、内容隔离、应用覆盖及既有边框像素回归保留。
- `GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build ./...` 和
  `GOOS=darwin GOARCH=arm64 CGO_ENABLED=0 go build ./...` 通过；
  `GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go build ./theme/modern` 通过，仅代表编译。
- 补充 `GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go build ./theme/modern ./example/modern ./example/demo`
  失败：`platform/linux/libs/libc/syscall_linux.go:29:42` 的 `syscall.SYS_PIPE` 与
  `:61:40` 的 `syscall.SYS_POLL` 未定义。该文件与 HEAD 无差异，确认属于既有平台
  编译问题，未扩大本轮修改范围；不能将 arm64 示例编译算作通过。
- `git diff --exit-code -- style gui widgets` 及本轮文件 `git diff --check` 通过；
  没有修改 GUI、widgets、样式值模型或公共绘制实现。
- 原生视觉由用户运行 `example/modern`／`example/demo` 用 Tab 再次验收；本轮不提交。

### Primary 中性色焦点（2026-10-12）

本节记录已被恢复默认焦点方案替代的尝试，不作为当前配色契约。

用户再次反馈亮色轮廓过深，并确认统一中性色。只将 Primary 的焦点色改为
浅灰 `#D8D8DE`／深灰 `#505056`，选择与其默认黑／白内容前景一致的明暗方向；
蓝、紫、黑等较深实底使用浅灰，黄、白等较浅实底使用深灰。亮暗模式复用相同
规则，取消焦点色的主题混合、端点判断和渐进调整。普通按钮、Flat、TextInput、
实底及交互颜色、独立内容样式、2 DIP 贴边绘制和圆角均不变。

独立不透明 sRGB 字节断言覆盖紫蓝、系统蓝、黑、白和黄；新增中性色断言
在旧代码失败后，修改配色使其通过。250 组主题色／亮暗组合同时断言固定灰色、
三态焦点至少 3:1 和正文至少 4.5:1，撤回仅亮色 Primary 的 2:1 例外；
不删除其他覆盖，不增加 skip 或颜色容差。示例入口注释同步说明中性色预期。

本轮验证（本机 Linux amd64）：

- 新 RGB 断言先在旧代码失败：亮色紫蓝焦点实际 `{23 47 94 255}`，预期
  `{216 216 222 255}`；修正后 `go test -count=1 ./theme/modern ./example/demo` 通过。
- `go test -json -count=1 ./...`：2358 测试／子测试通过、15 既有条件跳过、零失败。
  跳过仍为 14 个平台手动／opt-in 场景及非 macOS 的 `TestMacOSSigning`，未扩大。
  完整结果在 `/tmp/goui-primary-neutral.8HE5yb/all-tests.json`。
- `go test -race -count=1 ./theme/modern ./example/demo` 与
  `go vet ./theme/modern ./example/demo` 通过。
- `GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build ./...`、
  `GOOS=darwin GOARCH=arm64 CGO_ENABLED=0 go build ./...` 及
  `GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go build ./theme/modern` 通过，仅为编译检查。
  前一阶段发现的 Linux arm64 示例 libc 编译问题未修改，本轮不重复验证该入口。
- `git diff --exit-code -- style gui widgets` 与本轮文件 `git diff --check` 通过；
  未修改 GUI、widgets、样式值模型或公共绘制实现。
- 未操作原生桌面或远端；外观由用户通过 Modern／demo 再次验收，本轮不提交。

### Primary 恢复默认焦点（2026-10-12）

本节记录已被下节按亮暗模式选择灰色的方案替代的阶段，不作为当前配色契约。

按用户请求，撤回 Primary 专用焦点配色，复用普通 Button 现有的 Focused／
FocusVisible 规则。删除专用颜色生成与分配，保持透明背景、鼠标焦点零边框、
键盘焦点 2 DIP 贴边绘制和圆角。Primary 实底、三态反馈、显式 PrimaryContent、
普通按钮／Flat／TextInput 与 GUI 实现均不变；不增加整体提亮叠层。

回归以恢复默认契约为准：250 组主题色／亮暗组合断言 Primary 键盘焦点与
普通 Button 同色，不再检查已撤回专用颜色的实底 3:1 目标。默认主题的独立 RGB
字节预期（亮色 `{102 147 217 255}`／暗色 `{87 127 190 255}`）、75%／25% 的
独立整数混合预期、2 DIP 宽度、圆角、应用覆盖、禁用、正文对比度、实底及内容
隔离覆盖保留；其他控件原有对比断言不变，不增加 skip 或容差。

本轮验证（本机 Linux amd64）：

- 默认焦点回归先在旧代码失败：Primary 指针焦点的备用颜色为 `{216 216 222 255}`，
  默认按钮为 `{70 130 220 255}`；修正后
  `go test -count=1 ./theme/modern ./example/demo` 通过。
- `go test -json -count=1 ./...`：2358 测试／子测试通过、15 既有条件跳过、零失败。
  跳过仍为 14 个平台手动／opt-in 场景及非 macOS 的 `TestMacOSSigning`，未扩大。
  完整结果在 `/tmp/goui-primary-default.8rqN4M/all-tests.json`。
- `go test -race -count=1 ./theme/modern ./example/demo` 与
  `go vet ./theme/modern ./example/demo` 通过。
- `GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build ./...`、
  `GOOS=darwin GOARCH=arm64 CGO_ENABLED=0 go build ./...` 及
  `GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go build ./theme/modern` 通过，仅为编译检查。
- `git diff --exit-code -- style gui widgets` 与本轮文件 `git diff --check` 通过；
  没有扩展 GUI、样式字段或公共绘制实现。
- 未执行原生桌面或远端验收；用户运行 Modern／demo 再看，本轮不提交。

### Primary 按亮暗模式选择焦点灰色（2026-10-13）

按用户确认，只调整 Primary 的 FocusVisible：亮色 `#414147`、暗色 `#D8D8DE`。
选择只取决于 Options.Dark，不取决于主题色、实底或内容前景；撤回之前按实底
选择灰色的策略。鼠标 Focused 的透明背景、零宽和备用颜色、既有 2 DIP 贴边
绘制、6 DIP 圆角、Primary 实底与显式内容均不变。普通 Button／Flat／TextInput、
GUI、widgets、样式值模型和公共绘制实现不修改。

回归使用独立不透明 sRGB 字节预期；新断言先在旧实现失败：亮色默认 Primary
焦点实际 `{102 147 217 255}`，预期 `{65 65 71 255}`。250 组亮暗／主题色组合
验证固定灰色只随模式改变；保留普通按钮的独立整数混合、指针焦点一致性、
2 DIP、圆角、应用覆盖、禁用、文字 4.5:1、实底和内容隔离测试。不恢复已撤回的
任意实底焦点 3:1 承诺，不增加容差或 skip。

本轮验证（本机 Linux amd64；样式逻辑测试不涉及后端／缩放）：

- `go test -count=1 ./theme/modern ./example/demo` 通过。
- `go test -json -count=1 ./...`：2358 测试／子测试通过、15 既有条件跳过、零失败。
  跳过仍为 14 个平台手动／opt-in 场景及非 macOS 的 `TestMacOSSigning`，未扩大。
  完整结果在 `/tmp/goui-primary-mode-gray.os65dR/all-tests.json`。全仓包含既有原生窗口
  测试及图片输出副作用，不代表纯无头测试；本轮没有新增此类副作用。
- `go test -race -count=1 ./theme/modern ./example/demo` 与
  `go vet ./theme/modern ./example/demo` 通过。
- `GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build ./...`、
  `GOOS=darwin GOARCH=arm64 CGO_ENABLED=0 go build ./...` 及
  `GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go build ./theme/modern` 通过，仅代表编译。
  前一阶段确认的 Linux arm64 示例 libc 编译问题未修改，本轮不重复验证该入口。
- `git diff --exit-code -- style gui widgets` 通过；没有扩展样式字段或公共绘制实现。
- 未执行原生桌面或远端外观验收；由用户运行 Modern／demo 再看，本轮不提交。

### 通用表面与语义前景

| 常量 | StyleName | 消费方与默认值 |
| --- | --- | --- |
| `modern.Card` | `card` | 绘制背景的 Widget；中性表面、1 DIP 柔和边框、10 DIP 圆角、明确无阴影 |
| `modern.Sidebar` | `sidebar` | 绘制背景的 Widget；中性底色、直角、零边框、明确无阴影 |
| `modern.Flat` | `flat` | Button/MenuButton；静止和禁用透明无框、悬停/按下同普通按钮且有 1 DIP 描边、6 DIP 圆角 |
| `modern.Warning` | `warning` | Label/Icon 等前景消费者；警告色，不添加背景、边框、圆角或阴影 |
| `modern.Error` | `error` | Label/Icon 等前景消费者；错误色，不表示危险操作按钮或输入校验状态 |
| `modern.Success` | `success` | Label/Icon 等前景消费者；成功色，不添加表面装饰 |

Sidebar 使用独立的调色板值：亮色 `#E2E2E8`，暗色 `#18181C`。
两种模式均比 Window/Card 更深，不再通过窗口背景向正文混合 2.5% 推导。
Sidebar 调整仅改变侧栏底色，独立于后续 Flat 交互规则，不改变普通按钮、Primary 或语义前景；
子控件的悬停/按下、当前分类与 Tab 焦点仍由各自原有规则绘制。

每个预设都有独立的正文前景与主题字体规则。禁用时使用既有中性禁用色；
不影响控件行为。Flat 复用按钮的独立 focus part：鼠标聚焦零边框，
FocusVisible 为 2 DIP 柔化主题色边框，不改变布局或悬停/按下底色。
应用仍可在 `modern.Rules` 后追加规则覆盖同一 selector。

Color 按现有标准库 sRGB 值表达，交给既有绘制边界转换。状态色只随 Dark 改变，
不随 AccentColor 改变：

| 前景 | 亮色 | 暗色 |
| --- | --- | --- |
| Warning | `#8A5900` | `#EAC16D` |
| Error | `#BA3038` | `#F3959A` |
| Success | `#276B43` | `#83CAA2` |

三种状态前景在默认 Window/Card/Sidebar 和普通按钮表面上保持至少 4.5:1
对比度；禁用色沿用既有弱化表现，不承诺正文对比度。

样式名是完整预设，而不是叠加 class；`Style("flat primary")` 没有组合语义。
父控件不把样式传给子 Label/Icon，调用方分别设置。
Card/Sidebar 不提供 Padding 或布局，也不让不绘制背景的 HBox/VBox 自动变成表面。
只有自绘表面的消费者才绘制这些字段。

## 示例集成

`example/demo` 继续使用现有应用自定义 surface Bin：全部 section 选用 Card，
侧栏外壳选用 Sidebar，未选中的导航按钮选用 Flat，当前页使用私有中性选中样式。
浮动查找面板、两种确认 Dialog、Toast、辅助窗口内容、权重布局表面及拖放接收区
同样选用 Card；遮罩保留独立的半透明规则，透明窗口的外围保留独立背景规则。
基础页增加可点击/禁用 Flat 以及独立设置样式的警告、错误、成功文字和图标；
新增文案在 resource 中提供现有八语言翻译，固定 ID 不参与翻译。
不修改 Window Integrated 避让、透明窗口背景、页面滚动边距或控件默认行为。

`example/modern` 增加 Flat、禁用 Flat 和三种状态文字。
GUI 兜底模式清空这些预设名，不依赖未启用的 Modern 规则。

## 验证

包内测试检查独立颜色预期、亮暗/极端主题色、字体完整性、状态前景、
禁用、Flat 的六种状态、focus part、对比度、应用覆盖及其他名字不受影响。
demo 布局回归使用实际 surface 承载侧栏，仍须填满分配空间且不增加边缘内缩；
资源测试覆盖新文本的八语言与格式参数。

本机窗口通过真实桌面输入检查：亮暗/主题色切换、Flat 悬停/按下/点击、
禁用不可激活、Tab 焦点、语义图标与文字、侧栏导航、卡片边缘、缩窄后滚动。
不使用 DevServer；后端和缩放覆盖值与实际桌面缩放分别记录。

## 实施与验收记录（2026-10-09）

- 六个预设、完整 Flat 交互/focus、状态色与独立字体规则已实现；不修改 gui、
  ui、style 或 platform。demo 已移除私有 demo-card 配色，采用公共 Card/Sidebar，
  新增七条具名八语言资源。导航当前页仍为 Primary，其他项使用 Flat。
- `go test ./...` 与非缓存 `go test -json -count=1 ./...` 通过：2138 项通过、
  14 项既有窗口/剪贴板/GLX 人工测试跳过，无新增失败或跳过。
- `go test -race ./theme/modern ./example/demo/...` 与
  `go vet ./theme/modern ./example/demo/... ./example/modern` 通过。
- Windows amd64、macOS arm64 的 demo 与 modern 示例交叉编译通过，
  `CGO_ENABLED=0`；没有把编译记录为原生运行或桌面外观验收。
- 本机 Linux amd64/X11，隔离 TigerVNC 1.12.0 / Xfwm4 合成桌面：
  Software 1x、OpenGL 请求配置 2x 的 demo 亮暗、主题切换和显示通过。
  2x 是 `GOUI_PLAT_SCALE` 应用覆盖，不代表物理显示器 HiDPI/混合 DPI；
  平台工厂仍允许 OpenGL 初始化失败后回退 Software，未插入私有后端探针。
- 真实桌面输入验证 Flat 悬停/按下/点击反馈、禁用点击不改变反馈、
  Shift+Tab/Tab 聚焦边框、侧栏切页；Software 1x 另检查 820×600 缩窄和滚动。
  截图采样用独立字节预期断言 Flat 悬停 `#EDEDF0`、按下 `#E5E5E7`，
  Card/Sidebar 两套背景以及三种语义色的实心文字/图标像素；
  改主题色后语义行像素保持相同。形状、文字抗锯齿不套跨平台金图。
- modern 示例 Software 1x 检查 Flat MenuButton 打开菜单、Flat action 更新
  输入框以及 Bare GUI 清空预设名；截图的透明 popup 黑色外侧是单独 drawable
  采集时丢弃 alpha 的结果，不当作桌面合成外观证据。
- 验证产物在 `/tmp/goui-modern-presets.9vRfgV/`，不写入仓库。
  程序正常关闭；隔离桌面随后停止，不操作用户原来的桌面会话。

未执行：Windows/macOS 原生桌面、本机 Software 2x/OpenGL 请求配置 1x 的
完整组合、物理显示器 HiDPI、八语言母语外观校对。新增文本的八语言定义、
实际解析与格式参数已由 resource 包测试覆盖。本轮没有平台或绘制契约变更。

## Sidebar 背景调整（2026-10-09）

侧栏旧背景通过窗口/正文混合 2.5% 得到，与 Flat 的悬停表面过近。
此次改为独立 palette 字段，亮色从 `#F1F1F3` 改为 `#E2E2E8`，
暗色从 `#252529` 改为 `#18181C`。不改变 Flat、普通按钮、Primary、
语义前景、布局、边缘避让或 demo 的声明。

新增回归检查侧栏比 Window/Card 和既有 Flat 交互表面更深，正文、MutedText、
Warning/Error/Success 在实际 Sidebar 背景上的对比度不低于 4.5:1。
原有表面颜色预期随契约调整，Flat 的交互颜色预期保持不变。

已通过：`go test ./...`、Modern/demo 包非缓存测试、相关 race/vet；
Windows amd64/macOS arm64 的 Modern 测试目标编译。
本机隔离 TigerVNC/X11/Xfwm4、Software 1x 的 demo 检查亮暗模式、分类切换、
Flat 悬停/按下及 Primary 选中项。截图按独立 RGB 字节断言新背景和原交互色，
并与上一版 Software 1x 截图比较选中项内部及右侧卡片内容，确认不受影响。
验证产物在 `/tmp/goui-sidebar.O79RsC/`；测试程序和隔离桌面已退出。

本轮未重新执行 OpenGL/Direct2D 原生窗口、远端桌面或物理 HiDPI 验收。
此前按用户要求保留工作区供体验，本轮用户已明确要求提交。

## demo 卡片引用修复（2026-10-09）

用户反馈的浮动查找面板与遮罩 Dialog 透明，原因是 demo 删除私有 `demo-card`
规则后，五个场景文件中仍有八处旧样式引用。具名样式不存在就不提供背景，
surface 不应自行猜测 fallback；这不是 Sidebar 配色或透明绘制契约的问题。

将查找面板、确认 Dialog、Toast、标签关闭确认框、辅助窗口内容、两个权重布局
表面及拖放接收区统一改为 `modern.Card`。保留三处同名 `demo-card` 拖放数据
格式标识，不添加兼容样式别名，不修改 gui/ui/style/platform 或 surface 绘制实现。
遮罩 alpha 140、透明辅助窗口的外围、现有布局与事件处理保持不变。

回归测试 `TestDemoCardStyleMigration` 只检查 Style/SetStyleName 调用：在修复前
明确报告八处遗留引用，修复后通过；不将应用内数据格式误判为样式。

验证结果：

- `go test -json -count=1 ./...` 通过：2140 项通过、14 项既有测试跳过，
  无新增失败或跳过；不把无测试文件的包计作测试跳过。
- `go test -count=1 ./example/demo/... ./theme/modern`、相关 race 与 vet 通过。
- Windows amd64/macOS arm64 demo 的 `CGO_ENABLED=0` 交叉编译通过；未原生运行。
- 本机 Linux amd64，隔离 TigerVNC 1.12.0/X11/Xfwm4 合成桌面、Software 1x，
  真实鼠标输入复现旧版面板/Dialog 背景缺失，再验证修复后的亮暗模式。
  查找面板、确认 Dialog、Toast、标签关闭确认框、两处权重表面、拖放接收区和
  透明辅助窗口内部卡片均恢复背景；切换模式时同步更新。
- 截图在避开文字、边框与抗锯齿的内部空白像素按独立 RGB 预期断言：
  Card 亮色 `(255,255,255)`、暗色 `(40,40,45)`。另采集完整桌面验证透明辅助
  窗口外围透出下方侧栏，而不是使用丢弃 alpha 的单窗口图片推断合成结果。
  初次诊断采样将 Dialog 下方的白色卡片误当窗口底色，修正采样基准后，
  旧版遮罩像素符合 `255 × (1 − 140/255) = 115`，未调整容差或绘制代码。

未执行：本轮 OpenGL/Direct2D 原生窗口、远端桌面与物理 HiDPI 验收。
产物位于 `/tmp/goui-demo-card.XW7jPy/`；测试应用正常关闭，隔离桌面停止。

## 视觉原则与 Flat 交互统一验收（2026-10-09）

完整视觉约定已写入本文件，主设计文档同步引用；本轮代码仅调整 Flat 的交互
背景和描边，不改变其他控件的外观、控件行为、布局、GUI 兜底或 Painter。
示例说明同步区分常态无框、交互 1 DIP 描边与键盘焦点 2 DIP 描边。

- 调整 `TestFlatButtonAppearance` 的独立 RGB、宽度和颜色预期，保留全部六种
  状态、亮暗、极端主题色、圆角、前景和对比度覆盖。修复前该测试明确失败：
  悬停为 `(237,237,240)`，契约要求 `(246,246,246)`；修复后通过。
  独立预期通过后再检查完整 Hovered／Pressed 样式与普通按钮一致。
- 新增应用覆盖回归：普通按钮／Primary 仍继承常态的自定义描边，Flat 的交互
  描边可按状态关闭或改变，不影响常态／禁用；已有焦点覆盖回归保留。
- `go test -json -count=1 ./...` 通过：2141 项通过、14 项既有测试跳过，无新增
  失败或跳过；非缓存 Modern/demo 测试、相关 race 和 vet 均通过。
- Windows amd64/macOS arm64 的 Modern 示例及 Modern 测试目标、Linux arm64
  的 Modern 测试目标交叉编译通过，均使用 `CGO_ENABLED=0`；不计作原生运行。
- 本机 Linux amd64、隔离 TigerVNC 1.12.0/X11/Xfwm4 合成桌面、Software 1x：
  真实鼠标验证 Flat Button／MenuButton 的常态、悬停、按下、点击，禁用不可
  激活，菜单可打开；Tab／Shift+Tab 验证独立键盘焦点，两种模式均通过。
- 截图的内部空白和直边全覆盖像素按独立不透明 sRGB 字节断言，无抗锯齿容差：
  `(25,153)` 为背景，`(21,153)` 为 1 DIP／1 像素描边，下一列 `(22,153)`
  恢复背景；固定全覆盖文字像素位置在常态／悬停／按下间保持相同。
  菜单按钮采样验证相同交互色；禁用点击前后输入框截图保持相同。

本轮未执行 OpenGL/Direct2D 原生桌面、Windows/macOS 远端、分数缩放与物理
HiDPI 验收，不将编译或 Software 1x 的像素结果扩展为完整后端矩阵通过。
产物位于 `/tmp/goui-flat-interaction.4N1qN1/`，测试程序正常退出，隔离桌面停止。
