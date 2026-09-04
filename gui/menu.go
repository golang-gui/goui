package gui

import (
	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/core/signal"
	"github.com/golang-gui/goui/layout"
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
	onChanged func() // wired by Menu.Append; fires on SetEnabled/SetVisible
}

func NewMenuItem(label string, action func()) *MenuItem {
	return &MenuItem{label: label, action: action, enabled: true, visible: true}
}

func (mi *MenuItem) Label() string   { return mi.label }
func (mi *MenuItem) Action() func()  { return mi.action }
func (mi *MenuItem) Enabled() bool   { return mi.enabled }
func (mi *MenuItem) Visible() bool   { return mi.visible }
func (mi *MenuItem) Separator() bool { return mi.separator }

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
// Renderer internals (menuContent/ScrollView/ListView) are not public.
type PopoverMenu struct {
	anchor    Widget
	popover   Popover
	model     ListData[*MenuItem]
	content   *menuContent
	closed    signal.Signal0
	hClosed   signal.Handle
	maxHeight float32
}

const defaultMaxMenuHeight = 480
const minMenuWidth = 120

func NewPopoverMenu(anchor Widget) *PopoverMenu {
	return &PopoverMenu{anchor: anchor, maxHeight: defaultMaxMenuHeight}
}

func (pm *PopoverMenu) SetMenu(m MenuModel) {
	if pm.model == m {
		return
	}
	pm.model = m
	pm.content = nil // rebuilt on next ShowAt
}

func (pm *PopoverMenu) Menu() MenuModel { return pm.model }

func (pm *PopoverMenu) SetMaxHeight(h float32) {
	pm.maxHeight = h
}

func (pm *PopoverMenu) Visible() bool {
	return pm.popover != nil && pm.popover.Visible()
}

func (pm *PopoverMenu) ShowAt(pos geometry.Point) error {
	if pm.model == nil {
		return nil
	}
	if pm.popover == nil {
		p := NewPopover(pm.anchor)
		p.SetModal(true)
		// Dismiss (Esc / outside click / focus loss) must actually hide the
		// menu — popover.RequestDismiss only emits the request; the controller
		// owns the response.
		p.ConnectDismissRequest(func() { pm.Hide() })
		pm.hClosed = p.ConnectClosed(pm.closed.Emit)
		pm.popover = p
	}
	if pm.content == nil {
		pm.content = newMenuContent(pm.model, pm.maxHeight, pm.activate)
		pm.popover.SetWidget(pm.content)
	}
	pm.popover.SetPosition(pos)
	return pm.popover.Show()
}

func (pm *PopoverMenu) Hide() {
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
	if f := mi.Action(); f != nil {
		f()
	}
	pm.Hide()
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
}

func newMenuContent(m MenuModel, maxHeight float32, activate func(*MenuItem)) *menuContent {
	mc := &menuContent{model: m, maxHeight: maxHeight}
	mc.delegate = &menuItemDelegate{model: m, onActivate: activate}
	mc.list = NewListView()
	mc.list.SetDelegate(mc.delegate)
	mc.list.SetModel(m)
	mc.sv = NewScrollView()
	mc.sv.SetChild(mc.list)
	mc.WidgetBase.AddChild(mc, mc.sv)
	mc.hChanged = m.ConnectItems(mc.invalidate)
	return mc
}

func (mc *menuContent) invalidate() {
	mc.valid = false
}

func (mc *menuContent) SetModel(m ListData[*MenuItem]) {
	if mc.model == m {
		return
	}
	if mc.hChanged != nil {
		mc.hChanged.Disconnect()
	}
	mc.model = m
	mc.delegate.model = m
	mc.list.SetModel(m)
	mc.hChanged = m.ConnectItems(mc.invalidate)
	mc.invalidate()
}

func (mc *menuContent) Measure(c layout.Constraint) geometry.Size {
	if !mc.Visible() {
		return geometry.Size{}
	}
	if !mc.valid {
		mc.measureNatural()
	}
	w, h := mc.naturalW, mc.naturalH
	if h > mc.maxHeight {
		h = mc.maxHeight
	}
	if w < minMenuWidth {
		w = minMenuWidth
	}
	return geometry.Size{Width: w, Height: h}
}

