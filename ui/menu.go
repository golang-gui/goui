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
	label      string
	action     func()
	commandID  string
	commandRef *gui.Command
	definition *CommandView
	enabled    bool
	separator  bool
	submenu    []*MenuItemView
	branch     bool
}

// MenuItem creates an enabled, single-selectable menu entry.
func MenuItem(label string, action ...func()) *MenuItemView {
	if len(action) > 1 {
		panic("ui: MenuItem accepts at most one action")
	}
	v := &MenuItemView{label: label, enabled: true}
	if len(action) > 0 {
		v.action = action[0]
	}
	return v
}

// Command selects an operation from ancestor scopes instead of a direct action.
func (m *MenuItemView) Command(id string) *MenuItemView {
	m.commandID, m.commandRef, m.definition, m.action = id, nil, nil, nil
	m.submenu, m.branch = nil, false
	return m
}

// CommandRef borrows an external operation without requiring a CommandScope.
// It replaces Command and the direct action; the caller retains ownership.
func (m *MenuItemView) CommandRef(command *gui.Command) *MenuItemView {
	m.commandRef, m.commandID, m.definition, m.action = command, "", nil, nil
	m.submenu, m.branch = nil, false
	return m
}

// DefineCommand supplies a scope-local definition and uses it for this entry.
// It requires a mounted MenuButton below a CommandScope. Temporary Menu/MenuFor
// models must use Command or CommandRef instead. nil clears the operation.
func (m *MenuItemView) DefineCommand(command *CommandView) *MenuItemView {
	m.definition, m.commandID, m.commandRef, m.action = command, "", nil, nil
	m.submenu, m.branch = nil, false
	if command != nil {
		m.commandID = command.id
	}
	return m
}

