package widgets

import (
	"fmt"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/core/signal"
	"github.com/golang-gui/goui/gui"
)

// TabDetachRequest asks the application to prepare a destination after an
// unaccepted native drag. It does not assert that the pointer was over empty
// desktop: native backends cannot always distinguish a foreign rejection.
//
// The application owns window creation, positioning, showing and cleanup. It
// may retain the request while waiting for ordinary layout, then TransferTo or
// Cancel on the GUI thread. The page stays in the source until TransferTo.
// These input fields are read-only; Position is the end point in Window's
// client DIP and Hotspot is the original tab-local grab point, not a window
// offset. If the reference window moves before positioning, cancel or remap
// the saved point; never reinterpret it as a fresh pointer observation.
type TabDetachRequest struct {
	Page     *TabPage
	Window   gui.Window
	Position geometry.Point
	Hotspot  geometry.Point

	bar     *TabBar
	run     *tabTransferDrag
	unmount signal.Handle
}

// ConnectDetachRequest is opt-in: without a handler, an unaccepted drag keeps
// its page. A new local drag, source unload, SetView or disabling Transferable
// invalidates a pending request. It is independent of Closable. This signal
// never creates or closes a window and does not change the native DnD result.
func (b *TabBar) ConnectDetachRequest(fn func(*TabDetachRequest)) signal.Handle {
	return b.detachRequest.Connect(func(request *TabDetachRequest) {
		if request.valid() {
			fn(request)
		}
	})
}

// Cancel releases the pending request, not its still source-owned page. It is
// idempotent. The application must also close any unused window it created.
func (r *TabDetachRequest) Cancel() {
	if r == nil {
		return
	}
	if r.unmount != nil {
		r.unmount.Disconnect()
		r.unmount = nil
	}
	if r.bar != nil && r.bar.pendingDetach == r {
		r.bar.pendingDetach = nil
	}
	r.bar, r.run = nil, nil
}

// TransferTo consumes this request and moves its original page after the
// application's required window preparation has succeeded. Like TransferPage,
// it returns errors to the caller (not ConnectTransferError). One request can
// be attempted only once, including failure. It never opens/closes a window.
// Declarative pages require UI coordination and must not bypass it here.
func (r *TabDetachRequest) TransferTo(target *TabView, index int) error {
	if !r.valid() {
		r.Cancel()
		return fmt.Errorf("widgets: tab detach request is no longer valid")
	}
	run := r.run
	r.Cancel() // ordinary page Unmount during commit must not cancel the move
	return run.view.TransferPage(run.page, target, index)
}

func (r *TabDetachRequest) valid() bool {
	return r != nil && r.bar != nil && r.run != nil && r.bar.pendingDetach == r &&
		!r.bar.Destroyed() && r.bar.transferable && r.bar.view == r.run.view &&
		!r.run.view.Destroyed() && len(r.run.view.Pages()) > 1 && !r.run.page.Destroyed() &&
		r.run.page.view == r.run.view && r.run.page.Parent() == r.run.view &&
		r.bar.Root() == r.run.window && r.run.page.Root() == r.run.window
}

func (b *TabBar) cancelDetach() {
	if b.pendingDetach != nil {
		b.pendingDetach.Cancel()
	}
}

func (b *TabBar) requestDetach(run *tabTransferDrag, result gui.DragResult) {
	if run == nil || !run.ended || run.committed || run.window == nil ||
		result.Action != 0 || result.Canceled || result.Err != nil ||
		!result.PositionValid || result.LocalDrop || result.LocalTarget || result.Validate() != nil {
		return
	}
	b.cancelDetach()
	r := &TabDetachRequest{Page: run.page, Window: run.window, Position: result.Position,
		Hotspot: run.hotspot, bar: b, run: run}
	b.pendingDetach = r
	if !r.valid() {
		r.Cancel()
		return
	}
	r.unmount = run.page.ConnectUnmount(r.Cancel)
	b.detachRequest.Emit(r)
}
