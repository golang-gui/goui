package x11

import (
	"encoding/binary"
	"fmt"
	"os"
	"strconv"
	"unsafe"

	"github.com/golang-gui/goui/platform/linux/libs/xcursor"
	"github.com/golang-gui/goui/platform/linux/libs/xlib"
)

// cursorTheme owns display-local settings subscriptions, not GUI policy.
// XSETTINGS is read on GOUI's X connection; no GTK event loop is required.
type cursorTheme struct {
	display                                 xlib.Display
	root, owner                             xlib.Window
	selection, settings, manager, resources xlib.Atom
	rootMask, ownerMask                     int64
	available                               bool
	defaultSize                             int
	config                                  cursorConfig
	cursors                                 map[*cursor]struct{}
}

type cursorConfig struct {
	name string
	size int
}

func newCursorTheme(display xlib.Display) *cursorTheme {
	t := &cursorTheme{
		display: display, root: display.DefaultRootWindow(),
		available: xcursor.Available(), cursors: make(map[*cursor]struct{}),
	}
	if !t.available {
		return t
	}
	t.defaultSize = xcursor.GetDefaultSize(display)
	t.selection = display.InternAtom(fmt.Sprintf("_XSETTINGS_S%d", display.DefaultScreen()), false)
	t.settings = display.InternAtom("_XSETTINGS_SETTINGS", false)
	t.manager = display.InternAtom("MANAGER", false)
	t.resources = display.InternAtom("RESOURCE_MANAGER", false)
	t.rootMask = t.subscribe(t.root)
	t.refresh()
	return t
}

func (t *cursorTheme) subscribe(window xlib.Window) int64 {
	var attributes xlib.WindowAttributes
	t.display.GetWindowAttributes(window, &attributes)
	t.display.ChangeWindowAttributes(window, xlib.CwEventMask, &xlib.SetWindowAttributes{
		EventMask: uint64(attributes.YourEventMask) | xlib.EventMaskPropertyChange | xlib.EventMaskStructureNotify,
	})
	return attributes.YourEventMask
}

func (t *cursorTheme) restore(window xlib.Window, mask int64) {
	t.display.ChangeWindowAttributes(window, xlib.CwEventMask, &xlib.SetWindowAttributes{EventMask: uint64(mask)})
}

func (t *cursorTheme) handleEvent(event *xlib.Event) {
	if !t.available {
		return
	}
	switch event.Type {
	case xlib.PropertyNotify:
		e := event.PropertyEvent()
		if (e.Window == t.root && e.Atom == t.resources) ||
			(e.Window == t.owner && e.Atom == t.settings) {
			t.refresh()
		}
	case xlib.ClientMessage:
		e := event.ClientMessageEvent()
		if e.Window == t.root && e.MessageType == t.manager && e.Format == 32 && xlib.Atom(e.L[1]) == t.selection {
			t.refresh()
		}
	case xlib.DestroyNotify:
		// StructureNotify on the manager itself has event == window.
		if event.AnyEvent().Window == t.owner {
			t.owner = 0
			t.refresh()
		}
	}
}

func (t *cursorTheme) readSettings() []byte {
	// Keep selection lookup, subscription and property read atomic with
	// respect to manager destruction. The server grab covers only these X
	// requests, never parsing, cursor loading, disk I/O or user callbacks.
	t.display.GrabServer()
	owner := t.display.GetSelectionOwner(t.selection)
	if owner != t.owner {
		t.owner = owner
		if owner != 0 {
			t.ownerMask = t.subscribe(owner)
		}
	}
	var data []byte
	if owner != 0 {
		data = cursorProperty(t.display, owner, t.settings, t.settings)
	}
	t.display.UngrabServer()
	t.display.Flush()
	return data
}

