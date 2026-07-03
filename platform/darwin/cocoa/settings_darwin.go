package cocoa

import (
	"image/color"
	"strings"

	"github.com/golang-gui/goui/platform/common"
	. "github.com/golang-gui/goui/platform/darwin/frameworks/appkit"
	. "github.com/golang-gui/goui/platform/darwin/frameworks/core_graphics"
	. "github.com/golang-gui/goui/platform/darwin/frameworks/foundation"
	"github.com/golang-gui/goui/platform/graphics"
)

type Settings struct{}

func newSettings() (common.Settings, error) {
	return &Settings{}, nil
}

func (Settings) ColorScheme() (common.ColorScheme, error) {
	app := NSApp
	if app.IsNil() {
		app = NSApplicationClassId.SharedApplication()
	}
	if app.IsNil() || !app.RespondsToSelector("effectiveAppearance") {
		return 0, common.ErrSettingUnsupported
	}

	appearance := app.EffectiveAppearance()
	if appearance.IsNil() || !appearance.RespondsToSelector("name") {
		return 0, common.ErrSettingUnsupported
	}

	name := appearance.Name()
	if name == "" {
		return 0, common.ErrSettingUnsupported
	}
	if strings.Contains(name, "Dark") {
		return common.ColorSchemeDark, nil
	}
	return common.ColorSchemeLight, nil
}

func (Settings) AccentColor() (color.Color, error) {
	if NSColorClassId.ID() == 0 || !classRespondsToSelector(NSColorClassId.ID(), "controlAccentColor") {
		return graphics.Color{}, common.ErrSettingUnsupported
	}
	if NSColorSpaceClassId.ID() == 0 || !classRespondsToSelector(NSColorSpaceClassId.ID(), "sRGBColorSpace") {
		return graphics.Color{}, common.ErrSettingUnsupported
	}

	color := NSColorClassId.ControlAccentColor()
	space := NSColorSpaceClassId.SRGBColorSpace()
	if color.IsNil() || space.IsNil() || !color.RespondsToSelector("colorUsingColorSpace:") {
		return graphics.Color{}, common.ErrSettingUnsupported
	}

	rgb := color.ColorUsingColorSpace(space)
	if rgb.IsNil() {
		return graphics.Color{}, common.ErrSettingUnsupported
	}

	return graphics.Color{
		R: float32(rgb.RedComponent()),
		G: float32(rgb.GreenComponent()),
		B: float32(rgb.BlueComponent()),
		A: float32(rgb.AlphaComponent()),
	}, nil
}

func (Settings) FontFamily() (string, error) {
	font, err := systemFont()
	if err != nil {
		return "", err
	}
	if !font.RespondsToSelector("familyName") {
		return "", common.ErrSettingUnsupported
	}
	family := font.FamilyName()
	if family == "" {
		return "", common.ErrSettingUnsupported
	}
	return family, nil
}

func (Settings) FontSize() (float32, error) {
	font, err := systemFont()
	if err != nil {
		return 0, err
	}
	if !font.RespondsToSelector("pointSize") {
		return 0, common.ErrSettingUnsupported
	}
	size := font.PointSize()
	if size <= 0 {
		return 0, common.ErrSettingUnsupported
	}
	return float32(size), nil
}

func systemFont() (NSFont, error) {
	if NSFontClassId.ID() == 0 || !classRespondsToSelector(NSFontClassId.ID(), "systemFontOfSize:") {
		return NSFont{}, common.ErrSettingUnsupported
	}
	font := NSFontClassId.SystemFontOfSize(CGFloat(0))
	if font.IsNil() {
		return NSFont{}, common.ErrSettingUnsupported
	}
	return font, nil
}

func classRespondsToSelector(id ID, selector string) bool {
	return NSObject{ID: id}.RespondsToSelector(selector)
}
