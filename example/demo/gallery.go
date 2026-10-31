package main

import (
	"fmt"
	"image"

	"github.com/golang-gui/goui/example/demo/resource"
	"github.com/golang-gui/goui/gui"
	"github.com/golang-gui/goui/icon/svg"
	"github.com/golang-gui/goui/layout"
	"github.com/golang-gui/goui/mlang"
	"github.com/golang-gui/goui/style"
	"github.com/golang-gui/goui/theme/modern"
	"github.com/golang-gui/goui/ui"
)

var pageNames = []mlang.Getter{resource.Basic, resource.Text, resource.Layout, resource.Data, resource.Tabs, resource.Overlays, resource.Graphics, resource.System, resource.Windows}

// demo owns navigation, shared resources and the stable feature objects.
type demo struct {
	app      ui.App
	page     ui.State[int]
	mode     int
	accent   int
	status   ui.State[string]
	input    sampleInput
	language string
	icons    []gui.IconSource
	logo     gui.IconSource
	wordmark *svg.Source
	bitmap   image.Image
	basic    *basicDemo
	editor   *textDemo
	layout   *layoutDemo
	data     *dataDemo
	tabs     *tabsDemo
	overlays *overlaysDemo
	graphics *graphicsDemo
	system   *systemDemo
	windows  *windowsDemo
}

func newDemo() (*demo, error) {
	d := &demo{status: ui.MakeState(resource.Welcome()), input: newSampleInput(resource.InputSample()), language: mlang.Language()}
	d.basic = newBasicDemo(d)
	d.editor = newTextDemo(d)
	d.layout = newLayoutDemo(d)
	d.data = newDataDemo(d)
	d.tabs = newTabsDemo(d)
	d.overlays = newOverlaysDemo(d)
	d.graphics = newGraphicsDemo(d)
	d.system = newSystemDemo(d)
	d.windows = newWindowsDemo(d)
	if err := d.loadAssets(); err != nil {
		return nil, err
	}
	return d, nil
}

func (d *demo) report(message string) { d.status.Set(message) }
func (d *demo) result(operation string, err error) {
	if err != nil {
		d.report(fmt.Sprintf(resource.FailedFormat(), operation, err.Error()))
	} else {
		d.report(fmt.Sprintf(resource.CompletedFormat(), operation))
	}
}
func (d *demo) stopActivities() {
	d.graphics.motion.Stop()
	d.graphics.busy.Set(false)
	if d.system.tickTimer != nil {
		d.system.tickTimer.Stop()
		d.system.tickTimer = nil
	}
	if d.overlays.toastTimer != nil {
		d.overlays.toastTimer.Stop()
		d.overlays.toastTimer = nil
	}
	if d.overlays.popup != nil {
		d.overlays.popup.Destroy()
		d.overlays.popup = nil
	}
}
func (d *demo) navigate(index int) {
	if index == d.page.Get() {
		return
	}
	// Keep the source subtree alive during a page transfer. Cancel any pending
	// detach preparation before changing pages.
	if d.tabs.pendingDetach != nil {
		d.tabs.pendingDetach.Cancel()
		d.tabs.pendingDetach = nil
	}
	d.stopActivities()
	{
		d.page.Set(index)
		d.overlays.panel.Set(false)
		d.overlays.mask.Set(false)
		d.overlays.toast.Set("")
	}
	d.report(fmt.Sprintf(resource.CurrentPageFormat(), pageNames[index]()))
}

func (d *demo) build(app ui.App) ui.RootView {
	d.app = app
	settings := app.Settings()
	rules := d.styleRules(settings)
	pages := []func() ui.View{d.basic.View, d.editor.View, d.layout.View, d.data.View, d.tabs.View, d.overlays.View, d.graphics.View, d.system.View, d.windows.View}
	content := ui.VBox(
		ui.HeaderBar(ui.HBox(ui.Icon(d.logo).Size(24), ui.Label(resource.Title()).Style("demo-title")).Spacing(10)).Padding(6),
		ui.HWrap(
			ui.Button(fmt.Sprintf(resource.AppearanceFormat(), []string{resource.FollowSystem(), resource.Light(), resource.Dark()}[d.mode])).ID("theme-mode").OnClick(func() { d.mode = (d.mode + 1) % 3; app.RequestUpdate() }),
			ui.Button(fmt.Sprintf(resource.AccentFormat(), []string{resource.SystemAccent(), resource.Purple(), resource.Green(), resource.Orange()}[d.accent])).ID("theme-accent").OnClick(func() { d.accent = (d.accent + 1) % 4; app.RequestUpdate() }),
			d.languageMenu(settings.Language()),
			ui.Label(fmt.Sprintf(resource.LanguageStatus(), settings.Language(), mlang.Language())).ID("language-status").Style(modern.MutedText),
		).Padding(12).Spacing(10).LineSpacing(8),
		demoBody(d.sidebar(), ui.Modify(ui.Build(func() ui.View { return pages[d.page.Get()]() })).MainWeight(1)),
		ui.HBox(ui.Label().BindText(&d.status).ID("status").WrapMode(gui.WrapWordChar).Style(modern.MutedText).MainWeight(1)).Padding(6).MinHeight(28),
	).CrossAlign(layout.CrossStretch)
	windows := []ui.WindowView{ui.Window("main").Title(resource.Title()).Chrome(ui.WindowChromeIntegrated).Size(1120, 800).MinSize(820, 600).Center(true).
		Shortcuts(ui.Shortcut(ui.KeyF5).OnActivate(func() { app.RequestUpdate(); d.report(resource.Refreshed()) })).
		OnDestroy(func() { d.stopActivities(); d.system.closeFiles(); app.Quit() }).Content(surface(content).Style("demo-background"))}
	for _, window := range d.tabs.tabWindows[1:] {
		windows = append(windows, d.tabs.tabWindowView(window))
	}
	return ui.Root().StyleSheet(style.Sheet(rules...)).Windows(windows...)
}

// Window has already allocated the root's content area. Reserve window edges
// only once; navigation and pages keep their own padding, and ScrollView fills
// the remaining area.
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
	// Keep visual page spacing outside the viewport so it also clears scrollbars.
	// Window decoration insets are handled by the host.
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

func (d *demo) sidebar() ui.View {
	return ui.Build(func() ui.View {
		side := make([]ui.View, 0, len(pageNames)+1)
		side = append(side, ui.Label(resource.Categories()).Style("demo-subheading"))
		for i, name := range pageNames {
			b := ui.Button().Content(ui.Label(fmt.Sprintf("%02d  %s", i+1, name())).WrapMode(gui.WrapWordChar)).Style(modern.Flat).ID(fmt.Sprintf("nav-%d", i)).OnClick(func() { d.navigate(i) })
			if i == d.page.Get() {
				b.Style("demo-nav-current")
			}
			side = append(side, b)
		}

		return ui.VBox(side...).Spacing(8).Padding(12).CrossAlign(layout.CrossStretch).Style(modern.Sidebar).ID("demo-sidebar").MinWidth(190).MaxWidth(190)
	})
}
