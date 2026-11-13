package x11

import (
	"fmt"
	"image/color"
	"strings"
	"sync"

	"github.com/golang-gui/goui/platform/common"
	"github.com/golang-gui/goui/platform/graphics"
	"github.com/golang-gui/goui/platform/linux/libs/glib"
	"github.com/golang-gui/goui/platform/linux/libs/gtk"
	"github.com/golang-gui/goui/platform/linux/libs/pango"
	"github.com/golang-gui/goui/platform/linux/libs/pango_cairo"
	"golang.org/x/text/language"
)

type Settings struct {
	platform *Platform
}

var gtkInit struct {
	once sync.Once
	ok   bool
	err  error
}

func newSettings(p *Platform) (common.Settings, error) {
	return &Settings{platform: p}, nil
}

func (s Settings) Composited() (bool, error) { return s.platform.composited() }

func (p *Platform) composited() (bool, error) {
	if p == nil || p.display == 0 || p.atoms._NET_WM_CM_Sn == 0 {
		return false, common.ErrUnavailable
	}
	// The atom's existence is not evidence of a compositor. Query ownership
	// each time so an explicit later check observes manager start/stop.
	return p.display.GetSelectionOwner(p.atoms._NET_WM_CM_Sn) != 0, nil
}

func (Settings) Language() (string, error) {
	names := glib.GetLanguageNames()
	if len(names) == 0 {
		return "", common.ErrSettingUnsupported
	}
	return localeLanguage(names[0])
}

// POSIX locale encoding/currency modifiers are not language subtags. Preserve
// language/script/region facts; do not turn C into an invented English setting.
func localeLanguage(locale string) (string, error) {
	base, modifier, _ := strings.Cut(locale, "@")
	base, _, _ = strings.Cut(base, ".")
	if base == "C" || base == "POSIX" {
		return "und", nil
	}
	tag, err := language.Parse(base)
	if err != nil {
		return "", fmt.Errorf("system UI locale %q: %w", locale, err)
	}
	if modifier != "" && modifier != "euro" {
		scripts := map[string]string{"latin": "Latn", "cyrillic": "Cyrl", "arabic": "Arab"}
		if script, ok := scripts[modifier]; ok {
			tag, err = language.Compose(tag, language.MustParseScript(script))
		} else {
			tag, err = language.Parse(tag.String() + "-" + modifier)
		}
		if err != nil {
			return "", fmt.Errorf("system UI locale %q: %w", locale, err)
		}
	}
	return tag.String(), nil
}

func (s Settings) settingString(name string) (string, error) {
	if s.platform == nil || s.platform.display == 0 {
		return "", common.ErrUnavailable
	}
	return s.platform.desktopSettings().string(name)
}

func (s Settings) ColorScheme() (common.ColorScheme, error) {
	name, err := s.settingString("Net/ThemeName")
	if err != nil {
		return 0, err
	}
	// Preserve the existing GTK theme-name convention. XSETTINGS has no
	// standardized independent light/dark preference field.
	if strings.Contains(strings.ToLower(name), "dark") {
		return common.ColorSchemeDark, nil
	}
	return common.ColorSchemeLight, nil
}

func (s Settings) AccentColor() (color.Color, error) {
	name, err := s.settingString("Net/ThemeName")
	if err != nil {
		return nil, err
	}
	if err := initGTK(); err != nil {
		return nil, err
	}
	// Providers are GTK-owned, keyed by name, and borrowed. The new local
	// context owns its provider reference; no GtkSettings object is modified.
	provider, err := gtk.CssProviderGetNamed(name, "")
	if err != nil {
		return nil, err
	}
	if provider.IsNull() {
		return nil, common.ErrSettingUnsupported
	}
	context, err := gtk.StyleContextNew()
	if err != nil {
		return nil, err
	}
	if context.IsNull() {
		return nil, common.ErrSettingUnsupported
	}
	defer context.Unref()
	if err := context.AddProvider(provider, gtk.StyleProviderPriorityApplication); err != nil {
		return nil, err
	}
	rgba, ok, err := context.LookupColor("theme_selected_bg_color")
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, common.ErrSettingUnsupported
	}
	return gtkAccentColor(rgba), nil
}

func gtkAccentColor(rgba gtk.RGBA) graphics.Color {
	// GdkRGBA is straight-alpha; graphics.Color is always premultiplied.
	a := float32(rgba.Alpha)
	return graphics.Color{R: float32(rgba.Red) * a, G: float32(rgba.Green) * a, B: float32(rgba.Blue) * a, A: a}
}

func (s Settings) FontFamily() (string, error) {
	family, _, err := s.queryFont()
	if err != nil {
		return "", err
	}
	if family == "" {
		return "", common.ErrSettingUnsupported
	}
	return family, nil
}

func (s Settings) FontSize() (float32, error) {
	_, size, err := s.queryFont()
	if err != nil {
		return 0, err
	}
	if size <= 0 {
		return 0, common.ErrSettingUnsupported
	}
	return size, nil
}

func initGTK() error {
	gtkInit.once.Do(func() {
		gtkInit.ok, gtkInit.err = gtk.InitCheck()
		if gtkInit.err == nil && !gtkInit.ok {
			gtkInit.err = common.ErrSettingUnsupported
		}
	})
	return gtkInit.err
}

func (s Settings) queryFont() (family string, size float32, err error) {
	name, err := s.settingString("Gtk/FontName")
	if err != nil {
		return queryPangoFont()
	}
	desc := pango.FontDescriptionFromString(name)
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
	if rawSize > 0 && !desc.GetSizeIsAbsolute() {
		size = float32(rawSize) / pango.Scale
	}
	if family == "" && size <= 0 {
		return "", 0, common.ErrSettingUnsupported
	}
	return family, size, nil
}
