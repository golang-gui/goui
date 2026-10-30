package widgets

import (
	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/core/signal"
	"github.com/golang-gui/goui/gui"
	"github.com/golang-gui/goui/layout"
	"github.com/golang-gui/goui/platform/events"
)

// Dialog displays a centered content card and a full-allocation backdrop.
// Place it in an Overlay with Fill enabled. It does not install modality or
// close itself: associate a gui.ModalScope and handle CloseRequest explicitly.
// All operations belong to the GUI thread.
type Dialog struct {
	gui.WidgetBase
	child                           gui.Widget
	padding                         geometry.Insets
	card                            geometry.Rectangle
	backdropCaption, closeOnOutside bool
	revision                        uint64
	closeRequest                    signal.Signal1[func() bool]
	chrome                          signal.Handle
}

// NewDialog creates a visible dialog with 24 DIP padding. Outside presses do
// not close it and its backdrop is not a window drag region by default.
func NewDialog() *Dialog {
	d := &Dialog{padding: geometry.UniformInsets(24)}
	d.SetLayoutManager(dialogLayout{d})
	keys := gui.NewKeyEventController()
	keys.SetPhase(gui.PhaseBubble)
	keys.ConnectKeyDown(func(ctx gui.EventContext, e events.KeyEvent) {
		if !(gui.KeyGesture{Key: gui.KeyEscape}).Matches(e) || e.Repeat || e.Handled != nil && *e.Handled {
			return
		}
		ctx.StopPropagation()
		e.PreventDefault()
		d.requestClose()
	})
	d.AddEventController(keys)
	click := &dialogBackdropClick{ClickEventController: gui.NewClickEventController(), dialog: d}
	click.SetPhase(gui.PhaseBubble)
	click.ConnectClicked(func(ctx gui.EventContext) {
		if p, ok := ctx.Position(); ok && d.closeOnOutside && !d.backdropCaption && !pointInRect(d.card, p) {
			ctx.StopPropagation()
			d.requestClose()
		}
	})
	d.AddEventController(click)
	d.ConnectMount(func() {
		if win := d.Window(); win != nil {
			d.chrome = win.Chrome().ConnectQueryRegion(d.queryRegion)
		}
	})
	d.ConnectUnmount(func() {
		d.revision++
		if d.chrome != nil {
			d.chrome.Disconnect()
			d.chrome = nil
		}
	})
	return d
}

func (d *Dialog) Child() gui.Widget { return d.child }

// SetChild replaces content without destroying the detached child.
func (d *Dialog) SetChild(child gui.Widget) {
	if d.Destroyed() || child == d || child == d.child {
		return
	}
	if old := d.child; old != nil {
		d.child = nil
		d.RemoveChild(old)
	}
	if d.Destroyed() {
		return
	}
	d.child = child
	if child != nil {
		d.WidgetBase.AddChild(d, child)
		if child.Parent() != d {
			d.child = nil
		}
	}
	d.RequestLayout()
}

func (d *Dialog) Padding() geometry.Insets { return d.padding }

// SetPadding changes per-edge card padding in DIP; invalid edges become zero.
func (d *Dialog) SetPadding(padding geometry.Insets) {
	padding = normalizeInsets(padding)
	if !d.Destroyed() && d.padding != padding {
		d.padding = padding
		d.RequestLayout()
	}
}

func (d *Dialog) BackdropCaption() bool { return d.backdropCaption }

// SetBackdropCaption allows window movement through empty backdrop space.
// The card, including its padding and gaps, remains a client region. Caption
// handling takes precedence over CloseOnOutside. Native support follows Chrome.
func (d *Dialog) SetBackdropCaption(caption bool) { d.backdropCaption = caption }

func (d *Dialog) CloseOnOutside() bool         { return d.closeOnOutside }
func (d *Dialog) SetCloseOnOutside(close bool) { d.closeOnOutside = close }

func (d *Dialog) SetVisible(visible bool) {
	if d.Visible() != visible {
		d.revision++
	}
	d.WidgetBase.SetVisible(visible)
}

// ConnectCloseRequest reports an unconsumed Escape or completed outside click.
// The caller decides whether to hide the dialog or change application state.
func (d *Dialog) ConnectCloseRequest(fn func()) signal.Handle {
	return d.closeRequest.Connect(func(valid func() bool) {
		if valid() {
			fn()
		}
	})
}

