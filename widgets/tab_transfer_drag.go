package widgets

import (
	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/core/signal"
	"github.com/golang-gui/goui/gui"
	"github.com/golang-gui/goui/layout"
)

const tabTransferFormat = "goui.widgets.tab-page"

// tabTransferDrag belongs to one native session, not to an application-wide
// page registry. The source view retains its page until a successful Drop.
type tabTransferDrag struct {
	page                      *TabPage
	view                      *TabView
	source                    *gui.DragSource
	target                    *TabBar
	started, ended, committed bool
	window                    gui.Window
	hotspot                   geometry.Point
}

// SetTransferable opts this bar into native cross-TabView Move. Starting a
// transfer additionally requires Reorderable; receiving does not. Both bars
// must opt in. A sole remaining page may move to another bar, but an unaccepted
// release never requests a new window. This does not enable automatic window
// creation; the application handles ConnectDetachRequest.
func (b *TabBar) SetTransferable(value bool) {
	if b.transferable == value {
		return
	}
	b.transferable = value
	if b.dropTarget == nil && value {
		t := gui.NewDropTarget(gui.LocalFormat(tabTransferFormat))
		t.SetActions(gui.DragMove)
		t.ConnectEnter(b.moveIncoming)
		t.ConnectMotion(b.moveIncoming)
		t.ConnectLeave(b.clearIncoming)
		t.ConnectError(func(err error) { b.transferError.Emit(err) })
		t.ConnectDrop(b.dropIncoming)
		b.dropTarget = t
		b.AddEventController(t)
	}
	if b.dropTarget != nil {
		b.dropTarget.SetEnabled(value)
	}
	if !value {
		b.cancelDetach()
		b.clearIncoming()
		if b.nativeDrag != nil {
			b.nativeDrag.source.Cancel()
		}
	}
}

// ConnectTransferError reports rendering/native startup or transfer failures.
// Cancellation and an unaccepted release are not errors.
func (b *TabBar) ConnectTransferError(fn func(error)) signal.Handle {
	return b.transferError.Connect(fn)
}

func (b *TabBar) handoff(page *TabPage, gesture *gui.DragEventController, point geometry.Point) bool {
	viewport := b.viewport.Rect()
	if !b.transferable || b.view == nil || b.dragPage != page || b.nativeDrag != nil ||
		(point.X >= viewport.X && point.X <= viewport.X+viewport.Width && point.Y >= viewport.Y && point.Y <= viewport.Y+viewport.Height) {
		return false
	}
	item := b.items[page]
	bitmap, err := gui.RenderWidget(item, 0)
	if err != nil {
		b.cancelDrag()
		b.transferError.Emit(err)
		return true
	}
	run := &tabTransferDrag{page: page, view: b.view, source: gui.NewDragSource(),
		window: b.Window(), hotspot: geometry.Point{X: b.grabX, Y: b.grabY}}
	run.source.SetActions(gui.DragMove)
	if len(b.view.Pages()) > 1 {
		run.source.SetFeedback(gui.DragFeedback{NeutralOutsideTargets: true, DisableReturnAnimation: true})
	}
	run.source.ConnectBegin(func() {
		run.started = true
		b.syncDragVisibility()
	})
	run.source.ConnectEnd(func(result gui.DragResult) {
		run.ended = true
		if run.target != nil && run.target.incoming == run {
			run.target.clearIncoming()
		}
		if b.nativeDrag == run {
			b.nativeDrag = nil
		}
		if result.Err != nil {
			b.syncDragVisibility()
			b.transferError.Emit(result.Err)
			return
		}
		b.requestDetach(run, result)
		b.syncDragVisibility()
	})
	b.nativeDrag = run
	data := new(gui.DragData)
	data.SetLocal(tabTransferFormat, run)
	// Restore the source's ordinary layout before entering a possibly nested
	// native loop. Only a successful target may alter canonical page order.
	b.stopDrag()
	if err := run.source.Begin(gesture, data, gui.DragPreview{Image: bitmap, Hotspot: run.hotspot}); err != nil {
		run.ended = true
		if b.nativeDrag == run {
			b.nativeDrag = nil
		}
		b.syncDragVisibility()
		b.transferError.Emit(err)
	}
	return true
}

