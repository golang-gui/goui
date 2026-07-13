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
	Measure(c Constraint) geometry.Size
	Arrange(rect geometry.Rectangle)
	// MainWeight is the child's share of leftover main-axis space (0 = hug).
	// Linear-style layouts honor it; fill ignores it. It is the one universal
	// per-child layout hint carried on the Child contract.
	MainWeight() float32
}

type LayoutManager interface {
	Measure(children []Child, c Constraint) geometry.Size
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

// CrossAlign sizes/positions each child on the cross axis. Container-level
// default; the hug-vs-fill knob (relative axis, unlike GTK's absolute h/v).
type CrossAlign int

const (
	CrossStart   CrossAlign = iota // child hugs, sits at the cross start; default
	CrossCenter                    // child hugs, centered on the cross axis
	CrossEnd                       // child hugs, at the cross end
	CrossStretch                   // child fills the whole cross extent
)
