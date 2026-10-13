package gui

import (
	"image/color"
	"reflect"
	"testing"

	"github.com/golang-gui/goui/core/colors"
	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/layout"
	"github.com/golang-gui/goui/style"
	"github.com/golang-gui/goui/theme/basic"
)

func TestDefaultStyleSheetResolvesBuiltInStyles(t *testing.T) {
	sheet := DefaultStyleSheet()

	button := sheet.Resolve(style.Sel{Name: styleNameButton, State: style.Hovered})
	background, ok := button.BackgroundColor()
	if !ok || !colors.Equal(background, color.RGBA{R: 230, G: 230, B: 230, A: 255}) {
		t.Fatalf("unexpected button hover background: %v ok=%v", background, ok)
	}
	radius, ok := button.Radius()
	if !ok || radius != 4 {
		t.Fatalf("unexpected button radius: %v ok=%v", radius, ok)
	}

	input := sheet.Resolve(style.Sel{Name: styleNameTextInput, State: style.Focused})
	borderColor, ok := input.BorderColor()
	if !ok || !colors.Equal(borderColor, defaultAccentColor) {
		t.Fatalf("unexpected focused text input border: %v ok=%v", borderColor, ok)
	}
}

func TestDefaultStyleSheetResolvesWidgetStyles(t *testing.T) {
	// Independent, opaque sRGB expectations from the existing widget fallback.
	// No application or custom theme is necessary, including for child labels.
	for _, tc := range []struct {
		name, part string
		background color.RGBA
	}{
		{"check-button", "indicator-checked", color.RGBA{R: 51, G: 112, B: 204, A: 255}},
		{"switch", "track-checked", color.RGBA{R: 70, G: 130, B: 220, A: 255}},
		{"switch", "thumb-checked", color.RGBA{R: 255, G: 255, B: 255, A: 255}},
		{"progress-bar", "", color.RGBA{R: 225, G: 225, B: 225, A: 255}},
		{"tab-item", "selected", color.RGBA{R: 210, G: 210, B: 210, A: 255}},
		{"tree-item", "selected", color.RGBA{R: 210, G: 218, B: 230, A: 255}},
		{"table-view", "", color.RGBA{R: 255, G: 255, B: 255, A: 255}},
		{"drop-down", "", color.RGBA{R: 245, G: 245, B: 245, A: 255}},
	} {
		s := DefaultStyleSheet().Resolve(style.Sel{Name: tc.name, Part: tc.part})
		bg, ok := s.BackgroundColor()
		if !ok || !colors.Equal(bg, tc.background) {
			t.Errorf("%s/%s background=%v set=%v, want %v", tc.name, tc.part, bg, ok, tc.background)
		}
	}
	for _, name := range []string{"check-button-text", "switch-text", "tab-item-text", "tree-item-text", "table-cell-text", "drop-down-text", "drop-down-item-text"} {
		for _, state := range []style.State{style.Normal, style.Disabled} {
			s := DefaultStyleSheet().Resolve(style.Sel{Name: name, State: state})
			if size, ok := s.FontSize(); !ok || size != 14 {
				t.Errorf("%s state=%v font=%g set=%v; want 14 pt", name, state, size, ok)
			}
			if _, ok := s.FontFamily(); !ok {
				t.Errorf("%s has no independent font family", name)
			}
			if fg, ok := s.ForegroundColor(); !ok || fg == nil {
				t.Errorf("%s has no independent text color", name)
			}
		}
	}
}

func TestApplicationStyleSheetDefaultsToNilAndRequestsLayoutOnSet(t *testing.T) {
	app := &application{}
	if app.StyleSheet() != nil {
		t.Fatal("application style sheet should default to nil")
	}

	win := &window{}
	app.windows = []*window{win}
	win.layoutDirty = false
	win.paintDirty = false

	app.SetStyleSheet(style.Sheet(style.Name(styleNameButton).Radius(6)))
	if app.StyleSheet() == nil {
		t.Fatal("application style sheet was not stored")
	}
	if !win.layoutDirty || !win.paintDirty {
		t.Fatal("setting application style did not request layout")
	}

	app.SetStyleSheet(nil)
	if app.StyleSheet() != nil {
		t.Fatal("nil style sheet should clear application style")
	}
}

func TestApplicationStyleSheetInvalidatesWidgetMeasurementSubtree(t *testing.T) {
	app := &application{}
	win := &window{}
	box := NewLinearBox(layout.DirectionHorizontal)
	child := &countingMeasureWidget{size: geometry.Size{Width: 20, Height: 10}}
	box.AddChild(child)
	win.SetWidget(box)
	app.windows = []*window{win}
	c := layout.Loose(geometry.Size{Width: 100, Height: 40})

	measureWidget(box, c)
	app.SetStyleSheet(style.Sheet(style.Name(styleNameWidget).FontSize(18)))
	measureWidget(box, c)
	if child.measures != 2 {
		t.Fatalf("style change left a descendant measurement cached: measures=%d", child.measures)
	}
}

func TestStyleSheetReplacementAndRestoreBasic(t *testing.T) {
	if !style.SameRules(DefaultStyleRules(), basic.Rules()) {
		t.Fatal("GUI default entry point diverged from Basic")
	}
	app := &application{}
	useTestApplication(t, app)
	blue := color.RGBA{R: 70, G: 130, B: 220, A: 255}
	bg, ok := ResolveStyle("switch", "track-checked", style.Normal).BackgroundColor()
	if !ok || !colors.Equal(bg, blue) {
		t.Fatal("nil custom sheet did not provide Basic")
	}
	custom := style.Sheet(style.Name("switch").Part("track-checked").BackgroundColor(color.Black))
	app.SetStyleSheet(custom)
	if !reflect.DeepEqual(app.StyleSheet(), custom) {
		t.Fatal("getter must return the explicit stylesheet")
	}
	bg, ok = ResolveStyle("switch", "track-checked", style.Normal).BackgroundColor()
	if !ok || !colors.Equal(bg, color.Black) {
		t.Fatal("custom surface not applied")
	}
	if _, ok := ResolveStyle("switch", "thumb-checked", style.Normal).BackgroundColor(); ok {
		t.Fatal("custom sheet was silently merged with Basic")
	}
	app.SetStyleSheet(nil)
	bg, ok = ResolveStyle("switch", "track-checked", style.Normal).BackgroundColor()
	if app.StyleSheet() != nil || !ok || !colors.Equal(bg, blue) {
		t.Fatal("clearing the custom sheet did not restore Basic")
	}
}
