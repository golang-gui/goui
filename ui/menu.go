package ui

import (
	"github.com/golang-gui/goui/core/signal"
	"github.com/golang-gui/goui/gui"
)

// MenuModel is the imperative model consumed by GUI menu presenters.
type MenuModel = gui.MenuModel

// MenuItemView describes one menu entry (or a separator row). It is a plain
// declarative descriptor, not a widget: menu presenters consume the items to
// build a GUI menu model. Rebuild the descriptor each update (or each context
// menu request) to reflect current state, e.g. Enabled(state.Get()).
type MenuItemView struct {
	label     string
	action    func()
	enabled   bool
	separator bool
	submenu   []*MenuItemView // reserved for the future submenu slice
}

// MenuItem creates an enabled, single-selectable menu entry.
func MenuItem(label string, action func()) *MenuItemView {
	return &MenuItemView{label: label, action: action, enabled: true}
}

// Enabled marks the item's enable state; re-evaluate each rebuild to react to
// state changes (e.g. Enabled(canOpen.Get())).
func (m *MenuItemView) Enabled(enabled bool) *MenuItemView {
	m.enabled = enabled
	return m
}

// MenuSeparator creates a divider row. It is never selectable and carries no
// action or label.
func MenuSeparator() *MenuItemView {
	return &MenuItemView{separator: true}
}

// MenuButtonView is the declarative wrapper of gui.MenuButton: a button that
// opens a popover menu below itself when clicked. The button's content is a
// single child (label shorthand or a custom View), and Menu holds the entries.
type MenuButtonView struct {
	ViewBase[MenuButtonView]
	child       View
	items       []*MenuItemView
	group       string
	onClosed    func()
	onOpenError func(error)
}

type menuButtonState struct {
	lastSig     string // fingerprint of the last-applied menu items
	button      *gui.MenuButton
	items       []*MenuItemView
	onClosed    func()
	onOpenError func(error)
	handles     signal.Handles
}

// MenuButton creates a menu button. Like ui.Button, pass an optional label
// string; use Child for a custom label/view. Attach entries with Menu.
func MenuButton(text ...string) *MenuButtonView {
	if len(text) > 1 {
		panic("ui: MenuButton accepts at most one text argument")
	}
	v := &MenuButtonView{}
	v.Self = v
	if len(text) == 1 {
		v.child = Label(text[0])
	}
	return v
}

// Child sets a custom child view (overrides the label shorthand).
func (v *MenuButtonView) Child(child View) *MenuButtonView {
	v.child = child
	return v
}

// Menu sets the button's menu entries.
func (v *MenuButtonView) Menu(items ...*MenuItemView) *MenuButtonView {
	v.items = items
	return v
}

// Group references an explicit ancestor MenuButtonGroup declaration by ID.
// Empty leaves the button independent. Missing or incompatible groups panic.
func (v *MenuButtonView) Group(id string) *MenuButtonView            { v.group = id; return v }
func (v *MenuButtonView) OnClosed(fn func()) *MenuButtonView         { v.onClosed = fn; return v }
func (v *MenuButtonView) OnOpenError(fn func(error)) *MenuButtonView { v.onOpenError = fn; return v }

func (v *MenuButtonView) Mount(ctx BuildContext) gui.Widget {
	button := gui.NewMenuButton()
	s := &menuButtonState{button: button}
	s.handles = signal.Handles{
		button.ConnectClosed(func() {
			if s.onClosed != nil {
				s.onClosed()
			}
		}),
		button.ConnectOpenError(func(err error) {
			if s.onOpenError != nil {
				s.onOpenError(err)
			}
		}),
	}
	ctx.SetState(s)
	return button
}

func (v *MenuButtonView) applyGroup(ctx BuildContext, s *menuButtonState) {
	if v.group == "" {
		s.button.SetGroup(nil)
		return
	}
	group, ok := FindAncestor[*gui.MenuButtonGroup](ctx, v.group)
	if ctx.State() != s || s.button.Destroyed() {
		return
	}
	if !ok || group == nil {
		panic("ui: MenuButton requires an ancestor MenuButtonGroup with ID " + v.group)
	}
	s.button.SetGroup(group)
}
func (v *MenuButtonView) Update(ctx BuildContext, widget gui.Widget) {
	button, s := widget.(*gui.MenuButton), ctx.State().(*menuButtonState)
	s.onClosed, s.onOpenError = v.onClosed, v.onOpenError
	s.items = append(s.items[:0], v.items...)
	v.applyGroup(ctx, s)
	if ctx.State() != s || button.Destroyed() {
		return
	}
	ctx.UpdateChild(button, v.child)
	if ctx.State() != s || button.Destroyed() {
		return
	}
	sig := signatureOf(v.items)
	if sig == s.lastSig {
		return
	}
	// Keep the model across unrelated rebuilds. Its action closures resolve
	// the latest declaration through this node's state, without reloading rows.
	model := gui.NewMenu()
	for index, item := range v.items {
		if item == nil {
			continue
		}
		if item.separator {
			model.AppendSeparator()
			continue
		}
		model.Append(item.label, func() {
			if index < len(s.items) {
				item := s.items[index]
				if item != nil && !item.separator && item.enabled && item.action != nil {
					item.action()
				}
			}
		}).SetEnabled(item.enabled)
	}
	s.lastSig = sig
	button.SetMenu(model)
}
func (v *MenuButtonView) Unmount(ctx BuildContext, widget gui.Widget) {
	if s, ok := ctx.State().(*menuButtonState); ok {
		s.handles.Disconnect()
		s.items = nil
		s.onClosed = nil
		s.onOpenError = nil
	}
	button := widget.(*gui.MenuButton)
	button.SetGroup(nil)
	button.Close()
}
func (v *MenuButtonView) Reparented(ctx BuildContext) {
	if s, ok := ctx.State().(*menuButtonState); ok {
		v.applyGroup(ctx, s)
	}
}

var _ WidgetView = (*MenuButtonView)(nil)
var _ ReparentedView = (*MenuButtonView)(nil)

// Menu converts menu item declarations into a fresh imperative menu model.
// It does not mount a View or execute item actions. Composite controls can use
// it when coordinating their own menu declarations, including menus queried
// at input time. An empty declaration produces an empty model.
func Menu(items ...*MenuItemView) MenuModel {
	m := gui.NewMenu()
	for _, it := range items {
		if it == nil {
			continue
		}
		if it.separator {
			m.AppendSeparator()
			continue
		}
		mi := m.Append(it.label, it.action)
		if !it.enabled {
			mi.SetEnabled(false)
		}
	}
	return m
}

// signatureOf fingerprints the entries (labels, enabled flags, separators) for
// change detection. It intentionally ignores actions.
func signatureOf(items []*MenuItemView) string {
	// A simple composite: length-delimited labels plus enabled/separator markers.
	// Strings are stored in the descriptor, so reconstructing per call is fine.
	var b []byte
	for _, it := range items {
		if it == nil {
			b = append(b, ';')
			continue
		}
		if it.separator {
			b = append(b, '|')
			continue
		}
		b = strAppendUint(b, uint64(len(it.label)))
		b = append(b, it.label...)
		if it.enabled {
			b = append(b, '1')
		} else {
			b = append(b, '0')
		}
		b = append(b, ';')
	}
	return string(b)
}

func strAppendUint(b []byte, v uint64) []byte {
	var tmp [20]byte
	i := len(tmp)
	if v == 0 {
		i--
		tmp[i] = '0'
	}
	for v > 0 {
		i--
		tmp[i] = byte('0' + v%10)
		v /= 10
	}
	return append(b, tmp[i:]...)
}
