package common

import (
	"errors"
	"image/color"
)

type ColorScheme int

const (
	ColorSchemeLight ColorScheme = iota
	ColorSchemeDark
)

func (c ColorScheme) String() string {
	switch c {
	case ColorSchemeLight:
		return "light"
	case ColorSchemeDark:
		return "dark"
	default:
		return "unknown"
	}
}

type Settings interface {
	ColorScheme() (ColorScheme, error)
	AccentColor() (color.Color, error)
	FontFamily() (string, error)
	FontSize() (float32, error)
}

var ErrSettingUnsupported = errors.New("platform setting unsupported")