func (mc *menuContent) measureNatural() {
	var w, h float32
	n := 0
	if mc.model != nil {
		n = mc.model.ItemsCount()
	}
	for i := 0; i < n; i++ {
		row := newMenuItemRow(mc.delegate.onActivate)
		mc.delegate.Bind(i, row)
		if !row.Visible() {
			continue
		}
		s := row.Measure(layout.Constraint{Min: geometry.Size{}, Max: geometry.Size{Width: layout.Inf, Height: layout.Inf}})
		w = max(w, s.Width)
		h += s.Height
	}
	mc.naturalW, mc.naturalH = w, h
	mc.valid = true
}

func (mc *menuContent) Arrange(rect geometry.Rectangle) {
	mc.WidgetBase.Arrange(rect)
	mc.sv.Arrange(geometry.Rect(0, 0, rect.Width, rect.Height))
}

func (mc *menuContent) Paint(p Painter) {
	if !mc.Visible() {
		return
	}
	rect := geometry.Rect(0, 0, mc.Rect().Width, mc.Rect().Height)
	paintStyledBox(p, rect, ResolveStyle(styleNameMenu, style.PartDefault, style.Normal))
}

// menuItemDelegate renders model items into menuItemRow widgets.
type menuItemDelegate struct {
	model      MenuModel
	onActivate func(*MenuItem)
}

func (d *menuItemDelegate) Setup() Widget { return newMenuItemRow(d.onActivate) }

func (d *menuItemDelegate) Bind(i int, w Widget) {
	row := w.(*menuItemRow)
	var mi *MenuItem
	if d.model != nil && i < d.model.ItemsCount() {
		mi = d.model.ItemAt(i)
	}
	row.bind(mi)
}

func (d *menuItemDelegate) Unbind(i int, w Widget) {}

// menuItemRow is one rendered menu row: a label plus hover/pressed/disabled
// state. A separator row draws a thin line and is not interactive.
type menuItemRow struct {
	WidgetBase
	label    *Label
	mi       *MenuItem
	activate func(*MenuItem)
	hovered  bool
	pressed  bool
	motion   *MotionEventController
	click    *ClickEventController
}

const (
	menuItemPadding     = 6
	menuItemMinHeight   = 24
	menuSeparatorHeight = 9
)

