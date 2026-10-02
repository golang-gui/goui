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
// may retain the request while constructing the target, then TransferTo or
// Cancel on the GUI thread. The page stays in the source until TransferTo; its tab stays
// hidden with a reserved slot while the request is pending. Size and position
// the target before TransferTo and its first Show; no hidden layout is required.
// These input fields are read-only; Position is the end point in Window's
// client DIP and Hotspot is the original tab-local grab point, not a window
// offset. If the reference window moves before positioning, cancel or remap
// the saved point; never reinterpret it as a fresh pointer observation.
type TabDetachRequest struct {
	Page     *TabPage
	Window   gui.Window
	Position geometry.Point
	Hotspot  geometry.Point
	// ContentSize is the source TabView's current page viewport in DIP, not
	// the dragged page's possibly stale rectangle or its intrinsic size.
	// It excludes a separate TabBar and other window content. The application
	// adds its own target chrome/layout overhead for initial window sizing;
	// this snapshot imposes no lasting constraint on the page or target.
	ContentSize geometry.Size

	bar     *TabBar
	run     *tabTransferDrag
	unmount signal.Handle
}

// ConnectDetachRequest provides a handled result, initially false. A handler
// that retains or processes the request must set it to true. If it remains
// false after emission, the request is cancelled and its source tab restored.
// Connections run in order; later handlers may change the result while the
// request remains valid. The result pointer is only valid during the callback;
// never retain it. This cannot undo an already committed transfer.
// A new local drag, source unload, SetView or disabling Transferable
// invalidates a pending request. It is independent of Closable. This signal
// never creates or closes a window and does not change the native DnD result.
func (b *TabBar) ConnectDetachRequest(fn func(*TabDetachRequest, *bool)) signal.Handle {
	return b.detachRequest.Connect(func(request *TabDetachRequest, handled *bool) {
		if request.valid() {
			fn(request, handled)
		}
	})
}

// Cancel releases the pending request, not its still source-owned page. It is
// idempotent. The application must also close any unused window it created.
func (r *TabDetachRequest) Cancel() {
	r.release(true)
}

func (r *TabDetachRequest) release(restore bool) {
	if r == nil {
		return
	}
	if r.unmount != nil {
		r.unmount.Disconnect()
		r.unmount = nil
	}
	bar := r.bar
	if bar != nil && bar.pendingDetach == r {
		bar.pendingDetach = nil
	}
	r.bar, r.run = nil, nil
	if restore && bar != nil && !bar.Destroyed() {
		bar.syncDragVisibility()
	}
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
	bar := r.bar
	r.release(false) // ordinary Unmount during commit must not cancel the move
	err := run.view.TransferPage(run.page, target, index)
	if !bar.Destroyed() {
		bar.syncDragVisibility()
	}
	return err
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
		Hotspot: run.hotspot, ContentSize: run.view.Rect().Size, bar: b, run: run}
	b.pendingDetach = r
	if !r.valid() {
		r.Cancel()
		return
	}
	r.unmount = run.page.ConnectUnmount(r.Cancel)
	b.syncDragVisibility()
	handled := false
	b.detachRequest.Emit(r, &handled)
	if !handled {
		r.Cancel()
	}
}
