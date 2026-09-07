package layout

import "github.com/golang-gui/goui/core/geometry"

// Inf is the sentinel for an unbounded constraint axis (avoids raw math.Inf and
// its NaN pitfalls). Used e.g. by a scroll view on its scroll axis.
const Inf float32 = 1e9

// Constraint is the size range a parent hands a child during Measure: the child
// must return a size within [Min, Max] on each axis. Tight (Min==Max) forces a
// size (Window root, fill children); Loose (Min==0) lets the child size to its
// content (Popup, content-driven).
type Constraint struct {
	Min, Max geometry.Size
}

func Tight(s geometry.Size) Constraint   { return Constraint{Min: s, Max: s} }
func Loose(max geometry.Size) Constraint { return Constraint{Max: max} }
func Unbounded() Constraint {
	return Constraint{Max: geometry.Size{Width: Inf, Height: Inf}}
}

// Clamp fits s into [Min, Max] per axis. An over-constrained axis (Min>Max) is
// normalized so Max wins.
func (c Constraint) Clamp(s geometry.Size) geometry.Size {
	return geometry.Size{
		Width:  clamp(s.Width, c.Min.Width, c.Max.Width),
		Height: clamp(s.Height, c.Min.Height, c.Max.Height),
	}
}

// Inset returns the constraint for content inside equal padding on every edge.
// Unlike geometry.Size.Inset, it preserves Inf so an unbounded axis does not
// accidentally become a very large but finite axis after subtracting padding.
func (c Constraint) Inset(n float32) Constraint {
	return Constraint{
		Min: geometry.Size{
			Width:  insetExtent(c.Min.Width, n),
			Height: insetExtent(c.Min.Height, n),
		},
		Max: geometry.Size{
			Width:  insetExtent(c.Max.Width, n),
			Height: insetExtent(c.Max.Height, n),
		},
	}
}

func insetExtent(v, n float32) float32 {
	if v >= Inf {
		return Inf
	}
	v -= 2 * n
	return max(0, v)
}

// Measurement is the geometry a child reports for one specific Constraint.
// Baseline, when present, is the first typographic baseline measured down from
// the top edge of Size. Keeping it beside Size prevents callers from combining
// a baseline with geometry produced under different constraints.
type Measurement struct {
	geometry.Size
	Baseline    float32
	HasBaseline bool
}

// Measured constructs a size-only Measurement. It is the common result for
// non-text content and keeps size-only widget implementations concise.
func Measured(size geometry.Size) Measurement {
	return Measurement{Size: size}
}

// MeasuredWithBaseline constructs a Measurement with a first typographic
// baseline relative to its top edge.
func MeasuredWithBaseline(size geometry.Size, baseline float32) Measurement {
	return Measurement{Size: size, Baseline: baseline, HasBaseline: true}
}

// Constrain clamps the measured size while preserving its content baseline.
// A hard parent constraint may clip content above or below that baseline; the
// baseline must continue to describe where the content is actually drawn.
func (m Measurement) Constrain(c Constraint) Measurement {
	m.Size = c.Clamp(m.Size)
	return m
}

func clamp(v, lo, hi float32) float32 {
	if lo > hi {
		lo = hi // over-constrained: Max wins
	}
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

type Child interface {
	Measure(c Constraint) Measurement
	Arrange(rect geometry.Rectangle)
	// MainWeight is the child's share of leftover main-axis space (0 = hug).
	// Linear-style layouts honor it; fill ignores it. It is the one universal
	// per-child layout hint carried on the Child contract.
	MainWeight() float32
}

type LayoutManager interface {
	Measure(children []Child, c Constraint) Measurement
	Arrange(children []Child, rect geometry.Rectangle)
}

type Direction int

const (
	DirectionHorizontal Direction = iota
	DirectionVertical
)

// MainAlign places children as a block along the main axis when free space is
// left and no MainWeight consumes it. Container-level (see DesignLayout §12).
type MainAlign int

const (
	MainStart        MainAlign = iota // pack at the start, reading order; default
	MainCenter                        // center the block
	MainEnd                           // pack at the end
	MainSpaceBetween                  // first at start, last at end, gaps split evenly
)

// CrossAlign sizes and positions children on the cross axis. It is a
// container-level policy relative to the layout direction.
type CrossAlign int

const (
	CrossStart    CrossAlign = iota // child hugs, sits at the cross start; default
	CrossCenter                     // child hugs, centered on the cross axis
	CrossEnd                        // child hugs, at the cross end
	CrossStretch                    // child fills the whole cross extent
	CrossBaseline                   // horizontal only: align first typographic baselines
)
