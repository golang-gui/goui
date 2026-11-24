package cocoa

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/ebitengine/purego/objc"
	"github.com/golang-gui/goui/platform/common"
	. "github.com/golang-gui/goui/platform/darwin/frameworks/appkit"
	. "github.com/golang-gui/goui/platform/darwin/frameworks/foundation"
	"github.com/golang-gui/goui/platform/events"
)

// menuBar is the private AppKit application-menu adapter. Each Window owns its
// projection; AppKit main-window transitions choose the installed projection.
type menuBar struct {
	platform            *Platform
	target              NSObject
	base                menuSnapshot
	queued              []menuSelection
	menus               map[*nativeMenu]bool
	installed           *nativeMenu
	installedRoot       objc.ID
	current             *Window
	epoch               uint64
	tracking            map[objc.ID]bool
	deferred, destroyed bool
	handlingKey         bool // defer structural changes during native key delivery
	quitting            bool // finish native menu tracking before ending the event loop
	appName             string
}
type menuSelection struct {
	menu        *nativeMenu
	epoch, item uint64
}
type menuSnapshot struct {
	root      NSMenu
	delegates []NSMenu
	items     []common.MenuItem
	actions   map[objc.ID]uint64
	objects   map[uint64]NSMenuItem
	targets   []NSMenuItem
	parents   map[objc.ID]uint64 // native business submenu to its parent item; root is zero
}
type nativeMenu struct {
	bar   *menuBar
	owner *Window
	menuSnapshot
	pending   *menuSnapshot
	destroyed bool
}

func (s *menuSnapshot) release() {
	for _, item := range s.targets {
		item.SetTarget(NSObject{})
	}
	for _, menu := range s.delegates {
		menu.SetDelegate(NSObject{})
	}
	if s.root.ID != 0 {
		s.root.Release()
	}
	*s = menuSnapshot{}
}

var menuTargetClass objc.Class
var menuTargets = map[objc.ID]*menuBar{}
var menuActionSel objc.SEL

func newMenuBar(p *Platform) (*menuBar, error) {
	if menuTargetClass == 0 {
		menuActionSel = objc.RegisterName("gouiMenuAction:")
		var err error
		menuTargetClass, err = objc.RegisterClass("GouiMenuTarget", NSObjectClassId.Class, nil, nil, []objc.MethodDef{
			{Cmd: menuActionSel, Fn: func(self objc.ID, _ objc.SEL, sender objc.ID) {
				if b := menuTargets[self]; b != nil {
					b.activate(sender)
				}
			}},
			{Cmd: NSMenuDelegateSel.MenuHasKeyEquivalent, Fn: func(self objc.ID, _ objc.SEL, menu objc.ID, event objc.ID, target *objc.ID, action *objc.SEL) bool {
				if b := menuTargets[self]; b != nil {
					return b.handleKeyEquivalent(Cast[NSMenu](menu), Cast[NSEvent](event), target, action)
				}
				return false
			}},
			{Cmd: NSMenuDelegateSel.MenuWillOpen, Fn: func(self objc.ID, _ objc.SEL, menu objc.ID) {
				if b := menuTargets[self]; b != nil {
					b.tracking[menu] = true
				}
			}},
			{Cmd: NSMenuDelegateSel.MenuDidClose, Fn: func(self objc.ID, _ objc.SEL, menu objc.ID) {
				if b := menuTargets[self]; b != nil {
					delete(b.tracking, menu)
					b.schedule()
				}
			}},
		})
		if err != nil {
			return nil, err
		}
	}
	b := &menuBar{platform: p, menus: map[*nativeMenu]bool{}, tracking: map[objc.ID]bool{}, appName: filepath.Base(os.Args[0])}
	AutoReleasePool(func() {
		bundle := NSBundleClassId.MainBundle()
		for _, key := range []string{"CFBundleDisplayName", "CFBundleName"} {
			if name := StringFromObject(bundle.ObjectForInfoDictionaryKey(menuString(key)).ID); name != "" {
				b.appName = name
				break
			}
		}
		b.target = Cast[NSObject](objc.ID(menuTargetClass).Send(NSObjectSel.New))
	})
	if b.target.ID == 0 {
		return nil, errors.New("menu bar: create action target failed")
	}
	menuTargets[b.target.ID] = b
	var err error
	b.base, err = b.build(nil)
	if err != nil {
		delete(menuTargets, b.target.ID)
		b.target.Release()
		return nil, err
	}
	b.current = windowMap[NSApp.MainWindow()]
	b.install()
	return b, nil
}

