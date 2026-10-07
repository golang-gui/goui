// GOUI 控件与功能展厅，使用 ui 与 widgets/ui 的公开接口。
// 环境：Windows/macOS 桌面，或 Linux X11（Integrated 需要合成器）。
// 启动：go run ./example/demo；后端/应用缩放可通过 GOUI_PLAT_PAINTER / GOUI_PLAT_SCALE 指定。
// 初始：基础控件、系统主题；动画和计时器关闭，不启动 DevServer。
// 操作：依次点击九个分类，按页面提示操作；具体步骤及预期见 README.md。
// 副作用：显式点击才修改剪贴板、打开系统对话框/外部应用；不自动读写文件。
// 复位：重新启动。退出：关闭主窗口，会停止任务并关闭其他演示窗口。
package main

import (
	"fmt"
	"image"
	"image/color"
	"log"
	"time"

	"github.com/golang-gui/goui/animation"
	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/gui"
	"github.com/golang-gui/goui/layout"
	"github.com/golang-gui/goui/style"
	"github.com/golang-gui/goui/theme/modern"
	"github.com/golang-gui/goui/ui"
	"github.com/golang-gui/goui/widgets"
	wui "github.com/golang-gui/goui/widgets/ui"
)

var pageNames = []string{"基础控件", "文本编辑", "布局与滚动", "数据浏览", "多标签工作区", "菜单与覆盖层", "图形与动画", "系统与拖放", "窗口与检查"}

type demo struct {
	app                                ui.App
	page, mode, accent                 int
	status                             string
	enabled, checked, switched, toggle bool
	mixed                              wui.CheckState
	radio                              int
	group                              *widgets.CheckGroup
	drop, fancyDrop                    int
	choices                            *gui.SliceListModel[widgets.DropDownItem]
	input                              ui.State[string]
	text                               *ui.TextModel
	selection                          ui.TextSelection
	readonly, wrap                     bool
	tree                               *widgets.TreeStore[string]
	expanded, treeSelection            []string
	list                               *gui.SliceListModel[string]
	table                              *recordModel
	tableSelection                     []string
	sortColumn                         string
	sortOrder                          widgets.SortOrder
	tabWindows                         []*tabWindow
	documents                          map[string]*ui.TextModel
	nextDocument, nextWindow           int
	closingTab                         string
	pendingDetach                      *wui.TabDetachRequest
	detachWindow                       string
	detachPosition                     geometry.Point
	panel, mask                        bool
	toast                              string
	toastTimer, tickTimer              *ui.Timer
	ticks                              int
	busy                               bool
	scrollValue                        float32
	layoutStart, layoutRatio           float32
	dataStart, dataRatio               float32
	motion                             *animation.Animation[float32]
	motionValue                        ui.State[float32]
	icons                              []gui.IconSource
	bitmap, capture                    image.Image
	popup                              gui.Popover
	files                              []string
	dropped                            string
	inspection                         *ui.TextModel
}

func main() {
	d, err := newDemo()
	if err != nil {
		log.Fatal(err)
	}
	defer d.stopActivities()
	if err := ui.Run("org.golanggui.Demo", d.build); err != nil {
		log.Fatal(err)
	}
}

func newDemo() (*demo, error) {
	d := &demo{status: "欢迎。选择左侧分类开始体验；主题设置对所有窗口生效。", enabled: true, mixed: wui.CheckMixed,
		group: widgets.NewCheckGroup(), drop: 0, fancyDrop: 0, wrap: true,
		input: ui.MakeState("Hello, GOUI"), text: ui.NewTextModel("GOUI 文本编辑\n\n支持中文输入、选区、撤销与重做。\nEmoji: 👨‍👩‍👧‍👦 组合字符: é\n试试拖选文字、右键菜单，以及 Ctrl/Cmd+A、C、V、Z。\n\n这是一条较长的示例行，用来观察自动换行与水平滚动如何随着可用宽度发生变化。"),
		inspection:  ui.NewTextModel("点击“读取快照”，检查当前窗口的角色、ID、边界、状态与子树。"),
		motionValue: ui.MakeState(float32(32)), expanded: []string{"project", "src"}, dropped: "将文本、应用内卡片或外部文件拖到此处。"}
	d.layoutStart, d.layoutRatio, d.dataStart, d.dataRatio = 200, .4, 220, .68
	d.choices = gui.NewSliceListModel([]widgets.DropDownItem{{Text: "默认选项"}, {Text: "自定义展示"}, {Text: "不可选项目", Disabled: true}, {Text: "最后一个选项"}})
	d.tree = widgets.NewTreeStore[string]()
	d.tree.Append("", "project", "示例项目")
	d.tree.Append("project", "src", "src")
	d.tree.Append("src", "main", "main.go")
	d.tree.Append("src", "view", "view.go")
	d.tree.Append("project", "assets", "assets")
	d.tree.Append("assets", "logo", "logo.svg")
	d.tree.Append("project", "readme", "README.md")
	rows := make([]string, 1000)
	for i := range rows {
		rows[i] = fmt.Sprintf("虚拟列表 · 第 %04d 行", i+1)
	}
	d.list = gui.NewSliceListModel(rows)
	d.table = newRecordModel()
	d.tabWindows = []*tabWindow{{id: "main", pages: []string{"document-1", "document-2", "document-3"}, current: "document-1"}}
	d.nextDocument = 3
	d.documents = make(map[string]*ui.TextModel)
	for i, id := range d.tabWindows[0].pages {
		d.documents[id] = ui.NewTextModel(fmt.Sprintf("文档 %d\n\n输入内容后拖动标签：排序、移交到另一窗口，或拖到桌面创建窗口。\n编辑状态和撤销历史跟随页面。", i+1))
	}
	d.motion = animation.New(float32(32), animation.Float32)
	d.motion.ConnectUpdate(d.motionValue.Set)
	if err := d.loadAssets(); err != nil {
		return nil, err
	}
	return d, nil
}

