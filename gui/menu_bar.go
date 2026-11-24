package gui

import (
	"errors"
	"fmt"
	"slices"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/core/signal"
	"github.com/golang-gui/goui/gui/textformat"
	"github.com/golang-gui/goui/layout"
	"github.com/golang-gui/goui/platform"
	"github.com/golang-gui/goui/platform/events"
)

// MenuBarMode selects the presentation of a menu bar's business menus.
type MenuBarMode uint8

const (
	// MenuBarAuto uses the system menu bar when available, otherwise the window.
	MenuBarAuto MenuBarMode = iota
	// MenuBarInWindow always presents the business menus inside the window.
	MenuBarInWindow
)

// MenuBarInfoKey identifies MenuBarInfo in semantic snapshots.
const MenuBarInfoKey = "goui.menuBar"

// MenuBarInfo describes presentation without exposing native resources.
type MenuBarInfo struct {
	Mode   MenuBarMode `json:"mode"`
	Native bool        `json:"native"`
}

// MenuBar presents a borrowed menu model. Every top-level entry must be a
// submenu. On macOS Auto uses the application menu bar of the main window;
// elsewhere it behaves like a local MenuButtonGroup. Native presentation keeps
// the widget mounted but removes it from window layout, painting and input.
// All mutations and callbacks belong to the GUI thread.
type MenuBar struct {
	WidgetBase
	model                               MenuModel
	mode                                MenuBarMode
	group                               *MenuButtonGroup
	buttons                             map[*MenuItem][]*menuBarButton
	handles                             signal.Handles
	errors                              signal.Signal2[error, uint64]
	host                                *window
	revision                            uint64
	ids                                 map[menuBarID]uint64
	records                             map[uint64][]menuBarSource
	dirty, failed, conflict, refreshing bool
	contextHandle                       signal.Handle
	context                             menuBarContext
}
type menuBarButton struct {
	button   *MenuButton
	label    *Label
	revision uint64
}
type menuBarID struct {
	parent                    uint64
	item                      *MenuItem
	occurrence                int
	revision, commandRevision uint64
}
type menuBarSource struct {
	model           MenuModel
	item            *MenuItem
	revision        uint64
	commandRevision uint64
}

// NewMenuBar creates an empty, adaptive menu bar.
func NewMenuBar() *MenuBar {
	b := &MenuBar{group: NewMenuButtonGroup(), dirty: true}
	b.SetLayoutManager(layout.NewLinearLayout(layout.DirectionHorizontal))
	b.WidgetBase.AddChild(b, b.group)
	b.ConnectMount(b.mountMenuBar)
	b.ConnectUnmount(b.unmountMenuBar)
	return b
}
func (b *MenuBar) Menu() MenuModel { return b.model }

// SetMenu installs a borrowed model. Reinstall to notify changes from a model
// that does not publish item notifications. nil clears the business menus.
func (b *MenuBar) SetMenu(model MenuModel) {
	if b.Destroyed() {
		return
	}
	b.model = model
	b.ids, b.records = nil, nil
	b.invalidateMenu()
}
func (b *MenuBar) Mode() MenuBarMode { return b.mode }
func (b *MenuBar) SetMode(mode MenuBarMode) {
	if mode != MenuBarAuto && mode != MenuBarInWindow {
		panic("gui: invalid MenuBarMode")
	}
	if b.mode != mode {
		b.mode = mode
		b.ids, b.records = nil, nil
		b.invalidateMenu()
	}
}

