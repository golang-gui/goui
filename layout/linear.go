package layout

import "github.com/golang-gui/goui/core/geometry"

type LinearLayout struct {
	Direction  Direction
	Spacing    float32
	Padding    float32 // inner box padding, inset before laying out children
	MainAlign  MainAlign
	CrossAlign CrossAlign
}

func NewLinearLayout(direction Direction) *LinearLayout {
	return &LinearLayout{
		Direction: direction,
	}
}

func (l *LinearLayout) Measure(children []Child, c Constraint) Measurement {
	return l.compute(children, c).measured
}

func (l *LinearLayout) Arrange(children []Child, rect geometry.Rectangle) {
	plan := l.compute(children, Tight(rect.Size))
	inner := rect.Inset(l.Padding)
	for _, item := range plan.items {
		item.child.Arrange(l.childRect(
			inner,
			item.mainPos,
			item.crossPos,
			item.mainLen,
			item.crossLen,
		))
	}
}

type linearItem struct {
	child    Child
	measured Measurement
	weight   float32
	mainLen  float32
	crossLen float32
	mainPos  float32
	crossPos float32
}

type linearPlan struct {
	measured Measurement
	items    []linearItem
}

// compute performs the complete one-dimensional layout calculation. Measure
// uses its container geometry; Arrange calls it with a tight final rectangle so
// weighted children are measured again at the width/height they will receive.
// This is essential for height-for-width content such as wrapping text.
func (l *LinearLayout) compute(children []Child, c Constraint) linearPlan {
	items := make([]linearItem, 0, len(children))
	for _, child := range children {
		if child != nil {
			items = append(items, linearItem{child: child, weight: max(0, child.MainWeight())})
		}
	}

	if len(items) == 0 {
		return linearPlan{measured: Measured(c.Clamp(geometry.Size{}.Inset(-l.Padding)))}
	}

	innerConstraint := c.Inset(l.Padding)
	innerMax := innerConstraint.Max
	stretchCross, stretchCrossExtent := l.stretchCrossExtent(innerConstraint)
	var naturalMain, totalWeight float32
	for i := range items {
		item := &items[i]
		item.measured = item.child.Measure(l.childConstraint(innerMax, item.weight > 0, stretchCross, stretchCrossExtent))
		if stretchCross {
			// A tight layout axis wins even when a custom child forgets to clamp
			// its own result to the supplied constraint.
			l.setCrossSize(&item.measured.Size, stretchCrossExtent)
		}
		item.mainLen = l.mainSize(item.measured.Size)
		item.crossLen = l.crossSize(item.measured.Size)
		naturalMain += item.mainLen
		totalWeight += item.weight
	}
	naturalMain += l.Spacing * float32(len(items)-1)
	naturalCross, _, _ := l.crossMetrics(items)

	naturalOuter := l.makeSize(naturalMain, naturalCross).Inset(-l.Padding)
	finalSize := c.Clamp(naturalOuter)
	innerSize := finalSize.Inset(l.Padding)
	finalMain := l.mainSize(innerSize)

	// Positive free space grows weighted children from their natural basis. If
	// there is a deficit, fixed children retain their natural size and overflow;
	// weighted children share what remains and are remeasured under that final
	// tight main-axis constraint.
	freeMain := finalMain - naturalMain
	if totalWeight > 0 {
		if freeMain >= 0 {
			for i := range items {
				if items[i].weight > 0 {
					items[i].mainLen += freeMain * items[i].weight / totalWeight
				}
			}
		} else {
			fixedMain := l.Spacing * float32(len(items)-1)
			for i := range items {
				if items[i].weight == 0 {
					fixedMain += items[i].mainLen
				}
			}
			flexMain := max(0, finalMain-fixedMain)
			for i := range items {
				if items[i].weight > 0 {
					items[i].mainLen = flexMain * items[i].weight / totalWeight
				}
			}
		}

		for i := range items {
			item := &items[i]
			if item.weight == 0 {
				continue
			}
			childC := l.childConstraint(innerMax, true, stretchCross, stretchCrossExtent)
			l.setMainSize(&childC.Min, item.mainLen)
			l.setMainSize(&childC.Max, item.mainLen)
			item.measured = item.child.Measure(childC)
			// Parent allocation is authoritative even for a custom Child that
			// returns a size outside its tight constraint.
			l.setMainSize(&item.measured.Size, item.mainLen)
			if stretchCross {
				l.setCrossSize(&item.measured.Size, stretchCrossExtent)
			}
			item.crossLen = l.crossSize(item.measured.Size)
		}
	}

	// Remeasuring flexible content can change the cross extent (for example a
	// Label gets taller after wrapping at its final width).
	finalCrossNatural, commonBaseline, hasCommonBaseline := l.crossMetrics(items)
	desired := l.makeSize(finalMain, finalCrossNatural).Inset(-l.Padding)
	finalSize = c.Clamp(desired)
	innerSize = finalSize.Inset(l.Padding)
	finalCross := l.crossSize(innerSize)

	usedMain := l.Spacing * float32(len(items)-1)
	for i := range items {
		usedMain += items[i].mainLen
	}
	remainingMain := max(0, finalMain-usedMain)
	startMain, gap := float32(0), l.Spacing
	if totalWeight == 0 {
		startMain, gap = l.mainDistribution(remainingMain, len(items))
	}

	mainPos := startMain
	for i := range items {
		item := &items[i]
		item.mainPos = mainPos
		item.crossLen, item.crossPos = l.itemCrossPlacement(*item, finalCross, commonBaseline, hasCommonBaseline)
		mainPos += item.mainLen + gap
	}

	measured := Measured(finalSize)
	if l.Direction == DirectionHorizontal && l.effectiveCrossAlign() == CrossBaseline && hasCommonBaseline {
		measured.Baseline = l.Padding + commonBaseline
		measured.HasBaseline = true
	} else {
		// A compound widget exposes its first baseline-bearing descendant so it
		// can itself participate in a baseline-aligned ancestor.
		for i := range items {
			item := &items[i]
			if !item.measured.HasBaseline {
				continue
			}
			if l.Direction == DirectionHorizontal {
				measured.Baseline = l.Padding + item.crossPos + item.measured.Baseline
			} else {
				measured.Baseline = l.Padding + item.mainPos + item.measured.Baseline
			}
			measured.HasBaseline = true
			break
		}
	}

	return linearPlan{measured: measured, items: items}
}

