package x11

import (
	"errors"
	"image/color"
	"strings"
	"sync"
	"time"

	"github.com/golang-gui/goui/platform/common"
	"github.com/golang-gui/goui/platform/graphics"
	"github.com/golang-gui/goui/platform/linux/libs/gtk"
	"github.com/golang-gui/goui/platform/linux/libs/pango"
	"github.com/golang-gui/goui/platform/linux/libs/pango_cairo"
)

type Settings struct {
	onChanged func()
}

var gtkInit struct {
	once sync.Once
	ok   bool
	err  error
}

func newSettings(onChanged func()) (common.Settings, error) {
	s := &Settings{onChanged: onChanged}
	if onChanged != nil {
		go s.watch()
	}
	return s, nil
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

func (s *Settings) watch() {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	prev := s.snapshot()
	for range ticker.C {
		next := s.snapshot()
		if next == prev {
			continue
		}
		prev = next
		s.onChanged()
	}
}

type settingsSnapshot struct {
	ColorScheme    common.ColorScheme
	ColorSchemeErr string
	AccentR        uint32
	AccentG        uint32
	AccentB        uint32
	AccentA        uint32
	AccentErr      string
	FontFamily     string
	FontFamilyErr  string
	FontSize       float32
	FontSizeErr    string
}

func (s *Settings) snapshot() settingsSnapshot {
	var snapshot settingsSnapshot

	snapshot.ColorScheme, snapshot.ColorSchemeErr = snapshotColorScheme(s.ColorScheme())
	snapshot.AccentR, snapshot.AccentG, snapshot.AccentB, snapshot.AccentA, snapshot.AccentErr = snapshotColor(s.AccentColor())
	snapshot.FontFamily, snapshot.FontFamilyErr = snapshotString(s.FontFamily())
	snapshot.FontSize, snapshot.FontSizeErr = snapshotFloat32(s.FontSize())

	return snapshot
}

func snapshotColorScheme(value common.ColorScheme, err error) (common.ColorScheme, string) {
	return value, snapshotError(err)
}

func snapshotColor(value color.Color, err error) (uint32, uint32, uint32, uint32, string) {
	if err != nil {
		return 0, 0, 0, 0, snapshotError(err)
	}
	if value == nil {
		return 0, 0, 0, 0, "<nil>"
	}
	r, g, b, a := value.RGBA()
	return r, g, b, a, ""
}

func snapshotString(value string, err error) (string, string) {
	return value, snapshotError(err)
}

func snapshotFloat32(value float32, err error) (float32, string) {
	return value, snapshotError(err)
}

func snapshotError(err error) string {
	if err == nil {
		return ""
	}
	if errors.Is(err, common.ErrSettingUnsupported) {
		return common.ErrSettingUnsupported.Error()
	}
	return err.Error()
}
