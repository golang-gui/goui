# Label 格式化文本与可选选择

## 1. 范围与进度

Label 是小段展示文本。格式化不需要新增控件类型、格式模式 setter 或 UI 构建入口。
`NewLabel`、`Text/SetText`、`ui.Label`、`BindText` 保留，由文案开头的 `\{format\}`
决定是否解释格式。

已实现格式字符串、命名文字样式、嵌套范围、主题失效、纯文本语义及可选的单 Label 选择。
可选择切片复用原生 Cluster 与已有 GUI 交互机制，不接入编辑模型。
TextInput/TextView 的正文和 Placeholder 始终是纯文本，包括只读状态。
Tooltip、窗口标题、ID、Name 等纯文本入口不解释格式。首版不包含链接、图片、内嵌 Widget、
跨控件选择、富文本编辑或 HTML/Markdown。

参考证据：Pango 1.58.2 文档说明将格式与整句翻译一起保存可以避免维护字符下标；
GTK 4.24.1 的 Label 与 Qt 6.12 QLabel 允许可选选择。采用范围随文案移动和选择独立
开启的思想，不采用 Pango XML、Qt 的 HTML 自动检测，也不依赖 Swift 的编译器插值。

- https://docs.gtk.org/Pango/pango_markup.html
- https://docs.gtk.org/gtk4/method.Label.set_selectable.html
- https://doc.qt.io/qt-6/qlabel.html

## 2. 字符串契约

普通文本完全原样显示，只有字符串字节起始处准确匹配 `\{format\}` 才解析后续正文。
不跳过空白或 BOM，不识别出现在后面的开启标记，不保留跨 SetText 的模式状态。

```go
`普通 <文字> & C:\Users\Example`
`\{format\}请先\{bold:保存修改\}，再退出。`
`\{format\}\{warning:请保存 \{bold:所有修改\}\}。`
`\{format\}\{code:{"name":"GOUI"}\}`
```

格式模式只识别以下结构：

| 内容 | 含义 |
| --- | --- |
| `\{styleName:` | 开始一个命名样式范围 |
| `\}` | 结束当前范围 |
| `\\` | 一个字面反斜杠 |
| 其余字符 | 原样复制 |

正文不 Trim；空格、换行、后续冒号和普通 `{}`、`<>`、`&` 保留。
不另行解释 `\n`/`\t`；Go 双引号字面量本身的转义仍由 Go 处理，推荐使用原始字符串。
名称非空、区分大小写，不含冒号、反斜杠、花括号或换行；与样式名精确匹配，不修剪。
每层只有一个名称，组合使用嵌套，空范围合法。迭代栈解析，最大深度 32。
示例中的 code 是应用自定义样式名，Basic 不预设它；未定义时保留基础格式。

缺少冒号、非法名称、不配对的结构结束符、未闭合范围和超过深度限制均返回带源 UTF-8
字节位置的错误。未知样式名不是语法错误：该层没有覆盖值，保留正文和外层格式。

`Label.Text()` 返回原始源字符串，新增 `PlainText()` 返回实际显示文字。纯文本坐标
与源字符串坐标分开；排版、baseline 和 Snapshot 均使用纯文本，未来选择命中也以其为准。
语法错误时 Label 确定性显示完整原始源串（包含前缀），不显示部分解析结果、不丢字。
SetText 不改变签名，不新增错误信号、日志或默认错误弹窗。应用可单独 Parse 提前校验；
Snapshot 的 `goui.label` 属性报告格式状态和可选错误。

## 3. gui/textformat 子包

gui/textformat 是 GUI 展示文字的纯解析子包，不依赖 gui 本包、platform、style 或
mlang，资源包可单独导入它而不加载 GUI 运行时。它不属于 core 的通用基础类型，也不
占用根目录的独立公共模块。解析和转义入口保持同一个包，导入路径为
`github.com/golang-gui/goui/gui/textformat`。

```go
type Run struct {
    Start, End int
    Styles []string
}
type Text struct {
    Plain string
    Runs []Run
    Formatted bool
}
func Parse(source string) (Text, error)
func PlainText(source string) string
func EscapeArg(template, value string) string
func Literal(value string) string
```

