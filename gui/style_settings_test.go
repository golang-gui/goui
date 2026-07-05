package gui

import (
	"image/color"
	"testing"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/platform/graphics"
)

// fakePlatformSettings is a settable platform.Settings used to drive the
// settings-derived default style sheet in tests.
type fakePlatformSettings struct {
	scheme ColorScheme
	accent color.Color
	family string
	size   float32
}

func (f *fakePlatformSettings) ColorScheme() (ColorScheme, error) { return f.scheme, nil }
func (f *fakePlatformSettings) AccentColor() (color.Color, error) { return f.accent, nil }
func (f *fakePlatformSettings) FontFamily() (string, error)       { return f.family, nil }
func (f *fakePlatformSettings) FontSize() (float32, error)        { return f.size, nil }

// TestDefaultStyleSheetFollowsSystemAccent proves the built-in default sheet is
// derived from system settings: the text input focus border tracks the system
// accent color instead of the hard-coded fallback constant.
func TestDefaultStyleSheetFollowsSystemAccent(t *testing.T) {
	accent := color.RGBA{R: 200, G: 40, B: 120, A: 255}
	app := &application{settings: &Settings{settings: &fakePlatformSettings{
		scheme: ColorSchemeLight,
		accent: accent,
		size:   defaultFontSize,
	}}}
	app.rebuildDefaultStyle()
	useTestApplication(t, app)

	win := &window{}
	input := NewTextInput()
	win.SetWidget(input)
	input.Arrange(geometry.Rect(0, 0, 100, 24))
	if !win.SetFocusedWidget(input) {
		t.Fatal("text input should accept focus")
	}

	painter := new(testTextInputPainter)
	input.Paint(painter)
	if painter.drawRectBrush != graphics.ColorOf(accent) {
		t.Fatalf("focused border should follow system accent, got %+v", painter.drawRectBrush)
	}
}

// TestSettingsChangeRebuildsDefaultStyleAndRelayouts checks that a system
// setting change rebuilds the cached default sheet and marks windows dirty.
func TestSettingsChangeRebuildsDefaultStyleAndRelayouts(t *testing.T) {
	fake := &fakePlatformSettings{
		scheme: ColorSchemeLight,
		accent: color.RGBA{R: 10, G: 20, B: 30, A: 255},
		size:   defaultFontSize,
	}
	app := &application{settings: &Settings{settings: fake}}
	app.rebuildDefaultStyle()

	win := &window{app: app}
	app.windows = []*window{win}
	input := NewTextInput()
	win.SetWidget(input)
	input.Arrange(geometry.Rect(0, 0, 100, 24))
	if !win.SetFocusedWidget(input) {
		t.Fatal("text input should accept focus")
	}

	newAccent := color.RGBA{R: 240, G: 50, B: 60, A: 255}
	fake.accent = newAccent
	win.layoutDirty = false
	win.paintDirty = false

	app.onSettingsChanged()

	if !win.layoutDirty || !win.paintDirty {
		t.Fatal("settings change should request layout and paint")
	}
	painter := new(testTextInputPainter)
	input.Paint(painter)
	if painter.drawRectBrush != graphics.ColorOf(newAccent) {
		t.Fatalf("default style did not pick up the new accent, got %+v", painter.drawRectBrush)
	}
}
