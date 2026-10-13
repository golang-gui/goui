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
	"github.com/golang-gui/goui/example/demo/resource"
	"github.com/golang-gui/goui/gui"
	"github.com/golang-gui/goui/layout"
	"github.com/golang-gui/goui/mlang"
	"github.com/golang-gui/goui/style"
	"github.com/golang-gui/goui/theme/modern"
	"github.com/golang-gui/goui/ui"
	"github.com/golang-gui/goui/widgets"
	wui "github.com/golang-gui/goui/widgets/ui"
)

var pageNames = []mlang.Getter{resource.Basic, resource.Text, resource.Layout, resource.Data, resource.Tabs, resource.Overlays, resource.Graphics, resource.System, resource.Windows}

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
	input, baselineInput, splitInput   sampleInput
	username                           ui.State[string]
	text                               *ui.TextModel
	textRevision                       uint64
	language                           string
	selection                          ui.TextSelection
	readonly, wrap                     bool
	tree                               *widgets.TreeStore[string]
	expanded, treeSelection            []string
	list                               *gui.SliceListModel[string]
	table                              *recordModel
	tableSelection                     []string
	tableWidths                        [4]float32
	sortColumn                         string
	sortOrder                          widgets.SortOrder
	tabWindows                         []*tabWindow
	documents                          map[string]*ui.TextModel
	documentRevisions                  map[string]uint64
	nextDocument, nextWindow           int
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
	logo                               gui.IconSource
	wordmark, bitmap, capture          image.Image
	popup                              gui.Popover
	files                              []string
	fileAccess                         []ui.FileAccess
	dropped                            string
	droppedKind                        demoDropKind
	inspection                         *ui.TextModel
	inspectionRevision                 uint64
}

func main() {
	// Demo startup language is an application choice, not an implicit Settings policy.
	if err := mlang.SetLanguage("zh-CN"); err != nil {
		log.Fatal(err)
	}
	d, err := newDemo()
	if err != nil {
		log.Fatal(err)
	}
	defer d.stopActivities()
	languageChanged := mlang.ConnectChanged(d.translateUI)
	defer languageChanged.Disconnect()
	if err := ui.Run("org.golanggui.Demo", d.build); err != nil {
		log.Fatal(err)
	}
}

