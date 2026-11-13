package x11

import (
	"os"
	"strconv"

	"github.com/golang-gui/goui/platform/linux/libs/xcursor"
	"github.com/golang-gui/goui/platform/linux/libs/xlib"
)

// cursorTheme consumes display-owned XSETTINGS and X Resources. Its cursor
// caches do not own or outlive the shared platform settings subscription.
type cursorTheme struct {
	*xSettings
	resources   xlib.Atom
	available   bool
	defaultSize int
	config      cursorConfig
	cursors     map[*cursor]struct{}
}

type cursorConfig struct {
	name string
	size int
}

func newCursorTheme(settings *xSettings) *cursorTheme {
	t := &cursorTheme{xSettings: settings, resources: settings.display.InternAtom("RESOURCE_MANAGER", false),
		available: xcursor.Available(), cursors: make(map[*cursor]struct{})}
	if t.available {
		t.defaultSize = xcursor.GetDefaultSize(t.display)
		t.refresh()
	}
	return t
}

func (t *cursorTheme) handleEvent(event *xlib.Event) {
	changed := t.xSettings.handleEvent(event)
	if !t.available {
		return
	}
	if event.Type == xlib.PropertyNotify {
		e := event.PropertyEvent()
		changed = changed || e.Window == t.root && e.Atom == t.resources
	}
	if changed {
		t.apply()
	}
}

func (t *cursorTheme) refresh() {
	t.xSettings.refresh()
	t.apply()
}

func (t *cursorTheme) apply() {
	settings := cursorSettings(t.values)
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

// Keep the existing cursor projection small; the shared parser validates every
// XSETTINGS entry before any consumer can observe its values.
func cursorSettings(values map[string]xSetting) cursorConfig {
	var c cursorConfig
	if v, ok := values["Gtk/CursorThemeName"]; ok && v.kind == 1 {
		c.name = v.text
	}
	if v, ok := values["Gtk/CursorThemeSize"]; ok && v.kind == 0 {
		c.size = int(v.integer)
	}
	return c
}

func parseCursorSettings(data []byte) (cursorConfig, bool) {
	values, err := parseXSettings(data)
	return cursorSettings(values), err == nil
}

func cursorProperty(display xlib.Display, window xlib.Window, property, expected xlib.Atom) []byte {
	data, _ := settingsProperty(display, window, property, expected)
	return data
}
