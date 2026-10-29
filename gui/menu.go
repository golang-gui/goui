package gui

import (
	"errors"
	"fmt"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/core/signal"
	"github.com/golang-gui/goui/layout"
	"github.com/golang-gui/goui/platform"
	"github.com/golang-gui/goui/platform/events"
	"github.com/golang-gui/goui/platform/graphics"
	"github.com/golang-gui/goui/style"
)

type MenuModel ListData[*MenuItem]

// MenuItem is one menu entry (a model record, not a Widget).
type MenuItem struct {
	label     string
	action    func()
	enabled   bool
	visible   bool
	separator bool
	onChanged func() // wired by Menu.Append; fires when presentation changes
}

func NewMenuItem(label string, action func()) *MenuItem {
	return &MenuItem{label: label, action: action, enabled: true, visible: true}
}

func (mi *MenuItem) Label() string   { return mi.label }
func (mi *MenuItem) Action() func()  { return mi.action }
func (mi *MenuItem) Enabled() bool   { return mi.enabled }
func (mi *MenuItem) Visible() bool   { return mi.visible }
func (mi *MenuItem) Separator() bool { return mi.separator }

// SetLabel updates presentation without replacing the entry or its action.
// Menus containing this item are notified so open menus remeasure and repaint.
func (mi *MenuItem) SetLabel(label string) {
	if mi.label == label {
		return
	}
	mi.label = label
	if mi.onChanged != nil {
		mi.onChanged()
	}
}

func (mi *MenuItem) SetEnabled(v bool) {
	if mi.enabled == v {
		return
	}
	mi.enabled = v
	if mi.onChanged != nil {
		mi.onChanged()
	}
}

func (mi *MenuItem) SetVisible(v bool) {
	if mi.visible == v {
		return
	}
	mi.visible = v
	if mi.onChanged != nil {
		mi.onChanged()
	}
}

// Menu is a model (a specialized list). It reuses SliceListModel with all its
// concurrency guarantees and — crucially — its change notification, so a
// PopoverMenu can render live updates while open (via ListView's reloading).
type Menu struct {
	*SliceListModel[*MenuItem]
}

func NewMenu() *Menu {
	return &Menu{SliceListModel: NewSliceListModel[*MenuItem](nil)}
}

func (m *Menu) Append(label string, action func()) *MenuItem {
	mi := NewMenuItem(label, action)
	m.appendItem(mi)
	return mi
}

func (m *Menu) AppendSeparator() *MenuItem {
	mi := &MenuItem{enabled: true, visible: true, separator: true}
	m.appendItem(mi)
	return mi
}

func (m *Menu) appendItem(mi *MenuItem) {
	mi.onChanged = m.changed.Emit
	m.SliceListModel.Append(mi)
}

// --- PopoverMenu: a Popover that renders a ListData[*MenuItem] ---

// PopoverMenu is a command-style menu control: a modal Popover anchored to a
// widget that renders a Menu model into a scrollable, virtualized row list.
// Its body prefers a minimum width of 160 DIP, excluding shadow; longer content
// expands it, and the available work area remains a hard size limit.
// Renderer internals (menuContent/ScrollView/ListView) are not public.
type PopoverMenu struct {
	anchor    Widget
	popover   Popover
	model     ListData[*MenuItem]
	content   *menuContent
	closed    signal.Signal0
	hClosed   signal.Handle
	maxHeight float32
	modal     bool
}

const menuScrollbarGap = 6

func NewPopoverMenu(anchor Widget) *PopoverMenu {
	return &PopoverMenu{anchor: anchor, modal: true}
}

// Modal reports the menu's input policy. Menus are modal by default.
func (pm *PopoverMenu) Modal() bool { return pm.modal }

// SetModal selects owner input exclusivity. A modeless presenter leaves key
// forwarding to its controlling widget; automatic dismissal stays enabled.
func (pm *PopoverMenu) SetModal(modal bool) {
	pm.modal = modal
	if pm.popover != nil {
		pm.popover.SetModal(modal)
	}
}