// SetMenu copies a window's menu description. Clearing it cannot allocate and
// remains safe when the window is no longer the application's main window.
func (w *Window) SetMenu(items []common.MenuItem) error {
	if w.closed || w.destroying {
		return common.ErrUnavailable
	}
	if !w.menuHost {
		return common.ErrUnsupported
	}
	if platform == nil || platform.menus == nil {
		return common.ErrUnavailable
	}
	b := platform.menus
	if len(items) == 0 {
		if w.menu != nil {
			w.menu.destroy()
			w.menu = nil
		}
		return nil
	}
	m := w.menu
	if m == nil {
		m = &nativeMenu{bar: b, owner: w}
	}
	if err := m.setItems(items); err != nil {
		return err
	}
	b.menus[m], w.menu = true, m
	if !b.busy() {
		b.install()
	}
	return nil
}

var _ common.WindowMenuBar = (*Window)(nil)

func menuString(value string) NSString { s := ToNSString(value); s.AutoRelease(); return s }
func (b *menuBar) setWindow(w *Window) {
	if b.destroyed || b.current == w {
		return
	}
	b.current = w
	b.epoch++
	b.cancelTracking()
	if !b.busy() {
		b.install()
	} else {
		b.schedule()
	}
}
func (b *menuBar) cancelTracking() {
	// CancelTracking may synchronously send menuDidClose and mutate tracking.
	var menus []objc.ID
	for id := range b.tracking {
		menus = append(menus, id)
	}
	for _, id := range menus {
		Cast[NSMenu](id).CancelTracking()
	}
}
func (b *menuBar) install() {
	if b.destroyed {
		return
	}
	var m *nativeMenu
	if b.current != nil && !b.current.closed {
		m = b.current.menu
	}
	b.installed = m
	root := b.base.root
	if m != nil && !m.destroyed {
		root = m.root
	}
	if b.installedRoot != root.ID {
		AutoReleasePool(func() { NSApp.SetMainMenu(root) })
		b.installedRoot = root.ID
	}
}
func (b *menuBar) schedule() {
	if b.deferred || b.destroyed || b.busy() {
		return
	}
	b.deferred = true
	b.platform.loop.Post(func() {
		b.deferred = false
		if b.destroyed || b.busy() || b.platform.loop.state.Quitting() {
			return
		}
		if b.quitting {
			b.quitting = false
			b.platform.loop.Quit()
			return
		}
		for m := range b.menus {
			if m.destroyed {
				m.release()
			} else if m.pending != nil {
				m.commit()
			}
		}
		b.install()
		selections := b.queued
		b.queued = nil
		for _, e := range selections {
			if b.destroyed {
				break
			}
			m := e.menu
			if !m.destroyed && b.epoch == e.epoch && b.current == m.owner && !m.owner.closed && m.owner.menu == m {
				m.owner.emitEvent(events.MenuEvent{Item: e.item})
			}
		}
	})
}
func (b *menuBar) activate(sender objc.ID) {
	m := b.installed
	if m == nil || m.destroyed || m.owner != b.current {
		return
	}
	if item, ok := m.actions[sender]; ok {
		b.queued = append(b.queued, menuSelection{m, b.epoch, item})
		b.cancelTracking()
		b.schedule()
	}
}
func (b *menuBar) destroy() {
	if b.destroyed {
		return
	}
	b.cancelTracking()
	b.destroyed = true
	AutoReleasePool(func() { NSApp.SetMainMenu(NSMenu{}) })
	for m := range b.menus {
		m.release()
	}
	b.base.release()
	delete(menuTargets, b.target.ID)
	b.target.Release()
	b.queued = nil
}
func cloneMenuItems(items []common.MenuItem) []common.MenuItem {
	if items == nil {
		return nil
	}
	r := make([]common.MenuItem, len(items))
	copy(r, items)
	for i := range r {
		r[i].Children = cloneMenuItems(r[i].Children)
	}
	return r
}
func sameMenuStructure(a, b []common.MenuItem) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i].ID != b[i].ID || a[i].Separator != b[i].Separator || a[i].Submenu != b[i].Submenu || !sameMenuStructure(a[i].Children, b[i].Children) {
			return false
		}
	}
	return true
}
func (m *nativeMenu) setItems(items []common.MenuItem) error {
	seen := map[uint64]bool{}
	var validate func([]common.MenuItem) error
	validate = func(items []common.MenuItem) error {
		for _, item := range items {
			if item.ID == 0 || seen[item.ID] {
				return errors.New("menu: duplicate or zero item ID")
			}
			if !item.Submenu && len(item.Children) > 0 || item.Separator && (item.Submenu || len(item.Children) > 0) {
				return errors.New("menu: invalid child or separator description")
			}
			seen[item.ID] = true
			if err := validate(item.Children); err != nil {
				return err
			}
		}
		return nil
	}
	if err := validate(items); err != nil {
		return err
	}
	if sameMenuStructure(m.items, items) && m.root.ID != 0 {
		if m.pending != nil {
			m.pending.release()
			m.pending = nil
		}
		m.items = cloneMenuItems(items)
		AutoReleasePool(func() { m.update(items) })
		return nil
	}
	snapshot, err := m.bar.build(items)
	if err != nil {
		return err
	}
	if m.pending != nil {
		m.pending.release()
	}
	m.pending = &snapshot
	if !m.bar.busy() || m.bar.installed != m {
		m.commit()
	}
	return nil
}
func (m *nativeMenu) update(items []common.MenuItem) {
	for _, item := range items {
		i := m.objects[item.ID]
		if !item.Separator {
			setNativeMenuItem(i, item)
		}
		m.update(item.Children)
	}
}
func (m *nativeMenu) commit() {
	if m.pending == nil {
		return
	}
	old := m.menuSnapshot
	m.menuSnapshot = *m.pending
	m.pending = nil
	if !m.bar.busy() {
		m.bar.install()
	}
	old.release()
}
func (m *nativeMenu) destroy() {
	if m.destroyed {
		return
	}
	m.destroyed = true
	if m.owner.menu == m {
		m.owner.menu = nil
	}
	if m.bar.installed == m {
		m.bar.epoch++
		m.bar.cancelTracking()
	}
	if m.bar.busy() {
		m.bar.schedule()
		return
	}
	m.bar.install()
	m.release()
}
func (m *nativeMenu) release() {
	m.menuSnapshot.release()
	if m.pending != nil {
		m.pending.release()
		m.pending = nil
	}
	delete(m.bar.menus, m)
}
func setNativeMenuItem(i NSMenuItem, item common.MenuItem) {
	i.SetTitle(menuString(item.Title))
	i.SetEnabled(item.Enabled)
	state := NSInteger(0)
	if item.Checked {
		state = 1
	}
	i.SetState(state)
	key, flags := nativeMenuShortcut(item.Shortcut)
	i.SetKeyEquivalent(menuString(key))
	i.SetKeyEquivalentModifierMask(flags)
}

