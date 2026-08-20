package ui

import (
	"github.com/golang-gui/goui/gui"
)

// MenuItemView describes one menu entry (or a separator row). It is a plain
// declarative descriptor, not a widget: a MenuButton consumes the items to
// build and mount a gui.Menu model, which the imperative layer renders. Rebuild
// the descriptor each update to reflect current state (e.g. Enabled(state.Get())).
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
	child View
	items []*MenuItemView
}

type menuButtonState struct {
	lastSig string // fingerprint of the last-applied menu items
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

func (v *MenuButtonView) Build() View {
	return v
}

func (v *MenuButtonView) Mount(ctx BuildContext) gui.Widget {
	button := gui.NewMenuButton()
	ctx.SetState(&menuButtonState{})
	return button
}

func (v *MenuButtonView) Update(ctx BuildContext, widget gui.Widget) {
	button := widget.(*gui.MenuButton)
	state := ctx.State().(*menuButtonState)

	// Content child: reconcile the label/custom child into the button's single
	// slot. gui.MenuButton is a Bin, so updateBinChild handles it.
	if v.child == nil {
		ctx.UpdateChildren(button, nil)
	} else {
		ctx.UpdateChildren(button, []View{v.child})
	}

	// Menu model: rebuild only when the entries changed, so an open menu is not
	// needlessly reloaded on unrelated rebuilds. The fingerprint omits actions
	// (they rarely change identity per rebuild); changing a label or enabled
	// flag is what matters.
	sig := signatureOf(v.items)
	if sig != state.lastSig {
		button.SetMenu(buildMenu(v.items))
		state.lastSig = sig
	}
}

func (v *MenuButtonView) Unmount(BuildContext, gui.Widget) {}

// buildMenu coordinates the declarative item descriptors into a gui.Menu model.
func buildMenu(items []*MenuItemView) *gui.Menu {
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