// ConnectError reports configuration and native presentation failures once per
// changed request. An available window presentation remains usable on failure.
func (b *MenuBar) ConnectError(fn func(error)) signal.Handle {
	return b.errors.Connect(func(err error, revision uint64) {
		if !b.Destroyed() && b.revision == revision {
			fn(err)
		}
	})
}
func (b *MenuBar) Paint(p Painter) { PaintWidgetStyle(p, b, "menu-bar") }
func (b *MenuBar) Snapshot() WidgetInfo {
	info := b.WidgetBase.Snapshot()
	info.Role = RoleMenuBar
	info.SetAttribute(MenuBarInfoKey, MenuBarInfo{b.mode, b.windowHidden})
	if b.windowHidden {
		info.Bounds = geometry.Rectangle{}
		info.Children = nil
	} else {
		info.Children = nil
		for _, child := range b.group.Children() {
			entry := child.(*MenuButton).Snapshot()
			entry.Role = RoleMenuItem
			entry.Text = child.(*MenuButton).Child().(*Label).PlainText()
			info.Children = append(info.Children, entry)
		}
	}
	return info
}
func (b *MenuBar) invalidateMenu() {
	b.revision++
	b.dirty = true
	b.failed = false
	// State-only changes keep valid bindings. Removal/replacement must take
	// effect immediately, even if a later update restores the same model.
	for id, path := range b.records {
		if !menuBarPathValid(path) {
			delete(b.records, id)
		}
	}
	for key, id := range b.ids {
		if _, ok := b.records[id]; !ok {
			delete(b.ids, key)
		}
	}
	if b.host != nil {
		b.host.requestMenuSync()
	} else if !b.refreshing {
		revision := b.revision
		b.dirty = false
		_, err := b.refreshMenu()
		if !b.Destroyed() && b.revision == revision {
			b.failed = err != nil
			b.reportMenuError(err)
		}
	}
}
func (b *MenuBar) mountMenuBar() {
	root, ok := b.Root().(interface{ rootState() *rootBase })
	if !ok {
		return
	}
	b.host, _ = b.Root().(*window)
	// Detached presentation IDs never belong to a window's action namespace.
	b.ids, b.records = nil, nil
	if b.host != nil {
		b.host.menuBars = append(b.host.menuBars, b)
		b.contextHandle = root.rootState().contextChanged.Connect(b.updateContext)
		b.context = b.currentContext()
	}
	b.invalidateMenu()
	if b.host != nil {
		// Establish presentation before the newly mounted subtree can be laid out.
		b.host.syncMenuBars()
	}
}
func (b *MenuBar) unmountMenuBar() {
	host := b.host
	b.revision++
	b.handles.Disconnect()
	b.handles = nil
	if b.contextHandle != nil {
		b.contextHandle.Disconnect()
		b.contextHandle = nil
	}
	b.ids, b.records = nil, nil
	b.host = nil
	b.setWindowHidden(false)
	b.dirty = true
	if host != nil {
		host.menuBars = slices.DeleteFunc(host.menuBars, func(other *MenuBar) bool { return other == b })
		if host.menuBar == b {
			host.menuBar = nil
			if native, ok := host.platformWindow.(platform.WindowMenuBar); ok && !host.destroyed {
				if err := native.SetMenu(nil); err != nil {
					b.reportMenuError(err)
				}
			}
		}
		host.requestMenuSync()
	}
}

type menuBarContext struct{ visible, enabled, blocked bool }