// crossMetrics returns the natural cross extent. In a baseline-aligned row it
// combines the largest ascent and descent so differently sized text never
// overlaps or clips merely to share a baseline.
func (l *LinearLayout) crossMetrics(items []linearItem) (size, baseline float32, hasBaseline bool) {
	if l.Direction != DirectionHorizontal || l.effectiveCrossAlign() != CrossBaseline {
		for i := range items {
			size = max(size, l.crossSize(items[i].measured.Size))
		}
		return size, 0, false
	}

	var descent, withoutBaseline float32
	for i := range items {
		item := &items[i]
		cross := l.crossSize(item.measured.Size)
		if !item.measured.HasBaseline {
			withoutBaseline = max(withoutBaseline, cross)
			continue
		}
		hasBaseline = true
		ascent := max(0, item.measured.Baseline)
		baseline = max(baseline, ascent)
		descent = max(descent, max(0, cross-ascent))
	}
	return max(withoutBaseline, baseline+descent), baseline, hasBaseline
}

// mainDistribution returns the leading main-axis offset and the gap between
// children for the container's MainAlign (used only when no child has weight).
func (l *LinearLayout) mainDistribution(freeMain float32, n int) (start, gap float32) {
	switch l.MainAlign {
	case MainCenter:
		return freeMain / 2, l.Spacing
	case MainEnd:
		return freeMain, l.Spacing
	case MainSpaceBetween:
		if n > 1 {
			return 0, l.Spacing + freeMain/float32(n-1)
		}
		return 0, l.Spacing
	default: // MainStart
		return 0, l.Spacing
	}
}

func (l *LinearLayout) effectiveCrossAlign() CrossAlign {
	if l.Direction == DirectionVertical && l.CrossAlign == CrossBaseline {
		return CrossStart
	}
	return l.CrossAlign
}