func newMenuItemRow(activate func(*MenuItem)) *menuItemRow {
	r := &menuItemRow{activate: activate}
	r.label = NewLabel("")
	r.WidgetBase.AddChild(r, r.label)

	r.motion = NewMotionEventController()
	r.motion.ConnectContainsHover(r.setHovered)
	r.AddEventController(r.motion)

	r.click = NewClickEventController()
	r.click.ConnectPressed(func(ctx EventContext, pressed bool) {
		if r.mi != nil && !r.mi.Separator() {
			r.setPressed(pressed)
		}
	})
	r.click.ConnectClicked(func(ctx EventContext) {
		if r.mi == nil || r.mi.Separator() {
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
	r.mi = mi
	if mi == nil || !mi.Visible() {
		r.SetVisible(false)
		return
	}
	r.SetVisible(true)
	r.setHovered(false)
	r.setPressed(false)
	if mi.Separator() {
		r.label.SetText("")
		r.label.SetVisible(false)
	} else {
		r.label.SetText(mi.Label())
		r.label.SetVisible(true)
	}
	r.RequestLayout()
}

func (r *menuItemRow) Measure(c layout.Constraint) geometry.Size {
	if !r.Visible() {
		return geometry.Size{}
	}
	if r.mi != nil && r.mi.Separator() {
		return geometry.Size{Width: minMenuWidth, Height: menuSeparatorHeight}
	}
	s := r.label.Measure(layout.Constraint{Min: geometry.Size{}, Max: geometry.Size{Width: layout.Inf, Height: layout.Inf}})
	s.Width += menuItemPadding * 2
	s.Height = max(s.Height, menuItemMinHeight)
	return s
}

func (r *menuItemRow) Arrange(rect geometry.Rectangle) {
	r.WidgetBase.Arrange(rect)
	r.label.Arrange(geometry.Rect(menuItemPadding, 0, rect.Width-menuItemPadding*2, rect.Height))
}

func (r *menuItemRow) Paint(p Painter) {
	if !r.Visible() {
		return
	}
	rect := geometry.Rect(0, 0, r.Rect().Width, r.Rect().Height)
	if r.mi != nil && r.mi.Separator() {
		s := ResolveStyle(styleNameMenuSeparator, style.PartDefault, style.Normal)
		if bg, ok := s.BackgroundColor(); ok && bg != nil {
			brush := graphics.ColorOf(bg)
			p.FillRect(geometry.Rect(menuItemPadding, rect.Height/2-0.5, rect.Width-menuItemPadding*2, 1), brush)
		}
		return
	}
	paintStyledBox(p, rect, r.resolvedStyle())
}

func (r *menuItemRow) resolvedStyle() style.Style {
	st := style.Normal
	if r.mi != nil && !r.mi.Enabled() {
		st = style.Disabled
	} else if r.pressed {
		st = style.Pressed
	} else if r.hovered {
		st = style.Hovered
	}
	return ResolveStyle(styleNameMenuItem, style.PartDefault, st)
}

func (r *menuItemRow) setHovered(h bool) {
	if r.hovered == h {
		return
	}
	r.hovered = h
	r.requestPaint()
}

func (r *menuItemRow) setPressed(p bool) {
	if r.pressed == p {
		return
	}
	r.pressed = p
	r.requestPaint()
}

func (r *menuItemRow) requestPaint() {
	if root := r.Root(); root != nil {
		_ = root.RequestPaint()
	}
}

// --- MenuButton: a button that opens a PopoverMenu below itself ---

// MenuButton is a command-style button that opens a PopoverMenu on click.
// It mirrors the Button widget's single-child + fill layout conventions.
type MenuButton struct {
	WidgetBase
	content Widget
	padding float32
	hovered bool
	pressed bool
	menu    ListData[*MenuItem]
	pm      *PopoverMenu
	click   *ClickEventController
	motion  *MotionEventController
	closed  signal.Signal0
}

func NewMenuButton() *MenuButton {
	b := &MenuButton{padding: defaultButtonPadding}
	b.SetFocusable(true)
	b.SetLayoutManager(layout.NewFillLayout())

	b.motion = NewMotionEventController()
	b.motion.ConnectContainsHover(b.setHovered)
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

func (b *MenuButton) SetMenu(m MenuModel) {
	b.menu = m
}

func (b *MenuButton) Menu() MenuModel { return b.menu }

func (b *MenuButton) ConnectClosed(fn func()) signal.Handle {
	return b.closed.Connect(fn)
}

func (b *MenuButton) openMenu() {
	if b.menu == nil {
		return
	}
	if b.pm == nil {
		b.pm = NewPopoverMenu(b)
		b.pm.ConnectClosed(b.closed.Emit)
	}
	b.pm.SetMenu(b.menu)
	_ = b.pm.ShowAt(geometry.Point{X: 0, Y: b.Rect().Height})
}

func (b *MenuButton) Measure(c layout.Constraint) geometry.Size {
	if !b.Visible() {
		return geometry.Size{}
	}
	var content geometry.Size
	if manager := b.LayoutManager(); manager != nil {
		content = manager.Measure(b.visibleChildren(), layout.Loose(c.Max.Inset(b.padding))).Inset(-b.padding)
	}
	return b.constrain(c, content)
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
}

func (b *MenuButton) resolvedStyle() style.Style {
	st := style.Normal
	if b.pressed {
		st = style.Pressed
	} else if b.hovered {
		st = style.Hovered
	}
	return ResolveStyle(styleNameButton, style.PartDefault, st)
}

func (b *MenuButton) Snapshot() WidgetInfo {
	info := b.WidgetBase.Snapshot()
	info.Role = RoleButton
	info.Actions = append(info.Actions, ActionClick)
	return info
}

func (b *MenuButton) setHovered(h bool) {
	if b.hovered == h {
		return
	}
	b.hovered = h
	b.requestPaint()
}

func (b *MenuButton) setPressed(p bool) {
	if b.pressed == p {
		return
	}
	b.pressed = p
	b.requestPaint()
}

func (b *MenuButton) requestPaint() {
	if root := b.Root(); root != nil {
		_ = root.RequestPaint()
	}
}
