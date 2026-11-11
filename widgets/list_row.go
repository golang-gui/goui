package widgets

import (
	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/core/signal"
	"github.com/golang-gui/goui/gui"
	"github.com/golang-gui/goui/layout"
	"github.com/golang-gui/goui/platform/events"
	"github.com/golang-gui/goui/style"
)

// ListRow presents a title, optional subtitle and two optional content slots.
// It can stand alone, appear in a box, or be used by a ListView delegate. It
// has no selection, model or setting ownership. Slots retain their own styles
// and input handling; a child that wins a click prevents row activation.
type ListRow struct {
	gui.WidgetBase
	title, subtitle *gui.Label
	text            *gui.LinearBox
	prefix, suffix  gui.Widget
	padding         geometry.Insets
	spacing         float32
	activatable     bool
	revision        uint64
	activated       signal.Signal1[func() bool]
	motion          *gui.MotionEventController
	click           *gui.ClickEventController
	keys            *gui.KeyEventController
}

// NewListRow creates an inactive row with a 48 DIP minimum height, 12 DIP
// horizontal / 10 DIP vertical padding, and 12 DIP gaps around content slots.
// Title and subtitle follow Label's opt-in format syntax and wrap to the
// remaining content width.
func NewListRow(title string) *ListRow {
	r := &ListRow{padding: geometry.Insets{Left: 12, Right: 12, Top: 10, Bottom: 10}, spacing: 12}
	r.title = gui.NewLabel(title)
	r.title.SetStyleName("list-row-title")
	r.title.SetWrapMode(gui.WrapWordChar)
	r.title.SetVisible(title != "")
	r.subtitle = gui.NewLabel("")
	r.subtitle.SetStyleName("list-row-subtitle")
	r.subtitle.SetWrapMode(gui.WrapWordChar)
	r.subtitle.SetVisible(false)
	r.text = gui.NewLinearBox(layout.DirectionVertical)
	r.text.SetCrossAlign(layout.CrossStretch)
	r.text.SetSpacing(4)
	r.text.AddChild(r.title)
	r.text.AddChild(r.subtitle)
	r.text.SetVisible(title != "")
	r.WidgetBase.AddStructuralChild(r, r.text)
	r.SetMinSize(geometry.Size{Height: 48})
	r.SetLayoutManager(listRowLayout{row: r})
	r.ConnectFocused(func(bool) { r.RequestPaint() })
	r.motion = gui.NewMotionEventController()
	r.click = gui.NewClickEventController()
	r.click.ConnectPressed(func(_ gui.EventContext, pressed bool) { r.SetState(style.Pressed, pressed) })
	r.click.ConnectClicked(func(ctx gui.EventContext) {
		ctx.StopPropagation()
		r.activate()
	})
	r.keys = gui.NewKeyEventController()
	r.keys.ConnectKeyDown(func(ctx gui.EventContext, event events.KeyEvent) {
		if !r.Focused() || !r.activatable || !gui.IsEnabled(r) {
			return
		}
		if !(gui.KeyGesture{Key: gui.KeySpace}).Matches(event) && !(gui.KeyGesture{Key: gui.KeyEnter}).Matches(event) {
			return
		}
		ctx.StopPropagation()
		event.PreventDefault()
		if !event.Repeat {
			r.activate()
		}
	})
	return r
}

func (r *ListRow) Title() string    { return r.title.Text() }
func (r *ListRow) Subtitle() string { return r.subtitle.Text() }

func (r *ListRow) SetTitle(title string) {
	if r.Destroyed() {
		return
	}
	r.title.SetText(title)
	r.title.SetVisible(title != "")
	r.text.SetVisible(title != "" || r.Subtitle() != "")
}

func (r *ListRow) SetSubtitle(subtitle string) {
	if r.Destroyed() {
		return
	}
	r.subtitle.SetText(subtitle)
	r.subtitle.SetVisible(subtitle != "")
	r.text.SetVisible(subtitle != "" || r.Title() != "")
}

func (r *ListRow) Prefix() gui.Widget { return r.liveSlot(r.prefix) }
func (r *ListRow) Suffix() gui.Widget { return r.liveSlot(r.suffix) }

// SetPrefix replaces the leading slot without destroying the old content.
// A widget already in the other slot cannot occupy both slots.
func (r *ListRow) SetPrefix(child gui.Widget) { r.setSlot(true, child) }

// SetSuffix replaces the trailing slot without destroying the old content.
func (r *ListRow) SetSuffix(child gui.Widget) { r.setSlot(false, child) }

