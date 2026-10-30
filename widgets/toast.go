package widgets

import (
	"time"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/core/signal"
	"github.com/golang-gui/goui/gui"
	"github.com/golang-gui/goui/layout"
)

// Toast displays one replaceable notification. Its allocation includes shadow
// space; an ordinary Overlay decides its position. Show starts a presentation,
// Close ends it, and SetChild only updates content. Methods belong to the GUI
// thread. Hidden or unmounted presentations retain content without a live timer.
type Toast struct {
	gui.WidgetBase
	child                   gui.Widget
	padding                 geometry.Insets
	motion                  *gui.MotionEventController
	mounted, showing, timed bool
	revision                uint64
	schedule                uint64
	remaining               time.Duration
	started                 time.Time
	timer                   *gui.Timer
	timeout                 signal.Handle
	closed                  signal.Signal1[uint64]
}

// NewToast creates an inactive, hidden notification with 12 DIP padding.
// It has no timer resources until a timed presentation is mounted.
func NewToast() *Toast {
	t := &Toast{padding: geometry.UniformInsets(12)}
	t.WidgetBase.SetVisible(false)
	t.SetLayoutManager(toastLayout{t})
	t.motion = gui.NewMotionEventController()
	t.motion.ConnectContainsHover(func(bool) { t.syncTimer(time.Now()) })
	t.AddEventController(t.motion)
	t.ConnectContainsFocus(func(bool) { t.syncTimer(time.Now()) })
	t.ConnectMount(func() { t.mounted = true; t.syncTimer(time.Now()) })
	t.ConnectUnmount(func() {
		t.mounted = false
		t.pause(time.Now())
		t.releaseTimer()
	})
	return t
}

func (t *Toast) Child() gui.Widget { return t.child }

// SetChild updates content without starting or restarting a presentation.
func (t *Toast) SetChild(child gui.Widget) {
	if t.Destroyed() || child == t || t.child == child {
		return
	}
	revision := t.revision
	if old := t.child; old != nil {
		t.child = nil
		t.RemoveChild(old)
	}
	if t.Destroyed() || t.revision != revision {
		return
	}
	t.child = child
	if child != nil {
		t.WidgetBase.AddChild(t, child)
		if child.Parent() != t {
			t.child = nil
		}
	}
	t.RequestLayout()
}

func (t *Toast) Padding() geometry.Insets { return t.padding }

// SetPadding changes per-edge inner padding in DIP; invalid edges become zero.
func (t *Toast) SetPadding(padding geometry.Insets) {
	padding = normalizeInsets(padding)
	if !t.Destroyed() && t.padding != padding {
		t.padding = padding
		t.RequestLayout()
	}
}

// Show replaces the current presentation and resets its duration. Nonpositive
// duration stays visible until Close. Content must be non-nil. Replacing or
// closing detaches no content; SetChild(nil) explicitly clears the child slot.
func (t *Toast) Show(content gui.Widget, duration time.Duration) {
	if content == nil {
		panic("widgets: Toast.Show requires content")
	}
	if t.Destroyed() {
		return
	}
	t.revision++
	revision := t.revision
	t.pause(time.Now())
	t.showing = true
	t.timed = duration > 0
	t.remaining = max(0, duration)
	t.SetChild(content)
	if t.Destroyed() || t.revision != revision || !t.showing {
		return
	}
	t.WidgetBase.SetVisible(true)
	t.syncTimer(time.Now())
}

// Close ends the current presentation, hides it and reports Closed once.
// The detached/replaced child is never destroyed by this operation.
func (t *Toast) Close() {
	if t.Destroyed() || !t.showing {
		return
	}
	t.revision++
	t.showing = false
	t.remaining = 0
	t.pause(time.Now())
	t.WidgetBase.SetVisible(false)
	t.closed.Emit(t.revision)
}