func (b *MenuBar) currentContext() menuBarContext {
	return menuBarContext{menuBarVisible(b), IsEnabled(b), menuBarBlocked(b)}
}
func (b *MenuBar) updateContext() {
	next := b.currentContext()
	if next == b.context {
		return
	}
	b.context = next
	if !next.visible || !next.enabled || next.blocked {
		b.ids, b.records = nil, nil
	}
	b.invalidateMenu()
}
func (b *MenuBar) reportMenuError(err error) {
	if err != nil && !b.Destroyed() {
		b.errors.Emit(fmt.Errorf("menu-bar: %w", err), b.revision)
	}
}
func (b *MenuBar) refreshMenu() ([]platform.MenuItem, error) {
	if b.refreshing {
		return nil, errors.New("recursive menu update")
	}
	b.refreshing = true
	defer func() { b.refreshing = false }()
	b.handles.Disconnect()
	b.handles = nil
	if b.host != nil {
		b.handles = append(b.handles, b.host.shortcutChanges.Connect(b.invalidateMenu))
		if controller := b.host.dispatcher.shortcuts; controller != nil {
			for _, shortcut := range controller.shortcuts {
				b.handles = append(b.handles, shortcut.changed.Connect(b.invalidateMenu))
			}
		}
	}
	records := map[uint64][]menuBarSource{}
	ids := map[menuBarID]uint64{}
	path := map[*MenuItem]bool{}
	var walk func(MenuModel, uint64, []menuBarSource, bool) ([]platform.MenuItem, error)
	walk = func(model MenuModel, parent uint64, ancestors []menuBarSource, enabled bool) ([]platform.MenuItem, error) {
		if model == nil {
			return nil, nil
		}
		if b.Root() != nil {
			b.handles = append(b.handles, model.ConnectItems(b.invalidateMenu))
		}
		var result []platform.MenuItem
		occurrences := map[*MenuItem]int{}
		for i := 0; i < model.ItemsCount(); i++ {
			item := model.ItemAt(i)
			if item == nil {
				continue
			}
			if path[item] {
				return nil, errors.New("cyclic submenu model")
			}
			if parent == 0 && (item.Separator() || item.Submenu() == nil) {
				return nil, errors.New("top-level entries must be submenus")
			}
			if b.Root() != nil {
				b.handles = append(b.handles, item.actionChanged.Connect(b.invalidateMenu))
			}
			if !item.Visible() {
				continue
			}
			commandRevision := uint64(0)
			if command := item.Command(); command != nil {
				commandRevision = command.revision
			}
			key := menuBarID{parent, item, occurrences[item], item.revision, commandRevision}
			occurrences[item]++
			id := uint64(0)
			if b.host != nil {
				id = b.ids[key]
			}
			if id == 0 {
				if b.host != nil {
					b.host.nextMenuID++
					id = b.host.nextMenuID
				} else {
					id = uint64(len(ids) + 1)
				}
			}
			ids[key] = id
			source := append(slices.Clone(ancestors), menuBarSource{model, item, item.revision, commandRevision})
			entry := platform.MenuItem{ID: id, Title: textformat.PlainText(item.Label()), Enabled: enabled && item.available(), Separator: item.Separator(), Submenu: item.Submenu() != nil}
			if c := item.Command(); c != nil {
				entry.Checked = c.Checkable() && c.Checked()
				entry.Shortcut = b.menuShortcut(c)
				if b.Root() != nil {
					b.handles = append(b.handles, c.ConnectEnabled(func(bool) { b.invalidateMenu() }), c.ConnectChecked(func(bool) { b.invalidateMenu() }), c.ConnectCheckable(func(bool) { b.invalidateMenu() }), c.ConnectShortcuts(b.invalidateMenu))
				}
			}
			records[id] = source
			if item.Submenu() != nil {
				path[item] = true
				children, err := walk(item.Submenu(), id, source, entry.Enabled)
				delete(path, item)
				if err != nil {
					return nil, err
				}
				entry.Children = children
			}
			result = append(result, entry)
		}
		return result, nil
	}
	items, err := walk(b.model, 0, nil, IsEnabled(b) && !b.context.blocked)
	b.ids, b.records = ids, records
	if err != nil {
		items = nil
		b.records = nil
	}
	b.syncButtons(err == nil)
	b.RequestLayout()
	return items, err
}
func (b *MenuBar) syncButtons(valid bool) {
	old := b.buttons
	b.buttons = map[*MenuItem][]*menuBarButton{}
	var order []*MenuButton
	if valid && b.model != nil {
		used := map[*MenuItem]int{}
		for i := 0; i < b.model.ItemsCount(); i++ {
			item := b.model.ItemAt(i)
			if item == nil || !item.Visible() {
				continue
			}
			index := used[item]
			used[item]++
			var entry *menuBarButton
			if index < len(old[item]) {
				entry = old[item][index]
			} else {
				button := NewMenuButton()
				button.SetStyleName("menu-bar-item")
				label := NewLabel("")
				button.SetChild(label)
				entry = &menuBarButton{button: button, label: label, revision: ^uint64(0)}
				button.ConnectOpenError(b.reportMenuError)
				b.group.AddChild(button)
			}
			entry.label.SetText(item.Label())
			entry.button.SetEnabled(item.available())
			if entry.revision != item.revision {
				entry.revision = item.revision
				entry.button.SetMenu(item.Submenu())
			}
			b.buttons[item] = append(b.buttons[item], entry)
			order = append(order, entry.button)
		}
	}
	for item, entries := range old {
		for i, entry := range entries {
			if i >= len(b.buttons[item]) {
				entry.button.destroy(entry.button)
			}
		}
	}
	for i := len(order) - 1; i >= 0; i-- {
		var next Widget
		if i+1 < len(order) {
			next = order[i+1]
		}
		b.group.MoveChildBefore(order[i], next)
	}
}
func menuBarPathValid(path []menuBarSource) bool {
	if len(path) == 0 {
		return false
	}
	for _, source := range path {
		if source.item.revision != source.revision || !menuContains(source.model, source.item) || !source.item.Visible() || !source.item.available() {
			return false
		}
		if command := source.item.Command(); command != nil && command.revision != source.commandRevision {
			return false
		}
	}
	return true
}
func (b *MenuBar) dispatchMenu(e events.MenuEvent) {
	if b.host == nil || b.host.destroyed || b.host.menuBar != b || b.mode != MenuBarAuto || !b.windowHidden || b.failed || !menuBarVisible(b) || !IsEnabled(b) || menuBarBlocked(b) {
		return
	}
	path := b.records[e.Item]
	if !menuBarPathValid(path) {
		return
	}
	item := path[len(path)-1].item
	if item.Submenu() != nil || item.Separator() {
		return
	}
	if command := item.Command(); command != nil {
		command.Activate()
	} else if action := item.Action(); action != nil {
		action()
	}
}
func menuBarVisible(b *MenuBar) bool {
	if b.Destroyed() || b.host == nil || b.host.destroyed {
		return false
	}
	for w := Widget(b); w != nil; w = w.Parent() {
		if !w.Visible() {
			return false
		}
	}
	return true
}
func menuBarBlocked(b *MenuBar) bool {
	return b.host == nil || b.host.destroyed || b.host.modalTarget != nil || !modalWidgetAllowed(b)
}

