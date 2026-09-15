package modern

import (
	"image/color"
	"math"
	"testing"

	"github.com/golang-gui/goui/style"
)

func resolved(sheet style.StyleSheet, name string, state style.State) style.Style {
	return sheet.Resolve(style.Sel{Name: name, State: state})
}

func rgba(c color.Color) color.RGBA { return color.RGBAModel.Convert(c).(color.RGBA) }

func TestMenuInteractionColorsIgnoreAccent(t *testing.T) {
	for _, dark := range []bool{false, true} {
		want := []color.RGBA{rgb(0xE6E6E6), rgb(0xD1D1D1)}
		if dark {
			want = []color.RGBA{rgb(0x3E3E42), rgb(0x4F4F53)}
		}
		for _, accent := range []color.Color{nil, color.Black, color.White, rgb(0xFF0000), rgb(0x0080FF)} {
			options := Options{Dark: dark, AccentColor: accent}
			for i, state := range []style.State{style.Hovered, style.Pressed} {
				s := resolved(Sheet(options), "menu-item", state)
				bg, _ := s.BackgroundColor()
				if rgba(bg) != want[i] {
					t.Fatalf("dark=%v accent=%v state=%v: %v, want %v", dark, accent, state, bg, want[i])
				}
			}
			custom := style.Sheet(append(Rules(options), style.Name("menu-item").State(style.Hovered).BackgroundColor(rgb(0x123456)))...)
			bg, _ := resolved(custom, "menu-item", style.Hovered).BackgroundColor()
			if rgba(bg) != rgb(0x123456) {
				t.Fatal("application cannot override menu state background")
			}
		}
	}
}

func TestContrastReferenceValues(t *testing.T) {
	if contrast(rgb(0), rgb(0xFFFFFF)) != 21 || contrast(rgb(0x808080), rgb(0x808080)) != 1 {
		t.Fatal("contrast calculation does not match reference endpoints")
	}
	if math.Abs(luminance(rgb(0x808080))-.2158605) > .0000001 {
		t.Fatal("sRGB was not linearized before calculating luminance")
	}
	for state, want := range map[style.State]color.RGBA{
		style.Hovered: rgb(0xF6F6F6), style.Pressed: rgb(0xEDEDED),
	} {
		bg, _ := resolved(Sheet(Options{}), "button", state).BackgroundColor()
		if rgba(bg) != want {
			t.Fatalf("button interaction tint=%v, want %v", bg, want)
		}
	}
}

func TestPaletteAndCoverage(t *testing.T) {
	for _, dark := range []bool{false, true} {
		sheet := Sheet(Options{Dark: dark, FontFamily: "Test Family", FontSize: 17})
		p := colorsFor(dark)
		for name, want := range map[string]color.RGBA{
			"window": p.window, "header-bar": p.window, "button": p.button,
			"text-input": p.surface, "popover": p.surface, "menu": p.surface,
			"menu-separator": p.border,
		} {
			bg, ok := resolved(sheet, name, style.Normal).BackgroundColor()
			if !ok || rgba(bg) != want {
				t.Fatalf("dark=%v %s background=%v, want %v", dark, name, bg, want)
			}
		}
		for _, name := range []string{"widget", "label", Primary, MutedText, "button", "text-input", "scroll-view", "header-bar", "popover", "menu", "menu-item-text", "menu-item-text-disabled"} {
			s := resolved(sheet, name, style.Normal)
			family, _ := s.FontFamily()
			size, _ := s.FontSize()
			fg, ok := s.ForegroundColor()
			if family != "Test Family" || size != 17 || !ok || fg == nil {
				t.Fatalf("incomplete typography: %s", name)
			}
		}
		for name, want := range map[string]float32{"button": 6, Primary: 6, "text-input": 6, "menu-item": 4, "menu": 8, "popover": 8} {
			if got, _ := resolved(sheet, name, style.Normal).Radius(); got != want {
				t.Fatalf("%s radius=%v, want %v", name, got, want)
			}
		}
		for name, want := range map[string]color.RGBA{"menu-item-text": p.text, "menu-item-text-disabled": p.disabled} {
			fg, _ := resolved(sheet, name, style.Normal).ForegroundColor()
			if rgba(fg) != want {
				t.Fatalf("dark=%v %s foreground=%v, want %v", dark, name, fg, want)
			}
		}
		for _, part := range []string{"trough", "thumb"} {
			for _, state := range []style.State{style.Normal, style.Hovered, style.Pressed} {
				s := sheet.Resolve(style.Sel{Name: "scroll-view", Part: part, State: state})
				radius, _ := s.Radius()
				bg, ok := s.BackgroundColor()
				if radius != 4 || !ok || rgba(bg).A != 255 {
					t.Fatalf("scrollbar lost opaque capsule: %s state=%v", part, state)
				}
			}
		}
		shadow, ok := resolved(sheet, "menu", style.Normal).Shadow()
		if !ok || rgba(shadow.Color) != (color.RGBA{A: 46}) || shadow.BlurRadius != 16 || shadow.Offset.Y != 4 || shadow.SpreadRadius != 0 {
			t.Fatalf("unexpected menu shadow: %+v", shadow)
		}
		if shadow, ok := resolved(sheet, "popover", style.Normal).Shadow(); !ok || shadow != (style.Shadow{}) {
			t.Fatal("ordinary popover must explicitly have no shadow")
		}
		for _, rule := range Rules(Options{Dark: dark}) {
			if rule.Sel.Name == "window-control" || rule.Sel.Name == "window" && (rule.Sel.Part != "" || rule.Sel.State != style.Normal) {
				t.Fatal("modern exported decoration selectors")
			}
			if rule.Sel.Name == "window" {
				if !style.SameRules([]style.Rule{rule}, []style.Rule{style.Name("window").BackgroundColor(p.window)}) {
					t.Fatal("window received more than a background color")
				}
			}
		}
	}
}

