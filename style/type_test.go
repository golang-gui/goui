package style

import (
	"image/color"
	"testing"

	"github.com/golang-gui/goui/core/colors"
	"github.com/golang-gui/goui/core/geometry"
)

func TestShadowResolution(t *testing.T) {
	base := Shadow{
		Color:  color.NRGBA{R: 80, G: 120, B: 240, A: 64},
		Offset: geometry.Point{X: -2, Y: 4}, BlurRadius: 16, SpreadRadius: -1,
	}
	// A partial-looking value still replaces the whole shadow. In particular,
	// nil Color must not accidentally inherit base.Color.
	replacement := Shadow{Offset: geometry.Point{Y: 2}}
	sheet := Sheet(
		Name("menu").Shadow(base).Radius(8),
		Name("menu").Part("body").Shadow(replacement),
		Name("menu").State(Hovered).Shadow(Shadow{}),
		Name("menu").State(Pressed).BackgroundColor(color.White),
		Name("last-wins").Shadow(base),
		Name("last-wins").Shadow(replacement),
	)
	for _, test := range []struct {
		name string
		sel  Sel
		want Shadow
		set  bool
	}{
		{"unset", Sel{Name: "other"}, Shadow{}, false},
		{"base", Sel{Name: "menu"}, base, true},
		{"state fallback", Sel{Name: "menu", State: Focused}, base, true},
		{"unrelated field", Sel{Name: "menu", State: Pressed}, base, true},
		{"atomic part override", Sel{Name: "menu", Part: "body"}, replacement, true},
		{"part state fallback", Sel{Name: "menu", Part: "body", State: Hovered}, replacement, true},
		{"explicit disable", Sel{Name: "menu", State: Hovered}, Shadow{}, true},
		{"last wins", Sel{Name: "last-wins"}, replacement, true},
	} {
		t.Run(test.name, func(t *testing.T) {
			got, set := sheet.Resolve(test.sel).Shadow()
			if set != test.set || !colors.Equal(got.Color, test.want.Color) ||
				got.Offset != test.want.Offset || got.BlurRadius != test.want.BlurRadius || got.SpreadRadius != test.want.SpreadRadius {
				t.Fatalf("Shadow() = %+v, %v; want %+v, %v", got, set, test.want, test.set)
			}
		})
	}
	if radius, set := sheet.Resolve(Sel{Name: "menu", State: Hovered}).Radius(); !set || radius != 8 {
		t.Fatal("disabling a shadow changed another style field")
	}
}

func TestSameRulesComparesEveryShadowMember(t *testing.T) {
	base := Shadow{
		Color:  color.RGBA{R: 32, G: 64, B: 96, A: 128},
		Offset: geometry.Point{X: -2, Y: 4}, BlurRadius: 16, SpreadRadius: -1,
	}
	rules := []Rule{Name("menu").Shadow(base)}
	for _, test := range []struct {
		name   string
		change func(*Shadow)
		equal  bool
	}{
		{"same value", func(*Shadow) {}, true},
		{"equivalent color type", func(s *Shadow) {
			s.Color = color.RGBA64{R: 32 * 257, G: 64 * 257, B: 96 * 257, A: 128 * 257}
		}, true},
		{"red", func(s *Shadow) { s.Color = color.RGBA{R: 33, G: 64, B: 96, A: 128} }, false},
		{"green", func(s *Shadow) { s.Color = color.RGBA{R: 32, G: 65, B: 96, A: 128} }, false},
		{"blue", func(s *Shadow) { s.Color = color.RGBA{R: 32, G: 64, B: 97, A: 128} }, false},
		{"alpha", func(s *Shadow) { s.Color = color.RGBA{R: 32, G: 64, B: 96, A: 129} }, false},
		{"nil color", func(s *Shadow) { s.Color = nil }, false},
		{"offset x", func(s *Shadow) { s.Offset.X++ }, false},
		{"offset y", func(s *Shadow) { s.Offset.Y++ }, false},
		{"blur", func(s *Shadow) { s.BlurRadius++ }, false},
		{"spread", func(s *Shadow) { s.SpreadRadius++ }, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			other := base
			test.change(&other)
			if got := SameRules(rules, []Rule{Name("menu").Shadow(other)}); got != test.equal {
				t.Fatalf("SameRules = %v, want %v", got, test.equal)
			}
		})
	}
	if SameRules([]Rule{Name("menu")}, []Rule{Name("menu").Shadow(Shadow{})}) {
		t.Fatal("unset and explicitly disabled shadows must be distinct")
	}
}

func TestShadowBuilderCopiesValue(t *testing.T) {
	shadow := Shadow{Color: color.Black, BlurRadius: 16}
	base := Name("menu").Shadow(shadow)
	off := base.Shadow(Shadow{})
	shadow.BlurRadius = 2
	got, set := base.Style.Shadow()
	if !set || got.BlurRadius != 16 {
		t.Fatal("changing the source struct or derived rule changed the base rule")
	}
	if got, set := off.Style.Shadow(); !set || got.Color != nil || got.BlurRadius != 0 {
		t.Fatal("derived rule did not disable the shadow")
	}
}