// SetMenu installs the model and refreshes the content, including when the same
// instance is supplied. Model implementations need not be comparable.
func (pm *PopoverMenu) SetMenu(m MenuModel) {
	pm.model = m
	if pm.content != nil {
		pm.content.SetModel(m)
	}
	if m == nil {
		pm.Hide()
	}
}

func (pm *PopoverMenu) Menu() MenuModel { return pm.model }

// SetMaxHeight optionally limits the menu body height in DIP, excluding shadow.
// The default is unrestricted. Non-positive or non-finite values clear the limit.
func (pm *PopoverMenu) SetMaxHeight(h float32) {
	h = normalizeLayoutValue(h)
	if pm.maxHeight == h {
		return
	}
	pm.maxHeight = h
	if pm.content != nil {
		pm.content.maxHeight = h
		pm.content.invalidate()
	}
}

func (pm *PopoverMenu) Visible() bool {
	return pm.popover != nil && pm.popover.Visible()
}

func (pm *PopoverMenu) ShowAt(pos geometry.Point) error {
	return pm.show(PopoverPlacementPoint, pos)
}

func (pm *PopoverMenu) show(placement PopoverPlacement, pos geometry.Point) error {
	if pm.model == nil {
		return nil
	}
	if pm.content == nil {
		pm.content = newMenuContent(pm.model, pm.maxHeight, pm.activate)
	}
	// Reuse an opaque fallback until its native surface is released; a new
	// surface must probe transparency again (e.g. after anchor migration).
	if p, ok := pm.popover.(*popover); ok && !p.transparent && p.platformPopup == nil {
		pm.replacePopover(true)
	}
	if pm.popover == nil {
		pm.replacePopover(true)
	}
	show := func() error {
		pm.popover.SetPosition(pos)
		pm.popover.SetPlacement(placement)
		return pm.popover.Show()
	}
	err := show()
	var creation *popoverCreationError
	if !pm.popover.Transparent() || !errors.As(err, &creation) || (!errors.Is(err, platform.ErrUnsupported) && !errors.Is(err, platform.ErrUnavailable)) {
		if err != nil {
			pm.popover.(*popover).releaseNative()
		}
		return err
	}
	pm.replacePopover(false)
	if retry := show(); retry != nil {
		pm.popover.Destroy()
		return fmt.Errorf("menu popup: transparent: %w; opaque: %w", err, retry)
	}
	return nil
}

func (pm *PopoverMenu) replacePopover(transparent bool) {
	if pm.hClosed != nil {
		pm.hClosed.Disconnect()
	}
	if pm.popover != nil {
		pm.popover.Destroy()
	}
	p := NewPopover(pm.anchor, &PopoverOptions{Transparent: transparent})
	p.SetStyleName(styleNameMenu)
	p.SetModal(pm.modal)
	p.SetAutoDismiss(true)
	p.ConnectDismissRequest(pm.Hide)
	pm.hClosed = p.ConnectClosed(pm.closed.Emit)
	pm.popover = p
	p.SetWidget(pm.content)
}

func (pm *PopoverMenu) Hide() {
	if pm.content != nil {
		pm.content.selectItem(nil)
		for _, w := range pm.content.list.items {
			r := w.(*menuItemRow)
			r.setPressed(false)
		}
	}
	if pm.popover != nil {
		pm.popover.Hide()
	}
}

func (pm *PopoverMenu) ConnectClosed(fn func()) signal.Handle {
	return pm.closed.Connect(fn)
}

func (pm *PopoverMenu) activate(mi *MenuItem) {
	if mi == nil || !mi.Enabled() {
		return
	}
	// Actions may enter a native modal loop or open another menu. Hide this
	// popup and release its modal input target before handing off control.
	action := mi.Action()
	pm.Hide()
	if action != nil {
		action()
	}
}

// --- internal renderer ---

