package widgets

import (
	"math"
	"time"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/core/signal"
	"github.com/golang-gui/goui/gui"
	"github.com/golang-gui/goui/layout"
	"github.com/golang-gui/goui/platform/graphics"
	"github.com/golang-gui/goui/style"
)

// ProgressShape selects the visual shape, independently of the progress mode.
type ProgressShape uint8

const (
	ProgressLinear ProgressShape = iota
	ProgressCircular
)

// ProgressInfoKey identifies read-only progress data in WidgetInfo.Attributes.
const ProgressInfoKey = "goui.progress"

// ProgressInfo is detached snapshot data, not an adjustable range. Value is
// absent in indeterminate mode: the saved value is not the task's progress.
type ProgressInfo struct {
	Shape         ProgressShape `json:"shape"`
	Indeterminate bool          `json:"indeterminate"`
	Value         *float32      `json:"value,omitempty"`
}

// ProgressBar displays a task's completion fraction or indefinite activity.
// It does not manage the task, accept input or emit completion notifications.
// A circular indeterminate ProgressBar also serves as a spinner.
type ProgressBar struct {
	gui.WidgetBase
	value         float32
	shape         ProgressShape
	indeterminate bool
	thickness     float32
	mounted       bool
	frame         signal.Handle
	started       time.Time
	elapsed       time.Duration
}

// NewProgressBar creates a determinate linear indicator at zero, 4 DIP thick.
// Natural sizes are 160 x Thickness DIP (linear) and 24 x 24 DIP (circular).
func NewProgressBar() *ProgressBar {
	p := &ProgressBar{thickness: 4}
	p.SetLayoutManager(progressLayout{bar: p})
	p.ConnectMount(func() { p.mounted = true; p.syncAnimation() })
	p.ConnectUnmount(func() {
		p.mounted = false
		p.stopAnimation()
	})
	return p
}

func (p *ProgressBar) Value() float32 { return p.value }

// SetValue clamps finite values to [0,1]; non-finite values become zero. It
// preserves the mode, including the saved value while indeterminate.
func (p *ProgressBar) SetValue(value float32) {
	if math.IsNaN(float64(value)) || math.IsInf(float64(value), 0) {
		value = 0
	}
	value = max(0, min(value, 1))
	if p.value == value {
		return
	}
	p.value = value
	p.RequestPaint()
}

func (p *ProgressBar) Shape() ProgressShape { return p.shape }

// SetShape selects linear or circular drawing; unknown values become linear.
// An active indeterminate clock is preserved.
func (p *ProgressBar) SetShape(shape ProgressShape) {
	if shape != ProgressCircular {
		shape = ProgressLinear
	}
	if p.shape == shape {
		return
	}
	p.shape = shape
	p.RequestLayout()
}

func (p *ProgressBar) Indeterminate() bool { return p.indeterminate }

// SetIndeterminate switches activity mode without changing Value. Repeating
// the same mode does not reset the animation. Leaving activity disconnects it.
func (p *ProgressBar) SetIndeterminate(indeterminate bool) {
	if p.indeterminate == indeterminate {
		return
	}
	p.indeterminate = indeterminate
	p.syncAnimation()
	p.RequestPaint()
}

func (p *ProgressBar) Thickness() float32 { return p.thickness }

// SetThickness sets the preferred bar height / ring width in DIP. Negative
// and non-finite values become zero; drawing clamps it to available space.
func (p *ProgressBar) SetThickness(thickness float32) {
	thickness = progressNonnegative(thickness)
	if p.thickness == thickness {
		return
	}
	p.thickness = thickness
	p.syncAnimation()
	p.RequestLayout()
}

func (p *ProgressBar) SetVisible(visible bool) {
	p.WidgetBase.SetVisible(visible)
	p.syncAnimation()
}

func (p *ProgressBar) Arrange(rect geometry.Rectangle) {
	p.WidgetBase.Arrange(rect)
	p.syncAnimation()
}

