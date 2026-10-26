package gui

import (
	"fmt"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/core/signal"
	"github.com/golang-gui/goui/layout"
	"github.com/golang-gui/goui/platform/events"
	"github.com/golang-gui/goui/style"
)

// MenuButtonGroup identifies buttons whose open menus follow pointer and
// horizontal keyboard navigation. It owns no widgets, layout or native state.
// Use on the GUI thread; membership does not imply a common layout parent.
type MenuButtonGroup struct{ identity byte }

// NewMenuButtonGroup creates an independent menu-switching identity.
func NewMenuButtonGroup() *MenuButtonGroup { return new(MenuButtonGroup) }

// MenuButtonInfoKey identifies MenuButtonInfo in semantic snapshots.
const MenuButtonInfoKey = "goui.menuButton"

// MenuButtonInfo describes menu availability and presentation without exposing
// the borrowed presenter, model or group object in semantic snapshots.
type MenuButtonInfo struct {
	HasMenu bool `json:"hasMenu"`
	Opened  bool `json:"opened"`
	Grouped bool `json:"grouped"`
}

// MenuButton opens a modeless, automatically dismissed menu. It keeps logical
// focus in the owner and forwards menu keys through the popover's event path.
// A group adds switching without changing the button's layout or ownership.
type MenuButton struct {
	WidgetBase
	content            Widget
	padding            float32
	hovered, pressed   bool
	menu               MenuModel
	pm                 *PopoverMenu
	click              *ClickEventController
	motion             *MotionEventController
	group, linkedGroup *MenuButtonGroup
	revision           uint64
	closed             signal.Signal1[uint64]
	openError          signal.Signal2[error, uint64]
}

func NewMenuButton() *MenuButton {
	b := &MenuButton{padding: defaultButtonPadding}
	b.SetFocusable(true)
	b.ConnectFocused(func(focused bool) {
		if !focused {
			b.Close()
		}
		b.RequestPaint()
	})
	b.SetLayoutManager(buttonContentLayout{})
	b.motion = NewMotionEventController()
	b.motion.SetPhase(PhaseBubble)
	b.motion.ConnectContainsHover(b.setHovered)
	b.motion.ConnectMotion(func(MotionInfo) {
		if b.Opened() || b.group == nil || b.Destroyed() || !IsEnabled(b) {
			return
		}
		if active := b.activeGroupMenu(); active != nil && active != b {
			active.switchMenu(b, true)
		}
	})
	b.AddEventController(b.motion)
	b.click = NewClickEventController()
	b.click.ConnectPressed(func(ctx EventContext, pressed bool) {
		b.setPressed(pressed)
		ctx.StopPropagation()
	})
	b.click.ConnectClicked(func(ctx EventContext) {
		b.openMenu()
		ctx.StopPropagation()
	})
	b.AddEventController(b.click)
	keys := NewKeyEventController()
	keys.SetPhase(PhaseCapture)
	keys.ConnectKeyDown(b.menuKey)
	b.AddEventController(keys)
	addButtonKeys(b, b.openMenu)
	return b
}

func (b *MenuButton) SetChild(child Widget) {
	if b.content == child {
		return
	}
	if b.content != nil {
		b.WidgetBase.RemoveChild(b.content)
	}
	b.content = child
	if child != nil {
		b.WidgetBase.AddChild(b, child)
	}
	b.RequestLayout()
}
func (b *MenuButton) Child() Widget { return b.content }

// SetMenu installs the model, even when it is the current instance. Declarative
// callers should skip installation when only action callbacks have changed.
func (b *MenuButton) SetMenu(m MenuModel) {
	if b.Destroyed() {
		return
	}
	b.revision++
	b.menu = m
	if b.pm != nil {
		b.pm.SetMenu(m)
		if !b.hasMenu() {
			b.Close()
		}
	}
	b.RequestPaint()
}
func (b *MenuButton) Menu() MenuModel         { return b.menu }
func (b *MenuButton) Group() *MenuButtonGroup { return b.group }

// SetGroup changes explicit membership. An already open menu remains usable
// independently; its current group session ends until the next open.
func (b *MenuButton) SetGroup(group *MenuButtonGroup) {
	if b.Destroyed() || b.group == group {
		return
	}
	b.revision++
	b.group, b.linkedGroup = group, nil
}
func (b *MenuButton) Opened() bool { return b.pm != nil && b.pm.Visible() }

