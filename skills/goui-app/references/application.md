# 应用开发参考

示例以发布本技能时的公开 API 为基线。开发已有应用时，始终先核对其选中版本，
不能因示例较新就升级依赖。下面的源码路径相对于选中版本的 GOUI 模块目录，
不是相对于安装后的 Skill；安装本技能不要求存在整个框架仓库。

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
| 样式、亮暗和语义预设 | theme/modern/README.md、theme/basic、style、example/modern |
| SVG、图标、翻译和动画 | icon、mlang/README.md、animation、example/demo |
| 文件访问、外部打开、拖放 | ui/dialog.go、ui/app.go、ui/dragdrop.go、tests/window/ui |

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
                            ui.Label(fmt.Sprintf("Hello, %s", name.Get())),
                            ui.Label(fmt.Sprintf("Clicks: %d", count.Get())),
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

普通字段更新后调用 app.RequestUpdate；ui.State.Set／Update 已包含更新请求。
build 中读取状态，回调中改变状态；不在 build 中持续 Set／RequestUpdate 制造循环。
后台读取文件或网络时，通过 app.Post 提交结果，处理取消和过期结果，避免已关闭
的窗口或旧文档被异步结果重新更新。App 可跨 goroutine 调度，不代表 Widget 可并发访问。

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
    box.SetPadding(20)
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
    app.Run()
    return nil
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
- Integrated 不自动插入 HeaderBar。使用 ui.HeaderBar(child) 放自己的内容，
  由 GUI 处理原生控件避让和常见拖动命中；不手工为 Linux 留一套猜测边框偏移。
  Captions 显式指定命中的 Widget ID，不能拿按钮区域当普通空白拖拽区。
- Chrome、Size、Center、SkipTaskbar 等是窗口创建选项，不在同一个已挂载窗口
  ID 的普通重建中改变。窗口尺寸是客户区 DIP，不包含原生边框；位置请求可能被
  窗口管理器调整。Native／Integrated 的实际效果和回退应按当前公开契约核对。
- Overlay 是普通容器，浮动项目用 OverlayItem；默认按内容测量，Fill(true)
  只要求铺满，不自动建立模态、焦点隔离或拦截事件。遮罩 Dialog 的拦截、外部
  点击和键盘策略须由实际控件实现。菜单和 DropDown 复用已有原生 Popover，
  不因“覆盖层”需求另建全局弹层管理器。

## 外观、文本与资源

- Modern 的 Sheet／Rules 是普通样式值。需要追加规则时在 modern.Rules 后追加
  style.Name(...)，再用 style.Sheet(rules...)；不要替换成只有几条规则的表，
  却期待完整 Modern 规则自动继承。窗口样式只改背景，不控制原生装饰几何。
- Primary 自定义内容显式使用 modern.PrimaryContent，例如
  `ui.Button().Content(ui.Label("保存").Style(modern.PrimaryContent)).Style(modern.Primary)`。
  不假设父控件的主题色或字体会自动传给子 Label／Icon；Tab 焦点用 FocusVisible，
  不要为消除鼠标焦点外框而删除所有焦点样式。
- TextInput 可用 BindText(&state)。TextView 小文档可用 Text／OnText 或 BindText；
  持久编辑文档使用 build 外创建的 TextModel。非 nil Model 不能与 Text／BindText
  混用，外部替换内容可能重置历史、选区和 IME；选区端点是 UTF-8 字节位置，
  不是 rune 索引。编辑滚动可组合 ScrollView，不手写光标／输入法状态机。
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
- 普通定时用 App.TimeoutFunc／TickFunc，保留 Timer 用于停止或重启；不能在每次
  build 无条件重复启动。动画复用 animation 与 Root.ConnectFrame／RequestPaint，
  连接信号本身不会持续重绘；活动结束后断开，按帧驱动，不假设已存在原生 vsync。

## 扩展而不绕过框架

组合／扩展／实现 Widget 与 View 的六种方式、Self、State 归属和生命周期边界见
[自定义组件](custom-widget-view.md)。不要把嵌入当作虚函数继承，或假设纯 Build
组件的通用修饰器会自动传给展开结果。

可先用现有容器组合；确需自定义 Widget 时使用 gui.WidgetBase，自己实现状态、
Measure／Arrange、Paint 和必要的 Snapshot。Paint 只画自身的局部坐标内容，
框架遍历子树并隔离绘制状态；StyleChanged 更新本控件的派生资源，不改全局主题。
声明式自定义适配使用 WidgetView 和 BuildContext 的通用子树协调能力。

声明式维护的 Widget 子树不要通过 FindWidget 任意改父级。特殊树转移、标签脱离、
数据委托和复杂拖放先看同版本的公开机制与示例，不因为缺少某个方便方法就修改
模块缓存或在应用中复刻 root／dispatcher。缺失能力按 Skill 的 Issue／PR 规则反馈。
