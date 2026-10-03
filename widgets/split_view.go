package widgets

import (
	"math"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/core/signal"
	"github.com/golang-gui/goui/gui"
	"github.com/golang-gui/goui/layout"
)

// SplitView lays out two independently styled panes separated by a draggable
// handle. Horizontal means left/right, Vertical means top/bottom. Nest views
// for more panes. All operations and signals belong to the GUI thread.
type SplitView struct {
	gui.WidgetBase
	direction          layout.Direction
	start, end         *splitPane
	handle             *splitHandle
	preference         splitPreference
	startSize, endSize float32
	resize             signal.Signal3[float32, float32, func() bool]
	changingChildren   bool
	notifyingResize    bool
}

type splitSizing uint8

const (
	splitAuto splitSizing = iota
	splitRatio
	splitStart
	splitEnd
)

// Automatic sizing becomes proportional at the first nonempty two-pane
// arrangement. Constraints clamp allocations, never this preferred value.
type splitPreference struct {
	mode  splitSizing
	value float32
}

func NewSplitView(direction layout.Direction) *SplitView {
	v := &SplitView{direction: layout.DirectionHorizontal}
	v.start, v.end = newSplitPane(), newSplitPane()
	v.handle = newSplitHandle(v)
	v.WidgetBase.AddChild(v, v.start)
	v.WidgetBase.AddChild(v, v.end)
	v.WidgetBase.AddChild(v, v.handle)
	v.SetLayoutManager(&splitLayout{view: v})
	v.SetMainWeight(1)
	v.SetDirection(direction)
	v.ConnectUnmount(v.handle.cancel)
	return v
}

func (v *SplitView) Direction() layout.Direction { return v.direction }
func (v *SplitView) SetDirection(direction layout.Direction) {
	if v.Destroyed() || (direction != layout.DirectionHorizontal && direction != layout.DirectionVertical) || v.direction == direction {
		return
	}
	v.handle.cancel()
	if v.Destroyed() {
		return
	}
	v.direction = direction
	v.handle.updateCursor()
	v.RequestLayout()
}

func (v *SplitView) StartChild() gui.Widget { return v.start.child }
func (v *SplitView) EndChild() gui.Widget   { return v.end.child }

// SetStartChild detaches the old child without destroying it. A new child must
// be detached, alive and distinct from the other pane (and this view).
func (v *SplitView) SetStartChild(child gui.Widget) { v.setChild(v.start, child) }
func (v *SplitView) SetEndChild(child gui.Widget)   { v.setChild(v.end, child) }
func (v *SplitView) setChild(pane *splitPane, child gui.Widget) {
	if v.Destroyed() || v.changingChildren || pane.child == child {
		return
	}
	if child != nil {
		if child.Parent() != nil || child.Root() != nil {
			return
		}
		if dead, ok := child.(interface{ Destroyed() bool }); ok && dead.Destroyed() {
			return
		}
		for parent := gui.Widget(v); parent != nil; parent = parent.Parent() {
			if parent == child {
				return
			}
		}
	}
	v.changingChildren = true
	defer func() { v.changingChildren = false }()
	v.handle.cancel()
	if v.Destroyed() {
		return
	}
	old := pane.child
	pane.child = nil
	if old != nil {
		pane.RemoveChild(old)
	}
	if v.Destroyed() || pane.Destroyed() {
		return
	}
	// Unmount/resize callbacks may have attached or destroyed the requested child.
	if child != nil {
		if child.Parent() != nil || child.Root() != nil {
			v.RequestLayout()
			return
		}
		if dead, ok := child.(interface{ Destroyed() bool }); ok && dead.Destroyed() {
			v.RequestLayout()
			return
		}
	}
	pane.child = child
	if child != nil {
		pane.WidgetBase.AddChild(pane, child)
		if child.Parent() != pane {
			pane.child = nil
		}
	}
	v.RequestLayout()
}

// SetStartSize keeps the preferred start extent in DIP on window resize.
// SetEndSize and SetRatio select different policies; the last setter wins.
// Non-finite values are ignored; negative sizes become zero.
func (v *SplitView) SetStartSize(size float32) { v.setPreference(splitStart, size) }
func (v *SplitView) SetEndSize(size float32)   { v.setPreference(splitEnd, size) }