// itemCrossPlacement returns a child's cross-axis length and offset according
// to the container policy and, for a horizontal baseline row, the shared
// typographic baseline.
func (l *LinearLayout) itemCrossPlacement(item linearItem, availCross, baseline float32, hasBaseline bool) (crossLen, crossPos float32) {
	if l.Direction == DirectionHorizontal && l.effectiveCrossAlign() == CrossBaseline {
		if hasBaseline && item.measured.HasBaseline {
			return item.crossLen, baseline - item.measured.Baseline
		}
		return item.crossLen, 0
	}
	return l.crossPlacement(item.crossLen, availCross)
}

// crossPlacement returns a child's cross-axis length and offset according to
// the effective non-baseline CrossAlign policy.
func (l *LinearLayout) crossPlacement(childCross, availCross float32) (crossLen, crossPos float32) {
	switch l.effectiveCrossAlign() {
	case CrossStretch:
		return availCross, 0
	case CrossCenter:
		return childCross, (availCross - childCross) / 2
	case CrossEnd:
		return childCross, availCross - childCross
	default: // CrossStart
		return childCross, 0
	}
}

func (l *LinearLayout) setCrossSize(size *geometry.Size, cross float32) {
	if l.Direction == DirectionVertical {
		size.Width = cross
		return
	}
	size.Height = cross
}

func (l *LinearLayout) setMainSize(size *geometry.Size, main float32) {
	if l.Direction == DirectionVertical {
		size.Height = main
		return
	}
	size.Width = main
}

// childConstraint gives weighted children an unbounded main-axis basis. A
// viewport such as ScrollView can then report no intrinsic scroll height while
// still reporting its intrinsic width or finite cross-axis viewport.
func (l *LinearLayout) childConstraint(max geometry.Size, weighted, stretchCross bool, crossExtent float32) Constraint {
	c := Loose(max)
	if weighted {
		l.setMainSize(&c.Max, Inf)
	}
	if stretchCross {
		// Stretch is a tight constraint on the container's cross axis. The
		// child can measure content such as wrapped text at its final width.
		l.setCrossSize(&c.Min, crossExtent)
		l.setCrossSize(&c.Max, crossExtent)
	}
	return c
}

// stretchCrossExtent resolves the concrete inner cross-axis size imposed by
// CrossStretch. A finite maximum consumes all available cross space. With an
// unbounded maximum, a finite minimum still supplies a final size that must be
// used during Measure for height-for-width consistency.
func (l *LinearLayout) stretchCrossExtent(c Constraint) (bool, float32) {
	if l.effectiveCrossAlign() != CrossStretch {
		return false, 0
	}
	if extent := l.crossSize(c.Max); extent < Inf {
		return true, extent
	}
	if extent := l.crossSize(c.Min); extent > 0 && extent < Inf {
		return true, extent
	}
	return false, 0
}

// mainSize/crossSize/makeSize map absolute Width/Height onto the layout's
// relative main/cross axes (main = the flow direction). This is what lets
// CrossAlign mean the same thing in an HBox and a VBox.
func (l *LinearLayout) mainSize(s geometry.Size) float32 {
	if l.Direction == DirectionVertical {
		return s.Height
	}
	return s.Width
}

func (l *LinearLayout) crossSize(s geometry.Size) float32 {
	if l.Direction == DirectionVertical {
		return s.Width
	}
	return s.Height
}

func (l *LinearLayout) makeSize(main, cross float32) geometry.Size {
	if l.Direction == DirectionVertical {
		return geometry.Size{Width: cross, Height: main}
	}
	return geometry.Size{Width: main, Height: cross}
}

func (l *LinearLayout) childRect(rect geometry.Rectangle, mainOffset, crossOffset, mainLen, crossLen float32) geometry.Rectangle {
	if l.Direction == DirectionVertical {
		return geometry.Rect(rect.X+crossOffset, rect.Y+mainOffset, crossLen, mainLen)
	}
	return geometry.Rect(rect.X+mainOffset, rect.Y+crossOffset, mainLen, crossLen)
}