// Submenu replaces the leaf action or command with child declarations.
// Calling it without entries creates an explicit empty branch.
func (m *MenuItemView) Submenu(items ...*MenuItemView) *MenuItemView {
	m.submenu, m.branch = items, true
	m.action, m.commandID, m.commandRef, m.definition = nil, "", nil, nil
	return m
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
	commands    []commandReference
	definitions commandDefinitions
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

func (v *MenuButtonView) Update(ctx BuildContext, widget gui.Widget) {
	button, s := widget.(*gui.MenuButton), ctx.State().(*menuButtonState)
	s.onClosed, s.onOpenError = v.onClosed, v.onOpenError
	s.items = append(s.items[:0], v.items...)
	if ctx.State() != s || button.Destroyed() {
		return
	}
	ctx.UpdateChild(button, v.child)
	if ctx.State() != s || button.Destroyed() {
		return
	}
	var definitions []*CommandView
	walkMenuItems(v.items, func(item *MenuItemView) {
		if !item.separator && item.definition != nil {
			definitions = append(definitions, item.definition)
		}
	})
	s.definitions.update(ctx, nil, definitions, "menu-command:")
	if ctx.State() != s || button.Destroyed() {
		return
	}
	sig := signatureOf(v.items)
	if sig == s.lastSig {
		references := s.commands
		for i := range references {
			references[i].reparent()
		}
		// Object identity is intentionally separate from the presentation
		// signature. Swapping borrowed commands retains the open menu model.
		syncMenuRefs(s.button.Menu(), v.items, func(entry *gui.MenuItem, item *MenuItemView) {
			if ctx.State() == s && !button.Destroyed() && item.commandRef != nil {
				entry.SetCommand(item.commandRef)
			}
		})
		return
	}
	for i := range s.commands {
		s.commands[i].clear()
	}
	referenceCount := 0
	walkMenuItems(v.items, func(item *MenuItemView) {
		if !item.separator && !item.branch && item.commandID != "" {
			referenceCount++
		}
	})
	s.commands = make([]commandReference, 0, referenceCount)
	// Keep the model across unrelated rebuilds. Its action closures resolve
	// the latest declaration through this node's state, without reloading rows.
	var build func([]*MenuItemView, []int) *gui.Menu
	build = func(items []*MenuItemView, parent []int) *gui.Menu {
		model := gui.NewMenu()
		for index, item := range items {
			if item == nil {
				continue
			}
			if item.separator {
				model.AppendSeparator()
				continue
			}
			path := append(append([]int(nil), parent...), index)
			entry := model.Append(item.label, func() {
				latest := menuItemAt(s.items, path)
				if latest != nil && !latest.separator && !latest.branch && latest.enabled && latest.action != nil {
					latest.action()
				}
			})
			entry.SetEnabled(item.enabled)
			if item.branch {
				entry.SetSubmenu(build(item.submenu, path))
			} else if item.commandID != "" {
				s.commands = append(s.commands, commandReference{})
				s.commands[len(s.commands)-1].update(ctx, item.commandID, entry.SetCommand)
			} else if item.commandRef != nil {
				entry.SetCommand(item.commandRef)
			}
			if ctx.State() != s || button.Destroyed() {
				return model
			}
		}
		return model
	}
	model := build(v.items, nil)
	if ctx.State() != s || button.Destroyed() {
		return
	}
	s.lastSig = sig
	button.SetMenu(model)
}
func (v *MenuButtonView) Unmount(ctx BuildContext, widget gui.Widget) {
	if s, ok := ctx.State().(*menuButtonState); ok {
		s.handles.Disconnect()
		s.definitions.clear()
		for i := range s.commands {
			s.commands[i].clear()
		}
		s.commands = nil
		s.items = nil
		s.onClosed = nil
		s.onOpenError = nil
	}
	button := widget.(*gui.MenuButton)
	button.Close()
}

var _ WidgetView = (*MenuButtonView)(nil)

// Menu converts menu item declarations into a fresh imperative menu model.
// It does not mount a View or execute item actions. Command IDs remain disabled
// without a scope context; use MenuFor from a composite View. Controls can use
// it when coordinating their own menu declarations, including menus queried
// at input time. An empty declaration produces an empty model.
func Menu(items ...*MenuItemView) MenuModel { return makeMenu(items...) }

func makeMenu(items ...*MenuItemView) *gui.Menu {
	walkMenuItems(items, func(*MenuItemView) {})
	return makeMenuLevel(items)
}

func makeMenuLevel(items []*MenuItemView) *gui.Menu {
	m := gui.NewMenu()
	for _, it := range items {
		if it == nil {
			continue
		}
		if it.separator {
			m.AppendSeparator()
			continue
		}
		if it.definition != nil {
			panic("ui: MenuItem.DefineCommand requires a mounted MenuButton; temporary menus must reference a command")
		}
		mi := m.Append(it.label, it.action)
		if it.branch {
			mi.SetSubmenu(makeMenuLevel(it.submenu))
		} else if it.commandRef != nil {
			mi.SetCommand(it.commandRef)
		} else if it.commandID != "" {
			missing := gui.NewCommand()
			missing.SetEnabled(false)
			mi.SetCommand(missing)
		}
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
	walkMenuItems(items, func(*MenuItemView) {})
	var b []byte
	var visit func([]*MenuItemView)
	visit = func(items []*MenuItemView) {
		for _, it := range items {
			if it == nil {
				b = append(b, ';')
				continue
			}
			if it.separator {
				b = append(b, '|')
				continue
			}
			b = strAppendUint(b, uint64(len(it.commandID)))
			b = append(b, ':')
			b = append(b, it.commandID...)
			if it.commandRef != nil {
				b = append(b, 'r')
			} else {
				b = append(b, 'a')
			}
			b = strAppendUint(b, uint64(len(it.label)))
			b = append(b, it.label...)
			if it.enabled {
				b = append(b, '1')
			} else {
				b = append(b, '0')
			}
			if it.branch {
				b = append(b, '{')
				visit(it.submenu)
				b = append(b, '}')
			}
			b = append(b, ';')
		}
	}
	visit(items)
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

func (*MenuButtonView) Reparented(ctx BuildContext) {
	if s, ok := ctx.State().(*menuButtonState); ok {
		s.definitions.reparented()
		references := s.commands
		for i := range references {
			references[i].reparent()
		}
	}
}

// MenuFor resolves command IDs for a context-menu request. While a presenter
// observes the returned model, scope membership changes also refresh its IDs.
// The context is borrowed and follows explicit subtree transfers.
func MenuFor(ctx BuildContext, items ...*MenuItemView) MenuModel {
	return scopedMenuLevel(ctx, makeMenu(items...), items)
}

func scopedMenuLevel(ctx BuildContext, menu *gui.Menu, items []*MenuItemView) MenuModel {
	model := &scopedMenu{Menu: menu, ctx: ctx}
	index := 0
	for _, item := range items {
		if item == nil {
			continue
		}
		entry := menu.ItemAt(index)
		index++
		if item.separator {
			continue
		}
		if item.branch {
			entry.SetSubmenu(scopedMenuLevel(ctx, entry.Submenu().(*gui.Menu), item.submenu))
		} else if item.commandID != "" {
			model.targets = append(model.targets, scopedMenuTarget{item: entry, id: item.commandID})
		}
	}
	if len(model.targets) == 0 {
		return model.Menu
	}
	model.refresh()
	return model
}

type scopedMenuTarget struct {
	item    *gui.MenuItem
	id      string
	missing *gui.Command
}
type scopedMenu struct {
	*gui.Menu
	ctx     BuildContext
	targets []scopedMenuTarget
}

func (m *scopedMenu) refresh() {
	for i := range m.targets {
		target := &m.targets[i]
		command, ok := findCommand(m.ctx, target.id)
		if !ok {
			if target.missing == nil {
				target.missing = gui.NewCommand()
				target.missing.SetEnabled(false)
			}
			command = target.missing
		}
		target.item.SetCommand(command)
	}
}
func (m *scopedMenu) ConnectItems(fn func()) signal.Handle {
	m.refresh()
	handles := signal.Handles{m.Menu.ConnectItems(fn)}
	if m.ctx != nil {
		if scope, ok := FindAncestor[*commandScope](m.ctx); ok {
			handles = append(handles, scope.changed.Connect(m.refresh))
		}
	}
	return handles
}

// Declarations may share branches, but cycles on one ancestor path are invalid.
func walkMenuItems(items []*MenuItemView, visit func(*MenuItemView)) {
	path := make(map[*MenuItemView]bool)
	var walk func([]*MenuItemView)
	walk = func(items []*MenuItemView) {
		for _, item := range items {
			if item == nil {
				continue
			}
			if path[item] {
				panic("ui: cyclic submenu declaration")
			}
			visit(item)
			if item.branch && !item.separator {
				path[item] = true
				walk(item.submenu)
				delete(path, item)
			}
		}
	}
	walk(items)
}
func menuItemAt(items []*MenuItemView, path []int) *MenuItemView {
	var item *MenuItemView
	for _, index := range path {
		if index >= len(items) {
			return nil
		}
		item = items[index]
		if item == nil {
			return nil
		}
		items = item.submenu
	}
	return item
}
func syncMenuRefs(model gui.MenuModel, items []*MenuItemView, visit func(*gui.MenuItem, *MenuItemView)) {
	index := 0
	for _, item := range items {
		if item == nil {
			continue
		}
		if model == nil || index >= model.ItemsCount() {
			return
		}
		entry := model.ItemAt(index)
		index++
		visit(entry, item)
		if item.branch && !item.separator {
			syncMenuRefs(entry.Submenu(), item.submenu, visit)
		}
	}
}
