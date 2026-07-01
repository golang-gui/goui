package common

import (
	"errors"
	"image/color"
)

var ErrSettingUnsupported = errors.New("platform setting unsupported")

type ColorScheme int

const (
	ColorSchemeLight ColorScheme = iota
	ColorSchemeDark
)

type Settings interface {
	ColorScheme() (ColorScheme, error)
	AccentColor() (color.Color, error)
	FontFamily() (string, error)
	FontSize() (float32, error)
}

type UnsupportedSettings struct{}

func (UnsupportedSettings) ColorScheme() (ColorScheme, error) {
	return 0, ErrSettingUnsupported
}

func (UnsupportedSettings) AccentColor() (color.Color, error) {
	return color.RGBA{}, ErrSettingUnsupported
}

func (UnsupportedSettings) FontFamily() (string, error) {
	return "", ErrSettingUnsupported
}

func (UnsupportedSettings) FontSize() (float32, error) {
	return 0, ErrSettingUnsupported
}
