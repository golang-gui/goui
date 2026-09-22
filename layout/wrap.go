package layout

import "github.com/golang-gui/goui/core/geometry"

// WrapLayout packs children in order, starting another line when the main
// axis is full. A vertical layout wraps into columns. MainWeight is ignored.
// All spacing and padding values must be finite and non-negative.
type WrapLayout struct {
	Direction   Direction
	Spacing     float32
	LineSpacing float32
	Padding     float32
	MainAlign   MainAlign
	CrossAlign  CrossAlign
}

func NewWrapLayout(direction Direction) *WrapLayout {
	return &WrapLayout{Direction: direction}
}

func (l *WrapLayout) Measure(children []Child, c Constraint) Measurement {
	return l.compute(children, c).measured
}

func (l *WrapLayout) Arrange(children []Child, rect geometry.Rectangle) {
	plan := l.compute(children, Tight(rect.Size))
	axis := LinearLayout{Direction: l.Direction}
	inner := rect.Inset(l.Padding)
	for _, item := range plan.items {
		item.child.Arrange(axis.childRect(inner, item.mainPos, item.crossPos, item.mainLen, item.crossLen))
	}
}

type wrapLine struct {
	start, end  int
	main, cross float32
	baseline    float32
	hasBaseline bool
}

// compute shares only axis/alignment geometry with LinearLayout: there is no
// weighted allocation, implicit line widget, or retained measurement cache.
func (l *WrapLayout) compute(children []Child, c Constraint) linearPlan {
	axis := LinearLayout{Direction: l.Direction, Spacing: l.Spacing, MainAlign: l.MainAlign, CrossAlign: l.CrossAlign}
	inner := c.Inset(l.Padding)
	limit := axis.mainSize(inner.Max)
	items := make([]linearItem, 0, len(children))
	lines := make([]wrapLine, 0)
	line := wrapLine{}
	finish := func() {
		line.end = len(items)
		line.cross, line.baseline, line.hasBaseline = axis.crossMetrics(items[line.start:line.end])
		lines = append(lines, line)
		line = wrapLine{start: len(items)}
	}
	for _, child := range children {
		if child == nil {
			continue
		}
		// Use the whole content area, not the space left in the current line.
		m := child.Measure(Loose(inner.Max)).Constrain(Loose(inner.Max))
		main := axis.mainSize(m.Size)
		if len(items) > line.start && limit < Inf && line.main+l.Spacing+main > limit {
			finish()
		}
		if len(items) > line.start {
			line.main += l.Spacing
		}
		line.main += main
		items = append(items, linearItem{child: child, measured: m, mainLen: main, crossLen: axis.crossSize(m.Size)})
	}
	if len(items) > line.start {
		finish()
	}

	var naturalMain, naturalCross float32
	for i := range lines {
		run := &lines[i]
		naturalMain = max(naturalMain, run.main)
		naturalCross += run.cross
		if i > 0 {
			naturalCross += l.LineSpacing
		}
		if axis.effectiveCrossAlign() == CrossStretch {
			// Freeze membership and main sizes before stretching within a line.
			// Remeasure so text/layout state describes the actual allocation.
			for j := run.start; j < run.end; j++ {
				item := &items[j]
				allocation := axis.makeSize(item.mainLen, run.cross)
				item.measured = item.child.Measure(Tight(allocation)).Constrain(Tight(allocation))
				item.crossLen = run.cross
			}
		}
	}
	measured := Measured(c.Clamp(axis.makeSize(naturalMain, naturalCross).Inset(-l.Padding)))
	finalMain := axis.mainSize(measured.Size.Inset(l.Padding))
	var crossPos float32
	for _, run := range lines {
		mainPos, gap := axis.mainDistribution(max(0, finalMain-run.main), run.end-run.start)
		for j := run.start; j < run.end; j++ {
			item := &items[j]
			item.mainPos = mainPos
			var offset float32
			item.crossLen, offset = axis.itemCrossPlacement(*item, run.cross, run.baseline, run.hasBaseline)
			item.crossPos = crossPos + offset
			if !measured.HasBaseline && item.measured.HasBaseline {
				measured.HasBaseline = true
				measured.Baseline = l.Padding + item.measured.Baseline
				if l.Direction == DirectionVertical {
					measured.Baseline += item.mainPos
				} else {
					measured.Baseline += item.crossPos
				}
			}
			mainPos += item.mainLen + gap
		}
		crossPos += run.cross + l.LineSpacing
	}
	return linearPlan{measured: measured, items: items}
}
