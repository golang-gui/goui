package gui

import (
	"slices"

	"github.com/golang-gui/goui/core/signal"
	"github.com/golang-gui/goui/layout"
)

// ScrollBarClient receives scroll metrics in DIP and emits position requests.
// Setters must not emit requests. Implementations need not be Widgets. Each
// client must be associated with only one host axis at a time; multiple clients
// may control the same axis. All calls and handle operations are GUI-thread-only.
type ScrollBarClient interface {
	SetMax(float32)
	SetPageSize(float32)
	SetValue(float32)
	ConnectChange(func(float32)) signal.Handle
}

// BindScrollBar associates an independently owned client with one scroll axis.
// It immediately supplies the current metrics without forcing layout. Requests
// use normal scrolling and always receive the actual, rounded and clamped value.
// Binding does not change built-in bar options, client layout, style or ownership.
// Disconnect is idempotent. Block suspends both directions; the final Unblock
// supplies current metrics. Host destruction disconnects all clients; callers
// release bindings when clients leave their own lifetime. A nil client or dead
// host returns nil. An invalid axis panics.
func (sv *ScrollView) BindScrollBar(axis layout.Direction, client ScrollBarClient) signal.Handle {
	if axis != layout.DirectionHorizontal && axis != layout.DirectionVertical {
		panic("gui: invalid scrollbar axis")
	}
	if client == nil || !sv.scrollBindingAvailable() {
		return nil
	}
	b := &scrollBarBinding{owner: sv, axis: axis, client: client, dirty: true}
	sv.barBindings = append(sv.barBindings, b)
	request := client.ConnectChange(func(value float32) {
		if b.owner == nil || b.blocks != 0 || b.pushing || !sv.scrollBindingAvailable() {
			return
		}
		b.dirty = true
		if axis == layout.DirectionHorizontal {
			sv.SetScrollX(value)
		} else {
			sv.SetScrollY(value)
		}
		sv.syncBars()
	})
	// Connecting is external code and may destroy the host before returning.
	if b.owner == nil || !sv.scrollBindingAvailable() {
		if request != nil {
			request.Disconnect()
		}
		b.Disconnect()
		return nil
	}
	b.request = request
	sv.syncBars()
	if b.owner == nil || !sv.scrollBindingAvailable() {
		b.Disconnect()
		return nil
	}
	return b
}

type scrollBarState struct{ max, page, value float32 }

type scrollBarBinding struct {
	owner   *ScrollView
	axis    layout.Direction
	client  ScrollBarClient
	request signal.Handle
	blocks  int
	pushing bool
	dirty   bool
	last    scrollBarState
}

func (b *scrollBarBinding) Block() { b.blocks++ }

func (b *scrollBarBinding) Unblock() {
	if b.blocks > 0 {
		b.blocks--
		if b.blocks == 0 && b.owner != nil {
			b.dirty = true
			b.owner.syncBars()
		}
	}
}

func (b *scrollBarBinding) Disconnect() {
	if sv := b.owner; sv != nil {
		b.owner = nil
		i := slices.Index(sv.barBindings, b)
		if i >= 0 {
			sv.barBindings = slices.Delete(sv.barBindings, i, i+1)
		}
		b.client = nil
		if b.request != nil {
			b.request.Disconnect()
			b.request = nil
		}
	}
}

func (b *scrollBarBinding) sync(state scrollBarState, revision uint64) {
	sv := b.owner
	if sv == nil || b.blocks != 0 || !b.dirty && b.last == state {
		return
	}
	b.pushing = true
	defer func() { b.pushing = false }()
	valid := func() bool {
		return b.owner == sv && sv.scrollBindingAvailable() && b.blocks == 0 && sv.barRevision == revision
	}
	b.client.SetMax(state.max)
	if !valid() {
		return
	}
	b.client.SetPageSize(state.page)
	if !valid() {
		return
	}
	b.client.SetValue(state.value)
	if valid() {
		b.last, b.dirty = state, false
	}
}

var _ ScrollBarClient = (*ScrollBar)(nil)

// A host can defer widget destruction until its current frame ends. Stop
// external calls as soon as final host destruction starts, while keeping
// explicitly bound, temporarily detached widgets usable.
func (sv *ScrollView) scrollBindingAvailable() bool {
	if sv.Destroyed() {
		return false
	}
	if host, ok := sv.Root().(interface{ rootState() *rootBase }); ok {
		return !host.rootState().frameDestroyed
	}
	return true
}