func newDemo() (*demo, error) {
	d := &demo{status: resource.Welcome(), enabled: true, mixed: wui.CheckMixed,
		group: widgets.NewCheckGroup(), drop: 0, fancyDrop: 0, wrap: true,
		input: newSampleInput(resource.InputSample()), baselineInput: newSampleInput(resource.BaselineSample()), splitInput: newSampleInput(resource.SplitInputSample()),
		username: ui.MakeState("GOUI"),
		text:     ui.NewTextModel(resource.EditorSample()), inspection: ui.NewTextModel(resource.InspectionSample()), language: mlang.Language(),
		motionValue: ui.MakeState(float32(32)), expanded: []string{"project", "src"}}
	d.layoutStart, d.layoutRatio, d.dataStart, d.dataRatio = 200, .4, 220, .68
	d.choices = gui.NewSliceListModel([]widgets.DropDownItem{{Text: resource.DefaultOption()}, {Text: resource.CustomOption()}, {Text: resource.DisabledOption(), Disabled: true}, {Text: resource.LastOption()}})
	d.tree = widgets.NewTreeStore[string]()
	d.tree.Append("", "project", resource.SampleProject())
	d.tree.Append("project", "src", "src")
	d.tree.Append("src", "main", "main.go")
	d.tree.Append("src", "view", "view.go")
	d.tree.Append("project", "assets", "assets")
	d.tree.Append("assets", "logo", "logo.svg")
	d.tree.Append("project", "readme", "README.md")
	rows := make([]string, 1000)
	for i := range rows {
		rows[i] = fmt.Sprintf(resource.ListRow(), i+1)
	}
	d.list = gui.NewSliceListModel(rows)
	d.table = newRecordModel()
	d.tableWidths = [4]float32{180, 100, 110, 210}
	d.tabWindows = []*tabWindow{{id: "main", pages: []string{"document-1", "document-2", "document-3"}, current: "document-1"}}
	d.nextDocument = 3
	d.documents = make(map[string]*ui.TextModel)
	d.documentRevisions = make(map[string]uint64)
	for _, id := range d.tabWindows[0].pages {
		d.documents[id] = ui.NewTextModel(documentSample(id))
		d.documentRevisions[id] = 0
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
		d.report(fmt.Sprintf(resource.FailedFormat(), operation, err.Error()))
	} else {
		d.report(fmt.Sprintf(resource.CompletedFormat(), operation))
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
	d.report(fmt.Sprintf(resource.CurrentPageFormat(), pageNames[index]()))
}

func (d *demo) build(app ui.App) ui.RootView {
	d.app = app
	settings := app.Settings()
	dark := d.mode == 2 || d.mode == 0 && settings.ColorScheme() == ui.ColorSchemeDark
	accents := []color.Color{settings.AccentColor(), color.NRGBA{R: 118, G: 74, B: 213, A: 255}, color.NRGBA{R: 30, G: 151, B: 99, A: 255}, color.NRGBA{R: 217, G: 115, B: 36, A: 255}}
	rules := modern.Rules(modern.Options{Dark: dark, AccentColor: accents[d.accent], FontFamily: settings.FontFamily(), FontSize: settings.FontSize()})
	// 当前分类不是主要操作：复制普通按钮的完整规则，再显式指定中性选中底色。
	// 规则是独立的值，不引入样式继承，也不改变其子 Label 的前景。
	for _, rule := range rules {
		if rule.Sel.Name == "button" {
			rule.Sel.Name = "demo-nav-current"
			rules = append(rules, rule)
		}
	}
	selected := [3]color.NRGBA{{R: 208, G: 208, B: 214, A: 255}, {R: 200, G: 200, B: 207, A: 255}, {R: 189, G: 189, B: 197, A: 255}}
	if dark {
		selected = [3]color.NRGBA{{R: 52, G: 52, B: 60, A: 255}, {R: 61, G: 61, B: 69, A: 255}, {R: 69, G: 69, B: 78, A: 255}}
	}
	rules = append(rules,
		style.Name("demo-nav-current").BackgroundColor(selected[0]),
		style.Name("demo-nav-current").State(style.Hovered).BackgroundColor(selected[1]),
		style.Name("demo-nav-current").State(style.Pressed).BackgroundColor(selected[2]),
	)
	fontSize := settings.FontSize()
	if fontSize <= 0 {
		fontSize = 14
	}
	bg, fg := color.NRGBA{R: 245, G: 246, B: 248, A: 255}, color.NRGBA{R: 40, G: 45, B: 52, A: 255}
	if dark {
		bg = color.NRGBA{R: 30, G: 32, B: 36, A: 255}
		fg = color.NRGBA{R: 234, G: 236, B: 239, A: 255}
	}
	rules = append(rules,
		style.Name("demo-background").BackgroundColor(bg),
		style.Name("demo-transparent-window").BackgroundColor(color.Transparent),
		style.Name("demo-title").ForegroundColor(fg).FontSize(fontSize).FontFamily(settings.FontFamily()),
		style.Name("demo-heading").ForegroundColor(fg).FontSize(22).FontFamily(settings.FontFamily()),
		style.Name("demo-subheading").ForegroundColor(fg).FontSize(16).FontFamily(settings.FontFamily()),
		style.Name("demo-mask").BackgroundColor(color.NRGBA{A: 140}),
		style.Name("demo-swatch").ForegroundColor(accents[d.accent]),
		style.Name("demo-cell-edit").ForegroundColor(fg).FontFamily(settings.FontFamily()).FontSize(fontSize).BackgroundColor(color.Transparent).Radius(0).BorderWidth(0),
		style.Name("demo-cell-edit").State(style.Focused).BorderColor(accents[d.accent]).BorderWidth(1),
	)
	side := make([]ui.View, 0, len(pageNames)+1)
	side = append(side, ui.Label(resource.Categories()).Style("demo-subheading"))
	for i, name := range pageNames {
		b := ui.Button().Content(ui.Label(fmt.Sprintf("%02d  %s", i+1, name())).WrapMode(gui.WrapWordChar)).Style(modern.Flat).ID(fmt.Sprintf("nav-%d", i)).OnClick(func() { d.navigate(i) })
		if i == d.page {
			b.Style("demo-nav-current")
		}
		side = append(side, b)
	}
	pages := []func() ui.View{d.basicPage, d.textPage, d.layoutPage, d.dataPage, d.tabsPage, d.overlaysPage, d.graphicsPage, d.systemPage, d.windowPage}
	content := ui.VBox(
		ui.HeaderBar(ui.HBox(ui.Icon(d.logo).Size(24), ui.Label(resource.Title()).Style("demo-title")).Spacing(10)).Padding(6),
		ui.HWrap(
			ui.Button(fmt.Sprintf(resource.AppearanceFormat(), []string{resource.FollowSystem(), resource.Light(), resource.Dark()}[d.mode])).ID("theme-mode").OnClick(func() { d.mode = (d.mode + 1) % 3; app.RequestUpdate() }),
			ui.Button(fmt.Sprintf(resource.AccentFormat(), []string{resource.SystemAccent(), resource.Purple(), resource.Green(), resource.Orange()}[d.accent])).ID("theme-accent").OnClick(func() { d.accent = (d.accent + 1) % 4; app.RequestUpdate() }),
			d.languageMenu(settings.Language()),
			ui.Label(fmt.Sprintf(resource.LanguageStatus(), settings.Language(), mlang.Language())).ID("language-status").Style(modern.MutedText),
		).Padding(12).Spacing(10).LineSpacing(8),
		demoBody(surface(ui.VBox(side...).Spacing(8).Padding(12).CrossAlign(layout.CrossStretch)).
			Style(modern.Sidebar).ID("demo-sidebar").MinWidth(190).MaxWidth(190),
			pages[d.page]()),
		ui.HBox(ui.Label(d.status).ID("status").WrapMode(gui.WrapWordChar).Style(modern.MutedText).MainWeight(1)).Padding(6).MinHeight(28),
	).CrossAlign(layout.CrossStretch)
	windows := []ui.WindowView{ui.Window("main").Title(resource.Title()).Chrome(ui.WindowChromeIntegrated).Size(1120, 800).MinSize(820, 600).Center(true).
		Shortcuts(ui.Shortcut(ui.KeyF5).OnActivate(func() { d.report(resource.Refreshed()) })).
		OnDestroy(func() { d.stopActivities(); d.closeFiles(); app.Quit() }).Content(surface(content).Style("demo-background"))}
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
	return surface(ui.VBox(append([]ui.View{ui.Label(title).Style("demo-subheading")}, children...)...).Spacing(12).Padding(16).CrossAlign(layout.CrossStretch)).Style(modern.Card)
}
func page(id, title, description string, children ...ui.View) ui.View {
	scroll := ui.ScrollView(ui.VBox(append(heading(title, description), children...)...).Spacing(16).Padding(16).CrossAlign(layout.CrossStretch)).Direction(ui.ScrollVertical).ID("page-" + id).MainWeight(1)
	// 页面视觉留白在视口外，才能同时避让滚动条；不承担窗口装饰补偿。
	return ui.VBox(scroll).Padding(8).CrossAlign(layout.CrossStretch).MainWeight(1)
}
func fullPage(id, title, description string, children ...ui.View) ui.View {
	return ui.VBox(append(heading(title, description), children...)...).ID("page-" + id).Spacing(12).Padding(16).CrossAlign(layout.CrossStretch).MainWeight(1)
}

var demoLanguages = []struct{ tag, name string }{
	{"zh-CN", "简体中文"}, {"en", "English"}, {"ja", "日本語"}, {"ko", "한국어"},
	{"de", "Deutsch"}, {"fr", "Français"}, {"ru", "Русский"}, {"ar", "العربية"},
}

func (d *demo) languageMenu(systemLanguage string) ui.View {
	items := make([]*ui.MenuItemView, 0, len(demoLanguages)+2)
	for _, choice := range demoLanguages {
		label := choice.name
		if choice.tag == mlang.Language() {
			label = "✓ " + label
		}
		items = append(items, ui.MenuItem(label, func() {
			err := mlang.SetLanguage(choice.tag)
			d.result(resource.SelectLanguage(), err)
		}))
	}
	items = append(items, ui.MenuSeparator(), ui.MenuItem(resource.UseSystemLanguage()+" ("+systemLanguage+")", func() {
		err := mlang.SetLanguage(d.app.Settings().Language())
		d.result(resource.UseSystemLanguage(), err)
	}))
	return ui.MenuButton(fmt.Sprintf(resource.LanguageFormat(), mlang.Language())).ID("language-menu").Menu(items...)
}

func (d *demo) showToast(text string) {
	d.toast = text
	if d.toastTimer != nil {
		d.toastTimer.Stop()
	}
	d.toastTimer = d.app.TimeoutFunc(3*time.Second, func() { d.toast = ""; d.toastTimer = nil; d.app.RequestUpdate() })
	d.app.RequestUpdate()
}
