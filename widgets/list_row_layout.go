package widgets

import (
	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/layout"
)

type listRowLayout struct{ row *ListRow }

type listRowPlan struct {
	prefix, text, suffix layout.Child
	leading, middle, end layout.Measurement
	gap                  float32
}

func (l listRowLayout) plan(children []layout.Child, maxSize geometry.Size) listRowPlan {
	r := l.row
	var plan listRowPlan
	i := 0
	take := func(visible bool) layout.Child {
		if !visible || i >= len(children) {
			return nil
		}
		child := children[i]
		i++
		return child
	}
	plan.prefix = take(r.Prefix() != nil && r.Prefix().Visible())
	plan.text = take(r.text.Visible())
	plan.suffix = take(r.Suffix() != nil && r.Suffix().Visible())
	count := i
	plan.gap = r.spacing
	if count > 1 {
		plan.gap = min(plan.gap, maxSize.Width/float32(count-1))
	}
	bounded := maxSize.Width < layout.Inf
	remaining := maxSize.Width
	if bounded {
		remaining = max(0, remaining-plan.gap*float32(max(0, count-1)))
	}
	measure := func(child layout.Child, fill bool) layout.Measurement {
		if child == nil {
			return layout.Measurement{}
		}
		c := layout.Loose(geometry.Size{Width: remaining, Height: maxSize.Height})
		if fill && bounded {
			c.Min.Width = remaining
		}
		m := child.Measure(c).Constrain(c)
		if bounded {
			remaining = max(0, remaining-m.Width)
		}
		return m
	}
	// Keep the setting control visible first, then constrain the prefix and
	// give wrapping text exactly the remaining width. Slots never overlap.
	plan.end = measure(plan.suffix, false)
	plan.leading = measure(plan.prefix, false)
	plan.middle = measure(plan.text, true)
	return plan
}

func (l listRowLayout) Measure(children []layout.Child, c layout.Constraint) layout.Measurement {
	plan := l.plan(children, c.Insets(l.row.padding).Max)
	width := plan.leading.Width + plan.middle.Width + plan.end.Width
	if len(children) > 1 {
		width += plan.gap * float32(len(children)-1)
	}
	if c.Max.Width < layout.Inf {
		width = c.Insets(l.row.padding).Max.Width
	}
	size := c.Clamp(geometry.Size{
		Width:  width + l.row.padding.Horizontal(),
		Height: max(plan.leading.Height, plan.middle.Height, plan.end.Height) + l.row.padding.Vertical(),
	})
	return layout.Measured(size)
}

func (l listRowLayout) Arrange(children []layout.Child, rect geometry.Rectangle) {
	padding := l.row.padding
	area := rect.Insets(padding.Left, padding.Top, padding.Right, padding.Bottom)
	plan := l.plan(children, area.Size)
	x := area.X
	for _, item := range []struct {
		child layout.Child
		size  geometry.Size
	}{{plan.prefix, plan.leading.Size}, {plan.text, plan.middle.Size}, {plan.suffix, plan.end.Size}} {
		if item.child != nil {
			item.child.Arrange(geometry.Rect(x, area.Y+(area.Height-item.size.Height)/2, item.size.Width, item.size.Height))
			x += item.size.Width + plan.gap
		}
	}
}