// ConnectClosed reports an actual end, including timeout, not a replacement.
// Superseded or destroyed presentations stop notifying remaining listeners.
func (t *Toast) ConnectClosed(fn func()) signal.Handle {
	return t.closed.Connect(func(revision uint64) {
		if !t.Destroyed() && t.revision == revision && !t.showing {
			fn()
		}
	})
}

// SetVisible pauses or resumes an existing presentation. True cannot start or
// resurrect a closed presentation; call Show explicitly for a new request.
func (t *Toast) SetVisible(visible bool) {
	if visible && !t.showing {
		return
	}
	if !visible {
		t.pause(time.Now())
	}
	t.WidgetBase.SetVisible(visible)
	t.syncTimer(time.Now())
}

func (t *Toast) pause(now time.Time) {
	t.schedule++
	if !t.started.IsZero() {
		t.remaining = max(0, t.remaining-now.Sub(t.started))
		t.started = time.Time{}
	}
	if t.timer != nil {
		t.timer.Stop()
	}
}

func (t *Toast) releaseTimer() {
	if t.timeout != nil {
		t.timeout.Disconnect()
		t.timeout = nil
	}
	t.timer = nil
}

func (t *Toast) syncTimer(now time.Time) {
	if t.Destroyed() || !t.showing || !t.mounted || !t.Visible() || t.motion.ContainsHover() || t.ContainsFocus() {
		t.pause(now)
		return
	}
	if !t.timed || !t.started.IsZero() {
		return
	}
	if t.remaining <= 0 {
		t.Close()
		return
	}
	if gui.App == nil {
		return
	}
	if t.timer == nil {
		t.timer = gui.App.NewTimer()
	}
	if t.timeout != nil {
		t.timeout.Disconnect()
	}
	revision := t.revision
	schedule := t.schedule
	t.timeout = t.timer.ConnectTimeout(func() {
		t.expire(revision, schedule)
	})
	if t.timer.StartOnce(t.remaining) == nil {
		t.started = now
	}
}

func (t *Toast) expire(revision, schedule uint64) {
	if !t.Destroyed() && t.revision == revision && t.schedule == schedule && t.showing && !t.started.IsZero() {
		t.started = time.Time{}
		t.Close()
	}
}

func (t *Toast) Paint(p gui.Painter) {
	if !t.showing {
		return
	}
	s := gui.WidgetStyle(t, "toast", "")
	insets := surfaceInsets(s)
	paintSurface(p, geometry.Rectangle{Size: t.Rect().Size}.Insets(insets.Left, insets.Top, insets.Right, insets.Bottom), s)
}

func (t *Toast) Snapshot() gui.WidgetInfo {
	info := t.WidgetBase.Snapshot()
	info.Role = RoleToast
	return info
}

type toastLayout struct{ toast *Toast }

func (l toastLayout) insets() geometry.Insets {
	t := l.toast
	i := surfaceInsets(gui.WidgetStyle(t, "toast", ""))
	return geometry.Insets{Left: i.Left + t.padding.Left, Right: i.Right + t.padding.Right,
		Top: i.Top + t.padding.Top, Bottom: i.Bottom + t.padding.Bottom}
}
func (l toastLayout) Measure(children []layout.Child, c layout.Constraint) layout.Measurement {
	insets := l.insets()
	var child layout.Measurement
	if len(children) != 0 {
		child = children[0].Measure(layout.Loose(c.Insets(insets).Max))
	}
	child.Width += insets.Horizontal()
	child.Height += insets.Vertical()
	if child.HasBaseline {
		child.Baseline += insets.Top
	}
	return child.Constrain(c)
}
func (l toastLayout) Arrange(children []layout.Child, rect geometry.Rectangle) {
	if len(children) == 0 {
		return
	}
	i := l.insets()
	children[0].Arrange(rect.Insets(i.Left, i.Top, i.Right, i.Bottom))
}

var _ gui.Bin = (*Toast)(nil)