func (p *ProgressBar) Paint(painter gui.Painter) {
	if !p.Visible() || p.thickness <= 0 || p.Rect().Width <= 0 || p.Rect().Height <= 0 {
		return
	}
	elapsed := time.Duration(0)
	if p.frame != nil {
		elapsed = p.elapsed
	} else if p.shape == ProgressLinear {
		elapsed = 450 * time.Millisecond // A visible static pose if animation cannot run.
	}
	p.paint(painter, elapsed)
}

// Drawing takes the cached frame time; rendering never advances the animation.
func (p *ProgressBar) paint(painter gui.Painter, elapsed time.Duration) {
	name := p.StyleName()
	if name == "" {
		name = "progress-bar"
	}
	s := gui.ResolveStyle(name, "", style.Normal)
	background, _ := s.BackgroundColor()
	foreground, _ := s.ForegroundColor()
	if p.shape == ProgressCircular {
		edge := min(p.Rect().Width, p.Rect().Height)
		center := geometry.Point{X: p.Rect().Width / 2, Y: p.Rect().Height / 2}
		outer, thickness := edge/2, min(p.thickness, edge/2)
		if background != nil {
			painter.FillPath(progressRingPath(center, outer, thickness, 0, 2*math.Pi), graphics.ColorOf(background))
		}
		start, sweep := -math.Pi/2, 2*math.Pi*float64(p.value)
		if p.indeterminate {
			start, sweep = progressCircularMotion(elapsed)
		}
		if foreground != nil && sweep > 0 {
			painter.FillPath(progressRingPath(center, outer, thickness, start, sweep), graphics.ColorOf(foreground))
		}
		return
	}
	thickness := min(p.thickness, p.Rect().Height)
	rect := geometry.Rect(0, (p.Rect().Height-thickness)/2, p.Rect().Width, thickness)
	radius, _ := s.Radius()
	radius = min(progressNonnegative(radius), thickness/2)
	if background != nil {
		painter.FillRoundRect(rect, radius, graphics.ColorOf(background))
	}
	if p.indeterminate {
		position, length := progressLinearMotion(elapsed)
		rect.X, rect.Width = rect.Width*position, rect.Width*length
	} else {
		rect.Width *= p.value
	}
	if foreground != nil && rect.Width > 0 {
		painter.FillRoundRect(rect, min(radius, rect.Width/2), graphics.ColorOf(foreground))
	}
}

func (p *ProgressBar) Snapshot() gui.WidgetInfo {
	info := p.WidgetBase.Snapshot()
	info.Role = RoleProgressBar
	progress := ProgressInfo{Shape: p.shape, Indeterminate: p.indeterminate}
	if !p.indeterminate {
		value := p.value
		progress.Value = &value
	}
	info.SetAttribute(ProgressInfoKey, progress)
	return info
}

func (p *ProgressBar) canAnimate() bool {
	if !p.indeterminate || p.Destroyed() || !p.mounted || p.Root() == nil ||
		p.thickness <= 0 || p.Rect().Width <= 0 || p.Rect().Height <= 0 {
		return false
	}
	for widget := gui.Widget(p); widget != nil; widget = widget.Parent() {
		if !widget.Visible() {
			return false
		}
	}
	if host, ok := p.Root().(interface{ Visible() bool }); ok && !host.Visible() {
		return false
	}
	return true
}

func (p *ProgressBar) syncAnimation() {
	if !p.canAnimate() {
		p.stopAnimation()
		return
	}
	if p.frame != nil {
		return
	}
	root := p.Root()
	p.started = time.Now()
	p.elapsed = 0
	p.frame = root.ConnectFrame(func(now time.Time) {
		if p.Root() != root {
			p.stopAnimation()
			return
		}
		p.tick(now)
	})
	if root.RequestPaint() != nil {
		p.stopAnimation()
	}
}

func (p *ProgressBar) tick(now time.Time) {
	if !p.canAnimate() {
		p.stopAnimation()
		return
	}
	p.elapsed = max(0, now.Sub(p.started))
	if p.Root().RequestPaint() != nil {
		p.stopAnimation()
	}
}

