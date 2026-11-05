package mobile

import (
	"fmt"
	"image"
	"math"
	"sync"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/platform/common"
	"github.com/golang-gui/goui/platform/events"
)

// Window adopts the single content host provided by the mobile OS. Its lifetime
// is independent of Activity/Scene attachment and native drawing targets. Calls
// are confined to the GOUI thread; native ingress only revokes target leases and
// posts observations to that thread.
type Window struct {
	loop                                         *EventLoop
	handler                                      events.EventHandler
	native                                       Host
	source                                       *Source
	target                                       *surfaceTarget
	state                                        common.SurfaceState
	size                                         events.SizeEvent
	visible, destroyed, paintDirty, paintPending bool
}

// Host is the Go platform adapter. No native window protocol crosses this interface.
type Host interface {
	SetVisible(bool) error
	Close()
}

// NewWindow creates a logical content window without waiting for native geometry
// or a drawable. size is a validated preference, not an observed mobile size.
// The initial implementation supports one opaque content window per process.
func NewWindow(loop *EventLoop, size geometry.Size, handler events.EventHandler, options common.WindowOptions, host Host) (*Window, error) {
	if loop == nil {
		return nil, fmt.Errorf("mobile: event loop is nil")
	}
	if err := common.ValidateWindowSize(size); err != nil {
		return nil, err
	}
	if err := options.Validate(); err != nil {
		return nil, err
	}
	if options.Transparent {
		return nil, common.ErrUnsupported
	}
	if host == nil {
		return nil, fmt.Errorf("mobile: nil platform adapter")
	}
	w := &Window{loop: loop, handler: handler, native: host, paintDirty: true}
	w.source = &Source{window: w}
	return w, nil
}

func (*Window) Parent() common.Window         { return nil }
func (*Window) SetParent(common.Window) error { return common.ErrUnsupported }
func (*Window) Title() string                 { return "" }
func (*Window) SetTitle(string) error         { return common.ErrUnsupported }
func (*Window) Transparent() bool             { return false }

func (w *Window) SurfaceState() common.SurfaceState {
	s := w.state
	s.Ready = !w.destroyed && w.target != nil && w.target.ready()
	return s
}
func (w *Window) NativeHandle() uintptr {
	if !w.SurfaceState().Ready {
		return 0
	}
	return w.target.handle
}

// AcquireTarget borrows the current target until Release. Native revocation
// rejects new borrowers immediately and waits for existing borrowers, without
// waiting for GOUI event dispatch. Finish painting before destroying Window.
func (w *Window) AcquireTarget() (common.NativeTarget, error) {
	if w.destroyed || w.target == nil {
		return nil, common.ErrUnavailable
	}
	target := w.target
	if !target.users.TryRLock() {
		return nil, common.ErrUnavailable
	}
	if !target.ready() {
		target.users.RUnlock()
		return nil, common.ErrUnavailable
	}
	return &targetLease{target: target}, nil
}

type targetLease struct{ target *surfaceTarget }

func (l *targetLease) NativeHandle() uintptr {
	if l.target == nil {
		return 0
	}
	return l.target.handle
}
func (l *targetLease) Generation() uint64 {
	if l.target == nil {
		return 0
	}
	return l.target.generation
}
func (l *targetLease) Release() {
	if l.target != nil {
		target := l.target
		l.target = nil
		target.users.RUnlock()
	}
}
func (w *Window) Show() error {
	if w.destroyed {
		return common.ErrUnavailable
	}
	return w.native.SetVisible(true)
}
func (w *Window) Hide() error {
	if w.destroyed {
		return common.ErrUnavailable
	}
	return w.native.SetVisible(false)
}
func (w *Window) RequestClose() error {
	if w.destroyed {
		return common.ErrUnavailable
	}
	w.loop.Post(func() { w.emit(events.CloseEvent{}) })
	return nil
}
func (w *Window) Destroy() {
	if w.destroyed {
		return
	}
	w.destroyed = true
	if w.target != nil {
		w.target.revoke()
		w.target = nil
	}
	if w.source != nil {
		w.source.Close()
	}
	w.native.Close()
	w.paintDirty, w.paintPending = false, false
	if w.handler != nil {
		w.handler(events.DestroyEvent{})
	}
	w.handler = nil
}