func nativeMenuShortcut(shortcut common.MenuShortcut) (string, NSEventModifierFlags) {
	const supported = common.ModifierShift | common.ModifierControl | common.ModifierOption | common.ModifierCommand
	if shortcut.Modifiers & ^supported != 0 {
		return "", 0
	}
	var key string
	switch {
	case shortcut.Key >= common.KeyA && shortcut.Key <= common.KeyZ:
		key = string(rune('a' + shortcut.Key - common.KeyA))
	case shortcut.Key >= common.Key0 && shortcut.Key <= common.Key9:
		key = string(rune('0' + shortcut.Key - common.Key0))
	case shortcut.Key >= common.KeyF1 && shortcut.Key <= common.KeyF24:
		key = string(NSF1FunctionKey + rune(shortcut.Key-common.KeyF1))
	default:
		key = map[common.Key]string{
			common.KeyEscape: "\x1b", common.KeyEnter: "\r", common.KeyTab: "\t", common.KeyBackspace: string(NSBackspaceCharacter), common.KeyDelete: string(NSDeleteCharacter),
			common.KeySpace: " ", common.KeyMinus: "-", common.KeyEqual: "=", common.KeyBracketLeft: "[", common.KeyBracketRight: "]", common.KeyBackslash: "\\", common.KeySemicolon: ";", common.KeyQuote: "'", common.KeyComma: ",", common.KeyPeriod: ".", common.KeySlash: "/", common.KeyBackquote: "`",
			common.KeyArrowUp: string(NSUpArrowFunctionKey), common.KeyArrowDown: string(NSDownArrowFunctionKey), common.KeyArrowLeft: string(NSLeftArrowFunctionKey), common.KeyArrowRight: string(NSRightArrowFunctionKey),
			common.KeyInsert: string(NSInsertFunctionKey), common.KeyHome: string(NSHomeFunctionKey), common.KeyEnd: string(NSEndFunctionKey), common.KeyPageUp: string(NSPageUpFunctionKey), common.KeyPageDown: string(NSPageDownFunctionKey),
		}[shortcut.Key]
	}
	if key == "" {
		return "", 0
	}
	flags := NSEventModifierFlags(0)
	if shortcut.Modifiers&common.ModifierShift != 0 {
		flags |= NSEventModifierFlagShift
	}
	if shortcut.Modifiers&common.ModifierControl != 0 {
		flags |= NSEventModifierFlagControl
	}
	if shortcut.Modifiers&common.ModifierOption != 0 {
		flags |= NSEventModifierFlagOption
	}
	if shortcut.Modifiers&common.ModifierCommand != 0 {
		flags |= NSEventModifierFlagCommand
	}
	return key, flags
}

