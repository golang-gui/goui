package main

import (
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/example/demo/resource"
	"github.com/golang-gui/goui/gui"
	"github.com/golang-gui/goui/layout"
	"github.com/golang-gui/goui/ui"
	"github.com/golang-gui/goui/widgets"
	wui "github.com/golang-gui/goui/widgets/ui"
)

// tabsDemo owns this feature for the lifetime of the gallery.
// host is borrowed for shared assets and application operations.
type tabsDemo struct {
	host                     *demo
	tabWindows               []*tabWindow
	documents                map[string]*ui.TextModel
	documentRevisions        map[string]uint64
	nextDocument, nextWindow int
	pendingDetach            *wui.TabDetachRequest
	detachWindow             string
	detachPosition           geometry.Point
}

type tabWindow struct {
	id      string
	pages   []string
	current string
	size    geometry.Size
	closing string
}

func (d *tabsDemo) tabState(id string) *tabWindow {
	for _, w := range d.tabWindows {
		if w.id == id {
			return w
		}
	}
	return nil
}
func (d *tabsDemo) addTab(w *tabWindow) {
	d.nextDocument++
	id := fmt.Sprintf("document-%d", d.nextDocument)
	d.documents[id] = ui.NewTextModel(documentSample(id))
	d.documentRevisions[id] = 0
	w.pages = append(w.pages, id)
	w.current = id
	d.host.app.RequestUpdate()
}
func (d *tabsDemo) tabContents(w *tabWindow) ui.View {
	pages := make([]*wui.TabPageView, 0, len(w.pages))
	for _, id := range w.pages {
		pages = append(pages, wui.TabPage(id, ui.ScrollView(ui.TextView().ID("editor-"+id).Model(d.documents[id]).WrapMode(ui.WrapWordChar).Padding(16)).Style("text-view")).Title(fmt.Sprintf(resource.DocumentTitle(), strings.TrimPrefix(id, "document-"))).Icon(d.host.logo).Closable(true))
	}
	bar := wui.TabBar("documents").ID("tab-bar").Reorderable(true).Transferable(true).TabWidthRange(105, 200).
		ContextMenu(func(id string) []*ui.MenuItemView {
			return []*ui.MenuItemView{ui.MenuItem(resource.CloseTab(), func() { d.requestTabClose(w, id) }), ui.MenuItem(resource.NewTab(), func() { d.addTab(w) })}
		}).
		OnDetachRequest(func(r *wui.TabDetachRequest, handled *bool) { d.detachTab(w, r, handled) }).
		OnTransferError(func(e error) { d.host.result(resource.TabTransfer(), e) }).OnContextMenuError(func(e error) { d.host.result(resource.TabMenu(), e) })
	view := wui.TabView(pages...).ID("documents").MainWeight(1).Current(w.current).
		OnCurrent(func(id string) { w.current = id; d.host.app.RequestUpdate() }).
		OnCloseRequest(func(id string) { d.requestTabClose(w, id) }).
		OnMoved(func(id string, from, to int) {
			w.pages = moveID(w.pages, id, to)
			d.host.app.RequestUpdate()
			d.host.report(resource.TabsReordered())
		}).OnTransfer(d.transferTab)
	return ui.VBox(ui.HeaderBar(bar).Padding(4), view).CrossAlign(layout.CrossStretch)
}
func (d *tabsDemo) View() ui.View {
	w := d.tabWindows[0]
	return fullPage("tabs", resource.Tabs(), resource.TabsHint(),
		ui.HWrap(ui.Button(resource.NewTab()).ID("new-tab").OnClick(func() { d.addTab(w) }), ui.Button(resource.OpenTargetWindow()).ID("tab-target").OnClick(d.newTabWindow)).Spacing(10),
		d.tabWorkspace(w).MainWeight(1),
	)
}