func (d *demo) report(message string) { d.status = message; d.app.RequestUpdate() }
func (d *demo) result(operation string, err error) {
	if err != nil {
		d.report(operation + "失败：" + err.Error())
	} else {
		d.report(operation + "请求已完成")
	}
}
func (d *demo) stopActivities() {
	d.motion.Stop()
	d.busy = false
	if d.tickTimer != nil {
		d.tickTimer.Stop()
		d.tickTimer = nil
	}
	if d.toastTimer != nil {
		d.toastTimer.Stop()
		d.toastTimer = nil
	}
	if d.popup != nil {
		d.popup.Destroy()
		d.popup = nil
	}
}
func (d *demo) navigate(index int) {
	if index == d.page {
		return
	}
	// 正在移交的页面必须保留源树；先取消尚未完成的脱离准备。
	if d.pendingDetach != nil {
		d.pendingDetach.Cancel()
		d.pendingDetach = nil
	}
	d.stopActivities()
	d.page, d.panel, d.mask, d.toast = index, false, false, ""
	d.report("当前页面：" + pageNames[index])
}

func (d *demo) build(app ui.App) ui.RootView {
	d.app = app
	settings := app.Settings()
	dark := d.mode == 2 || d.mode == 0 && settings.ColorScheme() == ui.ColorSchemeDark
	accents := []color.Color{settings.AccentColor(), color.NRGBA{R: 118, G: 74, B: 213, A: 255}, color.NRGBA{R: 30, G: 151, B: 99, A: 255}, color.NRGBA{R: 217, G: 115, B: 36, A: 255}}
	rules := modern.Rules(modern.Options{Dark: dark, AccentColor: accents[d.accent], FontFamily: settings.FontFamily(), FontSize: settings.FontSize()})
	fontSize := settings.FontSize()
	if fontSize <= 0 {
		fontSize = 14
	}
	bg, card, border, fg := color.NRGBA{R: 245, G: 246, B: 248, A: 255}, color.NRGBA{R: 255, G: 255, B: 255, A: 255}, color.NRGBA{R: 216, G: 220, B: 225, A: 255}, color.NRGBA{R: 40, G: 45, B: 52, A: 255}
	if dark {
		bg = color.NRGBA{R: 30, G: 32, B: 36, A: 255}
		card = color.NRGBA{R: 40, G: 43, B: 48, A: 255}
		border = color.NRGBA{R: 65, G: 69, B: 76, A: 255}
		fg = color.NRGBA{R: 234, G: 236, B: 239, A: 255}
	}
	rules = append(rules,
		style.Name("demo-background").BackgroundColor(bg),
		style.Name("demo-transparent-window").BackgroundColor(color.Transparent),
		style.Name("demo-card").BackgroundColor(card).BorderColor(border).BorderWidth(1).Radius(10),
		style.Name("demo-heading").ForegroundColor(fg).FontSize(22).FontFamily(settings.FontFamily()),
		style.Name("demo-subheading").ForegroundColor(fg).FontSize(16).FontFamily(settings.FontFamily()),
		style.Name("demo-mask").BackgroundColor(color.NRGBA{A: 140}),
		style.Name("demo-swatch").ForegroundColor(accents[d.accent]),
		style.Name("demo-cell-edit").ForegroundColor(fg).FontFamily(settings.FontFamily()).FontSize(fontSize).BackgroundColor(color.Transparent).Radius(0).BorderWidth(0),
		style.Name("demo-cell-edit").State(style.Focused).BorderColor(accents[d.accent]).BorderWidth(1),
	)
	side := make([]ui.View, 0, len(pageNames)+2)
	side = append(side, ui.Label("功能分类").Style("demo-subheading"))
	for i, name := range pageNames {
		b := ui.Button(fmt.Sprintf("%02d  %s", i+1, name)).ID(fmt.Sprintf("nav-%d", i)).OnClick(func() { d.navigate(i) })
		if i == d.page {
			b.Style(modern.Primary)
		}
		side = append(side, b)
	}
	side = append(side, ui.Label("Tab / Shift+Tab：焦点导航\nF5：刷新声明\n点击才启动系统操作").WrapMode(gui.WrapWordChar).Style(modern.MutedText))
	pages := []func() ui.View{d.basicPage, d.textPage, d.layoutPage, d.dataPage, d.tabsPage, d.overlaysPage, d.graphicsPage, d.systemPage, d.windowPage}
	content := ui.VBox(
		ui.HeaderBar(ui.HBox(ui.Icon(d.icons[0]).Size(22), ui.Label("GOUI · 控件与功能展厅").Style("demo-subheading")).Spacing(10)),
		ui.HBox(
			ui.Button("外观："+[]string{"跟随系统", "浅色", "深色"}[d.mode]).ID("theme-mode").OnClick(func() { d.mode = (d.mode + 1) % 3; app.RequestUpdate() }),
			ui.Button("主题色："+[]string{"系统", "紫色", "绿色", "橙色"}[d.accent]).ID("theme-accent").OnClick(func() { d.accent = (d.accent + 1) % 4; app.RequestUpdate() }),
			ui.Label("同一套 Modern 样式，随系统设置和主题重新绘制").Style(modern.MutedText),
		).Padding(12).Spacing(10),
		demoBody(ui.VBox(side...).Spacing(8).Padding(12).MinWidth(190).MaxWidth(190).CrossAlign(layout.CrossStretch),
			pages[d.page]()),
		ui.HBox(ui.Label(d.status).ID("status").WrapMode(gui.WrapWordChar).Style(modern.MutedText).MainWeight(1)).Padding(10).MinHeight(44),
	).CrossAlign(layout.CrossStretch)
	windows := []ui.WindowView{ui.Window("main").Title("GOUI Demo").Chrome(ui.WindowChromeIntegrated).Size(1120, 800).MinSize(820, 600).
		Shortcuts(ui.Shortcut(ui.KeyF5).OnActivate(func() { d.report("已重新协调声明式控件树") })).
		OnDestroy(func() { d.stopActivities(); app.Quit() }).Content(surface(content).Style("demo-background"))}
	for _, window := range d.tabWindows[1:] {
		windows = append(windows, d.tabWindowView(window))
	}
	return ui.Root().StyleSheet(style.Sheet(rules...)).Windows(windows...)
}

