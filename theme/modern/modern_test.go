package modern

import (
	"image/color"
	"math"
	"reflect"
	"testing"

	"github.com/golang-gui/goui/gui"
	"github.com/golang-gui/goui/style"
	"github.com/golang-gui/goui/ui"
)

func resolved(sheet style.StyleSheet, name string, state style.State) style.Style {
	return sheet.Resolve(style.Sel{Name: name, State: state})
}

func rgba(c color.Color) color.RGBA { return color.RGBAModel.Convert(c).(color.RGBA) }

// Consumer-side interface: both existing Settings APIs must work unchanged.
var _ Settings = gui.Settings(nil)
var _ Settings = ui.Settings{}

type appearanceSettings struct {
	scheme gui.ColorScheme
	accent color.Color
	family string
	size   float32
	reads  [4]int
}

func (s *appearanceSettings) ColorScheme() gui.ColorScheme { s.reads[0]++; return s.scheme }
func (s *appearanceSettings) AccentColor() color.Color     { s.reads[1]++; return s.accent }
func (s *appearanceSettings) FontFamily() string           { s.reads[2]++; return s.family }
func (s *appearanceSettings) FontSize() float32            { s.reads[3]++; return s.size }

func TestFromSettings(t *testing.T) {
	for _, dark := range []bool{false, true} {
		s := &appearanceSettings{accent: rgb(0x0078D4), family: "Test UI Font", size: 17}
		window := rgb(0xF6F6F8)
		if dark {
			s.scheme, window = gui.ColorSchemeDark, rgb(0x202024)
		}
		sheet := FromSettings(s)
		if s.reads != [4]int{1, 1, 1, 1} {
			t.Fatalf("settings reads=%v; want one read per appearance value", s.reads)
		}
		bg, ok := sheet.Resolve(style.Sel{Name: "window"}).BackgroundColor()
		if !ok || rgba(bg) != window {
			t.Fatalf("dark=%v window=%v; want %v", dark, bg, window)
		}
		for _, name := range []string{"label", "check-button-text", "switch-text", PrimaryContent} {
			text := sheet.Resolve(style.Sel{Name: name})
			family, _ := text.FontFamily()
			size, _ := text.FontSize()
			if family != s.family || size != s.size {
				t.Fatalf("%s lost settings typography: %q/%g", name, family, size)
			}
		}
		bg, _ = sheet.Resolve(style.Sel{Name: Primary}).BackgroundColor()
		if rgba(bg) != rgb(0x0078D4) {
			t.Fatal("settings accent was not passed to Modern")
		}
		want := Sheet(Options{Dark: dark, AccentColor: s.accent, FontFamily: s.family, FontSize: s.size})
		if !reflect.DeepEqual(sheet, want) {
			t.Fatal("FromSettings differs from the explicit Options contract")
		}
	}
}

func TestFromSettingsSnapshotIsolationAndNormalization(t *testing.T) {
	accent := rgb(0x0078D4)
	s := &appearanceSettings{accent: &accent, family: "First", size: 16}
	first := FromSettings(s)
	accent = rgb(0xD03030)
	s.scheme, s.family, s.size = gui.ColorSchemeDark, "Second", 20
	second := FromSettings(s)
	for _, tc := range []struct {
		sheet  style.StyleSheet
		accent color.RGBA
		family string
		size   float32
	}{{first, rgb(0x0078D4), "First", 16}, {second, rgb(0xD03030), "Second", 20}} {
		bg, _ := tc.sheet.Resolve(style.Sel{Name: Primary}).BackgroundColor()
		text := tc.sheet.Resolve(style.Sel{Name: "label"})
		family, _ := text.FontFamily()
		size, _ := text.FontSize()
		if rgba(bg) != tc.accent || family != tc.family || size != tc.size {
			t.Fatal("settings mutation changed an already constructed sheet")
		}
	}
	if s.reads != [4]int{2, 2, 2, 2} {
		t.Fatal("constructed sheets kept reading their source")
	}
	if !reflect.DeepEqual(FromSettings(ui.Settings{}), Sheet(Options{})) {
		t.Fatal("zero settings no longer uses Modern's existing defaults")
	}
	if !reflect.DeepEqual(FromSettings(&appearanceSettings{size: float32(math.NaN())}), Sheet(Options{})) {
		t.Fatal("invalid font size did not use the existing Options normalization")
	}
}