// menuContent sizes the popover to the menu's natural size (capped at
// maxHeight) and hosts a ScrollView(ListView) for the scrollable, virtualized
// row list. The ScrollView content measure is viewport-driven, so this wrapper
// owns the intrinsic sizing.
type menuContent struct {
	WidgetBase
	sv       *ScrollView
	list     *ListView
	model    MenuModel
	delegate *menuItemDelegate

	maxHeight float32
	naturalW  float32 // cached natural size (invalidated on model change)
	naturalH  float32
	valid     bool
	hChanged  signal.Handle
	padding   float32
	active    *MenuItem
}

func newMenuContent(m MenuModel, maxHeight float32, activate func(*MenuItem)) *menuContent {
	mc := &menuContent{model: m, maxHeight: maxHeight}
	mc.SetMinSize(geometry.Size{Width: menuContentMinWidth})
	mc.delegate = &menuItemDelegate{model: m, onActivate: activate}
	mc.delegate.onHover = mc.selectItem
	mc.list = NewListView()
	mc.list.SetDelegate(mc.delegate)
	mc.sv = NewScrollView()
	mc.sv.SetChild(mc.list)
	mc.WidgetBase.AddChild(mc, mc.sv)
	mc.ConnectMount(mc.connectModel)
	mc.ConnectUnmount(mc.disconnectModel)
	key := NewKeyEventController()
	key.SetPhase(PhaseCapture)
	key.ConnectKeyDown(func(ctx EventContext, event events.KeyEvent) {
		if event.Modifiers != 0 {
			return
		}
		direction, edge := 0, false
		switch event.Key {
		case events.KeyArrowDown:
			direction = 1
		case events.KeyArrowUp:
			direction = -1
		case events.KeyHome:
			direction, edge = 1, true
		case events.KeyEnd:
			direction, edge = -1, true
		case events.KeyEnter, events.KeySpace:
			if mc.activeIndex() < 0 {
				mc.moveItem(1, true)
			}
			item := mc.active
			if item != nil && mc.activeIndex() >= 0 {
				activate(item)
			}
		default:
			return
		}
		if direction != 0 {
			mc.moveItem(direction, edge)
		}
		event.PreventDefault()
		ctx.StopPropagation()
	})
	mc.AddEventController(key)
	return mc
}

func (mc *menuContent) activeIndex() int {
	if mc.active == nil || !mc.active.Enabled() || !mc.active.Visible() || mc.active.Separator() || mc.model == nil {
		return -1
	}
	for i := 0; i < mc.model.ItemsCount(); i++ {
		if mc.model.ItemAt(i) == mc.active {
			return i
		}
	}
	return -1
}
func (mc *menuContent) selectItem(item *MenuItem) {
	if item != nil && (!item.Enabled() || !item.Visible() || item.Separator()) {
		return
	}
	if mc.active == item {
		return
	}
	mc.active = item
	mc.delegate.active = item
	for _, widget := range mc.list.items {
		row := widget.(*menuItemRow)
		row.SetState(style.Selected, row.mi == item && item != nil)
	}
}
func (mc *menuContent) moveItem(direction int, edge bool) {
	if mc.model == nil || mc.model.ItemsCount() == 0 {
		mc.selectItem(nil)
		return
	}
	n := mc.model.ItemsCount()
	at := mc.activeIndex()
	if edge || at < 0 {
		at = -1
		if direction < 0 {
			at = 0
		}
	}
	for range n {
		at = (at + direction + n) % n
		item := mc.model.ItemAt(at)
		if item != nil && item.Enabled() && item.Visible() && !item.Separator() {
			mc.selectItem(item)
			mc.list.Reveal(at)
			return
		}
	}
	mc.selectItem(nil)
}

func (mc *menuContent) connectModel() {
	if mc.hChanged != nil || mc.model == nil {
		return
	}
	mc.list.SetModel(mc.model)
	mc.hChanged = mc.model.ConnectItems(mc.invalidate)
	mc.invalidate()
}

func (mc *menuContent) disconnectModel() {
	mc.selectItem(nil)
	if mc.hChanged != nil {
		mc.hChanged.Disconnect()
		mc.hChanged = nil
	}
	mc.list.SetModel(nil) // unbind visible rows and disconnect the list's signal
	mc.valid = false
}