// Window 已为根布局分配可用内容区，主体不再重复预留窗口边缘。
// 导航和页面保留各自的内部 Padding，ScrollView 则填满剩余区域。
func demoBody(side, content ui.View) *ui.BoxView {
	return ui.HBox(side, content).CrossAlign(layout.CrossStretch).MainWeight(1)
}

func heading(title, description string) []ui.View {
	return []ui.View{ui.Label(title).Style("demo-heading"), ui.Label(description).WrapMode(gui.WrapWordChar).Style(modern.MutedText)}
}
func section(title string, children ...ui.View) ui.View {
	return surface(ui.VBox(append([]ui.View{ui.Label(title).Style("demo-subheading")}, children...)...).Spacing(12).Padding(16).CrossAlign(layout.CrossStretch)).Style("demo-card")
}
func page(title, description string, children ...ui.View) ui.View {
	return ui.ScrollView(flow(ui.VBox(append(heading(title, description), children...)...).Spacing(16).Padding(16).CrossAlign(layout.CrossStretch))).ID("page-" + title).MainWeight(1)
}
func fullPage(title, description string, children ...ui.View) ui.View {
	return ui.VBox(append(heading(title, description), children...)...).ID("page-" + title).Spacing(12).Padding(16).CrossAlign(layout.CrossStretch).MainWeight(1)
}
func (d *demo) showToast(text string) {
	d.toast = text
	if d.toastTimer != nil {
		d.toastTimer.Stop()
	}
	d.toastTimer = d.app.TimeoutFunc(3*time.Second, func() { d.toast = ""; d.toastTimer = nil; d.app.RequestUpdate() })
	d.app.RequestUpdate()
}
