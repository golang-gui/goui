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

func TestButtonFocusAppearance(t *testing.T) {
	for _, dark := range []bool{false, true} {
		neutral := color.RGBA{R: 198, G: 198, B: 206, A: 255}
		if dark {
			neutral = color.RGBA{R: 85, G: 85, B: 95, A: 255}
		}
		for _, accent := range []color.Color{nil, color.Black, color.White, color.RGBA{R: 255, A: 255}} {
			rules := Rules(Options{Dark: dark, AccentColor: accent})
			sheet := style.Sheet(rules...)
			for _, name := range []string{"button", Primary} {
				focused := sheet.Resolve(style.Sel{Name: name, Part: "focus", State: style.Focused})
				focus, _ := focused.BorderColor()
				base := rgba(focus)
				// Independent integer reference: 75% focus + 25% neutral,
				// nearest byte rounding. All inputs/output are opaque sRGB.
				blend := func(a, b uint8) uint8 { return uint8((3*int(a) + int(b) + 2) / 4) }
				want := color.RGBA{R: blend(base.R, neutral.R), G: blend(base.G, neutral.G), B: blend(base.B, neutral.B), A: 255}
				if accent == nil {
					blue := color.RGBA{R: 101, G: 145, B: 213, A: 255}
					if dark {
						blue = color.RGBA{R: 91, G: 130, B: 190, A: 255}
					}
					if want != blue {
						t.Fatalf("default blue reference changed: %v want %v", want, blue)
					}
				}
				for _, state := range []style.State{style.Focused, style.FocusVisible} {
					s := sheet.Resolve(style.Sel{Name: name, Part: "focus", State: state})
					bg, ok := s.BackgroundColor()
					if !ok || rgba(bg).A != 0 {
						t.Fatalf("focus added a background: dark=%v accent=%v name=%s state=%v bg=%v", dark, accent, name, state, bg)
					}
					wantWidth := float32(0)
					if state == style.FocusVisible {
						wantWidth = 2
						bc, ok := s.BorderColor()
						if !ok || rgba(bc) != want || rgba(bc) == neutral {
							t.Fatalf("keyboard border: dark=%v accent=%v name=%s got=%v want=%v", dark, accent, name, bc, want)
						}
					}
					if width, ok := s.BorderWidth(); !ok || width != wantWidth {
						t.Fatalf("focus width: state=%v got=%g want=%g, set=%v", state, width, wantWidth, ok)
					}
					if radius, _ := s.Radius(); radius != 6 {
						t.Fatalf("focus did not preserve button radius: %g", radius)
					}
				}
				customRules := append(rules, style.Name(name).Part("focus").State(style.Focused).BorderWidth(3).Radius(9))
				custom := style.Sheet(customRules...)
				if width, _ := custom.Resolve(style.Sel{Name: name, Part: "focus", State: style.Focused}).BorderWidth(); width != 3 {
					t.Fatal("application ordinary Focused override was lost")
				}
				sel := style.Sel{Name: name, Part: "focus", State: style.FocusVisible}
				if width, _ := custom.Resolve(sel).BorderWidth(); width != 2 {
					t.Fatal("generic Focused replaced the more specific keyboard border")
				}
				if radius, _ := custom.Resolve(sel).Radius(); radius != 9 {
					t.Fatal("unset FocusVisible radius did not inherit Focused")
				}
				custom = style.Sheet(append(customRules, style.Name(name).Part("focus").State(style.FocusVisible).
					BorderColor(color.RGBA{R: 255, A: 255}).BorderWidth(1))...)
				bc, _ := custom.Resolve(sel).BorderColor()
				if width, _ := custom.Resolve(sel).BorderWidth(); width != 1 || rgba(bc) != (color.RGBA{R: 255, A: 255}) {
					t.Fatal("explicit keyboard border override was lost")
				}
				custom = style.Sheet(append(customRules, style.Name(name).Part("focus").State(style.FocusVisible).BorderWidth(0))...)
				if width, _ := custom.Resolve(sel).BorderWidth(); width != 0 {
					t.Fatal("application could not disable the keyboard border")
				}
			}
			if bg, ok := resolved(sheet, "label", style.Normal).BackgroundColor(); ok && rgba(bg).A != 0 {
				t.Fatal("button hint leaked into its independent label style")
			}
			if width, _ := resolved(sheet, "text-input", style.Focused).BorderWidth(); width != 2 {
				t.Fatal("ordinary text-input focus border changed")
			}
		}
	}
}