Run 是纯文本的 UTF-8 半开范围，Styles 从外到内；范围不重叠，相邻同路径段合并。
结果属于调用方，无原生资源或全局可变缓存。没有开启标记时 Parse 不分配范围、不改文字。
Parse 解析失败返回错误；PlainText 是显式的显示文字提取入口，错误时返回完整源串，
与 Label 的降级保持一致。它只用于明确承载格式源码、但没有 Label 的元数据，例如
TabPage 的标题，在构造／setter 中调用；已有 Label 应读取其 PlainText() 缓存，
不在 Snapshot 中再次解析。

EscapeArg 对未开启格式的模板原样返回参数；开启格式时转义参数中的反斜杠，供 `%s` /
`%[n]s` 插值，不重新实现 fmt。Literal 返回保证 Label 显示原始 value 的源字符串，
用于显示任意外部数据或语法示例。它与参数转义是两种不同上下文。

```go
var deletePrompt = mlang.String(
    mlang.En(`\{format\}Delete \{bold:%[1]s\}?`),
    mlang.ZhCN(`\{format\}确定删除 \{bold:%[1]s\} 吗？`),
)
func DeletePrompt(name string) string {
    template := deletePrompt()
    return fmt.Sprintf(template, textformat.EscapeArg(template, name))
}
```

mlang 保持字符串资源和现有语言通知；不新增注册表或翻译格式类型。样式名和占位符
是结构标记，翻译不更名，可以调整整个范围的位置。裸 fmt.Sprintf 不自动获得参数隔离，
Label 无法从最终 string 判断参数来源。

## 4. 样式与字号

复用当前 StyleSheet/StyleScope/State，不创建富文本样式注册表。新增 FontWeight、
FontStyle、Underline、Strikethrough 的可选字段、getter 和 Rule builder；显式 Normal /
false 可取消外层格式，未设置则保留。style 不导入 platform，GUI 转换对应排版枚举。

每个 Label 的文字格式按当前作用域内的 label 基础样式、Label 自身 StyleName、各范围
外到内的次序逐字段组合。这是同一控件内的默认值和局部覆盖，不给子 Widget 传递样式，
不改 StyleSheet 的 selector/fallback 规则。

Basic 提供最小 bold/italic/underline/strikethrough 规则，Modern 复用，可由应用覆盖。
bold 只设置字重，italic 只设置斜体。Modern 的 warning/error/success/muted-text 等
文字颜色预设仅覆盖颜色，默认字体来自 Label 基础格式。二者必须同步修改，保证原有
`Label.Style("warning")` 的默认字体不变。

基础字号 24 pt 时，warning 和 bold 不改变字号；明确设为 12 pt 的命名样式则覆盖为
12 pt。不引入 em、百分比或相对字号。范围只使用字体、字号、字重、斜体、前景色和
文字装饰；背景、边框、圆角、阴影不参与行内绘制。

调用 TextLayout.SetTextFont 前先形成完整 FontInfo，避免零字段重置范围字体。
TextInput/TextView 不解释标记；共用文字格式构造可以读取新增字重与字形，但不增加
编辑控件的范围格式 API。本轮不改变平台绑定、图形后端或 TextLayout 公共接口。

## 5. 排版、缓存与生命周期

每个 Label 只拥有一个 TextLayout。先解析纯文本和范围，再创建基础 layout 并应用
完整范围字体、颜色和装饰，最后 Measure/MeasureMetrics/Paint；不拆成多个布局。
格式化空字符串与普通空 Label 的测量和 HasBaseline 行为一致。

- 源字符串变化才重新解析；同源 SetText 无操作。
- StyleChanged 释放排版，保留解析结果及名称；主题切换不重复解析。
- 尺寸变化复用 SetSize，重新计算 native 行和 Cluster，不缓存旧 baseline。
- Unmount 释放 TextLayout，保留 Widget 自身的源文字和解析值；Destroy 最终兜底。
- 不增加全局缓存；Measure 与 Paint 继续共享对象。

## 6. UI 与语义

