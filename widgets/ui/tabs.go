// Package ui adapts composite widgets to GOUI's declarative View layer.
package ui

import (
	"fmt"
	"slices"

	"github.com/golang-gui/goui/core/signal"
	"github.com/golang-gui/goui/gui"
	baseui "github.com/golang-gui/goui/ui"
	"github.com/golang-gui/goui/widgets"
)

// TabPageView is an ordinary retained node. Its nonempty ID is unique within
// the Root and retains the page and its content across local list reorders.
type TabPageView struct {
	baseui.ViewBase[TabPageView]
	id       string
	child    baseui.View
	title    string
	icon     gui.IconSource
	closable bool
}

func TabPage(id string, child baseui.View) *TabPageView {
	p := &TabPageView{id: id, child: child}
	p.Self = p
	p.ID(id)
	return p
}
func (p *TabPageView) ID(id string) *TabPageView {
	p.id = id
	return p.ViewBase.ID(id)
}
func (p *TabPageView) Title(value string) *TabPageView         { p.title = value; return p }
func (p *TabPageView) Icon(source gui.IconSource) *TabPageView { p.icon = source; return p }
func (p *TabPageView) Closable(value bool) *TabPageView        { p.closable = value; return p }

func (p *TabPageView) Build() baseui.View { return p }
func (p *TabPageView) Mount(baseui.BuildContext) gui.Widget {
	return widgets.NewTabPage(p.title, nil)
}
func (p *TabPageView) Update(ctx baseui.BuildContext, widget gui.Widget) {
	page := widget.(*widgets.TabPage)
	page.SetTitle(p.title)
	page.SetIcon(p.icon)
	page.SetClosable(p.closable)
	ctx.UpdateChild(page, p.child)
}
func (*TabPageView) Unmount(baseui.BuildContext, gui.Widget) {}

// tabPages adapts the existing imperative page API, not a second page registry.
type tabPages struct{ view *widgets.TabView }

func (p *tabPages) Widget() gui.Widget       { return p.view }
func (p *tabPages) AddChild(w gui.Widget)    { p.view.AppendPage(w.(*widgets.TabPage)) }
func (p *tabPages) RemoveChild(w gui.Widget) { p.view.RemovePage(w.(*widgets.TabPage)) }
func (p *tabPages) MoveChildBefore(child, sibling gui.Widget) {
	pages := p.view.Pages()
	page := child.(*widgets.TabPage)
	from, index := slices.Index(pages, page), len(pages)-1
	if sibling != nil {
		index = slices.Index(pages, sibling.(*widgets.TabPage))
		if from < index {
			index--
		}
	}
	p.view.MovePage(page, index)
}

type TabViewView struct {
	baseui.ViewBase[TabViewView]
	pages      []*TabPageView
	current    string
	currentSet bool
	onCurrent  func(string)
	onClose    func(string)
	onMoved    func(string, int, int)
	onTransfer func(TabTransfer)
}
type tabViewState struct {
	pages       *tabPages
	onCurrent   func(string)
	onClose     func(string)
	onMoved     func(string, int, int)
	connections signal.Handles
	updating    bool
	coordinator *baseui.Coordinator
	onTransfer  func(TabTransfer)
}

func TabView(pages ...*TabPageView) *TabViewView {
	v := &TabViewView{pages: slices.Clone(pages)}
	v.Self = v
	return v
}
func (v *TabViewView) Pages(pages ...*TabPageView) *TabViewView {
	v.pages = slices.Clone(pages)
	return v
}
func (v *TabViewView) Current(id string) *TabViewView                 { v.current, v.currentSet = id, true; return v }
func (v *TabViewView) OnCurrent(fn func(string)) *TabViewView         { v.onCurrent = fn; return v }
func (v *TabViewView) OnCloseRequest(fn func(string)) *TabViewView    { v.onClose = fn; return v }
func (v *TabViewView) OnMoved(fn func(string, int, int)) *TabViewView { v.onMoved = fn; return v }

// OnTransfer updates the application's page declarations after a coordinated
// cross-view move. The source must provide this handler to allow transfers.
// Update both collections and request an ordinary UI update; do not rebuild
// synchronously or recreate the page's contents.
func (v *TabViewView) OnTransfer(fn func(TabTransfer)) *TabViewView { v.onTransfer = fn; return v }
func (v *TabViewView) Build() baseui.View                           { return v }
func (v *TabViewView) Mount(ctx baseui.BuildContext) gui.Widget {
	view := widgets.NewTabView()
	state := &tabViewState{pages: &tabPages{view: view}, coordinator: ctx.Coordinator()}
	state.connections = signal.Handles{
		view.ConnectCurrent(func(page *widgets.TabPage) {
			if state.updating || state.onCurrent == nil {
				return
			}
			publish := func() {
				if !view.Destroyed() && state.onCurrent != nil && view.Current() == page {
					state.onCurrent(pageID(page))
				}
			}
			if state.coordinator != nil {
				state.coordinator.AfterTransfer(publish)
			} else {
				publish()
			}
		}),
		view.ConnectCloseRequest(func(page *widgets.TabPage) {
			if !state.updating && state.onClose != nil {
				state.onClose(pageID(page))
			}
		}),
		view.ConnectMoved(func(page *widgets.TabPage, from, to int) {
			if !state.updating && state.onMoved != nil {
				state.onMoved(pageID(page), from, to)
			}
		}),
		view.ConnectTransferRequest(state.transfer),
	}
	ctx.SetState(state)
	return view
}
func (v *TabViewView) Update(ctx baseui.BuildContext, widget gui.Widget) {
	view := widget.(*widgets.TabView)
	state := ctx.State().(*tabViewState)
	seen := make(map[string]bool, len(v.pages))
	for _, desc := range v.pages {
		if desc == nil || desc.id == "" {
			panic("widgets/ui: TabPage requires a nonempty ID")
		}
		if seen[desc.id] {
			panic(fmt.Sprintf("widgets/ui: duplicate TabPage ID %q", desc.id))
		}
		seen[desc.id] = true
	}
	state.updating = true
	defer func() { state.updating = false }()
	state.onCurrent, state.onClose, state.onMoved = v.onCurrent, v.onClose, v.onMoved
	state.onTransfer = v.onTransfer
	children := make([]baseui.View, len(v.pages))
	for i, page := range v.pages {
		children[i] = page
	}
	ctx.UpdateChildren(state.pages, children)
	if v.currentSet {
		for _, page := range view.Pages() {
			if page.ID() == v.current {
				view.SetCurrent(page)
				break
			}
		}
	}
	// Page visibility belongs to selection, not a page's constructor snapshot.
	for _, page := range view.Pages() {
		page.SetVisible(page == view.Current())
	}
}
func (v *TabViewView) Unmount(ctx baseui.BuildContext, _ gui.Widget) {
	if state, ok := ctx.State().(*tabViewState); ok {
		state.connections.Disconnect()
	}
}

