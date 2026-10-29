# Label 格式化文本与可选选择

## 1. 范围与进度

Label 是小段展示文本。格式化不需要新增控件类型、格式模式 setter 或 UI 构建入口。
`NewLabel`、`Text/SetText`、`ui.Label`、`BindText` 保留，由文案开头的 `\{format\}`
决定是否解释格式。

本轮实现格式字符串、命名文字样式、嵌套范围、主题失效、纯文本语义和验证程序。
**可选择文本尚未实现**：Selectable、选区、复制、菜单、焦点及相关 UI 绑定均留待后续。
TextInput/TextView 的正文和 Placeholder 始终是纯文本，包括只读状态。
窗口标题、ID、Name 等非 Label 内容不解释格式。首版不包含链接、图片、内嵌 Widget、
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

## 3. 独立 textformat 包

根目录的 textformat 不依赖 gui、platform、style 或 mlang，仅处理字符串和名称范围。

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
func EscapeArg(template, value string) string
func Literal(value string) string
```

Run 是纯文本的 UTF-8 半开范围，Styles 从外到内；范围不重叠，相邻同路径段合并。
结果属于调用方，无原生资源或全局可变缓存。没有开启标记时 Parse 不分配范围、不改文字。
解析失败返回错误，不在纯包里选择 GUI 降级策略。

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
菜单项、Tooltip、标签页等对显示文案直接建立语义字段时，也需使用纯文本；业务模型
和源字符串 getter 不改。Snapshot 不创建 TextLayout 或其他原生资源。

## 7. 后续可选择文本（本轮不实现）

计划提供 Selectable、Selection、SelectedText、ConnectSelection 和对应 UI 绑定。
默认不可选择，选择坐标是 PlainText 的 UTF-8 范围，按实际 Cluster 校正。
未来控制器复用事件分发、手势竞争、Unicode 分词、剪贴板和现有菜单，不创建 TextModel、
编辑历史或 IME。失焦保留选区，Unmount 释放窗口相关交互，保留 Widget 自身状态。
不新增 EventDispatcher/Root/BuildContext 的 Label 特例。

## 8. 测试与实施

包内测试覆盖语法/错误位置/转义/空白/UTF-8/嵌套深度、样式字段存在性与取消、作用域、
混合字号和 baseline、未知样式、主题失效/尺寸复用/卸载、UI BindText 及纯文本语义。
Software 原生排版像素比较用同后端独立构造的 TextLayout 作参考，不跨字体金图，
采样颜色按预乘 RGBA 解释，不放宽容差掩盖失败。

窗口例程放 tests/window/gui/label 和 tests/window/ui/label，中文入口说明环境、操作、
预期、副作用与退出。GUI 例程覆盖 Basic/Modern 亮暗、嵌套、字体/大小、装饰、换行、
格式参数、出错降级、主题和文本切换、TextInput/TextView 的原样显示；UI 例程验证现有
Text/BindText 和 mlang 路径，无 DevServer。支持自动断言和保留窗口截图的人工检查。

阶段：解析与独立测试 → 样式和 Label 及语义 → UI/窗口例程 → 本机独立 X11 全仓/race/
Software+OpenGL 1x/2x → Windows Direct2D/OpenGL/Software 与 macOS OpenGL/Software
的 1x/2x 原生验证。交叉编译不代替运行，真实窗口不等于真实输入法验收。
本轮无选择功能，不以复制、手势或编辑器动作作为验收项目。