`ui.Label(resource.Message())`、Text、BindText 保留，无格式开关或协调器特例。
Label Snapshot 的 Text 为显示纯文本、Role 为 Label；用 `goui.label` 附带 Formatted 和
可选 FormatError，普通 Label 不必创建 Attributes，不使用 TextEditingInfo。
Snapshot.Text 的含义是用于理解界面的显示语义，不是所有输入字符串的属性转储。
菜单项、表头、标签按钮等持有 Label 时直接使用 Label.PlainText；没有 Label 的
TabPage 标题明确承载 Label 源码，构造／SetTitle 时使用 gui/textformat.PlainText 提取并保存，
Snapshot 只读取结果。
业务模型和源字符串 getter 不改，Snapshot 消费方不再解释格式，且不创建原生资源。

Tooltip 保留既有纯文本契约：WidgetInfo.Tooltip 原样返回；内部 Label 通过 Literal
保护字面值，tooltipContent 从实际 Label 读取文字。TextInput/TextView 展示的格式源码
仍是文档内容，不因 Snapshot 使用纯文本语义就删除其中标记。

DropDown 默认展示 Label、占位 Label 直接读取 PlainText。自定义 Delegate 的内容
提供非空 Snapshot.Text 时使用它；未提供则原样使用模型 Text 作为语义名称，不假设
自定义 Widget 使用了 Label 语法。收起内容与列表内容各自负责自己的语义。
不新增通用 Widget 文本接口，也不让 WidgetBase/Snapshot 消费方承担格式解析。