func (b *menuBar) busy() bool { return len(b.tracking) > 0 || b.handlingKey }

func (m *nativeMenu) shortcutInMenu(menu NSMenu, key common.Key, modifiers common.Modifiers) bool {
	parent, ok := m.parents[menu.ID]
	if !ok {
		return false
	} // the standard application menu keeps AppKit handling
	var find func([]common.MenuItem, uint64, bool) ([]common.MenuItem, bool)
	find = func(items []common.MenuItem, id uint64, enabled bool) ([]common.MenuItem, bool) {
		for _, item := range items {
			if item.ID == id {
				return item.Children, enabled && item.Enabled
			}
			if child, active := find(item.Children, id, enabled && item.Enabled); child != nil {
				return child, active
			}
		}
		return nil, false
	}
	items, enabled := m.items, true
	if parent != 0 {
		items, enabled = find(m.items, parent, true)
	}
	if !enabled {
		return false
	}
	for _, item := range items {
		if item.Enabled && !item.Submenu && !item.Separator && item.Shortcut.Key == key && item.Shortcut.Modifiers == modifiers {
			equivalent, _ := nativeMenuShortcut(item.Shortcut)
			if equivalent != "" {
				return true
			}
		}
	}
	return false
}

func (b *menuBar) handleKeyEquivalent(menu NSMenu, event NSEvent, target *objc.ID, action *objc.SEL) bool {
	m := b.installed
	if b.destroyed || b.handlingKey || m == nil || m.destroyed || m.owner != b.current || m.owner.closed || event.Type() != NSEventTypeKeyDown {
		return false
	}
	key, _ := keyFromMacEvent(event)
	if !m.shortcutInMenu(menu, key, modifiersFromFlags(event.ModifierFlags())) {
		return false
	}
	b.deliverKeyEquivalent(m, event)
	*target, *action = 0, 0
	return true
}

func (b *menuBar) deliverKeyEquivalent(m *nativeMenu, event NSEvent) {
	if b.handlingKey {
		return
	}
	// Native dispatch may synchronously replace menus or destroy the owner.
	// Keep receivers alive and commit structural changes after this callback.
	b.target.Retain()
	defer b.target.Release()
	root := m.root
	root.Retain()
	defer root.Release()
	b.handlingKey = true
	defer func() { b.handlingKey = false; b.schedule() }()
	b.cancelTracking()
	keyDown(m.owner.view, event)
}