func (d *Dialog) requestClose() {
	if d.Destroyed() || !d.Visible() || !gui.IsEnabled(d) {
		return
	}
	revision := d.revision
	d.closeRequest.Emit(func() bool { return !d.Destroyed() && d.Visible() && d.revision == revision })
}

func (d *Dialog) queryRegion(point geometry.Point, region *gui.ChromeRegion) {
	if d.Destroyed() {
		return
	}
	switch *region {
	case gui.ChromeRegionDefault, gui.ChromeRegionClient, gui.ChromeRegionCaption:
	default:
		return // Window controls and the resize perimeter retain their operations.
	}
	for w := gui.Widget(d); w != nil; w = w.Parent() {
		if !w.Visible() {
			return
		}
	}
	win := d.Window()
	if win == nil {
		return
	}
	// Only report a region where this dialog actually owns the normal pick.
	target := gui.Pick(win.Widget(), localPoint(win.Widget(), point))
	for w := target; w != d; w = w.Parent() {
		if w == nil {
			return
		}
	}
	p := localPoint(d, point)
	if pointInRect(d.card, p) || !d.backdropCaption {
		*region = gui.ChromeRegionClient
	} else {
		*region = gui.ChromeRegionCaption
	}
}

func (d *Dialog) Paint(p gui.Painter) {
	gui.PaintStyledBox(p, geometry.Rectangle{Size: d.Rect().Size}, gui.WidgetStyle(d, "dialog", "backdrop"))
	paintSurface(p, d.card, gui.WidgetStyle(d, "dialog", "card"))
}

func (d *Dialog) Snapshot() gui.WidgetInfo {
	info := d.WidgetBase.Snapshot()
	info.Role = RoleDialog
	return info
}

type dialogLayout struct{ dialog *Dialog }

func (l dialogLayout) Measure(children []layout.Child, c layout.Constraint) layout.Measurement {
	d := l.dialog
	insets := surfaceInsets(gui.WidgetStyle(d, "dialog", "card"))
	padding := geometry.Insets{Left: insets.Left + d.padding.Left, Right: insets.Right + d.padding.Right,
		Top: insets.Top + d.padding.Top, Bottom: insets.Bottom + d.padding.Bottom}
	var size geometry.Size
	if len(children) != 0 {
		size = children[0].Measure(layout.Loose(c.Insets(padding).Max)).Size
	}
	return layout.Measured(c.Clamp(size.Add(geometry.Size{Width: padding.Horizontal(), Height: padding.Vertical()})))
}

func (l dialogLayout) Arrange(children []layout.Child, rect geometry.Rectangle) {
	d := l.dialog
	insets := surfaceInsets(gui.WidgetStyle(d, "dialog", "card"))
	available := rect.Insets(insets.Left, insets.Top, insets.Right, insets.Bottom)
	var size geometry.Size
	if len(children) != 0 {
		size = children[0].Measure(layout.Loose(available.Size.Insets(d.padding.Left, d.padding.Top, d.padding.Right, d.padding.Bottom))).Size
	}
	card := geometry.Size{Width: min(available.Width, size.Width+d.padding.Horizontal()), Height: min(available.Height, size.Height+d.padding.Vertical())}
	d.card = geometry.Rect((rect.Width-card.Width)/2, (rect.Height-card.Height)/2, card.Width, card.Height)
	if len(children) != 0 {
		children[0].Arrange(d.card.Insets(d.padding.Left, d.padding.Top, d.padding.Right, d.padding.Bottom))
	}
}

type dialogBackdropClick struct {
	*gui.ClickEventController
	dialog *Dialog
}

func (c *dialogBackdropClick) HandleEvent(ctx gui.EventContext) {
	if e, ok := ctx.Event().(events.PointerEvent); ok && e.EventType == events.PointerDown {
		p, valid := ctx.Position()
		if !valid || !c.dialog.closeOnOutside || c.dialog.backdropCaption || pointInRect(c.dialog.card, p) {
			return
		}
	}
	c.ClickEventController.HandleEvent(ctx)
}

var _ gui.Bin = (*Dialog)(nil)
