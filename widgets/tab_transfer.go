package widgets

import (
	"fmt"
	"slices"

	"github.com/golang-gui/goui/core/signal"
	"github.com/golang-gui/goui/gui"
)

// TabTransferRequest lets an adapter coordinate ownership before a page move.
// Inputs are read-only. Set Handled and Err to reject or provide coordinated
// handling; Commit performs the ordinary move once without emitting another
// request. The request is valid only during this synchronous notification.
type TabTransferRequest struct {
	Source, Target *TabView
	Page           *TabPage
	Index          int
	Handled        bool
	Err            error
	commit         func() error
	called         bool
}

func (r *TabTransferRequest) Commit() error {
	if r == nil || r.commit == nil || r.called {
		return fmt.Errorf("widgets: inactive tab transfer request")
	}
	r.called, r.Handled = true, true
	r.Err = r.commit()
	return r.Err
}

// ConnectTransferRequest is queried on the source, then on an unhandled
// destination. A declarative adapter can reject mixed ownership or wrap Commit
// in its own coordination transaction. Ordinary imperative views need no handler.
func (v *TabView) ConnectTransferRequest(fn func(*TabTransferRequest)) signal.Handle {
	return v.transferRequest.Connect(func(r *TabTransferRequest) {
		if !v.Destroyed() && !r.Handled {
			fn(r)
		}
	})
}

// TransferPage moves the retained page and its child to target, selecting it
// there. index is an insertion position in [0, len(target.Pages())]. Moving
// within this view instead uses MovePage's final-index and selection rules.
//
// Cross-view transfer unmounts and mounts the page normally, preserving its
// widget objects, not host-bound focus, gestures or rendering resources. It
// never sends CloseRequest or closes an empty source window. Page/selection
// notifications run after both views are consistent. Reentrant page mutations
// during lifecycle callbacks are ignored; destroying a host still takes effect.
// On ordinary failure the live source retains the original page and selection.
// A page explicitly destroyed by a callback is never restored.
//
// Declarative pages also require their UI ownership records to be transferred;
// callers must not use this method to bypass the declaration coordinator.
func (v *TabView) TransferPage(page *TabPage, target *TabView, index int) error {
	if v == nil || target == nil || page == nil || v.Destroyed() || target.Destroyed() || page.Destroyed() ||
		v.transferring || target.transferring || v.requestingTransfer || target.requestingTransfer || page.transferring || page.view != v || page.Parent() != v {
		return fmt.Errorf("widgets: tab transfer source or target is unavailable")
	}
	v.requestingTransfer, target.requestingTransfer = true, true
	defer func() { v.requestingTransfer, target.requestingTransfer = false, false }()
	request := &TabTransferRequest{Source: v, Target: target, Page: page, Index: index,
		commit: func() error { return v.transferPage(page, target, index) }}
	defer func() { request.commit = nil }()
	v.transferRequest.Emit(request)
	if !request.Handled && target != v {
		target.transferRequest.Emit(request)
	}
	if !request.Handled {
		return request.Commit()
	}
	if request.Err == nil && !request.called {
		return fmt.Errorf("widgets: handled tab transfer was not committed")
	}
	return request.Err
}

func (v *TabView) transferPage(page *TabPage, target *TabView, index int) error {
	if v == nil || target == nil || page == nil || v.Destroyed() || target.Destroyed() || page.Destroyed() ||
		v.transferring || target.transferring || page.transferring || page.view != v || page.Parent() != v {
		return fmt.Errorf("widgets: tab transfer source or target is unavailable")
	}
	if target == v {
		if index < 0 || index >= len(v.Pages()) {
			return fmt.Errorf("widgets: invalid tab reorder index %d", index)
		}
		v.MovePage(page, index)
		return nil
	}
	if index < 0 || index > len(target.Pages()) {
		return fmt.Errorf("widgets: invalid tab insertion index %d", index)
	}
	for ancestor := gui.Widget(target); ancestor != nil; ancestor = ancestor.Parent() {
		if ancestor == page {
			return fmt.Errorf("widgets: cannot transfer a tab into its own subtree")
		}
	}
	pages := v.Pages()
	from := slices.Index(pages, page)
	if from < 0 {
		return fmt.Errorf("widgets: tab is no longer in source")
	}
	previousSource, previousTarget := v.current, target.current
	next := previousSource
	if next == page {
		next = nil
		if from+1 < len(pages) {
			next = pages[from+1]
		} else if from > 0 {
			next = pages[from-1]
		}
	}
	v.transferring, target.transferring, page.transferring = true, true, true
	defer func() { v.transferring, target.transferring, page.transferring = false, false, false }()
	page.view = nil
	v.WidgetBase.RemoveChild(page)
	if !page.Destroyed() && page.Parent() == nil && !v.Destroyed() && !target.Destroyed() {
		page.view = target
		target.WidgetBase.AddChild(target, page)
		if page.Parent() == target && !page.Destroyed() && !target.Destroyed() {
			if index < len(target.Pages())-1 {
				target.WidgetBase.MoveChildBefore(page, target.Pages()[index])
			}
			v.selectAfterTransfer(next)
			target.selectAfterTransfer(page)
		}
	}
	committed := !page.Destroyed() && !target.Destroyed() && page.Parent() == target && page.view == target
	if !committed {
		// Do not resurrect objects destroyed by application callbacks, or take
		// a page back from an unrelated parent installed by application code.
		page.view = nil
		if !page.Destroyed() && !v.Destroyed() && page.Parent() == nil {
			page.view = v
			v.WidgetBase.AddChild(v, page)
			if page.Parent() == v && !page.Destroyed() {
				remaining := v.Pages()
				if from < len(remaining)-1 {
					v.WidgetBase.MoveChildBefore(page, remaining[from])
				}
			} else {
				page.view = nil
			}
		}
		v.selectAfterTransfer(previousSource)
		target.selectAfterTransfer(previousTarget)
	}
	// Notifications may now mutate either view, but not re-transfer this page
	// until this call returns. Recheck selection/lifetime after every emission.
	v.transferring, target.transferring = false, false
	v.notifyTransfer(previousSource)
	target.notifyTransfer(previousTarget)
	if !committed {
		return fmt.Errorf("widgets: tab transfer invalidated by lifecycle callback")
	}
	return nil
}

func (v *TabView) selectAfterTransfer(preferred *TabPage) {
	if v.Destroyed() {
		v.current = nil
		return
	}
	pages := v.Pages()
	if !slices.Contains(pages, preferred) {
		preferred = nil
		if len(pages) > 0 {
			preferred = pages[0]
		}
	}
	v.current = preferred
	for _, page := range pages {
		if v.Destroyed() {
			v.current = nil
			return
		}
		page.SetVisible(page == preferred)
	}
}

func (v *TabView) notifyTransfer(previous *TabPage) {
	if v.Destroyed() {
		return
	}
	current := v.current
	v.changed.Emit()
	if !v.Destroyed() && current != previous && v.current == current {
		v.selected.Emit(current)
	}
}
