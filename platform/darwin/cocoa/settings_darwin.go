package cocoa

import (
	"errors"
	"image/color"
	"strings"
	"time"

	"github.com/golang-gui/goui/platform/common"
	. "github.com/golang-gui/goui/platform/darwin/frameworks/appkit"
	. "github.com/golang-gui/goui/platform/darwin/frameworks/core_graphics"
	. "github.com/golang-gui/goui/platform/darwin/frameworks/foundation"
	"github.com/golang-gui/goui/platform/graphics"
)

type Settings struct {
	onChanged func()
}

func newSettings(onChanged func()) (common.Settings, error) {
	s := &Settings{onChanged: onChanged}
	if onChanged != nil {
		go s.watch()
	}
	return s, nil
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