func (d *tabsDemo) tabWorkspace(w *tabWindow) *ui.OverlayView {
	var confirm ui.View
	if w.id != "main" && w.closing != "" {
		confirm = d.confirmClose(w)
	}
	return ui.Overlay(d.tabContents(w), ui.OverlayItem(confirm).Fill(true).Modal(true))
}
func (d *tabsDemo) confirmClose(w *tabWindow) ui.View {
	return wui.Dialog(ui.VBox(ui.Label(resource.CloseTabQuestion()).Style("demo-subheading"), ui.Label(resource.CloseTabHint()).WrapMode(gui.WrapWordChar),
		ui.HBox(ui.Button(resource.Cancel()).OnClick(func() { w.closing = ""; d.host.app.RequestUpdate() }), ui.Button(resource.Close()).OnClick(func() { d.closeTab(w) })).Spacing(12),
	).Spacing(12).MaxWidth(440)).Padding(20).OnCloseRequest(func() { w.closing = ""; d.host.app.RequestUpdate() }).ID("tab-close-confirm")
}

func (d *tabsDemo) requestTabClose(w *tabWindow, id string) {
	if d.tabState(w.id) != w || !slices.Contains(w.pages, id) {
		return
	}
	w.closing = id
	d.host.app.RequestUpdate()
	if w.id == "main" {
		d.navigateToTabs()
	}
	d.host.report(fmt.Sprintf(resource.TabCloseRequested(), id))
}
func (d *tabsDemo) navigateToTabs() {
	if d.host.page.Get() != 4 {
		d.host.navigate(4)
	}
}
func (d *tabsDemo) closeTab(w *tabWindow) {
	id := w.closing
	w.closing = ""
	if d.tabState(w.id) != w {
		return
	}
	if i := slices.Index(w.pages, id); i >= 0 {
		w.pages = slices.Delete(w.pages, i, i+1)
		if w.current == id {
			w.current = ""
			if len(w.pages) > 0 {
				w.current = w.pages[min(i, len(w.pages)-1)]
			}
		}
		delete(d.documents, id)
		delete(d.documentRevisions, id)
		d.host.app.RequestUpdate()
		d.host.report(fmt.Sprintf(resource.TabClosed(), id))
	}
}
func moveID(ids []string, id string, index int) []string {
	out := slices.Clone(ids)
	if i := slices.Index(out, id); i >= 0 {
		out = slices.Delete(out, i, i+1)
	}
	index = max(0, min(index, len(out)))
	return slices.Insert(out, index, id)
}
func (d *tabsDemo) transferTab(e wui.TabTransfer) {
	source, target := d.tabState(e.SourceWindow), d.tabState(e.TargetWindow)
	if source == nil || target == nil {
		return
	}
	if i := slices.Index(source.pages, e.PageID); i >= 0 {
		source.pages = slices.Delete(source.pages, i, i+1)
	}
	if source.closing == e.PageID {
		source.closing = ""
	}
	if source.current == e.PageID {
		source.current = ""
		if len(source.pages) > 0 {
			source.current = source.pages[0]
		}
	}
	target.pages = moveID(target.pages, e.PageID, e.Index)
	target.current = e.PageID
	d.host.app.RequestUpdate()
	d.host.report(fmt.Sprintf(resource.TabTransferred(), e.PageID, target.id))
}
func (d *tabsDemo) newTabWindow() {
	d.nextWindow++
	w := &tabWindow{id: fmt.Sprintf("tabs-%d", d.nextWindow), size: geometry.Size{Width: 620, Height: 420}}
	d.tabWindows = append(d.tabWindows, w)
	d.addTab(w)
}
func (d *tabsDemo) tabWindowView(w *tabWindow) ui.WindowView {
	content := afterUpdate(d.tabWorkspace(w).Style("demo-background"), nil)
	if d.pendingDetach != nil && d.detachWindow == w.id {
		content.after = func() { d.prepareDetach(w) }
	}
	return ui.Window(w.id).Title(resource.TabWindowTitle()).Size(w.size.Width, w.size.Height).Chrome(ui.WindowChromeIntegrated).
		OnDestroy(func() {
			w.closing = ""
			d.tabWindows = slices.DeleteFunc(d.tabWindows, func(item *tabWindow) bool { return item == w })
			for _, id := range w.pages {
				delete(d.documents, id)
				delete(d.documentRevisions, id)
			}
			d.host.app.RequestUpdate()
		}).Content(content)
}
func (d *tabsDemo) detachTab(source *tabWindow, r *wui.TabDetachRequest, handled *bool) {
	if len(source.pages) < 2 || d.pendingDetach != nil {
		return
	}
	*handled = true
	d.nextWindow++
	id := fmt.Sprintf("tabs-%d", d.nextWindow)
	// Source and target share tabContents. Reuse the arranged HeaderBar and host
	// insets, excluding the gallery's navigation and toolbar. No hidden-window
	// layout pass is needed.
	bar := d.host.app.FindWidget(source.id, "tab-bar")
	if bar == nil || bar.Parent() == nil || r.Window.Widget() == nil {
		r.Cancel()
		d.host.report(resource.TabLayoutNotReady())
		return
	}
	inset := r.Window.Widget().Rect().Pos
	height := bar.Parent().Rect().Height
	size := geometry.Size{Width: r.ContentSize.Width + 2*inset.X, Height: r.ContentSize.Height + height + 2*inset.Y}
	// Estimate the target origin before its window controls reserve space;
	// correct it from the target's actual allocation after creation.
	header, ok := bar.Parent().(*gui.HeaderBar)
	if !ok {
		r.Cancel()
		return
	}
	// The target HeaderBar is at the top. Its initial X is padding, before
	// reserving target controls, rather than the source's adjusted X.
	barOrigin := geometry.Point{X: header.Padding().Left, Y: bar.Rect().Y}
	d.detachPosition = r.Position.Add(r.Hotspot.Add(barOrigin).Add(inset).Scale(-1))
	d.tabWindows = append(d.tabWindows, &tabWindow{id: id, size: size})
	d.pendingDetach, d.detachWindow = r, id
	d.host.app.RequestUpdate()
}
func (d *tabsDemo) prepareDetach(w *tabWindow) {
	r := d.pendingDetach
	if r == nil || d.detachWindow != w.id {
		return
	}
	d.pendingDetach = nil
	target := d.host.app.FindWindow(w.id)
	view, _ := d.host.app.FindWidget(w.id, "documents").(*widgets.TabView)
	err := errors.New(resource.TabContainerNotReady())
	if target != nil && view != nil {
		position := d.detachPosition
		// ConnectInfo reports the current allocation synchronously; borrow it once
		// without retaining a subscription.
		var info gui.ChromeInfo
		connection := target.Chrome().ConnectInfo(func(current gui.ChromeInfo) { info = current })
		connection.Disconnect()
		position.X -= tabControlsInset(info, r.Window.Widget().Rect().X)
		err = target.SetPosition(r.Window, position)
		if err == nil {
			err = r.TransferTo(view, 0)
		}
	}
	if err != nil {
		r.Cancel()
		if target != nil {
			target.Destroy()
		}
		d.tabWindows = slices.DeleteFunc(d.tabWindows, func(item *tabWindow) bool { return item == w })
		d.host.app.RequestUpdate()
		d.host.result(resource.CreateDetachedWindow(), err)
	}
}

func tabControlsInset(info gui.ChromeInfo, inset float32) float32 {
	if !info.Enabled || info.Controls != gui.ChromeControlsNative || info.ControlsBounds.Width <= 0 {
		return 0
	}
	return max(0, info.ControlsBounds.X+info.ControlsBounds.Width-inset)
}

func newTabsDemo(host *demo) *tabsDemo {
	d := &tabsDemo{host: host}
	d.tabWindows = []*tabWindow{{id: "main", pages: []string{"document-1", "document-2", "document-3"}, current: "document-1"}}
	d.nextDocument = 3
	d.documents = make(map[string]*ui.TextModel)
	d.documentRevisions = make(map[string]uint64)
	for _, id := range d.tabWindows[0].pages {
		d.documents[id] = ui.NewTextModel(documentSample(id))
		d.documentRevisions[id] = 0
	}

	return d
}
