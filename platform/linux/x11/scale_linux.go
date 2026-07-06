package x11

import (
	"os"
	"strconv"
	"sync"
)

// X11 has no per-window scale factor. We derive a single display-global scale
// and cache it. Dynamic changes (XSETTINGS/Xft.dpi updates) are not yet
// observed; that would require g_signal_connect bindings and driving the GLib
// main loop. TODO(hidpi): dynamic scale change -> synthesize SizeEvent.
var scaleCache struct {
	once  sync.Once
	value float32
}

// currentScale returns the display-global logical->physical scale factor.
func currentScale() float32 {
	scaleCache.once.Do(func() {
		scaleCache.value = detectScale()
	})
	return scaleCache.value
}

// physical converts a logical size to a physical pixel count, clamped to >= 1.
func physical(logical, scale float32) int {
	if p := int(logical * scale); p >= 1 {
		return p
	}
	return 1
}

func detectScale() float32 {
	// Explicit override, honoring the common GDK_SCALE convention.
	if v := os.Getenv("GDK_SCALE"); v != "" {
		if f, err := strconv.ParseFloat(v, 32); err == nil && f > 0 {
			return float32(f)
		}
	}

	// GNOME/mutter communicates the effective scale to non-scaling X11 apps via
	// Xft.dpi, surfaced by GTK as the gtk-xft-dpi setting (DPI * 1024).
	if settings, err := gtkSettings(); err == nil {
		if dpi, err := settings.IntProperty("gtk-xft-dpi"); err == nil && dpi > 0 {
			if scale := float32(dpi) / 1024 / 96; scale > 0 {
				return scale
			}
		}
	}

	// 96dpi = 1x is the X11 baseline fact, not an invented default.
	return 1
}
