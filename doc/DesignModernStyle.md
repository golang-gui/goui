# Modern 样式与运行时换肤

> 状态：六个切片的代码和本机自动测试已完成；跨平台运行与外观验收分别记录，
> 不以交叉编译替代真机验证。PopoverMenu 的具体宿主契约见
> [PopoverMenu 外观](DesignPopoverMenuAppearance.md)。

## Modern 视觉理念

2026-10-09 确认：Modern 是中性、克制、边界清晰的现代桌面风格。用表面表达
层次，用细描边明确独立操作区域，用颜色表达交互和重要状态；亮暗切换不改变
这些规则，原生窗口装饰仍归 GUI／平台。

统一的视觉原则、类别描边表、状态与参考边界见
[Modern 视觉原则与描边契约](DesignModernPresets.md#modern-视觉原则与描边契约)。
该约定适用于整个 Modern，后续控件与预设应遵循，不由局部实现自行选择。
Flat 是“常态收起表面的普通按钮”：常态、禁用透明无框，悬停／按下恢复普通
按钮对应的背景与 1 DIP 描边；键盘焦点保持独立 2 DIP 提示，不影响布局。

## 1. 目标与实施顺序

提供独立、显式启用的现代样式，支持主题色、亮暗切换和系统字体。继续使用普通
`style.StyleSheet`，不引入 Theme 运行时、CSS、隐式继承或依赖追踪。

严格按以下顺序，每步分别实现、测试和交付：

| 顺序 | 切片 | 完成边界 |
|---|---|---|
| 1 | BoxShadow 样式 | 阴影值、覆盖、关闭、比较及三后端阴影颜色约定 |
| 2 | 样式变化钩子 | Widget 样式通知与文本、测量、列表缓存失效 |
| 3 | 窗口装饰隔离 | 装饰由 GUI／平台管理，应用样式仅改变窗口背景 |
| 4 | UI Settings 自动更新 | 内部订阅、合并更新及监听生命周期 |
| 5 | Modern 样式 | 完整亮暗、主题色、系统字体、菜单及阴影规则 |
| 6 | PopoverMenu 外观 | 消费样式，完成透明外壳、定位、内容、输入和降级 |

普通 Overlay 容器及 UI 薄适配已实现。Menu 和 DropDown 继续用原生 Popover，
不迁移到 Overlay；Tooltip 尚非内置完整能力，不能据此承诺承载方式。

## 2. BoxShadow 样式

```go
type Shadow struct {
    Color        color.Color
    Offset       geometry.Point
    BlurRadius   float32
    SpreadRadius float32
}

func (s Style) Shadow() (Shadow, bool)
func (r Rule) Shadow(shadow Shadow) Rule
```

- `style` 只依赖基础几何及标准颜色值，不依赖 platform。
- 几何使用 DIP，与 `graphics.BoxShadow` 同义；GUI 消费时才转换颜色和类型。
- 首版单个阴影，作为完整字段覆盖，不逐成员合并，不增加 Elevation 或数组。
- 未设置与显式 `Shadow(Shadow{})` 不同；后者关闭从较一般规则得到的阴影。
- nil／透明颜色不绘制、不占位。几何归一化归绘制／宿主消费方，不由规则构造器完成。
- 字段标记、Resolve 和 SameRules 都支持阴影；颜色按 RGBA 值比较，不比较接口身份。
- 不自动使普通 Widget 绘制外阴影；首个消费者是第 6 步的 Popover 宿主。

同时校正现有阴影路径的颜色约定：`graphics.BoxShadow.Color` 按预乘颜色解释。
OpenGL/NanoVG 与 Direct2D 原生入口反预乘一次，Software 将预乘 RGB 和 alpha
同时乘遮罩覆盖率。清晰阴影及软阴影失败后的清晰降级使用相同约定。
alpha 为零时直接跳过，不除零。不修改普通 Brush、Clear 或整个颜色系统。

Direct2D 的 [Shadow Color 属性](https://learn.microsoft.com/en-us/windows/win32/direct2d/drop-shadow)
与 [ColorF 画刷颜色](https://learn.microsoft.com/en-us/windows/win32/direct2d/supported-pixel-formats-and-alpha-modes)
使用非预乘输入；黑色阴影不足以验证此契约，必须包含半透明彩色阴影。

## 3. Widget 样式变化钩子

在 `gui.Widget` 增加 `StyleChanged()`，`WidgetBase` 提供空实现。它是与
Measure／Paint 一样的框架钩子，不另设同义应用信号或 SetOn 回调。

通知覆盖全局替换样式表、当前控件 SetStyleName、实际启用状态变化、首次使用
和重新挂载；失效不保证最终解析出的样式值不同：

- 全局变化使窗口内容、内部装饰树及现有 Popover 内容失效。
- 样式名变化只影响该控件，不向子控件传播样式名。
- 实际启用状态变化使受影响子树的样式资源失效，各子节点独立解析 Disabled；
  输入取消由内核立即处理，不在 StyleChanged 中改变交互或业务状态。
- 框架先标记样式和测量失效，在后续框架测量／绘制入口派发通知，连续修改合并。
- 派发通过真实 Widget 接口，不给 WidgetBase 增加 owner/self 指针。
- 框架保证布局和绘制失效，不依赖实现者调用 Base。钩子仅清理或更新本地派生资源，
  不应在其中修改全局样式表、重构控件树或递归发起布局。
- 交互状态仍由控件管理，不把 Hover／Pressed 转换为全树换肤通知。

已实现的派发入口是框架的 measureWidget／paintWidget，以及 ScrollView 对
Scrollable 的 ContentSize／LayoutVisible 调用边界：虚拟列表可能跳过 Measure，
不能只在普通测量入口通知。控件内部不逐帧检查全局 revision，也不增加 owner 指针。
直接调用控件的 Measure／Paint 方法不会代替框架派发，和其他框架钩子的使用方式一致。

已挂载控件通过样式表替换统一标记；未挂载及复用池中的对象在重新挂接时标记。
Popover 原生资源尚未创建／已经释放时没有全局样式连接，下一次创建前重新使其
内容失效，避免先前自然尺寸测量缓存越过未监听的换肤。销毁对象不再接收钩子。

Label／TextInput 通过钩子释放旧 TextLayout，之后按需创建；不为换肤逐帧比较
字体、颜色和样式名。尺寸更新保留原有布局路径。继续复用 TextLayout 销毁通知
与 Painter 图像缓存，不增加第二套缓存。

ListView 清除行尺寸、宽度及定位缓存，但保留已存在的行控件、复用池和滚动偏移。
旧平均行高可作为重新测量时的估计，实际范围收缩时再钳制滚动位置。
ListView 的按索引尺寸缓存还必须检查行的测量有效性，不能遮蔽单行／子控件
SetStyleName 经 RequestLayout 向上传播的失效。没有变化的帧继续复用测量与文字图像。

TextInput 本轮仅处理样式变化后的显示资源，不重设计编辑模型、IME、选择及
UI 文本属性协调。不能因此承诺解决已有的声明式重建影响光标等编辑问题。
菜单行的独立文字样式接入留到第 6 步，文字资源继续由内部 Label 管理。
本步仅为现有 menuContent 接入自然尺寸失效，并释放不挂载的临时测量行的 Label
资源；没有改变菜单行外观、布局参数或 Label 归属。

## 4. 窗口装饰隔离

Native／Integrated 的窗口装饰由 GUI 或平台决定。应用样式仅控制窗口背景，
不控制边框几何、圆角、缩放热区和窗口控制按钮的外观。

- 将现有 `window/frame` 和窗口自绘按钮对应用 StyleSheet 的依赖收回 GUI 内部。
- Linux GUI Integrated 保留圆角背景、内部安全 ClipRect、边框和缩放区域。
  当前根内容的 Measure／Arrange 同样使用安全内容区域，避让四边缩放区；
  客户区坐标不变，HeaderBar／Controls 的协作计入内容起点。其他平台／Chrome 不自动内缩。
- 自绘控制按钮不再使用普通 Button／`window-control` 应用规则；尺寸和交互不变。
- GUI 自绘装饰根据最终窗口背景选择内部亮暗配色；原生装饰仍交给系统，不保证
  原生边框和 macOS 按钮与应用主题像素一致。
- 不新增公开装饰参数，不限制特殊透明 None 窗口中应用自己的内容绘制。
- 普通 HeaderBar 内容仍是 Widget，可以使用样式；Modern 默认令其背景与窗口一致。

第 3 步实现：

- `gui/window_appearance.go` 集中保存私有装饰尺寸、配色和背景亮暗判断，
  不再向 `DefaultStyleRules` 导出 `window/frame`、`window-control` 装饰规则。
- `window.frameInset` 独立计算安全裁剪／缩放边缘，不在输入命中时解析应用样式。
  圆角 12 DIP、边框 1 DIP、至少 5 DIP 的安全边缘及物理像素向内取整保持不变。
- `captionButton` 复用 Button 的事件和状态，但独立测量、绘制；测量不读取应用字体，
  图标颜色也来自私有配色。矩形按钮与 Linux 圆形按钮保留原有尺寸和亮色外观。
- 不透明宿主按窗口背景覆盖黑色清空底色后的亮度选择配色；透明宿主无法知道合成器
  背景，按声明的非预乘 RGB 选择，完全透明／缺失颜色固定使用亮色装饰。
  不尝试采样应用内容或桌面，不增加主题对象、缓存或公开装饰配置。
- `gui/window_appearance_test.go` 用完整替换的极端样式表验证普通 Button 仍可换肤，
  但窗口装饰的所有像素、测量、位置、安全边缘和缩放命中不变；覆盖正常／悬停／按下、
  1x／1.25x／1.5x／2x，以及运行时亮暗切换、图标对比和透明背景策略。

第 3 步已通过 `go test ./...`，Windows amd64／macOS arm64 的 GUI 测试目标交叉编译。
真机验收仍需检查 Windows 自绘按钮与 Snap、macOS 原生按钮、Linux 圆角和缩放光标；
本切片没有修改平台或 Painter，也不将交叉编译视为原生运行验证。

## 5. Settings 与 UI 更新

UI 在应用运行期间建立唯一的 Settings.ConnectChanged 连接，内部调用
RequestUpdate，复用更新合并机制；退出及初始化失败时断开连接。用户无需声明
OnSettingsChanged，不新增隐式响应式依赖跟踪。

GUI Settings 初始化即获取首份解析快照，getter 返回快照。保留当前轮询机制，
平台读取、快照更新和通知均在 GUI 线程执行；监听随应用运行结束而停止，已经投递
但过期的检查不再执行。本轮不扩展平台设置通知 API。

UI 构建函数读取设置并传给 Modern。固定模式或主题色直接传固定值；相同样式表
由现有比较机制跳过，不因无关系统设置变化反复清理资源。GUI 命令式应用继续通过
Settings 信号主动重建样式表；Modern 包本身不订阅设置。

第 4 步实现：

- `gui/settings.go` 创建时获取完整快照，逐字段应用 GUI fallback；颜色保存为自有
  RGBA64 值，不保留平台可变颜色对象。无效颜色模式、空字体、非正／非有限字号回退。
  getter 只读快照，变化通知前先替换整个快照，首轮轮询不再丢失变化。
- `gui.Application.Run` 启动两秒一次的设置轮询，返回时取消。后台只投递任务，
  平台查询和信号派发均在 GUI 线程；退出前已投递的任务通过取消标记失效。
  应用创建不启动后台监听，UI 初始化失败而未进入 Run 时也不存在遗留轮询。
- `ui.app.run` 在首轮 build 前连接唯一的 Settings 信号，沿用 RequestUpdate 的
  延迟／合并机制。初始化失败、运行中更新失败、主动 Quit 和原生循环直接返回
  都会断开连接；停止后的排队更新不再重建，Sync 也不在停止后直接执行。
- 不新增 OnSettingsChanged 或 Settings 的声明式回调；用户在 build 中读取设置。
  相同样式表仍由现有 `sameSheet` 比较跳过，未扩展样式表 API 或引入主题订阅。
- `gui/settings_test.go`、`ui/settings_test.go` 覆盖初始快照、第一次变化、颜色所有权、
  无效值／查询错误／恢复、相同值不通知、轮询退出、信号连接清理、更新合并、
  原有 Window／Widget 复用，以及无关设置变化不重复下发样式。

第 4 步已通过全仓库测试、Settings 专项竞态检查，以及 Windows amd64／macOS arm64
的 GUI／UI 测试目标交叉编译。系统原生设置切换仍需真机验证；轮询响应延迟约为
两秒加事件循环排队时间，不承诺即时通知。本步未修改 platform 或实现 Modern。

## 6. Modern 包与视觉规则

独立 `theme/modern` 保持显式选择，Rules／Sheet 只根据传入值生成样式。
2026-10-13 增加 FromSettings 便利入口，使用 GUI 设置类型接收已解析快照，
但 GUI／UI 不反向依赖 Modern，也不新增主题运行时或监听。
完整契约见 [Basic 兜底与 Modern 便利入口](DesignThemeFallback.md)。

```go
type Options struct {
    Dark        bool
    AccentColor color.Color
    FontFamily  string
    FontSize    float32 // pt
}

func Sheet(options Options) style.StyleSheet
func Rules(options Options) []style.Rule

func FromSettings(settings Settings) style.StyleSheet

const Primary = "primary"
const MutedText = "muted-text"
```

显式启用；nil custom sheet 使用完整 Basic，包含 widgets 与内部文本，不自动启用 Modern。
FromSettings 接收 gui.Settings／ui.Settings，并复制亮暗、主题色、字体族和字号。
UI 在 build 中一行调用，跟随变化复用现有 Settings 自动重建；GUI 仍显式
在 Settings.ConnectChanged 时替换样式表。生成函数不保存对象或注册监听。
零值 Options 为浅色；nil／透明主题色使用 #4682DC；空字体族交给平台默认字体；
非正或非有限字号使用 14pt。主题色转换为不透明配色种子，不保留调用者可变颜色
引用。同样输入生成相同规则，Rules 返回独立切片。

| 用途 | 浅色 | 深色 |
|---|---|---|
| 窗口背景 | #F6F6F8 | #202024 |
| 输入框、Popover 表面 | #FFFFFF | #28282D |
| 普通按钮 | #FFFFFF | #323238 |
| 正文 | #202024 | #ECECEE |
| 次要文本 | #61616B | #B8B8C2 |
| 禁用文本 | #97979F | #797983 |
| 边框、分隔线 | #C6C6CE | #55555F |

覆盖当前 Widget、Label、Button／MenuButton、TextInput、ScrollView 及 trough/thumb、
HeaderBar、Popover、Menu、MenuItem 和分隔线。window 仅提供背景规则，不提供
frame 或 window-control 装饰规则，也不导入或拼接 GUI 的默认规则。

- Button／TextInput 圆角 6 DIP，MenuItem 4 DIP；滚动条保持既有 8 DIP 宽胶囊。
- 普通按钮悬停／按下背景混合正文色 4%／8%；菜单项使用与主题色无关的固定中性色。
  浅色悬停／按下为 `#E6E6E6`／`#D1D1D1`，深色为 `#3E3E42`／`#4F4F53`。
  这些是 Modern 默认规则，应用仍可通过后追加的样式覆盖。
- Primary 使用主题色实底，默认无普通描边；内容显式使用 PrimaryContent 的
  对比前景／主题字体。2026-10-12 替换原浅底方案，完整契约见 [Modern 预设](DesignModernPresets.md#primary-实底与显式内容2026-10-12)。
- Button／MenuButton 保留独立 focus part；鼠标聚焦默认无额外描边，键盘 FocusVisible 为 2 DIP 内绘边框，应用可追加规则调整或关闭。
- 配色在生成时完成；除有意的透明背景和阴影外，混合结果存为不透明颜色。
- 正文对比度目标 4.5:1，文本输入框焦点指示目标 3:1。主题色过亮或过暗时，焦点色朝正文色
  调整至满足目标。Primary 按黑／白前景分别调整实底明度；仅其键盘焦点按模式
  使用固定灰色：亮色 `#414147`、暗色 `#D8D8DE`，不按实底或前景选择。
  普通 Button／Flat 仍使用 75% 既有主题焦点色／25% 中性边框色；Primary 鼠标
  焦点零边框及备用颜色不变。不承诺固定灰色与任意 Primary 实底的对比目标。
  线宽仍为 2 DIP，不额外内缩，不增加整体提亮叠层；普通按钮／Flat 的配色与
  几何不变，不扩展样式字段或修改公共绘制。默认规则不作为无障碍合规承诺。
- 菜单外壳 8 DIP 圆角、1 DIP 边框；默认黑色阴影 alpha=46/255，Y 偏移 4 DIP，
  BlurRadius=16 DIP、SpreadRadius=0。浅／深色均可通过应用追加规则覆盖。
- 普通 Popover 默认无阴影；不顺便改变控件 padding、行高或布局默认值。

第 5 步实现：

- 新增 `theme/modern`，提供上述 Options、Sheet、Rules、Primary 和 MutedText；
  依赖仅为 style、基础几何和标准库，不引用 GUI 默认规则、不读取 Settings。
  Rules 独立分配，颜色在生成时复制／混合／量化为自有值，应用追加规则可覆盖或关闭阴影。
- 主题色反预乘后四舍五入到 8 位不透明种子。普通焦点色沿主题色向正文色调整，
  逐步检验与窗口、输入表面、普通按钮各状态背景的对比；Primary 使用独立实底配色。
  普通正文与次要正文验证 4.5:1；Primary 键盘焦点验证模式对应的固定灰色字节，
  指针焦点验证与普通 Button 默认规则一致，
  其他控件原有焦点验证 3:1；禁用文字不是正文对比目标的对象。
- 用户体验反馈后，GUI Button／MenuButton 的焦点绘制入口、重绘监听和 Modern
  `focus` part 规则保留，仅将 GUI 兜底及 Modern 默认焦点边框设为 BorderWidth(0)。
  应用追加 focus/Focused 规则即可启用，不需要修改控件。此前误删的绘制实现已用
  Git 定向恢复。MenuButton 保留 StyleName 的读取，TextInput 的焦点提示不变。
- 默认 Tab 导航接入后，新增 `focus` Part/FocusVisible。按后续确认方案，取消
  初版中性遮罩，改为内绘圆角边框，背景透明；根据可辨性反馈宽度从 1 DIP
  调整为 2 DIP。边框是不透明的 75% 既有
  主题焦点色 + 25% 中性边框色，亮暗使用各自 palette，不统一加白。普通 Focused
  仍保持零边框，TextInput 既有 Focused 边框不叠加。FocusVisible 继承未覆盖的
  同 Part/Focused 字段，应用可直接追加 FocusVisible 规则改变/关闭键盘边框；
  不用通用控件边框检测或新 API。Hover/Pressed 与子 Label 独立。见
  DesignKeyboardNavigation §7.3；此前底色版与 1 DIP 版验证分别见 §7.1/7.2，不能当作新版验收。
- HeaderBar 默认最小高度从 48 DIP 调为 40 DIP，保留 8 DIP padding 和内容驱动测量；
  Linux 无 HeaderBar 的控制按钮兜底放置高度同步为 40 DIP。Linux 控制图标跨度
  从 10 DIP 缩为 8 DIP，保持 1 DIP 线宽、24 DIP 圆形背景和 28 DIP 点击区域。
  控制按钮组右侧外边距从 12 DIP 收紧为 6 DIP，可见圆形距右缘为 8 DIP，
  与默认 40 DIP HeaderBar 内的上下可见留白一致；按钮间距保持 8 DIP。
- GUI 兜底配色、窗口装饰、平台和 Painter 未修改。菜单和普通 Popover 的规则已提供，
  但完整外壳绘制以及菜单行文字的消费仍是第 6 步，不在本切片宣称菜单外观已完成。
- 新增 `example/modern`：系统／浅色／深色／GUI 兜底切换，系统／紫／绿／白／黑主题色，
  系统字体、Primary、次要文字、文本输入和滚动内容；HeaderBar 使用 Integrated 信号协作。
  系统跟随通过 build 读取 Settings 表达，手动变更显式 RequestUpdate。

使用方式见 `theme/modern/README.md`。运行 `go run ./example/modern`，建议分别检查
1x／1.5x／2x 下的悬停／按下、主题切换、窗口边缘和滚动条胶囊。
初版未实现 Tab 焦点遍历；当前已接入 Tab／Shift+Tab 及独立 FocusVisible，
示例操作提示与后续键盘导航设计保持一致。

第 5 步已通过 `go test ./...`；250 组亮暗／主题色采样验证量化后的对比度，另有
选项归一化、颜色和切片所有权、完整规则、应用覆盖、按钮焦点不改变外观及 GUI 兜底回归。
Windows amd64／macOS arm64 的 GUI 和 Modern 测试目标交叉编译通过。
尚未完成三平台真机外观验收；PopoverMenu 样式消费见下节。

## 7. PopoverMenu 接入

第 6 步按专门设计完成，视觉规则以本文件和 Modern 为准：

- Popover.StyleName、bodyRect、shadowInsets、contentClip；阴影不改变主体锚点。
- 透明表面上依次绘制阴影、背景、裁剪内容和边框；不增加 GUI Painter 能力。
- 菜单内容只绘制项目，移除重复外壳；行解析 `menu-item`，内部 Label 独立解析
  `menu-item-text` 或 `menu-item-text-disabled`，不继承行的样式，排版资源归 Label。
- 接入 StyleChanged，覆盖菜单自然尺寸、虚拟列表尺寸和文字资源。
- 阴影／圆角外侧点击通过统一 DispatchEvent 发出 DismissRequest，不实现鼠标穿透。
- 透明能力不可用时由 PopoverMenu 降级为无外阴影的矩形；普通 Popover 保留显式错误。
- macOS Popup 关闭系统阴影，避免重复；不影响普通 Window 原生阴影。

Modern 规则显式启用；完整、独立的 Basic 兜底已覆盖 GUI 与 widgets，不要求应用
拼接规则，也不把 Modern 配色塞进 GUI 内核。布局、定位、关闭及资源归属是 GUI 的职责。

第 6 步已实现。`example/modern` 增加普通菜单（含禁用项、分隔线）及长菜单入口。
本机验证涵盖阴影占位、主体定位、同步／异步尺寸事件、透明失败重试、关闭与卸载、
模型替换、隐藏项、列表语义角色及稳定帧文字复用。Software 在 1x／1.25x／1.5x／2x
验证彩色阴影、透明角、安全裁剪和边框；Pango 验证 Label 独立样式的颜色、字号切换，
并确认行的悬停、按下状态不改变文字资源。
全仓库测试通过；Windows amd64、macOS arm64 GUI 及 macOS 平台测试目标交叉编译通过。
三平台真实菜单定位、圆角／阴影、首尾项目悬停和长菜单滚动仍待运行验收。

## 8. 验证与进度

2026-10-09 通用 StyleName 扩展：Card、Sidebar、Flat、Warning、Error、Success。
具体规则、消费边界、demo 展示与本轮验证见 [Modern 通用预设](DesignModernPresets.md)。
不增加样式继承、容器绘制策略或平台接口。

- [x] 第 1 步代码：阴影规则覆盖／关闭／比较；三后端阴影颜色适配，含清晰与降级路径。
- [x] 第 1 步本机验证：Software 与真实 GLX/FBO 彩色像素测试，1x／1.25x／1.5x／2x，单层与叠加。
- [x] 第 1 步编译：Windows amd64 Direct2D／OpenGL 测试、macOS arm64 平台测试目标。
- [ ] 第 1 步跨平台运行验收：Windows 原生像素测试及 Windows／macOS 样例目测。
- [x] 第 2 步：通知先于测量／绘制；稳定帧不重复通知或创建资源；挂载、隐藏、销毁、装饰树、Popover、ListView。
- [x] 第 2 步缓存验证：字体／颜色变化使 TextLayout 失效；Software 实际像素换色且稳定帧仅栅格化一次；列表局部换样式重新测量、不整体 reload。
- [x] 第 3 步：应用样式不改变装饰几何和窗口按钮；Integrated 安全裁剪、缩放区域及亮暗切换像素回归。
- [ ] 第 3 步真机验收：Windows／macOS／Linux 装饰、控制按钮和窗口交互目测。
- [x] 第 4 步：首份快照、更新合并、连接清理、过期任务失效、相同样式去重。
- [ ] 第 4 步真机验收：运行中切换系统亮暗／字体／主题色，确认自动更新及无关设置不重复换肤。
- [x] 第 5 步：完整规则、极端主题色、亮暗切换、字体参数、显式启用和 GUI 兜底；按钮鼠标焦点无额外装饰，键盘焦点为独立 2 DIP 边框，保留样式覆盖能力。
- [ ] 第 5 步真机验收：Modern 示例三平台亮暗／主题色切换、系统字体、焦点和多倍缩放外观。
- [x] 第 6 步代码及本机测试：外壳、定位、内容、输入、降级及生命周期，见 PopoverMenu 专门设计。
- [ ] 第 6 步真机验收：三平台不同 DPI、亮暗外观、长菜单滚动及原生阴影不重复。

每个代码切片运行 `go test ./...`，有平台变更时交叉编译 Windows／macOS 测试目标。
三平台分别验证 1x、1.5x、2x；编译、自动测试和真机验收分别记录，不相互替代。

第 1 步已执行 `go test ./...`。Linux 原生像素测试使用：

```sh
GOUI_TEST_OPENGL=1 go test ./platform/graphics/opengl -run TestBoxShadow -count=1 -v
```

Windows 可运行 `go test ./platform/graphics/direct2d ./platform/graphics/opengl -run TestBoxShadow -count=1 -v`。
`platform/graphics/test` 保留既有阴影示例，其彩色阴影改用 ColorOf(NRGBA) 明确预乘转换；
未修改模糊、偏移、spread 等体验参数。普通 Brush／Clear 的颜色约定梳理仍是后续独立工作。
