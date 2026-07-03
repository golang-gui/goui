package x11

import (
	"image/color"
	"strings"
	"sync"

	"github.com/golang-gui/goui/platform/common"
	"github.com/golang-gui/goui/platform/graphics"
	"github.com/golang-gui/goui/platform/linux/libs/gtk"
	"github.com/golang-gui/goui/platform/linux/libs/pango"
	"github.com/golang-gui/goui/platform/linux/libs/pango_cairo"
)

type Settings struct{}

var gtkInit struct {
	once sync.Once
	ok   bool
	err  error
}

func newSettings() (common.Settings, error) {
	return &Settings{}, nil
}

func (Settings) ColorScheme() (common.ColorScheme, error) {
	settings, err := gtkSettings()
	if err != nil {
		return 0, err
	}

	dark, err := settings.BoolProperty("gtk-application-prefer-dark-theme")
	if err != nil {
		return 0, err
	}
	if dark {
		return common.ColorSchemeDark, nil
	}

	theme, err := settings.StringProperty("gtk-theme-name")
	if err != nil {
		return 0, err
	}
	if strings.Contains(strings.ToLower(theme), "dark") {
		return common.ColorSchemeDark, nil
	}
	return common.ColorSchemeLight, nil
}

func (Settings) AccentColor() (color.Color, error) {
	if _, err := gtkSettings(); err != nil {
		return graphics.Color{}, err
	}

	context, err := gtk.StyleContextNew()
	if err != nil {
		return graphics.Color{}, err
	}
	if context.IsNull() {
		return graphics.Color{}, common.ErrSettingUnsupported
	}
	defer context.Unref()

	rgba, ok, err := context.LookupColor("theme_selected_bg_color")
	if err != nil {
		return graphics.Color{}, err
	}
	if !ok {
		return graphics.Color{}, common.ErrSettingUnsupported
	}
	return graphics.Color{
		R: float32(rgba.Red),
		G: float32(rgba.Green),
		B: float32(rgba.Blue),
		A: float32(rgba.Alpha),
	}, nil
}

func (Settings) FontFamily() (string, error) {
	family, _, err := queryFont()
	if err != nil {
		return "", err
	}
	if family == "" {
		return "", common.ErrSettingUnsupported
	}
	return family, nil
}

func (Settings) FontSize() (float32, error) {
	_, size, err := queryFont()
	if err != nil {
		return 0, err
	}
	if size <= 0 {
		return 0, common.ErrSettingUnsupported
	}
	return size, nil
}

func gtkSettings() (gtk.Settings, error) {
	gtkInit.once.Do(func() {
		gtkInit.ok, gtkInit.err = gtk.InitCheck()
		if gtkInit.err == nil && !gtkInit.ok {
			gtkInit.err = common.ErrSettingUnsupported
		}
	})
	if gtkInit.err != nil {
		return gtk.Settings{}, gtkInit.err
	}

	settings, err := gtk.SettingsGetDefault()
	if err != nil {
		return gtk.Settings{}, err
	}
	if settings.IsNull() {
		return gtk.Settings{}, common.ErrSettingUnsupported
	}
	return settings, nil
}

func queryFont() (family string, size float32, err error) {
	family, size, err = queryGTKFont()
	if err == nil && (family != "" || size > 0) {
		return family, size, nil
	}
	return queryPangoFont()
}

func queryGTKFont() (family string, size float32, err error) {
	settings, err := gtkSettings()
	if err != nil {
		return "", 0, err
	}

	fontName, err := settings.StringProperty("gtk-font-name")
	if err != nil {
		return "", 0, err
	}
	if fontName == "" {
		return "", 0, common.ErrSettingUnsupported
	}

	desc := pango.FontDescriptionFromString(fontName)
	if desc == 0 {
		return "", 0, common.ErrSettingUnsupported
	}
	defer desc.Free()
	return fontFromDescription(desc)
}

func queryPangoFont() (family string, size float32, err error) {
	fontMap := pango_cairo.FontMapNew()
	if fontMap.IsNull() {
		return "", 0, common.ErrSettingUnsupported
	}
	defer fontMap.Unref()

	context := fontMap.CreateContext()
	if context.IsNull() {
		return "", 0, common.ErrSettingUnsupported
	}
	defer context.Unref()

	desc := context.GetFontDescription()
	if desc == 0 {
		return "", 0, common.ErrSettingUnsupported
	}
	return fontFromDescription(desc)
}

func fontFromDescription(desc pango.FontDescription) (family string, size float32, err error) {
	family = desc.GetFamily()
	rawSize := desc.GetSize()
	if rawSize > 0 {
		size = float32(rawSize) / pango.Scale
	}
	if family == "" && size <= 0 {
		return "", 0, common.ErrSettingUnsupported
	}
	return family, size, nil
}
