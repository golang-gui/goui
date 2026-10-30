package ui

import (
	"errors"
	"sync"
	"time"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/core/optional"
	"github.com/golang-gui/goui/core/signal"
	"github.com/golang-gui/goui/gui"
	baseui "github.com/golang-gui/goui/ui"
	"github.com/golang-gui/goui/widgets"
)

// ErrToastUnavailable means the handle has no live Toast declaration.
var ErrToastUnavailable = errors.New("widgets/ui: toast is not bound")

// ToastHandle submits notifications to one persistent Toast declaration. Its
// zero value is ready for binding. Show and Close may be called from any
// goroutine; do not copy the handle after first use. Submitted Views must not be
// mutated concurrently. Requests replace one another; there is no queue.
type ToastHandle struct {
	mu      sync.Mutex
	binding *toastBinding
}

type toastRequest struct {
	view     baseui.View
	duration time.Duration
	serial   uint64
}
type toastBinding struct {
	handle  *ToastHandle
	update  func()
	request *toastRequest
	serial  uint64
}

// Show asynchronously replaces the content and restarts its duration. A
// nonpositive duration stays until Close. Nil content is invalid; an unbound
// or released handle returns ErrToastUnavailable without retaining the View.
// Accepted requests may be superseded or discarded on declaration release
// before reconciliation; success is request acceptance, not display completion.
func (h *ToastHandle) Show(view baseui.View, duration time.Duration) error {
	if view == nil {
		return errors.New("widgets/ui: Toast.Show requires content")
	}
	return h.submit(view, duration)
}

// Close asynchronously clears the current request. It returns
// ErrToastUnavailable when no live declaration owns this handle.
func (h *ToastHandle) Close() error { return h.submit(nil, 0) }

func (h *ToastHandle) submit(view baseui.View, duration time.Duration) error {
	if h == nil {
		return ErrToastUnavailable
	}
	h.mu.Lock()
	b := h.binding
	if b == nil {
		h.mu.Unlock()
		return ErrToastUnavailable
	}
	b.serial++
	b.request = nil
	if view != nil {
		b.request = &toastRequest{view: view, duration: duration, serial: b.serial}
	}
	update := b.update
	h.mu.Unlock()
	update()
	return nil
}

func (b *toastBinding) snapshot() *toastRequest {
	b.handle.mu.Lock()
	defer b.handle.mu.Unlock()
	if b.handle.binding != b || b.request == nil {
		return nil
	}
	request := *b.request
	return &request
}
func (b *toastBinding) current(serial uint64) bool {
	b.handle.mu.Lock()
	defer b.handle.mu.Unlock()
	return b.handle.binding == b && b.request != nil && b.request.serial == serial
}
func (b *toastBinding) dismiss(serial uint64) {
	b.handle.mu.Lock()
	if b.handle.binding != b || b.request == nil || b.request.serial != serial {
		b.handle.mu.Unlock()
		return
	}
	b.request = nil
	update := b.update
	b.handle.mu.Unlock()
	update()
}
func (b *toastBinding) release() {
	b.handle.mu.Lock()
	defer b.handle.mu.Unlock()
	if b.handle.binding == b {
		b.handle.binding = nil
	}
	b.request = nil
}

// ToastView owns a presentation slot without a Widget until Show supplies
// content. OverlayItem controls its position. Give the declaration a stable ID
// when it shares a changing list; ordinary rebuilds do not replay requests.
type ToastView struct {
	baseui.ComposeBase[ToastView]
	handle    *ToastHandle
	styleName string
	padding   optional.Optional[geometry.Insets]
}

// Toast binds one non-nil handle. Binding the same handle to two live
// declarations is a programming error. The declaration exposes its handle to
// descendant ObjectView lookup when named with ID.
func Toast(handle *ToastHandle) *ToastView {
	if handle == nil {
		panic("widgets/ui: Toast requires a handle")
	}
	v := &ToastView{handle: handle}
	v.Self = v
	return v
}

// Style selects the toast surface's style name, not its content's styles.
func (v *ToastView) Style(name string) *ToastView { v.styleName = name; return v }
func (v *ToastView) Padding(padding float32) *ToastView {
	return v.Paddings(geometry.UniformInsets(padding))
}
func (v *ToastView) Paddings(padding geometry.Insets) *ToastView {
	v.padding.SetValue(padding)
	return v
}

func (v *ToastView) Build(ctx baseui.BuildContext) baseui.View {
	b, _ := ctx.State().(*toastBinding)
	if b == nil || b.handle != v.handle {
		if b != nil {
			b.release()
		}
		b = &toastBinding{handle: v.handle, update: ctx.RequestUpdate}
		v.handle.mu.Lock()
		if v.handle.binding != nil {
			v.handle.mu.Unlock()
			panic("widgets/ui: ToastHandle already bound")
		}
		v.handle.binding = b
		v.handle.mu.Unlock()
		ctx.SetState(b)
	}
	request := b.snapshot()
	if request == nil {
		return nil
	}
	view := &toastContentView{binding: b, request: *request, padding: v.padding}
	view.Self = view
	return view.Visible(true).Style(v.styleName)
}
func (*ToastView) Object(ctx baseui.BuildContext) any {
	if b, ok := ctx.State().(*toastBinding); ok {
		return b.handle
	}
	return nil
}
func (*ToastView) Release(ctx baseui.BuildContext) {
	if b, ok := ctx.State().(*toastBinding); ok {
		b.release()
	}
}

type toastContentView struct {
	baseui.ViewBase[toastContentView]
	binding *toastBinding
	request toastRequest
	padding optional.Optional[geometry.Insets]
}
type toastContentState struct {
	closed  signal.Handle
	binding *toastBinding
	shown   uint64
	padding geometry.Insets
}

func (*toastContentView) Mount(ctx baseui.BuildContext) gui.Widget {
	t := widgets.NewToast()
	s := new(toastContentState)
	s.padding = t.Padding()
	s.closed = t.ConnectClosed(func() {
		if s.binding != nil {
			s.binding.dismiss(s.shown)
		}
	})
	ctx.SetState(s)
	return t
}
func (v *toastContentView) Update(ctx baseui.BuildContext, widget gui.Widget) {
	t, s := widget.(*widgets.Toast), ctx.State().(*toastContentState)
	s.binding = v.binding
	t.SetPadding(v.padding.ValueOr(s.padding))
	request := v.request
	ctx.UpdateChild(t, request.view, func(child gui.Widget) {
		if !v.binding.current(request.serial) || t.Destroyed() {
			return
		}
		if child == nil {
			if s.shown == request.serial {
				t.SetVisible(false)
			} else {
				t.Close()
			}
			return
		}
		if s.shown == request.serial {
			t.SetVisible(true)
			return
		}
		ctx.AfterUpdate(func() {
			if !t.Destroyed() && v.binding.current(request.serial) && t.Child() != nil {
				s.shown = request.serial
				t.Show(t.Child(), request.duration)
			}
		})
	})
}
func (*toastContentView) Unmount(ctx baseui.BuildContext, widget gui.Widget) {
	s := ctx.State().(*toastContentState)
	s.closed.Disconnect()
	s.binding = nil
	widget.(*widgets.Toast).Close()
}

var _ baseui.ObjectView = (*ToastView)(nil)
var _ baseui.WidgetView = (*toastContentView)(nil)
