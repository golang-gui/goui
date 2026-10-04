package style

import (
	"image/color"
	"testing"

	"github.com/golang-gui/goui/gui"
	basestyle "github.com/golang-gui/goui/style"
)

func TestRulesComposeWithoutRegistering(t *testing.T) {
	for _, rule := range gui.DefaultStyleRules() {
		if len(rule.Sel.Name) >= 4 && rule.Sel.Name[:4] == "tab-" {
			t.Fatal("GUI fallback still owns tab rules")
		}
	}
	rules := append(gui.DefaultStyleRules(), Rules()...)
	sheet := basestyle.Sheet(rules...)
	for _, name := range []string{"tab-item-text", "tab-item-text-selected", "tab-close-button-text", "tab-scroll-button-text"} {
		s := sheet.Resolve(basestyle.Sel{Name: name})
		foreground, ok := s.ForegroundColor()
		if !ok || foreground != color.Black {
			t.Fatalf("missing readable foreground for %s", name)
		}
		fontSize, ok := s.FontSize()
		if !ok || fontSize <= 0 {
			t.Fatalf("missing font size for %s", name)
		}
		family, ok := s.FontFamily()
		defaultFamily, _ := gui.DefaultStyleSheet().Resolve(basestyle.Sel{Name: "label"}).FontFamily()
		if !ok || family != defaultFamily {
			t.Fatalf("font default changed for %s", name)
		}
	}
	selected := sheet.Resolve(basestyle.Sel{Name: "tab-item", Part: "selected"})
	bg, ok := selected.BackgroundColor()
	if !ok || bg != (color.RGBA{R: 210, G: 210, B: 210, A: 255}) {
		t.Fatal("missing selected appearance")
	}
	rules = append(rules, basestyle.Name("tab-item").Part("selected").BackgroundColor(color.White))
	bg, _ = basestyle.Sheet(rules...).Resolve(basestyle.Sel{Name: "tab-item", Part: "selected"}).BackgroundColor()
	if bg != color.White {
		t.Fatal("application override lost")
	}
	if _, ok := gui.DefaultStyleSheet().Resolve(basestyle.Sel{Name: "tab-item", Part: "selected"}).BackgroundColor(); ok {
		t.Fatal("explicit composition changed GUI fallback globally")
	}
	first, second := Rules(), Rules()
	first[0] = basestyle.Name("mutated")
	if second[0].Sel.Name != "tab-bar" {
		t.Fatal("rules share mutable backing storage")
	}
}

func TestTreeFallbackStatesAndIndependentText(t *testing.T) {
	sheet := basestyle.Sheet(append(gui.DefaultStyleRules(), Rules()...)...)
	var previous color.Color
	for _, state := range []basestyle.State{basestyle.Normal, basestyle.Hovered, basestyle.Pressed} {
		bg, ok := sheet.Resolve(basestyle.Sel{Name: "tree-item", Part: "selected", State: state}).BackgroundColor()
		if !ok || bg == previous {
			t.Fatalf("missing selected state %v", state)
		}
		previous = bg
	}
	for _, name := range []string{"tree-item-text", "tree-item-icon", "tree-expander"} {
		fg, ok := sheet.Resolve(basestyle.Sel{Name: name}).ForegroundColor()
		if !ok || fg != color.Black {
			t.Fatalf("%s relies on inherited foreground", name)
		}
	}
}