func (w *Window) RequestPaint() error {
	if w.destroyed {
		return common.ErrUnavailable
	}
	w.paintDirty = true
	if w.paintPending || !w.visible || !w.SurfaceState().Ready {
		return nil
	}
	w.paintPending = true
	generation := w.state.Generation
	w.loop.Post(func() {
		w.paintPending = false
		if w.destroyed {
			return
		}
		if generation != w.state.Generation {
			w.RequestPaint()
			return
		}
		if !w.visible || !w.SurfaceState().Ready {
			return
		}
		if w.paintDirty {
			w.paintDirty = false
			w.emit(events.PaintEvent{})
		}
	})
	return nil
}

func (w *Window) Draw(img image.Image) error {
	if w.destroyed || !w.visible || w.target == nil {
		return common.ErrUnavailable
	}
	if img == nil || img.Bounds().Dx() != int(w.size.PixelWidth) || img.Bounds().Dy() != int(w.size.PixelHeight) {
		return common.ErrUnavailable
	}
	return w.target.draw(img)
}

func (w *Window) attach(target *surfaceTarget, width, height int32, scale float32) {
	if w.destroyed || !target.ready() || target.generation <= w.state.Generation {
		return
	}
	if width <= 0 || height <= 0 || scale <= 0 || math.IsNaN(float64(scale)) || math.IsInf(float64(scale), 0) {
		target.revoke()
		return
	}
	if override := common.GetPreferScale(); override > 0 {
		scale = override
	}
	w.target, w.state = target, common.SurfaceState{Ready: true, Generation: target.generation}
	w.size = events.SizeEvent{Width: float32(width) / scale, Height: float32(height) / scale, PixelWidth: float32(width), PixelHeight: float32(height)}
	w.emit(events.SurfaceEvent{State: w.SurfaceState()})
	if w.destroyed {
		return
	}
	w.emit(w.size)
	if !w.destroyed {
		w.RequestPaint()
	}
}
func (w *Window) detach(target *surfaceTarget) {
	if w.destroyed || w.target != target {
		return
	}
	w.target = nil
	w.state.Ready = false
	w.paintDirty = true
	w.emit(events.SurfaceEvent{State: w.state})
}
func (w *Window) setVisible(visible bool) {
	if w.destroyed || w.visible == visible {
		return
	}
	w.visible = visible
	w.emit(events.VisibilityEvent{Visible: visible})
	if !w.destroyed && visible {
		w.RequestPaint()
	}
}
func (w *Window) emit(event events.Event) {
	if !w.destroyed && w.handler != nil {
		w.handler(event)
	}
}

// A lease protects native lifetime, independently of event-loop responsiveness.
// Native destruction waits only for a present already in progress. present must
// not synchronously wait for the system UI thread or retain Go image storage.
type surfaceTarget struct {
	mu         sync.Mutex
	users      sync.RWMutex
	handle     uintptr
	generation uint64
	alive      bool
	present    func(image.Image) error
}

func (t *surfaceTarget) ready() bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.alive && t.handle != 0
}
func (t *surfaceTarget) revoke() {
	t.mu.Lock()
	t.alive = false
	t.mu.Unlock()
	t.users.Lock()
	t.users.Unlock()
}
func (t *surfaceTarget) draw(img image.Image) error {
	if !t.users.TryRLock() {
		return common.ErrUnavailable
	}
	defer t.users.RUnlock()
	t.mu.Lock()
	defer t.mu.Unlock()
	if !t.alive {
		return common.ErrUnavailable
	}
	return t.present(img)
}

var _ common.Window = (*Window)(nil)