func (mc *menuContent) invalidate() {
	if mc.active != nil && mc.activeIndex() < 0 {
		mc.selectItem(nil)
	}
	mc.valid = false
	mc.RequestLayout()
}

// StyleChanged invalidates natural sizing; child Labels refresh their own text.
func (mc *menuContent) StyleChanged() { mc.invalidate() }

func (mc *menuContent) SetModel(m ListData[*MenuItem]) {
	mc.disconnectModel()
	mc.model = m
	mc.delegate.model = m
	if mc.Root() != nil {
		mc.connectModel()
	}
	mc.invalidate()
}

func (mc *menuContent) Measure(c layout.Constraint) layout.Measurement {
	if !mc.Visible() {
		return layout.Measurement{}
	}
	// The content owns padding, while its host owns the rounded perimeter.
	s := ResolveWidgetStyle(mc, styleNameMenu, style.PartDefault, style.Normal)
	radius, _ := s.Radius()
	border, _ := s.BorderWidth()
	padding := max(float32(menuContentPadding), popoverSafeInset(normalizeLayoutValue(radius), border))
	if padding != mc.padding {
		mc.padding, mc.valid = padding, false
	}
	if !mc.valid {
		mc.measureNatural()
	}
	c = mc.layoutConstraint(c)
	heightLimit := c.Max.Height
	if mc.maxHeight > 0 {
		heightLimit = min(heightLimit, mc.maxHeight)
	}
	w, h := mc.naturalW, min(mc.naturalH, heightLimit)
	// Scrollbars can force the other axis. Two monotonic passes account for
	// both, without overwriting the cached natural size or the user's limit.
	vertical, horizontal := false, false
	for range 2 {
		vertical = vertical || mc.naturalH > h
		w = mc.naturalW
		if vertical {
			w += scrollbarWidth + menuScrollbarGap
		}
		horizontal = horizontal || w > c.Max.Width
		h = mc.naturalH
		if horizontal {
			h += scrollbarWidth
		}
		h = min(h, heightLimit)
		if horizontal && mc.naturalH > max(0, h-scrollbarWidth) {
			vertical = true
		}
	}
	if vertical {
		w = mc.naturalW + scrollbarWidth + menuScrollbarGap
	}
	return layout.Measured(mc.constrain(c, geometry.Size{Width: w, Height: h}))
}

func (mc *menuContent) measureNatural() {
	var w, h float32
	n := 0
	if mc.model != nil {
		n = mc.model.ItemsCount()
	}
	for i := 0; i < n; i++ {
		row, mounted := mc.list.items[i].(*menuItemRow)
		if !mounted {
			row = newMenuItemRow(mc.delegate.onActivate)
			mc.delegate.Bind(i, row)
		}
		if !row.Visible() {
			continue
		}
		s := measureWidget(row, layout.Constraint{Min: geometry.Size{}, Max: geometry.Size{Width: layout.Inf, Height: layout.Inf}}).Size
		// Temporary rows never mount and need explicit resource cleanup.
		if !mounted {
			row.label.releaseLayout()
		}
		w = max(w, max(0, s.Width-row.trailingGap))
		h += s.Height
	}
	mc.naturalW, mc.naturalH = w+2*mc.padding, h+2*mc.padding
	mc.valid = true
}

func (mc *menuContent) Arrange(rect geometry.Rectangle) {
	mc.WidgetBase.Arrange(rect)
	inner := geometry.Rect(mc.padding, mc.padding, max(0, rect.Width-2*mc.padding), max(0, rect.Height-2*mc.padding))
	// Include the trailing gap in the scroll extent, not in the Label's text
	// width. Otherwise a wide row loses the last six pixels even when scrolled
	// fully right. Determine this from final geometry, not stale scrollbar state.
	contentW, contentH := mc.naturalW-2*mc.padding, mc.naturalH-2*mc.padding
	horizontal := contentW > inner.Width
	availableHeight := inner.Height
	if horizontal {
		availableHeight -= scrollbarWidth
	}
	vertical := contentH > availableHeight
	gap := float32(0)
	if vertical {
		gap = menuScrollbarGap
	}
	if gap != mc.delegate.trailingGap {
		mc.delegate.trailingGap = gap
		for _, widget := range mc.list.items {
			row := widget.(*menuItemRow)
			row.trailingGap = gap
			row.RequestLayout()
		}
		mc.list.StyleChanged()
	}
	measureWidget(mc.sv, layout.Tight(inner.Size))
	mc.sv.Arrange(inner)
}