func TestPrimarySolidColorsAndIndependentContent(t *testing.T) {
	// Independent opaque sRGB byte expectations. Interaction shades are 6%/12%
	// darker, except black which is lifted so it still has visible feedback.
	for _, tc := range []struct {
		accent, foreground color.RGBA
		backgrounds        [3]color.RGBA
	}{
		{rgb(0x3366CC), rgb(0xFFFFFF), [3]color.RGBA{rgb(0x3366CC), rgb(0x3060C0), rgb(0x2D5AB4)}},
		{rgb(0x0078D4), rgb(0xFFFFFF), [3]color.RGBA{rgb(0x0078D4), rgb(0x0071C7), rgb(0x006ABB)}},
		{rgb(0x000000), rgb(0xFFFFFF), [3]color.RGBA{rgb(0x000000), rgb(0x0F0F0F), rgb(0x1F1F1F)}},
		{rgb(0xFFFFFF), rgb(0x000000), [3]color.RGBA{rgb(0xFFFFFF), rgb(0xF0F0F0), rgb(0xE0E0E0)}},
		{rgb(0xFFDC46), rgb(0x000000), [3]color.RGBA{rgb(0xFFDC46), rgb(0xF0CF42), rgb(0xE0C23E)}},
	} {
		for _, dark := range []bool{false, true} {
			sheet := Sheet(Options{Dark: dark, AccentColor: tc.accent, FontFamily: "Primary Font", FontSize: 17})
			for i, state := range []style.State{style.Normal, style.Hovered, style.Pressed} {
				s := resolved(sheet, Primary, state)
				bg, _ := s.BackgroundColor()
				if rgba(bg) != tc.backgrounds[i] {
					t.Fatalf("primary dark=%v accent=%v state=%v background=%v; want %v", dark, tc.accent, state, bg, tc.backgrounds[i])
				}
				if border, _ := s.BorderWidth(); border != 0 {
					t.Fatal("solid primary acquired a resting/interaction border")
				}
				fg, _ := s.ForegroundColor()
				content := resolved(sheet, PrimaryContent, state)
				ink, ok := content.ForegroundColor()
				if !ok || rgba(ink) != tc.foreground || rgba(fg) != tc.foreground {
					t.Fatalf("primary/content foreground dark=%v accent=%v: %v/%v", dark, tc.accent, fg, ink)
				}
				if family, _ := content.FontFamily(); family != "Primary Font" {
					t.Fatal("primary content lost independent typography")
				}
				if size, _ := content.FontSize(); size != 17 {
					t.Fatal("primary content lost font size")
				}
				if _, hasBackground := content.BackgroundColor(); hasBackground {
					t.Fatal("primary content paints another button surface")
				}
			}
			p := colorsFor(dark)
			for _, name := range []string{"label", "icon"} {
				fg, _ := resolved(sheet, name, style.Normal).ForegroundColor()
				if rgba(fg) != p.text {
					t.Fatal("primary foreground leaked into ordinary content")
				}
			}
			bg, _ := resolved(sheet, Primary, style.Disabled).BackgroundColor()
			fg, _ := resolved(sheet, PrimaryContent, style.Disabled).ForegroundColor()
			if rgba(bg) != p.button || rgba(fg) != p.disabled {
				t.Fatal("disabled primary kept accent or pointer feedback")
			}
			disabled := resolved(sheet, Primary, style.Disabled)
			border, _ := disabled.BorderColor()
			if width, _ := disabled.BorderWidth(); width != 0 || rgba(border) != p.border {
				t.Fatal("disabled primary changed its optional neutral border")
			}
		}
	}
}