// SetRatio keeps the preferred start share of the space excluding the 1 DIP
// separator. Values are clamped to [0,1]; non-finite values are ignored.
// Without any sizing setter, the initial share comes from child measurement.
func (v *SplitView) SetRatio(ratio float32) { v.setPreference(splitRatio, ratio) }
func (v *SplitView) setPreference(mode splitSizing, value float32) {
	if v.Destroyed() || !splitFinite(value) {
		return
	}
	value = max(0, value)
	if mode == splitRatio {
		value = min(1, value)
	}
	next := splitPreference{mode: mode, value: value}
	if v.preference == next {
		return
	}
	v.handle.cancel()
	if v.Destroyed() {
		return
	}
	v.preference = next
	v.RequestLayout()
}

// Sizes returns the latest allocation in DIP (zero before layout). It does not
// expose the preferred value, which may be temporarily clamped by constraints.
func (v *SplitView) Sizes() (start, end float32) { return v.startSize, v.endSize }

// ConnectResize receives user adjustments, including restoration on canceled
// drag. Measure, window resize and programmatic setters do not emit it. Layout
// is requested before notification; children are arranged on the next layout.
func (v *SplitView) ConnectResize(fn func(start, end float32)) signal.Handle {
	return v.resize.Connect(func(start, end float32, alive func() bool) {
		if alive() {
			fn(start, end)
		}
	})
}

func (v *SplitView) active() bool { return v.start.present() && v.end.present() }
func (v *SplitView) axis(size geometry.Size) float32 {
	if v.direction == layout.DirectionVertical {
		return size.Height
	}
	return size.Width
}
func (v *SplitView) size(main, cross float32) geometry.Size {
	if v.direction == layout.DirectionVertical {
		return geometry.Size{Width: cross, Height: main}
	}
	return geometry.Size{Width: main, Height: cross}
}
func (v *SplitView) point(point geometry.Point) float32 {
	if v.direction == layout.DirectionVertical {
		return point.Y
	}
	return point.X
}
func (v *SplitView) limits(available float32) (float32, float32) {
	return splitBounds(available, v.start.limits(v), v.end.limits(v))
}
func (v *SplitView) available() float32 { return max(0, v.axis(v.Rect().Size)-1) }

func (v *SplitView) adjust(start float32) {
	if v.Destroyed() || !v.active() {
		return
	}
	a := v.available()
	lo, hi := v.limits(a)
	start = min(hi, max(lo, start))
	if start == v.startSize {
		return
	}
	switch v.preference.mode {
	case splitStart:
		v.preference.value = start
	case splitEnd:
		v.preference.value = a - start
	default:
		if a <= 0 {
			return
		}
		v.preference = splitPreference{mode: splitRatio, value: start / a}
	}
	v.startSize, v.endSize = start, a-start
	v.RequestLayout()
	v.emitResize()
}

func (v *SplitView) emitResize() {
	// A callback may replace content or close its host, canceling this drag.
	// The resulting restoration must not recursively notify that same callback.
	if v.notifyingResize {
		return
	}
	v.notifyingResize = true
	defer func() { v.notifyingResize = false }()
	root := v.Root()
	v.resize.Emit(v.startSize, v.endSize, func() bool { return !v.Destroyed() && v.Root() == root })
}

func (v *SplitView) Snapshot() gui.WidgetInfo {
	info := v.WidgetBase.Snapshot()
	info.Role, info.Children = RoleSplitView, nil
	if child := v.StartChild(); child != nil {
		info.Children = append(info.Children, child.Snapshot())
	}
	if child := v.EndChild(); child != nil {
		info.Children = append(info.Children, child.Snapshot())
	}
	if v.handle.Visible() {
		info.Children = append(info.Children, v.handle.Snapshot())
	}
	return info
}

// The pane is a structural clip boundary, not a public styling container.
type splitPane struct {
	gui.WidgetBase
	child gui.Widget
}

func newSplitPane() *splitPane {
	p := new(splitPane)
	p.SetLayoutManager(layout.NewFillLayout())
	return p
}
func (p *splitPane) present() bool { return p.child != nil && p.child.Visible() }

type splitLimit struct{ min, max float32 }

func (p *splitPane) limits(v *SplitView) splitLimit {
	if !p.present() {
		return splitLimit{max: layout.Inf}
	}
	lo, hi := v.axis(p.child.MinSize()), v.axis(p.child.MaxSize())
	if hi <= 0 {
		hi = layout.Inf
	}
	return splitLimit{min: lo, max: max(lo, hi)}
}
func splitFinite(value float32) bool {
	return !math.IsNaN(float64(value)) && !math.IsInf(float64(value), 0)
}