func (b *TabBar) validIncoming(run *tabTransferDrag) bool {
	return run != nil && !run.ended && !run.committed && b.transferable && b.view != nil &&
		!b.Destroyed() && !b.view.Destroyed() && !run.view.Destroyed() && !run.page.Destroyed() &&
		run.page.view == run.view && run.page.Parent() == run.view && (run.view != b.view || b.reorderable)
}

func (b *TabBar) moveIncoming(e *gui.DragMotion) {
	value, _ := e.Local(tabTransferFormat)
	run, _ := value.(*tabTransferDrag)
	if !b.validIncoming(run) {
		e.Action = 0
		b.clearIncoming()
		return
	}
	b.incoming, run.target, b.pointer = run, b, e.Position
	b.previewMotion = true
	b.syncDragVisibility()
	// The extra slot participates in normal equal-width/overflow allocation.
	// Refresh it now so Drop uses current geometry even before the next frame.
	b.RequestLayout()
	b.Arrange(b.Rect())
	e.Action = gui.DragMove
	b.updateFrames()
	b.viewport.RequestPaint()
}

func (b *TabBar) incomingIndex(point geometry.Point) int {
	index, _ := b.incomingSlot(point)
	return index
}

// incomingSlot returns a final insertion index and its content-space boundary.
// Canonical slots are stable during the slide animation. A source-owned page
// retains its canonical hit-test slot but is counted only once in final order;
// the visual gap replaces that page rather than adding another empty slot.
func (b *TabBar) incomingSlot(point geometry.Point) (int, float32) {
	// Arrange has already allocated arrow space, but not moved the viewport.
	x := point.X - (b.Rect().Width-b.viewportWidth)/2 + b.scroll
	position := float32(0)
	index := 0
	for _, page := range b.viewPages() {
		item := b.items[page]
		if item == nil {
			continue
		}
		if x < position+item.width/2 {
			return index, float32(index) * (b.tabWidth + tabSpacing)
		}
		position += item.width + tabSpacing
		if b.incoming == nil || page != b.incoming.page {
			index++
		}
	}
	return index, float32(index) * (b.tabWidth + tabSpacing)
}

func (b *TabBar) dropIncoming(e *gui.DropRequest) {
	value, _ := e.Data.Local(tabTransferFormat)
	run, _ := value.(*tabTransferDrag)
	if !b.validIncoming(run) || e.Action != gui.DragMove {
		b.clearIncoming()
		return
	}
	b.incoming = run
	index := b.incomingIndex(e.Position) // never trust an earlier motion's slot
	if err := run.view.TransferPage(run.page, b.view, index); err != nil {
		b.clearIncoming()
		b.transferError.Emit(err)
		return
	}
	run.committed = true
	b.clearIncoming()
	e.Accepted = true
}

func (b *TabBar) clearIncoming() {
	if b.incoming == nil {
		return
	}
	if b.incoming.target == b {
		b.incoming.target = nil
	}
	b.incoming = nil
	b.previewMotion = true
	b.syncDragVisibility()
	b.RequestLayout()
	if !b.Destroyed() {
		b.Arrange(b.Rect())
	}
	b.viewport.RequestPaint()
	b.updateFrames()
}

func (b *TabBar) updateIncomingSlot() {
	if !b.validIncoming(b.incoming) {
		return
	}
	index := b.incomingIndex(b.pointer)
	if index != b.incomingAt {
		b.incomingAt = index
		b.previewMotion = true
	}
}

func (b *TabBar) slotCount() int {
	count := len(b.items)
	if b.validIncoming(b.incoming) && b.incoming.view != b.view {
		count++
	}
	return count
}

func (b *TabBar) syncDragVisibility() {
	if b.Destroyed() {
		return
	}
	for page, item := range b.items {
		hidden := b.nativeDrag != nil && b.nativeDrag.started && !b.nativeDrag.ended &&
			!b.nativeDrag.committed && b.nativeDrag.page == page
		hidden = hidden || (b.validIncoming(b.incoming) && b.incoming.page == page)
		hidden = hidden || (b.pendingDetach != nil && b.pendingDetach.valid() && b.pendingDetach.Page == page)
		if hidden && item.Visible() {
			// WidgetBase deliberately measures invisible children as zero.
			// Preserve the source's intrinsic height before hiding its subtree.
			item.reservedHeight = item.Measure(layout.Unbounded()).Height
		}
		item.SetVisible(!hidden)
		if b.Destroyed() {
			return
		}
	}
	b.RequestPaint()
}