func (t *cursorTheme) refresh() {
	settings, _ := parseCursorSettings(t.readSettings())
	config := cursorConfig{name: "default", size: t.defaultSize}
	data := cursorProperty(t.display, t.root, t.resources, xlib.AtomString)
	if db := xlib.GetStringDatabase(string(data)); db != 0 {
		if name, ok := db.GetResource("Xcursor.theme", "Xcursor.Theme"); ok && name != "" {
			config.name = name
		}
		if size, ok := db.GetResource("Xcursor.size", "Xcursor.Size"); ok {
			if n, err := strconv.Atoi(size); err == nil && n > 0 {
				config.size = n
			}
		}
		db.Destroy()
	}
	if settings.name != "" {
		config.name = settings.name
	}
	if settings.size > 0 {
		config.size = settings.size
	}
	// Explicit process overrides take precedence over desktop configuration.
	if name := os.Getenv("XCURSOR_THEME"); name != "" {
		config.name = name
	}
	if size, err := strconv.Atoi(os.Getenv("XCURSOR_SIZE")); err == nil && size > 0 {
		config.size = size
	}
	if config == t.config {
		return
	}
	t.config = config
	xcursor.SetTheme(t.display, config.name)
	xcursor.SetDefaultSize(t.display, config.size)
	for cursor := range t.cursors {
		cursor.reloadTheme()
	}
}

func (t *cursorTheme) destroy() {
	if !t.available {
		return
	}
	t.display.GrabServer()
	// Only touch a manager window while it is known to be alive.
	if t.owner != 0 && t.display.GetSelectionOwner(t.selection) == t.owner {
		t.restore(t.owner, t.ownerMask)
	}
	t.restore(t.root, t.rootMask)
	t.display.UngrabServer()
	t.display.Flush()
}

// Bound settings reads to 1 MiB and reject wrong formats or truncated data.
func cursorProperty(display xlib.Display, window xlib.Window, property, expected xlib.Atom) []byte {
	var actual xlib.Atom
	var format int32
	var count, remaining uint
	var data *byte
	status := display.GetWindowProperty(window, property, 0, 1<<18, false, expected,
		&actual, &format, &count, &remaining, &data)
	if data != nil {
		defer xlib.Free(data)
	}
	if status != 0 || actual != expected || format != 8 || remaining != 0 || count > 1<<20 || data == nil {
		return nil
	}
	return append([]byte(nil), unsafe.Slice(data, int(count))...)
}

// XSETTINGS uses its own byte order, four-byte padding and typed entries.
// Parse all entries so malformed trailing data cannot produce a partial update.
func parseCursorSettings(data []byte) (cursorConfig, bool) {
	if len(data) < 12 || data[0] > 1 {
		return cursorConfig{}, false
	}
	var order binary.ByteOrder = binary.LittleEndian
	if data[0] == 1 {
		order = binary.BigEndian
	}
	take := func(n uint32) []byte {
		if uint64(n) > uint64(len(data)) {
			return nil
		}
		value := data[:n]
		data = data[n:]
		return value
	}
	count := order.Uint32(data[8:12])
	data = data[12:]
	var config cursorConfig
	for i := uint32(0); i < count; i++ {
		header := take(4)
		if header == nil {
			return cursorConfig{}, false
		}
		n := uint32(order.Uint16(header[2:]))
		name := take(n)
		if name == nil || take((4-n%4)%4) == nil || take(4) == nil {
			return cursorConfig{}, false
		}
		switch header[0] {
		case 0: // INT
			value := take(4)
			if value == nil {
				return cursorConfig{}, false
			}
			if string(name) == "Gtk/CursorThemeSize" {
				config.size = int(int32(order.Uint32(value)))
			}
		case 1: // STRING
			length := take(4)
			if length == nil {
				return cursorConfig{}, false
			}
			n = order.Uint32(length)
			value := take(n)
			if value == nil || take((4-n%4)%4) == nil {
				return cursorConfig{}, false
			}
			if string(name) == "Gtk/CursorThemeName" {
				config.name = string(value)
			}
		case 2: // COLOR: four CARD16 values
			if take(8) == nil {
				return cursorConfig{}, false
			}
		default:
			return cursorConfig{}, false
		}
	}
	return config, true
}