func TestButtonFocusAppearance(t *testing.T) {
	for _, dark := range []bool{false, true} {
		neutral := color.RGBA{R: 198, G: 198, B: 206, A: 255}
		if dark {
			neutral = color.RGBA{R: 85, G: 85, B: 95, A: 255}
		}
		for _, accent := range []color.Color{nil, color.Black, color.White, rgb(0xFF0000), rgb(0x3366CC), rgb(0x0078D4), rgb(0xFFDC46)} {
			rules := Rules(Options{Dark: dark, AccentColor: accent})
			sheet := style.Sheet(rules...)
			for _, name := range []string{"button", Primary, Flat} {
				focused := sheet.Resolve(style.Sel{Name: name, Part: "focus", State: style.Focused})
				focus, _ := focused.BorderColor()
				base := rgba(focus)
				if name == Primary {
					ordinary, _ := sheet.Resolve(style.Sel{Name: "button", Part: "focus", State: style.Focused}).BorderColor()
					if base != rgba(ordinary) {
						t.Fatalf("primary pointer focus changed: dark=%v accent=%v got=%v want %v", dark, accent, base, ordinary)
					}
				}
				// Independent integer reference: ordinary/Flat keyboard focus
				// retains the existing 75% focus + 25% neutral border blend.
				blend := func(a, b uint8) uint8 { return uint8((3*int(a) + int(b) + 2) / 4) }
				want := color.RGBA{R: blend(base.R, neutral.R), G: blend(base.G, neutral.G), B: blend(base.B, neutral.B), A: 255}
				if accent == nil {
					blue := color.RGBA{R: 102, G: 147, B: 217, A: 255}
					if dark {
						blue = color.RGBA{R: 87, G: 127, B: 190, A: 255}
					}
					if want != blue {
						t.Fatalf("default blue reference changed: %v want %v", want, blue)
					}
				}
				if name == Primary {
					// Fixed neutral bytes depend only on the mode, not the fill
					// or its foreground. Pointer focus keeps the default above.
					want = color.RGBA{R: 65, G: 65, B: 71, A: 255}
					if dark {
						want = color.RGBA{R: 216, G: 216, B: 222, A: 255}
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
							t.Fatalf("keyboard outline color: dark=%v accent=%v name=%s got=%v want=%v", dark, accent, name, bc, want)
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

func TestPrimaryContentOverridesAreIndependent(t *testing.T) {
	rules := Rules(Options{AccentColor: rgb(0x3366CC)})
	sheet := style.Sheet(append(rules,
		style.Name(PrimaryContent).ForegroundColor(rgb(0xEEEEEE)).FontSize(19),
	)...)
	for _, state := range []style.State{style.Normal, style.Hovered, style.Pressed, style.FocusVisible} {
		content := resolved(sheet, PrimaryContent, state)
		ink, _ := content.ForegroundColor()
		size, _ := content.FontSize()
		if rgba(ink) != rgb(0xEEEEEE) || size != 19 {
			t.Fatal("explicit primary content override was lost")
		}
	}
	for _, name := range []string{Primary, "label", "icon"} {
		original := resolved(style.Sheet(rules...), name, style.Normal)
		actual := resolved(sheet, name, style.Normal)
		if !style.SameRules([]style.Rule{{Style: original}}, []style.Rule{{Style: actual}}) {
			t.Fatalf("primary content override leaked into %s", name)
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

func TestSurfacePresets(t *testing.T) {
	for _, dark := range []bool{false, true} {
		card, edge, sidebar := rgb(0xFFFFFF), rgb(0xE4E4E5), rgb(0xE2E2E8)
		if dark {
			card, edge, sidebar = rgb(0x28282D), rgb(0x404044), rgb(0x18181C)
		}
		for _, accent := range []color.Color{nil, color.Black, color.White, rgb(0xFF0000)} {
			sheet := Sheet(Options{Dark: dark, AccentColor: accent})
			for _, test := range []struct {
				name          string
				background    color.RGBA
				width, radius float32
			}{{Card, card, 1, 10}, {Sidebar, sidebar, 0, 0}} {
				for _, state := range []style.State{style.Normal, style.Hovered, style.Pressed, style.Disabled} {
					s := resolved(sheet, test.name, state)
					bg, ok := s.BackgroundColor()
					width, hasWidth := s.BorderWidth()
					radius, hasRadius := s.Radius()
					if !ok || rgba(bg) != test.background || !hasWidth || width != test.width || !hasRadius || radius != test.radius {
						t.Fatalf("surface %s dark=%v state=%v: bg=%v border=%g radius=%g", test.name, dark, state, bg, width, radius)
					}
					if shadow, ok := s.Shadow(); !ok || shadow != (style.Shadow{}) {
						t.Fatal("surface should explicitly have no shadow")
					}
				}
			}
			if border, _ := resolved(sheet, Card, style.Normal).BorderColor(); rgba(border) != edge {
				t.Fatalf("card border dark=%v: %v, want %v", dark, border, edge)
			}
			custom := style.Sheet(append(Rules(Options{Dark: dark}), style.Name(Card).BorderWidth(0).Radius(14),
				style.Name(Sidebar).BackgroundColor(color.Black))...)
			if width, _ := resolved(custom, Card, style.Normal).BorderWidth(); width != 0 {
				t.Fatal("card application override ignored")
			}
			if radius, _ := resolved(custom, Card, style.Normal).Radius(); radius != 14 {
				t.Fatal("card radius override ignored")
			}
			if bg, _ := resolved(custom, Sidebar, style.Normal).BackgroundColor(); bg != color.Black {
				t.Fatal("sidebar application override ignored")
			}
		}
	}
}

func TestSemanticForegroundPresets(t *testing.T) {
	for _, dark := range []bool{false, true} {
		want := []color.RGBA{rgb(0x8A5900), rgb(0xBA3038), rgb(0x276B43)}
		backgrounds := []color.RGBA{rgb(0xF6F6F8), rgb(0xFFFFFF), rgb(0xE2E2E8)}
		disabled := rgb(0x97979F)
		if dark {
			want = []color.RGBA{rgb(0xEAC16D), rgb(0xF3959A), rgb(0x83CAA2)}
			backgrounds = []color.RGBA{rgb(0x202024), rgb(0x28282D), rgb(0x323238), rgb(0x18181C)}
			disabled = rgb(0x797983)
		}
		for _, accent := range []color.Color{nil, color.Black, color.White, rgb(0xFF0000), rgb(0x0080FF)} {
			sheet := Sheet(Options{Dark: dark, AccentColor: accent})
			for i, name := range []string{Warning, Error, Success} {
				for _, state := range []style.State{style.Normal, style.Hovered, style.Pressed, style.Focused, style.FocusVisible, style.Disabled} {
					s := resolved(sheet, name, state)
					fg, ok := s.ForegroundColor()
					expected := want[i]
					if state == style.Disabled {
						expected = disabled
					}
					if !ok || rgba(fg) != expected {
						t.Fatalf("semantic %s dark=%v state=%v: %v, want %v", name, dark, state, fg, expected)
					}
					_, background := s.BackgroundColor()
					_, border := s.BorderWidth()
					_, radius := s.Radius()
					_, shadow := s.Shadow()
					if background || border || radius || shadow {
						t.Fatal("foreground-only semantic style added surface decoration")
					}
				}
				// Opaque sRGB colors, no alpha compositing: status text must be
				// readable on the window, card, sidebar and ordinary button surfaces.
				for _, bg := range backgrounds {
					if ratio := contrast(want[i], bg); ratio < 4.5 {
						t.Fatalf("%s contrast against %v: %g, want >= 4.5", name, bg, ratio)
					}
				}
			}
			custom := style.Sheet(append(Rules(Options{Dark: dark}), style.Name(Warning).ForegroundColor(color.Black))...)
			if fg, _ := resolved(custom, Warning, style.Normal).ForegroundColor(); fg != color.Black {
				t.Fatal("warning application override ignored")
			}
			for _, name := range []string{"label", "icon", Error, Success} {
				if !style.SameRules([]style.Rule{{Style: resolved(custom, name, style.Normal)}}, []style.Rule{{Style: resolved(sheet, name, style.Normal)}}) {
					t.Fatalf("warning override leaked into %s", name)
				}
			}
		}
	}
}

func TestSidebarSurfaceHierarchy(t *testing.T) {
	for _, dark := range []bool{false, true} {
		for _, accent := range []color.Color{nil, color.Black, color.White, rgb(0xFF0000)} {
			sheet := Sheet(Options{Dark: dark, AccentColor: accent})
			bg, _ := resolved(sheet, Sidebar, style.Normal).BackgroundColor()
			// Sidebar remains darker than the window/card and flat
			// interaction surfaces; it no longer lies between window and hover.
			for _, sel := range []style.Sel{{Name: "window"}, {Name: Card}, {Name: Flat, State: style.Hovered}, {Name: Flat, State: style.Pressed}} {
				surface, _ := sheet.Resolve(sel).BackgroundColor()
				if luminance(rgba(bg)) >= luminance(rgba(surface)) {
					t.Fatalf("sidebar dark=%v: %v is not darker than %s/%v %v", dark, bg, sel.Name, sel.State, surface)
				}
			}
			for _, name := range []string{"label", MutedText, Warning, Error, Success} {
				fg, _ := resolved(sheet, name, style.Normal).ForegroundColor()
				if ratio := contrast(rgba(fg), rgba(bg)); ratio < 4.5 {
					t.Fatalf("sidebar dark=%v %s contrast=%g, want >= 4.5", dark, name, ratio)
				}
			}
		}
	}
}

func TestFlatButtonAppearance(t *testing.T) {
	for _, dark := range []bool{false, true} {
		hover, pressed, border := rgb(0xF6F6F6), rgb(0xEDEDED), rgb(0xC6C6CE)
		if dark {
			hover, pressed, border = rgb(0x39393F), rgb(0x414147), rgb(0x55555F)
		}
		for _, accent := range []color.Color{nil, color.Black, color.White, rgb(0xFF0000)} {
			sheet := Sheet(Options{Dark: dark, AccentColor: accent})
			text, disabled := rgb(0x202024), rgb(0x97979F)
			if dark {
				text, disabled = rgb(0xECECEE), rgb(0x797983)
			}
			for _, test := range []struct {
				state style.State
				bg    color.RGBA
				width float32
			}{{style.Normal, color.RGBA{}, 0}, {style.Hovered, hover, 1}, {style.Pressed, pressed, 1},
				{style.Focused, color.RGBA{}, 0}, {style.FocusVisible, color.RGBA{}, 0}, {style.Disabled, color.RGBA{}, 0}} {
				s := resolved(sheet, Flat, test.state)
				bg, ok := s.BackgroundColor()
				if !ok || rgba(bg) != test.bg {
					t.Fatalf("flat dark=%v state=%v: %v, want %v", dark, test.state, bg, test.bg)
				}
				if width, ok := s.BorderWidth(); !ok || width != test.width {
					t.Fatalf("flat border dark=%v state=%v: %g, want %g", dark, test.state, width, test.width)
				}
				if ink, ok := s.BorderColor(); !ok || rgba(ink) != border {
					t.Fatalf("flat border color dark=%v state=%v: %v, want %v", dark, test.state, ink, border)
				}
				if radius, _ := s.Radius(); radius != 6 {
					t.Fatal("flat button lost its rounded interaction surface")
				}
				fg, _ := s.ForegroundColor()
				wantForeground := text
				if test.state == style.Disabled {
					wantForeground = disabled
				}
				if rgba(fg) != wantForeground {
					t.Fatalf("flat foreground dark=%v state=%v: %v, want %v", dark, test.state, fg, wantForeground)
				}
				if test.state == style.Hovered || test.state == style.Pressed {
					// 先断言独立 RGB/宽度预期，再验证完整交互表面与普通按钮一致。
					ordinary := resolved(sheet, "button", test.state)
					if !style.SameRules([]style.Rule{{Style: s}}, []style.Rule{{Style: ordinary}}) {
						t.Fatalf("flat interaction differs from button: dark=%v state=%v", dark, test.state)
					}
				}
				for _, surface := range []string{"window", Card, Sidebar} {
					background, _ := resolved(sheet, surface, style.Normal).BackgroundColor()
					if test.bg.A > 0 {
						background = test.bg
					}
					if test.state != style.Disabled && contrast(rgba(fg), rgba(background)) < 4.5 {
						t.Fatal("flat text contrast fell below 4.5:1")
					}
				}
			}
		}
	}
}

func TestButtonInteractionBorderOverrides(t *testing.T) {
	for _, dark := range []bool{false, true} {
		rules := Rules(Options{Dark: dark})
		// 普通按钮仍从自身常态继承描边，不因 Flat 的状态规则被固定为 1 DIP。
		for _, name := range []string{"button", Primary} {
			custom := style.Sheet(append(rules, style.Name(name).BorderWidth(3))...)
			for _, state := range []style.State{style.Normal, style.Hovered, style.Pressed, style.Disabled} {
				if width, _ := resolved(custom, name, state).BorderWidth(); width != 3 {
					t.Fatalf("%s/%v lost inherited application border: %g", name, state, width)
				}
			}
		}
		custom := style.Sheet(append(rules,
			style.Name(Flat).State(style.Hovered).BorderWidth(0).BackgroundColor(color.White),
			style.Name(Flat).State(style.Pressed).BorderWidth(2).BorderColor(color.Black),
		)...)
		if width, _ := resolved(custom, Flat, style.Hovered).BorderWidth(); width != 0 {
			t.Fatal("application cannot disable flat hover border")
		}
		if bg, _ := resolved(custom, Flat, style.Hovered).BackgroundColor(); bg != color.White {
			t.Fatal("application cannot override flat hover background")
		}
		pressed := resolved(custom, Flat, style.Pressed)
		width, _ := pressed.BorderWidth()
		ink, _ := pressed.BorderColor()
		if width != 2 || ink != color.Black {
			t.Fatal("application cannot override flat pressed border")
		}
		for _, state := range []style.State{style.Normal, style.Disabled} {
			s := resolved(custom, Flat, state)
			width, _ := s.BorderWidth()
			bg, _ := s.BackgroundColor()
			if width != 0 || rgba(bg).A != 0 {
				t.Fatal("flat interaction overrides leaked into rest/disabled")
			}
		}
	}
}

func TestTableStylesNeutralSelectionAndIndependentText(t *testing.T) {
	for _, dark := range []bool{false, true} {
		// 独立的 sRGB 字节预期；与 TreeView 使用同一中性选中色阶。
		want := []color.RGBA{rgb(0xE1E1E3), rgb(0xD9D9DB), rgb(0xD2D2D4)}
		if dark {
			want = []color.RGBA{rgb(0x343438), rgb(0x3B3B3F), rgb(0x434347)}
		}
		for _, accent := range []color.Color{nil, color.Black, color.White, rgb(0xFF0000), rgb(0x0080FF)} {
			sheet := Sheet(Options{Dark: dark, AccentColor: accent, FontFamily: "Table Font", FontSize: 17})
			for i, state := range []style.State{style.Normal, style.Hovered, style.Pressed} {
				selected := sheet.Resolve(style.Sel{Name: "table-row", Part: "selected", State: state})
				bg, ok := selected.BackgroundColor()
				if !ok || rgba(bg) != want[i] {
					t.Fatalf("table selection dark=%v state=%v: %v", dark, state, bg)
				}
				if radius, _ := selected.Radius(); radius != 4 {
					t.Fatal("table selection lost its rounded corners")
				}
				fg, _ := resolved(sheet, "table-cell-text", style.Normal).ForegroundColor()
				if contrast(rgba(fg), rgba(bg)) < 4.5 {
					t.Fatal("table selection reduced text contrast below 4.5:1")
				}
				if width, _ := sheet.Resolve(style.Sel{Name: "table-row", Part: "current", State: state}).BorderWidth(); width != 0 {
					t.Fatal("pointer focus added a current outline")
				}
			}
			if width, _ := sheet.Resolve(style.Sel{Name: "table-row", Part: "current", State: style.FocusVisible}).BorderWidth(); width != 1 {
				t.Fatal("keyboard current lost its thin outline")
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
				style.Name("table-row").Part("selected").BackgroundColor(rgb(0x123456)),
				style.Name("table-row").Part("current").BorderColor(color.Black).BorderWidth(2),
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
			bg, _ = custom.Resolve(style.Sel{Name: "table-row", Part: "selected"}).BackgroundColor()
			current := custom.Resolve(style.Sel{Name: "table-row", Part: "current"})
			ink, _ := current.BorderColor()
			width, _ := current.BorderWidth()
			if rgba(bg) != rgb(0x123456) || ink != color.Black || width != 2 {
				t.Fatal("table selection/current application override ignored")
			}
		}
	}
}

func TestTableSurfaceHierarchy(t *testing.T) {
	for _, dark := range []bool{false, true} {
		background, edge, grid := rgb(0xF6F6F8), rgb(0xD8D8DA), rgb(0xE9E9EB)
		header := []color.RGBA{rgb(0xF2F2F4), rgb(0xEBEBED), rgb(0xE5E5E7)}
		if dark {
			background, edge, grid = rgb(0x202024), rgb(0x3D3D40), rgb(0x2C2C30)
			header = []color.RGBA{rgb(0x242428), rgb(0x2A2A2E), rgb(0x303034)}
		}
		sheet := Sheet(Options{Dark: dark})
		frame := resolved(sheet, "table-view", style.Normal)
		bg, _ := frame.BackgroundColor()
		border, _ := frame.BorderColor()
		if rgba(bg) != background || rgba(border) != edge {
			t.Fatalf("table surface dark=%v: background=%v edge=%v", dark, bg, border)
		}
		for i, state := range []style.State{style.Normal, style.Hovered, style.Pressed} {
			bg, _ := resolved(sheet, "table-header", state).BackgroundColor()
			separator := sheet.Resolve(style.Sel{Name: "table-header", Part: "separator", State: state})
			ink, _ := separator.ForegroundColor()
			width, _ := separator.BorderWidth()
			if rgba(bg) != header[i] || rgba(ink) != edge || width != 1 {
				t.Fatalf("header hierarchy dark=%v state=%v: background=%v separator=%v/%g", dark, state, bg, ink, width)
			}
		}
		ink, _ := sheet.Resolve(style.Sel{Name: "table-grid", Part: "horizontal"}).BorderColor()
		if rgba(ink) != grid {
			t.Fatalf("row separator dark=%v: %v, want %v", dark, ink, grid)
		}
	}
}

func TestTreeSelectionColorsIgnoreAccent(t *testing.T) {
	for _, dark := range []bool{false, true} {
		// 固定调色板预期，不通过被测实现的 mix 函数推导。
		want := []color.RGBA{rgb(0xE1E1E3), rgb(0xD9D9DB), rgb(0xD2D2D4)}
		wantBorder := rgb(0x61616B)
		if dark {
			want = []color.RGBA{rgb(0x343438), rgb(0x3B3B3F), rgb(0x434347)}
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
				if rgba(border) != wantBorder || width != 0 {
					t.Fatalf("pointer current outline=%v/%g, want neutral %v/0 DIP", border, width, wantBorder)
				}
			}
			keyboard := sheet.Resolve(style.Sel{Name: "tree-item", Part: "current", State: style.FocusVisible})
			if width, _ := keyboard.BorderWidth(); width != 1 {
				t.Fatalf("keyboard current outline=%g, want 1 DIP", width)
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
		for _, name := range []string{"widget", "label", Primary, PrimaryContent, MutedText, Card, Sidebar, Flat, Warning, Error, Success, "button", "text-input", "scroll-view", "header-bar", "popover", "menu", "menu-item-text", "menu-item-text-disabled"} {
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
				// Rules now includes Basic before Modern; validate the final value.
				if !style.SameRules([]style.Rule{{Sel: rule.Sel, Style: sheet.Resolve(rule.Sel)}}, []style.Rule{style.Name("window").BackgroundColor(p.window)}) {
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
						textColor, focusColor := p.text, rgba(focus)
						if name == Primary {
							// Children explicitly select PrimaryContent; the body
							// foreground is not inherited from the solid button.
							fg, _ := resolved(sheet, PrimaryContent, style.Normal).ForegroundColor()
							textColor = rgba(fg)
							ink, _ := sheet.Resolve(style.Sel{Name: Primary, Part: "focus", State: style.FocusVisible}).BorderColor()
							focusColor = rgba(ink)
							if textColor != rgb(0x000000) && textColor != rgb(0xFFFFFF) {
								t.Fatal("primary content has no stable contrasting foreground")
							}
							// Mode selects the fixed gray, regardless of the accent
							// or the fill's black/white foreground choice.
							wantFocus := rgb(0x414147)
							if dark {
								wantFocus = rgb(0xD8D8DE)
							}
							if focusColor != wantFocus {
								t.Fatalf("primary mode-neutral focus: dark=%v accent=%v got=%v want %v", dark, accent, focusColor, wantFocus)
							}
						}
						for _, state := range []style.State{style.Normal, style.Hovered, style.Pressed} {
							s := resolved(sheet, name, state)
							bg, _ := s.BackgroundColor()
							background := rgba(bg)
							if background.A == 0 { // menu text is over the menu surface
								background = p.surface
							}
							if contrast(textColor, background) < 4.5 {
								t.Fatalf("text contrast: dark=%v accent=%v %s/%v", dark, accent, name, state)
							}
							if ratio := contrast(focusColor, background); name != "menu-item" && name != Primary && ratio < 3 {
								t.Fatalf("focus contrast: dark=%v accent=%v %s/%v got=%g want >= 3", dark, accent, name, state, ratio)
							}
						}
					}
				}
			}
		}
	}
}