func (mc *menuContent) Snapshot() WidgetInfo {
	info := mc.WidgetBase.Snapshot()
	info.Role = RoleMenu
	return info
}

// menuItemDelegate renders model items into menuItemRow widgets.
type menuItemDelegate struct {
	model       MenuModel
	onActivate  func(*MenuItem)
	trailingGap float32
	active      *MenuItem
	onHover     func(*MenuItem)
}

func (d *menuItemDelegate) Setup() Widget {
	r := newMenuItemRow(d.onActivate)
	r.onHover = d.onHover
	r.trailingGap = d.trailingGap
	return r
}

func (d *menuItemDelegate) Bind(i int, w Widget) {
	row := w.(*menuItemRow)
	row.trailingGap = d.trailingGap
	var mi *MenuItem
	if d.model != nil && i < d.model.ItemsCount() {
		mi = d.model.ItemAt(i)
	}
	row.bind(mi)
	row.SetState(style.Selected, mi != nil && mi == d.active && mi.Enabled() && mi.Visible() && !mi.Separator())
}

func (d *menuItemDelegate) Unbind(i int, w Widget) { w.(*menuItemRow).bind(nil) }

// menuItemRow paints the row background and hosts an independently styled Label.
// A separator row hides its Label, draws a thin line and is not interactive.
type menuItemRow struct {
	WidgetBase
	label       *Label
	trailingGap float32
	mi          *MenuItem
	activate    func(*MenuItem)
	onHover     func(*MenuItem)
	motion      *MotionEventController
	click       *ClickEventController
}

const (
	menuContentMinWidth = 160
	menuContentPadding  = 6
	menuItemPadding     = 8
	menuItemMinHeight   = 28
	menuSeparatorHeight = 9
)

func newMenuItemRow(activate func(*MenuItem)) *menuItemRow {
	r := &menuItemRow{activate: activate}
	r.label = NewLabel("")
	r.label.SetStyleName(styleNameMenuItemText)
	r.WidgetBase.AddChild(r, r.label)

	r.motion = NewMotionEventController()
	// Motion must also reach the row when its Label is the picked target.
	// Contains-enter covers native PointerEnter, which has no Motion delivery.
	r.motion.SetPhase(PhaseBubble)
	r.motion.ConnectContainsHover(func(inside bool) {
		if inside {
			r.selectPointer()
		}
	})
	r.motion.ConnectMotion(func(MotionInfo) {
		r.selectPointer()
	})
	r.AddEventController(r.motion)

	r.click = NewClickEventController()
	r.click.ConnectPressed(func(ctx EventContext, pressed bool) {
		if r.mi != nil && r.mi.Enabled() && !r.mi.Separator() {
			if pressed {
				r.selectPointer()
			}
			r.setPressed(pressed)
		}
	})
	r.click.ConnectClicked(func(ctx EventContext) {
		if r.mi == nil || !r.mi.Enabled() || r.mi.Separator() {
			return // separators and unbound rows are not actions
		}
		if r.activate != nil {
			r.activate(r.mi)
		}
	})
	r.AddEventController(r.click)
	return r
}

func (r *menuItemRow) bind(mi *MenuItem) {
	r.label.releaseLayout()
	r.click.Reset()
	r.motion.Reset()
	r.SetState(style.Pressed|style.Selected, false)
	r.mi = mi
	r.label.SetText("")
	r.label.SetVisible(mi != nil && mi.Visible() && !mi.Separator())
	name := styleNameMenuItemText
	if mi != nil && !mi.Enabled() {
		name = styleNameMenuItemTextDisabled
	}
	r.label.SetStyleName(name)
	if mi == nil || !mi.Visible() {
		r.SetVisible(false)
		return
	}
	r.SetVisible(true)
	if !mi.Separator() {
		r.label.SetText(mi.Label())
	}
	r.RequestLayout()
}

