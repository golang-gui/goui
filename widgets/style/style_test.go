package style

import (
	"image/color"
	"testing"

	"github.com/golang-gui/goui/gui"
	basestyle "github.com/golang-gui/goui/style"
)

func TestRulesComposeWithoutRegistering(t *testing.T) {
	for _, rule := range gui.DefaultStyleRules() {
		if rule.Sel.Name == "progress-bar" {
			t.Fatal("GUI fallback still owns progress rules")
		}
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

func TestProgressFallbackIsExplicit(t *testing.T) {
	sheet := basestyle.Sheet(append(gui.DefaultStyleRules(), Rules()...)...)
	s := sheet.Resolve(basestyle.Sel{Name: "progress-bar"})
	bg, _ := s.BackgroundColor()
	fg, _ := s.ForegroundColor()
	radius, _ := s.Radius()
	if bg != (color.RGBA{R: 225, G: 225, B: 225, A: 255}) ||
		fg != (color.RGBA{R: 70, G: 130, B: 220, A: 255}) || radius != 2 {
		t.Fatal("missing progress track / foreground / radius")
	}
	if _, ok := gui.DefaultStyleSheet().Resolve(basestyle.Sel{Name: "progress-bar"}).ForegroundColor(); ok {
		t.Fatal("widget rules changed GUI fallback globally")
	}
}

func TestCheckButtonFallbackParts(t *testing.T) {
	sheet := basestyle.Sheet(append(gui.DefaultStyleRules(), Rules()...)...)
	for _, part := range []string{"indicator", "indicator-checked", "indicator-mixed", "radio", "radio-checked", "button", "button-checked", "button-mixed"} {
		for _, state := range []basestyle.State{basestyle.Normal, basestyle.Hovered, basestyle.Pressed, basestyle.Disabled} {
			s := sheet.Resolve(basestyle.Sel{Name: "check-button", Part: part, State: state})
			if bg, ok := s.BackgroundColor(); !ok || bg == nil {
				t.Fatalf("missing %s background", part)
			} else if part == "button-checked" || part == "button-mixed" {
				r, g, b, _ := bg.RGBA()
				if r != g || g != b {
					t.Fatalf("%s state=%v should have a neutral fill, got %v", part, state, bg)
				}
			}
			wantWidth := float32(1)
			if part == "radio-checked" {
				wantWidth = 2
			}
			if part == "button" || part == "button-checked" || part == "button-mixed" {
				// Bare GUI Button is borderless. The explicit widget fallback
				// follows that policy instead of forcing a different appearance.
				wantWidth = 0
			}
			if width, ok := s.BorderWidth(); !ok || width != wantWidth {
				t.Fatalf("%s state=%v border=%v; want %v", part, state, width, wantWidth)
			}
		}
	}
	if size, _ := sheet.Resolve(basestyle.Sel{Name: "check-button-text"}).FontSize(); size <= 0 {
		t.Fatal("label needs independent font")
	}
	if _, ok := gui.DefaultStyleSheet().Resolve(basestyle.Sel{Name: "check-button", Part: "indicator-checked"}).ForegroundColor(); ok {
		t.Fatal("fallback registered itself in GUI")
	}
}

func TestCheckButtonFallbackMatchesButtons(t *testing.T) {
	sheet := basestyle.Sheet(append(gui.DefaultStyleRules(), Rules()...)...)
	for _, state := range []basestyle.State{basestyle.Normal, basestyle.Hovered, basestyle.Pressed, basestyle.Disabled} {
		ordinary := sheet.Resolve(basestyle.Sel{Name: "button", State: state})
		toggle := sheet.Resolve(basestyle.Sel{Name: "check-button", Part: "button", State: state})
		wantBG, _ := ordinary.BackgroundColor()
		gotBG, _ := toggle.BackgroundColor()
		wantRadius, _ := ordinary.Radius()
		gotRadius, _ := toggle.Radius()
		wantEdge, _ := ordinary.BorderColor()
		gotEdge, _ := toggle.BorderColor()
		if gotBG != wantBG || gotRadius != wantRadius || gotEdge != wantEdge {
			t.Fatalf("unchecked state=%v does not match bare Button", state)
		}
		for _, selection := range []string{"", "-checked", "-mixed"} {
			s := sheet.Resolve(basestyle.Sel{Name: "check-button", Part: "indicator" + selection, State: state})
			if radius, _ := s.Radius(); radius != 4 {
				t.Fatal("checkbox radius is not 4 DIP")
			}
			if state != basestyle.Disabled {
				edge, _ := s.BorderColor()
				wantEdge, _ := sheet.Resolve(basestyle.Sel{Name: "check-button", Part: "indicator" + selection}).BorderColor()
				if selection != "" {
					wantEdge, _ = s.BackgroundColor()
					if fg, _ := s.ForegroundColor(); fg != color.White {
						t.Fatal("selected checkbox must use white ink")
					}
				}
				if edge != wantEdge {
					t.Fatal("hover/press changes indicator outline")
				}
			}
		}
	}
	if width, _ := sheet.Resolve(basestyle.Sel{Name: "check-button", Part: "focus", State: basestyle.FocusVisible}).BorderWidth(); width != 2 {
		t.Fatal("missing independent 2 DIP keyboard focus")
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

func TestTableFallbackDecorationParts(t *testing.T) {
	sheet := basestyle.Sheet(append(gui.DefaultStyleRules(), Rules()...)...)
	if bg, ok := sheet.Resolve(basestyle.Sel{Name: "table-view"}).BackgroundColor(); !ok || bg != color.White {
		t.Fatal("table fallback has no background")
	}
	for _, part := range []string{"outer-horizontal", "outer-vertical"} {
		if width, _ := sheet.Resolve(basestyle.Sel{Name: "table-view", Part: part}).BorderWidth(); width != 1 {
			t.Fatal("outer frame default missing")
		}
	}
	for part, want := range map[string]float32{"horizontal": 1, "vertical": 0} {
		s := sheet.Resolve(basestyle.Sel{Name: "table-grid", Part: part})
		width, _ := s.BorderWidth()
		if ink, ok := s.BorderColor(); !ok || ink == nil || width != want {
			t.Fatalf("missing grid style %s", part)
		}
	}
	if width, _ := sheet.Resolve(basestyle.Sel{Name: "table-header", Part: "separator"}).BorderWidth(); width != 1 {
		t.Fatal("header separator is not style controlled")
	}
}