func (r *ListRow) liveSlot(child gui.Widget) gui.Widget {
	if child == nil || child.Parent() != r {
		return nil
	}
	return child
}

func (r *ListRow) setSlot(prefix bool, child gui.Widget) {
	if r.Destroyed() || child == r || child == r.text || child == r.title || child == r.subtitle {
		return
	}
	slot, other := &r.suffix, r.Prefix()
	if prefix {
		slot, other = &r.prefix, r.Suffix()
	}
	if *slot == child || child != nil && child == other {
		return
	}
	if dead, ok := child.(interface{ Destroyed() bool }); ok && dead.Destroyed() {
		return
	}
	r.revision++
	revision := r.revision
	root := r.Root()
	old := *slot
	*slot = nil
	r.WidgetBase.RemoveChild(old)
	if r.Destroyed() || revision != r.revision || r.Root() != root {
		return
	}
	*slot = child
	if child != nil {
		r.WidgetBase.AddChild(r, child)
		if r.Destroyed() || revision != r.revision || r.Root() != root {
			return
		}
		if child.Parent() != r {
			*slot = nil
		} else if prefix {
			r.WidgetBase.MoveChildBefore(child, r.text)
		}
	}
	r.RequestLayout()
}

func (r *ListRow) Padding() geometry.Insets { return r.padding }
func (r *ListRow) SetPadding(padding geometry.Insets) {
	padding = normalizeInsets(padding)
	if !r.Destroyed() && r.padding != padding {
		r.padding = padding
		r.RequestLayout()
	}
}

func (r *ListRow) Spacing() float32 { return r.spacing }
func (r *ListRow) SetSpacing(spacing float32) {
	spacing = normalizeInset(spacing)
	if !r.Destroyed() && r.spacing != spacing {
		r.spacing = spacing
		r.RequestLayout()
	}
}

func (r *ListRow) Activatable() bool { return r.activatable }

// SetActivatable enables row activation and makes the row focusable. Turning
// it off removes its controllers and focus capability; child controls remain
// interactive. Call SetFocusable afterwards to opt out of row Tab navigation.
func (r *ListRow) SetActivatable(activatable bool) {
	if r.Destroyed() || r.activatable == activatable {
		return
	}
	r.activatable = activatable
	r.SetFocusable(activatable)
	if r.Destroyed() || r.activatable != activatable {
		return
	}
	if activatable {
		r.AddEventController(r.motion)
		r.AddEventController(r.click)
		r.AddEventController(r.keys)
	} else {
		r.RemoveEventController(r.keys)
		r.RemoveEventController(r.click)
		r.RemoveEventController(r.motion)
		r.SetState(style.Pressed, false)
	}
	r.RequestPaint()
}

// ConnectActivated observes user activation. Setters never emit Activated.
// Applications bind row actions and suffix changes to the same business state
// when clicking the row should also change a setting.
func (r *ListRow) ConnectActivated(fn func()) signal.Handle {
	if r.Destroyed() {
		return signal.Handles(nil)
	}
	return r.activated.Connect(func(valid func() bool) {
		if valid() {
			fn()
		}
	})
}

func (r *ListRow) activate() {
	if !r.Destroyed() && r.activatable && gui.IsEnabled(r) {
		root := r.Root()
		r.activated.Emit(func() bool { return !r.Destroyed() && r.Root() == root && r.activatable && gui.IsEnabled(r) })
	}
}

func (r *ListRow) Paint(p gui.Painter) {
	name := r.StyleName()
	if name == "" {
		name = "list-row"
	}
	rect := geometry.Rect(0, 0, r.Rect().Width, r.Rect().Height)
	state := r.State()
	if !r.activatable {
		state &^= style.Hovered | style.Pressed
	}
	paintStyledBox(r, p, rect, name, style.PartDefault, state)
	if r.activatable && r.FocusVisible() {
		paintStyledBox(r, p, rect, name, "focus", r.State())
	}
}

func (r *ListRow) Snapshot() gui.WidgetInfo {
	info := r.WidgetBase.Snapshot()
	info.Role = gui.RoleListItem
	info.Text = r.title.PlainText()
	if subtitle := r.subtitle.PlainText(); subtitle != "" {
		if info.Text != "" {
			info.Text += "\n"
		}
		info.Text += subtitle
	}
	info.Children = nil // Internal labels are represented by Text, not extra actions.
	for _, child := range []gui.Widget{r.Prefix(), r.Suffix()} {
		if child != nil {
			info.Children = append(info.Children, child.Snapshot())
		}
	}
	if r.activatable && info.Enabled && !r.Destroyed() {
		info.Actions = []gui.Action{gui.ActionClick}
	}
	return info
}
