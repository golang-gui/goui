// Package widgets contains composite controls built on the gui Widget kernel.
package widgets

import (
	"slices"

	"github.com/golang-gui/goui/core/signal"
	"github.com/golang-gui/goui/gui"
	"github.com/golang-gui/goui/layout"
)

// TabPage is a retained page and its tab metadata. Its child keeps its own
// visibility; TabView hides the page container when it is not current.
type TabPage struct {
	gui.WidgetBase
	title        string
	icon         gui.IconSource
	closable     bool
	child        gui.Widget
	view         *TabView
	transferring bool
}

func NewTabPage(title string, child gui.Widget) *TabPage {
	p := &TabPage{title: title}
	p.SetLayoutManager(layout.NewFillLayout())
	p.SetChild(child)
	return p
}

func (p *TabPage) Title() string { return p.title }
func (p *TabPage) SetTitle(title string) {
	if p.title != title {
		p.title = title
		p.changed()
	}
}
func (p *TabPage) Icon() gui.IconSource { return p.icon }

// SetIcon refreshes the icon even when the same source is supplied.
func (p *TabPage) SetIcon(source gui.IconSource) {
	p.icon = source
	p.changed()
}
func (p *TabPage) Closable() bool { return p.closable }
func (p *TabPage) SetClosable(value bool) {
	if p.closable != value {
		p.closable = value
		p.changed()
	}
}
func (p *TabPage) Child() gui.Widget { return p.child }
func (p *TabPage) SetChild(child gui.Widget) {
	if p.child == child {
		return
	}
	if p.child != nil {
		p.WidgetBase.RemoveChild(p.child)
	}
	p.child = child
	if child != nil {
		p.WidgetBase.AddChild(p, child)
	}
	p.RequestLayout()
}
func (p *TabPage) changed() {
	if p.view != nil {
		p.view.metadata.Emit(p)
	}
}
func (p *TabPage) Snapshot() gui.WidgetInfo {
	info := p.WidgetBase.Snapshot()
	info.Role, info.Text = RoleTabPanel, p.title
	info.Selected = p.view != nil && p.view.current == p
	return info
}

// TabView owns page order and current selection. TabBar is an optional,
// separately placed control associated with this view.
type TabView struct {
	gui.WidgetBase
	current            *TabPage
	changed            signal.Signal0
	metadata           signal.Signal1[*TabPage]
	selected           signal.Signal1[*TabPage]
	closeRequest       signal.Signal1[*TabPage]
	moved              signal.Signal3[*TabPage, int, int]
	transferring       bool
	requestingTransfer bool
	transferRequest    signal.Signal1[*TabTransferRequest]
}

func NewTabView() *TabView {
	v := new(TabView)
	v.SetMainWeight(1)
	v.SetLayoutManager(layout.NewFillLayout())
	return v
}

func (v *TabView) Pages() []*TabPage {
	children := v.Children()
	pages := make([]*TabPage, 0, len(children))
	for _, child := range children {
		if page, ok := child.(*TabPage); ok {
			pages = append(pages, page)
		}
	}
	return pages
}
func (v *TabView) Current() *TabPage        { return v.current }
func (v *TabView) AppendPage(page *TabPage) { v.InsertPage(len(v.Pages()), page) }
func (v *TabView) InsertPage(index int, page *TabPage) {
	if v.Destroyed() || v.transferring || page == nil || page.Destroyed() || page.transferring || page.view != nil || index < 0 || index > len(v.Pages()) || page.Parent() != nil || page.Root() != nil {
		return
	}
	page.view = v
	page.SetVisible(v.current == nil)
	v.WidgetBase.AddChild(v, page)
	if page.Parent() != v {
		page.view = nil
		page.SetVisible(true)
		return
	}
	if index < len(v.Pages())-1 {
		v.WidgetBase.MoveChildBefore(page, v.Pages()[index])
	}
	if v.current == nil {
		v.current = page
		v.changed.Emit()
		if v.current == page {
			v.selected.Emit(page)
		}
	} else {
		v.changed.Emit()
	}
}
func (v *TabView) RemovePage(page *TabPage) {
	if v.Destroyed() || v.transferring || page == nil || page.transferring || page.view != v {
		return
	}
	pages := v.Pages()
	index := slices.Index(pages, page)
	if index < 0 {
		return
	}
	previous := v.current
	if previous == page {
		v.current = nil
		if len(pages) > 1 {
			if index+1 < len(pages) {
				v.current = pages[index+1]
			} else {
				v.current = pages[index-1]
			}
		}
	}
	page.view = nil
	v.WidgetBase.RemoveChild(page)
	page.SetVisible(true)
	if v.current != nil {
		v.current.SetVisible(true)
	}
	v.changed.Emit()
	if previous != v.current {
		v.selected.Emit(v.current)
	}
}
func (v *TabView) MovePage(page *TabPage, index int) {
	if v.Destroyed() || v.transferring || page == nil || page.transferring {
		return
	}
	pages := v.Pages()
	from := slices.Index(pages, page)
	if from < 0 || index < 0 || index >= len(pages) || from == index {
		return
	}
	if from < index {
		v.WidgetBase.MoveChildAfter(page, pages[index])
	} else {
		v.WidgetBase.MoveChildBefore(page, pages[index])
	}
	v.changed.Emit()
	if page.view == v {
		v.moved.Emit(page, from, index)
	}
}
func (v *TabView) SetCurrent(page *TabPage) {
	if v.Destroyed() || v.transferring {
		return
	}
	if page == nil && len(v.Pages()) != 0 {
		return
	}
	if page != nil && page.view != v {
		return
	}
	if page == v.current {
		return
	}
	previous := v.current
	v.current = page
	if previous != nil {
		previous.SetVisible(false)
	}
	if page != nil {
		page.SetVisible(true)
	}
	v.changed.Emit()
	if v.current == page {
		v.selected.Emit(page)
	}
}

// RequestClose notifies the application without removing or selecting page.
// An asynchronous confirmation must check that page still belongs to this view
// before calling RemovePage. Removal itself is independent of Closable.
func (v *TabView) RequestClose(page *TabPage) {
	if v.validPage(page) && page.closable {
		v.closeRequest.Emit(page)
	}
}
func (v *TabView) ConnectCurrent(fn func(*TabPage)) signal.Handle { return v.selected.Connect(fn) }
func (v *TabView) ConnectCloseRequest(fn func(*TabPage)) signal.Handle {
	return v.closeRequest.Connect(func(page *TabPage) {
		if v.validPage(page) && page.closable {
			fn(page)
		}
	})
}

func (v *TabView) validPage(page *TabPage) bool {
	return !v.Destroyed() && !v.transferring && page != nil && !page.Destroyed() &&
		!page.transferring && page.view == v
}

func (v *TabView) ConnectMoved(fn func(*TabPage, int, int)) signal.Handle { return v.moved.Connect(fn) }
func (v *TabView) Snapshot() gui.WidgetInfo {
	info := v.WidgetBase.Snapshot()
	info.Role = RoleTabView
	return info
}