func TestTreeInteractionAndTypography(t *testing.T) {
	for _, dark := range []bool{false, true} {
		options := Options{Dark: dark, FontFamily: "Tree Font", FontSize: 17}
		sheet := Sheet(options)
		p := colorsFor(dark)
		// Selected is a part, not a new global State: all three interaction
		// states need explicit rules because parts do not inherit other parts.
		var previous color.RGBA
		for i, state := range []style.State{style.Normal, style.Hovered, style.Pressed} {
			selected := sheet.Resolve(style.Sel{Name: "tree-item", Part: "selected", State: state})
			bg, ok := selected.BackgroundColor()
			if !ok || rgba(bg).A != 255 || i > 0 && rgba(bg) == previous {
				t.Fatalf("missing selected interaction state: dark=%v state=%v", dark, state)
			}
			previous = rgba(bg)
			fg, _ := resolved(sheet, "tree-expander", state).ForegroundColor()
			want := p.muted
			if state != style.Normal {
				want = p.text
			}
			if rgba(fg) != want {
				t.Fatalf("disclosure foreground=%v, want %v", fg, want)
			}
		}
		text := resolved(sheet, "tree-item-text", style.Normal)
		family, _ := text.FontFamily()
		size, _ := text.FontSize()
		if family != "Tree Font" || size != 17 {
			t.Fatal("tree text ignored theme typography")
		}
		custom := style.Sheet(append(Rules(options), style.Name("tree-item").Part("selected").State(style.Hovered).BackgroundColor(color.Black))...)
		bg, _ := custom.Resolve(style.Sel{Name: "tree-item", Part: "selected", State: style.Hovered}).BackgroundColor()
		if bg != color.Black {
			t.Fatal("application state override lost")
		}
	}
}

func TestTableStylesNeutralSelectionAndIndependentText(t *testing.T) {
	for _, dark := range []bool{false, true} {
		want := []color.RGBA{rgb(0xD1D1D1), rgb(0xCACACA), rgb(0xC3C3C3)}
		if dark {
			want = []color.RGBA{rgb(0x4F4F53), rgb(0x555559), rgb(0x5C5C5F)}
		}
		for _, accent := range []color.Color{color.Black, rgb(0xFF0000)} {
			sheet := Sheet(Options{Dark: dark, AccentColor: accent, FontFamily: "Table Font", FontSize: 17})
			for i, state := range []style.State{style.Normal, style.Hovered, style.Pressed} {
				selected := sheet.Resolve(style.Sel{Name: "table-row", Part: "selected", State: state})
				bg, ok := selected.BackgroundColor()
				if !ok || rgba(bg) != want[i] {
					t.Fatalf("table selection dark=%v state=%v: %v", dark, state, bg)
				}
			}
			for _, name := range []string{"table-header-text", "table-cell-text"} {
				s := resolved(sheet, name, style.Normal)
				family, _ := s.FontFamily()
				size, _ := s.FontSize()
				fg, ok := s.ForegroundColor()
				if family != "Table Font" || size != 17 || !ok || rgba(fg).A != 255 {
					t.Fatal("table text depends on parent styling")
				}
			}
			for _, part := range []string{"separator", "sort"} {
				if _, ok := sheet.Resolve(style.Sel{Name: "table-header", Part: part}).ForegroundColor(); !ok {
					t.Fatal("missing header part")
				}
			}
			frame := resolved(sheet, "table-view", style.Normal)
			if bg, ok := frame.BackgroundColor(); !ok || rgba(bg).A != 255 {
				t.Fatal("table has no opaque theme background")
			}
			for _, part := range []string{"outer-horizontal", "outer-vertical"} {
				if width, ok := sheet.Resolve(style.Sel{Name: "table-view", Part: part}).BorderWidth(); !ok || width != 1 {
					t.Fatal("missing thin outer frame")
				}
			}
			for part, wantWidth := range map[string]float32{"horizontal": 1, "vertical": 0} {
				s := sheet.Resolve(style.Sel{Name: "table-grid", Part: part})
				width, ok := s.BorderWidth()
				ink, hasColor := s.BorderColor()
				if !ok || width != wantWidth || !hasColor || rgba(ink).A != 255 {
					t.Fatalf("grid default %s: width=%g color=%v", part, width, ink)
				}
			}
			bg, _ := sheet.Resolve(style.Sel{Name: "table-row", Part: "alternate"}).BackgroundColor()
			if rgba(bg).A != 0 {
				t.Fatal("zebra enabled without an application rule")
			}
			custom := style.Sheet(append(Rules(Options{Dark: dark}),
				style.Name("table-view").Part("outer-horizontal").BorderWidth(0),
				style.Name("table-grid").Part("vertical").BorderWidth(3),
				style.Name("table-row").Part("alternate").BackgroundColor(color.White),
			)...)
			if w, _ := custom.Resolve(style.Sel{Name: "table-view", Part: "outer-horizontal"}).BorderWidth(); w != 0 {
				t.Fatal("outer horizontal override ignored")
			}
			if w, _ := custom.Resolve(style.Sel{Name: "table-view", Part: "outer-vertical"}).BorderWidth(); w != 1 {
				t.Fatal("independent outer sides coupled")
			}
			if w, _ := custom.Resolve(style.Sel{Name: "table-grid", Part: "vertical"}).BorderWidth(); w != 3 {
				t.Fatal("grid application override ignored")
			}
		}
	}
}

