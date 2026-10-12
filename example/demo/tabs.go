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
	"github.com/golang-gui/goui/theme/modern"
	"github.com/golang-gui/goui/ui"
	"github.com/golang-gui/goui/widgets"
	wui "github.com/golang-gui/goui/widgets/ui"
)

type tabWindow struct {
	id      string
	pages   []string
	current string
	size    geometry.Size
	closing string
}

func (d *demo) tabState(id string) *tabWindow {
	for _, w := range d.tabWindows {
		if w.id == id {
			return w
		}
	}
	return nil
}
func (d *demo) addTab(w *tabWindow) {
	d.nextDocument++
	id := fmt.Sprintf("document-%d", d.nextDocument)
	d.documents[id] = ui.NewTextModel(documentSample(id))
	d.documentRevisions[id] = 0
	w.pages = append(w.pages, id)
	w.current = id
	d.app.RequestUpdate()
}
func (d *demo) tabContents(w *tabWindow) ui.View {
	pages := make([]*wui.TabPageView, 0, len(w.pages))
	for _, id := range w.pages {
		pages = append(pages, wui.TabPage(id, ui.ScrollView(ui.TextView().ID("editor-"+id).Model(d.documents[id]).WrapMode(ui.WrapWordChar).Padding(16))).Title(fmt.Sprintf(resource.DocumentTitle(), strings.TrimPrefix(id, "document-"))).Icon(d.logo).Closable(true))
	}
	bar := wui.TabBar("documents").ID("tab-bar").Reorderable(true).Transferable(true).TabWidthRange(105, 200).
		ContextMenu(func(id string) []*ui.MenuItemView {
			return []*ui.MenuItemView{ui.MenuItem(resource.CloseTab(), func() { d.requestTabClose(w, id) }), ui.MenuItem(resource.NewTab(), func() { d.addTab(w) })}
		}).
		OnDetachRequest(func(r *wui.TabDetachRequest, handled *bool) { d.detachTab(w, r, handled) }).
		OnTransferError(func(e error) { d.result(resource.TabTransfer(), e) }).OnContextMenuError(func(e error) { d.result(resource.TabMenu(), e) })
	view := wui.TabView(pages...).ID("documents").MainWeight(1).Current(w.current).
		OnCurrent(func(id string) { w.current = id; d.app.RequestUpdate() }).
		OnCloseRequest(func(id string) { d.requestTabClose(w, id) }).
		OnMoved(func(id string, from, to int) { w.pages = moveID(w.pages, id, to); d.report(resource.TabsReordered()) }).OnTransfer(d.transferTab)
	return ui.VBox(ui.HeaderBar(bar).Padding(4), view).CrossAlign(layout.CrossStretch)
}
func (d *demo) tabsPage() ui.View {
	w := d.tabWindows[0]
	return fullPage("tabs", resource.Tabs(), resource.TabsHint(),
		ui.HWrap(ui.Button(resource.NewTab()).ID("new-tab").OnClick(func() { d.addTab(w) }), ui.Button(resource.OpenTargetWindow()).ID("tab-target").OnClick(d.newTabWindow)).Spacing(10),
		d.tabWorkspace(w).MainWeight(1),
	)
}

