package x11

import (
	"testing"

	"github.com/golang-gui/goui/platform/linux/libs/xlib"
)

func TestXDNDLocalOnlyOfferStaysInProcess(t *testing.T) {
	previous := windowMap
	defer func() { windowMap = previous }()
	windowMap = map[xlib.Window]*Window{42: {}}
	p := new(Platform)
	p.atoms.GOUI_LOCAL = 7
	source := &xdndSource{service: &dragService{platform: p}}
	for _, test := range []struct {
		types  []xlib.Atom
		target xlib.Window
		want   bool
	}{
		{[]xlib.Atom{7}, 42, true},
		{[]xlib.Atom{7}, 43, false},
		{[]xlib.Atom{7, 8}, 43, true},
		{[]xlib.Atom{8}, 43, true},
	} {
		source.types = test.types
		if got := source.canOfferTo(test.target); got != test.want {
			t.Fatalf("types=%v target=%v: canOffer=%v, want %v", test.types, test.target, got, test.want)
		}
	}
}