func (r *menuItemRow) Measure(c layout.Constraint) layout.Measurement {
	if !r.Visible() {
		return layout.Measurement{}
	}
	if r.mi != nil && r.mi.Separator() {
		return layout.Measured(r.constrain(c, geometry.Size{Height: menuSeparatorHeight}))
	}
	s := measureWidget(r.label, layout.Unbounded()).Size
	s.Width += menuItemPadding*2 + r.trailingGap
	s.Height = max(s.Height+8, menuItemMinHeight)
	return layout.Measured(r.constrain(c, s))
}

func (r *menuItemRow) Arrange(rect geometry.Rectangle) {
	// ListView allocates a full-width slot. Leave its trailing gutter outside
	// the row's actual bounds, so painting, picking and snapshots agree.
	rect.Width = max(0, rect.Width-r.trailingGap)
	r.WidgetBase.Arrange(rect)
	if r.label.Visible() {
		s := measureWidget(r.label, layout.Unbounded()).Size
		height := min(s.Height, rect.Height)
		r.label.Arrange(geometry.Rect(menuItemPadding, max(0, (rect.Height-height)/2), max(0, rect.Width-2*menuItemPadding), height))
	}
}

func (r *menuItemRow) Paint(p Painter) {
	if !r.Visible() {
		return
	}
	rect := geometry.Rect(0, 0, r.Rect().Width, r.Rect().Height)
	if r.mi != nil && r.mi.Separator() {
		s := ResolveWidgetStyle(r, styleNameMenuSeparator, style.PartDefault, style.Normal)
		if bg, ok := s.BackgroundColor(); ok && bg != nil {
			brush := graphics.ColorOf(bg)
			p.FillRect(geometry.Rect(menuItemPadding, rect.Height/2-0.5, rect.Width-menuItemPadding*2, 1), brush)
		}
		return
	}
	PaintStyledBox(p, rect, r.resolvedStyle())
}

func (r *menuItemRow) Snapshot() WidgetInfo {
	info := r.WidgetBase.Snapshot()
	info.Role = RoleMenuItem
	info.Selected = r.State().Has(style.Selected)
	info.Hovered = r.State().Has(style.Hovered)
	info.Pressed = r.State().Has(style.Pressed)
	info.Enabled = info.Enabled && r.mi != nil && r.mi.Enabled()
	if r.mi != nil {
		info.Text = r.label.PlainText()
		if r.mi.Separator() {
			info.Role = RoleMenuSeparator
		} else if info.Enabled {
			info.Actions = append(info.Actions, ActionClick)
		}
	}
	return info
}

// Model-disabled rows still receive pointer observations to clear the menu's
// current item; they expose Disabled without changing that navigation policy.
func (r *menuItemRow) State() style.State {
	state := r.WidgetBase.State()
	if r.mi != nil && !r.mi.Enabled() {
		state = (state | style.Disabled).Normalized() &^ style.DropActive
	}
	return state
}

func (r *menuItemRow) resolvedStyle() style.Style {
	st := r.State() &^ (style.Hovered | style.Pressed)
	if st.Has(style.Selected) {
		st |= style.Hovered
		if r.State().Has(style.Pressed) {
			st |= style.Pressed
		}
	}
	return ResolveWidgetStyle(r, styleNameMenuItem, style.PartDefault, st)
}

func (r *menuItemRow) selectPointer() {
	if r.onHover == nil {
		return
	}
	item := r.mi
	if item != nil && (!item.Enabled() || !item.Visible() || item.Separator()) {
		item = nil
	}
	r.onHover(item)
}

func (r *menuItemRow) setPressed(p bool) {
	r.SetState(style.Pressed, p)
}
