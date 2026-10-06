package style

import (
	"testing"

	basestyle "github.com/golang-gui/goui/style"
)

func TestDropDownFallbackRules(t *testing.T) {
	sheet := basestyle.Sheet(Rules()...)
	for _, name := range []string{"drop-down", "drop-down-popup"} {
		s := sheet.Resolve(basestyle.Sel{Name: name})
		if width, _ := s.BorderWidth(); width != 1 {
			t.Fatal("missing surface outline")
		}
		if radius, _ := s.Radius(); radius <= 0 {
			t.Fatal("missing surface radius")
		}
	}
	if width, _ := sheet.Resolve(basestyle.Sel{Name: "drop-down", Part: "focus", State: basestyle.FocusVisible}).BorderWidth(); width != 2 {
		t.Fatal("focus-visible width")
	}
	for _, name := range []string{"drop-down-text", "drop-down-item-text", "drop-down-item-text-disabled", "drop-down-placeholder"} {
		s := sheet.Resolve(basestyle.Sel{Name: name})
		if _, ok := s.FontSize(); !ok {
			t.Fatal("child implicitly inherits typography")
		}
		if fg, ok := s.ForegroundColor(); !ok || fg == nil {
			t.Fatal("missing child foreground")
		}
	}
}