// Open shows an available menu and returns opening failures directly. Empty,
// hidden or disabled buttons are a no-op. It does not emit OpenError.
func (b *MenuButton) Open() error {
	if b.Destroyed() {
		return fmt.Errorf("menu-button: destroyed")
	}
	if !IsEnabled(b) || !visibleInTree(b) || !b.hasMenu() || b.Opened() {
		return nil
	}
	if previous := b.activeGroupMenu(); previous != nil && previous != b {
		return previous.switchMenu(b, false)
	}
	return b.showMenu()
}

func (b *MenuButton) showMenu() error {
	b.revision++
	revision := b.revision
	if b.pm == nil {
		b.pm = NewPopoverMenu(b)
		b.pm.SetModal(false)
		b.pm.ConnectClosed(func() {
			b.revision++
			if !b.Opened() {
				b.linkedGroup = nil
			}
			b.RequestPaint()
			b.closed.Emit(b.revision)
		})
	}
	b.pm.SetMenu(b.menu)
	if host, ok := b.Root().(EventTarget); ok {
		if !host.SetFocusedWidget(b) {
			return fmt.Errorf("menu-button: anchor cannot receive focus")
		}
	}
	if b.Destroyed() || b.revision != revision {
		return fmt.Errorf("menu-button: opening superseded")
	}
	if err := b.pm.show(PopoverPlacementBottom, geometry.Point{}); err != nil {
		return err
	}
	if b.Destroyed() || b.revision != revision || !b.Opened() {
		return fmt.Errorf("menu-button: released while opening")
	}
	b.linkedGroup = b.group
	b.RequestPaint()
	return nil
}

func (b *MenuButton) Close() {
	if b.pm != nil {
		b.pm.Hide()
	}
	b.setPressed(false)
}
func (b *MenuButton) ConnectClosed(fn func()) signal.Handle {
	return b.closed.Connect(func(revision uint64) {
		if !b.Destroyed() && b.revision == revision {
			fn()
		}
	})
}

// ConnectOpenError reports failures from user input, once per failed attempt.
// Open returns its error without notifying this signal.
func (b *MenuButton) ConnectOpenError(fn func(error)) signal.Handle {
	return b.openError.Connect(func(err error, revision uint64) {
		if !b.Destroyed() && b.revision == revision {
			fn(err)
		}
	})
}
func (b *MenuButton) openFromInput() {
	revision := b.revision
	if err := b.Open(); err != nil && !b.Destroyed() && b.revision == revision+1 {
		b.openError.Emit(err, b.revision)
	}
}
func (b *MenuButton) openMenu() {
	if b.Opened() {
		b.Close()
	} else {
		b.openFromInput()
	}
}

func (b *MenuButton) hasMenu() bool {
	if b.menu != nil {
		for i := 0; i < b.menu.ItemsCount(); i++ {
			if item := b.menu.ItemAt(i); item != nil && item.Visible() && !item.Separator() {
				return true
			}
		}
	}
	return false
}
func (b *MenuButton) activeGroupMenu() *MenuButton {
	if b.group == nil || b.Root() == nil {
		return nil
	}
	owner, ok := anchorWindow(b)
	if !ok {
		return nil
	}
	for _, p := range owner.Popovers() {
		candidate, ok := p.Anchor().(*MenuButton)
		if ok && candidate != b && candidate.Root() == b.Root() && candidate.group == b.group && candidate.linkedGroup == b.group && candidate.Opened() {
			return candidate
		}
	}
	return nil
}

// switchMenu closes the old surface before opening the candidate. A callback
// that changes either endpoint or shows another surface cancels this switch.
func (b *MenuButton) switchMenu(next *MenuButton, fromInput bool) error {
	group, root := b.linkedGroup, b.Root()
	if group == nil || b.group != group || !b.Opened() || next == nil || next == b || next.group != group || next.Root() != root || !IsEnabled(next) || !visibleInTree(next) || !next.hasMenu() {
		return nil
	}
	owner, ok := anchorWindow(b)
	if !ok {
		return nil
	}
	w, _ := owner.(*window)
	shown := uint64(0)
	if w != nil {
		shown = w.popoversShown
	}
	previous, candidate := b.revision, next.revision
	b.Close()
	if b.Destroyed() || next.Destroyed() || b.revision != previous+1 || next.revision != candidate || b.Root() != root || next.Root() != root || b.group != group || next.group != group || w != nil && (w.destroyed || w.popoversShown != shown) {
		return nil
	}
	if fromInput {
		next.openFromInput()
		return nil
	}
	return next.Open()
}

