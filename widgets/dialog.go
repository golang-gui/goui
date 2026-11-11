package widgets

import (
	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/core/signal"
	"github.com/golang-gui/goui/gui"
	"github.com/golang-gui/goui/layout"
	"github.com/golang-gui/goui/platform/events"
)

// BackdropCaptionMode selects the part of a Dialog backdrop that acts as a
// window caption.
type BackdropCaptionMode uint8

const (
	BackdropCaptionNone BackdropCaptionMode = iota
	BackdropCaptionAll
	// BackdropCaptionTop uses a top strip of the backdrop, excluding the card.
	// Its height is controlled by Dialog.BackdropCaptionHeight.
	BackdropCaptionTop
)

// Dialog displays a centered content card and a full-allocation backdrop.
// Place it in an Overlay with Fill enabled. It does not install modality or
// close itself: associate a gui.ModalScope and handle CloseRequest explicitly.
// All operations belong to the GUI thread.
type Dialog struct {
	gui.WidgetBase
	child           gui.Widget
	padding         geometry.Insets
	card            geometry.Rectangle
	backdropCaption BackdropCaptionMode
	captionHeight   float32
	closeOnOutside  bool
	revision        uint64
	closeRequest    signal.Signal1[func() bool]
	chrome          signal.Handle
}

// NewDialog creates a visible dialog with 24 DIP padding and a 40 DIP caption
// height. Outside presses do not close it and backdrop caption mode is off.
func NewDialog() *Dialog {
	d := &Dialog{padding: geometry.UniformInsets(24), captionHeight: 40}
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
		if p, ok := ctx.Position(); ok && d.closesAt(p) {
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

// BackdropCaption reports whether any backdrop caption mode is enabled.
func (d *Dialog) BackdropCaption() bool { return d.backdropCaption != BackdropCaptionNone }

// SetBackdropCaption selects the whole backdrop as caption when true and
// clears the caption region when false.
func (d *Dialog) SetBackdropCaption(caption bool) {
	if caption {
		d.SetBackdropCaptionMode(BackdropCaptionAll)
	} else {
		d.SetBackdropCaptionMode(BackdropCaptionNone)
	}
}

func (d *Dialog) BackdropCaptionMode() BackdropCaptionMode { return d.backdropCaption }

// SetBackdropCaptionMode selects the window drag region. The card, including
// its padding and gaps, remains a client region. CloseOnOutside applies to the
// remaining backdrop. Native movement follows the host's Chrome support.
// Unrecognized modes select BackdropCaptionNone.
func (d *Dialog) SetBackdropCaptionMode(mode BackdropCaptionMode) {
	if mode > BackdropCaptionTop {
		mode = BackdropCaptionNone
	}
	d.backdropCaption = mode
}

// BackdropCaptionHeight returns the top caption strip's height in local DIP.
func (d *Dialog) BackdropCaptionHeight() float32 { return d.captionHeight }

// SetBackdropCaptionHeight sets the height used by BackdropCaptionTop. The
// strip stays within the dialog bounds and excludes the card. Zero disables
// the strip; negative or non-finite heights become zero. This changes input
// regions only and does not affect layout or appearance.
func (d *Dialog) SetBackdropCaptionHeight(height float32) {
	if !d.Destroyed() {
		d.captionHeight = normalizeInset(height)
	}
}

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
	if d.captionAt(localPoint(d, point)) {
		*region = gui.ChromeRegionCaption
	} else {
		*region = gui.ChromeRegionClient
	}
}

func (d *Dialog) captionAt(p geometry.Point) bool {
	if !pointInRect(geometry.Rectangle{Size: d.Rect().Size}, p) || pointInRect(d.card, p) {
		return false
	}
	switch d.backdropCaption {
	case BackdropCaptionAll:
		return true
	case BackdropCaptionTop:
		return p.Y < d.captionHeight
	default:
		return false
	}
}

func (d *Dialog) closesAt(p geometry.Point) bool {
	return d.closeOnOutside && pointInRect(geometry.Rectangle{Size: d.Rect().Size}, p) &&
		!pointInRect(d.card, p) && !d.captionAt(p)
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
	start  geometry.Point
}

func (c *dialogBackdropClick) HandleEvent(ctx gui.EventContext) {
	if e, ok := ctx.Event().(events.PointerEvent); ok {
		switch e.EventType {
		case events.PointerDown:
			p, valid := ctx.Position()
			if e.Button != c.Button() || !valid || !c.dialog.closesAt(p) {
				return
			}
			c.start = e.Position
		case events.PointerMove, events.PointerUp:
			if c.Pressed() && (e.EventType == events.PointerMove || e.Button == c.Button()) {
				// Backdrop dismissal requires a click even when no competing
				// drag controller is present in this region.
				const clickDistance float32 = 4 // DIP
				dx, dy := e.Position.X-c.start.X, e.Position.Y-c.start.Y
				if dx*dx+dy*dy > clickDistance*clickDistance {
					c.Reset()
					return
				}
			}
		}
	}
	c.ClickEventController.HandleEvent(ctx)
}

var _ gui.Bin = (*Dialog)(nil)