func (b *MenuBar) menuShortcut(command *Command) platform.MenuShortcut {
	if b.host == nil || b.host.dispatcher.shortcuts == nil {
		return platform.MenuShortcut{}
	}
	type gesture struct {
		key       events.Key
		modifiers events.Modifiers
	}
	seen := map[gesture]bool{}
	for _, s := range b.host.dispatcher.shortcuts.shortcuts {
		if !s.Enabled() || s.Command() != nil && !s.Command().Enabled() {
			continue
		}
		key, ok := s.Gesture().Key.Resolve()
		if !ok {
			continue
		}
		mods, ok := s.Gesture().Modifiers.Resolve()
		if !ok {
			continue
		}
		g := gesture{key, mods}
		if seen[g] {
			continue
		}
		seen[g] = true
		if s.Command() != command {
			continue
		}
		return platform.MenuShortcut{Key: key, Modifiers: mods}
	}
	return platform.MenuShortcut{}
}

// Synchronization belongs to this window's registered presenters, not to
// application-wide layout or paint paths. Unrelated context notifications do
// not dirty a presenter; the queued update coalesces actual menu changes.
func (w *window) requestMenuSync() {
	if w.destroyed || w.menuSyncQueued {
		return
	}
	w.menuSyncQueued = true
	if w.menuSyncing {
		return
	}
	if w.app != nil && w.app.loop != nil {
		w.app.Post(w.syncMenuBars)
	} else {
		w.syncMenuBars()
	}
}
func (w *window) syncMenuBars() {
	if w.destroyed || w.menuSyncing {
		return
	}
	w.menuSyncQueued = false
	w.menuSyncing = true
	defer func() {
		w.menuSyncing = false
		if w.menuSyncQueued {
			w.menuSyncQueued = false
			w.requestMenuSync()
		}
	}()
	native, supported := w.platformWindow.(platform.WindowMenuBar)
	selected := w.menuBar
	eligible := func(b *MenuBar) bool {
		return b != nil && b.host == w && b.mode == MenuBarAuto && menuBarVisible(b) && supported
	}
	if !eligible(selected) {
		selected = nil
		for _, b := range w.menuBars {
			if eligible(b) {
				selected = b
				break
			}
		}
	}
	if w.menuBar != selected {
		if w.menuBar != nil {
			old := w.menuBar
			w.menuBar = nil
			old.ids, old.records = nil, nil
			old.setWindowHidden(false)
			if supported {
				if err := native.SetMenu(nil); err != nil {
					old.reportMenuError(err)
				}
			}
		}
		w.menuBar = selected
		if selected != nil {
			selected.dirty = true
		}
	}
	for _, b := range slices.Clone(w.menuBars) {
		if b.host != w || b.Destroyed() {
			continue
		}
		conflict := eligible(b) && b != selected
		if conflict != b.conflict {
			b.conflict = conflict
			b.dirty = true
			b.failed = false
			if conflict {
				b.reportMenuError(errors.New("another Auto menu bar is already registered for this window"))
			}
		}
		if w.destroyed || b.host != w || b.Destroyed() {
			continue
		}
		if b.dirty {
			revision := b.revision
			b.dirty = false
			items, err := b.refreshMenu()
			if w.destroyed || b.host != w || b.Destroyed() || b.revision != revision {
				continue
			}
			if err == nil && b == selected && !conflict {
				err = native.SetMenu(items)
			}
			if err != nil {
				b.failed = true
				b.ids, b.records = nil, nil
				if b == selected {
					err = errors.Join(err, native.SetMenu(nil))
				}
				b.setWindowHidden(false)
				b.reportMenuError(err)
			}
		}
		if w.destroyed || b.host != w || b.Destroyed() || b.dirty {
			continue
		}
		external := b == w.menuBar && !b.failed && !b.conflict
		if external && !b.windowHidden {
			for _, child := range b.group.Children() {
				child.(*MenuButton).Close()
				if w.destroyed || b.host != w || b.Destroyed() || b.dirty {
					break
				}
			}
		}
		if !w.destroyed && b.host == w && !b.Destroyed() && !b.dirty {
			b.setWindowHidden(external)
		}
	}
}