原文与显示文本分离也参考 GTK4 4.23.4 的
[get_text](https://docs.gtk.org/gtk4/method.Label.get_text.html) 和
[get_label](https://docs.gtk.org/gtk4/method.Label.get_label.html) 契约；它不代表采用 GTK
的 XML 语法。GOUI 的控件专属 Attributes 可报告格式状态和错误，Text 的语义保持统一。

## 7. 可选择文本

默认 Selectable=false。Label 自己管理选区，复用同一个 TextLayout、textedit.Geometry、
手势竞争、ContextMenuEventController、PopoverMenu 与系统剪贴板。不创建 TextModel、
撤销历史或 IME，不向 EventDispatcher/Root/BuildContext 加 Label 分支。

```go
func (l *Label) Selectable() bool
func (l *Label) SetSelectable(bool)
func (l *Label) Selection() TextSelection
func (l *Label) SetSelection(TextSelection)
func (l *Label) SelectedText() string
func (l *Label) SelectAll()
func (l *Label) Copy() error
func (l *Label) ConnectSelection(func(TextSelection)) signal.Handle
func (l *Label) ConnectCopyError(func(error)) signal.Handle
```

Selection 的 Anchor/Caret 是 PlainText UTF-8 字节位置，保留方向，范围半开；超出文本
时钳制。已有实际分配与排版时按 shaping Cluster 校正，未排版时仅保证 UTF-8 边界。
字体变化后的边界在下次输入或 Copy 前校正，Measure/Paint 不发送选区通知。
SelectedText 是无副作用的逻辑切片，不创建原生资源；Copy 先校正，再写纯文本。
软换行不产生新字符，源文本里的硬换行保留，不复制格式语法。

启用时提供 Focusable=true 与 CursorText 默认值；调用者可覆盖。关闭时清空选区并
移除控制器；默认值未改变时恢复启用前的值，不覆盖期间明确设置的不同值。
选择状态与信号按需分配，UI 普通 Label 也不建立选择信号连接。信号连接在开关和
重新挂载之间保留，控件最终销毁后不再调用观察者。外部回调改变正文、选区、启用状态、
挂载或销毁对象时，停止后续陈旧输入动作与通知。

交互：单击定位，Shift 单击延伸，拖动捕获直到边界外，双击 Unicode 单词并向外对齐
Cluster；箭头沿视觉字形和视觉行导航，Home/End 定位行端，Primary+Home/End 定位
全文首尾，Shift 扩展；Primary+A/C 全选/复制，Escape 取消拖选并折叠选区。
其余按键继续传播，Label 不处理输入、删除、剪切和粘贴。
可选 Label 主动认领选择手势，因此在 HeaderBar、Button 或 DragSource 内也优先选择；
普通 Label 的命中/拖动行为保持不变。Tab 可以到达可选 Label，失焦保留选区。

右键或 Shift+F10/Menu 打开 Copy/SelectAll；右键不移动已有选区。菜单使用框架已有
本地化资源。Copy 直接调用只返回错误；内置输入操作失败时单次发出 CopyError，
不重复报告、不弹默认对话框；菜单呈现为尽力而为。
卸载、隐藏、禁用取消活动选择和菜单；卸载销毁原生菜单与排版，但保留选区数据。
相同 source SetText 无操作；PlainText 相同的格式变更保留选区，正文变化清空选区。

样式：label 的 selection Part 用 BackgroundColor，Focused 是活动背景、Normal 是
较弱的失焦背景；caret Part 的 ForegroundColor 用于空选区的静态 1 DIP 位置线，
不引入闪烁计时器。先画背景再画原有文字，保留行内颜色/字重/装饰，不提供选中文字
前景覆盖，不因拖动重复 SetTextColor 或失效文字位图。控件自己的 StyleName 可以
覆盖相应 Part，未提供的颜色沿用 label 基础值。

UI：Label(...).Selectable(true).Selection(...) / BindSelection(&state)、OnSelection、
OnCopyError。默认自身管理选区；省略 Selection 不复位。绑定使用通用 BindValue 与
既有显式更新，不自动追踪；声明式应用与归一不回写通知。一次属性更新会按声明顺序
应用此节点的所有绑定，正文改变后，若声明了选区绑定，继续应用它并按新正文钳制。
Focusable/Cursor 共享修饰器可以显式覆盖选择默认值，不改 UI 公共协调流程。

Snapshot 仍是 RoleLabel、Text=PlainText；goui.label 增加 Selectable、Selection，
值拷贝脱离控件，非编辑器，不使用 TextEditingInfo 或通用条目 Selected 状态。
自动化仍经过正常 DispatchEvent，未增加语义动作捷径。

参考：GTK 4.24.1 [Label](https://docs.gtk.org/gtk4/class.Label.html) 的可选选择、键盘和
selection 样式节点；Qt 6.12 [QLabel](https://doc.qt.io/qt-6/qlabel.html#textInteractionFlags-prop)
的选择可选项；Avalonia 11.3.10
[SelectableTextBlock 实现](https://github.com/AvaloniaUI/Avalonia/blob/11.3.10/src/Avalonia.Controls/SelectableTextBlock.cs)
的独立选区状态与背景。采用单控件选择和独立背景绘制，不复制富文本编辑器结构。

首版不包含跨 Label 选择、触摸手柄、选中文字拖出、边缘自动滚动、Linux PRIMARY
selection、富文本剪贴板或三击段落。它们不是本轮验收承诺。

## 8. 测试与实施

包内测试覆盖语法/错误位置/转义/空白/UTF-8/嵌套深度、样式字段存在性与取消、作用域、
混合字号和 baseline、未知样式、主题失效/尺寸复用/卸载、UI BindText 及纯文本语义。
Software 原生排版像素比较用同后端独立构造的 TextLayout 作参考，不跨字体金图，
采样颜色按预乘 RGBA 解释，不放宽容差掩盖失败。

窗口例程放 tests/window/gui/label 和 tests/window/ui/label，中文入口说明环境、操作、
预期、副作用与退出。GUI 例程覆盖 Basic/Modern 亮暗、嵌套、字体/大小、装饰、换行、
格式参数、出错降级、主题和文本切换、TextInput/TextView 的原样显示；UI 例程验证现有
Text/BindText 和 mlang 路径，无 DevServer。支持自动断言和保留窗口截图的人工检查。
两个例程的根容器不设置 Padding，HeaderBar 位于内容区顶部；正文用独立容器留白。
自动检查 HeaderBar 贴顶，以及自绘窗口按钮与它的垂直中心一致，防止示例误导用法。

阶段：解析与独立测试 → 样式和 Label 及语义 → UI/窗口例程 → 本机独立 X11 全仓/race/
Software+OpenGL 1x/2x → Windows Direct2D/OpenGL/Software 与 macOS OpenGL/Software
的 1x/2x 原生验证。交叉编译不代替运行，真实窗口不等于真实输入法验收。
可选择窗口例程 tests/window/ui/label_selection 另外覆盖指针/键盘、Cluster、复制、菜单、
失焦、开关、主题与 UI 绑定；
不把已有格式化例程的通过当作选择功能验收。
