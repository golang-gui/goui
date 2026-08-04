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

func (l *LinearLayout) Measure(children []Child, c Constraint) geometry.Size {
	innerMax := c.Max.Inset(l.Padding)
	var size geometry.Size
	count := 0
	for _, child := range children {
		if child == nil {
			continue
		}
		childSize := child.Measure(l.childConstraint(
			innerMax,
			child.MainWeight() > 0,
			l.crossSize(c.Max) < Inf,
		))
		if l.CrossAlign == CrossStretch && l.crossSize(c.Max) < Inf {
			// Keep the measured contribution consistent with the container's
			// finite cross-axis stretch policy, even for simple custom children
			// that do not use the tight constraint themselves.
			l.setCrossSize(&childSize, l.crossSize(innerMax))
		}
		if count > 0 {
			l.addSpacing(&size)
		}
		l.addChildSize(&size, childSize)
		count++
	}
	// Add padding back onto the content size (empty box → 2*padding).
	return c.Clamp(size.Inset(-l.Padding))
}

func (l *LinearLayout) Arrange(children []Child, rect geometry.Rectangle) {
	rect = rect.Inset(l.Padding) // content area inside the padding

	items := make([]Child, 0, len(children))
	for _, child := range children {
		if child != nil {
			items = append(items, child)
		}
	}
	if len(items) == 0 {
		return
	}

	availMain := l.mainSize(rect.Size)
	availCross := l.crossSize(rect.Size)

	// Pass 1: intrinsic sizes and total weight. Weight never affects the intrinsic
	// measurement (a weighted child still hugs its content as a minimum); it only
	// decides how leftover space is shared out below.
	intrinsic := make([]geometry.Size, len(items))
	var usedMain, totalWeight float32
	for i, child := range items {
		intrinsic[i] = child.Measure(l.childConstraint(
			l.makeSize(availMain, availCross),
			child.MainWeight() > 0,
			availCross < Inf,
		))
		if l.CrossAlign == CrossStretch && availCross < Inf {
			l.setCrossSize(&intrinsic[i], availCross)
		}
		usedMain += l.mainSize(intrinsic[i])
		if w := child.MainWeight(); w > 0 {
			totalWeight += w
		}
	}
	usedMain += l.Spacing * float32(len(items)-1)
	freeMain := max(0, availMain-usedMain)

	// Weight consumes the free space first; only when nothing is weighted does
	// MainAlign get to place the leftover block.
	startMain, gap := float32(0), l.Spacing
	if totalWeight == 0 {
		startMain, gap = l.mainDistribution(freeMain, len(items))
	}

	offset := startMain
	for i, child := range items {
		mainLen := l.mainSize(intrinsic[i])
		if totalWeight > 0 {
			if w := child.MainWeight(); w > 0 {
				mainLen += freeMain * w / totalWeight
			}
		}
		crossLen, crossPos := l.crossPlacement(l.crossSize(intrinsic[i]), availCross)
		child.Arrange(l.childRect(rect, offset, crossPos, mainLen, crossLen))
		offset += mainLen + gap
	}
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

// crossPlacement returns a child's cross-axis length and offset according to
// the container's CrossAlign policy.
func (l *LinearLayout) crossPlacement(childCross, availCross float32) (crossLen, crossPos float32) {
	switch l.CrossAlign {
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
func (l *LinearLayout) childConstraint(max geometry.Size, weighted, crossBounded bool) Constraint {
	c := Loose(max)
	if weighted {
		l.setMainSize(&c.Max, Inf)
	}
	if l.CrossAlign == CrossStretch && crossBounded {
		// Stretch is a tight constraint on the container's cross axis. The
		// child can measure content such as wrapped text at its final width.
		l.setCrossSize(&c.Min, l.crossSize(max))
	}
	return c
}

func (l *LinearLayout) addSpacing(size *geometry.Size) {
	if l.Direction == DirectionVertical {
		size.Height += l.Spacing
		return
	}
	size.Width += l.Spacing
}

func (l *LinearLayout) addChildSize(size *geometry.Size, childSize geometry.Size) {
	if l.Direction == DirectionVertical {
		size.Width = max(size.Width, childSize.Width)
		size.Height += childSize.Height
		return
	}
	size.Width += childSize.Width
	size.Height = max(size.Height, childSize.Height)
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
