package widgets

import (
	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/layout"
)

// splitBounds never returns an inverted range. Under impossible minima the
// hard parent wins by compressing both minima proportionally. With insufficient
// maxima, fill wins and maxima are relaxed (minima remain honored).
func splitBounds(available float32, start, end splitLimit) (lo, hi float32) {
	available = max(0, available)
	if sum := start.min + end.min; sum > available {
		position := available * (start.min / sum)
		return position, position
	}
	if start.max+end.max < available {
		return start.min, available - end.min
	}
	return max(start.min, available-end.max), min(start.max, available-end.min)
}

func splitAllocate(available float32, preference splitPreference, naturalStart, naturalEnd float32, start, end splitLimit) float32 {
	target := available / 2
	switch preference.mode {
	case splitStart:
		target = preference.value
	case splitEnd:
		target = available - preference.value
	case splitRatio:
		target = available * preference.value
	case splitAuto:
		if total := naturalStart + naturalEnd; total > 0 {
			target = available * (naturalStart / total)
		}
	}
	lo, hi := splitBounds(available, start, end)
	return min(hi, max(lo, target))
}

type splitLayout struct{ view *SplitView }

func (l *splitLayout) natural(children []layout.Child, maximum geometry.Size) (start, end layout.Measurement) {
	// Wrappers always occupy the first two tree positions. Measuring through
	// layout.Child preserves GUI's cache and style-notification path.
	if l.view.start.present() {
		start = children[0].Measure(layout.Loose(maximum))
	}
	if l.view.end.present() {
		end = children[1].Measure(layout.Loose(maximum))
	}
	return
}
func (l *splitLayout) Measure(children []layout.Child, c layout.Constraint) layout.Measurement {
	v := l.view
	s, e := l.natural(children, c.Max)
	main := v.axis(s.Size) + v.axis(e.Size)
	if v.active() {
		main++
	}
	cross := max(v.cross(s.Size), v.cross(e.Size))
	result := c.Clamp(v.size(main, cross))
	// Re-measure at the allocated width/height for wrapping content. This is
	// observational: Measure must not establish or overwrite the saved ratio.
	if v.active() {
		a := max(0, v.axis(result)-1)
		position := splitAllocate(a, v.preference, v.axis(s.Size), v.axis(e.Size), v.start.limits(v), v.end.limits(v))
		s = children[0].Measure(layout.Constraint{Min: v.size(position, 0), Max: v.size(position, v.cross(c.Max))})
		e = children[1].Measure(layout.Constraint{Min: v.size(a-position, 0), Max: v.size(a-position, v.cross(c.Max))})
		result = c.Clamp(v.size(v.axis(result), max(v.cross(s.Size), v.cross(e.Size))))
	}
	return layout.Measured(result)
}
func (l *splitLayout) Arrange(children []layout.Child, rect geometry.Rectangle) {
	v := l.view
	main, cross := max(0, v.axis(rect.Size)), max(0, v.cross(rect.Size))
	var s, e layout.Measurement
	if v.preference.mode == splitAuto && v.active() {
		s, e = l.natural(children, rect.Size)
	}
	start, end, gap := float32(0), float32(0), float32(0)
	if v.active() {
		gap = min(1, main)
		a := main - gap
		if v.preference.mode == splitAuto && a > 0 {
			ratio := float32(.5)
			if total := v.axis(s.Size) + v.axis(e.Size); total > 0 {
				ratio = v.axis(s.Size) / total
			}
			v.preference = splitPreference{mode: splitRatio, value: ratio}
		}
		start = splitAllocate(a, v.preference, v.axis(s.Size), v.axis(e.Size), v.start.limits(v), v.end.limits(v))
		end = a - start
	} else if v.start.present() {
		start = main
	} else if v.end.present() {
		end = main
	}
	v.startSize, v.endSize = start, end
	for i, allocation := range []struct{ pos, extent float32 }{{0, start}, {start + gap, end}} {
		childSize := v.size(allocation.extent, cross)
		children[i].Measure(layout.Tight(childSize))
		childRect := geometry.Rectangle{Size: childSize}
		if v.direction == layout.DirectionVertical {
			childRect.Y = allocation.pos
		} else {
			childRect.X = allocation.pos
		}
		children[i].Arrange(childRect)
	}
	v.handle.SetVisible(v.active())
	if !v.active() {
		v.handle.cancel()
		return
	}
	// Wide hit area overlaps both panes; the actual separator occupies one DIP.
	position, extent := max(0, start-4), min(main, start+gap+4)-max(0, start-4)
	handleRect := geometry.Rectangle{Size: v.size(extent, cross)}
	if v.direction == layout.DirectionVertical {
		handleRect.Y = position
	} else {
		handleRect.X = position
	}
	v.handle.Arrange(handleRect)
}
func (v *SplitView) cross(size geometry.Size) float32 {
	if v.direction == layout.DirectionVertical {
		return size.Width
	}
	return size.Height
}