func (d *demo) tabWorkspace(w *tabWindow) *ui.OverlayView {
	confirm := surface(ui.VBox(ui.Label(resource.CloseTabQuestion()).Style("demo-subheading"), ui.Label(resource.CloseTabHint()),
		ui.HBox(ui.Button(resource.Cancel()).OnClick(func() { w.closing = ""; d.app.RequestUpdate() }), ui.Button(resource.Close()).OnClick(func() { d.closeTab(w) })).Spacing(12)).Padding(20).Spacing(12)).Style(modern.Card).MaxWidth(440)
	layer := ui.Overlay(surface(nil).Style("demo-mask"), ui.OverlayItem(confirm).OnPosition(center)).ID("tab-close-confirm").Visible(w.closing != "")
	return ui.Overlay(d.tabContents(w), ui.OverlayItem(layer).Fill(true))
}
func (d *demo) requestTabClose(w *tabWindow, id string) {
	if d.tabState(w.id) != w || !slices.Contains(w.pages, id) {
		return
	}
	w.closing = id
	if w.id == "main" {
		d.navigateToTabs()
	}
	d.report(fmt.Sprintf(resource.TabCloseRequested(), id))
}
func (d *demo) navigateToTabs() {
	if d.page != 4 {
		d.navigate(4)
	}
}
func (d *demo) closeTab(w *tabWindow) {
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
		d.report(fmt.Sprintf(resource.TabClosed(), id))
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
func (d *demo) transferTab(e wui.TabTransfer) {
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
	d.report(fmt.Sprintf(resource.TabTransferred(), e.PageID, target.id))
}
func (d *demo) newTabWindow() {
	d.nextWindow++
	w := &tabWindow{id: fmt.Sprintf("tabs-%d", d.nextWindow), size: geometry.Size{Width: 620, Height: 420}}
	d.tabWindows = append(d.tabWindows, w)
	d.addTab(w)
}
func (d *demo) tabWindowView(w *tabWindow) ui.WindowView {
	content := surface(d.tabWorkspace(w)).Style("demo-background")
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
			d.app.RequestUpdate()
		}).Content(content)
}
func (d *demo) detachTab(source *tabWindow, r *wui.TabDetachRequest, handled *bool) {
	if len(source.pages) < 2 || d.pendingDetach != nil {
		return
	}
	*handled = true
	d.nextWindow++
	id := fmt.Sprintf("tabs-%d", d.nextWindow)
	// 源和目标都使用同一个 tabContents。沿用已布局的 HeaderBar 及宿主
	// 内缩量，不带主展厅导航/工具栏的开销，也不布局隐藏窗口。
	bar := d.app.FindWidget(source.id, "tab-bar")
	if bar == nil || bar.Parent() == nil || r.Window.Widget() == nil {
		r.Cancel()
		d.report(resource.TabLayoutNotReady())
		return
	}
	inset := r.Window.Widget().Rect().Pos
	height := bar.Parent().Rect().Height
	size := geometry.Size{Width: r.ContentSize.Width + 2*inset.X, Height: r.ContentSize.Height + height + 2*inset.Y}
	// 暂存目标未避让控制按钮时的起点，创建后按目标的实际占位修正。
	header, ok := bar.Parent().(*gui.HeaderBar)
	if !ok {
		r.Cancel()
		return
	}
	// 目标是顶部 HeaderBar，基础 X 是 padding，不是源已避让后的 X。
	barOrigin := geometry.Point{X: header.Padding(), Y: bar.Rect().Y}
	d.detachPosition = r.Position.Add(r.Hotspot.Add(barOrigin).Add(inset).Scale(-1))
	d.tabWindows = append(d.tabWindows, &tabWindow{id: id, size: size})
	d.pendingDetach, d.detachWindow = r, id
	d.app.RequestUpdate()
}
func (d *demo) prepareDetach(w *tabWindow) {
	r := d.pendingDetach
	if r == nil || d.detachWindow != w.id {
		return
	}
	d.pendingDetach = nil
	target := d.app.FindWindow(w.id)
	view, _ := d.app.FindWidget(w.id, "documents").(*widgets.TabView)
	err := errors.New(resource.TabContainerNotReady())
	if target != nil && view != nil {
		position := d.detachPosition
		// ConnectInfo 同步报告当前占位；这里一次性借用，不注册长期监听。
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
		d.result(resource.CreateDetachedWindow(), err)
	}
}

func tabControlsInset(info gui.ChromeInfo, inset float32) float32 {
	if !info.Enabled || info.Controls != gui.ChromeControlsNative || info.ControlsBounds.Width <= 0 {
		return 0
	}
	return max(0, info.ControlsBounds.X+info.ControlsBounds.Width-inset)
}
