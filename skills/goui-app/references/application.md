# 应用开发参考

源码核对基线见 [SKILL.md](../SKILL.md#先确认依赖再选择接口)。开发已有应用时，
始终先核对其选中版本，不能因示例较新就升级依赖。下面的源码路径相对于选中版本的
GOUI 模块目录，不是相对于安装后的 Skill；安装本技能不要求存在整个框架仓库。

## 查找接口与示例

```sh
go list -m -json github.com/golang-gui/goui
go doc github.com/golang-gui/goui/ui.App
go doc github.com/golang-gui/goui/ui.TextInput
go doc github.com/golang-gui/goui/widgets/ui
```

根据 Dir／Replace 找到实际源码。新项目没有该模块时，先初始化应用模块并选定
已存在的版本，不能将“module not found”误判为 API 不支持。
模块缓存中的源码只读；不要直接编辑缓存实现应用功能。

| 需要的能力 | 优先查看的包／源码或示例 |
| --- | --- |
| 入口、窗口、更新与生命周期 | ui/app.go、ui/window.go、gui/application.go、gui/window.go |
| 线性／换行布局、滚动、覆盖层 | ui/box.go、ui/wrap.go、ui/scroll_view.go、ui/overlay.go、layout |
| 文本编辑、选区与快捷键 | ui/text_input.go、ui/text_view.go、ui/shortcut.go |
| 列表、树、表格、SplitView、标签页 | ui/list_view.go、widgets/ui、对应 widgets 实现和 tests/window/widgets |
| 设置行与成组设置页 | widgets/ui/list_row.go、widgets/ui/list_row_group.go、widgets/list_row.go；普通行组不提供虚拟列表行为 |
| 菜单栏、子菜单、命令与勾选 | ui/menu.go、ui/command.go、ui/shortcut.go、example/demo |
| 滚动条选项、外部滚动条、视口几何 | ui/scroll_view.go、ui/scroll_bar.go、gui/scroll_bar.go、gui/scroll_view.go |
| 样式、亮暗和语义预设 | theme/modern/README.md、theme/basic、style、example/modern |
| SVG、图标、翻译和动画 | icon、mlang/README.md、animation、example/demo |
| 文件访问、外部打开、拖放 | ui/dialog.go、ui/app.go、ui/dragdrop.go、tests/window/ui |
| 完成度优先的项目示例 | example/monitor（自绘图表、表格）、example/chat（列表、气泡、HeaderBar）、example/music（帧动画、自定义输入控件） |

GOUI 的 [README](https://github.com/golang-gui/goui) 和
[Demo](https://github.com/golang-gui/goui/tree/main/example/demo) 可作入口；
在线分支内容与依赖不同的时候，以该版本的源码和示例为准。

## 声明式入口

以下是完整 main.go，状态创建在 build 之外；输入修改和按钮操作都会更新界面：

```go
package main

import (
    "fmt"
    "log"

    "github.com/golang-gui/goui/theme/modern"
    "github.com/golang-gui/goui/ui"
)

func main() {
    name := ui.MakeState("GOUI")
    count := ui.MakeState(0)
    err := ui.Run("com.example.Hello", func(app ui.App) ui.RootView {
        return ui.Root().
            StyleSheet(modern.FromSettings(app.Settings())).
            Windows(
                ui.Window("main").Title("Hello GOUI").
                    Size(480, 280).Center(true).
                    Content(
                        ui.VBox(
                            ui.TextInput().ID("name").BindText(&name),
                            // Get inside Build records the dependency automatically.
                            ui.Build(func() ui.View {
                                return ui.Label(fmt.Sprintf("Hello, %s", name.Get()))
                            }),
                            ui.Build(func() ui.View {
                                return ui.Label(fmt.Sprintf("Clicks: %d", count.Get()))
                            }),
                            ui.Button("Click me").OnClick(func() {
                                count.Update(func(old int) int { return old + 1 })
                            }),
                        ).Spacing(12).Padding(20),
                    ),
            )
    })
    if err != nil {
        log.Fatal(err)
    }
}
```

App ID 与窗口 ID 不同。App ID 是稳定的应用身份，配置打包时与 goui.toml 的 id
保持一致；窗口 ID 用于协调顶层窗口。ui.Run 从 main 调用，一次进程运行一次。
框架的公共 platform 入口已保留启动线程，不需要在应用里重复锁线程。

View 每次 build 可以新建，底层 Widget 由协调保留；不要缓存一份 View 然后把它
当作 Widget 修改。持久状态放在调用方持有的对象中，或在自定义 WidgetView 的
生命周期里通过 BuildContext.State／SetState 管理。列表 ID 要稳定，不能使用
每次随机生成的 ID 或随排序变化的显示序号来表示业务身份。

ComposeView.Build 的同步执行范围内（包括 ui.Build(fn) 及其调用的辅助方法），
State.Get 和 ui.Read(source) 自动成为依赖，状态变化只重建这个局部；该范围外的读取
只是快照。简单文本用 ui.Label().BindText(&text)，打字只更新属性，不重建所在构建。
Observe 只用于 Build 内没有读取、但变化时需要重建的外部源。

- 按读取是否需要形成构建依赖选择 Get／Peek，不按函数、方法或回调的写法判断。
  例如 Build 中调用 vm.DisplayName()，该方法内读取的视图状态仍应使用 Get；
  改成 Peek 会丢失依赖，导致名称或对话框等内容不随状态更新。Peek 用于明确不希望
  订阅的读取；构建范围外 Get／Peek 都是快照，事件回调无需统一改用 Peek。
- 根构建（ui.Run 的 build 函数）不追踪读取。窗口集合、标题、样式表和普通 Go 字段
  变化调用 app.RequestUpdate；语言和系统设置变化由 ui.Run 自动重建。
- State 内含锁，创建后地址稳定、只按指针传递，`go vet` 的 copylocks 会报告复制。
  Set 值相同也会通知，频繁写入时先用 Peek 比较。不用 State.Connect 串联副作用，
  派生值在方法里或构建时计算。
- 不需要驱动界面的值用普通字段，例如分割条位置只在下次声明时恢复；
  BuildContext.State 仅保存自定义 View 的私有状态，不收集依赖。
- ListView、TreeView、TableView 的行构建在控件绑定行时调用，不在 Build 内，
  不追踪读取：行构建读取外层 Build 取得的快照，外层 Build 读 State，变化时整表刷新；
  行内需要独立更新的部分再包一层 ui.Build。模型对象保持同一指针，才能保留展开、
  选择和当前行。

build 中读取状态，回调中改变状态；不在 build 中持续 Set／RequestUpdate 制造循环。
后台读取文件或网络时，通过 app.Post 提交结果。每次请求递增代数，回调时代数不一致
就丢弃，避免取消、重发或已关闭窗口被旧结果更新；高速流式结果先合并，保证同一时刻
最多一个 Post 在途。App 可跨 goroutine 调度，不代表 Widget 可并发访问。

## 命令式入口

已有 gui 应用不必改成声明式。以下也是完整 main.go：

```go
package main

import (
    "fmt"
    "log"

    "github.com/golang-gui/goui/core/geometry"
    "github.com/golang-gui/goui/gui"
    "github.com/golang-gui/goui/layout"
    "github.com/golang-gui/goui/theme/modern"
)

func main() {
    if err := run(); err != nil {
        log.Fatal(err)
    }
}

func run() error {
    app, err := gui.NewApplication("com.example.Hello")
    if err != nil {
        return err
    }
    app.SetStyleSheet(modern.FromSettings(app.Settings()))
    settingsHandle := app.Settings().ConnectChanged(func() {
        app.SetStyleSheet(modern.FromSettings(app.Settings()))
    })
    defer settingsHandle.Disconnect()
    window, err := app.NewWindow(&gui.WindowOptions{
        Size: geometry.Size{Width: 480, Height: 240},
    })
    if err != nil {
        return err
    }
    defer window.Destroy()
    if err := window.SetTitle("Hello GOUI"); err != nil {
        return err
    }
    box := gui.NewLinearBox(layout.DirectionVertical)
    box.SetPadding(geometry.UniformInsets(20))
    box.SetSpacing(12)
    label := gui.NewLabel("Clicks: 0")
    button := gui.NewButton()
    button.SetChild(gui.NewLabel("Click me"))
    count := 0
    clickHandle := button.ConnectClicked(func() {
        count++
        label.SetText(fmt.Sprintf("Clicks: %d", count))
    })
    defer clickHandle.Disconnect()
    box.AddChild(label)
    box.AddChild(button)
    window.SetWidget(box)
    if err := window.Show(); err != nil {
        return err
    }
    return app.Run()
}
```

gui 信号返回可断开的 Handle；自行连接的长期外部信号需要由所有者清理。
ui 的 OnXXX 连接由协调层管理，不要另在 build 中重复 Connect。
现代主题的 FromSettings 是当次快照：ui.Run 自动响应系统设置变化；命令式 gui
应用如需跟随，显式连接 Settings.ConnectChanged，如上例。

## 布局和窗口

- HBox／VBox 是线性盒，不是 CSS flex。MainWeight 分配线性父容器主轴剩余空间；
  CrossAlign 管交叉轴。默认 HBox 交叉轴居中，VBox 起始对齐；横向文字基线对齐
  使用 layout.CrossBaseline；铺满交叉轴使用 layout.CrossStretch。
- Padding／Spacing 是控件的布局属性，不是样式字段。MinSize／MaxSize 是偏好，
  父约束仍优先；不要用无限宽或极大固定尺寸逼容器填满。
- 需要纵向滚动时使用 ScrollView(...).Direction(ui.ScrollVertical)。ScrollView
  默认 MainWeight 为 1；文本／列表实际滚动能力仍由内容提供。检查小窗口、长文本、
  嵌套滚动及横向换行，不只在初始窗口尺寸下观察。
- HBox 主轴上没有 MainWeight 的子项不受可用宽度限制。需要“最大宽度、窗口变窄时
  收缩换行”的内容（如聊天气泡）放进 VBox，用交叉轴对齐（CrossStart／CrossEnd）；
  交叉轴以可用宽度为上限，否则窄窗口下内容会按 MaxWidth 排版并被裁切。
- 相同 MainWeight 不保证等宽：有剩余空间时，按各项自然宽度加上权重分得的份额。
  相同 MinWidth 只有在它足以统一各项测量基准、且没有其他尺寸限制时才有帮助；
  例如自然宽度 200／300、MinWidth 都是 100、权重相同，在 800 宽度下仍分为 350／450。
  需要严格等分时应按可用宽度统一分配，不把 MinWidth 当作可覆盖自然宽度的基准属性，
  并检查窄窗口、长文案及换行。
- Integrated 不自动插入 HeaderBar。使用 ui.HeaderBar(child) 放自己的内容，
  由 GUI 处理原生控件避让和常见拖动命中；不手工为 Linux 留一套猜测边框偏移。
  Captions 显式指定命中的 Widget ID，不能拿按钮区域当普通空白拖拽区。
  HeaderBar 会把唯一子项横向拉满并按自然高度居中，固定尺寸的标志需要外包一层
  盒子并设置 MainAlign 才能居中；只需要拖动区时，可放一个空的 HeaderBar 占位。
  它仍代表标题行并影响窗口按钮定位，不把整块侧栏包进去实现任意区域拖窗。
  当前的 WindowHandle 分离设计尚未实现，不能据规划调用该接口。
- 菜单使用 ui.MenuBar 共享同一份 MenuItem 模型，Mode 选择系统菜单或窗口内菜单；
  子菜单用 MenuItem 的 Submenu 声明。可勾选命令用 ui.Command(...).Checkable／BindChecked，
  在 CommandScope 中定义，由菜单和快捷键引用同一操作。业务互斥（如视图模式）由应用维护。
- 滚动条的厚度、间距和显示策略用 ScrollView 的 HScrollBarOptions／VScrollBarOptions；
  需要实际视口和轨道分配时读 gui.ScrollView.ScrollGeometry，不从内容控件宽度推测视口。
  SplitView 的拖动热区用 HandleSize；独立 ScrollBar 通过目标 ID 绑定滚动宿主。
- Chrome、Size、Center、SkipTaskbar 等是窗口创建选项，不在同一个已挂载窗口
  ID 的普通重建中改变。窗口尺寸是客户区 DIP，不包含原生边框；位置请求可能被
  窗口管理器调整。Native／Integrated 的实际效果和回退应按当前公开契约核对。
- Overlay 是普通容器，浮动项目用 OverlayItem；只有主子项参与测量。全窗背景可作
  主子项，内容层用 OverlayItem(...).Fill(true) 浮在其上。默认按内容测量，Fill(true)
  只要求铺满，不自动建立模态、焦点隔离或拦截事件。遮罩 Dialog 的拦截、外部
  点击和键盘策略须由实际控件实现。菜单和 DropDown 复用已有原生 Popover，
  不因“覆盖层”需求另建全局弹层管理器。

## 外观、文本与资源

- Modern 的 Sheet／Rules 是普通样式值。需要追加规则时在 modern.Rules 后追加
  style.Name(...)，再用 style.Sheet(rules...)；不要替换成只有几条规则的表，
  却期待完整 Modern 规则自动继承。窗口样式只改背景，不控制原生装饰几何。
  自定义输入控件样式名时，按原控件的公开 Part 核对 selection、caret、focus 和禁用
  状态，不能只配背景色。覆盖正文底色也不意味着已生成的扩展控件派生色会自动重算，
  需要依照该扩展的样式／配色接口重新生成，而非直接修改其内部缓存。
- Primary 自定义内容显式使用 modern.PrimaryContent，例如
  `ui.Button().Content(ui.Label("保存").Style(modern.PrimaryContent)).Style(modern.Primary)`。
  不假设父控件的主题色或字体会自动传给子 Label／Icon；Tab 焦点用 FocusVisible，
  不要为消除鼠标焦点外框而删除所有焦点样式。
- TextInput 可用 BindText(&state)。TextView 小文档可用 Text／OnText 或 BindText；
  持久编辑文档使用 build 外创建的 TextModel。非 nil Model 不能与 Text／BindText
  混用，外部替换内容可能重置历史、选区和 IME；选区端点是 UTF-8 字节位置，
  不是 rune 索引。编辑滚动使用 `ui.ScrollView(ui.TextView()).Style("text-view")`，
  由宿主统一绘制包含滚动条的外框；裸 TextView 仅绘正文。不手写光标／输入法状态机。
- SVG 通过 icon/svg.Parse(reader) 提前解析，位图 Source 使用 icon.NewImage；
  加载错误提前处理。两者默认使用前景色，Logo／多色图片使用 WithRawColors()，配置在 build
  外创建。用 embed 保存静态资源，避免依赖启动工作目录中的相对文件。
- Image 用于固定像素，Icon Source 接收目标物理尺寸和前景色；使用 ui.Icon 的
  DIP Size，不手工重采样补偿 DPI。自定义 Painter 使用预乘颜色，不对已经预乘
  的颜色二次乘 Alpha；只在需要转换的原生边界处理格式。
- 翻译 Getter 和 Icon 可放应用自己的 resource 包集中定义。ui build 内调用
  Getter 取当前文案，ui.Run 自动监听语言变化；系统 Language 和应用选择独立，
  不在 build 中调用 mlang.SetLanguage。框架有八语言不代表应用文案自动翻译或
  RTL 布局已经实现，详见同版本 mlang/README.md。
  跟随系统语言由应用自己的设置策略决定；系统设置通知与视图重建不自动替应用
  调用 mlang.SetLanguage。
- 普通定时用 App.TimeoutFunc／TickFunc，保留 Timer 用于停止或重启；不能在每次
  build 无条件重复启动。动画复用 animation 与 Root.ConnectFrame／RequestPaint，
  连接信号本身不会持续重绘；活动结束后断开，按帧驱动，不假设已存在原生 vsync。

## 文件选择与访问

- 当前 FileFilter 使用裸 Extensions，例如 `[]string{"md", "txt"}`，不传 `*.md`、
  `.md` 或复合后缀。过滤器帮助用户选择，不代替业务校验；DefaultName 是完整建议
  文件名，框架不根据所选过滤器自动补后缀。打开、另存为和覆盖确认分别按业务处理。
- 原生选择器的标题和过滤器名称由应用提供，系统按钮文案和外观由系统决定；应用的
  主题／语言切换不承诺控制原生对话框，不为此修改用户系统设置。
- 取消可能正常返回空结果，不能把 nil 错误直接当成已选文件。成功后按同版本
  FileAccess 契约保持访问有效并及时释放；跨次恢复需要凭据，不只保存路径。

## 扩展而不绕过框架

组合／扩展／实现 Widget 与 View 的六种方式、Self、State 归属和生命周期边界见
[自定义组件](custom-widget-view.md)。不要把嵌入当作虚函数继承，或假设纯 Build
组件的通用修饰器会自动传给展开结果。仅需给组合组件或 ui.View 接口值应用共享
修饰器时，所选版本支持的 ui.Modify(view) 可修饰输出 Widget，无须新增 Widget／布局
容器或自定义 View。保持包装声明的身份与结构稳定，避免因时有时无的包装重建内容。

可先用现有容器组合；确需自定义 Widget 时使用 gui.WidgetBase，自己实现状态、
Measure／Arrange、Paint 和必要的 Snapshot。Paint 只画自身的局部坐标内容，
框架遍历子树并隔离绘制状态；StyleChanged 更新本控件的派生资源，不改全局主题。
声明式自定义适配使用 WidgetView 和 BuildContext 的通用子树协调能力。

声明式维护的 Widget 子树不要通过 FindWidget 任意改父级。特殊树转移、标签脱离、
数据委托和复杂拖放先看同版本的公开机制与示例，不因为缺少某个方便方法就修改
模块缓存或在应用中复刻 root／dispatcher。缺失能力按 Skill 的 Issue／PR 规则反馈。
涉及这些工作流时再读 [有状态工作区](stateful-apps.md)，不要把文档状态、临时控件
状态和窗口资源当作同一种可复用对象。
