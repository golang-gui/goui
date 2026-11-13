package x11

import (
	"math"
	"os"
	"strconv"
	"sync"

	"github.com/golang-gui/goui/platform/common"
	"github.com/golang-gui/goui/platform/linux/libs/xlib"
)

// X11 has no per-window scale factor. We derive a single display-global scale
// and cache it. Dynamic changes (XSETTINGS/Xft.dpi updates) are not yet
// applied to window geometry. Settings and cursors share an X11 subscription;
// TODO(hidpi): dynamic scale change -> synthesize SizeEvent.
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
	// Explicit override via environment variable.
	if scale := common.GetPreferScale(); scale > 0 {
		return scale
	}
	// Explicit override, honoring the common GDK_SCALE convention.
	if v := os.Getenv("GDK_SCALE"); v != "" {
		if f, err := strconv.ParseFloat(v, 32); err == nil && f > 0 {
			return float32(f)
		}
	}

	// Read native XSETTINGS directly. Window scale remains a startup snapshot;
	// dynamic geometry scaling is a separate contract.
	if platform != nil && platform.display != 0 {
		if dpi, err := platform.desktopSettings().integer("Xft/DPI"); err == nil && dpi > 0 {
			return float32(dpi) / 1024 / 96
		}
		resources := platform.display.InternAtom("RESOURCE_MANAGER", false)
		data := cursorProperty(platform.display, platform.display.DefaultRootWindow(), resources, xlib.AtomString)
		if db := xlib.GetStringDatabase(string(data)); db != 0 {
			defer db.Destroy()
			if value, ok := db.GetResource("Xft.dpi", "Xft.Dpi"); ok {
				if dpi, err := strconv.ParseFloat(value, 32); err == nil && dpi > 0 && !math.IsInf(dpi, 0) {
					return float32(dpi) / 96
				}
			}
		}
	}

	// 96dpi = 1x is the X11 baseline fact, not an invented default.
	return 1
}