func (b *menuBar) build(items []common.MenuItem) (snapshot menuSnapshot, err error) {
	var root NSMenu
	var delegates []NSMenu
	var targets []NSMenuItem
	var actions map[objc.ID]uint64
	var objects map[uint64]NSMenuItem
	parents := map[objc.ID]uint64{}
	actions = map[objc.ID]uint64{}
	objects = map[uint64]NSMenuItem{}
	AutoReleasePool(func() {
		newMenu := func(title string) NSMenu {
			menu := NSMenuClassId.Alloc().InitWithTitle(menuString(title))
			if menu.ID != 0 {
				menu.SetAutoenablesItems(false)
				menu.SetDelegate(b.target)
				delegates = append(delegates, menu)
			}
			return menu
		}
		root = newMenu("")
		if root.ID == 0 {
			err = errors.New("menu: create root failed")
			return
		}

		appItem := NSMenuItemClassId.Alloc().InitWithTitle(menuString(b.appName), 0, menuString(""))
		if appItem.ID == 0 {
			err = errors.New("menu: create application item failed")
			return
		}
		app := newMenu(b.appName)
		if app.ID == 0 {
			appItem.Release()
			err = errors.New("menu: create application menu failed")
			return
		}
		app.SetAutoenablesItems(true)
		appItem.SetEnabled(true)
		appItem.SetSubmenu(app)
		root.AddItem(appItem)
		appItem.Release()
		app.Release()

		for _, entry := range []struct {
			title, key string
			selector   objc.SEL
			flags      NSEventModifierFlags
		}{
			{"Hide " + b.appName, "h", NSApplicationSel.Hide, NSEventModifierFlagCommand},
			{"Hide Others", "h", NSApplicationSel.HideOtherApplications, NSEventModifierFlagCommand | NSEventModifierFlagOption},
			{"Show All", "", NSApplicationSel.UnhideAllApplications, 0},
			{"", "", 0, 0},
			{"Quit " + b.appName, "q", NSApplicationSel.Terminate, NSEventModifierFlagCommand},
		} {
			if entry.title == "" {
				app.AddItem(NSMenuItemClassId.SeparatorItem())
				continue
			}
			i := NSMenuItemClassId.Alloc().InitWithTitle(menuString(entry.title), entry.selector, menuString(entry.key))
			if i.ID == 0 {
				err = errors.New("menu: create application action failed")
				return
			}
			i.SetEnabled(true)
			i.SetTarget(NSApp.NSObject)
			i.SetKeyEquivalentModifierMask(entry.flags)
			app.AddItem(i)
			i.Release()
		}
		var appendItems func(NSMenu, []common.MenuItem, uint64)
		appendItems = func(parent NSMenu, items []common.MenuItem, parentID uint64) {
			parents[parent.ID] = parentID
			for _, item := range items {
				if err != nil {
					return
				}
				if item.Separator {
					i := NSMenuItemClassId.SeparatorItem()
					parent.AddItem(i)
					objects[item.ID] = i
					continue
				}
				i := NSMenuItemClassId.Alloc().InitWithTitle(menuString(item.Title), menuActionSel, menuString(""))
				if i.ID == 0 {
					err = fmt.Errorf("menu: create item %d failed", item.ID)
					return
				}
				i.SetTarget(b.target)
				targets = append(targets, i)
				setNativeMenuItem(i, item)
				parent.AddItem(i)
				objects[item.ID] = i
				i.Release()
				if item.Submenu {
					child := newMenu(item.Title)
					if child.ID == 0 {
						err = errors.New("menu: create child failed")
						return
					}
					i.SetSubmenu(child)
					appendItems(child, item.Children, item.ID)
					child.Release()
				} else {
					actions[i.ID] = item.ID
				}
			}
		}
		appendItems(root, items, 0)
	})
	snapshot = menuSnapshot{root: root, delegates: delegates, actions: actions, objects: objects, items: cloneMenuItems(items), targets: targets, parents: parents}
	if err != nil {
		snapshot.release()
	}
	return
}
