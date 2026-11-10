package ios

import (
	"image/color"

	"github.com/golang-gui/goui/platform/common"
)

// Settings currently exposes only composition. Unintegrated system settings
// report unsupported so the GUI layer owns their fallback.
type Settings struct {
	platform *Platform
}

func (p *Platform) NewSettings() (common.Settings, error) {
	if p == nil || p.destroyed {
		return nil, common.ErrUnavailable
	}
	return &Settings{platform: p}, nil
}

func (s Settings) Composited() (bool, error) {
	if s.platform == nil || s.platform.destroyed {
		return false, common.ErrUnavailable
	}
	// UIKit uses the system layer compositor; the host's own alpha/opaque
	// configuration and transparent-window support remain separate contracts.
	return true, nil
}

func (Settings) Language() (string, error)                { return "", common.ErrSettingUnsupported }
func (Settings) ColorScheme() (common.ColorScheme, error) { return 0, common.ErrSettingUnsupported }
func (Settings) AccentColor() (color.Color, error)        { return nil, common.ErrSettingUnsupported }
func (Settings) FontFamily() (string, error)              { return "", common.ErrSettingUnsupported }
func (Settings) FontSize() (float32, error)               { return 0, common.ErrSettingUnsupported }