type TabBarView struct {
	baseui.ViewBase[TabBarView]
	viewID                   string
	reorderable              bool
	minTabWidth, maxTabWidth float32
	transferable             bool
	onTransferError          func(error)
	onDetachRequest          func(*TabDetachRequest, *bool)
	onContextMenuError       func(error)
	contextMenu              func(string) []*baseui.MenuItemView
}

func TabBar(viewID string) *TabBarView {
	v := &TabBarView{viewID: viewID}
	v.Self = v
	return v
}
func (v *TabBarView) Reorderable(value bool) *TabBarView         { v.reorderable = value; return v }
func (v *TabBarView) Transferable(value bool) *TabBarView        { v.transferable = value; return v }
func (v *TabBarView) OnTransferError(fn func(error)) *TabBarView { v.onTransferError = fn; return v }

// ContextMenu declares this bar's menu for the requested page ID, not the
// current selection. The builder runs synchronously on input, not during View
// updates. Return nil/empty to suppress it. Building does not execute actions.
func (v *TabBarView) ContextMenu(fn func(string) []*baseui.MenuItemView) *TabBarView {
	v.contextMenu = fn
	return v
}

// OnContextMenuError observes failures to present an input-triggered menu.
func (v *TabBarView) OnContextMenuError(fn func(error)) *TabBarView {
	v.onContextMenuError = fn
	return v
}

// OnDetachRequest forwards the GUI request and its initially false result.
// Set handled to true when retaining or processing the request; otherwise the
// source tab is restored after emission. The result pointer is callback-local.
func (v *TabBarView) OnDetachRequest(fn func(*TabDetachRequest, *bool)) *TabBarView {
	v.onDetachRequest = fn
	return v
}

// TabWidthRange sets equal-tab width limits in DIP. Omission restores 112..240.
func (v *TabBarView) TabWidthRange(minWidth, maxWidth float32) *TabBarView {
	v.minTabWidth, v.maxTabWidth = minWidth, maxWidth
	return v
}
func (v *TabBarView) Build() baseui.View { return v }
func (v *TabBarView) Mount(ctx baseui.BuildContext) gui.Widget {
	bar := widgets.NewTabBar()
	state := new(tabBarState)
	state.handles = signal.Handles{
		bar.ConnectContextMenu(func(page *widgets.TabPage, menu *gui.MenuModel) {
			if state.contextMenu != nil {
				*menu = baseui.Menu(state.contextMenu(pageID(page))...)
			}
		}),
		bar.ConnectContextMenuError(func(err error) {
			if state.onContextMenuError != nil {
				state.onContextMenuError(err)
			}
		}),
		bar.ConnectTransferError(func(err error) {
			if state.onError != nil {
				state.onError(err)
			}
		}),
		bar.ConnectDetachRequest(func(request *widgets.TabDetachRequest, handled *bool) {
			if pageID(request.Page) != "" && state.onDetach != nil {
				state.onDetach(&TabDetachRequest{TabDetachRequest: request, PageID: request.Page.ID()}, handled)
			}
		}),
	}
	ctx.SetState(state)
	return bar
}
func (v *TabBarView) Update(ctx baseui.BuildContext, widget gui.Widget) {
	bar := widget.(*widgets.TabBar)
	state := ctx.State().(*tabBarState)
	state.onError, state.onDetach = v.onTransferError, v.onDetachRequest
	state.onContextMenuError = v.onContextMenuError
	state.contextMenu = v.contextMenu
	bar.SetTransferable(v.transferable)
	bar.SetReorderable(v.reorderable)
	bar.SetTabWidthRange(v.minTabWidth, v.maxTabWidth)
	viewID := v.viewID
	ctx.AfterUpdate(func() {
		var view *widgets.TabView
		if root := bar.Root(); root != nil {
			view, _ = gui.FindWidget(root, viewID).(*widgets.TabView)
		}
		bar.SetView(view)
	})
}
func (v *TabBarView) Unmount(ctx baseui.BuildContext, widget gui.Widget) {
	ctx.State().(*tabBarState).handles.Disconnect()
	widget.(*widgets.TabBar).SetView(nil)
}