func TestTreeSelectionColorsIgnoreAccent(t *testing.T) {
	for _, dark := range []bool{false, true} {
		// 固定调色板预期，不通过被测实现的 mix 函数推导。
		want := []color.RGBA{rgb(0xD1D1D1), rgb(0xCACACA), rgb(0xC3C3C3)}
		wantBorder := rgb(0x61616B)
		if dark {
			want = []color.RGBA{rgb(0x4F4F53), rgb(0x555559), rgb(0x5C5C5F)}
			wantBorder = rgb(0xB8B8C2)
		}
		for _, accent := range []color.Color{nil, color.Black, color.White, rgb(0xFF0000), rgb(0x0080FF)} {
			options := Options{Dark: dark, AccentColor: accent}
			sheet := Sheet(options)
			for i, state := range []style.State{style.Normal, style.Hovered, style.Pressed} {
				selected := sheet.Resolve(style.Sel{Name: "tree-item", Part: "selected", State: state})
				bg, ok := selected.BackgroundColor()
				if !ok || rgba(bg) != want[i] {
					t.Fatalf("dark=%v accent=%v state=%v: selected=%v, want %v", dark, accent, state, bg, want[i])
				}
				if radius, _ := selected.Radius(); radius != 4 {
					t.Fatal("selected row lost its rounded corners")
				}
				text, _ := resolved(sheet, "tree-item-text", style.Normal).ForegroundColor()
				if contrast(rgba(text), rgba(bg)) < 4.5 {
					t.Fatal("selected background reduced text contrast below 4.5:1")
				}
				current := sheet.Resolve(style.Sel{Name: "tree-item", Part: "current", State: state})
				border, _ := current.BorderColor()
				width, _ := current.BorderWidth()
				if rgba(border) != wantBorder || width != 1 {
					t.Fatalf("current outline=%v/%g, want neutral %v/1 DIP", border, width, wantBorder)
				}
			}
			custom := style.Sheet(append(Rules(options),
				style.Name("tree-item").Part("selected").BackgroundColor(rgb(0x123456)),
				style.Name("tree-item").Part("current").BorderColor(color.Black).BorderWidth(2),
			)...)
			bg, _ := custom.Resolve(style.Sel{Name: "tree-item", Part: "selected"}).BackgroundColor()
			current := custom.Resolve(style.Sel{Name: "tree-item", Part: "current"})
			border, _ := current.BorderColor()
			width, _ := current.BorderWidth()
			if rgba(bg) != rgb(0x123456) || border != color.Black || width != 2 {
				t.Fatal("application cannot override tree selection/current styles")
			}
		}
	}
}

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