func (p *ProgressBar) stopAnimation() {
	if p.frame != nil {
		p.frame.Disconnect()
		p.frame = nil
	}
	p.started = time.Time{}
	p.elapsed = 0
}

// Motion is a pure elapsed-time function, not an accumulation of timer ticks.
func progressLinearMotion(elapsed time.Duration) (position, length float32) {
	phase := float64(max(0, elapsed)%(1800*time.Millisecond)) / float64(1800*time.Millisecond)
	// The 30% segment enters from outside the left edge and leaves completely
	// on the right. Both endpoints are invisible at the wrap; no reverse leg.
	return float32(-.3 + 1.3*phase), .3
}

// WidgetBase owns visibility and min/max/parent constraint handling.
type progressLayout struct{ bar *ProgressBar }

func (l progressLayout) Measure(_ []layout.Child, c layout.Constraint) layout.Measurement {
	size := geometry.Size{Width: 160, Height: l.bar.thickness}
	if l.bar.shape == ProgressCircular {
		size = geometry.Size{Width: 24, Height: 24}
	}
	return layout.Measured(c.Clamp(size))
}

func (progressLayout) Arrange([]layout.Child, geometry.Rectangle) {}

func progressNonnegative(value float32) float32 {
	if value < 0 || math.IsNaN(float64(value)) || math.IsInf(float64(value), 0) {
		return 0
	}
	return value
}

func progressCircularMotion(elapsed time.Duration) (start, sweep float64) {
	phase := float64(max(0, elapsed)%(1400*time.Millisecond)) / float64(1400*time.Millisecond)
	// A fixed quarter-circle rotates clockwise; neither endpoint reverses.
	return -math.Pi/2 + 2*math.Pi*phase, math.Pi / 2
}

// A single closed contour includes both semicircular caps, so transparent
// foregrounds are composited once. Near-full arcs join when their caps meet.
func progressRingPath(center geometry.Point, outer, thickness float32, start, sweep float64) graphics.Path {
	inner := max(0, outer-thickness)
	middle, half := float64(outer-thickness/2), thickness/2
	full := sweep >= 2*math.Pi || sweep > math.Pi &&
		2*math.Pi-sweep <= 2*math.Asin(min(1, float64(half)/middle))
	if full {
		sweep = 2 * math.Pi
	}
	first := progressCirclePoint(center, outer, start)
	path := graphics.MoveTo(first.X, first.Y)
	path = progressPathArc(path, center, outer, start, sweep)
	end := progressCirclePoint(center, inner, start+sweep)
	if full {
		path = path.LineTo(end.X, end.Y)
	} else {
		path = path.ArcTo(half, half, 0, 0, 1, end.X, end.Y)
	}
	if inner > 0 {
		path = progressPathArc(path, center, inner, start+sweep, -sweep)
	}
	if !full {
		path = path.ArcTo(half, half, 0, 0, 1, first.X, first.Y)
	}
	return path.Close()
}

func progressCirclePoint(center geometry.Point, radius float32, angle float64) geometry.Point {
	return geometry.Point{
		X: center.X + radius*float32(math.Cos(angle)),
		Y: center.Y + radius*float32(math.Sin(angle)),
	}
}

func progressPathArc(path graphics.Path, center geometry.Point, radius float32, start, sweep float64) graphics.Path {
	// Splitting into at most half-turns also represents a complete circle: an
	// SVG-style ArcTo with identical endpoints cannot do so by itself.
	steps := max(1, int(math.Ceil(math.Abs(sweep)/math.Pi)))
	direction := float32(0)
	if sweep > 0 {
		direction = 1
	}
	for i := 1; i <= steps; i++ {
		point := progressCirclePoint(center, radius, start+sweep*float64(i)/float64(steps))
		path = path.ArcTo(radius, radius, 0, 0, direction, point.X, point.Y)
	}
	return path
}