func TestOptionsOwnershipAndOverrides(t *testing.T) {
	defaults := Rules(Options{})
	for _, size := range []float32{0, -4, float32(math.NaN()), float32(math.Inf(1)), float32(math.Inf(-1))} {
		if !style.SameRules(defaults, Rules(Options{FontSize: size, AccentColor: color.Transparent})) {
			t.Fatal("invalid options did not normalize to defaults")
		}
	}
	accent := &color.NRGBA{R: 255, G: 128, B: 0, A: 128}
	options := Options{AccentColor: accent}
	rules := Rules(options)
	copyBefore := Rules(options)
	if !style.SameRules(rules, Rules(Options{AccentColor: color.RGBA{255, 128, 0, 255}})) {
		t.Fatal("partial accent was not made opaque from straight RGB")
	}
	accent.G = 20
	if !style.SameRules(rules, copyBefore) || style.SameRules(rules, Rules(options)) {
		t.Fatal("rules retained a mutable accent or ignored a changed accent")
	}
	rules[0] = style.Name("changed")
	if !style.SameRules(copyBefore, Rules(Options{AccentColor: color.RGBA{255, 128, 0, 255}})) {
		t.Fatal("Rules calls shared backing storage")
	}
	if style.SameRules(defaults, Rules(Options{Dark: true})) {
		t.Fatal("dark mode did not change rules")
	}
	custom := style.Sheet(append(Rules(Options{}), style.Name("menu").Radius(3).Shadow(style.Shadow{}))...)
	s := resolved(custom, "menu", style.Normal)
	if radius, _ := s.Radius(); radius != 3 {
		t.Fatal("appended application rule did not override")
	}
	if shadow, _ := s.Shadow(); shadow != (style.Shadow{}) {
		t.Fatal("appended rule could not disable shadow")
	}
}

func TestQuantizedContrastAcrossAccentCube(t *testing.T) {
	for _, dark := range []bool{false, true} {
		for _, r := range []uint8{0, 64, 128, 192, 255} {
			for _, g := range []uint8{0, 64, 128, 192, 255} {
				for _, b := range []uint8{0, 64, 128, 192, 255} {
					accent := color.RGBA{r, g, b, 255}
					sheet := Sheet(Options{Dark: dark, AccentColor: accent})
					p := colorsFor(dark)
					focus, _ := sheet.Resolve(style.Sel{Name: "button", Part: "focus", State: style.Focused}).BorderColor()
					for _, bg := range []color.RGBA{p.window, p.surface} {
						if contrast(p.text, bg) < 4.5 || contrast(rgba(focus), bg) < 3 || contrast(p.muted, bg) < 4.5 {
							t.Fatalf("insufficient body contrast: dark=%v accent=%v", dark, accent)
						}
					}
					for _, name := range []string{"button", Primary, "text-input", "menu-item"} {
						for _, state := range []style.State{style.Normal, style.Hovered, style.Pressed} {
							s := resolved(sheet, name, state)
							bg, _ := s.BackgroundColor()
							background := rgba(bg)
							if background.A == 0 { // menu text is over the menu surface
								background = p.surface
							}
							if contrast(p.text, background) < 4.5 {
								t.Fatalf("text contrast: dark=%v accent=%v %s/%v", dark, accent, name, state)
							}
							if name != "menu-item" && contrast(rgba(focus), background) < 3 {
								t.Fatalf("focus contrast: dark=%v accent=%v %s/%v", dark, accent, name, state)
							}
						}
					}
				}
			}
		}
	}
}
