package widgets

import (
	"time"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/gui"
	"github.com/golang-gui/goui/platform/events"
	"github.com/golang-gui/goui/platform/graphics"
	"github.com/golang-gui/goui/style"
)

type tabItem struct {
	gui.WidgetBase
	bar              *TabBar
	page             *TabPage
	body             *tabBody
	close            *gui.Button
	hovered, pressed bool
	width            float32
	reservedHeight   float32 // measured before hiding the native source
	motion           tabMotion
	widthMotion      tabMotion
}

type tabBody struct {
	gui.WidgetBase
	icon  *gui.Icon
	label *gui.Label
}

func newTabItem(bar *TabBar, page *TabPage) *tabItem {
	item := &tabItem{bar: bar, page: page}
	item.SetMinSize(geometry.Size{Height: 32})
	item.SetLayoutManager(tabItemLayout{})
	body := &tabBody{label: gui.NewLabel(page.title), icon: gui.NewIcon(page.icon)}
	body.SetFocusable(true)
	body.SetMainWeight(1)
	body.SetLayoutManager(tabBodyLayout{page: page})
	body.label.SetMainWeight(1)
	body.icon.SetSize(16)
	body.icon.SetVisible(page.icon != nil)
	body.WidgetBase.AddChild(body, body.icon)
	body.WidgetBase.AddChild(body, body.label)
	item.body = body
	item.WidgetBase.AddChild(item, body)

	closeButton := gui.NewButton()
	closeButton.SetPadding(0)
	closeButton.SetMinSize(geometry.Size{Width: 24, Height: 24})
	closeButton.SetMaxSize(geometry.Size{Width: 24, Height: 24})
	closeButton.SetStyleName("tab-close-button")
	closeGlyph := new(tabGlyph)
	closeGlyph.SetMinSize(geometry.Size{Width: 16, Height: 16})
	closeGlyph.SetStyleName("tab-close-button-text")
	closeButton.SetChild(closeGlyph)
	closeButton.ConnectClicked(func() {
		if bar.view != nil {
			bar.view.RequestClose(page)
		}
	})
	item.close = closeButton
	closeButton.ConnectContainsFocus(func(bool) { item.RequestLayout() })
	item.WidgetBase.AddChild(item, closeButton)
	item.AddEventController(&tabContextMenuController{
		EventControllerBase: gui.NewEventControllerBase(gui.PhaseCapture), item: item,
	})
	bar.addWheel(item)

	motion := gui.NewMotionEventController()
	motion.ConnectContainsHover(func(v bool) {
		item.hovered = v
		item.RequestLayout()
	})
	item.AddEventController(motion)
	click := gui.NewClickEventController()
	click.ConnectPressed(func(_ gui.EventContext, v bool) { item.pressed = v; item.RequestLayout() })
	click.ConnectClicked(func(_ gui.EventContext) {
		if bar.view != nil {
			bar.view.SetCurrent(page)
		}
	})
	body.AddEventController(click)
	drag := gui.NewDragEventController()
	drag.SetThreshold(4)
	drag.ConnectBegin(func(point geometry.Point, _ events.Modifiers) {
		bar.beginDrag(page, item.barPoint(point))
	})
	drag.ConnectUpdate(func(point geometry.Point, _ events.Modifiers) {
		point = item.barPoint(point)
		if !bar.handoff(page, drag, point) {
			bar.updateDrag(page, point)
		}
	})
	drag.ConnectEnd(func(point geometry.Point, _ events.Modifiers) {
		bar.endDrag(page, item.barPoint(point))
	})
	drag.ConnectCancel(func() {
		if bar.dragPage == page {
			bar.cancelDrag()
		}
	})
	body.AddEventController(drag)
	return item
}

func (item *tabItem) barPoint(point geometry.Point) geometry.Point {
	return point.Add(item.body.Rect().Pos).Add(item.Rect().Pos).Add(item.bar.viewport.Rect().Pos)
}
func (item *tabItem) updateMetadata() {
	item.body.label.SetText(item.page.title)
	item.body.icon.SetSource(item.page.icon)
	item.body.icon.SetVisible(item.page.icon != nil)
	item.body.RequestLayout() // closability changes its reserved trailing space
	item.updateSelection()
}
func (item *tabItem) updateSelection() {
	selected := item.bar.view != nil && item.bar.view.Current() == item.page
	textStyle := "tab-item-text"
	iconStyle := "tab-item-icon"
	if selected {
		textStyle += "-selected"
		iconStyle += "-selected"
	}
	item.body.label.SetStyleName(textStyle)
	item.body.icon.SetStyleName(iconStyle)
	item.RequestLayout()
}