func (b *MenuButton) neighbor(direction int) *MenuButton {
	if b.linkedGroup == nil || b.Root() == nil {
		return nil
	}
	var members []*MenuButton
	var walk func(Widget)
	walk = func(widget Widget) {
		if widget == nil || !widget.Visible() || widget.base().destroyed {
			return
		}
		if candidate, ok := widget.(*MenuButton); ok && candidate.group == b.linkedGroup && IsEnabled(candidate) && candidate.hasMenu() {
			members = append(members, candidate)
		}
		for _, child := range widget.Children() {
			walk(child)
		}
	}
	walk(b.Root().Widget())
	for i, member := range members {
		if member == b && len(members) > 1 {
			return members[(i+direction+len(members))%len(members)]
		}
	}
	return nil
}
func (b *MenuButton) menuKey(ctx EventContext, event events.KeyEvent) {
	if !b.Opened() || !b.Focused() {
		return
	}
	if event.Key == events.KeyTab && (event.Modifiers == 0 || event.Modifiers == events.ModifierShift) {
		b.Close()
		return
	}
	if event.Modifiers != 0 {
		return
	}
	switch event.Key {
	case events.KeyArrowDown, events.KeyArrowUp, events.KeyHome, events.KeyEnd, events.KeyEnter, events.KeySpace, events.KeyArrowLeft, events.KeyArrowRight:
	default:
		return
	}
	handled := false
	forwarded := event
	forwarded.Handled = &handled
	if err := b.pm.popover.DispatchEvent(forwarded); err != nil {
		return
	}
	if !handled && (event.Key == events.KeyArrowLeft || event.Key == events.KeyArrowRight) {
		direction := 1
		if event.Key == events.KeyArrowLeft {
			direction = -1
		}
		if next := b.neighbor(direction); next != nil {
			b.switchMenu(next, true)
			handled = true
		}
	}
	if handled {
		event.PreventDefault()
		ctx.StopPropagation()
	}
}

func (b *MenuButton) Measure(c layout.Constraint) layout.Measurement {
	if !b.Visible() {
		return layout.Measurement{}
	}
	return measureButtonContent(&b.WidgetBase, c, b.padding, 0)
}
func (b *MenuButton) Arrange(rect geometry.Rectangle) {
	b.rect = rect
	if manager := b.LayoutManager(); manager != nil {
		manager.Arrange(b.visibleChildren(), geometry.Rect(0, 0, rect.Width, rect.Height).Inset(b.padding))
	}
}
func (b *MenuButton) Paint(p Painter) {
	if !b.Visible() {
		return
	}
	rect := geometry.Rect(0, 0, b.Rect().Width, b.Rect().Height)
	paintStyledBox(p, rect, b.resolvedStyle())
	paintButtonFocus(p, rect, &b.WidgetBase)
}
func (b *MenuButton) resolvedStyle() style.Style {
	state := style.Normal
	if !IsEnabled(b) {
		state = style.Disabled
	} else if b.pressed {
		state = style.Pressed
	} else if b.hovered {
		state = style.Hovered
	}
	part := style.PartDefault
	if b.Opened() {
		part = "expanded"
	}
	return ResolveWidgetStyle(b, buttonStyleName(&b.WidgetBase), part, state)
}
func (b *MenuButton) Snapshot() WidgetInfo {
	info := b.WidgetBase.Snapshot()
	info.Role = RoleButton
	if info.Enabled {
		info.Actions = append(info.Actions, ActionClick)
	}
	info.SetAttribute(MenuButtonInfoKey, MenuButtonInfo{HasMenu: b.hasMenu(), Opened: b.Opened(), Grouped: b.group != nil})
	return info
}
func (b *MenuButton) setHovered(value bool) {
	if b.hovered != value {
		b.hovered = value
		b.requestPaint()
	}
}
func (b *MenuButton) setPressed(value bool) {
	if b.pressed != value {
		b.pressed = value
		b.requestPaint()
	}
}
func (b *MenuButton) requestPaint() {
	if root := b.Root(); root != nil {
		_ = root.RequestPaint()
	}
}
