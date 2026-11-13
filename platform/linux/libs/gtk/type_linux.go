package gtk

import "github.com/golang-gui/goui/platform/linux/libs/glib"

type Settings struct {
	glib.Object
}

type StyleContext struct {
	glib.Object
}

type RGBA struct {
	Red   float64
	Green float64
	Blue  float64
	Alpha float64
}

// CssProvider may be owned or borrowed as specified by its constructor.
type CssProvider struct{ glib.Object }

const StyleProviderPriorityApplication uint32 = 600