func (item *tabItem) Arrange(rect geometry.Rectangle) {
	selected := item.bar.view != nil && item.bar.view.Current() == item.page
	// Apply disclosure with layout, not from hover crossing: the dispatcher
	// updates hover before picking a PointerDown target. Revealing immediately
	// could turn a press on an invisible close area into an accidental close.
	// The full-width body retains this hit area and always reserves the same
	// text space, independently of the overlaid button's visibility.
	// Pointer-down focuses the body even when the following drag does not
	// select this page. Body focus alone must not keep an inactive close shown;
	// retain it only when the close button itself owns keyboard focus.
	show := item.page.closable && (selected || item.hovered || item.close.ContainsFocus())
	// Do not introduce a sibling hit target underneath an ongoing body click.
	// Its release must still select the tab even if a frame runs in between.
	item.close.SetVisible(show && (!item.pressed || item.close.Visible()))
	item.WidgetBase.Arrange(rect)
}
func (item *tabItem) Paint(p gui.Painter) {
	state := style.Normal
	if item.pressed {
		state = style.Pressed
	} else if item.hovered {
		state = style.Hovered
	}
	part := ""
	if item.bar.view != nil && item.bar.view.Current() == item.page {
		part = "selected"
	}
	if item.bar.lifted == item.page {
		part = "dragging"
	}
	paintStyledBox(p, geometry.Rect(0, 0, item.Rect().Width, item.Rect().Height), "tab-item", part, state)
}

// Content-space positions keep scrolling independent of the slide animation.
// Retargeting begins at the current position, including when direction reverses.
type tabMotion struct {
	x, from, target float32
	elapsed         time.Duration
	positioned      bool
}

const tabSlideDuration = 140 * time.Millisecond

func (m *tabMotion) place(x float32, animate bool) {
	if !m.positioned || !animate {
		*m = tabMotion{x: x, from: x, target: x, elapsed: tabSlideDuration, positioned: true}
	} else if m.target != x {
		m.from, m.target, m.elapsed = m.x, x, 0
	}
}

func (m *tabMotion) advance(elapsed time.Duration) {
	if m.x == m.target {
		return
	}
	m.elapsed = min(tabSlideDuration, m.elapsed+elapsed)
	if m.elapsed == tabSlideDuration {
		m.x = m.target
		return
	}
	u := 1 - float32(m.elapsed)/float32(tabSlideDuration)
	m.x = m.from + (m.target-m.from)*(1-u*u*u)
}
func (item *tabItem) Snapshot() gui.WidgetInfo {
	info := item.WidgetBase.Snapshot()
	info.Role, info.Text = RoleTab, item.page.title
	info.Selected = item.bar.view != nil && item.bar.view.Current() == item.page
	if !item.Visible() {
		info.Children = nil // the reserved slot has no clickable content
	}
	return info
}

// Control symbols have fixed geometry, independent of the title's font size.
// Their style names remain the foreground-color hooks used by the theme.
type tabGlyph struct {
	gui.WidgetBase
	direction int // zero: close, negative: left, positive: right
}

func (g *tabGlyph) Paint(p gui.Painter) {
	s := gui.ResolveStyle(g.StyleName(), "", style.Normal)
	color, ok := s.ForegroundColor()
	if !ok || color == nil {
		return
	}
	c := geometry.Point{X: g.Rect().Width / 2, Y: g.Rect().Height / 2}
	brush := graphics.ColorOf(color)
	if g.direction == 0 {
		p.DrawLine(c.Add(geometry.Point{X: -4, Y: -4}), c.Add(geometry.Point{X: 4, Y: 4}), 1.5, brush)
		p.DrawLine(c.Add(geometry.Point{X: 4, Y: -4}), c.Add(geometry.Point{X: -4, Y: 4}), 1.5, brush)
	} else {
		dx := float32(g.direction) * 3
		p.DrawLine(c.Add(geometry.Point{X: -dx, Y: -5}), c.Add(geometry.Point{X: dx}), 1.5, brush)
		p.DrawLine(c.Add(geometry.Point{X: dx}), c.Add(geometry.Point{X: -dx, Y: 5}), 1.5, brush)
	}
}

func (g *tabGlyph) Snapshot() gui.WidgetInfo {
	info := g.WidgetBase.Snapshot()
	info.Role, info.Text = gui.RoleImage, "Close tab"
	if g.direction < 0 {
		info.Text = "Scroll tabs left"
	} else if g.direction > 0 {
		info.Text = "Scroll tabs right"
	}
	return info
}
